// Package logwriter produces a valid change log from a seed, so a package can
// be built and tested without Conversations.
package logwriter

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

// Entry is one change of the log, in its wire form.
type Entry struct {
	Org    string
	Number int64
	Kind   string
	JSON   []byte
}

// Config says what to write.
type Config struct {
	Orgs    []string // the same content for each: same names, IDs and numbers
	Seed    int64
	Changes int // at least the opening script, which uses every kind once
	Start   time.Time
}

type message struct {
	id, conversation, author string
	thread                   *string
	number, version          int64
	text                     string
	mentions                 []string
	level                    any
	agent                    bool
	participants             []string
	created                  string
	edited                   any
	deleted                  bool
	reactions                map[string][]string
}

type writer struct {
	org    string
	rng    *rand.Rand
	now    time.Time
	number int64
	out    []Entry
	msgs   []*message
	nextID int
	reply  int
}

var (
	people   = []string{"anna", "marco", "lena", "priya", "tomas", "yuki"}
	agents   = []string{"release-agent", "triage-agent"}
	channels = []string{"#launch", "#incidents", "#general"}
	emoji    = []string{"eyes", "white_check_mark", "tada", "thinking_face"}
	lines    = []string{
		"Can you sign off the pricing page?", "The staging deploy is green.", "Who owns the rollback plan?",
		"Moved the review to Thursday.", "Latency is back under 200 ms.", "I left comments on the draft.",
		"Build 2.4.1 is ready for a look.", "Customer call notes are in the doc.",
	}
)

// Write returns the log of each organization. Every organization gets the same
// content, differing only in its name.
func Write(c Config) map[string][]Entry {
	if c.Start.IsZero() {
		c.Start = time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)
	}
	out := map[string][]Entry{}
	for _, org := range c.Orgs {
		w := &writer{org: org, rng: rand.New(rand.NewSource(c.Seed)), now: c.Start}
		w.opening()
		for len(w.out) < c.Changes {
			w.step()
		}
		out[org] = w.out
	}
	return out
}

func (w *writer) emit(kind string, fields map[string]any) int64 {
	w.now = w.now.Add(time.Duration(50+w.rng.Intn(4000)) * time.Millisecond)
	return w.emitAtNow(kind, fields)
}

func (w *writer) emitAtNow(kind string, fields map[string]any) int64 {
	w.number++
	fields["kind"] = kind
	fields["org"] = w.org
	fields["number"] = w.number
	fields["previous"] = w.number - 1
	fields["at"] = w.stamp()
	b, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}
	w.out = append(w.out, Entry{Org: w.org, Number: w.number, Kind: kind, JSON: b})
	return w.number
}

func (w *writer) stamp() string { return w.now.UTC().Format("2006-01-02T15:04:05.000Z") }

func (m *message) wire() map[string]any {
	reactions := []map[string]any{}
	for _, e := range emoji {
		if p := m.reactions[e]; len(p) > 0 {
			reactions = append(reactions, map[string]any{"emoji": e, "people": p})
		}
	}
	var participants any
	if m.participants != nil {
		participants = m.participants
	}
	return map[string]any{
		"id": m.id, "number": m.number, "version": m.version, "conversation": m.conversation, "thread": m.thread,
		"author": m.author, "author_is_agent": m.agent, "participants": participants, "text": m.text,
		"mentions": m.mentions, "level": m.level, "reactions": reactions, "stopped": false,
		"created_at": m.created, "edited_at": m.edited, "deleted": false,
	}
}

func (w *writer) id() string {
	w.nextID++
	return fmt.Sprintf("msg_%05d", w.nextID)
}

func (w *writer) create(conv, author string, thread *string, mentions []string, level any, id string) *message {
	if id == "" {
		id = w.id()
	}
	m := &message{id: id, conversation: conv, author: author, thread: thread, version: 1, mentions: mentions, level: level,
		text: lines[w.rng.Intn(len(lines))], agent: isAgent(author), reactions: map[string][]string{}}
	if len(conv) > 3 && conv[:3] == "dm:" {
		m.participants = []string{"anna", "marco"}
	}
	m.number = w.number + 1
	w.now = w.now.Add(time.Duration(50+w.rng.Intn(4000)) * time.Millisecond)
	m.created = w.stamp()
	w.emitAtNow("message.created", map[string]any{"message": m.wire()})
	w.msgs = append(w.msgs, m)
	return m
}

func isAgent(p string) bool {
	for _, a := range agents {
		if a == p {
			return true
		}
	}
	return false
}

