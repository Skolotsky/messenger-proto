//go:build t057faults

package freshness

import (
	"testing"
	"time"

	"contexia/tests/e2e/stack"
)

// G-013 must fail when a worker is delayed past its 2 s target. A delay of 3 s meets
// the call limit and turns into an outage, and 2.9 s does too on a busy machine, so
// this holds each delivery for 2.5 s: a receiver that is slow, not one that is down.
func TestFailsWhenAWorkerIsDelayed(t *testing.T) {
	s := stack.Start(t, stack.Options{})
	s.FeedMeter.DelayEveryDelivery(2500 * time.Millisecond)
	stack.MustFail(t, "the feed reflected 19 in 20 changes", func(tb testing.TB) { run(tb, s) })
	if s.FeedMeter.Delayed.Load() == 0 {
		t.Fatal("the planted delay was never applied")
	}
}
