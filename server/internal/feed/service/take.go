package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"contexia/internal/feed/rules"
	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

// How a change was handled, as the journal remembers it.
const (
	dispApplied  = "applied"
	dispCounted  = "counted"   // a kind this version does not know (contract, section 5)
	dispSetAside = "set_aside" // inside a fence
	dispFenced   = "fenced"    // could not be applied, and raised a fence
)

// incoming is one change as delivered: the envelope every change has, and what
// says which conversation and reader it belongs to.
type incoming struct {
	raw                  []byte
	org                  string
	number, previous     int64
	at, kind             string
	conversation, person string
}

func parseIncoming(raw []byte) (incoming, error) {
	var v struct {
		Org          string          `json:"org"`
		Number       int64           `json:"number"`
		Previous     int64           `json:"previous"`
		At           string          `json:"at"`
		Kind         string          `json:"kind"`
		Conversation string          `json:"conversation"`
		Person       string          `json:"person"`
		Message      json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return incoming{}, err
	}
	if v.Number <= 0 || v.Previous < 0 || v.Kind == "" {
		return incoming{}, errors.New("not a change")
	}
	c := incoming{raw: raw, org: v.Org, number: v.Number, previous: v.Previous, at: v.At, kind: v.Kind, conversation: v.Conversation, person: v.Person}
	if c.conversation == "" && len(v.Message) > 0 && v.Message[0] == '{' {
		var m struct {
			Conversation string `json:"conversation"`
		}
		if json.Unmarshal(v.Message, &m) == nil {
			c.conversation = m.Conversation
		}
	}
	return c, nil
}

// scope is the fence a fault on this change raises. No conversation means no
// smaller fence can hold it: the organization stops (D-032).
func (c incoming) scope() (fenceScope, bool) {
	if c.conversation == "" {
		return fenceScope{}, false
	}
	if strings.HasPrefix(c.kind, "reader.") {
		return fenceScope{c.conversation, c.person}, true
	}
	return fenceScope{c.conversation, ""}, true
}

func (m *mem) inFence(c incoming) bool {
	if c.conversation == "" {
		return false
	}
	if m.fences[fenceScope{c.conversation, ""}] {
		return true
	}
	return strings.HasPrefix(c.kind, "reader.") && m.fences[fenceScope{c.conversation, c.person}]
}

func (m *mem) rowFenced(person, conversation string) bool {
	return m.fences[fenceScope{conversation, ""}] || m.fences[fenceScope{conversation, person}]
}

// holdsNUL says whether a decoded value carries a NUL character, which the note
// log cannot store: such a change cannot be applied.
func holdsNUL(raw []byte) bool {
	if !bytes.Contains(raw, []byte(`\u0000`)) {
		return false
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return true
	}
	var walk func(any) bool
	walk = func(x any) bool {
		switch t := x.(type) {
		case string:
			return strings.ContainsRune(t, 0)
		case []any:
			for _, y := range t {
				if walk(y) {
					return true
				}
			}
		case map[string]any:
			for k, y := range t {
				if strings.ContainsRune(k, 0) || walk(y) {
					return true
				}
			}
		}
		return false
	}
	return walk(v)
}

// state is the organization's row in feed_state.
type state struct {
	found                                     bool
	edition, through, rowHead, historyThrough int64
	stopped                                   bool
}

func readState(ctx context.Context, tx pgx.Tx, org ids.Org, lock bool) (state, error) {
	q := `SELECT edition, through, row_head, history_through, stopped FROM feed_state WHERE org = $1`
	if lock {
		q += ` FOR UPDATE`
	}
	st := state{found: true}
	err := tx.QueryRow(ctx, q, string(org)).Scan(&st.edition, &st.through, &st.rowHead, &st.historyThrough, &st.stopped)
	if errors.Is(err, pgx.ErrNoRows) {
		return state{edition: 1}, nil
	}
	return st, err
}

