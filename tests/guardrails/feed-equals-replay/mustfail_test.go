//go:build feedfaults

package feedequalsreplay

import (
	"math/rand"
	"testing"

	"contexia/internal/feed/service"
	"contexia/tests/guardrails/feed-equals-replay/feedrig"
	"contexia/tests/guardrails/notification-once/rig"
)

// Injected failure, not a broken rule: every third save dies after writing all
// of it and before the commit. Nothing of it may count, and retrying gives
// exactly what one clean run gives.
func TestFailsNothingWhenASaveDiesBeforeItsCommit(t *testing.T) {
	service.SetFault(service.FaultDieBeforeCommit)
	defer service.SetFault("")
	r := feedrig.New(t)
	org := rig.NewOrg("g007d")
	log := feedrig.FromLog(rig.Log(org, 11, 200))
	last := log[len(log)-1].Number
	c := r.Start(11)
	feedrig.Deliver(t, c.URL, log, last, rand.New(rand.NewSource(11)))
	service.SetFault("")
	truth := feedrig.Fold(t, org, log, last, nil)
	bad := feedrig.Diff(t, c.URL, org, truth)
	bad = append(bad, feedrig.DiffNotes(r.Notes(org, feedrig.Place(t, c.URL, org).Edition), truth.Notes)...)
	bad = append(bad, feedrig.DiffUpdates(t, c.URL, org, truth.E.Readers())...)
	if len(bad) > 0 {
		t.Fatalf("after saves that died before their commit the feed differs (%d): %s", len(bad), bad[0])
	}
}

// G-007 must fail if the feed accepts any newer change.
func TestFailsWhenAnyNewerChangeIsAccepted(t *testing.T) {
	service.SetFault(service.FaultAcceptNewer)
	defer service.SetFault("")
	bad := scenario(t, 1)
	if len(bad) == 0 {
		t.Fatal("the check passed with the feed accepting any newer change")
	}
	t.Logf("seen failing, %d differences, first: %s", len(bad), bad[0])
}
