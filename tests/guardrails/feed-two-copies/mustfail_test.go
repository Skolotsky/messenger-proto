//go:build feedfaults

package feedtwocopies

import (
	"testing"

	"contexia/internal/feed/service"
)

// G-027 must fail for a copy that trusts its memory instead of the journal.
func TestFailsWithoutCatchingUpFromTheJournal(t *testing.T) {
	service.SetFault(service.FaultNoCatchUp)
	defer service.SetFault("")
	bad := scenario(t, 1)
	if len(bad) == 0 {
		t.Fatal("the check passed with copies that do not catch up")
	}
	t.Logf("seen failing, %d differences, first: %s", len(bad), bad[0])
}
