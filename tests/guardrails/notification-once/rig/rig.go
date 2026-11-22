// Package rig runs a stand-in on PostgreSQL behind the troublemaker, plays the
// delivery worker, and derives the worker's input from the log writer. It is
// shared by the tests of G-006, G-010 and G-011 until T-047 and T-052 exist.
package rig

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"contexia/internal/feed/rules"
	"contexia/internal/shared/call"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/migrate"
	"contexia/internal/shared/orgdb"
	"contexia/internal/standins"
	"contexia/migrations"
	"contexia/tests/harness"
	"contexia/tests/harness/logwriter"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Start is when the log writer's log and the controlled clock begin.
var Start = time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)

// Rig is one stand-in behind a troublemaker.
type Rig struct {
	T      testing.TB
	Dir    orgdb.Single
	Clock  *clock.Manual
	Direct string // the stand-in itself
	Front  string // through the troublemaker
	TM     *harness.Troublemaker

	mu     sync.Mutex
	refuse map[int64]bool
}

// DB connects with a part's own login and brings its schema up to date.
func DB(t testing.TB, login string) orgdb.Single {
	t.Helper()
	host := os.Getenv("CONTEXIA_PG_HOST")
	if host == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	pool, err := pgxpool.New(context.Background(), fmt.Sprintf("postgres://%s:%s_dev@%s:5432/contexia", login, login, host))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.Apply(context.Background(), pool, migrations.Files, login); err != nil {
		t.Fatal(err)
	}
	return orgdb.Single{Pool: pool}
}

// NewOrg returns an organization name no earlier run used.
func NewOrg(prefix string) string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + "-" + hex.EncodeToString(b)
}

// New starts the stand-in of the given part ("search" or "sender").
func New(t testing.TB, part string, seed int64) *Rig {
	t.Helper()
	r := &Rig{T: t, Dir: DB(t, part), Clock: clock.NewManual(Start), refuse: map[int64]bool{}}
	opt := standins.Options{Dir: r.Dir, Clock: r.Clock, Refuse: func(_ string, n int64) string {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.refuse[n] {
			return "refused by the test"
		}
		return ""
	}}
	var h http.Handler
	if part == "search" {
		h = standins.NewSearch(opt).Routes()
	} else {
		h = standins.NewSender(opt).Routes()
	}
	direct := httptest.NewServer(h)
	t.Cleanup(direct.Close)
	r.Direct = direct.URL
	r.TM = harness.NewTroublemaker(direct.URL, seed, harness.Trouble{})
	front := httptest.NewServer(r.TM)
	t.Cleanup(front.Close)
	r.Front = front.URL
	return r
}

// RefuseNumber makes the stand-in refuse one number, or stop refusing it.
func (r *Rig) RefuseNumber(n int64, on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refuse[n] = on
}

// Answer is what a call came back with. Outage means no usable answer.
type Answer struct {
	Status int
	Body   []byte
	Outage bool
}

// Do makes one call through the shared client, with a short limit.
func Do(base, method, path string, in any, limit time.Duration) Answer {
	var out json.RawMessage
	err := call.New(base, limit).JSON(context.Background(), method, path, in, &out)
	var no *call.Refused
	switch {
	case err == nil:
		return Answer{Status: http.StatusOK, Body: out}
	case errors.As(err, &no):
		return Answer{Status: no.Status, Body: []byte(no.Body)}
	}
	return Answer{Outage: true}
}

// Must makes a call that has to succeed, and decodes its answer.
func (r *Rig) Must(method, path string, in, out any) {
	r.T.Helper()
	a := Do(r.Direct, method, path, in, 2*time.Second)
	if a.Outage || a.Status != http.StatusOK {
		r.T.Fatalf("%s %s: status %d, outage %v: %s", method, path, a.Status, a.Outage, a.Body)
	}
	if out != nil {
		if err := json.Unmarshal(a.Body, out); err != nil {
			r.T.Fatalf("%s %s: %v", method, path, err)
		}
	}
}

// Mode switches the stand-in for one organization.
func (r *Rig) Mode(org, mode string) {
	r.T.Helper()
	r.Must("POST", "/orgs/"+org+"/control/mode", map[string]string{"mode": mode}, nil)
}

// Wipe wipes one organization.
func (r *Rig) Wipe(org string) {
	r.T.Helper()
	r.Must("POST", "/orgs/"+org+"/control/wipe", map[string]any{}, nil)
}

// Log returns the log writer's log for one organization.
func Log(org string, seed int64, changes int) []logwriter.Entry {
	return logwriter.Write(logwriter.Config{Orgs: []string{org}, Seed: seed, Changes: changes, Start: Start})[org]
}

