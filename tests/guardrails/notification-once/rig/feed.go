package rig

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"contexia/internal/feed/rules"
	"contexia/internal/shared/contract"
)

// Feed drives the feed's real rules with changes the test writes, so a hold
// case runs through the code that decides it (D-034).
type Feed struct {
	T      testing.TB
	Org    string
	E      *rules.Engine
	Drafts []rules.NoteDraft
	n      int64
	names  map[string]string // person + message -> the name put on hold
	ver    map[string]int64
}

// NewFeed makes an organization with anna, marco and lena in #a and #b.
func NewFeed(t testing.TB, org string) *Feed {
	f := &Feed{T: t, Org: org, E: rules.New(org), names: map[string]string{}, ver: map[string]int64{}}
	at := stamp(Start.Add(-time.Hour))
	for _, p := range []string{"anna", "marco", "lena"} {
		n, prev := f.next()
		f.apply(contract.ChangePersonJoined{Kind: "person.joined", Org: org, Number: n, Previous: prev, At: at, Person: p, Name: p})
	}
	for _, c := range []string{"#a", "#b"} {
		n, prev := f.next()
		f.apply(contract.ChangeChannelCreated{Kind: "channel.created", Org: org, Number: n, Previous: prev, At: at, Conversation: c, Name: c,
			People: []string{"anna", "marco", "lena"}})
	}
	return f
}

func (f *Feed) next() (number, previous int64) {
	f.n++
	return f.n, f.n - 1
}

func (f *Feed) apply(v any) rules.Effects {
	f.T.Helper()
	fx, err := f.E.Apply(contract.Change{Value: v}, rules.Options{})
	if err != nil {
		f.T.Fatalf("apply %T: %v", v, err)
	}
	for _, h := range fx.Held {
		f.names[h.Person+"/"+h.Message] = h.Name
	}
	f.Drafts = append(f.Drafts, fx.Notes...)
	return fx
}

// Name is the notification of this message for this person.
func (f *Feed) Name(person, message string) string {
	f.T.Helper()
	name, ok := f.names[person+"/"+message]
	if !ok {
		f.T.Fatalf("nothing was put on hold for %s about %s", person, message)
	}
	return name
}

// Post writes a top-level message from anna and answers its log number.
func (f *Feed) Post(when time.Time, id, conversation string, mentions ...string) int64 {
	f.T.Helper()
	n, prev := f.next()
	f.ver[id] = 1
	f.apply(contract.ChangeMessageCreated{Kind: "message.created", Org: f.Org, Number: n, Previous: prev, At: stamp(when), Message: contract.Message{
		Id: id, Number: n, Version: 1, Conversation: conversation, Author: "anna", Text: "text of " + id,
		Mentions: append([]string{}, mentions...), Reactions: []contract.Reaction{}, CreatedAt: stamp(when)}})
	return n
}

// Reply writes a reply from the given author in a thread.
func (f *Feed) Reply(when time.Time, id, conversation, root, author string) {
	f.T.Helper()
	n, prev := f.next()
	f.ver[id] = 1
	f.apply(contract.ChangeMessageCreated{Kind: "message.created", Org: f.Org, Number: n, Previous: prev, At: stamp(when), Message: contract.Message{
		Id: id, Number: n, Version: 1, Conversation: conversation, Thread: &root, Author: author, Text: "text of " + id,
		Mentions: []string{}, Reactions: []contract.Reaction{}, CreatedAt: stamp(when)}})
}

// Edit changes a message's text and mentions.
func (f *Feed) Edit(when time.Time, id, conversation, text string, mentions ...string) {
	f.T.Helper()
	n, prev := f.next()
	f.ver[id]++
	f.apply(contract.ChangeMessageChanged{Kind: "message.changed", Org: f.Org, Number: n, Previous: prev, At: stamp(when), Message: contract.Message{
		Id: id, Number: n, Version: f.ver[id], Conversation: conversation, Author: "anna", Text: text,
		Mentions: append([]string{}, mentions...), Reactions: []contract.Reaction{}, CreatedAt: stamp(when)}})
}

// Delete deletes a message.
func (f *Feed) Delete(when time.Time, id, conversation string) {
	f.T.Helper()
	n, prev := f.next()
	f.ver[id]++
	f.apply(contract.ChangeMessageDeleted{Kind: "message.deleted", Org: f.Org, Number: n, Previous: prev, At: stamp(when),
		Message: contract.DeletedMessage{Id: id, Version: f.ver[id], Conversation: conversation}})
}

// Read reads a message's thread up to the given number.
func (f *Feed) Read(when time.Time, person, conversation, thread string, upTo int64) {
	f.T.Helper()
	n, prev := f.next()
	f.apply(contract.ChangeReaderRead{Kind: "reader.read", Org: f.Org, Number: n, Previous: prev, At: stamp(when), Person: person,
		Conversation: conversation, Thread: &thread, UpTo: upTo})
}

// React adds a reaction.
func (f *Feed) React(when time.Time, person, id, conversation string) {
	f.T.Helper()
	n, prev := f.next()
	f.apply(contract.ChangeReactionAdded{Kind: "reaction.added", Org: f.Org, Number: n, Previous: prev, At: stamp(when), Person: person,
		Emoji: "+1", Message: id, Conversation: conversation})
}

// Unreact takes a reaction back.
func (f *Feed) Unreact(when time.Time, person, id, conversation string) {
	f.T.Helper()
	n, prev := f.next()
	f.apply(contract.ChangeReactionRemoved{Kind: "reaction.removed", Org: f.Org, Number: n, Previous: prev, At: stamp(when), Person: person,
		Emoji: "+1", Message: id, Conversation: conversation})
}

// Dial sets a thread's level.
func (f *Feed) Dial(when time.Time, person, conversation, thread, level string) {
	f.T.Helper()
	n, prev := f.next()
	f.apply(contract.ChangeReaderDial{Kind: "reader.dial", Org: f.Org, Number: n, Previous: prev, At: stamp(when), Person: person,
		Conversation: conversation, Thread: &thread, Level: contract.DialLevel(level)})
}

// Tick is one clock tick of the log.
func (f *Feed) Tick(when time.Time) {
	f.T.Helper()
	n, prev := f.next()
	f.apply(contract.ChangeTick{Kind: "tick", Org: f.Org, Number: n, Previous: prev, At: stamp(when)})
}

// Notes is what the feed wrote, numbered for its log.
func (f *Feed) Notes(edition int64) []Note {
	return FromDrafts(f.Org, edition, f.Drafts)
}

// FromDrafts numbers the rules' notes the way the feed's log does.
func FromDrafts(org string, edition int64, drafts []rules.NoteDraft) []Note {
	out := make([]Note, 0, len(drafts))
	for i, d := range drafts {
		n := int64(i) + 1
		raw, err := json.Marshal(d.Contract(org, n, n-1, edition))
		if err != nil {
			panic(err)
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			panic(err)
		}
		out = append(out, Note{Number: n, Kind: d.Kind, Name: d.Name, At: at(d.At), Body: body})
	}
	return out
}

// Sent is the set of names among the notes that must reach the provider.
func Sent(notes []Note) map[string]bool {
	out := map[string]bool{}
	for _, n := range notes {
		if h, _ := n.Body["history"].(bool); n.Kind == "send" && !h {
			out[n.Name] = true
		}
	}
	return out
}

// Describe is a short form of the notes, for a failure message.
func Describe(notes []Note) string {
	s := ""
	for _, n := range notes {
		s += fmt.Sprintf("%d %s %s; ", n.Number, n.Kind, n.Name)
	}
	return s
}
