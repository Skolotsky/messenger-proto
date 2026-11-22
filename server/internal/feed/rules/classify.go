package rules

import (
	"fmt"
	"sort"

	"contexia/internal/shared/contract"
)

// view is one reader looking at one conversation. A question can pass a
// changed copy to ask "what if" without touching anything (D-020).
type view struct {
	p *person
	r *convReader
}

// What decided a row's place; the explanation is built from it (D-020).
const (
	byDMPerson     = "dm_person"
	byDMOther      = "dm_other"
	byDMAgent      = "dm_agent"
	byConvRule     = "conversation_rule"
	byFlagRule     = "flag_rule"
	byFlagNoRule   = "flag_no_rule"
	byFlagOpen     = "flag_open"
	byMention      = "mention"
	byMentionRule  = "mention_rule"
	byGroup        = "group"
	byThreadFocus  = "thread_focus"
	byThreadNeeds  = "thread_needs"
	byThreadFYI    = "thread_fyi"
	byChannelFocus = "channel_focus"
	byChannelAll   = "channel_all"
	byPersonRule   = "person_rule"
	byInfo         = "info"
	byAnswer       = "answer"
	byApplause     = "applause"
	byStopped      = "stopped"
)

// class is where a message lands for a reader.
type class struct {
	bucket  Bucket
	reason  string
	key     rowKey
	flag    string // "named" or "open" when a flag is why it is there
	decider string
}

// channelDial is the level in effect: a "this channel" Focus rule is the same
// thing as the channel's dial at Focus (D-001).
func channelDial(cv *conv, v view) string {
	if v.p.rules[contract.FocusRule{Type: "conversation", Value: cv.id}] {
		return "focus"
	}
	if v.r.dial == "" {
		return "default"
	}
	return v.r.dial
}

// autoLevel is the thread dial's automatic part (D-001): started or replied
// means Needs you, mentioned means For your information, otherwise Off. Only
// messages before the one in question count. Deleting your own message does
// not take you out of its thread (D-013); a mention edited away or deleted no
// longer follows you (R44).
func autoLevel(cv *conv, u, root string, beforeSeq int) string {
	mentioned := false
	check := func(m *message) bool {
		if m.seq >= beforeSeq {
			return false
		}
		if m.author == u {
			return true
		}
		if m.mentions[u] && !m.deleted {
			mentioned = true
		}
		return false
	}
	if r := cv.msgs[root]; r != nil && check(r) {
		return levelNeeds
	}
	for _, m := range cv.replies[root] {
		if check(m) {
			return levelNeeds
		}
	}
	if mentioned {
		return levelFYI
	}
	return levelOff
}

// threadLevel is the level that applies to a reply, and whether it was set by hand.
func threadLevel(cv *conv, v view, m *message) (string, bool) {
	if l, ok := v.r.thDial[m.root]; ok {
		return l, true
	}
	l := autoLevel(cv, v.p.id, m.root, m.seq)
	if l != levelOff && !cv.direct && channelDial(cv, v) == "focus" {
		l = levelFocus // D-001; R41 for a thread you were only mentioned in
	}
	return l, false
}

func levelBucket(l string) Bucket {
	switch l {
	case levelFocus:
		return Focus
	case levelNeeds:
		return Needs
	case levelFYI:
		return FYI
	}
	return NoBucket
}

// flagRecipients is who a flag names: the people mentioned, and the other
// person of a direct conversation (D-001). A whole-channel mention names
// nobody (R03).
func flagRecipients(cv *conv, m *message) map[string]bool {
	out := map[string]bool{}
	if m.level != "flag" {
		return out
	}
	for p := range m.mentions {
		out[p] = true
	}
	if cv.direct {
		for p := range cv.members {
			if p != m.author {
				out[p] = true
			}
		}
	}
	return out
}

// inFocusPeople says whether direct messages from who land in the reader's
// Focus (D-021). The automatic part is kept with the direct conversation, so
// that a conversation's result depends only on its own changes (G-030).
func inFocusPeople(p *person, r *convReader, who string) bool {
	return !p.removed[who] && (p.focusPeople[who] || r.autoFocus)
}

type candidate struct {
	class
	tag string
}

