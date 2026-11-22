// Package router is the small router the G-027 tests put in front of copies of a
// program. The product's own entry point is not built (D-029): this one exists so a
// test decides which copy serves an organization and when that changes.
package router

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
)

// Mode says how a router picks a copy.
type Mode int

const (
	// ByOrg sends everything of one organization to one copy, as Conversations needs.
	ByOrg Mode = iota
	// Spread sends each call to the next copy, as a program without memory allows.
	Spread
)

// Router forwards calls, including live connections, to one of its copies.
type Router struct {
	mode    Mode
	proxies []*httputil.ReverseProxy
	turn    atomic.Uint64

	mu    sync.Mutex
	down  map[int]bool
	pins  map[string]int
	open  map[string]map[int64]context.CancelFunc
	next  int64
	Moves atomic.Int64
}

// New returns a router in front of the given base addresses.
func New(mode Mode, backends ...string) (*Router, error) {
	r := &Router{mode: mode, down: map[int]bool{}, pins: map[string]int{}, open: map[string]map[int64]context.CancelFunc{}}
	for _, b := range backends {
		u, err := url.Parse(b)
		if err != nil {
			return nil, fmt.Errorf("router: %q: %w", b, err)
		}
		p := httputil.NewSingleHostReverseProxy(u)
		p.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
			w.WriteHeader(http.StatusBadGateway)
		}
		p.FlushInterval = -1
		r.proxies = append(r.proxies, p)
	}
	if len(r.proxies) == 0 {
		return nil, fmt.Errorf("router: no copies")
	}
	return r, nil
}

// OrgOf reads the organization from an address like /orgs/acme/…
func OrgOf(path string) string {
	rest, ok := strings.CutPrefix(path, "/orgs/")
	if !ok {
		return ""
	}
	org, _, _ := strings.Cut(rest, "/")
	return org
}

func (r *Router) live(i int) bool { return !r.down[i] }

func (r *Router) firstLive(from int) int {
	for k := range r.proxies {
		if i := (from + k) % len(r.proxies); r.live(i) {
			return i
		}
	}
	return from % len(r.proxies)
}

// CopyFor says which copy serves an organization right now.
func (r *Router) CopyFor(org string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.copyFor(org)
}

func (r *Router) copyFor(org string) int {
	if i, ok := r.pins[org]; ok {
		return i
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(org))
	i := r.firstLive(int(h.Sum32() % uint32(len(r.proxies))))
	r.pins[org] = i
	return i
}

// Move sends an organization to another copy and cuts its open calls and live
// connections, so its screens reconnect and resume by number.
func (r *Router) Move(org string, to int) {
	r.mu.Lock()
	r.pins[org] = to
	cuts := r.open[org]
	delete(r.open, org)
	r.mu.Unlock()
	r.Moves.Add(1)
	for _, cancel := range cuts {
		cancel()
	}
}

// SetDown takes a copy out of rotation, or puts it back. Organizations pinned to
// a copy that goes down move to a live one.
func (r *Router) SetDown(i int, down bool) {
	r.mu.Lock()
	r.down[i] = down
	var moved []string
	if down {
		for org, at := range r.pins {
			if at == i {
				moved = append(moved, org)
			}
		}
	}
	to := r.firstLive(i + 1)
	r.mu.Unlock()
	for _, org := range moved {
		r.Move(org, to)
	}
}

func (r *Router) track(org string, cancel context.CancelFunc) func() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	id := r.next
	if r.open[org] == nil {
		r.open[org] = map[int64]context.CancelFunc{}
	}
	r.open[org][id] = cancel
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.open[org], id)
	}
}

// ServeHTTP forwards one call.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	org := OrgOf(req.URL.Path)
	if r.mode == ByOrg && org != "" {
		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		r.mu.Lock()
		i := r.copyFor(org)
		r.mu.Unlock()
		defer r.track(org, cancel)()
		r.proxies[i].ServeHTTP(w, req.WithContext(ctx))
		return
	}
	r.mu.Lock()
	i := r.firstLive(int(r.turn.Add(1)) % len(r.proxies))
	r.mu.Unlock()
	r.proxies[i].ServeHTTP(w, req)
}
