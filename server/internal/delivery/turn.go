package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"contexia/internal/shared/ids"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// spot is what a turn knows of one organization for this receiver.
type spot struct {
	head, orgEdition     int64
	place, wipe, edition int64
	refusals             int
	paused               bool
}

type entry struct {
	number int64
	kind   string
	body   []byte
}

// after says how a pass ended.
type after int

const (
	again   after = iota // more to do in this turn
	finish               // done for now: give the claim back
	holdOff              // keep the claim, so the organization rests until it runs out
)

// turn serves one claimed organization until it is level, its budget is spent
// or half its claim has passed.
func (l *lane) turn(ctx context.Context, pool *pgxpool.Pool, org ids.Org, failures int) {
	start := l.s.cfg.Clock.Now()
	result := again
	for spent := 0; result == again && spent < turnBudget; spent += batchSize {
		if l.s.cfg.Clock.Now().Sub(start) >= claimFor/2 {
			break
		}
		var err error
		result, err = l.pass(ctx, pool, org, &failures)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("delivery turn failed", "receiver", l.name, "org", org, "err", err)
			}
			result = holdOff
		}
	}
	if result != holdOff {
		l.release(ctx, pool, org)
	}
}

func (l *lane) pass(ctx context.Context, pool *pgxpool.Pool, org ids.Org, failures *int) (after, error) {
	var s spot
	var items []entry
	found, stopped := true, false
	err := l.inOrg(ctx, org, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, haltedSQL, string(org)).Scan(&stopped)
		if err != nil || stopped {
			return err
		}
		s, err = l.load(ctx, tx, org)
		if errors.Is(err, pgx.ErrNoRows) {
			found = false
			return nil
		}
		if err != nil {
			return err
		}
		if (l.notes && s.edition != s.orgEdition) || s.paused || s.place >= s.head {
			return nil
		}
		items, err = l.read(ctx, tx, org, s)
		return err
	})
	if err != nil {
		return holdOff, err
	}
	switch {
	case stopped:
		return finish, l.dropDue(ctx, pool, org)
	case !found || s.paused:
		return finish, nil
	case l.notes && s.edition != s.orgEdition:
		return again, l.newEdition(ctx, pool, org, s)
	case s.place >= s.head:
		return finish, l.removeDue(ctx, pool, org, s.place)
	case len(items) == 0:
		return finish, nil
	}
	if l.name == Feed && fault("shared-place") {
		if held, err := l.heldBySearch(ctx, pool, org, s.place); held || err != nil {
			return holdOff, err
		}
	}
	switch l.name {
	case Feed:
		return l.deliverFeed(ctx, pool, org, s, items, failures)
	case Search:
		return l.deliverSearch(ctx, pool, org, s, items, failures)
	}
	return l.deliverNotes(ctx, pool, org, s, items, failures)
}

func (l *lane) load(ctx context.Context, tx pgx.Tx, org ids.Org) (spot, error) {
	var s spot
	if l.notes {
		var stopped *int64
		err := tx.QueryRow(ctx, `SELECT h.head, h.edition, p.place, p.edition, p.stopped_at
			FROM feed.note_heads h JOIN feed.note_places p ON p.org = h.org WHERE h.org = $1`, string(org)).
			Scan(&s.head, &s.orgEdition, &s.place, &s.edition, &stopped)
		s.paused = stopped != nil
		return s, err
	}
	err := tx.QueryRow(ctx, `SELECT l.head, p.place, p.wipe, p.refusals, p.paused
		FROM org_log l JOIN delivery_places p ON p.org = l.org AND p.receiver = $2 WHERE l.org = $1`, string(org), l.name).
		Scan(&s.head, &s.place, &s.wipe, &s.refusals, &s.paused)
	return s, err
}

func (l *lane) read(ctx context.Context, tx pgx.Tx, org ids.Org, s spot) ([]entry, error) {
	var rows pgx.Rows
	var err error
	if l.notes {
		rows, err = tx.Query(ctx, `SELECT number, kind, body::text FROM feed.note_log
			WHERE org = $1 AND edition = $2 AND number > $3 ORDER BY number LIMIT $4`, string(org), s.edition, s.place, batchSize)
	} else {
		rows, err = tx.Query(ctx, `SELECT number, kind, body::text FROM change_log
			WHERE org = $1 AND number > $2 ORDER BY number LIMIT $3`, string(org), s.place, batchSize)
	}
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (entry, error) {
		var e entry
		var body string
		err := r.Scan(&e.number, &e.kind, &body)
		e.body = []byte(body)
		return e, err
	})
}

// wentDown notes an outage; the place stays and nothing is skipped.
func (l *lane) wentDown(ctx context.Context, pool *pgxpool.Pool, failures *int) (after, error) {
	l.markDown(ctx, pool, *failures)
	return finish, nil
}

func (l *lane) cameBack(ctx context.Context, pool *pgxpool.Pool, failures *int) {
	if *failures > 0 {
		l.markUp(ctx, pool)
		*failures = 0
	}
}

