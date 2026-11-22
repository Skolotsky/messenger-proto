package standins

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

// Hold is how long a notification waits, by the notes' own clock.
const Hold = 5 * time.Second

// DelayedAfter is how late a notification may go out unmarked.
const DelayedAfter = 10 * time.Minute

// Sender is the notification sender stand-in. "Sending" is one row in
// deliveries, written in the same transaction as the place.
type Sender struct {
	opt   Options
	modes *modes
}

// NewSender returns the sender.
func NewSender(o Options) *Sender { return &Sender{opt: o, modes: newModes()} }

// Routes serves the sender's side of its contracts.
func (s *Sender) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /notes", s.note)
	mux.HandleFunc("GET /orgs/{org}/place", s.place)
	mux.HandleFunc("GET /orgs/{org}/notifications", s.notifications)
	controlRoutes(mux, s.modes, s.opt.Dir, s.wipeOrg)
	return mux
}

type wireNote struct {
	Kind     string `json:"kind"`
	Org      string `json:"org"`
	Reset    int64  `json:"reset"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	Edition  int64  `json:"edition"`
	At       string `json:"at"`
	Name     string `json:"name"`
	Person   string `json:"person"`
	Why      string `json:"why"`
	Text     string `json:"text"`
	Because  string `json:"because"`
}

func (n wireNote) problem() (time.Time, string) {
	at, err := time.Parse(time.RFC3339Nano, n.At)
	if err != nil {
		return at, "a note needs a time"
	}
	if n.Number < 1 || n.Previous < 0 || n.Edition < 0 {
		return at, "a note needs a number, a previous number and an edition"
	}
	if strings.ContainsRune(n.Text, 0) || strings.ContainsRune(n.Why, 0) {
		return at, "text holds a NUL character"
	}
	switch n.Kind {
	case "clock":
		return at, ""
	case "due":
		if n.Person == "" {
			return at, "a due note needs a person"
		}
	case "changed":
	case "withdrawn":
		switch n.Because {
		case "read", "deleted", "fenced", "rule_changed":
		default:
			return at, "unknown reason for withdrawing"
		}
	default:
		return at, "unknown kind"
	}
	if !strings.HasPrefix(n.Name, n.Org+" / ") {
		return at, "a name begins with its organization"
	}
	return at, ""
}

type senderState struct {
	reset, edition, place int64
	clock                 *time.Time
}

func lockSenderState(ctx context.Context, tx pgx.Tx, org ids.Org) (senderState, error) {
	if _, err := tx.Exec(ctx, "INSERT INTO state (org) VALUES ($1) ON CONFLICT DO NOTHING", string(org)); err != nil {
		return senderState{}, err
	}
	var st senderState
	err := tx.QueryRow(ctx, "SELECT reset, edition, place, clock FROM state WHERE org = $1 FOR UPDATE", string(org)).
		Scan(&st.reset, &st.edition, &st.place, &st.clock)
	return st, err
}

func saveSenderState(ctx context.Context, tx pgx.Tx, org ids.Org, st senderState) error {
	_, err := tx.Exec(ctx, "UPDATE state SET reset = $2, edition = $3, place = $4, clock = $5 WHERE org = $1",
		string(org), st.reset, st.edition, st.place, st.clock)
	return err
}

func (s *Sender) note(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	var n wireNote
	decodeErr := json.Unmarshal(body, &n)
	if !s.modes.gate(r.Context(), s.opt.Clock, n.Org) {
		return
	}
	org, orgErr := ids.ParseOrg(n.Org)
	if decodeErr != nil || orgErr != nil {
		refuse(w, http.StatusUnprocessableEntity, "not a valid note")
		return
	}
	at, why := n.problem()
	if why != "" {
		refuse(w, http.StatusUnprocessableEntity, why)
		return
	}
	ctx := r.Context()
	var status int
	var answer any
	err = orgdb.InOrg(ctx, s.opt.Dir, org, func(tx pgx.Tx) error {
		st, err := lockSenderState(ctx, tx, org)
		if err != nil {
			return err
		}
		if n.Reset < st.reset {
			status, answer = http.StatusConflict, contract.StaleReset{StaleReset: st.reset}
			return nil
		}
		st.reset = n.Reset
		if n.Edition < st.edition {
			status, answer = http.StatusConflict, contract.NoteBehind{IAmAt: st.place, Edition: st.edition}
			return nil
		}
		if n.Edition > st.edition {
			// A new edition: start again; the replay recreates what was held, by name.
			st.edition, st.place, st.clock = n.Edition, 0, nil
			if _, err := tx.Exec(ctx, "DELETE FROM notifications WHERE org = $1 AND state = 'held'", string(org)); err != nil {
				return err
			}
		}
		switch {
		case n.Number <= st.place:
			already := true
			status, answer = http.StatusOK, contract.NoteTaken{Taken: true, Already: &already}
		case n.Previous != st.place:
			status, answer = http.StatusConflict, contract.NoteBehind{IAmAt: st.place, Edition: st.edition}
		default:
			if s.opt.Refuse != nil {
				if no := s.opt.Refuse(n.Org, n.Number); no != "" {
					status, answer = http.StatusUnprocessableEntity, contract.Refusal{Refused: no}
					break
				}
			}
			if err := s.apply(ctx, tx, org, n, at, &st); err != nil {
				return err
			}
			st.place = n.Number
			status, answer = http.StatusOK, contract.NoteTaken{Taken: true}
		}
		return saveSenderState(ctx, tx, org, st)
	})
	if err != nil {
		unavailable(w, "sender note", err)
		return
	}
	writeJSON(w, status, answer)
}

func (s *Sender) apply(ctx context.Context, tx pgx.Tx, org ids.Org, n wireNote, at time.Time, st *senderState) error {
	now := s.opt.Clock.Now()
	switch n.Kind {
	case "due":
		onConflict := " WHERE notifications.state <> 'sent'"
		if fault(FaultNoSentList) {
			onConflict = ""
		}
		if _, err := tx.Exec(ctx, `INSERT INTO notifications (org, name, person, state, due_at, arrived_at, why, body)
			VALUES ($1, $2, $3, 'held', $4, $5, $6, $7)
			ON CONFLICT (org, name) DO UPDATE SET state = 'held', due_at = EXCLUDED.due_at, arrived_at = EXCLUDED.arrived_at,
				sent_at = NULL, why = EXCLUDED.why, body = EXCLUDED.body, now = NULL`+onConflict,
			string(org), n.Name, n.Person, at, now, n.Why, n.Text); err != nil {
			return err
		}
	case "changed":
		if _, err := tx.Exec(ctx, "UPDATE notifications SET body = $3 WHERE org = $1 AND name = $2 AND now IS DISTINCT FROM 'deleted'",
			string(org), n.Name, n.Text); err != nil {
			return err
		}
	case "withdrawn":
		if err := s.withdraw(ctx, tx, org, n, at, now); err != nil {
			return err
		}
	case "clock":
		if st.clock == nil || at.After(*st.clock) {
			st.clock = &at
		}
		if !fault(FaultHoldArrival) {
			if err := s.sendHeld(ctx, tx, org, *st.clock, now); err != nil {
				return err
			}
		}
	}
	if fault(FaultAnyNoteTime) {
		if st.clock == nil || at.After(*st.clock) {
			st.clock = &at
		}
		if err := s.sendHeld(ctx, tx, org, *st.clock, now); err != nil {
			return err
		}
	}
	if fault(FaultHoldArrival) {
		return s.sendWhere(ctx, tx, org, "arrived_at <= $2", now.Add(-Hold), now)
	}
	return nil
}

// withdraw takes back a held notification. One whose hold had already ended by
// the log's own clock was not withdrawn in time: it goes out, showing the truth.
// A withdrawal because the rules changed lists it as history, never sent.
func (s *Sender) withdraw(ctx context.Context, tx pgx.Tx, org ids.Org, n wireNote, at, now time.Time) error {
	var state string
	var due time.Time
	err := tx.QueryRow(ctx, "SELECT state, due_at FROM notifications WHERE org = $1 AND name = $2", string(org), n.Name).Scan(&state, &due)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if n.Because == "rule_changed" {
		if state == "held" {
			_, err = tx.Exec(ctx, "UPDATE notifications SET state = 'history', sent_at = due_at WHERE org = $1 AND name = $2", string(org), n.Name)
		}
		return err
	}
	if state == "held" {
		if at.After(due.Add(Hold)) && n.Because != "fenced" {
			if err := s.handOver(ctx, tx, org, n.Name, due, now); err != nil {
				return err
			}
			state = "sent"
		} else {
			state = "withdrawn"
		}
	}
	var mark any
	if n.Because == "read" || n.Because == "deleted" {
		mark = n.Because
	}
	blank := ""
	if n.Because == "deleted" {
		blank = ", body = ''"
	}
	_, err = tx.Exec(ctx, "UPDATE notifications SET state = $3, now = COALESCE($4::text, now)"+blank+" WHERE org = $1 AND name = $2",
		string(org), n.Name, state, mark)
	return err
}

func (s *Sender) handOver(ctx context.Context, tx pgx.Tx, org ids.Org, name string, due, now time.Time) error {
	if fault(FaultSendNothing) {
		return nil
	}
	if _, err := tx.Exec(ctx, "UPDATE notifications SET state = 'sent', sent_at = $3, delayed = $4 WHERE org = $1 AND name = $2",
		string(org), name, now, now.Sub(due) > DelayedAfter); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "INSERT INTO deliveries (org, name, at) VALUES ($1, $2, $3)", string(org), name, now)
	return err
}

// sendHeld hands over everything whose hold ended before the clock note.
func (s *Sender) sendHeld(ctx context.Context, tx pgx.Tx, org ids.Org, noteClock, now time.Time) error {
	return s.sendWhere(ctx, tx, org, "due_at < $2", noteClock.Add(-Hold), now)
}

func (s *Sender) sendWhere(ctx context.Context, tx pgx.Tx, org ids.Org, cond string, limit, now time.Time) error {
	rows, err := tx.Query(ctx, "SELECT name, due_at FROM notifications WHERE org = $1 AND state = 'held' AND "+cond+" ORDER BY due_at, name", string(org), limit)
	if err != nil {
		return err
	}
	type held struct {
		name string
		due  time.Time
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (held, error) {
		var h held
		return h, r.Scan(&h.name, &h.due)
	})
	if err != nil {
		return err
	}
	for _, h := range list {
		if err := s.handOver(ctx, tx, org, h.name, h.due, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sender) place(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok || !s.modes.gate(r.Context(), s.opt.Clock, string(org)) {
		return
	}
	var p contract.SenderPlace
	err := orgdb.InOrg(r.Context(), s.opt.Dir, org, func(tx pgx.Tx) error {
		st, err := lockSenderState(r.Context(), tx, org)
		p = contract.SenderPlace{Reset: st.reset, Through: st.place, Edition: st.edition}
		return err
	})
	if err != nil {
		unavailable(w, "sender place", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Sender) notifications(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok || !s.modes.gate(r.Context(), s.opt.Clock, string(org)) {
		return
	}
	person := r.URL.Query().Get("person")
	if person == "" {
		refuse(w, http.StatusBadRequest, "person is required")
		return
	}
	out := contract.NotificationList{Notifications: []contract.NotificationEntry{}}
	err := orgdb.InOrg(r.Context(), s.opt.Dir, org, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `SELECT name, person, state, sent_at, delayed, why, body, now
			FROM notifications WHERE org = $1 AND person = $2 ORDER BY due_at DESC, name`, string(org), person)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e contract.NotificationEntry
			var at *time.Time
			var now *string
			if err := rows.Scan(&e.Name, &e.Person, &e.State, &at, &e.Delayed, &e.Why, &e.Text, &now); err != nil {
				return err
			}
			if at != nil {
				v := stamp(*at)
				e.At = &v
			}
			if now != nil {
				v := contract.NoteNow(*now)
				e.Now = &v
			}
			out.Notifications = append(out.Notifications, e)
		}
		return rows.Err()
	})
	if err != nil {
		unavailable(w, "sender list", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// wipeOrg empties what the sender holds, except the list of what it sent.
func (s *Sender) wipeOrg(ctx context.Context, org ids.Org) error {
	return orgdb.InOrg(ctx, s.opt.Dir, org, func(tx pgx.Tx) error {
		st, err := lockSenderState(ctx, tx, org)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "DELETE FROM notifications WHERE org = $1 AND state <> 'sent'", string(org)); err != nil {
			return err
		}
		st.place, st.clock = 0, nil
		return saveSenderState(ctx, tx, org, st)
	})
}