// create makes the organization's state on its first change. The edition is the
// note log's, so a state wiped by a delete can never reuse a number.
func createState(ctx context.Context, tx pgx.Tx, org ids.Org) (state, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO note_heads (org) VALUES ($1) ON CONFLICT DO NOTHING`, string(org)); err != nil {
		return state{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO feed_state (org, edition)
		SELECT org, edition FROM note_heads WHERE org = $1 ON CONFLICT DO NOTHING`, string(org)); err != nil {
		return state{}, err
	}
	return readState(ctx, tx, org, true)
}

const replayPage = 1000

// sync brings the copy's rules up to the store's place by replaying the journal.
// keepSummaries names readers whose counts this transaction has yet to report.
func (s *Service) sync(ctx context.Context, tx pgx.Tx, m *mem, st state, keepSummaries map[string]bool) error {
	if fault(FaultNoCatchUp) && m.e != nil && m.edition == st.edition {
		return nil
	}
	if m.e == nil || m.edition != st.edition || m.through > st.through || m.historyThrough != st.historyThrough {
		m.drop()
		m.e = rules.New(string(m.org))
		m.edition, m.historyThrough = st.edition, st.historyThrough
		m.fences, m.touched = map[fenceScope]bool{}, map[string]bool{}
	}
	if m.through < st.through {
		m.cachesRead = false // another copy wrote: what this one remembers of the store is old
	}
	for m.through < st.through {
		rows, err := tx.Query(ctx, `SELECT number, disposition, fence_conversation, fence_person, body FROM feed_journal
			WHERE org = $1 AND number > $2 AND number <= $3 ORDER BY number LIMIT $4`, string(m.org), m.through, st.through, replayPage)
		if err != nil {
			return err
		}
		type entry struct {
			number                    int64
			disposition, fc, fp, body string
		}
		page, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (entry, error) {
			var e entry
			err := r.Scan(&e.number, &e.disposition, &e.fc, &e.fp, &e.body)
			return e, err
		})
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return fmt.Errorf("feed journal of %s ends at %d, its place is %d", m.org, m.through, st.through)
		}
		for _, e := range page {
			switch e.disposition {
			case dispApplied:
				var c contract.Change
				if err := json.Unmarshal([]byte(e.body), &c); err != nil {
					return fmt.Errorf("journal entry %d: %w", e.number, err)
				}
				fx, err := m.apply(c, e.number)
				if err != nil {
					return fmt.Errorf("replay of %d: %w", e.number, err)
				}
				for _, u := range fx.Touched {
					m.touched[u] = true
				}
			case dispFenced:
				f := fenceScope{e.fc, e.fp}
				m.fences[f] = true
				if !fault(FaultFenceHeld) {
					m.e.Fence(f.conversation, f.person)
				}
			}
			if c, err := parseIncoming([]byte(e.body)); err == nil {
				m.noteTime(c.at)
			}
			m.through = e.number
		}
	}
	for u := range m.touched {
		if !keepSummaries[u] {
			m.e.Summaries(u)
		}
	}
	m.touched = map[string]bool{}
	return nil
}

func (m *mem) noteTime(at string) {
	if t, err := time.Parse(time.RFC3339Nano, at); err == nil && t.After(m.lastAt) {
		m.lastAt = t
	}
}

// apply runs the rules on one change and keeps every fence's promise: nothing
// inside one stays on hold. A panic in the rules counts as a change that cannot
// be applied; the caller then replays, so no half-applied state survives.
func (m *mem) apply(c contract.Change, number int64) (fx rules.Effects, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: panic: %v", rules.ErrCannotApply, r)
		}
	}()
	fx, err = m.e.Apply(c, rules.Options{NoSummaries: true, History: number <= m.historyThrough})
	if err == nil && !fault(FaultFenceHeld) {
		for f := range m.fences {
			m.e.Fence(f.conversation, f.person)
		}
	}
	return fx, err
}

// work is what one call has done and has yet to save.
type work struct {
	journal  []journalRow
	notes    []rules.NoteDraft
	updates  []*rowUpdate
	touched  map[string]bool
	allDirty bool
	changed  bool
}

type journalRow struct {
	number      int64
	disposition string
	fence       fenceScope
	body        string
}

type rowUpdate struct {
	person string
	parts  []contract.FeedPart
}

