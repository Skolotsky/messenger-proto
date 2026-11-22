package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

const (
	agentMessagesPerMinute = 20
	agentMaxWait           = 25 * time.Second
	refusedClosed          = "closed"
	refusedSlowDown        = "slow down"
	refusedNotAMember      = "not a member"
)

// AgentPush is how open screens hear what travels outside the numbering: the
// pieces of a reply in progress and an agent's status. conversation is empty for
// a status, which concerns everyone in the organization; a piece must reach only
// screens that may see that conversation. It must not block.
type AgentPush func(org ids.Org, conversation string, update contract.LiveOut)

// SetAgentPush gives the agent door the live connection's way to the screens.
func (s *Store) SetAgentPush(fn AgentPush) {
	s.agents.mu.Lock()
	defer s.agents.mu.Unlock()
	s.agents.push = fn
}

// agentDoor is the door's memory. An organization's agents are served by one
// copy (D-029); what is here is lost with it, and the database has enough to
// carry on: the text saved last, and the events themselves.
type agentDoor struct {
	mu      sync.Mutex
	replies map[replyKey]*replyDraft
	waiters map[ids.Org]map[chan struct{}]struct{}
	rates   map[rateKey][]time.Time
	push    AgentPush
}

type rateKey struct {
	org   ids.Org
	agent string
}

func (d *agentDoor) pushed(org ids.Org, conversation string, update any) {
	d.mu.Lock()
	fn := d.push
	d.mu.Unlock()
	if fn != nil {
		fn(org, conversation, contract.LiveOut{Value: update})
	}
}

// listen returns a channel closed by the next save of the organization.
func (d *agentDoor) listen(org ids.Org) (chan struct{}, func()) {
	ch := make(chan struct{})
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.waiters == nil {
		d.waiters = map[ids.Org]map[chan struct{}]struct{}{}
	}
	if d.waiters[org] == nil {
		d.waiters[org] = map[chan struct{}]struct{}{}
	}
	d.waiters[org][ch] = struct{}{}
	return ch, func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		delete(d.waiters[org], ch)
		if len(d.waiters[org]) == 0 {
			delete(d.waiters, org)
		}
	}
}

func (d *agentDoor) wake(org ids.Org) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for ch := range d.waiters[org] {
		close(ch)
		delete(d.waiters[org], ch)
	}
}

