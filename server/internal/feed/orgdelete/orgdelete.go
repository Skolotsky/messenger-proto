// Package orgdelete is the feed's part of deleting an organization (D-033): every
// table of the service and the note log. The note tables the delivery workers
// keep (due, places, parked) are removed by
// delivery's own delete.
package orgdelete

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	"contexia/internal/shared/orgwipe"

	"github.com/jackc/pgx/v5"
)

// Tables are the feed's own tables that hold an organization.
var Tables = []string{"feed_rows", "feed_readers", "feed_held", "feed_row_updates", "feed_fences", "feed_journal", "feed_state", "note_log", "note_heads"}

// Delete removes every row of org from the feed's tables. Repeating it changes
// nothing.
func Delete(ctx context.Context, dir orgdb.Directory, org ids.Org) error {
	return orgdb.InOrg(ctx, dir, org, func(tx pgx.Tx) error {
		return orgwipe.Delete(ctx, tx, org, Tables...)
	})
}

// Routes answers DELETE /orgs/{org}.
func Routes(dir orgdb.Directory) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /orgs/{org}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		org, err := ids.ParseOrg(r.PathValue("org"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(contract.Refusal{Refused: "not an organization"})
			return
		}
		if err := Delete(r.Context(), dir, org); err != nil {
			slog.Error("delete organization", "org", org, "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "try again"})
			return
		}
		_ = json.NewEncoder(w).Encode(contract.Empty{})
	})
	return mux
}
