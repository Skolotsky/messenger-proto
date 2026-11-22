# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** Research into existing products is done (R-002); eight decisions are `open` in the decision index. Order set by the user on 2026-11-22: product decisions first (D-001 to D-004); technical ones (D-005 to D-008) are not opened until the product scope is final and the experiments are agreed. Now asking the first question of D-001: who the prototype is for and how it is judged.

## Now

_Nothing in progress._

## Waiting on user

- **T-007** `decide` D-001, first question: who looks at the prototype, how it reaches them, and by when. Asked 2026-11-22. Overview of all decisions: `docs/visuals/R-002-decision-landscape.html`

## Next

_Empty until the first decision is made._

## Backlog

- **T-002** `research` Kickoff: confirm the context of the brief with the user (who evaluates it, by when, how it will be shown) before scoping anything. Becomes the first questions of D-001
- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-004** `setup` Give the agent a browser it can drive, so interface work can be checked on the rendered page. Today none is connected or installed. Needed before the first user-facing task
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
