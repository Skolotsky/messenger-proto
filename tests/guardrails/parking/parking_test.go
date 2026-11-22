// Package parking holds G-014: a refused change is parked after three tries,
// shown, and can be sent again; refusals pause one organization and no other.
// It also holds how the workers follow a receiver's reset, wipe and edition.
package parking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	"contexia/tests/guardrails/receivers-independent/rig"

	"github.com/jackc/pgx/v5"
)

var bg = context.Background()

func saveUpTo(t *testing.T, b *rig.Bench, org string, n int) {
	t.Helper()
	for range n {
		if _, err := b.DB.Save(bg, org); err != nil {
			t.Fatal(err)
		}
	}
}

// messages lists the numbers of the org's log that search is sent, up to upTo.
func messages(t *testing.T, b *rig.Bench, org string, upTo int) []int64 {
	t.Helper()
	var out []int64
	err := orgdb.InOrg(bg, b.DB.Directory(), ids.Org(org), func(tx pgx.Tx) error {
		rows, err := tx.Query(bg, `SELECT number FROM change_log WHERE org = $1 AND number <= $2
			AND kind IN ('message.created', 'message.changed', 'message.deleted') ORDER BY number`, org, upTo)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowTo[int64])
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func status(t *testing.T, b *rig.Bench, org, receiver string) contract.ReceiverStatus {
	t.Helper()
	resp, err := http.Get(b.Ops + "/orgs/" + org + "/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out contract.DeliveryStatus
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	for _, r := range out.Receivers {
		if string(r.Name) == receiver {
			return r
		}
	}
	t.Fatalf("no receiver %s in the status", receiver)
	return contract.ReceiverStatus{}
}

func parked(t *testing.T, b *rig.Bench, org string) []contract.ParkedChange {
	t.Helper()
	resp, err := http.Get(b.Ops + "/orgs/" + org + "/parked")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out contract.ParkedList
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.Parked
}

func retry(t *testing.T, b *rig.Bench, org, receiver string, number int64) {
	t.Helper()
	resp, err := http.Post(fmt.Sprintf("%s/orgs/%s/parked/%d/retry?receiver=%s", b.Ops, org, number, receiver), "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("retry of %d answered %d", number, resp.StatusCode)
	}
}

func TestARefusedChangeIsParkedShownAndCanBeSentAgain(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org = "acme"
	b.DB.AddOrg(t, org, 80)
	msgs := messages2(t, b, org, 40)
	bad := msgs[3]
	last := msgs[len(msgs)-1]
	var refusing atomic.Bool
	refusing.Store(true)
	var tries atomic.Int32
	b.Search.Refuse(func(o string, n int64) bool {
		if o == org && n == bad && refusing.Load() {
			tries.Add(1)
			return true
		}
		return false
	})
	b.Start(t, "copy-1")
	saveUpTo(t, b, org, 40)

	rig.Until(t, 15*time.Second, "the worker moves past the refused change", func() bool { return b.Search.Place(org) >= last })
	if got := tries.Load(); got != 3 {
		t.Fatalf("the change was tried %d times before parking, not 3", got)
	}
	if b.Search.Has(org, bad) {
		t.Fatal("the index holds a change it refused")
	}
	list := parked(t, b, org)
	if len(list) != 1 || list[0].Number != bad || list[0].Receiver != "search" || list[0].Tries != 3 || list[0].Reason == "" {
		t.Fatalf("parked list: %+v", list)
	}
	s := status(t, b, org, "search")
	if s.Parked != 1 || s.State != "running" {
		t.Fatalf("status while parked: %+v (the receiver must report it holds something parked)", s)
	}

	refusing.Store(false)
	retry(t, b, org, "search", bad)
	if !b.Search.Has(org, bad) {
		t.Fatal("after release the index still lacks the change")
	}
	if l := parked(t, b, org); len(l) != 0 {
		t.Fatalf("still parked after release: %+v", l)
	}
	if s := status(t, b, org, "search"); s.Parked != 0 {
		t.Fatalf("status after release: %+v", s)
	}
}

// messages2 reads the script before it is saved, by saving nothing: the log
// writer gives every organization the same content, so a twin shows the numbers.
func messages2(t *testing.T, b *rig.Bench, org string, upTo int) []int64 {
	t.Helper()
	twin := org + "-twin"
	b.DB.AddOrg(t, twin, 80)
	saveUpTo(t, b, twin, upTo)
	return messages(t, b, twin, upTo)
}

func TestThreeRefusalsPauseThatOrganizationAndNoOther(t *testing.T) {
	b := rig.NewBench(t, nil)
	const bad, good = "acme", "globex"
	b.DB.AddOrg(t, bad, 80)
	b.DB.AddOrg(t, good, 80)
	var refusing atomic.Bool
	refusing.Store(true)
	b.Search.Refuse(func(o string, _ int64) bool { return o == bad && refusing.Load() })
	b.Start(t, "copy-1")
	saveUpTo(t, b, bad, 40)
	saveUpTo(t, b, good, 40)

	rig.Until(t, 15*time.Second, "the refusing organization is paused", func() bool { return status(t, b, bad, "search").State == "paused" })
	msgs := messages(t, b, bad, 40)
	list := parked(t, b, bad)
	if len(list) != 3 {
		t.Fatalf("parked %d changes before pausing, not 3: %+v", len(list), list)
	}
	for i, p := range list {
		if p.Number != msgs[i] {
			t.Fatalf("parked %d, expected %d", p.Number, msgs[i])
		}
	}
	lastGood := messages(t, b, good, 40)
	rig.Until(t, 10*time.Second, "the other organization stays level", func() bool {
		return b.Search.Place(good) >= lastGood[len(lastGood)-1] && b.Feed.Place(good) >= 40
	})
	if got := b.Search.Count(bad); got != 0 {
		t.Fatalf("the paused organization has %d changes in the index", got)
	}
	rig.Until(t, 10*time.Second, "the feed of the paused organization still runs", func() bool { return b.Feed.Place(bad) >= 40 })

	// Released, it carries on and everything arrives.
	refusing.Store(false)
	for _, p := range list {
		retry(t, b, bad, "search", p.Number)
	}
	rig.Until(t, 15*time.Second, "the organization carries on after release", func() bool {
		return b.Search.Count(bad) == len(msgs) && status(t, b, bad, "search").State == "running"
	})
}

func TestARefusedNoteStopsThatOrganizationsNotesUntilRetried(t *testing.T) {
	b := rig.NewBench(t, nil)
	const bad, good = "acme", "globex"
	b.DB.AddOrg(t, bad, 60)
	b.DB.AddOrg(t, good, 60)
	var refusing atomic.Bool
	refusing.Store(true)
	b.Sender.Refuse(func(o string, n int64) bool { return o == bad && n == 3 && refusing.Load() })
	b.Start(t, "copy-1")
	for _, o := range []string{bad, good} {
		for i := 0; i < 6; i++ {
			kind := "due"
			if i%2 == 1 {
				kind = "clock"
			}
			if _, err := b.DB.SaveNote(bg, o, kind); err != nil {
				t.Fatal(err)
			}
		}
	}
	rig.Until(t, 15*time.Second, "the note is parked", func() bool { return len(parked(t, b, bad)) == 1 })
	rig.Until(t, 10*time.Second, "the other organization's notes carry on", func() bool { return b.Sender.Place(good) >= 6 })
	if s := status(t, b, bad, "sender"); s.State != "paused" || s.Place != 2 {
		t.Fatalf("status at a refused note: %+v (it must hold there)", s)
	}
	if b.Sender.Has(bad, 4) {
		t.Fatal("a note after the refused one was sent: a clock note could let a notification past a parked withdrawal")
	}
	refusing.Store(false)
	retry(t, b, bad, "sender", 3)
	rig.Until(t, 10*time.Second, "the notes carry on after the retry", func() bool { return b.Sender.Place(bad) >= 6 })
	if s := status(t, b, bad, "sender"); s.State != "running" || s.Parked != 0 {
		t.Fatalf("status after the retry: %+v", s)
	}
}

func TestAStoppedFeedIsShownAndOthersCarryOn(t *testing.T) {
	b := rig.NewBench(t, nil)
	const bad, good = "acme", "globex"
	b.DB.AddOrg(t, bad, 60)
	b.DB.AddOrg(t, good, 60)
	b.Feed.StopOrg(bad, true)
	b.Start(t, "copy-1")
	saveUpTo(t, b, bad, 20)
	saveUpTo(t, b, good, 20)
	rig.Until(t, 10*time.Second, "the stopped feed is shown", func() bool { return status(t, b, bad, "feed").State == "stopped" })
	rig.Until(t, 10*time.Second, "the other organization carries on", func() bool { return b.Feed.Place(good) >= 20 })
	if s := status(t, b, bad, "feed"); s.Place != 0 || s.Waiting != 20 {
		t.Fatalf("a stopped organization keeps its place: %+v", s)
	}
	// Repair is a rebuild: the receiver restarts from the start.
	b.Feed.StopOrg(bad, false)
	post(t, b.Ops+"/orgs/"+bad+"/receivers/feed/restart", `{"edition": 2}`)
	rig.Until(t, 10*time.Second, "the feed refills after restart", func() bool { return b.Feed.Place(bad) >= 20 })
}

func post(t *testing.T, url, body string) {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s answered %d", url, resp.StatusCode)
	}
}

func TestAWipedIndexIsRefilledFromTheStart(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org = "acme"
	b.DB.AddOrg(t, org, 100)
	b.Start(t, "copy-1")
	saveUpTo(t, b, org, 40)
	msgs := messages(t, b, org, 40)
	rig.Until(t, 10*time.Second, "search is level", func() bool { return b.Search.Place(org) >= msgs[len(msgs)-1] })

	b.Search.Wipe(org)
	saveUpTo(t, b, org, 10) // the next answer carries the new wipe number
	msgs = messages(t, b, org, 50)
	rig.Until(t, 15*time.Second, "the wiped index holds every message again", func() bool {
		return b.Search.Count(org) == len(msgs)
	})
}

func TestTheFeedsPlaceWinsWhenOutOfSequence(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org = "acme"
	b.DB.AddOrg(t, org, 60)
	saveUpTo(t, b, org, 30)
	// The worker believes 25 was delivered; the feed stands at 20.
	if _, err := b.DB.Del.Exec(bg, `INSERT INTO delivery_places (org, receiver, place) VALUES ($1, 'feed', 25)`, org); err != nil {
		t.Fatal(err)
	}
	b.Feed.SetReset(org, 1, 20)
	b.Start(t, "copy-1")
	rig.Until(t, 10*time.Second, "the worker continues from the feed's place", func() bool { return b.Feed.Place(org) >= 30 })
	for n := int64(21); n <= 30; n++ {
		if !b.Feed.Has(org, n) {
			t.Fatalf("change %d was stepped over: the worker must go back to the place the feed reports", n)
		}
	}
}

func TestANewEditionRestartsTheNotesAtZero(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org = "acme"
	b.DB.AddOrg(t, org, 60)
	b.Start(t, "copy-1")
	for range 5 {
		if _, err := b.DB.SaveNote(bg, org, "clock"); err != nil {
			t.Fatal(err)
		}
	}
	rig.Until(t, 10*time.Second, "the sender has the first edition", func() bool { return b.Sender.Place(org) >= 5 })

	// The feed is rebuilt: a new edition, numbered from 1 again.
	if _, err := b.DB.Feed.Exec(bg, `UPDATE note_heads SET edition = 2, head = 0 WHERE org = $1`, org); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := b.DB.SaveNote(bg, org, "clock"); err != nil {
			t.Fatal(err)
		}
	}
	rig.Until(t, 10*time.Second, "the sender is at the new edition's third note", func() bool {
		return b.Sender.Place(org) == 3 && status(t, b, org, "sender").Place == 3
	})
}

func TestAResetIsFollowed(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org = "acme"
	b.DB.AddOrg(t, org, 80)
	b.Start(t, "copy-1")
	saveUpTo(t, b, org, 20)
	rig.Until(t, 10*time.Second, "the feed is level", func() bool { return b.Feed.Place(org) >= 20 })

	// Reset: the log's number rises and the feed is restored to its saved copy at 10.
	b.Feed.SetReset(org, 2, 10)
	if _, err := b.DB.Conversations.Exec(bg, `UPDATE org_log SET reset = 2 WHERE org = $1`, org); err != nil {
		t.Fatal(err)
	}
	saveUpTo(t, b, org, 5)
	rig.Until(t, 25*time.Second, "the worker follows the reset and carries on from the receiver's place", func() bool {
		return b.Feed.Place(org) >= 25
	})
}

// TestAClaimThatRunsOutIsTakenOver: a worker that died holding a claim blocks
// its organization only until the claim ends.
func TestAClaimThatRunsOutIsTakenOver(t *testing.T) {
	start := time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)
	c := clock.NewManual(start)
	b := rig.NewBench(t, c)
	const org = "acme"
	b.DB.AddOrg(t, org, 60)
	saveUpTo(t, b, org, 20)
	if _, err := b.DB.Del.Exec(bg, `INSERT INTO delivery_places (org, receiver, claimed_by, claimed_until) VALUES ($1, 'feed', 'dead', $2)`,
		org, start.Add(8*time.Second)); err != nil {
		t.Fatal(err)
	}
	b.Start(t, "copy-1")
	rig.Until(t, 5*time.Second, "search, which nobody claimed, is served", func() bool { return b.Search.Place(org) > 0 })
	if b.Feed.Place(org) != 0 {
		t.Fatal("the feed was served while another worker's claim was valid")
	}
	c.Advance(9 * time.Second)
	rig.Until(t, 5*time.Second, "the claim that ran out is taken over", func() bool { return b.Feed.Place(org) >= 20 })
}
