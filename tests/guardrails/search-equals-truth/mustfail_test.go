//go:build standinfaults

package searchequalstruth

import (
	"testing"

	"contexia/internal/standins"
)

// G-006 must fail without the version rule.
func TestFailsWithoutVersionRule(t *testing.T) {
	standins.SetFault(standins.FaultNoVersion)
	defer standins.SetFault("")
	bad := scenario(t, 6)
	if len(bad) == 0 {
		t.Fatal("the check passed with the version rule switched off")
	}
	t.Logf("seen failing, %d differences, first: %s", len(bad), bad[0])
}
