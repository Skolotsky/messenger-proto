// Package feedfence holds G-009: a change the feed cannot apply fences only the
// conversation or the reader it touches; every row outside equals a clean replay;
// more than five fences stop that organization's feed and no other.
package feedfence

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"testing"

	"contexia/internal/shared/contract"
	"contexia/tests/guardrails/feed-equals-replay/feedrig"
	"contexia/tests/guardrails/notification-once/rig"
)

const at = "2026-11-22T12:00:00.000Z"

// builder writes a small log of two channels with a held mention in each.
type builder struct {
	org string
	log []feedrig.Change
}

func (b *builder) add(kind string, fields map[string]any) int64 {
	n := int64(len(b.log)) + 1
	fields["org"], fields["number"], fields["previous"], fields["kind"] = b.org, n, n-1, kind
	if _, ok := fields["at"]; !ok {
		fields["at"] = at
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}
	b.log = append(b.log, feedrig.Change{Number: n, Kind: kind, Body: raw})
	return n
}

func message(id, conv, author, text string, n int64, version int64, mentions ...string) map[string]any {
	if mentions == nil {
		mentions = []string{}
	}
	return map[string]any{"id": id, "number": n, "version": version, "conversation": conv, "thread": nil, "author": author,
		"author_is_agent": false, "participants": nil, "text": text, "mentions": mentions, "level": nil, "reactions": []any{},
		"stopped": false, "created_at": at, "edited_at": nil, "deleted": false}
}

func (b *builder) post(id, conv, text string, mentions ...string) int64 {
	n := int64(len(b.log)) + 1
	return b.add("message.created", map[string]any{"message": message(id, conv, "anna", text, n, 1, mentions...)})
}

func base(org string) *builder {
	b := &builder{org: org}
	for _, p := range []string{"anna", "marco", "lena"} {
		b.add("person.joined", map[string]any{"person": p, "name": p, "is_agent": false})
	}
	for _, c := range []string{"#a", "#b", "#c", "#d", "#e", "#f", "#g", "#h"} {
		b.add("channel.created", map[string]any{"conversation": c, "name": c[1:], "people": []string{"anna", "marco", "lena"}})
	}
	return b
}

// nulEdit is a change the feed cannot apply: its note log cannot store a NUL.
func (b *builder) nulEdit(id, conv string) int64 {
	n := int64(len(b.log)) + 1
	return b.add("message.changed", map[string]any{"message": message(id, conv, "anna", "bad \x00 text", n, 2, "marco")})
}

func deliver(t *testing.T, c *feedrig.Copy, log []feedrig.Change) feedrig.Answer {
	t.Helper()
	return feedrig.Deliver(t, c.URL, log, log[len(log)-1].Number, rand.New(rand.NewSource(7)))
}

func outside(conv ...string) func(string) bool {
	return func(c string) bool {
		for _, x := range conv {
			if c == x {
				return false
			}
		}
		return true
	}
}

func rowsOf(t *testing.T, c *feedrig.Copy, org, person string) []contract.FeedRow {
	t.Helper()
	return feedrig.Ask(t, c.URL, org, person).Rows
}

