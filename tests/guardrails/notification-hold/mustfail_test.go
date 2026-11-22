//go:build standinfaults

package notificationhold

import (
	"testing"

	"contexia/internal/feed/rules"
)

func init() { machine = rules.SetArrival }

// G-011 must fail when the hold is counted from arrival, when any change
// releases instead of a tick, and when a read inside the hold leaves the entry.
func TestFailsWhenBroken(t *testing.T) {
	for _, f := range []string{rules.FaultHoldArrival, rules.FaultAnyChange, rules.FaultReadKeepsHold} {
		t.Run(f, func(t *testing.T) {
			rules.SetFault(f)
			defer rules.SetFault("")
			bad := scenarios(t)
			if len(bad) == 0 {
				t.Fatal("the check passed with the rule switched off")
			}
			for _, b := range bad {
				t.Logf("seen failing: %s", b)
			}
		})
	}
}
