package standins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

// PageSize is how many results one search answer holds.
const PageSize = 20

// Search is the search index stand-in.
type Search struct {
	opt   Options
	modes *modes
}

// NewSearch returns the index.
func NewSearch(o Options) *Search { return &Search{opt: o, modes: newModes()} }

// Routes serves the index's side of its contracts.
func (s *Search) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /changes", s.change)
	mux.HandleFunc("GET /orgs/{org}/query", s.query)
	mux.HandleFunc("GET /orgs/{org}/place", s.place)
	controlRoutes(mux, s.modes, s.opt.Dir, s.wipeOrg, "messages", "state")
	return mux
}

type searchState struct {
	through, wipe int64
	refillHigh    *int64
}

func lockSearchState(ctx context.Context, tx pgx.Tx, org ids.Org) (searchState, error) {
	// One statement, so a delete between two steps cannot leave it with no row.
	var st searchState
	err := tx.QueryRow(ctx, `INSERT INTO state (org) VALUES ($1)
		ON CONFLICT (org) DO UPDATE SET org = EXCLUDED.org
		RETURNING through, wipe, refill_high`, string(org)).Scan(&st.through, &st.wipe, &st.refillHigh)
	return st, err
}

// findSearchState locks the row if there is one, and creates nothing: only a
// delivered change makes an organization known here.
func findSearchState(ctx context.Context, tx pgx.Tx, org ids.Org) (searchState, bool, error) {
	var st searchState
	err := tx.QueryRow(ctx, "SELECT through, wipe, refill_high FROM state WHERE org = $1 FOR UPDATE", string(org)).
		Scan(&st.through, &st.wipe, &st.refillHigh)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, false, nil
	}
	return st, err == nil, err
}

// nextThrough moves the place. Normally it only rises. After a wipe the worker
// comes back from the start, so the place follows what arrives until the
// replay has passed where it stood: a copy still in flight from before the
// wipe cannot claim the index is further along than it is.
func nextThrough(st searchState, n int64) (through int64, refillHigh *int64) {
	if st.refillHigh == nil {
		return max(st.through, n), nil
	}
	if n >= *st.refillHigh && st.through > 0 && n > st.through {
		return n, nil
	}
	return n, st.refillHigh
}

type wireChange struct {
	Kind    string                 `json:"kind"`
	Org     string                 `json:"org"`
	Number  int64                  `json:"number"`
	Message contract.SearchMessage `json:"message"`
}

func (c wireChange) problem() string {
	m := c.Message
	switch c.Kind {
	case "message.created", "message.changed":
		if m.Number < 1 || m.Conversation == "" || m.Author == "" {
			return "a message needs a number, a conversation and an author"
		}
	case "message.deleted":
	default:
		return "unknown kind"
	}
	if c.Number < 1 || m.Id == "" || m.Version < 1 {
		return "a change needs a number, a message and a version"
	}
	if strings.ContainsRune(m.Text, 0) {
		return "text holds a NUL character"
	}
	return ""
}

func (s *Search) change(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	var c wireChange
	decodeErr := json.Unmarshal(body, &c)
	if !s.modes.gate(r.Context(), s.opt.Clock, c.Org) {
		return
	}
	org, orgErr := ids.ParseOrg(c.Org)
	if decodeErr != nil || orgErr != nil {
		writeJSON(w, http.StatusUnprocessableEntity, contract.SearchRefusal{Refused: "not a valid change", Wipe: 0})
		return
	}
	ctx := r.Context()
	var status int
	var answer any
	err = orgdb.InOrg(ctx, s.opt.Dir, org, func(tx pgx.Tx) error {
		st, err := lockSearchState(ctx, tx, org)
		if err != nil {
			return err
		}
		why := c.problem()
		if why == "" && s.opt.Refuse != nil {
			why = s.opt.Refuse(c.Org, c.Number)
		}
		if why != "" {
			status, answer = http.StatusUnprocessableEntity, contract.SearchRefusal{Refused: why, Wipe: st.wipe}
			return nil
		}
		applied, err := applyChange(ctx, tx, org, c)
		if err != nil {
			return err
		}
		through, refill := nextThrough(st, c.Number)
		if _, err := tx.Exec(ctx, "UPDATE state SET through = $2, refill_high = $3 WHERE org = $1",
			string(org), through, refill); err != nil {
			return err
		}
		a := contract.SearchApplied{Applied: true, Wipe: st.wipe}
		if !applied {
			already := true
			a.Already = &already
		}
		status, answer = http.StatusOK, a
		return nil
	})
	if err != nil {
		unavailable(w, "search change", err)
		return
	}
	writeJSON(w, status, answer)
}

// applyChange is the version rule: a change is applied only if it is newer than
// what is held, and a delete is final whatever arrives after it.
func applyChange(ctx context.Context, tx pgx.Tx, org ids.Org, c wireChange) (bool, error) {
	m := c.Message
	var have int64
	var deleted bool
	err := tx.QueryRow(ctx, "SELECT version, deleted FROM messages WHERE org = $1 AND id = $2", string(org), m.Id).Scan(&have, &deleted)
	exists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if c.Kind == "message.deleted" {
		if exists && deleted && !fault(FaultNoVersion) {
			return false, nil
		}
		_, err := tx.Exec(ctx, `INSERT INTO messages (org, id, number, version, deleted) VALUES ($1, $2, $3, $4, true)
			ON CONFLICT (org, id) DO UPDATE SET version = GREATEST(messages.version, EXCLUDED.version), deleted = true, body = '', folded = ''`,
			string(org), m.Id, c.Number, m.Version)
		return err == nil, err
	}
	if exists && !fault(FaultNoVersion) && (deleted || m.Version <= have) {
		return false, nil
	}
	_, err = tx.Exec(ctx, `INSERT INTO messages (org, id, number, version, conversation, thread, author, edited, participants, created_at, body, folded, deleted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, false)
		ON CONFLICT (org, id) DO UPDATE SET number = EXCLUDED.number, version = EXCLUDED.version, conversation = EXCLUDED.conversation,
			thread = EXCLUDED.thread, author = EXCLUDED.author, edited = EXCLUDED.edited, participants = EXCLUDED.participants,
			created_at = EXCLUDED.created_at, body = EXCLUDED.body, folded = EXCLUDED.folded, deleted = false`,
		string(org), m.Id, m.Number, m.Version, m.Conversation, m.Thread, m.Author, m.Edited, m.Participants, m.CreatedAt, m.Text, fold(m.Text))
	return err == nil, err
}

