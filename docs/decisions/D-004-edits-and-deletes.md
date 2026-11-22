# D-004: What an edit or a delete does to what people already have

- Status: proposed
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-003, D-009
- Evidence: R-002
- Visual: `docs/visuals/D-004-edits-and-deletes.html`

## Why this needs deciding now

A message that changes after it was sent has already left marks: a row in someone's feed, a notification, an entry in search. The brief asks for exactly this to be exercised, and R-002 found that existing products disagree or stay silent on it, so there is no answer to copy. It is the last product decision, and it defines what the delivery guarantees later have to carry.

Not open here:
- The feed, search and the notification tray always show the current truth (D-001).
- Nothing jumps: an edit or delete never changes the size or position of what is on screen. A longer edit sits behind an expander, a shorter one keeps its space, a deleted message leaves a marker in its place, and emptied space is filled with care (D-001, the user's rule).
- The list does not move while it is viewed; changes apply when the person moves on (D-003).

## The question

When something aimed at a person is taken back (the message is deleted, an edit removes their mention or the flag, a reaction is removed), what is left for that person?

## Options

### A. Quiet: it is simply gone

Once the person moves on, the row is gone and nothing says it was ever there.

- Upside: the sender's right to take something back is fully respected. The feed stays clean.
- Risk: a person who read something, or kept it, comes back to find it missing with no explanation, and starts doubting the feed.

### B. Transparent: every retraction leaves a trace

A row always remains: "A message that mentioned you was deleted."

- Upside: nothing ever disappears unexplained.
- Risk: it tells people that something was said and withdrawn, which defeats deleting a message sent by mistake and invites "what did you delete?". Retractions become feed noise.

### C. A trace only where the person already spent attention

- **Not yet seen:** it goes without a trace, and any count drops with it.
- **On screen right now:** it turns into a marker in the same space ("Anna deleted this message") and stays until the person moves on.
- **Read earlier:** in the Earlier list the row shows the marker in place of the text.
- **Kept, or flagged for the person:** the marker stays in Focus until the person marks it done, because they were told, or told themselves, to deal with it and should learn that there is nothing left to do.

- Upside: a retraction is respected where it costs the reader nothing, and explained where the reader would otherwise be confused.
- Risk: more states to build and to get right, and the rule needs a precise meaning of "seen". Uses the meaning of "read" from D-003.

## Best practice

Products diverge here (R-002): Zulip notifies a person mentioned by an edit, Mattermost does not, Slack does not document either case, and none documents what its feed does on delete. The nearest established rule is from interface design, not from chat products: do not change what a person is looking at, and do not remove what a person is relying on without saying so. C applies that rule; A and B each break half of it.

## Recommendation

**C**, with these rules for the other cases:

- **An edit that adds a mention or a flag** reaches the newly named person as a new item, exactly as if they had been named at the start. (Zulip's behaviour.)
- **An edit that removes a mention or a flag** is a retraction for that person, handled as above.
- **An edit that only changes the text** updates the row in place and shows "edited". It does not make a read row unread again. A kept row shows "edited since you kept it".
- **Reactions:** the grouped count follows the truth. When the last reaction is removed, the row is retracted as above.
- **A deleted message with replies** leaves its marker in the conversation for good, so the thread still has a top. Without replies, the marker holds the space for those looking at it and is gone afterwards.
- **Search:** a deleted message is no longer found; an edited one is found by its new text only.
- **Notifications:** one not yet sent is cancelled or carries the new text. One already sent cannot be taken back; its entry in the tray shows the current state ("message deleted"). A brief hold before sending, so that a quick delete or a typo fix produces nothing, is a number for the quality decisions (D-007).
- **On a phone-sized screen** the marker has the same height as what it replaces, and the expander for a longer edit never opens by itself.

Out, as known shortcuts: edit history (only "edited" and when); undoing a delete.

Locks in: every feed row, tray entry and search entry can be traced back to the message and its current version, and each has a "retracted" state as well as a "changed" one.

Reversibility: moderate. The states shape the feed's data and every receiver.

No second opinion was sought: nothing is built yet. T-008 (looking at what Slack really does) would inform this but does not block it.

## Decision

_Open._

## Consequences

_Filled in when accepted._

## Follow-ups

- With this accepted the product scope is complete. Next, per the user's order: agree the experiments, then the technical decisions (D-005 to D-008).
