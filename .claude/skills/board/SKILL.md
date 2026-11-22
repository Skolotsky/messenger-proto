---
name: board
description: Show where the project stands and manage tasks - current phase, what is in progress, what is waiting on the user, what is next, recent results. Use when the user asks for status, what is next, what is open or blocked, where we are, or wants to add, reorder, park or drop tasks.
---

# Board

Request: $ARGUMENTS

With no request, show status.

## Showing status

Read `tasks/board.md`, the latest entries of `tasks/done.md` and `docs/decisions/README.md`. Run `git status --short` to spot uncommitted work the board does not explain. Then answer briefly, in this order:

1. **Where we are:** phase and focus, one line.
2. **Waiting on you:** decisions or answers needed, most blocking first. If nothing, say so.
3. **Now:** what is in progress and its next step.
4. **Next:** the next two or three items.
5. **Recently done:** the last few results, each with how it was verified.

Flag anything stale: an item in Now that nobody is working on, or a board that disagrees with the repo.

For more than a glance (a milestone review, a long backlog) make it a visual (the `visual` skill).

## Changing the board

- **Add:** next free `T-nnn`, a phase tag, one line stating the outcome, the decision it serves. New items go to Backlog unless the user says otherwise.
- **Reorder, park or drop** on the user's word. A dropped item keeps one line in `tasks/done.md` marked `dropped`, with the reason.
- **If the board and reality disagree,** fix the board and say what changed.

Line format: `- **T-012** \`implement\` Outcome in a few words (D-003) · [detail](T-012-slug.md)`

Phase tags: `research`, `experiment`, `visual`, `decide`, `guardrails`, `implement`, `validate`, `setup`.
