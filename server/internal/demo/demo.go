// Package demo is the demonstration program: the scripted team, which acts only
// through the people door and the agent door, Reset, Rebuild, cleanup, and the one
// address the product, the control panel and the notification app are reached on.
// It alone knows which organization is current (D-033).
package demo

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"contexia/internal/democontent"
	"contexia/internal/shared/call"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config is what the program needs from outside.
type Config struct {
	Pool          *pgxpool.Pool // the demo schema, reached through orgdb
	Clock         clock.Clock
	Content       *democontent.Content
	Conversations string
	Delivery      string
	Search        string
	Sender        string
	Feed          string
	// SeedFirst seeds the first organization on start when there is none.
	SeedFirst bool
	// PrepareAhead keeps the next organization seeded and level, so Reset is immediate.
	PrepareAhead bool
	// LevelLimit is how long Reset waits for search and the feed; zero means 150 s.
	LevelLimit time.Duration
	// Gap is the pause between two situations of the schedule; zero means 20 s.
	Gap time.Duration
}

// Service is the running program.
type Service struct {
	cfg  Config
	conv *call.Client
	dlv  *call.Client
	recv map[string]*call.Client

	resetMu   sync.Mutex // one Reset at a time
	prepareMu sync.Mutex // one organization being seeded at a time

	teamsMu sync.Mutex
	teams   map[string]*team

	life context.Context // ends with Run, and with it every team and background seed
	end  context.CancelFunc
}

// New returns the program; Run starts its background work.
func New(cfg Config) *Service {
	if cfg.Clock == nil {
		cfg.Clock = clock.Default()
	}
	if cfg.LevelLimit <= 0 {
		cfg.LevelLimit = 150 * time.Second
	}
	if cfg.Gap <= 0 {
		cfg.Gap = 20 * time.Second
	}
	life, end := context.WithCancel(context.Background())
	return &Service{
		life: life,
		end:  end,
		cfg:  cfg,
		conv: call.New(cfg.Conversations, 0),
		dlv:  call.New(cfg.Delivery, 0),
		recv: map[string]*call.Client{
			"search": call.New(cfg.Search, 0),
			"sender": call.New(cfg.Sender, 0),
			"feed":   call.New(cfg.Feed, 0),
		},
		teams: map[string]*team{},
	}
}

// Run does what must happen without anyone asking: abandon a seed a crash left
// half done, tell Conversations about retirements, seed the first organization.
func (s *Service) Run(ctx context.Context) {
	defer s.end()
	if err := s.abandonUnfinished(ctx); err != nil {
		slog.Error("demo: cannot read organizations", "err", err)
	}
	if s.cfg.SeedFirst {
		go s.firstOrganization(ctx)
	}
	for {
		s.tellRetired(ctx)
		select {
		case <-ctx.Done():
			return
		case <-s.cfg.Clock.After(5 * time.Second):
		}
	}
}

func (s *Service) firstOrganization(ctx context.Context) {
	for ctx.Err() == nil {
		if cur, err := s.current(ctx); err == nil && cur != nil {
			if s.cfg.PrepareAhead {
				s.prepareInBackground()
			}
			return
		}
		_, err := s.Reset(ctx)
		if err == nil {
			return
		}
		slog.Warn("demo: first organization not ready yet", "err", err)
		select {
		case <-ctx.Done():
		case <-s.cfg.Clock.After(2 * time.Second):
		}
	}
}

// Routes answers the demo operations of operations-and-control.md.
func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /current", s.handleCurrent)
	mux.HandleFunc("POST /reset", s.handleReset)
	mux.HandleFunc("GET /organizations", s.handleOrganizations)
	mux.HandleFunc("DELETE /organizations/{id}", s.handleDelete)
	mux.HandleFunc("POST /organizations/delete-retired", s.handleDeleteRetired)
	mux.HandleFunc("POST /orgs/{org}/rebuild", s.handleRebuild)
	mux.HandleFunc("GET /orgs/{org}/script", s.handleScript)
	mux.HandleFunc("POST /orgs/{org}/script", s.handleCommand)
}

