package delivery

import (
	"context"
	"net/http"

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

// enter notes a turn starting; false means the organization was stopped.
func (s *Service) enter(org ids.Org) bool {
	s.turnsMu.Lock()
	defer s.turnsMu.Unlock()
	if s.stopped[org] {
		return false
	}
	s.turning[org]++
	return true
}

func (s *Service) leave(org ids.Org) {
	s.turnsMu.Lock()
	defer s.turnsMu.Unlock()
	s.turning[org]--
	if s.turning[org] > 0 {
		return
	}
	delete(s.turning, org)
	for _, c := range s.idle[org] {
		close(c)
	}
	delete(s.idle, org)
}

func (s *Service) isStopped(org ids.Org) bool {
	s.turnsMu.Lock()
	defer s.turnsMu.Unlock()
	return s.stopped[org]
}

// haltTurns makes this copy's turns for org end before their next call, and
// waits for those under way. A call lasts at most 3 s, so the wait is bounded.
func (s *Service) haltTurns(ctx context.Context, org ids.Org) error {
	s.turnsMu.Lock()
	s.stopped[org] = true
	if s.turning[org] == 0 {
		s.turnsMu.Unlock()
		return nil
	}
	done := make(chan struct{})
	s.idle[org] = append(s.idle[org], done)
	s.turnsMu.Unlock()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop ends delivery for an organization: the mark is what every copy reads at
// the start of each pass, and the work list loses it so finding work never
// looks at it again (G-029). Repeating it changes nothing.
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
	return s.haltTurns(ctx, org)
}

// DeleteOrg removes every delivery row of the organization. Repeating it changes
// nothing.
func (s *Service) DeleteOrg(ctx context.Context, org ids.Org) error {
	if err := s.haltTurns(ctx, org); err != nil {
		return err
	}
	err := orgdb.InOrg(ctx, s.cfg.DB, org, func(tx pgx.Tx) error {
		return orgwipe.Delete(ctx, tx, org, ownTables...)
	})
	if err != nil {
		return err
	}
	s.turnsMu.Lock()
	delete(s.stopped, org)
	s.turnsMu.Unlock()
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

func (s *Service) handleStop(w http.ResponseWriter, r *http.Request, org ids.Org) {
	if err := s.Stop(r.Context(), org); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	answerJSON(w, struct{}{})
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request, org ids.Org) {
	if err := s.DeleteOrg(r.Context(), org); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	answerJSON(w, struct{}{})
}
