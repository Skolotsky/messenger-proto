// Package liveorder holds the server half of G-015: a screen ends equal to a
// freshly loaded one, whatever happened to its connection, and a copy pushes
// only what it reads from the log, in number order (T-053).
package liveorder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"contexia/internal/conversations"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/tests/guardrails/live-order/liverig"
)

func level(t *testing.T, r *liverig.Rig, c *liverig.Copy, s *liverig.Screen) {
	t.Helper()
	r.Await("the screen to equal a fresh load", func() bool { return liverig.Diff(s.Now(), r.Fresh(c, s.Person)) == "" })
}

// TestTwoSavesReachingTheHubSwapped holds a save's wake-up back until a later
// save has been through. A hub that sends what handlers hand it sends the later
// number first; the screen then ignores the earlier one and loses it silently.
func TestTwoSavesReachingTheHubSwapped(t *testing.T) {
	r := liverig.New(t)
	c := r.NewCopy(conversations.LiveOptions{})
	screen := r.Open(c, "marco")
	before := r.Say(c, "anna", "#launch", "settling")
	r.Await("the first update", func() bool { return screen.Now().Number == before })

	saved := c.Live.Saved
	var held atomic.Bool
	release := make(chan struct{})
	reached := make(chan struct{})
	c.Store.OnSaved = func(org ids.Org, first, last int64) {
		if held.CompareAndSwap(false, true) {
			close(reached)
			<-release // the first save is held before it reaches the hub
		}
		saved(org, first, last)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	var one int64
	go func() { defer wg.Done(); one = r.Say(c, "anna", "#launch", "first") }()
	<-reached
	two := r.Say(c, "anna", "#launch", "second")
	r.Await("the later save to reach the screen", func() bool { return slices.Contains(screen.Received(), two) })
	close(release)
	wg.Wait()
	if one >= two {
		t.Fatalf("test setup: first save got %d, second %d", one, two)
	}
	three := r.Say(c, "anna", "#launch", "third")
	r.Await("the last message to arrive", func() bool { return slices.Contains(screen.Received(), three) })
	got := screen.Received()
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("the screen was sent numbers out of order: %v. A copy must send only what it reads from the log, in number order (T-053)", got)
		}
	}
	for _, want := range []int64{one, two, three} {
		if !slices.Contains(screen.Applied(), want) {
			t.Fatalf("change %d was never applied by the screen (applied %v): a message was lost silently", want, screen.Applied())
		}
	}
	level(t, r, c, screen)
}

func TestCutConnectionResumesAndEndsEqual(t *testing.T) {
	r := liverig.New(t)
	c := r.NewCopy(conversations.LiveOptions{})
	screen := r.Open(c, "marco")
	r.Say(c, "anna", "#launch", "before the cut")
	level(t, r, c, screen)
	screen.Cut()
	r.Say(c, "anna", "#launch", "while away")
	id := "msg_edit_me"
	n := r.Act(c, "send", "marco", map[string]any{"message": map[string]any{"id": id, "conversation": "#launch", "thread": nil, "text": "mine", "mentions": []string{}, "level": nil}})
	r.Act(c, "edit", "marco", map[string]any{"message": id, "text": "mine, edited", "mentions": []string{}, "level": nil})
	r.Act(c, "read", "marco", map[string]any{"conversation": "#launch", "thread": nil, "up_to": n})
	r.Say(c, "lena", "#other", "not for marco")
	screen.Connect(c)
	level(t, r, c, screen)
}

