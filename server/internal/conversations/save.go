// Package conversations saves what people do and writes it to the numbered
// change log in the same transaction (D-005, D-006, D-017).
package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"contexia/internal/shared/clock"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

// TimeFormat is how every time on the wire is written.
const TimeFormat = "2006-01-02T15:04:05.000Z"

// tickAfter is when an organization's log wants its next clock tick
// (conversations-to-feed.md). Housekeeping (T-053) writes the ticks.
const tickAfter = time.Second

// ErrUnknownOrg says the organization has no log.
var ErrUnknownOrg = errors.New("unknown organization")

// Store is Conversations' way into its database.
type Store struct {
	Dir   orgdb.Directory
	Clock clock.Clock
}

// Step is one saving transaction: it holds the organization's log row, so
// numbers commit in order (G-005). Everything appended commits together.
type Step struct {
	ctx     context.Context
	Tx      pgx.Tx
	org     ids.Org
	head    int64
	headAt  *time.Time
	at      time.Time
	wrote   bool
	nonTick bool
	noTick  bool
}

// Save runs fn as one step of the organization's log. An error from fn rolls
// back everything it wrote.
func (s *Store) Save(ctx context.Context, org ids.Org, fn func(*Step) error) error {
	return orgdb.InOrg(ctx, s.Dir, org, func(tx pgx.Tx) error {
		step, err := s.open(ctx, tx, org)
		if err != nil {
			return err
		}
		if err := fn(step); err != nil {
			return err
		}
		return step.finish()
	})
}

func (s *Store) open(ctx context.Context, tx pgx.Tx, org ids.Org) (*Step, error) {
	step := &Step{ctx: ctx, Tx: tx, org: org}
	if plainSequence {
		// Test-only fault for G-005: no lock, so numbers can commit out of order.
		var one int
		if err := tx.QueryRow(ctx, `SELECT 1 FROM org_log WHERE org = $1`, string(org)).Scan(&one); err != nil {
			return nil, unknownOrg(err)
		}
		step.at = s.Clock.Now().UTC().Truncate(time.Millisecond)
		return step, nil
	}
	err := tx.QueryRow(ctx, `SELECT head, head_at FROM org_log WHERE org = $1 FOR UPDATE`, string(org)).
		Scan(&step.head, &step.headAt)
	if err != nil {
		return nil, unknownOrg(err)
	}
	step.at = s.Clock.Now().UTC().Truncate(time.Millisecond)
	if step.headAt != nil && step.headAt.After(step.at) {
		step.at = step.headAt.UTC() // timestamps never decrease within a log
	}
	return step, nil
}

func unknownOrg(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUnknownOrg
	}
	return err
}

// Next is the number the next appended change will get.
func (s *Step) Next() int64 { return s.head + 1 }

// At is the time every change of this step is stamped with.
func (s *Step) At() string { return s.at.Format(TimeFormat) }

// Append writes one change, given as its contract type with the header left
// empty, and applies it to the tables. It returns the change's number.
func (s *Step) Append(change any) (int64, error) {
	body, err := json.Marshal(change)
	if err != nil {
		return 0, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return 0, err
	}
	var kind string
	if err := json.Unmarshal(fields["kind"], &kind); err != nil || kind == "" {
		return 0, fmt.Errorf("change without a kind: %T", change)
	}
	number, previous := s.head+1, s.head
	if plainSequence {
		if err := s.Tx.QueryRow(s.ctx, faultSequenceSQL).Scan(&number); err != nil {
			return 0, err
		}
		previous = number - 1
	}
	stamp := s.At()
	for k, v := range map[string]any{"org": string(s.org), "number": number, "previous": previous, "at": stamp} {
		b, _ := json.Marshal(v)
		fields[k] = b
	}
	whole, err := json.Marshal(fields)
	if err != nil {
		return 0, err
	}
	if _, err := s.Tx.Exec(s.ctx, `INSERT INTO change_log (org, number, at, kind, body) VALUES ($1, $2, $3, $4, $5)`,
		string(s.org), number, s.at, kind, whole); err != nil {
		return 0, fmt.Errorf("write log entry: %w", err)
	}
	if err := s.project(change, number); err != nil {
		return 0, err
	}
	s.head = number
	s.wrote = true
	if kind != "tick" {
		s.nonTick = true
	}
	return number, nil
}

// finish moves the log's head and says the receivers have work, in the same
// transaction as the changes.
func (s *Step) finish() error {
	if !s.wrote || plainSequence {
		return nil
	}
	var err error
	if s.nonTick {
		var due *time.Time
		if !s.noTick {
			t := s.at.Add(tickAfter)
			due = &t
		}
		_, err = s.Tx.Exec(s.ctx, `UPDATE org_log SET head = $2, head_at = $3, last_change_at = $3, tick_due_at = $4 WHERE org = $1`,
			string(s.org), s.head, s.at, due)
	} else {
		_, err = s.Tx.Exec(s.ctx, `UPDATE org_log SET head = $2, head_at = $3 WHERE org = $1`, string(s.org), s.head, s.at)
	}
	if err != nil {
		return fmt.Errorf("move the head: %w", err)
	}
	_, err = s.Tx.Exec(s.ctx, `INSERT INTO delivery_due (receiver, org, since)
		VALUES ('search', $1, $2), ('feed', $1, $2) ON CONFLICT DO NOTHING`, string(s.org), s.at)
	if err != nil {
		return fmt.Errorf("mark delivery due: %w", err)
	}
	return nil
}
