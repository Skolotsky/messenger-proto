// Package conversationsindependent holds guardrail G-030 (D-030, D-017): the
// feed's result for a conversation depends only on that conversation's changes,
// in their order, and on organization-wide changes at their place in the log.
//
// To see it fail:
//
//	CONTEXIA_G030_FAULT=move: the test moves organization-wide changes.
//	go test -tags g030fault:  the rules are built with one that reads across
//	                          conversations.
package conversationsindependent

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"pgregory.net/rapid"

	"contexia/internal/feed/rules"
	"contexia/internal/shared/contract"
)

var base = time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)

// conversationOf names the conversation a change belongs to, or "" for an
// organization-wide one.
func conversationOf(c contract.Change) string {
	switch v := c.Value.(type) {
	case contract.ChangeMessageCreated:
		return v.Message.Conversation
	case contract.ChangeMessageChanged:
		return v.Message.Conversation
	case contract.ChangeMessageDeleted:
		return v.Message.Conversation
	case contract.ChangeReplyStarted:
		return v.Conversation
	case contract.ChangeReplyAbandoned:
		return v.Conversation
	case contract.ChangeReactionAdded:
		return v.Conversation
	case contract.ChangeReactionRemoved:
		return v.Conversation
	case contract.ChangeChannelCreated:
		return v.Conversation
	case contract.ChangeChannelPeopleAdded:
		return v.Conversation
	case contract.ChangeDirectCreated:
		return v.Conversation
	case contract.ChangeReaderRead:
		return v.Conversation
	case contract.ChangeReaderWatchingStarted:
		return v.Conversation
	case contract.ChangeReaderWatchingEnded:
		return v.Conversation
	case contract.ChangeReaderKeep:
		return v.Conversation
	case contract.ChangeReaderDone:
		return v.Conversation
	case contract.ChangeReaderDial:
		return v.Conversation
	}
	return ""
}

type msg struct {
	id, author string
	top        bool
	alive      bool
	reacted    map[string]string
}

// builder writes a valid log from what rapid draws.
type builder struct {
	t       *rapid.T
	number  int64
	log     []contract.Change
	members map[string][]string
	convs   []string
	msgs    map[string][]*msg
	next    int
	joined  int
}

func (b *builder) at() (int64, int64, string) {
	b.number++
	return b.number, b.number - 1, base.Add(time.Duration(b.number) * 700 * time.Millisecond).Format(time.RFC3339Nano)
}

func (b *builder) add(v any) { b.log = append(b.log, contract.Change{Value: v}) }

func pick[T any](t *rapid.T, label string, list []T) T {
	return list[rapid.IntRange(0, len(list)-1).Draw(t, label)]
}

var people = []string{"ann", "bob", "cem", "dee"}

func newBuilder(t *rapid.T) *builder {
	b := &builder{t: t, msgs: map[string][]*msg{}, members: map[string][]string{
		"#one":   {"ann", "bob", "cem", "zed"},
		"#two":   {"bob", "cem", "dee"},
		"dm:a|b": {"ann", "bob"},
		"dm:b|c": {"bob", "cem"},
		"dm:a|z": {"ann", "zed"},
	}}
	b.convs = []string{"#one", "#two", "dm:a|b", "dm:a|z", "dm:b|c"}
	for _, p := range people {
		n, pv, at := b.at()
		b.add(contract.ChangePersonJoined{Kind: "person.joined", Org: "o", Number: n, Previous: pv, At: at, Person: p, Name: p})
	}
	n, pv, at := b.at()
	b.add(contract.ChangePersonJoined{Kind: "person.joined", Org: "o", Number: n, Previous: pv, At: at, Person: "zed", Name: "zed", IsAgent: true})
	for _, c := range b.convs {
		n, pv, at := b.at()
		if c[0] == '#' {
			b.add(contract.ChangeChannelCreated{Kind: "channel.created", Org: "o", Number: n, Previous: pv, At: at, Conversation: c, Name: c, People: b.members[c]})
		} else {
			b.add(contract.ChangeDirectCreated{Kind: "direct.created", Org: "o", Number: n, Previous: pv, At: at, Conversation: c, People: b.members[c]})
		}
	}
	return b
}

func (b *builder) reader(conv string) string {
	var out []string
	for _, p := range b.members[conv] {
		if p != "zed" {
			out = append(out, p)
		}
	}
	return pick(b.t, "reader", out)
}

