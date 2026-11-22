// Package contract holds the Go form of contracts/schema/contexia.json.
// Written by hand from the schema and checked against every example; see contract_test.go.
package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Span is a from and to, in Unicode code points.
type Span [2]int64

// Level is one of the values the contract lists.
type Level string

// Bucket is one of the values the contract lists.
type Bucket string

// DialLevel is one of the values the contract lists.
type DialLevel string

// ChannelDial is one of the values the contract lists.
type ChannelDial string

// ItemKind is one of the values the contract lists.
type ItemKind string

// Stream is one of the values the contract lists.
type Stream string

// NoteNow is one of the values the contract lists.
type NoteNow string

// RuleType is one of the values the contract lists.
type RuleType string

// KeptBy is one of the values the contract lists.
type KeptBy string

// RowState is one of the values the contract lists.
type RowState string

// HistoryTab is one of the values the contract lists.
type HistoryTab string

// ConversationType is one of the values the contract lists.
type ConversationType string

// Mode is one of the values the contract lists.
type Mode string

// ReceiverState is one of the values the contract lists.
type ReceiverState string

// ReceiverName is one of the values the contract lists.
type ReceiverName string

// NoteState is one of the values the contract lists.
type NoteState string

// AgentState is one of the values the contract lists.
type AgentState string

// ScriptCommand is one of the values the contract lists.
type ScriptCommand string

// RowKey is the contract type of the same name.
type RowKey struct {
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
}

// Reaction is the contract type of the same name.
type Reaction struct {
	Emoji  string   `json:"emoji"`
	People []string `json:"people"`
}

// FocusRule is the contract type of the same name.
type FocusRule struct {
	Type  RuleType `json:"type"`
	Value string   `json:"value"`
}

// Message is the contract type of the same name.
type Message struct {
	Id            string     `json:"id"`
	Number        int64      `json:"number"`
	Version       int64      `json:"version"`
	Conversation  string     `json:"conversation"`
	Thread        *string    `json:"thread"`
	Author        string     `json:"author"`
	AuthorIsAgent bool       `json:"author_is_agent"`
	Participants  []string   `json:"participants"`
	Text          string     `json:"text"`
	Mentions      []string   `json:"mentions"`
	Level         *Level     `json:"level"`
	Reactions     []Reaction `json:"reactions"`
	Stopped       bool       `json:"stopped"`
	CreatedAt     string     `json:"created_at"`
	EditedAt      *string    `json:"edited_at"`
	Deleted       bool       `json:"deleted"`
}

// DeletedMessage is the contract type of the same name.
type DeletedMessage struct {
	Id           string  `json:"id"`
	Version      int64   `json:"version"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
}

// ThreadSummary is the contract type of the same name.
type ThreadSummary struct {
	Replies     int64    `json:"replies"`
	LastReplyAt *string  `json:"last_reply_at"`
	People      []string `json:"people"`
	Unread      bool     `json:"unread"`
}

// MessageView is the contract type of the same name.
type MessageView struct {
	Message       Message        `json:"message"`
	ThreadSummary *ThreadSummary `json:"thread_summary"`
}

// PersonStatus is the contract type of the same name.
type PersonStatus struct {
	State AgentState `json:"state"`
	On    *string    `json:"on"`
}

// PersonView is the contract type of the same name.
type PersonView struct {
	Id      string        `json:"id"`
	Name    string        `json:"name"`
	IsAgent bool          `json:"is_agent"`
	Line    *string       `json:"line"`
	Status  *PersonStatus `json:"status"`
}

// Preference is the contract type of the same name.
type Preference struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ConversationSummary is the contract type of the same name.
type ConversationSummary struct {
	Conversation string           `json:"conversation"`
	Type         ConversationType `json:"type"`
	Name         string           `json:"name"`
	People       []string         `json:"people"`
	Unread       bool             `json:"unread"`
	Dial         ChannelDial      `json:"dial"`
}

// ChangeMessageCreated is the contract type of the same name.
type ChangeMessageCreated struct {
	Kind     string  `json:"kind"`
	Org      string  `json:"org"`
	Number   int64   `json:"number"`
	Previous int64   `json:"previous"`
	At       string  `json:"at"`
	Message  Message `json:"message"`
}

// ChangeMessageChanged is the contract type of the same name.
type ChangeMessageChanged struct {
	Kind     string  `json:"kind"`
	Org      string  `json:"org"`
	Number   int64   `json:"number"`
	Previous int64   `json:"previous"`
	At       string  `json:"at"`
	Message  Message `json:"message"`
}

// ChangeMessageDeleted is the contract type of the same name.
type ChangeMessageDeleted struct {
	Kind     string         `json:"kind"`
	Org      string         `json:"org"`
	Number   int64          `json:"number"`
	Previous int64          `json:"previous"`
	At       string         `json:"at"`
	Message  DeletedMessage `json:"message"`
}

// ChangeReplyStarted is the contract type of the same name.
type ChangeReplyStarted struct {
	Kind         string  `json:"kind"`
	Org          string  `json:"org"`
	Number       int64   `json:"number"`
	Previous     int64   `json:"previous"`
	At           string  `json:"at"`
	Message      string  `json:"message"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	Author       string  `json:"author"`
}

// ChangeReplyAbandoned is the contract type of the same name.
type ChangeReplyAbandoned struct {
	Kind         string  `json:"kind"`
	Org          string  `json:"org"`
	Number       int64   `json:"number"`
	Previous     int64   `json:"previous"`
	At           string  `json:"at"`
	Message      string  `json:"message"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
}

// ChangeReactionAdded is the contract type of the same name.
type ChangeReactionAdded struct {
	Kind         string  `json:"kind"`
	Org          string  `json:"org"`
	Number       int64   `json:"number"`
	Previous     int64   `json:"previous"`
	At           string  `json:"at"`
	Person       string  `json:"person"`
	Emoji        string  `json:"emoji"`
	Message      string  `json:"message"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
}

// ChangeReactionRemoved is the contract type of the same name.
type ChangeReactionRemoved struct {
	Kind         string  `json:"kind"`
	Org          string  `json:"org"`
	Number       int64   `json:"number"`
	Previous     int64   `json:"previous"`
	At           string  `json:"at"`
	Person       string  `json:"person"`
	Emoji        string  `json:"emoji"`
	Message      string  `json:"message"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
}

// ChangePersonJoined is the contract type of the same name.
type ChangePersonJoined struct {
	Kind     string `json:"kind"`
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	At       string `json:"at"`
	Person   string `json:"person"`
	Name     string `json:"name"`
	IsAgent  bool   `json:"is_agent"`
}

// ChangeChannelCreated is the contract type of the same name.
type ChangeChannelCreated struct {
	Kind         string   `json:"kind"`
	Org          string   `json:"org"`
	Number       int64    `json:"number"`
	Previous     int64    `json:"previous"`
	At           string   `json:"at"`
	Conversation string   `json:"conversation"`
	Name         string   `json:"name"`
	People       []string `json:"people"`
}

// ChangeChannelPeopleAdded is the contract type of the same name.
type ChangeChannelPeopleAdded struct {
	Kind         string   `json:"kind"`
	Org          string   `json:"org"`
	Number       int64    `json:"number"`
	Previous     int64    `json:"previous"`
	At           string   `json:"at"`
	Conversation string   `json:"conversation"`
	People       []string `json:"people"`
}

// ChangeDirectCreated is the contract type of the same name.
type ChangeDirectCreated struct {
	Kind         string   `json:"kind"`
	Org          string   `json:"org"`
	Number       int64    `json:"number"`
	Previous     int64    `json:"previous"`
	At           string   `json:"at"`
	Conversation string   `json:"conversation"`
	People       []string `json:"people"`
}

// ChangeReaderRead is the contract type of the same name.
type ChangeReaderRead struct {
	Kind         string  `json:"kind"`
	Org          string  `json:"org"`
	Number       int64   `json:"number"`
	Previous     int64   `json:"previous"`
	At           string  `json:"at"`
	Person       string  `json:"person"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	UpTo         int64   `json:"up_to"`
}

// ChangeReaderWatchingStarted is the contract type of the same name.
type ChangeReaderWatchingStarted struct {
	Kind         string  `json:"kind"`
	Org          string  `json:"org"`
	Number       int64   `json:"number"`
	Previous     int64   `json:"previous"`
	At           string  `json:"at"`
	Person       string  `json:"person"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	Connection   string  `json:"connection"`
}

// ChangeReaderWatchingEnded is the contract type of the same name.
type ChangeReaderWatchingEnded struct {
	Kind         string  `json:"kind"`
	Org          string  `json:"org"`
	Number       int64   `json:"number"`
	Previous     int64   `json:"previous"`
	At           string  `json:"at"`
	Person       string  `json:"person"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	Connection   string  `json:"connection"`
}

// ChangeReaderKeep is the contract type of the same name.
type ChangeReaderKeep struct {
	Kind         string  `json:"kind"`
	Org          string  `json:"org"`
	Number       int64   `json:"number"`
	Previous     int64   `json:"previous"`
	At           string  `json:"at"`
	Person       string  `json:"person"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
}

// ChangeReaderDone is the contract type of the same name.
type ChangeReaderDone struct {
	Kind         string  `json:"kind"`
	Org          string  `json:"org"`
	Number       int64   `json:"number"`
	Previous     int64   `json:"previous"`
	At           string  `json:"at"`
	Person       string  `json:"person"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
}

