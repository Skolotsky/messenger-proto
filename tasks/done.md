# Done

Finished work, newest first. One line each: what is now true and how it was verified. Commits carry the task ID, so `git log --grep T-nnn` finds the change.

- 2026-11-22 · **T-006** `setup` Simple lookups go to a cheap `scout` agent (haiku, read-only); rule refined in `CLAUDE.md`, practice in R-003. Verified: `./scripts/check` passes; the agent definition is not yet seen running.
- 2026-11-22 · **T-001** `setup` Agentic development infrastructure is in place: instructions, board, decision and guardrail registers, phase skills, verifier agent, project check. Verified: `./scripts/check` passes; pre-commit hook and permission rules tested by hand (see the setup report in the session).
