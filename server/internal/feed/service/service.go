// Package service is the activity feed as a running part. It takes the log's
// changes strictly in order, applies the pure rules, and saves rows, counts,
// held notifications, notes and its place in one transaction. Everything it
// holds can be wiped and worked out again from the log; it never calls
// Conversations and never reads its schema (D-017, G-008).
package service

import (
	"container/list"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"contexia/internal/feed/rules"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
)

// The faults a guardrail must fail with (test-only build, tag feedfaults).
const (
	FaultAcceptNewer = "accept-any-newer" // G-007: take a change that does not follow the last one
	FaultNoCatchUp   = "no-catch-up"      // G-027: a copy trusts its memory instead of the journal
	FaultFenceHeld   = "fence-keeps-held" // G-009: a fence leaves held notifications
	FaultNoSetAside  = "no-set-aside"     // G-009: later changes inside a fence are applied
	FaultNoOrgStop   = "no-org-stop"      // G-009: more than five fences do not stop the organization
	// FaultDieBeforeCommit is not a broken rule but an injected failure: every
	// third save dies after writing everything and before committing.
	FaultDieBeforeCommit = "die-before-commit"
)

// maxFences is how many fences an organization may have (conversations-to-feed.md).
const maxFences = 5

// maxBatch is the largest batch the contract allows.
const maxBatch = 100

// slowBy is how long the slow mode takes to answer.
const slowBy = 10 * time.Second

// keepEngines is how many organizations' rules are kept in memory. One left out
// is replayed from its journal when next needed.
const keepEngines = 128

// Options are what the service runs with.
type Options struct {
	Dir   orgdb.Directory
	Clock clock.Clock
}

// Service is one copy of the feed. Any number can run side by side: the row
// lock on feed_state orders them and the journal brings each up to date.
type Service struct {
	opt   Options
	modes *modes
	dies  atomic.Int64 // only with FaultDieBeforeCommit

	mu   sync.Mutex
	mems map[ids.Org]*list.Element
	lru  *list.List
}

// New makes a copy of the feed.
func New(o Options) *Service {
	if o.Clock == nil {
		o.Clock = clock.Real{}
	}
	return &Service{opt: o, modes: newModes(), mems: map[ids.Org]*list.Element{}, lru: list.New()}
}

type fenceScope struct{ conversation, person string }

// mem is one organization's rules in this copy. It is only a cache of the
// journal: at any doubt it is dropped and replayed.
type mem struct {
	org ids.Org
	mu  sync.Mutex

	e              *rules.Engine
	edition        int64
	through        int64
	historyThrough int64
	fences         map[fenceScope]bool
	lastAt         time.Time
	touched        map[string]bool // readers the replay touched, whose summaries are not yet settled

	// What the store holds, loaded when first needed.
	shown      map[string]map[string]string // person -> row key -> fingerprint
	heads      map[string]int64             // person -> last row update
	held       map[string]bool
	cachesRead bool
}

func (m *mem) drop() {
	m.e, m.fences, m.shown, m.heads, m.held, m.touched = nil, nil, nil, nil, nil, nil
	m.through, m.edition, m.historyThrough, m.cachesRead = 0, 0, 0, false
	m.lastAt = time.Time{}
}

func (s *Service) memFor(org ids.Org) *mem {
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.mems[org]; ok {
		s.lru.MoveToFront(el)
		return el.Value.(*mem)
	}
	m := &mem{org: org}
	s.mems[org] = s.lru.PushFront(m)
	for el := s.lru.Back(); el != nil && s.lru.Len() > keepEngines; {
		prev := el.Prev()
		if o := el.Value.(*mem); o != m && o.mu.TryLock() {
			delete(s.mems, o.org)
			s.lru.Remove(el)
			o.mu.Unlock()
		}
		el = prev
	}
	return m
}

func (s *Service) forget(org ids.Org) {
	s.mu.Lock()
	if el, ok := s.mems[org]; ok {
		delete(s.mems, org)
		s.lru.Remove(el)
	}
	s.mu.Unlock()
	s.modes.forget(string(org))
}

// Routes is everything the feed answers.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /changes", s.change)
	mux.HandleFunc("POST /changes/batch", s.batch)
	mux.HandleFunc("GET /orgs/{org}/place", s.place)
	mux.HandleFunc("GET /orgs/{org}/people/{person}/rows", s.rows)
	mux.HandleFunc("GET /orgs/{org}/people/{person}/history", s.history)
	mux.HandleFunc("GET /orgs/{org}/people/{person}/settings", s.settings)
	mux.HandleFunc("GET /orgs/{org}/people/{person}/preview", s.preview)
	mux.HandleFunc("GET /orgs/{org}/updates", s.updates)
	mux.HandleFunc("DELETE /orgs/{org}", s.deleteOrg)
	mux.HandleFunc("GET /orgs/{org}/control/mode", s.modeRead)
	mux.HandleFunc("POST /orgs/{org}/control/mode", s.modeSet)
	mux.HandleFunc("POST /control/mode", s.modeSetAll)
	mux.HandleFunc("POST /orgs/{org}/control/wipe", s.wipe)
	mux.HandleFunc("POST /control/wipe", s.wipeAll)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func refuse(w http.ResponseWriter, status int, why string) {
	writeJSON(w, status, contract.Refusal{Refused: why})
}

func unavailable(w http.ResponseWriter, what string, err error) {
	slog.Error(what, "part", "feed", "err", err)
	http.Error(w, "unavailable", http.StatusInternalServerError)
}

func orgOf(w http.ResponseWriter, r *http.Request) (ids.Org, bool) {
	org, err := ids.ParseOrg(r.PathValue("org"))
	if err != nil {
		refuse(w, http.StatusBadRequest, "not an organization")
		return "", false
	}
	return org, true
}

// modes holds running, slow and off, for all and per organization. It lives in
// the program: a restarted feed comes back running.
type modes struct {
	mu      sync.Mutex
	all     contract.Mode
	byOrg   map[string]contract.Mode
	changed chan struct{}
}

func newModes() *modes {
	return &modes{all: "running", byOrg: map[string]contract.Mode{}, changed: make(chan struct{})}
}

func (m *modes) get(org string) contract.Mode {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.byOrg[org]; ok {
		return v
	}
	return m.all
}

func (m *modes) set(org string, v contract.Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byOrg[org] = v
	close(m.changed)
	m.changed = make(chan struct{})
}

func (m *modes) setAll(v contract.Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.all, m.byOrg = v, map[string]contract.Mode{}
	close(m.changed)
	m.changed = make(chan struct{})
}

func (m *modes) forget(org string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.byOrg, org)
}

// gate applies the mode. Off never answers: the connection is dropped when the
// caller gives up or the mode changes.
func (m *modes) gate(ctx context.Context, c clock.Clock, org string) {
	m.mu.Lock()
	mode, ok := m.byOrg[org]
	if !ok {
		mode = m.all
	}
	changed := m.changed
	m.mu.Unlock()
	switch mode {
	case "off":
		select {
		case <-ctx.Done():
		case <-changed:
		}
		panic(http.ErrAbortHandler)
	case "slow":
		select {
		case <-c.After(slowBy):
			if ctx.Err() != nil {
				panic(http.ErrAbortHandler)
			}
		case <-ctx.Done():
			panic(http.ErrAbortHandler)
		case <-changed:
			panic(http.ErrAbortHandler)
		}
	}
}
