package conversations

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Housekeeping's timings (conversations-to-feed.md, screen-to-conversations.md).
const (
	tickLooksEvery = 500 * time.Millisecond
	tickUntil      = 6 * time.Second // ticks stop once one is stamped this long after the last real change
	leaseRenew     = 5 * time.Second
	leaseValid     = 15 * time.Second
	lockValid      = 15 * time.Second
	tickBatch      = 100
	sweepBatch     = 20
)

// Question is one thing housekeeping asks on its timer, for G-029.
type Question struct {
	Name string
	SQL  string
	Args []any
}

const (
	sqlTicksDue    = `SELECT org FROM org_log WHERE ` + ticksDueWhere + ` ORDER BY tick_due_at LIMIT $2`
	sqlLeasesOut   = `SELECT copy FROM copy_leases WHERE NOT dead AND valid_until < $1 ORDER BY valid_until LIMIT $2`
	sqlDeadCopies  = `SELECT copy FROM copy_leases WHERE dead LIMIT $1`
	sqlOrgsOfACopy = `SELECT org FROM org_log WHERE watch_copies <> '{}' AND watch_copies @> ARRAY[$1]::text[]`
)

// HousekeepingQuestions lists what housekeeping asks across organizations.
func HousekeepingQuestions(now time.Time) []Question {
	return []Question{
		{"who needs a clock tick", sqlTicksDue, []any{now, tickBatch}},
		{"whose lease ran out", sqlLeasesOut, []any{now, sweepBatch}},
		{"which copies are dead", sqlDeadCopies, []any{sweepBatch}},
		{"where a dead copy was watching", sqlOrgsOfACopy, []any{"copy_none"}},
	}
}

// wide is the database for questions that span organizations. The one directory
// function is still the way in (G-028); per-database housekeeping is D-029's step.
func (s *Store) wide(ctx context.Context) (*pgxpool.Pool, error) {
	return s.Dir.PoolFor(ctx, ids.Org(""))
}

// RegisterLease writes this copy's lease.
func (l *Live) RegisterLease(ctx context.Context) error {
	pool, err := l.s.wide(ctx)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `INSERT INTO copy_leases (copy, valid_until) VALUES ($1, $2)
		ON CONFLICT (copy) DO UPDATE SET valid_until = EXCLUDED.valid_until WHERE NOT copy_leases.dead`,
		l.Copy(), l.s.Clock.Now().Add(leaseValid))
	return err
}

// RenewLease extends the lease. A copy that finds itself declared dead takes a
// new identity.
func (l *Live) RenewLease(ctx context.Context) error {
	pool, err := l.s.wide(ctx)
	if err != nil {
		return err
	}
	tag, err := pool.Exec(ctx, `UPDATE copy_leases SET valid_until = $2 WHERE copy = $1 AND NOT dead`,
		l.Copy(), l.s.Clock.Now().Add(leaseValid))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		l.leaseLost()
	}
	return nil
}

// holdLock takes or keeps the one housekeeping lock; it passes on by itself when
// its holder stops renewing it.
func (l *Live) holdLock(ctx context.Context) (bool, error) {
	pool, err := l.s.wide(ctx)
	if err != nil {
		return false, err
	}
	now := l.s.Clock.Now()
	tag, err := pool.Exec(ctx, `UPDATE housekeeping_lock SET holder = $1, until = $2
		WHERE id = 1 AND (holder = $1 OR holder IS NULL OR until < $3)`, l.Copy(), now.Add(lockValid), now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Run keeps this copy's lease and runs housekeeping until ctx ends.
func (l *Live) Run(ctx context.Context) {
	if err := l.RegisterLease(ctx); err != nil {
		slog.Error("register lease", "err", err)
	}
	go l.every(ctx, tickLooksEvery, func() {
		if ok, err := l.holdLock(ctx); err == nil && ok {
			if _, err := l.TickPass(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("clock ticks", "err", err)
			}
		}
	})
	l.every(ctx, checkEvery, func() { l.CheckPass(ctx) })
}

func (l *Live) every(ctx context.Context, d time.Duration, fn func()) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.s.Clock.After(d):
			fn()
		}
	}
}

// CheckPass is the 5-second round: renew the lease, sweep dead copies, retry
// ends that could not be saved, and look at every organization with open screens.
func (l *Live) CheckPass(ctx context.Context) {
	if err := l.RenewLease(ctx); err != nil && ctx.Err() == nil {
		slog.Warn("renew lease", "err", err)
	}
	if ok, err := l.holdLock(ctx); err == nil && ok {
		if err := l.SweepPass(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("sweeping copies", "err", err)
		}
	}
	l.retryEnds(ctx)
	for _, h := range l.allHubs() {
		h.wakeLog()
		h.wakeFeed()
	}
}

