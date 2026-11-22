// Package orgwipe removes one organization's rows from a part's own tables
// (D-033, G-031). Every part's delete goes through it, so the rule "only this
// organization" is written once and its must-fail build can break it once.
package orgwipe

import (
	"context"
	"fmt"
	"strings"

	"contexia/internal/shared/ids"

	"github.com/jackc/pgx/v5"
)

// Delete removes org's rows from each table, in the order given. Run it inside
// the transaction bound to org, so the part's delete commits whole or not at all
// and a repeat after a crash finds either everything or nothing.
func Delete(ctx context.Context, tx pgx.Tx, org ids.Org, tables ...string) error {
	for _, table := range tables {
		if fault("leave-table:" + table[strings.LastIndex(table, ".")+1:]) {
			continue
		}
		sql, args := "DELETE FROM "+table+" WHERE org = $1", []any{string(org)}
		if fault("ignore-org") {
			sql, args = "DELETE FROM "+table, nil
		}
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return fmt.Errorf("delete from %s: %w", table, err)
		}
	}
	return nil
}
