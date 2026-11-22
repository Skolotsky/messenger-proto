// Command standinfeed answers as the activity feed does at its two boundaries
// (conversations-to-feed.md, door-reads.md), so the Milestone 1 tests can be proven
// before the real feed exists. It is test equipment, never the product.
//
// The rules are the product's own pure package. What the real feed keeps in its
// own tables, this keeps as the accepted changes in one table: every copy brings
// its memory up to date from there under a per-organization lock, so any number of
// copies can run side by side and a copy that was killed loses nothing.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"

	"contexia/internal/feed/rules"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/migrate"
	"contexia/internal/shared/orgdb"
	"contexia/internal/shared/part"
	"contexia/migrations"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const tables = `
CREATE TABLE IF NOT EXISTS standin_changes (org text, number bigint, body text NOT NULL, PRIMARY KEY (org, number));
CREATE TABLE IF NOT EXISTS standin_updates (org text, number bigint, person text NOT NULL, after_number bigint NOT NULL, parts text NOT NULL, PRIMARY KEY (org, number));
CREATE INDEX IF NOT EXISTS standin_updates_person ON standin_updates (org, person, number DESC);`

const edition = int64(1)

type orgState struct {
	engine *rules.Engine
	at     int64
}

type feed struct {
	pool *pgxpool.Pool
	mu   sync.Mutex
	orgs map[string]*orgState
}

func (f *feed) state(org string) *orgState {
	s := f.orgs[org]
	if s == nil {
		s = &orgState{engine: rules.New(org)}
		f.orgs[org] = s
	}
	return s
}

// catchUp replays what other copies accepted. Replaying uses the same options as
// applying, so the holds in memory are the same; only the notes are not written again.
func (f *feed) catchUp(ctx context.Context, tx pgx.Tx, org string, s *orgState) error {
	rows, err := tx.Query(ctx, `SELECT number, body FROM standin_changes WHERE org = $1 AND number > $2 ORDER BY number`, org, s.at)
	if err != nil {
		return err
	}
	type item struct {
		n    int64
		body string
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (item, error) {
		var it item
		return it, r.Scan(&it.n, &it.body)
	})
	if err != nil {
		return err
	}
	for _, it := range items {
		var c contract.Change
		if err := json.Unmarshal([]byte(it.body), &c); err != nil {
			return err
		}
		if _, err := s.engine.Apply(c, rules.Options{}); err != nil {
			return err
		}
		s.at = it.n
	}
	return nil
}

type head struct {
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
}

type refusal struct {
	status int
	body   any
}

func (f *feed) apply(ctx context.Context, tx pgx.Tx, s *orgState, h head, raw []byte) (*refusal, error) {
	if h.Number <= s.at {
		return nil, nil
	}
	if h.Previous != s.at {
		return &refusal{http.StatusConflict, map[string]int64{"i_am_at": s.at}}, nil
	}
	var c contract.Change
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	fx, err := s.engine.Apply(c, rules.Options{})
	if err != nil {
		return nil, err
	}
	s.at = h.Number
	if _, err := tx.Exec(ctx, `INSERT INTO standin_changes (org, number, body) VALUES ($1, $2, $3)`, h.Org, h.Number, string(raw)); err != nil {
		return nil, err
	}
	for _, r := range fx.Readers {
		parts, err := json.Marshal(r.Parts)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO standin_updates (org, number, person, after_number, parts)
			SELECT $1, coalesce((SELECT max(number) FROM standin_updates WHERE org = $1), 0) + 1, $2,
			       coalesce((SELECT max(number) FROM standin_updates WHERE org = $1 AND person = $2), 0), $3`,
			h.Org, r.Person, string(parts)); err != nil {
			return nil, err
		}
	}
	for _, d := range fx.Notes {
		var number int64
		if err := tx.QueryRow(ctx, `UPDATE note_heads SET head = head + 1 WHERE org = $1 RETURNING head`, h.Org).Scan(&number); err != nil {
			return nil, err
		}
		note, err := json.Marshal(d.Contract(h.Org, number, number-1, edition))
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO note_log (org, edition, number, at, kind, body) VALUES ($1, $2, $3, $4, $5, $6)`,
			h.Org, edition, number, d.At, d.Kind, string(note)); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO note_due (org) VALUES ($1) ON CONFLICT DO NOTHING`, h.Org); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// take applies changes of one organization in one transaction, under its lock.
func (f *feed) take(ctx context.Context, org string, raws []json.RawMessage, heads []head) (*refusal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.state(org)
	var answer *refusal
	err := orgdb.InOrg(ctx, orgdb.Single{Pool: f.pool}, orgID(org), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, org); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO note_heads (org) VALUES ($1) ON CONFLICT DO NOTHING`, org); err != nil {
			return err
		}
		if err := f.catchUp(ctx, tx, org, s); err != nil {
			return err
		}
		for i, raw := range raws {
			r, err := f.apply(ctx, tx, s, heads[i], raw)
			if err != nil {
				return err
			}
			if r != nil {
				answer = r
				break
			}
		}
		return nil
	})
	if err != nil {
		delete(f.orgs, org) // memory may be ahead of what was saved: replay it
		return nil, err
	}
	return answer, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *feed) postChanges(w http.ResponseWriter, r *http.Request, batch bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"refused": "unreadable"})
		return
	}
	raws := []json.RawMessage{body}
	if batch {
		var b struct {
			Changes []json.RawMessage `json:"changes"`
		}
		if err := json.Unmarshal(body, &b); err != nil || len(b.Changes) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"refused": "unreadable"})
			return
		}
		raws = b.Changes
	}
	heads := make([]head, len(raws))
	for i, raw := range raws {
		if err := json.Unmarshal(raw, &heads[i]); err != nil || heads[i].Org == "" || heads[i].Org != heads[0].Org {
			writeJSON(w, http.StatusBadRequest, map[string]string{"refused": "one organization per call"})
			return
		}
	}
	answer, err := f.take(r.Context(), heads[0].Org, raws, heads)
	if err != nil {
		slog.Error("apply", "org", heads[0].Org, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot apply"})
		return
	}
	if answer != nil {
		writeJSON(w, answer.status, answer.body)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"applied": true})
}

