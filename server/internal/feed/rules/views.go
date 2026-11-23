package rules

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"contexia/internal/shared/contract"
)

// rowBucket is the bucket a row sits in, or NoBucket once it has left the feed.
func rowBucket(r *row, p *person) Bucket {
	if r.kept || r.hold || flagHolds(r, p) {
		return Focus
	}
	var best Bucket
	for _, it := range r.items {
		if !it.read && !it.retracted && it.bucket > best {
			best = it.bucket
		}
	}
	return best
}

// flagHolds says whether a flag keeps the row in Focus now. Once read it does,
// also without the rule "Flags that name you" (R37). While unread and with that
// rule removed the row sits in Needs you (D-013 point 3, T-038 point 1).
func flagHolds(r *row, p *person) bool {
	for f := range r.flagged {
		if it := r.items[itemKey{f, itemMsg}]; it == nil || it.read || p.rules[flagsRule] {
			return true
		}
	}
	return false
}

// rowSection is "new" or "kept" for a row in Focus (D-003, D-011, R16).
func rowSection(r *row) string {
	flagRead := len(r.flagged) > 0
	for f := range r.flagged {
		if it := r.items[itemKey{f, itemMsg}]; it != nil && !it.read {
			flagRead = false
		}
	}
	if r.kept || flagRead {
		return "kept"
	}
	for _, it := range r.items {
		if !it.read && !it.retracted {
			return "new"
		}
	}
	return "kept"
}

func keptBy(r *row, p *person) *contract.KeptBy {
	var k contract.KeptBy
	flag := flagHolds(r, p)
	switch {
	case r.kept && flag:
		k = "both"
	case r.kept || r.hold:
		k = "you"
	case flag:
		k = "flag"
	default:
		return nil
	}
	return &k
}

// lead is the item a row shows: its newest unread one, else its newest.
func lead(r *row) *item {
	var best, newest *item
	for _, it := range sortedItems(r) {
		newest = it
		if !it.read && !it.retracted {
			best = it
		}
	}
	if best != nil {
		return best
	}
	return newest
}

// decidingItem is the newest unread item in the row's bucket: what put the row
// there. It is nil when nothing unread reaches it and the row is only held there.
func decidingItem(r *row, b Bucket) *item {
	var found *item
	for _, it := range sortedItems(r) {
		if !it.read && !it.retracted && it.bucket == b {
			found = it
		}
	}
	return found
}

// shownAsKept says the row's place comes from being kept or flagged, not from an item.
func shownAsKept(r *row, p *person, b Bucket) bool {
	return keptBy(r, p) != nil && b == Focus && (rowSection(r) == "kept" || decidingItem(r, b) == nil)
}

func reactionsOf(m *message) []contract.Reaction {
	out := []contract.Reaction{}
	byEmoji := map[string][]string{}
	for _, x := range m.reactions {
		byEmoji[x.emoji] = append(byEmoji[x.emoji], x.person)
	}
	for _, em := range sortedKeys(byEmoji) {
		people := byEmoji[em]
		sort.Strings(people)
		out = append(out, contract.Reaction{Emoji: em, People: people})
	}
	return out
}

// render gives the row as the contract shows it.
func (e *Engine) render(cv *conv, p *person, r *convReader, rw *row) contract.FeedRow {
	it := lead(rw)
	bucket := rowBucket(rw, p)
	shown := bucket
	if shown == NoBucket {
		shown = it.bucket
	}
	var unread, newest int64
	var firstNew *int64
	for _, x := range sortedItems(rw) {
		if x.number > newest {
			newest = x.number
		}
		if !x.read && !x.retracted {
			unread++
			if firstNew == nil || x.msg.number < *firstNew {
				n := x.msg.number
				firstNew = &n
			}
		}
	}
	state := contract.RowState("none")
	switch {
	case it.retracted:
		state = "retracted"
	case it.edited || it.msg.edited:
		state = "changed"
	}
	// The row shows its newest item, and says why by the item that placed it (D-014, D-020).
	decided := it
	if d := decidingItem(rw, bucket); d != nil {
		decided = d
	}
	why := decided.reason
	k := keptBy(rw, p)
	if shownAsKept(rw, p, bucket) {
		why = "Kept by you"
		if *k == "flag" {
			why = "Flagged by " + e.flagAuthor(cv, rw)
		}
	}
	return contract.FeedRow{
		Conversation: cv.id, Thread: rw.key.thread(), Message: it.msg.id, Bucket: shown.Contract(),
		Unread: unread > 0, Kept: k, Why: why, Newest: newest, FirstNew: firstNew, Author: it.msg.author,
		Preview: it.msg.text, At: it.msg.at, NewItems: unread, Reactions: reactionsOf(it.msg), State: state,
		Explanation: e.explain(cv, p, r, rw, decided, bucket),
	}
}

