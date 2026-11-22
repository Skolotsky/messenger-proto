package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"contexia/internal/shared/contract"
)

// The E-001 model's cases, exported by testdata/export_e001.py, replayed as log
// changes. At every case the whole state of every person is compared with the
// model's, not only the value the scenario asserted.

type e001File struct {
	Cases []struct {
		Name     string `json:"name"`
		Basis    string `json:"basis"`
		Got      string `json:"got"`
		Expected string `json:"expected"`
	} `json:"cases"`
	Worlds []struct {
		People   []string            `json:"people"`
		Agents   []string            `json:"agents"`
		Channels map[string][]string `json:"channels"`
		Ops      []map[string]any    `json:"ops"`
	} `json:"worlds"`
}

type e001Person struct {
	Rows          []StateRow          `json:"rows"`
	ThreadDials   map[string]string   `json:"thread_dials"`
	FocusPeople   []string            `json:"focus_people"`
	Notifications [][]any             `json:"notifications"`
	History       map[string][]string `json:"history"`
}

// e001Differences lists every case where the product deliberately differs from
// the model, with the reason. The same list is in tasks/T-049-feed-rules.md.
var e001Differences = map[string]string{}

type replay struct {
	t      *testing.T
	e      *Engine
	number int64
	base   time.Time
	now    float64
	conv   map[string]string // message -> conversation
	emoji  map[string]string // message/person -> emoji
	dues   map[string][][]any
	made   map[string]bool
	ver    map[string]int64
}

func (r *replay) apply(build func(n, prev int64, at string) any) {
	r.t.Helper()
	r.number++
	at := r.base.Add(time.Duration(r.now * float64(time.Second))).Format(time.RFC3339Nano)
	fx, err := r.e.Apply(contract.Change{Value: build(r.number, r.number-1, at)}, Options{})
	if err != nil {
		r.t.Fatalf("apply: %v", err)
	}
	for _, n := range fx.Notes {
		if n.Kind == "due" {
			b := map[contract.Bucket]float64{"focus": 3, "needs_you": 2, "fyi": 1}[n.Bucket]
			r.dues[n.Person] = append(r.dues[n.Person], []any{n.Message, b, n.Item})
		}
	}
}

func (r *replay) direct(conv string) {
	if r.made[conv] || !strings.HasPrefix(conv, "dm:") {
		return
	}
	r.made[conv] = true
	people := strings.Split(strings.TrimPrefix(conv, "dm:"), "|")
	r.apply(func(n, p int64, at string) any {
		return contract.ChangeDirectCreated{Kind: "direct.created", Org: "o", Number: n, Previous: p, At: at, Conversation: conv, People: people}
	})
}

func strs(v any) []string {
	out := []string{}
	if list, ok := v.([]any); ok {
		for _, x := range list {
			out = append(out, x.(string))
		}
	}
	return out
}

func strPtr(v any) *string {
	if s, ok := v.(string); ok {
		return &s
	}
	return nil
}

func (r *replay) rowOf(key string) (string, *string) {
	switch {
	case strings.HasPrefix(key, "th:"):
		id := strings.TrimPrefix(key, "th:")
		return r.conv[id], &id
	case strings.HasPrefix(key, "ch:"):
		return strings.TrimPrefix(key, "ch:"), nil
	}
	return strings.TrimPrefix(key, "dm:"), nil
}

func (r *replay) message(op map[string]any, id string, version int64, text string) contract.Message {
	mentions := strs(op["mentions"])
	if g, _ := op["group"].(bool); g {
		mentions = append(mentions, "channel")
	}
	var level *contract.Level
	if l, ok := op["level"].(string); ok {
		x := contract.Level(l)
		level = &x
	}
	author, _ := op["author"].(string)
	return contract.Message{Id: id, Version: version, Conversation: r.conv[id], Thread: strPtr(op["parent"]), Author: author,
		AuthorIsAgent: r.e.isAgent(author), Text: text, Mentions: mentions, Level: level, Reactions: []contract.Reaction{},
		CreatedAt: r.base.Add(time.Duration(r.now * float64(time.Second))).Format(time.RFC3339Nano)}
}

