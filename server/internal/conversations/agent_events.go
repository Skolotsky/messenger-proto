package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

const (
	agentEventsKept = 7 * 24 * time.Hour
	agentEventPage  = 100
	// maxAgentChain is how many agents in a row may stand behind a message
	// before no further agent is told (D-008).
	maxAgentChain = 3
)

// tellAgents writes, in the step that saves a change, what it means for each
// agent. Being in the same transaction is why an agent misses nothing (G-016),
// and it is a few rows: nothing ever waits for an agent. Seeded history tells
// nobody: it happened before the agents were listening.
func (s *Step) tellAgents(change any) error {
	if s.noTick {
		return nil
	}
	switch c := change.(type) {
	case contract.ChangeMessageCreated:
		return s.tellNew(c.Message)
	case contract.ChangeMessageChanged:
		return s.tellChanged(c.Message.Id, "changed")
	case contract.ChangeMessageDeleted:
		return s.tellChanged(c.Message.Id, "deleted")
	case contract.ChangeReactionAdded:
		return s.tellReaction(c.Message, c.Person, c.Emoji, true)
	case contract.ChangeReactionRemoved:
		return s.tellReaction(c.Message, c.Person, c.Emoji, false)
	}
	return nil
}

func (s *Step) orgAgents() ([]string, error) {
	rows, err := s.Tx.Query(s.ctx, `SELECT id FROM people WHERE org = $1 AND is_agent ORDER BY id`, string(s.org))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s *Step) column(sql string, args ...any) ([]string, error) {
	rows, err := s.Tx.Query(s.ctx, sql, append([]any{string(s.org)}, args...)...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// audience is who could be addressed by a message of this conversation.
type audience struct {
	subscribers, tookPart, members []string
}

func (s *Step) audienceOf(m *contract.Message) (audience, error) {
	var a audience
	var err error
	if a.subscribers, err = s.column(`SELECT agent FROM agent_subscriptions WHERE org = $1 AND conversation = $2`, m.Conversation); err != nil {
		return a, err
	}
	if m.Thread != nil {
		if a.tookPart, err = s.column(`SELECT DISTINCT author FROM messages WHERE org = $1 AND author_is_agent AND (id = $2 OR thread = $2)`, *m.Thread); err != nil {
			return a, err
		}
	}
	if slices.Contains(m.Mentions, "channel") {
		if a.members, err = s.participants(m.Conversation); err != nil {
			return a, err
		}
	}
	return a, nil
}

// addressKind is why this message concerns the agent, or "". An agent hears of
// another agent's message only when it is named in it (D-008). "channel" names
// the channel's members when a person writes it.
func addressKind(agent string, m *contract.Message, a audience) string {
	if slices.Contains(m.Mentions, agent) || (!m.AuthorIsAgent && slices.Contains(a.members, agent)) {
		return "mentioned"
	}
	if m.AuthorIsAgent {
		return ""
	}
	if slices.Contains(m.Participants, agent) {
		return "direct_message"
	}
	if m.Thread != nil && slices.Contains(a.tookPart, agent) {
		return "reply"
	}
	if slices.Contains(a.subscribers, agent) {
		return "reply" // the contract's kinds have none for "new in a channel you follow"
	}
	return ""
}

func (s *Step) tellNew(m contract.Message) error {
	agents, err := s.orgAgents()
	if err != nil || len(agents) == 0 {
		return err
	}
	chain, err := s.chainOf(&m)
	if err != nil {
		return err
	}
	if chain >= maxAgentChain {
		return nil
	}
	a, err := s.audienceOf(&m)
	if err != nil {
		return err
	}
	for _, agent := range agents {
		if agent == m.Author {
			continue
		}
		if kind := addressKind(agent, &m, a); kind != "" {
			if err := s.writeEvent(agent, kind, &m, chain, "", "", false); err != nil {
				return err
			}
		}
	}
	return nil
}

// chainOf counts the agents in a row behind a new message, in its own row of
// the conversation: a person's message starts again from zero. An answer to an
// event of another row carries that event's count with it, so a loop cannot
// escape the cap by moving between rows.
func (s *Step) chainOf(m *contract.Message) (int, error) {
	if !m.AuthorIsAgent || agentFault("no-chain") {
		return 0, nil
	}
	var prev string
	var prevAgent bool
	var err error
	if m.Thread != nil {
		err = s.Tx.QueryRow(s.ctx, `SELECT id, author_is_agent FROM messages WHERE org = $1 AND (id = $2 OR thread = $2) AND number < $3
			ORDER BY number DESC LIMIT 1`, string(s.org), *m.Thread, m.Number).Scan(&prev, &prevAgent)
	} else {
		err = s.Tx.QueryRow(s.ctx, `SELECT id, author_is_agent FROM messages WHERE org = $1 AND conversation = $2 AND thread IS NULL AND number < $3
			ORDER BY number DESC LIMIT 1`, string(s.org), m.Conversation, m.Number).Scan(&prev, &prevAgent)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	chain := 1
	if err == nil && prevAgent {
		before, err := s.storedChain(prev)
		if err != nil {
			return 0, err
		}
		chain = max(before, 1) + 1 // an agent's seeded message has no row, and counts one
	}
	if s.agentBecauseOf != nil {
		var evChain int
		var conversation, id string
		var thread *string
		err := s.Tx.QueryRow(s.ctx, `SELECT (body->>'agent_chain')::int, body->'message'->>'conversation', body->'message'->>'thread', message
			FROM agent_events WHERE org = $1 AND agent = $2 AND number = $3`, string(s.org), m.Author, *s.agentBecauseOf).
			Scan(&evChain, &conversation, &thread, &id)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return 0, err
		}
		if err == nil && !sameRow(m, conversation, thread, id) {
			chain = max(chain, evChain+1)
		}
	}
	if _, err := s.Tx.Exec(s.ctx, `INSERT INTO agent_chains (org, message, chain) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		string(s.org), m.Id, chain); err != nil {
		return 0, err
	}
	return chain, nil
}

func sameRow(m *contract.Message, conversation string, thread *string, id string) bool {
	if conversation != m.Conversation {
		return false
	}
	if m.Thread == nil {
		return thread == nil
	}
	return *m.Thread == id || (thread != nil && *thread == *m.Thread)
}

func (s *Step) storedChain(message string) (int, error) {
	var chain int
	err := s.Tx.QueryRow(s.ctx, `SELECT chain FROM agent_chains WHERE org = $1 AND message = $2`, string(s.org), message).Scan(&chain)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return chain, err
}

// tellChanged tells whoever heard of the message, and whoever it addresses now.
// An edit that newly names an agent reaches it as new (D-004).
func (s *Step) tellChanged(id, kind string) error {
	agents, err := s.orgAgents()
	if err != nil || len(agents) == 0 {
		return err
	}
	m, err := s.loadMessage(id)
	if err != nil || m == nil {
		return err
	}
	told, err := s.column(`SELECT DISTINCT agent FROM agent_events WHERE org = $1 AND message = $2`, id)
	if err != nil {
		return err
	}
	chain, err := s.storedChain(id)
	if err != nil {
		return err
	}
	a, err := s.audienceOf(m)
	if err != nil {
		return err
	}
	for _, agent := range agents {
		if agent == m.Author {
			continue
		}
		was := slices.Contains(told, agent)
		now := addressKind(agent, m, a)
		switch {
		case kind == "changed" && !was && now == "mentioned":
			if chain >= maxAgentChain {
				continue
			}
			err = s.writeEvent(agent, "mentioned", m, chain, "", "", false)
		case was || now != "":
			err = s.writeEvent(agent, kind, m, chain, "", "", false)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// tellReaction tells the message's author, if an agent, and the agents following
// the channel when a person reacted. An agent's reaction counts as one more agent
// in the chain, so two agents cannot keep each other busy with reactions either.
func (s *Step) tellReaction(id, by, emoji string, added bool) error {
	agents, err := s.orgAgents()
	if err != nil || len(agents) == 0 {
		return err
	}
	m, err := s.loadMessage(id)
	if err != nil || m == nil {
		return err
	}
	byAgent := slices.Contains(agents, by)
	chain := 0
	if byAgent {
		before, err := s.storedChain(id)
		if err != nil {
			return err
		}
		if before >= maxAgentChain {
			return nil
		}
		chain = before + 1
	}
	subscribers, err := s.column(`SELECT agent FROM agent_subscriptions WHERE org = $1 AND conversation = $2`, m.Conversation)
	if err != nil {
		return err
	}
	for _, agent := range agents {
		if agent == by {
			continue
		}
		if agent == m.Author || (!byAgent && slices.Contains(subscribers, agent)) {
			if err := s.writeEvent(agent, "reaction", m, chain, by, emoji, added); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Step) writeEvent(agent, kind string, m *contract.Message, chain int, by, emoji string, added bool) error {
	if agentFault("drop-replies") && kind == "reply" {
		return nil
	}
	var number int64
	if err := s.Tx.QueryRow(s.ctx, `INSERT INTO agent_heads (org, agent, head) VALUES ($1, $2, 1)
		ON CONFLICT (org, agent) DO UPDATE SET head = agent_heads.head + 1 RETURNING head`, string(s.org), agent).Scan(&number); err != nil {
		return err
	}
	org, c := string(s.org), int64(chain)
	var event any
	switch kind {
	case "mentioned":
		event = contract.AgentEventMentioned{Kind: kind, Org: org, Number: number, AgentChain: c, Message: *m}
	case "direct_message":
		event = contract.AgentEventDirectMessage{Kind: kind, Org: org, Number: number, AgentChain: c, Message: *m}
	case "reply":
		event = contract.AgentEventReply{Kind: kind, Org: org, Number: number, AgentChain: c, Message: *m}
	case "changed":
		event = contract.AgentEventChanged{Kind: kind, Org: org, Number: number, AgentChain: c, Message: *m}
	case "deleted":
		event = contract.AgentEventDeleted{Kind: kind, Org: org, Number: number, AgentChain: c, Message: *m}
	default:
		event = contract.AgentEventReaction{Kind: "reaction", Org: org, Number: number, AgentChain: c, Message: *m, By: by, Emoji: emoji, Added: added}
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := s.Tx.Exec(s.ctx, `INSERT INTO agent_events (org, agent, number, at, message, body) VALUES ($1, $2, $3, $4, $5, $6)`,
		org, agent, number, s.at, m.Id, body); err != nil {
		return err
	}
	_, err = s.Tx.Exec(s.ctx, `INSERT INTO agent_org_due (org, events_due_at) VALUES ($1, $2)
		ON CONFLICT (org) DO UPDATE SET events_due_at = EXCLUDED.events_due_at WHERE agent_org_due.events_due_at IS NULL`,
		org, s.at.Add(agentEventsKept))
	return err
}

// agentEvents answers one read of the agent's events. Anything older than 7 days
// counts as gone from that moment, whether or not housekeeping has trimmed it.
func (s *Store) agentEvents(ctx context.Context, org ids.Org, agent string, after int64) (int, any, error) {
	var status int
	var answer any
	cutoff := s.Clock.Now().Add(-agentEventsKept)
	err := orgdb.InOrg(ctx, s.Dir, org, func(tx pgx.Tx) error {
		var retired bool
		if err := tx.QueryRow(ctx, `SELECT retired_at IS NOT NULL FROM org_log WHERE org = $1`, string(org)).Scan(&retired); err != nil {
			return unknownOrg(err)
		}
		if retired {
			return ErrOrganizationGone
		}
		var floor, expired int64
		err := tx.QueryRow(ctx, `SELECT floor FROM agent_heads WHERE org = $1 AND agent = $2`, string(org), agent).Scan(&floor)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT coalesce(max(number), 0) FROM agent_events WHERE org = $1 AND agent = $2 AND at < $3`,
			string(org), agent, cutoff).Scan(&expired); err != nil {
			return err
		}
		floor = max(floor, expired)
		if after < floor && !agentFault("no-too-far") {
			status, answer = http.StatusGone, contract.TooFarBehind{TooFarBehind: true, Oldest: floor + 1}
			return nil
		}
		rows, err := tx.Query(ctx, `SELECT number, body FROM agent_events WHERE org = $1 AND agent = $2 AND number > $3 ORDER BY number LIMIT $4`,
			string(org), agent, after, agentEventPage+1)
		if err != nil {
			return err
		}
		defer rows.Close()
		out := contract.AgentEvents{Events: []contract.AgentEvent{}}
		var last int64
		for rows.Next() {
			var number int64
			var body []byte
			if err := rows.Scan(&number, &body); err != nil {
				return err
			}
			if len(out.Events) == agentEventPage {
				from := last
				out.ContinueFrom = &from
				break
			}
			var event contract.AgentEvent
			if err := json.Unmarshal(body, &event); err != nil {
				return err
			}
			out.Events = append(out.Events, event)
			last = number
		}
		if err := rows.Err(); err != nil {
			return err
		}
		status, answer = http.StatusOK, out
		return nil
	})
	return status, answer, err
}

