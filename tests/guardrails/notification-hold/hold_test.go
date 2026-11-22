// Package notificationhold holds G-011: nothing is sent for something read or
// deleted within the 5-second hold, however late the feed or the sender are,
// and when the machine clock steps backwards. The hold lives in the feed's rules
// (D-034), so each case runs the real rules and hands what they wrote to the
// real sender; what counts is what the provider was given.
package notificationhold

import (
	"testing"
	"time"

	"contexia/tests/guardrails/notification-once/rig"
)

const withinHold = 300 * time.Millisecond

// machine moves the clock a planted "from arrival" hold would read. The real
// rules have no clock, so it does nothing outside the must-fail build.
var machine = func(time.Time) {}

// scenarios returns every notification sent that should not have been, and
// every one that should have been and was not.
func scenarios(t *testing.T) []string {
	var bad []string
	seed := int64(0)
	run := func(what string, play func(f *rig.Feed) map[string]bool) {
		seed++
		r := rig.New(t, "sender", seed)
		org := rig.NewOrg("hold")
		machine(rig.Start)
		f := rig.NewFeed(t, org)
		want := play(f)
		rig.DeliverNotes(t, r.Direct, f.Notes(1))
		for _, b := range rig.OnceEach(rig.Deliveries(t, r.Dir, org), want) {
			bad = append(bad, what+": "+b)
		}
	}
	none := map[string]bool{}
	t0 := rig.Start

	run("feed stopped", func(f *rig.Feed) map[string]bool {
		read := f.Post(t0, "m1", "#a", "marco")
		f.Post(t0, "m2", "#b", "lena")
		machine(t0.Add(time.Hour)) // everything is applied long after it happened
		f.Read(t0.Add(withinHold), "marco", "#a", "m1", read)
		f.Delete(t0.Add(withinHold), "m2", "#b")
		f.Tick(t0.Add(time.Minute))
		return none
	})

	run("feed slower than the traffic", func(f *rig.Feed) map[string]bool {
		read := f.Post(t0, "m1", "#a", "marco")
		machine(t0.Add(6 * time.Second))
		f.Post(t0.Add(100*time.Millisecond), "m2", "#b", "lena")
		f.Read(t0.Add(withinHold), "marco", "#a", "m1", read)
		f.Tick(t0.Add(time.Minute))
		return map[string]bool{f.Name("lena", "m2"): true}
	})

	run("machine clock stepped back", func(f *rig.Feed) map[string]bool {
		read := f.Post(t0, "m1", "#a", "marco")
		machine(t0.Add(-time.Hour))
		f.Tick(t0.Add(2 * time.Second))
		f.Read(t0.Add(3*time.Second), "marco", "#a", "m1", read)
		machine(t0.Add(time.Hour))
		f.Tick(t0.Add(time.Minute))
		return none
	})

	run("a tick stamped before the hold ended releases nothing", func(f *rig.Feed) map[string]bool {
		f.Post(t0, "m1", "#a", "marco")
		f.Tick(t0.Add(4 * time.Second))
		f.Tick(t0.Add(3 * time.Second)) // a stamp that went back must not count either
		return none
	})

	// Two conversations applied out of step: a later change of one must not end
	// the hold of the other. Only a tick releases.
	run("conversations out of step", func(f *rig.Feed) map[string]bool {
		read := f.Post(t0, "m1", "#a", "marco")
		f.Post(t0.Add(6*time.Second), "m2", "#b", "lena")
		f.Read(t0.Add(withinHold), "marco", "#a", "m1", read)
		f.Tick(t0.Add(12 * time.Second))
		return map[string]bool{f.Name("lena", "m2"): true}
	})

	// A busy organization: changes under a second apart, ticks timed from the
	// previous tick. Its holds still end, and a read inside one still counts.
	run("busy organization", func(f *rig.Feed) map[string]bool {
		f.Post(t0, "m1", "#a", "marco")
		read := f.Post(t0.Add(100*time.Millisecond), "m2", "#a", "lena")
		f.Read(t0.Add(4*time.Second), "lena", "#a", "m2", read)
		for i := 1; i <= 16; i++ {
			at := t0.Add(time.Duration(i) * 400 * time.Millisecond)
			id := "x" + string(rune('a'+i))
			f.Post(at, id, "#b")
			if i%3 == 0 {
				f.Tick(at)
			}
		}
		f.Tick(t0.Add(7 * time.Second))
		return map[string]bool{f.Name("marco", "m1"): true}
	})

	run("a fenced read", func(f *rig.Feed) map[string]bool {
		f.Post(t0, "m1", "#a", "marco")
		f.Post(t0, "m2", "#b", "marco")
		// The read could not be applied: its reader is fenced in #a.
		if got := f.E.Fence("#a", "marco"); len(got) != 1 {
			t.Errorf("the fence removed %d entries", len(got))
		}
		f.Tick(t0.Add(time.Minute))
		return map[string]bool{f.Name("marco", "m2"): true}
	})

	run("a mention edited away", func(f *rig.Feed) map[string]bool {
		f.Post(t0, "m1", "#a", "marco")
		f.Edit(t0.Add(withinHold), "m1", "#a", "no mention")
		f.Tick(t0.Add(time.Minute))
		return none
	})

	run("a reaction taken back", func(f *rig.Feed) map[string]bool {
		f.Post(t0, "m1", "#a", "marco")
		f.Read(t0, "marco", "#a", "m1", 1000)
		f.React(t0.Add(withinHold), "marco", "m1", "#a")
		_ = f.Name("anna", "m1")
		f.Unreact(t0.Add(2*withinHold), "marco", "m1", "#a")
		f.Tick(t0.Add(time.Minute))
		return none
	})

	run("a dial change", func(f *rig.Feed) map[string]bool {
		f.Post(t0, "m1", "#a")
		f.Reply(t0, "r1", "#a", "m1", "marco")
		_ = f.Name("anna", "r1")
		f.Dial(t0.Add(withinHold), "anna", "#a", "m1", "off")
		f.Tick(t0.Add(time.Minute))
		return none
	})

	run("exactly at the end of the hold", func(f *rig.Feed) map[string]bool {
		f.Post(t0, "m1", "#a", "marco")
		f.Tick(t0.Add(5 * time.Second))
		return none
	})

	run("just past the hold", func(f *rig.Feed) map[string]bool {
		f.Post(t0, "m1", "#a", "marco")
		f.Tick(t0.Add(5*time.Second + time.Millisecond))
		return map[string]bool{f.Name("marco", "m1"): true}
	})
	return bad
}

func TestNothingSentWithinTheHold(t *testing.T) {
	for _, bad := range scenarios(t) {
		t.Error(bad)
	}
}

// A hold ends within 8 s when healthy: with a tick about once a second the entry
// goes out on the first tick more than 5 s after it became due.
func TestReleasedByTheFirstTickPastTheHold(t *testing.T) {
	t0 := rig.Start
	f := rig.NewFeed(t, rig.NewOrg("timely"))
	f.Post(t0, "m1", "#a", "marco")
	var at time.Time
	for s := 1; s <= 8; s++ {
		at = t0.Add(time.Duration(s) * time.Second)
		f.Tick(at)
		if len(f.Drafts) > 0 {
			break
		}
	}
	if len(f.Drafts) != 1 || f.Drafts[0].Kind != "send" {
		t.Fatalf("notes: %+v", f.Drafts)
	}
	if got := at.Sub(t0); got != 6*time.Second {
		t.Errorf("released %v after it became due, want the first tick past 5 s", got)
	}
}
