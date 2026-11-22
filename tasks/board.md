# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** The product scope is decided (2026-11-22). Accepted: D-001 the three-bucket feed; D-002 scripted participants; D-003 reading is enough, with Keep and Done; D-004 edits and deletes; D-009 the first-version surfaces; D-010 the large seeded team with a control panel; D-011 a flag has no state of its own; D-012 notifications follow the bucket; D-013 readings from E-001; D-014 reactions; D-015 what read means; D-016 history tabs. E-001 was rerun with all of them and checked independently twice: the rules hold together, and the notification default comes to about 29 a day in the simulated team. The architecture proposal (T-022) is drafted, tried out in E-003 (the design held; the notification hold was changed to the log's own clock; the PostgreSQL-specific part was checked in E-004 after the user chose PostgreSQL), and waits for the user: D-005 is accepted; D-006 to D-008 are proposed, D-017 and D-018 registered. The look-and-feel experiment runs in a separate session. Text rules judged by a model are in scope but built last, after an experiment.

## Now

_Nothing in progress._

## Waiting on user

- **T-031** `decide` Proposals for the four things still undesigned, shown in the prototype for the user to try: writing your own message (mention suggestions, editing in place, delete that asks first, removing a flag); what going wrong looks like (offline, a message that fails to send, search behind); search results; and two pages outside the product, `control-panel.html` and `notification-app.html`. All in `experiments/E-002-look-and-feel/`; the first three in `slack-like.html`, with "When things go wrong" on its Demo tab. Needs the user's yes or changes, then they are recorded
- **T-025** `decide` Smaller proposal points in D-023 still need the user's word: the arrivals button called "Show"; which swipe direction is which; the redrawn Notifications and Channel levels pages of settings. The points the user stated are accepted in D-023
- **T-022** `decide` The architecture is ready for review on one page, from the big picture down: `docs/visuals/T-022-architecture-proposal.html` (map, parts and their qualities, contracts, six decisions with options, the agent's smaller decisions, risks, evidence from E-003 and E-004). D-005 is accepted (2026-11-22): shape B (separate programs over the network, no message broker), PostgreSQL in Docker, WebSocket with numbered updates, TypeScript on the server; when a broker would become right is in `docs/visuals/D-005-message-broker.html`. Next question: D-017 and `docs/visuals/T-022-walkthroughs.html` (seven walk-throughs). D-006 to D-008 are proposed; D-017 and D-018 are registered. Next question to the user: D-017. Then D-018, D-006, D-007, D-008, one at a time. Detail: `tasks/T-022-architecture-draft.md`
- **T-021** `decide` Nine small behaviours the E-001 model still chooses on its own (list in `experiments/E-001-feed-rules-model/README.md`, "Still open after this run"). None blocks the look-and-feel work. The notification default was confirmed by the user at about 29 a day

## Next

- **T-029** `experiment` E-005, the live connection: numbered updates, resume after a drop, reload when too far behind, a silently lost update, many tabs, in a real browser. Asked by the user on 2026-11-22 to close the gap on the Conversations to Screen contract. Check: a screen cut off mid-burst ends equal to a fresh load


## Backlog

- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
