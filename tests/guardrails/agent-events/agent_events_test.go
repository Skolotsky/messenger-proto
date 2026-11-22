// Package agentevents holds G-016: an agent misses no event addressed to it
// within 7 days and receives them in order; after 7 days it is told it is too
// far behind; an agent that never reads delays no person and no other agent.
package agentevents

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	r "contexia/tests/guardrails/agent-events/agentrig"
)

const seed = 54

func kinds(events []r.Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Kind
	}
	return out
}

func inOrder(events []r.Event, from int64) string {
	for i, e := range events {
		if e.Number != from+int64(i) {
			return fmt.Sprintf("event %d has number %d, expected %d", i, e.Number, from+int64(i))
		}
	}
	return ""
}

// toldProblems drives each way of addressing an agent and compares what it was told.
func toldProblems(h *r.H) []string {
	var p []string
	mention := h.Say(r.Anna, r.Launch, nil, "@release can you look?", r.Release)
	h.Say(r.Anna, r.Launch, nil, "nothing for an agent here")
	if s, a := h.Agent(r.Release, "POST", "/messages", map[string]any{"id": "a-1", "conversation": r.Launch, "thread": r.Root, "text": "On it.", "level": nil, "because_of": nil}); s != 200 {
		p = append(p, fmt.Sprintf("the agent's post answered %d: %v", s, a))
	}
	h.Say(r.Marco, r.Launch, r.Root, "a reply in a thread it takes part in")
	direct := h.Act("open_direct", r.Anna, map[string]any{"with": r.Release})["conversation"].(string)
	h.Say(r.Anna, direct, nil, "a direct message")
	posted := ""
	for _, e := range h.Log() {
		if e.Kind == "message.created" && strings.Contains(e.Body, "On it.") {
			var b struct {
				Message struct {
					Id string `json:"id"`
				} `json:"message"`
			}
			_ = json.Unmarshal([]byte(e.Body), &b)
			posted = b.Message.Id
		}
	}
	h.Act("react", r.Anna, map[string]any{"message": posted, "emoji": "eyes", "on": true})
	h.Act("edit", r.Anna, map[string]any{"message": mention, "text": "@release please look", "mentions": []string{r.Release}, "level": nil})
	h.Act("delete", r.Anna, map[string]any{"message": mention})

	got := h.Events(r.Release, 0)
	want := []string{"mentioned", "reply", "direct_message", "reaction", "changed", "deleted"}
	if !slices.Equal(kinds(got), want) {
		p = append(p, fmt.Sprintf("the agent was told %v, expected %v", kinds(got), want))
		return p
	}
	if why := inOrder(got, 1); why != "" {
		p = append(p, why)
	}
	if e := got[3]; e.By != r.Anna || e.Emoji != "eyes" || !e.Added {
		p = append(p, fmt.Sprintf("the reaction event says by %q emoji %q added %v", e.By, e.Emoji, e.Added))
	}
	if got[5].Message.Text != "" {
		p = append(p, "a deleted message's event still holds its text")
	}
	if n := len(h.Events(r.Review, 0)); n != 0 {
		p = append(p, fmt.Sprintf("an agent nothing addressed was told %d events", n))
	}
	return p
}

func TestAnAgentIsToldWhatAddressesIt(t *testing.T) {
	for _, p := range toldProblems(r.New(t)) {
		t.Error(p)
	}
}

func TestASubscriptionTellsEverythingFromNowOn(t *testing.T) {
	h := r.New(t)
	h.Say(r.Anna, r.Launch, nil, "before the subscription")
	if s, a := h.Agent(r.Review, "PUT", r.Sub(r.Launch), nil); s != 200 {
		t.Fatalf("subscribe answered %d: %v", s, a)
	}
	id := h.Say(r.Anna, r.Launch, nil, "in the channel")
	h.Act("react", r.Marco, map[string]any{"message": id, "emoji": "eyes", "on": true})
	if got := kinds(h.Events(r.Review, 0)); !slices.Equal(got, []string{"reply", "reaction"}) {
		t.Errorf("a subscribed agent was told %v, expected a new message and a reaction, and nothing from before", got)
	}
	if s, a := h.Agent(r.Review, "DELETE", r.Sub(r.Launch), nil); s != 200 {
		t.Fatalf("unsubscribe answered %d: %v", s, a)
	}
	h.Say(r.Anna, r.Launch, nil, "after unsubscribing")
	if n := len(h.Events(r.Review, 0)); n != 2 {
		t.Errorf("after unsubscribing the agent has %d events, expected 2", n)
	}
	if s, _ := h.Agent(r.Review, "PUT", r.Sub("#nowhere"), nil); s != http.StatusNotFound {
		t.Errorf("subscribing to a channel it is not a member of answered %d, expected 404", s)
	}
}

