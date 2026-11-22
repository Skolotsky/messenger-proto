// Package retiredeleteslow holds the parts of G-031 that need a receiver slower
// than a delivery claim, on a real clock: a frozen clock never lets a claim run
// out, so this failure cannot appear on it. The claim is shortened for the test
// and stays longer than every call made here, as the real 8 s is to the 3 s limit.
package retiredeleteslow

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"contexia/internal/conversations"
	"contexia/internal/delivery"
	"contexia/internal/feed/orgdelete"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	noterig "contexia/tests/guardrails/notification-once/rig"
	"contexia/tests/guardrails/receivers-independent/rig"

	"github.com/jackc/pgx/v5"
)

const (
	shortClaim = 500 * time.Millisecond
	callTakes  = 120 * time.Millisecond
	// settle is long enough for a copy that kept going to land several more calls.
	settle = 5 * callTakes
)

var bg = context.Background()

// slow answers every delivery after a delay, in front of a real stand-in.
type slow struct {
	delay   atomic.Int64
	proxy   *httputil.ReverseProxy
	armed   atomic.Bool
	entered chan struct{} // closed when the armed call has arrived
}

// holdNext makes the next delivery stay in flight for hold.
const hold = 350 * time.Millisecond

func (s *slow) holdNext() <-chan struct{} {
	s.entered = make(chan struct{})
	s.armed.Store(true)
	return s.entered
}

func newSlow(t *testing.T, target string) (*slow, string) {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	s := &slow{proxy: httputil.NewSingleHostReverseProxy(u)}
	s.delay.Store(int64(callTakes))
	front := httptest.NewServer(s)
	t.Cleanup(front.Close)
	return s, front.URL
}

func (s *slow) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if s.armed.CompareAndSwap(true, false) {
			close(s.entered)
			time.Sleep(hold)
		} else {
			time.Sleep(time.Duration(s.delay.Load()))
		}
	}
	s.proxy.ServeHTTP(w, r)
}

// bench is real search and sender stand-ins, each slow, and one running copy.
type bench struct {
	b                  *rig.Bench
	searchRig, sendRig *noterig.Rig
	searchSlow, sender *slow
	ops                string
	store              *conversations.Store
}

func newBench(t *testing.T) *bench {
	t.Helper()
	b := rig.NewBench(t, nil)
	x := &bench{b: b, searchRig: noterig.New(t, "search", 3), sendRig: noterig.New(t, "sender", 3)}
	var searchURL, senderURL string
	x.searchSlow, searchURL = newSlow(t, x.searchRig.Direct)
	x.sender, senderURL = newSlow(t, x.sendRig.Direct)
	x.store = &conversations.Store{Dir: orgdb.Single{Pool: b.DB.Conversations}, Clock: clock.Real{}}
	s := delivery.New(delivery.Config{DB: b.DB.Directory(), Clock: clock.Real{}, Search: searchURL, Feed: b.Feed.URL, Sender: senderURL,
		Seed: 47, Worker: "copy-1", ClaimFor: shortClaim})
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
	ops := httptest.NewServer(s.Routes())
	t.Cleanup(ops.Close)
	x.ops = ops.URL
	return x
}

func ask(t *testing.T, method, url string) int {
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
	return resp.StatusCode
}

func (x *bench) fill(t *testing.T, org string, changes, notes int) {
	t.Helper()
	x.b.DB.AddOrg(t, org, changes+10)
	for range changes {
		if _, err := x.b.DB.Save(bg, org); err != nil {
			t.Fatal(err)
		}
	}
	for range notes {
		if _, err := x.b.DB.SaveNote(bg, org, "clock"); err != nil {
			t.Fatal(err)
		}
	}
}

func count(t *testing.T, r *noterig.Rig, org, table string) int {
	t.Helper()
	var n int
	if err := orgdb.InOrg(bg, r.Dir, ids.Org(org), func(tx pgx.Tx) error {
		return tx.QueryRow(bg, `SELECT count(*) FROM `+table+` WHERE org = $1`, org).Scan(&n)
	}); err != nil {
		t.Fatal(err)
	}
	return n
}

