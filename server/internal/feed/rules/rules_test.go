package rules

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"contexia/internal/shared/contract"
)

// W is a small organization driven by log changes.
type W struct {
	t     *testing.T
	e     *Engine
	n     int64
	sec   float64
	notes []NoteDraft
	last  Effects
	next  int
	made  map[string]bool
}

func world(t *testing.T) *W {
	w := &W{t: t, e: New("acme"), made: map[string]bool{}}
	for _, p := range []string{"you", "anna", "marco", "lena"} {
		p := p
		w.apply(func(h hdr) any {
			return contract.ChangePersonJoined{Kind: "person.joined", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, Name: strings.ToUpper(p[:1]) + p[1:]}
		})
	}
	w.apply(func(h hdr) any {
		return contract.ChangePersonJoined{Kind: "person.joined", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: "bot", Name: "Bot", IsAgent: true}
	})
	for name, people := range map[string][]string{"#a": {"you", "anna", "marco", "bot"}, "#b": {"anna", "marco"}} {
		name, people := name, people
		w.apply(func(h hdr) any {
			return contract.ChangeChannelCreated{Kind: "channel.created", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Conversation: name, Name: name, People: people}
		})
	}
	return w
}

type hdr struct {
	n, p int64
	at   string
}

func (w *W) time() string {
	return time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC).Add(time.Duration(w.sec * float64(time.Second))).Format(time.RFC3339Nano)
}

func (w *W) apply(build func(hdr) any) Effects {
	w.t.Helper()
	w.n++
	fx, err := w.e.Apply(contract.Change{Value: build(hdr{w.n, w.n - 1, w.time()})}, Options{})
	if err != nil {
		w.t.Fatalf("apply: %v", err)
	}
	w.notes = append(append(w.notes, fx.Held...), fx.Notes...)
	w.last = fx
	return fx
}

func (w *W) wait(s float64) { w.sec += s }

func dmID(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return "dm:" + a + "|" + b
}

func (w *W) dm(a, b string) string {
	id := dmID(a, b)
	if !w.made[id] {
		w.made[id] = true
		w.apply(func(h hdr) any {
			return contract.ChangeDirectCreated{Kind: "direct.created", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Conversation: id, People: []string{a, b}}
		})
	}
	return id
}

type opt func(*contract.Message)

func to(p ...string) opt { return func(m *contract.Message) { m.Mentions = append(m.Mentions, p...) } }
func in(root string) opt { return func(m *contract.Message) { r := root; m.Thread = &r } }
func lvl(l string) opt {
	return func(m *contract.Message) { x := contract.Level(l); m.Level = &x }
}

var flag, info = lvl("flag"), lvl("info")
var stopped opt = func(m *contract.Message) { m.Stopped = true }

func (w *W) post(author, conv string, opts ...opt) string {
	w.t.Helper()
	w.next++
	id := fmt.Sprintf("m%d", w.next)
	w.apply(func(h hdr) any {
		m := contract.Message{Id: id, Number: h.n, Version: 1, Conversation: conv, Author: author, AuthorIsAgent: author == "bot",
			Text: "text of " + id, Reactions: []contract.Reaction{}, CreatedAt: h.at}
		for _, o := range opts {
			o(&m)
		}
		return contract.ChangeMessageCreated{Kind: "message.created", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Message: m}
	})
	return id
}

func (w *W) msg(id string) *message {
	for _, cv := range w.e.allConvs() {
		if m := cv.msgs[id]; m != nil {
			return m
		}
	}
	w.t.Fatalf("no message %s", id)
	return nil
}

func (w *W) edit(id, text string, opts ...opt) {
	w.t.Helper()
	old := w.msg(id)
	w.apply(func(h hdr) any {
		m := contract.Message{Id: id, Number: old.number, Version: old.version + 1, Conversation: old.conv, Author: old.author,
			AuthorIsAgent: old.agentAuthor, Text: text, Reactions: []contract.Reaction{}, CreatedAt: old.at}
		if old.parent != "" {
			m.Thread = &old.parent
		}
		for _, o := range opts {
			o(&m)
		}
		return contract.ChangeMessageChanged{Kind: "message.changed", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Message: m}
	})
}

func (w *W) del(id string) {
	old := w.msg(id)
	w.apply(func(h hdr) any {
		return contract.ChangeMessageDeleted{Kind: "message.deleted", Org: "acme", Number: h.n, Previous: h.p, At: h.at,
			Message: contract.DeletedMessage{Id: id, Version: old.version + 1, Conversation: old.conv}}
	})
}

func (w *W) react(p, id, emoji string) {
	conv := w.msg(id).conv
	w.apply(func(h hdr) any {
		return contract.ChangeReactionAdded{Kind: "reaction.added", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, Emoji: emoji, Message: id, Conversation: conv}
	})
}

func (w *W) unreact(p, id, emoji string) {
	conv := w.msg(id).conv
	w.apply(func(h hdr) any {
		return contract.ChangeReactionRemoved{Kind: "reaction.removed", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, Emoji: emoji, Message: id, Conversation: conv}
	})
}

func th(id string) *string { return &id }

func (w *W) read(p, conv string, thread *string) {
	upTo := w.n
	w.apply(func(h hdr) any {
		return contract.ChangeReaderRead{Kind: "reader.read", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, Conversation: conv, Thread: thread, UpTo: upTo}
	})
}

func (w *W) keep(p, conv string, thread *string) {
	w.apply(func(h hdr) any {
		return contract.ChangeReaderKeep{Kind: "reader.keep", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, Conversation: conv, Thread: thread}
	})
}

func (w *W) done(p, conv string, thread *string) {
	w.apply(func(h hdr) any {
		return contract.ChangeReaderDone{Kind: "reader.done", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, Conversation: conv, Thread: thread}
	})
}

func (w *W) dial(p, conv string, thread *string, level string) {
	w.apply(func(h hdr) any {
		return contract.ChangeReaderDial{Kind: "reader.dial", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, Conversation: conv, Thread: thread, Level: contract.DialLevel(level)}
	})
}

func (w *W) rule(p, typ, value string, on bool) {
	w.apply(func(h hdr) any {
		return contract.ChangeReaderFocusRule{Kind: "reader.focus_rule", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, Rule: contract.FocusRule{Type: contract.RuleType(typ), Value: value}, On: on}
	})
}

func (w *W) focusPerson(p, about string, on bool) {
	w.apply(func(h hdr) any {
		return contract.ChangeReaderFocusPerson{Kind: "reader.focus_person", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, About: about, On: on}
	})
}

func (w *W) autoAdd(p string, on bool) {
	w.apply(func(h hdr) any {
		return contract.ChangeReaderFocusAutoAdd{Kind: "reader.focus_auto_add", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, On: on}
	})
}

func (w *W) focusOnly(p string, on bool) {
	w.apply(func(h hdr) any {
		return contract.ChangeReaderFocusOnly{Kind: "reader.focus_only", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, On: on}
	})
}

func (w *W) release(p string, on bool) {
	w.apply(func(h hdr) any {
		return contract.ChangeReaderReleaseSetting{Kind: "reader.release_setting", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, On: on}
	})
}

func (w *W) watch(p, conv string, thread *string, conn string, on bool) {
	w.apply(func(h hdr) any {
		if on {
			return contract.ChangeReaderWatchingStarted{Kind: "reader.watching_started", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, Conversation: conv, Thread: thread, Connection: conn}
		}
		return contract.ChangeReaderWatchingEnded{Kind: "reader.watching_ended", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: p, Conversation: conv, Thread: thread, Connection: conn}
	})
}

func (w *W) tick() Effects {
	return w.apply(func(h hdr) any {
		return contract.ChangeTick{Kind: "tick", Org: "acme", Number: h.n, Previous: h.p, At: h.at}
	})
}

// where says where the row holding this item sits for the person.
func (w *W) where(p, id string, kind ...string) string {
	k := itemMsg
	if len(kind) > 0 {
		k = kind[0]
	}
	for _, r := range w.e.State(p).Rows {
		for _, it := range r.Items {
			if it.Msg == id && it.Kind == k {
				if it.Retracted {
					return r.Where + " (marker)"
				}
				return r.Where
			}
		}
	}
	return "nothing"
}

func (w *W) row(p, id string) contract.FeedRow {
	w.t.Helper()
	for _, r := range w.e.Rows(p) {
		if r.Message == id || (r.Thread != nil && *r.Thread == id) {
			return r
		}
	}
	w.t.Fatalf("%s has no row for %s", p, id)
	return contract.FeedRow{}
}

