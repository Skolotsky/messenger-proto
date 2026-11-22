# D-012: What sends a notification

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-003, D-004, D-011, D-015
- Evidence: E-001 (rows per person per day), R-002 (Slack separated what appears in Activity from what is pushed; Teams sends nothing for in-progress agent output)
- Visual: none; the options differ by one line each

## Why this needs deciding now

A notification interrupts; the feed waits. So far only the edges are decided: an Info message sends none (D-001), text still streaming in sends none (D-001), something arriving in a conversation the person is watching sends none (D-015), and one not yet sent is cancelled when its message is deleted (D-004). Which feed rows notify is open. It is the last open product decision, the notification sender is one of the three systems the brief names, and the delivery guarantees (D-006, D-007) need to know what it is asked to send.

In the first version a notification is an entry in the notification tray (D-009), which stands in for push.

## Options

Numbers are new rows per person per day from E-001's simulated 30-person team under the starting rules. The traffic is assumed, so read them as proportions.

### A. Everything in the feed notifies

About 38 a day.

- Upside: nothing is missed; it is what people coming from other tools expect.
- Risk: it interrupts for a reaction as readily as for a flag. The sorting the feed does is undone at the moment it matters most.

### B. Focus and Needs you notify; For your information never does

About 21 a day (4.5 Focus, 16.5 Needs you).

- Upside: an interruption means "this is directed at you". A direct mention still pings, as people expect.
- Risk: 21 a day is still a lot. Most of them are thread replies.

### C. Only Focus notifies

About 4.5 a day.

- Upside: the calmest. An interruption means "you said this matters, or someone flagged you".
- Risk: a direct mention no longer pings unless the person adds the rule "any direct mention" to Focus. People coming from Slack will miss things in their first week and blame the product.

## Best practice

Separate what is recorded from what interrupts, and let the person choose how much interrupts (Slack's 2026 notification rebuild gives independent control over activity and push; R-002). Send one notification per thing, not one per message in it.

## Recommendation

**B as the default, with one personal switch to C** ("notify me for Focus only").

Rules that go with it, whichever option:

- **One notification per row, not per message.** A row notifies when it becomes unread in a bucket that notifies. Further items on a row that is still unread send nothing more. A row that moves up into Focus while unread notifies once more.
- **The channel and thread dials decide through the bucket.** A muted channel sends nothing except what names you, because nothing else reaches Focus or Needs you from it. No second set of notification settings per channel.
- **The tray shows the current truth** (D-004): an entry for a message that was edited shows the new text; for a deleted one, "message deleted"; for a row read anywhere, it is marked read (D-015).
- Already decided and unchanged: Info sends none; streaming text sends none; nothing for a conversation the person is watching.

Out, as known shortcuts: do-not-disturb hours, sounds, email, digests, reminders for kept rows.

Locks in: notifications follow the bucket; one switch per person; no notification settings per channel.

Reversibility: cheap. It is a rule applied when a row is created or moves.

No second opinion was sought: cheap to reverse.

## Decision

Accepted by the user on 2026-11-22: **B as the default, with one personal switch to C**, and the rules listed under the recommendation.

## Consequences

- Easier: one rule to explain: a notification means a row became unread in a bucket that notifies. The dials need no notification settings of their own.
- Harder: the notification sender has to know a row's bucket and whether it is already unread, so it depends on the feed's state, not only on the message. A row moving up into Focus is a second trigger.
- Committed to: notifications follow the bucket; one per row; one switch per person.

## Follow-ups

- Rerun E-001 with D-014, D-015 and this decision, with an independent check, before the look-and-feel work.
