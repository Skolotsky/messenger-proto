package conversations

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"
	"contexia/internal/shared/orgwipe"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrOrganizationGone says the organization is retired (D-033).
var ErrOrganizationGone = errors.New("organization gone")

// ErrNotRetired refuses deleting an organization that is still in use.
var ErrNotRetired = errors.New("organization not retired")

// errDeliveryRemains says delivery still holds rows that point at this log.
var errDeliveryRemains = errors.New("delivery not deleted")

// ownTables are Conversations' tables, children before what they point at; the
// log and its row come last. Delivery's tables point at the log row, so its
// delete has to have run first.
var ownTables = []string{
	"reactions", "conversation_people", "messages", "conversations", "people",
	"reader_reads", "reader_kept", "reader_dials", "reader_focus_rules", "reader_focus_people",
	"reader_settings", "reader_preferences", "watching", "action_answers", "seed_sessions",
	"change_log", "org_log",
}

// Retired says whether the organization is retired. It is the one question every
// door asks: the people door through door below; the agent door (T-054) and the
// live connection (T-053) call it too, and answer with WriteGone or close with
// the same message.
func (s *Store) Retired(ctx context.Context, org ids.Org) (bool, error) {
	var retired bool
	err := orgdb.InOrg(ctx, s.Dir, org, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT retired_at IS NOT NULL FROM org_log WHERE org = $1`, string(org)).Scan(&retired)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
	return retired, err
}

// WriteGone is a retired organization's answer on every door.
func WriteGone(w http.ResponseWriter) {
	send(w, http.StatusGone, contract.OrganizationGone{OrganizationGone: true})
}

// door refuses every call to a retired organization before reading it. A save
// checks again under the log's lock, so one that raced the retirement is refused
// as well.
func (s *Store) door(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, ok := orgOf(w, r)
		if !ok {
			return
		}
		retired, err := s.Retired(r.Context(), org)
		if err != nil {
			fail(w, r, err)
			return
		}
		if retired {
			WriteGone(w)
			return
		}
		next(w, r)
	}
}

// Retire stops the organization accepting anything. Repeating it changes
// nothing, and so does retiring one that was already deleted: a delete that
// crashed halfway starts again from this step. It takes the log's lock, so
// every save either committed before it or is refused after it.
func (s *Store) Retire(ctx context.Context, org ids.Org) error {
	now := s.Clock.Now().UTC()
	err := orgdb.InOrg(ctx, s.Dir, org, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE org_log SET retired_at = coalesce(retired_at, $2), tick_due_at = NULL WHERE org = $1`,
			string(org), now)
		return err
	})
	if err == nil && s.OnSaved != nil {
		s.OnSaved(org, 0, 0) // its open screens are told at once
	}
	return err
}

// DeleteOrg removes every row of a retired organization from Conversations'
// tables, in one transaction. Repeating it changes nothing.
func (s *Store) DeleteOrg(ctx context.Context, org ids.Org) error {
	err := orgdb.InOrg(ctx, s.Dir, org, func(tx pgx.Tx) error {
		var retired *time.Time
		err := tx.QueryRow(ctx, `SELECT retired_at FROM org_log WHERE org = $1 FOR UPDATE`, string(org)).Scan(&retired)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // already deleted
		}
		if err != nil {
			return err
		}
		if retired == nil {
			return ErrNotRetired
		}
		return orgwipe.Delete(ctx, tx, org, ownTables...)
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" {
		return errDeliveryRemains
	}
	return err
}

func (s *Store) handleRetire(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	if err := s.Retire(r.Context(), org); err != nil {
		fail(w, r, err)
		return
	}
	send(w, http.StatusOK, contract.Empty{})
}

func (s *Store) handleDelete(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	switch err := s.DeleteOrg(r.Context(), org); {
	case err == nil:
		send(w, http.StatusOK, contract.Empty{})
	case errors.Is(err, ErrNotRetired):
		send(w, http.StatusConflict, contract.Refusal{Refused: "not retired"})
	case errors.Is(err, errDeliveryRemains):
		send(w, http.StatusConflict, contract.Refusal{Refused: "delivery not deleted"})
	default:
		slog.Error("delete organization", "org", org, "err", err)
		send(w, http.StatusInternalServerError, map[string]string{"error": "try again"})
	}
}
