# D-003: How an item leaves the feed

- Status: accepted
- Level: product
- Date: 2026-11-22 (proposed, reshaped by the user and accepted the same day)
- Decided by: user
- Depends on: D-001, D-009
- Evidence: R-002
- Visual: `docs/visuals/D-003-how-items-leave-the-feed.html`

## Why this needs deciding now

D-001 decided what enters the feed and where it lands. It did not decide how anything leaves. That is the other half of "a short list": if rows only pile up, Focus stops being short within a day, and the signal we want from early access ("people clear the feed daily") has nothing to measure.

Not open here, from D-001: three buckets; one row per conversation; a reason on every row; a flag closes when the flagged person responds or the sender withdraws it; nothing jumps.

## Options

### A. Read is enough

Opening a conversation marks its row read, and a read row leaves. This is how Slack's Activity and Teams' feed mostly behave.

- Upside: nothing to learn; nothing to do.
- Risk: "read" and "dealt with" are the same state, so something opened on a phone and meant for later is gone.

### B. Done, by hand or by acting

A row stays until it is done: the person marks it, acts on it, or its reason goes away. Reading alone does not remove it.

- Upside: a bucket can reach empty, and empty means something.
- Risk: one more thing to do for every row.

### C. Gone once seen

A row disappears as soon as it has been on screen.

- Upside: no effort at all.
- Risk: anything seen and not handled is lost.

### D. Read is enough, and you can keep what you still mean to handle (the user's proposal)

A, plus an explicit Keep on any row. A kept row stays until the person marks it done or acts on it.

- Upside: the effort falls only on the few rows that matter, not on every row.
- Risk: keeping takes foresight at the moment of reading; a row opened by accident and not kept leaves the bucket. Reduced by an Earlier list.

## Best practice

Tools built around attention separate "seen" from "handled" (GitHub's Done, Linear's inbox, Slack's clear). They differ on the default: most make handling the explicit step, as in B. D turns that around, and the reason holds for a team messenger: most feed rows need nothing more than reading.

## Recommendation

The agent first recommended B. After the user's proposal it recommends **D**, shaped as below.

## Decision

Accepted by the user on 2026-11-22: **D.** "One wouldn't like to mark every message as done. I think the typical read is enough for the most." If you have read something and still want to do something with it, you mark it explicitly, and it does not go until you mark it handled.

| A row… | Leaves when |
|---|---|
| is ordinary | you have read it |
| is flagged for you | the flag closes: you respond, someone else answers one that named nobody, or the sender withdraws it. Reading is not enough |
| you kept | you mark it done, or you reply or react to it |

- **Keep and Done** are the names: Keep on any row, Done to release it. Keeping is private to the person, unlike a flag, which everyone in the conversation sees.
- **Kept rows live in Focus,** whichever bucket they came from, with the reason "Kept by you". Keeping is the person's own judgement applied to one item, which is what Focus means. No fourth bucket.
- **Focus has two sections:** new items on top, Kept below. The Focus count shows unread items only; Kept has its own small count and collapses past a handful ("Kept · 9"), so a growing Kept section cannot push new items down.
- **What "read" means.** In Focus and Needs you, a row is read when it is opened: expanded in the feed, or its conversation opened. Scrolling past does not count. In For your information, viewing the bucket is enough.
- **Read rows do not vanish under you.** They stay in place, dimmed, until the person leaves the feed. On the next visit they are in an Earlier list, where a row can still be found and kept.
- **A row returns** to its bucket, unread, when something new happens in that conversation.
- **Reply and react from the row,** without leaving the feed. Doing so releases a kept row and closes a flag for that person.
- **Mark all read** per bucket.
- **The list does not move while you look at it.** New rows and new activity do not push anything down or reorder the list. A quiet "3 new" marker appears at the top and applies when the person chooses. This extends "nothing jumps" from edits to arrivals.
- **Order:** newest activity first within a section, applied only at those moments.
- **Counts:** Focus and Needs you show a number of unread rows. For your information shows a dot.
- **An empty bucket** shows an illustration and one line. An empty Focus with no rules offers the suggested rules.

Out, as known shortcuts: snooze and "remind me later".

## Consequences

- Easier: the feed asks nothing of a person by default. Open flags and kept rows are the only things that persist, and they look alike on purpose: one is someone else's "this stays until you deal with it", the other is your own.
- Harder: read state is now what removes a row, so it has to be right on every device a person uses, and "opened" needs a precise meaning on a phone. Early access cannot measure "done"; it measures whether people reach zero unread and how often they keep.
- Committed to: read as the default way out; Keep and Done; kept rows in Focus; an Earlier list; replying and reacting in place; a list that is stable while viewed.

## Follow-ups

- D-004 (edits and deletes) builds on the stable-list rule, and has to say what happens to a kept row whose message changes or is deleted.
