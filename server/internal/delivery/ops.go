package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

// Routes answers the delivery operations of operations-and-control.md.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /orgs/{org}/status", s.withOrg(s.status))
	mux.HandleFunc("GET /orgs/{org}/parked", s.withOrg(s.parked))
	mux.HandleFunc("POST /orgs/{org}/parked/{number}/retry", s.withOrg(s.retry))
	mux.HandleFunc("POST /orgs/{org}/receivers/{receiver}/restart", s.withOrg(s.restart))
	mux.HandleFunc("POST /orgs/{org}/stop", s.withOrg(s.handleStop))
	mux.HandleFunc("DELETE /orgs/{org}", s.withOrg(s.handleDelete))
	return mux
}

func (s *Service) withOrg(h func(http.ResponseWriter, *http.Request, ids.Org)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, err := ids.ParseOrg(r.PathValue("org"))
		if err != nil {
			refuse(w, http.StatusBadRequest, "not an organization")
			return
		}
		h(w, r, org)
	}
}

func refuse(w http.ResponseWriter, status int, why string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"refused": why})
}

func answerJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Service) status(w http.ResponseWriter, r *http.Request, org ids.Org) {
	ctx := r.Context()
	pool, err := s.cfg.DB.PoolFor(ctx, org)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := contract.DeliveryStatus{Receivers: []contract.ReceiverStatus{}}
	for _, name := range []string{Search, Feed, Sender} {
		l := s.lanes[name]
		var rs contract.ReceiverStatus
		rs.Name = contract.ReceiverName(name)
		rs.State = "running"
		var head int64
		var halted bool
		if l.notes {
			var stopped *int64
			err = pool.QueryRow(ctx, `SELECT coalesce((SELECT head FROM feed.note_heads WHERE org = $1), 0),
				coalesce(p.place, 0), p.stopped_at, (SELECT count(*) FROM feed.note_parked n WHERE n.org = $1 AND n.tries >= $2)
				FROM (SELECT 1) one LEFT JOIN feed.note_places p ON p.org = $1`, string(org), triesToPark).
				Scan(&head, &rs.Place, &stopped, &rs.Parked)
			halted = stopped != nil
		} else {
			err = pool.QueryRow(ctx, `SELECT coalesce((SELECT head FROM org_log WHERE org = $1), 0),
				coalesce(p.place, 0), coalesce(p.paused, false),
				(SELECT count(*) FROM delivery_parked n WHERE n.org = $1 AND n.receiver = $2 AND n.tries >= $3)
				FROM (SELECT 1) one LEFT JOIN delivery_places p ON p.org = $1 AND p.receiver = $2`, string(org), name, triesToPark).
				Scan(&head, &rs.Place, &halted, &rs.Parked)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rs.Waiting = max(head-rs.Place, 0)
		var isDown bool
		if err := pool.QueryRow(ctx, `SELECT coalesce((SELECT failures > 0 FROM delivery_receivers WHERE receiver = $1), false)`,
			l.stateKey()).Scan(&isDown); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		switch {
		case halted && name == Feed:
			rs.State = "stopped"
		case halted:
			rs.State = "paused"
		case isDown:
			rs.State = "down"
		}
		out.Receivers = append(out.Receivers, rs)
	}
	answerJSON(w, out)
}

func (s *Service) parked(w http.ResponseWriter, r *http.Request, org ids.Org) {
	ctx := r.Context()
	pool, err := s.cfg.DB.PoolFor(ctx, org)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows, err := pool.Query(ctx, `SELECT receiver, number, reason, tries FROM delivery_parked WHERE org = $1 AND tries >= $2
		UNION ALL
		SELECT 'sender', n.number, n.reason, n.tries FROM feed.note_parked n
		  JOIN feed.note_places p ON p.org = n.org AND p.edition = n.edition
		 WHERE n.org = $1 AND n.tries >= $2
		ORDER BY 1, 2`, string(org), triesToPark)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (contract.ParkedChange, error) {
		var p contract.ParkedChange
		var name string
		err := r.Scan(&name, &p.Number, &p.Reason, &p.Tries)
		p.Receiver = contract.ReceiverName(name)
		return p, err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []contract.ParkedChange{}
	}
	answerJSON(w, contract.ParkedList{Parked: list})
}

// retry sends a parked change again now. Sent and taken, it leaves the parked
// list and the organization carries on; refused again or receiver down, it
// stays parked, which the list shows.
func (s *Service) retry(w http.ResponseWriter, r *http.Request, org ids.Org) {
	number, err := strconv.ParseInt(r.PathValue("number"), 10, 64)
	if err != nil {
		refuse(w, http.StatusBadRequest, "not a number")
		return
	}
	name := r.URL.Query().Get("receiver")
	l, ok := s.lanes[name]
	if !ok || name == Feed {
		refuse(w, http.StatusBadRequest, "no such receiver")
		return
	}
	var halted bool
	if err := orgdb.InOrg(r.Context(), s.cfg.DB, org, func(tx pgx.Tx) error {
		return tx.QueryRow(r.Context(), haltedSQL, string(org)).Scan(&halted)
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if halted {
		refuse(w, http.StatusNotFound, "not parked") // a stopped organization is sent nothing more
		return
	}
	var found bool
	if l.notes {
		found, err = l.retryNote(r.Context(), org, number)
	} else {
		found, err = l.retryChange(r.Context(), org, number)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		refuse(w, http.StatusNotFound, "not parked")
		return
	}
	answerJSON(w, struct{}{})
}

func (l *lane) retryChange(ctx context.Context, org ids.Org, number int64) (bool, error) {
	var kind, body string
	err := l.inOrg(ctx, org, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT c.kind, c.body::text
			FROM delivery_parked n
			JOIN change_log c ON c.org = n.org AND c.number = n.number
			WHERE n.org = $1 AND n.receiver = $2 AND n.number = $3 AND n.tries >= $4`,
			string(org), l.name, number, triesToPark).Scan(&kind, &body)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	item, send, err := forSearch(org, number, kind, []byte(body))
	if err != nil {
		return true, err
	}
	o := outcome{answer: applied}
	if send {
		o = l.rc.post(ctx, "/changes", item)
	}
	pool, err := l.s.cfg.DB.PoolFor(ctx, org)
	if err != nil {
		return true, err
	}
	switch o.answer {
	case applied:
		return true, pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `DELETE FROM delivery_parked WHERE org = $1 AND receiver = $2 AND number = $3`, string(org), l.name, number); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE delivery_places SET refusals = 0, paused = false WHERE org = $1 AND receiver = $2`, string(org), l.name); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO delivery_due (receiver, org, since) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
				l.name, string(org), l.s.cfg.Clock.Now())
			return err
		})
	case refusedOne:
		_, err := pool.Exec(ctx, `UPDATE delivery_parked SET tries = tries + 1, reason = $4, parked_at = $5
			WHERE org = $1 AND receiver = $2 AND number = $3`, string(org), l.name, number, o.reason, l.s.cfg.Clock.Now())
		return true, err
	}
	return true, nil
}

