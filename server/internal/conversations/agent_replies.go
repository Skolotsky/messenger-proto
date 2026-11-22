package conversations

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	replySilence     = time.Minute      // no new piece for this long closes a reply
	replyLongest     = 10 * time.Minute // a reply older than this is closed
	agentSweepEvery  = time.Second      // also how often the text so far is saved
	eventSweepAtMost = time.Hour        // trimming old events is batched
)

type replyKey struct {
	org       ids.Org
	agent, id string
}

// replyDraft is a reply in progress in memory. Its text is the newest; the
// database holds what was saved about a second ago. Lock order: mu, then the
// organization's log row. A question holding the log row reads view instead.
type replyDraft struct {
	mu           sync.Mutex
	key          replyKey
	message      string
	conversation string
	thread       *string
	becauseOf    *int64
	started      time.Time
	lastPiece    time.Time
	text         []rune
	dirty        bool
	closed       bool
	view         atomic.Pointer[string]
}

func (r *replyDraft) deadline() time.Time {
	silent, longest := r.lastPiece.Add(replySilence), r.started.Add(replyLongest)
	if silent.Before(longest) {
		return silent
	}
	return longest
}

func (r *replyDraft) setView() {
	text := string(r.text)
	r.view.Store(&text)
}

var errReplyClosed = errors.New("reply closed")

