# D-032: Second corrections to the contracts, and the schemas settle the shapes

- Status: accepted
- Level: contract
- Date: 2026-11-22
- Decided by: user
- Depends on: D-004, D-008, D-010, D-012, D-014, D-015, D-023, D-029, D-031
- Evidence: second independent check of the contracts, 2026-11-22 (14 blockers, 11 gaps, 6 minor), worked through concrete sequences; its product citations re-read against D-010, D-014, D-015, D-029 and T-038
- Visual: `docs/visuals/D-032-second-contract-corrections.html`

## Why this needs deciding now

D-031's corrections are in the contracts, but running sequences through them shows guarantees that still break: an update can be silently lost when a second view loads, Reset leaves screens ignoring every new update, and seeding would send about 12,000 old notifications. Four of the fixes change what a person sees, so they are the user's. T-045 waits on them.

## The four choices

### 1. Does opening a channel also read its threads?

- **A (recommended).** No. Opening a channel reads the channel; each thread is read when it is opened. Follows D-015 ("nothing is read while the person is not looking"): replies are not shown inline (D-023), so a person opening the channel has not seen them. Risk: a thread row stays in the feed until opened or marked read. Changes T-038 point 6.
- **B.** Yes, as T-038 point 6 says today. Upside: one click clears everything. Risk: replies the person never saw count as read and leave the feed.

D-031's wording in `docs/contracts/README.md` already assumed A; this choice makes that explicit, because the user was asked only about opening a thread.

### 2. Does a reply in a thread mark its channel unread in the sidebar?

- **A (recommended).** No. Replies show in Activity and the Threads tab. This is the habit of the work messenger people know (D-019). Risk: someone who ignores Activity can miss a reply.
- **B.** Yes. Upside: nothing goes unnoticed in the sidebar. Risk: busy threads keep channels lit, and opening the channel would not clear the mark (choice 1).

### 3. What a stopped reply keeps if the server crashes mid-reply

- **A (recommended).** The text so far is saved about once a second, outside the log. A crash loses at most the last second. A reply with no saved text is closed with no message. Risk: a small write per active reply.
- **B.** Text stays in memory. Upside: no writes. Risk: after a crash the reply is lost, breaking "closed with the text so far" (G-017).

### 4. Do the seeded two weeks appear in the notification app?

- **A (recommended).** No. Seeded history creates no notifications, and the app starts empty. Every entry it shows was really sent while the demonstration ran, which makes the delivery guarantees visible. Risk: the app looks empty for the first moments.
- **B.** The app starts with a written history of past notifications, marked as sent. Upside: looks lived-in at once. Risk: entries the system never sent, and a second path into the sender's list.

## Corrections with one sensible answer

Accepting this record accepts them.

- **Screen numbering.** Only the first full load sets a stream's position; later views never move it.
- **Reset.** One rising reset number per organization on live connections, agent events and delivery places; a change forces a fresh load.
- **Editions** are rising integers per organization. The sender's worker keeps the edition with its place and restarts at 0 when it changes; a reset drops held notifications, and the replay recreates them by name.
- **A refused note** stops that organization's notes until it is retried, so a parked note can never let a notification past its hold.
- **Search wipes** are detected: the index returns a wipe number in every answer, and a change sends the worker back to the start.
- **Acting implies read.** The act writes its fact plus a `reader.read` in one step and answers with the fact's number. Per D-014, reacting, keeping or finishing reads the whole row.
- **Watched arrivals.** Conversations writes the read in the same step as the arrival, while someone is watching that row.
- **Direct conversations** can be opened from both doors, once per pair.
- **A rebuild after a rule change** lists notifications from before the rebuild as history and does not send them. An ordinary rebuild keeps the "never lost" rule.
- **A sender wipe** keeps each sent name with its time and "delayed" mark. G-020 is reworded.
- **Seeding** is a session: opened, filled in batches, sealed.
- **Fences** cover a reader in one conversation; a fault on a reader-wide change counts as organization-wide.
- **A stopped reply** is For your information at any dial except Off, and never notifies.
- **Message** gains `number`, `created_at`, `edited_at`, `deleted` and `participants`. A streamed reply's number is that of its `message.created`.
- **Build plan.** Every operation in `operations-and-control.md` gets an owning package.

**The shapes move into the schemas.** The full list of screen update parts, the feed row, history entries, settings and rule vocabulary are defined in T-045 as schemas with examples, which are checked by machine. The independent check then reviews those schemas, not a third round of prose.

## Best practice

Replies and channels are read separately in Slack-style products. Reset and rebuild use a monotonic epoch or generation so stale consumers can detect them, the standard fencing technique. Data shapes are pinned in machine-checked schemas rather than prose.

## Recommendation

A on all four, plus the corrections. Reversibility: cheap now, moderate after T-045.

## Decision

The user accepted the recommended answer on all four on 2026-11-22 ("Accept all four"): 1A opening a channel does not read its threads; 2A a reply does not mark its channel in the sidebar; 3A the text of a reply in progress is saved about once a second; 4A seeded history creates no notifications. The corrections with one sensible answer are accepted with them.

## Consequences

If accepted: the contracts, G-015, G-017, G-020 and T-038 point 6 are corrected with this record cited; T-045 starts.

## Follow-ups

T-063 carries this through. T-045's schemas get an independent check before Wave 1.