func (b *builder) alive(conv string) []*msg {
	var out []*msg
	for _, m := range b.msgs[conv] {
		if m.alive {
			out = append(out, m)
		}
	}
	return out
}

func (b *builder) threadOf(conv string) *string {
	var tops []string
	for _, m := range b.msgs[conv] {
		if m.top {
			tops = append(tops, m.id)
		}
	}
	if len(tops) == 0 || rapid.Bool().Draw(b.t, "top level") {
		return nil
	}
	s := pick(b.t, "thread", tops)
	return &s
}

func (b *builder) content() ([]string, *contract.Level) {
	var mentions []string
	for _, p := range append([]string{"channel", "zed"}, people...) {
		if rapid.IntRange(0, 3).Draw(b.t, "mention") == 0 {
			mentions = append(mentions, p)
		}
	}
	var level *contract.Level
	switch rapid.IntRange(0, 3).Draw(b.t, "level") {
	case 0:
		l := contract.Level("flag")
		level = &l
	case 1:
		l := contract.Level("info")
		level = &l
	}
	return mentions, level
}

func (b *builder) message(conv string, thread *string, author string, stopped bool) {
	mentions, level := b.content()
	if stopped {
		mentions, level = nil, nil
	}
	b.next++
	id := fmt.Sprintf("m%d", b.next)
	n, pv, at := b.at()
	b.add(contract.ChangeMessageCreated{Kind: "message.created", Org: "o", Number: n, Previous: pv, At: at, Message: contract.Message{
		Id: id, Number: n, Version: 1, Conversation: conv, Thread: thread, Author: author, AuthorIsAgent: author == "zed",
		Text: "text of " + id, Mentions: mentions, Level: level, Reactions: []contract.Reaction{}, Stopped: stopped, CreatedAt: at}})
	b.msgs[conv] = append(b.msgs[conv], &msg{id: id, author: author, top: thread == nil, alive: true, reacted: map[string]string{}})
}

func (b *builder) orgWide() {
	n, pv, at := b.at()
	who := pick(b.t, "person", people)
	on := rapid.Bool().Draw(b.t, "on")
	switch rapid.IntRange(0, 6).Draw(b.t, "org-wide kind") {
	case 0:
		rule := pick(b.t, "rule", []contract.FocusRule{{Type: "item_kind", Value: "flags"}, {Type: "item_kind", Value: "mentions"},
			{Type: "person", Value: "ann"}, {Type: "person", Value: "zed"}, {Type: "conversation", Value: "#one"}, {Type: "conversation", Value: "dm:a|z"}})
		b.add(contract.ChangeReaderFocusRule{Kind: "reader.focus_rule", Org: "o", Number: n, Previous: pv, At: at, Person: who, Rule: rule, On: on})
	case 1:
		b.add(contract.ChangeReaderFocusPerson{Kind: "reader.focus_person", Org: "o", Number: n, Previous: pv, At: at, Person: who, About: pick(b.t, "about", people), On: on})
	case 2:
		b.add(contract.ChangeReaderFocusAutoAdd{Kind: "reader.focus_auto_add", Org: "o", Number: n, Previous: pv, At: at, Person: who, On: on})
	case 3:
		b.add(contract.ChangeReaderFocusOnly{Kind: "reader.focus_only", Org: "o", Number: n, Previous: pv, At: at, Person: who, On: on})
	case 4:
		b.add(contract.ChangeReaderReleaseSetting{Kind: "reader.release_setting", Org: "o", Number: n, Previous: pv, At: at, Person: who, On: on})
	case 5:
		b.joined++
		id := fmt.Sprintf("new%d", b.joined)
		b.add(contract.ChangePersonJoined{Kind: "person.joined", Org: "o", Number: n, Previous: pv, At: at, Person: id, Name: id})
	default:
		b.add(contract.ChangeTick{Kind: "tick", Org: "o", Number: n, Previous: pv, At: at})
	}
}

