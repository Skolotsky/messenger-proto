// Package e2e is the spine of Milestone 1 (T-057): what a person does at the door
// reaches search, the feed and the notification sender, on the real programs.
package e2e

import (
	"strings"
	"testing"
	"time"

	"contexia/tests/e2e/stack"
)

func thread(id string) *string { return &id }

func TestTheSpine(t *testing.T) {
	spine(t, stack.Start(t, stack.Options{}))
}

func spine(t testing.TB, s *stack.Stack) {
	o := s.Seed("spine", []string{"anna", "marco", "lena"}, []string{"#launch"})

	// A message, then its edit.
	var last stack.SearchAnswer
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("last search answer: %+v", last)
		}
	})
	o.Do("anna", "send", stack.Message("msg_plain", "#launch", "the walrus plan is ready", nil))
	o.Do("anna", "edit", stack.Edit("msg_plain", "the narwhal plan is ready"))
	stack.Until(t, 10*time.Second, "search finds the edited text", func() bool {
		a := o.SearchFor("marco", "narwhal")
		last = a
		return len(a.Results) == 1
	})
	if got := o.SearchFor("marco", "walrus"); got.Total != 0 {
		t.Fatalf("search still finds the text from before the edit: %+v", got)
	}

	// A message that is deleted is gone from search.
	o.Do("anna", "send", stack.Message("msg_gone", "#launch", "the pelican budget", nil))
	stack.Until(t, 10*time.Second, "search finds the message before its delete", func() bool {
		return o.SearchFor("marco", "pelican").Total == 1
	})
	o.Do("anna", "delete", stack.Delete("msg_gone"))
	stack.Until(t, 10*time.Second, "search no longer finds the deleted message", func() bool {
		return o.SearchFor("marco", "pelican").Total == 0
	})

	// A mention read within the hold, then a mention left unread.
	readNumber := o.Do("anna", "send", stack.Message("msg_read", "#launch", "@Marco please check the otter page", nil, "marco"))
	o.Do("marco", "read", map[string]any{"conversation": "#launch", "thread": thread("msg_read"), "up_to": readNumber})
	unreadNumber := o.Do("anna", "send", stack.Message("msg_unread", "#launch", "@Lena please check the heron page", nil, "lena"))

	// The feed holds what the rules say for each reader.
	stack.Until(t, 10*time.Second, "Lena's feed shows the mention, unread, in Needs you", func() bool {
		return findRow(o.FeedFor("lena").Rows, "msg_unread", true, "needs_you")
	})
	stack.Until(t, 10*time.Second, "Marco's mention is read in the feed", func() bool {
		a := o.FeedFor("marco")
		return a.Through != nil && *a.Through >= unreadNumber && !findRow(a.Rows, "msg_read", true, "")
	})

	// The unread one is sent once, within 8 s of real time from its log timestamp.
	var savedAt time.Time
	for _, e := range o.Log() {
		if e.Number == unreadNumber {
			savedAt = e.At
		}
	}
	if savedAt.IsZero() {
		t.Fatalf("change %d is not in the log", unreadNumber)
	}
	var sent stack.Notification
	stack.Until(t, 15*time.Second, "the sender has Lena's notification", func() bool {
		for _, n := range o.NotificationsFor("lena") {
			if strings.Contains(n.Name, "msg_unread") {
				sent = n
				return n.State == "sent"
			}
		}
		return false
	})
	if sent.At == nil {
		t.Fatalf("the sent notification has no time: %+v", sent)
	}
	if took := sent.At.Sub(savedAt); took > 8*time.Second {
		t.Fatalf("the notification took %v from the log's timestamp; the contract says within 8 s", took)
	} else {
		t.Logf("the unread mention was sent %v after its log timestamp", took.Round(time.Millisecond))
	}
	if n := o.HandOvers(sent.Name); n != 1 {
		t.Fatalf("%s was handed to the provider %d times, not once", sent.Name, n)
	}

	// The read mention came first, so the tick that released Lena's has passed its
	// hold too: if it were going to be sent, it would be on the list by now.
	for _, n := range o.NotificationsFor("marco") {
		if strings.Contains(n.Name, "msg_read") {
			t.Fatalf("a notification was sent for a mention read within the hold: %+v", n)
		}
	}
	if n := o.HandOvers(sent.Name); n != 1 {
		t.Fatalf("%s was handed over again: %d times", sent.Name, n)
	}
}

func findRow(rows []stack.Row, message string, unread bool, bucket string) bool {
	for _, r := range rows {
		matches := r.Message == message || (r.Thread != nil && *r.Thread == message)
		if matches && r.Unread == unread && (bucket == "" || r.Bucket == bucket) {
			return true
		}
	}
	return false
}
