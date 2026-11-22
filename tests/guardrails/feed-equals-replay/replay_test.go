// Package feedequalsreplay holds G-007: after an outage, repeats, a restart, a
// misplaced worker and a wipe the feed equals a clean fold of the log, and a
// rebuilt feed equals the live one.
package feedequalsreplay

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"contexia/internal/democontent"
	"contexia/tests/guardrails/feed-equals-replay/feedrig"
	"contexia/tests/guardrails/notification-once/rig"
	"contexia/tests/harness"
)

const changes = 320

// scenario runs one generated log through every fault and returns what differs
// from a clean fold.
func scenario(t *testing.T, seed int64) []string {
	t.Helper()
	r := feedrig.New(t)
	org := rig.NewOrg("g007")
	log := feedrig.FromLog(rig.Log(org, seed, changes))
	last := log[len(log)-1].Number
	rng := rand.New(rand.NewSource(seed))

	// Repeats, lost answers and late copies.
	c := r.Start(seed)
	c.TM.Set(harness.Trouble{Repeat: 0.2, LoseAnswer: 0.1, Late: 0.1, LateBy: 3 * time.Millisecond})
	feedrig.Deliver(t, c.Front, log, last/3, rng)
	c.TM.Wait()

	// A restart: a new copy with no memory, and the old one gone.
	c.Close()
	c = r.Start(seed + 1)

	// A misplaced worker: a change far ahead, and old ones again.
	at := feedrig.Place(t, c.URL, org).Through
	feedrig.Post(c.URL, log[min(int(at)+5, len(log)-1):min(int(at)+7, len(log))])
	feedrig.Post(c.URL, log[max(int(at)-4, 0):at])
	c.TM.Set(harness.Trouble{Repeat: 0.2, LoseAnswer: 0.1, Late: 0.1, LateBy: 3 * time.Millisecond})
	feedrig.Deliver(t, c.Front, log, last*2/3, rng)
	c.TM.Wait()
	c.TM.Set(harness.Trouble{})

	// Before anything wipes it: the live feed equals a fold up to its place.
	before := feedrig.Place(t, c.URL, org)
	early := feedrig.Fold(t, org, log, before.Through, nil)
	if bad := feedrig.Diff(t, c.URL, org, early); len(bad) > 0 {
		return bad
	}
	if bad := feedrig.DiffNotes(r.Notes(org, before.Edition), early.Notes); len(bad) > 0 {
		return bad
	}

	// A wipe in the middle: refilled from the start under a new edition.
	feedrig.Wipe(t, c.URL, org, false)
	if p := feedrig.Place(t, c.URL, org); p.Through != 0 || p.Edition != before.Edition+1 {
		return []string{fmt.Sprintf("after a wipe the place is %+v, was %+v", p, before)}
	}
	feedrig.Deliver(t, c.Front, log, last, rng)

	truth := feedrig.Fold(t, org, log, last, nil)
	bad := feedrig.Diff(t, c.URL, org, truth)
	edition := feedrig.Place(t, c.URL, org).Edition
	bad = append(bad, feedrig.DiffNotes(r.Notes(org, edition), truth.Notes)...)
	bad = append(bad, feedrig.DiffUpdates(t, c.URL, org, truth.E.Readers())...)
	if got, want := fmt.Sprint(r.Held(org)), fmt.Sprint(truth.HeldNames()); got != want {
		bad = append(bad, "held entries are "+got+", the fold has "+want)
	}

	// A rebuilt feed equals the live one, and a further restart changes nothing.
	feedrig.Wipe(t, c.URL, org, false)
	feedrig.Deliver(t, c.URL, log, last, rng)
	c.Close()
	c = r.Start(seed + 2)
	bad = append(bad, feedrig.Diff(t, c.URL, org, truth)...)
	return bad
}

func TestFeedEqualsReplay(t *testing.T) {
	for seed := int64(1); seed <= 5; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			if bad := scenario(t, seed); len(bad) > 0 {
				t.Fatalf("the feed does not equal a clean replay of the log (%d differences). Fix the service, not this test. First: %s", len(bad), bad[0])
			}
		})
	}
}

// The seeded organization, the size a person sees: rebuilt from nothing it equals
// a clean fold, in less than G-020's two minutes.
func TestSeededOrganizationRebuilds(t *testing.T) {
	content, err := democontent.Load(os.DirFS("../../../demo/content"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := content.Build(democontent.Options{})
	if err != nil {
		t.Fatal(err)
	}
	org := rig.NewOrg("g007-seed")
	log := make([]feedrig.Change, len(h.Changes))
	for i, ch := range h.Changes {
		raw, err := json.Marshal(ch)
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		v["org"] = org
		body, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		log[i] = feedrig.Change{Number: int64(v["number"].(float64)), Kind: v["kind"].(string), Body: body}
	}
	last := log[len(log)-1].Number
	r := feedrig.New(t)
	c := r.Start(1)
	deliverAll := func() time.Duration {
		start := time.Now()
		for i := 0; i < len(log); i += 100 {
			end := min(i+100, len(log))
			if a := feedrig.Post(c.URL, log[i:end]); a.Outage || a.Status != 200 {
				t.Fatalf("batch at %d: %+v", i, a)
			}
		}
		return time.Since(start)
	}
	first := deliverAll()
	feedrig.Wipe(t, c.URL, org, false)
	rebuild := deliverAll()
	t.Logf("%d changes: first fill %v, rebuild from nothing %v", len(log), first.Round(time.Millisecond), rebuild.Round(time.Millisecond))
	if rebuild > 2*time.Minute {
		t.Errorf("rebuilding the seeded organization took %v, the limit is 2 minutes", rebuild)
	}
	truth := feedrig.Fold(t, org, log, last, nil)
	if bad := feedrig.Diff(t, c.URL, org, truth); len(bad) > 0 {
		t.Fatalf("the rebuilt seeded organization differs from a clean fold (%d): %s", len(bad), bad[0])
	}
	if n := r.Notes(org, feedrig.Place(t, c.URL, org).Edition); len(n) != 0 {
		t.Errorf("seeded history wrote %d notes; it must write none (D-032)", len(n))
	}
}