// takeRate counts one message against the agent's minute, or says it is full.
func (d *agentDoor) takeRate(org ids.Org, agent string, now time.Time) bool {
	if agentFault("no-rate") {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rates == nil {
		d.rates = map[rateKey][]time.Time{}
	}
	key := rateKey{org, agent}
	kept := d.rates[key][:0]
	for _, t := range d.rates[key] {
		if now.Sub(t) < time.Minute {
			kept = append(kept, t)
		}
	}
	if len(kept) >= agentMessagesPerMinute {
		d.rates[key] = kept
		return false
	}
	d.rates[key] = append(kept, now)
	return true
}

// giveBack returns the slot of a message that was not saved after all.
func (d *agentDoor) giveBack(org ids.Org, agent string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	key := rateKey{org, agent}
	if n := len(d.rates[key]); n > 0 {
		d.rates[key] = d.rates[key][:n-1]
	}
}

func (s *Store) agentRoutes(mux *http.ServeMux) {
	const base = "/orgs/{org}/agents/{agent}"
	mux.HandleFunc("GET "+base+"/events", s.agentCall(s.handleAgentEvents))
	mux.HandleFunc("POST "+base+"/messages", s.agentCall(s.handleAgentPost))
	mux.HandleFunc("POST "+base+"/replies", s.agentCall(s.handleAgentReplyStart))
	mux.HandleFunc("POST "+base+"/replies/{reply}/pieces", s.agentCall(s.handleAgentPiece))
	mux.HandleFunc("POST "+base+"/replies/{reply}/finish", s.agentCall(s.handleAgentFinish))
	mux.HandleFunc("POST "+base+"/status", s.agentCall(s.handleAgentStatus))
	mux.HandleFunc("POST "+base+"/reactions", s.agentCall(s.handleAgentReact))
	mux.HandleFunc("POST "+base+"/direct", s.agentCall(s.handleAgentDirect))
	mux.HandleFunc("PUT "+base+"/subscriptions/{conversation}", s.agentCall(s.handleAgentSubscribe))
	mux.HandleFunc("DELETE "+base+"/subscriptions/{conversation}", s.agentCall(s.handleAgentUnsubscribe))
	mux.HandleFunc("GET "+base+"/conversations/{conversation}/messages", s.agentCall(s.handleAgentMessages))
	mux.HandleFunc("GET /orgs/{org}/replies/{message}", s.door(s.handleReplyText))
}

type agentHandler func(w http.ResponseWriter, r *http.Request, org ids.Org, agent string)

// agentCall answers for a retired organization (410), and for an organization or
// agent that does not exist (404), before the operation runs.
func (s *Store) agentCall(next agentHandler) http.HandlerFunc {
	return s.door(func(w http.ResponseWriter, r *http.Request) {
		org, ok := orgOf(w, r)
		if !ok {
			return
		}
		agent := r.PathValue("agent")
		var known bool
		err := orgdb.InOrg(r.Context(), s.Dir, org, func(tx pgx.Tx) error {
			return tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM people WHERE org = $1 AND id = $2 AND is_agent)`,
				string(org), agent).Scan(&known)
		})
		if err != nil {
			fail(w, r, err)
			return
		}
		if !known {
			send(w, http.StatusNotFound, contract.Refusal{Refused: "unknown agent"})
			return
		}
		next(w, r, org, agent)
	})
}

// agentAnswer is what one action with an ID came to. forget leaves the ID
// unspent, for an answer that sending the same thing again can change.
type agentAnswer struct {
	status int
	body   any
	forget bool
}

func agentRefused(reason string) agentAnswer {
	return agentAnswer{status: http.StatusConflict, body: contract.Refusal{Refused: reason}}
}

// agentAct runs one action under the agent's ID, in one step: the same ID with
// the same body gets the first answer, with another body it is refused (G-003).
func (s *Store) agentAct(ctx context.Context, org ids.Org, agent, id, op string, raw []byte, fn func(*Step) (agentAnswer, error)) (agentAnswer, error) {
	if id == "" {
		return agentRefused(refusedUnreadable), nil
	}
	hash, err := bodyHash(raw)
	if err != nil {
		return agentRefused(refusedUnreadable), nil
	}
	hash = op + ":" + hash
	var out agentAnswer
	err = s.Save(ctx, org, func(st *Step) error {
		if !agentFault("no-ids") {
			var firstHash string
			var firstStatus int
			var firstAnswer json.RawMessage
			err := st.Tx.QueryRow(ctx, `SELECT body_hash, status, answer FROM action_answers WHERE org = $1 AND caller = $2 AND id = $3`,
				string(org), agent, id).Scan(&firstHash, &firstStatus, &firstAnswer)
			if err == nil {
				if firstHash != hash {
					out = agentRefused(refusedIDReused)
				} else {
					out = agentAnswer{status: firstStatus, body: firstAnswer}
				}
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if open, err := st.seedOpen(); err != nil {
			return err
		} else if open {
			out = agentRefused(refusedSeeding)
			return nil
		}
		var err error
		if out, err = fn(st); err != nil {
			return err
		}
		if out.forget || out.status == http.StatusTooManyRequests {
			return nil
		}
		body, err := json.Marshal(out.body)
		if err != nil {
			return err
		}
		_, err = st.Tx.Exec(ctx, `INSERT INTO action_answers (org, caller, id, body_hash, status, answer) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT DO NOTHING`, string(org), agent, id, hash, out.status, body)
		return err
	})
	return out, err
}

func readBody(w http.ResponseWriter, r *http.Request, into any) ([]byte, bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err == nil {
		err = json.Unmarshal(raw, into)
	}
	if err != nil {
		send(w, http.StatusConflict, contract.Refusal{Refused: refusedUnreadable})
		return nil, false
	}
	return raw, true
}

func answerAgent(w http.ResponseWriter, r *http.Request, out agentAnswer, err error) {
	if err != nil {
		fail(w, r, err)
		return
	}
	send(w, out.status, out.body)
}

func (s *Store) handleAgentPost(w http.ResponseWriter, r *http.Request, org ids.Org, agent string) {
	var in contract.AgentPost
	raw, ok := readBody(w, r, &in)
	if !ok {
		return
	}
	out, err := s.agentAct(r.Context(), org, agent, in.Id, "post", raw, func(st *Step) (agentAnswer, error) {
		if !s.agents.takeRate(org, agent, s.Clock.Now()) {
			return agentAnswer{status: http.StatusTooManyRequests, body: contract.Refusal{Refused: refusedSlowDown}}, nil
		}
		message := ids.New("msg")
		st.agentBecauseOf = in.BecauseOf
		done, err := st.send(contract.ActionSend{Kind: "send", Id: in.Id, As: agent, Message: contract.NewMessage{
			Id: message, Conversation: in.Conversation, Thread: in.Thread, Text: in.Text, Mentions: in.Mentions, Level: in.Level}})
		if err != nil {
			s.agents.giveBack(org, agent)
			return agentAnswer{}, err
		}
		if done.refused != "" {
			s.agents.giveBack(org, agent)
			return agentRefused(done.refused), nil
		}
		return agentAnswer{status: http.StatusOK, body: contract.AgentPosted{Message: message, Number: *done.number}}, nil
	})
	answerAgent(w, r, out, err)
}

func (s *Store) handleAgentReact(w http.ResponseWriter, r *http.Request, org ids.Org, agent string) {
	var in contract.AgentReaction
	raw, ok := readBody(w, r, &in)
	if !ok {
		return
	}
	out, err := s.agentAct(r.Context(), org, agent, in.Id, "react", raw, func(st *Step) (agentAnswer, error) {
		done, err := st.react(contract.ActionReact{Kind: "react", Id: in.Id, As: agent, Message: in.Message, Emoji: in.Emoji, On: in.On})
		if err != nil {
			return agentAnswer{}, err
		}
		if done.refused != "" {
			return agentRefused(done.refused), nil
		}
		number := st.head // already so: nothing was written, the log stands here
		if done.number != nil {
			number = *done.number
		}
		return agentAnswer{status: http.StatusOK, body: contract.AgentPosted{Message: in.Message, Number: number}}, nil
	})
	answerAgent(w, r, out, err)
}

func (s *Store) handleAgentDirect(w http.ResponseWriter, r *http.Request, org ids.Org, agent string) {
	var in contract.AgentDirect
	if _, ok := readBody(w, r, &in); !ok {
		return
	}
	var out agentAnswer
	err := s.Save(r.Context(), org, func(st *Step) error {
		if open, err := st.seedOpen(); err != nil {
			return err
		} else if open {
			out = agentRefused(refusedSeeding)
			return nil
		}
		done, err := st.openDirect(contract.ActionOpenDirect{Kind: "open_direct", As: agent, With: in.With})
		if err != nil {
			return err
		}
		if done.refused != "" {
			out = agentRefused(done.refused)
			return nil
		}
		out = agentAnswer{status: http.StatusOK, body: contract.AgentDirectAnswer{Conversation: *done.conversation}}
		return nil
	})
	answerAgent(w, r, out, err)
}

func (s *Store) handleAgentStatus(w http.ResponseWriter, r *http.Request, org ids.Org, agent string) {
	var in contract.AgentStatus
	if _, ok := readBody(w, r, &in); !ok {
		return
	}
	if in.State != "working" && in.State != "waiting" && in.State != "done" {
		send(w, http.StatusConflict, contract.Refusal{Refused: refusedUnreadable})
		return
	}
	var unknown bool
	err := orgdb.InOrg(r.Context(), s.Dir, org, func(tx pgx.Tx) error {
		if in.On != nil {
			var ok bool
			if err := tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM people WHERE org = $1 AND id = $2)`, string(org), *in.On).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				unknown = true
				return nil
			}
		}
		_, err := tx.Exec(r.Context(), `INSERT INTO agent_status (org, agent, state, on_person) VALUES ($1, $2, $3, $4)
			ON CONFLICT (org, agent) DO UPDATE SET state = EXCLUDED.state, on_person = EXCLUDED.on_person`,
			string(org), agent, string(in.State), in.On)
		return err
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	if unknown {
		send(w, http.StatusNotFound, contract.Refusal{Refused: refusedUnknownPerson})
		return
	}
	s.agents.pushed(org, "", contract.LiveAgentStatus{AgentStatus: contract.AgentStatusBody{Agent: agent, State: in.State, On: in.On}})
	send(w, http.StatusOK, contract.Empty{})
}

// withAgentStatus puts each agent's latest status on the person, for a fresh load.
func withAgentStatus(ctx context.Context, tx pgx.Tx, org ids.Org, people []contract.PersonView) error {
	rows, err := tx.Query(ctx, `SELECT agent, state, on_person FROM agent_status WHERE org = $1`, string(org))
	if err != nil {
		return err
	}
	defer rows.Close()
	statuses := map[string]*contract.PersonStatus{}
	for rows.Next() {
		var agent, state string
		var on *string
		if err := rows.Scan(&agent, &state, &on); err != nil {
			return err
		}
		statuses[agent] = &contract.PersonStatus{State: contract.AgentState(state), On: on}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range people {
		people[i].Status = statuses[people[i].Id]
	}
	return nil
}

func (s *Store) handleAgentSubscribe(w http.ResponseWriter, r *http.Request, org ids.Org, agent string) {
	conversation := r.PathValue("conversation")
	var member bool
	err := orgdb.InOrg(r.Context(), s.Dir, org, func(tx pgx.Tx) error {
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM conversation_people p JOIN conversations c ON c.org = p.org AND c.id = p.conversation
			WHERE p.org = $1 AND p.conversation = $2 AND p.person = $3 AND c.type = 'channel')`, string(org), conversation, agent).Scan(&member); err != nil || !member {
			return err
		}
		_, err := tx.Exec(r.Context(), `INSERT INTO agent_subscriptions (org, agent, conversation) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			string(org), agent, conversation)
		return err
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	if !member {
		send(w, http.StatusNotFound, contract.Refusal{Refused: refusedNotAMember})
		return
	}
	send(w, http.StatusOK, contract.Empty{})
}

func (s *Store) handleAgentUnsubscribe(w http.ResponseWriter, r *http.Request, org ids.Org, agent string) {
	err := orgdb.InOrg(r.Context(), s.Dir, org, func(tx pgx.Tx) error {
		_, err := tx.Exec(r.Context(), `DELETE FROM agent_subscriptions WHERE org = $1 AND agent = $2 AND conversation = $3`,
			string(org), agent, r.PathValue("conversation"))
		return err
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	send(w, http.StatusOK, contract.Empty{})
}

func (s *Store) handleAgentMessages(w http.ResponseWriter, r *http.Request, org ids.Org, agent string) {
	v := r.URL.Query()
	q := MessagesQuery{Conversation: r.PathValue("conversation"), As: agent}
	if t := v.Get("thread"); t != "" {
		q.Thread = &t
	}
	for name, into := range map[string]**int64{"around": &q.Around, "before": &q.Before} {
		if text := v.Get(name); text != "" {
			n, err := strconv.ParseInt(text, 10, 64)
			if err != nil {
				send(w, http.StatusBadRequest, contract.Refusal{Refused: "bad " + name})
				return
			}
			*into = &n
		}
	}
	if text := v.Get("limit"); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil || n <= 0 {
			send(w, http.StatusBadRequest, contract.Refusal{Refused: "bad limit"})
			return
		}
		q.Limit = n
	}
	if strings.HasPrefix(q.Conversation, "dm:") {
		var mine bool
		err := orgdb.InOrg(r.Context(), s.Dir, org, func(tx pgx.Tx) error {
			return tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM conversation_people WHERE org = $1 AND conversation = $2 AND person = $3)`,
				string(org), q.Conversation, agent).Scan(&mine)
		})
		if err != nil {
			fail(w, r, err)
			return
		}
		if !mine {
			send(w, http.StatusConflict, contract.Refusal{Refused: refusedNotParticipant})
			return
		}
	}
	answer, err := s.Messages(r.Context(), org, q)
	if err != nil {
		fail(w, r, err)
		return
	}
	send(w, http.StatusOK, answer)
}
