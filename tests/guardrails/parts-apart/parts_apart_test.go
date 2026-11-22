// Package partsapart holds the first half of G-008: each part's login is
// refused on every other part's schema. The second half (the feed rebuilt with
// Conversations unreachable) belongs to T-052.
package partsapart

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var logins = []string{"conversations", "delivery", "feed", "search", "sender", "demo"}
var schemas = []string{"conversations", "feed", "search", "sender", "demo"}

const insufficientPrivilege = "42501"

func connect(t *testing.T, user, password string) *pgx.Conn {
	t.Helper()
	host := os.Getenv("CONTEXIA_PG_HOST")
	if host == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	url := fmt.Sprintf("postgres://%s:%s@%s:5432/contexia", user, password, host)
	c, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("connect as %s: %v", user, err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func exec(t *testing.T, c *pgx.Conn, sql string) {
	t.Helper()
	if _, err := c.Exec(context.Background(), sql); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// reachable lists what login can do in schema; empty means fully refused.
func reachable(t *testing.T, c *pgx.Conn, schema string) []string {
	t.Helper()
	var got []string
	for what, sql := range map[string]string{
		"read":   "SELECT * FROM " + schema + ".parts_apart_probe",
		"write":  "INSERT INTO " + schema + ".parts_apart_probe VALUES (1)",
		"change": "UPDATE " + schema + ".parts_apart_probe SET x = 2",
		"remove": "DELETE FROM " + schema + ".parts_apart_probe",
		"create": "CREATE TABLE " + schema + ".parts_apart_intruder (x int)",
	} {
		tx, err := c.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(context.Background(), sql)
		_ = tx.Rollback(context.Background())
		var pgErr *pgconn.PgError
		switch {
		case err == nil:
			got = append(got, what)
		case errors.As(err, &pgErr) && pgErr.Code == insufficientPrivilege:
		default:
			t.Fatalf("%s: expected a permission refusal, got %v", sql, err)
		}
	}
	return got
}

func withProbes(t *testing.T) *pgx.Conn {
	t.Helper()
	admin := connect(t, "contexia_admin", "admin_dev")
	for _, s := range schemas {
		exec(t, admin, "CREATE TABLE "+s+".parts_apart_probe (x int)")
		exec(t, admin, "ALTER TABLE "+s+".parts_apart_probe OWNER TO "+s)
	}
	t.Cleanup(func() {
		for _, s := range schemas {
			_, _ = admin.Exec(context.Background(), "DROP TABLE IF EXISTS "+s+".parts_apart_probe")
		}
	})
	return admin
}

func TestEachLoginIsRefusedOnEveryOtherSchema(t *testing.T) {
	withProbes(t)
	for _, login := range logins {
		c := connect(t, login, login+"_dev")
		for _, schema := range schemas {
			got := reachable(t, c, schema)
			if login == schema {
				if len(got) != 5 {
					t.Errorf("%s should have full use of its own schema, has %v", login, got)
				}
				continue
			}
			if len(got) != 0 {
				t.Errorf("%s can %v in schema %s. Each part reaches only its own store (G-008)", login, got, schema)
			}
		}
	}
}

// The check above must be able to fail: with one login shared across parts, it
// has to see the access.
func TestTheCheckSeesASharedLogin(t *testing.T) {
	admin := withProbes(t)
	exec(t, admin, "GRANT search TO feed")
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "REVOKE search FROM feed") })

	c := connect(t, "feed", "feed_dev")
	if got := reachable(t, c, "search"); len(got) == 0 {
		t.Fatal("feed was given the search login's rights, yet the check saw nothing")
	}
}
