// Package orgsindependent holds G-026: what happens in one organization does not
// delay another. Each condition runs at the same time as the others, in a
// database of its own, so the whole guardrail takes about a minute.
package orgsindependent

import (
	"context"
	"sync"
	"testing"
	"time"

	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	"contexia/tests/guardrails/receivers-independent/rig"

	"github.com/jackc/pgx/v5"
)

const (
	loud, quiet = "loud", "quiet"
	window      = 60 * time.Second
	quietRate   = 10
	loudRate    = 50
	backlog     = 10000
)

type condition struct {
	name  string
	setup func(t *testing.T, b *rig.Bench)
	loud  func(ctx context.Context, b *rig.Bench) error // runs during the window
	held  string                                        // the receiver the loud organization must have ended up paused on
}

func save(ctx context.Context, b *rig.Bench, org string, hold time.Duration) error {
	_, err := b.DB.SaveHolding(ctx, org, hold)
	return err
}

func loudTraffic(hold time.Duration) func(context.Context, *rig.Bench) error {
	return func(ctx context.Context, b *rig.Bench) error {
		rate := loudRate
		if hold > 0 {
			rate = 3 // one save at a time, each holding its transaction open
		}
		return rig.Pace(ctx, rate, window, func() error { return save(ctx, b, loud, hold) })
	}
}

// fillBacklog writes the loud organization's log directly: 10,000 changes waiting.
func fillBacklog(t *testing.T, b *rig.Bench) {
	t.Helper()
	ctx := context.Background()
	err := orgdb.InOrg(ctx, orgdb.Single{Pool: b.DB.Conversations}, ids.Org(loud), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO change_log (org, number, at, kind, body)
			SELECT $1, n, now(), 'tick', jsonb_build_object('kind', 'tick', 'org', $1::text, 'reset', 1, 'number', n, 'previous', n - 1,
				'at', '2026-11-22T10:00:00.000Z')
			FROM generate_series(1, $2::bigint) n`, loud, backlog); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE org_log SET head = $2, head_at = now() WHERE org = $1`, loud, backlog); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO delivery_due (receiver, org, since)
			VALUES ('search', $1, now() - interval '1 minute'), ('feed', $1, now() - interval '1 minute') ON CONFLICT DO NOTHING`, loud)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOneOrganizationDelaysNoOther(t *testing.T) {
	conditions := []condition{
		{name: "keeps saves open 300 ms", loud: loudTraffic(300 * time.Millisecond)},
		{name: "runs at 50 actions a second", loud: loudTraffic(0)},
		{name: "has 10,000 changes waiting", setup: func(t *testing.T, b *rig.Bench) {
			fillBacklog(t, b)
			// Applying costs something, so the backlog lasts through the window.
			b.Feed.PerItem.Store(int64(2 * time.Millisecond))
		}},
		{name: "has three refusals in a row", setup: func(t *testing.T, b *rig.Bench) {
			b.Search.Refuse(func(org string, _ int64) bool { return org == loud })
		}, loud: loudTraffic(0), held: "search"},
		{name: "has a stopped feed", setup: func(t *testing.T, b *rig.Bench) {
			b.Feed.StopOrg(loud, true)
		}, loud: loudTraffic(0), held: "feed"},
	}
	for _, c := range conditions {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			runCondition(t, c)
		})
	}
}

func runCondition(t *testing.T, c condition) {
	b := rig.NewBench(t, nil)
	need := quietRate*int(window/time.Second) + 100
	b.DB.AddOrg(t, quiet, need)
	b.DB.AddOrg(t, loud, loudRate*int(window/time.Second)+100)
	if c.setup != nil {
		c.setup(t, b)
	}
	b.Start(t, "copy-1")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	loudErr := make(chan error, 1)
	if c.loud != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.loud(ctx, b); err != nil && ctx.Err() == nil {
				loudErr <- err
			}
		}()
	}

	saved := map[int64]time.Time{}
	var saveTimes []time.Duration
	err := rig.Pace(ctx, quietRate, window, func() error {
		start := time.Now()
		n, err := b.DB.Save(ctx, quiet)
		if err != nil {
			return err
		}
		saveTimes = append(saveTimes, time.Since(start))
		saved[n] = start
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	select {
	case err := <-loudErr:
		t.Fatal(err)
	default:
	}

	if c.held != "" { // otherwise this condition would prove nothing
		var paused bool
		if err := b.DB.Del.QueryRow(ctx, `SELECT paused FROM delivery_places WHERE org = $1 AND receiver = $2`, loud, c.held).Scan(&paused); err != nil {
			t.Fatal(err)
		}
		if !paused {
			t.Fatalf("the condition did not happen: %s is not paused for the loud organization", c.held)
		}
	}

	var last int64
	for n := range saved {
		last = max(last, n)
	}
	rig.Until(t, 20*time.Second, "the quiet organization's feed has every change", func() bool { return b.Feed.Place(quiet) >= last })

	var feed, search []time.Duration
	msgs := messageNumbers(t, b, quiet)
	for n, at := range saved {
		first, ok := b.Feed.FirstSeen(quiet, n)
		if !ok {
			t.Fatalf("change %d never reached the feed", n)
		}
		feed = append(feed, first.Sub(at))
		if msgs[n] {
			if first, ok := b.Search.FirstSeen(quiet, n); ok {
				search = append(search, first.Sub(at))
			} else {
				rig.Until(t, 10*time.Second, "search gets the change", func() bool { return b.Search.Has(quiet, n) })
				first, _ := b.Search.FirstSeen(quiet, n)
				search = append(search, first.Sub(at))
			}
		}
	}
	pSave, pFeed, pSearch := rig.P95(saveTimes), rig.P95(feed), rig.P95(search)
	t.Logf("%s: quiet saves 19 in 20 within %v; feed %v; search %v (%d changes)", c.name, pSave, pFeed, pSearch, len(feed))
	if pFeed > 2*time.Second || pSearch > 2*time.Second {
		t.Fatalf("while one organization %s, another's changes took %v (feed) and %v (search) for 19 in 20, over the 2 s of G-026. Organizations are not being taken in turn", c.name, pFeed, pSearch)
	}
	if pSave > 100*time.Millisecond {
		t.Fatalf("while one organization %s, another's saves took %v for 19 in 20, over the 100 ms of G-026", c.name, pSave)
	}
}

func messageNumbers(t *testing.T, b *rig.Bench, org string) map[int64]bool {
	t.Helper()
	out := map[int64]bool{}
	err := orgdb.InOrg(context.Background(), b.DB.Directory(), ids.Org(org), func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), `SELECT number FROM change_log
			WHERE org = $1 AND kind IN ('message.created', 'message.changed', 'message.deleted')`, org)
		if err != nil {
			return err
		}
		ns, err := pgx.CollectRows(rows, pgx.RowTo[int64])
		for _, n := range ns {
			out[n] = true
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
