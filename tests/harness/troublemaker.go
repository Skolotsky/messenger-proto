// Package harness is what guardrail tests use to break things on purpose.
package harness

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Trouble says how often a delivery is interfered with, each from 0 to 1.
type Trouble struct {
	Repeat     float64       // delivered twice
	LoseAnswer float64       // delivered, but the caller never hears the answer
	Late       float64       // an extra copy arrives later
	LateBy     time.Duration // how much later
}

// Troublemaker stands between a delivery worker and a receiver. The same seed
// makes the same choices for the same order of requests.
type Troublemaker struct {
	Target string

	mu     sync.Mutex
	rng    *rand.Rand
	config Trouble
	late   sync.WaitGroup
	client http.Client

	Repeats, LostAnswers, LateCopies atomic.Int64
}

// NewTroublemaker returns a troublemaker in front of target.
func NewTroublemaker(target string, seed int64, t Trouble) *Troublemaker {
	return &Troublemaker{Target: target, rng: rand.New(rand.NewSource(seed)), config: t}
}

// Set changes the trouble while running.
func (m *Troublemaker) Set(t Trouble) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = t
}

// Wait returns once every late copy has been delivered.
func (m *Troublemaker) Wait() { m.late.Wait() }

func (m *Troublemaker) roll() (repeat, lose, late bool, by time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rng.Float64() < m.config.Repeat, m.rng.Float64() < m.config.LoseAnswer, m.rng.Float64() < m.config.Late, m.config.LateBy
}

func (m *Troublemaker) forward(r *http.Request, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(r.Method, m.Target+r.URL.RequestURI(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = r.Header.Clone()
	return m.client.Do(req)
}

// ServeHTTP passes the request on, and misbehaves as configured.
func (m *Troublemaker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	repeat, lose, late, by := m.roll()
	resp, err := m.forward(r, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	answer, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if repeat {
		m.Repeats.Add(1)
		if again, err := m.forward(r, body); err == nil {
			_, _ = io.Copy(io.Discard, again.Body)
			again.Body.Close()
		}
	}
	if late {
		m.LateCopies.Add(1)
		m.late.Add(1)
		copyReq := r.Clone(context.Background())
		go func() {
			defer m.late.Done()
			time.Sleep(by)
			if again, err := m.forward(copyReq, body); err == nil {
				_, _ = io.Copy(io.Discard, again.Body)
				again.Body.Close()
			}
		}()
	}
	if lose {
		m.LostAnswers.Add(1)
		panic(http.ErrAbortHandler) // closes the connection without an answer
	}
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(answer)
}