// deleteAll is the contract's order, each step asked again while it says "not yet".
func (x *bench) deleteAll(t *testing.T, org string) {
	t.Helper()
	if err := x.store.Retire(bg, ids.Org(org)); err != nil {
		t.Fatal(err)
	}
	rig.Until(t, 20*time.Second, "stop is answered", func() bool { return ask(t, "POST", x.ops+"/orgs/"+org+"/stop") == http.StatusOK })
	for _, base := range []string{x.searchRig.Direct, x.sendRig.Direct} {
		if status := ask(t, "DELETE", base+"/orgs/"+org); status != http.StatusOK {
			t.Fatalf("delete on a receiver: %d", status)
		}
	}
	if err := orgdelete.Delete(bg, orgdb.Single{Pool: x.b.DB.Feed}, ids.Org(org)); err != nil {
		t.Fatal(err)
	}
	rig.Until(t, 20*time.Second, "delivery's delete is answered", func() bool { return ask(t, "DELETE", x.ops+"/orgs/"+org) == http.StatusOK })
	if err := x.store.DeleteOrg(bg, ids.Org(org)); err != nil {
		t.Fatalf("delete conversations: %v", err)
	}
}

// A receiver slower than the claim: once every part has answered the delete,
// nothing of the organization comes back in search or the sender.
func TestASlowReceiverGetsNothingAfterDelete(t *testing.T) {
	x := newBench(t)
	org := noterig.NewOrg("slow")
	x.fill(t, org, 60, 40)
	rig.Until(t, 15*time.Second, "both receivers are under way, past the first claim", func() bool {
		return count(t, x.searchRig, org, "messages") >= 3 && count(t, x.sendRig, org, "state") == 1
	})
	time.Sleep(shortClaim + callTakes) // the turn now outlives the claim it started with
	x.deleteAll(t, org)
	time.Sleep(settle)
	if m, s := count(t, x.searchRig, org, "messages"), count(t, x.searchRig, org, "state"); m+s != 0 {
		t.Errorf("after a finished delete search holds %d message(s) and %d state row(s)", m, s)
	}
	if s := count(t, x.sendRig, org, "state"); s != 0 {
		t.Errorf("after a finished delete the sender holds %d state row(s)", s)
	}
}

// A call in flight, from a turn older than the claim it began with, is waited for:
// it cannot land after the delete, on search or on the sender.
func TestACallInFlightIsWaitedFor(t *testing.T) {
	for _, name := range []string{"search", "sender"} {
		t.Run(name, func(t *testing.T) {
			x := newBench(t)
			org := noterig.NewOrg("held")
			gate, r, table := x.searchSlow, x.searchRig, "messages"
			if name == "sender" {
				gate, r, table = x.sender, x.sendRig, "state"
			}
			x.fill(t, org, 60, 40)
			rig.Until(t, 15*time.Second, "the receiver is under way", func() bool { return count(t, r, org, table) >= 1 })
			time.Sleep(shortClaim + callTakes)
			select {
			case <-gate.holdNext():
			case <-time.After(10 * time.Second):
				t.Fatal("no further call arrived")
			}
			x.deleteAll(t, org)
			time.Sleep(hold + settle)
			if n := count(t, r, org, "state"); n != 0 {
				t.Errorf("a call in flight landed after the delete: %s holds %d state row(s)", name, n)
			}
			if name == "search" {
				if n := count(t, r, org, "messages"); n != 0 {
					t.Errorf("a call in flight landed after the delete: search holds %d message(s)", n)
				}
			}
		})
	}
}

