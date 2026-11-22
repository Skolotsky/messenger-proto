package delivery

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// lane delivers one receiver's work.
type lane struct {
	s     *Service
	name  string
	notes bool // reads the feed's note log instead of the change log
	rc    *client

	mu      sync.Mutex
	recheck map[recheck]int64 // the transaction ID every older one must have ended by
}

// recheck is a removed due row to look at again once every transaction that
// was running when it was removed has ended. Such a transaction may have saved
// a change and found the old row still there, so added none of its own.
type recheck struct {
	pool *pgxpool.Pool
	org  ids.Org
}

func (l *lane) stateKey() string { return l.name }

// run is the lane's loop. The poll is a pace, not a measure of time, so it is
// the one wait not on the injected clock: a test clock must not stop delivery.
func (l *lane) run(ctx context.Context, pool *pgxpool.Pool) {
	free := make(chan struct{}, slots)
	for range slots {
		free <- struct{}{}
	}
	var running sync.WaitGroup
	defer running.Wait()
	tick := time.NewTicker(pollEvery)
	defer tick.Stop()
	var lastErr time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		orgs, failures, err := l.findWork(ctx, pool, len(free))
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if now := l.s.cfg.Clock.Now(); lastErr.IsZero() || now.Sub(lastErr) > time.Minute {
				slog.Warn("delivery cannot read its work", "receiver", l.name, "err", err)
				lastErr = now
			}
			continue
		}
		for _, org := range orgs {
			<-free
			running.Add(1)
			go func() {
				defer running.Done()
				defer func() { free <- struct{}{} }()
				l.turn(ctx, pool, org, failures)
			}()
		}
		l.runRechecks(ctx, pool)
	}
}

// findWork claims organizations for this copy. While the receiver is down it
// claims nothing until the shared retry time, and then one, as the probe.
func (l *lane) findWork(ctx context.Context, pool *pgxpool.Pool, room int) ([]ids.Org, int, error) {
	if room == 0 {
		return nil, 0, nil
	}
	now := l.s.cfg.Clock.Now()
	failures, retryAt, err := l.receiverState(ctx, pool)
	if err != nil {
		return nil, 0, err
	}
	if failures > 0 && !fault("retry-storm") {
		if retryAt != nil && retryAt.After(now) {
			return nil, failures, nil
		}
		won, err := l.takeProbe(ctx, pool, failures, now)
		if err != nil || !won {
			return nil, failures, err
		}
		room = 1
		orgs, err := l.claim(ctx, pool, room, now)
		if err == nil && len(orgs) == 0 {
			// Nothing to carry the try: ask whether it is back, or it would
			// read as down until the next change.
			if l.rc.alive(ctx) {
				l.markUp(ctx, pool)
			} else {
				l.markDown(ctx, pool, failures)
			}
		}
		return orgs, failures, err
	}
	orgs, err := l.claim(ctx, pool, room, now)
	if err != nil || len(orgs) == 0 || fault("retry-storm") {
		return orgs, failures, err
	}
	// Look again now the claims are held. A copy notes an outage before giving
	// its claim back, so whoever takes that organization over sees the outage
	// here and does not add a try of its own to the shared wait.
	again, _, err := l.receiverState(ctx, pool)
	if err != nil || again > 0 {
		for _, org := range orgs {
			l.release(ctx, pool, org)
		}
		return nil, again, err
	}
	return orgs, failures, nil
}

func (l *lane) receiverState(ctx context.Context, pool *pgxpool.Pool) (int, *time.Time, error) {
	var failures int
	var retryAt *time.Time
	err := pool.QueryRow(ctx, `SELECT failures, retry_at FROM delivery_receivers WHERE receiver = $1`, l.stateKey()).Scan(&failures, &retryAt)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = pool.Exec(ctx, `INSERT INTO delivery_receivers (receiver) VALUES ($1) ON CONFLICT DO NOTHING`, l.stateKey())
		return 0, nil, err
	}
	return failures, retryAt, err
}

// takeProbe lets one copy try a receiver that is down, so more workers do not
// mean more retries.
func (l *lane) takeProbe(ctx context.Context, pool *pgxpool.Pool, failures int, now time.Time) (bool, error) {
	tag, err := pool.Exec(ctx, `UPDATE delivery_receivers SET retry_at = $1
		WHERE receiver = $2 AND failures = $3 AND (retry_at IS NULL OR retry_at <= $4)`,
		now.Add(probeFor), l.stateKey(), failures, now)
	return err == nil && tag.RowsAffected() == 1, err
}

