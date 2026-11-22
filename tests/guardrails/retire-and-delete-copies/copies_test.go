// Package retiredeletecopies holds the parts of G-031 that need more than one
// delivery copy, or a question put to a part after the delete: a retired or
// stopped organization is delivered nothing by any copy, and a deleted one
// leaves nothing behind and does not stay halted if its ID comes back.
package retiredeletecopies

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"contexia/internal/conversations"
	"contexia/internal/delivery"
	"contexia/internal/feed/orgdelete"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	noterig "contexia/tests/guardrails/notification-once/rig"
	"contexia/tests/guardrails/receivers-independent/rig"

	"github.com/jackc/pgx/v5"
)

var bg = context.Background()

// gate stands in front of a receiver and holds the first delivery for one
// organization inside the call, which forces the interleaving that is otherwise
// a race: a copy is mid-call while another copy is told to stop.
type gate struct {
	org     string
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	calls   atomic.Int64
	proxy   *httputil.ReverseProxy
}

func newGate(t *testing.T, target, org string) (*gate, string) {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	g := &gate{org: org, entered: make(chan struct{}), release: make(chan struct{}), proxy: httputil.NewSingleHostReverseProxy(u)}
	s := httptest.NewServer(g)
	t.Cleanup(func() {
		g.open()
		s.Close()
	})
	return g, s.URL
}

func (g *gate) open() { g.once.Do(func() { close(g.release) }) }

func (g *gate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		if strings.Contains(string(body), `"org":"`+g.org+`"`) {
			if g.calls.Add(1) == 1 {
				close(g.entered)
				<-g.release
			}
		}
	}
	g.proxy.ServeHTTP(w, r)
}

// startCopy builds one delivery copy, serving its operations; run says whether
// its workers run too.
func startCopy(t *testing.T, b *rig.Bench, name, search string, run bool) string {
	t.Helper()
	s := delivery.New(delivery.Config{DB: b.DB.Directory(), Clock: clock.Real{}, Search: search, Feed: b.Feed.URL, Sender: b.Sender.URL, Seed: 47, Worker: name})
	if run {
		ctx, cancel := context.WithCancel(bg)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Run(ctx)
		}()
		t.Cleanup(func() {
			cancel()
			wg.Wait()
		})
	}
	ops := httptest.NewServer(s.Routes())
	t.Cleanup(ops.Close)
	return ops.URL
}