func (w *W) kinds(person string) string {
	var out []string
	for _, n := range w.notes {
		if n.Person == person {
			s := n.Kind
			if n.Now != "" {
				s += ":" + string(n.Now)
			}
			if n.HasText {
				s += ":text"
			}
			out = append(out, s)
		}
	}
	return strings.Join(out, " ")
}

func (w *W) tab(p string, tab contract.HistoryTab) string {
	var out []string
	for _, e := range w.e.History(p, tab).Entries {
		s := e.Message
		if e.Unread {
			s += "*"
		}
		out = append(out, s)
	}
	return strings.Join(out, " ")
}

type ruleCase struct {
	name string
	run  func(w *W) (got, want any)
}

func runCases(t *testing.T, decision string, cases []ruleCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, want := c.run(world(t))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: %s\n got  %v\n want %v", decision, c.name, got, want)
			}
		})
	}
}

func TestStartingBuckets(t *testing.T) {
	runCases(t, "D-001 starting rules, D-013", []ruleCase{
		{"a direct message from a person lands in Focus", func(w *W) (any, any) {
			return w.where("you", w.post("anna", w.dm("anna", "you"))), "Focus/new"
		}},
		{"a direct message from an agent lands in Needs you", func(w *W) (any, any) {
			return w.where("you", w.post("bot", w.dm("bot", "you"))), "Needs you"
		}},
		{"a mention lands in Needs you", func(w *W) (any, any) { return w.where("you", w.post("anna", "#a", to("you"))), "Needs you" }},
		{"a whole-channel mention lands in For your information", func(w *W) (any, any) { return w.where("you", w.post("anna", "#a", to("channel"))), "FYI" }},
		{"ordinary channel traffic stays out", func(w *W) (any, any) { return w.where("you", w.post("anna", "#a")), "nothing" }},
		{"your own message is never in your feed", func(w *W) (any, any) { return w.where("you", w.post("you", "#a", to("you"))), "nothing" }},
		{"an Info mention lands in For your information", func(w *W) (any, any) { return w.where("you", w.post("anna", "#a", to("you"), info)), "FYI" }},
		{"a non-member gets only what names them", func(w *W) (any, any) {
			a, b := w.post("anna", "#b", to("channel")), w.post("anna", "#b", to("you"))
			return []string{w.where("you", a), w.where("you", b)}, []string{"nothing", "Needs you"}
		}},
		{"a whole-channel mention inside a reply reaches nobody extra (R30)", func(w *W) (any, any) {
			r := w.post("anna", "#a")
			return w.where("you", w.post("marco", "#a", in(r), to("channel"))), "nothing"
		}},
		{"replies are one row with their thread", func(w *W) (any, any) {
			r := w.post("anna", "#a", to("you"))
			w.post("marco", "#a", in(r), to("you"))
			return len(w.e.Rows("you")), 1
		}},
	})
}

func TestChannelDial(t *testing.T) {
	runCases(t, "D-001 channel dial, D-013, T-038 point 3", []ruleCase{
		{"Focus brings every top-level post", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "focus")
			return w.where("you", w.post("anna", "#a")), "Focus/new"
		}},
		{"a burst in a focused channel is one row", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "focus")
			w.post("anna", "#a")
			w.post("marco", "#a")
			return len(w.e.Rows("you")), 1
		}},
		{"Focus leaves replies in threads you are not part of out", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "focus")
			r := w.post("anna", "#a")
			return w.where("you", w.post("marco", "#a", in(r))), "nothing"
		}},
		{"Focus lifts what addresses you", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "focus")
			return w.where("you", w.post("anna", "#a", to("you"))), "Focus/new"
		}},
		{"Info stays below a focused channel", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "focus")
			return w.where("you", w.post("anna", "#a", info)), "FYI"
		}},
		{"All activity puts posts in For your information", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "all")
			return w.where("you", w.post("anna", "#a")), "FYI"
		}},
		{"All activity leaves addressed items as at Default", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "all")
			return w.where("you", w.post("anna", "#a", to("you"))), "Needs you"
		}},
		{"Muted lets a mention through", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "muted")
			return w.where("you", w.post("anna", "#a", to("you"))), "Needs you"
		}},
		{"Muted drops a whole-channel mention and a flag naming nobody", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "muted")
			a, b := w.post("anna", "#a", to("channel")), w.post("anna", "#a", flag)
			return []string{w.where("you", a), w.where("you", b)}, []string{"nothing", "nothing"}
		}},
		{"Muted does not silence your own thread", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "muted")
			r := w.post("you", "#a")
			return w.where("you", w.post("anna", "#a", in(r))), "Needs you"
		}},
	})
}

func TestThreadDial(t *testing.T) {
	runCases(t, "D-001 thread dial, T-038 point 4", []ruleCase{
		{"a reply in a thread you started is Needs you", func(w *W) (any, any) {
			r := w.post("you", "#a")
			return w.where("you", w.post("anna", "#a", in(r))), "Needs you"
		}},
		{"a reply in a thread you replied in is Needs you", func(w *W) (any, any) {
			r := w.post("anna", "#a")
			w.post("you", "#a", in(r))
			return w.where("you", w.post("marco", "#a", in(r))), "Needs you"
		}},
		{"a reply in a thread you were mentioned in is For your information", func(w *W) (any, any) {
			r := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(r))
			return w.where("you", w.post("marco", "#a", in(r))), "FYI"
		}},
		{"a thread you are not part of stays out", func(w *W) (any, any) {
			r := w.post("anna", "#a")
			return w.where("you", w.post("marco", "#a", in(r))), "nothing"
		}},
		{"Follow brings replies to For your information", func(w *W) (any, any) {
			r := w.post("anna", "#a")
			w.dial("you", "#a", th(r), "fyi")
			return w.where("you", w.post("marco", "#a", in(r))), "FYI"
		}},
		{"a thread in Focus brings every reply to Focus", func(w *W) (any, any) {
			r := w.post("anna", "#a")
			w.dial("you", "#a", th(r), "focus")
			return w.where("you", w.post("marco", "#a", in(r))), "Focus/new"
		}},
		{"Off keeps replies out, Info included", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.dial("you", "#a", th(r), "off")
			a, b := w.post("anna", "#a", in(r)), w.post("anna", "#a", in(r), info)
			return []string{w.where("you", a), w.where("you", b)}, []string{"nothing", "nothing"}
		}},
		{"what names you gets through Off", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.dial("you", "#a", th(r), "off")
			return w.where("you", w.post("anna", "#a", in(r), to("you"))), "Needs you"
		}},
		{"replying in a thread you turned off sets it back to Needs you", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.dial("you", "#a", th(r), "off")
			w.post("you", "#a", in(r))
			return w.where("you", w.post("anna", "#a", in(r))), "Needs you"
		}},
		{"auto clears the level you set", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.dial("you", "#a", th(r), "off")
			w.dial("you", "#a", th(r), "auto")
			return w.where("you", w.post("anna", "#a", in(r))), "Needs you"
		}},
		{"a thread level by hand wins over a focused channel", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "focus")
			r := w.post("you", "#a")
			w.dial("you", "#a", th(r), "fyi")
			return w.where("you", w.post("anna", "#a", in(r))), "FYI"
		}},
		{"deleting your own message leaves you in its thread (D-013)", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.post("anna", "#a", in(r))
			w.del(r)
			return w.where("you", w.post("marco", "#a", in(r))), "Needs you"
		}},
		{"a mention edited away stops the thread following you (R44)", func(w *W) (any, any) {
			r := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(r))
			w.edit(r, "no mention")
			return w.where("you", w.post("marco", "#a", in(r))), "nothing"
		}},
	})
}