// ChangeReaderDial is the contract type of the same name.
type ChangeReaderDial struct {
	Kind         string    `json:"kind"`
	Org          string    `json:"org"`
	Number       int64     `json:"number"`
	Previous     int64     `json:"previous"`
	At           string    `json:"at"`
	Person       string    `json:"person"`
	Conversation string    `json:"conversation"`
	Thread       *string   `json:"thread"`
	Level        DialLevel `json:"level"`
}

// ChangeReaderFocusRule is the contract type of the same name.
type ChangeReaderFocusRule struct {
	Kind     string    `json:"kind"`
	Org      string    `json:"org"`
	Number   int64     `json:"number"`
	Previous int64     `json:"previous"`
	At       string    `json:"at"`
	Person   string    `json:"person"`
	Rule     FocusRule `json:"rule"`
	On       bool      `json:"on"`
}

// ChangeReaderFocusPerson is the contract type of the same name.
type ChangeReaderFocusPerson struct {
	Kind     string `json:"kind"`
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	At       string `json:"at"`
	Person   string `json:"person"`
	About    string `json:"about"`
	On       bool   `json:"on"`
}

// ChangeReaderFocusAutoAdd is the contract type of the same name.
type ChangeReaderFocusAutoAdd struct {
	Kind     string `json:"kind"`
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	At       string `json:"at"`
	Person   string `json:"person"`
	On       bool   `json:"on"`
}

// ChangeReaderFocusOnly is the contract type of the same name.
type ChangeReaderFocusOnly struct {
	Kind     string `json:"kind"`
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	At       string `json:"at"`
	Person   string `json:"person"`
	On       bool   `json:"on"`
}

// ChangeReaderReleaseSetting is the contract type of the same name.
type ChangeReaderReleaseSetting struct {
	Kind     string `json:"kind"`
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	At       string `json:"at"`
	Person   string `json:"person"`
	On       bool   `json:"on"`
}

// ChangeReaderPreference is the contract type of the same name.
type ChangeReaderPreference struct {
	Kind     string `json:"kind"`
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	At       string `json:"at"`
	Person   string `json:"person"`
	Key      string `json:"key"`
	Value    string `json:"value"`
}

// ChangeReaderProfileLine is the contract type of the same name.
type ChangeReaderProfileLine struct {
	Kind     string `json:"kind"`
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	At       string `json:"at"`
	Person   string `json:"person"`
	Text     string `json:"text"`
}

// ChangeTick is the contract type of the same name.
type ChangeTick struct {
	Kind     string `json:"kind"`
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	At       string `json:"at"`
}

// ChangeSeedOpened is the contract type of the same name.
type ChangeSeedOpened struct {
	Kind     string `json:"kind"`
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	At       string `json:"at"`
}

// ChangeSeedSealed is the contract type of the same name.
type ChangeSeedSealed struct {
	Kind     string `json:"kind"`
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	At       string `json:"at"`
}

// Change is any one of its member types.
type Change struct{ Value any }

func (u Change) MarshalJSON() ([]byte, error) { return json.Marshal(u.Value) }

func (u *Change) UnmarshalJSON(b []byte) error {
	var k struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(b, &k); err != nil {
		return err
	}
	switch k.Kind {
	case "message.created":
		v, err := decode[ChangeMessageCreated](b)
		u.Value = v
		return err
	case "message.changed":
		v, err := decode[ChangeMessageChanged](b)
		u.Value = v
		return err
	case "message.deleted":
		v, err := decode[ChangeMessageDeleted](b)
		u.Value = v
		return err
	case "reply.started":
		v, err := decode[ChangeReplyStarted](b)
		u.Value = v
		return err
	case "reply.abandoned":
		v, err := decode[ChangeReplyAbandoned](b)
		u.Value = v
		return err
	case "reaction.added":
		v, err := decode[ChangeReactionAdded](b)
		u.Value = v
		return err
	case "reaction.removed":
		v, err := decode[ChangeReactionRemoved](b)
		u.Value = v
		return err
	case "person.joined":
		v, err := decode[ChangePersonJoined](b)
		u.Value = v
		return err
	case "channel.created":
		v, err := decode[ChangeChannelCreated](b)
		u.Value = v
		return err
	case "channel.people_added":
		v, err := decode[ChangeChannelPeopleAdded](b)
		u.Value = v
		return err
	case "direct.created":
		v, err := decode[ChangeDirectCreated](b)
		u.Value = v
		return err
	case "reader.read":
		v, err := decode[ChangeReaderRead](b)
		u.Value = v
		return err
	case "reader.watching_started":
		v, err := decode[ChangeReaderWatchingStarted](b)
		u.Value = v
		return err
	case "reader.watching_ended":
		v, err := decode[ChangeReaderWatchingEnded](b)
		u.Value = v
		return err
	case "reader.keep":
		v, err := decode[ChangeReaderKeep](b)
		u.Value = v
		return err
	case "reader.done":
		v, err := decode[ChangeReaderDone](b)
		u.Value = v
		return err
	case "reader.dial":
		v, err := decode[ChangeReaderDial](b)
		u.Value = v
		return err
	case "reader.focus_rule":
		v, err := decode[ChangeReaderFocusRule](b)
		u.Value = v
		return err
	case "reader.focus_person":
		v, err := decode[ChangeReaderFocusPerson](b)
		u.Value = v
		return err
	case "reader.focus_auto_add":
		v, err := decode[ChangeReaderFocusAutoAdd](b)
		u.Value = v
		return err
	case "reader.focus_only":
		v, err := decode[ChangeReaderFocusOnly](b)
		u.Value = v
		return err
	case "reader.release_setting":
		v, err := decode[ChangeReaderReleaseSetting](b)
		u.Value = v
		return err
	case "reader.preference":
		v, err := decode[ChangeReaderPreference](b)
		u.Value = v
		return err
	case "reader.profile_line":
		v, err := decode[ChangeReaderProfileLine](b)
		u.Value = v
		return err
	case "tick":
		v, err := decode[ChangeTick](b)
		u.Value = v
		return err
	case "seed.opened":
		v, err := decode[ChangeSeedOpened](b)
		u.Value = v
		return err
	case "seed.sealed":
		v, err := decode[ChangeSeedSealed](b)
		u.Value = v
		return err
	}
	return fmt.Errorf("Change: unknown kind %q", k.Kind)
}

// ChangeBatch is the contract type of the same name.
type ChangeBatch struct {
	Changes []Change `json:"changes"`
}

// ChangeApplied is the contract type of the same name.
type ChangeApplied struct {
	Applied bool  `json:"applied"`
	Already *bool `json:"already,omitempty"`
	Fenced  *bool `json:"fenced,omitempty"`
}

// IAmAt is the contract type of the same name.
type IAmAt struct {
	IAmAt int64 `json:"i_am_at"`
}

// OrganizationGone is the contract type of the same name: a retired
// organization's answer on every door and its last live message (D-033).
type OrganizationGone struct {
	OrganizationGone bool `json:"organization_gone"`
}

// OrganizationStopped is the contract type of the same name.
type OrganizationStopped struct {
	OrganizationStopped string `json:"organization_stopped"`
}

// NewMessage is the contract type of the same name.
type NewMessage struct {
	Id           string   `json:"id"`
	Conversation string   `json:"conversation"`
	Thread       *string  `json:"thread"`
	Text         string   `json:"text"`
	Mentions     []string `json:"mentions"`
	Level        *Level   `json:"level"`
}

// ActionSend is the contract type of the same name.
type ActionSend struct {
	Kind    string     `json:"kind"`
	Id      string     `json:"id"`
	As      string     `json:"as"`
	Message NewMessage `json:"message"`
}

// ActionEdit is the contract type of the same name.
type ActionEdit struct {
	Kind     string   `json:"kind"`
	Id       string   `json:"id"`
	As       string   `json:"as"`
	Message  string   `json:"message"`
	Text     string   `json:"text"`
	Mentions []string `json:"mentions"`
	Level    *Level   `json:"level"`
}

// ActionDelete is the contract type of the same name.
type ActionDelete struct {
	Kind    string `json:"kind"`
	Id      string `json:"id"`
	As      string `json:"as"`
	Message string `json:"message"`
}

// ActionCreateChannel is the contract type of the same name.
type ActionCreateChannel struct {
	Kind   string   `json:"kind"`
	Id     string   `json:"id"`
	As     string   `json:"as"`
	Name   string   `json:"name"`
	People []string `json:"people"`
}

// ActionAddPeople is the contract type of the same name.
type ActionAddPeople struct {
	Kind         string   `json:"kind"`
	Id           string   `json:"id"`
	As           string   `json:"as"`
	Conversation string   `json:"conversation"`
	People       []string `json:"people"`
}

// ActionOpenDirect is the contract type of the same name.
type ActionOpenDirect struct {
	Kind string `json:"kind"`
	Id   string `json:"id"`
	As   string `json:"as"`
	With string `json:"with"`
}

// ActionReact is the contract type of the same name.
type ActionReact struct {
	Kind    string `json:"kind"`
	Id      string `json:"id"`
	As      string `json:"as"`
	Message string `json:"message"`
	Emoji   string `json:"emoji"`
	On      bool   `json:"on"`
}

// ActionRead is the contract type of the same name.
type ActionRead struct {
	Kind         string  `json:"kind"`
	Id           string  `json:"id"`
	As           string  `json:"as"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	UpTo         int64   `json:"up_to"`
}

// ActionKeep is the contract type of the same name.
type ActionKeep struct {
	Kind         string  `json:"kind"`
	Id           string  `json:"id"`
	As           string  `json:"as"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
}

