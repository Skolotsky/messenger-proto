//go:build standinfaults

package notificationonce

import (
	"testing"

	"contexia/internal/standins"
)

// G-010 must fail without the list of sent names, and for a sender that sends
// nothing.
func TestFailsWhenBroken(t *testing.T) {
	for _, f := range []string{standins.FaultNoSentList, standins.FaultSendNothing} {
		t.Run(f, func(t *testing.T) {
			standins.SetFault(f)
			defer standins.SetFault("")
			bad := scenario(t, 10)
			if len(bad) == 0 {
				t.Fatal("the check passed with the rule switched off")
			}
			t.Logf("seen failing, %d differences, first: %s", len(bad), bad[0])
		})
	}
}
