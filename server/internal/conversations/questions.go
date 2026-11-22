package conversations

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"
	"contexia/internal/shared/orgdb"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound says a question named something the organization does not have.
var ErrNotFound = errors.New("not found")

const (
	defaultMessages = 50
	maxMessages     = 200
	maxSuggestions  = 10
)

const messageColumns = `id, number, version, conversation, thread, author, author_is_agent, participants,
	text, mentions, level, stopped, created_at, edited_at, deleted`

type scanner interface{ Scan(...any) error }

func scanMessage(row scanner) (contract.Message, error) {
	var m contract.Message
	var level *string
	var createdAt time.Time
	var editedAt *time.Time
	if err := row.Scan(&m.Id, &m.Number, &m.Version, &m.Conversation, &m.Thread, &m.Author, &m.AuthorIsAgent, &m.Participants,
		&m.Text, &m.Mentions, &level, &m.Stopped, &createdAt, &editedAt, &m.Deleted); err != nil {
		return m, err
	}
	if level != nil {
		l := contract.Level(*level)
		m.Level = &l
	}
	m.CreatedAt = createdAt.UTC().Format(TimeFormat)
	if editedAt != nil {
		e := editedAt.UTC().Format(TimeFormat)
		m.EditedAt = &e
	}
	m.Reactions = []contract.Reaction{}
	return m, nil
}