func (l *lane) deliverFeed(ctx context.Context, pool *pgxpool.Pool, org ids.Org, s spot, items []entry, failures *int) (after, error) {
	bodies := make([]json.RawMessage, len(items))
	for i, e := range items {
		bodies[i] = e.body
	}
	if h, err := l.halted(ctx, pool, org); h || err != nil {
		return finish, err
	}
	var o outcome
	if len(bodies) == 1 {
		o = l.rc.post(ctx, "/changes", bodies[0])
	} else {
		o = l.rc.post(ctx, "/changes/batch", map[string]any{"changes": bodies})
	}
	if o.answer == down {
		return l.wentDown(ctx, pool, failures)
	}
	l.cameBack(ctx, pool, failures)
	switch o.answer {
	case applied:
		last := items[len(items)-1].number
		if err := l.note(ctx, pool, org, last, 0, nil); err != nil {
			return holdOff, err
		}
		l.s.nudge(org)
		return again, nil
	case behind:
		if o.iAmAt > s.head {
			return holdOff, nil // the receiver claims more than the log holds: rest, skip nothing
		}
		return again, l.note(ctx, pool, org, o.iAmAt, 0, nil)
	case orgStopped:
		return finish, l.setPaused(ctx, pool, org)
	}
	return l.wentDown(ctx, pool, failures)
}

func (l *lane) deliverSearch(ctx context.Context, pool *pgxpool.Pool, org ids.Org, s spot, items []entry, failures *int) (after, error) {
	progress, refusals := s.place, s.refusals
	save := func() error {
		if progress == s.place && refusals == s.refusals {
			return nil
		}
		return l.note(ctx, pool, org, progress, refusals, nil)
	}
	for _, e := range items {
		body, send, err := forSearch(org, e.number, e.kind, e.body)
		if err != nil {
			return holdOff, err
		}
		if !send {
			progress = e.number
			continue
		}
		if h, err := l.halted(ctx, pool, org); h || err != nil {
			if saveErr := save(); saveErr != nil {
				return holdOff, saveErr
			}
			return finish, err
		}
		o := l.rc.post(ctx, "/changes", body)
		if o.answer == down {
			if err := save(); err != nil {
				return holdOff, err
			}
			return l.wentDown(ctx, pool, failures)
		}
		l.cameBack(ctx, pool, failures)
		if o.hasWipe && o.wipe != s.wipe {
			return again, l.wiped(ctx, pool, org, o.wipe)
		}
		switch o.answer {
		case applied:
			progress, refusals = e.number, 0
		case refusedOne:
			tries, err := l.refused(ctx, pool, org, s, e.number, o.reason)
			if err != nil {
				return holdOff, err
			}
			if tries < triesToPark || fault("no-parking") {
				return finish, save()
			}
			progress, refusals = e.number, refusals+1
			if refusals >= refusalsTo && !fault("no-pause") {
				if err := save(); err != nil {
					return holdOff, err
				}
				return finish, l.setPaused(ctx, pool, org)
			}
		default:
			if err := save(); err != nil {
				return holdOff, err
			}
			return l.wentDown(ctx, pool, failures)
		}
	}
	return again, save()
}

func (l *lane) deliverNotes(ctx context.Context, pool *pgxpool.Pool, org ids.Org, s spot, items []entry, failures *int) (after, error) {
	progress := s.place
	save := func() error {
		if progress == s.place {
			return nil
		}
		return l.note(ctx, pool, org, progress, 0, nil)
	}
	for _, e := range items {
		if h, err := l.halted(ctx, pool, org); h || err != nil {
			if saveErr := save(); saveErr != nil {
				return holdOff, saveErr
			}
			return finish, err
		}
		o := l.rc.post(ctx, "/notes", json.RawMessage(e.body))
		if o.answer == down {
			if err := save(); err != nil {
				return holdOff, err
			}
			return l.wentDown(ctx, pool, failures)
		}
		l.cameBack(ctx, pool, failures)
		switch o.answer {
		case applied:
			progress = e.number
		case behind:
			if err := save(); err != nil {
				return holdOff, err
			}
			if o.edition != s.edition {
				return holdOff, nil // the next turn compares editions with the log
			}
			if o.iAmAt > s.head {
				return holdOff, nil
			}
			return again, l.note(ctx, pool, org, o.iAmAt, 0, nil)
		case refusedOne:
			if err := save(); err != nil {
				return holdOff, err
			}
			tries, err := l.refused(ctx, pool, org, s, e.number, o.reason)
			if err != nil {
				return holdOff, err
			}
			if tries < triesToPark {
				return finish, nil
			}
			if fault("notes-pass") {
				progress = e.number
				continue
			}
			return finish, l.stopNotes(ctx, pool, org, e.number)
		default:
			if err := save(); err != nil {
				return holdOff, err
			}
			return l.wentDown(ctx, pool, failures)
		}
	}
	return again, save()
}

