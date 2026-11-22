// Package clock is the one way programs read time, so tests can move it.
package clock

import (
	"sync"
	"time"
)

// Clock is injected into everything that reads or waits on time.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// Real is the machine's clock.
type Real struct{}

func (Real) Now() time.Time                         { return time.Now() }
func (Real) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Manual is a clock that moves only when told to.
type Manual struct {
	mu      sync.Mutex
	now     time.Time
	waiters []waiter
}

type waiter struct {
	at time.Time
	ch chan time.Time
}

// NewManual returns a clock standing at start.
func NewManual(start time.Time) *Manual { return &Manual{now: start} }

// Now returns the clock's current time.
func (m *Manual) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

// After fires once the clock has been moved by at least d.
func (m *Manual) After(d time.Duration) <-chan time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- m.now
		return ch
	}
	m.waiters = append(m.waiters, waiter{at: m.now.Add(d), ch: ch})
	return ch
}

// Advance moves the clock forward and wakes whatever has come due.
func (m *Manual) Advance(d time.Duration) { m.Set(m.Now().Add(d)) }

// Set puts the clock at t, which may be earlier: guardrails test backward steps.
// Waiters count against the new time.
func (m *Manual) Set(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = t
	kept := m.waiters[:0]
	for _, w := range m.waiters {
		if w.at.After(t) {
			kept = append(kept, w)
			continue
		}
		w.ch <- t
	}
	m.waiters = kept
}