func (w *writer) opening() {
	w.emitAtNow("seed.opened", map[string]any{})
	for _, p := range people {
		w.emit("person.joined", map[string]any{"person": p, "name": p, "is_agent": false})
	}
	for _, a := range agents {
		w.emit("person.joined", map[string]any{"person": a, "name": a, "is_agent": true})
	}
	for _, c := range channels {
		w.emit("channel.created", map[string]any{"conversation": c, "name": c[1:], "people": people[:4]})
	}
	w.emit("channel.people_added", map[string]any{"conversation": "#launch", "people": people[4:]})
	w.emit("direct.created", map[string]any{"conversation": "dm:anna:marco", "people": []string{"anna", "marco"}})

	root := w.create("#launch", "anna", nil, []string{"marco"}, "flag", "")
	w.create("dm:anna:marco", "anna", nil, []string{}, nil, "")
	rootID := root.id
	w.create("#launch", "lena", &rootID, []string{}, nil, "")
	w.edit(root)
	gone := w.create("#general", "priya", nil, []string{}, nil, "")
	w.remove(gone)
	w.emit("reaction.added", w.reaction("lena", "eyes", root, true))
	w.emit("reaction.removed", w.reaction("lena", "eyes", root, false))
	w.streamed("#launch", &rootID, true)
	w.streamed("#launch", &rootID, false)

	row := map[string]any{"person": "marco", "conversation": "#launch", "thread": rootID}
	w.emit("reader.read", merge(row, map[string]any{"up_to": root.number}))
	w.emit("reader.watching_started", merge(row, map[string]any{"connection": "conn_1"}))
	w.emit("reader.watching_ended", merge(row, map[string]any{"connection": "conn_1"}))
	w.emit("reader.keep", merge(row, nil))
	w.emit("reader.done", merge(row, nil))
	w.emit("reader.dial", merge(row, map[string]any{"level": "fyi"}))
	w.emit("reader.dial", map[string]any{"person": "marco", "conversation": "#general", "thread": nil, "level": "muted"})
	w.emit("reader.focus_rule", map[string]any{"person": "marco", "rule": map[string]any{"type": "item_kind", "value": "flags"}, "on": true})
	w.emit("reader.focus_person", map[string]any{"person": "marco", "about": "anna", "on": true})
	w.emit("reader.focus_auto_add", map[string]any{"person": "marco", "on": true})
	w.emit("reader.focus_only", map[string]any{"person": "marco", "on": false})
	w.emit("reader.release_setting", map[string]any{"person": "marco", "on": true})
	w.emit("reader.preference", map[string]any{"person": "marco", "key": "send_key", "value": "enter"})
	w.emit("reader.profile_line", map[string]any{"person": "marco", "text": "Pricing and launch"})
	w.emit("tick", map[string]any{})
	// The opening stands for the seeded start: it ends the way a seed session does.
	// At the same time as the entry before it, so the times that follow do not move.
	w.emitAtNow("seed.sealed", map[string]any{})
}

func merge(a, b map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func (w *writer) reaction(person, e string, m *message, on bool) map[string]any {
	if on {
		m.reactions[e] = append(m.reactions[e], person)
	} else {
		kept := []string{}
		for _, p := range m.reactions[e] {
			if p != person {
				kept = append(kept, p)
			}
		}
		m.reactions[e] = kept
	}
	return map[string]any{"person": person, "emoji": e, "message": m.id, "conversation": m.conversation, "thread": m.thread}
}

func (w *writer) edit(m *message) {
	m.version++
	m.text += " (edited)"
	m.edited = w.stamp()
	w.emit("message.changed", map[string]any{"message": m.wire()})
}

func (w *writer) remove(m *message) {
	m.version++
	m.deleted = true
	w.emit("message.deleted", map[string]any{"message": map[string]any{
		"id": m.id, "version": m.version, "conversation": m.conversation, "thread": m.thread}})
}

func (w *writer) streamed(conv string, thread *string, finish bool) {
	id := w.id()
	author := agents[w.rng.Intn(len(agents))]
	w.emit("reply.started", map[string]any{"message": id, "conversation": conv, "thread": thread, "author": author})
	if finish {
		w.create(conv, author, thread, []string{}, "info", id)
	} else {
		w.emit("reply.abandoned", map[string]any{"message": id, "conversation": conv, "thread": thread})
	}
}

func (w *writer) live() *message {
	for range 30 {
		if m := w.msgs[w.rng.Intn(len(w.msgs))]; !m.deleted {
			return m
		}
	}
	return w.create("#general", "anna", nil, []string{}, nil, "")
}

func (w *writer) step() {
	who := people[w.rng.Intn(len(people))]
	conv := channels[w.rng.Intn(len(channels))]
	switch r := w.rng.Intn(100); {
	case r < 35:
		mentions := []string{}
		if w.rng.Intn(3) == 0 {
			mentions = []string{people[w.rng.Intn(len(people))]}
		}
		w.create(conv, who, nil, mentions, nil, "")
	case r < 55:
		root := w.live()
		for root.thread != nil {
			root = w.live()
		}
		id := root.id
		w.create(root.conversation, who, &id, []string{}, nil, "")
	case r < 63:
		w.edit(w.live())
	case r < 67:
		w.remove(w.live())
	case r < 77:
		m := w.live()
		e := emoji[w.rng.Intn(len(emoji))]
		kind, on := "reaction.added", true
		for _, p := range m.reactions[e] {
			if p == who {
				kind, on = "reaction.removed", false
			}
		}
		w.emit(kind, w.reaction(who, e, m, on))
	case r < 95:
		m := w.live()
		w.emit("reader.read", map[string]any{"person": who, "conversation": m.conversation, "thread": m.thread, "up_to": m.number})
	default:
		w.emit("tick", map[string]any{})
	}
}
