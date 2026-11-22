// Package feedrig runs real copies of the feed service on PostgreSQL behind the
// troublemaker, plays its delivery worker, and works out what the feed must hold
// by folding the log through the rules alone. It is shared by the tests of G-007,
// G-008, G-009 and G-027.
package feedrig

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"
	"time"

	"contexia/internal/feed/rules"
	"contexia/internal/feed/service"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	"contexia/tests/guardrails/notification-once/rig"
	"contexia/tests/harness"
	"contexia/tests/harness/logwriter"

	"github.com/jackc/pgx/v5"
)

// Limit is how long one call may take in these tests.
const Limit = 10 * time.Second

// Change is one log entry on its way to the feed.
type Change struct {
	Number int64
	Kind   string
	Body   json.RawMessage
}

// FromLog turns the log writer's entries into changes.
func FromLog(log []logwriter.Entry) []Change {
	out := make([]Change, len(log))
	for i, e := range log {
		out[i] = Change{Number: e.Number, Kind: e.Kind, Body: e.JSON}
	}
	return out
}

// Rig is the feed's database with any number of copies on it.
type Rig struct {
	T   testing.TB
	Dir orgdb.Single
}

// New connects with the feed's own login.
func New(t testing.TB) *Rig { return &Rig{T: t, Dir: rig.DB(t, "feed")} }

// Copy is one running copy of the feed.
type Copy struct {
	URL   string
	TM    *harness.Troublemaker
	Front string
	stop  []func()
}

// Close ends the copy.
func (c *Copy) Close() {
	for _, f := range c.stop {
		f()
	}
	c.stop = nil
}

// Start runs a new copy, with no memory of any earlier one.
func (r *Rig) Start(seed int64) *Copy {
	direct := httptest.NewServer(service.New(service.Options{Dir: r.Dir}).Routes())
	tm := harness.NewTroublemaker(direct.URL, seed, harness.Trouble{})
	front := httptest.NewServer(tm)
	c := &Copy{URL: direct.URL, TM: tm, Front: front.URL, stop: []func(){front.Close, direct.Close}}
	r.T.Cleanup(c.Close)
	return c
}

// Answer is what a delivery came back with.
type Answer struct {
	Status              int
	IAmAt               *int64  `json:"i_am_at"`
	Applied             bool    `json:"applied"`
	Already             *bool   `json:"already"`
	Fenced              *bool   `json:"fenced"`
	OrganizationStopped *string `json:"organization_stopped"`
	Outage              bool
}

// Post delivers one change or a batch.
func Post(base string, batch []Change) Answer {
	var path string
	var in any
	if len(batch) == 1 {
		path, in = "/changes", batch[0].Body
	} else {
		bodies := make([]json.RawMessage, len(batch))
		for i, c := range batch {
			bodies[i] = c.Body
		}
		path, in = "/changes/batch", map[string]any{"changes": bodies}
	}
	a := rig.Do(base, "POST", path, in, Limit)
	if a.Outage {
		return Answer{Outage: true}
	}
	out := Answer{Status: a.Status}
	_ = json.Unmarshal(a.Body, &out)
	out.Status = a.Status
	return out
}

// Deliver plays the worker: in order, in batches of a random size, retrying what
// got no answer and continuing from wherever the feed says it is. It stops when
// the feed has taken change number `to`, or says the organization is stopped.
func Deliver(t testing.TB, base string, log []Change, to int64, rng *rand.Rand) Answer {
	t.Helper()
	at := map[int64]int{}
	for i, c := range log {
		at[c.Number] = i
	}
	var place int64
	var last Answer
	for tries := 0; place < to; {
		first := 0
		if place > 0 {
			i, ok := at[place]
			if !ok {
				t.Fatalf("the feed says it is at %d, which the log does not hold", place)
			}
			first = i + 1
		}
		n := 1 + rng.Intn(12)
		end := min(first+n, len(log))
		for end > first && log[end-1].Number > to {
			end--
		}
		last = Post(base, log[first:end])
		switch {
		case last.Outage || last.Status >= 500:
			if tries++; tries > 200 {
				t.Fatalf("no answer after %d tries at %d", tries, place)
			}
		case last.Status == http.StatusConflict && last.IAmAt != nil:
			place, tries = *last.IAmAt, 0
		case last.Status == http.StatusLocked:
			return last
		case last.Status == http.StatusOK:
			place, tries = log[end-1].Number, 0
		default:
			t.Fatalf("unexpected answer %d at %d", last.Status, place)
		}
	}
	return last
}

