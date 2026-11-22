// Package part is what every program shares at start: its name, its address
// and the "alive" answer.
package part

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"contexia/internal/shared/clock"
)

// Handler answers GET /alive with the part's name.
func Handler(name string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /alive", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"part": name, "status": "alive"})
	})
	if h := clock.ControlHandler(); h != nil {
		mux.Handle("/test/clock", h)
	}
	return mux
}

// Run serves the part until the process is told to stop.
func Run(name string) {
	addr := os.Getenv("CONTEXIA_LISTEN")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{Addr: addr, Handler: Handler(name), ReadHeaderTimeout: 3 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	slog.Info("started", "part", name, "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("stopped", "part", name, "err", err)
		os.Exit(1)
	}
}
