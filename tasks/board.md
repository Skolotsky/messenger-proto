# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** The product scope is decided (2026-11-22). Accepted: D-001 the three-bucket feed; D-002 scripted participants; D-003 reading is enough, with Keep and Done; D-004 edits and deletes; D-009 the first-version surfaces; D-010 the large seeded team with a control panel; D-011 a flag has no state of its own; D-012 notifications follow the bucket; D-013 readings from E-001; D-014 reactions; D-015 what read means; D-016 history tabs. E-001 was rerun with all of them and checked independently twice: the rules hold together, and the notification default comes to about 29 a day in the simulated team. The architecture proposal (T-022) is drafted, tried out in E-003, E-004 and E-005 (delivery, PostgreSQL numbering, the screen's catch-up and the agent door all held; the notification hold by the log's clock, a fence around a change the feed cannot apply, and Reset from a saved copy came out of them) and checked against the look-and-feel decisions D-019 to D-023 (they fit), and waits for the user: All technical decisions are accepted (the server language changed to Go, D-025; the architecture is ready for many organizations, D-026; a message broker was tried in E-006 and dropped again, D-027 and D-028): D-005, D-017 and D-018 by the user, D-006 to D-008 by the agent on the user's delegation. Contracts and guardrails are written (T-034) and wait for the user's review. The build plan is written (T-043): T-044 to T-062 under Next. The design phase is closed (T-035): the Slack-like prototype in E-002 is the agreed look and behaviour (D-019 to D-024). Text rules judged by a model are in scope but built last, after an experiment.

## Now

- **T-047** `implement` Wave 1 · Delivery workers (G-012, G-014, G-023, G-026, G-029). Needs T-045
- **T-048** `implement` Wave 1 · Search and notification stand-ins (G-006, G-010, G-011). Needs T-045
- **T-049** `implement` Wave 1 · The feed's rules as a pure package (G-030; T-038 points 1 to 19). Needs T-045
- **T-050** `implement` Wave 1 · Web app: the frame, the look, a conversation, against the mock door. Needs T-045
- **T-051** `implement` Wave 1 · The seeded organization as data, with D-010's situations. Needs T-045


## Waiting on user

- **T-034** `guardrails` Seven contracts (`docs/contracts/`) and guardrails G-003 to G-030 are written (G-027 to G-030 for growing by adding copies and databases and for one large organization, D-029 and D-030) (with the organization in every contract since D-026, and G-025, G-026 for keeping organizations apart and independent) from the accepted technical decisions; the user reviews the sentences in `docs/visuals/T-034-guardrails.html`. All product guardrails are `pending` until their tests exist. An independent verifier found four blockers and six larger gaps (delivery of notifications unprotected, a screen numbering that could not work with filtered updates, two boundaries without a contract, a guardrail whose check could pass while it was false); all answered, corrections not re-checked
- **T-021** `decide` Nine small behaviours the E-001 model still chooses on its own (list in `experiments/E-001-feed-rules-model/README.md`, "Still open after this run"). None blocks the look-and-feel work. The notification default was confirmed by the user at about 29 a day

## Next

- **T-052** `implement` Wave 2 · The feed as a service (G-007, G-009, G-008). Needs T-049
- **T-053** `implement` Wave 2 · Conversations: live connection, reading through the door, housekeeping (G-024; server half of G-015, G-019). Needs T-046
- **T-054** `implement` Wave 2 · Conversations: the agent door (G-016, G-017, G-018). Needs T-046
- **T-055** `implement` Wave 2 · Web app: Activity, history tabs, settings. Needs T-050
- **T-056** `implement` Wave 2 · Web app: writing, going wrong, search, staying current (screen half of G-015). Needs T-050
- **T-057** `validate` Wave 3 · Milestone 1: the spine runs end to end; delivery guardrails with two copies and two databases (G-013, G-025, G-027, G-028); independent verifier. Needs T-046, T-047, T-048, T-052
- **T-058** `validate` Wave 3 · Milestone 2: the web app on the real programs at both widths (G-015, G-019, all of T-038); independent verifier. Needs T-053, T-055, T-056, T-057
- **T-059** `implement` Wave 3 · The demonstration: scripted team, control panel, notification app, reset (G-020). Needs T-051, T-053, T-054
- **T-060** `implement` Wave 3 · A coding agent can join through the agent door (D-002). Needs T-054
- **T-061** `validate` Wave 4 · Milestone 3: validation of the running product against the brief, contracts and guardrails; how to run it; known shortcuts up to date. Needs all above
- **T-062** `experiment` Wave 4 · Text rules judged by a model: the experiment, then the build (D-001). Last
- **T-038** `implement` Rules the prototype only approximates, kept as acceptance criteria for the build. Detail: `tasks/T-038-build-acceptance-from-e002.md`



## Backlog

- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
