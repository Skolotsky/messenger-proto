# D-003: How an item leaves the feed

- Status: proposed
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-009
- Evidence: R-002
- Visual: `docs/visuals/D-003-how-items-leave-the-feed.html`

## Why this needs deciding now

D-001 decided what enters the feed and where it lands. It did not decide how anything leaves. That is the other half of "a short list": if rows only pile up, Focus stops being short within a day, and the signal we want from early access ("people clear the feed daily") has nothing to measure. This is the last open piece of the feed's behaviour before edits and deletes (D-004).

Not open here, from D-001: three buckets; one row per conversation; a reason on every row; a flag closes when the flagged person responds or the sender withdraws it; nothing jumps.

## Options

### A. Read is enough

Opening a conversation marks its row read. Read rows stay in the bucket, dimmed, and age out after some weeks. This is how Slack's Activity and Teams' feed mostly behave.

- Upside: nothing to learn; nothing to do.
- Risk: the list never empties. "Read" and "dealt with" are the same state, so something opened on a phone and meant for later looks finished. The bet of a short list is weakened.

### B. Done, by hand or by acting

A row stays until it is done. It becomes done in one of three ways: the person marks it done (one click, or a swipe on a phone); the person acts on it from the row (a reply or a reaction); or its reason goes away (a flag is answered by someone else or withdrawn). Read and unread are still shown, but reading alone does not remove a row. Done rows move to a Done list and can be brought back. A row returns to its bucket when something new happens in that conversation.

- Upside: a bucket can reach empty, and empty means something. "Read but not dealt with" survives. It gives early access a clear signal.
- Risk: one more thing to do per row, and people who never mark anything done end up with A anyway. Reduced by acting-is-done and by "mark all done" on a bucket.

### C. Gone once seen

A row disappears as soon as its conversation is opened.

- Upside: no effort at all.
- Risk: anything opened and not handled is lost, which is the fastest way to stop trusting the feed.

## Best practice

Tools built around attention separate "seen" from "handled" and let acting on an item handle it: GitHub's Done, Linear's inbox, Slack's "clear" (which Slack also applies automatically when you reply). Feed libraries keep seen and read as two states for the same reason (R-002).

## Recommendation

**B**, with these rules:

- **Acting is done.** Replying or reacting from the row marks it done. For a flag, that reply or reaction is also what closes it for you.
- **Reply and react from the row,** without leaving the feed. Opening the conversation in place is one more click.
- **Mark all done** per bucket.
- **The list does not move while you look at it.** New rows and new activity do not push anything down or reorder the list. A quiet "3 new" marker appears at the top and applies when you choose. A row marked done leaves its space until you move on. This extends "nothing jumps" from edits to arrivals.
- **Order:** newest activity first within a bucket, applied only at the moments above.
- **Counts:** Focus and Needs you show how many rows are not done. For your information shows a dot, not a number.
- **An empty bucket** shows an illustration and one line, not a blank space. An empty Focus with no rules offers the suggested rules.

Out, as known shortcuts: snooze and "remind me later"; saving a row.

Locks in: done is a state separate from read; rows can be acted on in place; the list is stable while viewed.

Reversibility: moderate once built, because the stable-list rule shapes how every update reaches the screen.

No second opinion was sought: nothing is built yet.

## Decision

_Open._

## Consequences

_Filled in when accepted._

## Follow-ups

- D-004 (edits and deletes) builds on the stable-list rule.
