---
name: implement
description: Implementation phase. Build one task to its acceptance criteria - agree what done looks like, write the checks, implement in small steps, verify against the real system, record evidence, commit. Use when the user asks to build, implement, fix or change product code, or to pick up the next task from the board.
argument-hint: "<task ID, or what to build>"
---

# Implement

Task: $ARGUMENTS

If the line above is empty, take the first item under Next on the board.

## Before coding

1. **Find the ground.** Read the task's line on the board, the decisions and guardrails it traces to, and the contracts it touches. If the task rests on an undecided product, contract or quality question, stop and prepare that decision first (the `decide` skill).
2. **Write down what done looks like.** For a task that touches a contract, a guarantee or a user-facing flow, create `tasks/T-nnn-<slug>.md` from `.claude/skills/implement/task-template.md`: outcome, acceptance criteria (each with how it is checked), verification plan. For a smaller task, put the acceptance check in its board line. Move the task to Now.
3. **Say the plan** to the user in a few lines: the outcome, how it will be proven, anything you are assuming. Do not wait for approval unless an assumption is theirs to decide.

## Building

4. **Checks you can trust.** Guardrail and contract checks exist before the code (they come from the guardrails phase). Other tests can be written alongside the code. Either way, see each new check fail once, so you know it can.
5. **Small steps.** Keep the system runnable and run `./scripts/check` often. Note surprises and internal choices in the task file's log.
6. **Stay in scope.** Unrelated problems you notice become board items, not extra changes. If the task turns out bigger or different than agreed, say so before continuing.
7. **When stuck** after two honest attempts at the same problem, stop and report what you tried and what you think is wrong. Do not work around a guardrail or a contract.

## Finishing

8. **Verify the real thing.** Run the system and exercise the outcome the way a person or a neighbouring service would. For reliability behaviour, inject the failure. Check interface changes in a browser.
9. **Independent check** for significant work (a contract, a guarantee, a user-facing flow): give the task file and the change to the `verifier` agent, without your reasoning. Fix what it finds, or report it.
10. **Record evidence** in the task file, or in the done line for a small task: commands run, results, what was not verified and why.
11. **Close out.** `./scripts/check` passes; commit according to the git rules in `CLAUDE.md`, including the `Verified:` line; move the task to `tasks/done.md` with one line of evidence; add follow-up tasks.
12. **Report** in a few lines: what is now true for a user or a neighbouring service, how it was verified, what is not verified, and any choice on internals worth knowing.

## Standards

- Done means every acceptance criterion is checked and the evidence is written down. "Should work" is not done.
- Never weaken, skip or delete a check to get a pass.
- Prefer boring, well-known solutions. A prototype still deserves clean boundaries, because the contracts are what the user reviews.
- Do not add features, abstractions or configurability the task did not ask for.
