//go:build integration

package delivery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The insert delivery used before: it wrote the row, then waited for the log's lock.
const dueInsertLockingLate = `INSERT INTO delivery_due (receiver, org, since)
	SELECT $3, h.org, $2::timestamptz FROM org_log h JOIN delivery_places p ON p.org = h.org AND p.receiver = $3
	WHERE h.org = $1 AND h.head > p.place ON CONFLICT DO NOTHING`

func connectAs(t *testing.T, login, password string) *pgx.Conn {
	t.Helper()
	host := os.Getenv("CONTEXIA_PG_HOST")
	if host == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	c, err := pgx.Connect(context.Background(), fmt.Sprintf("postgres://%s:%s@%s:5432/contexia", login, password, host))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// race holds a save's lock on the log while delivery runs stmt, then lets the
// save mark the organization due, and reports whether PostgreSQL saw a deadlock.
func race(t *testing.T, org, stmt string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin := connectAs(t, "contexia_admin", "admin_dev")
	for _, q := range []string{
		`INSERT INTO conversations.org_log (org, head) VALUES ($1, 5)`,
		`INSERT INTO conversations.delivery_places (org, receiver, place) VALUES ($1, 'feed', 0)`,
	} {
		if _, err := admin.Exec(ctx, q, org); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = admin.Exec(bg, `DELETE FROM conversations.delivery_due WHERE org = $1`, org)
		_, _ = admin.Exec(bg, `DELETE FROM conversations.delivery_places WHERE org = $1`, org)
		_, _ = admin.Exec(bg, `DELETE FROM conversations.org_log WHERE org = $1`, org)
	})

	saver := connectAs(t, "conversations", "conversations_dev")
	worker := connectAs(t, "delivery", "delivery_dev")
	save, err := saver.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = save.Rollback(context.Background()) }()
	if _, err := save.Exec(ctx, `SELECT 1 FROM org_log WHERE org = $1 FOR UPDATE`, org); err != nil {
		t.Fatal(err)
	}

	workerDone := make(chan error, 1)
	go func() {
		_, err := worker.Exec(ctx, stmt, org, time.Now(), "feed")
		workerDone <- err
	}()
	for blocked := false; !blocked; {
		if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = $1 AND wait_event_type = 'Lock')`,
			worker.PgConn().PID()).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
	}

	_, saveErr := save.Exec(ctx, `INSERT INTO delivery_due (receiver, org, since)
		VALUES ('search', $1, now()), ('feed', $1, now()) ON CONFLICT DO NOTHING`, org)
	if saveErr == nil {
		saveErr = save.Commit(ctx)
	} else {
		_ = save.Rollback(ctx)
	}
	workerErr := <-workerDone

	deadlock := func(err error) bool {
		var pg *pgconn.PgError
		return errors.As(err, &pg) && pg.Code == "40P01"
	}
	if deadlock(saveErr) || deadlock(workerErr) {
		return true
	}
	if saveErr != nil || workerErr != nil {
		t.Fatalf("save: %v, worker: %v", saveErr, workerErr)
	}
	return false
}

func TestMarkingDueTakesTheLogLockFirst(t *testing.T) {
	if race(t, "lock-order-safe", dueInsertIfBehind) {
		t.Fatal("a save and delivery's re-check deadlocked")
	}
}

// The check can fail: the old order deadlocks in exactly this interleaving.
func TestFailsWhenTheRowIsWrittenBeforeTheLogLock(t *testing.T) {
	if !race(t, "lock-order-late", dueInsertLockingLate) {
		t.Fatal("the old order did not deadlock, so this test proves nothing")
	}
}
