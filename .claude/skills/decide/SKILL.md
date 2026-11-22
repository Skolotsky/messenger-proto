---
name: decide
description: Decision phase. Prepare one decision for the user (why now, options, risks, best practice, recommendation), get their call, and record it in docs/decisions/. Use when a choice about product behaviour, system shape, a contract between services or a quality guarantee is needed, when something expensive to reverse comes up, or when the user asks what the options are.
argument-hint: "<the decision to prepare>"
---

# Decide

Decision: $ARGUMENTS

The user decides product behaviour, system shape, contracts and qualities. Your job is to make that decision easy to make well: one topic, the real options, an honest recommendation. If the line above is empty, take the decision from the conversation.

## Before preparing it

- **Is it theirs?** Internals (code structure, naming, library plumbing) are yours. Decide, and list the notable ones under "Delegated" in `docs/decisions/README.md`.
- **Is it ready?** Top-down: if a higher-level decision this one depends on is still open, prepare that one first.
- **Is there evidence?** If the options cannot be compared honestly yet, say so and propose research or an experiment first.
- **One topic.** If the question contains "and", split it.

## Procedure

1. **Register.** Next `D-nnn`, a line in the right section of `docs/decisions/README.md` with status `open`, and a board item (`T-nnn` `decide`).
2. **Write the record** `docs/decisions/D-nnn-<slug>.md` from `.claude/skills/decide/record-template.md`, status `proposed`.
3. **Second opinion** for anything expensive to reverse: give a fresh subagent the context and the options without your recommendation, and ask what it would choose and what could go wrong. Reconcile disagreements in the record.
4. **Present the brief.** Make a visual (the `visual` skill) when the options differ in structure, which most system-shape and contract decisions do. In chat keep to:
   - what needs deciding and why now (two lines)
   - the options, each with its main upside and main risk
   - what best practice says
   - your recommendation, the reason, and what it locks in

   Then ask one question. Move the board item to "Waiting on user".
5. **Record the outcome.** Set status `accepted`, write down the user's choice and their reasoning (in their words where it differs from yours), update the index line, and list the follow-ups: guardrails to form, tasks to add.
6. **If they choose against your recommendation,** state the risk once, then commit to their choice fully.

## Standards

- Options are real alternatives a competent team would consider, not a recommendation plus strawmen.
- Risks are concrete: what breaks, for whom, how likely, how we would notice.
- Say how reversible the choice is, and spend the user's attention in proportion.
- The record holds the depth. The brief stays short.
- Accepted records are not edited. A change of mind is a new record that supersedes the old one.
