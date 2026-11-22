package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	"contexia/internal/shared/orgwipe"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ownTables are the delivery workers' rows of an organization, also those kept
// beside the feed's notes.
var ownTables = []string{
	"delivery_due", "delivery_parked", "delivery_places", "delivery_stopped",
	"feed.note_due", "feed.note_parked", "feed.note_places",
}

// stopWait is how long a stop waits for other copies to let go before telling
// the caller to ask again. It stays inside the 3 s limit on calls between parts.
const stopWait = 2 * time.Second

// ErrStillDelivering says a copy may still be inside a call for the organization.
// The stop is recorded; asking again is safe.
var ErrStillDelivering = errors.New("a delivery copy still holds this organization")

// errNoOrganization says the organization has no log row: it was deleted, or
// never existed.
var errNoOrganization = errors.New("no such organization")

// haltedSQL says that nothing may be delivered for an organization: it was told
// to stop, is retired (D-033), or has no log row, because a row that is gone means
// halted and never "go on". Each half reads one row by key, so idle organizations
// cost nothing (G-029).
const haltedSQL = `SELECT EXISTS (SELECT 1 FROM delivery_stopped WHERE org = $1)
	OR NOT EXISTS (SELECT 1 FROM org_log WHERE org = $1 AND retired_at IS NULL)`

