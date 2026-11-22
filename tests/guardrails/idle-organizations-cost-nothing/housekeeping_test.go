package idle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"contexia/internal/conversations"
	"contexia/internal/shared/migrate"
	"contexia/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// housekeepingWide are the tables housekeeping's questions could read in
// proportion to the organizations that exist.
var housekeepingWide = map[string]bool{"org_log": true, "copy_leases": true}

const housekeepingBusy = 3

func rowsRead(node map[string]any) float64 {
	var read float64
	if rel, _ := node["Relation Name"].(string); housekeepingWide[rel] {
		loops, _ := node["Actual Loops"].(float64)
		rows, _ := node["Actual Rows"].(float64)
		removed, _ := node["Rows Removed by Filter"].(float64)
		read += (rows + removed) * loops
	}
	if kids, ok := node["Plans"].([]any); ok {
		for _, k := range kids {
			read += rowsRead(k.(map[string]any))
		}
	}
	return read
}

func askHousekeeping(t *testing.T, pool *pgxpool.Pool, q conversations.Question) float64 {
	t.Helper()
	tx, err := pool.BeginTx(context.Background(), pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	// On a table this small PostgreSQL rightly prefers to read it all; beside
	// 20,000 organizations it would not. Taking that choice away shows whether the
	// question can reach its rows without the idle ones: one that cannot still
	// reads every row, which is what the planted fault does.
	if _, err := tx.Exec(context.Background(), "SET LOCAL enable_seqscan = off"); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := tx.QueryRow(context.Background(), "EXPLAIN (ANALYZE, FORMAT JSON) "+q.SQL, q.Args...).Scan(&raw); err != nil {
		t.Fatalf("%s: %v", q.Name, err)
	}
	var plans []map[string]any
	if err := json.Unmarshal([]byte(raw), &plans); err != nil {
		t.Fatal(err)
	}
	return rowsRead(plans[0]["Plan"].(map[string]any))
}

// TestHousekeepingReadsNoIdleOrganization asks housekeeping's timer questions
// beside 50 and beside 500 idle organizations, with three that have a tick due.
// Judged by rows read, which does not depend on how busy the machine is.
func TestHousekeepingReadsNoIdleOrganization(t *testing.T) {
	host := os.Getenv("CONTEXIA_PG_HOST")
	if host == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://conversations:conversations_dev@%s:5432/contexia", host))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.Apply(ctx, pool, migrations.Files, "conversations"); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	prefix := "hk-" + hex.EncodeToString(b)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM org_log WHERE org LIKE $1`, prefix+"-%")
	})
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO org_log (org, head, head_at, last_change_at, tick_due_at)
		SELECT $1 || '-due-' || n, 40, $2::timestamptz, $2::timestamptz, $2::timestamptz - interval '1 second' FROM generate_series(1, $3::int) n`,
		prefix, now, housekeepingBusy)
	addIdle := func(from, to int) {
		exec(`INSERT INTO org_log (org, head, head_at, last_change_at)
			SELECT $1 || '-idle-' || n, 40, $2::timestamptz - interval '1 day', $2::timestamptz - interval '1 day'
			FROM generate_series($3::int, $4::int) n`, prefix, now, from, to)
		exec(`ANALYZE org_log`)
	}
	addIdle(1, 50)
	small := map[string]float64{}
	for _, q := range conversations.HousekeepingQuestions(now) {
		small[q.Name] = askHousekeeping(t, pool, q)
	}
	addIdle(51, 500)
	for _, q := range conversations.HousekeepingQuestions(now) {
		large := askHousekeeping(t, pool, q)
		t.Logf("%-34s rows read %.0f beside 50 idle, %.0f beside 500", q.Name, small[q.Name], large)
		// Other tests' organizations with a tick due may sit in the same database,
		// so the bound is on growth with the idle ones, and on an absolute handful.
		if large > small[q.Name]+float64(housekeepingBusy) {
			t.Errorf("%s read %.0f rows beside 500 idle organizations and %.0f beside 50: it reads the idle ones (G-029)", q.Name, large, small[q.Name])
		}
		if large > 120 {
			t.Errorf("%s read %.0f rows with %d organizations due: it must reach them through its index (G-029)", q.Name, large, housekeepingBusy)
		}
	}
}