// replyDraft finds the reply in memory, or brings it back from what was last
// saved, as after a restart.
func (s *Store) replyDraft(ctx context.Context, key replyKey) (*replyDraft, error) {
	s.agents.mu.Lock()
	r := s.agents.replies[key]
	s.agents.mu.Unlock()
	if r != nil {
		return r, nil
	}
	loaded := &replyDraft{key: key}
	var text string
	var closed *time.Time
	err := orgdb.InOrg(ctx, s.Dir, key.org, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT message, conversation, thread, because_of, started_at, last_piece_at, text, closed_at
			FROM agent_replies WHERE org = $1 AND agent = $2 AND id = $3`, string(key.org), key.agent, key.id).
			Scan(&loaded.message, &loaded.conversation, &loaded.thread, &loaded.becauseOf, &loaded.started, &loaded.lastPiece, &text, &closed)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if closed != nil {
		return nil, errReplyClosed
	}
	loaded.text = []rune(text)
	loaded.setView()
	s.agents.mu.Lock()
	defer s.agents.mu.Unlock()
	if s.agents.replies == nil {
		s.agents.replies = map[replyKey]*replyDraft{}
	}
	if r := s.agents.replies[key]; r != nil {
		return r, nil
	}
	s.agents.replies[key] = loaded
	return loaded, nil
}

func (s *Store) forgetReply(r *replyDraft) {
	r.closed = true
	s.agents.mu.Lock()
	defer s.agents.mu.Unlock()
	delete(s.agents.replies, r.key)
}

// checkWhere says why an agent cannot write here, or "": the same rules a person's
// message meets.
func (s *Step) checkWhere(conversation string, thread *string, agent string) (string, error) {
	kind, err := s.conversationType(conversation)
	if err != nil {
		return "", err
	}
	if kind == "" {
		return refusedUnknownConv, nil
	}
	if kind == "direct" {
		if thread != nil {
			return refusedDirectThread, nil
		}
		people, err := s.participants(conversation)
		if err != nil {
			return "", err
		}
		if !slices.Contains(people, agent) {
			return refusedNotParticipant, nil
		}
	}
	if thread != nil {
		root, err := s.loadMessage(*thread)
		if err != nil {
			return "", err
		}
		if root == nil || root.Conversation != conversation || root.Thread != nil {
			return refusedUnknownThread, nil
		}
		if root.Deleted {
			var live bool
			if err := s.Tx.QueryRow(s.ctx, `SELECT EXISTS (SELECT 1 FROM messages WHERE org = $1 AND thread = $2 AND NOT deleted)`,
				string(s.org), root.Id).Scan(&live); err != nil {
				return "", err
			}
			if !live {
				return refusedDeleted, nil
			}
		}
	}
	return "", nil
}

func (s *Store) handleAgentReplyStart(w http.ResponseWriter, r *http.Request, org ids.Org, agent string) {
	var in contract.AgentReplyStart
	raw, ok := readBody(w, r, &in)
	if !ok {
		return
	}
	out, err := s.agentAct(r.Context(), org, agent, in.Id, "reply-start", raw, func(st *Step) (agentAnswer, error) {
		if why, err := st.checkWhere(in.Conversation, in.Thread, agent); err != nil {
			return agentAnswer{}, err
		} else if why != "" {
			return agentRefused(why), nil
		}
		if !s.agents.takeRate(org, agent, s.Clock.Now()) {
			return agentAnswer{status: http.StatusTooManyRequests, body: contract.Refusal{Refused: refusedSlowDown}}, nil
		}
		message := ids.New("msg")
		number, err := st.Append(contract.ChangeReplyStarted{Kind: "reply.started", Message: message,
			Conversation: in.Conversation, Thread: in.Thread, Author: agent})
		if err != nil {
			s.agents.giveBack(org, agent)
			return agentAnswer{}, err
		}
		due := st.at.Add(replySilence)
		if _, err := st.Tx.Exec(st.ctx, `INSERT INTO agent_replies (org, agent, id, message, conversation, thread, because_of, started_at, last_piece_at, due_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9)`, string(org), agent, in.Id, message, in.Conversation, in.Thread, in.BecauseOf, st.at, due); err != nil {
			return agentAnswer{}, err
		}
		if _, err := st.Tx.Exec(st.ctx, `INSERT INTO agent_org_due (org, reply_due_at) VALUES ($1, $2)
			ON CONFLICT (org) DO UPDATE SET reply_due_at = LEAST(agent_org_due.reply_due_at, EXCLUDED.reply_due_at)`, string(org), due); err != nil {
			return agentAnswer{}, err
		}
		return agentAnswer{status: http.StatusOK, body: contract.AgentPosted{Message: message, Number: number}}, nil
	})
	answerAgent(w, r, out, err)
}

func (s *Store) handleAgentPiece(w http.ResponseWriter, r *http.Request, org ids.Org, agent string) {
	var in contract.AgentPiece
	if _, ok := readBody(w, r, &in); !ok {
		return
	}
	if in.At < 0 {
		send(w, http.StatusConflict, contract.Refusal{Refused: refusedUnreadable})
		return
	}
	key := replyKey{org, agent, r.PathValue("reply")}
	reply, err := s.replyDraft(r.Context(), key)
	if errors.Is(err, errReplyClosed) {
		send(w, http.StatusConflict, contract.Refusal{Refused: refusedClosed})
		return
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	reply.mu.Lock()
	defer reply.mu.Unlock()
	if reply.closed {
		send(w, http.StatusConflict, contract.Refusal{Refused: refusedClosed})
		return
	}
	now := s.Clock.Now()
	if !now.Before(reply.deadline()) && !agentFault("no-timeout") {
		if err := s.closeLocked(r.Context(), reply); err != nil {
			fail(w, r, err)
			return
		}
		send(w, http.StatusConflict, contract.Refusal{Refused: refusedClosed})
		return
	}
	have := int64(len(reply.text))
	if in.At > have {
		send(w, http.StatusConflict, contract.PieceMissing{Have: have})
		return
	}
	piece := []rune(in.Text)
	var fresh []rune
	if agentFault("no-repeat-guard") {
		fresh = piece // the fault: a repeat is added again
	} else if skip := have - in.At; skip < int64(len(piece)) {
		fresh = piece[skip:]
	}
	if len(fresh) > 0 {
		reply.text = append(reply.text, fresh...)
		reply.lastPiece = now
		reply.dirty = true
		reply.setView()
		if agentFault("text-on-log") {
			if err := s.Save(r.Context(), org, func(st *Step) error {
				_, err := st.Append(contract.ChangeReplyAbandoned{Kind: "reply.abandoned", Message: string(reply.text),
					Conversation: reply.conversation, Thread: reply.thread})
				return err
			}); err != nil {
				fail(w, r, err)
				return
			}
		}
		s.agents.pushed(org, reply.conversation, contract.LivePiece{Piece: contract.PieceBody{
			Message: reply.message, At: have, Text: string(fresh)}})
	}
	send(w, http.StatusOK, contract.Empty{})
}

func (s *Store) handleAgentFinish(w http.ResponseWriter, r *http.Request, org ids.Org, agent string) {
	var in contract.AgentReplyFinish
	raw, ok := readBody(w, r, &in)
	if !ok {
		return
	}
	key := replyKey{org, agent, r.PathValue("reply")}
	reply, err := s.replyDraft(r.Context(), key)
	if err != nil && !errors.Is(err, errReplyClosed) {
		fail(w, r, err)
		return
	}
	if reply != nil {
		reply.mu.Lock()
		defer reply.mu.Unlock()
		if !reply.closed && !s.Clock.Now().Before(reply.deadline()) && !agentFault("no-timeout") {
			if err := s.closeLocked(r.Context(), reply); err != nil {
				fail(w, r, err)
				return
			}
		}
	}
	finished := false
	out, err := s.agentAct(r.Context(), org, agent, in.Id, "reply-finish:"+key.id, raw, func(st *Step) (agentAnswer, error) {
		if reply == nil || reply.closed {
			return agentRefused(refusedClosed), nil
		}
		if strings.TrimSpace(string(reply.text)) == "" {
			out := agentRefused(refusedEmptyMessage)
			out.forget = true // more text may still come under the same ID
			return out, nil
		}
		if why, err := st.checkWriting(string(reply.text), in.Mentions, in.Level); err != nil {
			return agentAnswer{}, err
		} else if why != "" {
			return agentRefused(why), nil
		}
		number, err := st.finishReply(reply, in.Mentions, in.Level, false)
		if err != nil {
			return agentAnswer{}, err
		}
		finished = true
		return agentAnswer{status: http.StatusOK, body: contract.AgentPosted{Message: reply.message, Number: number}}, nil
	})
	if err == nil && finished {
		s.forgetReply(reply)
	}
	answerAgent(w, r, out, err)
}

// finishReply writes the reply's one whole message and marks the reply closed,
// in the step that holds the log's lock.
func (st *Step) finishReply(r *replyDraft, mentions []string, level *contract.Level, stopped bool) (int64, error) {
	var participants []string
	kind, err := st.conversationType(r.conversation)
	if err != nil {
		return 0, err
	}
	if kind == "direct" {
		if participants, err = st.participants(r.conversation); err != nil {
			return 0, err
		}
	}
	st.agentBecauseOf = r.becauseOf
	number, err := st.Append(contract.ChangeMessageCreated{Kind: "message.created", Message: contract.Message{
		Id: r.message, Number: st.Next(), Version: 1, Conversation: r.conversation, Thread: r.thread,
		Author: r.key.agent, AuthorIsAgent: true, Participants: participants, Text: string(r.text), Mentions: unique(mentions),
		Level: level, Reactions: []contract.Reaction{}, Stopped: stopped, CreatedAt: st.At()}})
	if err != nil {
		return 0, err
	}
	readers := []string{}
	if r.thread != nil || kind == "direct" {
		readers = append(readers, r.key.agent)
	}
	watchers, err := st.watchers(r.conversation, r.thread)
	if err != nil {
		return 0, err
	}
	for _, p := range unique(append(readers, watchers...)) {
		if err := st.readUp(p, r.conversation, r.thread, number); err != nil {
			return 0, err
		}
	}
	return number, st.markClosed(r)
}

func (st *Step) markClosed(r *replyDraft) error {
	_, err := st.Tx.Exec(st.ctx, `UPDATE agent_replies SET closed_at = $4, text = '' WHERE org = $1 AND agent = $2 AND id = $3`,
		string(st.org), r.key.agent, r.key.id, st.at)
	return err
}

// closeLocked closes a reply that ran out of time: a finished message marked
// stopped with the text so far, or, with no text, reply.abandoned and no message
// (D-031, D-032). The caller holds r.mu.
func (s *Store) closeLocked(ctx context.Context, r *replyDraft) error {
	err := s.Save(ctx, r.key.org, func(st *Step) error {
		var closed *time.Time
		if err := st.Tx.QueryRow(ctx, `SELECT closed_at FROM agent_replies WHERE org = $1 AND agent = $2 AND id = $3 FOR UPDATE`,
			string(st.org), r.key.agent, r.key.id).Scan(&closed); err != nil {
			return err
		}
		if closed != nil {
			return nil
		}
		if strings.TrimSpace(string(r.text)) == "" {
			if _, err := st.Append(contract.ChangeReplyAbandoned{Kind: "reply.abandoned", Message: r.message,
				Conversation: r.conversation, Thread: r.thread}); err != nil {
				return err
			}
			return st.markClosed(r)
		}
		_, err := st.finishReply(r, nil, nil, true)
		return err
	})
	if err != nil {
		return err
	}
	s.forgetReply(r)
	return nil
}

// handleReplyText answers a screen that saw a gap or loaded during a reply.
func (s *Store) handleReplyText(w http.ResponseWriter, r *http.Request) {
	org, ok := orgOf(w, r)
	if !ok {
		return
	}
	message := r.PathValue("message")
	var out contract.ReplyText
	var closed *time.Time
	err := orgdb.InOrg(r.Context(), s.Dir, org, func(tx pgx.Tx) error {
		return tx.QueryRow(r.Context(), `SELECT agent, conversation, thread, text, closed_at FROM agent_replies WHERE org = $1 AND message = $2`,
			string(org), message).Scan(&out.Author, &out.Conversation, &out.Thread, &out.Text, &closed)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		send(w, http.StatusNotFound, contract.Refusal{Refused: "not found"})
		return
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	if closed != nil {
		send(w, http.StatusConflict, contract.Refusal{Refused: refusedClosed})
		return
	}
	out.Message = message
	if text, ok := s.agents.textOf(org, message); ok {
		out.Text = text
	}
	out.At = int64(len([]rune(out.Text)))
	send(w, http.StatusOK, out)
}

func (d *agentDoor) textOf(org ids.Org, message string) (string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for key, r := range d.replies {
		if key.org == org && r.message == message {
			if v := r.view.Load(); v != nil {
				return *v, true
			}
		}
	}
	return "", false
}

// agentInProgress lists the replies being written in this conversation or thread.
func (s *Store) agentInProgress(ctx context.Context, tx pgx.Tx, org ids.Org, conversation string, thread *string) ([]contract.ReplyInProgress, error) {
	rows, err := tx.Query(ctx, `SELECT message, conversation, thread, agent, text FROM agent_replies
		WHERE org = $1 AND conversation = $2 AND thread IS NOT DISTINCT FROM $3 AND closed_at IS NULL ORDER BY started_at, message`,
		string(org), conversation, thread)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []contract.ReplyInProgress{}
	for rows.Next() {
		var p contract.ReplyInProgress
		if err := rows.Scan(&p.Message, &p.Conversation, &p.Thread, &p.Author, &p.Text); err != nil {
			return nil, err
		}
		if text, ok := s.agents.textOf(org, p.Message); ok {
			p.Text = text
		}
		p.At = int64(len([]rune(p.Text)))
		out = append(out, p)
	}
	return out, rows.Err()
}

// AgentQuestion is one timed question of the door's housekeeping, listed so
// G-029 can read its plan.
type AgentQuestion struct {
	Name string
	SQL  string
	Args []any
}

// AgentQuestions are what housekeeping asks on its timer. Each runs with
// sequential scans off, so it stays on its partial index at any table size: a
// small table would otherwise be read whole, idle organizations included.
func AgentQuestions(now time.Time) []AgentQuestion {
	reply, events := `reply_due_at <= $1`, `events_due_at <= $1`
	if agentFault("scan-idle") {
		reply, events = `reply_due_at + interval '0' <= $1`, `events_due_at + interval '0' <= $1`
	}
	const limit = ` LIMIT 200`
	return []AgentQuestion{
		{"agent door: replies to close", `SELECT org FROM agent_org_due WHERE ` + reply + ` ORDER BY reply_due_at` + limit, []any{now}},
		{"agent door: old events to trim", `SELECT org FROM agent_org_due WHERE ` + events + ` ORDER BY events_due_at` + limit, []any{now}},
	}
}

func dueOrgs(ctx context.Context, pool *pgxpool.Pool, q AgentQuestion) ([]ids.Org, error) {
	var out []ids.Org
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, q.SQL, q.Args...)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (ids.Org, error) {
			var org string
			err := row.Scan(&org)
			return ids.Org(org), err
		})
		return err
	})
	return out, err
}

// AgentHousekeep is one pass: save the text so far, close the replies that ran
// out of time, and trim events older than 7 days.
func (s *Store) AgentHousekeep(ctx context.Context, pools []*pgxpool.Pool) error {
	s.saveReplyText(ctx)
	now := s.Clock.Now()
	questions := AgentQuestions(now)
	var first error
	note := func(err error) {
		if err != nil && first == nil {
			first = err
		}
	}
	for _, pool := range pools {
		orgs, err := dueOrgs(ctx, pool, questions[0])
		note(err)
		for _, org := range orgs {
			note(s.closeDueReplies(ctx, org, now))
		}
		orgs, err = dueOrgs(ctx, pool, questions[1])
		note(err)
		for _, org := range orgs {
			note(s.trimEvents(ctx, org, now))
		}
	}
	return first
}

// RunAgentHousekeeping runs passes until ctx ends.
func (s *Store) RunAgentHousekeeping(ctx context.Context, pools []*pgxpool.Pool) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.Clock.After(agentSweepEvery):
		}
		if err := s.AgentHousekeep(ctx, pools); err != nil && ctx.Err() == nil {
			slog.Error("agent housekeeping", "err", err)
		}
	}
}

// saveReplyText writes each changed reply's text so far, outside the log.
func (s *Store) saveReplyText(ctx context.Context) {
	s.agents.mu.Lock()
	replies := make([]*replyDraft, 0, len(s.agents.replies))
	for _, r := range s.agents.replies {
		replies = append(replies, r)
	}
	s.agents.mu.Unlock()
	for _, r := range replies {
		r.mu.Lock()
		if r.dirty && !r.closed {
			err := orgdb.InOrg(ctx, s.Dir, r.key.org, func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `UPDATE agent_replies SET text = $4, last_piece_at = $5, due_at = $6
					WHERE org = $1 AND agent = $2 AND id = $3 AND closed_at IS NULL`,
					string(r.key.org), r.key.agent, r.key.id, string(r.text), r.lastPiece, r.deadline())
				return err
			})
			if err == nil {
				r.dirty = false
			} else if ctx.Err() == nil {
				slog.Error("save a reply's text", "org", r.key.org, "err", err)
			}
		}
		r.mu.Unlock()
	}
}

func (s *Store) closeDueReplies(ctx context.Context, org ids.Org, now time.Time) error {
	if agentFault("no-timeout") {
		return nil
	}
	var due []replyKey
	err := orgdb.InOrg(ctx, s.Dir, org, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT agent, id FROM agent_replies WHERE org = $1 AND closed_at IS NULL AND due_at <= $2`, string(org), now)
		if err != nil {
			return err
		}
		due, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (replyKey, error) {
			k := replyKey{org: org}
			err := row.Scan(&k.agent, &k.id)
			return k, err
		})
		return err
	})
	if err != nil {
		return err
	}
	for _, key := range due {
		r, err := s.replyDraft(ctx, key)
		if errors.Is(err, errReplyClosed) || errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		r.mu.Lock()
		if !r.closed && !now.Before(r.deadline()) {
			err = s.closeLocked(ctx, r)
		}
		r.mu.Unlock()
		if errors.Is(err, ErrOrganizationGone) {
			return s.dropRetired(ctx, org)
		}
		if err != nil {
			return err
		}
	}
	err = s.Save(ctx, org, func(st *Step) error {
		_, err := st.Tx.Exec(ctx, `UPDATE agent_org_due SET reply_due_at =
			(SELECT min(due_at) FROM agent_replies WHERE org = $1 AND closed_at IS NULL) WHERE org = $1`, string(org))
		return err
	})
	if errors.Is(err, ErrOrganizationGone) {
		return s.dropRetired(ctx, org)
	}
	return err
}

