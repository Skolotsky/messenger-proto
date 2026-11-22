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

// haltedSQL is the one question every copy asks before every receiver call. A
// retired organization is stopped whether or not anyone called stop (D-033). Both
// halves read one row by key, so idle organizations cost nothing (G-029).
const haltedSQL = `SELECT EXISTS (SELECT 1 FROM delivery_stopped WHERE org = $1)
	OR EXISTS (SELECT 1 FROM org_log WHERE org = $1 AND retired_at IS NOT NULL)`

// halted says whether nothing more may be delivered for org.
func (l *lane) halted(ctx context.Context, pool *pgxpool.Pool, org ids.Org) (bool, error) {
	var h bool
	err := pool.QueryRow(ctx, haltedSQL, string(org)).Scan(&h)
	return h, err
}

// liveClaims counts the claims any copy holds on org. A copy reads the mark before
// each call and keeps its claim through the call, so with the mark committed and
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
// copy can still deliver, or with ErrStillDelivering. The work list loses the
// organization, so finding work does not look at it again (G-029). Repeating it
// changes nothing. The wait is a pace, like the lanes' poll, so it is not on the
// injected clock: a test clock must not hold up a stop.
func (s *Service) Stop(ctx context.Context, org ids.Org) error {
	err := orgdb.InOrg(ctx, s.cfg.DB, org, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO delivery_stopped (org) VALUES ($1) ON CONFLICT DO NOTHING`, string(org)); err != nil {
			return err
		}
		return orgwipe.Delete(ctx, tx, org, "delivery_due", "feed.note_due")
	})
	if err != nil {
		return err
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
// still deliver for it. Repeating it changes nothing.
func (s *Service) DeleteOrg(ctx context.Context, org ids.Org) error {
	if err := s.Stop(ctx, org); err != nil {
		return err
	}
	err := orgdb.InOrg(ctx, s.cfg.DB, org, func(tx pgx.Tx) error {
		return orgwipe.Delete(ctx, tx, org, ownTables...)
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
	case err == nil:
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
