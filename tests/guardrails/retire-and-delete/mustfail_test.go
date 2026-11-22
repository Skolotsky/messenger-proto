//go:build deletefaults

package retiredelete

import (
	"strings"
	"testing"

	"contexia/internal/shared/orgwipe"
)

func TestFailsWhenADeleteLeavesOutOneTable(t *testing.T) {
	orgwipe.SetFault("leave-table:notifications")
	defer orgwipe.SetFault("")
	s := newSystem(t)
	problems := deleteProblems(t, s)
	for _, p := range problems {
		if strings.Contains(p, "rows of the deleted organization are left in") && strings.Contains(p, "sender.notifications") {
			t.Logf("failed for the right reason: %s", p)
			return
		}
	}
	t.Fatalf("a delete that leaves out sender.notifications was not caught: %v", problems)
}

func TestFailsWhenADeleteIgnoresTheOrganization(t *testing.T) {
	orgwipe.SetFault("ignore-org")
	defer orgwipe.SetFault("")
	s := newSystem(t)
	problems := deleteProblems(t, s)
	for _, p := range problems {
		if strings.Contains(p, "the other organization changed") {
			t.Logf("failed for the right reason: %s", p)
			return
		}
	}
	t.Fatalf("a delete that ignores the organization was not caught: %v", problems)
}
