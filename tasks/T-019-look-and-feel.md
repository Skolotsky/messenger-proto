# T-019: E-002, look and feel of the feed

- Phase: experiment
- Asked by the user on 2026-11-22, to be done in a separate session from the one that prepared it
- Depends on: T-004 (a browser the agent can drive); decisions D-001, D-003, D-004, D-009, D-010, D-011, D-012, D-013, D-014, D-015, D-016

## Outcome

The user can click through three versions of the same product moment and say which direction to build, or what to take from each.

Three directions, set by the user:

1. **Slack-like.** The conventions people know from a work messenger: sidebar of channels, dense message list, activity as a view.
2. **Telegram-like.** The user likes Telegram's style and wants to see whether it carries a work product: chat list as the main surface, large touch targets, a phone-first feel.
3. **The agent's own.** Whatever shows our model at its best: three buckets, reasons on rows, Kept, flags, one row per conversation.

Borrow layout patterns, density and interaction habits. Do not copy logos, product names, icon sets or brand colours of either product.

## What each version must show

The same content in all three, from the seeded team (D-010), viewed as the product manager:

- The feed with its three buckets, each row with its reason, and Focus with New and Kept (D-001, D-003, D-011).
- One row per conversation, expanded in place, with reply and react from the row (D-003).
- A flagged row that names the viewer, read and therefore kept; Done on it (D-011).
- An agent: a result, an Info progress update in For your information, and a flag asking for approval (D-001, D-002).
- A reaction that is an answer, and reactions shown on a row (D-014).
- A message that changes while on screen: an edit to a longer text behind an expander, and a delete leaving a marker in the same space, with nothing jumping (D-001, D-004).
- "3 new" at the top while the list stays still (D-003).
- An empty bucket (D-003).
- The four history tabs and where they sit (D-016).
- The channel dial and the thread dial with Follow (D-001), and adding a Focus rule with its count (D-001).
- A conversation view next to or behind the feed, with a deleted message that has replies (D-004).
- The notification tray (D-009, D-012).

## Acceptance criteria

1. Three self-contained pages under `experiments/E-002-look-and-feel/`, one per direction, each clickable through the list above with realistic content, no placeholder text.
2. Each works at a phone width (about 390px) and at desktop width, in light and dark.
3. Each was looked at rendered, at both widths, by the agent, with screenshots kept in the experiment folder.
4. The experiment's README states for each direction what it does well for our model and where it strains, and gives a recommendation.
5. `./scripts/check` passes.

## Verification plan

- Open each page in the browser the agent drives; click every listed interaction; capture desktop and phone screenshots.
- Check "nothing jumps" by measuring the position of the row below an edited or deleted message before and after.
- The user reviews the three pages. Their choice becomes a decision record.

## Notes for the session that picks this up

- The user approved running a headless browser in Docker on 2026-11-22, in the session that prepared this task. That approval was for that conversation: say what will be downloaded and confirm once before pulling an image.
- No technical decision is needed or implied. The pages are mock-ups, not the start of the product; do not choose a front-end stack here.
- The history tabs (D-016) include the agent's reading that they replace the Earlier list. Show it that way and ask the user to confirm.
- Names and wording of the seeded team are the agent's to write (D-010).

## Log

- 2026-11-22: task prepared. Not started.

## Evidence

_None yet._
