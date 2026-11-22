# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** Accepted (2026-11-22): D-001 the three-bucket feed; D-009 the first-version surfaces; D-002 scripted participants; D-010 the large seeded team with a control panel; D-003 reading is enough, with Keep and Done; D-004 edits and deletes; D-011 a flag is a marker for each reader's feed with no state of its own; D-013 the readings E-001 needed. E-001 modelled the rules and found them sound apart from the hole D-011 closes. Also accepted: D-014 reactions (one from someone you addressed is an answer) and D-015 what read means (one shared read position). Remaining product decision: D-012, what sends a notification, proposed and waiting on the user. Then the product scope is complete and the technical decisions (D-005 to D-008) can open. Text rules judged by a model are in scope but built last, after an experiment.

## Now

_Nothing in progress._

## Waiting on user

- **T-016** `decide` D-012: pick what sends a notification. Recommended: B, Focus and Needs you notify, with a personal switch to Focus only. Record: `docs/decisions/D-012-what-sends-a-notification.md`

## Next

- **T-018** `experiment` Rerun E-001 with D-012, D-014 and D-015 in the model: measure how many rows "a reaction from someone you addressed" adds to Needs you and how many notifications a person gets a day; have a fresh verifier check the model again. Starts once D-012 is accepted
- **T-019** `experiment` E-002, look and feel: clickable mock-ups of the feed at desktop and phone width, comparing two or three ways to lay out the three buckets. Needs T-004 (a browser the agent can drive) to check what it draws

## Backlog

- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-004** `setup` Give the agent a browser it can drive, so interface work can be checked on the rendered page. Today none is connected or installed. Needed before the first user-facing task, and it must be able to show a phone-sized screen (D-009)
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