func (w *work) flushJournal(ctx context.Context, tx pgx.Tx, org ids.Org) error {
	if len(w.journal) == 0 {
		return nil
	}
	b := &pgx.Batch{}
	for _, j := range w.journal {
		b.Queue(`INSERT INTO feed_journal (org, number, disposition, fence_conversation, fence_person, body)
			VALUES ($1, $2, $3, $4, $5, $6)`, string(org), j.number, j.disposition, j.fence.conversation, j.fence.person, j.body)
	}
	w.journal = nil
	return tx.SendBatch(ctx, b).Close()
}

// takeIn applies the changes in order inside the transaction. It answers with
// the first change that is not answered 200, or with what happened to them all.
func (s *Service) takeIn(ctx context.Context, tx pgx.Tx, m *mem, st *state, w *work, list []incoming) (int, any, error) {
	synced, fresh, fenced := false, true, false
	for _, c := range list {
		if c.number <= st.through {
			continue
		}
		fresh = false
		if st.stopped {
			return http.StatusLocked, contract.OrganizationStopped{OrganizationStopped: string(m.org)}, nil
		}
		if c.previous != st.through && !fault(FaultAcceptNewer) {
			return http.StatusConflict, contract.IAmAt{IAmAt: st.through}, nil
		}
		if !st.found {
			created, err := createState(ctx, tx, m.org)
			if err != nil {
				return 0, nil, err
			}
			*st = created
		}
		if !synced {
			if err := s.sync(ctx, tx, m, *st, w.touched); err != nil {
				return 0, nil, err
			}
			synced = true
		}
		if m.inFence(c) && !fault(FaultNoSetAside) {
			w.journal = append(w.journal, journalRow{number: c.number, disposition: dispSetAside, body: string(c.raw)})
			st.through, w.changed, fenced = c.number, true, true
			m.through = c.number
			m.noteTime(c.at)
			continue
		}
		var change contract.Change
		decodeErr := json.Unmarshal(c.raw, &change)
		if decodeErr != nil && strings.Contains(decodeErr.Error(), "unknown kind") {
			w.journal = append(w.journal, journalRow{number: c.number, disposition: dispCounted, body: string(c.raw)})
			st.through, w.changed = c.number, true
			m.through = c.number
			m.noteTime(c.at)
			continue
		}
		problem := decodeErr
		if problem == nil && holdsNUL(c.raw) {
			problem = errors.New("holds a NUL character")
		}
		var fx rules.Effects
		if problem == nil {
			fx, problem = m.apply(change, c.number)
		}
		if problem != nil {
			stopped, err := s.raise(ctx, tx, m, st, w, c, problem)
			if err != nil {
				return 0, nil, err
			}
			if stopped {
				return http.StatusLocked, contract.OrganizationStopped{OrganizationStopped: string(m.org)}, nil
			}
			fenced = true
			continue
		}
		w.journal = append(w.journal, journalRow{number: c.number, disposition: dispApplied, body: string(c.raw)})
		st.through, w.changed = c.number, true
		m.through = c.number
		m.noteTime(c.at)
		w.notes = append(w.notes, fx.Notes...)
		for _, u := range fx.Touched {
			w.touched[u] = true
		}
		for _, re := range fx.Readers {
			w.updates = append(w.updates, &rowUpdate{person: re.Person, parts: re.Parts})
		}
	}
	a := contract.ChangeApplied{Applied: true}
	if fresh {
		yes := true
		a.Already = &yes
	}
	if fenced {
		yes := true
		a.Fenced = &yes
	}
	return http.StatusOK, a, nil
}

