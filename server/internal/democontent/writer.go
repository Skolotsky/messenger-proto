package democontent

import (
	"fmt"
	"slices"
	"time"

	"contexia/internal/shared/contract"
)

const stampFormat = "2006-01-02T15:04:05.000Z"

// msg is a message as the log knows it so far.
type msg struct {
	id, conv, author string
	root             *msg
	number, version  int64
	text             string
	mentions         []string
	level            string
	reactions        []contract.Reaction
	created          string
	edited           *string
	deleted          bool
}

// writer turns actions into numbered changes, the way Conversations writes them:
// a reply, reaction, Keep or Done is followed by the reader.read it implies.
type writer struct {
	c        *Content
	org      string
	out      []contract.Change
	number   int64
	at       time.Time
	nextID   int
	idPrefix string

	top      map[string]int64 // conversation: newest top-level number
	inThread map[string]int64 // thread root: newest number in it
	pos      map[string]int64 // person, conversation, thread: read position
	threads  map[string][]*msg
	inList   map[string]bool
}

func newWriter(c *Content, number int64, at time.Time, idPrefix string) *writer {
	return &writer{c: c, org: c.Org.Org, number: number, at: at, idPrefix: idPrefix,
		top: map[string]int64{}, inThread: map[string]int64{}, pos: map[string]int64{},
		threads: map[string][]*msg{}, inList: map[string]bool{}}
}

// next stamps the next change: numbers never skip and times never decrease.
func (w *writer) next(at time.Time) (number, previous int64, stamp string) {
	if at.After(w.at) {
		w.at = at
	}
	w.number++
	return w.number, w.number - 1, w.at.UTC().Format(stampFormat)
}

func (w *writer) add(v any) { w.out = append(w.out, contract.Change{Value: v}) }

func threadOf(m *msg) *string {
	if m.root == nil {
		return nil
	}
	return &m.root.id
}

func (w *writer) wire(m *msg) contract.Message {
	var level *contract.Level
	if m.level != "" {
		l := contract.Level(m.level)
		level = &l
	}
	var participants []string
	if p := w.c.members[m.conv]; len(m.conv) > 3 && m.conv[:3] == "dm:" {
		for id := range p {
			participants = append(participants, id)
		}
		slices.Sort(participants)
	}
	reactions := make([]contract.Reaction, 0, len(m.reactions))
	for _, r := range m.reactions {
		reactions = append(reactions, contract.Reaction{Emoji: r.Emoji, People: slices.Clone(r.People)})
	}
	return contract.Message{Id: m.id, Number: m.number, Version: m.version, Conversation: m.conv,
		Thread: threadOf(m), Author: m.author, AuthorIsAgent: w.c.people[m.author].Agent,
		Participants: participants, Text: m.text, Mentions: slices.Clone(m.mentions), Level: level,
		Reactions: reactions, Stopped: false, CreatedAt: m.created, EditedAt: m.edited, Deleted: m.deleted}
}

func (w *writer) checkedMentions(conv, author, text string) ([]string, error) {
	ids, err := w.c.mentionsIn(text)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, id := range ids {
		if !w.c.members[conv][id] {
			return nil, fmt.Errorf("%s mentions %s, who is not in %s", author, id, conv)
		}
		if id != author {
			out = append(out, id)
		}
	}
	return out, nil
}

func (w *writer) post(at time.Time, author, conv string, root *msg, text, level string) (*msg, error) {
	if !w.c.members[conv][author] {
		return nil, fmt.Errorf("%s is not in %s", author, conv)
	}
	mentions, err := w.checkedMentions(conv, author, text)
	if err != nil {
		return nil, err
	}
	n, prev, stamp := w.next(at)
	w.nextID++
	m := &msg{id: fmt.Sprintf("msg_%s%05d", w.idPrefix, w.nextID), conv: conv, author: author, root: root,
		number: n, version: 1, text: text, mentions: mentions, level: level, created: stamp}
	w.add(contract.ChangeMessageCreated{Kind: "message.created", Org: w.org, Number: n, Previous: prev, At: stamp, Message: w.wire(m)})
	if root == nil {
		w.top[conv] = n
		w.inThread[m.id] = n
	} else {
		w.inThread[root.id] = n
	}
	thread := m
	if root != nil {
		thread = root
	}
	w.follow(author, thread)
	for _, p := range mentions {
		w.follow(p, thread)
	}
	if !w.c.people[author].Agent {
		if root != nil {
			w.read(at, author, conv, &root.id, n)
		} else {
			w.read(at, author, conv, nil, n)
		}
	}
	return m, nil
}

func (w *writer) follow(person string, thread *msg) {
	k := person + "|" + thread.id
	if !w.inList[k] {
		w.inList[k] = true
		w.threads[person] = append(w.threads[person], thread)
	}
}

// edit applies a new text, and a new level when level is not "same" ("none" clears it).
func (w *writer) edit(at time.Time, m *msg, text, level string) error {
	if m.deleted {
		return nil
	}
	mentions, err := w.checkedMentions(m.conv, m.author, text)
	if err != nil {
		return err
	}
	n, prev, stamp := w.next(at)
	m.version++
	m.text, m.mentions = text, mentions
	if level == "none" {
		m.level = ""
	} else if level != "same" {
		m.level = level
	}
	m.edited = &stamp
	for _, p := range mentions {
		if m.root != nil {
			w.follow(p, m.root)
		} else {
			w.follow(p, m)
		}
	}
	w.add(contract.ChangeMessageChanged{Kind: "message.changed", Org: w.org, Number: n, Previous: prev, At: stamp, Message: w.wire(m)})
	return nil
}