func answer(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func notAvailable(w http.ResponseWriter, err error) {
	slog.Warn("demo: not available", "err", err)
	answer(w, http.StatusServiceUnavailable, map[string]string{"not_available": err.Error()})
}

func orgParam(w http.ResponseWriter, r *http.Request, name string) (ids.Org, bool) {
	org, err := ids.ParseOrg(r.PathValue(name))
	if err != nil {
		answer(w, http.StatusNotFound, contract.Refusal{Refused: "unknown organization"})
		return "", false
	}
	return org, true
}

func (s *Service) handleCurrent(w http.ResponseWriter, r *http.Request) {
	cur, err := s.current(r.Context())
	switch {
	case err != nil:
		notAvailable(w, err)
	case cur == nil:
		answer(w, http.StatusServiceUnavailable, map[string]string{"not_available": "the first organization is being prepared"})
	default:
		answer(w, http.StatusOK, cur)
	}
}

func (s *Service) handleReset(w http.ResponseWriter, r *http.Request) {
	// A caller that gives up does not cancel a Reset halfway.
	cur, err := s.Reset(context.WithoutCancel(r.Context()))
	if err != nil {
		notAvailable(w, err)
		return
	}
	answer(w, http.StatusOK, cur)
}

func (s *Service) handleOrganizations(w http.ResponseWriter, r *http.Request) {
	list, err := s.organizations(r.Context())
	if err != nil {
		notAvailable(w, err)
		return
	}
	answer(w, http.StatusOK, contract.OrganizationList{Organizations: list})
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request) {
	org, ok := orgParam(w, r, "id")
	if !ok {
		return
	}
	err := s.Delete(context.WithoutCancel(r.Context()), org)
	var refused refusal
	switch {
	case errors.As(err, &refused):
		answer(w, http.StatusConflict, contract.Refusal{Refused: string(refused)})
	case err != nil:
		notAvailable(w, err)
	default:
		answer(w, http.StatusOK, contract.Empty{})
	}
}

func (s *Service) handleDeleteRetired(w http.ResponseWriter, r *http.Request) {
	deleted, err := s.DeleteRetired(context.WithoutCancel(r.Context()))
	if err != nil {
		answer(w, http.StatusServiceUnavailable, map[string]any{"not_available": err.Error(), "deleted": deleted})
		return
	}
	answer(w, http.StatusOK, map[string]any{"deleted": deleted})
}

func (s *Service) handleRebuild(w http.ResponseWriter, r *http.Request) {
	org, ok := orgParam(w, r, "org")
	if !ok {
		return
	}
	var in contract.RebuildRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		answer(w, http.StatusBadRequest, contract.Refusal{Refused: "unreadable body"})
		return
	}
	if _, known := s.recv[string(in.Receiver)]; !known {
		answer(w, http.StatusBadRequest, contract.Refusal{Refused: "no such receiver"})
		return
	}
	if err := s.Rebuild(context.WithoutCancel(r.Context()), org, string(in.Receiver), in.RulesChanged); err != nil {
		notAvailable(w, err)
		return
	}
	answer(w, http.StatusOK, contract.Empty{})
}

// retrying repeats a call that is safe to repeat while the other side is out,
// until the budget runs out. A refusal is final.
func (s *Service) retrying(ctx context.Context, budget time.Duration, fn func() error) error {
	deadline := s.cfg.Clock.Now().Add(budget)
	wait := 200 * time.Millisecond
	for {
		err := fn()
		if err == nil || !errors.Is(err, call.ErrOutage) || !s.cfg.Clock.Now().Before(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.cfg.Clock.After(wait):
		}
		wait = min(wait*2, 2*time.Second)
	}
}
