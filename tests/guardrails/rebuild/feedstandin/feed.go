// Package feedstandin answers what the feed's contract asks of a receiver, until
// the feed service (T-052) exists: it takes numbered changes in order, says where
// it is, can be wiped (raising its edition) and deleted. It decides nothing about
// rows; it remembers which change it applied, which is what G-020 compares.
package feedstandin

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
)

type orgState struct {
	through int64
	edition int64
	applied map[int64]string
	mode    string
}

// Feed is the stand-in.
type Feed struct {
	mu   sync.Mutex
	orgs map[string]*orgState
	off  bool
}

// New returns an empty stand-in.
func New() *Feed { return &Feed{orgs: map[string]*orgState{}} }

// SetOff makes it answer nothing useful, as a feed that is down.
func (f *Feed) SetOff(off bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.off = off
}

func (f *Feed) org(id string) *orgState {
	o, ok := f.orgs[id]
	if !ok {
		o = &orgState{edition: 1, applied: map[int64]string{}, mode: "running"}
		f.orgs[id] = o
	}
	return o
}

// Applied returns the changes held for an organization, by number.
func (f *Feed) Applied(id string) map[int64]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int64]string{}
	if o, ok := f.orgs[id]; ok {
		for n, k := range o.applied {
			out[n] = k
		}
	}
	return out
}

func send(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type head struct {
	Org    string `json:"org"`
	Number int64  `json:"number"`
	Kind   string `json:"kind"`
}

func (f *Feed) apply(raw json.RawMessage) (int, any) {
	var c head
	if err := json.Unmarshal(raw, &c); err != nil || c.Org == "" {
		return http.StatusBadRequest, map[string]string{"refused": "unreadable"}
	}
	o := f.org(c.Org)
	switch {
	case c.Number <= o.through:
		return http.StatusOK, map[string]bool{"applied": true}
	case c.Number != o.through+1:
		return http.StatusConflict, map[string]int64{"i_am_at": o.through, "edition": o.edition}
	}
	o.applied[c.Number] = c.Kind
	o.through = c.Number
	return http.StatusOK, map[string]bool{"applied": true}
}

// Routes is the feed's side of the contract.
func (f *Feed) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /alive", func(w http.ResponseWriter, _ *http.Request) {
		send(w, http.StatusOK, map[string]string{"part": "feed", "status": "alive"})
	})
	mux.HandleFunc("POST /changes", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.off {
			send(w, http.StatusServiceUnavailable, map[string]string{"not_available": "off"})
			return
		}
		status, out := f.apply(body)
		send(w, status, out)
	})
	mux.HandleFunc("POST /changes/batch", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Changes []json.RawMessage `json:"changes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			send(w, http.StatusBadRequest, map[string]string{"refused": "unreadable"})
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.off {
			send(w, http.StatusServiceUnavailable, map[string]string{"not_available": "off"})
			return
		}
		status, out := http.StatusOK, any(map[string]bool{"applied": true})
		for _, c := range in.Changes {
			if status, out = f.apply(c); status != http.StatusOK {
				break
			}
		}
		send(w, status, out)
	})
	mux.HandleFunc("GET /orgs/{org}/place", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		o := f.org(r.PathValue("org"))
		send(w, http.StatusOK, map[string]int64{"through": o.through, "edition": o.edition})
	})
	mux.HandleFunc("POST /orgs/{org}/control/wipe", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		o := f.org(r.PathValue("org"))
		o.through, o.applied = 0, map[int64]string{}
		o.edition++
		send(w, http.StatusOK, struct{}{})
	})
	mux.HandleFunc("POST /control/wipe", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, o := range f.orgs {
			o.through, o.applied = 0, map[int64]string{}
			o.edition++
		}
		send(w, http.StatusOK, struct{}{})
	})
	mux.HandleFunc("GET /orgs/{org}/control/mode", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		send(w, http.StatusOK, map[string]string{"mode": f.org(r.PathValue("org")).mode})
	})
	mux.HandleFunc("POST /orgs/{org}/control/mode", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Mode string `json:"mode"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.org(r.PathValue("org")).mode = in.Mode
		send(w, http.StatusOK, struct{}{})
	})
	mux.HandleFunc("DELETE /orgs/{org}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		delete(f.orgs, r.PathValue("org"))
		send(w, http.StatusOK, struct{}{})
	})
	return mux
}