func (s *Store) handleAgentEvents(w http.ResponseWriter, r *http.Request, org ids.Org, agent string) {
	var after int64
	var wait time.Duration
	if text := r.URL.Query().Get("after"); text != "" {
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil || n < 0 {
			send(w, http.StatusBadRequest, contract.Refusal{Refused: "bad after"})
			return
		}
		after = n
	}
	if text := r.URL.Query().Get("wait"); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil || n < 0 {
			send(w, http.StatusBadRequest, contract.Refusal{Refused: "bad wait"})
			return
		}
		wait = min(time.Duration(n)*time.Second, agentMaxWait)
	}
	var timer <-chan time.Time
	if wait > 0 {
		timer = s.Clock.After(wait)
	}
	for {
		woken, stop := s.agents.listen(org) // before reading, so nothing falls between
		status, answer, err := s.agentEvents(r.Context(), org, agent, after)
		if err != nil {
			stop()
			fail(w, r, err)
			return
		}
		events, isEvents := answer.(contract.AgentEvents)
		if !isEvents || len(events.Events) > 0 || wait == 0 {
			stop()
			send(w, status, answer)
			return
		}
		select {
		case <-woken:
			stop()
		case <-timer:
			stop()
			send(w, status, answer)
			return
		case <-r.Context().Done():
			stop()
			return
		}
	}
}
