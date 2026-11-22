# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** The user kicked off on 2026-11-22. Research into existing products is done (R-002) and eight decisions are identified as `open` in the decision index. Nothing is decided or built yet. Next: the user picks the first decision to open; the agent recommends D-001, then D-002.

## Now

_Nothing in progress._

## Waiting on user

- **T-007** `decide` Which decision to open first. Recommended: D-001 scope and audience, then D-002 the role of AI agents. Overview: `docs/visuals/R-002-decision-landscape.html`

## Next

_Empty until the first decision is made._

## Backlog

- **T-002** `research` Kickoff: confirm the context of the brief with the user (who evaluates it, by when, how it will be shown) before scoping anything. Becomes the first questions of D-001
- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-004** `setup` Give the agent a browser it can drive, so interface work can be checked on the rendered page. Today none is connected or installed. Needed before the first user-facing task
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