// ActionDone is the contract type of the same name.
type ActionDone struct {
	Kind         string  `json:"kind"`
	Id           string  `json:"id"`
	As           string  `json:"as"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
}

// ActionDial is the contract type of the same name.
type ActionDial struct {
	Kind         string    `json:"kind"`
	Id           string    `json:"id"`
	As           string    `json:"as"`
	Conversation string    `json:"conversation"`
	Thread       *string   `json:"thread"`
	Level        DialLevel `json:"level"`
}

// ActionFocusRule is the contract type of the same name.
type ActionFocusRule struct {
	Kind string    `json:"kind"`
	Id   string    `json:"id"`
	As   string    `json:"as"`
	Rule FocusRule `json:"rule"`
	On   bool      `json:"on"`
}

// ActionFocusPerson is the contract type of the same name.
type ActionFocusPerson struct {
	Kind  string `json:"kind"`
	Id    string `json:"id"`
	As    string `json:"as"`
	About string `json:"about"`
	On    bool   `json:"on"`
}

// ActionFocusAutoAdd is the contract type of the same name.
type ActionFocusAutoAdd struct {
	Kind string `json:"kind"`
	Id   string `json:"id"`
	As   string `json:"as"`
	On   bool   `json:"on"`
}

// ActionFocusOnly is the contract type of the same name.
type ActionFocusOnly struct {
	Kind string `json:"kind"`
	Id   string `json:"id"`
	As   string `json:"as"`
	On   bool   `json:"on"`
}

// ActionReleaseSetting is the contract type of the same name.
type ActionReleaseSetting struct {
	Kind string `json:"kind"`
	Id   string `json:"id"`
	As   string `json:"as"`
	On   bool   `json:"on"`
}

// ActionPreference is the contract type of the same name.
type ActionPreference struct {
	Kind  string `json:"kind"`
	Id    string `json:"id"`
	As    string `json:"as"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ActionProfileLine is the contract type of the same name.
type ActionProfileLine struct {
	Kind string `json:"kind"`
	Id   string `json:"id"`
	As   string `json:"as"`
	Text string `json:"text"`
}

// PeopleAction is any one of its member types.
type PeopleAction struct{ Value any }

func (u PeopleAction) MarshalJSON() ([]byte, error) { return json.Marshal(u.Value) }

func (u *PeopleAction) UnmarshalJSON(b []byte) error {
	var k struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(b, &k); err != nil {
		return err
	}
	switch k.Kind {
	case "send":
		v, err := decode[ActionSend](b)
		u.Value = v
		return err
	case "edit":
		v, err := decode[ActionEdit](b)
		u.Value = v
		return err
	case "delete":
		v, err := decode[ActionDelete](b)
		u.Value = v
		return err
	case "create_channel":
		v, err := decode[ActionCreateChannel](b)
		u.Value = v
		return err
	case "add_people":
		v, err := decode[ActionAddPeople](b)
		u.Value = v
		return err
	case "open_direct":
		v, err := decode[ActionOpenDirect](b)
		u.Value = v
		return err
	case "react":
		v, err := decode[ActionReact](b)
		u.Value = v
		return err
	case "read":
		v, err := decode[ActionRead](b)
		u.Value = v
		return err
	case "keep":
		v, err := decode[ActionKeep](b)
		u.Value = v
		return err
	case "done":
		v, err := decode[ActionDone](b)
		u.Value = v
		return err
	case "dial":
		v, err := decode[ActionDial](b)
		u.Value = v
		return err
	case "focus_rule":
		v, err := decode[ActionFocusRule](b)
		u.Value = v
		return err
	case "focus_person":
		v, err := decode[ActionFocusPerson](b)
		u.Value = v
		return err
	case "focus_auto_add":
		v, err := decode[ActionFocusAutoAdd](b)
		u.Value = v
		return err
	case "focus_only":
		v, err := decode[ActionFocusOnly](b)
		u.Value = v
		return err
	case "release_setting":
		v, err := decode[ActionReleaseSetting](b)
		u.Value = v
		return err
	case "preference":
		v, err := decode[ActionPreference](b)
		u.Value = v
		return err
	case "profile_line":
		v, err := decode[ActionProfileLine](b)
		u.Value = v
		return err
	}
	return fmt.Errorf("PeopleAction: unknown kind %q", k.Kind)
}

// ActionSaved is the contract type of the same name.
type ActionSaved struct {
	Saved        bool    `json:"saved"`
	Number       *int64  `json:"number"`
	Conversation *string `json:"conversation,omitempty"`
}

// Refusal is the contract type of the same name.
type Refusal struct {
	Refused string `json:"refused"`
}

// StartAnswer is the contract type of the same name.
type StartAnswer struct {
	Org           string                `json:"org"`
	OrgName       string                `json:"org_name"`
	Person        PersonView            `json:"person"`
	People        []PersonView          `json:"people"`
	Preferences   []Preference          `json:"preferences"`
	Conversations []ConversationSummary `json:"conversations"`
	Number        int64                 `json:"number"`
}

// ReplyInProgress is the contract type of the same name.
type ReplyInProgress struct {
	Message      string  `json:"message"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	Author       string  `json:"author"`
	At           int64   `json:"at"`
	Text         string  `json:"text"`
}

// MessagesAnswer is the contract type of the same name.
type MessagesAnswer struct {
	Number     int64             `json:"number"`
	Messages   []MessageView     `json:"messages"`
	InProgress []ReplyInProgress `json:"in_progress"`
	ReadUpTo   *int64            `json:"read_up_to"`
	HasBefore  bool              `json:"has_before"`
	HasAfter   bool              `json:"has_after"`
}

// RowActDial is the contract type of the same name.
type RowActDial struct {
	Kind         string    `json:"kind"`
	Conversation string    `json:"conversation"`
	Thread       *string   `json:"thread"`
	Level        DialLevel `json:"level"`
}

// RowActFocusPerson is the contract type of the same name.
type RowActFocusPerson struct {
	Kind  string `json:"kind"`
	About string `json:"about"`
	On    bool   `json:"on"`
}

// RowActFocusRule is the contract type of the same name.
type RowActFocusRule struct {
	Kind string    `json:"kind"`
	Rule FocusRule `json:"rule"`
	On   bool      `json:"on"`
}

// RowActDone is the contract type of the same name.
type RowActDone struct {
	Kind         string  `json:"kind"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
}

// RowActKeep is the contract type of the same name.
type RowActKeep struct {
	Kind         string  `json:"kind"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
}

// RowAct is any one of its member types.
type RowAct struct{ Value any }

func (u RowAct) MarshalJSON() ([]byte, error) { return json.Marshal(u.Value) }

func (u *RowAct) UnmarshalJSON(b []byte) error {
	var k struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(b, &k); err != nil {
		return err
	}
	switch k.Kind {
	case "dial":
		v, err := decode[RowActDial](b)
		u.Value = v
		return err
	case "focus_person":
		v, err := decode[RowActFocusPerson](b)
		u.Value = v
		return err
	case "focus_rule":
		v, err := decode[RowActFocusRule](b)
		u.Value = v
		return err
	case "done":
		v, err := decode[RowActDone](b)
		u.Value = v
		return err
	case "keep":
		v, err := decode[RowActKeep](b)
		u.Value = v
		return err
	}
	return fmt.Errorf("RowAct: unknown kind %q", k.Kind)
}

// RowChange is the contract type of the same name.
type RowChange struct {
	Label          string `json:"label"`
	Do             RowAct `json:"do"`
	Undo           RowAct `json:"undo"`
	MovedLast7Days *int64 `json:"moved_last_7_days"`
}

// RowExplanation is the contract type of the same name.
type RowExplanation struct {
	What      string     `json:"what"`
	DecidedBy string     `json:"decided_by"`
	Otherwise string     `json:"otherwise"`
	Change    *RowChange `json:"change"`
}

// FeedRow is the contract type of the same name.
type FeedRow struct {
	Conversation string         `json:"conversation"`
	Thread       *string        `json:"thread"`
	Message      string         `json:"message"`
	Bucket       Bucket         `json:"bucket"`
	Unread       bool           `json:"unread"`
	Kept         *KeptBy        `json:"kept"`
	Why          string         `json:"why"`
	Newest       int64          `json:"newest"`
	FirstNew     *int64         `json:"first_new"`
	Author       string         `json:"author"`
	Preview      string         `json:"preview"`
	At           string         `json:"at"`
	NewItems     int64          `json:"new_items"`
	Reactions    []Reaction     `json:"reactions"`
	State        RowState       `json:"state"`
	Fenced       bool           `json:"fenced"`
	Explanation  RowExplanation `json:"explanation"`
}

// FeedCounts is the contract type of the same name.
type FeedCounts struct {
	Focus    int64 `json:"focus"`
	NeedsYou int64 `json:"needs_you"`
	Fyi      bool  `json:"fyi"`
	Kept     int64 `json:"kept"`
}

// FeedRowsAnswer is the contract type of the same name.
type FeedRowsAnswer struct {
	Edition int64      `json:"edition"`
	Through int64      `json:"through"`
	Number  int64      `json:"number"`
	Rows    []FeedRow  `json:"rows"`
	Counts  FeedCounts `json:"counts"`
}

// ScreenFeedAnswer is the contract type of the same name.
type ScreenFeedAnswer struct {
	Edition *int64      `json:"edition"`
	Through *int64      `json:"through"`
	Number  *int64      `json:"number"`
	Rows    []FeedRow   `json:"rows"`
	Counts  *FeedCounts `json:"counts"`
	Notice  *string     `json:"notice"`
}

// HistoryEntry is the contract type of the same name.
type HistoryEntry struct {
	Message      string     `json:"message"`
	Conversation string     `json:"conversation"`
	Thread       *string    `json:"thread"`
	Author       string     `json:"author"`
	Preview      string     `json:"preview"`
	At           string     `json:"at"`
	Number       int64      `json:"number"`
	Unread       bool       `json:"unread"`
	By           *string    `json:"by"`
	Emoji        *string    `json:"emoji"`
	Level        *DialLevel `json:"level"`
	Kept         *KeptBy    `json:"kept"`
}

// HistoryAnswer is the contract type of the same name.
type HistoryAnswer struct {
	Tab     HistoryTab     `json:"tab"`
	Entries []HistoryEntry `json:"entries"`
	Unread  int64          `json:"unread"`
	Next    *string        `json:"next"`
}

// FocusPerson is the contract type of the same name.
type FocusPerson struct {
	Person         string `json:"person"`
	ItemsLast7Days int64  `json:"items_last_7_days"`
}

// LevelEntry is the contract type of the same name.
type LevelEntry struct {
	Conversation string    `json:"conversation"`
	Thread       *string   `json:"thread"`
	Level        DialLevel `json:"level"`
}

// SettingsAnswer is the contract type of the same name.
type SettingsAnswer struct {
	FocusPeople   []FocusPerson `json:"focus_people"`
	RemovedPeople []string      `json:"removed_people"`
	AutoAdd       bool          `json:"auto_add"`
	FocusOnly     bool          `json:"focus_only"`
	ReleaseOnAct  bool          `json:"release_on_act"`
	Rules         []FocusRule   `json:"rules"`
	RemovedRules  []FocusRule   `json:"removed_rules"`
	Levels        []LevelEntry  `json:"levels"`
}

// PreviewCounts is the contract type of the same name.
type PreviewCounts struct {
	Moved int64   `json:"moved"`
	From  *Bucket `json:"from"`
	To    *Bucket `json:"to"`
	Stay  int64   `json:"stay"`
}

// PreviewAnswer is the contract type of the same name.
type PreviewAnswer struct {
	Last7Days PreviewCounts `json:"last_7_days"`
}

// NotAvailable is the contract type of the same name.
type NotAvailable struct {
	NotAvailable string `json:"not_available"`
}

// SuggestAnswer is the contract type of the same name.
type SuggestAnswer struct {
	People   []PersonView `json:"people"`
	Everyone bool         `json:"everyone"`
}

// SearchResult is the contract type of the same name.
type SearchResult struct {
	Message      string  `json:"message"`
	Number       int64   `json:"number"`
	CreatedAt    string  `json:"created_at"`
	Version      int64   `json:"version"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	Author       string  `json:"author"`
	Edited       bool    `json:"edited"`
	Text         string  `json:"text"`
	Match        []Span  `json:"match"`
}

// SearchAnswer is the contract type of the same name.
type SearchAnswer struct {
	Through int64          `json:"through"`
	Total   int64          `json:"total"`
	Next    *string        `json:"next"`
	Results []SearchResult `json:"results"`
}

// ScreenSearchAnswer is the contract type of the same name.
type ScreenSearchAnswer struct {
	Answer *SearchAnswer `json:"answer"`
	Notice *string       `json:"notice"`
}

// ReplyText is the contract type of the same name.
type ReplyText struct {
	Message      string  `json:"message"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	Author       string  `json:"author"`
	At           int64   `json:"at"`
	Text         string  `json:"text"`
}

// ResumeNumbers is the contract type of the same name.
type ResumeNumbers struct {
	Conversations int64  `json:"conversations"`
	Feed          *int64 `json:"feed"`
	FeedEdition   *int64 `json:"feed_edition"`
}

// LiveResume is the contract type of the same name.
type LiveResume struct {
	Resume ResumeNumbers `json:"resume"`
}

// WatchingNote is the contract type of the same name.
type WatchingNote struct {
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	Open         bool    `json:"open"`
}

// LiveWatching is the contract type of the same name.
type LiveWatching struct {
	Watching WatchingNote `json:"watching"`
}

// LiveIn is any one of its member types.
type LiveIn struct{ Value any }

func (u LiveIn) MarshalJSON() ([]byte, error) { return json.Marshal(u.Value) }

func (u *LiveIn) UnmarshalJSON(b []byte) error {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(b, &keys); err != nil {
		return err
	}
	if _, ok := keys["resume"]; ok {
		v, err := decode[LiveResume](b)
		u.Value = v
		return err
	}
	if _, ok := keys["watching"]; ok {
		v, err := decode[LiveWatching](b)
		u.Value = v
		return err
	}
	return fmt.Errorf("LiveIn: unknown message")
}

// PartMessageCreated is the contract type of the same name.
type PartMessageCreated struct {
	Kind    string  `json:"kind"`
	Message Message `json:"message"`
}

// PartMessageChanged is the contract type of the same name.
type PartMessageChanged struct {
	Kind    string  `json:"kind"`
	Message Message `json:"message"`
}

// PartMessageDeleted is the contract type of the same name.
type PartMessageDeleted struct {
	Kind    string         `json:"kind"`
	Message DeletedMessage `json:"message"`
}

// PartReactionChanged is the contract type of the same name.
type PartReactionChanged struct {
	Kind         string     `json:"kind"`
	Message      string     `json:"message"`
	Conversation string     `json:"conversation"`
	Thread       *string    `json:"thread"`
	Reactions    []Reaction `json:"reactions"`
}

// PartThreadSummary is the contract type of the same name.
type PartThreadSummary struct {
	Kind         string   `json:"kind"`
	Conversation string   `json:"conversation"`
	Thread       string   `json:"thread"`
	Replies      int64    `json:"replies"`
	LastReplyAt  *string  `json:"last_reply_at"`
	People       []string `json:"people"`
}

// PartUnread is the contract type of the same name.
type PartUnread struct {
	Kind         string  `json:"kind"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	Unread       bool    `json:"unread"`
}

// PartReadMoved is the contract type of the same name.
type PartReadMoved struct {
	Kind         string  `json:"kind"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	UpTo         int64   `json:"up_to"`
}

// PartReplyStarted is the contract type of the same name.
type PartReplyStarted struct {
	Kind         string  `json:"kind"`
	Message      string  `json:"message"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	Author       string  `json:"author"`
}

// PartReplyAbandoned is the contract type of the same name.
type PartReplyAbandoned struct {
	Kind         string  `json:"kind"`
	Message      string  `json:"message"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
}

// PartConversationAdded is the contract type of the same name.
type PartConversationAdded struct {
	Kind                string              `json:"kind"`
	ConversationSummary ConversationSummary `json:"conversation_summary"`
}

// PartPeopleAdded is the contract type of the same name.
type PartPeopleAdded struct {
	Kind         string   `json:"kind"`
	Conversation string   `json:"conversation"`
	People       []string `json:"people"`
}

// PartPersonAdded is the contract type of the same name.
type PartPersonAdded struct {
	Kind       string     `json:"kind"`
	PersonView PersonView `json:"person_view"`
}

// PartPreference is the contract type of the same name.
type PartPreference struct {
	Kind  string `json:"kind"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PartProfileLine is the contract type of the same name.
type PartProfileLine struct {
	Kind   string `json:"kind"`
	Person string `json:"person"`
	Text   string `json:"text"`
}

// ConversationPart is any one of its member types.
type ConversationPart struct{ Value any }

func (u ConversationPart) MarshalJSON() ([]byte, error) { return json.Marshal(u.Value) }

func (u *ConversationPart) UnmarshalJSON(b []byte) error {
	var k struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(b, &k); err != nil {
		return err
	}
	switch k.Kind {
	case "message.created":
		v, err := decode[PartMessageCreated](b)
		u.Value = v
		return err
	case "message.changed":
		v, err := decode[PartMessageChanged](b)
		u.Value = v
		return err
	case "message.deleted":
		v, err := decode[PartMessageDeleted](b)
		u.Value = v
		return err
	case "reaction.changed":
		v, err := decode[PartReactionChanged](b)
		u.Value = v
		return err
	case "thread.summary":
		v, err := decode[PartThreadSummary](b)
		u.Value = v
		return err
	case "unread":
		v, err := decode[PartUnread](b)
		u.Value = v
		return err
	case "read.moved":
		v, err := decode[PartReadMoved](b)
		u.Value = v
		return err
	case "reply.started":
		v, err := decode[PartReplyStarted](b)
		u.Value = v
		return err
	case "reply.abandoned":
		v, err := decode[PartReplyAbandoned](b)
		u.Value = v
		return err
	case "conversation.added":
		v, err := decode[PartConversationAdded](b)
		u.Value = v
		return err
	case "people.added":
		v, err := decode[PartPeopleAdded](b)
		u.Value = v
		return err
	case "person.added":
		v, err := decode[PartPersonAdded](b)
		u.Value = v
		return err
	case "preference":
		v, err := decode[PartPreference](b)
		u.Value = v
		return err
	case "profile_line":
		v, err := decode[PartProfileLine](b)
		u.Value = v
		return err
	}
	return fmt.Errorf("ConversationPart: unknown kind %q", k.Kind)
}

// FeedPartRowChanged is the contract type of the same name.
type FeedPartRowChanged struct {
	Kind string  `json:"kind"`
	Row  FeedRow `json:"row"`
}

// FeedPartRowRemoved is the contract type of the same name.
type FeedPartRowRemoved struct {
	Kind string `json:"kind"`
	Row  RowKey `json:"row"`
}

// FeedPartCounts is the contract type of the same name.
type FeedPartCounts struct {
	Kind   string     `json:"kind"`
	Counts FeedCounts `json:"counts"`
}

// FeedPartHistoryChanged is the contract type of the same name.
type FeedPartHistoryChanged struct {
	Kind   string     `json:"kind"`
	Tab    HistoryTab `json:"tab"`
	Unread int64      `json:"unread"`
}

// FeedPartSettingsChanged is the contract type of the same name.
type FeedPartSettingsChanged struct {
	Kind string `json:"kind"`
}

// FeedPart is any one of its member types.
type FeedPart struct{ Value any }

func (u FeedPart) MarshalJSON() ([]byte, error) { return json.Marshal(u.Value) }

func (u *FeedPart) UnmarshalJSON(b []byte) error {
	var k struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(b, &k); err != nil {
		return err
	}
	switch k.Kind {
	case "row.changed":
		v, err := decode[FeedPartRowChanged](b)
		u.Value = v
		return err
	case "row.removed":
		v, err := decode[FeedPartRowRemoved](b)
		u.Value = v
		return err
	case "counts":
		v, err := decode[FeedPartCounts](b)
		u.Value = v
		return err
	case "history.changed":
		v, err := decode[FeedPartHistoryChanged](b)
		u.Value = v
		return err
	case "settings.changed":
		v, err := decode[FeedPartSettingsChanged](b)
		u.Value = v
		return err
	}
	return fmt.Errorf("FeedPart: unknown kind %q", k.Kind)
}

// LiveConversationsUpdate is the contract type of the same name.
type LiveConversationsUpdate struct {
	Stream string             `json:"stream"`
	Number int64              `json:"number"`
	After  int64              `json:"after"`
	Parts  []ConversationPart `json:"parts"`
}

// LiveFeedUpdate is the contract type of the same name.
type LiveFeedUpdate struct {
	Stream  string     `json:"stream"`
	Number  int64      `json:"number"`
	After   int64      `json:"after"`
	Edition int64      `json:"edition"`
	Parts   []FeedPart `json:"parts"`
}

// LastSentNumbers is the contract type of the same name.
type LastSentNumbers struct {
	Conversations int64  `json:"conversations"`
	Feed          *int64 `json:"feed"`
	FeedEdition   *int64 `json:"feed_edition"`
}

// LiveLastSent is the contract type of the same name.
type LiveLastSent struct {
	LastSent LastSentNumbers `json:"last_sent"`
}

// LiveTooFarBehind is the contract type of the same name.
type LiveTooFarBehind struct {
	TooFarBehind Stream `json:"too_far_behind"`
}

// PieceBody is the contract type of the same name.
type PieceBody struct {
	Message string `json:"message"`
	At      int64  `json:"at"`
	Text    string `json:"text"`
}

// LivePiece is the contract type of the same name.
type LivePiece struct {
	Piece PieceBody `json:"piece"`
}

// AgentStatusBody is the contract type of the same name.
type AgentStatusBody struct {
	Agent string     `json:"agent"`
	State AgentState `json:"state"`
	On    *string    `json:"on"`
}

// LiveAgentStatus is the contract type of the same name.
type LiveAgentStatus struct {
	AgentStatus AgentStatusBody `json:"agent_status"`
}

// LiveOut is any one of its member types.
type LiveOut struct{ Value any }

func (u LiveOut) MarshalJSON() ([]byte, error) { return json.Marshal(u.Value) }

func (u *LiveOut) UnmarshalJSON(b []byte) error {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(b, &keys); err != nil {
		return err
	}
	if _, ok := keys["edition"]; ok {
		v, err := decode[LiveFeedUpdate](b)
		u.Value = v
		return err
	}
	if _, ok := keys["stream"]; ok {
		v, err := decode[LiveConversationsUpdate](b)
		u.Value = v
		return err
	}
	if _, ok := keys["last_sent"]; ok {
		v, err := decode[LiveLastSent](b)
		u.Value = v
		return err
	}
	if _, ok := keys["too_far_behind"]; ok {
		v, err := decode[LiveTooFarBehind](b)
		u.Value = v
		return err
	}
	if _, ok := keys["organization_gone"]; ok {
		v, err := decode[OrganizationGone](b)
		u.Value = v
		return err
	}
	if _, ok := keys["piece"]; ok {
		v, err := decode[LivePiece](b)
		u.Value = v
		return err
	}
	if _, ok := keys["agent_status"]; ok {
		v, err := decode[LiveAgentStatus](b)
		u.Value = v
		return err
	}
	return fmt.Errorf("LiveOut: unknown message")
}

// RowUpdate is the contract type of the same name.
type RowUpdate struct {
	Number int64      `json:"number"`
	Person string     `json:"person"`
	After  int64      `json:"after"`
	Parts  []FeedPart `json:"parts"`
}

// RowUpdatesPage is the contract type of the same name.
type RowUpdatesPage struct {
	Updates []RowUpdate `json:"updates"`
	Next    *int64      `json:"next"`
}

// NewEdition is the contract type of the same name.
type NewEdition struct {
	NewEdition int64 `json:"new_edition"`
}

// SearchMessage is the contract type of the same name.
type SearchMessage struct {
	Id           string   `json:"id"`
	Number       int64    `json:"number"`
	CreatedAt    string   `json:"created_at"`
	Version      int64    `json:"version"`
	Conversation string   `json:"conversation"`
	Thread       *string  `json:"thread"`
	Edited       bool     `json:"edited"`
	Participants []string `json:"participants"`
	Author       string   `json:"author"`
	Text         string   `json:"text"`
	Mentions     []string `json:"mentions"`
	Level        *Level   `json:"level"`
}

// SearchDeleted is the contract type of the same name.
type SearchDeleted struct {
	Id      string `json:"id"`
	Version int64  `json:"version"`
}

// SearchCreated is the contract type of the same name.
type SearchCreated struct {
	Kind    string        `json:"kind"`
	Org     string        `json:"org"`
	Number  int64         `json:"number"`
	Message SearchMessage `json:"message"`
}

// SearchChanged is the contract type of the same name.
type SearchChanged struct {
	Kind    string        `json:"kind"`
	Org     string        `json:"org"`
	Number  int64         `json:"number"`
	Message SearchMessage `json:"message"`
}

// SearchDeletedChange is the contract type of the same name.
type SearchDeletedChange struct {
	Kind    string        `json:"kind"`
	Org     string        `json:"org"`
	Number  int64         `json:"number"`
	Message SearchDeleted `json:"message"`
}

// SearchChange is any one of its member types.
type SearchChange struct{ Value any }

func (u SearchChange) MarshalJSON() ([]byte, error) { return json.Marshal(u.Value) }

func (u *SearchChange) UnmarshalJSON(b []byte) error {
	var k struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(b, &k); err != nil {
		return err
	}
	switch k.Kind {
	case "message.created":
		v, err := decode[SearchCreated](b)
		u.Value = v
		return err
	case "message.changed":
		v, err := decode[SearchChanged](b)
		u.Value = v
		return err
	case "message.deleted":
		v, err := decode[SearchDeletedChange](b)
		u.Value = v
		return err
	}
	return fmt.Errorf("SearchChange: unknown kind %q", k.Kind)
}

// SearchApplied is the contract type of the same name.
type SearchApplied struct {
	Applied bool  `json:"applied"`
	Already *bool `json:"already,omitempty"`
	Wipe    int64 `json:"wipe"`
}

// SearchRefusal is the contract type of the same name.
type SearchRefusal struct {
	Refused string `json:"refused"`
	Wipe    int64  `json:"wipe"`
}

// NoteSend is the contract type of the same name.
type NoteSend struct {
	Kind     string `json:"kind"`
	Org      string `json:"org"`
	Number   int64  `json:"number"`
	Previous int64  `json:"previous"`
	Edition  int64  `json:"edition"`
	At       string `json:"at"`
	Name     string `json:"name"`
	Row      RowKey `json:"row"`
	Person   string `json:"person"`
	Bucket   Bucket `json:"bucket"`
	Why      string `json:"why"`
	Text     string `json:"text"`
	DueAt    string `json:"due_at"`
	History  *bool  `json:"history,omitempty"`
	Skip     *bool  `json:"skip,omitempty"`
}

// NoteUpdate is the contract type of the same name.
type NoteUpdate struct {
	Kind     string   `json:"kind"`
	Org      string   `json:"org"`
	Number   int64    `json:"number"`
	Previous int64    `json:"previous"`
	Edition  int64    `json:"edition"`
	At       string   `json:"at"`
	Name     string   `json:"name"`
	Text     *string  `json:"text,omitempty"`
	Now      *NoteNow `json:"now,omitempty"`
	Skip     *bool    `json:"skip,omitempty"`
}

// Note is any one of its member types.
type Note struct{ Value any }

func (u Note) MarshalJSON() ([]byte, error) { return json.Marshal(u.Value) }

func (u *Note) UnmarshalJSON(b []byte) error {
	var k struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(b, &k); err != nil {
		return err
	}
	switch k.Kind {
	case "send":
		v, err := decode[NoteSend](b)
		u.Value = v
		return err
	case "update":
		v, err := decode[NoteUpdate](b)
		u.Value = v
		return err
	}
	return fmt.Errorf("Note: unknown kind %q", k.Kind)
}

// NoteTaken is the contract type of the same name.
type NoteTaken struct {
	Taken   bool  `json:"taken"`
	Already *bool `json:"already,omitempty"`
}

// NoteBehind is the contract type of the same name.
type NoteBehind struct {
	IAmAt   int64 `json:"i_am_at"`
	Edition int64 `json:"edition"`
}

// NotificationEntry is the contract type of the same name.
type NotificationEntry struct {
	Name    string    `json:"name"`
	Person  string    `json:"person"`
	State   NoteState `json:"state"`
	At      *string   `json:"at"`
	Delayed bool      `json:"delayed"`
	Why     string    `json:"why"`
	Text    string    `json:"text"`
	Now     *NoteNow  `json:"now"`
}

// AgentEventMentioned is the contract type of the same name.
type AgentEventMentioned struct {
	Kind       string  `json:"kind"`
	Org        string  `json:"org"`
	Number     int64   `json:"number"`
	AgentChain int64   `json:"agent_chain"`
	Message    Message `json:"message"`
}

// AgentEventDirectMessage is the contract type of the same name.
type AgentEventDirectMessage struct {
	Kind       string  `json:"kind"`
	Org        string  `json:"org"`
	Number     int64   `json:"number"`
	AgentChain int64   `json:"agent_chain"`
	Message    Message `json:"message"`
}

// AgentEventReply is the contract type of the same name.
type AgentEventReply struct {
	Kind       string  `json:"kind"`
	Org        string  `json:"org"`
	Number     int64   `json:"number"`
	AgentChain int64   `json:"agent_chain"`
	Message    Message `json:"message"`
}

// AgentEventChanged is the contract type of the same name.
type AgentEventChanged struct {
	Kind       string  `json:"kind"`
	Org        string  `json:"org"`
	Number     int64   `json:"number"`
	AgentChain int64   `json:"agent_chain"`
	Message    Message `json:"message"`
}

// AgentEventDeleted is the contract type of the same name.
type AgentEventDeleted struct {
	Kind       string  `json:"kind"`
	Org        string  `json:"org"`
	Number     int64   `json:"number"`
	AgentChain int64   `json:"agent_chain"`
	Message    Message `json:"message"`
}

// AgentEventReaction is the contract type of the same name.
type AgentEventReaction struct {
	Kind       string  `json:"kind"`
	Org        string  `json:"org"`
	Number     int64   `json:"number"`
	AgentChain int64   `json:"agent_chain"`
	Message    Message `json:"message"`
	By         string  `json:"by"`
	Emoji      string  `json:"emoji"`
	Added      bool    `json:"added"`
}

// AgentEvent is any one of its member types.
type AgentEvent struct{ Value any }

func (u AgentEvent) MarshalJSON() ([]byte, error) { return json.Marshal(u.Value) }

func (u *AgentEvent) UnmarshalJSON(b []byte) error {
	var k struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(b, &k); err != nil {
		return err
	}
	switch k.Kind {
	case "mentioned":
		v, err := decode[AgentEventMentioned](b)
		u.Value = v
		return err
	case "direct_message":
		v, err := decode[AgentEventDirectMessage](b)
		u.Value = v
		return err
	case "reply":
		v, err := decode[AgentEventReply](b)
		u.Value = v
		return err
	case "reaction":
		v, err := decode[AgentEventReaction](b)
		u.Value = v
		return err
	case "changed":
		v, err := decode[AgentEventChanged](b)
		u.Value = v
		return err
	case "deleted":
		v, err := decode[AgentEventDeleted](b)
		u.Value = v
		return err
	}
	return fmt.Errorf("AgentEvent: unknown kind %q", k.Kind)
}

// AgentEvents is the contract type of the same name.
type AgentEvents struct {
	ContinueFrom *int64       `json:"continue_from"`
	Events       []AgentEvent `json:"events"`
}

// TooFarBehind is the contract type of the same name.
type TooFarBehind struct {
	TooFarBehind bool  `json:"too_far_behind"`
	Oldest       int64 `json:"oldest"`
}

// AgentPost is the contract type of the same name.
type AgentPost struct {
	Id           string   `json:"id"`
	Conversation string   `json:"conversation"`
	Thread       *string  `json:"thread"`
	Text         string   `json:"text"`
	Level        *Level   `json:"level"`
	BecauseOf    *int64   `json:"because_of"`
	Mentions     []string `json:"mentions,omitempty"`
}

// AgentPosted is the contract type of the same name.
type AgentPosted struct {
	Message string `json:"message"`
	Number  int64  `json:"number"`
}

// AgentReplyStart is the contract type of the same name.
type AgentReplyStart struct {
	Id           string  `json:"id"`
	Conversation string  `json:"conversation"`
	Thread       *string `json:"thread"`
	BecauseOf    *int64  `json:"because_of"`
}

// AgentPiece is the contract type of the same name.
type AgentPiece struct {
	At   int64  `json:"at"`
	Text string `json:"text"`
}

// AgentReplyFinish is the contract type of the same name.
type AgentReplyFinish struct {
	Id       string   `json:"id"`
	Level    *Level   `json:"level"`
	Mentions []string `json:"mentions"`
}

// PieceMissing is the contract type of the same name.
type PieceMissing struct {
	Have int64 `json:"have"`
}

// AgentStatus is the contract type of the same name.
type AgentStatus struct {
	State AgentState `json:"state"`
	On    *string    `json:"on"`
}

// AgentReaction is the contract type of the same name.
type AgentReaction struct {
	Id      string `json:"id"`
	Message string `json:"message"`
	Emoji   string `json:"emoji"`
	On      bool   `json:"on"`
}

// AgentDirect is the contract type of the same name.
type AgentDirect struct {
	With string `json:"with"`
}

// AgentDirectAnswer is the contract type of the same name.
type AgentDirectAnswer struct {
	Conversation string `json:"conversation"`
}

// FeedPlace is the contract type of the same name.
type FeedPlace struct {
	Through int64 `json:"through"`
	Edition int64 `json:"edition"`
}

// SearchPlace is the contract type of the same name.
type SearchPlace struct {
	Through int64 `json:"through"`
	Wipe    int64 `json:"wipe"`
}

// SenderPlace is the contract type of the same name.
type SenderPlace struct {
	Through int64 `json:"through"`
	Edition int64 `json:"edition"`
}

// ReceiverStatus is the contract type of the same name.
type ReceiverStatus struct {
	Name    ReceiverName  `json:"name"`
	Place   int64         `json:"place"`
	Waiting int64         `json:"waiting"`
	State   ReceiverState `json:"state"`
	Parked  int64         `json:"parked"`
}

// DeliveryStatus is the contract type of the same name.
type DeliveryStatus struct {
	Receivers []ReceiverStatus `json:"receivers"`
}

// ParkedChange is the contract type of the same name.
type ParkedChange struct {
	Receiver ReceiverName `json:"receiver"`
	Number   int64        `json:"number"`
	Reason   string       `json:"reason"`
	Tries    int64        `json:"tries"`
}

// ParkedList is the contract type of the same name.
type ParkedList struct {
	Parked []ParkedChange `json:"parked"`
}

// ModeBody is the contract type of the same name.
type ModeBody struct {
	Mode Mode `json:"mode"`
}

// RebuildRequest is the contract type of the same name.
type RebuildRequest struct {
	Receiver     ReceiverName `json:"receiver"`
	RulesChanged bool         `json:"rules_changed"`
}

// RestartRequest is the contract type of the same name.
type RestartRequest struct {
	Edition *int64 `json:"edition"`
}

// NotificationList is the contract type of the same name.
type NotificationList struct {
	Notifications []NotificationEntry `json:"notifications"`
}

// PeopleList is the contract type of the same name.
type PeopleList struct {
	People []PersonView `json:"people"`
}

// ScriptStep is the contract type of the same name.
type ScriptStep struct {
	At        string  `json:"at"`
	Who       string  `json:"who"`
	What      string  `json:"what"`
	Situation *string `json:"situation"`
}

// Situation is the contract type of the same name.
type Situation struct {
	Id      string `json:"id"`
	Title   string `json:"title"`
	Outcome string `json:"outcome"`
}

// ScriptState is the contract type of the same name.
type ScriptState struct {
	Running    bool         `json:"running"`
	Pace       float64      `json:"pace"`
	Happened   []ScriptStep `json:"happened"`
	Next       []ScriptStep `json:"next"`
	Situations []Situation  `json:"situations"`
}

// ScriptRequest is the contract type of the same name.
type ScriptRequest struct {
	Do        ScriptCommand `json:"do"`
	Pace      *float64      `json:"pace,omitempty"`
	Situation *string       `json:"situation,omitempty"`
}

// SeedOpened is the contract type of the same name.
type SeedOpened struct {
	Session string `json:"session"`
}

// SeedOpenRequest is the contract type of the same name.
type SeedOpenRequest struct {
	Name *string `json:"name,omitempty"`
}

// CurrentOrganization is the contract type of the same name.
type CurrentOrganization struct {
	Org  string `json:"org"`
	Name string `json:"name"`
}

// OrganizationEntry is the contract type of the same name.
type OrganizationEntry struct {
	Org           string `json:"org"`
	Name          string `json:"name"`
	State         string `json:"state"`
	CreatedAt     string `json:"created_at"`
	ApproxChanges int64  `json:"approx_changes"`
}

// OrganizationList is the contract type of the same name.
type OrganizationList struct {
	Organizations []OrganizationEntry `json:"organizations"`
}

// SeedBatch is the contract type of the same name.
type SeedBatch struct {
	Changes []Change `json:"changes"`
}

// SeedThrough is the contract type of the same name.
type SeedThrough struct {
	Through int64 `json:"through"`
}

// WipeRequest is the contract type of the same name.
type WipeRequest struct {
	RulesChanged *bool `json:"rules_changed,omitempty"`
}

// Empty is the contract type of the same name.
type Empty struct {
}

func decode[T any](b []byte) (T, error) {
	var v T
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	err := d.Decode(&v)
	return v, err
}

// Types lists a decoder for every concrete type, by its schema name.
var Types = map[string]func([]byte) (any, error){
	"RowKey":                      func(b []byte) (any, error) { return decode[RowKey](b) },
	"Reaction":                    func(b []byte) (any, error) { return decode[Reaction](b) },
	"FocusRule":                   func(b []byte) (any, error) { return decode[FocusRule](b) },
	"Message":                     func(b []byte) (any, error) { return decode[Message](b) },
	"DeletedMessage":              func(b []byte) (any, error) { return decode[DeletedMessage](b) },
	"ThreadSummary":               func(b []byte) (any, error) { return decode[ThreadSummary](b) },
	"MessageView":                 func(b []byte) (any, error) { return decode[MessageView](b) },
	"PersonStatus":                func(b []byte) (any, error) { return decode[PersonStatus](b) },
	"PersonView":                  func(b []byte) (any, error) { return decode[PersonView](b) },
	"Preference":                  func(b []byte) (any, error) { return decode[Preference](b) },
	"ConversationSummary":         func(b []byte) (any, error) { return decode[ConversationSummary](b) },
	"ChangeMessageCreated":        func(b []byte) (any, error) { return decode[ChangeMessageCreated](b) },
	"ChangeMessageChanged":        func(b []byte) (any, error) { return decode[ChangeMessageChanged](b) },
	"ChangeMessageDeleted":        func(b []byte) (any, error) { return decode[ChangeMessageDeleted](b) },
	"ChangeReplyStarted":          func(b []byte) (any, error) { return decode[ChangeReplyStarted](b) },
	"ChangeReplyAbandoned":        func(b []byte) (any, error) { return decode[ChangeReplyAbandoned](b) },
	"ChangeReactionAdded":         func(b []byte) (any, error) { return decode[ChangeReactionAdded](b) },
	"ChangeReactionRemoved":       func(b []byte) (any, error) { return decode[ChangeReactionRemoved](b) },
	"ChangePersonJoined":          func(b []byte) (any, error) { return decode[ChangePersonJoined](b) },
	"ChangeChannelCreated":        func(b []byte) (any, error) { return decode[ChangeChannelCreated](b) },
	"ChangeChannelPeopleAdded":    func(b []byte) (any, error) { return decode[ChangeChannelPeopleAdded](b) },
	"ChangeDirectCreated":         func(b []byte) (any, error) { return decode[ChangeDirectCreated](b) },
	"ChangeReaderRead":            func(b []byte) (any, error) { return decode[ChangeReaderRead](b) },
	"ChangeReaderWatchingStarted": func(b []byte) (any, error) { return decode[ChangeReaderWatchingStarted](b) },
	"ChangeReaderWatchingEnded":   func(b []byte) (any, error) { return decode[ChangeReaderWatchingEnded](b) },
	"ChangeReaderKeep":            func(b []byte) (any, error) { return decode[ChangeReaderKeep](b) },
	"ChangeReaderDone":            func(b []byte) (any, error) { return decode[ChangeReaderDone](b) },
	"ChangeReaderDial":            func(b []byte) (any, error) { return decode[ChangeReaderDial](b) },
	"ChangeReaderFocusRule":       func(b []byte) (any, error) { return decode[ChangeReaderFocusRule](b) },
	"ChangeReaderFocusPerson":     func(b []byte) (any, error) { return decode[ChangeReaderFocusPerson](b) },
	"ChangeReaderFocusAutoAdd":    func(b []byte) (any, error) { return decode[ChangeReaderFocusAutoAdd](b) },
	"ChangeReaderFocusOnly":       func(b []byte) (any, error) { return decode[ChangeReaderFocusOnly](b) },
	"ChangeReaderReleaseSetting":  func(b []byte) (any, error) { return decode[ChangeReaderReleaseSetting](b) },
	"ChangeReaderPreference":      func(b []byte) (any, error) { return decode[ChangeReaderPreference](b) },
	"ChangeReaderProfileLine":     func(b []byte) (any, error) { return decode[ChangeReaderProfileLine](b) },
	"ChangeTick":                  func(b []byte) (any, error) { return decode[ChangeTick](b) },
	"ChangeSeedOpened":            func(b []byte) (any, error) { return decode[ChangeSeedOpened](b) },
	"ChangeSeedSealed":            func(b []byte) (any, error) { return decode[ChangeSeedSealed](b) },
	"SeedOpenRequest":             func(b []byte) (any, error) { return decode[SeedOpenRequest](b) },
	"CurrentOrganization":         func(b []byte) (any, error) { return decode[CurrentOrganization](b) },
	"OrganizationEntry":           func(b []byte) (any, error) { return decode[OrganizationEntry](b) },
	"OrganizationList":            func(b []byte) (any, error) { return decode[OrganizationList](b) },
	"ChangeBatch":                 func(b []byte) (any, error) { return decode[ChangeBatch](b) },
	"ChangeApplied":               func(b []byte) (any, error) { return decode[ChangeApplied](b) },
	"IAmAt":                       func(b []byte) (any, error) { return decode[IAmAt](b) },
	"OrganizationGone":            func(b []byte) (any, error) { return decode[OrganizationGone](b) },
	"OrganizationStopped":         func(b []byte) (any, error) { return decode[OrganizationStopped](b) },
	"NewMessage":                  func(b []byte) (any, error) { return decode[NewMessage](b) },
	"ActionSend":                  func(b []byte) (any, error) { return decode[ActionSend](b) },
	"ActionEdit":                  func(b []byte) (any, error) { return decode[ActionEdit](b) },
	"ActionDelete":                func(b []byte) (any, error) { return decode[ActionDelete](b) },
	"ActionCreateChannel":         func(b []byte) (any, error) { return decode[ActionCreateChannel](b) },
	"ActionAddPeople":             func(b []byte) (any, error) { return decode[ActionAddPeople](b) },
	"ActionOpenDirect":            func(b []byte) (any, error) { return decode[ActionOpenDirect](b) },
	"ActionReact":                 func(b []byte) (any, error) { return decode[ActionReact](b) },
	"ActionRead":                  func(b []byte) (any, error) { return decode[ActionRead](b) },
	"ActionKeep":                  func(b []byte) (any, error) { return decode[ActionKeep](b) },
	"ActionDone":                  func(b []byte) (any, error) { return decode[ActionDone](b) },
	"ActionDial":                  func(b []byte) (any, error) { return decode[ActionDial](b) },
	"ActionFocusRule":             func(b []byte) (any, error) { return decode[ActionFocusRule](b) },
	"ActionFocusPerson":           func(b []byte) (any, error) { return decode[ActionFocusPerson](b) },
	"ActionFocusAutoAdd":          func(b []byte) (any, error) { return decode[ActionFocusAutoAdd](b) },
	"ActionFocusOnly":             func(b []byte) (any, error) { return decode[ActionFocusOnly](b) },
	"ActionReleaseSetting":        func(b []byte) (any, error) { return decode[ActionReleaseSetting](b) },
	"ActionPreference":            func(b []byte) (any, error) { return decode[ActionPreference](b) },
	"ActionProfileLine":           func(b []byte) (any, error) { return decode[ActionProfileLine](b) },
	"ActionSaved":                 func(b []byte) (any, error) { return decode[ActionSaved](b) },
	"Refusal":                     func(b []byte) (any, error) { return decode[Refusal](b) },
	"StartAnswer":                 func(b []byte) (any, error) { return decode[StartAnswer](b) },
	"ReplyInProgress":             func(b []byte) (any, error) { return decode[ReplyInProgress](b) },
	"MessagesAnswer":              func(b []byte) (any, error) { return decode[MessagesAnswer](b) },
	"RowActDial":                  func(b []byte) (any, error) { return decode[RowActDial](b) },
	"RowActFocusPerson":           func(b []byte) (any, error) { return decode[RowActFocusPerson](b) },
	"RowActFocusRule":             func(b []byte) (any, error) { return decode[RowActFocusRule](b) },
	"RowActDone":                  func(b []byte) (any, error) { return decode[RowActDone](b) },
	"RowActKeep":                  func(b []byte) (any, error) { return decode[RowActKeep](b) },
	"RowChange":                   func(b []byte) (any, error) { return decode[RowChange](b) },
	"RowExplanation":              func(b []byte) (any, error) { return decode[RowExplanation](b) },
	"FeedRow":                     func(b []byte) (any, error) { return decode[FeedRow](b) },
	"FeedCounts":                  func(b []byte) (any, error) { return decode[FeedCounts](b) },
	"FeedRowsAnswer":              func(b []byte) (any, error) { return decode[FeedRowsAnswer](b) },
	"ScreenFeedAnswer":            func(b []byte) (any, error) { return decode[ScreenFeedAnswer](b) },
	"HistoryEntry":                func(b []byte) (any, error) { return decode[HistoryEntry](b) },
	"HistoryAnswer":               func(b []byte) (any, error) { return decode[HistoryAnswer](b) },
	"FocusPerson":                 func(b []byte) (any, error) { return decode[FocusPerson](b) },
	"LevelEntry":                  func(b []byte) (any, error) { return decode[LevelEntry](b) },
	"SettingsAnswer":              func(b []byte) (any, error) { return decode[SettingsAnswer](b) },
	"PreviewCounts":               func(b []byte) (any, error) { return decode[PreviewCounts](b) },
	"PreviewAnswer":               func(b []byte) (any, error) { return decode[PreviewAnswer](b) },
	"NotAvailable":                func(b []byte) (any, error) { return decode[NotAvailable](b) },
	"SuggestAnswer":               func(b []byte) (any, error) { return decode[SuggestAnswer](b) },
	"SearchResult":                func(b []byte) (any, error) { return decode[SearchResult](b) },
	"SearchAnswer":                func(b []byte) (any, error) { return decode[SearchAnswer](b) },
	"ScreenSearchAnswer":          func(b []byte) (any, error) { return decode[ScreenSearchAnswer](b) },
	"ReplyText":                   func(b []byte) (any, error) { return decode[ReplyText](b) },
	"ResumeNumbers":               func(b []byte) (any, error) { return decode[ResumeNumbers](b) },
	"LiveResume":                  func(b []byte) (any, error) { return decode[LiveResume](b) },
	"WatchingNote":                func(b []byte) (any, error) { return decode[WatchingNote](b) },
	"LiveWatching":                func(b []byte) (any, error) { return decode[LiveWatching](b) },
	"PartMessageCreated":          func(b []byte) (any, error) { return decode[PartMessageCreated](b) },
	"PartMessageChanged":          func(b []byte) (any, error) { return decode[PartMessageChanged](b) },
	"PartMessageDeleted":          func(b []byte) (any, error) { return decode[PartMessageDeleted](b) },
	"PartReactionChanged":         func(b []byte) (any, error) { return decode[PartReactionChanged](b) },
	"PartThreadSummary":           func(b []byte) (any, error) { return decode[PartThreadSummary](b) },
	"PartUnread":                  func(b []byte) (any, error) { return decode[PartUnread](b) },
	"PartReadMoved":               func(b []byte) (any, error) { return decode[PartReadMoved](b) },
	"PartReplyStarted":            func(b []byte) (any, error) { return decode[PartReplyStarted](b) },
	"PartReplyAbandoned":          func(b []byte) (any, error) { return decode[PartReplyAbandoned](b) },
	"PartConversationAdded":       func(b []byte) (any, error) { return decode[PartConversationAdded](b) },
	"PartPeopleAdded":             func(b []byte) (any, error) { return decode[PartPeopleAdded](b) },
	"PartPersonAdded":             func(b []byte) (any, error) { return decode[PartPersonAdded](b) },
	"PartPreference":              func(b []byte) (any, error) { return decode[PartPreference](b) },
	"PartProfileLine":             func(b []byte) (any, error) { return decode[PartProfileLine](b) },
	"FeedPartRowChanged":          func(b []byte) (any, error) { return decode[FeedPartRowChanged](b) },
	"FeedPartRowRemoved":          func(b []byte) (any, error) { return decode[FeedPartRowRemoved](b) },
	"FeedPartCounts":              func(b []byte) (any, error) { return decode[FeedPartCounts](b) },
	"FeedPartHistoryChanged":      func(b []byte) (any, error) { return decode[FeedPartHistoryChanged](b) },
	"FeedPartSettingsChanged":     func(b []byte) (any, error) { return decode[FeedPartSettingsChanged](b) },
	"LiveConversationsUpdate":     func(b []byte) (any, error) { return decode[LiveConversationsUpdate](b) },
	"LiveFeedUpdate":              func(b []byte) (any, error) { return decode[LiveFeedUpdate](b) },
	"LastSentNumbers":             func(b []byte) (any, error) { return decode[LastSentNumbers](b) },
	"LiveLastSent":                func(b []byte) (any, error) { return decode[LiveLastSent](b) },
	"LiveTooFarBehind":            func(b []byte) (any, error) { return decode[LiveTooFarBehind](b) },
	"PieceBody":                   func(b []byte) (any, error) { return decode[PieceBody](b) },
	"LivePiece":                   func(b []byte) (any, error) { return decode[LivePiece](b) },
	"AgentStatusBody":             func(b []byte) (any, error) { return decode[AgentStatusBody](b) },
	"LiveAgentStatus":             func(b []byte) (any, error) { return decode[LiveAgentStatus](b) },
	"RowUpdate":                   func(b []byte) (any, error) { return decode[RowUpdate](b) },
	"RowUpdatesPage":              func(b []byte) (any, error) { return decode[RowUpdatesPage](b) },
	"NewEdition":                  func(b []byte) (any, error) { return decode[NewEdition](b) },
	"SearchMessage":               func(b []byte) (any, error) { return decode[SearchMessage](b) },
	"SearchDeleted":               func(b []byte) (any, error) { return decode[SearchDeleted](b) },
	"SearchCreated":               func(b []byte) (any, error) { return decode[SearchCreated](b) },
	"SearchChanged":               func(b []byte) (any, error) { return decode[SearchChanged](b) },
	"SearchDeletedChange":         func(b []byte) (any, error) { return decode[SearchDeletedChange](b) },
	"SearchApplied":               func(b []byte) (any, error) { return decode[SearchApplied](b) },
	"SearchRefusal":               func(b []byte) (any, error) { return decode[SearchRefusal](b) },
	"NoteSend":                    func(b []byte) (any, error) { return decode[NoteSend](b) },
	"NoteUpdate":                  func(b []byte) (any, error) { return decode[NoteUpdate](b) },
	"NoteTaken":                   func(b []byte) (any, error) { return decode[NoteTaken](b) },
	"NoteBehind":                  func(b []byte) (any, error) { return decode[NoteBehind](b) },
	"NotificationEntry":           func(b []byte) (any, error) { return decode[NotificationEntry](b) },
	"AgentEventMentioned":         func(b []byte) (any, error) { return decode[AgentEventMentioned](b) },
	"AgentEventDirectMessage":     func(b []byte) (any, error) { return decode[AgentEventDirectMessage](b) },
	"AgentEventReply":             func(b []byte) (any, error) { return decode[AgentEventReply](b) },
	"AgentEventChanged":           func(b []byte) (any, error) { return decode[AgentEventChanged](b) },
	"AgentEventDeleted":           func(b []byte) (any, error) { return decode[AgentEventDeleted](b) },
	"AgentEventReaction":          func(b []byte) (any, error) { return decode[AgentEventReaction](b) },
	"AgentEvents":                 func(b []byte) (any, error) { return decode[AgentEvents](b) },
	"TooFarBehind":                func(b []byte) (any, error) { return decode[TooFarBehind](b) },
	"AgentPost":                   func(b []byte) (any, error) { return decode[AgentPost](b) },
	"AgentPosted":                 func(b []byte) (any, error) { return decode[AgentPosted](b) },
	"AgentReplyStart":             func(b []byte) (any, error) { return decode[AgentReplyStart](b) },
	"AgentPiece":                  func(b []byte) (any, error) { return decode[AgentPiece](b) },
	"AgentReplyFinish":            func(b []byte) (any, error) { return decode[AgentReplyFinish](b) },
	"PieceMissing":                func(b []byte) (any, error) { return decode[PieceMissing](b) },
	"AgentStatus":                 func(b []byte) (any, error) { return decode[AgentStatus](b) },
	"AgentReaction":               func(b []byte) (any, error) { return decode[AgentReaction](b) },
	"AgentDirect":                 func(b []byte) (any, error) { return decode[AgentDirect](b) },
	"AgentDirectAnswer":           func(b []byte) (any, error) { return decode[AgentDirectAnswer](b) },
	"FeedPlace":                   func(b []byte) (any, error) { return decode[FeedPlace](b) },
	"SearchPlace":                 func(b []byte) (any, error) { return decode[SearchPlace](b) },
	"SenderPlace":                 func(b []byte) (any, error) { return decode[SenderPlace](b) },
	"ReceiverStatus":              func(b []byte) (any, error) { return decode[ReceiverStatus](b) },
	"DeliveryStatus":              func(b []byte) (any, error) { return decode[DeliveryStatus](b) },
	"ParkedChange":                func(b []byte) (any, error) { return decode[ParkedChange](b) },
	"ParkedList":                  func(b []byte) (any, error) { return decode[ParkedList](b) },
	"ModeBody":                    func(b []byte) (any, error) { return decode[ModeBody](b) },
	"RebuildRequest":              func(b []byte) (any, error) { return decode[RebuildRequest](b) },
	"RestartRequest":              func(b []byte) (any, error) { return decode[RestartRequest](b) },
	"NotificationList":            func(b []byte) (any, error) { return decode[NotificationList](b) },
	"PeopleList":                  func(b []byte) (any, error) { return decode[PeopleList](b) },
	"ScriptStep":                  func(b []byte) (any, error) { return decode[ScriptStep](b) },
	"Situation":                   func(b []byte) (any, error) { return decode[Situation](b) },
	"ScriptState":                 func(b []byte) (any, error) { return decode[ScriptState](b) },
	"ScriptRequest":               func(b []byte) (any, error) { return decode[ScriptRequest](b) },
	"SeedOpened":                  func(b []byte) (any, error) { return decode[SeedOpened](b) },
	"SeedBatch":                   func(b []byte) (any, error) { return decode[SeedBatch](b) },
	"SeedThrough":                 func(b []byte) (any, error) { return decode[SeedThrough](b) },
	"Empty":                       func(b []byte) (any, error) { return decode[Empty](b) },
	"WipeRequest":                 func(b []byte) (any, error) { return decode[WipeRequest](b) },
}
