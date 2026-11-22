//go:build integration

package conversations

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/migrate"
	"contexia/internal/shared/orgdb"
	"contexia/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

type fixture struct {
	ctx   context.Context
	store *Store
	clock *clock.Manual
	org   ids.Org
	n     int
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("CONTEXIA_PG_HOST")
	if host == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	pool, err := pgxpool.New(context.Background(), fmt.Sprintf("postgres://conversations:conversations_dev@%s:5432/contexia", host))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.Apply(context.Background(), pool, migrations.Files, "conversations"); err != nil {
		t.Fatal(err)
	}
	return pool
}

func newOrg(prefix string) ids.Org {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return ids.Org(prefix + "-" + hex.EncodeToString(b))
}

const seedAt = "2026-11-01T09:00:00.000Z"

// seedTeam gives an organization the people, channels and messages the
// contract examples name.
func seedTeam(t *testing.T, s *Store, org ids.Org) {
	t.Helper()
	ctx := context.Background()
	session, err := s.SeedOpen(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	n := int64(0)
	h := func() (string, int64, int64, string) { n++; return string(org), n, n - 1, seedAt }
	var list []contract.Change
	for _, p := range []string{"anna", "marco", "lena"} {
		o, num, prev, at := h()
		list = append(list, contract.Change{Value: contract.ChangePersonJoined{Kind: "person.joined", Org: o, Number: num, Previous: prev, At: at,
			Person: p, Name: strings.ToUpper(p[:1]) + p[1:] + " Silva", IsAgent: false}})
	}
	o, num, prev, at := h()
	list = append(list, contract.Change{Value: contract.ChangeChannelCreated{Kind: "channel.created", Org: o, Number: num, Previous: prev, At: at,
		Conversation: "#launch", Name: "launch", People: []string{"anna", "marco"}}})
	o, num, prev, at = h()
	list = append(list, contract.Change{Value: contract.ChangeMessageCreated{Kind: "message.created", Org: o, Number: num, Previous: prev, At: at, Message: contract.Message{
		Id: "msg_7Q2", Number: num, Version: 1, Conversation: "#launch", Author: "anna", Text: "@Marco can you sign off?",
		Mentions: []string{"marco"}, Reactions: []contract.Reaction{}, CreatedAt: at}}})
	if _, err := s.SeedBatch(ctx, org, session, contract.SeedBatch{Changes: list}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SeedSeal(ctx, org, session); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) *fixture {
	t.Helper()
	pool := testPool(t)
	c := clock.NewManual(time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC))
	f := &fixture{ctx: context.Background(), clock: c, org: newOrg("t046"), store: &Store{Dir: orgdb.Single{Pool: pool}, Clock: c}}
	seedTeam(t, f.store, f.org)
	return f
}

// act sends an action with a fresh ID and returns the status and answer.
func (f *fixture) act(t *testing.T, kind, as string, fields map[string]any) (int, map[string]any) {
	t.Helper()
	f.n++
	body := map[string]any{"kind": kind, "id": fmt.Sprintf("act_%d", f.n), "as": as}
	for k, v := range fields {
		body[k] = v
	}
	raw, _ := json.Marshal(body)
	status, answer, err := f.store.Act(f.ctx, f.org, raw)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(answer)
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return status, out
}

func (f *fixture) ok(t *testing.T, kind, as string, fields map[string]any) int64 {
	t.Helper()
	status, out := f.act(t, kind, as, fields)
	if status != http.StatusOK {
		t.Fatalf("%s: %d %v", kind, status, out)
	}
	n, _ := out["number"].(float64)
	return int64(n)
}

func (f *fixture) refused(t *testing.T, want, kind, as string, fields map[string]any) {
	t.Helper()
	status, out := f.act(t, kind, as, fields)
	if status != http.StatusConflict || out["refused"] != want {
		t.Errorf("%s: want 409 %q, got %d %v", kind, want, status, out)
	}
}

func (f *fixture) log(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	err := orgdb.InOrg(f.ctx, f.store.Dir, f.org, func(tx pgx.Tx) error {
		rows, err := tx.Query(f.ctx, `SELECT body FROM change_log WHERE org = $1 ORDER BY number`, string(f.org))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var b []byte
			if err := rows.Scan(&b); err != nil {
				return err
			}
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				return err
			}
			out = append(out, m)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func kindsAfter(log []map[string]any, number int64) []string {
	out := []string{}
	for _, c := range log {
		if int64(c["number"].(float64)) > number {
			out = append(out, c["kind"].(string))
		}
	}
	return out
}

func compileChange(t *testing.T) *jsonschema.Schema {
	t.Helper()
	path, err := filepath.Abs("../../../contracts/schema/contexia.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("file:///contexia.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("file:///contexia.json#/$defs/Change")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Every contract example of an action is accepted, its answer has the
// contract's form, and everything written to the log matches the schema.
func TestEveryActionExample(t *testing.T) {
	f := setup(t)
	dirs, err := filepath.Glob("../../../contracts/examples/Action*")
	if err != nil {
		t.Fatal(err)
	}
	tried := 0
	for _, dir := range dirs {
		name := filepath.Base(dir)
		if name == "ActionSaved" {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
		for _, file := range files {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatal(err)
			}
			tried++
			body["id"] = fmt.Sprintf("example_%d", tried)
			switch name {
			case "ActionSend":
				body["message"].(map[string]any)["id"] = "msg_example"
				body["message"].(map[string]any)["thread"] = nil
			case "ActionEdit", "ActionDelete":
				f.ok(t, "send", "marco", map[string]any{"message": map[string]any{"id": "msg_own_" + name, "conversation": "#launch", "thread": nil, "text": "mine", "mentions": []string{}, "level": nil}})
				body["message"] = "msg_own_" + name
				body["as"] = "marco"
			case "ActionCreateChannel":
				body["name"] = "from-example"
			case "ActionRead":
				body["up_to"] = 1
			}
			if t := body["thread"]; t != nil && t != "msg_7Q2" {
				body["thread"] = "msg_7Q2"
			}
			send, _ := json.Marshal(body)
			status, answer, err := f.store.Act(f.ctx, f.org, send)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if status != http.StatusOK {
				t.Errorf("%s: %d %v", name, status, answer)
				continue
			}
			if _, ok := answer.(contract.ActionSaved); !ok {
				t.Errorf("%s: answer is %T", name, answer)
			}
		}
	}
	if tried != 18 {
		t.Errorf("tried %d action examples, the contract has 18 kinds", tried)
	}
	schema := compileChange(t)
	for _, c := range f.log(t) {
		b, _ := json.Marshal(c)
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Errorf("log entry %v does not match the schema: %v", c["kind"], err)
		}
	}
}

func TestRefusals(t *testing.T) {
	f := setup(t)
	msg := func(fields map[string]any) map[string]any {
		m := map[string]any{"id": "msg_x", "conversation": "#launch", "thread": nil, "text": "hello", "mentions": []string{}, "level": nil}
		for k, v := range fields {
			m[k] = v
		}
		return map[string]any{"message": m}
	}
	f.refused(t, "unknown kind", "teleport", "anna", nil)
	f.refused(t, "unknown person", "send", "ghost", msg(nil))
	f.refused(t, "unknown conversation", "send", "anna", msg(map[string]any{"conversation": "#nope"}))
	f.refused(t, "unknown thread", "send", "anna", msg(map[string]any{"thread": "msg_none"}))
	f.refused(t, "empty message", "send", "anna", msg(map[string]any{"text": "  \n"}))
	f.refused(t, "unknown person", "send", "anna", msg(map[string]any{"mentions": []string{"ghost"}}))
	f.refused(t, "unknown level", "send", "anna", msg(map[string]any{"level": "loud"}))
	f.refused(t, "message id taken", "send", "anna", msg(map[string]any{"id": "msg_7Q2"}))

	f.refused(t, "unknown message", "edit", "anna", map[string]any{"message": "msg_none", "text": "x", "mentions": []string{}, "level": nil})
	f.refused(t, "not your message", "edit", "marco", map[string]any{"message": "msg_7Q2", "text": "x", "mentions": []string{}, "level": nil})
	f.refused(t, "not your message", "delete", "marco", map[string]any{"message": "msg_7Q2"})
	f.ok(t, "send", "anna", msg(map[string]any{"id": "msg_gone"}))
	f.ok(t, "delete", "anna", map[string]any{"message": "msg_gone"})
	f.refused(t, "message deleted", "edit", "anna", map[string]any{"message": "msg_gone", "text": "x", "mentions": []string{}, "level": nil})
	f.refused(t, "message deleted", "delete", "anna", map[string]any{"message": "msg_gone"})
	f.refused(t, "message deleted", "react", "anna", map[string]any{"message": "msg_gone", "emoji": "eyes", "on": true})
	f.refused(t, "message deleted", "send", "anna", msg(map[string]any{"id": "msg_late", "thread": "msg_gone"}))
	f.refused(t, "unknown message", "react", "anna", map[string]any{"message": "msg_none", "emoji": "eyes", "on": true})
	f.refused(t, "empty emoji", "react", "anna", map[string]any{"message": "msg_7Q2", "emoji": "", "on": true})

	f.refused(t, "name taken", "create_channel", "anna", map[string]any{"name": "launch", "people": []string{}})
	f.refused(t, "bad channel name", "create_channel", "anna", map[string]any{"name": "two words", "people": []string{}})
	f.refused(t, "unknown person", "create_channel", "anna", map[string]any{"name": "new", "people": []string{"ghost"}})
	f.refused(t, "unknown conversation", "add_people", "anna", map[string]any{"conversation": "#nope", "people": []string{"lena"}})
	f.refused(t, "unknown person", "add_people", "anna", map[string]any{"conversation": "#launch", "people": []string{"ghost"}})
	f.refused(t, "not with yourself", "open_direct", "anna", map[string]any{"with": "anna"})
	f.refused(t, "unknown person", "open_direct", "anna", map[string]any{"with": "ghost"})

	f.refused(t, "unknown conversation", "read", "anna", map[string]any{"conversation": "#nope", "thread": nil, "up_to": 1})
	f.refused(t, "unknown thread", "read", "anna", map[string]any{"conversation": "#launch", "thread": "msg_none", "up_to": 1})
	f.refused(t, "past the end of the log", "read", "anna", map[string]any{"conversation": "#launch", "thread": nil, "up_to": 999999})
	f.refused(t, "unknown conversation", "keep", "anna", map[string]any{"conversation": "#nope", "thread": nil})
	f.refused(t, "unknown thread", "done", "anna", map[string]any{"conversation": "#launch", "thread": "msg_none"})
	f.refused(t, "unknown dial level", "dial", "anna", map[string]any{"conversation": "#launch", "thread": nil, "level": "loud"})
	f.refused(t, "not a channel level", "dial", "anna", map[string]any{"conversation": "#launch", "thread": nil, "level": "fyi"})
	f.refused(t, "unknown rule", "focus_rule", "anna", map[string]any{"rule": map[string]any{"type": "item_kind", "value": "nope"}, "on": true})
	f.refused(t, "unknown rule", "focus_rule", "anna", map[string]any{"rule": map[string]any{"type": "nope", "value": "x"}, "on": true})
	f.refused(t, "unknown person", "focus_person", "anna", map[string]any{"about": "ghost", "on": true})
	f.refused(t, "empty key", "preference", "anna", map[string]any{"key": "", "value": "x"})

	direct := f.direct(t, "anna", "marco")
	f.refused(t, "not a channel", "add_people", "anna", map[string]any{"conversation": direct, "people": []string{"lena"}})
	f.refused(t, "not a participant", "send", "lena", msg(map[string]any{"conversation": direct}))
	f.refused(t, "no threads in a direct conversation", "send", "anna", msg(map[string]any{"conversation": direct, "thread": "msg_7Q2"}))
}

func (f *fixture) direct(t *testing.T, a, b string) string {
	t.Helper()
	status, out := f.act(t, "open_direct", a, map[string]any{"with": b})
	if status != 200 {
		t.Fatalf("open direct: %d %v", status, out)
	}
	return out["conversation"].(string)
}

func TestSameIDGetsTheFirstAnswer(t *testing.T) {
	f := setup(t)
	body := []byte(`{"kind":"send","id":"same","as":"anna","message":{"id":"msg_s","conversation":"#launch","thread":null,"text":"once","mentions":[],"level":null}}`)
	_, first, err := f.store.Act(f.ctx, f.org, body)
	if err != nil {
		t.Fatal(err)
	}
	before := len(f.log(t))
	reordered := []byte(`{"as":"anna","id":"same","kind":"send","message":{"level":null,"mentions":[],"text":"once","thread":null,"conversation":"#launch","id":"msg_s"}}`)
	status, again, err := f.store.Act(f.ctx, f.org, reordered)
	if err != nil || status != 200 {
		t.Fatalf("%d %v", status, err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Errorf("answers differ: %s, %s", a, b)
	}
	if after := len(f.log(t)); after != before {
		t.Errorf("the resend added %d log entries", after-before)
	}
	status, answer, err := f.store.Act(f.ctx, f.org, []byte(`{"kind":"send","id":"same","as":"anna","message":{"id":"msg_s","conversation":"#launch","thread":null,"text":"other","mentions":[],"level":null}}`))
	if err != nil || status != http.StatusConflict || answer.(contract.Refusal).Refused != "id reused" {
		t.Errorf("a different body with the same ID: %d %v %v", status, answer, err)
	}
	// The same ID from another caller is another action.
	status, _, err = f.store.Act(f.ctx, f.org, []byte(`{"kind":"focus_only","id":"same","as":"marco","on":true}`))
	if err != nil || status != 200 {
		t.Errorf("another caller's ID: %d %v", status, err)
	}
	// A refusal is an answer too.
	refusal := []byte(`{"kind":"open_direct","id":"r1","as":"anna","with":"ghost"}`)
	s1, _, _ := f.store.Act(f.ctx, f.org, refusal)
	s2, a2, _ := f.store.Act(f.ctx, f.org, refusal)
	if s1 != http.StatusConflict || s2 != http.StatusConflict {
		t.Errorf("repeated refusal: %d, %d %v", s1, s2, a2)
	}
}

func TestActingImpliesRead(t *testing.T) {
	f := setup(t)
	root := "msg_7Q2"
	reply := func(id, as string) int64 {
		return f.ok(t, "send", as, map[string]any{"message": map[string]any{"id": id, "conversation": "#launch", "thread": root, "text": "r", "mentions": []string{}, "level": nil}})
	}
	r1 := reply("msg_r1", "anna")
	last := f.log(t)
	if c := last[len(last)-1]; c["kind"] != "reader.read" || c["person"] != "anna" || c["thread"] != root || int64(c["up_to"].(float64)) != r1 {
		t.Errorf("a reply must read its row in the same step: %v", c)
	}

	before := int64(len(f.log(t)))
	n := f.ok(t, "keep", "marco", map[string]any{"conversation": "#launch", "thread": root})
	if n != before+1 {
		t.Errorf("the answer must carry the fact's number: %d, want %d", n, before+1)
	}
	if got := kindsAfter(f.log(t), before); strings.Join(got, ",") != "reader.keep,reader.read" {
		t.Errorf("keep wrote %v", got)
	}
	c := f.log(t)[before+1]
	if c["person"] != "marco" || int64(c["up_to"].(float64)) != r1 {
		t.Errorf("keep must read the whole row up to its newest message: %v", c)
	}

	r2 := reply("msg_r2", "anna")
	before = int64(len(f.log(t)))
	f.ok(t, "react", "marco", map[string]any{"message": "msg_r1", "emoji": "eyes", "on": true})
	if got := kindsAfter(f.log(t), before); strings.Join(got, ",") != "reaction.added,reader.read" {
		t.Errorf("react wrote %v", got)
	}
	if c := f.log(t)[before+1]; int64(c["up_to"].(float64)) != r2 || c["thread"] != root {
		t.Errorf("a reaction must read its row up to the newest: %v", c)
	}

	// The same reaction again is already so: nothing is written.
	before = int64(len(f.log(t)))
	status, out := f.act(t, "react", "marco", map[string]any{"message": "msg_r1", "emoji": "eyes", "on": true})
	if status != 200 || out["number"] != nil || int64(len(f.log(t))) != before {
		t.Errorf("a repeated reaction: %d %v", status, out)
	}

	// done reads too, and a position never moves back.
	r3 := reply("msg_r3", "anna")
	before = int64(len(f.log(t)))
	f.ok(t, "done", "marco", map[string]any{"conversation": "#launch", "thread": root})
	if got := kindsAfter(f.log(t), before); strings.Join(got, ",") != "reader.done,reader.read" {
		t.Errorf("done wrote %v", got)
	}
	before = int64(len(f.log(t)))
	status, out = f.act(t, "read", "marco", map[string]any{"conversation": "#launch", "thread": root, "up_to": r3 - 1})
	if status != 200 || out["number"] != nil || int64(len(f.log(t))) != before {
		t.Errorf("a read backwards must change nothing: %d %v", status, out)
	}
}

func TestWatchedArrivalIsReadInTheSameStep(t *testing.T) {
	f := setup(t)
	// The live connection (T-053) writes the watching row with its log entry.
	err := f.store.Save(f.ctx, f.org, func(st *Step) error {
		if _, err := st.Append(contract.ChangeReaderWatchingStarted{Kind: "reader.watching_started", Person: "marco", Conversation: "#launch", Connection: "conn_1"}); err != nil {
			return err
		}
		_, err := st.Tx.Exec(f.ctx, `INSERT INTO watching (org, connection, person, conversation, thread) VALUES ($1, 'conn_1', 'marco', '#launch', NULL)`, string(f.org))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	before := int64(len(f.log(t)))
	n := f.ok(t, "send", "anna", map[string]any{"message": map[string]any{"id": "msg_w", "conversation": "#launch", "thread": nil, "text": "hi", "mentions": []string{}, "level": nil}})
	log := f.log(t)
	if got := kindsAfter(log, before); strings.Join(got, ",") != "message.created,reader.read" {
		t.Fatalf("arrival wrote %v", got)
	}
	if c := log[n]; c["person"] != "marco" || int64(c["up_to"].(float64)) != n || c["thread"] != nil {
		t.Errorf("the watcher's read: %v", c)
	}
	// A reply in a thread nobody watches is not read for the channel's watcher.
	before = int64(len(log))
	f.ok(t, "send", "lena", map[string]any{"message": map[string]any{"id": "msg_w2", "conversation": "#launch", "thread": "msg_w", "text": "r", "mentions": []string{}, "level": nil}})
	for _, c := range f.log(t)[before:] {
		if c["kind"] == "reader.read" && c["person"] == "marco" {
			t.Errorf("marco is not watching that thread: %v", c)
		}
	}
}

func TestQuestions(t *testing.T) {
	f := setup(t)
	f.clock.Advance(time.Minute)
	reply := f.ok(t, "send", "anna", map[string]any{"message": map[string]any{"id": "msg_r", "conversation": "#launch", "thread": "msg_7Q2", "text": "reply", "mentions": []string{}, "level": nil}})
	top := f.ok(t, "send", "marco", map[string]any{"message": map[string]any{"id": "msg_t", "conversation": "#launch", "thread": nil, "text": "top", "mentions": []string{}, "level": "info"}})

	start, err := f.store.Start(f.ctx, f.org, "marco")
	if err != nil {
		t.Fatal(err)
	}
	if start.Person.Id != "marco" || len(start.People) != 3 || len(start.Conversations) != 1 {
		t.Fatalf("start: %+v", start)
	}
	if !start.Conversations[0].Unread || start.Conversations[0].Dial != "default" {
		t.Errorf("marco has anna's unread message: %+v", start.Conversations[0])
	}
	if start.Number < top {
		t.Errorf("start number %d is before marco's own message %d", start.Number, top)
	}
	if _, err := f.store.Start(f.ctx, f.org, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown person: %v", err)
	}
	// A reply does not mark the channel; only top-level messages do (D-032).
	f.ok(t, "read", "marco", map[string]any{"conversation": "#launch", "thread": nil, "up_to": top})
	start, _ = f.store.Start(f.ctx, f.org, "marco")
	if start.Conversations[0].Unread {
		t.Errorf("after reading the channel nothing marks it: %+v", start.Conversations[0])
	}

	page, err := f.store.Messages(f.ctx, f.org, MessagesQuery{Conversation: "#launch", As: "marco"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 2 || page.Messages[0].Message.Id != "msg_7Q2" {
		t.Fatalf("channel view: %+v", page.Messages)
	}
	s := page.Messages[0].ThreadSummary
	if s == nil || s.Replies != 1 || !s.Unread || s.People[0] != "anna" {
		t.Errorf("thread summary: %+v", s)
	}
	if page.ReadUpTo == nil || *page.ReadUpTo != top {
		t.Errorf("read_up_to: %v", page.ReadUpTo)
	}
	f.ok(t, "read", "marco", map[string]any{"conversation": "#launch", "thread": "msg_7Q2", "up_to": reply})
	page, _ = f.store.Messages(f.ctx, f.org, MessagesQuery{Conversation: "#launch", As: "marco"})
	if page.Messages[0].ThreadSummary.Unread {
		t.Errorf("the thread was read: %+v", page.Messages[0].ThreadSummary)
	}

	root := "msg_7Q2"
	thread, err := f.store.Messages(f.ctx, f.org, MessagesQuery{Conversation: "#launch", Thread: &root})
	if err != nil || len(thread.Messages) != 2 || thread.Messages[1].Message.Id != "msg_r" {
		t.Fatalf("thread view: %v %+v", err, thread.Messages)
	}
	around := top
	win, err := f.store.Messages(f.ctx, f.org, MessagesQuery{Conversation: "#launch", Around: &around, Limit: 1})
	if err != nil || len(win.Messages) != 1 || win.Messages[0].Message.Id != "msg_t" || !win.HasBefore || win.HasAfter {
		t.Errorf("around: %v %+v", err, win)
	}

	// A deleted message stays only while a reply depends on it (D-004).
	f.ok(t, "delete", "anna", map[string]any{"message": "msg_7Q2"})
	page, _ = f.store.Messages(f.ctx, f.org, MessagesQuery{Conversation: "#launch"})
	if m := page.Messages[0].Message; m.Id != "msg_7Q2" || !m.Deleted || m.Text != "" || len(m.Mentions) != 0 {
		t.Errorf("a deleted root with a reply keeps its identity and nothing it said: %+v", m)
	}
	f.ok(t, "delete", "anna", map[string]any{"message": "msg_r"})
	page, _ = f.store.Messages(f.ctx, f.org, MessagesQuery{Conversation: "#launch"})
	if len(page.Messages) != 1 || page.Messages[0].Message.Id != "msg_t" {
		t.Errorf("with its last reply gone the root leaves no trace: %+v", page.Messages)
	}

	sug, err := f.store.Suggest(f.ctx, f.org, "sil", "marco")
	if err != nil || len(sug.People) != 2 || sug.Everyone {
		t.Errorf("suggest by a later word, without the asker: %v %+v", err, sug)
	}
	sug, _ = f.store.Suggest(f.ctx, f.org, "cha", "marco")
	if !sug.Everyone {
		t.Errorf("suggest channel: %+v", sug)
	}
	people, err := f.store.People(f.ctx, f.org)
	if err != nil || len(people.People) != 3 {
		t.Errorf("people: %v %+v", err, people)
	}
}

// The start number is the last change of the stream that concerns this person,
// so the first live update's "after" can match it.
func TestStartNumberIsThePersonsOwn(t *testing.T) {
	f := setup(t)
	mine := f.ok(t, "send", "anna", map[string]any{"message": map[string]any{"id": "msg_n", "conversation": "#launch", "thread": nil, "text": "x", "mentions": []string{}, "level": nil}})
	f.ok(t, "keep", "marco", map[string]any{"conversation": "#launch", "thread": "msg_7Q2"}) // marco's, on the feed stream
	f.ok(t, "read", "anna", map[string]any{"conversation": "#launch", "thread": nil, "up_to": 1})
	f.ok(t, "create_channel", "anna", map[string]any{"name": "elsewhere", "people": []string{"anna"}})
	start, err := f.store.Start(f.ctx, f.org, "lena") // lena is in no channel: only people joining concern her
	if err != nil {
		t.Fatal(err)
	}
	if start.Number != 3 {
		t.Errorf("lena's number is %d, want 3, the last person joining", start.Number)
	}
	start, err = f.store.Start(f.ctx, f.org, "marco")
	if err != nil {
		t.Fatal(err)
	}
	// marco's keep wrote a read of his own after anna's message.
	if start.Number <= mine {
		t.Errorf("marco's number is %d, want past %d: his own read concerns him", start.Number, mine)
	}
	f.ok(t, "profile_line", "anna", map[string]any{"text": "Launch lead"})
	after, _ := f.store.Start(f.ctx, f.org, "lena")
	if after.Number <= 3 {
		t.Errorf("a profile line concerns everyone: lena's number stayed %d", after.Number)
	}
}

func TestOpenDirectTwiceIsOneConversation(t *testing.T) {
	f := setup(t)
	before := len(f.log(t))
	a := f.direct(t, "marco", "anna")
	b := f.direct(t, "anna", "marco")
	if a != "dm:anna:marco" || a != b {
		t.Errorf("%s, %s", a, b)
	}
	if got := len(f.log(t)) - before; got != 1 {
		t.Errorf("two opens wrote %d changes", got)
	}
}

func TestSeedSession(t *testing.T) {
	pool := testPool(t)
	c := clock.NewManual(time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC))
	s := &Store{Dir: orgdb.Single{Pool: pool}, Clock: c}
	org := newOrg("seed")
	ctx := context.Background()
	session, err := s.SeedOpen(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.SeedOpen(ctx, org)
	if err != nil || again != session {
		t.Fatalf("opening an empty organization again: %q %v", again, err)
	}
	change := func(n int64, at string) contract.Change {
		return contract.Change{Value: contract.ChangePersonJoined{Kind: "person.joined", Org: string(org), Number: n, Previous: n - 1, At: at,
			Person: fmt.Sprintf("p%d", n), Name: "P", IsAgent: false}}
	}
	refusedBatch := func(name string, list ...contract.Change) {
		t.Helper()
		_, err := s.SeedBatch(ctx, org, session, contract.SeedBatch{Changes: list})
		var r SeedRefused
		if !errors.As(err, &r) {
			t.Errorf("%s: want a refusal, got %v", name, err)
		}
	}
	refusedBatch("a gap", change(2, seedAt))
	if through, err := s.SeedBatch(ctx, org, session, contract.SeedBatch{Changes: []contract.Change{change(1, seedAt), change(2, "2026-11-01T10:00:00.000Z")}}); err != nil || through != 2 {
		t.Fatalf("batch: %d %v", through, err)
	}
	refusedBatch("time going back", change(3, seedAt))
	if through, err := s.SeedBatch(ctx, org, session, contract.SeedBatch{Changes: []contract.Change{change(3, "2026-11-01T10:00:00.000Z")}}); err != nil || through != 3 {
		t.Fatalf("second batch: %d %v", through, err)
	}
	status, answer, err := s.Act(ctx, org, []byte(`{"kind":"focus_only","id":"early","as":"p1","on":true}`))
	if err != nil || status != http.StatusConflict || answer.(contract.Refusal).Refused != "seeding" {
		t.Errorf("an action while seeding: %d %v %v", status, answer, err)
	}
	if through, err := s.SeedSeal(ctx, org, session); err != nil || through != 3 {
		t.Fatalf("seal: %d %v", through, err)
	}
	var r SeedRefused
	if _, err := s.SeedSeal(ctx, org, session); !errors.As(err, &r) || r.Reason != "not empty" {
		t.Errorf("sealing twice: %v", err)
	}
	if _, err := s.SeedBatch(ctx, org, session, contract.SeedBatch{Changes: []contract.Change{change(4, seedAt)}}); !errors.As(err, &r) || r.Reason != "not empty" {
		t.Errorf("writing to a sealed session: %v", err)
	}
	if _, err := s.SeedOpen(ctx, org); !errors.As(err, &r) || r.Reason != "not empty" {
		t.Errorf("opening on an organization with changes: %v", err)
	}
	// The log continues from the seed's last number and time.
	status, answer, err = s.Act(ctx, org, []byte(`{"kind":"focus_only","id":"after","as":"p1","on":true}`))
	if err != nil || status != 200 || *answer.(contract.ActionSaved).Number != 4 {
		t.Errorf("first action after the seed: %d %v %v", status, answer, err)
	}
}

// Timestamps never decrease within a log, even when the clock steps back.
func TestTimeNeverGoesBack(t *testing.T) {
	f := setup(t)
	f.ok(t, "focus_only", "anna", map[string]any{"on": true})
	f.clock.Advance(-time.Hour)
	f.ok(t, "focus_only", "anna", map[string]any{"on": false})
	log := f.log(t)
	if log[len(log)-1]["at"].(string) < log[len(log)-2]["at"].(string) {
		t.Errorf("time went back: %v then %v", log[len(log)-2]["at"], log[len(log)-1]["at"])
	}
}

// The first half of G-025: row rules on every table that holds an
// organization's content. Built with -tags fault_no_row_rules the rules are
// switched off for the planted query, and this test must fail.
func TestOrganizationsAreKeptApartByRowRules(t *testing.T) {
	pool := testPool(t)
	c := clock.NewManual(time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC))
	s := &Store{Dir: orgdb.Single{Pool: pool}, Clock: c}
	ctx := context.Background()
	a, b := newOrg("apart-a"), newOrg("apart-b")
	for _, org := range []ids.Org{a, b} {
		seedTeam(t, s, org)
		for _, body := range []string{
			`{"kind":"send","id":"x1","as":"marco","message":{"id":"msg_same","conversation":"#launch","thread":"msg_7Q2","text":"same id","mentions":[],"level":null}}`,
			`{"kind":"react","id":"x2","as":"marco","message":"msg_7Q2","emoji":"eyes","on":true}`,
			`{"kind":"keep","id":"x3","as":"marco","conversation":"#launch","thread":"msg_7Q2"}`,
			`{"kind":"dial","id":"x4","as":"marco","conversation":"#launch","thread":null,"level":"muted"}`,
			`{"kind":"focus_rule","id":"x5","as":"marco","rule":{"type":"person","value":"anna"},"on":true}`,
			`{"kind":"focus_person","id":"x6","as":"marco","about":"anna","on":true}`,
			`{"kind":"focus_only","id":"x7","as":"marco","on":true}`,
			`{"kind":"preference","id":"x8","as":"marco","key":"send_key","value":"enter"}`,
		} {
			if status, answer, err := s.Act(ctx, org, []byte(body)); err != nil || status != 200 {
				t.Fatalf("%s: %d %v %v", body, status, answer, err)
			}
		}
		err := s.Save(ctx, org, func(st *Step) error {
			if _, err := st.Append(contract.ChangeReaderWatchingStarted{Kind: "reader.watching_started", Person: "marco", Conversation: "#launch", Connection: "c1"}); err != nil {
				return err
			}
			_, err := st.Tx.Exec(ctx, `INSERT INTO watching (org, connection, person, conversation, thread) VALUES ($1, 'c1', 'marco', '#launch', NULL)`, string(org))
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	tables := []string{"people", "conversations", "conversation_people", "messages", "reactions", "reader_reads", "reader_kept",
		"reader_dials", "reader_focus_rules", "reader_focus_people", "reader_settings", "reader_preferences", "watching",
		"action_answers", "seed_sessions", "change_log"}
	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			// The planted query leaves out the organization.
			planted := `SELECT count(*) FILTER (WHERE org = $1), count(*) FILTER (WHERE org = $2) FROM ` + table
			errRolledBack := errors.New("roll back")
			err := orgdb.InOrg(ctx, s.Dir, a, func(tx pgx.Tx) error {
				if rowRulesOff {
					if _, err := tx.Exec(ctx, `ALTER TABLE `+table+` DISABLE ROW LEVEL SECURITY`); err != nil {
						return err
					}
				}
				var mine, other int64
				if err := tx.QueryRow(ctx, planted, string(a), string(b)).Scan(&mine, &other); err != nil {
					return err
				}
				if mine == 0 {
					t.Errorf("%s holds nothing for the organization, so this proves nothing", table)
				}
				if other != 0 {
					t.Errorf("G-025 broken: a query bound to one organization saw %d rows of another in %s", other, table)
				}
				return errRolledBack
			})
			if !errors.Is(err, errRolledBack) {
				t.Fatal(err)
			}
			// With the rules off the same query does see the other organization,
			// so it is the rules that keep them apart.
			err = orgdb.InOrg(ctx, s.Dir, a, func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, `ALTER TABLE `+table+` DISABLE ROW LEVEL SECURITY`); err != nil {
					return err
				}
				var mine, other int64
				if err := tx.QueryRow(ctx, planted, string(a), string(b)).Scan(&mine, &other); err != nil {
					return err
				}
				if other == 0 {
					t.Errorf("the planted query cannot see the other organization even with the rules off: it proves nothing")
				}
				return errRolledBack
			})
			if !errors.Is(err, errRolledBack) {
				t.Fatal(err)
			}
		})
	}
	// Writing another organization's row is refused.
	err := orgdb.InOrg(ctx, s.Dir, a, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO people (org, id, name, is_agent) VALUES ($1, 'intruder', 'I', false)`, string(b))
		return err
	})
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "42501" {
		t.Errorf("writing into another organization must be refused by the row rule, got %v", err)
	}
	// The same message ID means each organization's own message.
	ma, err := s.Messages(ctx, a, MessagesQuery{Conversation: "#launch", Thread: ptr("msg_7Q2")})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range ma.Messages {
		if v.Message.Id == "msg_same" && v.Message.Text != "same id" {
			t.Errorf("wrong message: %+v", v.Message)
		}
	}
}

func ptr(s string) *string { return &s }
