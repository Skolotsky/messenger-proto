---
name: research
description: Research phase. Investigate a question (established practice, prior art, constraints, options) before experimenting or deciding, and file the findings as a short sourced note in docs/research/. Use when the user asks to research, look into or find out something, or when a decision lacks evidence.
---

# Research

Question: $ARGUMENTS

Research exists to make a later decision better. Start from the decision it serves. If the question above is empty, take it from the conversation; if you cannot name what the user wants to learn, ask.

## Procedure

1. **Frame.** State the question in one sentence and the decision it feeds. Add a board item (`T-nnn` `research`) under Now.
2. **Check what we already know:** `docs/research/`, `docs/decisions/README.md`, `experiments/`.
3. **Investigate.** Prefer primary sources (official documentation, write-ups by people who ran the thing, source code) over summaries of them. For a broad question, hand independent sub-questions to subagents in parallel and keep only their conclusions. Note the date of every source; practice changes.
4. **Separate** established practice, contested points, and your own inference. Say which claims you could not verify.
5. **Write the note** `docs/research/R-nnn-<slug>.md` from `.claude/skills/research/note-template.md`. One page: findings first, sources last.
6. **Hand off.** Tell the user in a few lines: the answer, what it means for the decision, and what remains uncertain (a candidate for an experiment). Offer a visual when the findings have structure. Move the board item to `tasks/done.md`.

## Standards

- A finding without a source is an opinion. Label it as one.
- Do not bury a recommendation in neutral language. If the evidence points one way, say so and say why.
- Research does not decide. Product, contract and quality choices go to the user as a decision (the `decide` skill).
