package stack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"contexia/internal/feed/rules"
	"contexia/internal/shared/contract"

	"github.com/jackc/pgx/v5"
)

func (o *Org) head(t testing.TB) int64 {
	t.Helper()
	var n int64
	if err := o.S.Admin.QueryRow(context.Background(), `SELECT head FROM conversations.org_log WHERE org = $1`, o.Name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (o *Org) lastMessage(t testing.TB) int64 {
	t.Helper()
	var n int64
	if err := o.S.Admin.QueryRow(context.Background(), `SELECT coalesce(max(number), 0) FROM conversations.change_log
		WHERE org = $1 AND kind IN ('message.created', 'message.changed', 'message.deleted')`, o.Name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (o *Org) noteHead(t testing.TB) int64 {
	t.Helper()
	var n int64
	err := o.S.Admin.QueryRow(context.Background(), `SELECT head FROM feed.note_heads WHERE org = $1`, o.Name).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func place(address string) (int64, bool) {
	status, out, err := Call("GET", address, nil)
	if err != nil || status != 200 {
		return 0, false
	}
	var p struct {
		Through int64 `json:"through"`
	}
	if json.Unmarshal(out, &p) != nil {
		return 0, false
	}
	return p.Through, true
}

// Settle waits until the log has stopped growing (its clock ticks end about 6 s
// after the last change) and every receiver is level with it.
func (o *Org) Settle(t testing.TB, limit time.Duration) {
	t.Helper()
	deadline := time.Now().Add(limit)
	last, since := int64(-1), time.Now()
	for {
		h := o.head(t)
		if h != last {
			last, since = h, time.Now()
		}
		feedAt, ok1 := place(o.S.Feed + "/orgs/" + o.Name + "/place")
		searchAt, ok2 := place(o.S.Search + "/orgs/" + o.Name + "/place")
		senderAt, ok3 := place(o.S.Sender + "/orgs/" + o.Name + "/place")
		level := ok1 && ok2 && ok3 && feedAt >= h && searchAt >= o.lastMessage(t) && senderAt >= o.noteHead(t)
		if level && time.Since(since) > 2*time.Second {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s did not settle within %v: log %d, feed %d, search %d, sender %d of %d notes",
				o.Name, limit, h, feedAt, searchAt, senderAt, o.noteHead(t))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// CheckSearchEqualsTruth compares the index with the saved messages.
func (o *Org) CheckSearchEqualsTruth(t testing.TB) {
	t.Helper()
	read := func(q string) map[string]string {
		rows, err := o.S.Admin.Query(context.Background(), q, o.Name)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for rows.Next() {
			var id, text string
			var version int64
			if err := rows.Scan(&id, &version, &text); err != nil {
				t.Fatal(err)
			}
			out[id] = fmt.Sprintf("v%d %s", version, text)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	truth := read(`SELECT id, version, text FROM conversations.messages WHERE org = $1 AND NOT deleted`)
	index := read(`SELECT id, version, body FROM search.messages WHERE org = $1 AND NOT deleted`)
	for id, want := range truth {
		if got, ok := index[id]; !ok {
			t.Fatalf("%s: search is missing %s (%s)", o.Name, id, want)
		} else if got != want {
			t.Fatalf("%s: search holds %q for %s, the truth is %q", o.Name, got, id, want)
		}
	}
	for id, got := range index {
		if _, ok := truth[id]; !ok {
			t.Fatalf("%s: search holds %s (%s), which is deleted or never existed", o.Name, id, got)
		}
	}
}

// Replay folds the organization's log through the product's rules from nothing.
func (o *Org) Replay(t testing.TB) (*rules.Engine, map[string]bool) {
	t.Helper()
	e := rules.New(o.Name)
	due := map[string]bool{}
	for _, raw := range o.LogBodies() {
		var c contract.Change
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatalf("a log entry does not decode: %v: %s", err, raw)
		}
		fx, err := e.Apply(c, rules.Options{})
		if err != nil {
			t.Fatalf("a clean replay cannot apply %s: %v", raw, err)
		}
		for _, n := range fx.Notes {
			if n.Kind == "send" && !n.History {
				due[n.Name] = true
			}
		}
	}
	return e, due
}

// CheckFeedEqualsReplay compares what the running feed answers for every person
// with a clean replay of the log.
func (o *Org) CheckFeedEqualsReplay(t testing.TB) {
	t.Helper()
	e, _ := o.Replay(t)
	for _, p := range o.people {
		var a contract.FeedRowsAnswer
		o.S.MustGet(o.S.Feed+"/orgs/"+o.Name+"/people/"+p+"/rows", &a)
		want, got := canonical(t, e.Rows(p)), canonical(t, a.Rows)
		if want != got {
			t.Fatalf("%s: %s's feed differs from a clean replay of the log.\nreplay: %s\nfeed:   %s", o.Name, p, clip(want), clip(got))
		}
	}
}

func canonical(t testing.TB, rows []contract.FeedRow) string {
	t.Helper()
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(b))
	}
	sort.Strings(out)
	return strings.Join(out, "\n")
}

func clip(s string) string {
	if len(s) > 1500 {
		return s[:1500] + "…"
	}
	return s
}

// CheckSentOnce says every notification due by a clean replay was handed to the
// provider exactly once, and nothing else was.
func (o *Org) CheckSentOnce(t testing.TB) {
	t.Helper()
	_, due := o.Replay(t)
	rows, err := o.S.Admin.Query(context.Background(), `SELECT name, count(*) FROM sender.deliveries WHERE org = $1 GROUP BY name`, o.Name)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err != nil {
			t.Fatal(err)
		}
		got[name] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for name := range due {
		if got[name] != 1 {
			t.Fatalf("%s: %q was handed over %d times; it is due once", o.Name, name, got[name])
		}
	}
	for name := range got {
		if !due[name] {
			t.Fatalf("%s: %q was handed over and a clean replay says it is not due", o.Name, name)
		}
	}
}

// CheckScreenEqualsFresh compares what an open screen shows of a channel with a
// fresh load.
func (o *Org) CheckScreenEqualsFresh(t testing.TB, sc *Screen, person string, channels []string) {
	t.Helper()
	fresh := map[string]ScreenMessage{}
	for _, c := range channels {
		for id, m := range o.FreshMessages(person, c) {
			fresh[id] = m
		}
	}
	shown := sc.Messages()
	for id, want := range fresh {
		if want.Deleted {
			if got, ok := shown[id]; ok && !got.Deleted {
				t.Fatalf("%s: the screen still shows deleted message %s", o.Name, id)
			}
			continue
		}
		got, ok := shown[id]
		if !ok {
			t.Fatalf("%s: the screen is missing %s", o.Name, id)
		}
		if got.Version != want.Version || got.Text != want.Text {
			t.Fatalf("%s: the screen shows %s as %+v, a fresh load as %+v", o.Name, id, got, want)
		}
	}
	for id, got := range shown {
		if _, ok := fresh[id]; !ok && !got.Deleted {
			t.Fatalf("%s: the screen shows %s, which a fresh load does not", o.Name, id)
		}
	}
}
