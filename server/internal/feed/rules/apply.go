package rules

import (
	"fmt"
	"sort"
	"strings"

	"contexia/internal/shared/contract"
)

func (e *Engine) newMessage(cv *conv, c contract.Message) *message {
	m := &message{id: c.Id, conv: cv.id, author: c.Author, number: c.Number, version: c.Version,
		agentAuthor: c.AuthorIsAgent, stopped: c.Stopped, at: c.CreatedAt, seq: len(cv.order)}
	m.root = c.Id
	if c.Thread != nil && *c.Thread != c.Id {
		m.parent, m.root = *c.Thread, *c.Thread
	}
	setContent(m, c)
	return m
}

func setContent(m *message, c contract.Message) {
	m.text = c.Text
	m.mentions, m.group = map[string]bool{}, false
	for _, p := range c.Mentions {
		switch p {
		case everyone:
			m.group = true
		case m.author:
		default:
			m.mentions[p] = true
		}
	}
	m.level = ""
	if c.Level != nil {
		m.level = string(*c.Level)
	}
}

func (e *Engine) messageCreated(c contract.Message, f *fx) error {
	cv := e.conv(c.Conversation)
	if cv == nil {
		return cannot("message %s in unknown conversation %s", c.Id, c.Conversation)
	}
	if cv.msgs[c.Id] != nil {
		return cannot("message %s created twice", c.Id)
	}
	if c.Thread != nil && *c.Thread != c.Id && cv.msgs[*c.Thread] == nil {
		return cannot("reply %s to unknown message %s", c.Id, *c.Thread)
	}
	if c.Number == 0 {
		c.Number = f.number
	}
	m := e.newMessage(cv, c)
	cv.msgs[m.id] = m
	cv.order = append(cv.order, m)
	if m.parent != "" {
		cv.replies[m.root] = append(cv.replies[m.root], m)
	}

	// The author acted: their own rows here are read and released, and a
	// thread they had turned off is on again (D-001, D-003, D-015, D-023 point 26).
	if a := e.people[m.author]; a != nil && !a.agent {
		r := cv.reader(a.id)
		f.touchReader(cv, a.id) // writing can change the thread's level, so its tab
		if m.parent != "" && r.thDial[m.root] == levelOff {
			r.thDial[m.root] = levelNeeds
		}
		var keys []rowKey
		if m.parent != "" {
			keys = append(keys, rowKey{"th", m.root})
			if k, ok := r.itemRow[itemKey{m.root, itemMsg}]; ok {
				keys = append(keys, k)
			}
		}
		if cv.direct {
			keys = append(keys, rowKey{"dm", cv.id})
		}
		for _, k := range keys {
			row := r.rows[k]
			if row == nil {
				continue
			}
			for _, it := range row.items {
				it.read = true
			}
			if a.releaseOnAct {
				row.kept, row.hold = false, false
				row.flagged = map[string]bool{}
			}
			f.touch(cv, a.id, k)
		}
	}

	// D-021: the first direct message from a person adds them to the reader's Focus people.
	if cv.direct && !m.agentAuthor {
		for _, u := range sortedKeys(cv.members) {
			if p := e.people[u]; p != nil && u != m.author && p.autoAdd && !p.removed[m.author] {
				cv.reader(u).autoFocus = true
			}
		}
	}

	for _, u := range e.readers {
		v := view{e.people[u], cv.peek(u)}
		if cl := e.classify(cv, v, m); cl != nil {
			e.deliver(cv, e.people[u], cv.reader(u), m, *cl, itemMsg, f)
		}
	}
	e.settleConversation(cv, f)
	return nil
}

// unreadBucket is the highest bucket among the row's unread items that could
// themselves have notified, so not a silent Info item.
func unreadBucket(r *row, skip *item) Bucket {
	var best Bucket
	if r == nil {
		return best
	}
	for _, it := range r.items {
		if it == skip || it.read || it.retracted || (it.kind == itemMsg && it.msg.level == "info") {
			continue
		}
		if it.bucket > best {
			best = it.bucket
		}
	}
	return best
}

// watches says whether the reader has this message open in front of them:
// its thread, or the conversation when the message is shown there (D-015).
func watches(cv *conv, r *convReader, m *message) bool {
	if len(r.watching[m.root]) > 0 {
		return true
	}
	return (cv.direct || m.parent == "") && len(r.watching[""]) > 0
}

