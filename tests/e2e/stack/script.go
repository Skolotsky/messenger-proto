package stack

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"time"
)

// Action is one thing a person does at the door, with the ID it keeps on a retry.
type Action struct {
	ID     string
	As     string
	Kind   string
	Fields map[string]any
}

type liveMessage struct {
	id, author, conversation string
	top                      bool
}

// Script makes a seeded mix of what a team does: messages, replies, mentions,
// edits, deletes and reactions. The same seed makes the same actions.
type Script struct {
	rng      *rand.Rand
	prefix   string
	people   []string
	channels []string
	live     []liveMessage
	reacted  map[string]bool
	n        int
}

// NewScript returns a script over these people and channels.
func NewScript(seed int64, prefix string, people, channels []string) *Script {
	return &Script{rng: rand.New(rand.NewSource(seed)), prefix: prefix, people: people, channels: channels, reacted: map[string]bool{}}
}

func (sc *Script) person() string { return sc.people[sc.rng.Intn(len(sc.people))] }

// Next is the next action.
func (sc *Script) Next() Action {
	sc.n++
	id := fmt.Sprintf("%s_act_%d", sc.prefix, sc.n)
	roll := sc.rng.Float64()
	switch {
	case len(sc.live) > 0 && roll < 0.20:
		m := sc.live[sc.rng.Intn(len(sc.live))]
		return Action{ID: id, As: m.author, Kind: "edit", Fields: Edit(m.id, fmt.Sprintf("%s edited %d", m.id, sc.n))}
	case len(sc.live) > 0 && roll < 0.30:
		i := sc.rng.Intn(len(sc.live))
		m := sc.live[i]
		sc.live = append(sc.live[:i], sc.live[i+1:]...)
		return Action{ID: id, As: m.author, Kind: "delete", Fields: Delete(m.id)}
	case len(sc.live) > 0 && roll < 0.45:
		m := sc.live[sc.rng.Intn(len(sc.live))]
		who := sc.person()
		key := who + "/" + m.id
		on := !sc.reacted[key] // taking back a reaction nobody gave would change nothing
		sc.reacted[key] = on
		return Action{ID: id, As: who, Kind: "react", Fields: map[string]any{"message": m.id, "emoji": "eyes", "on": on}}
	}
	author := sc.person()
	msg := fmt.Sprintf("%s_msg_%d", sc.prefix, sc.n)
	conversation := sc.channels[sc.rng.Intn(len(sc.channels))]
	var thread *string
	top := true
	if sc.rng.Float64() < 0.25 {
		var roots []liveMessage
		for _, m := range sc.live {
			if m.top {
				roots = append(roots, m)
			}
		}
		if len(roots) > 0 {
			root := roots[sc.rng.Intn(len(roots))]
			thread, conversation, top = &root.id, root.conversation, false
		}
	}
	var mentions []string
	text := fmt.Sprintf("%s says %d", msg, sc.n)
	if sc.rng.Float64() < 0.30 {
		if other := sc.person(); other != author {
			mentions = []string{other}
			text = "@" + other + " " + text
		}
	}
	sc.live = append(sc.live, liveMessage{id: msg, author: author, conversation: conversation, top: top})
	return Action{ID: id, As: author, Kind: "send", Fields: Message(msg, conversation, text, thread, mentions...)}
}

// Send saves an action, sending the same ID again while the door is away, as a
// screen does. A refusal is final and fails the test.
func (o *Org) Send(a Action) int64 {
	o.S.T.Helper()
	body := map[string]any{"id": a.ID, "as": a.As, "kind": a.Kind}
	for k, v := range a.Fields {
		body[k] = v
	}
	deadline := time.Now().Add(45 * time.Second)
	for {
		status, out, err := Call(http.MethodPost, o.Address("/people/actions"), body)
		if err == nil && status == http.StatusOK {
			var saved struct {
				Number int64 `json:"number"`
			}
			if err := json.Unmarshal(out, &saved); err != nil {
				o.S.T.Fatal(err)
			}
			return saved.Number
		}
		if err == nil && status < 500 {
			o.S.T.Fatalf("%s %s by %s was refused: %d %s", a.Kind, a.ID, a.As, status, out)
		}
		if time.Now().After(deadline) {
			o.S.T.Fatalf("%s %s was not saved within 45 s: %d %v %s", a.Kind, a.ID, status, err, out)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// NineteenInTwenty is the time within which 19 of 20 of these fall.
func NineteenInTwenty(d []time.Duration) time.Duration {
	if len(d) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	k := (len(s)*19 + 19) / 20
	return s[min(k, len(s))-1]
}
