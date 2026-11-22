//go:build integration

package conversations

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

func (f *fixture) ticks(t *testing.T) []time.Time {
	t.Helper()
	var out []time.Time
	err := orgdb.InOrg(f.ctx, f.store.Dir, f.org, func(tx pgx.Tx) error {
		rows, err := tx.Query(f.ctx, `SELECT at FROM change_log WHERE org = $1 AND kind = 'tick' ORDER BY number`, string(f.org))
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowTo[time.Time])
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func (f *fixture) say(t *testing.T, text string) int64 {
	t.Helper()
	f.n++
	return f.ok(t, "send", "anna", map[string]any{"message": map[string]any{
		"id": "msg_h" + strings.ReplaceAll(time.Duration(f.n).String(), "ns", ""), "conversation": "#launch", "thread": nil,
		"text": text, "mentions": []string{}, "level": nil}})
}

func (f *fixture) due(t *testing.T) *time.Time {
	t.Helper()
	var due *time.Time
	if err := orgdb.InOrg(f.ctx, f.store.Dir, f.org, func(tx pgx.Tx) error {
		return tx.QueryRow(f.ctx, `SELECT tick_due_at FROM org_log WHERE org = $1`, string(f.org)).Scan(&due)
	}); err != nil {
		t.Fatal(err)
	}
	return due
}

// tickIfDue writes this organization's tick if it is due; other tests'
// organizations in the same database are not touched.
func (f *fixture) tickIfDue(t *testing.T) {
	t.Helper()
	if _, err := f.store.writeTick(f.ctx, f.org, f.clock.Now()); err != nil {
		t.Fatal(err)
	}
}

// TestABusyOrganizationStillGetsTicks is D-034 reading 1: with a change every
// 300 ms the log still gets about one tick a second. Timed from the newest entry
// it would get none.
func TestABusyOrganizationStillGetsTicks(t *testing.T) {
	f := setup(t)
	f.say(t, "start")
	for range 40 { // 12 s of steady traffic
		f.clock.Advance(300 * time.Millisecond)
		f.say(t, "busy")
		f.tickIfDue(t)
	}
	got := f.ticks(t)
	if len(got) < 10 || len(got) > 13 {
		t.Fatalf("12 s of traffic got %d ticks, want about one a second (D-034)", len(got))
	}
	for i := 1; i < len(got); i++ {
		if gap := got[i].Sub(got[i-1]); gap < 900*time.Millisecond || gap > 1500*time.Millisecond {
			t.Fatalf("ticks %d and %d are %s apart, want about 1 s", i-1, i, gap)
		}
	}
}

// TestTicksStopSixSecondsAfterTheLastChange: quiet organizations cost nothing.
func TestTicksStopSixSecondsAfterTheLastChange(t *testing.T) {
	f := setup(t)
	f.say(t, "the last change")
	last := f.clock.Now()
	for range 20 {
		f.clock.Advance(500 * time.Millisecond)
		f.tickIfDue(t)
	}
	got := f.ticks(t)
	if len(got) == 0 {
		t.Fatal("no ticks after a change")
	}
	final := got[len(got)-1]
	if final.Before(last.Add(6 * time.Second)) {
		t.Fatalf("the last tick is stamped %s after the change, want at least 6 s", final.Sub(last))
	}
	if final.After(last.Add(7500 * time.Millisecond)) {
		t.Fatalf("ticks ran on until %s after the change", final.Sub(last))
	}
	if d := f.due(t); d != nil {
		t.Fatalf("a tick is still due at %s after the window closed", d)
	}
}

// TestLiveFilterMatchesTheLoad holds the filter for new entries equal, entry by
// entry and for each person, to the rule the first load takes its number from.
// A difference would leave a screen with a number it is never sent an update
// after, or one that skips an update.
func TestLiveFilterMatchesTheLoad(t *testing.T) {
	f := setup(t)
	root := "msg_filter_root"
	f.ok(t, "send", "anna", map[string]any{"message": map[string]any{"id": root, "conversation": "#launch", "thread": nil,
		"text": "root", "mentions": []string{"marco"}, "level": nil}})
	f.ok(t, "send", "marco", map[string]any{"message": map[string]any{"id": "msg_filter_reply", "conversation": "#launch", "thread": root,
		"text": "reply", "mentions": []string{}, "level": nil}})
	f.ok(t, "edit", "anna", map[string]any{"message": root, "text": "root, edited", "mentions": []string{}, "level": nil})
	f.ok(t, "react", "marco", map[string]any{"message": root, "emoji": "ok", "on": true})
	f.ok(t, "react", "marco", map[string]any{"message": root, "emoji": "ok", "on": false})
	f.ok(t, "delete", "marco", map[string]any{"message": "msg_filter_reply"})
	f.ok(t, "create_channel", "lena", map[string]any{"name": "lenas", "people": []string{"anna"}})
	f.ok(t, "add_people", "anna", map[string]any{"conversation": "#launch", "people": []string{"lena"}})
	f.ok(t, "open_direct", "marco", map[string]any{"with": "lena"})
	f.ok(t, "keep", "marco", map[string]any{"conversation": "#launch", "thread": nil})
	f.ok(t, "done", "marco", map[string]any{"conversation": "#launch", "thread": nil})
	f.ok(t, "dial", "marco", map[string]any{"conversation": "#launch", "thread": nil, "level": "muted"})
	f.ok(t, "preference", "marco", map[string]any{"key": "send_key", "value": "enter"})
	f.ok(t, "profile_line", "lena", map[string]any{"text": "away"})
	f.ok(t, "focus_only", "anna", map[string]any{"on": true})
	thread := root
	if err := f.store.Save(f.ctx, f.org, func(st *Step) error {
		for _, change := range []any{
			contract.ChangeReplyStarted{Kind: "reply.started", Message: "msg_r", Conversation: "#launch", Thread: &thread, Author: "anna"},
			contract.ChangeReplyAbandoned{Kind: "reply.abandoned", Message: "msg_r", Conversation: "#launch", Thread: &thread},
			contract.ChangeReaderWatchingStarted{Kind: "reader.watching_started", Person: "marco", Conversation: "#launch", Connection: "c"},
			contract.ChangeReaderWatchingEnded{Kind: "reader.watching_ended", Person: "marco", Conversation: "#launch", Connection: "c"},
			contract.ChangeTick{Kind: "tick"},
			contract.ChangeSeedOpened{Kind: "seed.opened"},
			contract.ChangeSeedSealed{Kind: "seed.sealed"},
		} {
			if _, err := st.Append(change); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	err := orgdb.InOrg(f.ctx, f.store.Dir, f.org, func(tx pgx.Tx) error {
		for _, person := range []string{"anna", "marco", "lena"} {
			mine, err := myConversations(f.ctx, tx, f.org, person)
			if err != nil {
				return err
			}
			set := map[string]bool{}
			for _, m := range mine {
				set[m] = true
			}
			rows, err := tx.Query(f.ctx, `SELECT number, kind, body, `+concernsPerson+` FROM change_log WHERE org = $1 ORDER BY number`,
				string(f.org), person, mine)
			if err != nil {
				return err
			}
			type row struct {
				number int64
				kind   string
				body   []byte
				sql    *bool
			}
			var list []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.number, &r.kind, &r.body, &r.sql); err != nil {
					rows.Close()
					return err
				}
				list = append(list, r)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			for _, r := range list {
				kinds[r.kind] = true
				e, err := parseEntry(r.number, r.kind, r.body)
				if err != nil {
					return err
				}
				want := r.sql != nil && *r.sql
				if got := concerns(e, person, set); got != want {
					t.Errorf("%s (number %d) for %s: the live filter says %v, the first load's rule says %v", r.kind, r.number, person, got, want)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"message.created", "message.changed", "message.deleted", "reaction.added", "reaction.removed", "person.joined",
		"channel.created", "channel.people_added", "direct.created", "reader.read", "reader.keep", "reader.done", "reader.dial", "reader.preference",
		"reader.profile_line", "reader.focus_only", "reply.started", "reply.abandoned", "reader.watching_started", "reader.watching_ended", "tick", "seed.opened", "seed.sealed"} {
		if !kinds[k] {
			t.Errorf("the log of this test has no %s entry, so the filter was not compared for it", k)
		}
	}
}

// TestTickPassFindsDueOrganization goes through the real question.
func TestTickPassFindsDueOrganization(t *testing.T) {
	f := setup(t)
	l := NewLive(f.store, LiveOptions{})
	f.say(t, "a change")
	f.clock.Advance(1100 * time.Millisecond)
	if _, err := l.TickPass(f.ctx); err != nil {
		t.Fatal(err)
	}
	if got := len(f.ticks(t)); got != 1 {
		t.Fatalf("got %d ticks, want 1", got)
	}
}

type doorParts struct {
	feedDown, searchDown atomic.Bool
	feed, search         *httptest.Server
}

func newDoorParts(t *testing.T) *doorParts {
	d := &doorParts{}
	answer := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	d.feed = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if d.feedDown.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/rows"):
			answer(w, contract.FeedRowsAnswer{Edition: 7, Through: 3, Number: 9, Rows: []contract.FeedRow{}, Counts: contract.FeedCounts{NeedsYou: 2}})
		case strings.HasSuffix(r.URL.Path, "/history"):
			answer(w, contract.HistoryAnswer{Tab: contract.HistoryTab(r.URL.Query().Get("tab")), Entries: []contract.HistoryEntry{}, Unread: 4})
		case strings.HasSuffix(r.URL.Path, "/settings"):
			answer(w, contract.SettingsAnswer{FocusPeople: []contract.FocusPerson{}, RemovedPeople: []string{}, Rules: []contract.FocusRule{}, RemovedRules: []contract.FocusRule{}, Levels: []contract.LevelEntry{}})
		case strings.HasSuffix(r.URL.Path, "/preview"):
			answer(w, contract.PreviewAnswer{Last7Days: contract.PreviewCounts{Moved: 14, Stay: 3}})
		}
	}))
	d.search = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if d.searchDown.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		if r.URL.Query().Get("as") != "marco" {
			http.Error(w, "wrong person", http.StatusBadRequest)
			return
		}
		answer(w, contract.SearchAnswer{Through: 3, Total: 1, Results: []contract.SearchResult{}})
	}))
	t.Cleanup(d.feed.Close)
	t.Cleanup(d.search.Close)
	return d
}

func getJSON(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (f *fixture) place(t *testing.T, receiver string, place int64) {
	t.Helper()
	pool, err := f.store.Dir.PoolFor(f.ctx, f.org)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO delivery_places (org, receiver, place) VALUES ($1, $2, $3)
		ON CONFLICT (org, receiver) DO UPDATE SET place = EXCLUDED.place`, string(f.org), receiver, place); err != nil {
		t.Fatal(err)
	}
}

// TestDoorReadsAndTheirNotices is the server half of G-019: each surface says it
// is behind when it is more than 10 s behind, and says nothing when it is not.
func TestDoorReadsAndTheirNotices(t *testing.T) {
	f := setup(t)
	parts := newDoorParts(t)
	l := NewLive(f.store, LiveOptions{Feed: parts.feed.URL, Search: parts.search.URL})
	mux := http.NewServeMux()
	f.store.Routes(mux)
	l.Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	base := srv.URL + "/orgs/" + string(f.org) + "/people/marco/"

	head := f.say(t, "a change")
	f.place(t, "feed", head)
	f.place(t, "search", head)

	code, a := getJSON(t, base+"feed")
	if code != 200 || a["notice"] != nil || a["number"] != float64(9) {
		t.Fatalf("level feed: %d %v", code, a)
	}
	code, a = getJSON(t, base+"search?q=pricing")
	if code != 200 || a["notice"] != nil || a["answer"] == nil {
		t.Fatalf("level search: %d %v", code, a)
	}
	if code, a = getJSON(t, base+"history?tab=mentions"); code != 200 || a["unread"] != float64(4) {
		t.Fatalf("history: %d %v", code, a)
	}
	if code, _ = getJSON(t, base+"settings"); code != 200 {
		t.Fatalf("settings: %d", code)
	}
	if code, a = getJSON(t, base+"preview?change=%7B%7D"); code != 200 || a["last_7_days"] == nil {
		t.Fatalf("preview: %d %v", code, a)
	}

	// Five seconds behind says nothing, fifteen says so.
	waiting := f.say(t, "waiting for the receivers")
	f.clock.Advance(5 * time.Second)
	if _, a = getJSON(t, base+"feed"); a["notice"] != nil {
		t.Fatalf("5 s behind must show no notice, got %v", a["notice"])
	}
	if _, a = getJSON(t, base+"search?q=x"); a["notice"] != nil {
		t.Fatalf("5 s behind must show no notice, got %v", a["notice"])
	}
	f.clock.Advance(10 * time.Second)
	if _, a = getJSON(t, base+"feed"); a["notice"] != "Catching up" || a["rows"] == nil {
		t.Fatalf("15 s behind: the rows it has, with Catching up; got %v", a)
	}
	if _, a = getJSON(t, base+"search?q=x"); a["answer"] == nil || !strings.Contains(a["notice"].(string), "may be missing") {
		t.Fatalf("15 s behind: the answer with its notice; got %v", a)
	}
	if code, a = getJSON(t, base+"preview?change=%7B%7D"); code != 503 || a["not_available"] == nil {
		t.Fatalf("preview while the feed is behind: %d %v", code, a)
	}

	// Level again removes it. A later change search is not sent does not count.
	f.place(t, "feed", waiting)
	f.place(t, "search", waiting)
	f.ok(t, "dial", "marco", map[string]any{"conversation": "#launch", "thread": nil, "level": "muted"})
	f.clock.Advance(30 * time.Second)
	if _, a = getJSON(t, base+"search?q=x"); a["notice"] != nil {
		t.Fatalf("search has everything it is sent, so a quiet stretch must not look late: %v", a["notice"])
	}

	// Out: an answer at once, with the notice, and nothing else stops.
	parts.feedDown.Store(true)
	parts.searchDown.Store(true)
	if code, a = getJSON(t, base+"feed"); code != 200 || a["notice"] != "Catching up" || a["number"] != nil {
		t.Fatalf("feed down: %d %v", code, a)
	}
	if code, a = getJSON(t, base+"search?q=x"); code != 200 || a["answer"] != nil || a["notice"] != "Search is unavailable right now. Your messages are safe." {
		t.Fatalf("search down: %d %v", code, a)
	}
	for _, q := range []string{"history?tab=mentions", "settings", "preview?change=%7B%7D"} {
		if code, a = getJSON(t, base+q); code != 503 || a["not_available"] == nil {
			t.Fatalf("%s with the feed down: %d %v", q, code, a)
		}
	}
	f.say(t, "sending does not depend on them")

	if code, _ = getJSON(t, srv.URL+"/orgs/"+string(f.org)+"/people/nobody/feed"); code != 404 {
		t.Fatalf("an unknown person: %d", code)
	}
	if err := f.store.Retire(f.ctx, f.org); err != nil {
		t.Fatal(err)
	}
	if code, _ = getJSON(t, base+"feed"); code != 410 {
		t.Fatalf("a retired organization: %d", code)
	}
}
