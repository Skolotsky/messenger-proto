// Package feedtwocopies holds G-027 for the feed: two copies side by side lose
// nothing and apply nothing twice, also when one dies halfway, and whichever
// copy is asked gives the same answer.
package feedtwocopies

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"testing"

	"contexia/tests/guardrails/feed-equals-replay/feedrig"
	"contexia/tests/guardrails/notification-once/rig"
)

// scenario delivers one log to two copies at once, each change offered to both,
// kills one copy halfway and starts a fresh one in its place.
func scenario(t *testing.T, seed int64) []string {
	t.Helper()
	r := feedrig.New(t)
	org := rig.NewOrg("g027f")
	log := feedrig.FromLog(rig.Log(org, seed, 300))
	last := log[len(log)-1].Number
	a, b := r.Start(seed), r.Start(seed+100)

	race := func(from, to int, x, y *feedrig.Copy) {
		var wg sync.WaitGroup
		for _, c := range []*feedrig.Copy{x, y} {
			wg.Add(1)
			go func(base string) {
				defer wg.Done()
				rng := rand.New(rand.NewSource(seed))
				for i := from; i < to; {
					end := min(i+1+rng.Intn(6), to)
					ans := feedrig.Post(base, log[i:end])
					switch {
					case ans.Outage || ans.Status >= 500:
					case ans.Status == http.StatusConflict && ans.IAmAt != nil:
						i = max(int(*ans.IAmAt), from)
					case ans.Status == http.StatusOK:
						i = end
					}
				}
			}(c.URL)
		}
		wg.Wait()
	}
	// Taking turns, so that each copy must learn what the other applied.
	rng := rand.New(rand.NewSource(seed))
	third := len(log) / 3
	for i, turn := 0, 0; i < third; turn++ {
		end := min(i+1+rng.Intn(5), third)
		c := a
		if turn%2 == 1 {
			c = b
		}
		if ans := feedrig.Post(c.URL, log[i:end]); ans.Status != http.StatusOK {
			return []string{fmt.Sprintf("taking turns at %d: %+v", i, ans)}
		}
		i = end
	}
	race(third, third*2, a, b)
	a.Close() // dies; its memory goes with it
	fresh := r.Start(seed + 200)
	race(third*2, len(log), b, fresh)

	truth := feedrig.Fold(t, org, log, last, nil)
	var bad []string
	for name, c := range map[string]*feedrig.Copy{"the surviving copy": b, "the fresh copy": fresh} {
		for _, d := range feedrig.Diff(t, c.URL, org, truth) {
			bad = append(bad, name+": "+d)
		}
	}
	edition := feedrig.Place(t, b.URL, org).Edition
	bad = append(bad, feedrig.DiffNotes(r.Notes(org, edition), truth.Notes)...)
	bad = append(bad, feedrig.DiffUpdates(t, b.URL, org, truth.E.Readers())...)
	if got, want := fmt.Sprint(r.Held(org)), fmt.Sprint(truth.HeldNames()); got != want {
		bad = append(bad, "held entries are "+got+", the fold has "+want)
	}
	return bad
}

func TestTwoCopies(t *testing.T) {
	for seed := int64(1); seed <= 4; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			if bad := scenario(t, seed); len(bad) > 0 {
				t.Fatalf("two copies of the feed do not equal one clean replay (%d). First: %s", len(bad), bad[0])
			}
		})
	}
}