func (l *lane) retryNote(ctx context.Context, org ids.Org, number int64) (bool, error) {
	var body string
	var edition int64
	err := l.inOrg(ctx, org, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT c.body::text, h.edition
			FROM feed.note_parked n
			JOIN feed.note_heads h ON h.org = n.org AND h.edition = n.edition
			JOIN feed.note_log c ON c.org = n.org AND c.edition = n.edition AND c.number = n.number
			WHERE n.org = $1 AND n.number = $2 AND n.tries >= $3`, string(org), number, triesToPark).Scan(&body, &edition)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	o := l.rc.post(ctx, "/notes", json.RawMessage(body))
	pool, err := l.s.cfg.DB.PoolFor(ctx, org)
	if err != nil {
		return true, err
	}
	switch o.answer {
	case applied:
		return true, pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `DELETE FROM feed.note_parked WHERE org = $1 AND edition = $2 AND number = $3`, string(org), edition, number); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE feed.note_places SET stopped_at = NULL, place = greatest(place, $2) WHERE org = $1`, string(org), number); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO feed.note_due (org, since) VALUES ($1, $2) ON CONFLICT DO NOTHING`, string(org), l.s.cfg.Clock.Now())
			return err
		})
	case refusedOne:
		_, err := pool.Exec(ctx, `UPDATE feed.note_parked SET tries = tries + 1, reason = $4, parked_at = $5
			WHERE org = $1 AND edition = $2 AND number = $3`, string(org), edition, number, o.reason, l.s.cfg.Clock.Now())
		return true, err
	}
	return true, nil
}

// restart sets a receiver's place to the start, for a rebuild. It takes any
// claim away, so a turn still running cannot write its old place back.
func (s *Service) restart(w http.ResponseWriter, r *http.Request, org ids.Org) {
	ctx := r.Context()
	name := r.PathValue("receiver")
	l, ok := s.lanes[name]
	if !ok {
		refuse(w, http.StatusBadRequest, "no such receiver")
		return
	}
	var in contract.RestartRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			refuse(w, http.StatusBadRequest, "unreadable body")
			return
		}
	}
	pool, err := s.cfg.DB.PoolFor(ctx, org)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	now := s.cfg.Clock.Now()
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if l.notes {
			edition := in.Edition
			if edition == nil {
				var current int64
				if err := tx.QueryRow(ctx, `SELECT coalesce((SELECT edition FROM feed.note_heads WHERE org = $1), 1)`, string(org)).Scan(&current); err != nil {
					return err
				}
				edition = &current
			}
			if _, err := tx.Exec(ctx, `INSERT INTO feed.note_places (org, edition, place) VALUES ($1, $2, 0)
				ON CONFLICT (org) DO UPDATE SET edition = $2, place = 0, stopped_at = NULL, claimed_by = NULL, claimed_until = NULL`,
				string(org), *edition); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `DELETE FROM feed.note_parked WHERE org = $1`, string(org)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO feed.note_due (org, since) VALUES ($1, $2) ON CONFLICT DO NOTHING`, string(org), now)
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO delivery_places (org, receiver, place) VALUES ($1, $2, 0)
			ON CONFLICT (org, receiver) DO UPDATE SET place = 0, refusals = 0, paused = false, claimed_by = NULL, claimed_until = NULL`,
			string(org), name); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM delivery_parked WHERE org = $1 AND receiver = $2`, string(org), name); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO delivery_due (receiver, org, since) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, name, string(org), now)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	answerJSON(w, struct{}{})
}
