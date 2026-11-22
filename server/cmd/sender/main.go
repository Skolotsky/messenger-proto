// Command sender is the notification sender stand-in of Contexia.
package main

import (
	"context"
	"log/slog"
	"os"

	"contexia/internal/shared/clock"
	"contexia/internal/shared/migrate"
	"contexia/internal/shared/orgdb"
	"contexia/internal/shared/part"
	"contexia/internal/standins"
	"contexia/migrations"
)

func main() {
	ctx := context.Background()
	dir, err := orgdb.Open(ctx)
	if err == nil {
		err = migrate.Apply(ctx, dir.Pool, migrations.Files, "sender")
	}
	if err != nil {
		slog.Error("cannot start", "part", "sender", "err", err)
		os.Exit(1)
	}
	part.Serve("sender", standins.NewSender(standins.Options{Dir: dir, Clock: clock.Default()}).Routes())
}
