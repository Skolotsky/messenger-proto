# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** Accepted so far (2026-11-22): D-001, the three-bucket feed with dials, Flag and Info; D-009, the feed plus what makes it observable, as a PWA, with creating channels; D-002, scripted people and agents, and a coding agent can join as either for verification; D-010, a large seeded team (about 30 people, 6 agents) with a control panel for the demonstration. D-003 (how an item leaves the feed) is proposed and waiting on the user. After it: D-004 edits and deletes, the last product decision. Order set by the user: product decisions first; technical ones (D-005 to D-008) wait until the product scope is final and the experiments are agreed. Text rules judged by a model are in scope but built last, after an experiment.

## Now

_Nothing in progress._

## Waiting on user

- **T-012** `decide` D-003: pick how a row leaves the feed. Recommended: B, done by hand or by acting. Brief: `docs/visuals/D-003-how-items-leave-the-feed.html`

## Next

_Empty until the first decision is made._

## Backlog

- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-004** `setup` Give the agent a browser it can drive, so interface work can be checked on the rendered page. Today none is connected or installed. Needed before the first user-facing task, and it must be able to show a phone-sized screen (D-009)
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