func (r *replay) do(op map[string]any) {
	r.t.Helper()
	r.now, _ = op["now"].(float64)
	person, _ := op["person"].(string)
	h := func(n, p int64, at string) (string, int64, int64, string) { return "o", n, p, at }
	switch op["op"] {
	case "post", "finish":
		id := op["id"].(string)
		conv := op["conv"].(string)
		r.direct(conv)
		r.conv[id] = conv
		if s, _ := op["streaming"].(bool); s {
			r.apply(func(n, p int64, at string) any {
				return contract.ChangeReplyStarted{Kind: "reply.started", Org: "o", Number: n, Previous: p, At: at, Message: id, Conversation: conv, Thread: strPtr(op["parent"]), Author: op["author"].(string)}
			})
			return
		}
		r.ver[id] = 1
		r.apply(func(n, p int64, at string) any {
			m := r.message(op, id, 1, "text of "+id)
			m.Number = n
			return contract.ChangeMessageCreated{Kind: "message.created", Org: "o", Number: n, Previous: p, At: at, Message: m}
		})
	case "edit":
		id := op["id"].(string)
		r.ver[id]++
		v := r.ver[id]
		r.apply(func(n, p int64, at string) any {
			m := r.message(op, id, v, fmt.Sprintf("text of %s, version %d", id, v))
			m.Author = r.e.conv(r.conv[id]).msgs[id].author
			m.Thread = nil
			if x := r.e.conv(r.conv[id]).msgs[id]; x.parent != "" {
				m.Thread = &x.parent
			}
			return contract.ChangeMessageChanged{Kind: "message.changed", Org: "o", Number: n, Previous: p, At: at, Message: m}
		})
	case "delete":
		id := op["mid"].(string)
		r.ver[id]++
		v := r.ver[id]
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeMessageDeleted{Kind: "message.deleted", Org: "o", Number: n, Previous: p, At: at, Message: contract.DeletedMessage{Id: id, Version: v, Conversation: r.conv[id]}}
		})
	case "react":
		id := op["mid"].(string)
		emoji, _ := op["emoji"].(string)
		if emoji == "" {
			emoji = "+1"
		}
		r.emoji[id+"/"+person] = emoji
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeReactionAdded{Kind: "reaction.added", Org: "o", Number: n, Previous: p, At: at, Person: person, Emoji: emoji, Message: id, Conversation: r.conv[id]}
		})
	case "unreact":
		id := op["mid"].(string)
		emoji := r.emoji[id+"/"+person]
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeReactionRemoved{Kind: "reaction.removed", Org: "o", Number: n, Previous: p, At: at, Person: person, Emoji: emoji, Message: id, Conversation: r.conv[id]}
		})
	case "read":
		conv, thread := r.rowOf(op["key"].(string))
		upTo := r.number
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeReaderRead{Kind: "reader.read", Org: "o", Number: n, Previous: p, At: at, Person: person, Conversation: conv, Thread: thread, UpTo: upTo}
		})
	case "keep":
		conv, thread := r.rowOf(op["key"].(string))
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeReaderKeep{Kind: "reader.keep", Org: "o", Number: n, Previous: p, At: at, Person: person, Conversation: conv, Thread: thread}
		})
	case "done":
		conv, thread := r.rowOf(op["key"].(string))
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeReaderDone{Kind: "reader.done", Org: "o", Number: n, Previous: p, At: at, Person: person, Conversation: conv, Thread: thread}
		})
	case "watch":
		conv, thread := r.rowOf(op["key"].(string))
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeReaderWatchingStarted{Kind: "reader.watching_started", Org: "o", Number: n, Previous: p, At: at, Person: person, Conversation: conv, Thread: thread, Connection: "c1"}
		})
	case "channel_dial":
		conv, level := op["conv"].(string), op["level"].(string)
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeReaderDial{Kind: "reader.dial", Org: "o", Number: n, Previous: p, At: at, Person: person, Conversation: conv, Level: contract.DialLevel(level)}
		})
	case "thread_dial":
		root, level := op["root"].(string), op["level"].(string)
		if level == "needs" {
			level = levelNeeds
		}
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeReaderDial{Kind: "reader.dial", Org: "o", Number: n, Previous: p, At: at, Person: person, Conversation: r.conv[root], Thread: &root, Level: contract.DialLevel(level)}
		})
	case "add_rule", "remove_rule":
		x := op["rule"].(map[string]any)
		rule := contract.FocusRule{Type: contract.RuleType(x["type"].(string)), Value: x["value"].(string)}
		on := op["op"] == "add_rule"
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeReaderFocusRule{Kind: "reader.focus_rule", Org: "o", Number: n, Previous: p, At: at, Person: person, Rule: rule, On: on}
		})
	case "remove_person", "add_person":
		who := op["who"].(string)
		on := op["op"] == "add_person"
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeReaderFocusPerson{Kind: "reader.focus_person", Org: "o", Number: n, Previous: p, At: at, Person: person, About: who, On: on}
		})
	case "auto_add_off":
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeReaderFocusAutoAdd{Kind: "reader.focus_auto_add", Org: "o", Number: n, Previous: p, At: at, Person: person, On: false}
		})
		for _, who := range strs(op["remove"]) {
			who := who
			r.apply(func(n, p int64, at string) any {
				return contract.ChangeReaderFocusPerson{Kind: "reader.focus_person", Org: "o", Number: n, Previous: p, At: at, Person: person, About: who, On: false}
			})
		}
	case "only_done":
		r.apply(func(n, p int64, at string) any {
			return contract.ChangeReaderReleaseSetting{Kind: "reader.release_setting", Org: "o", Number: n, Previous: p, At: at, Person: person, On: false}
		})
	case "preview":
		x := op["rule"].(map[string]any)
		rule := contract.FocusRule{Type: contract.RuleType(x["type"].(string)), Value: x["value"].(string)}
		ans, err := r.e.Preview(person, contract.RowAct{Value: contract.RowActFocusRule{Kind: "focus_rule", Rule: rule, On: true}}, r.base.Add(time.Hour))
		if err != nil {
			r.t.Fatal(err)
		}
		if float64(ans.Last7Days.Moved) != op["expected"].(float64) {
			r.t.Errorf("preview of %v: moved %d, the model says %v", rule, ans.Last7Days.Moved, op["expected"])
		}
	case "open_feed", "leave_feed":
		// What is on screen is the web app's; the log does not carry it.
	default:
		r.t.Fatalf("unknown op %v", op["op"])
	}
	_ = h
}

