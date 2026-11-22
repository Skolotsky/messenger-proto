// Package retiredelete holds G-031: a retired organization accepts no action
// and is delivered nothing more; deleting an organization, also when interrupted
// and repeated, removes every row of it from every part and leaves every other
// organization unchanged (D-033).
package retiredelete

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"contexia/internal/feed/orgdelete"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/migrate"
	"contexia/internal/shared/orgdb"
	"contexia/internal/standins"
	sqlfiles "contexia/migrations"
	"contexia/tests/guardrails/receivers-independent/rig"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var bg = context.Background()

// system is every part's real routes on a database of its own. No worker runs
// here unless a test starts one, so only the calls under test move anything.
type system struct {
	admin                                   *pgxpool.Pool
	store                                   *conversations.Store
	conv, deliveryOps, search, sender, feed string
}

func dbURL(login, password, db string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:5432/%s", login, password, os.Getenv("CONTEXIA_PG_HOST"), db)
}

func pool(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(bg, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func serve(t *testing.T, h http.Handler) string {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s.URL
}

func newSystem(t *testing.T) *system {
	t.Helper()
	if os.Getenv("CONTEXIA_PG_HOST") == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	name := fmt.Sprintf("t064_%d_%d", time.Now().UnixNano(), rand.Intn(1000))
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
	admin := pool(t, dbURL("contexia_admin", "admin_dev", name))
	for _, q := range []string{
		"REVOKE ALL ON SCHEMA public FROM PUBLIC",
		"CREATE SCHEMA conversations AUTHORIZATION conversations",
		"CREATE SCHEMA feed AUTHORIZATION feed",
		"CREATE SCHEMA search AUTHORIZATION search",
		"CREATE SCHEMA sender AUTHORIZATION sender",
		"GRANT USAGE ON SCHEMA conversations TO delivery",
		"GRANT USAGE ON SCHEMA feed TO delivery",
	} {
		if _, err := admin.Exec(bg, q); err != nil {
			t.Fatal(err)
		}
	}
	login := func(part string) orgdb.Single {
		p := pool(t, dbURL(part, part+"_dev", name))
		if part != "delivery" {
			if err := migrate.Apply(bg, p, sqlfiles.Files, part); err != nil {
				t.Fatal(err)
			}
		}
		return orgdb.Single{Pool: p}
	}
	convDB, feedDB, searchDB, senderDB := login("conversations"), login("feed"), login("search"), login("sender")
	deliveryDB := login("delivery")

	s := &system{admin: admin, store: &conversations.Store{Dir: convDB, Clock: clock.Real{}}}
	mux := http.NewServeMux()
	s.store.Routes(mux)
	s.conv = serve(t, mux)
	s.deliveryOps = serve(t, delivery.New(delivery.Config{DB: delivery.OneDatabase{Single: deliveryDB}, Clock: clock.Real{}, Seed: 47}).Routes())
	s.search = serve(t, standins.NewSearch(standins.Options{Dir: searchDB, Clock: clock.Real{}}).Routes())
	s.sender = serve(t, standins.NewSender(standins.Options{Dir: senderDB, Clock: clock.Real{}}).Routes())
	s.feed = serve(t, orgdelete.Routes(feedDB))
	return s
}

// tables finds, from the database catalogue, every table holding an org column
// in any schema, so a table added later is covered without touching this test.
func (s *system) tables(t testing.TB) []string {
	t.Helper()
	rows, err := s.admin.Query(bg, `SELECT c.table_schema || '.' || c.table_name
		FROM information_schema.columns c
		JOIN information_schema.tables b ON b.table_schema = c.table_schema AND b.table_name = c.table_name
		WHERE c.column_name = 'org' AND b.table_type = 'BASE TABLE'
		  AND c.table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 25 {
		t.Fatalf("the catalogue shows only %d tables with an organization: %v", len(names), names)
	}
	return names
}

// fingerprint is every row of org, per table, as the administrator sees them
// (past the row rules). The organization's own name is taken out, so two
// organizations with the same content have the same fingerprint.
func (s *system) fingerprint(t testing.TB, org string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range s.tables(t) {
		var n int64
		var sum string
		err := s.admin.QueryRow(bg, `SELECT count(*), coalesce(md5(string_agg(replace(x::text, $1, '<org>'), '|' ORDER BY x::text)), '')
			FROM `+table+` x WHERE x.org = $1`, org).Scan(&n, &sum)
		if err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		if n > 0 {
			out[table] = fmt.Sprintf("%d:%s", n, sum)
		}
	}
	return out
}

// fill gives org a row in every table, with the same names and numbers for
// every organization. Written as the administrator, with foreign keys and
// triggers off, because the point is what the deletes do to the rows.
func (s *system) fill(t testing.TB, org string) {
	t.Helper()
	c, err := s.admin.Acquire(bg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Release()
	if _, err := c.Exec(bg, "SET session_replication_role = replica"); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = c.Exec(bg, "SET session_replication_role = DEFAULT") }()
	const at = "'2026-11-22T10:00:00Z'"
	for _, q := range []string{
		`INSERT INTO conversations.org_log (org, head, head_at) VALUES ($1, 3, ` + at + `)`,
		`INSERT INTO conversations.change_log (org, number, at, kind, body) SELECT $1, n, ` + at + `, 'tick', jsonb_build_object('kind', 'tick', 'number', n) FROM generate_series(1, 3) n`,
		`INSERT INTO conversations.delivery_due (receiver, org, since) VALUES ('search', $1, ` + at + `), ('feed', $1, ` + at + `)`,
		`INSERT INTO conversations.delivery_places (org, receiver, place) VALUES ($1, 'search', 1), ($1, 'feed', 2)`,
		`INSERT INTO conversations.delivery_parked (org, receiver, number, reason, tries, parked_at) VALUES ($1, 'search', 2, 'refused', 3, ` + at + `)`,
		`INSERT INTO conversations.delivery_stopped (org) VALUES ($1)`,
		`INSERT INTO conversations.people (org, id, name, is_agent) VALUES ($1, 'anna', 'Anna', false), ($1, 'marco', 'Marco', false)`,
		`INSERT INTO conversations.conversations (org, id, type, name) VALUES ($1, '#launch', 'channel', 'launch')`,
		`INSERT INTO conversations.conversation_people (org, conversation, person) VALUES ($1, '#launch', 'anna'), ($1, '#launch', 'marco')`,
		`INSERT INTO conversations.messages (org, id, number, version, conversation, author, author_is_agent, text, mentions, created_at)
			VALUES ($1, 'msg_1', 1, 1, '#launch', 'anna', false, 'the same words', '{marco}', ` + at + `)`,
		`INSERT INTO conversations.reactions (org, message, emoji, person, number) VALUES ($1, 'msg_1', 'eyes', 'marco', 2)`,
		`INSERT INTO conversations.reader_reads (org, person, conversation, thread, up_to) VALUES ($1, 'marco', '#launch', NULL, 1)`,
		`INSERT INTO conversations.reader_kept (org, person, conversation, thread) VALUES ($1, 'marco', '#launch', 'msg_1')`,
		`INSERT INTO conversations.reader_dials (org, person, conversation, thread, level) VALUES ($1, 'marco', '#launch', NULL, 'fyi')`,
		`INSERT INTO conversations.reader_focus_rules (org, person, type, value, is_on) VALUES ($1, 'marco', 'item_kind', 'flags', true)`,
		`INSERT INTO conversations.reader_focus_people (org, person, about, is_on) VALUES ($1, 'marco', 'anna', true)`,
		`INSERT INTO conversations.reader_settings (org, person, auto_add) VALUES ($1, 'marco', true)`,
		`INSERT INTO conversations.reader_preferences (org, person, key, value) VALUES ($1, 'marco', 'send_key', 'enter')`,
		`INSERT INTO conversations.watching (org, connection, person, conversation) VALUES ($1, 'conn_1', 'marco', '#launch')`,
		`INSERT INTO conversations.action_answers (org, caller, id, body_hash, status, answer) VALUES ($1, 'marco', 'a1', 'h', 200, '{}')`,
		`INSERT INTO conversations.seed_sessions (org, session, sealed) VALUES ($1, 's1', true)`,
		`INSERT INTO feed.note_heads (org, head) VALUES ($1, 2)`,
		`INSERT INTO feed.note_log (org, edition, number, at, kind, body) SELECT $1, 1, n, ` + at + `, 'clock', jsonb_build_object('kind', 'clock', 'number', n) FROM generate_series(1, 2) n`,
		`INSERT INTO feed.note_due (org, since) VALUES ($1, ` + at + `)`,
		`INSERT INTO feed.note_places (org, place) VALUES ($1, 1)`,
		`INSERT INTO feed.note_parked (org, edition, number, reason, tries, parked_at) VALUES ($1, 1, 2, 'refused', 3, ` + at + `)`,
		`INSERT INTO search.state (org, through) VALUES ($1, 3)`,
		`INSERT INTO search.messages (org, id, number, version, body, folded) VALUES ($1, 'msg_1', 1, 1, 'the same words', 'the same words')`,
		`INSERT INTO sender.state (org, place) VALUES ($1, 2)`,
		`INSERT INTO sender.notifications (org, name, person, state, due_at, arrived_at, why, body)
			VALUES ($1, 'n1', 'marco', 'sent', ` + at + `, ` + at + `, 'Anna mentioned you', 'the same words')`,
		`INSERT INTO sender.deliveries (org, name, at) VALUES ($1, 'n1', ` + at + `)`,
	} {
		if _, err := c.Exec(bg, q, org); err != nil {
			t.Fatalf("fill %s: %v\n%s", org, err, q)
		}
	}
}

// ask makes one call and returns its status and body.
func ask(t testing.TB, method, url string, body any) (int, []byte) {
	t.Helper()
	var in io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		in = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, in)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

// steps is the delete in the contract's order: retire, stop delivery, the
// receivers and the feed, the log and originals last.
func (s *system) steps(org string) []struct{ name, method, url string } {
	return []struct{ name, method, url string }{
		{"retire", "POST", s.conv + "/orgs/" + org + "/retire"},
		{"stop delivery", "POST", s.deliveryOps + "/orgs/" + org + "/stop"},
		{"delete search", "DELETE", s.search + "/orgs/" + org},
		{"delete sender", "DELETE", s.sender + "/orgs/" + org},
		{"delete feed", "DELETE", s.feed + "/orgs/" + org},
		{"delete delivery", "DELETE", s.deliveryOps + "/orgs/" + org},
		{"delete conversations", "DELETE", s.conv + "/orgs/" + org},
	}
}

func same(a, b map[string]string) []string {
	var out []string
	for k, v := range a {
		if b[k] != v {
			out = append(out, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// deleteProblems interrupts a delete after each step, repeats it, and returns
// everything that went wrong, so the must-fail build can read the reason.
func deleteProblems(t *testing.T, s *system) []string {
	t.Helper()
	var problems []string
	count := len(s.steps("x"))
	for crash := 0; crash <= count; crash++ {
		gone, kept := fmt.Sprintf("gone-%d", crash), fmt.Sprintf("kept-%d", crash)
		s.fill(t, gone)
		s.fill(t, kept)
		before := s.fingerprint(t, kept)
		victim := s.fingerprint(t, gone)
		for _, table := range s.tables(t) {
			if _, ok := victim[table]; !ok {
				problems = append(problems, fmt.Sprintf("the fixture has no row in %s. Add one to fill in this test, and to that part's delete", table))
			}
		}
		if d := same(before, victim); len(d) > 0 {
			problems = append(problems, fmt.Sprintf("the two organizations are not the same before the delete: %v", d))
		}
		run := func(list []struct{ name, method, url string }, when string) {
			for _, st := range list {
				if status, body := ask(t, st.method, st.url, nil); status != http.StatusOK {
					problems = append(problems, fmt.Sprintf("%s: %s answered %d: %s", when, st.name, status, body))
				}
			}
		}
		all := s.steps(gone)
		run(all[:min(crash, count)], fmt.Sprintf("before the crash after step %d", crash))
		if d := same(before, s.fingerprint(t, kept)); len(d) > 0 {
			problems = append(problems, fmt.Sprintf("crash after step %d: the other organization changed in %v", crash, d))
		}
		run(all, fmt.Sprintf("repeat after the crash after step %d", crash))
		run(all, fmt.Sprintf("second repeat after the crash after step %d", crash))
		if left := s.fingerprint(t, gone); len(left) > 0 {
			var names []string
			for k := range left {
				names = append(names, k)
			}
			sort.Strings(names)
			problems = append(problems, fmt.Sprintf("crash after step %d: rows of the deleted organization are left in %v", crash, names))
		}
		if d := same(before, s.fingerprint(t, kept)); len(d) > 0 {
			problems = append(problems, fmt.Sprintf("crash after step %d: after the delete the other organization changed in %v", crash, d))
		}
	}
	return problems
}

func TestDeleteRemovesOneOrganizationAndNoOther(t *testing.T) {
	s := newSystem(t)
	for _, p := range deleteProblems(t, s) {
		t.Error(p)
	}
}

// A table that holds an organization and that no delete knows about must fail.
func TestANewTableWithoutADeleteFails(t *testing.T) {
	s := newSystem(t)
	if _, err := s.admin.Exec(bg, `CREATE TABLE sender.new_table (org text NOT NULL, note text)`); err != nil {
		t.Fatal(err)
	}
	problems := deleteProblems(t, s)
	found := false
	for _, p := range problems {
		found = found || strings.Contains(p, "sender.new_table")
	}
	if !found {
		t.Fatalf("a table with an organization and no delete went unnoticed: %v", problems)
	}
}

func TestDeletingAnOrganizationThatIsNotRetiredIsRefused(t *testing.T) {
	s := newSystem(t)
	s.fill(t, "live")
	before := s.fingerprint(t, "live")
	status, body := ask(t, "DELETE", s.conv+"/orgs/live", nil)
	var r contract.Refusal
	_ = json.Unmarshal(body, &r)
	if status != http.StatusConflict || r.Refused == "" {
		t.Errorf("status %d: %s", status, body)
	}
	// G-004's exception is for retired organizations only: on a live one, removing
	// a fact without a log entry is still refused by the database itself.
	err := orgdb.InOrg(bg, s.store.Dir, "live", func(tx pgx.Tx) error {
		_, err := tx.Exec(bg, `DELETE FROM reader_preferences WHERE org = 'live'`)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "G-004") {
		t.Errorf("removing a fact of a live organization without a log entry: %v", err)
	}
	if d := same(before, s.fingerprint(t, "live")); len(d) > 0 {
		t.Errorf("a refused delete changed %v", d)
	}
}

func seeded(t *testing.T, s *system, org string) {
	t.Helper()
	session, err := s.store.SeedOpen(bg, ids.Org(org))
	if err != nil {
		t.Fatal(err)
	}
	at := "2026-11-01T09:00:00.000Z"
	changes := []contract.Change{
		{Value: contract.ChangePersonJoined{Kind: "person.joined", Org: org, Number: 1, Previous: 0, At: at, Person: "anna", Name: "Anna"}},
		{Value: contract.ChangePersonJoined{Kind: "person.joined", Org: org, Number: 2, Previous: 1, At: at, Person: "marco", Name: "Marco"}},
		{Value: contract.ChangeChannelCreated{Kind: "channel.created", Org: org, Number: 3, Previous: 2, At: at, Conversation: "#launch", Name: "launch", People: []string{"anna", "marco"}}},
	}
	if _, err := s.store.SeedBatch(bg, ids.Org(org), session, contract.SeedBatch{Changes: changes}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.SeedSeal(bg, ids.Org(org), session); err != nil {
		t.Fatal(err)
	}
}

func action(id string) map[string]any {
	return map[string]any{"kind": "send", "id": id, "as": "anna", "message": map[string]any{
		"id": "msg_" + id, "conversation": "#launch", "thread": nil, "text": "hello", "mentions": []string{}, "level": nil}}
}

func headOf(t *testing.T, s *system, org string) int64 {
	t.Helper()
	var n int64
	if err := s.admin.QueryRow(bg, `SELECT head FROM conversations.org_log WHERE org = $1`, org).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestARetiredOrganizationAcceptsNothing(t *testing.T) {
	s := newSystem(t)
	seeded(t, s, "old")
	seeded(t, s, "next")
	send := func(org, id string) (int, []byte) {
		return ask(t, "POST", s.conv+"/orgs/"+org+"/people/actions", action(id))
	}
	if status, body := send("old", "a1"); status != http.StatusOK {
		t.Fatalf("before retiring: %d %s", status, body)
	}
	for range 2 { // repeating it changes nothing
		if status, body := ask(t, "POST", s.conv+"/orgs/old/retire", map[string]any{}); status != http.StatusOK {
			t.Fatalf("retire: %d %s", status, body)
		}
	}
	head := headOf(t, s, "old")
	door := []struct{ name, method, path string }{
		{"start", "GET", "/people/marco/start"},
		{"messages", "GET", "/conversations/%23launch/messages?as=marco"},
		{"suggest", "GET", "/suggest?q=a&as=marco"},
	}
	gone := func(what string, status int, body []byte) {
		t.Helper()
		var g contract.OrganizationGone
		err := json.Unmarshal(body, &g)
		if status != http.StatusGone || err != nil || !g.OrganizationGone {
			t.Errorf("%s: want 410 organization_gone, got %d: %s", what, status, body)
		}
	}
	for _, d := range door {
		status, body := ask(t, d.method, s.conv+"/orgs/old"+d.path, nil)
		gone(d.name, status, body)
		if status, body := ask(t, d.method, s.conv+"/orgs/next"+d.path, nil); status != http.StatusOK {
			t.Errorf("%s on the other organization: %d %s", d.name, status, body)
		}
	}
	status, body := send("old", "a2")
	gone("a new action", status, body)
	status, body = send("old", "a1")
	gone("an action sent again", status, body)
	status, body = ask(t, "POST", s.conv+"/orgs/old/people/actions", "not an action")
	gone("an unreadable action", status, body)
	if got := headOf(t, s, "old"); got != head {
		t.Errorf("a retired organization's log moved from %d to %d", head, got)
	}
	if status, body := send("next", "a1"); status != http.StatusOK {
		t.Errorf("the other organization: %d %s", status, body)
	}
	if status, _ := ask(t, "POST", s.conv+"/orgs/old/seed/open", map[string]any{}); status != http.StatusConflict {
		t.Errorf("a seed session on a retired organization: %d", status)
	}
}

// A stopped organization is delivered nothing more, also what was on its way;
// the other carries on. After the stop's answer, no receiver gets another call.
func TestAStoppedOrganizationIsDeliveredNothingMore(t *testing.T) {
	b := rig.NewBench(t, nil)
	b.DB.AddOrg(t, "stopped", 120)
	b.DB.AddOrg(t, "running", 120)
	b.Feed.PerItem.Store(int64(2 * time.Millisecond))
	b.Search.PerItem.Store(int64(2 * time.Millisecond))
	for range 60 {
		for _, org := range []string{"stopped", "running"} {
			if _, err := b.DB.Save(bg, org); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := b.DB.SaveNote(bg, "stopped", "clock"); err != nil {
			t.Fatal(err)
		}
	}
	b.Start(t, "copy-1")
	rig.Until(t, 10*time.Second, "delivery has begun", func() bool { return b.Feed.Place("stopped") > 0 })
	if status, body := ask(t, "POST", b.Ops+"/orgs/stopped/stop", map[string]any{}); status != http.StatusOK {
		t.Fatalf("stop: %d %s", status, body)
	}
	search, feed, sender := b.Search.Count("stopped"), b.Feed.Count("stopped"), b.Sender.Count("stopped")
	for range 20 { // what arrives afterwards stays undelivered too
		if _, err := b.DB.Save(bg, "stopped"); err != nil {
			t.Fatal(err)
		}
		if _, err := b.DB.SaveNote(bg, "stopped", "clock"); err != nil {
			t.Fatal(err)
		}
		if _, err := b.DB.Save(bg, "running"); err != nil {
			t.Fatal(err)
		}
	}
	rig.Until(t, 20*time.Second, "the other organization is served to its end", func() bool {
		return b.Search.Place("running") >= 80 && b.Feed.Place("running") >= 80
	})
	rig.Until(t, 5*time.Second, "the stopped organization leaves the work list", func() bool {
		var n int
		if err := b.DB.Del.QueryRow(bg, `SELECT (SELECT count(*) FROM delivery_due WHERE org = 'stopped') + (SELECT count(*) FROM feed.note_due WHERE org = 'stopped')`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 0
	})
	if s, f, n := b.Search.Count("stopped"), b.Feed.Count("stopped"), b.Sender.Count("stopped"); s != search || f != feed || n != sender {
		t.Errorf("delivered after the stop: search %d to %d, feed %d to %d, sender %d to %d", search, s, feed, f, sender, n)
	}
}