// note moves the place, if this copy still holds the claim. It also forgets
// tries of changes that have since gone through.
func (l *lane) note(ctx context.Context, pool *pgxpool.Pool, org ids.Org, to int64, refusals int, _ *int64) error {
	if l.notes {
		_, err := pool.Exec(ctx, `WITH moved AS (
				UPDATE feed.note_places SET place = $2 WHERE org = $1 AND claimed_by = $3 RETURNING edition
			)
			DELETE FROM feed.note_parked n USING moved
			WHERE n.org = $1 AND n.edition = moved.edition AND n.number <= $2 AND n.tries < $4`,
			string(org), to, l.s.me, triesToPark)
		return err
	}
	_, err := pool.Exec(ctx, `WITH moved AS (
			UPDATE delivery_places SET place = $2, refusals = $5
			WHERE org = $1 AND receiver = $3 AND claimed_by = $4 RETURNING org
		)
		DELETE FROM delivery_parked n USING moved
		WHERE n.org = $1 AND n.receiver = $3 AND n.number <= $2 AND n.tries < $6`,
		string(org), to, l.name, l.s.me, refusals, triesToPark)
	return err
}

// refused counts one more refusal of this change, kept with the place so it
// survives a restart. A row with fewer than three tries is not yet parked.
func (l *lane) refused(ctx context.Context, pool *pgxpool.Pool, org ids.Org, s spot, number int64, reason string) (int, error) {
	var tries int
	now := l.s.cfg.Clock.Now()
	var err error
	if l.notes {
		err = pool.QueryRow(ctx, `INSERT INTO feed.note_parked (org, edition, number, reason, tries, parked_at)
			VALUES ($1, $2, $3, $4, 1, $5)
			ON CONFLICT (org, edition, number) DO UPDATE SET tries = feed.note_parked.tries + 1, reason = $4, parked_at = $5
			RETURNING tries`, string(org), s.edition, number, reason, now).Scan(&tries)
	} else {
		err = pool.QueryRow(ctx, `INSERT INTO delivery_parked (org, receiver, number, reason, tries, parked_at)
			VALUES ($1, $2, $3, $4, 1, $5)
			ON CONFLICT (org, receiver, number) DO UPDATE SET tries = delivery_parked.tries + 1, reason = $4, parked_at = $5
			RETURNING tries`, string(org), l.name, number, reason, now).Scan(&tries)
	}
	return tries, err
}

func (l *lane) setPaused(ctx context.Context, pool *pgxpool.Pool, org ids.Org) error {
	_, err := pool.Exec(ctx, `UPDATE delivery_places SET paused = true WHERE org = $1 AND receiver = $2`, string(org), l.name)
	return err
}

// stopNotes holds this organization's notes at a refused one: a later clock note
// must never let a notification past a withdrawal that is parked.
func (l *lane) stopNotes(ctx context.Context, pool *pgxpool.Pool, org ids.Org, at int64) error {
	_, err := pool.Exec(ctx, `UPDATE feed.note_places SET stopped_at = $2 WHERE org = $1`, string(org), at)
	return err
}

// wiped sends the worker back to the start of a search index that was wiped.
func (l *lane) wiped(ctx context.Context, pool *pgxpool.Pool, org ids.Org, wipe int64) error {
	_, err := pool.Exec(ctx, `UPDATE delivery_places SET place = 0, wipe = $2, refusals = 0
		WHERE org = $1 AND receiver = $3 AND claimed_by = $4`, string(org), wipe, l.name, l.s.me)
	return err
}

// newEdition restarts the notes at 0 of the edition the feed now writes.
func (l *lane) newEdition(ctx context.Context, pool *pgxpool.Pool, org ids.Org, s spot) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE feed.note_places SET edition = $2, place = 0, stopped_at = NULL
			WHERE org = $1 AND claimed_by = $3`, string(org), s.orgEdition, l.s.me); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM feed.note_parked WHERE org = $1 AND edition < $2`, string(org), s.orgEdition)
		return err
	})
}

// release gives the claim back. The time stays, as "served then", for turns.
func (l *lane) release(ctx context.Context, pool *pgxpool.Pool, org ids.Org) {
	now := l.s.cfg.Clock.Now()
	var err error
	if l.notes {
		_, err = pool.Exec(ctx, `UPDATE feed.note_places SET claimed_by = NULL, claimed_until = $3 WHERE org = $1 AND claimed_by = $2`,
			string(org), l.s.me, now)
	} else {
		_, err = pool.Exec(ctx, `UPDATE delivery_places SET claimed_by = NULL, claimed_until = $4
			WHERE org = $1 AND receiver = $2 AND claimed_by = $3`, string(org), l.name, l.s.me, now)
	}
	if err != nil && ctx.Err() == nil {
		slog.Warn("delivery cannot give a claim back", "receiver", l.name, "org", org, "err", err)
	}
}

// heldBySearch is the planted fault of G-012: the feed may not pass search.
func (l *lane) heldBySearch(ctx context.Context, pool *pgxpool.Pool, org ids.Org, at int64) (bool, error) {
	var searchAt int64
	err := pool.QueryRow(ctx, `SELECT place FROM delivery_places WHERE org = $1 AND receiver = 'search'`, string(org)).Scan(&searchAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	return at >= searchAt, err
}
