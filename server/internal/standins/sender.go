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

// DelayedAfter is how late a notification may go out unmarked.
const DelayedAfter = 10 * time.Minute

// Sender is the notification sender stand-in. It only delivers: the feed has
// already held and judged every note (D-034). "Sending" is one row in
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
	controlRoutes(mux, s.modes, s.opt.Dir, s.wipeOrg, "notifications", "deliveries", "state")
	return mux
}

type wireNote struct {
	Kind     string  `json:"kind"`
	Org      string  `json:"org"`
	Number   int64   `json:"number"`
	Previous int64   `json:"previous"`
	Edition  int64   `json:"edition"`
	At       string  `json:"at"`
	Name     string  `json:"name"`
	Person   string  `json:"person"`
	Why      string  `json:"why"`
	Text     *string `json:"text"`
	Now      *string `json:"now"`
	DueAt    string  `json:"due_at"`
	History  bool    `json:"history"`
	Skip     bool    `json:"skip"`
}

func (n wireNote) text() string {
	if n.Text == nil {
		return ""
	}
	return *n.Text
}

// problem says what is wrong with a note. A note being stepped over needs only
// its numbers: it may be the very note that cannot be read.
func (n wireNote) problem() (at, due time.Time, why string) {
	if n.Number < 1 || n.Previous < 0 || n.Edition < 0 {
		return at, due, "a note needs a number, a previous number and an edition"
	}
	if n.Skip {
		return at, due, ""
	}
	at, err := time.Parse(time.RFC3339Nano, n.At)
	if err != nil {
		return at, due, "a note needs a time"
	}
	if strings.ContainsRune(n.text(), 0) || strings.ContainsRune(n.Why, 0) {
		return at, due, "text holds a NUL character"
	}
	switch n.Kind {
	case "send":
		if n.Person == "" || n.Text == nil {
			return at, due, "a send note needs a person and a text"
		}
		if due, err = time.Parse(time.RFC3339Nano, n.DueAt); err != nil {
			return at, due, "a send note needs the time it became due"
		}
	case "update":
		if n.Text == nil && n.Now == nil {
			return at, due, "an update needs a text or what is now true"
		}
		if n.Now != nil && *n.Now != "read" && *n.Now != "deleted" {
			return at, due, "unknown value for now"
		}
	default:
		return at, due, "unknown kind"
	}
	if !strings.HasPrefix(n.Name, n.Org+" / ") {
		return at, due, "a name begins with its organization"
	}
	return at, due, ""
}

type senderState struct {
	edition, place int64
}

func lockSenderState(ctx context.Context, tx pgx.Tx, org ids.Org) (senderState, error) {
	// One statement, so a delete between two steps cannot leave it with no row.
	var st senderState
	err := tx.QueryRow(ctx, `INSERT INTO state (org) VALUES ($1)
		ON CONFLICT (org) DO UPDATE SET org = EXCLUDED.org
		RETURNING edition, place`, string(org)).Scan(&st.edition, &st.place)
	return st, err
}

// findSenderState locks the row if there is one, and creates nothing. With no
// row it answers the start: the first edition, place 0.
func findSenderState(ctx context.Context, tx pgx.Tx, org ids.Org) (senderState, bool, error) {
	st := senderState{edition: 1}
	err := tx.QueryRow(ctx, "SELECT edition, place FROM state WHERE org = $1 FOR UPDATE", string(org)).
		Scan(&st.edition, &st.place)
	if errors.Is(err, pgx.ErrNoRows) {
		return senderState{edition: 1}, false, nil
	}
	return st, err == nil, err
}

