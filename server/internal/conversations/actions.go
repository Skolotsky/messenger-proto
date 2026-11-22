package conversations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"contexia/internal/shared/contract"
	"contexia/internal/shared/ids"

	"github.com/jackc/pgx/v5"
)

// The refusals of the people door. A refusal is final: trying again cannot help.
const (
	refusedUnreadable      = "unreadable"
	refusedUnknownKind     = "unknown kind"
	refusedIDReused        = "id reused"
	refusedDeleted         = "message deleted"
	refusedUnknownPerson   = "unknown person"
	refusedUnknownConv     = "unknown conversation"
	refusedUnknownMessage  = "unknown message"
	refusedUnknownThread   = "unknown thread"
	refusedNotYours        = "not your message"
	refusedNotParticipant  = "not a participant"
	refusedDirectThread    = "no threads in a direct conversation"
	refusedMessageIDTaken  = "message id taken"
	refusedEmptyMessage    = "empty message"
	refusedBadLevel        = "unknown level"
	refusedNameTaken       = "name taken"
	refusedBadName         = "bad channel name"
	refusedNotChannel      = "not a channel"
	refusedWithYourself    = "not with yourself"
	refusedPastEnd         = "past the end of the log"
	refusedBadDial         = "unknown dial level"
	refusedNotChannelLevel = "not a channel level"
	refusedBadRule         = "unknown rule"
	refusedEmptyEmoji      = "empty emoji"
	refusedEmptyKey        = "empty key"
	refusedSeeding         = "seeding"
)

// outcome is what an action came to. A non-empty refused means nothing was
// written for it.
type outcome struct {
	number       *int64
	conversation *string
	refused      string
}

func refuse(reason string) (outcome, error) { return outcome{refused: reason}, nil }

func saved(number int64) (outcome, error) { return outcome{number: &number}, nil }

