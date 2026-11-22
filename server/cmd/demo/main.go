// Command demo is the demonstration program of Contexia: the scripted team, Reset,
// Rebuild, cleanup and the one address the three pages are reached on.
package main

import (
	"context"
	"log/slog"
	"os"

	"contexia/internal/demo"
	"contexia/internal/democontent"
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
	if err == nil {
		err = migrate.Apply(ctx, db.Pool, migrations.Files, "demo")
	}
	var content *democontent.Content
	if err == nil {
		content, err = democontent.Load(os.DirFS(address("CONTEXIA_CONTENT", "/content")))
	}
	if err != nil {
		slog.Error("cannot start", "part", "demo", "err", err)
		os.Exit(1)
	}
	on := os.Getenv("CONTEXIA_DEMO_SEED_FIRST") == "1"
	s := demo.New(demo.Config{
		Pool:          db.Pool,
		Clock:         clock.Default(),
		Content:       content,
		Conversations: address("CONTEXIA_CONVERSATIONS_URL", "http://conversations:8080"),
		Delivery:      address("CONTEXIA_DELIVERY_URL", "http://delivery:8080"),
		Search:        address("CONTEXIA_SEARCH_URL", "http://search:8080"),
		Sender:        address("CONTEXIA_SENDER_URL", "http://sender:8080"),
		Feed:          address("CONTEXIA_FEED_URL", "http://feed:8080"),
		SeedFirst:     on,
		PrepareAhead:  on,
	})
	go s.Run(ctx)
	part.Serve("demo", s.Front(os.Getenv("CONTEXIA_WEB_DIR")))
}
