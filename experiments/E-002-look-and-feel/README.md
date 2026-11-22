# E-002: Which look and feel carries the three-bucket feed?

- Date: 2026-11-22
- Status: closed. The user chose the Slack-like direction (D-019) and signed off the design (T-035)
- Feeds: D-019 to D-024
- What the prototype only approximates: `tasks/T-038-build-acceptance-from-e002.md`

## Open it

Double-click, no server. Everything resets on reload except the theme.

- **`slack-like.html`: the agreed design.** The black Demo tab on the edge plays what other people do and what goes wrong. It is not part of the product.
- `control-panel.html`, `notification-app.html`: the two pages outside the product (D-010, D-022). Static pictures, connected to nothing.
- `telegram-like.html`, `own.html`: the directions not chosen, kept as a record. They share the behaviour but not the later layout work, and were not reviewed.

These are mock-ups: no stack was chosen here, and product code does not import from this folder.

## Hypothesis

The feed's rules (three buckets, a reason on every row, Kept, flags, one row per conversation, nothing jumps) can be carried by a familiar messenger layout, so the choice is about emphasis and habit, not whether the model fits.

## Method

One shared script (`src/core.js`) holds the content and behaviour; each direction adds a shell and a stylesheet. `build.py` inlines them into self-contained pages. `check.py` drives the pages in headless Chromium at 1280×800 and 390×844 and keeps screenshots in `screenshots/`. "Nothing jumps" is measured: the position of what is below a change, before and after.

Rerun everything, from the repo root: `./experiments/E-002-look-and-feel/run.sh`

## What would have changed our mind

A direction that could not show a listed behaviour without bending a decision, or could not hold "nothing jumps" at phone width.

## Result

- All three directions carried the model. Row height and reach differed: the Telegram-like one hid Needs you behind a tab and left one line of text; the agent's own put the explanation first but did not read as a messenger; the Slack-like one needed nothing explained.
- The agent recommended its own direction. **The user chose Slack-like:** "People used to it. We can reuse habits."
- The user then shaped the interface over many rounds. Every point is in D-019 to D-024; the largest changes were no expanded rows, threads beside the channel, one settings screen, and notifications outside the product.
- **674 checks pass** at both widths (`results.json`). After another person's edit or delete and after arrivals, what is on screen moves 0 px.
- **Independent verifier, three rounds.** The first found 6 blockers and 21 should-fix items; two rechecks found what the fixes had broken. Design-level findings are fixed, each with a regression check.

## Conclusion

The Slack-like layout carries the model, with the model showing through the reason line, the buckets and Kept. Confidence: good for layout and flows, which the user reviewed by hand.

### What this does not show

- **The rules are approximated.** Sorting after a change, read state across surfaces, arrivals and search are faked with a few lines, so some behave wrongly here. They are acceptance criteria for the build (T-038). Do not reuse this code.
- The fixes after the verifier's third round have regression checks but were not rechecked by it.
- No real phone, on-screen keyboard or installed web app; swipes were driven by a mouse drag. History tabs were not reviewed in dark.
- Switch account keeps Maya's messages. Counts in "Add to Focus" and the daily notification figures are written by hand or come from E-001, which predates D-021.
- No real user besides the person directing the work has used it.