// awayProblems: an agent that crashes before noting its place reads the same
// events again and misses none; one that never reads changes nothing for anyone.
func awayProblems(h *r.H) []string {
	var p []string
	rng := rand.New(rand.NewSource(seed))
	sent := 0
	var place int64
	var handled []int64
	for range 60 {
		for range 1 + rng.Intn(3) {
			if rng.Intn(4) == 0 {
				h.Say(r.Anna, r.Launch, nil, "not for an agent")
				continue
			}
			h.Say(r.Marco, r.Launch, nil, fmt.Sprintf("@both number %d", sent), r.Release, r.Review)
			sent++
		}
		if rng.Intn(3) == 0 {
			continue // away for this round
		}
		events := h.Events(r.Release, place)
		if len(events) == 0 {
			continue
		}
		again := h.Events(r.Release, place)
		if a, b := fmt.Sprint(events), fmt.Sprint(again); a != b {
			p = append(p, "reading the same place twice gave different events")
		}
		upTo := 1 + rng.Intn(len(events))
		for _, e := range events[:upTo] {
			if !slices.Contains(handled, e.Number) {
				handled = append(handled, e.Number)
			}
		}
		if rng.Intn(3) == 0 {
			continue // crashed before noting its place
		}
		place = events[upTo-1].Number
	}
	for _, e := range h.Events(r.Release, place) {
		if !slices.Contains(handled, e.Number) {
			handled = append(handled, e.Number)
		}
	}
	if len(handled) != sent {
		p = append(p, fmt.Sprintf("the agent handled %d events of the %d sent to it", len(handled), sent))
	}
	for i, n := range handled {
		if n != int64(i+1) {
			p = append(p, fmt.Sprintf("handled out of order: position %d is event %d", i, n))
			break
		}
	}
	if n := len(h.Events(r.Review, 0)); n != sent {
		p = append(p, fmt.Sprintf("the agent that never read has %d events waiting, expected %d", n, sent))
	}
	return p
}

func TestAnAgentThatIsAwayOrCrashesMissesNothing(t *testing.T) {
	for _, p := range awayProblems(r.New(t)) {
		t.Error(p)
	}
}

// tooFarProblems: 7 days and an hour is too far, and says where to go on from,
// before and after housekeeping trimmed it; within 7 days nothing is lost.
func tooFarProblems(h *r.H) []string {
	var p []string
	for range 3 {
		h.Say(r.Anna, r.Launch, nil, "old", r.Release)
	}
	h.Clock.Advance(6*24*time.Hour + 23*time.Hour)
	if n := len(h.Events(r.Release, 0)); n != 3 {
		p = append(p, fmt.Sprintf("just under 7 days later the agent reads %d of 3 events", n))
	}
	h.Clock.Advance(2 * time.Hour)
	h.Say(r.Anna, r.Launch, nil, "new", r.Release)
	for _, when := range []string{"before trimming", "after trimming"} {
		status, raw := h.Call("GET", fmt.Sprintf("/orgs/%s/agents/%s/events?after=0", r.Org, r.Release), nil)
		var body struct {
			TooFarBehind bool  `json:"too_far_behind"`
			Oldest       int64 `json:"oldest"`
		}
		_ = json.Unmarshal(raw, &body)
		if status != http.StatusGone || !body.TooFarBehind || body.Oldest != 4 {
			p = append(p, fmt.Sprintf("%s: reading from the start after 7 days answered %d %s, expected 410 too_far_behind with oldest 4", when, status, raw))
		}
		if e := h.Events(r.Release, 3); len(e) != 1 || e[0].Number != 4 {
			p = append(p, fmt.Sprintf("%s: continuing from where it was told gave %v", when, kinds(e)))
		}
		h.Housekeep()
	}
	return p
}

