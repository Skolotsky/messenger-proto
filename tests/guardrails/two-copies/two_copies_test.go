// Package twocopies holds G-027: any program can run as two copies. Nothing is lost,
// nothing is applied twice, and in a run without faults nothing is even delivered
// twice. An organization moved from one copy of Conversations to the other mid-burst
// loses nothing, and its screens end equal to a fresh load.
package twocopies

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"contexia/tests/e2e/stack"
)

const (
	perOrg = 150
	seed   = 27
)

var (
	people   = []string{"anna", "marco", "lena", "omar", "yuki", "sam"}
	channels = []string{"#launch", "#ops", "#design"}
	orgNames = []string{"one", "two", "three"}
)

type run struct {
	s       *stack.Stack
	orgs    []*stack.Org
	screens []*stack.Screen
}

// burst drives every organization in turn; at the given step it calls event.
func burst(t testing.TB, s *stack.Stack, events map[int]func(*run)) *run {
	t.Helper()
	r := &run{s: s}
	var scripts []*stack.Script
	for i, name := range orgNames {
		o := s.Seed(name, people, channels)
		r.orgs = append(r.orgs, o)
		r.screens = append(r.screens, o.OpenScreen(people[0]))
		scripts = append(scripts, stack.NewScript(seed+int64(i), name, people, channels))
	}
	for step := range perOrg {
		if e := events[step]; e != nil {
			e(r)
		}
		for i, o := range r.orgs {
			o.Send(scripts[i].Next())
		}
	}
	return r
}

// holds checks what G-027 promises whatever happened on the way.
func (r *run) holds(t testing.TB) {
	t.Helper()
	for i, o := range r.orgs {
		o.Settle(t, 90*time.Second)
		o.CheckSearchEqualsTruth(t)
		o.CheckFeedEqualsReplay(t)
		o.CheckSentOnce(t)
		sc := r.screens[i]
		stack.Until(t, 25*time.Second, o.Name+": the screen reached the last message", func() bool {
			return screenLevel(o, sc)
		})
		o.CheckScreenEqualsFresh(t, sc, people[0], channels)
	}
}

func screenLevel(o *stack.Org, sc *stack.Screen) bool {
	shown := sc.Messages()
	for _, c := range channels {
		for id, want := range o.FreshMessages(people[0], c) {
			got, ok := shown[id]
			if want.Deleted {
				if ok && !got.Deleted {
					return false
				}
				continue
			}
			if !ok || got.Version != want.Version {
				return false
			}
		}
	}
	return true
}

func noRepeats(t testing.TB, s *stack.Stack) {
	t.Helper()
	for name, n := range map[string]int{"search": s.SearchMeter.Repeats(), "the feed": s.FeedMeter.Repeats(), "the sender": s.SenderMeter.Repeats()} {
		if n != 0 {
			t.Fatalf("%s was delivered %d changes twice in a run without faults. Two copies are delivering the same organization: check the claim", name, n)
		}
	}
}

func moveOne(r *run) {
	cr := r.s.ConversationsRouter
	cr.Move("one", 1-cr.CopyFor("one"))
}

func withoutFaults(t testing.TB, s *stack.Stack) {
	r := burst(t, s, map[int]func(*run){perOrg / 2: moveOne})
	r.holds(t)
	noRepeats(t, s)
	if s.ConversationsRouter.Moves.Load() != 1 {
		t.Fatal("the organization was not moved")
	}
	if r.screens[0].Resumes() < 2 {
		t.Fatalf("the moved organization's screen never reconnected (%d resumes), so the move proved nothing", r.screens[0].Resumes())
	}
}

func twoOfEach() stack.Options { return stack.Options{Conversations: 2, Delivery: 2, Feeds: 2} }

func TestTwoCopiesWithoutFaults(t *testing.T) {
	withoutFaults(t, stack.Start(t, twoOfEach()))
}

// TestACopyOfEachDies kills one copy of the workers, the feed and Conversations
// during the burst. Repeats are allowed here; loss and double effects are not.
func TestACopyOfEachDies(t *testing.T) {
	s := stack.Start(t, twoOfEach())
	r := burst(t, s, map[int]func(*run){
		perOrg / 4: func(r *run) { r.s.Deliveries[0].Kill() },
		perOrg / 2: func(r *run) {
			r.s.Feeds[0].Kill() // first, so calls in flight meet the death
			r.s.FeedRouter.SetDown(0, true)
		},
		3 * perOrg / 4: func(r *run) {
			i := r.s.ConversationsRouter.CopyFor("one")
			r.s.Conversations[i].Kill()
			r.s.ConversationsRouter.SetDown(i, true)
		},
	})
	r.holds(t)
	t.Log(fmt.Sprintf("repeats after the kills: search %d, feed %d, sender %d", s.SearchMeter.Repeats(), s.FeedMeter.Repeats(), s.SenderMeter.Repeats()))
}

// TestReceiversDownAndBack is the outage of G-006 and G-010 with two copies of
// everything: search and the sender die mid-burst and return later.
func TestReceiversDownAndBack(t *testing.T) {
	s := stack.Start(t, twoOfEach())
	var searchProc, senderProc *stack.Proc
	for _, p := range s.Others {
		switch p.Name {
		case "search":
			searchProc = p
		case "sender":
			senderProc = p
		}
	}
	r := burst(t, s, map[int]func(*run){
		perOrg / 3: func(*run) {
			searchProc.Kill()
			senderProc.Kill()
		},
		2 * perOrg / 3: func(*run) {
			s.Restart(searchProc)
			s.Restart(senderProc)
		},
	})
	r.holds(t)
}

// TestTheSameFirstChangeAtBothFeedCopies: a claim that runs out means a change can
// reach the feed twice, and any number of feed copies may run
// (conversations-to-feed.md, section 4). Both calls are then answered 200: one
// applied, one "already had it". Anything else counts as the feed being down for
// every organization.
func TestTheSameFirstChangeAtBothFeedCopies(t *testing.T) {
	s := stack.Start(t, twoOfEach())
	const rounds = 60
	bad := map[int]int{}
	for i := range rounds {
		org := fmt.Sprintf("race-%d", i)
		change := map[string]any{"org": org, "number": 1, "previous": 0, "at": "2026-11-22T10:00:00.000Z", "kind": "tick"}
		var wg sync.WaitGroup
		statuses := make([]int, 2)
		start := make(chan struct{})
		for c := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				statuses[c], _, _ = stack.Call(http.MethodPost, s.Feeds[c].URL+"/changes", change)
			}()
		}
		close(start)
		wg.Wait()
		for _, st := range statuses {
			if st != http.StatusOK {
				bad[st]++
			}
		}
	}
	if len(bad) > 0 {
		t.Fatalf("the same first change sent to both feed copies at once was not answered 200 by both, in %d rounds: answers %v. The contract's answer is 200 \"already had it\"; look at how a change re-checks its place after creating the organization's state (feed/service/take.go, takeIn)", rounds, bad)
	}
}
