// Package migrate brings a part's own schema up to date when it starts.
package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Apply runs the part's files in name order, each once and in its own
// transaction. Copies starting together wait for one another.
func Apply(ctx context.Context, pool *pgxpool.Pool, files fs.FS, part string) error {
	names, err := fs.Glob(files, path.Join(part, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(names)
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(hashtext($1))", "migrate:"+part); err != nil {
		return err
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock(hashtext($1))", "migrate:"+part)
	}()
	table := pgx.Identifier{part, "schema_migrations"}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE TABLE IF NOT EXISTS "+table+" (name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return fmt.Errorf("migrations table: %w", err)
	}
	for _, name := range names {
		var done bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+table+" WHERE name = $1)", name).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		sql, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			// No arguments: the simple protocol, so a file may hold several statements.
			if _, err := tx.Exec(ctx, string(sql), pgx.QueryExecModeSimpleProtocol); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, "INSERT INTO "+table+" (name) VALUES ($1)", name)
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}
