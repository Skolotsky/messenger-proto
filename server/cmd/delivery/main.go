// Command delivery is the delivery part of Contexia: one lane per receiver,
// feeding search and the feed from the change logs and the notification sender
// from the feed's note logs.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"contexia/internal/delivery"
	"contexia/internal/shared/orgdb"
	"contexia/internal/shared/part"
)

func address(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return fallback
}

func main() {
	db, err := orgdb.Open(context.Background())
	if err != nil {
		slog.Error("no database", "err", err)
		os.Exit(1)
	}
	s := delivery.New(delivery.Config{
		DB:            delivery.OneDatabase{Single: db},
		Search:        address("CONTEXIA_SEARCH_URL", "http://search:8080"),
		Feed:          address("CONTEXIA_FEED_URL", "http://feed:8080"),
		Sender:        address("CONTEXIA_SENDER_URL", "http://sender:8080"),
		Conversations: address("CONTEXIA_CONVERSATIONS_URL", "http://conversations:8080"),
		Seed:          time.Now().UnixNano(),
	})
	go s.Run(context.Background())
	part.Serve("delivery", s.Routes())
}