// holdsClaim runs before every call to a receiver. In one statement it confirms
// this copy still holds a live claim and may deliver, and renews the claim for
// longer than a call can last, so a live claim covers any call in flight and
// Stop can wait for claims to clear. False means end the turn. The check on the
// row's own halted flag is what a renewal racing Stop rereads once Stop has
// updated the row; the other conditions come from the statement's snapshot.
func (l *lane) holdsClaim(ctx context.Context, pool *pgxpool.Pool, org ids.Org) (bool, error) {
	if fault("no-claim") { // the planted fault of G-027: claims keep nobody out
		var h bool
		err := pool.QueryRow(ctx, haltedSQL, string(org)).Scan(&h)
		return !h, err
	}
	now := l.s.cfg.Clock.Now()
	const allowed = ` AND NOT p.halted
		AND NOT EXISTS (SELECT 1 FROM delivery_stopped x WHERE x.org = p.org)
		AND EXISTS (SELECT 1 FROM org_log o WHERE o.org = p.org AND o.retired_at IS NULL)`
	var sql string
	args := []any{string(org), l.s.me, now, now.Add(l.s.cfg.ClaimFor)}
	if l.notes {
		sql = `UPDATE feed.note_places p SET claimed_until = $4
			WHERE p.org = $1 AND p.claimed_by = $2 AND p.claimed_until >= $3` + allowed
	} else {
		sql = `UPDATE delivery_places p SET claimed_until = $4
			WHERE p.org = $1 AND p.receiver = $5 AND p.claimed_by = $2 AND p.claimed_until >= $3` + allowed
		args = append(args, l.name)
	}
	tag, err := pool.Exec(ctx, sql, args...)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// liveClaims counts the claims any copy holds on org. A copy renews its claim
// before each call, for longer than a call lasts, so with every renewal ended and
// no live claim, no copy can still deliver. A claim of a copy that died runs out.
func (s *Service) liveClaims(ctx context.Context, org ids.Org) (int, error) {
	pool, err := s.cfg.DB.PoolFor(ctx, org)
	if err != nil {
		return 0, err
	}
	var n int
	err = pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM delivery_places WHERE org = $1 AND claimed_by IS NOT NULL AND claimed_until >= $2) +
		(SELECT count(*) FROM feed.note_places WHERE org = $1 AND claimed_by IS NOT NULL AND claimed_until >= $2)`,
		string(org), s.cfg.Clock.Now()).Scan(&n)
	return n, err
}

// Stop ends delivery for an organization on every copy. It answers only once no
// copy can still deliver, or with ErrStillDelivering, and creates nothing for an
// organization with no log row. The work list loses the organization, so finding
// work does not look at it again (G-029). Repeating it changes nothing. The wait
// is a pace, like the lanes' poll, so it is not on the injected clock.
func (s *Service) Stop(ctx context.Context, org ids.Org) error {
	err := orgdb.InOrg(ctx, s.cfg.DB, org, func(tx pgx.Tx) error {
		var known bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM org_log WHERE org = $1)`, string(org)).Scan(&known); err != nil {
			return err
		}
		if !known {
			return errNoOrganization
		}
		if _, err := tx.Exec(ctx, `INSERT INTO delivery_stopped (org) VALUES ($1) ON CONFLICT DO NOTHING`, string(org)); err != nil {
			return err
		}
		return orgwipe.Delete(ctx, tx, org, "delivery_due", "feed.note_due")
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" { // the log row went while this ran
		return errNoOrganization
	}
	if err != nil {
		return err
	}
	// After the mark has committed: a renewal that got to the row first is counted
	// below, and one that comes later rereads this flag and is refused.
	pool, err := s.cfg.DB.PoolFor(ctx, org)
	if err != nil {
		return err
	}
	for _, table := range []string{"delivery_places", "feed.note_places"} {
		if _, err := pool.Exec(ctx, `UPDATE `+table+` SET halted = true WHERE org = $1 AND NOT halted`, string(org)); err != nil {
			return err
		}
	}
	limit := time.NewTimer(stopWait)
	defer limit.Stop()
	tick := time.NewTicker(pollEvery)
	defer tick.Stop()
	for {
		n, err := s.liveClaims(ctx, org)
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
		select {
		case <-tick.C:
		case <-limit.C:
			return ErrStillDelivering
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// DeleteOrg removes every delivery row of the organization, once no copy can
// still deliver for it, and notes on the log row that they are gone, which is
// what lets Conversations delete last. Repeating it changes nothing, also when
// the organization is already gone.
func (s *Service) DeleteOrg(ctx context.Context, org ids.Org) error {
	if err := s.Stop(ctx, org); err != nil && !errors.Is(err, errNoOrganization) {
		return err
	}
	now := s.cfg.Clock.Now()
	err := orgdb.InOrg(ctx, s.cfg.DB, org, func(tx pgx.Tx) error {
		if err := orgwipe.Delete(ctx, tx, org, ownTables...); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE org_log SET delivery_cleared_at = $2 WHERE org = $1 AND retired_at IS NOT NULL`, string(org), now)
		return err
	})
	if err != nil {
		return err
	}
	for _, l := range s.lanes {
		l.forget(org)
	}
	return nil
}

// forget drops a recheck of an organization that no longer exists.
func (l *lane) forget(org ids.Org) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k := range l.recheck {
		if k.org == org {
			delete(l.recheck, k)
		}
	}
}

// dropDue takes a stopped organization off this lane's work list, in case
// something put it back after the stop.
func (l *lane) dropDue(ctx context.Context, pool *pgxpool.Pool, org ids.Org) error {
	var err error
	if l.notes {
		_, err = pool.Exec(ctx, `DELETE FROM feed.note_due WHERE org = $1`, string(org))
	} else {
		_, err = pool.Exec(ctx, `DELETE FROM delivery_due WHERE org = $1 AND receiver = $2`, string(org), l.name)
	}
	return err
}

// answerStop is the answer of both calls that wait for the other copies.
func answerStop(w http.ResponseWriter, err error) {
	switch {
	// An organization already gone is stopped: nothing was created, and a delete
	// repeated from its first step must get through this one (G-031).
	case err == nil, errors.Is(err, errNoOrganization):
		answerJSON(w, struct{}{})
	case errors.Is(err, ErrStillDelivering):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(contract.NotAvailable{NotAvailable: "a delivery copy is still finishing; ask again"})
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Service) handleStop(w http.ResponseWriter, r *http.Request, org ids.Org) {
	answerStop(w, s.Stop(r.Context(), org))
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request, org ids.Org) {
	answerStop(w, s.DeleteOrg(r.Context(), org))
}