func (w *writer) remove(at time.Time, m *msg) {
	if m.deleted {
		return
	}
	n, prev, stamp := w.next(at)
	m.version++
	m.deleted, m.text, m.mentions, m.reactions = true, "", []string{}, nil
	w.add(contract.ChangeMessageDeleted{Kind: "message.deleted", Org: w.org, Number: n, Previous: prev, At: stamp,
		Message: contract.DeletedMessage{Id: m.id, Version: m.version, Conversation: m.conv, Thread: threadOf(m)}})
}

// rowOf is the row an act on message m by person belongs to (docs/contracts/README.md, "Row").
func (w *writer) rowOf(person string, m *msg) *string {
	if m.root != nil {
		return &m.root.id
	}
	if len(m.conv) > 3 && m.conv[:3] == "dm:" {
		return nil
	}
	if slices.Contains(m.mentions, person) {
		return &m.id
	}
	return nil
}

func (w *writer) newest(conv string, thread *string) int64 {
	if thread != nil {
		return w.inThread[*thread]
	}
	return w.top[conv]
}

func (w *writer) react(at time.Time, person string, m *msg, emoji string) {
	if m.deleted {
		return
	}
	i := slices.IndexFunc(m.reactions, func(r contract.Reaction) bool { return r.Emoji == emoji })
	if i >= 0 && slices.Contains(m.reactions[i].People, person) {
		return
	}
	n, prev, stamp := w.next(at)
	if i < 0 {
		m.reactions = append(m.reactions, contract.Reaction{Emoji: emoji, People: []string{person}})
	} else {
		m.reactions[i].People = append(m.reactions[i].People, person)
	}
	w.add(contract.ChangeReactionAdded{Kind: "reaction.added", Org: w.org, Number: n, Previous: prev, At: stamp,
		Person: person, Emoji: emoji, Message: m.id, Conversation: m.conv, Thread: threadOf(m)})
	if !w.c.people[person].Agent {
		row := w.rowOf(person, m)
		w.read(at, person, m.conv, row, w.newest(m.conv, row))
	}
}

func (w *writer) unreact(at time.Time, person string, m *msg, emoji string) {
	if m.deleted {
		return
	}
	i := slices.IndexFunc(m.reactions, func(r contract.Reaction) bool { return r.Emoji == emoji })
	if i < 0 || !slices.Contains(m.reactions[i].People, person) {
		return
	}
	n, prev, stamp := w.next(at)
	m.reactions[i].People = slices.DeleteFunc(m.reactions[i].People, func(p string) bool { return p == person })
	if len(m.reactions[i].People) == 0 {
		m.reactions = slices.Delete(m.reactions, i, i+1)
	}
	w.add(contract.ChangeReactionRemoved{Kind: "reaction.removed", Org: w.org, Number: n, Previous: prev, At: stamp,
		Person: person, Emoji: emoji, Message: m.id, Conversation: m.conv, Thread: threadOf(m)})
}

// read moves a read position forward; it writes nothing when there is nothing new.
func (w *writer) read(at time.Time, person, conv string, thread *string, upTo int64) {
	k := person + "|" + conv + "|"
	if thread != nil {
		k += *thread
	}
	if upTo <= w.pos[k] {
		return
	}
	w.pos[k] = upTo
	n, prev, stamp := w.next(at)
	w.add(contract.ChangeReaderRead{Kind: "reader.read", Org: w.org, Number: n, Previous: prev, At: stamp,
		Person: person, Conversation: conv, Thread: thread, UpTo: upTo})
}

func (w *writer) keep(at time.Time, person string, m *msg) {
	row := w.keepRow(m)
	n, prev, stamp := w.next(at)
	w.add(contract.ChangeReaderKeep{Kind: "reader.keep", Org: w.org, Number: n, Previous: prev, At: stamp,
		Person: person, Conversation: m.conv, Thread: row})
	w.read(at, person, m.conv, row, w.newest(m.conv, row))
}

func (w *writer) done(at time.Time, person string, m *msg) {
	row := w.keepRow(m)
	n, prev, stamp := w.next(at)
	w.add(contract.ChangeReaderDone{Kind: "reader.done", Org: w.org, Number: n, Previous: prev, At: stamp,
		Person: person, Conversation: m.conv, Thread: row})
	w.read(at, person, m.conv, row, w.newest(m.conv, row))
}

// keepRow is the row of a message for Keep and Done: its thread, or the direct conversation.
func (w *writer) keepRow(m *msg) *string {
	if len(m.conv) > 3 && m.conv[:3] == "dm:" {
		return nil
	}
	if m.root != nil {
		return &m.root.id
	}
	return &m.id
}

func (w *writer) dial(at time.Time, person, conv string, thread *string, level string) {
	n, prev, stamp := w.next(at)
	w.add(contract.ChangeReaderDial{Kind: "reader.dial", Org: w.org, Number: n, Previous: prev, At: stamp,
		Person: person, Conversation: conv, Thread: thread, Level: contract.DialLevel(level)})
}