// raise handles a change the rules cannot apply. The rules are replayed first,
// so nothing of the failed attempt remains. It answers whether the organization
// stopped: a fault with no smaller fence, or more than five fences.
func (s *Service) raise(ctx context.Context, tx pgx.Tx, m *mem, st *state, w *work, c incoming, problem error) (bool, error) {
	if err := w.flushJournal(ctx, tx, m.org); err != nil {
		return false, err
	}
	m.drop()
	if err := s.sync(ctx, tx, m, *st, w.touched); err != nil {
		return false, err
	}
	scope, ok := c.scope()
	if !ok || (len(m.fences) >= maxFences && !fault(FaultNoOrgStop)) {
		st.stopped, w.changed = true, true
		return true, nil
	}
	reason := problem.Error()
	if len(reason) > 300 {
		reason = reason[:300]
	}
	reason = strings.ToValidUTF8(strings.ReplaceAll(reason, "\x00", ""), "")
	if _, err := tx.Exec(ctx, `INSERT INTO feed_fences (org, conversation, person, number, reason)
		VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`, string(m.org), scope.conversation, scope.person, c.number, reason); err != nil {
		return false, err
	}
	m.fences[scope] = true
	if !fault(FaultFenceHeld) {
		m.e.Fence(scope.conversation, scope.person)
	}
	w.journal = append(w.journal, journalRow{number: c.number, disposition: dispFenced, fence: scope, body: string(c.raw)})
	st.through, w.changed, w.allDirty = c.number, true, true
	m.through = c.number
	m.noteTime(c.at)
	return false, nil
}

// loadCaches reads what the store holds, the first time a copy writes after
// replaying.
func (m *mem) loadCaches(ctx context.Context, tx pgx.Tx) error {
	if m.cachesRead {
		return nil
	}
	m.shown, m.heads, m.held = map[string]map[string]string{}, map[string]int64{}, map[string]bool{}
	rows, err := tx.Query(ctx, `SELECT person, row_key, section, last, body FROM feed_rows WHERE org = $1`, string(m.org))
	if err != nil {
		return err
	}
	var person, key, body string
	var section int
	var last int64
	if _, err := pgx.ForEachRow(rows, []any{&person, &key, &section, &last, &body}, func() error {
		if m.shown[person] == nil {
			m.shown[person] = map[string]string{}
		}
		m.shown[person][key] = fingerprint(section, last, body)
		return nil
	}); err != nil {
		return err
	}
	rows, err = tx.Query(ctx, `SELECT person, number FROM feed_readers WHERE org = $1`, string(m.org))
	if err != nil {
		return err
	}
	var number int64
	if _, err := pgx.ForEachRow(rows, []any{&person, &number}, func() error {
		m.heads[person] = number
		return nil
	}); err != nil {
		return err
	}
	rows, err = tx.Query(ctx, `SELECT name FROM feed_held WHERE org = $1`, string(m.org))
	if err != nil {
		return err
	}
	var name string
	if _, err := pgx.ForEachRow(rows, []any{&name}, func() error {
		m.held[name] = true
		return nil
	}); err != nil {
		return err
	}
	m.cachesRead = true
	return nil
}

func fingerprint(section int, last int64, body string) string {
	return fmt.Sprintf("%d|%d|%s", section, last, body)
}

func rowID(conversation string, thread *string) string {
	if thread == nil {
		return conversation + "\x00"
	}
	return conversation + "\x00" + *thread
}

// flush saves what the call did, in the transaction that moves the place.
func (w *work) flush(ctx context.Context, tx pgx.Tx, m *mem, st *state) error {
	if !w.changed {
		return nil
	}
	if err := w.flushJournal(ctx, tx, m.org); err != nil {
		return err
	}
	if m.e != nil {
		if err := w.saveRows(ctx, tx, m, st); err != nil {
			return err
		}
		if err := w.saveHeld(ctx, tx, m); err != nil {
			return err
		}
		if err := w.saveNotes(ctx, tx, m, st); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `UPDATE feed_state SET through = $2, row_head = $3, stopped = $4 WHERE org = $1`,
		string(m.org), st.through, st.rowHead, st.stopped)
	return err
}