// loadMessage returns the message with its reactions, or nil.
func (s *Step) loadMessage(id string) (*contract.Message, error) {
	m, err := scanMessage(s.Tx.QueryRow(s.ctx, `SELECT `+messageColumns+` FROM messages WHERE org = $1 AND id = $2`, string(s.org), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	reactions, err := loadReactions(s.ctx, s.Tx, s.org, []string{id})
	if err != nil {
		return nil, err
	}
	if r := reactions[id]; r != nil {
		m.Reactions = r
	}
	return &m, nil
}

func loadReactions(ctx context.Context, tx pgx.Tx, org ids.Org, messages []string) (map[string][]contract.Reaction, error) {
	rows, err := tx.Query(ctx, `SELECT message, emoji, array_agg(person ORDER BY number, person) FROM reactions
		WHERE org = $1 AND message = ANY($2) GROUP BY message, emoji ORDER BY message, min(number), emoji`, string(org), messages)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]contract.Reaction{}
	for rows.Next() {
		var id string
		var r contract.Reaction
		if err := rows.Scan(&id, &r.Emoji, &r.People); err != nil {
			return nil, err
		}
		out[id] = append(out[id], r)
	}
	return out, rows.Err()
}

// question runs fn while saves of this organization wait, so what it reads and
// the log number it answers with belong together. Saves commit in number order
// under this same row, so nothing can fall between the two.
func (s *Store) question(ctx context.Context, org ids.Org, fn func(tx pgx.Tx, head int64) error) error {
	return orgdb.InOrg(ctx, s.Dir, org, func(tx pgx.Tx) error {
		var head int64
		err := tx.QueryRow(ctx, `SELECT head FROM org_log WHERE org = $1 FOR SHARE`, string(org)).Scan(&head)
		if err != nil {
			return unknownOrg(err)
		}
		return fn(tx, head)
	})
}

func loadPeople(ctx context.Context, tx pgx.Tx, org ids.Org, where string, args ...any) ([]contract.PersonView, error) {
	rows, err := tx.Query(ctx, `SELECT id, name, is_agent, line FROM people WHERE org = $1 `+where,
		append([]any{string(org)}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []contract.PersonView{}
	for rows.Next() {
		var p contract.PersonView
		if err := rows.Scan(&p.Id, &p.Name, &p.IsAgent, &p.Line); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// People is the operators' list of everyone in the organization.
func (s *Store) People(ctx context.Context, org ids.Org) (contract.PeopleList, error) {
	var out contract.PeopleList
	err := s.question(ctx, org, func(tx pgx.Tx, _ int64) error {
		var err error
		out.People, err = loadPeople(ctx, tx, org, `ORDER BY name, id`)
		return err
	})
	return out, err
}

// Start is a screen's first full load for the conversations stream.
func (s *Store) Start(ctx context.Context, org ids.Org, person string) (contract.StartAnswer, error) {
	out := contract.StartAnswer{Org: string(org), OrgName: string(org)}
	err := s.question(ctx, org, func(tx pgx.Tx, _ int64) error {
		var display *string
		err := tx.QueryRow(ctx, `SELECT name FROM seed_sessions WHERE org = $1`, string(org)).Scan(&display)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if display != nil {
			out.OrgName = *display
		}
		if out.People, err = loadPeople(ctx, tx, org, `ORDER BY name, id`); err != nil {
			return err
		}
		i := slices.IndexFunc(out.People, func(p contract.PersonView) bool { return p.Id == person })
		if i < 0 {
			return ErrNotFound
		}
		out.Person = out.People[i]
		if out.Preferences, err = loadPreferences(ctx, tx, org, person); err != nil {
			return err
		}
		if out.Conversations, err = loadConversations(ctx, tx, org, person); err != nil {
			return err
		}
		mine := make([]string, len(out.Conversations))
		for i, c := range out.Conversations {
			mine[i] = c.Conversation
		}
		out.Number, err = lastNumberFor(ctx, tx, org, person, mine)
		return err
	})
	return out, err
}

func loadPreferences(ctx context.Context, tx pgx.Tx, org ids.Org, person string) ([]contract.Preference, error) {
	rows, err := tx.Query(ctx, `SELECT key, value FROM reader_preferences WHERE org = $1 AND person = $2 ORDER BY key`, string(org), person)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []contract.Preference{}
	for rows.Next() {
		var p contract.Preference
		if err := rows.Scan(&p.Key, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// loadConversations lists the person's conversations with their sidebar mark.
// The mark counts top-level messages past the channel's position that were not
// already read through their own thread; a reply never marks it (D-032).
func loadConversations(ctx context.Context, tx pgx.Tx, org ids.Org, person string) ([]contract.ConversationSummary, error) {
	rows, err := tx.Query(ctx, `
		SELECT c.id, c.type, c.name,
		  (SELECT array_agg(p.person ORDER BY p.person) FROM conversation_people p WHERE p.org = c.org AND p.conversation = c.id),
		  coalesce(d.level, 'default'),
		  EXISTS (
		    SELECT 1 FROM messages m
		    LEFT JOIN reader_reads tr ON tr.org = m.org AND tr.person = $2 AND tr.conversation = m.conversation AND tr.thread = m.id
		    WHERE m.org = c.org AND m.conversation = c.id AND m.thread IS NULL AND NOT m.deleted AND m.author <> $2
		      AND m.number > coalesce(cr.up_to, 0) AND m.number > coalesce(tr.up_to, 0))
		FROM conversation_people me
		JOIN conversations c ON c.org = me.org AND c.id = me.conversation
		LEFT JOIN reader_reads cr ON cr.org = c.org AND cr.person = $2 AND cr.conversation = c.id AND cr.thread IS NULL
		LEFT JOIN reader_dials d ON d.org = c.org AND d.person = $2 AND d.conversation = c.id AND d.thread IS NULL
		WHERE me.org = $1 AND me.person = $2
		ORDER BY c.type, c.name`, string(org), person)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []contract.ConversationSummary{}
	for rows.Next() {
		var c contract.ConversationSummary
		var kind, dial string
		if err := rows.Scan(&c.Conversation, &kind, &c.Name, &c.People, &dial, &c.Unread); err != nil {
			return nil, err
		}
		c.Type = contract.ConversationType(kind)
		c.Dial = contract.ChannelDial(dial)
		if !slices.Contains(channelDials, contract.DialLevel(dial)) {
			c.Dial = "default"
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// lastNumberFor is the newest change of the conversations stream that concerns
// this person. The live connection (T-053) must send exactly these, since a
// screen applies an update only if its "after" equals the number it holds: the
// person's own read and preference, anyone's joining or profile line, and what
// happens in their conversations. Other reader facts travel on the feed stream.
func lastNumberFor(ctx context.Context, tx pgx.Tx, org ids.Org, person string, mine []string) (int64, error) {
	var number int64
	err := tx.QueryRow(ctx, `
		SELECT number FROM change_log
		WHERE org = $1 AND kind <> 'tick' AND (
		  CASE
		    WHEN kind IN ('reader.read', 'reader.preference') THEN body->>'person' = $2
		    WHEN kind = 'reader.profile_line' THEN true
		    WHEN kind LIKE 'reader.%' THEN false
		    WHEN kind = 'person.joined' THEN true
		    WHEN kind IN ('channel.created', 'direct.created') THEN body->'people' ? $2
		    WHEN kind LIKE 'message.%' THEN body->'message'->>'conversation' = ANY($3)
		    ELSE body->>'conversation' = ANY($3)
		  END)
		ORDER BY number DESC LIMIT 1`, string(org), person, mine).Scan(&number)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return number, err
}

// MessagesQuery is what a screen asks of a conversation. As is the person whose
// read position and unread marks the answer carries; without it they are empty.
type MessagesQuery struct {
	Conversation string
	Thread       *string
	Around       *int64
	Before       *int64
	Limit        int
	As           string
}

// Messages answers with a window of a conversation or a thread. A deleted
// message is shown only while a reply depends on it (D-004).
func (s *Store) Messages(ctx context.Context, org ids.Org, q MessagesQuery) (contract.MessagesAnswer, error) {
	out := contract.MessagesAnswer{Messages: []contract.MessageView{}, InProgress: []contract.ReplyInProgress{}}
	if q.Limit <= 0 {
		q.Limit = defaultMessages
	}
	q.Limit = min(q.Limit, maxMessages)
	err := s.question(ctx, org, func(tx pgx.Tx, head int64) error {
		out.Number = head
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM conversations WHERE org = $1 AND id = $2)`, string(org), q.Conversation).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		where := `m.org = $1 AND m.conversation = $2 AND m.thread IS NULL AND (NOT m.deleted OR ` + liveReply + `)`
		args := []any{string(org), q.Conversation}
		if q.Thread != nil {
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM messages WHERE org = $1 AND conversation = $2 AND id = $3 AND thread IS NULL)`,
				string(org), q.Conversation, *q.Thread).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrNotFound
			}
			where = `m.org = $1 AND m.conversation = $2 AND (m.id = $3 OR m.thread = $3) AND (NOT m.deleted OR (m.id = $3 AND ` + liveReply + `))`
			args = append(args, *q.Thread)
		}
		page, err := window(ctx, tx, where, args, q)
		if err != nil {
			return err
		}
		if len(page) > 0 {
			args2 := append(slices.Clone(args), page[0].Number)
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM messages m WHERE `+where+` AND m.number < $`+strconv.Itoa(len(args2))+`)`, args2...).Scan(&out.HasBefore); err != nil {
				return err
			}
			args2 = append(slices.Clone(args), page[len(page)-1].Number)
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM messages m WHERE `+where+` AND m.number > $`+strconv.Itoa(len(args2))+`)`, args2...).Scan(&out.HasAfter); err != nil {
				return err
			}
		}
		var channelAt int64
		if q.As != "" {
			var at *int64
			err := tx.QueryRow(ctx, `SELECT up_to FROM reader_reads WHERE org = $1 AND person = $2 AND conversation = $3 AND thread IS NOT DISTINCT FROM $4`,
				string(org), q.As, q.Conversation, q.Thread).Scan(&at)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			out.ReadUpTo = at
			if q.Thread == nil && at != nil {
				channelAt = *at
			} else if q.Thread != nil {
				err := tx.QueryRow(ctx, `SELECT up_to FROM reader_reads WHERE org = $1 AND person = $2 AND conversation = $3 AND thread IS NULL`,
					string(org), q.As, q.Conversation).Scan(&channelAt)
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return err
				}
			}
		}
		out.Messages, err = withSummaries(ctx, tx, org, page, q.As, channelAt)
		return err
	})
	return out, err
}

const liveReply = `EXISTS (SELECT 1 FROM messages r WHERE r.org = m.org AND r.thread = m.id AND NOT r.deleted)`

// window picks the messages of the answer, oldest first.
func window(ctx context.Context, tx pgx.Tx, where string, args []any, q MessagesQuery) ([]contract.Message, error) {
	fetch := func(extra, order string, limit int, bound *int64) ([]contract.Message, error) {
		all := slices.Clone(args)
		clause := where
		if bound != nil {
			all = append(all, *bound)
			clause += ` AND m.number ` + extra + ` $` + strconv.Itoa(len(all))
		}
		sql := `SELECT ` + prefixed(messageColumns) + ` FROM messages m WHERE ` + clause + ` ORDER BY m.number ` + order + ` LIMIT ` + strconv.Itoa(limit)
		rows, err := tx.Query(ctx, sql, all...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []contract.Message{}
		for rows.Next() {
			m, err := scanMessage(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, m)
		}
		return out, rows.Err()
	}
	if q.Around != nil {
		upper := q.Limit / 2
		older, err := fetch("<=", "DESC", q.Limit-upper, q.Around)
		if err != nil {
			return nil, err
		}
		newer, err := fetch(">", "ASC", upper, q.Around)
		if err != nil {
			return nil, err
		}
		slices.Reverse(older)
		return append(older, newer...), nil
	}
	page, err := fetch("<", "DESC", q.Limit, q.Before)
	if err != nil {
		return nil, err
	}
	slices.Reverse(page)
	return page, nil
}

func prefixed(columns string) string {
	parts := strings.Split(columns, ",")
	for i, c := range parts {
		parts[i] = "m." + strings.TrimSpace(c)
	}
	return strings.Join(parts, ", ")
}

// withSummaries adds reactions and, under each root, its thread summary. A
// thread is unread while its root is past both the channel's and the thread's
// position, or a reply is past the thread's (contracts README, "Read position").
func withSummaries(ctx context.Context, tx pgx.Tx, org ids.Org, page []contract.Message, as string, channelAt int64) ([]contract.MessageView, error) {
	out := make([]contract.MessageView, 0, len(page))
	if len(page) == 0 {
		return out, nil
	}
	messageIDs := make([]string, len(page))
	roots := []string{}
	for i, m := range page {
		messageIDs[i] = m.Id
		if m.Thread == nil {
			roots = append(roots, m.Id)
		}
	}
	reactions, err := loadReactions(ctx, tx, org, messageIDs)
	if err != nil {
		return nil, err
	}
	type summary struct {
		replies     int64
		lastAt      time.Time
		authors     []string
		newestOther *int64
	}
	summaries := map[string]summary{}
	rows, err := tx.Query(ctx, `SELECT thread, count(*), max(created_at), array_agg(author ORDER BY number),
		  max(number) FILTER (WHERE author <> $3)
		FROM messages WHERE org = $1 AND thread = ANY($2) AND NOT deleted GROUP BY thread`, string(org), roots, as)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var v summary
		if err := rows.Scan(&id, &v.replies, &v.lastAt, &v.authors, &v.newestOther); err != nil {
			return nil, err
		}
		summaries[id] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	threadAt := map[string]int64{}
	if as != "" {
		rows, err := tx.Query(ctx, `SELECT thread, up_to FROM reader_reads WHERE org = $1 AND person = $2 AND thread = ANY($3)`, string(org), as, roots)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var at int64
			if err := rows.Scan(&id, &at); err != nil {
				return nil, err
			}
			threadAt[id] = at
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	for _, m := range page {
		if r := reactions[m.Id]; r != nil {
			m.Reactions = r
		}
		view := contract.MessageView{Message: m}
		if v, ok := summaries[m.Id]; ok && m.Thread == nil {
			last := v.lastAt.UTC().Format(TimeFormat)
			unread := false
			if as != "" {
				at := threadAt[m.Id]
				unread = v.newestOther != nil && *v.newestOther > at ||
					!m.Deleted && m.Author != as && m.Number > at && m.Number > channelAt
			}
			view.ThreadSummary = &contract.ThreadSummary{Replies: v.replies, LastReplyAt: &last, People: unique(v.authors), Unread: unread}
		}
		out = append(out, view)
	}
	return out, nil
}

// Suggest answers the "@" box: people whose name or ID starts with the text, or
// who have a later word in their name that does, and the "channel" mention.
func (s *Store) Suggest(ctx context.Context, org ids.Org, q, as string) (contract.SuggestAnswer, error) {
	text := strings.ToLower(strings.TrimSpace(q))
	out := contract.SuggestAnswer{Everyone: strings.HasPrefix("channel", text)}
	like := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(text) + "%"
	err := s.question(ctx, org, func(tx pgx.Tx, _ int64) error {
		var err error
		out.People, err = loadPeople(ctx, tx, org,
			`AND id <> $2 AND (lower(name) LIKE $3 OR lower(id) LIKE $3 OR lower(name) LIKE '% ' || $3) ORDER BY name, id LIMIT `+strconv.Itoa(maxSuggestions),
			as, like)
		return err
	})
	return out, err
}