// The same ID seeded again holds exactly its own messages.
func TestAReusedIDHoldsOnlyItsOwnMessages(t *testing.T) {
	x := newBench(t)
	org := noterig.NewOrg("reseed")
	x.fill(t, org, 80, 0)
	rig.Until(t, 15*time.Second, "search is under way", func() bool { return count(t, x.searchRig, org, "messages") >= 3 })
	time.Sleep(shortClaim + callTakes)
	x.deleteAll(t, org)
	time.Sleep(settle)

	x.searchSlow.delay.Store(0)
	x.fill(t, org, 45, 0)
	var head int64
	rig.Until(t, 15*time.Second, "the new log is delivered to its end", func() bool {
		var place int64
		err := x.b.DB.Del.QueryRow(bg, `SELECT h.head, coalesce((SELECT place FROM delivery_places p WHERE p.org = h.org AND p.receiver = 'search'), 0)
			FROM org_log h WHERE h.org = $1`, org).Scan(&head, &place)
		if err != nil {
			t.Fatal(err)
		}
		return place >= head
	})
	own := map[string]bool{}
	if err := orgdb.InOrg(bg, orgdb.Single{Pool: x.b.DB.Conversations}, ids.Org(org), func(tx pgx.Tx) error {
		rows, err := tx.Query(bg, `SELECT body->'message'->>'id' FROM change_log WHERE org = $1 AND kind = 'message.created'`, org)
		if err != nil {
			return err
		}
		list, err := pgx.CollectRows(rows, pgx.RowTo[string])
		for _, id := range list {
			own[id] = true
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var held []string
	var through int64
	if err := orgdb.InOrg(bg, x.searchRig.Dir, ids.Org(org), func(tx pgx.Tx) error {
		if err := tx.QueryRow(bg, `SELECT through FROM state WHERE org = $1`, org).Scan(&through); err != nil {
			return err
		}
		rows, err := tx.Query(bg, `SELECT id FROM messages WHERE org = $1`, org)
		if err != nil {
			return err
		}
		held, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	}); err != nil {
		t.Fatal(err)
	}
	foreign := 0
	for _, id := range held {
		if !own[id] {
			foreign++
		}
	}
	if foreign != 0 || len(held) != len(own) || through > head {
		t.Errorf("the reused ID: search holds %d message(s), %d not in the organization, of %d it has; through %d against a head of %d",
			len(held), foreign, len(own), through, head)
	}
}

func deliveryRows(t *testing.T, x *bench, org string) string {
	t.Helper()
	var out string
	for _, table := range []string{"delivery_due", "delivery_places", "delivery_parked", "delivery_stopped", "feed.note_due", "feed.note_places", "feed.note_parked"} {
		var n int
		if err := x.b.DB.Del.QueryRow(bg, `SELECT count(*) FROM `+table+` WHERE org = $1`, org).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			out += fmt.Sprintf("%s:%d ", table, n)
		}
	}
	return out
}

// Telling delivery to stop or restart a deleted organization creates nothing,
// and the ID can come back and be delivered.
func TestStopAndRestartAfterADeleteCreateNothing(t *testing.T) {
	x := newBench(t)
	org := noterig.NewOrg("after")
	x.fill(t, org, 5, 1)
	x.deleteAll(t, org)

	if status := ask(t, "POST", x.ops+"/orgs/"+org+"/stop"); status != http.StatusOK && status != http.StatusNotFound {
		t.Errorf("stop on a deleted organization answered %d", status)
	}
	if left := deliveryRows(t, x, org); left != "" {
		t.Errorf("stop on a deleted organization created %s", left)
	}
	for _, receiver := range []string{"search", "feed", "sender"} {
		if status := ask(t, "POST", x.ops+"/orgs/"+org+"/receivers/"+receiver+"/restart"); status != http.StatusNotFound {
			t.Errorf("restart of %s on a deleted organization answered %d, want 404", receiver, status)
		}
	}
	if left := deliveryRows(t, x, org); left != "" {
		t.Errorf("restart on a deleted organization created %s", left)
	}

	x.fill(t, org, 20, 0)
	rig.Until(t, 15*time.Second, "the reused ID is delivered", func() bool { return x.b.Feed.Place(org) >= 20 })
}

// A delete that crashed after the stop step, or that already finished, can be
// repeated from the start.
func TestARepeatedDeleteSucceeds(t *testing.T) {
	x := newBench(t)
	org := noterig.NewOrg("repeat")
	x.fill(t, org, 5, 1)
	if err := x.store.Retire(bg, ids.Org(org)); err != nil {
		t.Fatal(err)
	}
	rig.Until(t, 10*time.Second, "stop is answered", func() bool { return ask(t, "POST", x.ops+"/orgs/"+org+"/stop") == http.StatusOK })
	x.deleteAll(t, org)
	if status := ask(t, "DELETE", x.ops+"/orgs/"+org); status != http.StatusOK {
		t.Errorf("delivery's delete repeated after the organization is gone: %d", status)
	}
	if err := x.store.DeleteOrg(bg, ids.Org(org)); err != nil {
		t.Errorf("Conversations' delete repeated: %v", err)
	}
	if left := deliveryRows(t, x, org); left != "" {
		t.Errorf("a repeated delete left %s", left)
	}
}

// Conversations goes last: while delivery still holds rows of the organization,
// also the ones it keeps beside the feed's notes, its delete is refused.
func TestConversationsCannotDeleteBeforeDelivery(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org = "out-of-order"
	b.DB.AddOrg(t, org, 10)
	if _, err := b.DB.Del.Exec(bg, `INSERT INTO feed.note_places (org, place) VALUES ($1, 0)`, org); err != nil {
		t.Fatal(err)
	}
	st := &conversations.Store{Dir: orgdb.Single{Pool: b.DB.Conversations}, Clock: clock.Real{}}
	if err := st.Retire(bg, ids.Org(org)); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteOrg(bg, ids.Org(org)); err == nil {
		t.Error("Conversations deleted the organization while delivery still held its note place")
	}
	var logs int
	if err := b.DB.Del.QueryRow(bg, `SELECT count(*) FROM org_log WHERE org = $1`, org).Scan(&logs); err != nil {
		t.Fatal(err)
	}
	if logs != 1 {
		t.Errorf("the log row is gone (%d) while delivery still holds rows", logs)
	}
}
