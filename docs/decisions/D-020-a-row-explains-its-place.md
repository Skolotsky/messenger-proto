# D-020: The reason line on a feed row opens why it is there and what to change

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-019
- Evidence: E-002 (three variants built into the Slack-like page: `experiments/E-002-look-and-feel/slack-like.html#why=a`, `#why=b`, `#why=c`)
- Visual: `experiments/E-002-look-and-feel/screenshots/why-a-desktop.png`, `why-b-desktop.png`, `why-c-desktop.png`

## Why this needs deciding now

D-001 commits us to explained rules: every row says why it is where it is. The user asked for two things on top of the one-line reason: "I want to answer why it is there" and "I may want to not put something like this into focus".

## Options

### A. The reason line is the control

Clicking the reason on a row opens a small panel on the row: what the item is, what put it in the bucket, where it would go otherwise, and one button that changes the deciding rule or dial.

- Upside: one click, on the row, and it does not read the row.
- Risk: the reason line has to look clickable.

### B. Inside the expanded row

- Upside: no new control.
- Risk: expanding reads the row (D-015), so asking why makes the row leave.

### C. A "Why? Change…" link on every row, opening a side panel with the whole path through the buckets

- Upside: the fullest explanation.
- Risk: a link on every row is noise, and the answer appears far from the row.

## Best practice

Put the explanation where the question arises, and let it lead straight to the control that changes the outcome.

## Recommendation

**A.** Reversibility: cheap.

## Decision

Accepted by the user on 2026-11-22: **A.** "Yes, A is good enough."

- The panel never reads the row.
- What the button does follows from what decided the row's place: a person (D-021), a rule, or a channel dial. It shows how many items the change would have moved in the last 7 days and where they would have gone.
- A change made from a row acts on the narrowest thing that explains the row, at once, says what it did and offers Undo. It does not ask first: the click is the intent (the user's rule, stated for direct messages in D-021).
- The row itself does not move while it is looked at (D-003); the change applies the next time the feed is opened (D-013).

## Consequences

- Easier: "explained rules" has one place in the interface.
- Harder: every way a row can land in a bucket needs its three plain sentences and its one change.
- Committed to: the reason line as a control on every feed row.

## Follow-ups

- The mock-up shows it for Focus rows; the wording for Needs you and For your information rows is still to write.
