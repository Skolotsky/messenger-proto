// Package watching holds G-024: nothing is marked read on a person's behalf
// unless a live connection of theirs says the conversation is open.
package watching

import (
	"testing"
	"time"

	"contexia/internal/conversations"
	"contexia/tests/guardrails/live-order/liverig"
)

const channel = "#launch"

func started(r *liverig.Rig, after int64) int { return r.LogKinds(after, "reader.watching_started") }
func ended(r *liverig.Rig, after int64) int   { return r.LogKinds(after, "reader.watching_ended") }
func openWatch(t *testing.T, r *liverig.Rig, s *liverig.Screen, after int64, want int) {
	t.Helper()
	s.Watch(channel, true)
	r.Await("the start of watching to be saved", func() bool { return started(r, after) == want })
}

// messageLeavesItUnread is what the guardrail is for: a message 61 s later is
// not read on the person's behalf.
func messageLeavesItUnread(t *testing.T, r *liverig.Rig, c *liverig.Copy) {
	t.Helper()
	r.Clock.Advance(61 * time.Second)
	before := r.ReadUpTo("marco", channel)
	n := r.Say(c, "anna", channel, "61 seconds later")
	if got := r.ReadUpTo("marco", channel); got != before {
		t.Fatalf("message %d moved marco's read position from %d to %d although nothing of his is watching (G-024)", n, before, got)
	}
	start, err := c.Store.Start(r.Ctx, r.Org, "marco")
	if err != nil {
		t.Fatal(err)
	}
	for _, conv := range start.Conversations {
		if conv.Conversation == channel && !conv.Unread {
			t.Fatalf("%s is not unread for marco after a message nobody of his was watching (G-024)", channel)
		}
	}
}

func TestWhileWatchedAnArrivalIsRead(t *testing.T) {
	r := liverig.New(t)
	c := r.NewCopy(conversations.LiveOptions{})
	from := r.Head()
	screen := r.Open(c, "marco")
	openWatch(t, r, screen, from, 1)
	n := r.Say(c, "anna", channel, "seen as it arrives")
	if got := r.ReadUpTo("marco", channel); got != n {
		t.Fatalf("an arrival on a watched row must be read in the same step: position %d, message %d", got, n)
	}
	screen.Watch(channel, false)
	r.Await("the end to be saved", func() bool { return ended(r, from) == 1 })
	messageLeavesItUnread(t, r, c)
}

func TestVanishedScreenIsGoneWithin40Seconds(t *testing.T) {
	r := liverig.New(t)
	c := r.NewCopy(conversations.LiveOptions{})
	from := r.Head()
	screen := r.Open(c, "marco")
	openWatch(t, r, screen, from, 1)
	// Wake the blocked Read so it sees it has gone deaf, then stop answering.
	screen.GoDeaf()
	r.Say(c, "anna", channel, "wakes the screen's read")
	moved := r.AdvanceUntil("a screen that vanished to be no longer watching", 45*time.Second, func() bool { return ended(r, from) == 1 })
	if moved > 41*time.Second {
		t.Fatalf("a vanished screen ended after %s; the contract says about 40 s", moved)
	}
	t.Logf("no longer watching after %s on the clock", moved)
	messageLeavesItUnread(t, r, c)
}

func TestDeadCopysWatchingsEndWithin25Seconds(t *testing.T) {
	r := liverig.New(t)
	dead := r.NewCopy(conversations.LiveOptions{})
	sweeper := r.NewCopy(conversations.LiveOptions{})
	from := r.Head()
	screen := r.Open(dead, "marco")
	openWatch(t, r, screen, from, 1)
	// The copy is killed: nothing of it runs any more. Only the other one sweeps.
	moved := r.AdvanceUntil("the dead copy's watching to end", 25*time.Second, func() bool {
		if err := sweeper.Live.RenewLease(r.Ctx); err != nil {
			t.Fatal(err)
		}
		if err := sweeper.Live.SweepPass(r.Ctx); err != nil {
			t.Fatal(err)
		}
		return ended(r, from) == 1
	})
	t.Logf("a dead copy's watching ended after %s on the clock", moved)
	messageLeavesItUnread(t, r, sweeper)
}