// dropRetired stops housekeeping for a retired organization: what was in
// progress there is dropped (D-033).
func (s *Store) dropRetired(ctx context.Context, org ids.Org) error {
	s.forgetOrgReplies(org)
	return orgdb.InOrg(ctx, s.Dir, org, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE agent_org_due SET reply_due_at = NULL, events_due_at = NULL WHERE org = $1`, string(org))
		return err
	})
}

// forgetOrgReplies lets go of what the door holds in memory for an organization
// that is retired. Called with no lock held.
func (s *Store) forgetOrgReplies(org ids.Org) {
	s.agents.mu.Lock()
	var mine []*replyDraft
	for key, r := range s.agents.replies {
		if key.org == org {
			mine = append(mine, r)
			delete(s.agents.replies, key)
		}
	}
	s.agents.mu.Unlock()
	for _, r := range mine {
		r.mu.Lock()
		r.closed = true
		r.mu.Unlock()
	}
}

// trimEvents removes events older than 7 days and remembers the newest number
// gone per agent, under the log's lock so it cannot pass a save in flight.
func (s *Store) trimEvents(ctx context.Context, org ids.Org, now time.Time) error {
	cutoff := now.Add(-agentEventsKept)
	err := s.Save(ctx, org, func(st *Step) error {
		if _, err := st.Tx.Exec(ctx, `WITH gone AS (
			  DELETE FROM agent_events WHERE org = $1 AND at < $2 RETURNING agent, number
			), top AS (SELECT agent, max(number) AS n FROM gone GROUP BY agent)
			UPDATE agent_heads h SET floor = GREATEST(h.floor, top.n) FROM top WHERE h.org = $1 AND h.agent = top.agent`,
			string(org), cutoff); err != nil {
			return err
		}
		if _, err := st.Tx.Exec(ctx, `DELETE FROM agent_replies WHERE org = $1 AND closed_at < $2`, string(org), cutoff); err != nil {
			return err
		}
		_, err := st.Tx.Exec(ctx, `UPDATE agent_org_due SET events_due_at = (
			  SELECT CASE WHEN count(*) = 0 THEN NULL ELSE GREATEST(min(at) + $2::interval, $3::timestamptz) END
			  FROM agent_events WHERE org = $1
			) WHERE org = $1`, string(org), agentEventsKept, now.Add(eventSweepAtMost))
		return err
	})
	if errors.Is(err, ErrOrganizationGone) {
		return s.dropRetired(ctx, org)
	}
	return err
}