func (e *Engine) flagAuthor(cv *conv, r *row) string {
	for _, f := range sortedKeys(r.flagged) {
		if m := cv.msgs[f]; m != nil {
			return e.displayName(m.author)
		}
	}
	return "someone"
}

func fingerprint(fr contract.FeedRow) string {
	b, _ := json.Marshal(fr)
	return string(b)
}

func dialAct(cv *conv, thread *string, level string) contract.RowAct {
	return contract.RowAct{Value: contract.RowActDial{Kind: "dial", Conversation: cv.id, Thread: thread, Level: contract.DialLevel(level)}}
}

func ruleAct(rule contract.FocusRule, on bool) contract.RowAct {
	return contract.RowAct{Value: contract.RowActFocusRule{Kind: "focus_rule", Rule: rule, On: on}}
}

func personAct(about string, on bool) contract.RowAct {
	return contract.RowAct{Value: contract.RowActFocusPerson{Kind: "focus_person", About: about, On: on}}
}

// explain gives the row's three plain sentences and its one change (D-020).
// The change acts on the narrowest thing that decided the row's place.
func (e *Engine) explain(cv *conv, p *person, r *convReader, rw *row, it *item, bucket Bucket) contract.RowExplanation {
	who := e.displayName(it.msg.author)
	thread := rw.key.thread()
	personRule := contract.FocusRule{Type: "person", Value: it.msg.author}
	mk := func(what, decided, otherwise, label string, do, undo contract.RowAct) contract.RowExplanation {
		return contract.RowExplanation{What: what, DecidedBy: decided, Otherwise: otherwise,
			Change: &contract.RowChange{Label: label, Do: do, Undo: undo}}
	}
	rowDone := contract.RowAct{Value: contract.RowActDone{Kind: "done", Conversation: cv.id, Thread: thread}}
	rowKeep := contract.RowAct{Value: contract.RowActKeep{Kind: "keep", Conversation: cv.id, Thread: thread}}
	if shownAsKept(rw, p, bucket) {
		if *keptBy(rw, p) == "flag" {
			return mk("A message "+e.flagAuthor(cv, rw)+" flagged for you.", "A flag that names you is kept for you until you press Done or respond.",
				"Done releases it; it then leaves the feed.", "Mark done", rowDone, rowKeep)
		}
		return mk("A row you kept.", "You kept it, so it stays in Focus until you mark it done.",
			"Done releases it; it then leaves the feed.", "Mark done", rowDone, rowKeep)
	}
	moveIn := func(what, decided string) contract.RowExplanation {
		return mk(what, decided, "With "+who+" in your Focus it would land in Focus.", "Move what "+who+" sends into Focus",
			ruleAct(personRule, true), ruleAct(personRule, false))
	}
	// A thread level needs a thread: a direct conversation's row has none of its
	// own, so the change names the thread of the message it shows.
	ownThread := thread
	if ownThread == nil {
		root := it.msg.root
		ownThread = &root
	}
	unfollow := func(what, decided string) contract.RowExplanation {
		return mk(what, decided, "With the thread off, only what names you would reach you.", "Turn this thread off",
			dialAct(cv, ownThread, levelOff), dialAct(cv, ownThread, levelAuto))
	}
	switch it.decider {
	case byDMPerson:
		return mk("A direct message from "+who+".", who+" is in your Focus people.", "Without them it would land in Needs you.",
			"Remove "+who+" from Focus", personAct(it.msg.author, false), personAct(it.msg.author, true))
	case byDMOther, byDMAgent:
		return mk("A direct message from "+who+".", "A direct message from someone outside your Focus people lands in Needs you.",
			"With "+who+" in Focus it would land in Focus.", "Add "+who+" to Focus", personAct(it.msg.author, true), personAct(it.msg.author, false))
	case byConvRule:
		rule := contract.FocusRule{Type: "conversation", Value: cv.id}
		return mk("A message in "+cv.name+".", "You have a Focus rule for this conversation.", "Without the rule it would land by the usual rules.",
			"Remove this Focus rule", ruleAct(rule, false), ruleAct(rule, true))
	case byFlagRule:
		return mk("A message "+who+" flagged for you.", "Your rule “Flags that name you” puts it in Focus.", "Without the rule it would land in Needs you.",
			"Remove the rule “Flags that name you”", ruleAct(flagsRule, false), ruleAct(flagsRule, true))
	case byFlagNoRule:
		return mk("A message "+who+" flagged for you.", "A flag that names you lands in Needs you while the rule is off.", "With the rule “Flags that name you” it would land in Focus.",
			"Add the rule “Flags that name you”", ruleAct(flagsRule, true), ruleAct(flagsRule, false))
	case byFlagOpen:
		return moveIn("A flagged message for everyone here.", "A flag that names nobody lands in Needs you.")
	case byMentionRule:
		return mk("A message from "+who+" that mentions you.", "Your rule “Any direct mention” puts it in Focus.", "Without the rule it would land in Needs you.",
			"Remove the rule “Any direct mention”", ruleAct(mentionsRule, false), ruleAct(mentionsRule, true))
	case byMention:
		return moveIn("A message from "+who+" that mentions you.", "A mention lands in Needs you.")
	case byPersonRule:
		return mk("A message from "+who+".", who+" is in your Focus.", "Without the rule it would land lower.",
			"Stop moving "+who+" into Focus", ruleAct(personRule, false), ruleAct(personRule, true))
	case byChannelFocus:
		return mk("A message in "+cv.name+".", cv.name+" is set to Focus.", "At Default only what addresses you would reach you.",
			"Set "+cv.name+" to Default", dialAct(cv, nil, "default"), dialAct(cv, nil, "focus"))
	case byChannelAll:
		return mk("A post in "+cv.name+".", cv.name+" is set to All activity.", "At Default this post would not reach you.",
			"Set "+cv.name+" to Default", dialAct(cv, nil, "default"), dialAct(cv, nil, "all"))
	case byGroup:
		return mk("A mention of everyone in "+cv.name+".", "A mention of the whole channel lands in For your information.", "With the channel muted it would not reach you.",
			"Mute "+cv.name, dialAct(cv, nil, "muted"), dialAct(cv, nil, channelDial(cv, view{p, r})))
	case byThreadFocus:
		return mk("A reply in a thread you put in Focus.", "The thread's own level is Focus.", "At its usual level it would land lower.",
			"Take this thread out of Focus", dialAct(cv, ownThread, levelAuto), dialAct(cv, ownThread, levelFocus))
	case byThreadNeeds:
		return unfollow("A reply in a thread you are part of.", "A reply in a thread you started or replied in lands in Needs you.")
	case byThreadFYI:
		return unfollow("A reply in a thread you follow.", "A reply in a thread you follow or were mentioned in lands in For your information.")
	case byAnswer:
		return unfollow("A reaction from someone you addressed.", "A reaction from someone you addressed is an answer, so it lands in Needs you.")
	case byApplause:
		return unfollow("Reactions to your message.", "Reactions to your message land in For your information.")
	case byStopped:
		return unfollow("An agent's reply that stopped halfway.", "A stopped reply is For your information and never notifies.")
	case byInfo:
		return contract.RowExplanation{What: "A message from " + who + " marked Info.", DecidedBy: "The sender marked it Info, so it lands no higher than For your information.",
			Otherwise: "Without Info it would land by the usual rules."}
	}
	return contract.RowExplanation{What: "A message from " + who + ".", DecidedBy: it.reason, Otherwise: "It would otherwise not be in your feed."}
}