func compareCase(r *replay, want map[string]e001Person) []string {
	var diffs []string
	for _, u := range sortedKeys(want) {
		w := want[u]
		got := r.e.State(u)
		gr, _ := json.Marshal(got.Rows)
		wr, _ := json.Marshal(normalRows(w.Rows))
		if string(gr) != string(wr) {
			diffs = append(diffs, fmt.Sprintf("%s rows:\n   got  %s\n   want %s", u, gr, wr))
		}
		dials := map[string]string{}
		for k, v := range w.ThreadDials {
			if v == "needs" {
				v = levelNeeds
			}
			dials[k] = v
		}
		if !reflect.DeepEqual(got.ThreadDials, dials) {
			diffs = append(diffs, fmt.Sprintf("%s thread dials: got %v want %v", u, got.ThreadDials, dials))
		}
		if !reflect.DeepEqual(append([]string{}, got.FocusPeople...), append([]string{}, w.FocusPeople...)) {
			diffs = append(diffs, fmt.Sprintf("%s Focus people: got %v want %v", u, got.FocusPeople, w.FocusPeople))
		}
		gd, _ := json.Marshal(append([][]any{}, r.dues[u]...))
		wd, _ := json.Marshal(append([][]any{}, w.Notifications...))
		if string(gd) != string(wd) {
			diffs = append(diffs, fmt.Sprintf("%s notifications: got %s want %s", u, gd, wd))
		}
		for tab, name := range map[string]contract.HistoryTab{"dms": "direct", "mentions": "mentions", "threads": "threads", "reactions": "reactions"} {
			ids := []string{}
			seen := map[string]bool{}
			for _, en := range got.History[name].Entries {
				if !seen[en.Message] {
					seen[en.Message] = true
					ids = append(ids, en.Message)
				}
			}
			sort.Slice(ids, func(i, j int) bool { return msgLess(ids[j], ids[i]) })
			if !reflect.DeepEqual(ids, append([]string{}, w.History[tab]...)) {
				diffs = append(diffs, fmt.Sprintf("%s tab %s: got %v want %v", u, tab, ids, w.History[tab]))
			}
		}
	}
	return diffs
}

