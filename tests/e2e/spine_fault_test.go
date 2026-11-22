//go:build t057faults

package e2e

import (
	"testing"

	"contexia/tests/e2e/stack"
)

// The spine must fail when a read inside the hold leaves the notification on hold.
// The name is the rules' FaultReadKeepsHold, which exists only in the faults build.
func TestFailsWhenAMentionReadInTheHoldIsSent(t *testing.T) {
	s := stack.Start(t, stack.Options{RuleFault: "hold-read-keeps-it"})
	stack.MustFail(t, "sent for a mention read within the hold", func(tb testing.TB) { spine(tb, s) })
}