func TestFocusRules(t *testing.T) {
	runCases(t, "D-001 Focus rules, D-013 point 3, T-038 points 1 and 2", []ruleCase{
		{"the suggested rule lifts a mention", func(w *W) (any, any) {
			w.rule("you", "item_kind", "mentions", true)
			return w.where("you", w.post("anna", "#a", to("you"))), "Focus/new"
		}},
		{"adding a rule re-sorts an unread row", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.rule("you", "item_kind", "mentions", true)
			return w.where("you", m), "Focus/new"
		}},
		{"turning the rule off moves what it brought back to Needs you", func(w *W) (any, any) {
			w.rule("you", "item_kind", "mentions", true)
			m := w.post("anna", "#a", to("you"))
			w.rule("you", "item_kind", "mentions", false)
			return w.where("you", m), "Needs you"
		}},
		{"a rule change leaves a read row where it was", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(m))
			w.rule("you", "item_kind", "mentions", true)
			return w.where("you", m), "Earlier"
		}},
		{"removing Flags that name you moves an unread flag to Needs you", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), flag)
			w.rule("you", "item_kind", "flags", false)
			return w.where("you", m), "Needs you"
		}},
		{"removing it leaves a flag you read kept (R37)", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), flag)
			w.read("you", "#a", th(m))
			w.rule("you", "item_kind", "flags", false)
			return w.where("you", m), "Focus/kept"
		}},
		{"a person rule lifts what reaches you from them", func(w *W) (any, any) {
			w.rule("you", "person", "anna", true)
			return w.where("you", w.post("anna", "#a", to("you"))), "Focus/new"
		}},
		{"a person rule pulls in nothing else", func(w *W) (any, any) {
			w.rule("you", "person", "anna", true)
			return w.where("you", w.post("anna", "#a")), "nothing"
		}},
		{"Info stays below a person rule", func(w *W) (any, any) {
			w.rule("you", "person", "anna", true)
			return w.where("you", w.post("anna", "#a", to("you"), info)), "FYI"
		}},
		{"a channel rule is the same as the channel at Focus", func(w *W) (any, any) {
			w.rule("you", "conversation", "#a", true)
			return w.where("you", w.post("anna", "#a")), "Focus/new"
		}},
		{"a direct conversation rule lifts an agent's direct messages", func(w *W) (any, any) {
			id := w.dm("bot", "you")
			w.rule("you", "conversation", id, true)
			return w.where("you", w.post("bot", id)), "Focus/new"
		}},
		{"muting re-sorts: an unaddressed row leaves, a mention stays", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "all")
			a, b := w.post("anna", "#a"), w.post("anna", "#a", to("you"))
			w.dial("you", "#a", nil, "muted")
			return []string{w.where("you", a), w.where("you", b)}, []string{"nothing", "Needs you"}
		}},
	})
}

func TestFocusPeople(t *testing.T) {
	people := func(w *W) []string { return w.e.State("you").FocusPeople }
	runCases(t, "D-021", []ruleCase{
		{"someone who writes directly is added", func(w *W) (any, any) {
			w.post("anna", w.dm("anna", "you"))
			return people(w), []string{"anna"}
		}},
		{"an agent is not added", func(w *W) (any, any) {
			w.post("bot", w.dm("bot", "you"))
			return people(w), []string{}
		}},
		{"the option off stops adding and keeps those added", func(w *W) (any, any) {
			w.post("anna", w.dm("anna", "you"))
			w.autoAdd("you", false)
			a, b := w.post("anna", w.dm("anna", "you")), w.post("marco", w.dm("marco", "you"))
			return []string{w.where("you", a), w.where("you", b)}, []string{"Focus/new", "Needs you"}
		}},
		{"someone removed is not added back by writing", func(w *W) (any, any) {
			w.post("anna", w.dm("anna", "you"))
			w.focusPerson("you", "anna", false)
			return w.where("you", w.post("anna", w.dm("anna", "you"))), "Needs you"
		}},
		{"removing one leaves the others", func(w *W) (any, any) {
			w.post("anna", w.dm("anna", "you"))
			w.post("marco", w.dm("marco", "you"))
			w.focusPerson("you", "anna", false)
			return people(w), []string{"marco"}
		}},
		{"Add back returns them", func(w *W) (any, any) {
			w.post("anna", w.dm("anna", "you"))
			w.focusPerson("you", "anna", false)
			w.focusPerson("you", "anna", true)
			return w.where("you", w.post("anna", w.dm("anna", "you"))), "Focus/new"
		}},
		{"removing moves an unread row to Needs you", func(w *W) (any, any) {
			m := w.post("anna", w.dm("anna", "you"))
			w.focusPerson("you", "anna", false)
			return w.where("you", m), "Needs you"
		}},
		{"a Focus person lifts only direct messages", func(w *W) (any, any) {
			w.post("anna", w.dm("anna", "you"))
			return w.where("you", w.post("anna", "#a", to("you"))), "Needs you"
		}},
		{"settings list people, removed people and the option", func(w *W) (any, any) {
			w.post("anna", w.dm("anna", "you"))
			w.post("marco", w.dm("marco", "you"))
			w.focusPerson("you", "marco", false)
			w.autoAdd("you", false)
			s := w.e.Settings("you")
			return []any{s.FocusPeople[0].Person, s.FocusPeople[0].ItemsLast7Days, s.RemovedPeople, s.AutoAdd}, []any{"anna", int64(1), []string{"marco"}, false}
		}},
	})
}

func TestFlags(t *testing.T) {
	runCases(t, "D-001 flags, D-011", []ruleCase{
		{"a flag naming you lands in Focus", func(w *W) (any, any) { return w.where("you", w.post("anna", "#a", to("you"), flag)), "Focus/new" }},
		{"once read it is kept for you", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), flag)
			w.read("you", "#a", th(m))
			return []any{w.where("you", m), *w.row("you", m).Kept, w.row("you", m).Why}, []any{"Focus/kept", contract.KeptBy("flag"), "Flagged by Anna"}
		}},
		{"replying releases it", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), flag)
			w.read("you", "#a", th(m))
			w.post("you", "#a", in(m))
			return w.where("you", m), "Earlier"
		}},
		{"reacting releases it", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), flag)
			w.react("you", m, "+1")
			return w.where("you", m), "Earlier"
		}},
		{"Done releases it", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), flag)
			w.done("you", "#a", th(m))
			return w.where("you", m), "Earlier"
		}},
		{"someone else's answer changes nothing for you", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you", "marco"), flag)
			w.read("you", "#a", th(m))
			w.react("marco", m, "+1")
			w.post("marco", "#a", in(m))
			return w.where("you", m), "Focus/kept"
		}},
		{"a flag naming nobody is Needs you and is not kept", func(w *W) (any, any) {
			m := w.post("anna", "#a", flag)
			before := w.where("you", m)
			w.read("you", "#a", th(m))
			return []string{before, w.where("you", m)}, []string{"Needs you", "Earlier"}
		}},
		{"a flag with only a whole-channel mention names nobody", func(w *W) (any, any) {
			return w.where("you", w.post("anna", "#a", to("channel"), flag)), "Needs you"
		}},
		{"a flag naming only an agent reaches no person", func(w *W) (any, any) {
			return w.where("you", w.post("anna", "#a", to("bot"), flag)), "nothing"
		}},
		{"a flag in a direct conversation is for the other person", func(w *W) (any, any) {
			id := w.dm("anna", "you")
			m := w.post("anna", id, flag)
			w.read("you", id, nil)
			return w.where("you", m), "Focus/kept"
		}},
		{"the flag removed from a read row releases it", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), flag)
			w.read("you", "#a", th(m))
			w.edit(m, "x", to("you"))
			return w.where("you", m), "Earlier"
		}},
		{"the flag removed before reading leaves an ordinary mention", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), flag)
			w.edit(m, "x", to("you"))
			return w.where("you", m), "Needs you"
		}},
		{"a row you also kept stays kept when the flag goes", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), flag)
			w.keep("you", "#a", th(m))
			w.edit(m, "x", to("you"))
			return w.where("you", m), "Focus/kept"
		}},
	})
}