// markDown records one failed try. Calls that failed together count once.
func (l *lane) markDown(ctx context.Context, pool *pgxpool.Pool, failures int) {
	now := l.s.cfg.Clock.Now()
	_, err := pool.Exec(ctx, `UPDATE delivery_receivers
		SET failures = failures + 1, down_since = coalesce(down_since, $1), retry_at = $2
		WHERE receiver = $3 AND failures = $4`,
		now, now.Add(l.s.backoff(failures+1)), l.stateKey(), failures)
	if err != nil && ctx.Err() == nil {
		slog.Warn("delivery cannot note an outage", "receiver", l.name, "err", err)
	}
}

func (l *lane) markUp(ctx context.Context, pool *pgxpool.Pool) {
	_, err := pool.Exec(ctx, `UPDATE delivery_receivers SET failures = 0, down_since = NULL, retry_at = NULL
		WHERE receiver = $1 AND failures > 0`, l.stateKey())
	if err != nil && ctx.Err() == nil {
		slog.Warn("delivery cannot note a recovery", "receiver", l.name, "err", err)
	}
}

// claim takes up to room organizations: most of them ones a single turn will
// clear, and at least one far behind, so fresh traffic never waits behind a wave
// of catching up and catching up never starves (D-029). Within each group the
// one served longest ago goes first.
func (l *lane) claim(ctx context.Context, pool *pgxpool.Pool, room int, now time.Time) ([]ids.Org, error) {
	if err := l.addPlaces(ctx, pool); err != nil {
		return nil, err
	}
	if fault("arrival-order") {
		if room < slots { // one organization at a time, as the fault promises
			return nil, nil
		}
		return l.claimOldest(ctx, pool, now)
	}
	far := max(1, room/4)
	orgs, err := l.claimGroup(ctx, pool, now, true, far)
	if err != nil {
		return nil, err
	}
	near, err := l.claimGroup(ctx, pool, now, false, room-len(orgs))
	if err != nil {
		return orgs, err
	}
	return append(orgs, near...), nil
}

// addPlaces gives every organization with work a place. It starts at 0 and the
// first answer corrects it.
func (l *lane) addPlaces(ctx context.Context, pool *pgxpool.Pool) error {
	sql, args := l.placesQuestion()
	_, err := pool.Exec(ctx, sql, args...)
	return err
}

func (l *lane) placesQuestion() (string, []any) {
	if l.notes {
		return `INSERT INTO feed.note_places (org)
			SELECT d.org FROM feed.note_due d
			WHERE NOT EXISTS (SELECT 1 FROM feed.note_places p WHERE p.org = d.org OFFSET 0)
			ON CONFLICT DO NOTHING`, nil
	}
	return `INSERT INTO delivery_places (org, receiver)
			SELECT d.org, d.receiver FROM delivery_due d
			WHERE d.receiver = $1
			  AND NOT EXISTS (SELECT 1 FROM delivery_places p WHERE p.org = d.org AND p.receiver = d.receiver OFFSET 0)
			ON CONFLICT DO NOTHING`, []any{l.name}
}

// The queries below start from the due table and reach everything else per row
// (LATERAL with OFFSET 0, which the planner cannot turn back into a join of whole
// tables), so a plan can never read the organizations that have no work. The
// claim is tested again on the locked row: what "wanted" saw may be out of date.
func (l *lane) claimGroup(ctx context.Context, pool *pgxpool.Pool, now time.Time, far bool, limit int) ([]ids.Org, error) {
	if limit <= 0 {
		return nil, nil
	}
	sql, args := l.claimQuestion(now, far, limit)
	return collectOrgs(ctx, pool, sql, args...)
}