func TestLostUpdateIsFetchedByLastSentWithEveryNudgeLost(t *testing.T) {
	r := liverig.New(t)
	c := r.NewCopy(conversations.LiveOptions{})
	other := r.NewCopy(conversations.LiveOptions{}) // what it saves is not told to c
	screen := r.Open(c, "marco")
	first := r.Say(c, "anna", "#launch", "seen")
	r.Await("the update", func() bool { return screen.Now().Number == first })

	var dropped atomic.Int64
	screen.Drop = func(n int64) bool { return n > first && dropped.CompareAndSwap(0, n) }
	r.Say(other, "anna", "#launch", "lost on the way")
	moved := r.AdvanceUntil("the lost update to be fetched", 20*time.Second, func() bool {
		c.Live.CheckPass(r.Ctx)
		return dropped.Load() != 0 && liverig.Diff(screen.Now(), r.Fresh(c, "marco")) == ""
	})
	t.Logf("a lost update on a connection with no nudges was fetched after %s on the clock", moved)
}

func TestFurtherThan500BehindIsToldToLoadFresh(t *testing.T) {
	r := liverig.New(t)
	c := r.NewCopy(conversations.LiveOptions{})
	screen := r.Open(c, "marco")
	first := r.Say(c, "anna", "#launch", "seen")
	r.Await("the update", func() bool { return screen.Now().Number == first })
	screen.Cut()
	applied := len(screen.Applied())
	for i := range 501 {
		r.Say(c, "anna", "#launch", fmt.Sprintf("while away %d", i))
	}
	screen.Connect(c)
	r.Await("too far behind", func() bool { return len(screen.TooFar()) == 1 && screen.TooFar()[0] == "conversations" })
	if n := len(screen.Applied()); n != applied {
		t.Fatalf("a screen more than 500 behind must get no replay, got %d updates", n-applied)
	}
	screen.Reload(c)
	r.Say(c, "anna", "#launch", "after the reload")
	level(t, r, c, screen)
}

func TestExactly500BehindIsReplayed(t *testing.T) {
	r := liverig.New(t)
	c := r.NewCopy(conversations.LiveOptions{})
	screen := r.Open(c, "marco")
	first := r.Say(c, "anna", "#launch", "seen")
	r.Await("the update", func() bool { return screen.Now().Number == first })
	screen.Cut()
	for i := range 500 {
		r.Say(c, "anna", "#launch", fmt.Sprintf("while away %d", i))
	}
	screen.Connect(c)
	level(t, r, c, screen)
	if got := len(screen.TooFar()); got != 0 {
		t.Fatalf("500 behind is replayed, but the screen was told to load fresh %d times", got)
	}
}

// TestEveryScreenFollowsItsOwnFilter checks the stream each person gets against
// the first load's number, across every kind that concerns only some people.
func TestEveryScreenFollowsItsOwnFilter(t *testing.T) {
	r := liverig.New(t)
	c := r.NewCopy(conversations.LiveOptions{})
	screens := map[string]*liverig.Screen{}
	for _, p := range []string{"anna", "marco", "lena"} {
		screens[p] = r.Open(c, p)
	}
	settle := r.Say(c, "anna", "#launch", "settling")
	r.Await("marco live", func() bool { return screens["marco"].Now().Number == settle })
	r.Say(c, "anna", "#launch", "not for lena")
	r.Say(c, "lena", "#other", "not for marco")
	r.Act(c, "open_direct", "anna", map[string]any{"with": "lena"})
	r.Act(c, "send", "anna", map[string]any{"message": map[string]any{"id": "msg_dm", "conversation": "dm:anna:lena", "thread": nil, "text": "private", "mentions": []string{}, "level": nil}})
	r.Act(c, "add_people", "anna", map[string]any{"conversation": "#launch", "people": []string{"lena"}})
	r.Say(c, "anna", "#launch", "now for lena too")
	r.Act(c, "preference", "marco", map[string]any{"key": "send_key", "value": "enter"})
	r.Act(c, "profile_line", "lena", map[string]any{"text": "on leave"})
	r.Act(c, "dial", "marco", map[string]any{"conversation": "#launch", "thread": nil, "level": "muted"})
	for p, s := range screens {
		if p != "lena" { // lena's model does not hold what she joined; her number is compared below
			level(t, r, c, s)
		}
		if _, leaked := s.Now().Messages["msg_dm"]; leaked && p == "marco" {
			t.Fatal("a direct message reached someone outside it")
		}
	}
	lena := screens["lena"]
	r.Await("lena's number to equal the first load's", func() bool { return lena.Now().Number == r.Fresh(c, "lena").Number })
}

