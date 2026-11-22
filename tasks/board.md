# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** The product scope is decided (2026-11-22). Accepted: D-001 the three-bucket feed; D-002 scripted participants; D-003 reading is enough, with Keep and Done; D-004 edits and deletes; D-009 the first-version surfaces; D-010 the large seeded team with a control panel; D-011 a flag has no state of its own; D-012 notifications follow the bucket; D-013 readings from E-001; D-014 reactions; D-015 what read means; D-016 history tabs. E-001 was rerun with all of them and checked independently twice: the rules hold together, and the notification default comes to about 29 a day in the simulated team. The architecture proposal (T-022) is drafted and waits for the user: D-005 to D-008 are proposed, D-017 and D-018 registered, none taken. The look-and-feel experiment runs in a separate session. Text rules judged by a model are in scope but built last, after an experiment.

## Now

_Nothing in progress._

## Waiting on user

- **T-025** `decide` Many messages in one row, context, and going straight to the conversation. Proposal shown in `experiments/E-002-look-and-feel/slack-like.html` (Anna's row has ten new messages): the row shows the latest message and the count; expanding shows the message before the new ones (or the start of the thread), then the new ones six at a time, and only what was shown counts as read; an arrow on every row opens the conversation at the first new message in one click. Needs the user's yes or changes; then it is recorded as an addition to D-003 and D-015
- **T-022** `decide` The architecture proposal is ready for review: `docs/visuals/T-022-architecture-proposal.html` (system map, six technical decisions with options, payloads and failure tests) and `docs/visuals/T-022-walkthroughs.html` (seven walk-throughs). D-005 to D-008 are proposed; D-017 and D-018 are registered. First question to the user: the shape (D-005). Then D-017, D-018, D-006, D-007, D-008, one at a time. Detail: `tasks/T-022-architecture-draft.md`
- **T-021** `decide` Nine small behaviours the E-001 model still chooses on its own (list in `experiments/E-001-feed-rules-model/README.md`, "Still open after this run"). None blocks the look-and-feel work. The notification default was confirmed by the user at about 29 a day

## Next


## Backlog

- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
