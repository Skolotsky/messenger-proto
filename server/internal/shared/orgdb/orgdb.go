// Package orgdb is the only way to reach a database: "the database for this
// organization" (G-028), with the organization bound to the transaction so row
// rules apply (G-025).
package orgdb

import (
	"context"
	"fmt"

	"contexia/internal/shared/ids"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Setting is the session value row rules read:
// current_setting('app.org', true).
const Setting = "app.org"

// Directory says which database holds an organization. Today there is one
// (D-029); a second is a new Directory, not a change to callers.
type Directory interface {
	PoolFor(ctx context.Context, org ids.Org) (*pgxpool.Pool, error)
}

// Single is the directory of an installation with one database.
type Single struct{ Pool *pgxpool.Pool }

// PoolFor returns the only database.
func (s Single) PoolFor(context.Context, ids.Org) (*pgxpool.Pool, error) { return s.Pool, nil }

// InOrg runs fn in a transaction bound to org. The binding is local to the
// transaction, so it cannot reach the next user of a pooled connection. fn's
// error rolls everything back.
func InOrg(ctx context.Context, d Directory, org ids.Org, fn func(pgx.Tx) error) error {
	if _, err := ids.ParseOrg(string(org)); err != nil {
		return err
	}
	pool, err := d.PoolFor(ctx, org)
	if err != nil {
		return fmt.Errorf("database for %s: %w", org, err)
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", Setting, string(org)); err != nil {
			return fmt.Errorf("bind organization: %w", err)
		}
		return fn(tx)
	})
}
