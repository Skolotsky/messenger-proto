package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

// orgHub is what one copy runs for one organization with open screens: one reader
// of the change log and one of the feed's row log. Each sends in number order.
type orgHub struct {
	l      *Live
	org    ids.Org
	ctx    context.Context
	cancel context.CancelFunc

	mu    sync.Mutex
	conns map[*conn]struct{}

	logWake  chan struct{}
	feedWake chan struct{}

	// Touched by the log reader alone.
	pos    int64
	posSet bool

	// Touched by the feed reader alone.
	feedPos     *int64
	feedEdition *int64
}

func newOrgHub(l *Live, org ids.Org) *orgHub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &orgHub{l: l, org: org, ctx: ctx, cancel: cancel, conns: map[*conn]struct{}{},
		logWake: make(chan struct{}, 1), feedWake: make(chan struct{}, 1)}
	go h.runLog()
	go h.runFeed()
	return h
}

func (h *orgHub) stop() { h.cancel() }

func (h *orgHub) add(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[c] = struct{}{}
}

func (h *orgHub) remove(c *conn) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, c)
	return len(h.conns)
}

func (h *orgHub) connections() []*conn {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*conn, 0, len(h.conns))
	for c := range h.conns {
		out = append(out, c)
	}
	return out
}

func (h *orgHub) wakeLog() {
	select {
	case h.logWake <- struct{}{}:
	default:
	}
}

func (h *orgHub) wakeFeed() {
	select {
	case h.feedWake <- struct{}{}:
	default:
	}
}

// resume records where a screen says it is; the readers act on it, so a replay
// and the live tail can never interleave.
func (h *orgHub) resume(c *conn, n contract.ResumeNumbers) {
	c.mu.Lock()
	conv := n.Conversations
	c.resumeConv = &conv
	nums := n
	c.resumeFeed = &nums
	c.mu.Unlock()
	h.wakeLog()
	h.wakeFeed()
}

func (h *orgHub) runLog() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-h.logWake:
		}
		if err := h.passLog(h.ctx); err != nil && h.ctx.Err() == nil {
			slog.Warn("reading the log for screens", "org", h.org, "err", err)
		}
	}
}

// passLog brings every screen up to the log's head: first the live tail, then
// the screens that asked to resume, up to the same place.
func (h *orgHub) passLog(ctx context.Context) error {
	exists, retired, head, err := h.l.s.orgState(ctx, h.org)
	if err != nil {
		return err
	}
	if !exists || retired {
		for _, c := range h.connections() {
			c.mu.Lock()
			already := c.goneWritten
			c.goneWritten = true
			c.mu.Unlock()
			if !already {
				c.enqueue(outMessage{body: goneMessage, last: true})
			}
		}
		return nil
	}
	if !h.posSet {
		h.pos, h.posSet = head, true
	}
	if !hubSendsWhatItIsHanded {
		for h.pos < head {
			entries, err := h.readEntries(ctx, h.pos, head)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				break
			}
			if err := h.dispatch(ctx, entries); err != nil {
				return err
			}
			h.pos = entries[len(entries)-1].number
		}
	} else {
		h.pos = max(h.pos, head)
	}
	for _, c := range h.connections() {
		c.mu.Lock()
		from := c.resumeConv
		c.resumeConv = nil
		c.mu.Unlock()
		if from == nil {
			continue
		}
		if err := h.replay(ctx, c, *from); err != nil {
			c.mu.Lock()
			if c.resumeConv == nil {
				c.resumeConv = from // try again on the next pass
			}
			c.mu.Unlock()
			return err
		}
	}
	return nil
}

// logEntry is one row of the change log, as far as a screen cares.
type logEntry struct {
	number int64
	kind   string
	body   logBody
}

type logBody struct {
	Person       string          `json:"person"`
	Conversation string          `json:"conversation"`
	Thread       *string         `json:"thread"`
	Message      json.RawMessage `json:"message"`
	People       []string        `json:"people"`
	Key          string          `json:"key"`
	Value        string          `json:"value"`
	Text         string          `json:"text"`
	UpTo         int64           `json:"up_to"`
	Author       string          `json:"author"`
}

// where is the conversation and thread an entry belongs to.
func (e logEntry) where() (string, *string) {
	if strings.HasPrefix(e.kind, "message.") {
		var m struct {
			Conversation string  `json:"conversation"`
			Thread       *string `json:"thread"`
		}
		_ = json.Unmarshal(e.body.Message, &m)
		return m.Conversation, m.Thread
	}
	return e.body.Conversation, e.body.Thread
}