func (e *Engine) deliver(cv *conv, p *person, r *convReader, m *message, cl class, kind string, f *fx) *item {
	if crossConversationFault {
		e.mu.Lock()
		p.faultSeen++
		if p.faultSeen%2 == 0 {
			cl.bucket = FYI
		}
		e.mu.Unlock()
	}
	rw := r.rows[cl.key]
	prev := unreadBucket(rw, nil)
	if rw == nil {
		rw = &row{key: cl.key, items: map[itemKey]*item{}, flagged: map[string]bool{}}
		r.rows[cl.key] = rw
	}
	it := &item{msg: m, kind: kind, bucket: cl.bucket, reason: cl.reason, decider: cl.decider, number: f.number, flag: cl.flag}
	k := itemKey{m.id, kind}
	rw.items[k] = it
	r.itemRow[k] = cl.key
	if cl.flag == "named" {
		rw.flagged[m.id] = true // D-011: a flag that names you keeps the row for you
	}
	rw.last = f.number
	f.touch(cv, p.id, cl.key)
	if watches(cv, r, m) {
		it.read = true // D-015 rule 6: read at once, no row and no notification
		return it
	}
	e.notify(cv, p, rw, it, prev, f)
	return it
}

// notify applies D-012: a row notifies when it becomes unread in a bucket that
// notifies, once, and again if it moves up into Focus.
func (e *Engine) notify(cv *conv, p *person, rw *row, it *item, prev Bucket, f *fx) {
	if f.noNotes || it.msg.stopped || (it.kind == itemMsg && it.msg.level == "info") {
		return
	}
	least := Needs
	if p.focusOnly {
		least = Focus
	}
	if it.bucket < least || !(prev < least || (it.bucket == Focus && prev < Focus)) {
		return
	}
	name := fmt.Sprintf("%s / %s / %s / %d", e.org, p.id, rowName(cv, rw.key), f.number)
	d := &openDue{name: name, item: itemKey{it.msg.id, it.kind}, at: f.at, atText: f.atText, number: f.number, person: p.id, key: rw.key, held: true, came: arrivalNow()}
	rw.dues = append(rw.dues, d)
	cv.holding[d] = true
	f.held = append(f.held, NoteDraft{Kind: "held", At: f.atText, Name: name, Conversation: cv.id,
		Row: contract.RowKey{Conversation: cv.id, Thread: rw.key.thread()}, Person: p.id, Bucket: it.bucket.Contract(),
		Why: it.reason, Text: it.msg.text, DueAt: f.atText, Message: it.msg.id, Item: it.kind})
}

func rowName(cv *conv, k rowKey) string {
	if k.kind == "th" {
		return cv.id + ":" + k.id
	}
	return cv.id
}

// notifying is what keeps a notification of this row due: something unread
// that earns a notifying bucket for this person.
func notifying(p *person, it *item) bool {
	least := Needs
	if p.focusOnly {
		least = Focus
	}
	return !it.read && !it.retracted && !it.msg.stopped && it.bucket >= least && !(it.kind == itemMsg && it.msg.level == "info")
}

// liveItems is what still earns this row a notification, oldest first.
func liveItems(p *person, rw *row) []*item {
	var live []*item
	for _, it := range sortedItems(rw) {
		if notifying(p, it) {
			live = append(live, it)
		}
	}
	return live
}

// update writes what an entry already sent must now show.
func (f *fx) update(cv *conv, d *openDue, n NoteDraft) {
	if f.noNotes {
		return
	}
	n.Kind, n.At, n.Name, n.Conversation, n.Person = "update", f.atText, d.name, cv.id, d.person
	f.notes = append(f.notes, n)
}

// follow moves a sent notification to the message it now speaks of.
func (cv *conv) follow(d *openDue, to itemKey) {
	if d.item.msg != to.msg {
		list := cv.sent[d.item.msg]
		for i, x := range list {
			if x == d {
				cv.sent[d.item.msg] = append(list[:i:i], list[i+1:]...)
				break
			}
		}
		cv.sent[to.msg] = append(cv.sent[to.msg], d)
	}
	d.item = to
}

// settleDues says what became of the row's notifications. One on hold that
// nothing earns any more is removed and leaves no note (D-034). One already
// sent cannot be recalled: its entry is told the truth.
func (e *Engine) settleDues(cv *conv, p *person, rw *row, f *fx) {
	if len(rw.dues) == 0 {
		return
	}
	live := liveItems(p, rw)
	keep := rw.dues[:0]
	for _, d := range rw.dues {
		src := rw.items[d.item]
		gone := src == nil || src.retracted
		if d.held {
			if len(live) == 0 && !fault(FaultReadKeepsHold) {
				delete(cv.holding, d)
				continue
			}
			if gone && len(live) > 0 { // what it spoke of is gone, the row is still due: it must not show it
				n := live[len(live)-1]
				d.item = itemKey{n.msg.id, n.kind}
			}
			keep = append(keep, d)
			continue
		}
		switch {
		case len(live) == 0:
			if src != nil && src.read && !src.retracted {
				d.shown = "read"
				f.update(cv, d, NoteDraft{Now: "read"})
			}
		case gone:
			n := live[len(live)-1]
			cv.follow(d, itemKey{n.msg.id, n.kind})
			f.update(cv, d, NoteDraft{Text: n.msg.text, HasText: true})
		}
		if d.shown == "" {
			keep = append(keep, d)
		}
	}
	rw.dues = keep
}