func TestRetiredOrganizationClosesItsConnections(t *testing.T) {
	r := liverig.New(t)
	c := r.NewCopy(conversations.LiveOptions{})
	screen := r.Open(c, "marco")
	first := r.Say(c, "anna", "#launch", "seen")
	r.Await("live", func() bool { return screen.Now().Number == first })
	if err := c.Store.Retire(r.Ctx, r.Org); err != nil {
		t.Fatal(err)
	}
	r.Await("organization_gone, then the close", func() bool { return screen.Gone() && screen.Closed() })
	late := r.NewCopy(conversations.LiveOptions{})
	s2 := r.OpenRetired(late, "marco")
	r.Await("a new connection to a retired organization is told too", func() bool { return s2.Gone() && s2.Closed() })
}

// feedStandIn answers the door's questions per door-reads.md, from a list.
type feedStandIn struct {
	mu      sync.Mutex
	edition int64
	updates []contract.RowUpdate
	down    atomic.Bool
}

func (f *feedStandIn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.down.Load() {
		http.Error(w, "down", http.StatusServiceUnavailable)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	edition, _ := strconv.ParseInt(r.URL.Query().Get("edition"), 10, 64)
	if edition != f.edition {
		_ = json.NewEncoder(w).Encode(contract.NewEdition{NewEdition: f.edition})
		return
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	out := []contract.RowUpdate{}
	for _, u := range f.updates {
		if u.Number > after {
			out = append(out, u)
		}
	}
	_ = json.NewEncoder(w).Encode(contract.RowUpdatesPage{Updates: out})
}

func (f *feedStandIn) add(person string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var prev int64
	for _, u := range f.updates {
		if u.Person == person {
			prev = u.Number
		}
	}
	f.updates = append(f.updates, contract.RowUpdate{Number: int64(len(f.updates) + 1), Person: person, After: prev,
		Parts: []contract.FeedPart{{Value: contract.FeedPartSettingsChanged{Kind: "settings.changed"}}}})
}

func TestFeedStreamFollowsAndANewEditionLoadsFresh(t *testing.T) {
	r := liverig.New(t)
	feed := &feedStandIn{edition: 7}
	stand := httptest.NewServer(feed)
	t.Cleanup(stand.Close)
	c := r.NewCopy(conversations.LiveOptions{Feed: stand.URL})
	marco := r.Open(c, "marco")
	marco.SetFeed(0, 7)
	feed.add("marco")
	feed.add("anna") // not for this screen
	feed.add("marco")
	r.AdvanceUntil("the feed updates, by the 5-second check alone", 10*time.Second, func() bool {
		c.Live.CheckPass(r.Ctx)
		return len(marco.FeedUpdates()) == 2
	})
	got := marco.FeedUpdates()
	if got[0].Number != 1 || got[0].After != 0 || got[1].Number != 3 || got[1].After != 1 {
		t.Fatalf("the screen must get its own updates with `after` the number before it for this reader, got %+v", got)
	}
	// Down: nothing else stops. Up again with a rebuilt feed: load fresh.
	feed.down.Store(true)
	n := r.Say(c, "anna", "#launch", "the feed is down")
	r.Await("conversations to carry on", func() bool { return marco.Now().Number == n })
	feed.down.Store(false)
	feed.mu.Lock()
	feed.edition, feed.updates = 8, nil
	feed.mu.Unlock()
	r.AdvanceUntil("the screen to be told to load the feed fresh", 10*time.Second, func() bool {
		c.Live.CheckPass(r.Ctx)
		far := marco.TooFar()
		return len(far) == 1 && far[0] == "feed"
	})
}
