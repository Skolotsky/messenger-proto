// Command feed is the activity feed part of Contexia (T-052).
package main

import (
	"context"
	"log/slog"
	"os"

	"contexia/internal/feed/service"
	"contexia/internal/shared/clock"
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
	part.Serve("feed", service.New(service.Options{Dir: db, Clock: clock.Default()}).Routes())
}