// releaseHeld is the only way out of the hold that sends something: a tick from
// the log ends every hold more than five seconds older than itself, by the
// log's timestamps. The note carries what is true at that moment.
func (e *Engine) releaseHeld(f *fx) {
	if f.noNotes {
		return
	}
	for _, cv := range e.allConvs() {
		if len(cv.holding) == 0 {
			continue
		}
		list := make([]*openDue, 0, len(cv.holding))
		for d := range cv.holding {
			list = append(list, d)
		}
		sort.Slice(list, func(i, j int) bool {
			if !list[i].at.Equal(list[j].at) {
				return list[i].at.Before(list[j].at)
			}
			return list[i].name < list[j].name
		})
		for _, d := range list {
			if !f.at.After(d.at.Add(holdFor)) {
				continue
			}
			delete(cv.holding, d)
			d.held = false
			r, p := cv.readers[d.person], e.people[d.person]
			var rw *row
			if r != nil {
				rw = r.rows[d.key]
			}
			if rw == nil || p == nil {
				dropDue(rw, d)
				continue
			}
			live := liveItems(p, rw)
			if len(live) == 0 && !fault(FaultReadKeepsHold) { // "Focus only" was switched on during the hold
				dropDue(rw, d)
				continue
			}
			src := rw.items[d.item]
			if len(live) > 0 && (src == nil || !notifying(p, src)) {
				src = live[len(live)-1]
			}
			if src == nil {
				dropDue(rw, d)
				continue
			}
			d.item = itemKey{src.msg.id, src.kind}
			cv.sent[d.item.msg] = append(cv.sent[d.item.msg], d)
			f.notes = append(f.notes, NoteDraft{Kind: "send", At: f.atText, Name: d.name, Conversation: cv.id,
				Row: contract.RowKey{Conversation: cv.id, Thread: d.key.thread()}, Person: d.person, Bucket: src.bucket.Contract(),
				Why: src.reason, Text: src.msg.text, DueAt: d.atText, History: f.history, Message: src.msg.id, Item: src.kind})
		}
	}
}

// releaseByArrival is the planted fault of G-011: a hold counted from when the
// change reached the feed, which a late feed makes meaningless.
func (e *Engine) releaseByArrival(f *fx) {
	now := arrivalNow()
	for _, cv := range e.allConvs() {
		for d := range cv.holding {
			if now.Sub(d.came) > holdFor {
				d.at = f.at.Add(-holdFor - 1)
			}
		}
	}
	e.releaseHeld(f)
}

func dropDue(rw *row, d *openDue) {
	if rw == nil {
		return
	}
	for i, x := range rw.dues {
		if x == d {
			rw.dues = append(rw.dues[:i:i], rw.dues[i+1:]...)
			return
		}
	}
}

// Fence removes what is on hold inside a fence: the whole conversation, or one
// reader in it when person is given. It writes no note (D-034). It answers the
// names removed, so the feed can log them.
func (e *Engine) Fence(conversation, person string) []string {
	cv := e.conv(conversation)
	if cv == nil {
		return nil
	}
	var names []string
	for d := range cv.holding {
		if person != "" && d.person != person {
			continue
		}
		delete(cv.holding, d)
		if r := cv.readers[d.person]; r != nil {
			dropDue(r.rows[d.key], d)
		}
		names = append(names, d.name)
	}
	sort.Strings(names)
	return names
}

// Held names what is on hold for a person, sorted.
func (e *Engine) Held(person string) []string {
	var names []string
	for _, cv := range e.allConvs() {
		for d := range cv.holding {
			if d.person == person {
				names = append(names, d.name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// settleConversation settles every touched row of this change.
func (e *Engine) settleConversation(cv *conv, f *fx) {
	for _, u := range sortedKeys(f.touched[cv.id]) {
		p, r := e.people[u], cv.readers[u]
		if p == nil || r == nil {
			continue
		}
		for _, k := range sortedRowKeys(f.touched[cv.id][u]) {
			if rw := r.rows[k]; rw != nil {
				e.settleDues(cv, p, rw, f)
				if len(rw.items) == 0 {
					delete(r.rows, k)
				}
			}
		}
	}
}

func sortedRowKeys(m map[rowKey]bool) []rowKey {
	out := make([]rowKey, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].kind != out[j].kind {
			return out[i].kind < out[j].kind
		}
		return out[i].id < out[j].id
	})
	return out
}

func sortedItems(r *row) []*item {
	out := make([]*item, 0, len(r.items))
	for _, it := range r.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].number != out[j].number {
			return out[i].number < out[j].number
		}
		if out[i].msg.id != out[j].msg.id {
			return out[i].msg.id < out[j].msg.id
		}
		return out[i].kind < out[j].kind
	})
	return out
}

