# D-031: Corrections to the contracts before they become schemas

- Status: proposed
- Level: contract
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-004, D-006, D-007, D-008, D-012, D-014, D-015, D-017, D-018, D-026, D-029, D-030
- Evidence: independent re-check of the contracts and guardrails, 2026-11-22 (12 blockers, 11 gaps, 5 minor); its main citations re-read against the decisions, E-001 and E-003
- Visual: `docs/visuals/D-031-contract-corrections.html`

## Why this needs deciding now

T-045 turns the seven contracts into the schemas every later package builds on. The re-check found that some cannot be implemented as written: reading one thread has no way to say which thread, a notification can go out for something already read, and the 8-second notification promise cannot hold. Fixing them now costs a few sentences; after T-045 each fix spreads through several packages. Accepted contracts change only through a decision, so this record is that decision.

## The five choices

### 1. What a feed row is, and what "read" points at

- **A (recommended).** A row is a conversation plus an optional thread. Each channel, each thread and each direct conversation has its own read position. Upside: matches D-015 ("reading a thread marks its feed row read") and the E-001 model; reading one thread leaves the rest of the channel unread. Risk: more positions per person.
- **B.** One position per channel. Upside: simplest. Risk: opening one thread marks the whole channel read up to that point, so people miss messages.

### 2. How the 8-second notification promise is kept

- **A (recommended).** Keep 8 s. After the last change, the log gets a small clock entry about once a second for up to 6 s. Upside: the promise stays as decided. Risk: up to six extra small entries each time an organization goes quiet; none while it is idle, so G-029 is unaffected.
- **B.** Raise the promise to about 17 s. Upside: no extra entries. Risk: a mention can feel slow to reach a phone.

### 3. Who can find a direct message in search

- **A (recommended).** Only its two participants. Upside: search is useful and private. Risk: the search stand-in must filter correctly, which G-025's style of test covers.
- **B.** Direct messages are not searchable. Upside: nothing to filter. Risk: people cannot find what was said to them, and D-016's history is the only way back.

### 4. What survives wiping the notification sender

- **A (recommended).** The list of names already sent survives a wipe, as a real provider's delivery history would. Upside: "sent once, never lost" holds with no exception. Risk: a wipe is not quite from nothing; Reset restores this list from its saved copy.
- **B.** A wipe clears everything, and anything due over 10 minutes ago is listed as history without being sent (today's text). Upside: a true blank start. Risk: an unsent notification is lost and a recent one can be sent twice, which contradicts G-010.

### 5. What happens to an agent's reply that stopped halfway

- **A (recommended).** It is saved with the text so far, marked "stopped", is searchable, and gives people in the thread a For your information row, which never notifies (D-012). Upside: whoever was waiting can see the agent gave up. Risk: a little noise in the feed.
- **B.** It stays visible in the thread only. Upside: no noise. Risk: someone waiting on the agent is never told.

## Corrections with one sensible answer

These follow from accepted decisions; accepting this record accepts them.

- **Everything the feed needs is on the log.** Every log kind is listed with its payload, including reactions, channel creation, people added, direct-conversation participants and whether an author is an agent. A reaction does not change a message's edit version. The D-023 setting "a reply or reaction releases a kept row" becomes a reader-wide change.
- **Screen updates.** One update per number, carrying a list of parts. Every fresh load returns the reader's last number for its stream, read together with the content. `last_sent` carries the edition.
- **Watching** is said only over the live connection, with defined start and stop messages and a log entry of person, conversation and connection.
- **A fence withdraws** the held notifications it covers (`because: "fenced"`), and set-aside changes count as applied for clock notes, so one fence cannot freeze an organization's notifications. A change that raises a fence is answered 200 with `fenced: true`.
- **New edition.** A note from a new edition resets the sender's place and clock; names sent stay remembered; a note from an older edition is ignored.
- **Search.** Lag is measured from the search worker's place, so quiet channels show no false notice. The event carries thread, edited and, for direct messages, participants.
- **Missing boundaries get a contract:** the nudge; asking a receiver for its place; wipe, rebuild, Reset, parked list, try again, Running/Slow/Off; the notification app; the control panel; seeding history with past times; the screens' own read calls.
- **The agent door** gains a conversation on posts, the posted message's ID in the answer, reading a channel, the "everything in a channel" subscription, and reacting.
- **Acting implies read:** Conversations moves the read position in the same step as a reply, reaction, Keep or Done.
- **The log and replies:** the feed receives a text-free "reply started" and ignores it, so numbering has no holes. G-017 is reworded to say no reply text reaches a receiver.
- **IDs:** an action's ID is unique per organization and caller; the same ID with a different body is refused.
- **Text positions** count Unicode code points, and the search example is corrected.
- **The feed has two numbered logs** (row updates, notification notes) sharing one edition; G-005's check extends to both. `through` means "includes everything up to here, and possibly later changes".
- **Delivery's database rights:** the `delivery` login may read the two logs it delivers and write its own places, table by table. G-008 is reworded to say so.
- **Notes:** an organization-scoped refusal for a bad note (as search has), one organization per batch, and G-018's stated exception when a copy restarts.

## Best practice

Contract-first teams fix ambiguity before generating code from a schema, give each stream one consumer position, make idempotency keys explicitly scoped, and keep a provider's delivery record outside anything a consumer can reset. All five recommendations follow that. No second-opinion agent was run: the corrections come from an independent reviewer, and nothing has been built on them yet, so each is cheap to change now.

## Recommendation

A on all five, plus the corrections above. Reversibility: cheap now, moderate after T-045.

## Decision

_Waiting for the user._

## Consequences

If accepted: the contracts and G-005, G-008, G-009, G-010, G-011, G-017 and G-018 are corrected with this record cited, a fresh verifier checks the result, and T-045 starts.

## Follow-ups

T-063 (this decision). Superseded details in D-007 and D-029 are noted here rather than edited: the tick timing and watching renewal in D-029 and the contracts stand.