// classify says where message m lands for the reader now, or nil.
func (e *Engine) classify(cv *conv, v view, m *message) *class {
	u := v.p.id
	if m.deleted || m.author == u {
		return nil
	}
	isDM := cv.direct
	inDM := isDM && cv.members[u]
	if isDM && !inDM {
		return nil
	}
	who := e.displayName(m.author)
	isReply := m.parent != ""

	if m.stopped { // D-031, D-032: For your information at any dial except Off
		if inDM {
			return &class{bucket: FYI, reason: who + "'s reply stopped", key: rowKey{"dm", cv.id}, decider: byStopped}
		}
		if isReply {
			if l, _ := threadLevel(cv, v, m); l != levelOff {
				return &class{bucket: FYI, reason: who + "'s reply stopped", key: rowKey{"th", m.root}, decider: byStopped}
			}
		}
		return nil
	}

	dial := "default"
	if !isDM {
		dial = channelDial(cv, v)
	}
	member := inDM || cv.members[u]
	named := m.mentions[u]
	thKey := rowKey{"th", m.root}
	chKey := rowKey{"ch", cv.id}
	dmKey := rowKey{"dm", cv.id}

	lvl, byHand := levelOff, false
	if isReply {
		lvl, byHand = threadLevel(cv, v, m)
	}
	var cands []candidate
	add := func(b Bucket, reason string, k rowKey, flag, tag, decider string) {
		cands = append(cands, candidate{class{bucket: b, reason: reason, key: k, flag: flag, decider: decider}, tag})
	}

	if m.level == "flag" {
		recipients := flagRecipients(cv, m)
		if recipients[u] {
			b, d := Needs, byFlagNoRule // R31
			if v.p.rules[flagsRule] {
				b, d = Focus, byFlagRule
			}
			add(b, "Flagged for you by "+who, thKey, "named", "flag", d)
		} else if len(recipients) == 0 {
			inScope := member && dial != "muted"
			if isReply {
				inScope = lvl != levelOff
			}
			if inScope {
				where := cv.name
				if isReply {
					where = "the thread"
				}
				add(Needs, fmt.Sprintf("Flagged for everyone in %s by %s", where, who), thKey, "open", "flag", byFlagOpen)
			}
		}
	}

	if inDM {
		switch {
		case v.p.rules[contract.FocusRule{Type: "conversation", Value: cv.id}]:
			add(Focus, "Direct message from "+who, dmKey, "", "dm", byConvRule)
		case m.agentAuthor: // R04, D-021 point 4
			add(Needs, "Direct message from agent "+who, dmKey, "", "dm", byDMAgent)
		case inFocusPeople(v.p, v.r, m.author):
			add(Focus, "Direct message from "+who, dmKey, "", "dm", byDMPerson)
		default:
			add(Needs, "Direct message from "+who, dmKey, "", "dm", byDMOther)
		}
	}

	if named {
		if v.p.rules[mentionsRule] {
			add(Focus, who+" mentioned you", thKey, "", "mention", byMentionRule)
		} else {
			add(Needs, who+" mentioned you", thKey, "", "mention", byMention)
		}
	}

	// R30: a whole-channel mention inside a thread reply reaches nobody extra.
	if m.group && !isReply && member && dial != "muted" {
		add(FYI, fmt.Sprintf("%s mentioned everyone in %s", who, cv.name), thKey, "", "group", byGroup)
	}

	if isReply && lvl != levelOff {
		text, d := "Reply in a thread you follow", byThreadFYI
		switch lvl {
		case levelFocus:
			text, d = "Thread is in Focus", byThreadFocus
		case levelNeeds:
			text, d = "Reply in your thread", byThreadNeeds
		}
		add(levelBucket(lvl), text, thKey, "", "reply", d)
	}

	if !isReply && !isDM && member {
		switch dial {
		case "focus":
			add(Focus, cv.name+" is in Focus", chKey, "", "channel", byChannelFocus)
		case "all":
			add(FYI, cv.name+" is set to All activity", chKey, "", "channel", byChannelAll)
		}
	}

	// D-001: a channel in Focus brings everything addressed to you there. A
	// thread level set by hand still wins for plain replies.
	if !isDM && member && dial == "focus" {
		for i := range cands {
			c := &cands[i]
			if c.bucket != Focus && !(c.tag == "reply" && byHand) {
				c.bucket, c.reason, c.decider = Focus, c.reason+" · "+cv.name+" is in Focus", byChannelFocus
			}
		}
	}
	if len(cands) == 0 {
		return nil
	}

	// Highest bucket wins; among equals an addressed reason beats the channel's.
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.bucket != b.bucket {
			return a.bucket > b.bucket
		}
		if ac, bc := a.key.kind != "ch", b.key.kind != "ch"; ac != bc {
			return ac
		}
		return a.flag != "" && b.flag == ""
	})
	out := cands[0].class
	out.flag = ""
	for _, c := range cands {
		if c.flag != "" {
			out.flag = c.flag
			break
		}
	}
	for _, c := range cands {
		if c.key.kind != "ch" {
			out.key = c.key
			break
		}
	}
	if inDM {
		out.key = dmKey // a direct conversation is one row, replies included
	}

	// R05: a person rule lifts what already reaches you from them.
	if v.p.rules[contract.FocusRule{Type: "person", Value: m.author}] && out.bucket < Focus {
		out.bucket, out.reason, out.decider = Focus, out.reason+" · "+who+" is in your Focus", byPersonRule
	}
	// D-001: Info never lands higher than For your information.
	if m.level == "info" && out.bucket > FYI {
		out.bucket, out.reason, out.decider = FYI, out.reason+" · marked Info", byInfo
	}
	return &out
}
