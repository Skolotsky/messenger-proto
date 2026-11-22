// Package rules holds the activity feed's product rules for one organization.
//
// It is pure: no database, no network and no clock. Apply takes one change of
// the organization's log and says what happens to each reader's rows, counts,
// history tabs and notification notes. Time comes only from the change.
//
// State is divided on purpose (D-030, G-030). What belongs to a conversation
// lives in its conv and is touched only by that conversation's changes.
// What belongs to a reader as a whole lives in person and is touched only by
// organization-wide changes, which stay at their place in the log.
package rules

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"contexia/internal/shared/contract"
)

// Bucket is where a row sits. Higher wins (D-001).
type Bucket int

// The three buckets, and "no bucket".
const (
	NoBucket Bucket = 0
	FYI      Bucket = 1
	Needs    Bucket = 2
	Focus    Bucket = 3
)

// Contract gives the bucket's name in the contracts.
func (b Bucket) Contract() contract.Bucket {
	switch b {
	case Focus:
		return "focus"
	case Needs:
		return "needs_you"
	}
	return "fyi"
}

// The thread levels, in the contract's words (D-001).
const (
	levelFocus = "focus"
	levelNeeds = "needs_you"
	levelFYI   = "fyi"
	levelOff   = "off"
	levelAuto  = "auto"
)

const everyone = "channel" // a whole-channel mention in a message's mentions (screen-to-conversations.md)

// holdFor is the notification hold (D-007, feed-to-notification-sender.md).
const holdFor = 5 * time.Second

// ErrCannotApply marks a change these rules cannot apply. The feed sets it
// aside and fences what it touches (conversations-to-feed.md, section 4).
var ErrCannotApply = errors.New("rules: change cannot be applied")

// Options changes how one change is applied.
type Options struct {
	// NoNotes writes no notification notes: seeded history (D-032).
	NoNotes bool
	// NoSummaries leaves out the counts and history parts, which read across
	// conversations, so that different conversations can be applied at the
	// same time (D-030). Call Summaries for the readers once they are done.
	NoSummaries bool
}

// ReaderEffect is what one change did for one reader.
type ReaderEffect struct {
	Person string
	Parts  []contract.FeedPart
}

// Effects is what one change did.
type Effects struct {
	// Known is false for a kind that has no effect on the feed.
	Known   bool
	Readers []ReaderEffect
	Notes   []NoteDraft
	// Touched lists the readers whose counts or tabs may have changed.
	Touched []string
}

// NoteDraft is a notification note before the feed gives it its number.
type NoteDraft struct {
	Kind         string // due, changed, withdrawn, clock
	At           string
	Name         string
	Conversation string // empty for a clock note
	Row          contract.RowKey
	Person       string
	Bucket       contract.Bucket
	Why          string
	Text         string
	Because      contract.WithdrawReason
	// Message and Item say what caused a due note. They are not sent.
	Message string
	Item    string
}

// Contract gives the note its place in the feed's numbered log.
func (n NoteDraft) Contract(org string, number, previous, edition int64) contract.Note {
	switch n.Kind {
	case "due":
		return contract.Note{Value: contract.NoteDue{Kind: "due", Org: org, Number: number, Previous: previous, Edition: edition, At: n.At, Name: n.Name, Row: n.Row, Person: n.Person, Bucket: n.Bucket, Why: n.Why, Text: n.Text}}
	case "changed":
		return contract.Note{Value: contract.NoteChanged{Kind: "changed", Org: org, Number: number, Previous: previous, Edition: edition, At: n.At, Name: n.Name, Text: n.Text}}
	case "withdrawn":
		return contract.Note{Value: contract.NoteWithdrawn{Kind: "withdrawn", Org: org, Number: number, Previous: previous, Edition: edition, At: n.At, Name: n.Name, Because: n.Because}}
	}
	return contract.Note{Value: contract.NoteClock{Kind: "clock", Org: org, Number: number, Previous: previous, Edition: edition, At: n.At}}
}

// Engine is the rules' state for one organization.
type Engine struct {
	org     string
	mu      sync.RWMutex // guards the convs map only
	convs   map[string]*conv
	people  map[string]*person
	readers []string // people who are not agents, sorted
}

// New gives an empty organization.
func New(org string) *Engine {
	return &Engine{org: org, convs: map[string]*conv{}, people: map[string]*person{}}
}