// concerns is concernsPerson for one entry, against the conversations the screen
// knows the person is in.
func concerns(e logEntry, person string, mine map[string]bool) bool {
	switch {
	case e.kind == "tick":
		return false
	case e.kind == "reader.read", e.kind == "reader.preference":
		return e.body.Person == person
	case e.kind == "reader.profile_line":
		return true
	case strings.HasPrefix(e.kind, "reader."):
		return false
	case e.kind == "person.joined":
		return true
	case e.kind == "channel.created", e.kind == "direct.created":
		return slices.Contains(e.body.People, person)
	}
	conversation, _ := e.where()
	return mine[conversation]
}

func parseEntry(number int64, kind string, raw []byte) (logEntry, error) {
	e := logEntry{number: number, kind: kind}
	err := json.Unmarshal(raw, &e.body)
	return e, err
}

func (h *orgHub) readEntries(ctx context.Context, after, upTo int64) ([]logEntry, error) {
	var out []logEntry
	err := orgdb.InOrg(ctx, h.l.s.Dir, h.org, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT number, kind, body FROM change_log
			WHERE org = $1 AND number > $2 AND number <= $3 ORDER BY number LIMIT $4`, string(h.org), after, upTo, logPage)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var number int64
			var kind string
			var raw []byte
			if err := rows.Scan(&number, &kind, &raw); err != nil {
				return err
			}
			e, err := parseEntry(number, kind, raw)
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}

// dispatch sends entries, in order, to every screen they concern.
func (h *orgHub) dispatch(ctx context.Context, entries []logEntry) error {
	conns := h.connections()
	return orgdb.InOrg(ctx, h.l.s.Dir, h.org, func(tx pgx.Tx) error {
		for _, e := range entries {
			var targets []*conn
			people := []string{}
			for _, c := range conns {
				if c.takes(e) {
					targets = append(targets, c)
					if !slices.Contains(people, c.person) {
						people = append(people, c.person)
					}
				}
			}
			if len(targets) == 0 {
				continue
			}
			parts, err := renderParts(ctx, tx, h.org, e, people)
			if err != nil {
				return err
			}
			for _, c := range targets {
				c.sendUpdate(e.number, parts[c.person])
			}
		}
		return nil
	})
}

// takes notes a membership change and says whether this entry is for this screen.
func (c *conn) takes(e logEntry) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch e.kind {
	case "channel.created", "direct.created", "channel.people_added":
		if slices.Contains(e.body.People, c.person) {
			c.mine[e.body.Conversation] = true
		}
	}
	if c.last < 0 || e.number <= c.last {
		return false
	}
	return concerns(e, c.person, c.mine)
}

func (c *conn) sendUpdate(number int64, parts []contract.ConversationPart) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if number <= c.last {
		return
	}
	if parts == nil {
		parts = []contract.ConversationPart{}
	}
	b, err := json.Marshal(contract.LiveConversationsUpdate{Stream: "conversations", Number: number, After: c.last, Parts: parts})
	if err != nil {
		slog.Error("encode update", "err", err)
		return
	}
	c.last = number
	c.enqueue(outMessage{body: b})
}

// replay sends a resuming screen what it missed, up to the reader's place, with
// the same rule the first load used for its number.
func (h *orgHub) replay(ctx context.Context, c *conn, from int64) error {
	return orgdb.InOrg(ctx, h.l.s.Dir, h.org, func(tx pgx.Tx) error {
		mine, err := myConversations(ctx, tx, h.org, c.person)
		if err != nil {
			return err
		}
		c.mu.Lock()
		c.mine = map[string]bool{}
		for _, m := range mine {
			c.mine[m] = true
		}
		c.last = from
		c.mu.Unlock()
		if from >= h.pos {
			return nil
		}
		tooFar := h.pos-from > maxReplayScan
		var entries []logEntry
		if !tooFar {
			rows, err := tx.Query(ctx, `SELECT number, kind, body FROM change_log
				WHERE org = $1 AND number > $4 AND number <= $5 AND `+concernsPerson+` ORDER BY number LIMIT $6`,
				string(h.org), c.person, mine, from, h.pos, replayLimit+1)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var number int64
				var kind string
				var raw []byte
				if err := rows.Scan(&number, &kind, &raw); err != nil {
					return err
				}
				e, err := parseEntry(number, kind, raw)
				if err != nil {
					return err
				}
				entries = append(entries, e)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			rows.Close()
			tooFar = len(entries) > replayLimit
		}
		if tooFar {
			newest, err := lastNumberFor(ctx, tx, h.org, c.person, mine)
			if err != nil {
				return err
			}
			c.mu.Lock()
			c.last = max(from, min(newest, h.pos))
			c.mu.Unlock()
			b, _ := json.Marshal(contract.LiveTooFarBehind{TooFarBehind: "conversations"})
			c.enqueue(outMessage{body: b})
			return nil
		}
		for _, e := range entries {
			parts, err := renderParts(ctx, tx, h.org, e, []string{c.person})
			if err != nil {
				return err
			}
			c.sendUpdate(e.number, parts[c.person])
		}
		return nil
	})
}

func myConversations(ctx context.Context, tx pgx.Tx, org ids.Org, person string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT conversation FROM conversation_people WHERE org = $1 AND person = $2`, string(org), person)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if out == nil {
		out = []string{}
	}
	return out, err
}