type wireMessage struct {
	ID           string   `json:"id"`
	Number       int64    `json:"number"`
	CreatedAt    string   `json:"created_at"`
	Version      int64    `json:"version"`
	Conversation string   `json:"conversation"`
	Thread       *string  `json:"thread"`
	Author       string   `json:"author"`
	Participants []string `json:"participants"`
	Text         string   `json:"text"`
	Mentions     []string `json:"mentions"`
	Level        *string  `json:"level"`
	EditedAt     *string  `json:"edited_at"`
}

type wireEntry struct {
	Kind         string      `json:"kind"`
	Number       int64       `json:"number"`
	At           string      `json:"at"`
	Message      wireMessage `json:"message"`
	Person       string      `json:"person"`
	Conversation string      `json:"conversation"`
	Thread       *string     `json:"thread"`
	UpTo         int64       `json:"up_to"`
}

func parse(e logwriter.Entry) wireEntry {
	var w wireEntry
	// A reaction names its message by ID, which is not this shape, and is not used here.
	if e.Kind == "reaction.added" || e.Kind == "reaction.removed" || e.Kind == "reply.started" || e.Kind == "reply.abandoned" {
		return wireEntry{Kind: e.Kind, Number: e.Number}
	}
	if err := json.Unmarshal(e.JSON, &w); err != nil {
		panic(fmt.Sprintf("log entry %d: %v", e.Number, err))
	}
	return w
}

// Change is one delivery to the search index.
type Change struct {
	Number int64
	ID     string
	Delete bool
	Body   map[string]any
}

// Doc is what the index must hold for one message.
type Doc struct {
	Text         string
	Version      int64
	Participants []string
}

// SearchChanges is what the search worker would send, and the truth after it.
func SearchChanges(org string, log []logwriter.Entry) ([]Change, map[string]Doc) {
	var out []Change
	truth := map[string]Doc{}
	for _, e := range log {
		if e.Kind != "message.created" && e.Kind != "message.changed" && e.Kind != "message.deleted" {
			continue
		}
		w := parse(e)
		m := w.Message
		body := map[string]any{"kind": e.Kind, "org": org, "number": e.Number}
		if e.Kind == "message.deleted" {
			body["message"] = map[string]any{"id": m.ID, "version": m.Version}
			delete(truth, m.ID)
			out = append(out, Change{Number: e.Number, ID: m.ID, Delete: true, Body: body})
			continue
		}
		body["message"] = map[string]any{
			"id": m.ID, "number": m.Number, "created_at": m.CreatedAt, "version": m.Version, "conversation": m.Conversation,
			"thread": m.Thread, "edited": m.EditedAt != nil, "participants": m.Participants, "author": m.Author,
			"text": m.Text, "mentions": m.Mentions, "level": m.Level,
		}
		truth[m.ID] = Doc{Text: m.Text, Version: m.Version, Participants: m.Participants}
		out = append(out, Change{Number: e.Number, ID: m.ID, Body: body})
	}
	return out, truth
}

// Index reads what the index holds for an organization, with the index's login.
func (r *Rig) Index(org string) map[string]Doc {
	r.T.Helper()
	out := map[string]Doc{}
	err := orgdb.InOrg(context.Background(), r.Dir, ids.Org(org), func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), "SELECT id, body, version, participants FROM messages WHERE org = $1 AND NOT deleted", org)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var d Doc
			if err := rows.Scan(&id, &d.Text, &d.Version, &d.Participants); err != nil {
				return err
			}
			out[id] = d
		}
		return rows.Err()
	})
	if err != nil {
		r.T.Fatal(err)
	}
	return out
}

// Differences lists how index differs from truth.
func Differences(index, truth map[string]Doc) []string {
	var out []string
	for id, want := range truth {
		got, ok := index[id]
		switch {
		case !ok:
			out = append(out, id+" is missing")
		case got.Text != want.Text || got.Version != want.Version:
			out = append(out, fmt.Sprintf("%s holds version %d %q, truth is version %d %q", id, got.Version, got.Text, want.Version, want.Text))
		}
	}
	for id := range index {
		if _, ok := truth[id]; !ok {
			out = append(out, id+" should not be in the index")
		}
	}
	return out
}

// Note is one note the feed would write, with what the test needs to judge it.
type Note struct {
	Number int64
	Kind   string
	Name   string
	At     time.Time
	Body   map[string]any
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		panic(err)
	}
	return t
}

func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// Builder writes a numbered list of notes.
type Builder struct {
	Org     string
	Edition int64
	Notes   []Note
}