// fold lowers text one code point at a time, so a position in the result is the
// same position in the original.
func fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// Spans marks where any term occurs in text, in code points from 0, sorted,
// with overlapping marks joined.
func Spans(text string, terms []string) []contract.Span {
	hay := []rune(fold(text))
	var found [][2]int64
	for _, t := range terms {
		needle := []rune(fold(t))
		if len(needle) == 0 {
			continue
		}
		for i := 0; i+len(needle) <= len(hay); i++ {
			match := true
			for j := range needle {
				if hay[i+j] != needle[j] {
					match = false
					break
				}
			}
			if match {
				found = append(found, [2]int64{int64(i), int64(i + len(needle))})
			}
		}
	}
	sort.Slice(found, func(a, b int) bool { return found[a][0] < found[b][0] })
	out := []contract.Span{}
	for _, f := range found {
		if n := len(out); n > 0 && f[0] <= out[n-1][1] {
			out[n-1][1] = max(out[n-1][1], f[1])
			continue
		}
		out = append(out, contract.Span{f[0], f[1]})
	}
	return out
}

func (s *Search) query(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok || !s.modes.gate(r.Context(), s.opt.Clock, string(org)) {
		return
	}
	as := r.URL.Query().Get("as")
	if as == "" {
		refuse(w, http.StatusBadRequest, "as is required")
		return
	}
	var before *int64
	if v := r.URL.Query().Get("after"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			refuse(w, http.StatusBadRequest, "after is not a cursor from an answer")
			return
		}
		before = &n
	}
	terms := strings.Fields(r.URL.Query().Get("q"))
	ctx := r.Context()
	answer := contract.SearchAnswer{Results: []contract.SearchResult{}}
	err := orgdb.InOrg(ctx, s.opt.Dir, org, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, "SELECT COALESCE((SELECT through FROM state WHERE org = $1), 0)", string(org)).Scan(&answer.Through); err != nil {
			return err
		}
		if len(terms) == 0 {
			return nil
		}
		where := "org = $1 AND NOT deleted AND (participants IS NULL OR $2 = ANY(participants))"
		args := []any{string(org), as}
		for _, t := range terms {
			args = append(args, fold(t))
			where += fmt.Sprintf(" AND strpos(folded, $%d) > 0", len(args))
		}
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM messages WHERE "+where, args...).Scan(&answer.Total); err != nil {
			return err
		}
		if before != nil {
			args = append(args, *before)
			where += fmt.Sprintf(" AND number < $%d", len(args))
		}
		rows, err := tx.Query(ctx, `SELECT id, number, created_at, version, conversation, thread, author, edited, body
			FROM messages WHERE `+where+fmt.Sprintf(" ORDER BY number DESC LIMIT %d", PageSize+1), args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var x contract.SearchResult
			if err := rows.Scan(&x.Message, &x.Number, &x.CreatedAt, &x.Version, &x.Conversation, &x.Thread, &x.Author, &x.Edited, &x.Text); err != nil {
				return err
			}
			x.Match = Spans(x.Text, terms)
			answer.Results = append(answer.Results, x)
		}
		return rows.Err()
	})
	if err != nil {
		unavailable(w, "search query", err)
		return
	}
	if len(answer.Results) > PageSize {
		answer.Results = answer.Results[:PageSize]
		next := strconv.FormatInt(answer.Results[PageSize-1].Number, 10)
		answer.Next = &next
	}
	writeJSON(w, http.StatusOK, answer)
}

func (s *Search) place(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok || !s.modes.gate(r.Context(), s.opt.Clock, string(org)) {
		return
	}
	var p contract.SearchPlace
	err := orgdb.InOrg(r.Context(), s.opt.Dir, org, func(tx pgx.Tx) error {
		st, _, err := findSearchState(r.Context(), tx, org)
		p = contract.SearchPlace{Through: st.through, Wipe: st.wipe}
		return err
	})
	if err != nil {
		unavailable(w, "search place", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Search) wipeOrg(ctx context.Context, org ids.Org) error {
	return orgdb.InOrg(ctx, s.opt.Dir, org, func(tx pgx.Tx) error {
		st, found, err := findSearchState(ctx, tx, org)
		if err != nil || !found {
			return err // nothing was ever delivered, or it was deleted: nothing to wipe
		}
		if _, err := tx.Exec(ctx, "DELETE FROM messages WHERE org = $1", string(org)); err != nil {
			return err
		}
		high := st.through
		if st.refillHigh != nil {
			high = max(high, *st.refillHigh)
		}
		var refill *int64
		if high > 0 {
			refill = &high
		}
		_, err = tx.Exec(ctx, "UPDATE state SET through = 0, wipe = wipe + 1, refill_high = $2 WHERE org = $1", string(org), refill)
		return err
	})
}
