---
name: validate
description: Validation phase. Independently check what has been built against the brief, the contracts and the guardrails, using the real running system, and turn the findings into tasks. Use after a milestone or a group of tasks, before showing the product to anyone, or when the user asks whether something really works, asks for a quality review, or wants a validation iteration.
argument-hint: "<scope: a task, a flow, a contract, or all>"
---

# Validate

Scope: $ARGUMENTS

Implementation proves the builder's own understanding. Validation asks a different question: would someone who read only the brief and the contracts agree this is right? A builder is a lenient judge of their own work, so the judging is done by fresh eyes.

## Procedure

1. **Define the round.** The scope, and the yardsticks in play: lines of the brief, contracts, guardrails, acceptance criteria. Add a board item (`T-nnn` `validate`) under Now.
2. **Run `./scripts/check` first.** If it fails, that is the first finding.
3. **Hand to the `verifier` agent** with the scope, the yardstick files and how to run the system. Do not pass on your implementation reasoning or your opinion of what works. It judges against the yardsticks and skips style. For a broad scope, run several verifiers in parallel on separate angles:
   - the product as a person would use it: flows, edge cases, craft
   - contracts and guarantees under failure: slow, down, duplicate, out of order
   - the brief line by line: met, partly met, not met
4. **Check the findings yourself.** Reproduce each one before accepting it. Drop what does not reproduce. Never drop what is merely inconvenient.
5. **Report to the user:** a short verdict per yardstick (met, partly, not met), the findings by severity with evidence, and what was not checked. Make a visual for more than a handful of findings.
6. **Turn findings into tasks** on the board, ordered by severity. A finding that questions a decision goes to the user as a decision, not straight to a fix.
7. **Close the round.** Move the item to `tasks/done.md` with the verdict. The next round re-checks the fixes.
8. **Stop after three rounds on the same scope.** If findings keep coming, the cause is upstream (an unclear decision, a missing guardrail). Bring that pattern to the user instead of looping.

## Severity

- **Blocker:** breaks a guarantee, a contract or a line of the brief.
- **Major:** a person would notice and be let down, or a quality target is missed.
- **Minor:** polish.

## Standards

- Evidence for every finding: steps, what was expected and which yardstick says so, what happened.
- Use the system for real: through the interface, through the service boundaries, with failures injected. Reading code is not validation.
- Report what was not checked as plainly as what was.
- A round that finds nothing must say exactly what was tried.
