// Package standins holds the two fakes of external systems: the search index
// and the notification sender. Each is reached only over the network and keeps
// its data in its own schema.
package standins

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

// The rules a guardrail says it must fail without (test-only build).
const (
	FaultNoVersion   = "search-no-version"     // G-006: apply every change
	FaultNoSentList  = "sender-no-sent-list"   // G-010: forget which names went out
	FaultSendNothing = "sender-sends-nothing"  // G-010
	FaultHoldArrival = "sender-hold-arrival"   // G-011: count the hold from arrival
	FaultAnyNoteTime = "sender-any-note-clock" // G-011: any note moves the clock
)

// SlowBy is how long the slow mode takes to answer.
const SlowBy = 10 * time.Second

// Options are what both stand-ins run with. Refuse is how a test makes a
// stand-in refuse one item; a program leaves it nil.
type Options struct {
	Dir    orgdb.Directory
	Clock  clock.Clock
	Refuse func(org string, number int64) string
}

// modes holds Running, Slow and Off, for all and per organization. It lives in
// the program: a restarted stand-in comes back running.
type modes struct {
	mu      sync.Mutex
	all     contract.Mode
	byOrg   map[string]contract.Mode
	changed chan struct{} // closed and replaced whenever a mode changes
}

func newModes() *modes {
	return &modes{all: "running", byOrg: map[string]contract.Mode{}, changed: make(chan struct{})}
}

func (m *modes) announce() {
	close(m.changed)
	m.changed = make(chan struct{})
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
	m.announce()
}

func (m *modes) setAll(v contract.Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.all = v
	m.byOrg = map[string]contract.Mode{}
	m.announce()
}

// gate applies the mode. Off never answers: the connection is dropped when the
// caller gives up or the mode changes, so nobody can read silence as success.
func (m *modes) gate(ctx context.Context, c clock.Clock, org string) bool {
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
		case <-c.After(SlowBy):
			if ctx.Err() != nil {
				panic(http.ErrAbortHandler)
			}
		case <-ctx.Done():
			panic(http.ErrAbortHandler)
		case <-changed:
			panic(http.ErrAbortHandler)
		}
	}
	return true
}

func validMode(v contract.Mode) bool { return v == "running" || v == "slow" || v == "off" }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func refuse(w http.ResponseWriter, status int, why string) {
	writeJSON(w, status, contract.Refusal{Refused: why})
}

func unavailable(w http.ResponseWriter, what string, err error) {
	slog.Error(what, "err", err)
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

// controlRoutes adds the mode switches and the wipes, the same on both.
func controlRoutes(mux *http.ServeMux, m *modes, dir orgdb.Directory, wipe func(context.Context, ids.Org) error) {
	readMode := func(w http.ResponseWriter, r *http.Request) (contract.Mode, bool) {
		var in contract.ModeBody
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || !validMode(in.Mode) {
			refuse(w, http.StatusBadRequest, "mode is running, slow or off")
			return "", false
		}
		return in.Mode, true
	}
	mux.HandleFunc("GET /orgs/{org}/control/mode", func(w http.ResponseWriter, r *http.Request) {
		if org, ok := orgOf(w, r); ok {
			writeJSON(w, http.StatusOK, contract.ModeBody{Mode: m.get(string(org))})
		}
	})
	mux.HandleFunc("POST /orgs/{org}/control/mode", func(w http.ResponseWriter, r *http.Request) {
		org, ok := orgOf(w, r)
		if !ok {
			return
		}
		if v, ok := readMode(w, r); ok {
			m.set(string(org), v)
			writeJSON(w, http.StatusOK, struct{}{})
		}
	})
	mux.HandleFunc("POST /control/mode", func(w http.ResponseWriter, r *http.Request) {
		if v, ok := readMode(w, r); ok {
			m.setAll(v)
			writeJSON(w, http.StatusOK, struct{}{})
		}
	})
	mux.HandleFunc("POST /orgs/{org}/control/wipe", func(w http.ResponseWriter, r *http.Request) {
		org, ok := orgOf(w, r)
		if !ok {
			return
		}
		if err := wipe(r.Context(), org); err != nil {
			unavailable(w, "wipe", err)
			return
		}
		writeJSON(w, http.StatusOK, struct{}{})
	})
	mux.HandleFunc("POST /control/wipe", func(w http.ResponseWriter, r *http.Request) {
		orgs, err := knownOrgs(r.Context(), dir)
		for _, org := range orgs {
			if err == nil {
				err = wipe(r.Context(), org)
			}
		}
		if err != nil {
			unavailable(w, "wipe all", err)
			return
		}
		writeJSON(w, http.StatusOK, struct{}{})
	})
}

// knownOrgs lists the organizations a stand-in holds, from its bookkeeping
// table, which has numbers only. The session is bound to a name no row uses, so
// no content table can answer through it.
func knownOrgs(ctx context.Context, d orgdb.Directory) ([]ids.Org, error) {
	var out []ids.Org
	err := orgdb.InOrg(ctx, d, "bookkeeping-only", func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, "SELECT org FROM state ORDER BY org")
		if err != nil {
			return err
		}
		names, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		for _, n := range names {
			out = append(out, ids.Org(n))
		}
		return nil
	})
	return out, err
}

// stamp is the one way a time is written in an answer.
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