func TestKeepAndDone(t *testing.T) {
	runCases(t, "D-003, D-011, D-016, D-023 points 22 and 26", []ruleCase{
		{"a kept row lives in Focus", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			return []any{w.where("you", m), *w.row("you", m).Kept, w.row("you", m).Why}, []any{"Focus/kept", contract.KeptBy("you"), "Kept by you"}
		}},
		{"keeping marks it read", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			return w.row("you", m).Unread, false
		}},
		{"Done releases it", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			w.done("you", "#a", th(m))
			return w.where("you", m), "Earlier"
		}},
		{"Done can be taken back with Keep", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			w.done("you", "#a", th(m))
			w.keep("you", "#a", th(m))
			return w.where("you", m), "Focus/kept"
		}},
		{"a reply releases a kept row", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			w.post("you", "#a", in(m))
			return w.where("you", m), "Earlier"
		}},
		{"a reaction releases a kept row", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			w.react("you", m, "+1")
			return w.where("you", m), "Earlier"
		}},
		{"with the setting off a reply does not release", func(w *W) (any, any) {
			w.release("you", false)
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			w.post("you", "#a", in(m))
			return w.where("you", m), "Focus/kept"
		}},
		{"with the setting off a reaction does not release a flag, and still reads", func(w *W) (any, any) {
			w.release("you", false)
			m := w.post("anna", "#a", to("you"), flag)
			w.react("you", m, "+1")
			return []any{w.where("you", m), w.row("you", m).Unread}, []any{"Focus/kept", false}
		}},
		{"new activity on a kept row shows inside Kept (R16)", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			w.post("marco", "#a", in(m), to("you"))
			return []any{w.where("you", m), w.e.Counts("you").Kept, w.e.Counts("you").Focus}, []any{"Focus/kept", int64(1), int64(0)}
		}},
		{"a new message brings a Done row back", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.done("you", "#a", th(m))
			r := w.post("marco", "#a", in(m), to("you"))
			return w.where("you", r), "Needs you"
		}},
		{"a row can be kept from a history tab though nothing of it is in the feed", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "muted")
			m := w.post("you", "#a")
			w.react("anna", m, "+1")
			w.keep("you", "#a", th(m))
			return w.where("you", m), "Focus/kept"
		}},
	})
}

func TestReading(t *testing.T) {
	unread := func(w *W, p, id string) bool {
		for _, r := range w.e.State(p).Rows {
			for _, it := range r.Items {
				if it.Msg == id {
					return !it.Read
				}
			}
		}
		return false
	}
	runCases(t, "D-015, D-032, contracts README, T-038 points 5 to 8 and 10", []ruleCase{
		{"reading a thread reads its row", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(m))
			return w.where("you", m), "Earlier"
		}},
		{"opening a channel reads a mention shown there", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.read("you", "#a", nil)
			return w.where("you", m), "Earlier"
		}},
		{"opening a channel leaves its threads' replies unread", func(w *W) (any, any) {
			m := w.post("you", "#a")
			r := w.post("anna", "#a", in(m))
			w.read("you", "#a", nil)
			return w.where("you", r), "Needs you"
		}},
		{"opening a thread reads the top and every reply", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			r := w.post("marco", "#a", in(m))
			w.read("you", "#a", th(m))
			return []bool{unread(w, "you", m), unread(w, "you", r)}, []bool{false, false}
		}},
		{"reading one thread leaves another unread", func(w *W) (any, any) {
			a, b := w.post("anna", "#a", to("you")), w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(a))
			return []string{w.where("you", a), w.where("you", b)}, []string{"Earlier", "Needs you"}
		}},
		{"a read reaches only as far as its position", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			upTo := w.n
			r := w.post("marco", "#a", in(m), to("you"))
			w.apply(func(h hdr) any {
				return contract.ChangeReaderRead{Kind: "reader.read", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: "you", Conversation: "#a", Thread: th(m), UpTo: upTo}
			})
			return []bool{unread(w, "you", m), unread(w, "you", r)}, []bool{false, true}
		}},
		{"an older read position moves nothing back", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(m))
			w.apply(func(h hdr) any {
				return contract.ChangeReaderRead{Kind: "reader.read", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Person: "you", Conversation: "#a", Thread: th(m), UpTo: 1}
			})
			return w.where("you", m), "Earlier"
		}},
		{"a direct conversation is read whole", func(w *W) (any, any) {
			id := w.dm("anna", "you")
			m := w.post("anna", id)
			r := w.post("anna", id, in(m))
			w.read("you", id, nil)
			return []bool{unread(w, "you", m), unread(w, "you", r)}, []bool{false, false}
		}},
		{"an arrival in a watched thread is read at once: no count", func(w *W) (any, any) {
			m := w.post("you", "#a")
			w.watch("you", "#a", th(m), "c1", true)
			r := w.post("anna", "#a", in(m))
			return []any{w.where("you", r), w.e.Counts("you").NeedsYou}, []any{"Earlier", int64(0)}
		}},
		{"watching a channel does not read a reply in its threads", func(w *W) (any, any) {
			m := w.post("you", "#a")
			w.watch("you", "#a", nil, "c1", true)
			return w.where("you", w.post("anna", "#a", in(m))), "Needs you"
		}},
		{"watching lasts while one connection still has it open", func(w *W) (any, any) {
			m := w.post("you", "#a")
			w.watch("you", "#a", th(m), "c1", true)
			w.watch("you", "#a", th(m), "c2", true)
			w.watch("you", "#a", th(m), "c1", false)
			return w.where("you", w.post("anna", "#a", in(m))), "Earlier"
		}},
		{"nothing is read once nobody is watching", func(w *W) (any, any) {
			m := w.post("you", "#a")
			w.watch("you", "#a", th(m), "c1", true)
			w.watch("you", "#a", th(m), "c1", false)
			return w.where("you", w.post("anna", "#a", in(m))), "Needs you"
		}},
		{"a deleted unread row no longer counts", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.del(m)
			return w.e.Counts("you").NeedsYou, int64(0)
		}},
	})
}

func TestEditsAndDeletes(t *testing.T) {
	runCases(t, "D-004, D-011, D-013", []ruleCase{
		{"an edit that adds a mention arrives as new", func(w *W) (any, any) {
			m := w.post("anna", "#a")
			w.edit(m, "now @you", to("you"))
			return w.where("you", m), "Needs you"
		}},
		{"an edit that names you on a row you had read arrives as new, and notifies", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "all")
			m := w.post("anna", "#a")
			w.read("you", "#a", nil)
			w.edit(m, "now @you", to("you"))
			return []string{w.where("you", m), w.kinds("you")}, []string{"Needs you", "held"}
		}},
		{"a second edit that removes it before reading leaves no trace", func(w *W) (any, any) {
			m := w.post("anna", "#a")
			w.edit(m, "x", to("you"))
			w.edit(m, "y")
			return w.where("you", m), "nothing"
		}},
		{"an edit that removes your mention after reading leaves a marker", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(m))
			w.edit(m, "y")
			return w.where("you", m), "Earlier (marker)"
		}},
		{"a text-only edit does not make a read row unread", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(m))
			w.edit(m, "typo fixed", to("you"))
			return w.where("you", m), "Earlier"
		}},
		{"a text-only edit updates the row in place and says edited", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.edit(m, "typo fixed", to("you"))
			r := w.row("you", m)
			return []any{r.Preview, r.State, r.Bucket}, []any{"typo fixed", contract.RowState("changed"), contract.Bucket("needs_you")}
		}},
		{"an edit removing Info arrives as new, also if read", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), info)
			w.read("you", "#a", th(m))
			w.edit(m, "x", to("you"))
			return w.where("you", m), "Needs you"
		}},
		{"an edit lowering an unread message moves it down quietly (R13)", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.edit(m, "x", to("you"), info)
			return w.where("you", m), "FYI"
		}},
		{"deleted before seen: nothing", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.del(m)
			return w.where("you", m), "nothing"
		}},
		{"deleted after reading: a marker", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(m))
			w.del(m)
			return w.where("you", m), "Earlier (marker)"
		}},
		{"a kept row keeps its marker in Focus until Done", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			w.del(m)
			a := w.where("you", m)
			state := w.row("you", m).State
			w.done("you", "#a", th(m))
			return []any{a, state, w.where("you", m)}, []any{"Focus/kept (marker)", contract.RowState("retracted"), "Earlier (marker)"}
		}},
		{"a deleted message shows no text anywhere", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			w.del(m)
			return w.row("you", m).Preview, ""
		}},
		{"a deleted flag you read is released with a marker", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), flag)
			w.read("you", "#a", th(m))
			w.del(m)
			return w.where("you", m), "Earlier (marker)"
		}},
		{"an edit of a deleted message changes nothing: a delete wins (D-006)", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(m))
			w.del(m)
			w.edit(m, "back", to("you"))
			return w.where("you", m), "Earlier (marker)"
		}},
	})
}

