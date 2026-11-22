package stack

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// Item is one change or note a meter saw: organization, edition (notes) and number.
type Item struct {
	Org     string
	Edition int64
	Number  int64
}

// Meter stands at a receiver's door. It records when the receiver confirmed each
// item, counts everything handed over, and can delay deliveries on purpose.
type Meter struct {
	mu        sync.Mutex
	next      http.Handler
	confirmed map[Item]time.Time
	arrivals  map[Item]int

	delayNanos atomic.Int64
	Delayed    atomic.Int64
}

// NewMeter returns a meter in front of a base address ("" to set it later).
func NewMeter(target string) *Meter {
	m := &Meter{confirmed: map[Item]time.Time{}, arrivals: map[Item]int{}}
	if target != "" {
		u, err := url.Parse(target)
		if err != nil {
			panic(err)
		}
		m.next = httputil.NewSingleHostReverseProxy(u)
	}
	return m
}

// SetNext says what the meter forwards to.
func (m *Meter) SetNext(h http.Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.next = h
}

// DelayEveryDelivery holds each delivery this long before passing it on.
func (m *Meter) DelayEveryDelivery(d time.Duration) { m.delayNanos.Store(int64(d)) }

func itemsIn(body []byte) []Item {
	type head struct {
		Org     string `json:"org"`
		Edition int64  `json:"edition"`
		Number  int64  `json:"number"`
	}
	var batch struct {
		Changes []head `json:"changes"`
	}
	if json.Unmarshal(body, &batch) == nil && len(batch.Changes) > 0 {
		out := make([]Item, 0, len(batch.Changes))
		for _, h := range batch.Changes {
			out = append(out, Item{h.Org, h.Edition, h.Number})
		}
		return out
	}
	var one head
	if json.Unmarshal(body, &one) != nil || one.Org == "" {
		return nil
	}
	return []Item{{one.Org, one.Edition, one.Number}}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// ServeHTTP passes a call on and records a delivery's outcome.
func (m *Meter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	next := m.next
	m.mu.Unlock()
	if next == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		next.ServeHTTP(w, r)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	items := itemsIn(body)
	if d := time.Duration(m.delayNanos.Load()); d > 0 && len(items) > 0 {
		m.Delayed.Add(1)
		select {
		case <-time.After(d):
		case <-r.Context().Done():
			return
		}
	}
	m.mu.Lock()
	for _, it := range items {
		m.arrivals[it]++
	}
	m.mu.Unlock()
	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	next.ServeHTTP(sw, r)
	if sw.status != http.StatusOK {
		return
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, it := range items {
		if _, ok := m.confirmed[it]; !ok {
			m.confirmed[it] = now
		}
	}
}

// Confirmed says when the receiver first confirmed an item.
func (m *Meter) Confirmed(org string, edition, number int64) (time.Time, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	at, ok := m.confirmed[Item{org, edition, number}]
	return at, ok
}

// Repeats counts arrivals beyond the first of every item.
func (m *Meter) Repeats() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, c := range m.arrivals {
		n += c - 1
	}
	return n
}

// Arrivals counts the different items handed over.
func (m *Meter) Arrivals() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.arrivals)
}
