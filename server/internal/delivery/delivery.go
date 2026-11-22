// Package delivery feeds every receiver from the logs: search and the feed from
// each organization's change log, the notification sender from the feed's note
// log. Each receiver has its own lane and its own place per organization, so
// one that is slow, stopped or refusing delays nothing else (D-006, D-028).
//
// A lane finds work through the due tables, which hold only organizations that
// have some (G-029); claims a few for a few seconds; delivers outside any
// transaction; and notes the new place. A claim saves work and is not what
// keeps delivery correct: receivers ignore repeats (D-029).
package delivery

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"contexia/internal/shared/call"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Limits of one turn. A turn stops starting calls at half the claim, and a call
// lasts at most 3 s, so a healthy turn always ends inside its claim (G-027).
const (
	claimFor    = 8 * time.Second
	pollEvery   = 25 * time.Millisecond
	slots       = 16
	batchSize   = 100
	turnBudget  = 200
	triesToPark = 3
	refusalsTo  = 3                // different changes refused in a row that pause an organization
	maxBackoff  = 20 * time.Second // D-035: a receiver back can be level within G-006's 30 s
	probeFor    = call.Limit + time.Second
)

// Receiver names, as the contracts and tables spell them.
const (
	Search = "search"
	Feed   = "feed"
	Sender = "sender"
)

// Databases is the directory, plus the list of databases a lane asks "which
// organizations have work?" (D-029: one question per database).
type Databases interface {
	orgdb.Directory
	All() []*pgxpool.Pool
}

// OneDatabase is the installation with a single database.
type OneDatabase struct{ orgdb.Single }

// All returns the only database.
func (d OneDatabase) All() []*pgxpool.Pool { return []*pgxpool.Pool{d.Pool} }

// Config is what a Service needs.
type Config struct {
	DB            Databases
	Clock         clock.Clock
	Search        string // base addresses of the parts
	Feed          string
	Sender        string
	Conversations string
	Seed          int64  // for the jitter of retries
	Worker        string // this copy's name in claims; random when empty
	// ClaimFor shortens the claim for a test, so a receiver slower than the claim
	// takes less than a minute to show. It must stay longer than any call the test
	// makes. Zero is the real value.
	ClaimFor time.Duration
}

// Service runs one lane per receiver and database.
type Service struct {
	cfg    Config
	me     string
	lanes  map[string]*lane
	nudger *call.Client

	mu  sync.Mutex
	rng *rand.Rand

	nudging sync.Map
}

// New builds a Service. Run starts it; Routes answers its operations.
func New(cfg Config) *Service {
	if cfg.Clock == nil {
		cfg.Clock = clock.Default()
	}
	if cfg.ClaimFor == 0 {
		cfg.ClaimFor = claimFor
	}
	me := cfg.Worker
	if me == "" {
		me = ids.New("worker")
	}
	s := &Service{cfg: cfg, me: me, rng: rand.New(rand.NewSource(cfg.Seed)), lanes: map[string]*lane{}}
	s.lanes[Search] = &lane{s: s, name: Search, rc: newClient(Search, cfg.Search)}
	s.lanes[Feed] = &lane{s: s, name: Feed, rc: newClient(Feed, cfg.Feed)}
	s.lanes[Sender] = &lane{s: s, name: Sender, notes: true, rc: newClient(Sender, cfg.Sender)}
	if cfg.Conversations != "" {
		s.nudger = call.New(cfg.Conversations, time.Second)
	}
	return s
}

// Run delivers until ctx ends.
func (s *Service) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, l := range s.lanes {
		for _, pool := range s.cfg.DB.All() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				l.run(ctx, pool)
			}()
		}
	}
	wg.Wait()
}

// backoff is the wait before try number failures+1: 1, 2, 4 … up to 20 s, each
// varied by up to a fifth either way so retries do not arrive in step (D-035).
func (s *Service) backoff(failures int) time.Duration {
	d := maxBackoff
	if failures <= 5 {
		d = min(time.Second<<max(failures-1, 0), maxBackoff)
	}
	s.mu.Lock()
	j := 0.8 + 0.4*s.rng.Float64()
	s.mu.Unlock()
	return min(time.Duration(float64(d)*j), maxBackoff)
}

// nudge tells Conversations this organization has news. It carries nothing and
// may be lost, so it never holds up delivery, and an organization has at most
// one on the way.
func (s *Service) nudge(org ids.Org) {
	if s.nudger == nil {
		return
	}
	if _, busy := s.nudging.LoadOrStore(org, true); busy {
		return
	}
	go func() {
		defer s.nudging.Delete(org)
		if err := s.nudger.JSON(context.Background(), "POST", "/orgs/"+string(org)+"/nudge", struct{}{}, nil); err != nil {
			slog.Debug("nudge lost", "org", org, "err", err)
		}
	}()
}
