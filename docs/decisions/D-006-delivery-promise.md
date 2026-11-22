# D-006: The delivery promise to each receiver

- Status: proposed
- Level: contract
- Date: 2026-11-22
- Decided by: user (not yet decided)
- Depends on: D-005, D-017, D-018
- Evidence: R-002; the brief; second opinion and claim checks recorded in `tasks/T-022-architecture-draft.md`
- Visual: `docs/visuals/T-022-architecture-proposal.html`, section 8; walk-throughs in `docs/visuals/T-022-walkthroughs.html`

## Why this needs deciding now

The brief makes the guarantee part of the task: messages, edits and deletes must reach the search index, the notification sender and the feed although each can be slow or unavailable. What we promise about loss, repeats and order decides how every receiver is built and what the failure tests have to prove.

## Options

**A. Send once and hope.** After saving, call each receiver; on failure move on. Trivial. Every outage loses changes for good.

**B. Never lost, possibly repeated, in log order.** A delivery worker per receiver reads the change log from its own saved place, in order. An outage (no answer in time, a connection error, a server error): the worker waits, retries with growing pauses and resumes from its place, without limit; nothing is skipped. A refusal of one particular change (the receiver answers "I cannot take this"): after three tries the change goes to a visible parking place and the worker moves on. Each change carries the whole current message and its version; a receiver applies it only if it is newer, and a delete is remembered by its version so a late edit cannot bring the message back. Risk: telling "down" from "refuses this one" has to be exact, or a broken receiver stalls or parks everything; three different changes refused in a row are therefore treated as an outage. A parked change is shown with "try again", and the receiver counts as not level until the parking place is empty.

**C. Each change on its own, a few retries, then dropped** (Slack's Events API towards outside apps: three retries). A bad change never holds up others. Order is lost routinely, a longer outage loses changes, and a repair job is needed.

Not offered: exactly-once. No product promises it across a network (R-002).

## Best practice

Transactional outbox, at-least-once delivery, idempotent receivers, a rising version per message with a delete marker that wins, one cursor per receiver, timeouts, backoff with jitter, a dead-letter place, replay from the source (R-002). B is that practice.

## Recommendation

**B**, with this per receiver:

| Receiver | Is sent | Loss | Repeats | Order |
|---|---|---|---|---|
| Search index | finished messages: created, changed, deleted | none once saved | possible; ignored by version | in log order, not relied on; newest version wins, delete wins |
| Activity feed | every change, including what readers did (D-017) | none | possible; the feed takes only the change that follows the last one it applied (saved with its rows) and answers anything else with its position, so in effect once | strictly in log order and relied on; the feed never parks, it stops visibly and is fixed or rebuilt |
| Notification sender | due, changed, withdrawn from the feed's log (D-018) | none; may be late | possible; a named notification appears in the tray once, because the tray is the fake's own store. With a real provider the rule becomes "rather twice than never", passing the name as the provider's duplicate key where it has one | in log order; one already sent cannot be recalled, the tray shows the current truth |
| Agents | events that address them (D-008) | none within the kept period | possible; same number | in order per agent |

An unfinished (streaming) agent reply is never sent to search, the feed or notifications.

Locks in: the shape of a change (whole message plus version), a cursor per receiver, the version rule in every receiver. Reversibility: expensive.

Each claim and the failure a test injects for it is listed in D-007.

## Decision

Not yet taken.

## Consequences

- Easier: one sentence states the guarantee; a receiver can be rebuilt by replaying the log.
- Harder: every receiver must be written to ignore repeats and stale versions; the feed must apply a change and move its place in one step.
- Committed to, if accepted: at-least-once, in order per receiver, pause on outage, park on refusal.

## Follow-ups

- Contracts in `docs/contracts/`: core to search; core to feed; feed to notification sender.
- Guardrails with failure tests (see D-007).
