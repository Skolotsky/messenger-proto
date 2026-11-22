// Package freshness holds G-013: when healthy, at 50 actions a second for 60 s with
// the seeded team's size, 19 changes in 20 are reflected by search and the feed
// within 2 s and reach another open screen within 1 s.
package freshness

import (
	"fmt"
	"testing"
	"time"

	"contexia/tests/e2e/stack"
)

const (
	rate     = 50
	duration = 60 * time.Second
	receiver = 2 * time.Second
	onScreen = time.Second
	seed     = 13
)

func team() (people, channels []string) {
	for i := 1; i <= 30; i++ {
		people = append(people, fmt.Sprintf("person%02d", i))
	}
	for i := 1; i <= 20; i++ {
		channels = append(channels, fmt.Sprintf("#channel-%02d", i))
	}
	return people, channels
}

type measured struct {
	name    string
	took    []time.Duration
	missing int
	limit   time.Duration
}

func (m measured) check(t testing.TB) {
	t.Helper()
	if len(m.took)+m.missing == 0 {
		t.Fatalf("%s: nothing was measured", m.name)
	}
	// One that never arrived counts as later than any limit.
	all := append([]time.Duration(nil), m.took...)
	for range m.missing {
		all = append(all, time.Hour)
	}
	p := stack.NineteenInTwenty(all)
	if p > m.limit {
		t.Fatalf("%s reflected 19 in 20 changes in %v, over the %v of G-013 (%d measured, %d never arrived). Look for what delays this lane", m.name, p, m.limit, len(m.took), m.missing)
	}
	t.Logf("%s: 19 in 20 within %v over %d changes", m.name, p.Round(time.Millisecond), len(all))
}

// run drives the load and measures from the log's timestamp to each receiver's
// confirmation and to the screen's receipt.
func run(t testing.TB, s *stack.Stack) {
	people, channels := team()
	o := s.Seed("fresh", people, channels)
	screen := o.OpenScreen(people[0])
	script := stack.NewScript(seed, "fresh", people, channels)

	total := rate * int(duration/time.Second)
	every := time.Second / rate
	start := time.Now()
	numbers := make(map[int64]bool, total)
	for i := range total {
		if wait := time.Until(start.Add(time.Duration(i) * every)); wait > 0 {
			time.Sleep(wait)
		}
		numbers[o.Send(script.Next())] = true
	}
	if took := time.Since(start); took > duration+3*time.Second {
		t.Fatalf("the test could not keep the pace: %d actions took %v", total, took)
	}

	if len(numbers) != total {
		t.Fatalf("%d actions made %d changes: the load is not what G-013 names", total, len(numbers))
	}
	var last, lastMessage int64
	for _, e := range o.Log() {
		if !numbers[e.Number] {
			continue
		}
		last = max(last, e.Number)
		if e.Kind == "message.created" || e.Kind == "message.changed" || e.Kind == "message.deleted" {
			lastMessage = max(lastMessage, e.Number)
		}
	}
	// Wait for the stragglers, so that a late one is measured rather than missed.
	stack.Until(t, 90*time.Second, "the screen, search and the feed have the last action", func() bool {
		_, f := s.FeedMeter.Confirmed(o.Name, 0, last)
		_, q := s.SearchMeter.Confirmed(o.Name, 0, lastMessage)
		return f && q && screen.Last() >= lastMessage
	})

	var searchTook, feedTook, screenTook []time.Duration
	var searchMissing, feedMissing, screenMissing int
	for _, e := range o.Log() {
		if !numbers[e.Number] {
			continue
		}
		if at, ok := s.FeedMeter.Confirmed(o.Name, 0, e.Number); ok {
			feedTook = append(feedTook, at.Sub(e.At))
		} else {
			feedMissing++
		}
		switch e.Kind {
		case "message.created", "message.changed", "message.deleted":
			if at, ok := s.SearchMeter.Confirmed(o.Name, 0, e.Number); ok {
				searchTook = append(searchTook, at.Sub(e.At))
			} else {
				searchMissing++
			}
			if at, ok := screen.ReceivedAt(e.Number); ok {
				screenTook = append(screenTook, at.Sub(e.At))
			} else {
				screenMissing++
			}
		}
	}
	measured{"search", searchTook, searchMissing, receiver}.check(t)
	measured{"the feed", feedTook, feedMissing, receiver}.check(t)
	measured{"another open screen", screenTook, screenMissing, onScreen}.check(t)
}

func TestFreshnessWhenHealthy(t *testing.T) {
	run(t, stack.Start(t, stack.Options{}))
}
