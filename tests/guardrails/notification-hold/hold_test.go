// Package notificationhold holds G-011: nothing is sent for something read or
// deleted within the 5-second hold, however late the feed or the sender are,
// and when the machine clock steps backwards.
package notificationhold

import (
	"fmt"
	"testing"
	"time"

	"contexia/tests/guardrails/notification-once/rig"
)

const withinHold = 300 * time.Millisecond

// scenarios returns every notification sent that should not have been, and
// every one that should have been and was not.
func scenarios(t *testing.T) []string {
	var bad []string
	check := func(what string, r *rig.Rig, org string, want map[string]bool) {
		for _, b := range rig.OnceEach(rig.Deliveries(t, r.Dir, org), want) {
			bad = append(bad, what+": "+b)
		}
	}
	t0 := rig.Start

	// The feed was stopped: everything arrives long after it happened.
	{
		r := rig.New(t, "sender", 1)
		org := rig.NewOrg("hold")
		b := &rig.Builder{Org: org, Edition: 1}
		read := b.Due("marco", "#a", 1, t0)
		gone := b.Due("lena", "#b", 2, t0)
		b.Add("withdrawn", read, t0.Add(withinHold), map[string]any{"because": "read"})
		b.Add("withdrawn", gone, t0.Add(withinHold), map[string]any{"because": "deleted"})
		b.Add("clock", "", t0.Add(time.Minute), nil)
		r.Clock.Advance(time.Hour)
		rig.DeliverNotes(t, r.Direct, b.Notes)
		check("feed stopped", r, org, map[string]bool{})
	}

	// The feed is slower than the traffic: the withdrawal arrives more than
	// five machine seconds after the due note.
	{
		r := rig.New(t, "sender", 2)
		org := rig.NewOrg("hold")
		b := &rig.Builder{Org: org, Edition: 1}
		name := b.Due("marco", "#a", 1, t0)
		rig.DeliverNotes(t, r.Direct, b.Notes)
		r.Clock.Advance(6 * time.Second)
		b.Due("lena", "#b", 2, t0.Add(100*time.Millisecond))
		b.Add("withdrawn", name, t0.Add(withinHold), map[string]any{"because": "read"})
		rig.DeliverNotes(t, r.Direct, b.Notes)
		b.Add("clock", "", t0.Add(time.Minute), nil)
		rig.DeliverNotes(t, r.Direct, b.Notes)
		check("feed slow", r, org, map[string]bool{fmt.Sprintf("%s / lena / #b / 2", org): true})
	}

	// The machine clock steps backwards in the middle of a hold.
	{
		r := rig.New(t, "sender", 3)
		org := rig.NewOrg("hold")
		b := &rig.Builder{Org: org, Edition: 1}
		name := b.Due("marco", "#a", 1, t0)
		rig.DeliverNotes(t, r.Direct, b.Notes)
		r.Clock.Set(t0.Add(-time.Hour))
		b.Add("withdrawn", name, t0.Add(withinHold), map[string]any{"because": "read"})
		b.Add("clock", "", t0.Add(time.Minute), nil)
		rig.DeliverNotes(t, r.Direct, b.Notes)
		check("clock stepped back", r, org, map[string]bool{})
	}

	// Two conversations applied out of step: a later note of one must not end
	// the hold of the other. Only a clock note moves the sender's clock.
	{
		r := rig.New(t, "sender", 4)
		org := rig.NewOrg("hold")
		b := &rig.Builder{Org: org, Edition: 1}
		name := b.Due("marco", "#a", 1, t0)
		other := b.Due("lena", "#b", 2, t0.Add(6*time.Second))
		b.Add("withdrawn", name, t0.Add(withinHold), map[string]any{"because": "read"})
		b.Add("clock", "", t0.Add(12*time.Second), nil)
		rig.DeliverNotes(t, r.Direct, b.Notes)
		check("out of step", r, org, map[string]bool{other: true})
	}

	// The hold ends strictly after five seconds, by a clock note.
	{
		r := rig.New(t, "sender", 5)
		org := rig.NewOrg("hold")
		b := &rig.Builder{Org: org, Edition: 1}
		name := b.Due("marco", "#a", 1, t0)
		b.Add("clock", "", t0.Add(5*time.Second), nil)
		rig.DeliverNotes(t, r.Direct, b.Notes)
		check("exactly at the end of the hold", r, org, map[string]bool{})
		b.Add("clock", "", t0.Add(5*time.Second+time.Millisecond), nil)
		rig.DeliverNotes(t, r.Direct, b.Notes)
		check("just past the hold", r, org, map[string]bool{name: true})
	}
	return bad
}

func TestNothingSentWithinTheHold(t *testing.T) {
	for _, bad := range scenarios(t) {
		t.Error(bad)
	}
}
