package conversations

import (
	"fmt"
	"time"

	"contexia/internal/shared/contract"
)

// project applies one change to Conversations' own tables. Actions and the
// seed session go through it alike, so the tables always follow the log.
func (s *Step) project(change any, number int64) error {
	org := string(s.org)
	exec := func(sql string, args ...any) error {
		_, err := s.Tx.Exec(s.ctx, sql, append([]any{org}, args...)...)
		return err
	}
	switch c := change.(type) {
	case contract.ChangeMessageCreated:
		m := c.Message
		createdAt, err := parseTime(m.CreatedAt, s.at)
		if err != nil {
			return err
		}
		editedAt, err := parseOptional(m.EditedAt)
		if err != nil {
			return err
		}
		if err := exec(`INSERT INTO messages (org, id, number, version, conversation, thread, author, author_is_agent,
				participants, text, mentions, level, stopped, created_at, edited_at, deleted)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
			m.Id, number, m.Version, m.Conversation, m.Thread, m.Author, m.AuthorIsAgent,
			m.Participants, m.Text, nonNil(m.Mentions), m.Level, m.Stopped, createdAt, editedAt, m.Deleted); err != nil {
			return err
		}
		for _, r := range m.Reactions {
			for _, p := range r.People {
				if err := exec(`INSERT INTO reactions (org, message, emoji, person, number) VALUES ($1, $2, $3, $4, $5)
					ON CONFLICT DO NOTHING`, m.Id, r.Emoji, p, number); err != nil {
					return err
				}
			}
		}
		return nil
	case contract.ChangeMessageChanged:
		m := c.Message
		editedAt, err := parseOptional(m.EditedAt)
		if err != nil {
			return err
		}
		return exec(`UPDATE messages SET version = $3, text = $4, mentions = $5, level = $6, edited_at = $7
			WHERE org = $1 AND id = $2`, m.Id, m.Version, m.Text, nonNil(m.Mentions), m.Level, editedAt)
	case contract.ChangeMessageDeleted:
		if err := exec(`DELETE FROM reactions WHERE org = $1 AND message = $2`, c.Message.Id); err != nil {
			return err
		}
		return exec(`UPDATE messages SET version = $3, text = '', mentions = '{}', level = NULL, deleted = true
			WHERE org = $1 AND id = $2`, c.Message.Id, c.Message.Version)
	case contract.ChangeReactionAdded:
		return exec(`INSERT INTO reactions (org, message, emoji, person, number) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING`, c.Message, c.Emoji, c.Person, number)
	case contract.ChangeReactionRemoved:
		return exec(`DELETE FROM reactions WHERE org = $1 AND message = $2 AND emoji = $3 AND person = $4`, c.Message, c.Emoji, c.Person)
	case contract.ChangePersonJoined:
		return exec(`INSERT INTO people (org, id, name, is_agent) VALUES ($1, $2, $3, $4)`, c.Person, c.Name, c.IsAgent)
	case contract.ChangeChannelCreated:
		if err := exec(`INSERT INTO conversations (org, id, type, name) VALUES ($1, $2, 'channel', $3)`, c.Conversation, c.Name); err != nil {
			return err
		}
		return s.addPeople(c.Conversation, c.People)
	case contract.ChangeChannelPeopleAdded:
		return s.addPeople(c.Conversation, c.People)
	case contract.ChangeDirectCreated:
		if err := exec(`INSERT INTO conversations (org, id, type, name) VALUES ($1, $2, 'direct', $2)`, c.Conversation); err != nil {
			return err
		}
		return s.addPeople(c.Conversation, c.People)
	case contract.ChangeReaderRead:
		return exec(`INSERT INTO reader_reads (org, person, conversation, thread, up_to) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (org, person, conversation, thread) DO UPDATE SET up_to = GREATEST(reader_reads.up_to, EXCLUDED.up_to)`,
			c.Person, c.Conversation, c.Thread, c.UpTo)
	case contract.ChangeReaderKeep:
		return exec(`INSERT INTO reader_kept (org, person, conversation, thread) VALUES ($1, $2, $3, $4)
			ON CONFLICT (org, person, conversation, thread) DO NOTHING`, c.Person, c.Conversation, c.Thread)
	case contract.ChangeReaderDone:
		return exec(`DELETE FROM reader_kept WHERE org = $1 AND person = $2 AND conversation = $3 AND thread IS NOT DISTINCT FROM $4`,
			c.Person, c.Conversation, c.Thread)
	case contract.ChangeReaderDial:
		return exec(`INSERT INTO reader_dials (org, person, conversation, thread, level) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (org, person, conversation, thread) DO UPDATE SET level = EXCLUDED.level`,
			c.Person, c.Conversation, c.Thread, string(c.Level))
	case contract.ChangeReaderFocusRule:
		return exec(`INSERT INTO reader_focus_rules (org, person, type, value, is_on) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (org, person, type, value) DO UPDATE SET is_on = EXCLUDED.is_on`,
			c.Person, string(c.Rule.Type), c.Rule.Value, c.On)
	case contract.ChangeReaderFocusPerson:
		return exec(`INSERT INTO reader_focus_people (org, person, about, is_on) VALUES ($1, $2, $3, $4)
			ON CONFLICT (org, person, about) DO UPDATE SET is_on = EXCLUDED.is_on`, c.Person, c.About, c.On)
	case contract.ChangeReaderFocusAutoAdd:
		return exec(`INSERT INTO reader_settings (org, person, auto_add) VALUES ($1, $2, $3)
			ON CONFLICT (org, person) DO UPDATE SET auto_add = EXCLUDED.auto_add`, c.Person, c.On)
	case contract.ChangeReaderFocusOnly:
		return exec(`INSERT INTO reader_settings (org, person, focus_only) VALUES ($1, $2, $3)
			ON CONFLICT (org, person) DO UPDATE SET focus_only = EXCLUDED.focus_only`, c.Person, c.On)
	case contract.ChangeReaderReleaseSetting:
		return exec(`INSERT INTO reader_settings (org, person, release) VALUES ($1, $2, $3)
			ON CONFLICT (org, person) DO UPDATE SET release = EXCLUDED.release`, c.Person, c.On)
	case contract.ChangeReaderPreference:
		return exec(`INSERT INTO reader_preferences (org, person, key, value) VALUES ($1, $2, $3, $4)
			ON CONFLICT (org, person, key) DO UPDATE SET value = EXCLUDED.value`, c.Person, c.Key, c.Value)
	case contract.ChangeReaderProfileLine:
		return exec(`UPDATE people SET line = $3 WHERE org = $1 AND id = $2`, c.Person, c.Text)
	case contract.ChangeTick, contract.ChangeReplyStarted, contract.ChangeReplyAbandoned,
		contract.ChangeReaderWatchingStarted, contract.ChangeReaderWatchingEnded:
		// In the log only. The live connection (T-053) keeps the watching table
		// and the agent door (T-054) the text of a reply in progress.
		return nil
	}
	return fmt.Errorf("no way to apply a change of type %T", change)
}

func (s *Step) addPeople(conversation string, people []string) error {
	for _, p := range people {
		if _, err := s.Tx.Exec(s.ctx, `INSERT INTO conversation_people (org, conversation, person) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, string(s.org), conversation, p); err != nil {
			return err
		}
	}
	return nil
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func parseTime(v string, fallback time.Time) (time.Time, error) {
	if v == "" {
		return fallback, nil
	}
	return time.Parse(time.RFC3339Nano, v)
}

func parseOptional(v *string) (*time.Time, error) {
	if v == nil {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, *v)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
