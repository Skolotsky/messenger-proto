// Package receiversindependent holds G-012 (a stopped or slow receiver delays no
// other) and, for the workers, G-027 (two copies lose nothing and repeat nothing).
package receiversindependent

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	"contexia/tests/guardrails/receivers-independent/rig"

	"github.com/jackc/pgx/v5"
)

const (
	rate     = 50
	duration = 60 * time.Second
	target   = 2 * time.Second
)

// latencies times each saved change from its save to the receiver's first record of it.
func latencies(r *rig.Receiver, org string, saved map[int64]time.Time) (got []time.Duration, missing int) {
	for n, at := range saved {
		first, ok := r.FirstSeen(org, n)
		if !ok {
			missing++
			continue
		}
		got = append(got, first.Sub(at))
	}
	return got, missing
}

// TestAStoppedOrSlowReceiverDelaysNoOther runs its three conditions side by
// side, each in a database of its own, so the whole guardrail takes one minute.
func TestAStoppedOrSlowReceiverDelaysNoOther(t *testing.T) {
	for _, c := range []struct {
		name        string
		bad         func(*rig.Bench) *rig.Receiver
		mode        rig.Mode
		other       func(*rig.Bench) *rig.Receiver
		otherIsFeed bool
	}{
		{"search stopped", func(b *rig.Bench) *rig.Receiver { return b.Search }, rig.Off, func(b *rig.Bench) *rig.Receiver { return b.Feed }, true},
		{"search answers after 10 s", func(b *rig.Bench) *rig.Receiver { return b.Search }, rig.Slow, func(b *rig.Bench) *rig.Receiver { return b.Feed }, true},
		{"feed stopped", func(b *rig.Bench) *rig.Receiver { return b.Feed }, rig.Off, func(b *rig.Bench) *rig.Receiver { return b.Search }, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b := rig.NewBench(t, nil)
			const org = "acme"
			b.DB.AddOrg(t, org, rate*int(duration/time.Second)+200)
			b.Start(t, "copy-1")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			if _, err := b.DB.Save(ctx, org); err != nil {
				t.Fatal(err)
			}
			// The first change is not a message: search's worker only moves past it.
			rig.Until(t, 10*time.Second, "both workers have handled the first change", func() bool {
				var n int
				err := b.DB.Del.QueryRow(ctx, `SELECT count(*) FROM delivery_places WHERE org = $1 AND place >= 1`, org).Scan(&n)
				return err == nil && n == 2
			})
			c.bad(b).SetMode(c.mode)

			saved := map[int64]time.Time{}
			var slowest time.Duration
			err := rig.Pace(ctx, rate, duration, func() error {
				start := time.Now()
				n, err := b.DB.Save(ctx, org)
				if err != nil {
					return err
				}
				if d := time.Since(start); d > slowest {
					slowest = d
				}
				saved[n] = start
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			other := c.other(b)
			var last int64
			for n := range saved {
				last = max(last, n)
			}
			rig.Until(t, 10*time.Second, "the healthy receiver has every change", func() bool {
				if c.otherIsFeed {
					return other.Place(org) >= last
				}
				return other.Has(org, last) || skippedBySearch(t, b, org, last, other)
			})

			if !c.otherIsFeed {
				saved = onlyMessages(t, b, org, saved)
			}
			got, missing := latencies(other, org, saved)
			if missing > 0 {
				t.Fatalf("%d of %d changes never reached the healthy receiver", missing, len(saved))
			}
			if p := rig.P95(got); p > target {
				t.Fatalf("with %s the other receiver reflected 19 in 20 changes in %v, over the %v of G-012. A receiver's trouble is reaching another: look for anything the lanes share", c.name, p, target)
			}
			if slowest > time.Second {
				t.Fatalf("a save took %v while a receiver was down: saving must not depend on delivery", slowest)
			}
			t.Logf("%s: other receiver 19 in 20 within %v over %d changes; slowest save %v", c.name, rig.P95(got), len(got), slowest)

			// Nothing was lost for the receiver that was out.
			c.bad(b).SetMode(rig.Up)
			b.Clock.Now()
			rig.Until(t, 90*time.Second, "the receiver that was out catches up with everything", func() bool {
				return c.bad(b).Place(org) >= lastFor(t, b, org, c.bad(b).Kind, last)
			})
		})
	}
}

// lastFor is the last number the receiver is really sent: search gets messages only.
func lastFor(t *testing.T, b *rig.Bench, org, kind string, last int64) int64 {
	if kind != "search" {
		return last
	}
	return lastMessage(t, b, org, last)
}

func lastMessage(t *testing.T, b *rig.Bench, org string, upTo int64) int64 {
	t.Helper()
	var n int64
	err := orgdb.InOrg(context.Background(), b.DB.Directory(), ids.Org(org), func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT coalesce(max(number), 0) FROM change_log
			WHERE org = $1 AND number <= $2 AND kind IN ('message.created', 'message.changed', 'message.deleted')`, org, upTo).Scan(&n)
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func skippedBySearch(t *testing.T, b *rig.Bench, org string, last int64, r *rig.Receiver) bool {
	return r.Place(org) >= lastMessage(t, b, org, last)
}

func onlyMessages(t *testing.T, b *rig.Bench, org string, saved map[int64]time.Time) map[int64]time.Time {
	t.Helper()
	out := map[int64]time.Time{}
	err := orgdb.InOrg(context.Background(), b.DB.Directory(), ids.Org(org), func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), `SELECT number FROM change_log
			WHERE org = $1 AND kind IN ('message.created', 'message.changed', 'message.deleted')`, org)
		if err != nil {
			return err
		}
		ns, err := pgx.CollectRows(rows, pgx.RowTo[int64])
		for _, n := range ns {
			if at, ok := saved[n]; ok {
				out[n] = at
			}
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestTwoCopiesLoseNothingAndRepeatNothing is G-027 for the workers: in a run
// without faults nothing is even delivered twice.
func TestTwoCopiesLoseNothingAndRepeatNothing(t *testing.T) {
	b := rig.NewBench(t, nil)
	orgs := []string{"one", "two", "three", "four"}
	const each = 400
	for _, o := range orgs {
		b.DB.AddOrg(t, o, each)
	}
	b.Start(t, "copy-1")
	b.Start(t, "copy-2")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var wg sync.WaitGroup
	errs := make(chan error, len(orgs))
	for _, o := range orgs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range each {
				if _, err := b.DB.Save(ctx, o); err != nil {
					errs <- err
					return
				}
				for range 2 {
					if _, err := b.DB.SaveNote(ctx, o, "clock"); err != nil {
						errs <- err
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	for _, o := range orgs {
		rig.Until(t, 60*time.Second, "feed, search and sender are level for "+o, func() bool {
			return b.Feed.Place(o) >= each && b.Sender.Place(o) >= 2*each && b.Search.Place(o) >= lastMessage(t, b, o, each)
		})
		if got := b.Feed.Count(o); got != each {
			t.Fatalf("%s: the feed got %d different changes of %d", o, got, each)
		}
	}
	for name, r := range map[string]*rig.Receiver{"search": b.Search, "feed": b.Feed, "sender": b.Sender} {
		if n := r.Repeats(); n != 0 {
			t.Errorf("%s received %d repeats in a run without faults. Two copies are delivering the same organization: check the claim", name, n)
		}
	}
	if b.Nudges.Load() == 0 {
		t.Error("no nudge was sent after the feed confirmed changes")
	}
}

// TestACopyThatDiesLosesNothing kills one of two copies mid-way.
func TestACopyThatDiesLosesNothing(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org, each = "acme", 600
	b.DB.AddOrg(t, org, each)
	b.Feed.PerItem.Store(int64(200 * time.Microsecond))
	b.Start(t, "copy-1")
	b.Start(t, "copy-2")
	ctx := context.Background()
	for i := 0; i < each; i++ {
		if _, err := b.DB.Save(ctx, org); err != nil {
			t.Fatal(err)
		}
		if i == each/3 {
			b.StopCopy(0)
		}
	}
	rig.Until(t, 40*time.Second, "the surviving copy takes over and the feed is level", func() bool {
		return b.Feed.Place(org) >= each
	})
	if got := b.Feed.Count(org); got != each {
		t.Fatalf("the feed got %d different changes of %d", got, each)
	}
}

// TestASaveCaughtMidRemovalIsNotStranded is the lost wake-up: a save that found
// the due row just before the worker removed it adds none of its own.
func TestASaveCaughtMidRemovalIsNotStranded(t *testing.T) {
	b := rig.NewBench(t, nil)
	const org = "acme"
	b.DB.AddOrg(t, org, 60)
	ctx := context.Background()
	if _, err := b.DB.Save(ctx, org); err != nil {
		t.Fatal(err)
	}
	// Level, with the due row still there, as a worker finds it.
	if _, err := b.DB.Del.Exec(ctx, `INSERT INTO delivery_places (org, receiver, place) VALUES ($1, 'search', 1), ($1, 'feed', 1)`, org); err != nil {
		t.Fatal(err)
	}
	b.Feed.SetReset(org, 1, 1)

	// A save that has already passed its "add the due row" step, uncommitted.
	committed := make(chan struct{})
	entered := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- orgdb.InOrg(ctx, orgdb.Single{Pool: b.DB.Conversations}, ids.Org(org), func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE org_log SET head = 2, head_at = now() WHERE org = $1`, org); err != nil {
				return err
			}
			body := fmt.Sprintf(`{"kind":"tick","org":%q,"reset":1,"number":2,"previous":1,"at":"2026-11-22T10:00:00.000Z"}`, org)
			if _, err := tx.Exec(ctx, `INSERT INTO change_log (org, number, at, kind, body) VALUES ($1, 2, now(), 'tick', $2)`, org, body); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO delivery_due (receiver, org) VALUES ('search', $1), ('feed', $1) ON CONFLICT DO NOTHING`, org); err != nil {
				return err
			}
			close(entered)
			<-committed
			return nil
		})
	}()
	<-entered

	b.Start(t, "copy-1")
	rig.Until(t, 10*time.Second, "the worker, seeing itself level, removes the due row", func() bool {
		var n int
		if err := b.DB.Del.QueryRow(ctx, `SELECT count(*) FROM delivery_due WHERE org = $1 AND receiver = 'feed'`, org).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 0
	})
	close(committed)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	rig.Until(t, 10*time.Second, "the change saved during the removal reaches the feed. It is stranded: the due row was removed and nothing put it back", func() bool {
		return b.Feed.Place(org) >= 2
	})
}