func (l *lane) claimQuestion(now time.Time, far bool, limit int) (string, []any) {
	cmp := "<="
	if far {
		cmp = ">"
	}
	free := `(p.claimed_by IS NULL OR p.claimed_until < $2)`
	if fault("no-claim") {
		free = `($2::timestamptz IS NOT NULL)`
	}
	if fault("scan-all") { // the planted fault of G-029: read every place first
		table := "delivery_places"
		if l.notes {
			table = "feed.note_places"
		}
		free += ` AND (SELECT count(*) FROM ` + table + `) >= 0`
	}
	var sql string
	var args []any
	if l.notes {
		sql = `WITH wanted AS (
			SELECT d.org FROM feed.note_due d
			CROSS JOIN LATERAL (SELECT head FROM feed.note_heads h WHERE h.org = d.org OFFSET 0) h
			CROSS JOIN LATERAL (SELECT place, claimed_by, claimed_until, stopped_at FROM feed.note_places p WHERE p.org = d.org OFFSET 0) p
			WHERE p.stopped_at IS NULL AND ` + free + ` AND h.head - p.place ` + cmp + ` $3
			ORDER BY p.claimed_until NULLS FIRST LIMIT $4
		), locked AS (
			SELECT x.org FROM wanted w CROSS JOIN LATERAL (
				SELECT p.org FROM feed.note_places p WHERE p.org = w.org AND ` + free + ` FOR UPDATE SKIP LOCKED
			) x
		)
		UPDATE feed.note_places p SET claimed_by = $1, claimed_until = $5
		FROM locked WHERE p.org = locked.org RETURNING p.org`
		args = []any{l.s.me, now, turnBudget, limit, now.Add(claimFor)}
	} else {
		sql = `WITH wanted AS (
			SELECT d.org FROM delivery_due d
			CROSS JOIN LATERAL (SELECT head FROM org_log h WHERE h.org = d.org OFFSET 0) h
			CROSS JOIN LATERAL (SELECT place, paused, claimed_by, claimed_until FROM delivery_places p
				WHERE p.org = d.org AND p.receiver = d.receiver OFFSET 0) p
			WHERE d.receiver = $6 AND NOT p.paused AND ` + free + ` AND h.head - p.place ` + cmp + ` $3
			ORDER BY p.claimed_until NULLS FIRST LIMIT $4
		), locked AS (
			SELECT x.org FROM wanted w CROSS JOIN LATERAL (
				SELECT p.org FROM delivery_places p WHERE p.org = w.org AND p.receiver = $6 AND ` + free + ` FOR UPDATE SKIP LOCKED
			) x
		)
		UPDATE delivery_places p SET claimed_by = $1, claimed_until = $5
		FROM locked WHERE p.org = locked.org AND p.receiver = $6 RETURNING p.org`
		args = []any{l.s.me, now, turnBudget, limit, now.Add(claimFor), l.name}
	}
	return sql, args
}

// Question is one thing a lane asks the database on its timer.
type Question struct {
	Name string
	SQL  string
	Args []any
}

// Questions lists what the lanes ask on their timer, so G-029 can read the
// database's plan for each.
func (s *Service) Questions(now time.Time) []Question {
	var out []Question
	for _, name := range []string{Search, Feed, Sender} {
		l := s.lanes[name]
		near, nearArgs := l.claimQuestion(now, false, slots)
		far, farArgs := l.claimQuestion(now, true, slots)
		out = append(out,
			Question{name + ": organizations one turn clears", near, nearArgs},
			Question{name + ": organizations far behind", far, farArgs})
		places, placeArgs := l.placesQuestion()
		out = append(out, Question{name + ": give new work a place", places, placeArgs})
	}
	return out
}

// claimOldest is the planted fault of G-026: one organization at a time, in
// order of arrival, until it has nothing left.
func (l *lane) claimOldest(ctx context.Context, pool *pgxpool.Pool, now time.Time) ([]ids.Org, error) {
	due, places, extra := "delivery_due", "delivery_places", " AND p.receiver = $4 AND d.receiver = $4"
	args := []any{l.s.me, now, now.Add(claimFor), l.name}
	if l.notes {
		due, places, extra = "feed.note_due", "feed.note_places", ""
		args = args[:3]
	}
	return collectOrgs(ctx, pool, `UPDATE `+places+` u SET claimed_by = $1, claimed_until = $3
		WHERE u.org = (SELECT d.org FROM `+due+` d JOIN `+places+` p ON p.org = d.org
			WHERE (p.claimed_by IS NULL OR p.claimed_until < $2)`+extra+`
			ORDER BY d.since, d.org LIMIT 1)`+replaceReceiver(l.notes)+` RETURNING u.org`, args...)
}