// Place asks the feed where it is.
func Place(t testing.TB, base, org string) contract.FeedPlace {
	t.Helper()
	var p contract.FeedPlace
	Get(t, base, "/orgs/"+org+"/place", &p)
	return p
}

// Get makes a question that has to succeed.
func Get(t testing.TB, base, path string, out any) {
	t.Helper()
	a := rig.Do(base, "GET", path, nil, Limit)
	if a.Outage || a.Status != http.StatusOK {
		t.Fatalf("GET %s: status %d, outage %v: %s", path, a.Status, a.Outage, a.Body)
	}
	if err := json.Unmarshal(a.Body, out); err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
}

// Wipe wipes one organization's feed.
func Wipe(t testing.TB, base, org string, rulesChanged bool) {
	t.Helper()
	var in any = map[string]any{}
	if rulesChanged {
		in = map[string]any{"rules_changed": true}
	}
	if a := rig.Do(base, "POST", "/orgs/"+org+"/control/wipe", in, Limit); a.Outage || a.Status != http.StatusOK {
		t.Fatalf("wipe %s: status %d, outage %v: %s", org, a.Status, a.Outage, a.Body)
	}
}

// Truth is what a clean fold of the log through the rules gives.
type Truth struct {
	E     *rules.Engine
	Notes []rules.NoteDraft
}

// Fold applies the changes for which keep says yes. A nil keep keeps all.
func Fold(t testing.TB, org string, log []Change, to int64, keep func(Change) bool) *Truth {
	t.Helper()
	tr := &Truth{E: rules.New(org)}
	for _, c := range log {
		if c.Number > to {
			break
		}
		if keep != nil && !keep(c) {
			continue
		}
		var ch contract.Change
		if err := json.Unmarshal(c.Body, &ch); err != nil {
			t.Fatalf("change %d: %v", c.Number, err)
		}
		fx, err := tr.E.Apply(ch, rules.Options{})
		if err != nil {
			t.Fatalf("clean fold of change %d: %v", c.Number, err)
		}
		tr.Notes = append(tr.Notes, fx.Notes...)
	}
	return tr
}

// Reader is everything the door can ask about one person, in comparable form.
type Reader struct {
	Rows     []contract.FeedRow
	Counts   contract.FeedCounts
	History  map[contract.HistoryTab]contract.HistoryAnswer
	Settings contract.SettingsAnswer
}

var tabs = []contract.HistoryTab{"direct", "mentions", "threads", "reactions"}

// Ask reads one person through the feed's door, following history pages.
func Ask(t testing.TB, base, org, person string) Reader {
	t.Helper()
	var rows contract.FeedRowsAnswer
	Get(t, base, "/orgs/"+org+"/people/"+url.PathEscape(person)+"/rows", &rows)
	out := Reader{Rows: rows.Rows, Counts: rows.Counts, History: map[contract.HistoryTab]contract.HistoryAnswer{}}
	for _, tab := range tabs {
		var whole contract.HistoryAnswer
		after := ""
		for {
			var page contract.HistoryAnswer
			Get(t, base, "/orgs/"+org+"/people/"+url.PathEscape(person)+"/history?tab="+string(tab)+after, &page)
			whole.Tab, whole.Unread = page.Tab, page.Unread
			whole.Entries = append(whole.Entries, page.Entries...)
			if page.Next == nil {
				break
			}
			after = "&after=" + *page.Next
		}
		if whole.Entries == nil {
			whole.Entries = []contract.HistoryEntry{}
		}
		out.History[tab] = whole
	}
	Get(t, base, "/orgs/"+org+"/people/"+url.PathEscape(person)+"/settings", &out.Settings)
	return out
}