func dropItem(r *convReader, rw *row, k itemKey) {
	delete(rw.items, k)
	delete(r.itemRow, k)
}

// retract is what is left when something aimed at a person is taken back
// (D-004 option C). A row the person kept is marked retracted, and the screen
// shows that only until its list is next laid out (D-039); something
// they had read keeps a marker where it was; something never seen goes without
// a trace. What is on screen right now is the screen's: it holds the space of
// a row that is removed until the person moves on.
func retract(r *convReader, rw *row, it *item) {
	if it.retracted {
		return
	}
	k := itemKey{it.msg.id, it.kind}
	switch {
	case rw.kept:
		it.retracted, it.read = true, true
		rw.hold = true
	case it.read:
		it.retracted = true
	default:
		dropItem(r, rw, k)
	}
	it.flag = ""
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func (e *Engine) messageChanged(c contract.Message, f *fx) error {
	cv := e.conv(c.Conversation)
	if cv == nil || cv.msgs[c.Id] == nil {
		return cannot("edit of unknown message %s", c.Id)
	}
	m := cv.msgs[c.Id]
	if m.deleted {
		return nil // a delete wins (D-006)
	}
	oldMentions, oldLevel, oldGroup := m.mentions, m.level, m.group
	oldRecipients := flagRecipients(cv, m)
	textChanged := m.text != c.Text
	setContent(m, c)
	m.version = c.Version
	m.edited = true
	textOnly := sameSet(oldMentions, m.mentions) && oldLevel == m.level && oldGroup == m.group

	for _, u := range e.readers {
		r := cv.readers[u]
		p := e.people[u]
		if textOnly {
			// D-004: an edit that only changes the text updates the row in place.
			if r == nil {
				continue
			}
			if k, ok := r.itemRow[itemKey{m.id, itemMsg}]; ok {
				rw := r.rows[k]
				if it := rw.items[itemKey{m.id, itemMsg}]; !it.retracted {
					it.edited = true
					f.touch(cv, u, k)
				}
			}
			continue
		}
		e.editFor(cv, p, m, oldMentions, oldLevel, oldRecipients, f)
		if cv.readers[u] != nil || m.mentions[u] {
			cv.reader(u)
			f.touchReader(cv, u) // a mention edited in or out changes its tab and the thread's level
		}
	}
	if textChanged && !f.noNotes {
		e.noteNewText(cv, m, f)
	}
	e.settleConversation(cv, f)
	touchShowing(cv, m, f) // a row showing it through a reaction shows the new text too
	return nil
}

func (e *Engine) editFor(cv *conv, p *person, m *message, oldMentions map[string]bool, oldLevel string, oldRecipients map[string]bool, f *fx) {
	u := p.id
	cl := e.classify(cv, view{p, cv.peek(u)}, m)
	if cl == nil && cv.readers[u] == nil {
		return
	}
	r := cv.reader(u)
	k := itemKey{m.id, itemMsg}
	var rw *row
	var it *item
	if rk, ok := r.itemRow[k]; ok {
		rw = r.rows[rk]
		it = rw.items[k]
		f.touch(cv, u, rk)
		if it.retracted {
			it = nil
		}
	}
	recipients := flagRecipients(cv, m)
	if rw != nil && !recipients[u] {
		delete(rw.flagged, m.id) // D-011: if the flag is removed, what it kept is released
	}
	newlyNamed := (m.mentions[u] && !oldMentions[u]) || (recipients[u] && !oldRecipients[u])
	asNew := newlyNamed || (oldLevel == "info" && m.level != "info") // D-004, D-013
	switch {
	case it == nil && cl != nil:
		c := *cl
		if rw != nil {
			dropItem(r, rw, k)
			if rw.kept {
				c.key = rw.key // a row you kept keeps its message when it is named again
			}
		}
		e.deliver(cv, p, r, m, c, itemMsg, f)
	case it != nil && cl == nil:
		retract(r, rw, it)
	case it != nil:
		it.edited, it.flag = true, cl.flag
		if cl.flag == "named" {
			rw.flagged[m.id] = true
		}
		if asNew {
			prev := unreadBucket(rw, it)
			it.read = false
			it.bucket, it.reason, it.decider, it.number = cl.bucket, cl.reason, cl.decider, f.number
			rw.last = f.number
			e.notify(cv, p, rw, it, prev, f) // R36, R12
		} else {
			it.bucket, it.reason, it.decider = cl.bucket, cl.reason, cl.decider // R13: moved quietly
		}
	}
}

// noteNewText gives every entry already sent about this message its new text
// (D-004). One still on hold needs nothing: it takes the text when released.
func (e *Engine) noteNewText(cv *conv, m *message, f *fx) {
	for _, d := range cv.sent[m.id] {
		f.update(cv, d, NoteDraft{Text: m.text, HasText: true})
	}
}

func (e *Engine) messageDeleted(c contract.DeletedMessage, f *fx) error {
	cv := e.conv(c.Conversation)
	if cv == nil || cv.msgs[c.Id] == nil {
		return cannot("delete of unknown message %s", c.Id)
	}
	m := cv.msgs[c.Id]
	if m.deleted {
		return nil
	}
	m.deleted, m.version = true, c.Version
	m.text, m.mentions, m.group, m.level, m.reactions = "", map[string]bool{}, false, "", nil
	for _, u := range sortedKeys(cv.readers) {
		r := cv.readers[u]
		for _, rk := range rowKeysOf(r) {
			rw := r.rows[rk]
			for _, it := range sortedItems(rw) {
				if it.msg == m {
					delete(rw.flagged, m.id) // D-011: deleting the message removes its flag too
					retract(r, rw, it)
					f.touch(cv, u, rk)
				}
			}
		}
		f.touchReader(cv, u)
	}
	e.settleConversation(cv, f)
	// Entries sent about it that did not move to another message say so.
	for _, d := range cv.sent[m.id] {
		d.shown = "deleted"
		f.update(cv, d, NoteDraft{Now: "deleted"})
	}
	delete(cv.sent, m.id)
	return nil
}

func rowKeysOf(r *convReader) []rowKey {
	out := make(map[rowKey]bool, len(r.rows))
	for k := range r.rows {
		out[k] = true
	}
	return sortedRowKeys(out)
}

func addressed(cv *conv, m *message, u string) bool {
	return m.mentions[u] || (cv.direct && cv.members[u] && u != m.author)
}

func (e *Engine) isAgent(id string) bool {
	p := e.people[id]
	return p != nil && p.agent
}

// refreshSplit keeps the grouped row's count equal to the reactions on its message.
func refreshSplit(cv *conv, m *message) {
	r := cv.readers[m.author]
	if r == nil || len(m.reactions) == 0 {
		return
	}
	k := itemKey{m.id, itemReaction}
	if rk, ok := r.itemRow[k]; ok {
		if it := r.rows[rk].items[k]; it != nil && !it.retracted {
			it.reason = splitLine(m.reactions)
		}
	}
}

// applause is whose reaction keeps a grouped row alive: everyone except the
// author and people whose reaction is an answer (D-014). What the row counts is
// every reaction on the message (T-038 point 18).
func (e *Engine) applause(cv *conv, m *message) []*reaction {
	var out []*reaction
	for _, x := range m.reactions {
		if x.person != m.author && !(addressed(cv, m, x.person) && !e.isAgent(x.person)) {
			out = append(out, x)
		}
	}
	return out
}

// splitLine is the grouped row's reason: the true count per emoji (D-014, T-038 point 18).
func splitLine(rs []*reaction) string {
	counts := map[string]int{}
	var order []string
	for _, x := range rs {
		if counts[x.emoji] == 0 {
			order = append(order, x.emoji)
		}
		counts[x.emoji]++
	}
	sort.Strings(order)
	parts := make([]string, len(order))
	for i, em := range order {
		parts[i] = fmt.Sprintf("%s %d", em, counts[em])
	}
	return strings.Join(parts, " · ") + " on your message"
}

func (e *Engine) reactionAdded(v contract.ChangeReactionAdded, f *fx) error {
	cv := e.conv(v.Conversation)
	if cv == nil || cv.msgs[v.Message] == nil {
		return cannot("reaction on unknown message %s", v.Message)
	}
	if e.people[v.Person] == nil {
		return cannot("reaction from unknown person %s", v.Person)
	}
	m := cv.msgs[v.Message]
	if m.deleted {
		return nil
	}
	for _, x := range m.reactions {
		if x.person == v.Person && x.emoji == v.Emoji {
			return nil
		}
	}
	m.reactions = append(m.reactions, &reaction{person: v.Person, emoji: v.Emoji, number: f.number, at: f.atText})
	defer touchShowing(cv, m, f)

	// The person reacting acted on their own row (D-014, D-011, D-023 point 26).
	if p := e.people[v.Person]; !p.agent {
		r := cv.reader(p.id)
		if rk, ok := r.itemRow[itemKey{m.id, itemMsg}]; ok {
			rw := r.rows[rk]
			for _, it := range rw.items {
				if it.msg == m {
					it.read = true
				}
			}
			if p.releaseOnAct {
				rw.kept, rw.hold = false, false // R42
				delete(rw.flagged, m.id)
			}
			f.touch(cv, p.id, rk)
		}
	}
	e.reactionToAuthor(cv, m, v.Person, v.Emoji, f)
	refreshSplit(cv, m)
	e.settleConversation(cv, f)
	return nil
}

// touchShowing marks every row that shows this message: a row carries its
// message's reactions (D-014, T-038 point 18), so a reaction changes the row of
// everyone who has it, not only the author's and the reactor's.
func touchShowing(cv *conv, m *message, f *fx) {
	for _, u := range sortedKeys(cv.readers) {
		r := cv.readers[u]
		for k, rk := range r.itemRow {
			if k.msg == m.id && r.rows[rk] != nil {
				f.touch(cv, u, rk)
			}
		}
	}
}

func (e *Engine) reactionToAuthor(cv *conv, m *message, reactor, emoji string, f *fx) {
	a := e.people[m.author]
	if m.author == reactor || a == nil || a.agent {
		return
	}
	r := cv.reader(a.id)
	f.touchReader(cv, a.id)
	key := rowKey{"th", m.root}
	if cv.direct {
		key = rowKey{"dm", cv.id}
	}
	if addressed(cv, m, reactor) && !e.isAgent(reactor) {
		// D-014: an answer lands in Needs you and names them; R35: also from a muted channel.
		if r.thDial[m.root] == levelOff {
			return
		}
		k := itemKey{m.id, answerPrefix + reactor}
		if rw := r.rows[key]; rw != nil {
			dropItem(r, rw, k)
		}
		reason := fmt.Sprintf("%s reacted %s to your message", e.displayName(reactor), emoji)
		e.deliver(cv, a, r, m, class{bucket: Needs, reason: reason, key: key, decider: byAnswer}, k.kind, f)
		return
	}
	// R18: reactions follow the thread: none when it is Off or the channel Muted.
	if (!cv.direct && channelDial(cv, view{a, r}) == "muted") || r.thDial[m.root] == levelOff {
		return
	}
	reason := splitLine(m.reactions)
	k := itemKey{m.id, itemReaction}
	if rw := r.rows[key]; rw != nil && rw.items[k] != nil {
		it := rw.items[k]
		it.reason = reason
		f.touch(cv, a.id, key)
		if watches(cv, r, m) {
			it.read, it.retracted = true, false
			return
		}
		prev := unreadBucket(rw, it)
		it.read, it.retracted, it.number = false, false, f.number // D-003: a row returns, unread
		rw.last = f.number
		e.notify(cv, a, rw, it, prev, f)
		return
	}
	e.deliver(cv, a, r, m, class{bucket: FYI, reason: reason, key: key, decider: byApplause}, itemReaction, f)
}

func (e *Engine) reactionRemoved(v contract.ChangeReactionRemoved, f *fx) error {
	cv := e.conv(v.Conversation)
	if cv == nil || cv.msgs[v.Message] == nil {
		return cannot("reaction on unknown message %s", v.Message)
	}
	m := cv.msgs[v.Message]
	found := false
	still := false // the same person's other emoji stay
	kept := m.reactions[:0]
	for _, x := range m.reactions {
		if x.person == v.Person && x.emoji == v.Emoji {
			found = true
			continue
		}
		if x.person == v.Person {
			still = true
		}
		kept = append(kept, x)
	}
	m.reactions = kept
	if !found {
		return nil
	}
	defer touchShowing(cv, m, f)
	refreshSplit(cv, m)
	a := e.people[m.author]
	if a == nil || a.agent || cv.readers[a.id] == nil {
		return nil
	}
	r := cv.readers[a.id]
	f.touchReader(cv, a.id)
	if !still {
		k := itemKey{m.id, answerPrefix + v.Person}
		if rk, ok := r.itemRow[k]; ok {
			rw := r.rows[rk]
			retract(r, rw, rw.items[k]) // D-004: taken back
			f.touch(cv, a.id, rk)
			e.settleConversation(cv, f)
			return nil
		}
	}
	k := itemKey{m.id, itemReaction}
	rk, ok := r.itemRow[k]
	if !ok {
		e.settleConversation(cv, f)
		return nil
	}
	rw := r.rows[rk]
	f.touch(cv, a.id, rk)
	if len(e.applause(cv, m)) == 0 {
		retract(r, rw, rw.items[k]) // D-004: when the last reaction goes, the row is taken back
	}
	e.settleConversation(cv, f)
	return nil
}

// readerRead moves a read position (D-015, contracts README "Read position").
// Opening a channel covers its top-level messages; opening a thread covers
// that thread; a direct conversation's row covers everything in it. An item
// that arrived after this read stays unread, whatever its message's number.
func (e *Engine) readerRead(v contract.ChangeReaderRead, f *fx) error {
	cv := e.conv(v.Conversation)
	if cv == nil {
		return cannot("read in unknown conversation %s", v.Conversation)
	}
	if e.people[v.Person] == nil {
		return cannot("read by unknown person %s", v.Person)
	}
	r := cv.reader(v.Person)
	if v.Thread == nil {
		if v.UpTo > r.posConv {
			r.posConv = v.UpTo
		}
	} else if v.UpTo > r.posThread[*v.Thread] {
		r.posThread[*v.Thread] = v.UpTo
	}
	f.touchReader(cv, v.Person)
	for _, rk := range rowKeysOf(r) {
		rw := r.rows[rk]
		for _, it := range rw.items {
			if it.read || it.msg.number > v.UpTo {
				continue
			}
			covered := it.msg.root == derefOr(v.Thread, "\x00")
			if v.Thread == nil {
				covered = cv.direct || it.msg.parent == ""
			}
			if covered {
				it.read = true
				f.touch(cv, v.Person, rk)
			}
		}
	}
	e.settleConversation(cv, f)
	return nil
}

func derefOr(s *string, other string) string {
	if s == nil {
		return other
	}
	return *s
}

func (e *Engine) watching(person, conversation string, thread *string, connection string, on bool) error {
	cv := e.conv(conversation)
	if cv == nil {
		return cannot("watching in unknown conversation %s", conversation)
	}
	if e.people[person] == nil {
		return cannot("watching by unknown person %s", person)
	}
	r := cv.reader(person)
	k := derefOr(thread, "")
	if on {
		if r.watching[k] == nil {
			r.watching[k] = map[string]bool{}
		}
		r.watching[k][connection] = true
		return nil
	}
	delete(r.watching[k], connection)
	return nil
}

// keyFor is the row a keep, done or dial names.
func keyFor(cv *conv, thread *string) rowKey {
	if thread != nil {
		return rowKey{"th", *thread}
	}
	if cv.direct {
		return rowKey{"dm", cv.id}
	}
	return rowKey{"ch", cv.id}
}

// keep keeps a row (D-003). A row can be kept from a history tab even if
// nothing of it is in the feed, so it is made when needed (D-016).
func (e *Engine) keep(person, conversation string, thread *string, f *fx) error {
	cv := e.conv(conversation)
	if cv == nil {
		return cannot("keep in unknown conversation %s", conversation)
	}
	if e.people[person] == nil {
		return cannot("keep by unknown person %s", person)
	}
	r := cv.reader(person)
	k := keyFor(cv, thread)
	rw := r.rows[k]
	if rw == nil {
		m := anchorMessage(cv, k)
		if m == nil {
			return nil
		}
		rw = &row{key: k, items: map[itemKey]*item{}, flagged: map[string]bool{}, last: f.number}
		ik := itemKey{m.id, itemMsg}
		if old, ok := r.itemRow[ik]; ok && old != k {
			ik = itemKey{m.id, "kept"} // one item, one place: the message already sits in another row
		}
		rw.items[ik] = &item{msg: m, kind: ik.kind, bucket: FYI, reason: "Kept by you", decider: "kept", number: f.number, read: true}
		r.itemRow[ik] = k
		r.rows[k] = rw
	}
	rw.kept = true
	for _, it := range rw.items {
		it.read = true // D-015: acting implies read
	}
	f.touch(cv, person, k)
	e.settleConversation(cv, f)
	return nil
}

func anchorMessage(cv *conv, k rowKey) *message {
	if k.kind == "th" {
		return cv.msgs[k.id]
	}
	for i := len(cv.order) - 1; i >= 0; i-- {
		if m := cv.order[i]; !m.deleted && (cv.direct || m.parent == "") {
			return m
		}
	}
	return nil
}

func (e *Engine) done(person, conversation string, thread *string, f *fx) error {
	cv := e.conv(conversation)
	if cv == nil {
		return cannot("done in unknown conversation %s", conversation)
	}
	if e.people[person] == nil {
		return cannot("done by unknown person %s", person)
	}
	r := cv.reader(person)
	k := keyFor(cv, thread)
	rw := r.rows[k]
	if rw == nil {
		return nil
	}
	rw.kept, rw.hold = false, false
	rw.flagged = map[string]bool{}
	for _, it := range rw.items {
		it.read = true
	}
	f.touch(cv, person, k)
	e.settleConversation(cv, f)
	return nil
}

// dial changes a channel's or a thread's level and re-sorts the reader's
// unread rows there (D-013 point 3).
func (e *Engine) dial(v contract.ChangeReaderDial, f *fx) error {
	cv := e.conv(v.Conversation)
	if cv == nil {
		return cannot("dial in unknown conversation %s", v.Conversation)
	}
	p := e.people[v.Person]
	if p == nil {
		return cannot("dial by unknown person %s", v.Person)
	}
	level := string(v.Level)
	valid := map[string]bool{"focus": true, "all": true, "default": true, "muted": true}
	if v.Thread != nil {
		valid = map[string]bool{levelFocus: true, levelNeeds: true, levelFYI: true, levelOff: true, levelAuto: true}
	}
	if !valid[level] {
		return cannot("level %q does not apply here", level)
	}
	r := cv.reader(v.Person)
	if v.Thread != nil {
		if level == levelAuto {
			delete(r.thDial, *v.Thread)
		} else {
			r.thDial[*v.Thread] = level
		}
	} else if !cv.direct {
		r.dial = level
	}
	f.touchReader(cv, p.id) // the level decides the Threads tab, also with no row
	e.resort(cv, p, r, f)
	e.settleConversation(cv, f)
	return nil
}

// resort applies D-013 point 3 in one conversation: unread rows move or leave;
// read ones stay. A row lifted by your own change sends nothing (R40).
func (e *Engine) resort(cv *conv, p *person, r *convReader, f *fx) {
	v := view{p, r}
	for _, rk := range rowKeysOf(r) {
		rw := r.rows[rk]
		f.touch(cv, p.id, rk)
		for _, it := range sortedItems(rw) {
			if it.read || it.retracted {
				continue
			}
			switch it.kind {
			case itemMsg:
				if cl := e.classify(cv, v, it.msg); cl != nil {
					it.bucket, it.reason, it.decider = cl.bucket, cl.reason, cl.decider
				} else {
					delete(rw.flagged, it.msg.id)
					dropItem(r, rw, itemKey{it.msg.id, it.kind})
				}
			case itemReaction:
				if (!cv.direct && channelDial(cv, v) == "muted") || r.thDial[it.msg.root] == levelOff {
					dropItem(r, rw, itemKey{it.msg.id, it.kind})
				}
			}
		}
	}
}

func (e *Engine) resortEverywhere(p *person, f *fx) {
	for _, cv := range e.allConvs() {
		if r := cv.readers[p.id]; r != nil {
			e.resort(cv, p, r, f)
			e.settleConversation(cv, f)
		}
		f.touchReader(cv, p.id)
	}
}

func (e *Engine) focusRule(v contract.ChangeReaderFocusRule, f *fx) error {
	p := e.people[v.Person]
	if p == nil {
		return cannot("rule of unknown person %s", v.Person)
	}
	if v.On {
		p.rules[v.Rule] = true
		delete(p.removedRules, v.Rule)
	} else {
		if p.rules[v.Rule] {
			p.removedRules[v.Rule] = true
		}
		delete(p.rules, v.Rule)
	}
	e.resortEverywhere(p, f)
	return nil
}

func (e *Engine) focusPerson(v contract.ChangeReaderFocusPerson, f *fx) error {
	p := e.people[v.Person]
	if p == nil {
		return cannot("Focus person of unknown person %s", v.Person)
	}
	if v.On {
		delete(p.removed, v.About)
		p.focusPeople[v.About] = true
	} else {
		p.removed[v.About] = true
		delete(p.focusPeople, v.About)
	}
	e.resortEverywhere(p, f)
	return nil
}

// flushRows turns the touched rows into feed parts for each reader.
func (e *Engine) flushRows(f *fx) map[string][]contract.FeedPart {
	parts := map[string][]contract.FeedPart{}
	convs := make([]string, 0, len(f.touched))
	for id := range f.touched {
		convs = append(convs, id)
	}
	sort.Strings(convs)
	for _, id := range convs {
		cv := e.conv(id)
		for _, u := range sortedKeys(f.touched[id]) {
			p, r := e.people[u], cv.readers[u]
			if p == nil || r == nil {
				continue
			}
			e.recountTabs(cv, p, r)
			for _, k := range sortedRowKeys(f.touched[id][u]) {
				rw := r.rows[k]
				_, had := r.emitted[k]
				if rw == nil {
					if had {
						delete(r.emitted, k)
						parts[u] = append(parts[u], contract.FeedPart{Value: contract.FeedPartRowRemoved{Kind: "row.removed", Row: contract.RowKey{Conversation: cv.id, Thread: k.thread()}}})
					}
					continue
				}
				visible := rowBucket(rw, p) != NoBucket
				if !visible && !had {
					continue
				}
				fr := e.render(cv, p, r, rw)
				text := fingerprint(fr)
				if r.emitted[k] != text {
					r.emitted[k] = text
					parts[u] = append(parts[u], contract.FeedPart{Value: contract.FeedPartRowChanged{Kind: "row.changed", Row: fr}})
				}
			}
		}
	}
	return parts
}
