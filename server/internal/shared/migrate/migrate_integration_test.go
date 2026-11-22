//go:build integration

package migrate

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"
)

func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("CONTEXIA_PG_HOST")
	if host == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	p, err := pgxpool.New(context.Background(), fmt.Sprintf("postgres://demo:demo_dev@%s:5432/contexia", host))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func TestRunsOnceInOrderEvenFromTwoCopies(t *testing.T) {
	ctx := context.Background()
	p := pool(t)
	t.Cleanup(func() {
		_, _ = p.Exec(ctx, "DROP TABLE IF EXISTS demo.migrate_probe; DELETE FROM demo.schema_migrations WHERE name LIKE 'demo/9%'")
	})
	files := fstest.MapFS{
		"demo/900_a.sql": {Data: []byte("CREATE TABLE demo.migrate_probe (x int); INSERT INTO demo.migrate_probe VALUES (1);")},
		"demo/901_b.sql": {Data: []byte("INSERT INTO demo.migrate_probe VALUES (2);")},
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- Apply(ctx, p, files, "demo") }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var n, sum int
	if err := p.QueryRow(ctx, "SELECT count(*), sum(x) FROM demo.migrate_probe").Scan(&n, &sum); err != nil {
		t.Fatal(err)
	}
	if n != 2 || sum != 3 {
		t.Fatalf("rows=%d sum=%d", n, sum)
	}
}

func TestAFailedFileLeavesNothingBehind(t *testing.T) {
	ctx := context.Background()
	p := pool(t)
	t.Cleanup(func() { _, _ = p.Exec(ctx, "DROP TABLE IF EXISTS demo.migrate_half") })
	files := fstest.MapFS{"demo/910_bad.sql": {Data: []byte("CREATE TABLE demo.migrate_half (x int); SELECT nonsense;")}}
	if err := Apply(ctx, p, files, "demo"); err == nil {
		t.Fatal("a broken file was accepted")
	}
	var exists bool
	if err := p.QueryRow(ctx, "SELECT to_regclass('demo.migrate_half') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("half of a failed file was kept")
	}
}
