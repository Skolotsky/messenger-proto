package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"contexia/internal/feed/orgdelete"
	"contexia/internal/feed/rules"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

const (
	historyPage    = 50
	updatesDefault = 200
	updatesMost    = 1000
)

func (s *Service) place(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	s.modes.gate(r.Context(), s.opt.Clock, string(org))
	var st state
	err := orgdb.InOrg(r.Context(), s.opt.Dir, org, func(tx pgx.Tx) error {
		var err error
		st, err = readState(r.Context(), tx, org, false)
		return err
	})
	if err != nil {
		unavailable(w, "place", err)
		return
	}
	writeJSON(w, http.StatusOK, contract.FeedPlace{Through: st.through, Edition: st.edition})
}

// rows answers from the saved rows alone.
func (s *Service) rows(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	s.modes.gate(r.Context(), s.opt.Clock, string(org))
	ctx := r.Context()
	person := r.PathValue("person")
	out := contract.FeedRowsAnswer{Rows: []contract.FeedRow{}}
	err := orgdb.InOrg(ctx, s.opt.Dir, org, func(tx pgx.Tx) error {
		// The place is read first: the answer holds everything up to it and
		// possibly later changes, as door-reads.md allows.
		st, err := readState(ctx, tx, org, false)
		if err != nil {
			return err
		}
		out.Edition, out.Through = st.edition, st.through
		var counts string
		err = tx.QueryRow(ctx, `SELECT number, counts FROM feed_readers WHERE org = $1 AND person = $2`, string(org), person).Scan(&out.Number, &counts)
		switch {
		case err == pgx.ErrNoRows:
		case err != nil:
			return err
		default:
			if err := json.Unmarshal([]byte(counts), &out.Counts); err != nil {
				return err
			}
		}
		rows, err := tx.Query(ctx, `SELECT body FROM feed_rows WHERE org = $1 AND person = $2 ORDER BY section, last DESC, row_key`, string(org), person)
		if err != nil {
			return err
		}
		bodies, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		for _, b := range bodies {
			var fr contract.FeedRow
			if err := json.Unmarshal([]byte(b), &fr); err != nil {
				return err
			}
			out.Rows = append(out.Rows, fr)
		}
		return nil
	})
	if err != nil {
		unavailable(w, "rows", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// withRules brings this copy's rules up to the store's place and asks them.
func (s *Service) withRules(ctx context.Context, org ids.Org, ask func(*rules.Engine, *mem)) error {
	m := s.memFor(org)
	m.mu.Lock()
	defer m.mu.Unlock()
	err := orgdb.InOrg(ctx, s.opt.Dir, org, func(tx pgx.Tx) error {
		st, err := readState(ctx, tx, org, false)
		if err != nil {
			return err
		}
		if !st.found {
			m.drop()
			ask(rules.New(string(org)), m)
			return nil
		}
		if err := s.sync(ctx, tx, m, st, nil); err != nil {
			return err
		}
		ask(m.e, m)
		return nil
	})
	if err != nil {
		m.drop()
	}
	return err
}

func validTab(t contract.HistoryTab) bool {
	return t == "direct" || t == "mentions" || t == "threads" || t == "reactions"
}

func (s *Service) history(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	tab := contract.HistoryTab(r.URL.Query().Get("tab"))
	if !validTab(tab) {
		refuse(w, http.StatusBadRequest, "tab is direct, mentions, threads or reactions")
		return
	}
	var before int64 = -1
	if v := r.URL.Query().Get("after"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			refuse(w, http.StatusBadRequest, "not a cursor")
			return
		}
		before = n
	}
	s.modes.gate(r.Context(), s.opt.Clock, string(org))
	var out contract.HistoryAnswer
	err := s.withRules(r.Context(), org, func(e *rules.Engine, _ *mem) {
		out = e.History(r.PathValue("person"), tab)
	})
	if err != nil {
		unavailable(w, "history", err)
		return
	}
	// Entries are newest first; the cursor is the number of the last one shown.
	page := make([]contract.HistoryEntry, 0, historyPage)
	var next *string
	for _, en := range out.Entries {
		if before >= 0 && en.Number >= before {
			continue
		}
		if len(page) == historyPage {
			c := strconv.FormatInt(page[len(page)-1].Number, 10)
			next = &c
			break
		}
		page = append(page, en)
	}
	out.Entries, out.Next = page, next
	writeJSON(w, http.StatusOK, out)
}

func (s *Service) settings(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	s.modes.gate(r.Context(), s.opt.Clock, string(org))
	var out contract.SettingsAnswer
	err := s.withRules(r.Context(), org, func(e *rules.Engine, _ *mem) {
		out = e.Settings(r.PathValue("person"))
	})
	if err != nil {
		unavailable(w, "settings", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// preview answers from the rules; "now" is the time of the last change the feed
// took, since the feed reads no clock for what it decides.
func (s *Service) preview(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	var act contract.RowAct
	if err := json.Unmarshal([]byte(r.URL.Query().Get("change")), &act); err != nil {
		refuse(w, http.StatusBadRequest, "change is a row action as JSON")
		return
	}
	s.modes.gate(r.Context(), s.opt.Clock, string(org))
	out := contract.PreviewAnswer{}
	err := s.withRules(r.Context(), org, func(e *rules.Engine, m *mem) {
		if a, err := e.Preview(r.PathValue("person"), act, m.lastAt); err == nil {
			out = a
		}
	})
	if err != nil {
		unavailable(w, "preview", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Service) updates(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	after, err := strconv.ParseInt(q.Get("after"), 10, 64)
	if err != nil || after < 0 {
		refuse(w, http.StatusBadRequest, "after is a row-log number")
		return
	}
	edition, err := strconv.ParseInt(q.Get("edition"), 10, 64)
	if err != nil {
		refuse(w, http.StatusBadRequest, "edition is a number")
		return
	}
	limit := updatesDefault
	if v := q.Get("limit"); v != "" {
		if limit, err = strconv.Atoi(v); err != nil || limit <= 0 {
			refuse(w, http.StatusBadRequest, "limit is a count")
			return
		}
		limit = min(limit, updatesMost)
	}
	s.modes.gate(r.Context(), s.opt.Clock, string(org))
	ctx := r.Context()
	page := contract.RowUpdatesPage{Updates: []contract.RowUpdate{}}
	var moved *contract.NewEdition
	err = orgdb.InOrg(ctx, s.opt.Dir, org, func(tx pgx.Tx) error {
		st, err := readState(ctx, tx, org, false)
		if err != nil {
			return err
		}
		if st.edition != edition {
			moved = &contract.NewEdition{NewEdition: st.edition}
			return nil
		}
		rows, err := tx.Query(ctx, `SELECT number, person, after, parts FROM feed_row_updates
			WHERE org = $1 AND number > $2 ORDER BY number LIMIT $3`, string(org), after, limit+1)
		if err != nil {
			return err
		}
		type raw struct {
			u     contract.RowUpdate
			parts string
		}
		list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (raw, error) {
			var x raw
			err := r.Scan(&x.u.Number, &x.u.Person, &x.u.After, &x.parts)
			return x, err
		})
		if err != nil {
			return err
		}
		if len(list) > limit {
			list = list[:limit]
			n := list[limit-1].u.Number
			page.Next = &n
		}
		for _, x := range list {
			if err := json.Unmarshal([]byte(x.parts), &x.u.Parts); err != nil {
				return err
			}
			page.Updates = append(page.Updates, x.u)
		}
		return nil
	})
	if err != nil {
		unavailable(w, "updates", err)
		return
	}
	if moved != nil {
		writeJSON(w, http.StatusOK, moved)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Service) deleteOrg(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	if err := orgdelete.Delete(r.Context(), s.opt.Dir, org); err != nil {
		unavailable(w, "delete organization", err)
		return
	}
	s.forget(org)
	writeJSON(w, http.StatusOK, contract.Empty{})
}

func readMode(w http.ResponseWriter, r *http.Request) (contract.Mode, bool) {
	var in contract.ModeBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || (in.Mode != "running" && in.Mode != "slow" && in.Mode != "off") {
		refuse(w, http.StatusBadRequest, "mode is running, slow or off")
		return "", false
	}
	return in.Mode, true
}

func (s *Service) modeRead(w http.ResponseWriter, r *http.Request) {
	if org, ok := orgOf(w, r); ok {
		writeJSON(w, http.StatusOK, contract.ModeBody{Mode: s.modes.get(string(org))})
	}
}

func (s *Service) modeSet(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	if v, ok := readMode(w, r); ok {
		s.modes.set(string(org), v)
		writeJSON(w, http.StatusOK, contract.Empty{})
	}
}

func (s *Service) modeSetAll(w http.ResponseWriter, r *http.Request) {
	if v, ok := readMode(w, r); ok {
		s.modes.setAll(v)
		writeJSON(w, http.StatusOK, contract.Empty{})
	}
}

// wipeBody is the wipe's request. The contract's body is empty; rules_changed is
// how a rebuild made because the rules changed reaches the feed (D-034).
func wipeBody(r *http.Request) bool {
	raw, err := io.ReadAll(r.Body)
	if err != nil || len(raw) == 0 {
		return false
	}
	var in struct {
		RulesChanged bool `json:"rules_changed"`
	}
	return json.Unmarshal(raw, &in) == nil && in.RulesChanged
}

func (s *Service) wipe(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	if err := s.wipeOrg(r.Context(), org, wipeBody(r)); err != nil {
		unavailable(w, "wipe", err)
		return
	}
	writeJSON(w, http.StatusOK, contract.Empty{})
}

func (s *Service) wipeAll(w http.ResponseWriter, r *http.Request) {
	rulesChanged := wipeBody(r)
	var orgs []string
	err := orgdb.InOrg(r.Context(), s.opt.Dir, "bookkeeping-only", func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `SELECT org FROM feed_state ORDER BY org`)
		if err != nil {
			return err
		}
		orgs, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	for _, o := range orgs {
		if err == nil {
			err = s.wipeOrg(r.Context(), ids.Org(o), rulesChanged)
		}
	}
	if err != nil {
		unavailable(w, "wipe all", err)
		return
	}
	writeJSON(w, http.StatusOK, contract.Empty{})
}

// wipeOrg removes everything the feed holds for an organization and raises its
// edition, so the sender and the screens start again. An ordinary rebuild keeps
// "never lost"; one after a rule change marks what it finds up to the old place
// as history (D-034).
func (s *Service) wipeOrg(ctx context.Context, org ids.Org, rulesChanged bool) error {
	m := s.memFor(org)
	m.mu.Lock()
	defer m.mu.Unlock()
	err := orgdb.InOrg(ctx, s.opt.Dir, org, func(tx pgx.Tx) error {
		st, err := readState(ctx, tx, org, true)
		if err != nil || !st.found {
			return err // nothing was ever delivered, or it was deleted: nothing to wipe
		}
		for _, table := range []string{"feed_rows", "feed_readers", "feed_held", "feed_row_updates", "feed_fences", "feed_journal", "note_log"} {
			if _, err := tx.Exec(ctx, "DELETE FROM "+table+" WHERE org = $1", string(org)); err != nil {
				return err
			}
		}
		history := st.historyThrough
		if rulesChanged {
			history = max(history, st.through)
		}
		if _, err := tx.Exec(ctx, `UPDATE note_heads SET edition = $2, head = 0 WHERE org = $1`, string(org), st.edition+1); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE feed_state SET edition = $2, through = 0, row_head = 0, history_through = $3, stopped = false WHERE org = $1`,
			string(org), st.edition+1, history)
		return err
	})
	m.drop()
	return err
}