func (b *builder) step() {
	if rapid.IntRange(0, 5).Draw(b.t, "organization-wide") == 0 {
		b.orgWide()
		return
	}
	conv := pick(b.t, "conversation", b.convs)
	reader := b.reader(conv)
	var thread *string
	if tops := b.threadOf(conv); tops != nil {
		thread = tops
	}
	switch rapid.IntRange(0, 11).Draw(b.t, "kind") {
	case 0, 1, 2, 3:
		b.message(conv, thread, pick(b.t, "author", b.members[conv]), false)
	case 4:
		if thread != nil {
			author := pick(b.t, "author", b.members[conv])
			n, pv, at := b.at()
			b.add(contract.ChangeReplyStarted{Kind: "reply.started", Org: "o", Number: n, Previous: pv, At: at, Message: "s", Conversation: conv, Thread: thread, Author: author})
			b.message(conv, thread, author, true)
		}
	case 5:
		if list := b.alive(conv); len(list) > 0 {
			m := pick(b.t, "edited", list)
			mentions, level := b.content()
			n, pv, at := b.at()
			var th *string
			if !m.top {
				th = b.parentOf(conv, m)
			}
			b.add(contract.ChangeMessageChanged{Kind: "message.changed", Org: "o", Number: n, Previous: pv, At: at, Message: contract.Message{
				Id: m.id, Number: n, Version: 2, Conversation: conv, Thread: th, Author: m.author, AuthorIsAgent: m.author == "zed",
				Text: fmt.Sprintf("edit %d", n), Mentions: mentions, Level: level, Reactions: []contract.Reaction{}, CreatedAt: at}})
		}
	case 6:
		if list := b.alive(conv); len(list) > 0 {
			m := pick(b.t, "deleted", list)
			m.alive = false
			n, pv, at := b.at()
			b.add(contract.ChangeMessageDeleted{Kind: "message.deleted", Org: "o", Number: n, Previous: pv, At: at, Message: contract.DeletedMessage{Id: m.id, Version: 3, Conversation: conv}})
		}
	case 7:
		if list := b.alive(conv); len(list) > 0 {
			m := pick(b.t, "reacted", list)
			who := pick(b.t, "reactor", b.members[conv])
			n, pv, at := b.at()
			if em, ok := m.reacted[who]; ok {
				delete(m.reacted, who)
				b.add(contract.ChangeReactionRemoved{Kind: "reaction.removed", Org: "o", Number: n, Previous: pv, At: at, Person: who, Emoji: em, Message: m.id, Conversation: conv})
			} else {
				em := pick(b.t, "emoji", []string{"+1", "eyes"})
				m.reacted[who] = em
				b.add(contract.ChangeReactionAdded{Kind: "reaction.added", Org: "o", Number: n, Previous: pv, At: at, Person: who, Emoji: em, Message: m.id, Conversation: conv})
			}
		}
	case 8:
		upTo := b.number
		n, pv, at := b.at()
		b.add(contract.ChangeReaderRead{Kind: "reader.read", Org: "o", Number: n, Previous: pv, At: at, Person: reader, Conversation: conv, Thread: thread, UpTo: upTo})
	case 9:
		n, pv, at := b.at()
		if rapid.Bool().Draw(b.t, "keep") {
			b.add(contract.ChangeReaderKeep{Kind: "reader.keep", Org: "o", Number: n, Previous: pv, At: at, Person: reader, Conversation: conv, Thread: thread})
		} else {
			b.add(contract.ChangeReaderDone{Kind: "reader.done", Org: "o", Number: n, Previous: pv, At: at, Person: reader, Conversation: conv, Thread: thread})
		}
	case 10:
		n, pv, at := b.at()
		level := contract.DialLevel(pick(b.t, "thread level", []string{"focus", "needs_you", "fyi", "off", "auto"}))
		if thread == nil {
			level = contract.DialLevel(pick(b.t, "channel level", []string{"focus", "all", "default", "muted"}))
		}
		b.add(contract.ChangeReaderDial{Kind: "reader.dial", Org: "o", Number: n, Previous: pv, At: at, Person: reader, Conversation: conv, Thread: thread, Level: level})
	default:
		n, pv, at := b.at()
		if rapid.Bool().Draw(b.t, "watch") {
			b.add(contract.ChangeReaderWatchingStarted{Kind: "reader.watching_started", Org: "o", Number: n, Previous: pv, At: at, Person: reader, Conversation: conv, Thread: thread, Connection: "c1"})
		} else {
			b.add(contract.ChangeReaderWatchingEnded{Kind: "reader.watching_ended", Org: "o", Number: n, Previous: pv, At: at, Person: reader, Conversation: conv, Thread: thread, Connection: "c1"})
		}
	}
}