func normalRows(rows []StateRow) []StateRow {
	out := make([]StateRow, len(rows))
	for i, x := range rows {
		x.Flagged = append([]string{}, x.Flagged...)
		sort.Strings(x.Flagged)
		out[i] = x
	}
	return out
}

func TestE001CasesReplayed(t *testing.T) {
	raw, err := os.ReadFile("testdata/e001_cases.json")
	if err != nil {
		t.Fatalf("%v. Run: python3 server/internal/feed/rules/testdata/export_e001.py", err)
	}
	var file e001File
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) < 123 {
		t.Fatalf("only %d cases exported", len(file.Cases))
	}
	seen, failed := 0, 0
	for _, w := range file.Worlds {
		r := &replay{t: t, e: New("o"), base: time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC), conv: map[string]string{},
			emoji: map[string]string{}, dues: map[string][][]any{}, made: map[string]bool{}, ver: map[string]int64{}}
		for _, p := range w.People {
			p := p
			r.apply(func(n, pv int64, at string) any {
				return contract.ChangePersonJoined{Kind: "person.joined", Org: "o", Number: n, Previous: pv, At: at, Person: p, Name: p}
			})
		}
		for _, a := range w.Agents {
			a := a
			r.apply(func(n, pv int64, at string) any {
				return contract.ChangePersonJoined{Kind: "person.joined", Org: "o", Number: n, Previous: pv, At: at, Person: a, Name: a, IsAgent: true}
			})
		}
		for _, name := range sortedKeys(w.Channels) {
			name, people := name, w.Channels[name]
			r.apply(func(n, pv int64, at string) any {
				return contract.ChangeChannelCreated{Kind: "channel.created", Org: "o", Number: n, Previous: pv, At: at, Conversation: name, Name: name, People: people}
			})
		}
		for _, op := range w.Ops {
			if op["op"] != "case" {
				r.do(op)
				continue
			}
			seen++
			c := file.Cases[int(op["index"].(float64))]
			var want map[string]e001Person
			b, _ := json.Marshal(op["state"])
			if err := json.Unmarshal(b, &want); err != nil {
				t.Fatal(err)
			}
			if diffs := compareCase(r, want); len(diffs) > 0 {
				if why, ok := e001Differences[c.Name]; ok {
					t.Logf("explained difference in %q: %s", c.Name, why)
					continue
				}
				failed++
				t.Errorf("case %q [%s] differs from the model:\n%s", c.Name, c.Basis, strings.Join(diffs, "\n"))
			}
		}
	}
	if seen != len(file.Cases) {
		t.Errorf("replayed %d cases of %d", seen, len(file.Cases))
	}
	t.Logf("%d cases replayed, %d unexplained differences, %d explained", seen, failed, len(e001Differences))
}