// conversationFence: an edit that cannot be applied fences #a, and what follows
// in #a is set aside, while #b stays exact and nothing new notifies from #a.
func conversationFence(t *testing.T) []string {
	r := feedrig.New(t)
	org := rig.NewOrg("g009")
	b := base(org)
	b.post("m_a1", "#a", "for marco", "marco")
	b.post("m_b1", "#b", "for marco too", "marco")
	bad := b.nulEdit("m_a1", "#a")
	b.post("m_a2", "#a", "after the fence", "lena")
	b.post("m_b2", "#b", "still fine", "lena")
	b.add("tick", map[string]any{"at": "2026-11-22T12:00:10.000Z"})
	c := r.Start(1)
	if a := deliver(t, c, b.log); a.Status != http.StatusOK {
		return []string{fmt.Sprintf("the fenced log ended with %+v", a)}
	}
	if a := feedrig.Post(c.URL, []feedrig.Change{b.log[bad-1]}); a.Status != http.StatusOK {
		return []string{fmt.Sprintf("a repeat of the fenced change got %+v", a)}
	}
	if p := feedrig.Place(t, c.URL, org); p.Through != int64(len(b.log)) {
		return []string{fmt.Sprintf("a set-aside change must count for the place: through %d of %d", p.Through, len(b.log))}
	}
	clean := feedrig.Fold(t, org, b.log, int64(len(b.log)), func(x feedrig.Change) bool {
		return x.Number != bad && !strings.Contains(string(x.Body), `"conversation":"#a"`)
	})
	var out []string
	for _, p := range []string{"anna", "marco", "lena"} {
		out = append(out, feedrig.DiffReader(p, feedrig.Ask(t, c.URL, org, p), clean.Of(p), outside("#a"))...)
		for _, row := range rowsOf(t, c, org, p) {
			if row.Conversation == "#a" && !row.Fenced {
				out = append(out, p+": a row in the fenced #a is not marked")
			}
			if row.Conversation != "#a" && row.Fenced {
				out = append(out, p+": a row outside the fence is marked")
			}
			if row.Message == "m_a2" {
				out = append(out, p+": a change inside the fence was applied instead of set aside")
			}
		}
	}
	for _, n := range r.Notes(org, feedrig.Place(t, c.URL, org).Edition) {
		if n.Kind == "send" && strings.Contains(n.Name, "#a") {
			out = append(out, "a notification was sent from inside the fence: "+n.Name)
		}
	}
	for _, h := range r.Held(org) {
		if strings.Contains(h, "#a") {
			out = append(out, "still on hold inside the fence: "+h)
		}
	}
	// A restarted copy holds the same fence.
	c.Close()
	again := r.Start(2)
	for _, p := range []string{"anna", "marco", "lena"} {
		out = append(out, feedrig.DiffReader(p, feedrig.Ask(t, again.URL, org, p), clean.Of(p), outside("#a"))...)
	}
	return out
}

// readerFence: a dial that cannot be applied fences that reader in that
// conversation only; everyone else's rows there, and their own elsewhere, stay.
func readerFence(t *testing.T) []string {
	r := feedrig.New(t)
	org := rig.NewOrg("g009r")
	b := base(org)
	root := "m_r1"
	b.post(root, "#a", "for marco and lena", "marco", "lena")
	bad := b.add("reader.dial", map[string]any{"person": "marco", "conversation": "#a", "thread": nil, "level": "not-a-level"})
	b.add("reader.keep", map[string]any{"person": "marco", "conversation": "#a", "thread": root})
	b.post("m_r2", "#a", "again for both", "marco", "lena")
	b.post("m_r3", "#b", "elsewhere", "marco")
	b.add("tick", map[string]any{"at": "2026-11-22T12:00:10.000Z"})
	c := r.Start(3)
	if a := deliver(t, c, b.log); a.Status != http.StatusOK {
		return []string{fmt.Sprintf("the fenced log ended with %+v", a)}
	}
	clean := feedrig.Fold(t, org, b.log, int64(len(b.log)), func(x feedrig.Change) bool {
		if x.Number == bad {
			return false
		}
		s := string(x.Body)
		return !(strings.HasPrefix(x.Kind, "reader.") && strings.Contains(s, `"person":"marco"`) && strings.Contains(s, `"conversation":"#a"`))
	})
	var out []string
	out = append(out, feedrig.DiffReader("lena", feedrig.Ask(t, c.URL, org, "lena"), clean.Of("lena"), nil)...)
	out = append(out, feedrig.DiffReader("marco", feedrig.Ask(t, c.URL, org, "marco"), clean.Of("marco"), outside("#a"))...)
	for _, row := range rowsOf(t, c, org, "marco") {
		if row.Conversation == "#a" && !row.Fenced {
			out = append(out, "marco's row in #a is not marked")
		}
	}
	for _, row := range rowsOf(t, c, org, "lena") {
		if row.Fenced {
			out = append(out, "lena's row is marked, but only marco is fenced")
		}
	}
	for _, n := range r.Notes(org, feedrig.Place(t, c.URL, org).Edition) {
		if n.Kind == "send" && strings.Contains(n.Name, "/ marco / #a") {
			out = append(out, "a notification went out for the fenced reader: "+n.Name)
		}
	}
	return out
}

