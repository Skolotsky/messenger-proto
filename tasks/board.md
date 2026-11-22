# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** The product scope is decided (2026-11-22). Accepted: D-001 the three-bucket feed; D-002 scripted participants; D-003 reading is enough, with Keep and Done; D-004 edits and deletes; D-009 the first-version surfaces; D-010 the large seeded team with a control panel; D-011 a flag has no state of its own; D-012 notifications follow the bucket; D-013 readings from E-001; D-014 reactions; D-015 what read means; D-016 history tabs. E-001 was rerun with all of them and checked independently twice: the rules hold together, and the notification default comes to about 29 a day in the simulated team. The architecture proposal (T-022) is drafted, tried out in E-003, E-004 and E-005 (delivery, PostgreSQL numbering, the screen's catch-up and the agent door all held; the notification hold by the log's clock, a fence around a change the feed cannot apply, and Reset from a saved copy came out of them) and checked against the look-and-feel decisions D-019 to D-023 (they fit), and waits for the user: All technical decisions are accepted (the server language changed to Go, D-025; the architecture is ready for many organizations, D-026): D-005, D-017 and D-018 by the user, D-006 to D-008 by the agent on the user's delegation. Contracts and guardrails are written (T-034) and wait for the user's review; then building. The design phase is closed (T-035): the Slack-like prototype in E-002 is the agreed look and behaviour (D-019 to D-024). Text rules judged by a model are in scope but built last, after an experiment.

## Now

_Nothing in progress._

## Waiting on user

- **T-034** `guardrails` Seven contracts (`docs/contracts/`) and guardrails G-003 to G-026 are written (with the organization in every contract since D-026, and G-025, G-026 for keeping organizations apart and independent) from the accepted technical decisions; the user reviews the sentences in `docs/visuals/T-034-guardrails.html`. All product guardrails are `pending` until their tests exist. An independent verifier found four blockers and six larger gaps (delivery of notifications unprotected, a screen numbering that could not work with filtered updates, two boundaries without a contract, a guardrail whose check could pass while it was false); all answered, corrections not re-checked
- **T-021** `decide` Nine small behaviours the E-001 model still chooses on its own (list in `experiments/E-001-feed-rules-model/README.md`, "Still open after this run"). None blocks the look-and-feel work. The notification default was confirmed by the user at about 29 a day

## Next

- **T-038** `implement` Rules the prototype only approximates, kept as acceptance criteria for the build. Detail: `tasks/T-038-build-acceptance-from-e002.md`
- **T-039** `experiment` Rerun the E-001 feed-rules model with D-021 (Focus people) and D-023 point 26 (only Done releases). Until then its figures, such as about 29 notifications a day, predate these decisions



## Backlog

- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
