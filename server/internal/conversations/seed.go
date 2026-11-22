package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const refusedNotEmpty = "not empty"

// SeedRefused is a seed call the contract answers with a refusal.
type SeedRefused struct{ Reason string }

func (e SeedRefused) Error() string { return "seed refused: " + e.Reason }

// SeedOpen opens the one seed session of an organization whose log is empty.
// It is also what first creates the organization's log row. Asking again while
// it is still empty and open gives the same session.
func (s *Store) SeedOpen(ctx context.Context, org ids.Org) (string, error) {
	var session string
	err := orgdb.InOrg(ctx, s.Dir, org, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO org_log (org) VALUES ($1) ON CONFLICT DO NOTHING`, string(org)); err != nil {
			return err
		}
		var head int64
		if err := tx.QueryRow(ctx, `SELECT head FROM org_log WHERE org = $1 FOR UPDATE`, string(org)).Scan(&head); err != nil {
			return err
		}
		if head != 0 {
			return SeedRefused{refusedNotEmpty}
		}
		var sealed bool
		err := tx.QueryRow(ctx, `SELECT session, sealed FROM seed_sessions WHERE org = $1`, string(org)).Scan(&session, &sealed)
		if err == nil {
			if sealed {
				return SeedRefused{refusedNotEmpty}
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		session = ids.New("seed")
		_, err = tx.Exec(ctx, `INSERT INTO seed_sessions (org, session) VALUES ($1, $2)`, string(org), session)
		return err
	})
	return session, err
}

func (st *Step) checkSession(session string) error {
	var sealed bool
	err := st.Tx.QueryRow(st.ctx, `SELECT sealed FROM seed_sessions WHERE org = $1 AND session = $2`, string(st.org), session).Scan(&sealed)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && sealed {
		return SeedRefused{refusedNotEmpty}
	}
	return err
}

// SeedBatch saves ordinary changes with their past times. Each must follow the
// log's head by number and never go back in time. The batch saves whole or not
// at all.
func (s *Store) SeedBatch(ctx context.Context, org ids.Org, session string, batch contract.SeedBatch) (int64, error) {
	var through int64
	err := s.Save(ctx, org, func(st *Step) error {
		if err := st.checkSession(session); err != nil {
			return err
		}
		st.noTick = true // seeded history asks for no clock ticks
		for _, change := range batch.Changes {
			if err := st.appendSeeded(change); err != nil {
				return err
			}
		}
		through = st.head
		return nil
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && (pg.Code == "23503" || pg.Code == "23505" || pg.Code == "23514") {
		return 0, SeedRefused{"invalid change: " + pg.ConstraintName}
	}
	return through, err
}

func (st *Step) appendSeeded(change contract.Change) error {
	h, err := seedHeader(change)
	if err != nil {
		return SeedRefused{err.Error()}
	}
	if h.Org != string(st.org) {
		return SeedRefused{"another organization"}
	}
	if h.Number != st.head+1 || h.Previous != st.head {
		return SeedRefused{fmt.Sprintf("out of order: expected number %d", st.head+1)}
	}
	at, err := time.Parse(time.RFC3339Nano, h.At)
	if err != nil {
		return SeedRefused{"bad time"}
	}
	at = at.UTC().Truncate(time.Millisecond)
	if st.headAt != nil && at.Before(*st.headAt) {
		return SeedRefused{"time goes back"}
	}
	if m, ok := change.Value.(contract.ChangeMessageCreated); ok && m.Message.Number != h.Number {
		return SeedRefused{"a message's number must be its change's number"}
	}
	st.at, st.headAt = at, &at
	_, err = st.Append(change.Value) // Append sets the header
	return err
}

type header struct {
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	At       string `json:"at"`
}

func seedHeader(change contract.Change) (header, error) {
	var h header
	b, err := json.Marshal(change)
	if err != nil {
		return h, err
	}
	return h, json.Unmarshal(b, &h)
}

// SeedSeal closes the session for good and answers with the log's last number.
// The log continues from there.
func (s *Store) SeedSeal(ctx context.Context, org ids.Org, session string) (int64, error) {
	var through int64
	err := s.Save(ctx, org, func(st *Step) error {
		if err := st.checkSession(session); err != nil {
			return err
		}
		through = st.head
		_, err := st.Tx.Exec(ctx, `UPDATE seed_sessions SET sealed = true WHERE org = $1`, string(org))
		return err
	})
	return through, err
}
