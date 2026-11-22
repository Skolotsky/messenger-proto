//go:build deliveryfaults

package retiredeleteretry

import (
	"net/http"
	"testing"
)

// With the retry making its call without a claim, as it did before, stop answers
// 200 while the call is still in flight.
func TestFailsWhenBroken(t *testing.T) {
	t.Setenv("CONTEXIA_DELIVERY_FAULT", "retry-no-claim")
	for _, receiver := range []string{"search", "sender"} {
		t.Run(receiver, func(t *testing.T) {
			got := stopDuringRetry(t, receiver)
			if got != http.StatusOK {
				t.Fatalf("stop answered %d with the rule off; the test cannot see the fault", got)
			}
			t.Logf("seen failing: stop answered %d during a retry in flight", got)
		})
	}
}