// Rows is the reader's feed: what sits in a bucket now, newest activity first
// within a section (D-003). A read row is not in it: it is found in its history
// tab; a screen that shows it keeps it, dimmed, until the person moves on.
func (e *Engine) Rows(person string) []contract.FeedRow {
	list := e.KeyedRows(person)
	if list == nil {
		return nil
	}
	out := make([]contract.FeedRow, len(list))
	for i, x := range list {
		out[i] = x.Row
	}
	return out
}

// HeldNow is one notification on hold, for the store that keeps it with its row.
type HeldNow struct {
	Name, Person, Conversation, Message, Item, At string
}

// AllHeld lists everything on hold in the organization, sorted by name.
func (e *Engine) AllHeld() []HeldNow {
	var out []HeldNow
	for _, cv := range e.allConvs() {
		for d := range cv.holding {
			out = append(out, HeldNow{d.name, d.person, cv.id, d.item.msg, d.item.kind, d.atText})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// KeyedRow is a feed row with what orders it, for a store that keeps rows one
// by one: section, then newest activity first, then key.
type KeyedRow struct {
	Key     string
	Section int
	Last    int64
	Row     contract.FeedRow
}

// KeyedRows is Rows with each row's key and sort position.
func (e *Engine) KeyedRows(person string) []KeyedRow {
	p := e.people[person]
	if p == nil {
		return nil
	}
	list := []KeyedRow{}
	for _, cv := range e.allConvs() {
		r := cv.readers[person]
		if r == nil {
			continue
		}
		for _, k := range rowKeysOf(r) {
			rw := r.rows[k]
			b := rowBucket(rw, p)
			if b == NoBucket {
				continue
			}
			section := 3 - int(b)
			if b == Focus && rowSection(rw) == "kept" {
				section = 1 // Focus: new on top, Kept below
			} else if b != Focus {
				section++
			}
			list = append(list, KeyedRow{cv.id + "/" + k.kind + "/" + k.id, section, rw.last, e.render(cv, p, r, rw)})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Section != list[j].Section {
			return list[i].Section < list[j].Section
		}
		if list[i].Last != list[j].Last {
			return list[i].Last > list[j].Last
		}
		return list[i].Key < list[j].Key
	})
	return list
}

// Counts is the feed's counts: unread rows in Focus and Needs you, a dot for
// For your information, and the size of Kept (D-003).
func (e *Engine) Counts(person string) contract.FeedCounts {
	var c contract.FeedCounts
	p := e.people[person]
	if p == nil {
		return c
	}
	for _, cv := range e.allConvs() {
		r := cv.readers[person]
		if r == nil {
			continue
		}
		for _, rw := range r.rows {
			switch b := rowBucket(rw, p); {
			case b == Focus && rowSection(rw) == "kept":
				c.Kept++
			case b == Focus:
				c.Focus++
			case b == Needs:
				c.NeedsYou++
			case b == FYI:
				c.Fyi = true
			}
		}
	}
	return c
}

// Summaries gives the counts and history parts for a reader, where they
// changed since the last call.
func (e *Engine) Summaries(person string) []contract.FeedPart {
	p := e.people[person]
	if p == nil || p.agent {
		return nil
	}
	var out []contract.FeedPart
	c := e.Counts(person)
	if p.lastCounts == nil || *p.lastCounts != c {
		p.lastCounts = &c
		out = append(out, contract.FeedPart{Value: contract.FeedPartCounts{Kind: "counts", Counts: c}})
	}
	totals := e.TabCounts(person)
	for _, tab := range historyTabs {
		if totals[tab] != p.lastTabs[tab] {
			p.lastTabs[tab] = totals[tab]
			out = append(out, contract.FeedPart{Value: contract.FeedPartHistoryChanged{Kind: "history.changed", Tab: tab, Unread: totals[tab]}})
		}
	}
	return out
}

var historyTabs = []contract.HistoryTab{"direct", "mentions", "threads", "reactions"}

func (e *Engine) recountTabs(cv *conv, p *person, r *convReader) {
	for _, tab := range historyTabs {
		var n int64
		walkHistory(cv, p, r, tab, func(x historyEntry) {
			if x.unread {
				n++
			}
		})
		r.tabs[tab] = n
	}
}

// TabCounts is the unread count of each history tab, as the feed keeps it.
func (e *Engine) TabCounts(person string) map[contract.HistoryTab]int64 {
	out := map[contract.HistoryTab]int64{}
	for _, tab := range historyTabs {
		out[tab] = 0
	}
	for _, cv := range e.allConvs() {
		if r := cv.readers[person]; r != nil {
			for tab, n := range r.tabs {
				out[tab] += n
			}
		}
	}
	return out
}

func positionCovers(cv *conv, r *convReader, m *message, number int64) bool {
	if cv.direct {
		return number <= r.posConv || number <= r.posThread[m.root]
	}
	if m.parent != "" {
		return number <= r.posThread[m.root]
	}
	return number <= r.posConv || number <= r.posThread[m.id]
}

func entryUnread(cv *conv, r *convReader, m *message, kind string, number int64) bool {
	if rk, ok := r.itemRow[itemKey{m.id, kind}]; ok {
		if rw := r.rows[rk]; rw != nil {
			if it := rw.items[itemKey{m.id, kind}]; it != nil {
				return !it.read && !it.retracted
			}
		}
	}
	return !positionCovers(cv, r, m, number)
}

func keptOf(r *convReader, p *person, m *message) *contract.KeptBy {
	if rk, ok := r.itemRow[itemKey{m.id, itemMsg}]; ok {
		if rw := r.rows[rk]; rw != nil {
			return keptBy(rw, p)
		}
	}
	if rw := r.rows[rowKey{"th", m.root}]; rw != nil {
		return keptBy(rw, p)
	}
	return nil
}

func historyThread(cv *conv, m *message) *string {
	if cv.direct {
		return nil
	}
	root := m.root
	return &root
}

// historyEntry is one entry of a tab before it is given its contract form.
type historyEntry struct {
	m         *message
	number    int64
	at        string
	unread    bool
	by, emoji *string
	level     *contract.DialLevel
}

// walkHistory calls emit for each of a tab's entries in one conversation: the
// current truth, read and unread together, not sorted by buckets or dials
// (D-016). One pass over the conversation, so that keeping a tab's count
// costs no more than the conversation's length.
func walkHistory(cv *conv, p *person, r *convReader, tab contract.HistoryTab, emit func(historyEntry)) {
	u := p.id
	if cv.direct && !cv.members[u] {
		return
	}
	type part struct{ authored, mentioned bool }
	threads := map[string]*part{}
	for _, m := range cv.order {
		st := threads[m.root]
		if st == nil {
			st = &part{}
			threads[m.root] = st
		}
		auto := levelOff // the thread's automatic level before this message
		switch {
		case st.authored:
			auto = levelNeeds
		case st.mentioned:
			auto = levelFYI
		}
		if m.author == u {
			st.authored = true
		}
		if m.mentions[u] {
			st.mentioned = true
		}
		if m.deleted {
			continue
		}
		if tab == "reactions" {
			if m.author != u {
				continue
			}
			for _, x := range m.reactions {
				if x.person == u {
					continue
				}
				kind := itemReaction
				if _, ok := r.itemRow[itemKey{m.id, answerPrefix + x.person}]; ok {
					kind = answerPrefix + x.person
				}
				by, emoji := x.person, x.emoji
				emit(historyEntry{m: m, number: x.number, at: x.at, unread: entryUnread(cv, r, m, kind, x.number), by: &by, emoji: &emoji})
			}
			continue
		}
		if m.author == u {
			continue
		}
		en := historyEntry{m: m, number: m.number, at: m.at}
		switch tab {
		case "mentions":
			if !m.mentions[u] {
				continue
			}
		case "direct":
			if !cv.direct {
				continue
			}
		case "threads":
			if m.parent == "" {
				continue
			}
			l, ok := r.thDial[m.root]
			if !ok {
				l = auto
			}
			if l == levelOff {
				continue
			}
			level := contract.DialLevel(l)
			en.level = &level
		}
		en.unread = entryUnread(cv, r, m, itemMsg, m.number)
		emit(en)
	}
}

func (e *Engine) historyIn(cv *conv, p *person, r *convReader, tab contract.HistoryTab) []contract.HistoryEntry {
	var out []contract.HistoryEntry
	walkHistory(cv, p, r, tab, func(x historyEntry) {
		out = append(out, contract.HistoryEntry{Message: x.m.id, Conversation: cv.id, Thread: historyThread(cv, x.m), Author: x.m.author,
			Preview: x.m.text, At: x.at, Number: x.number, Unread: x.unread, By: x.by, Emoji: x.emoji, Level: x.level, Kept: keptOf(r, p, x.m)})
	})
	return out
}

// History answers one tab, newest first.
func (e *Engine) History(person string, tab contract.HistoryTab) contract.HistoryAnswer {
	ans := contract.HistoryAnswer{Tab: tab, Entries: []contract.HistoryEntry{}}
	p := e.people[person]
	if p == nil {
		return ans
	}
	for _, cv := range e.allConvs() {
		ans.Entries = append(ans.Entries, e.historyIn(cv, p, cv.peek(person), tab)...)
	}
	sort.SliceStable(ans.Entries, func(i, j int) bool { return ans.Entries[i].Number > ans.Entries[j].Number })
	for _, en := range ans.Entries {
		if en.Unread {
			ans.Unread++
		}
	}
	return ans
}

// Settings answers what is in effect for the reader (D-021, D-023).
func (e *Engine) Settings(person string) contract.SettingsAnswer {
	out := contract.SettingsAnswer{FocusPeople: []contract.FocusPerson{}, RemovedPeople: []string{}, Rules: []contract.FocusRule{}, RemovedRules: []contract.FocusRule{}, Levels: []contract.LevelEntry{}}
	p := e.people[person]
	if p == nil {
		return out
	}
	out.AutoAdd, out.FocusOnly, out.ReleaseOnAct = p.autoAdd, p.focusOnly, p.releaseOnAct
	out.RemovedPeople = append(out.RemovedPeople, sortedKeys(p.removed)...)
	out.Rules = sortedRules(p.rules)
	out.RemovedRules = sortedRules(p.removedRules)
	people := map[string]bool{}
	for w := range p.focusPeople {
		people[w] = true
	}
	var lastWeek *time.Time
	for _, cv := range e.allConvs() {
		r := cv.readers[person]
		if r == nil {
			continue
		}
		if cv.direct && r.autoFocus {
			for w := range cv.members {
				if w != person && !p.removed[w] {
					people[w] = true
				}
			}
		}
		for _, m := range cv.order {
			if t, err := time.Parse(time.RFC3339Nano, m.at); err == nil && (lastWeek == nil || t.After(*lastWeek)) {
				lastWeek = &t
			}
		}
		if r.dial != "" && !cv.direct {
			out.Levels = append(out.Levels, contract.LevelEntry{Conversation: cv.id, Level: contract.DialLevel(r.dial)})
		}
		for _, root := range sortedKeys(r.thDial) {
			t := root
			out.Levels = append(out.Levels, contract.LevelEntry{Conversation: cv.id, Thread: &t, Level: contract.DialLevel(r.thDial[root])})
		}
	}
	for _, w := range sortedKeys(people) {
		n := int64(0)
		if lastWeek != nil {
			n = e.itemsFrom(person, w, lastWeek.Add(-7*24*time.Hour))
		}
		out.FocusPeople = append(out.FocusPeople, contract.FocusPerson{Person: w, ItemsLast7Days: n})
	}
	return out
}

func (e *Engine) itemsFrom(reader, who string, since time.Time) int64 {
	var n int64
	for _, cv := range e.allConvs() {
		if !cv.direct || !cv.members[reader] || !cv.members[who] {
			continue
		}
		for _, m := range cv.order {
			if t, err := time.Parse(time.RFC3339Nano, m.at); err == nil && m.author == who && !m.deleted && !t.Before(since) {
				n++
			}
		}
	}
	return n
}

func sortedRules(m map[contract.FocusRule]bool) []contract.FocusRule {
	out := make([]contract.FocusRule, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].Value < out[j].Value
	})
	return out
}

// Preview says what a change would have moved in the seven days before now
// (D-001, D-020). It changes nothing; now is given by the caller.
func (e *Engine) Preview(person string, act contract.RowAct, now time.Time) (contract.PreviewAnswer, error) {
	p := e.people[person]
	if p == nil {
		return contract.PreviewAnswer{}, fmt.Errorf("rules: unknown person %s", person)
	}
	since := now.Add(-7 * 24 * time.Hour)
	hypo := *p
	hypo.rules, hypo.focusPeople, hypo.removed = copySet(p.rules), copySet(p.focusPeople), copySet(p.removed)
	dialConv, dialThread, dialLevel := "", "", ""
	switch a := act.Value.(type) {
	case contract.RowActFocusRule:
		if a.On {
			hypo.rules[a.Rule] = true
		} else {
			delete(hypo.rules, a.Rule)
		}
	case contract.RowActFocusPerson:
		if a.On {
			delete(hypo.removed, a.About)
			hypo.focusPeople[a.About] = true
		} else {
			hypo.removed[a.About] = true
			delete(hypo.focusPeople, a.About)
		}
	case contract.RowActDial:
		dialConv, dialLevel = a.Conversation, string(a.Level)
		dialThread = derefOr(a.Thread, "")
	default:
		return contract.PreviewAnswer{Last7Days: contract.PreviewCounts{}}, nil
	}
	var moved, stay int64
	from, to := map[contract.Bucket]int64{}, map[contract.Bucket]int64{}
	for _, cv := range e.allConvs() {
		base := cv.peek(person)
		after := base
		if cv.id == dialConv {
			c := *base
			c.thDial = map[string]string{}
			for k, l := range base.thDial {
				c.thDial[k] = l
			}
			if dialThread == "" {
				c.dial = dialLevel
			} else if dialLevel == levelAuto {
				delete(c.thDial, dialThread)
			} else {
				c.thDial[dialThread] = dialLevel
			}
			after = &c
		}
		for _, m := range cv.order {
			t, err := time.Parse(time.RFC3339Nano, m.at)
			if err != nil || t.Before(since) || t.After(now) {
				continue
			}
			a, b := e.classify(cv, view{p, base}, m), e.classify(cv, view{&hypo, after}, m)
			if a == nil && b == nil {
				continue
			}
			if a != nil && b != nil && a.bucket == b.bucket {
				stay++
				continue
			}
			moved++
			if a != nil {
				from[a.bucket.Contract()]++
			}
			if b != nil {
				to[b.bucket.Contract()]++
			}
		}
	}
	return contract.PreviewAnswer{Last7Days: contract.PreviewCounts{Moved: moved, From: mostOf(from), To: mostOf(to), Stay: stay}}, nil
}

func copySet[K comparable](m map[K]bool) map[K]bool {
	out := make(map[K]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func mostOf(m map[contract.Bucket]int64) *contract.Bucket {
	var best *contract.Bucket
	var n int64
	for _, b := range []contract.Bucket{"focus", "needs_you", "fyi"} {
		if m[b] > n {
			bb := b
			best, n = &bb, m[b]
		}
	}
	return best
}

// StateItem, StateRow and ReaderState are the whole state of a reader's rows,
// for comparing two runs (G-030) and for the E-001 replay.
type StateItem struct {
	Msg       string `json:"msg"`
	Kind      string `json:"kind"`
	Bucket    int    `json:"bucket"`
	Read      bool   `json:"read"`
	Retracted bool   `json:"retracted"`
}

// StateRow is one row's whole state.
type StateRow struct {
	Key     string      `json:"key"`
	Where   string      `json:"where"`
	Kept    bool        `json:"kept"`
	Hold    bool        `json:"hold"`
	Flagged []string    `json:"flagged"`
	Items   []StateItem `json:"items"`
}

// ReaderState is everything a reader has: rows, counts, tabs and settings.
type ReaderState struct {
	Rows        []StateRow                                     `json:"rows"`
	ThreadDials map[string]string                              `json:"thread_dials"`
	FocusPeople []string                                       `json:"focus_people"`
	Counts      contract.FeedCounts                            `json:"counts"`
	History     map[contract.HistoryTab]contract.HistoryAnswer `json:"history"`
	Settings    contract.SettingsAnswer                        `json:"settings"`
	Feed        []contract.FeedRow                             `json:"feed"`
	// Held is what is on hold for this reader, and what each entry speaks of:
	// held entries are rebuilt from the log like everything else (D-034).
	Held []HeldEntry `json:"held"`
}

// HeldEntry is one notification on hold.
type HeldEntry struct {
	Name string `json:"name"`
	Msg  string `json:"msg"`
	Item string `json:"item"`
	At   string `json:"at"`
}

func keyString(cv *conv, k rowKey) string {
	if k.kind == "th" {
		return "th:" + k.id
	}
	return k.kind + ":" + cv.id
}

// State gives the reader's whole state in a form that compares by value.
func (e *Engine) State(person string) ReaderState {
	out := ReaderState{Rows: []StateRow{}, ThreadDials: map[string]string{}, FocusPeople: []string{},
		Counts: e.Counts(person), History: map[contract.HistoryTab]contract.HistoryAnswer{},
		Settings: e.Settings(person), Feed: e.Rows(person), Held: []HeldEntry{}}
	for _, tab := range historyTabs {
		out.History[tab] = e.History(person, tab)
	}
	for _, fp := range out.Settings.FocusPeople {
		out.FocusPeople = append(out.FocusPeople, fp.Person)
	}
	for _, cv := range e.allConvs() {
		for d := range cv.holding {
			if d.person == person {
				out.Held = append(out.Held, HeldEntry{Name: d.name, Msg: d.item.msg, Item: d.item.kind, At: d.atText})
			}
		}
	}
	sort.Slice(out.Held, func(i, j int) bool { return out.Held[i].Name < out.Held[j].Name })
	for _, cv := range e.allConvs() {
		r := cv.readers[person]
		if r == nil {
			continue
		}
		for root, l := range r.thDial {
			out.ThreadDials[root] = l
		}
		for _, k := range rowKeysOf(r) {
			rw := r.rows[k]
			sr := StateRow{Key: keyString(cv, k), Kept: rw.kept, Hold: rw.hold, Flagged: sortedKeys(rw.flagged), Items: []StateItem{}}
			switch b := rowBucket(rw, e.people[person]); b {
			case NoBucket:
				sr.Where = "Earlier"
			case Focus:
				sr.Where = "Focus/" + rowSection(rw)
			case Needs:
				sr.Where = "Needs you"
			default:
				sr.Where = "FYI"
			}
			for _, it := range sortedItems(rw) {
				sr.Items = append(sr.Items, StateItem{it.msg.id, it.kind, int(it.bucket), it.read, it.retracted})
			}
			sort.Slice(sr.Items, func(i, j int) bool {
				a, b := sr.Items[i], sr.Items[j]
				if a.Msg != b.Msg {
					return msgLess(a.Msg, b.Msg)
				}
				return a.Kind < b.Kind
			})
			out.Rows = append(out.Rows, sr)
		}
	}
	sort.Slice(out.Rows, func(i, j int) bool { return out.Rows[i].Key < out.Rows[j].Key })
	return out
}

// msgLess orders IDs of the same length-prefixed form numerically.
func msgLess(a, b string) bool {
	if len(a) != len(b) && strings.HasPrefix(a, "m") && strings.HasPrefix(b, "m") {
		return len(a) < len(b)
	}
	return a < b
}

// Readers lists the people with a feed, sorted.
func (e *Engine) Readers() []string {
	return append([]string(nil), e.readers...)
}