// Act handles one action of the people door and returns the status and answer.
// The first answer to an ID is kept, inside the step that saves the action, so
// a resend after any failure gets the same answer and changes nothing (G-003).
func (s *Store) Act(ctx context.Context, org ids.Org, raw []byte) (int, any, error) {
	var head struct {
		Id   string `json:"id"`
		As   string `json:"as"`
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &head); err != nil || head.Id == "" || head.As == "" {
		return http.StatusConflict, contract.Refusal{Refused: refusedUnreadable}, nil
	}
	hash, err := bodyHash(raw)
	if err != nil {
		return http.StatusConflict, contract.Refusal{Refused: refusedUnreadable}, nil
	}
	var status int
	var answer any
	err = s.Save(ctx, org, func(st *Step) error {
		if rememberActionIDs {
			var firstHash string
			var firstStatus int
			var firstAnswer json.RawMessage
			err := st.Tx.QueryRow(ctx, `SELECT body_hash, status, answer FROM action_answers WHERE org = $1 AND caller = $2 AND id = $3`,
				string(org), head.As, head.Id).Scan(&firstHash, &firstStatus, &firstAnswer)
			if err == nil {
				if firstHash != hash {
					status, answer = http.StatusConflict, contract.Refusal{Refused: refusedIDReused}
					return nil
				}
				status, answer = firstStatus, firstAnswer
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		out, err := st.dispatch(raw, head.Kind, head.As)
		if err != nil {
			return err
		}
		if out.refused != "" {
			status, answer = http.StatusConflict, contract.Refusal{Refused: out.refused}
		} else {
			status, answer = http.StatusOK, contract.ActionSaved{Saved: true, Number: out.number, Conversation: out.conversation}
		}
		if !rememberActionIDs {
			return nil
		}
		body, err := json.Marshal(answer)
		if err != nil {
			return err
		}
		_, err = st.Tx.Exec(ctx, `INSERT INTO action_answers (org, caller, id, body_hash, status, answer) VALUES ($1, $2, $3, $4, $5, $6)`,
			string(org), head.As, head.Id, hash, status, body)
		return err
	})
	if err != nil {
		return 0, nil, err
	}
	return status, answer, nil
}

// bodyHash is the same for the same JSON, whatever its key order or spacing.
func bodyHash(raw []byte) (string, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

var knownKinds = []string{"send", "edit", "delete", "create_channel", "add_people", "open_direct", "react", "read",
	"keep", "done", "dial", "focus_rule", "focus_person", "focus_auto_add", "focus_only", "release_setting",
	"preference", "profile_line"}

func (s *Step) dispatch(raw []byte, kind, as string) (outcome, error) {
	if !slices.Contains(knownKinds, kind) {
		return refuse(refusedUnknownKind)
	}
	var action contract.PeopleAction
	if err := json.Unmarshal(raw, &action); err != nil {
		return refuse(refusedUnreadable)
	}
	if open, err := s.seedOpen(); err != nil {
		return outcome{}, err
	} else if open {
		return refuse(refusedSeeding)
	}
	if ok, err := s.personExists(as); err != nil {
		return outcome{}, err
	} else if !ok {
		return refuse(refusedUnknownPerson)
	}
	switch a := action.Value.(type) {
	case contract.ActionSend:
		return s.send(a)
	case contract.ActionEdit:
		return s.edit(a)
	case contract.ActionDelete:
		return s.deleteMessage(a)
	case contract.ActionCreateChannel:
		return s.createChannel(a)
	case contract.ActionAddPeople:
		return s.addPeopleAction(a)
	case contract.ActionOpenDirect:
		return s.openDirect(a)
	case contract.ActionReact:
		return s.react(a)
	case contract.ActionRead:
		return s.read(a)
	case contract.ActionKeep:
		return s.keepOrDone(a.As, a.Conversation, a.Thread, contract.ChangeReaderKeep{
			Kind: "reader.keep", Person: a.As, Conversation: a.Conversation, Thread: a.Thread})
	case contract.ActionDone:
		return s.keepOrDone(a.As, a.Conversation, a.Thread, contract.ChangeReaderDone{
			Kind: "reader.done", Person: a.As, Conversation: a.Conversation, Thread: a.Thread})
	case contract.ActionDial:
		return s.dial(a)
	case contract.ActionFocusRule:
		return s.focusRule(a)
	case contract.ActionFocusPerson:
		if ok, err := s.personExists(a.About); err != nil {
			return outcome{}, err
		} else if !ok {
			return refuse(refusedUnknownPerson)
		}
		return s.fact(contract.ChangeReaderFocusPerson{Kind: "reader.focus_person", Person: a.As, About: a.About, On: a.On})
	case contract.ActionFocusAutoAdd:
		return s.fact(contract.ChangeReaderFocusAutoAdd{Kind: "reader.focus_auto_add", Person: a.As, On: a.On})
	case contract.ActionFocusOnly:
		return s.fact(contract.ChangeReaderFocusOnly{Kind: "reader.focus_only", Person: a.As, On: a.On})
	case contract.ActionReleaseSetting:
		return s.fact(contract.ChangeReaderReleaseSetting{Kind: "reader.release_setting", Person: a.As, On: a.On})
	case contract.ActionPreference:
		if a.Key == "" {
			return refuse(refusedEmptyKey)
		}
		return s.fact(contract.ChangeReaderPreference{Kind: "reader.preference", Person: a.As, Key: a.Key, Value: a.Value})
	case contract.ActionProfileLine:
		return s.fact(contract.ChangeReaderProfileLine{Kind: "reader.profile_line", Person: a.As, Text: a.Text})
	}
	return refuse(refusedUnknownKind)
}

func (s *Step) fact(change any) (outcome, error) {
	number, err := s.Append(change)
	if err != nil {
		return outcome{}, err
	}
	return saved(number)
}

func (s *Step) seedOpen() (bool, error) {
	var open bool
	err := s.Tx.QueryRow(s.ctx, `SELECT EXISTS (SELECT 1 FROM seed_sessions WHERE org = $1 AND NOT sealed)`, string(s.org)).Scan(&open)
	return open, err
}

func (s *Step) personExists(id string) (bool, error) {
	var ok bool
	err := s.Tx.QueryRow(s.ctx, `SELECT EXISTS (SELECT 1 FROM people WHERE org = $1 AND id = $2)`, string(s.org), id).Scan(&ok)
	return ok, err
}

func (s *Step) conversationType(id string) (string, error) {
	var kind string
	err := s.Tx.QueryRow(s.ctx, `SELECT type FROM conversations WHERE org = $1 AND id = $2`, string(s.org), id).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return kind, err
}

func (s *Step) participants(conversation string) ([]string, error) {
	var people []string
	err := s.Tx.QueryRow(s.ctx, `SELECT coalesce(array_agg(person ORDER BY person), '{}') FROM conversation_people
		WHERE org = $1 AND conversation = $2`, string(s.org), conversation).Scan(&people)
	return people, err
}

// checkRow says why a row cannot be acted on, or "" if it can.
func (s *Step) checkRow(conversation string, thread *string) (string, error) {
	kind, err := s.conversationType(conversation)
	if err != nil {
		return "", err
	}
	if kind == "" {
		return refusedUnknownConv, nil
	}
	if thread == nil {
		return "", nil
	}
	if kind == "direct" {
		return refusedDirectThread, nil
	}
	root, err := s.loadMessage(*thread)
	if err != nil {
		return "", err
	}
	if root == nil || root.Conversation != conversation || root.Thread != nil {
		return refusedUnknownThread, nil
	}
	return "", nil
}

func (s *Step) checkWriting(text string, mentions []string, level *contract.Level) (string, error) {
	if strings.TrimSpace(text) == "" {
		return refusedEmptyMessage, nil
	}
	if level != nil && *level != "flag" && *level != "info" {
		return refusedBadLevel, nil
	}
	for _, p := range mentions {
		if p == "channel" {
			continue
		}
		ok, err := s.personExists(p)
		if err != nil {
			return "", err
		}
		if !ok {
			return refusedUnknownPerson, nil
		}
	}
	return "", nil
}

func (s *Step) send(a contract.ActionSend) (outcome, error) {
	m := a.Message
	if m.Id == "" {
		return refuse(refusedUnreadable)
	}
	kind, err := s.conversationType(m.Conversation)
	if err != nil {
		return outcome{}, err
	}
	if kind == "" {
		return refuse(refusedUnknownConv)
	}
	var participants []string
	if kind == "direct" {
		if m.Thread != nil {
			return refuse(refusedDirectThread)
		}
		if participants, err = s.participants(m.Conversation); err != nil {
			return outcome{}, err
		}
		if !slices.Contains(participants, a.As) {
			return refuse(refusedNotParticipant)
		}
	}
	if m.Thread != nil {
		root, err := s.loadMessage(*m.Thread)
		if err != nil {
			return outcome{}, err
		}
		if root == nil || root.Conversation != m.Conversation || root.Thread != nil {
			return refuse(refusedUnknownThread)
		}
		if root.Deleted {
			// A deleted root stays only while replies depend on it (D-004).
			var live bool
			if err := s.Tx.QueryRow(s.ctx, `SELECT EXISTS (SELECT 1 FROM messages WHERE org = $1 AND thread = $2 AND NOT deleted)`,
				string(s.org), root.Id).Scan(&live); err != nil {
				return outcome{}, err
			}
			if !live {
				return refuse(refusedDeleted)
			}
		}
	}
	if existing, err := s.loadMessage(m.Id); err != nil {
		return outcome{}, err
	} else if existing != nil {
		return refuse(refusedMessageIDTaken)
	}
	if why, err := s.checkWriting(m.Text, m.Mentions, m.Level); err != nil {
		return outcome{}, err
	} else if why != "" {
		return refuse(why)
	}
	var isAgent bool
	if err := s.Tx.QueryRow(s.ctx, `SELECT is_agent FROM people WHERE org = $1 AND id = $2`, string(s.org), a.As).Scan(&isAgent); err != nil {
		return outcome{}, err
	}
	message := contract.Message{Id: m.Id, Number: s.Next(), Version: 1, Conversation: m.Conversation, Thread: m.Thread,
		Author: a.As, AuthorIsAgent: isAgent, Participants: participants, Text: m.Text, Mentions: unique(m.Mentions),
		Level: m.Level, Reactions: []contract.Reaction{}, CreatedAt: s.At()}
	number, err := s.Append(contract.ChangeMessageCreated{Kind: "message.created", Message: message})
	if err != nil {
		return outcome{}, err
	}
	// A reply is acting on its row, and so is writing in a direct conversation.
	readers := []string{}
	if m.Thread != nil || kind == "direct" {
		readers = append(readers, a.As)
	}
	watchers, err := s.watchers(m.Conversation, m.Thread)
	if err != nil {
		return outcome{}, err
	}
	for _, p := range unique(append(readers, watchers...)) {
		if err := s.readUp(p, m.Conversation, m.Thread, number); err != nil {
			return outcome{}, err
		}
	}
	return saved(number)
}

// watchers are the people with this row open on a live connection.
func (s *Step) watchers(conversation string, thread *string) ([]string, error) {
	rows, err := s.Tx.Query(s.ctx, `SELECT DISTINCT person FROM watching
		WHERE org = $1 AND conversation = $2 AND thread IS NOT DISTINCT FROM $3 ORDER BY person`, string(s.org), conversation, thread)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// readUp writes a reader.read if it moves the person's position forward.
func (s *Step) readUp(person, conversation string, thread *string, upTo int64) error {
	var at int64
	err := s.Tx.QueryRow(s.ctx, `SELECT up_to FROM reader_reads WHERE org = $1 AND person = $2 AND conversation = $3 AND thread IS NOT DISTINCT FROM $4`,
		string(s.org), person, conversation, thread).Scan(&at)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if upTo <= at {
		return nil
	}
	_, err = s.Append(contract.ChangeReaderRead{Kind: "reader.read", Person: person, Conversation: conversation, Thread: thread, UpTo: upTo})
	return err
}

// readWholeRow is what acting implies: read up to the row's newest message.
func (s *Step) readWholeRow(person, conversation string, thread *string) error {
	var newest *int64
	var err error
	if thread != nil {
		err = s.Tx.QueryRow(s.ctx, `SELECT max(number) FROM messages WHERE org = $1 AND conversation = $2 AND (id = $3 OR thread = $3)`,
			string(s.org), conversation, *thread).Scan(&newest)
	} else {
		err = s.Tx.QueryRow(s.ctx, `SELECT max(number) FROM messages WHERE org = $1 AND conversation = $2 AND thread IS NULL`,
			string(s.org), conversation).Scan(&newest)
	}
	if err != nil || newest == nil {
		return err
	}
	return s.readUp(person, conversation, thread, *newest)
}

// rowOf is the row a message is on for this person (contracts README, "Row").
func (s *Step) rowOf(m *contract.Message, person string) (*string, error) {
	if m.Thread != nil {
		return m.Thread, nil
	}
	kind, err := s.conversationType(m.Conversation)
	if err != nil || kind == "direct" {
		return nil, err
	}
	if slices.Contains(m.Mentions, person) {
		id := m.Id
		return &id, nil
	}
	return nil, nil
}

func (s *Step) ownMessage(id, as string) (*contract.Message, string, error) {
	m, err := s.loadMessage(id)
	if err != nil {
		return nil, "", err
	}
	if m == nil {
		return nil, refusedUnknownMessage, nil
	}
	if m.Deleted {
		return nil, refusedDeleted, nil
	}
	if m.Author != as {
		return nil, refusedNotYours, nil
	}
	return m, "", nil
}

func (s *Step) edit(a contract.ActionEdit) (outcome, error) {
	m, why, err := s.ownMessage(a.Message, a.As)
	if err != nil {
		return outcome{}, err
	}
	if why != "" {
		return refuse(why)
	}
	if why, err := s.checkWriting(a.Text, a.Mentions, a.Level); err != nil {
		return outcome{}, err
	} else if why != "" {
		return refuse(why)
	}
	stamp := s.At()
	m.Version++
	m.Text, m.Mentions, m.Level, m.EditedAt = a.Text, unique(a.Mentions), a.Level, &stamp
	return s.fact(contract.ChangeMessageChanged{Kind: "message.changed", Message: *m})
}

func (s *Step) deleteMessage(a contract.ActionDelete) (outcome, error) {
	m, why, err := s.ownMessage(a.Message, a.As)
	if err != nil {
		return outcome{}, err
	}
	if why != "" {
		return refuse(why)
	}
	return s.fact(contract.ChangeMessageDeleted{Kind: "message.deleted",
		Message: contract.DeletedMessage{Id: m.Id, Version: m.Version + 1, Conversation: m.Conversation, Thread: m.Thread}})
}

func (s *Step) createChannel(a contract.ActionCreateChannel) (outcome, error) {
	if a.Name == "" || strings.ContainsAny(a.Name, " \t\r\n#") {
		return refuse(refusedBadName)
	}
	id := "#" + a.Name
	if kind, err := s.conversationType(id); err != nil {
		return outcome{}, err
	} else if kind != "" {
		return refuse(refusedNameTaken)
	}
	people := unique(append([]string{a.As}, a.People...))
	for _, p := range people {
		if ok, err := s.personExists(p); err != nil {
			return outcome{}, err
		} else if !ok {
			return refuse(refusedUnknownPerson)
		}
	}
	number, err := s.Append(contract.ChangeChannelCreated{Kind: "channel.created", Conversation: id, Name: a.Name, People: people})
	if err != nil {
		return outcome{}, err
	}
	return outcome{number: &number, conversation: &id}, nil
}

func (s *Step) addPeopleAction(a contract.ActionAddPeople) (outcome, error) {
	kind, err := s.conversationType(a.Conversation)
	if err != nil {
		return outcome{}, err
	}
	if kind == "" {
		return refuse(refusedUnknownConv)
	}
	if kind != "channel" {
		return refuse(refusedNotChannel)
	}
	present, err := s.participants(a.Conversation)
	if err != nil {
		return outcome{}, err
	}
	added := []string{}
	for _, p := range unique(a.People) {
		if ok, err := s.personExists(p); err != nil {
			return outcome{}, err
		} else if !ok {
			return refuse(refusedUnknownPerson)
		}
		if !slices.Contains(present, p) {
			added = append(added, p)
		}
	}
	if len(added) == 0 {
		return outcome{}, nil
	}
	return s.fact(contract.ChangeChannelPeopleAdded{Kind: "channel.people_added", Conversation: a.Conversation, People: added})
}

// DirectID is the one conversation of a pair of people.
func DirectID(a, b string) string {
	if b < a {
		a, b = b, a
	}
	return "dm:" + a + ":" + b
}

func (s *Step) openDirect(a contract.ActionOpenDirect) (outcome, error) {
	if a.With == a.As {
		return refuse(refusedWithYourself)
	}
	if ok, err := s.personExists(a.With); err != nil {
		return outcome{}, err
	} else if !ok {
		return refuse(refusedUnknownPerson)
	}
	id := DirectID(a.As, a.With)
	kind, err := s.conversationType(id)
	if err != nil {
		return outcome{}, err
	}
	if kind != "" {
		return outcome{conversation: &id}, nil
	}
	people := []string{a.As, a.With}
	slices.Sort(people)
	number, err := s.Append(contract.ChangeDirectCreated{Kind: "direct.created", Conversation: id, People: people})
	if err != nil {
		return outcome{}, err
	}
	return outcome{number: &number, conversation: &id}, nil
}

func (s *Step) react(a contract.ActionReact) (outcome, error) {
	if a.Emoji == "" {
		return refuse(refusedEmptyEmoji)
	}
	m, err := s.loadMessage(a.Message)
	if err != nil {
		return outcome{}, err
	}
	if m == nil {
		return refuse(refusedUnknownMessage)
	}
	if m.Deleted {
		return refuse(refusedDeleted)
	}
	var has bool
	if err := s.Tx.QueryRow(s.ctx, `SELECT EXISTS (SELECT 1 FROM reactions WHERE org = $1 AND message = $2 AND emoji = $3 AND person = $4)`,
		string(s.org), a.Message, a.Emoji, a.As).Scan(&has); err != nil {
		return outcome{}, err
	}
	if has == a.On {
		return outcome{}, nil // already so: nothing to save
	}
	var change any
	if a.On {
		change = contract.ChangeReactionAdded{Kind: "reaction.added", Person: a.As, Emoji: a.Emoji, Message: m.Id, Conversation: m.Conversation, Thread: m.Thread}
	} else {
		change = contract.ChangeReactionRemoved{Kind: "reaction.removed", Person: a.As, Emoji: a.Emoji, Message: m.Id, Conversation: m.Conversation, Thread: m.Thread}
	}
	number, err := s.Append(change)
	if err != nil {
		return outcome{}, err
	}
	thread, err := s.rowOf(m, a.As)
	if err != nil {
		return outcome{}, err
	}
	// D-035: only up to this message. What is newer has not been seen.
	if err := s.readUp(a.As, m.Conversation, thread, m.Number); err != nil {
		return outcome{}, err
	}
	return saved(number)
}

func (s *Step) read(a contract.ActionRead) (outcome, error) {
	if why, err := s.checkRow(a.Conversation, a.Thread); err != nil {
		return outcome{}, err
	} else if why != "" {
		return refuse(why)
	}
	if a.UpTo < 0 || a.UpTo > s.head {
		return refuse(refusedPastEnd)
	}
	before := s.head
	if err := s.readUp(a.As, a.Conversation, a.Thread, a.UpTo); err != nil {
		return outcome{}, err
	}
	if s.head == before {
		return outcome{}, nil // a read position only moves forward
	}
	return saved(s.head)
}

func (s *Step) keepOrDone(as, conversation string, thread *string, change any) (outcome, error) {
	if why, err := s.checkRow(conversation, thread); err != nil {
		return outcome{}, err
	} else if why != "" {
		return refuse(why)
	}
	number, err := s.Append(change)
	if err != nil {
		return outcome{}, err
	}
	if err := s.readWholeRow(as, conversation, thread); err != nil {
		return outcome{}, err
	}
	return saved(number)
}

var dialLevels = []contract.DialLevel{"focus", "all", "default", "muted", "needs_you", "fyi", "off", "auto"}
var channelDials = []contract.DialLevel{"focus", "all", "default", "muted"}

func (s *Step) dial(a contract.ActionDial) (outcome, error) {
	if !slices.Contains(dialLevels, a.Level) {
		return refuse(refusedBadDial)
	}
	if a.Thread == nil && !slices.Contains(channelDials, a.Level) {
		return refuse(refusedNotChannelLevel)
	}
	if why, err := s.checkRow(a.Conversation, a.Thread); err != nil {
		return outcome{}, err
	} else if why != "" {
		return refuse(why)
	}
	return s.fact(contract.ChangeReaderDial{Kind: "reader.dial", Person: a.As, Conversation: a.Conversation, Thread: a.Thread, Level: a.Level})
}

func (s *Step) focusRule(a contract.ActionFocusRule) (outcome, error) {
	switch a.Rule.Type {
	case "person", "conversation", "text":
		if a.Rule.Value == "" {
			return refuse(refusedBadRule)
		}
	case "item_kind":
		if a.Rule.Value != "flags" && a.Rule.Value != "mentions" {
			return refuse(refusedBadRule)
		}
	default:
		return refuse(refusedBadRule)
	}
	return s.fact(contract.ChangeReaderFocusRule{Kind: "reader.focus_rule", Person: a.As, Rule: a.Rule, On: a.On})
}

func unique(list []string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}