func TestReactions(t *testing.T) {
	runCases(t, "D-014, D-004, T-038 points 18 and 19", []ruleCase{
		{"a reaction from someone you addressed is an answer in Needs you", func(w *W) (any, any) {
			m := w.post("you", "#a", to("marco"))
			w.react("marco", m, "check")
			return []any{w.where("you", m, "answer:marco"), w.row("you", m).Why}, []any{"Needs you", "Marco reacted check to your message"}
		}},
		{"anyone else's stays grouped in For your information", func(w *W) (any, any) {
			m := w.post("you", "#a", to("marco"))
			w.react("anna", m, "tada")
			return w.where("you", m, "reaction"), "FYI"
		}},
		{"the grouped row shows the true count per emoji", func(w *W) (any, any) {
			m := w.post("you", "#a")
			w.react("anna", m, "check")
			w.react("marco", m, "check")
			w.react("lena", m, "x")
			return w.row("you", m).Why, "check 2 · x 1 on your message"
		}},
		{"a row shows the reactions on its message", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.react("marco", m, "eyes")
			return w.row("you", m).Reactions, []contract.Reaction{{Emoji: "eyes", People: []string{"marco"}}}
		}},
		{"no emoji has a meaning of its own", func(w *W) (any, any) {
			a, b := w.post("you", "#a", to("marco")), w.post("you", "#a", to("marco"))
			w.react("marco", a, "check")
			w.react("marco", b, "x")
			return w.where("you", a, "answer:marco"), w.where("you", b, "answer:marco")
		}},
		{"a reaction never lands in Focus", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "focus")
			m := w.post("you", "#a", to("marco"))
			w.react("marco", m, "+1")
			return w.where("you", m, "answer:marco"), "Needs you"
		}},
		{"an agent's reaction stays grouped (R34)", func(w *W) (any, any) {
			m := w.post("you", "#a", to("bot"))
			w.react("bot", m, "eyes")
			return w.where("you", m, "reaction"), "FYI"
		}},
		{"an answer reaches you from a muted channel (R35)", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "muted")
			m := w.post("you", "#a", to("marco"))
			w.react("marco", m, "+1")
			return w.where("you", m, "answer:marco"), "Needs you"
		}},
		{"applause does not reach you from a muted channel (R18)", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "muted")
			m := w.post("you", "#a")
			w.react("anna", m, "+1")
			return w.where("you", m, "reaction"), "nothing"
		}},
		{"a reaction to a flag naming nobody is applause (R43)", func(w *W) (any, any) {
			m := w.post("you", "#a", flag)
			w.react("anna", m, "+1")
			return w.where("you", m, "reaction"), "FYI"
		}},
		{"an answer taken back before you saw it leaves nothing", func(w *W) (any, any) {
			m := w.post("you", "#a", to("marco"))
			w.react("marco", m, "+1")
			w.unreact("marco", m, "+1")
			return w.where("you", m, "answer:marco"), "nothing"
		}},
		{"the same person's other emoji keeps the answer", func(w *W) (any, any) {
			m := w.post("you", "#a", to("marco"))
			w.react("marco", m, "+1")
			w.react("marco", m, "eyes")
			w.unreact("marco", m, "+1")
			return w.where("you", m, "answer:marco"), "Needs you"
		}},
		{"the count follows when one is taken back", func(w *W) (any, any) {
			m := w.post("you", "#a")
			w.react("anna", m, "+1")
			w.react("lena", m, "+1")
			w.unreact("anna", m, "+1")
			return w.row("you", m).Why, "+1 1 on your message"
		}},
		{"a repeated reaction counts once", func(w *W) (any, any) {
			m := w.post("you", "#a")
			w.react("anna", m, "+1")
			w.react("anna", m, "+1")
			return w.row("you", m).Why, "+1 1 on your message"
		}},
	})
}

func TestStoppedReplies(t *testing.T) {
	runCases(t, "D-031 choice 5, D-032", []ruleCase{
		{"a stopped reply is For your information for people in the thread", func(w *W) (any, any) {
			r := w.post("you", "#a")
			return w.where("you", w.post("bot", "#a", in(r), stopped)), "FYI"
		}},
		{"it overrides the thread's level", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.dial("you", "#a", th(r), "focus")
			return w.where("you", w.post("bot", "#a", in(r), stopped)), "FYI"
		}},
		{"at Off it does not arrive", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.dial("you", "#a", th(r), "off")
			return w.where("you", w.post("bot", "#a", in(r), stopped)), "nothing"
		}},
		{"nobody outside the thread gets it", func(w *W) (any, any) {
			r := w.post("anna", "#a")
			return w.where("you", w.post("bot", "#a", in(r), stopped)), "nothing"
		}},
		{"in a direct conversation it reaches the other person", func(w *W) (any, any) {
			return w.where("you", w.post("bot", w.dm("bot", "you"), stopped)), "FYI"
		}},
		{"it never notifies", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.post("bot", "#a", in(r), stopped)
			return w.kinds("you"), ""
		}},
	})
}

func TestNotifications(t *testing.T) {
	runCases(t, "D-012, D-018, D-004, D-015, feed-to-notification-sender.md", []ruleCase{
		{"a row becoming unread in Needs you is due, with the contract's name", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			n := w.notes[len(w.notes)-1]
			return []any{n.Kind, n.Name, n.Bucket, n.Text, *n.Row.Thread}, []any{"held", fmt.Sprintf("acme / you / #a:%s / %d", m, w.n), contract.Bucket("needs_you"), "text of " + m, m}
		}},
		{"For your information never notifies", func(w *W) (any, any) {
			w.post("anna", "#a", to("channel"))
			return w.kinds("you"), ""
		}},
		{"Info never notifies", func(w *W) (any, any) {
			w.post("anna", "#a", to("you"), info)
			return w.kinds("you"), ""
		}},
		{"one notification per row while unread", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.post("anna", "#a", in(r))
			w.post("marco", "#a", in(r))
			return w.kinds("you"), "held"
		}},
		{"a row moving up into Focus notifies once more", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.post("anna", "#a", in(r))
			w.post("marco", "#a", in(r), to("you"), flag)
			return w.kinds("you"), "held held"
		}},
		{"a row read and active again notifies again", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.post("anna", "#a", in(r))
			w.wait(6)
			w.tick()
			w.read("you", "#a", th(r))
			w.post("anna", "#a", in(r))
			return w.kinds("you"), "held send update:read held"
		}},
		{"Focus only: a Needs you row sends nothing", func(w *W) (any, any) {
			w.focusOnly("you", true)
			w.post("anna", "#a", to("you"))
			return w.kinds("you"), ""
		}},
		{"Focus only: a Focus row is due", func(w *W) (any, any) {
			w.focusOnly("you", true)
			w.post("anna", w.dm("anna", "you"))
			return w.kinds("you"), "held"
		}},
		{"an arrival in a watched conversation sends nothing", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.watch("you", "#a", th(r), "c1", true)
			w.post("anna", "#a", in(r))
			return w.kinds("you"), ""
		}},
		{"read within the hold removes the entry and writes nothing", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.wait(0.3)
			w.read("you", "#a", th(m))
			w.wait(6)
			w.tick()
			return []any{w.kinds("you"), len(w.e.Held("you"))}, []any{"held", 0}
		}},
		{"deleted within the hold removes the entry and writes nothing", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.wait(0.3)
			w.del(m)
			w.wait(6)
			w.tick()
			return w.kinds("you"), "held"
		}},
		{"read after the send updates the entry", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.wait(6)
			w.tick()
			w.wait(60)
			w.read("you", "#a", th(m))
			return w.kinds("you"), "held send update:read"
		}},
		{"deleted after the send updates the entry, also once read", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.wait(6)
			w.tick()
			w.read("you", "#a", th(m))
			w.del(m)
			return w.kinds("you"), "held send update:read update:deleted"
		}},
		{"an edit during the hold is in the send, with no note of its own", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.edit(m, "new text", to("you"))
			w.wait(6)
			w.tick()
			n := w.notes[len(w.notes)-1]
			return []string{w.kinds("you"), n.Text}, []string{"held send", "new text"}
		}},
		{"an edit after the send carries the new text", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.wait(6)
			w.tick()
			w.edit(m, "new text", to("you"))
			n := w.notes[len(w.notes)-1]
			return []string{n.Kind, n.Text}, []string{"update", "new text"}
		}},
		{"a mention edited away within the hold removes the entry", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.wait(1)
			w.edit(m, "no mention")
			w.wait(6)
			w.tick()
			return w.kinds("you"), "held"
		}},
		{"a reaction taken back within the hold removes the entry", func(w *W) (any, any) {
			m := w.post("you", "#a", to("marco"))
			w.react("marco", m, "+1")
			w.wait(1)
			w.unreact("marco", m, "+1")
			w.wait(6)
			w.tick()
			return w.kinds("you"), "held"
		}},
		{"a dial change within the hold removes the entry", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.post("anna", "#a", in(r))
			w.wait(1)
			w.dial("you", "#a", th(r), "off")
			w.wait(6)
			w.tick()
			return w.kinds("you"), "held"
		}},
		{"a dial change after the send writes nothing", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.post("anna", "#a", in(r))
			w.wait(6)
			w.tick()
			w.dial("you", "#a", th(r), "off")
			return w.kinds("you"), "held send"
		}},
		{"a row lifted by your own rule sends nothing (R40)", func(w *W) (any, any) {
			w.post("anna", "#a", to("you"))
			w.rule("you", "item_kind", "mentions", true)
			return w.kinds("you"), "held"
		}},
		{"a deleted message's text is not sent from a row still due", func(w *W) (any, any) {
			r := w.post("you", "#a")
			a := w.post("anna", "#a", in(r))
			b := w.post("marco", "#a", in(r))
			w.del(a)
			w.wait(6)
			w.tick()
			n := w.notes[len(w.notes)-1]
			return []string{n.Kind, n.Text}, []string{"send", "text of " + b}
		}},
		{"a sent entry whose message goes takes the text of what is left", func(w *W) (any, any) {
			r := w.post("you", "#a")
			a := w.post("anna", "#a", in(r))
			b := w.post("marco", "#a", in(r))
			w.wait(6)
			w.tick()
			w.del(a)
			n := w.notes[len(w.notes)-1]
			return []string{n.Kind, n.Text}, []string{"update", "text of " + b}
		}},
		{"\"Focus only\" switched on during the hold removes a Needs you entry", func(w *W) (any, any) {
			w.post("anna", "#a", to("you"))
			w.focusOnly("you", true)
			w.wait(6)
			w.tick()
			return w.kinds("you"), "held"
		}},
		{"a tick writes nothing when nothing is due", func(w *W) (any, any) {
			fx := w.tick()
			return len(fx.Notes), 0
		}},
		{"a fence removes what is on hold, and only inside it", func(w *W) (any, any) {
			w.post("anna", "#a", to("you"))
			w.post("anna", "#a", to("marco"))
			w.post("anna", "#b", to("you"))
			removed := w.e.Fence("#a", "you")
			w.wait(6)
			w.tick()
			return []any{len(removed), w.kinds("you"), w.kinds("marco")}, []any{1, "held held send", "held send"}
		}},
		{"an edit that newly names you notifies (R36)", func(w *W) (any, any) {
			m := w.post("anna", "#a")
			w.edit(m, "x", to("you"))
			return w.kinds("you"), "held"
		}},
	})
}