// view brings this copy up to date and answers from its memory.
func (f *feed) view(w http.ResponseWriter, r *http.Request, fn func(*orgState, pgx.Tx) (any, error)) {
	org := r.PathValue("org")
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.state(org)
	var out any
	err := orgdb.InOrg(r.Context(), orgdb.Single{Pool: f.pool}, orgID(org), func(tx pgx.Tx) error {
		if err := f.catchUp(r.Context(), tx, org, s); err != nil {
			return err
		}
		var err error
		out, err = fn(s, tx)
		return err
	})
	if err != nil {
		delete(f.orgs, org)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (f *feed) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /changes", func(w http.ResponseWriter, r *http.Request) { f.postChanges(w, r, false) })
	mux.HandleFunc("POST /changes/batch", func(w http.ResponseWriter, r *http.Request) { f.postChanges(w, r, true) })
	mux.HandleFunc("GET /orgs/{org}/place", func(w http.ResponseWriter, r *http.Request) {
		f.view(w, r, func(s *orgState, _ pgx.Tx) (any, error) {
			return contract.FeedPlace{Through: s.at, Edition: edition}, nil
		})
	})
	mux.HandleFunc("GET /orgs/{org}/people/{person}/rows", func(w http.ResponseWriter, r *http.Request) {
		person := r.PathValue("person")
		f.view(w, r, func(s *orgState, tx pgx.Tx) (any, error) {
			var number int64
			if err := tx.QueryRow(r.Context(), `SELECT coalesce(max(number), 0) FROM standin_updates WHERE org = $1 AND person = $2`,
				r.PathValue("org"), person).Scan(&number); err != nil {
				return nil, err
			}
			rows := s.engine.Rows(person)
			if rows == nil {
				rows = []contract.FeedRow{}
			}
			return contract.FeedRowsAnswer{Edition: edition, Through: s.at, Number: number, Rows: rows, Counts: s.engine.Counts(person)}, nil
		})
	})
	mux.HandleFunc("GET /orgs/{org}/people/{person}/history", func(w http.ResponseWriter, r *http.Request) {
		f.view(w, r, func(s *orgState, _ pgx.Tx) (any, error) {
			return s.engine.History(r.PathValue("person"), contract.HistoryTab(r.URL.Query().Get("tab"))), nil
		})
	})
	mux.HandleFunc("GET /orgs/{org}/people/{person}/settings", func(w http.ResponseWriter, r *http.Request) {
		f.view(w, r, func(s *orgState, _ pgx.Tx) (any, error) { return s.engine.Settings(r.PathValue("person")), nil })
	})
	mux.HandleFunc("GET /orgs/{org}/people/{person}/preview", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusServiceUnavailable, contract.NotAvailable{NotAvailable: "the stand-in feed has no preview"})
	})
	mux.HandleFunc("GET /orgs/{org}/updates", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("edition") != strconv.FormatInt(edition, 10) {
			writeJSON(w, http.StatusOK, contract.NewEdition{NewEdition: edition})
			return
		}
		after, _ := strconv.ParseInt(q.Get("after"), 10, 64)
		limit, err := strconv.Atoi(q.Get("limit"))
		if err != nil || limit <= 0 || limit > 500 {
			limit = 200
		}
		f.view(w, r, func(_ *orgState, tx pgx.Tx) (any, error) {
			rows, err := tx.Query(r.Context(), `SELECT number, person, after_number, parts FROM standin_updates
				WHERE org = $1 AND number > $2 ORDER BY number LIMIT $3`, r.PathValue("org"), after, limit)
			if err != nil {
				return nil, err
			}
			updates, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (contract.RowUpdate, error) {
				var u contract.RowUpdate
				var parts string
				if err := row.Scan(&u.Number, &u.Person, &u.After, &parts); err != nil {
					return u, err
				}
				return u, json.Unmarshal([]byte(parts), &u.Parts)
			})
			if err != nil {
				return nil, err
			}
			if updates == nil {
				updates = []contract.RowUpdate{}
			}
			page := contract.RowUpdatesPage{Updates: updates}
			if len(updates) == limit {
				next := updates[len(updates)-1].Number
				page.Next = &next
			}
			return page, nil
		})
	})
	return mux
}

func main() {
	ctx := context.Background()
	db, err := orgdb.Open(ctx)
	if err == nil {
		err = migrate.Apply(ctx, db.Pool, migrations.Files, "feed")
	}
	if err == nil {
		_, err = db.Pool.Exec(ctx, tables)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("cannot start", "part", "standinfeed", "err", err)
		os.Exit(1)
	}
	part.Serve("feed", (&feed{pool: db.Pool, orgs: map[string]*orgState{}}).routes())
}
