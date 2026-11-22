//go:build integration

package orgdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"contexia/internal/shared/ids"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// onePool has a single connection, so the "next user" really is the same session.
func onePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("CONTEXIA_PG_HOST")
	if host == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	cfg, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://feed:feed_dev@%s:5432/contexia?pool_max_conns=1", host))
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func bound(t *testing.T, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) string {
	t.Helper()
	var v *string
	if err := q.QueryRow(context.Background(), "SELECT current_setting('app.org', true)").Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v == nil {
		return ""
	}
	return *v
}

func TestOrganizationIsBoundAndDoesNotLeak(t *testing.T) {
	ctx := context.Background()
	pool := onePool(t)
	dir := Single{Pool: pool}

	err := InOrg(ctx, dir, "acme", func(tx pgx.Tx) error {
		if got := bound(t, tx); got != "acme" {
			t.Errorf("inside: bound to %q", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := bound(t, pool); got != "" {
		t.Errorf("after commit the pooled session is still bound to %q", got)
	}

	boom := errors.New("boom")
	if err := InOrg(ctx, dir, "acme", func(pgx.Tx) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("want the function's error, got %v", err)
	}
	if got := bound(t, pool); got != "" {
		t.Errorf("after rollback the pooled session is still bound to %q", got)
	}
}

func TestMalformedOrganizationNeverReachesTheDatabase(t *testing.T) {
	called := false
	err := InOrg(context.Background(), Single{Pool: onePool(t)}, ids.Org("x'; drop"), func(pgx.Tx) error {
		called = true
		return nil
	})
	if err == nil || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}