// The nine behaviours T-021 still has open, as the E-001 model has them.
func TestOpenBehavioursOfT021(t *testing.T) {
	runCases(t, "T-021, E-001 README \"Still open\"", []ruleCase{
		{"R34 an agent you addressed reacts: grouped, not an answer", func(w *W) (any, any) {
			m := w.post("you", "#a", to("bot"))
			w.react("bot", m, "eyes")
			return []string{w.where("you", m, "reaction"), w.where("you", m, "answer:bot")}, []string{"FYI", "nothing"}
		}},
		{"R35 an answer by reaction reaches you from a muted channel", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "muted")
			m := w.post("you", "#a", to("marco"))
			w.react("marco", m, "+1")
			return w.where("you", m, "answer:marco"), "Needs you"
		}},
		{"R37 a row kept by a flag sits in Focus without the flags rule, once read", func(w *W) (any, any) {
			w.rule("you", "item_kind", "flags", false)
			m := w.post("anna", "#a", to("you"), flag)
			before := w.where("you", m)
			w.read("you", "#a", th(m))
			return []string{before, w.where("you", m)}, []string{"Needs you", "Focus/kept"}
		}},
		{"R38 new activity on a kept row notifies by the bucket the new item earns", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			w.notes = nil
			w.post("marco", "#a", in(m), to("you"))
			return w.notes[0].Bucket, contract.Bucket("needs_you")
		}},
		{"R40 a row lifted into Focus by your own change sends nothing", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(m))
			r := w.post("marco", "#a", in(m))
			w.notes = nil
			w.dial("you", "#a", th(m), "focus")
			return []any{w.where("you", r), len(w.notes)}, []any{"Focus/new", 0}
		}},
		{"R41 in a focused channel, replies in a thread you were only mentioned in land in Focus", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "focus")
			m := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(m))
			return w.where("you", w.post("marco", "#a", in(m))), "Focus/new"
		}},
		{"R42 a reply releases everything kept on the row", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), flag)
			r := w.post("marco", "#a", in(m), to("you"), flag)
			w.keep("you", "#a", th(m))
			w.post("you", "#a", in(m))
			return []string{w.where("you", m), w.where("you", r)}, []string{"Earlier", "Earlier"}
		}},
		{"R42 a reaction releases the flag on that message and your Keep, not another flag", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"), flag)
			r := w.post("marco", "#a", in(m), to("you"), flag)
			w.keep("you", "#a", th(m))
			w.react("you", m, "+1")
			return w.e.State("you").Rows[0].Flagged, []string{r}
		}},
		{"R43 a reaction to a flag naming nobody is applause", func(w *W) (any, any) {
			m := w.post("you", "#a", flag)
			w.react("anna", m, "+1")
			return []string{w.where("you", m, "reaction"), w.where("you", m, "answer:anna")}, []string{"FYI", "nothing"}
		}},
		{"R44 when a mention of you is deleted, the thread stops following you", func(w *W) (any, any) {
			m := w.post("anna", "#a")
			n := w.post("marco", "#a", in(m), to("you"))
			w.read("you", "#a", th(m))
			w.del(n)
			return w.where("you", w.post("lena", "#a", in(m))), "nothing"
		}},
	})
}

func TestDialLevelsMustFit(t *testing.T) {
	// The schema has one list of levels for channels and threads; each takes its own.
	w := world(t)
	bad := []contract.ChangeReaderDial{
		{Kind: "reader.dial", Number: 90, At: w.time(), Person: "you", Conversation: "#a", Level: "needs_you"},
		{Kind: "reader.dial", Number: 91, At: w.time(), Person: "you", Conversation: "#a", Thread: th("m1"), Level: "muted"},
	}
	for _, c := range bad {
		if _, err := w.e.Apply(contract.Change{Value: c}, Options{}); !errors.Is(err, ErrCannotApply) {
			t.Errorf("level %s: got %v, want ErrCannotApply", c.Level, err)
		}
	}
}

