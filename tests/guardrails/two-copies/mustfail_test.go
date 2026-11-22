//go:build t057faults

package twocopies

import (
	"testing"

	"contexia/tests/e2e/stack"
)

// With the claim taken out of the workers, the run without faults must show repeats.
func TestFailsWhenTheClaimIsTakenOut(t *testing.T) {
	o := twoOfEach()
	o.DeliveryFault = "no-claim"
	s := stack.Start(t, o)
	stack.MustFail(t, "twice in a run without faults", func(tb testing.TB) { withoutFaults(tb, s) })
}