func (b *builder) parentOf(conv string, m *msg) *string {
	for _, c := range b.log {
		if v, ok := c.Value.(contract.ChangeMessageCreated); ok && v.Message.Id == m.id {
			return v.Message.Thread
		}
	}
	return nil
}

// segments splits the log at its organization-wide changes.
func segments(log []contract.Change) (between [][]contract.Change, wide []contract.Change) {
	cur := []contract.Change{}
	for _, c := range log {
		if rules.IsOrgWide(c) {
			between = append(between, cur)
			wide = append(wide, c)
			cur = []contract.Change{}
			continue
		}
		cur = append(cur, c)
	}
	return append(between, cur), wide
}

// interleave merges each stretch's conversations in a drawn order, keeping
// the order inside each conversation and the organization-wide changes in place.
func interleave(t *rapid.T, log []contract.Change) []contract.Change {
	between, wide := segments(log)
	var out []contract.Change
	for i, seg := range between {
		queues := map[string][]contract.Change{}
		var names []string
		for _, c := range seg {
			id := conversationOf(c)
			if queues[id] == nil {
				names = append(names, id)
			}
			queues[id] = append(queues[id], c)
		}
		for len(names) > 0 {
			k := rapid.IntRange(0, len(names)-1).Draw(t, "next conversation")
			out = append(out, queues[names[k]][0])
			queues[names[k]] = queues[names[k]][1:]
			if len(queues[names[k]]) == 0 {
				names = append(names[:k], names[k+1:]...)
			}
		}
		if i < len(wide) {
			out = append(out, wide[i])
		}
	}
	return out
}

// moved takes the first organization-wide change that has something before it
// to its stretch's far side: the fault this guardrail must catch.
func moved(log []contract.Change) []contract.Change {
	out := append([]contract.Change{}, log...)
	for i := len(out) - 1; i > 0; i-- {
		if rules.IsOrgWide(out[i]) && !rules.IsOrgWide(out[i-1]) {
			c := out[i]
			copy(out[1:i+1], out[:i])
			out[0] = c
			// person.joined must stay first for the log to be valid at all
			for j, x := range out {
				if _, ok := x.Value.(contract.ChangePersonJoined); ok && j > 0 {
					copy(out[1:j+1], out[:j])
					out[0] = x
				}
			}
			break
		}
	}
	return out
}

type result struct {
	state map[string]string
	notes map[string][]string
}

func noteText(n rules.NoteDraft) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func finish(e *rules.Engine, notes map[string][]string) result {
	r := result{state: map[string]string{}, notes: notes}
	for _, u := range e.Readers() {
		b, err := json.Marshal(e.State(u))
		if err != nil {
			panic(err)
		}
		r.state[u] = string(b)
		for tab, n := range e.TabCounts(u) {
			if got := e.History(u, tab).Unread; got != n {
				panic(fmt.Sprintf("%s: the count kept for tab %s is %d, the tab has %d unread", u, tab, n, got))
			}
		}
	}
	return r
}

func replay(t interface{ Fatalf(string, ...any) }, log []contract.Change) result {
	e := rules.New("o")
	notes := map[string][]string{}
	for _, c := range log {
		fx, err := e.Apply(c, rules.Options{})
		if err != nil {
			t.Fatalf("apply %T: %v", c.Value, err)
		}
		for _, n := range fx.Notes {
			notes[n.Conversation] = append(notes[n.Conversation], noteText(n))
		}
	}
	return finish(e, notes)
}

