// Package migrations checks the shared log tables load and that the delivery
// login gets exactly the rights G-008 names.
package migrations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"contexia/internal/shared/migrate"
	sql "contexia/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func pool(t *testing.T, login string) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("CONTEXIA_PG_HOST")
	if host == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	p, err := pgxpool.New(context.Background(), fmt.Sprintf("postgres://%s:%s_dev@%s:5432/contexia", login, login, host))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func refused(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "42501"
}

func TestDeliveryHasExactlyItsRights(t *testing.T) {
	ctx := context.Background()
	for _, part := range []string{"conversations", "feed"} {
		if err := migrate.Apply(ctx, pool(t, part), sql.Files, part); err != nil {
			t.Fatal(err)
		}
	}
	d := pool(t, "delivery")
	for _, q := range []string{
		"SELECT * FROM conversations.org_log", "SELECT * FROM conversations.delivery_places",
		"SELECT * FROM feed.note_heads", "SELECT * FROM feed.note_places",
	} {
		if _, err := d.Exec(ctx, q); err != nil {
			t.Errorf("delivery should be allowed: %s: %v", q, err)
		}
	}
	for _, q := range []string{
		"INSERT INTO conversations.change_log VALUES ('x', 1, now(), 'tick', '{}')",
		"DELETE FROM conversations.change_log",
		"UPDATE conversations.org_log SET head = 0",
		"INSERT INTO feed.note_log VALUES ('x', 1, 1, now(), 'clock', '{}')",
		"DELETE FROM feed.note_heads",
		"SELECT * FROM feed.schema_migrations",
		"SELECT * FROM conversations.schema_migrations",
	} {
		if _, err := d.Exec(ctx, q); !refused(err) {
			t.Errorf("delivery must be refused: %s (got %v)", q, err)
		}
	}
}

func TestLogRowsAreKeptApartByOrganization(t *testing.T) {
	ctx := context.Background()
	c := pool(t, "conversations")
	if err := migrate.Apply(ctx, c, sql.Files, "conversations"); err != nil {
		t.Fatal(err)
	}
	const a, b = "rls-test-a", "rls-test-b"
	t.Cleanup(func() {
		for _, o := range []string{a, b} {
			_ = pgx.BeginFunc(ctx, c, func(tx pgx.Tx) error {
				_, _ = tx.Exec(ctx, "SELECT set_config('app.org', $1, true)", o)
				_, _ = tx.Exec(ctx, "DELETE FROM conversations.change_log WHERE org = $1", o)
				_, err := tx.Exec(ctx, "DELETE FROM conversations.org_log WHERE org = $1", o)
				return err
			})
		}
	})
	in := func(org string, f func(pgx.Tx) error) error {
		return pgx.BeginFunc(ctx, c, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SELECT set_config('app.org', $1, true)", org); err != nil {
				return err
			}
			return f(tx)
		})
	}
	for _, o := range []string{a, b} {
		if err := in(o, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "INSERT INTO conversations.org_log (org) VALUES ($1)", o); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, "INSERT INTO conversations.change_log VALUES ($1, 1, now(), 'tick', '{}')", o)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := in(a, func(tx pgx.Tx) error {
		var n int
		// Deliberately forgets the organization: the row rule must hold it.
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM conversations.change_log WHERE org LIKE 'rls-test-%'").Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			t.Errorf("a session of %s sees %d log rows", a, n)
		}
		_, err := tx.Exec(ctx, "INSERT INTO conversations.change_log VALUES ($1, 2, now(), 'tick', '{}')", b)
		if err == nil || !strings.Contains(err.Error(), "row-level security") {
			t.Errorf("a session of %s wrote into %s: %v", a, b, err)
		}
		return errors.New("rollback")
	}); err == nil {
		t.Fatal("expected the rollback")
	}
}