// person is what is known about a reader as a whole: organization-wide state.
type person struct {
	id, name     string
	agent        bool
	rules        map[contract.FocusRule]bool
	removedRules map[contract.FocusRule]bool
	focusPeople  map[string]bool // added by hand
	removed      map[string]bool // taken out by hand; not added back by writing again (D-021)
	autoAdd      bool
	focusOnly    bool
	releaseOnAct bool
	// last summary sent, so that only a change produces a part
	lastCounts *contract.FeedCounts
	lastTabs   map[contract.HistoryTab]int64
	faultSeen  int // only with the g030fault build tag
}

var flagsRule = contract.FocusRule{Type: "item_kind", Value: "flags"}
var mentionsRule = contract.FocusRule{Type: "item_kind", Value: "mentions"}

func newPerson(id, name string, agent bool) *person {
	return &person{id: id, name: name, agent: agent,
		rules: map[contract.FocusRule]bool{flagsRule: true}, removedRules: map[contract.FocusRule]bool{},
		focusPeople: map[string]bool{}, removed: map[string]bool{},
		autoAdd: true, releaseOnAct: true, lastTabs: map[contract.HistoryTab]int64{}}
}

// conv is everything that belongs to one conversation (D-030).
type conv struct {
	id, name string
	direct   bool
	members  map[string]bool
	msgs     map[string]*message
	order    []*message
	replies  map[string][]*message // thread root -> replies, in order
	readers  map[string]*convReader
}

// message is the feed's copy of a message.
type message struct {
	id, conv, author, root string
	parent                 string // empty for a top-level message
	seq                    int
	number, version        int64
	agentAuthor            bool
	text                   string
	mentions               map[string]bool
	group                  bool
	level                  string
	stopped, deleted       bool
	edited                 bool
	at                     string
	reactions              []*reaction
}

type reaction struct {
	person, emoji string
	number        int64
	at            string
}

// convReader is one reader's state inside one conversation.
type convReader struct {
	dial      string            // the channel's dial; empty is default
	thDial    map[string]string // explicit thread levels
	rows      map[rowKey]*row
	itemRow   map[itemKey]rowKey
	posConv   int64            // read position of the channel or direct conversation
	posThread map[string]int64 // read position per thread
	watching  map[string]map[string]bool
	autoFocus bool // D-021: this person wrote to the reader here while the option was on
	emitted   map[rowKey]string
	tabs      map[contract.HistoryTab]int64
}

func newConvReader() *convReader {
	return &convReader{thDial: map[string]string{}, rows: map[rowKey]*row{}, itemRow: map[itemKey]rowKey{},
		posThread: map[string]int64{}, watching: map[string]map[string]bool{}, emitted: map[rowKey]string{},
		tabs: map[contract.HistoryTab]int64{}}
}

// rowKey names a row. "th" is a thread (a top-level message that addresses
// you is the row of its own thread), "ch" the channel-level row, "dm" a direct
// conversation (contracts README, "Row").
type rowKey struct{ kind, id string }

func (k rowKey) thread() *string {
	if k.kind == "th" {
		id := k.id
		return &id
	}
	return nil
}

type itemKey struct{ msg, kind string }

const (
	itemMsg      = "msg"
	itemReaction = "reaction"
	answerPrefix = "answer:"
)

// row is one reader's row.
type row struct {
	key     rowKey
	items   map[itemKey]*item
	kept    bool            // kept by the person (D-003)
	flagged map[string]bool // messages whose flag keeps this row for the person (D-011)
	hold    bool            // a marker that stays until Done (D-004)
	last    int64
	dues    []*openDue
}

type item struct {
	msg       *message
	kind      string
	bucket    Bucket
	reason    string
	decider   string
	number    int64
	read      bool
	retracted bool
	edited    bool
	flag      string
}

// openDue is a notification that was written due and not withdrawn.
type openDue struct {
	name   string
	item   itemKey
	at     time.Time
	number int64
}

func (cv *conv) reader(u string) *convReader {
	r := cv.readers[u]
	if r == nil {
		r = newConvReader()
		cv.readers[u] = r
	}
	return r
}

// peek gives the reader's state without creating it, for questions.
func (cv *conv) peek(u string) *convReader {
	if r := cv.readers[u]; r != nil {
		return r
	}
	return newConvReader()
}

func (e *Engine) conv(id string) *conv {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.convs[id]
}

func (e *Engine) allConvs() []*conv {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*conv, 0, len(e.convs))
	for _, c := range e.convs {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

func (e *Engine) addConv(c *conv) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.convs[c.id] = c
}

func (e *Engine) displayName(id string) string {
	if p := e.people[id]; p != nil && p.name != "" {
		return p.name
	}
	return id
}