func TestAfterSevenDaysAnAgentIsToldItIsTooFarBehind(t *testing.T) {
	for _, p := range tooFarProblems(r.New(t)) {
		t.Error(p)
	}
}

func TestALongPollAnswersWhenSomethingArrives(t *testing.T) {
	h := r.New(t)
	got := make(chan int, 1)
	go func() {
		s, _ := h.Call("GET", fmt.Sprintf("/orgs/%s/agents/%s/events?after=0&wait=25", r.Org, r.Release), nil)
		got <- s
	}()
	h.Say(r.Anna, r.Launch, nil, "wake up", r.Release)
	select {
	case s := <-got:
		if s != 200 {
			t.Fatalf("the waiting read answered %d", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a waiting read did not answer when an event arrived, with the clock standing still")
	}
}

func TestALongPollEndsAfterItsWait(t *testing.T) {
	h := r.New(t)
	got := make(chan []byte, 1)
	go func() {
		_, raw := h.Call("GET", fmt.Sprintf("/orgs/%s/agents/%s/events?after=0&wait=99", r.Org, r.Release), nil)
		got <- raw
	}()
	deadline := time.After(5 * time.Second)
	for {
		h.Clock.Advance(25 * time.Second) // repeated: the read may not be waiting yet
		select {
		case raw := <-got:
			if !strings.Contains(string(raw), `"events":[]`) {
				t.Fatalf("an empty wait answered %s", raw)
			}
			return
		case <-deadline:
			t.Fatal("a read with a wait never ended")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestAWaitingAgentHearsThatTheOrganizationIsGone(t *testing.T) {
	h := r.New(t)
	got := make(chan int, 1)
	go func() {
		s, _ := h.Call("GET", fmt.Sprintf("/orgs/%s/agents/%s/events?after=0&wait=25", r.Org, r.Release), nil)
		got <- s
	}()
	deadline := time.After(5 * time.Second)
	for {
		h.Retire() // repeated: the read may not be waiting yet, and retiring twice is harmless
		select {
		case s := <-got:
			if s != http.StatusGone {
				t.Fatalf("a waiting read of a retired organization answered %d", s)
			}
			return
		case <-deadline:
			t.Fatal("a waiting read was not told the organization is gone")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// Every operation answers 410 for a retired organization and 404 once deleted.
func TestRetiredAnswersGoneAndDeletedAnswersNotFound(t *testing.T) {
	h := r.New(t)
	calls := []struct {
		method, tail string
		body         any
	}{
		{"GET", "/events", nil},
		{"POST", "/messages", map[string]any{"id": "x", "conversation": r.Launch, "thread": nil, "text": "t", "level": nil, "because_of": nil}},
		{"POST", "/replies", map[string]any{"id": "y", "conversation": r.Launch, "thread": nil, "because_of": nil}},
		{"POST", "/replies/y/pieces", map[string]any{"at": 0, "text": "t"}},
		{"POST", "/replies/y/finish", map[string]any{"id": "z", "level": nil, "mentions": []string{}}},
		{"POST", "/status", map[string]any{"state": "working", "on": nil}},
		{"POST", "/reactions", map[string]any{"id": "w", "message": r.Root, "emoji": "eyes", "on": true}},
		{"POST", "/direct", map[string]any{"with": r.Anna}},
		{"PUT", r.Sub(r.Launch), nil},
		{"DELETE", r.Sub(r.Launch), nil},
		{"GET", "/conversations/%23launch/messages", nil},
	}
	h.Retire()
	for _, c := range calls {
		s, a := h.Agent(r.Release, c.method, c.tail, c.body)
		if s != http.StatusGone || a["organization_gone"] != true {
			t.Errorf("retired: %s %s answered %d %v", c.method, c.tail, s, a)
		}
	}
	h.Delete()
	for _, c := range calls {
		if s, a := h.Agent(r.Release, c.method, c.tail, c.body); s != http.StatusNotFound {
			t.Errorf("deleted: %s %s answered %d %v", c.method, c.tail, s, a)
		}
	}
}