// tooMany: a fault on every edit, in eight conversations: the sixth stops this
// organization, and another organization goes on.
func tooMany(t *testing.T) []string {
	r := feedrig.New(t)
	org, other := rig.NewOrg("g009s"), rig.NewOrg("g009o")
	b, o := base(org), base(other)
	for _, conv := range []string{"#a", "#b", "#c", "#d", "#e", "#f", "#g", "#h"} {
		id := "m_" + conv[1:]
		b.post(id, conv, "text", "marco")
		b.nulEdit(id, conv)
		o.post(id, conv, "text", "marco")
	}
	c := r.Start(4)
	a := deliver(t, c, b.log)
	if a.Status != http.StatusLocked || a.OrganizationStopped == nil || *a.OrganizationStopped != org {
		return []string{fmt.Sprintf("after more than five fences the answer was %+v, want 423 organization_stopped", a)}
	}
	var out []string
	place := feedrig.Place(t, c.URL, org).Through
	again := feedrig.Post(c.URL, b.log[place:place+1])
	if again.Status != http.StatusLocked {
		out = append(out, fmt.Sprintf("a stopped organization answered %d", again.Status))
	}
	// A restart does not lift the stop.
	c.Close()
	c = r.Start(5)
	if a := feedrig.Post(c.URL, b.log[place:place+1]); a.Status != http.StatusLocked {
		out = append(out, fmt.Sprintf("after a restart the stopped organization answered %d", a.Status))
	}
	if a := deliver(t, c, o.log); a.Status != http.StatusOK {
		out = append(out, fmt.Sprintf("the other organization got %+v", a))
	}
	truth := feedrig.Fold(t, other, o.log, int64(len(o.log)), nil)
	out = append(out, feedrig.Diff(t, c.URL, other, truth)...)
	// Repair: a rebuild of the organization lifts the stop.
	feedrig.Wipe(t, c.URL, org, false)
	if p := feedrig.Place(t, c.URL, org); p.Through != 0 {
		out = append(out, fmt.Sprintf("after the rebuild's wipe the place is %d", p.Through))
	}
	if a := feedrig.Post(c.URL, b.log[:1]); a.Status != http.StatusOK {
		out = append(out, fmt.Sprintf("after the wipe the organization answered %d", a.Status))
	}
	return out
}

// readerWide: a fault on a reader-wide change stops the organization at once.
func readerWide(t *testing.T) []string {
	r := feedrig.New(t)
	org := rig.NewOrg("g009w")
	b := base(org)
	b.post("m_1", "#a", "text", "marco")
	b.add("reader.focus_rule", map[string]any{"person": "nobody", "rule": map[string]any{"type": "person", "value": "anna"}, "on": true})
	c := r.Start(6)
	a := deliver(t, c, b.log)
	if a.Status != http.StatusLocked {
		return []string{fmt.Sprintf("a fault on a reader-wide change answered %+v, want 423", a)}
	}
	return nil
}

func TestConversationFence(t *testing.T) {
	if bad := conversationFence(t); len(bad) > 0 {
		t.Fatalf("G-009 (%d): %s", len(bad), bad[0])
	}
}

func TestReaderFence(t *testing.T) {
	if bad := readerFence(t); len(bad) > 0 {
		t.Fatalf("G-009 (%d): %s", len(bad), bad[0])
	}
}

func TestMoreThanFiveFencesStopOneOrganization(t *testing.T) {
	if bad := tooMany(t); len(bad) > 0 {
		t.Fatalf("G-009 (%d): %s", len(bad), bad[0])
	}
}

func TestFaultOnReaderWideChangeStopsTheOrganization(t *testing.T) {
	if bad := readerWide(t); len(bad) > 0 {
		t.Fatalf("G-009 (%d): %s", len(bad), bad[0])
	}
}
