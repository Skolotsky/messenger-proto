# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** Accepted (2026-11-22): D-001 the three-bucket feed; D-009 the first-version surfaces; D-002 scripted participants; D-010 the large seeded team with a control panel; D-003 reading is enough, with Keep and Done; D-004 edits and deletes; D-011 a flag is a marker for each reader's feed with no state of its own; D-013 the readings E-001 needed. E-001 modelled the rules and found them sound apart from the hole D-011 closes. Remaining product decision: D-012, what sends a notification. Then the product scope is complete and the technical decisions (D-005 to D-008) can open. Text rules judged by a model are in scope but built last, after an experiment.

## Now

- **T-017** `experiment` Bring the E-001 model in line with D-011 and D-013 and rerun it, to confirm the hole is closed. Done when: `./experiments/E-001-feed-rules-model/run.sh` passes and the README states the new result

## Waiting on user

- **T-016** `decide` D-012: what sends a notification. The last product decision E-001 raised; to be prepared

## Next

_Empty until the first decision is made._

## Backlog

- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-004** `setup` Give the agent a browser it can drive, so interface work can be checked on the rendered page. Today none is connected or installed. Needed before the first user-facing task, and it must be able to show a phone-sized screen (D-009)
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