func (w *work) saveRows(ctx context.Context, tx pgx.Tx, m *mem, st *state) error {
	if err := m.loadCaches(ctx, tx); err != nil {
		return err
	}
	dirty := map[string]bool{}
	for u := range w.touched {
		dirty[u] = true
		if parts := m.e.Summaries(u); len(parts) > 0 {
			w.updates = append(w.updates, &rowUpdate{person: u, parts: parts})
		}
	}
	if w.allDirty {
		for _, u := range m.e.Readers() {
			dirty[u] = true
		}
	}
	people := make([]string, 0, len(dirty))
	for u := range dirty {
		people = append(people, u)
	}
	sort.Strings(people)

	emitted := map[string]map[string]bool{}
	for _, up := range w.updates {
		for i, part := range up.parts {
			v, ok := part.Value.(contract.FeedPartRowChanged)
			if !ok {
				continue
			}
			if emitted[up.person] == nil {
				emitted[up.person] = map[string]bool{}
			}
			emitted[up.person][rowID(v.Row.Conversation, v.Row.Thread)] = true
			if m.rowFenced(up.person, v.Row.Conversation) {
				v.Row.Fenced = true
				up.parts[i] = contract.FeedPart{Value: v}
			}
		}
	}

	b := &pgx.Batch{}
	for _, u := range people {
		now := map[string]bool{}
		var extra []contract.FeedPart
		for _, k := range m.e.KeyedRows(u) {
			row := k.Row
			row.Fenced = m.rowFenced(u, row.Conversation)
			body, err := json.Marshal(row)
			if err != nil {
				return err
			}
			now[k.Key] = true
			fp := fingerprint(k.Section, k.Last, string(body))
			if m.shown[u][k.Key] == fp {
				continue
			}
			if m.shown[u] == nil {
				m.shown[u] = map[string]string{}
			}
			m.shown[u][k.Key] = fp
			b.Queue(`INSERT INTO feed_rows (org, person, row_key, section, last, body) VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (org, person, row_key) DO UPDATE SET section = $4, last = $5, body = $6`,
				string(m.org), u, k.Key, k.Section, k.Last, string(body))
			if row.Fenced && !emitted[u][rowID(row.Conversation, row.Thread)] {
				extra = append(extra, contract.FeedPart{Value: contract.FeedPartRowChanged{Kind: "row.changed", Row: row}})
			}
		}
		for key := range m.shown[u] {
			if !now[key] {
				delete(m.shown[u], key)
				b.Queue(`DELETE FROM feed_rows WHERE org = $1 AND person = $2 AND row_key = $3`, string(m.org), u, key)
			}
		}
		if len(extra) > 0 {
			w.updates = append(w.updates, &rowUpdate{person: u, parts: extra})
		}
	}
	for _, up := range w.updates {
		parts, err := json.Marshal(up.parts)
		if err != nil {
			return err
		}
		st.rowHead++
		b.Queue(`INSERT INTO feed_row_updates (org, number, person, after, parts) VALUES ($1, $2, $3, $4, $5)`,
			string(m.org), st.rowHead, up.person, m.heads[up.person], string(parts))
		m.heads[up.person] = st.rowHead
		dirty[up.person] = true
	}
	for u := range dirty {
		counts, err := json.Marshal(m.e.Counts(u))
		if err != nil {
			return err
		}
		b.Queue(`INSERT INTO feed_readers (org, person, number, counts) VALUES ($1, $2, $3, $4)
			ON CONFLICT (org, person) DO UPDATE SET number = $3, counts = $4`, string(m.org), u, m.heads[u], string(counts))
	}
	if b.Len() == 0 {
		return nil
	}
	return tx.SendBatch(ctx, b).Close()
}

