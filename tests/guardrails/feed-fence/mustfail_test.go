//go:build feedfaults

package feedfence

import (
	"testing"

	"contexia/internal/feed/service"
)

func mustFail(t *testing.T, name string, run func(*testing.T) []string) {
	t.Helper()
	service.SetFault(name)
	defer service.SetFault("")
	bad := run(t)
	if len(bad) == 0 {
		t.Fatalf("the check passed with the fault %q", name)
	}
	t.Logf("seen failing with %s, %d findings, first: %s", name, len(bad), bad[0])
}

func TestFailsWhenAFenceLeavesHeldEntries(t *testing.T) {
	mustFail(t, service.FaultFenceHeld, conversationFence)
}

func TestFailsWhenChangesInsideAFenceAreApplied(t *testing.T) {
	mustFail(t, service.FaultNoSetAside, conversationFence)
}

func TestFailsWhenManyFencesDoNotStop(t *testing.T) {
	mustFail(t, service.FaultNoOrgStop, tooMany)
}
