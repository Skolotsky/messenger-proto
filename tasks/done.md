# Done

Finished work, newest first. One line each: what is now true and how it was verified. Commits carry the task ID, so `git log --grep T-nnn` finds the change.

- 2026-11-22 · **T-002** `research` The context is known: built for small to mid-size startup teams with mixed roles and heavy AI use, as a PoC to launch an early access programme. Verified: stated by the user; recorded in D-001.
- 2026-11-22 · **T-005** `research` We know how Slack, Teams, Discord, Zulip and others handle the feed, delivery to downstream systems and AI agents, and which eight decisions follow (R-002, visual `R-002-decision-landscape.html`). Verified: eight key claims re-fetched by a second agent, seven held and one was corrected; the rest rests on the research agents' reading; the visual was not looked at rendered (no browser, T-004).
- 2026-11-22 · **T-006** `setup` Simple lookups go to a cheap `scout` agent (haiku, read-only); rule refined in `CLAUDE.md`, practice in R-003. Verified: `./scripts/check` passes; the agent definition is not yet seen running.
- 2026-11-22 · **T-001** `setup` Agentic development infrastructure is in place: instructions, board, decision and guardrail registers, phase skills, verifier agent, project check. Verified: `./scripts/check` passes; nine injected faults each caught; the hook refused a bad commit in a throwaway copy. Not verified: the permission rules (T-003), page rendering (T-004).
