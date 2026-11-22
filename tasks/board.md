# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** D-001 is accepted (2026-11-22): the feed sorts what deserves attention into three buckets, tuned by dials and by the sender's Flag or Info. D-009 (which surfaces the first version includes) is proposed and waiting on the user. After it: D-010 the seeded team, then D-002 agents, D-003 feed detail, D-004 edits and deletes. Order set by the user: product decisions first; technical ones (D-005 to D-008) wait until the product scope is final and the experiments are agreed. Text rules judged by a model are in scope but built last, after an experiment.

## Now

_Nothing in progress._

## Waiting on user

- **T-009** `decide` D-009: pick the scope of the first version. Recommended: B, the feed plus what makes it observable. Brief: `docs/visuals/D-009-first-version-surfaces.html`

## Next

_Empty until the first decision is made._

## Backlog

- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-004** `setup` Give the agent a browser it can drive, so interface work can be checked on the rendered page. Today none is connected or installed. Needed before the first user-facing task
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