// replaySideBySide applies each stretch's conversations at the same time, as
// the feed may do (D-030), with the organization-wide changes between them.
func replaySideBySide(t interface{ Fatalf(string, ...any) }, log []contract.Change) result {
	e := rules.New("o")
	notes := map[string][]string{}
	between, wide := segments(log)
	for i, seg := range between {
		queues := map[string][]contract.Change{}
		for _, c := range seg {
			id := conversationOf(c)
			queues[id] = append(queues[id], c)
		}
		// A conversation must exist before its changes run side by side.
		var wg sync.WaitGroup
		var mu sync.Mutex
		var failure error
		for id, q := range queues {
			wg.Add(1)
			go func(id string, q []contract.Change) {
				defer wg.Done()
				var own []string
				for _, c := range q {
					fx, err := e.Apply(c, rules.Options{NoSummaries: true})
					if err != nil {
						mu.Lock()
						failure = err
						mu.Unlock()
						return
					}
					for _, n := range fx.Notes {
						own = append(own, noteText(n))
					}
				}
				mu.Lock()
				notes[id] = append(notes[id], own...)
				mu.Unlock()
			}(id, q)
		}
		wg.Wait()
		if failure != nil {
			t.Fatalf("apply side by side: %v", failure)
		}
		if i < len(wide) {
			fx, err := e.Apply(wide[i], rules.Options{})
			if err != nil {
				t.Fatalf("apply %T: %v", wide[i].Value, err)
			}
			for _, n := range fx.Notes {
				notes[n.Conversation] = append(notes[n.Conversation], noteText(n))
			}
		}
	}
	return finish(e, notes)
}

func difference(a, b result) string {
	for u, s := range a.state {
		if b.state[u] != s {
			return fmt.Sprintf("reader %s differs:\n a: %s\n b: %s", u, s, b.state[u])
		}
	}
	if len(a.state) != len(b.state) {
		return "different readers"
	}
	for c, list := range a.notes {
		if fmt.Sprint(list) != fmt.Sprint(b.notes[c]) {
			return fmt.Sprintf("notes of %q differ:\n a: %v\n b: %v", c, list, b.notes[c])
		}
	}
	for c, list := range b.notes {
		if fmt.Sprint(list) != fmt.Sprint(a.notes[c]) {
			return fmt.Sprintf("notes of %q differ:\n a: %v\n b: %v", c, a.notes[c], list)
		}
	}
	return ""
}

// TestConversationsAreIndependent is G-030.
func TestConversationsAreIndependent(t *testing.T) {
	moveFault := os.Getenv("CONTEXIA_G030_FAULT") == "move"
	rapid.Check(t, func(rt *rapid.T) {
		b := newBuilder(rt)
		steps := rapid.IntRange(5, 60).Draw(rt, "steps")
		for i := 0; i < steps; i++ {
			b.step()
		}
		want := replay(rt, b.log)
		other := interleave(rt, b.log)
		if moveFault {
			other = moved(other)
		}
		if d := difference(want, replay(rt, other)); d != "" {
			rt.Fatalf("interleaving the conversations changed the result. D-030: a rule may depend only on its own conversation and on organization-wide changes at their place.\n%s", d)
		}
		if d := difference(want, replaySideBySide(rt, b.log)); d != "" {
			rt.Fatalf("applying the conversations at the same time changed the result (D-030).\n%s", d)
		}
	})
}

// TestMovingAnOrganizationWideChangeIsSeen holds the guardrail's own power: a
// Focus rule moved across a message must change what the reader has.
func TestMovingAnOrganizationWideChangeIsSeen(t *testing.T) {
	at := func(n int) string { return base.Add(time.Duration(n) * time.Second).Format(time.RFC3339Nano) }
	log := []contract.Change{
		{Value: contract.ChangePersonJoined{Kind: "person.joined", Org: "o", Number: 1, At: at(1), Person: "ann", Name: "ann"}},
		{Value: contract.ChangePersonJoined{Kind: "person.joined", Org: "o", Number: 2, Previous: 1, At: at(2), Person: "bob", Name: "bob"}},
		{Value: contract.ChangeChannelCreated{Kind: "channel.created", Org: "o", Number: 3, Previous: 2, At: at(3), Conversation: "#one", Name: "#one", People: []string{"ann", "bob"}}},
		{Value: contract.ChangeMessageCreated{Kind: "message.created", Org: "o", Number: 4, Previous: 3, At: at(4), Message: contract.Message{
			Id: "m1", Number: 4, Version: 1, Conversation: "#one", Author: "ann", Text: "hello", Mentions: []string{"bob"}, Reactions: []contract.Reaction{}, CreatedAt: at(4)}}},
		{Value: contract.ChangeReaderFocusOnly{Kind: "reader.focus_only", Org: "o", Number: 5, Previous: 4, At: at(5), Person: "bob", On: true}},
	}
	before := append(append([]contract.Change{}, log[:3]...), log[4], log[3])
	if difference(replay(t, log), replay(t, before)) == "" {
		t.Fatal("moving “Focus only” across a mention changed nothing: the comparison has lost its power")
	}
}
