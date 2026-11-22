// Package rebuild holds G-020: each receiver can be rebuilt for one organization
// and then equals what it held, and Reset gives the same starting state under a
// new organization, touching no other. The parts are the real ones except the
// feed, which is a stand-in answering its contract until T-052 lands.
package rebuild

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"contexia/internal/conversations"
	"contexia/internal/delivery"
	"contexia/internal/demo"
	"contexia/internal/democontent"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/migrate"
	"contexia/internal/shared/orgdb"
	"contexia/internal/standins"
	sqlfiles "contexia/migrations"
	"contexia/tests/guardrails/rebuild/feedstandin"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var bg = context.Background()

const rebuildLimit = 2 * time.Minute // G-020

type system struct {
	admin                     *pgxpool.Pool
	content                   *democontent.Content
	feed                      *feedstandin.Feed
	demo                      *demo.Service
	conv, ops, search, sender string
	feedURL                   string
}

func dbURL(login, password, db string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:5432/%s", login, password, os.Getenv("CONTEXIA_PG_HOST"), db)
}

func serve(t *testing.T, h http.Handler) string {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s.URL
}

func newSystem(t *testing.T, limit time.Duration) *system {
	t.Helper()
	if os.Getenv("CONTEXIA_PG_HOST") == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	name := fmt.Sprintf("t059_%d_%d", time.Now().UnixNano(), rand.Intn(1000))
	root, err := pgx.Connect(bg, dbURL("contexia_admin", "admin_dev", "contexia"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close(bg)
	if _, err := root.Exec(bg, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(bg, dbURL("contexia_admin", "admin_dev", "contexia"))
		if err != nil {
			return
		}
		defer c.Close(bg)
		_, _ = c.Exec(bg, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	open := func(login, password string) *pgxpool.Pool {
		p, err := pgxpool.New(bg, dbURL(login, password, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(p.Close)
		return p
	}
	admin := open("contexia_admin", "admin_dev")
	for _, q := range []string{
		"REVOKE ALL ON SCHEMA public FROM PUBLIC",
		"CREATE SCHEMA conversations AUTHORIZATION conversations",
		"CREATE SCHEMA feed AUTHORIZATION feed",
		"CREATE SCHEMA search AUTHORIZATION search",
		"CREATE SCHEMA sender AUTHORIZATION sender",
		"CREATE SCHEMA demo AUTHORIZATION demo",
		"GRANT USAGE ON SCHEMA conversations TO delivery",
		"GRANT USAGE ON SCHEMA feed TO delivery",
	} {
		if _, err := admin.Exec(bg, q); err != nil {
			t.Fatal(err)
		}
	}
	login := func(part string) orgdb.Single {
		p := open(part, part+"_dev")
		if part != "delivery" {
			if err := migrate.Apply(bg, p, sqlfiles.Files, part); err != nil {
				t.Fatal(err)
			}
		}
		return orgdb.Single{Pool: p}
	}
	convDB, _, searchDB, senderDB, demoDB := login("conversations"), login("feed"), login("search"), login("sender"), login("demo")
	deliveryDB := login("delivery")

	content, err := democontent.Load(os.DirFS("../../../demo/content"))
	if err != nil {
		t.Fatal(err)
	}
	s := &system{admin: admin, content: content, feed: feedstandin.New()}
	store := &conversations.Store{Dir: convDB, Clock: clock.Real{}}
	mux := http.NewServeMux()
	store.Routes(mux)
	s.conv = serve(t, mux)
	s.search = serve(t, standins.NewSearch(standins.Options{Dir: searchDB, Clock: clock.Real{}}).Routes())
	s.sender = serve(t, standins.NewSender(standins.Options{Dir: senderDB, Clock: clock.Real{}}).Routes())
	s.feedURL = serve(t, s.feed.Routes())
	d := delivery.New(delivery.Config{DB: delivery.OneDatabase{Single: deliveryDB}, Clock: clock.Real{},
		Search: s.search, Feed: s.feedURL, Sender: s.sender, Conversations: s.conv, Seed: 59})
	s.ops = serve(t, d.Routes())
	ctx, cancel := context.WithCancel(bg)
	done := make(chan struct{})
	go func() { defer close(done); d.Run(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	s.demo = demo.New(demo.Config{Pool: demoDB.Pool, Clock: clock.Real{}, Content: content,
		Conversations: s.conv, Delivery: s.ops, Search: s.search, Sender: s.sender, Feed: s.feedURL, LevelLimit: limit})
	demoCtx, demoCancel := context.WithCancel(bg)
	demoDone := make(chan struct{})
	go func() { defer close(demoDone); s.demo.Run(demoCtx) }()
	t.Cleanup(func() { demoCancel(); <-demoDone })
	return s
}

func (s *system) post(t *testing.T, url string, in, out any) int {
	t.Helper()
	var body bytes.Buffer
	if in != nil {
		_ = json.NewEncoder(&body).Encode(in)
	}
	resp, err := http.Post(url, "application/json", &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

// bystander is a second, active organization seeded from a prefix of the history.
func (s *system) bystander(t *testing.T, org string) {
	t.Helper()
	content := *s.content
	content.Org.Org = org
	h, err := content.Build(democontent.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var opened contract.SeedOpened
	if c := s.post(t, s.conv+"/orgs/"+org+"/seed/open", map[string]string{"name": "Bystander"}, &opened); c != 200 {
		t.Fatalf("seed open: %d", c)
	}
	changes := h.Changes[:400]
	for i := 0; i < len(changes); i += 200 {
		if c := s.post(t, s.conv+"/orgs/"+org+"/seed/"+opened.Session, contract.SeedBatch{Changes: changes[i:min(i+200, len(changes))]}, nil); c != 200 {
			t.Fatalf("seed batch: %d", c)
		}
	}
	if c := s.post(t, s.conv+"/orgs/"+org+"/seed/"+opened.Session+"/seal", nil, nil); c != 200 {
		t.Fatalf("seal: %d", c)
	}
	s.waitLevel(t, org)
}

func (s *system) status(t *testing.T, org string) contract.DeliveryStatus {
	t.Helper()
	resp, err := http.Get(s.ops + "/orgs/" + org + "/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out contract.DeliveryStatus
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (s *system) waitLevel(t *testing.T, org string) {
	t.Helper()
	deadline := time.Now().Add(rebuildLimit)
	for time.Now().Before(deadline) {
		level := true
		for _, r := range s.status(t, org).Receivers {
			if r.Waiting != 0 {
				level = false
			}
		}
		if level {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s did not become level: %+v", org, s.status(t, org))
}

// act sends one message through the people door.
func (s *system) act(t *testing.T, org, id string) {
	t.Helper()
	p := s.content.Org.People[0].ID
	channel := "#" + s.content.Org.Channels[0].Name
	body := contract.ActionSend{Kind: "send", Id: id, As: p, Message: contract.NewMessage{
		Id: "msg_" + id, Conversation: channel, Text: "Still working " + id, Mentions: []string{}}}
	if c := s.post(t, s.conv+"/orgs/"+org+"/people/actions", body, nil); c != 200 {
		t.Fatalf("act on %s: %d", org, c)
	}
}

// note writes the next notification note, the feed's side of the sender's boundary.
func (s *system) note(t *testing.T, org string, number int64) {
	t.Helper()
	at := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	name := fmt.Sprintf("%s / marco / #launch:msg_1 / %d", org, number)
	body, _ := json.Marshal(map[string]any{"org": org, "number": number, "previous": number - 1, "edition": 1, "at": at, "kind": "send",
		"name": name, "person": "marco", "bucket": "needs_you", "why": "test", "text": "text", "due_at": at,
		"row": map[string]any{"conversation": "#launch", "thread": "msg_1"}})
	err := pgx.BeginFunc(bg, s.admin, func(tx pgx.Tx) error {
		if _, err := tx.Exec(bg, `INSERT INTO feed.note_heads (org, head) VALUES ($1, $2) ON CONFLICT (org) DO UPDATE SET head = $2`, org, number); err != nil {
			return err
		}
		if _, err := tx.Exec(bg, `INSERT INTO feed.note_log (org, edition, number, at, kind, body) VALUES ($1, 1, $2, now(), 'send', $3)`, org, number, string(body)); err != nil {
			return err
		}
		_, err := tx.Exec(bg, `INSERT INTO feed.note_due (org) VALUES ($1) ON CONFLICT DO NOTHING`, org)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// rows is one receiver's content for one organization, keyed, in comparable form.
func (s *system) rows(t *testing.T, receiver, org string) map[string]string {
	t.Helper()
	out := map[string]string{}
	switch receiver {
	case "feed":
		for n, k := range s.feed.Applied(org) {
			out[fmt.Sprint(n)] = k
		}
		return out
	case "search":
		rows, err := s.admin.Query(bg, `SELECT id, concat_ws('|', version, conversation, thread, author, edited, created_at, body, deleted) FROM search.messages WHERE org = $1`, org)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err != nil {
				t.Fatal(err)
			}
			out[k] = v
		}
		return out
	case "sender":
		rows, err := s.admin.Query(bg, `SELECT n.name, concat_ws('|', n.person, n.state, n.delayed, n.why, n.body, n.now,
			(SELECT count(*) FROM sender.deliveries d WHERE d.org = n.org AND d.name = n.name))
			FROM sender.notifications n WHERE n.org = $1`, org)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err != nil {
				t.Fatal(err)
			}
			out[k] = v
		}
		return out
	}
	t.Fatalf("no receiver %q", receiver)
	return nil
}

// contains says what of the reference is missing or changed in now.
func contains(reference, now map[string]string) error {
	var bad []string
	for k, v := range reference {
		if got, ok := now[k]; !ok {
			bad = append(bad, "missing "+k)
		} else if got != v {
			bad = append(bad, fmt.Sprintf("changed %s: %q was %q", k, got, v))
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return fmt.Errorf("%d differ, first: %s", len(bad), bad[0])
	}
	return nil
}

// startingState is what a person finds: the whole log and the search index, with
// the organization's own ID taken out.
func (s *system) startingState(t *testing.T, org string) map[string]string {
	t.Helper()
	out := map[string]string{}
	rows, err := s.admin.Query(bg, `SELECT number, kind, at, body::text FROM conversations.change_log WHERE org = $1 AND kind <> 'clock.tick' ORDER BY number`, org)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var n int64
		var kind, body string
		var at time.Time
		if err := rows.Scan(&n, &kind, &at, &body); err != nil {
			t.Fatal(err)
		}
		out[fmt.Sprintf("log/%d", n)] = kind + "|" + at.UTC().Format(time.RFC3339Nano) + "|" + strings.ReplaceAll(body, org, "<org>")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for k, v := range s.rows(t, "search", org) {
		out["search/"+k] = strings.ReplaceAll(v, org, "<org>")
	}
	return out
}

func sameStates(a, b map[string]string) error {
	if len(a) != len(b) {
		return fmt.Errorf("sizes differ: %d and %d", len(a), len(b))
	}
	return contains(a, b)
}

// checkTwoResets is the rule: two Resets give equal starting states under
// different organizations, and the first is retired.
func checkTwoResets(t *testing.T, s *system) error {
	first, err := s.demo.Reset(bg)
	if err != nil {
		return err
	}
	s.waitLevel(t, first.Org)
	before := s.startingState(t, first.Org)
	second, err := s.demo.Reset(bg)
	if err != nil {
		return err
	}
	if second.Org == first.Org || second.Name != first.Name {
		return fmt.Errorf("Reset gave %+v after %+v", second, first)
	}
	s.waitLevel(t, second.Org)
	if err := sameStates(before, s.startingState(t, second.Org)); err != nil {
		return fmt.Errorf("the two starting states differ: %w", err)
	}
	if c := s.post(t, s.conv+"/orgs/"+first.Org+"/people/actions", contract.ActionRead{Kind: "read", Id: "late", As: s.content.Org.People[0].ID, Conversation: "#" + s.content.Org.Channels[0].Name, UpTo: 1}, nil); c != http.StatusGone {
		return fmt.Errorf("the previous organization answers %d, want 410", c)
	}
	return nil
}

func TestTwoResetsGiveEqualStartingStates(t *testing.T) {
	if err := checkTwoResets(t, newSystem(t, 0)); err != nil {
		t.Fatal(err)
	}
}

// checkLevelBeforeCurrent is the rule that Reset makes nothing current until search
// and the feed are level.
func checkLevelBeforeCurrent(t *testing.T, s *system) error {
	first, err := s.demo.Reset(bg)
	if err != nil {
		return err
	}
	s.feed.SetOff(true)
	if _, err := s.demo.Reset(bg); err == nil {
		return fmt.Errorf("Reset answered while the feed could not be level")
	}
	if cur := s.currentOrg(t); cur != first.Org {
		return fmt.Errorf("current moved to %s before the feed was level", cur)
	}
	s.feed.SetOff(false)
	second, err := s.demo.Reset(bg)
	if err != nil {
		return err
	}
	if second.Org == first.Org {
		return fmt.Errorf("Reset stayed on %s", first.Org)
	}
	return nil
}

func (s *system) currentOrg(t *testing.T) string {
	t.Helper()
	var org string
	if err := s.admin.QueryRow(bg, `SELECT id FROM demo.organizations WHERE state = 'current'`).Scan(&org); err != nil {
		t.Fatal(err)
	}
	return org
}

func TestResetWaitsUntilLevel(t *testing.T) {
	if err := checkLevelBeforeCurrent(t, newSystem(t, 8*time.Second)); err != nil {
		t.Fatal(err)
	}
}

// checkRebuild rebuilds each receiver of one organization and compares with the
// reference taken before the wipe. Active, both organizations keep taking changes
// meanwhile. Idle, nothing else moves: delivery repairs a receiver that fell behind
// when the next change reaches it, so only an idle run shows a rebuild that did not
// refill by itself, or a wipe that reached another organization.
func checkRebuild(t *testing.T, s *system, active bool) error {
	cur, err := s.demo.Reset(bg)
	if err != nil {
		return err
	}
	const other = "bystander"
	s.bystander(t, other)
	s.note(t, cur.Org, 1)
	s.note(t, cur.Org, 2)
	s.note(t, other, 1)
	s.waitLevel(t, cur.Org)
	s.waitLevel(t, other)

	for i, receiver := range []string{"search", "sender", "feed"} {
		reference, untouched := s.rows(t, receiver, cur.Org), s.rows(t, receiver, other)
		if len(reference) == 0 || len(untouched) == 0 {
			return fmt.Errorf("%s holds nothing to compare (%d, %d)", receiver, len(reference), len(untouched))
		}
		began := time.Now()
		done := make(chan error, 1)
		go func() { done <- s.demo.Rebuild(bg, contractOrg(cur.Org), receiver, false) }()
		if active {
			s.act(t, cur.Org, fmt.Sprintf("during-%d", i))
			s.act(t, other, fmt.Sprintf("other-%d", i))
		}
		if err := <-done; err != nil {
			return fmt.Errorf("rebuild %s: %w", receiver, err)
		}
		s.waitLevel(t, cur.Org)
		s.waitLevel(t, other)
		if took := time.Since(began); took > rebuildLimit {
			return fmt.Errorf("rebuilding %s took %s", receiver, took)
		}
		if err := contains(reference, s.rows(t, receiver, cur.Org)); err != nil {
			return fmt.Errorf("%s after its rebuild: %w", receiver, err)
		}
		if err := contains(untouched, s.rows(t, receiver, other)); err != nil {
			return fmt.Errorf("the other organization's %s: %w", receiver, err)
		}
	}
	return nil
}

func TestRebuildEqualsWhatItHeldAndTouchesNoOther(t *testing.T) {
	if err := checkRebuild(t, newSystem(t, 0), true); err != nil {
		t.Fatal(err)
	}
}

func TestRebuildRefillsAnIdleOrganizationByItself(t *testing.T) {
	if err := checkRebuild(t, newSystem(t, 0), false); err != nil {
		t.Fatal(err)
	}
}

// TestCleanupLeavesTheCurrentAndRemovesRetired goes through the program's Delete.
func TestCleanupLeavesTheCurrentAndRemovesRetired(t *testing.T) {
	s := newSystem(t, 0)
	first, err := s.demo.Reset(bg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.demo.Reset(bg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.demo.Delete(bg, contractOrg(second.Org)); err == nil || !strings.Contains(err.Error(), "current") {
		t.Fatalf("deleting the current organization answered %v", err)
	}
	deleted, err := s.demo.DeleteRetired(bg)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 || deleted[0] != first.Org {
		t.Fatalf("deleted %v, want %s", deleted, first.Org)
	}
	var left int
	if err := s.admin.QueryRow(bg, `SELECT count(*) FROM conversations.change_log WHERE org = $1`, first.Org).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("%d log entries of the deleted organization remain", left)
	}
	if err := s.demo.Delete(bg, contractOrg(first.Org)); err != nil {
		t.Fatalf("repeating a delete: %v", err)
	}
	var kept int
	if err := s.admin.QueryRow(bg, `SELECT count(*) FROM conversations.change_log WHERE org = $1`, second.Org).Scan(&kept); err != nil || kept == 0 {
		t.Fatalf("the current organization lost its log: %d, %v", kept, err)
	}
}