func (w *work) saveHeld(ctx context.Context, tx pgx.Tx, m *mem) error {
	b := &pgx.Batch{}
	now := map[string]bool{}
	for _, h := range m.e.AllHeld() {
		now[h.Name] = true
		if m.held[h.Name] {
			b.Queue(`UPDATE feed_held SET message = $3, item = $4 WHERE org = $1 AND name = $2`, string(m.org), h.Name, h.Message, h.Item)
			continue
		}
		m.held[h.Name] = true
		b.Queue(`INSERT INTO feed_held (org, name, person, conversation, message, item, at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			string(m.org), h.Name, h.Person, h.Conversation, h.Message, h.Item, h.At)
	}
	for name := range m.held {
		if !now[name] {
			delete(m.held, name)
			b.Queue(`DELETE FROM feed_held WHERE org = $1 AND name = $2`, string(m.org), name)
		}
	}
	if b.Len() == 0 {
		return nil
	}
	return tx.SendBatch(ctx, b).Close()
}

func (w *work) saveNotes(ctx context.Context, tx pgx.Tx, m *mem, st *state) error {
	if len(w.notes) == 0 {
		return nil
	}
	var head int64
	if err := tx.QueryRow(ctx, `SELECT head FROM note_heads WHERE org = $1 FOR UPDATE`, string(m.org)).Scan(&head); err != nil {
		return fmt.Errorf("note head: %w", err)
	}
	b := &pgx.Batch{}
	for _, n := range w.notes {
		body, err := json.Marshal(n.Contract(string(m.org), head+1, head, st.edition))
		if err != nil {
			return err
		}
		head++
		b.Queue(`INSERT INTO note_log (org, edition, number, at, kind, body) VALUES ($1, $2, $3, $4, $5, $6::jsonb)`,
			string(m.org), st.edition, head, n.At, n.Kind, string(body))
	}
	b.Queue(`UPDATE note_heads SET head = $2 WHERE org = $1`, string(m.org), head)
	b.Queue(`INSERT INTO note_due (org) VALUES ($1) ON CONFLICT DO NOTHING`, string(m.org))
	return tx.SendBatch(ctx, b).Close()
}

// takeAll runs one call: one transaction, whose failure leaves the store as it
// was and the copy's memory dropped.
func (s *Service) takeAll(ctx context.Context, org ids.Org, list []incoming) (int, any, error) {
	m := s.memFor(org)
	m.mu.Lock()
	defer m.mu.Unlock()
	var status int
	var answer any
	err := orgdb.InOrg(ctx, s.opt.Dir, org, func(tx pgx.Tx) error {
		st, err := readState(ctx, tx, org, true)
		if err != nil {
			return err
		}
		w := &work{touched: map[string]bool{}}
		status, answer, err = s.takeIn(ctx, tx, m, &st, w, list)
		if err != nil {
			return err
		}
		if err := w.flush(ctx, tx, m, &st); err != nil {
			return err
		}
		if fault(FaultDieBeforeCommit) && s.dies.Add(1)%3 == 0 {
			return errors.New("planted: died before the commit")
		}
		return nil
	})
	if err != nil {
		m.drop()
		return 0, nil, err
	}
	return status, answer, nil
}

func (s *Service) change(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	c, err := parseIncoming(raw)
	if err != nil {
		refuse(w, http.StatusBadRequest, "not a change")
		return
	}
	s.modes.gate(r.Context(), s.opt.Clock, c.org)
	org, err := ids.ParseOrg(c.org)
	if err != nil {
		refuse(w, http.StatusBadRequest, "not an organization")
		return
	}
	s.answerTake(w, r, org, []incoming{c})
}

func (s *Service) batch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Changes []json.RawMessage `json:"changes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || len(in.Changes) == 0 || len(in.Changes) > maxBatch {
		refuse(w, http.StatusBadRequest, "a batch is 1 to 100 changes")
		return
	}
	list := make([]incoming, len(in.Changes))
	for i, raw := range in.Changes {
		c, err := parseIncoming(raw)
		if err != nil {
			refuse(w, http.StatusBadRequest, "not a change")
			return
		}
		if i > 0 && c.org != list[0].org {
			refuse(w, http.StatusBadRequest, "a batch is of one organization")
			return
		}
		list[i] = c
	}
	s.modes.gate(r.Context(), s.opt.Clock, list[0].org)
	org, err := ids.ParseOrg(list[0].org)
	if err != nil {
		refuse(w, http.StatusBadRequest, "not an organization")
		return
	}
	s.answerTake(w, r, org, list)
}

func (s *Service) answerTake(w http.ResponseWriter, r *http.Request, org ids.Org, list []incoming) {
	status, answer, err := s.takeAll(r.Context(), org, list)
	if err != nil {
		unavailable(w, "take change", err)
		return
	}
	writeJSON(w, status, answer)
}