// TickPass writes the clock ticks that are due, through the index that holds
// only organizations inside the window, so idle organizations cost nothing.
func (l *Live) TickPass(ctx context.Context) (int, error) {
	pool, err := l.s.wide(ctx)
	if err != nil {
		return 0, err
	}
	now := l.s.Clock.Now()
	rows, err := pool.Query(ctx, sqlTicksDue, now, tickBatch)
	if err != nil {
		return 0, err
	}
	due, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return 0, err
	}
	written := 0
	for _, name := range due {
		org, err := ids.ParseOrg(name)
		if err != nil {
			continue
		}
		wrote, err := l.s.writeTick(ctx, org, now)
		if err != nil {
			if errors.Is(err, ErrOrganizationGone) || errors.Is(err, ErrUnknownOrg) {
				continue
			}
			return written, err
		}
		if wrote {
			written++
			l.nudge(ctx, org)
		}
	}
	return written, nil
}

// writeTick writes one tick and says when the next is due: a second after this
// one, until a tick is stamped at least 6 s after the last change that was not a
// tick (D-034, reading 1). A busy organization still gets its ticks.
func (s *Store) writeTick(ctx context.Context, org ids.Org, now time.Time) (bool, error) {
	wrote := false
	err := s.Save(ctx, org, func(st *Step) error {
		var due, last *time.Time
		if err := st.Tx.QueryRow(ctx, `SELECT tick_due_at, last_change_at FROM org_log WHERE org = $1`, string(org)).Scan(&due, &last); err != nil {
			return err
		}
		if due == nil || due.After(now) {
			return nil // another pass got there first
		}
		if _, err := st.Append(contract.ChangeTick{Kind: "tick"}); err != nil {
			return err
		}
		var next *time.Time
		if last != nil && st.at.Before(last.Add(tickUntil)) {
			t := st.at.Add(tickAfter)
			next = &t
		}
		if _, err := st.Tx.Exec(ctx, `UPDATE org_log SET tick_due_at = $2 WHERE org = $1`, string(org), next); err != nil {
			return err
		}
		wrote = true
		return nil
	})
	return wrote, err
}

// SweepPass declares copies whose lease ran out dead and ends their watchings. A
// sweep that dies halfway is finished by the next, from the rows that remain.
func (l *Live) SweepPass(ctx context.Context) error {
	pool, err := l.s.wide(ctx)
	if err != nil {
		return err
	}
	now := l.s.Clock.Now()
	rows, err := pool.Query(ctx, sqlLeasesOut, now, sweepBatch)
	if err != nil {
		return err
	}
	expired, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, copyID := range expired {
		if _, err := pool.Exec(ctx, `UPDATE copy_leases SET dead = true WHERE copy = $1 AND NOT dead AND valid_until < $2`, copyID, now); err != nil {
			return err
		}
	}
	rows, err = pool.Query(ctx, sqlDeadCopies, sweepBatch)
	if err != nil {
		return err
	}
	dead, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, copyID := range dead {
		if err := l.endCopy(ctx, pool, copyID); err != nil {
			return err
		}
	}
	return nil
}

// endCopy ends every watching a dead copy had started, one organization at a time.
func (l *Live) endCopy(ctx context.Context, pool *pgxpool.Pool, copyID string) error {
	rows, err := pool.Query(ctx, sqlOrgsOfACopy, copyID)
	if err != nil {
		return err
	}
	orgs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, name := range orgs {
		org, err := ids.ParseOrg(name)
		if err != nil {
			continue
		}
		err = l.s.Save(ctx, org, func(st *Step) error {
			rows, err := st.Tx.Query(ctx, `DELETE FROM watching WHERE org = $1 AND copy = $2 RETURNING connection, person, conversation, thread`, string(org), copyID)
			if err != nil {
				return err
			}
			type ended struct {
				connection, person, conversation string
				thread                           *string
			}
			var list []ended
			for rows.Next() {
				var e ended
				if err := rows.Scan(&e.connection, &e.person, &e.conversation, &e.thread); err != nil {
					rows.Close()
					return err
				}
				list = append(list, e)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			for _, e := range list {
				if _, err := st.Append(contract.ChangeReaderWatchingEnded{Kind: "reader.watching_ended", Person: e.person,
					Conversation: e.conversation, Thread: e.thread, Connection: e.connection}); err != nil {
					return err
				}
			}
			_, err = st.Tx.Exec(ctx, `UPDATE org_log SET watch_copies = array_remove(watch_copies, $1) WHERE org = $2`, copyID, string(org))
			return err
		})
		if errors.Is(err, ErrOrganizationGone) || errors.Is(err, ErrUnknownOrg) {
			// Nothing of a retired or deleted organization needs ending.
			if _, err := pool.Exec(ctx, `UPDATE org_log SET watch_copies = array_remove(watch_copies, $1) WHERE org = $2`, copyID, name); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		l.nudge(ctx, org)
	}
	_, err = pool.Exec(ctx, `DELETE FROM copy_leases WHERE copy = $1 AND dead`, copyID)
	return err
}

// nudge tells whichever copy serves the organization that it has news. It carries
// nothing and may be lost: the 5-second check covers it.
func (l *Live) nudge(ctx context.Context, org ids.Org) {
	if h := l.hub(org); h != nil {
		h.wakeLog()
	}
	if l.nudger == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if err := l.nudger.JSON(ctx, "POST", "/orgs/"+string(org)+"/nudge", struct{}{}, nil); err != nil {
			slog.Debug("nudge lost", "org", org, "err", err)
		}
	}()
}