func TestFrozenCopyCannotStartAWatchingAfterItWasSwept(t *testing.T) {
	r := liverig.New(t)
	frozen := r.NewCopy(conversations.LiveOptions{})
	sweeper := r.NewCopy(conversations.LiveOptions{})
	from := r.Head()
	screen := r.Open(frozen, "marco")
	r.Await("the connection", func() bool { return !screen.Closed() })
	r.Clock.Advance(16 * time.Second)
	if err := sweeper.Live.RenewLease(r.Ctx); err != nil {
		t.Fatal(err)
	}
	if err := sweeper.Live.SweepPass(r.Ctx); err != nil {
		t.Fatal(err)
	}
	// It wakes and acts on a "watching" that was on its way.
	screen.Watch(channel, true)
	r.Await("the woken copy to drop the connection", func() bool { return screen.Closed() || started(r, from) > 0 })
	if got := started(r, from); got != 0 {
		t.Fatalf("a copy declared dead saved %d start(s) of watching under its old identity (G-024)", got)
	}
	messageLeavesItUnread(t, r, sweeper)
}

func TestTwoConnectionsEndInEitherOrder(t *testing.T) {
	for _, order := range []string{"new start first, old end after", "old end first, new start after"} {
		t.Run(order, func(t *testing.T) {
			r := liverig.New(t)
			a := r.NewCopy(conversations.LiveOptions{})
			b := r.NewCopy(conversations.LiveOptions{})
			from := r.Head()
			oldScreen := r.Open(a, "marco")
			openWatch(t, r, oldScreen, from, 1)
			newScreen := r.Open(b, "marco")
			if order == "new start first, old end after" {
				openWatch(t, r, newScreen, from, 2)
				oldScreen.Cut()
				r.Await("the old end", func() bool { return ended(r, from) == 1 })
				n := r.Say(b, "anna", channel, "the new connection still watches")
				if got := r.ReadUpTo("marco", channel); got != n {
					t.Fatalf("marco still watches on the new connection, but the message was not read: %d, want %d", got, n)
				}
			} else {
				oldScreen.Cut()
				r.Await("the old end", func() bool { return ended(r, from) == 1 })
				openWatch(t, r, newScreen, from, 2)
				n := r.Say(b, "anna", channel, "watched again")
				if got := r.ReadUpTo("marco", channel); got != n {
					t.Fatalf("marco watches on the new connection, but the message was not read: %d, want %d", got, n)
				}
			}
			newScreen.Cut()
			r.Await("the last end", func() bool { return ended(r, from) == 2 })
			messageLeavesItUnread(t, r, b)
		})
	}
}

func TestAnHourOfWatchingAddsTwoLogEntries(t *testing.T) {
	r := liverig.New(t)
	c := r.NewCopy(conversations.LiveOptions{})
	from := r.Head()
	screen := r.Open(c, "marco")
	openWatch(t, r, screen, from, 1)
	for range 180 { // every ping of an hour, each answered
		pongs := c.Live.PingsAnswered()
		r.Clock.Advance(19 * time.Second) // the rest a second at a time, so the answer is in before its 20 s run out
		r.AdvanceUntil("the ping to be answered", 45*time.Second, func() bool { return c.Live.PingsAnswered() > pongs })
	}
	if screen.Closed() {
		t.Fatal("a screen that answers every ping was dropped")
	}
	screen.Watch(channel, false)
	r.Await("the end", func() bool { return ended(r, from) == 1 })
	kinds := []string{"reader.watching_started", "reader.watching_ended", "reader.read"}
	if got := r.LogKinds(from, kinds...); got != 2 {
		t.Fatalf("an hour of watching wrote %d log entries, want 2 (G-024, D-029)", got)
	}
}
