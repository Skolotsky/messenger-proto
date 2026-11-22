// Package agentlimits holds G-018: after three agents in a row no further agent
// is told until a person writes; an agent is held to 20 messages a minute.
package agentlimits

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	r "contexia/tests/guardrails/agent-events/agentrig"
)

// bounce lets each agent answer every new event by naming the other, until
// neither has anything new or the bound is hit. It returns the posts made.
func bounce(h *r.H, places map[string]int64, bound int) int {
	other := map[string]string{r.Release: r.Review, r.Review: r.Release}
	posts := 0
	for quiet := 0; quiet < 2 && posts < bound; {
		quiet++
		for _, agent := range []string{r.Release, r.Review} {
			for _, e := range h.Events(agent, places[agent]) {
				places[agent] = e.Number
				if e.Kind != "mentioned" && e.Kind != "reply" {
					continue
				}
				quiet = 0
				posts++
				s, a := h.Agent(agent, "POST", "/messages", map[string]any{"id": fmt.Sprintf("%s-%d", agent, e.Number),
					"conversation": r.Launch, "thread": r.Root, "text": "over to you", "level": nil,
					"because_of": e.Number, "mentions": []string{other[agent]}})
				if s != 200 {
					h.T.Fatalf("%s's post answered %d: %v", agent, s, a)
				}
			}
		}
	}
	return posts
}

// chainProblems: two agents that always answer each other stop at three, and a
// person writing in the thread starts them again.
func chainProblems(h *r.H) []string {
	var p []string
	places := map[string]int64{}
	h.Say(r.Anna, r.Launch, r.Root, "@release please start", r.Release)
	if n := bounce(h, places, 12); n != 3 {
		p = append(p, fmt.Sprintf("two agents answering each other posted %d messages, expected to stop at 3", n))
	}
	h.Say(r.Marco, r.Launch, r.Root, "a person writes in the thread")
	if n := bounce(h, places, 12); n < 1 || n > 4 {
		p = append(p, fmt.Sprintf("after a person wrote the agents posted %d messages, expected them told again and stopped again", n))
	}
	return p
}

func TestAgentsStopAtThreeInARow(t *testing.T) {
	for _, p := range chainProblems(r.New(t)) {
		t.Error(p)
	}
}

func TestAnAgentIsNotToldOfAnotherAgentUnlessNamed(t *testing.T) {
	h := r.New(t)
	if s, a := h.Agent(r.Review, "PUT", r.Sub(r.Launch), nil); s != 200 {
		t.Fatalf("subscribe answered %d: %v", s, a)
	}
	post := func(id string, mentions []string) {
		if s, a := h.Agent(r.Release, "POST", "/messages", map[string]any{"id": id, "conversation": r.Launch, "thread": nil,
			"text": "from an agent", "level": nil, "because_of": nil, "mentions": mentions}); s != 200 {
			t.Fatalf("post answered %d: %v", s, a)
		}
	}
	post("a-1", nil)
	if n := len(h.Events(r.Review, 0)); n != 0 {
		t.Errorf("a subscribed agent was told of another agent's message that does not name it (%d events)", n)
	}
	post("a-2", []string{r.Review})
	if n := len(h.Events(r.Review, 0)); n != 1 {
		t.Errorf("an agent named in another agent's message has %d events, expected 1", n)
	}
}

// A loop cannot escape the cap by answering in another row of the channel.
func TestTheCountFollowsAnAnswerIntoAnotherRow(t *testing.T) {
	h := r.New(t)
	h.Say(r.Anna, r.Launch, r.Root, "@release go", r.Release)
	var place int64
	agent, other := r.Release, r.Review
	places := map[string]int64{}
	posts := 0
	for i := range 8 {
		events := h.Events(agent, places[agent])
		if len(events) == 0 {
			break
		}
		place = events[len(events)-1].Number
		places[agent] = place
		var thread any
		if i%2 == 1 {
			thread = r.Root
		}
		s, a := h.Agent(agent, "POST", "/messages", map[string]any{"id": fmt.Sprintf("row-%d", i), "conversation": r.Launch, "thread": thread,
			"text": "moving rows", "level": nil, "because_of": place, "mentions": []string{other}})
		if s != 200 {
			t.Fatalf("post answered %d: %v", s, a)
		}
		posts++
		agent, other = other, agent
	}
	if posts != 3 {
		t.Errorf("agents moving between rows posted %d messages, expected to stop at 3", posts)
	}
}