// sendHanded is the test-only fault for the "reads from the log" rule: it sends
// exactly what a save handed over, in the order the saves got here.
func (h *orgHub) sendHanded(first, last int64) {
	entries, err := h.readEntries(h.ctx, first-1, last)
	if err == nil {
		err = h.dispatch(h.ctx, entries)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Warn("handed update", "err", err)
	}
}

// renderParts turns one log entry into each person's list of parts.
func renderParts(ctx context.Context, tx pgx.Tx, org ids.Org, e logEntry, people []string) (map[string][]contract.ConversationPart, error) {
	out := make(map[string][]contract.ConversationPart, len(people))
	common := func(p ...contract.ConversationPart) {
		for _, person := range people {
			out[person] = append(out[person], p...)
		}
	}
	conversation, thread := e.where()
	switch e.kind {
	case "message.created", "message.changed":
		var m contract.Message
		if err := json.Unmarshal(e.body.Message, &m); err != nil {
			return nil, err
		}
		if e.kind == "message.created" {
			common(contract.ConversationPart{Value: contract.PartMessageCreated{Kind: e.kind, Message: m}})
		} else {
			common(contract.ConversationPart{Value: contract.PartMessageChanged{Kind: e.kind, Message: m}})
			return out, nil
		}
		if thread != nil {
			part, err := threadSummaryPart(ctx, tx, org, conversation, *thread)
			if err != nil {
				return nil, err
			}
			common(part)
		}
		return out, addUnread(ctx, tx, org, out, people, conversation, thread)
	case "message.deleted":
		var m contract.DeletedMessage
		if err := json.Unmarshal(e.body.Message, &m); err != nil {
			return nil, err
		}
		common(contract.ConversationPart{Value: contract.PartMessageDeleted{Kind: e.kind, Message: m}})
		if thread != nil {
			part, err := threadSummaryPart(ctx, tx, org, conversation, *thread)
			if err != nil {
				return nil, err
			}
			common(part)
		}
		return out, addUnread(ctx, tx, org, out, people, conversation, thread)
	case "reaction.added", "reaction.removed":
		var id string
		if err := json.Unmarshal(e.body.Message, &id); err != nil {
			return nil, err
		}
		reactions, err := loadReactions(ctx, tx, org, []string{id})
		if err != nil {
			return nil, err
		}
		list := reactions[id]
		if list == nil {
			list = []contract.Reaction{}
		}
		common(contract.ConversationPart{Value: contract.PartReactionChanged{Kind: "reaction.changed", Message: id,
			Conversation: conversation, Thread: thread, Reactions: list}})
	case "reply.started":
		var id string
		if err := json.Unmarshal(e.body.Message, &id); err != nil {
			return nil, err
		}
		common(contract.ConversationPart{Value: contract.PartReplyStarted{Kind: e.kind, Message: id, Conversation: conversation, Thread: thread, Author: e.body.Author}})
	case "reply.abandoned":
		var id string
		if err := json.Unmarshal(e.body.Message, &id); err != nil {
			return nil, err
		}
		common(contract.ConversationPart{Value: contract.PartReplyAbandoned{Kind: e.kind, Message: id, Conversation: conversation, Thread: thread}})
	case "person.joined":
		list, err := loadPeople(ctx, tx, org, `AND id = $2`, e.body.Person)
		if err != nil {
			return nil, err
		}
		if len(list) == 1 {
			common(contract.ConversationPart{Value: contract.PartPersonAdded{Kind: "person.added", PersonView: list[0]}})
		}
	case "channel.created", "direct.created":
		for _, person := range people {
			if err := addConversation(ctx, tx, org, out, person, conversation); err != nil {
				return nil, err
			}
		}
	case "channel.people_added":
		common(contract.ConversationPart{Value: contract.PartPeopleAdded{Kind: "people.added", Conversation: conversation, People: e.body.People}})
		for _, person := range people {
			if slices.Contains(e.body.People, person) {
				if err := addConversation(ctx, tx, org, out, person, conversation); err != nil {
					return nil, err
				}
			}
		}
	case "reader.read":
		common(contract.ConversationPart{Value: contract.PartReadMoved{Kind: "read.moved", Conversation: conversation, Thread: thread, UpTo: e.body.UpTo}})
		return out, addUnread(ctx, tx, org, out, people, conversation, thread)
	case "reader.preference":
		common(contract.ConversationPart{Value: contract.PartPreference{Kind: "preference", Key: e.body.Key, Value: e.body.Value}})
	case "reader.profile_line":
		common(contract.ConversationPart{Value: contract.PartProfileLine{Kind: "profile_line", Person: e.body.Person, Text: e.body.Text}})
	}
	return out, nil
}