func replaceReceiver(notes bool) string {
	if notes {
		return ""
	}
	return " AND u.receiver = $4"
}

func collectOrgs(ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) ([]ids.Org, error) {
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	out := make([]ids.Org, len(names))
	for i, n := range names {
		out[i] = ids.Org(n)
	}
	return out, nil
}

// removeDue takes an organization off the due table once level, skipping a row
// a save holds right now. A save that found the row before it went, and has not
// committed, added none of its own; runRechecks catches it.
func (l *lane) removeDue(ctx context.Context, pool *pgxpool.Pool, org ids.Org, at int64) error {
	var sql string
	var args []any
	if l.notes {
		sql = `WITH mine AS (SELECT org FROM feed.note_due WHERE org = $1 FOR UPDATE SKIP LOCKED)
			DELETE FROM feed.note_due d USING mine
			WHERE d.org = mine.org AND (SELECT head FROM feed.note_heads h WHERE h.org = $1) <= $2`
		args = []any{string(org), at}
	} else {
		sql = `WITH mine AS (SELECT org FROM delivery_due WHERE receiver = $3 AND org = $1 FOR UPDATE SKIP LOCKED)
			DELETE FROM delivery_due d USING mine
			WHERE d.receiver = $3 AND d.org = mine.org AND (SELECT head FROM org_log h WHERE h.org = $1) <= $2`
		args = []any{string(org), at, l.name}
	}
	tag, err := pool.Exec(ctx, sql, args...)
	if err != nil || tag.RowsAffected() == 0 || fault("eager-remove") {
		return err
	}
	// Read after the delete committed: any save that saw the old row has an
	// ID below this.
	var xmax int64
	if err := pool.QueryRow(ctx, `SELECT pg_snapshot_xmax(pg_current_snapshot())::text::bigint`).Scan(&xmax); err != nil {
		return err
	}
	l.mu.Lock()
	if l.recheck == nil {
		l.recheck = map[recheck]int64{}
	}
	l.recheck[recheck{pool, org}] = xmax
	l.mu.Unlock()
	return nil
}

// runRechecks puts back the due row of any organization whose head moved past
// its place while its row was being removed.
func (l *lane) runRechecks(ctx context.Context, pool *pgxpool.Pool) {
	l.mu.Lock()
	pending := make(map[ids.Org]int64)
	for k, xmax := range l.recheck {
		if k.pool == pool {
			pending[k.org] = xmax
		}
	}
	l.mu.Unlock()
	if len(pending) == 0 {
		return
	}
	var xmin int64
	if err := pool.QueryRow(ctx, `SELECT pg_snapshot_xmin(pg_current_snapshot())::text::bigint`).Scan(&xmin); err != nil {
		return
	}
	for org, xmax := range pending {
		if xmax > xmin {
			continue
		}
		var err error
		if l.notes {
			_, err = pool.Exec(ctx, `INSERT INTO feed.note_due (org, since)
				SELECT h.org, $2::timestamptz FROM feed.note_heads h JOIN feed.note_places p ON p.org = h.org
				WHERE h.org = $1 AND h.head > p.place ON CONFLICT DO NOTHING`, string(org), l.s.cfg.Clock.Now())
		} else {
			_, err = pool.Exec(ctx, `INSERT INTO delivery_due (receiver, org, since)
				SELECT $3, h.org, $2::timestamptz FROM org_log h JOIN delivery_places p ON p.org = h.org AND p.receiver = $3
				WHERE h.org = $1 AND h.head > p.place ON CONFLICT DO NOTHING`, string(org), l.s.cfg.Clock.Now(), l.name)
		}
		if err != nil {
			continue
		}
		l.mu.Lock()
		if l.recheck[recheck{pool, org}] == xmax { // not removed again meanwhile
			delete(l.recheck, recheck{pool, org})
		}
		l.mu.Unlock()
	}
}

// inOrg is the organization-bound transaction; everything of one organization's
// log is read through it (G-025).
func (l *lane) inOrg(ctx context.Context, org ids.Org, fn func(pgx.Tx) error) error {
	return orgdb.InOrg(ctx, l.s.cfg.DB, org, fn)
}
