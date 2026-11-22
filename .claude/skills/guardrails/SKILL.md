---
name: guardrails
description: Guardrails phase. Turn accepted decisions into enforceable checks before implementation (contracts between services, invariants, measurable quality targets, architecture rules), each tied to a test or hook that fails when it is broken. Use after decisions are accepted and before building, or when the user asks how a guarantee or quality will be protected.
---

# Guardrails

Scope: $ARGUMENTS

A decision the code can silently drift away from is only a wish. A guardrail makes the decision fail loudly when broken, and it is written before the implementation so the implementation has something to be measured against.

## Procedure

1. **Collect** the accepted decisions in scope (add a board item, `T-nnn` `guardrails`, and move it to `tasks/done.md` when the user has reviewed the list) and what each one promises: a contract between services, an invariant, a quality target, a structural rule.
2. **Write the contract** for each service boundary in `docs/contracts/` (format in `docs/contracts/README.md`). Plain language first, schema second.
3. **State each guardrail** as one checkable sentence a non-coder can read. For example: "a payment is never charged twice, even when the request is retried".
4. **Choose the strongest enforcement that fits,** in this order:
   1. impossible by construction (types, schema, a single code path)
   2. an automated test at the boundary, including the failure path
   3. an architecture or lint rule
   4. a hook or pre-commit check
   5. a scripted check the verifier runs

   An instruction in prose is not enforcement.
5. **Write the check first** and watch it fail for the right reason. Wire it into `./scripts/check`.
6. **Register** each guardrail in `docs/guardrails.md`: ID, type, sentence, source decision, enforced by, status. It stays `pending` until its check exists and runs.
7. **Review with the user.** Show the guardrail sentences and how each is enforced; a table in a visual works well. This is their review surface: they should recognise their decisions in it.

## Standards

- Qualities need numbers and conditions: "the page loads within one second at the 95th percentile with the seeded data", not "fast".
- Reliability guardrails test the failure: slow, down, duplicate, out of order. A test that passes only on the happy path proves nothing.
- Tests are deterministic: a controlled clock, seeded randomness, no sleeping and hoping.
- A rule that must hold for every input (nothing lost, nothing applied twice, order preserved) suits a property-based test better than a handful of examples.
- A failing check says how to fix the problem. The next reader is an agent.
- A guardrail check is never weakened to get a pass. Changing one requires a decision. To make that structural, add the guardrail test paths to `permissions.ask` in `.claude/settings.json`, so editing them needs the user's approval.
- Keep the list short. Enforce what must never be broken; preferences stay as prose. A guardrail earns its place by protecting something the user decided or the brief demands.