func saveSenderState(ctx context.Context, tx pgx.Tx, org ids.Org, st senderState) error {
	_, err := tx.Exec(ctx, "UPDATE state SET edition = $2, place = $3 WHERE org = $1", string(org), st.edition, st.place)
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
	at, due, why := n.problem()
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
		if n.Edition < st.edition {
			status, answer = http.StatusConflict, contract.NoteBehind{IAmAt: st.place, Edition: st.edition}
			return nil
		}
		if n.Edition > st.edition {
			// A new edition: start again. The names sent stay; updates still
			// waiting for a send are written again by the replay.
			st.edition, st.place = n.Edition, 0
			if _, err := tx.Exec(ctx, "DELETE FROM notifications WHERE org = $1 AND state = 'waiting'", string(org)); err != nil {
				return err
			}
		}
		already := true
		switch {
		case n.Skip && n.Number <= st.place:
			status, answer = http.StatusOK, contract.NoteTaken{Taken: true, Already: &already}
		case n.Number > st.place && n.Previous != st.place:
			status, answer = http.StatusConflict, contract.NoteBehind{IAmAt: st.place, Edition: st.edition}
		case n.Skip: // parked by the worker: step over it, unapplied, until it is tried again
			st.place = n.Number
			status, answer = http.StatusOK, contract.NoteTaken{Taken: true}
		default:
			if n.Number <= st.place { // a repeat, or a parked note tried again
				seen, err := s.seen(ctx, tx, org, n)
				if err != nil {
					return err
				}
				if seen {
					status, answer = http.StatusOK, contract.NoteTaken{Taken: true, Already: &already}
					break
				}
			}
			if s.opt.Refuse != nil {
				if no := s.opt.Refuse(n.Org, n.Number); no != "" {
					status, answer = http.StatusUnprocessableEntity, contract.Refusal{Refused: no}
					break
				}
			}
			if err := s.apply(ctx, tx, org, n, at, due); err != nil {
				return err
			}
			st.place = max(st.place, n.Number)
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

// seen says whether a note at or behind the place has already had its effect.
func (s *Sender) seen(ctx context.Context, tx pgx.Tx, org ids.Org, n wireNote) (bool, error) {
	var state string
	var textNumber int64
	var now *string
	err := tx.QueryRow(ctx, "SELECT state, text_number, now FROM notifications WHERE org = $1 AND name = $2", string(org), n.Name).
		Scan(&state, &textNumber, &now)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if n.Kind == "send" {
		return state != "waiting" && !fault(FaultNoSentList), nil
	}
	textDone := n.Text == nil || textNumber >= n.Number || (now != nil && *now == "deleted")
	nowDone := n.Now == nil || (now != nil && (*now == "deleted" || *now == *n.Now))
	return textDone && nowDone, nil
}

func (s *Sender) apply(ctx context.Context, tx pgx.Tx, org ids.Org, n wireNote, at, due time.Time) error {
	if n.Kind == "update" {
		return s.update(ctx, tx, org, n, at)
	}
	return s.send(ctx, tx, org, n, at, due)
}

// send hands a notification over, once per name. A name listed as history is
// never handed over, whatever arrives for it later (D-032, D-034). A row waiting
// with an earlier update becomes the entry and keeps what that update said.
func (s *Sender) send(ctx context.Context, tx pgx.Tx, org ids.Org, n wireNote, at, due time.Time) error {
	now := s.opt.Clock.Now()
	state, sentAt, delayed := "sent", now, false
	if n.History {
		state, sentAt = "history", due
	} else {
		// Late by whichever is later: this machine's hand-over, or the note's own
		// time, so a machine clock stepped back cannot hide a late notification.
		handed := now
		if at.After(handed) {
			handed = at
		}
		delayed = handed.Sub(due) > DelayedAfter
	}
	only := " WHERE notifications.state = 'waiting'"
	if fault(FaultNoSentList) {
		only = ""
	}
	tag, err := tx.Exec(ctx, `INSERT INTO notifications (org, name, person, state, due_at, arrived_at, sent_at, delayed, why, body)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (org, name) DO UPDATE SET person = EXCLUDED.person, state = EXCLUDED.state, due_at = EXCLUDED.due_at,
			arrived_at = EXCLUDED.arrived_at, sent_at = EXCLUDED.sent_at, delayed = EXCLUDED.delayed, why = EXCLUDED.why,
			body = CASE WHEN notifications.now = 'deleted' THEN '' WHEN notifications.text_number > 0 THEN notifications.body ELSE EXCLUDED.body END`+only,
		string(org), n.Name, n.Person, state, due, now, sentAt, delayed, n.Why, n.text())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 || state != "sent" || fault(FaultSendNothing) {
		return nil
	}
	_, err = tx.Exec(ctx, "INSERT INTO deliveries (org, name, at) VALUES ($1, $2, $3)", string(org), n.Name, now)
	return err
}

// update carries what is true now to an entry, or to a waiting row for a send
// still to come (a parked send may be tried again after its update).
func (s *Sender) update(ctx context.Context, tx pgx.Tx, org ids.Org, n wireNote, at time.Time) error {
	if _, err := tx.Exec(ctx, `INSERT INTO notifications (org, name, person, state, due_at, arrived_at, why, body)
		VALUES ($1, $2, '', 'waiting', $3, $4, '', '') ON CONFLICT (org, name) DO NOTHING`,
		string(org), n.Name, at, s.opt.Clock.Now()); err != nil {
		return err
	}
	return applyUpdate(ctx, tx, org, n.Name, n.Text, n.Number, n.Now)
}

// applyUpdate changes an entry. A newer text wins over an older one that was
// tried again later, and a delete wins over everything (D-004).
func applyUpdate(ctx context.Context, tx pgx.Tx, org ids.Org, name string, text *string, textNumber int64, now *string) error {
	if text != nil {
		if _, err := tx.Exec(ctx, `UPDATE notifications SET body = $3, text_number = $4
			WHERE org = $1 AND name = $2 AND text_number < $4 AND now IS DISTINCT FROM 'deleted'`,
			string(org), name, *text, textNumber); err != nil {
			return err
		}
	}
	if now == nil {
		return nil
	}
	if *now == "deleted" {
		_, err := tx.Exec(ctx, "UPDATE notifications SET now = 'deleted', body = '' WHERE org = $1 AND name = $2", string(org), name)
		return err
	}
	_, err := tx.Exec(ctx, "UPDATE notifications SET now = $3 WHERE org = $1 AND name = $2 AND now IS DISTINCT FROM 'deleted'", string(org), name, *now)
	return err
}

func (s *Sender) place(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok || !s.modes.gate(r.Context(), s.opt.Clock, string(org)) {
		return
	}
	var p contract.SenderPlace
	err := orgdb.InOrg(r.Context(), s.opt.Dir, org, func(tx pgx.Tx) error {
		st, _, err := findSenderState(r.Context(), tx, org)
		p = contract.SenderPlace{Through: st.place, Edition: st.edition}
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
			FROM notifications WHERE org = $1 AND person = $2 AND state <> 'waiting' ORDER BY due_at DESC, name`, string(org), person)
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

// wipeOrg empties what the sender holds, except its list of names: a name once
// sent or listed as history is never handed over again.
func (s *Sender) wipeOrg(ctx context.Context, org ids.Org) error {
	return orgdb.InOrg(ctx, s.opt.Dir, org, func(tx pgx.Tx) error {
		st, found, err := findSenderState(ctx, tx, org)
		if err != nil || !found {
			return err // nothing was ever delivered, or it was deleted: nothing to wipe
		}
		if _, err := tx.Exec(ctx, "DELETE FROM notifications WHERE org = $1 AND state = 'waiting'", string(org)); err != nil {
			return err
		}
		st.place = 0
		return saveSenderState(ctx, tx, org, st)
	})
}