// Add appends one note; fields is what is specific to its kind.
func (b *Builder) Add(kind, name string, when time.Time, fields map[string]any) {
	n := int64(len(b.Notes)) + 1
	if kind == "clock" { // the protected G-031 tests still ask for a plain note by its old kind (D-034)
		kind, name, fields = "update", fmt.Sprintf("%s / nobody / #none / %d", b.Org, n), map[string]any{"now": "read"}
	}
	body := map[string]any{"kind": kind, "org": b.Org, "number": n, "previous": n - 1, "edition": b.Edition, "at": stamp(when)}
	if name != "" {
		body["name"] = name
	}
	for k, v := range fields {
		body[k] = v
	}
	b.Notes = append(b.Notes, Note{Number: n, Kind: kind, Name: name, At: when, Body: body})
}

// Send appends a "send" note for a person and returns its name. It became due
// at when, and is released by a tick six seconds later.
func (b *Builder) Send(person, row string, cause int64, when time.Time) string {
	name := fmt.Sprintf("%s / %s / %s / %d", b.Org, person, row, cause)
	b.Add("send", name, when.Add(6*time.Second), map[string]any{"row": map[string]any{"conversation": row, "thread": nil}, "person": person,
		"bucket": "needs_you", "why": "mentioned you", "text": "text of " + name, "due_at": stamp(when)})
	return name
}

// Notes runs a log through the feed's real rules, with a tick after every third
// change and one past every hold at the end, and answers the notes it wrote.
func Notes(org string, edition int64, log []logwriter.Entry) []Note {
	e := rules.New(org)
	var drafts []rules.NoteDraft
	apply := func(c contract.Change) {
		fx, err := e.Apply(c, rules.Options{})
		if err != nil {
			panic(fmt.Sprintf("the rules refused a log entry: %v", err))
		}
		drafts = append(drafts, fx.Notes...)
	}
	var last time.Time
	var number int64
	for i, entry := range log {
		var c contract.Change
		if err := json.Unmarshal(entry.JSON, &c); err != nil {
			panic(fmt.Sprintf("log entry %d: %v", entry.Number, err))
		}
		apply(c)
		w := parse(entry)
		number = entry.Number
		if w.At != "" {
			last = at(w.At)
		}
		if i%3 == 0 && !last.IsZero() {
			apply(contract.Change{Value: contract.ChangeTick{Kind: "tick", Org: org, Number: number, Previous: number, At: stamp(last)}})
		}
	}
	apply(contract.Change{Value: contract.ChangeTick{Kind: "tick", Org: org, Number: number, Previous: number, At: stamp(last.Add(6 * time.Second))}})
	return FromDrafts(org, edition, drafts)
}

// DueToSend is the set of names that must go out.
func DueToSend(notes []Note) map[string]bool { return Sent(notes) }

// DeliverNotes plays the worker: in order, retrying an outage, following the
// sender's place when it says where it is.
func DeliverNotes(t testing.TB, base string, notes []Note) {
	t.Helper()
	for i, tries := 0, 0; i < len(notes); {
		a := Do(base, "POST", "/notes", notes[i].Body, 2*time.Second)
		switch {
		case a.Outage:
			if tries++; tries > 400 {
				t.Fatalf("note %d: no answer after %d tries", notes[i].Number, tries)
			}
			time.Sleep(10 * time.Millisecond) // the worker's pause between retries
		case a.Status == http.StatusOK:
			i++
			tries = 0
		case a.Status == http.StatusConflict:
			var behind struct {
				IAmAt *int64 `json:"i_am_at"`
			}
			if err := json.Unmarshal(a.Body, &behind); err != nil || behind.IAmAt == nil {
				t.Fatalf("note %d: %s", notes[i].Number, a.Body)
			}
			i = int(*behind.IAmAt)
		default:
			t.Fatalf("note %d: status %d: %s", notes[i].Number, a.Status, a.Body)
		}
	}
}

// Deliveries counts what the fake provider was handed, per name.
func Deliveries(t testing.TB, d orgdb.Directory, org string) map[string]int {
	t.Helper()
	out := map[string]int{}
	err := orgdb.InOrg(context.Background(), d, ids.Org(org), func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), "SELECT name, count(*) FROM deliveries WHERE org = $1 GROUP BY name", org)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			var n int
			if err := rows.Scan(&name, &n); err != nil {
				return err
			}
			out[name] = n
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// OnceEach lists how the deliveries differ from "each expected name, once".
func OnceEach(got map[string]int, want map[string]bool) []string {
	var out []string
	for name := range want {
		if got[name] != 1 {
			out = append(out, fmt.Sprintf("%s was sent %d times", name, got[name]))
		}
	}
	for name, n := range got {
		if !want[name] {
			out = append(out, fmt.Sprintf("%s was sent %d times and should not have been", name, n))
		}
	}
	return out
}