// fx collects what one change does.
type fx struct {
	number  int64
	atText  string
	at      time.Time
	noNotes bool
	notes   []NoteDraft
	touched map[string]map[string]map[rowKey]bool // conversation -> reader -> rows
	readers map[string]bool
}

func (f *fx) touch(cv *conv, u string, k rowKey) {
	f.touchReader(cv, u)
	f.touched[cv.id][u][k] = true
}

func (f *fx) touchReader(cv *conv, u string) {
	if f.touched[cv.id] == nil {
		f.touched[cv.id] = map[string]map[rowKey]bool{}
	}
	if f.touched[cv.id][u] == nil {
		f.touched[cv.id][u] = map[rowKey]bool{}
	}
	f.readers[u] = true
}

func cannot(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrCannotApply, fmt.Sprintf(format, a...))
}

func header(c contract.Change) (number int64, at string, err error) {
	switch v := c.Value.(type) {
	case contract.ChangeMessageCreated:
		return v.Number, v.At, nil
	case contract.ChangeMessageChanged:
		return v.Number, v.At, nil
	case contract.ChangeMessageDeleted:
		return v.Number, v.At, nil
	case contract.ChangeReplyStarted:
		return v.Number, v.At, nil
	case contract.ChangeReplyAbandoned:
		return v.Number, v.At, nil
	case contract.ChangeReactionAdded:
		return v.Number, v.At, nil
	case contract.ChangeReactionRemoved:
		return v.Number, v.At, nil
	case contract.ChangePersonJoined:
		return v.Number, v.At, nil
	case contract.ChangeChannelCreated:
		return v.Number, v.At, nil
	case contract.ChangeChannelPeopleAdded:
		return v.Number, v.At, nil
	case contract.ChangeDirectCreated:
		return v.Number, v.At, nil
	case contract.ChangeReaderRead:
		return v.Number, v.At, nil
	case contract.ChangeReaderWatchingStarted:
		return v.Number, v.At, nil
	case contract.ChangeReaderWatchingEnded:
		return v.Number, v.At, nil
	case contract.ChangeReaderKeep:
		return v.Number, v.At, nil
	case contract.ChangeReaderDone:
		return v.Number, v.At, nil
	case contract.ChangeReaderDial:
		return v.Number, v.At, nil
	case contract.ChangeReaderFocusRule:
		return v.Number, v.At, nil
	case contract.ChangeReaderFocusPerson:
		return v.Number, v.At, nil
	case contract.ChangeReaderFocusAutoAdd:
		return v.Number, v.At, nil
	case contract.ChangeReaderFocusOnly:
		return v.Number, v.At, nil
	case contract.ChangeReaderReleaseSetting:
		return v.Number, v.At, nil
	case contract.ChangeReaderPreference:
		return v.Number, v.At, nil
	case contract.ChangeReaderProfileLine:
		return v.Number, v.At, nil
	case contract.ChangeTick:
		return v.Number, v.At, nil
	}
	return 0, "", cannot("unknown change %T", c.Value)
}

// IsOrgWide says whether a change is organization-wide: everything before it
// must have been applied first (conversations-to-feed.md, section 3; D-030).
func IsOrgWide(c contract.Change) bool {
	switch c.Value.(type) {
	case contract.ChangeReaderFocusRule, contract.ChangeReaderFocusPerson, contract.ChangeReaderFocusAutoAdd,
		contract.ChangeReaderFocusOnly, contract.ChangeReaderReleaseSetting, contract.ChangePersonJoined, contract.ChangeTick:
		return true
	}
	return false
}

// Apply applies one change and says what it did. On an error nothing changed.
func (e *Engine) Apply(c contract.Change, opt Options) (Effects, error) {
	number, atText, err := header(c)
	if err != nil {
		return Effects{}, err
	}
	at, err := time.Parse(time.RFC3339Nano, atText)
	if err != nil {
		return Effects{}, cannot("change %d has a bad time %q", number, atText)
	}
	f := &fx{number: number, atText: atText, at: at, noNotes: opt.NoNotes,
		touched: map[string]map[string]map[rowKey]bool{}, readers: map[string]bool{}}
	known, err := e.dispatch(c, f)
	if err != nil {
		return Effects{}, err
	}
	out := Effects{Known: known, Notes: f.notes}
	parts := e.flushRows(f)
	for u := range f.readers {
		out.Touched = append(out.Touched, u)
	}
	sort.Strings(out.Touched)
	if !opt.NoSummaries {
		for _, u := range out.Touched {
			parts[u] = append(parts[u], e.Summaries(u)...)
		}
	}
	names := make([]string, 0, len(parts))
	for u := range parts {
		names = append(names, u)
	}
	sort.Strings(names)
	for _, u := range names {
		if len(parts[u]) > 0 {
			out.Readers = append(out.Readers, ReaderEffect{Person: u, Parts: parts[u]})
		}
	}
	return out, nil
}

