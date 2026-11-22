//go:build standinfaults

package notificationhold

import (
	"testing"

	"contexia/internal/standins"
)

// G-011 must fail when the hold is counted from arrival, and when the clock is
// moved by any later note instead of by clock notes only.
func TestFailsWhenBroken(t *testing.T) {
	for _, f := range []string{standins.FaultHoldArrival, standins.FaultAnyNoteTime} {
		t.Run(f, func(t *testing.T) {
			standins.SetFault(f)
			defer standins.SetFault("")
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
