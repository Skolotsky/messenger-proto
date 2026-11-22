# D-019: The product is laid out like the work messenger people already know

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-003, D-009, D-016
- Evidence: E-002
- Visual: `experiments/E-002-look-and-feel/slack-like.html` (the chosen direction, clickable), beside `telegram-like.html` and `own.html`

## Why this needs deciding now

The product decisions say what the feed does, not what it looks like. Every user-facing task from here on needs a layout to build against, at two widths (D-009), and D-016 left open where the history tabs sit.

## Options

All three were built as clickable pages with the same content and behaviour, and checked at desktop and phone width (E-002).

### A. Slack-like

A sidebar of channels, a dense list, Activity as a view with the feed and the history tabs in one row of tabs, the conversation in a pane beside it.

- Upside: nothing to learn. Channels and their unread marks stay on screen, so it reads as a messenger.
- Risk: the feed is one tab among five, so the history tabs look like its siblings (the risk D-016 names). Every row weighs the same, so For your information looks as heavy as Focus. On a phone the feed is the second tab, not the first screen.

### B. Telegram-like

One list, large targets, the buckets as folders across the top, conversations as bubbles.

- Upside: the best of the three on a phone; the feed is the first screen.
- Risk: one line of about 35 characters per row, so nearly every row must be opened, and opening is what makes a row leave. Needs you is hidden behind a tab.

### C. The agent's own

The reason leads every row and a row's size follows its bucket.

- Upside: shows the model most clearly.
- Risk: does not read as a messenger at first glance; on a phone Needs you starts 1.2 screens down.

## Best practice

Keep the conventions people already have for everything that is not the bet, and spend novelty only where the product differs. Our bet is how the feed sorts (D-001), not how a messenger is laid out.

## Recommendation

The agent recommended C with a channel list from A and the phone habits of B. Reversibility: cheap now, moderate once the interface is built.

## Decision

Accepted by the user on 2026-11-22: **A, the Slack-like direction.** "People used to it. We can reuse habits."

With it, one addition to the history tabs (D-016): **each history tab shows a count of its unread entries, and an unread entry is marked.** "I don't see unread count and markers in history tabs, I would expect to have such." This fits D-016 as written (the tabs hold read and unread together, and nothing leaves them by being read) and D-015 (one read state): opening an entry reads it, the mark and the count go, the entry stays.

The agent chose against, so the risk is stated once: the three strains listed under A remain, and a count on every history tab makes the tabs look more like inboxes beside the feed. What is done about it inside this direction, as the agent's choice of detail: the history counts are grey and small, and only the feed's count is coloured, so one number calls for attention.

### Added by the user, 2026-11-22 (after acceptance; the decision is unchanged)

On a phone the first screen is Activity, "with an easy way to switch to channels of course". The bottom bar is Activity, Channels, Settings.

## Consequences

- Easier: the layout needs no explaining, and the channel list, unread marks and the conversation pane come with habits attached. The Telegram-like and the agent's own pages stay in the experiment folder as a record and are not developed further.
- Harder: the model has to show itself through small things inside a familiar frame: the bucket headings, the reason line on each row, the Kept section. The same item can now be counted twice on one screen, once in the feed's count and once on a history tab.
- Committed to: a sidebar of channels and direct messages; Activity as a view holding the feed and the four history tabs; the conversation beside the feed on a wide screen and over it on a phone; unread counts and marks in the history tabs.

## Follow-ups

- Still to confirm with the user (from D-016): the history tabs replace the single Earlier list.
- The mock-up's shell and styles (`experiments/E-002-look-and-feel/src/slack-like.*`) are a reference for the look, not code to reuse (the experiment rule).
