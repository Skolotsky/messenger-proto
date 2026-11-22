# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** Research is done (R-002). Audience set by the user: small to mid-size startup teams, mixed roles, heavy AI use; the PoC exists to launch a closed early access programme. D-001 (the product hypothesis) is proposed: option A with a third bucket, Focus, added by the user; flags and the starting rules are written in; it waits for the user to accept it. Order set by the user: product decisions first (D-001 to D-004, D-009); technical ones (D-005 to D-008) wait until the product scope is final and the experiments are agreed. Text rules judged by a model are in scope but built last, after an experiment.

## Now

_Nothing in progress._

## Waiting on user

- **T-007** `decide` D-001: accept the record as it now stands (three buckets, flags, starting rules with direct mentions in Needs you). Brief: `docs/visuals/D-001-product-hypothesis.html`

## Next

_Empty until the first decision is made._

## Backlog

- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-004** `setup` Give the agent a browser it can drive, so interface work can be checked on the rendered page. Today none is connected or installed. Needed before the first user-facing task
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