func TestSeededHistoryWritesNoNotes(t *testing.T) {
	w := world(t) // D-032: seeded history creates no notifications
	w.n++
	fx, err := w.e.Apply(contract.Change{Value: contract.ChangeMessageCreated{Kind: "message.created", Org: "acme", Number: w.n, Previous: w.n - 1, At: w.time(),
		Message: contract.Message{Id: "s1", Number: w.n, Version: 1, Conversation: "#a", Author: "anna", Text: "old", Mentions: []string{"you"}, Reactions: []contract.Reaction{}, CreatedAt: w.time()}}}, Options{NoNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(fx.Notes) != 0 || len(fx.Held) != 0 {
		t.Fatalf("seeded history wrote notes %v or held %v", fx.Notes, fx.Held)
	}
	if w.where("you", "s1") != "Needs you" {
		t.Fatalf("seeded history did not reach the feed: %s", w.where("you", "s1"))
	}
}

// D-033: the log itself says where seeded history starts and ends. Nothing from
// seed.opened up to seed.sealed writes a note, with no option from the caller;
// the first change after it does, and a log without the opener is as before.
func TestNoNotesUpToTheSeal(t *testing.T) {
	const at = "2026-11-22T10:00:00.000Z"
	e := New("acme")
	n := int64(0)
	apply := func(build func(n, prev int64) any) Effects {
		t.Helper()
		n++
		fx, err := e.Apply(contract.Change{Value: build(n, n-1)}, Options{})
		if err != nil {
			t.Fatal(err)
		}
		return fx
	}
	opened := contract.ChangeSeedOpened{Kind: "seed.opened", Org: "acme", Number: 1, Previous: 0, At: at}
	if !IsOrgWide(contract.Change{Value: opened}) {
		t.Fatal("seed.opened must be applied as an organization-wide change")
	}
	if fx := apply(func(int64, int64) any { return opened }); !fx.Known || len(fx.Notes) != 0 {
		t.Fatalf("the opener: known %v, notes %v", fx.Known, fx.Notes)
	}
	mention := func(id string) func(n, prev int64) any {
		return func(n, prev int64) any {
			return contract.ChangeMessageCreated{Kind: "message.created", Org: "acme", Number: n, Previous: prev, At: at,
				Message: contract.Message{Id: id, Number: n, Version: 1, Conversation: "#a", Author: "anna", Text: "@you", Mentions: []string{"you"}, Reactions: []contract.Reaction{}, CreatedAt: at}}
		}
	}
	for _, p := range []string{"anna", "you"} {
		apply(func(n, prev int64) any {
			return contract.ChangePersonJoined{Kind: "person.joined", Org: "acme", Number: n, Previous: prev, At: at, Person: p, Name: p}
		})
	}
	apply(func(n, prev int64) any {
		return contract.ChangeChannelCreated{Kind: "channel.created", Org: "acme", Number: n, Previous: prev, At: at, Conversation: "#a", Name: "a", People: []string{"anna", "you"}}
	})
	if fx := apply(mention("seeded")); len(fx.Notes) != 0 || len(fx.Held) != 0 {
		t.Fatalf("a seeded mention wrote notes %v or held %v", fx.Notes, fx.Held)
	}
	if fx := apply(func(n, prev int64) any {
		return contract.ChangeTick{Kind: "tick", Org: "acme", Number: n, Previous: prev, At: at}
	}); len(fx.Notes) != 0 {
		t.Fatalf("a tick before the seal wrote notes: %v", fx.Notes)
	}
	sealed := contract.Change{Value: contract.ChangeSeedSealed{Kind: "seed.sealed", Org: "acme", Number: n + 1, Previous: n, At: at}}
	if !IsOrgWide(sealed) {
		t.Fatal("seed.sealed must be applied as an organization-wide change")
	}
	fx := apply(func(n, prev int64) any { return sealed.Value })
	if !fx.Known || len(fx.Notes) != 0 {
		t.Fatalf("the seal: known %v, notes %v", fx.Known, fx.Notes)
	}
	if fx := apply(mention("live")); len(fx.Held) == 0 {
		t.Fatal("the first mention after the seal put nothing on hold")
	}

	plain := New("acme")
	for i, c := range []any{
		contract.ChangePersonJoined{Kind: "person.joined", Org: "acme", Number: 1, Previous: 0, At: at, Person: "anna", Name: "anna"},
		contract.ChangePersonJoined{Kind: "person.joined", Org: "acme", Number: 2, Previous: 1, At: at, Person: "you", Name: "you"},
		contract.ChangeChannelCreated{Kind: "channel.created", Org: "acme", Number: 3, Previous: 2, At: at, Conversation: "#a", Name: "a", People: []string{"anna", "you"}},
	} {
		if _, err := plain.Apply(contract.Change{Value: c}, Options{}); err != nil {
			t.Fatalf("change %d: %v", i, err)
		}
	}
	fx, err := plain.Apply(contract.Change{Value: mention("no-opener")(4, 3)}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fx.Held) == 0 {
		t.Fatal("a log without the opener put nothing on hold for a mention")
	}
}

func TestHistoryTabs(t *testing.T) {
	runCases(t, "D-016, D-014, D-004, T-038 point 14", []ruleCase{
		{"Mentions holds read and unread, from any channel, newest first", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "muted")
			a := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(a))
			b := w.post("marco", "#b", to("you"))
			return w.tab("you", "mentions"), b + "* " + a
		}},
		{"a deleted message or removed mention is not there", func(w *W) (any, any) {
			a, b := w.post("anna", "#a", to("you")), w.post("anna", "#a", to("you"))
			w.del(a)
			w.edit(b, "x")
			return w.tab("you", "mentions"), ""
		}},
		{"a deleted message leaves every tab", func(w *W) (any, any) {
			id := w.dm("anna", "you")
			d := w.post("anna", id)
			t1 := w.post("you", "#a")
			r := w.post("anna", "#a", in(t1))
			w.react("anna", t1, "+1")
			w.del(d)
			w.del(r)
			w.del(t1)
			return []string{w.tab("you", "direct"), w.tab("you", "threads"), w.tab("you", "reactions")}, []string{"", "", ""}
		}},
		{"Direct messages lists the other person's messages", func(w *W) (any, any) {
			id := w.dm("anna", "you")
			a := w.post("anna", id)
			w.post("you", id)
			return w.tab("you", "direct"), a
		}},
		{"Threads lists replies in threads that are not Off, with the level", func(w *W) (any, any) {
			t1 := w.post("you", "#a")
			r := w.post("anna", "#a", in(t1))
			t2 := w.post("anna", "#a")
			w.post("marco", "#a", in(t2))
			e := w.e.History("you", "threads").Entries
			return []any{len(e), e[0].Message, *e[0].Level}, []any{1, r, contract.DialLevel("needs_you")}
		}},
		{"Reactions lists each reaction with who and what", func(w *W) (any, any) {
			m := w.post("you", "#a", to("marco"))
			w.react("marco", m, "check")
			w.react("anna", m, "tada")
			e := w.e.History("you", "reactions").Entries
			return []string{*e[0].By, *e[0].Emoji, *e[1].By}, []string{"anna", "tada", "marco"}
		}},
		{"a reaction from a muted channel still is in the tab, unread", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "muted")
			m := w.post("you", "#a")
			w.react("anna", m, "+1")
			return w.e.History("you", "reactions").Unread, int64(1)
		}},
		{"the count of a tab follows reading", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			before := w.e.History("you", "mentions").Unread
			w.read("you", "#a", th(m))
			return []int64{before, w.e.History("you", "mentions").Unread}, []int64{1, 0}
		}},
		{"nothing leaves a tab by being read", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(m))
			return w.tab("you", "mentions"), m
		}},
		{"an arrival sends the tab's new count", func(w *W) (any, any) {
			w.post("anna", "#a", to("you"))
			for _, re := range w.last.Readers {
				if re.Person == "you" {
					for _, p := range re.Parts {
						if h, ok := p.Value.(contract.FeedPartHistoryChanged); ok && h.Tab == "mentions" {
							return h.Unread, int64(1)
						}
					}
				}
			}
			return "no part", int64(1)
		}},
	})
}

func TestRowExplanation(t *testing.T) {
	label := func(w *W, id string) string {
		c := w.row("you", id).Explanation.Change
		if c == nil {
			return "no change"
		}
		return c.Label
	}
	runCases(t, "D-020, D-021", []ruleCase{
		{"a Focus person's row offers to remove that person", func(w *W) (any, any) {
			m := w.post("anna", w.dm("anna", "you"))
			c := w.row("you", m).Explanation.Change
			return []any{c.Label, c.Do.Value, c.Undo.Value}, []any{"Remove Anna from Focus",
				contract.RowActFocusPerson{Kind: "focus_person", About: "anna", On: false}, contract.RowActFocusPerson{Kind: "focus_person", About: "anna", On: true}}
		}},
		{"a mention offers the sender's person rule", func(w *W) (any, any) {
			return label(w, w.post("anna", "#a", to("you"))), "Move what Anna sends into Focus"
		}},
		{"a flag in Focus offers the rule that put it there", func(w *W) (any, any) {
			return label(w, w.post("anna", "#a", to("you"), flag)), "Remove the rule “Flags that name you”"
		}},
		{"a focused channel offers its dial", func(w *W) (any, any) {
			w.dial("you", "#a", nil, "focus")
			return label(w, w.post("anna", "#a")), "Set #a to Default"
		}},
		{"a reply in your thread offers to turn it off", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.post("anna", "#a", in(r))
			return label(w, r), "Turn this thread off"
		}},
		{"a kept row offers Done", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			return label(w, m), "Mark done"
		}},
		{"an Info row has no change: the sender's level is theirs", func(w *W) (any, any) {
			return label(w, w.post("anna", "#a", to("you"), info)), "no change"
		}},
		{"every row says what, what decided, and otherwise", func(w *W) (any, any) {
			e := w.row("you", w.post("anna", "#a", to("you"))).Explanation
			return e.What != "" && e.DecidedBy != "" && e.Otherwise != "", true
		}},
		{"asking why does not read the row", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			_ = w.row("you", m).Explanation
			_, _ = w.e.Preview("you", ruleAct(contract.FocusRule{Type: "person", Value: "anna"}, true), time.Date(2026, 11, 22, 11, 0, 0, 0, time.UTC))
			return w.row("you", m).Unread, true
		}},
		{"the preview counts what would have moved and where to", func(w *W) (any, any) {
			w.post("anna", "#a", to("you"))
			w.post("anna", "#a", to("you"))
			w.post("marco", "#a", to("you"))
			a, _ := w.e.Preview("you", ruleAct(contract.FocusRule{Type: "person", Value: "anna"}, true), time.Date(2026, 11, 22, 11, 0, 0, 0, time.UTC))
			return []any{a.Last7Days.Moved, *a.Last7Days.From, *a.Last7Days.To, a.Last7Days.Stay}, []any{int64(2), contract.Bucket("needs_you"), contract.Bucket("focus"), int64(1)}
		}},
		{"the preview looks back seven days only", func(w *W) (any, any) {
			w.post("anna", "#a", to("you"))
			a, _ := w.e.Preview("you", ruleAct(contract.FocusRule{Type: "person", Value: "anna"}, true), time.Date(2026, 12, 1, 11, 0, 0, 0, time.UTC))
			return a.Last7Days.Moved, int64(0)
		}},
		{"a preview changes nothing", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			_, _ = w.e.Preview("you", dialAct(w.e.conv("#a"), nil, "focus"), time.Date(2026, 11, 22, 11, 0, 0, 0, time.UTC))
			return w.where("you", m), "Needs you"
		}},
	})
}

