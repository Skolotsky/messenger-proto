//go:build t057faults

package freshness

import (
	"testing"
	"time"

	"contexia/tests/e2e/stack"
)

// G-013 must fail when a worker is given a 3 s delay. A delay of exactly 3 s meets
// the call limit and turns into an outage, so this holds each delivery just under it:
// a receiver that is slow, not one that is down.
func TestFailsWhenAWorkerIsDelayed(t *testing.T) {
	s := stack.Start(t, stack.Options{})
	s.FeedMeter.DelayEveryDelivery(2900 * time.Millisecond)
	stack.MustFail(t, "the feed reflected 19 in 20 changes", func(tb testing.TB) { run(tb, s) })
	if s.FeedMeter.Delayed.Load() == 0 {
		t.Fatal("the planted delay was never applied")
	}
}
