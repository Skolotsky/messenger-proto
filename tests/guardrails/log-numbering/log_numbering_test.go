// Package lognumbering holds G-005: a reader of an organization's change log
// never steps over a change, with many writers at once. It runs the real save
// path on PostgreSQL. Built with -tags fault_plain_sequence the save takes its
// number from a plain sequence, and this test must fail.
package lognumbering

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"contexia/internal/conversations"
	"contexia/internal/shared/clock"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/migrate"
	"contexia/internal/shared/orgdb"
	sql "contexia/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	writers     = 16
	savesEach   = 20
	holdAtLeast = 400 * time.Millisecond // how long the first save stays open if nothing can pass it
)

func TestFollowerNeverStepsOverAChange(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	host := os.Getenv("CONTEXIA_PG_HOST")
	if host == "" {
		t.Fatal("CONTEXIA_PG_HOST is not set. Run this test through ./scripts/verify")
	}
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://conversations:conversations_dev@%s:5432/contexia?pool_max_conns=24", host))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.Apply(ctx, pool, sql.Files, "conversations"); err != nil {
		t.Fatal(err)
	}
	// Only the deliberate fault build reads this sequence.
	if _, err := pool.Exec(ctx, `CREATE SEQUENCE IF NOT EXISTS conversations.fault_log_numbers START 1000000`); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	org := ids.Org("g005-" + hex.EncodeToString(b))
	dir := orgdb.Single{Pool: pool}
	store := &conversations.Store{Dir: dir, Clock: clock.Real{}}
	session, err := store.SeedOpen(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SeedSeal(ctx, org, session); err != nil {
		t.Fatal(err)
	}
	save := func() error {
		return store.Save(ctx, org, func(st *conversations.Step) error {
			_, err := st.Append(contract.ChangeTick{Kind: "tick"})
			return err
		})
	}

	// The follower reads like a delivery worker: everything past its place, in
	// order, then moves its place to the last number it saw.
	seen := map[int64]bool{}
	var passes atomic.Int64
	stop := make(chan struct{})
	followed := make(chan error, 1)
	pass := func(place *int64) error {
		return orgdb.InOrg(ctx, dir, org, func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `SELECT number FROM change_log WHERE org = $1 AND number > $2 ORDER BY number`, string(org), *place)
			if err != nil {
				return err
			}
			numbers, err := pgx.CollectRows(rows, pgx.RowTo[int64])
			if err != nil {
				return err
			}
			for _, n := range numbers {
				seen[n] = true
				*place = n
			}
			return nil
		})
	}
	go func() {
		var place int64
		for {
			select {
			case <-stop:
				followed <- pass(&place)
				return
			default:
			}
			if err := pass(&place); err != nil {
				followed <- err
				return
			}
			passes.Add(1)
		}
	}()

	// One save stays open while the others try to commit past it.
	held := make(chan struct{})
	othersDone := make(chan struct{})
	holder := make(chan error, 1)
	go func() {
		holder <- store.Save(ctx, org, func(st *conversations.Step) error {
			if _, err := st.Append(contract.ChangeTick{Kind: "tick"}); err != nil {
				return err
			}
			close(held)
			select {
			case <-othersDone:
				// They got past: let the follower see their numbers first.
				for from := passes.Load(); passes.Load() < from+2; {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(time.Millisecond):
					}
				}
			case <-time.After(holdAtLeast):
			}
			return nil
		})
	}()
	<-held

	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for range writers - 1 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range savesEach {
				if err := save(); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	go func() { wg.Wait(); close(othersDone) }()
	<-othersDone
	if err := <-holder; err != nil {
		t.Fatal(err)
	}
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	close(stop)
	if err := <-followed; err != nil {
		t.Fatal(err)
	}

	var all []int64
	err = orgdb.InOrg(ctx, dir, org, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT number FROM change_log WHERE org = $1 ORDER BY number`, string(org))
		if err != nil {
			return err
		}
		all, err = pgx.CollectRows(rows, pgx.RowTo[int64])
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := 1 + (writers-1)*savesEach; len(all) != want {
		t.Fatalf("the log holds %d changes, expected %d", len(all), want)
	}
	var missed []int64
	for _, n := range all {
		if !seen[n] {
			missed = append(missed, n)
		}
	}
	if len(missed) > 0 {
		t.Fatalf("G-005 broken: the follower stepped over %d committed change(s), first %d. Numbers must commit in order: take the org_log row lock for the number", len(missed), missed[0])
	}
	for i, n := range all {
		if n != int64(i+1) {
			t.Fatalf("G-005 broken: number %d at position %d; the log must have no gaps", n, i+1)
		}
	}
}