func post(h *r.H, agent, id string) (int, map[string]any) {
	return h.Agent(agent, "POST", "/messages", map[string]any{"id": id, "conversation": r.Launch, "thread": nil,
		"text": "fast " + id, "level": nil, "because_of": nil})
}

// rateProblems: of 45 at once, 20 are accepted; the rest are told to slow down
// and are accepted when sent again a minute later.
func rateProblems(h *r.H) []string {
	var p []string
	var mu sync.Mutex
	statuses := map[int]int{}
	refused := []string{}
	var wg sync.WaitGroup
	work := make(chan string, 45)
	for i := range 45 {
		work <- fmt.Sprintf("burst-%d", i)
	}
	close(work)
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range work {
				s, _ := post(h, r.Release, id)
				mu.Lock()
				statuses[s]++
				if s == http.StatusTooManyRequests {
					refused = append(refused, id)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if statuses[200] != 20 || statuses[http.StatusTooManyRequests] != 25 {
		p = append(p, fmt.Sprintf("45 messages at once were answered %v, expected 20 accepted and 25 told to slow down", statuses))
		return p
	}
	created := 0
	for _, e := range h.Log() {
		if e.Kind == "message.created" {
			created++
		}
	}
	if created != 20 {
		p = append(p, fmt.Sprintf("the log has %d messages, expected 20", created))
	}
	if s, _ := post(h, r.Review, "other-agent"); s != 200 {
		p = append(p, fmt.Sprintf("another agent was slowed by this one's limit: %d", s))
	}
	h.Clock.Advance(61 * time.Second)
	accepted := 0
	for _, id := range refused {
		if s, _ := post(h, r.Release, id); s == 200 {
			accepted++
		}
	}
	if accepted != 20 {
		p = append(p, fmt.Sprintf("a minute later %d of the refused messages were accepted when sent again, expected 20", accepted))
	}
	return p
}

func TestAnAgentIsHeldToTwentyMessagesAMinute(t *testing.T) {
	for _, p := range rateProblems(r.New(t)) {
		t.Error(p)
	}
}

func TestResendingASavedMessageIsNotCountedOrSavedAgain(t *testing.T) {
	h := r.New(t)
	s, first := post(h, r.Release, "same")
	if s != 200 {
		t.Fatalf("post answered %d", s)
	}
	for range 30 {
		s, again := post(h, r.Release, "same")
		if s != 200 || again["message"] != first["message"] || again["number"] != first["number"] {
			t.Fatalf("a resend answered %d %v, expected the first answer %v", s, again, first)
		}
	}
	created := 0
	for _, e := range h.Log() {
		if e.Kind == "message.created" {
			created++
		}
	}
	if created != 1 {
		t.Errorf("a message sent 31 times is on the log %d times", created)
	}
	for i := range 19 {
		if s, _ := post(h, r.Release, fmt.Sprintf("more-%d", i)); s != 200 {
			t.Fatalf("message %d of the minute answered %d: resends were counted", i+2, s)
		}
	}
	if s, _ := post(h, r.Release, "one-too-many"); s != http.StatusTooManyRequests {
		t.Errorf("the 21st message of the minute answered %d, expected 429", s)
	}
}

func TestTheSameIDWithAnotherBodyIsRefused(t *testing.T) {
	h := r.New(t)
	if s, _ := post(h, r.Release, "x"); s != 200 {
		t.Fatal("first post refused")
	}
	s, a := h.Agent(r.Release, "POST", "/messages", map[string]any{"id": "x", "conversation": r.Launch, "thread": nil, "text": "other words", "level": nil, "because_of": nil})
	if s != http.StatusConflict || a["refused"] != "id reused" {
		t.Errorf("the same ID with another body answered %d %v", s, a)
	}
}