func ask(t *testing.T, method, url string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(bg, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func saves(t *testing.T, b *rig.Bench, org string, n int) {
	t.Helper()
	for range n {
		if _, err := b.DB.Save(bg, org); err != nil {
			t.Fatal(err)
		}
	}
}

func notes(t *testing.T, b *rig.Bench, org string, n int) {
	t.Helper()
	for range n {
		if _, err := b.DB.SaveNote(bg, org, "clock"); err != nil {
			t.Fatal(err)
		}
	}
}

func store(b *rig.Bench) *conversations.Store {
	return &conversations.Store{Dir: orgdb.Single{Pool: b.DB.Conversations}, Clock: clock.Real{}}
}

// The copy holding the claim is not the one told to stop. Stop may not answer
// "stopped" while that copy is inside a call, and once it does, that copy makes
// no further call.
func TestStopWaitsForTheCopyHoldingTheClaim(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org = "two-copies"
	b.DB.AddOrg(t, org, 80)
	saves(t, b, org, 60)
	g, front := newGate(t, b.Search.URL, org)
	told := startCopy(t, b, "told", front, false)
	startCopy(t, b, "holder", front, true)
	select {
	case <-g.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the holding copy never reached search")
	}

	status, body := ask(t, "POST", told+"/orgs/"+org+"/stop")
	if status == http.StatusOK {
		t.Errorf("stop answered 200 while another copy was inside a call to search: %s", body)
	}
	g.open()
	rig.Until(t, 15*time.Second, "stop is answered once the other copy has let go", func() bool {
		status, _ := ask(t, "POST", told+"/orgs/"+org+"/stop")
		return status == http.StatusOK
	})
	after := g.calls.Load()
	if after > 1 {
		t.Errorf("the holding copy made %d calls to search: it must read the stop before each one", after)
	}
	saves(t, b, org, 5)
	time.Sleep(300 * time.Millisecond) // several polls of the running copy: nothing may arrive
	if got := g.calls.Load(); got != after {
		t.Errorf("search got %d more calls after stop answered", got-after)
	}
}

// Retiring is enough: no receiver gets anything, with nobody calling stop.
func TestRetireAloneStopsEveryReceiver(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org, other = "retired-only", "still-running"
	for _, o := range []string{org, other} {
		b.DB.AddOrg(t, o, 60)
		saves(t, b, o, 30)
		notes(t, b, o, 10)
	}
	if err := store(b).Retire(bg, ids.Org(org)); err != nil {
		t.Fatal(err)
	}
	b.Start(t, "copy-1")
	rig.Until(t, 15*time.Second, "the other organization is served", func() bool {
		// Search is sent messages only, so its worker's place says how far it got.
		var searched int64
		err := b.DB.Del.QueryRow(bg, `SELECT coalesce((SELECT place FROM delivery_places WHERE org = $1 AND receiver = 'search'), 0)`, other).Scan(&searched)
		if err != nil {
			t.Fatal(err)
		}
		return searched >= 30 && b.Feed.Place(other) >= 30 && b.Sender.Place(other) >= 10
	})
	if s, f, n := b.Search.Count(org), b.Feed.Count(org), b.Sender.Count(org); s+f+n != 0 {
		t.Errorf("a retired organization was delivered: search %d, feed %d, sender %d", s, f, n)
	}
}

// Retiring while a copy is in the middle of a batch ends it at the next call.
func TestRetireStopsABatchUnderWay(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org = "retired-mid-batch"
	b.DB.AddOrg(t, org, 80)
	saves(t, b, org, 60)
	g, front := newGate(t, b.Search.URL, org)
	startCopy(t, b, "holder", front, true)
	select {
	case <-g.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the copy never reached search")
	}
	if err := store(b).Retire(bg, ids.Org(org)); err != nil {
		t.Fatal(err)
	}
	g.open()
	rig.Until(t, 5*time.Second, "the copy lets go of the organization", func() bool {
		var n int
		if err := b.DB.Del.QueryRow(bg, `SELECT count(*) FROM delivery_places WHERE org = $1 AND receiver = 'search' AND claimed_by IS NOT NULL`, org).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 0
	})
	if got := g.calls.Load(); got > 1 {
		t.Errorf("search got %d calls: the batch went on after the organization was retired", got)
	}
}

func stateRows(t *testing.T, r *noterig.Rig, org string) int {
	t.Helper()
	var n int
	if err := r.Dir.Pool.QueryRow(bg, `SELECT count(*) FROM state WHERE org = $1`, org).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAskingSearchAboutADeletedOrganizationCreatesNothing(t *testing.T) {
	r := noterig.New(t, "search", 3)
	org := noterig.NewOrg("asked")
	change := map[string]any{"kind": "message.created", "org": org, "number": 1, "message": map[string]any{
		"id": "m1", "number": 1, "version": 1, "conversation": "#a", "thread": nil, "author": "anna", "edited": false,
		"participants": nil, "created_at": "2026-11-22T09:00:00.000Z", "text": "words"}}
	r.Must("POST", "/changes", change, nil)
	r.Must("DELETE", "/orgs/"+org, nil, nil)
	var place contract.SearchPlace
	r.Must("GET", "/orgs/"+org+"/place", nil, &place)
	if place.Through != 0 || place.Wipe != 0 {
		t.Errorf("the place of a deleted organization: %+v, want the start", place)
	}
	r.Wipe(org)
	if n := stateRows(t, r, org); n != 0 {
		t.Errorf("asking about a deleted organization created %d state row(s) in search", n)
	}
}

func TestAskingTheSenderAboutADeletedOrganizationCreatesNothing(t *testing.T) {
	r := noterig.New(t, "sender", 3)
	org := noterig.NewOrg("asked")
	n := &noterig.Builder{Org: org, Edition: 1}
	n.Add("clock", "", noterig.Start, nil)
	noterig.DeliverNotes(t, r.Direct, n.Notes)
	r.Must("DELETE", "/orgs/"+org, nil, nil)
	var place contract.SenderPlace
	r.Must("GET", "/orgs/"+org+"/place", nil, &place)
	if place.Through != 0 || place.Edition != 1 {
		t.Errorf("the place of a deleted organization: %+v, want the start", place)
	}
	r.Wipe(org)
	if got := stateRows(t, r, org); got != 0 {
		t.Errorf("asking about a deleted organization created %d state row(s) in the sender", got)
	}
}

// One copy is told to stop and another to delete. The first must not keep a
// private memory of the stop: when the ID comes back it is delivered.
func TestAReusedIDIsDelivered(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org = "reused"
	b.DB.AddOrg(t, org, 40)
	saves(t, b, org, 3)
	st := store(b)
	if err := st.Retire(bg, ids.Org(org)); err != nil {
		t.Fatal(err)
	}
	stopper := delivery.New(delivery.Config{DB: b.DB.Directory(), Clock: clock.Real{}, Search: b.Search.URL, Feed: b.Feed.URL, Sender: b.Sender.URL, Seed: 47, Worker: "stopper"})
	ops := httptest.NewServer(stopper.Routes())
	t.Cleanup(ops.Close)
	if status, body := ask(t, "POST", ops.URL+"/orgs/"+org+"/stop"); status != http.StatusOK {
		t.Fatalf("stop: %d %s", status, body)
	}
	other := startCopy(t, b, "deleter", b.Search.URL, false)
	if status, body := ask(t, "DELETE", other+"/orgs/"+org); status != http.StatusOK {
		t.Fatalf("delete on the other copy: %d %s", status, body)
	}
	if err := orgdelete.Delete(bg, orgdb.Single{Pool: b.DB.Feed}, ids.Org(org)); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteOrg(bg, ids.Org(org)); err != nil {
		t.Fatal(err)
	}

	b.DB.AddOrg(t, org, 40) // the same ID, a new organization
	saves(t, b, org, 10)
	ctx, cancel := context.WithCancel(bg)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		stopper.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})
	deadline := time.Now().Add(5 * time.Second)
	for b.Feed.Place(org) < 10 {
		if time.Now().After(deadline) {
			t.Fatalf("the copy told to stop the old organization delivers nothing for the reused ID: feed at %d", b.Feed.Place(org))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The whole sequence on the real search stand-in with two running copies, without
// forcing who holds the claim: a finished delete leaves nothing in search.
func TestDeleteWithTwoRunningCopiesLeavesNothingInSearch(t *testing.T) {
	b := rig.NewBench(t, nil)
	r := noterig.New(t, "search", 3)
	one := startCopy(t, b, "copy-1", r.Direct, true)
	startCopy(t, b, "copy-2", r.Direct, true)
	st := store(b)
	for trial := range 12 {
		org := noterig.NewOrg("race")
		b.DB.AddOrg(t, org, 140)
		saves(t, b, org, 130)
		rig.Until(t, 10*time.Second, "search has begun", func() bool {
			var n int
			if err := orgdb.InOrg(bg, r.Dir, ids.Org(org), func(tx pgx.Tx) error {
				return tx.QueryRow(bg, `SELECT count(*) FROM messages WHERE org = $1`, org).Scan(&n)
			}); err != nil {
				t.Fatal(err)
			}
			return n >= 3
		})
		if err := st.Retire(bg, ids.Org(org)); err != nil {
			t.Fatal(err)
		}
		rig.Until(t, 15*time.Second, "stop is answered", func() bool {
			status, _ := ask(t, "POST", one+"/orgs/"+org+"/stop")
			return status == http.StatusOK
		})
		if status, body := ask(t, "DELETE", r.Direct+"/orgs/"+org); status != http.StatusOK {
			t.Fatalf("delete search: %d %s", status, body)
		}
		time.Sleep(150 * time.Millisecond) // long enough for a copy still going to write again
		var messages, state int
		if err := orgdb.InOrg(bg, r.Dir, ids.Org(org), func(tx pgx.Tx) error {
			return tx.QueryRow(bg, `SELECT (SELECT count(*) FROM messages WHERE org = $1), (SELECT count(*) FROM state WHERE org = $1)`, org).Scan(&messages, &state)
		}); err != nil {
			t.Fatal(err)
		}
		if messages+state != 0 {
			t.Fatalf("trial %d: after a finished delete search holds %d message(s) and %d state row(s)", trial, messages, state)
		}
	}
}