func addConversation(ctx context.Context, tx pgx.Tx, org ids.Org, out map[string][]contract.ConversationPart, person, conversation string) error {
	list, err := loadConversations(ctx, tx, org, person)
	if err != nil {
		return err
	}
	for _, s := range list {
		if s.Conversation == conversation {
			out[person] = append(out[person], contract.ConversationPart{Value: contract.PartConversationAdded{Kind: "conversation.added", ConversationSummary: s}})
		}
	}
	return nil
}

func threadSummaryPart(ctx context.Context, tx pgx.Tx, org ids.Org, conversation, thread string) (contract.ConversationPart, error) {
	p := contract.PartThreadSummary{Kind: "thread.summary", Conversation: conversation, Thread: thread, People: []string{}}
	var authors []string
	var lastAt *string
	err := tx.QueryRow(ctx, `SELECT count(*), to_char(max(created_at) AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
		  coalesce(array_agg(author ORDER BY number), '{}')
		FROM messages WHERE org = $1 AND thread = $2 AND NOT deleted`, string(org), thread).Scan(&p.Replies, &lastAt, &authors)
	if err != nil {
		return contract.ConversationPart{}, err
	}
	p.LastReplyAt = lastAt
	p.People = unique(authors)
	return contract.ConversationPart{Value: p}, nil
}

// addUnread adds each person's marks for the row: the channel's sidebar mark
// and, for a thread, the thread's. Same rules as the first load
// (loadConversations, withSummaries).
func addUnread(ctx context.Context, tx pgx.Tx, org ids.Org, out map[string][]contract.ConversationPart, people []string, conversation string, thread *string) error {
	rows, err := tx.Query(ctx, `
		SELECT p.person,
		  EXISTS (
		    SELECT 1 FROM messages m
		    LEFT JOIN reader_reads tr ON tr.org = m.org AND tr.person = p.person AND tr.conversation = m.conversation AND tr.thread = m.id
		    WHERE m.org = $1 AND m.conversation = $2 AND m.thread IS NULL AND NOT m.deleted AND m.author <> p.person
		      AND m.number > coalesce(cr.up_to, 0) AND m.number > coalesce(tr.up_to, 0)),
		  $4::text IS NOT NULL AND (
		    EXISTS (SELECT 1 FROM messages r WHERE r.org = $1 AND r.thread = $4 AND NOT r.deleted AND r.author <> p.person
		            AND r.number > coalesce(th.up_to, 0))
		    OR EXISTS (SELECT 1 FROM messages root WHERE root.org = $1 AND root.id = $4 AND NOT root.deleted AND root.author <> p.person
		            AND root.number > coalesce(th.up_to, 0) AND root.number > coalesce(cr.up_to, 0)))
		FROM unnest($3::text[]) AS p(person)
		LEFT JOIN reader_reads cr ON cr.org = $1 AND cr.person = p.person AND cr.conversation = $2 AND cr.thread IS NULL
		LEFT JOIN reader_reads th ON th.org = $1 AND th.person = p.person AND th.conversation = $2 AND th.thread = $4`,
		string(org), conversation, people, thread)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var person string
		var channel, inThread bool
		if err := rows.Scan(&person, &channel, &inThread); err != nil {
			return err
		}
		out[person] = append(out[person], contract.ConversationPart{Value: contract.PartUnread{Kind: "unread", Conversation: conversation, Thread: nil, Unread: channel}})
		if thread != nil {
			out[person] = append(out[person], contract.ConversationPart{Value: contract.PartUnread{Kind: "unread", Conversation: conversation, Thread: thread, Unread: inThread}})
		}
	}
	return rows.Err()
}