func TestCountsAndOrder(t *testing.T) {
	runCases(t, "D-003, T-038 points 12, 13, 15, 16 and 17", []ruleCase{
		{"counts are unread rows, and a dot for For your information", func(w *W) (any, any) {
			w.post("anna", w.dm("anna", "you"))
			w.post("anna", "#a", to("you"))
			w.post("marco", "#a", to("you"))
			w.post("anna", "#a", to("channel"))
			return w.e.Counts("you"), contract.FeedCounts{Focus: 1, NeedsYou: 2, Fyi: true}
		}},
		{"Kept has its own count, apart from Focus", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(m))
			return w.e.Counts("you"), contract.FeedCounts{Kept: 1}
		}},
		{"newest activity first within a section", func(w *W) (any, any) {
			a := w.post("anna", "#a", to("you"))
			b := w.post("marco", "#a", to("you"))
			w.post("lena", "#a", in(a), to("you"))
			rows := w.e.Rows("you")
			return []string{*rows[0].Thread, *rows[1].Thread}, []string{a, b}
		}},
		{"Focus new sits above Kept, above Needs you", func(w *W) (any, any) {
			k := w.post("anna", "#a", to("you"))
			w.keep("you", "#a", th(k))
			n := w.post("marco", "#a", to("you"))
			f := w.post("anna", w.dm("anna", "you"))
			rows := w.e.Rows("you")
			return []string{rows[0].Message, rows[1].Message, rows[2].Message}, []string{f, k, n}
		}},
		{"a row that changed is one part, however many messages", func(w *W) (any, any) {
			r := w.post("you", "#a")
			w.post("anna", "#a", in(r))
			fx := w.apply(func(h hdr) any {
				return contract.ChangeMessageCreated{Kind: "message.created", Org: "acme", Number: h.n, Previous: h.p, At: h.at, Message: contract.Message{
					Id: "x1", Number: h.n, Version: 1, Conversation: "#a", Thread: &r, Author: "marco", Text: "t", Reactions: []contract.Reaction{}, CreatedAt: h.at}}
			})
			n := 0
			for _, re := range fx.Readers {
				if re.Person == "you" {
					for _, p := range re.Parts {
						if _, ok := p.Value.(contract.FeedPartRowChanged); ok {
							n++
						}
					}
				}
			}
			return n, 1
		}},
		{"a row never seen and taken back is removed", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.del(m)
			for _, re := range w.last.Readers {
				if re.Person == "you" {
					_, ok := re.Parts[0].Value.(contract.FeedPartRowRemoved)
					return ok, true
				}
			}
			return false, true
		}},
		{"a read row stays with its screen, marked read, and is not in the next load", func(w *W) (any, any) {
			m := w.post("anna", "#a", to("you"))
			w.read("you", "#a", th(m))
			var part contract.FeedPartRowChanged
			for _, re := range w.last.Readers {
				if re.Person == "you" {
					part, _ = re.Parts[0].Value.(contract.FeedPartRowChanged)
				}
			}
			return []any{part.Row.Unread, len(w.e.Rows("you"))}, []any{false, 0}
		}},
	})
}

func TestChangesThatCannotBeApplied(t *testing.T) {
	// conversations-to-feed.md, section 4: the feed fences what it cannot apply.
	w := world(t)
	before := w.e.State("you")
	for name, c := range map[string]any{
		"unknown conversation":   contract.ChangeMessageCreated{Kind: "message.created", Number: 99, At: w.time(), Message: contract.Message{Id: "x", Conversation: "#nope", Author: "anna"}},
		"reply to nothing":       contract.ChangeMessageCreated{Kind: "message.created", Number: 99, At: w.time(), Message: contract.Message{Id: "x", Conversation: "#a", Author: "anna", Thread: th("gone")}},
		"bad time":               contract.ChangeTick{Kind: "tick", Number: 99, At: "yesterday"},
		"reaction by a stranger": contract.ChangeReactionAdded{Kind: "reaction.added", Number: 99, At: w.time(), Person: "nobody", Message: "x", Conversation: "#a"},
		"unknown person's rule":  contract.ChangeReaderFocusOnly{Kind: "reader.focus_only", Number: 99, At: w.time(), Person: "nobody", On: true},
	} {
		if _, err := w.e.Apply(contract.Change{Value: c}, Options{}); !errors.Is(err, ErrCannotApply) {
			t.Errorf("%s: got %v, want ErrCannotApply", name, err)
		}
	}
	if !reflect.DeepEqual(before, w.e.State("you")) {
		t.Error("a refused change altered the state")
	}
}

func TestKindsWithNoEffectAreCounted(t *testing.T) {
	// conversations-to-feed.md: counted and ignored.
	w := world(t)
	for _, c := range []any{
		contract.ChangeReaderPreference{Kind: "reader.preference", Number: 90, At: w.time(), Person: "you", Key: "k", Value: "v"},
		contract.ChangeReaderProfileLine{Kind: "reader.profile_line", Number: 91, At: w.time(), Person: "you", Text: "t"},
		contract.ChangeReplyStarted{Kind: "reply.started", Number: 92, At: w.time(), Message: "r", Conversation: "#a", Author: "bot"},
		contract.ChangeReplyAbandoned{Kind: "reply.abandoned", Number: 93, At: w.time(), Message: "r", Conversation: "#a"},
	} {
		fx, err := w.e.Apply(contract.Change{Value: c}, Options{})
		if err != nil || fx.Known || len(fx.Readers) != 0 || len(fx.Notes) != 0 {
			t.Errorf("%T: known=%v readers=%d notes=%d err=%v", c, fx.Known, len(fx.Readers), len(fx.Notes), err)
		}
	}
}

func TestNoteContractForm(t *testing.T) {
	// The drafts become the contract's notes with the feed's own numbers.
	send := NoteDraft{Kind: "send", At: "t", Name: "n", Person: "you", Bucket: "focus", Why: "w", Text: "x", DueAt: "d"}.Contract("acme", 10, 9, 7)
	v, ok := send.Value.(contract.NoteSend)
	if !ok || v.Number != 10 || v.Previous != 9 || v.Edition != 7 || v.Org != "acme" || v.Name != "n" || v.DueAt != "d" || v.History != nil {
		t.Errorf("send note: %+v", send.Value)
	}
	hist := NoteDraft{Kind: "send", History: true}.Contract("acme", 11, 10, 7).Value.(contract.NoteSend)
	if hist.History == nil || !*hist.History {
		t.Error("a history send has no mark")
	}
	u, ok := NoteDraft{Kind: "update", Name: "n", Now: "read"}.Contract("acme", 12, 11, 7).Value.(contract.NoteUpdate)
	if !ok || u.Now == nil || *u.Now != "read" || u.Text != nil {
		t.Errorf("update note: %+v", u)
	}
	x, _ := NoteDraft{Kind: "update", Name: "n", Text: "", HasText: true}.Contract("acme", 13, 12, 7).Value.(contract.NoteUpdate)
	if x.Text == nil {
		t.Error("an update to an empty text lost it")
	}
}