func (e *Engine) dispatch(c contract.Change, f *fx) (bool, error) {
	switch v := c.Value.(type) {
	case contract.ChangeMessageCreated:
		return true, e.messageCreated(v.Message, f)
	case contract.ChangeMessageChanged:
		return true, e.messageChanged(v.Message, f)
	case contract.ChangeMessageDeleted:
		return true, e.messageDeleted(v.Message, f)
	case contract.ChangeReactionAdded:
		return true, e.reactionAdded(v, f)
	case contract.ChangeReactionRemoved:
		return true, e.reactionRemoved(v, f)
	case contract.ChangePersonJoined:
		e.personJoined(v)
		return true, nil
	case contract.ChangeChannelCreated:
		e.addConv(&conv{id: v.Conversation, name: v.Name, members: setOf(v.People),
			msgs: map[string]*message{}, replies: map[string][]*message{}, readers: map[string]*convReader{}})
		return true, nil
	case contract.ChangeDirectCreated:
		if len(v.People) != 2 {
			return false, cannot("direct conversation %s does not have two people", v.Conversation)
		}
		e.addConv(&conv{id: v.Conversation, name: v.Conversation, direct: true, members: setOf(v.People),
			msgs: map[string]*message{}, replies: map[string][]*message{}, readers: map[string]*convReader{}})
		return true, nil
	case contract.ChangeChannelPeopleAdded:
		cv := e.conv(v.Conversation)
		if cv == nil {
			return false, cannot("people added to unknown conversation %s", v.Conversation)
		}
		for _, p := range v.People {
			cv.members[p] = true
		}
		return true, nil
	case contract.ChangeReaderRead:
		return true, e.readerRead(v, f)
	case contract.ChangeReaderWatchingStarted:
		return true, e.watching(v.Person, v.Conversation, v.Thread, v.Connection, true)
	case contract.ChangeReaderWatchingEnded:
		return true, e.watching(v.Person, v.Conversation, v.Thread, v.Connection, false)
	case contract.ChangeReaderKeep:
		return true, e.keep(v.Person, v.Conversation, v.Thread, f)
	case contract.ChangeReaderDone:
		return true, e.done(v.Person, v.Conversation, v.Thread, f)
	case contract.ChangeReaderDial:
		return true, e.dial(v, f)
	case contract.ChangeReaderFocusRule:
		return true, e.focusRule(v, f)
	case contract.ChangeReaderFocusPerson:
		return true, e.focusPerson(v, f)
	case contract.ChangeReaderFocusAutoAdd:
		p := e.people[v.Person]
		if p == nil {
			return false, cannot("unknown person %s", v.Person)
		}
		p.autoAdd = v.On
		f.readers[v.Person] = true
		return true, nil
	case contract.ChangeReaderFocusOnly:
		p := e.people[v.Person]
		if p == nil {
			return false, cannot("unknown person %s", v.Person)
		}
		p.focusOnly = v.On
		f.readers[v.Person] = true
		return true, nil
	case contract.ChangeReaderReleaseSetting:
		p := e.people[v.Person]
		if p == nil {
			return false, cannot("unknown person %s", v.Person)
		}
		p.releaseOnAct = v.On
		f.readers[v.Person] = true
		return true, nil
	case contract.ChangeTick:
		if !f.noNotes {
			f.notes = append(f.notes, NoteDraft{Kind: "clock", At: f.atText})
		}
		return true, nil
	}
	return false, nil // reply.started, reply.abandoned, reader.preference, reader.profile_line: counted, no effect
}

func (e *Engine) personJoined(v contract.ChangePersonJoined) {
	if p := e.people[v.Person]; p != nil {
		p.name, p.agent = v.Name, v.IsAgent
	} else {
		e.people[v.Person] = newPerson(v.Person, v.Name, v.IsAgent)
	}
	e.readers = e.readers[:0]
	for id, p := range e.people {
		if !p.agent {
			e.readers = append(e.readers, id)
		}
	}
	sort.Strings(e.readers)
}

func setOf(list []string) map[string]bool {
	s := make(map[string]bool, len(list))
	for _, x := range list {
		s[x] = true
	}
	return s
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
