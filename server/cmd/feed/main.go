// Command feed is the feed part of Contexia. Until T-052 builds the service it
// only answers the removal of an organization's notes (D-033).
package main

import (
	"context"
	"log/slog"
	"os"

	"contexia/internal/feed/orgdelete"
	"contexia/internal/shared/migrate"
	"contexia/internal/shared/orgdb"
	"contexia/internal/shared/part"
	"contexia/migrations"
)

func main() {
	ctx := context.Background()
	db, err := orgdb.Open(ctx)
	if err == nil {
		err = migrate.Apply(ctx, db.Pool, migrations.Files, "feed")
	}
	if err != nil {
		slog.Error("cannot start", "part", "feed", "err", err)
		os.Exit(1)
	}
	part.Serve("feed", orgdelete.Routes(db))
}
