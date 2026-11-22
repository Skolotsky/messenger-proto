// Command conversations is the conversations part of Contexia.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"contexia/internal/conversations"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/migrate"
	"contexia/internal/shared/orgdb"
	"contexia/internal/shared/part"
	"contexia/migrations"
)

func address(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return fallback
}

func main() {
	ctx := context.Background()
	db, err := orgdb.Open(ctx)
	if err != nil {
		slog.Error("no database", "err", err)
		os.Exit(1)
	}
	if err := migrate.Apply(ctx, db.Pool, migrations.Files, "conversations"); err != nil {
		slog.Error("migrations failed", "err", err)
		os.Exit(1)
	}
	store := &conversations.Store{Dir: db, Clock: clock.Default()}
	mux := http.NewServeMux()
	store.Routes(mux)
	live := conversations.NewLive(store, conversations.LiveOptions{
		Feed:    address("CONTEXIA_FEED_URL", "http://feed:8080"),
		Search:  address("CONTEXIA_SEARCH_URL", "http://search:8080"),
		SelfURL: os.Getenv("CONTEXIA_CONVERSATIONS_URL"),
	})
	live.Routes(mux)
	go live.Run(ctx)
	part.Serve("conversations", mux)
}
