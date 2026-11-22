# D-018: The feed decides that a notification is due; the sender only delivers

- Status: accepted
- Level: contract
- Date: 2026-11-22
- Decided by: user
- Depends on: D-005, D-017; D-012 (what notifies), D-015 (read), D-022 (where notifications are shown)
- Evidence: E-003 (0 of 160 notifications sent for something read or deleted within the hold, with the feed stopped or overloaded; the sender's list refilled to the same 565 entries; no repeat after a feed rebuild); R-002 (Slack separates deciding from delivering)
- Visual: `docs/visuals/T-022-architecture-proposal.html`, section 7

## Why this needs deciding now

The brief lists the notification sender beside the feed, as an independent system that receives messages. D-012 says a notification is sent when a feed row becomes unread in Focus or Needs you, once per row, and once more when it moves up into Focus. Only the feed knows that. The two have to be reconciled before the delivery contract (D-006) can name what the sender is sent.

## Options

### A. The feed decides; the sender delivers

When a row becomes unread in a bucket that notifies, the feed writes "due" to its own numbered log; later it may write "changed" or "withdrawn". A delivery worker carries these to the sender with the same promise as any other change. Upside: the rule lives in one place; the feed and the notification app cannot disagree. Risk: the sender stands behind the feed, not beside it; while the feed is down, notifications wait and then arrive late. Reversal: moderate.

### B. The sender receives messages and applies the rules itself

Three receivers side by side, as the brief draws it. Upside: the sender does not depend on the feed. Risk: the feed's rules, and every reader's dials and read state, exist twice and drift; "one per row" cannot be told from messages alone. Reversal: moderate.

### C. A, and deletes also go straight to the sender

Upside: a deleted message is not announced even while the feed lags. Risk: two sources to reconcile for a rare case; E-003 showed A alone sends nothing for what was read or deleted within the hold. Reversal: cheap.

## Best practice

Separate deciding what deserves attention from delivering it (Slack's 2026 notification rebuild, R-002). A is that separation.

## Recommendation

**A.** It is the only option in which "one per row, again on moving into Focus, never for Info" has a single home, and it is what the experiments tested.

## Decision

Accepted by the user on 2026-11-22: **A.** ("Yes, the feed.")

What goes with it, from the experiments:

- Every notification has a fixed name (person, row, the change that caused it); the sender keeps the list of names it has sent and sends a name once.
- The hold before sending is measured by the log's own clock: the core writes clock ticks into the log, the feed passes them on, and the sender sends only once something stamped later than the end of the hold has reached it. Log timestamps never decrease.
- The personal "Focus only" switch (D-012) is a reader fact the feed applies before writing "due".
- The sender's list is shown in the separate mock notification app (D-022), which reads the sender directly.

This departs from a literal reading of the brief, which draws the sender as a receiver of messages beside the feed. The sender is still an independent system behind a network boundary that can be slow or unavailable; what it receives is the feed's decisions, not raw messages. The departure is stated wherever the design is presented.

## Consequences

- Easier: one rule, one place; a rebuilt feed never notifies twice; the notification app can be refilled exactly.
- Harder: notifications depend on the feed being up; the feed needs its own numbered log and a worker reading it.
- Committed to: the sender never judges; the feed's notes ("due", "changed", "withdrawn", clock ticks) are a contract.

## Follow-ups

- Contract: Activity feed to Notification sender (`docs/contracts/`, after D-006).
- Guardrails: a name is sent once; nothing is sent for what was read or deleted within the hold (the tests exist in E-003 and are rewritten for the product).