// Of gives the same for a clean fold.
func (tr *Truth) Of(person string) Reader {
	out := Reader{Rows: tr.E.Rows(person), Counts: tr.E.Counts(person), History: map[contract.HistoryTab]contract.HistoryAnswer{},
		Settings: tr.E.Settings(person)}
	if out.Rows == nil {
		out.Rows = []contract.FeedRow{}
	}
	for _, tab := range tabs {
		h := tr.E.History(person, tab)
		h.Next = nil
		out.History[tab] = h
	}
	return out
}

func text(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// DiffReader says how two readers differ. only, if given, limits rows and
// history to what it accepts, for comparing outside a fence.
func DiffReader(person string, got, want Reader, only func(conversation string) bool) []string {
	var out []string
	filterRows := func(in []contract.FeedRow) []contract.FeedRow {
		kept := []contract.FeedRow{}
		for _, r := range in {
			if only == nil || only(r.Conversation) {
				kept = append(kept, r)
			}
		}
		return kept
	}
	a, b := filterRows(got.Rows), filterRows(want.Rows)
	if text(a) != text(b) {
		out = append(out, fmt.Sprintf("%s: rows differ (%d against %d): %s", person, len(a), len(b), firstDiff(a, b)))
	}
	if only == nil && got.Counts != want.Counts {
		out = append(out, fmt.Sprintf("%s: counts %+v, want %+v", person, got.Counts, want.Counts))
	}
	for _, tab := range tabs {
		x, y := got.History[tab], want.History[tab]
		filter := func(in []contract.HistoryEntry) []contract.HistoryEntry {
			kept := []contract.HistoryEntry{}
			for _, e := range in {
				if only == nil || only(e.Conversation) {
					kept = append(kept, e)
				}
			}
			return kept
		}
		if text(filter(x.Entries)) != text(filter(y.Entries)) || (only == nil && x.Unread != y.Unread) {
			out = append(out, fmt.Sprintf("%s: history tab %s differs (%d against %d entries)", person, tab, len(x.Entries), len(y.Entries)))
		}
	}
	if only == nil && text(got.Settings) != text(want.Settings) {
		out = append(out, fmt.Sprintf("%s: settings differ: %s against %s", person, text(got.Settings), text(want.Settings)))
	}
	return out
}

func firstDiff(a, b []contract.FeedRow) string {
	for i := range a {
		if i >= len(b) {
			return "extra " + text(a[i])
		}
		if x, y := text(a[i]), text(b[i]); x != y {
			return x + " against " + y
		}
	}
	if len(b) > len(a) {
		return "missing " + text(b[len(a)])
	}
	return ""
}

// Diff compares every reader of the fold with the feed's answers.
func Diff(t testing.TB, base, org string, tr *Truth) []string {
	t.Helper()
	var out []string
	for _, p := range tr.E.Readers() {
		out = append(out, DiffReader(p, Ask(t, base, org, p), tr.Of(p), nil)...)
	}
	return out
}

// DiffUpdates plays the feed's row updates from the start, the way a screen
// would, and compares the result with the rows answer. Each update must name the
// reader's previous one in `after`, so a screen can see a gap.
func DiffUpdates(t testing.TB, base, org string, people []string) []string {
	t.Helper()
	edition := Place(t, base, org).Edition
	type seen struct {
		rows   map[string]contract.FeedRow
		counts *contract.FeedCounts
		last   int64
	}
	byPerson := map[string]*seen{}
	var out []string
	var after int64
	for {
		var page contract.RowUpdatesPage
		Get(t, base, fmt.Sprintf("/orgs/%s/updates?after=%d&edition=%d&limit=500", org, after, edition), &page)
		for _, u := range page.Updates {
			if u.Number <= after {
				out = append(out, fmt.Sprintf("update %d came after %d", u.Number, after))
			}
			after = u.Number
			s := byPerson[u.Person]
			if s == nil {
				s = &seen{rows: map[string]contract.FeedRow{}}
				byPerson[u.Person] = s
			}
			if u.After != s.last {
				out = append(out, fmt.Sprintf("%s: update %d says the one before was %d, it was %d", u.Person, u.Number, u.After, s.last))
			}
			s.last = u.Number
			for _, part := range u.Parts {
				switch v := part.Value.(type) {
				case contract.FeedPartRowChanged:
					s.rows[keyOf(v.Row.Conversation, v.Row.Thread)] = v.Row
				case contract.FeedPartRowRemoved:
					delete(s.rows, keyOf(v.Row.Conversation, v.Row.Thread))
				case contract.FeedPartCounts:
					c := v.Counts
					s.counts = &c
				}
			}
		}
		if page.Next == nil {
			break
		}
	}
	for _, p := range people {
		var ans contract.FeedRowsAnswer
		Get(t, base, "/orgs/"+org+"/people/"+url.PathEscape(p)+"/rows", &ans)
		s := byPerson[p]
		if s == nil {
			if len(ans.Rows) > 0 {
				out = append(out, p+": has rows but no updates")
			}
			continue
		}
		if ans.Number != s.last {
			out = append(out, fmt.Sprintf("%s: the rows answer says update %d, the log's last is %d", p, ans.Number, s.last))
		}
		inAnswer := map[string]bool{}
		for _, row := range ans.Rows {
			k := keyOf(row.Conversation, row.Thread)
			inAnswer[k] = true
			if got, ok := s.rows[k]; !ok || text(got) != text(row) {
				out = append(out, fmt.Sprintf("%s: a screen playing the updates holds %s for a row that is %s", p, text(got), text(row)))
			}
		}
		for k, row := range s.rows {
			if !inAnswer[k] && row.Unread {
				out = append(out, fmt.Sprintf("%s: a screen still shows %s as unread; the feed no longer has it", p, k))
			}
		}
		if s.counts != nil && *s.counts != ans.Counts {
			out = append(out, fmt.Sprintf("%s: a screen's counts are %+v, the feed's %+v", p, *s.counts, ans.Counts))
		}
	}
	return out
}

func keyOf(conversation string, thread *string) string {
	if thread == nil {
		return conversation
	}
	return conversation + ":" + *thread
}

// StoredNote is one row of the feed's note log.
type StoredNote struct {
	Number  int64
	Kind    string
	Name    string
	History bool
	Body    string
}

// Notes reads one edition of an organization's note log.
func (r *Rig) Notes(org string, edition int64) []StoredNote {
	r.T.Helper()
	var out []StoredNote
	err := orgdb.InOrg(context.Background(), r.Dir, ids.Org(org), func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), `SELECT number, kind, coalesce(body->>'name', ''), coalesce((body->>'history')::boolean, false), body::text
			FROM note_log WHERE org = $1 AND edition = $2 ORDER BY number`, org, edition)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(x pgx.CollectableRow) (StoredNote, error) {
			var n StoredNote
			err := x.Scan(&n.Number, &n.Kind, &n.Name, &n.History, &n.Body)
			return n, err
		})
		return err
	})
	if err != nil {
		r.T.Fatal(err)
	}
	return out
}

// DiffNotes checks the log is numbered without a gap and says the same as the fold.
func DiffNotes(stored []StoredNote, want []rules.NoteDraft) []string {
	var out []string
	for i, n := range stored {
		if n.Number != int64(i)+1 {
			out = append(out, fmt.Sprintf("note %d has number %d", i+1, n.Number))
			break
		}
	}
	if len(stored) != len(want) {
		out = append(out, fmt.Sprintf("%d notes, the fold wrote %d", len(stored), len(want)))
	}
	for i := 0; i < min(len(stored), len(want)); i++ {
		if stored[i].Kind != want[i].Kind || stored[i].Name != want[i].Name {
			out = append(out, fmt.Sprintf("note %d is %s %q, the fold wrote %s %q", i+1, stored[i].Kind, stored[i].Name, want[i].Kind, want[i].Name))
			break
		}
	}
	return out
}

// Held lists what the store holds on hold for an organization.
func (r *Rig) Held(org string) []string {
	r.T.Helper()
	out := []string{}
	err := orgdb.InOrg(context.Background(), r.Dir, ids.Org(org), func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), `SELECT name FROM feed_held WHERE org = $1 ORDER BY name`, org)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	if err != nil {
		r.T.Fatal(err)
	}
	return out
}

// HeldNames is what the fold has on hold, in the same order.
func (tr *Truth) HeldNames() []string {
	out := []string{}
	for _, h := range tr.E.AllHeld() {
		out = append(out, h.Name)
	}
	sort.Strings(out)
	return out
}
