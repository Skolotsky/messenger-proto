# R-001: What are current best practices for agent-driven development in a solo repo?

- Date: 2026-11-22
- Feeds: the project setup (T-001): `CLAUDE.md`, the phase skills, the board, the checks
- Method: web research by a subagent, 31 sources cited, most from 2026. Claude documentation was read as raw text; the other pages came through a summarising fetch, so the wording below is paraphrase and figures were not re-checked against the pages.

## Answer

The setup's core choices match what primary sources agree on: a short always-loaded instruction file, state on disk, a runnable check plus evidence behind every "done", a fresh-eyes reviewer for significant work, tooling to enforce the few rules that have no exceptions, and human attention spent on plans and irreversible choices instead of code.

The main correction from 2026 sources is to do less: fewer rules, fewer documents, fewer approval prompts, and independent evaluation only where it pays.

## Findings

**Context and instructions**

- Keep the always-loaded file under 200 lines and limited to what the agent cannot find in the repo; one team uses about 100 lines as a map into `docs/` (established) [1][2][12].
- Layout and architecture overviews in instruction files add cost without clear benefit (contested: one study measured over 20% more cost and no general gain [28]; other guidance still lists project layout [2][31]).
- Over-constraint is a 2026 failure mode. Anthropic reports cutting more than 80% of Claude Code's system prompt for current models, and cautions that pile up in notes and memory degrade results (established, though no evaluations were published) [16][7][9].
- Procedures belong in skills that load on demand (established) [2][4].

**Tasks and decisions**

- Keep state on disk and read it at session start: a status file, progress notes, git history (established) [6][10].
- One task in progress per agent (established) [10][16].
- Decision records suit agents when status has teeth: accepted binds, proposed informs, superseded is history, and the agent stops on conflict (established, thin evidence) [26].
- Heavy specification toolchains produce more Markdown than a person can review, and agents ignore parts of it (contested) [13][25].
- People should review plans and mockups, not code (established) [22][16].

**Verification**

- Give the agent a runnable pass/fail check and require evidence: the command with its output, or a screenshot (established) [1].
- Prefer end-to-end checks through the running app, add property-based tests, grade tests by mutation (established) [10][20][25].
- Use a fresh-context, read-only reviewer that checks against the spec, skips style, and stops after three rounds (established) [1][18][20].
- Known cheats: declaring done early, marking untested features as passing, editing or deleting tests, tautological tests, lenient evaluators. Prevention: the agent cannot modify the check that judges it, and every commit records how it was verified (established) [10][11][15][20].
- Strict test-first inside the agent loop is contested: Willison recommends it [23]; a small study found no gain at 3 to 8.5 times the tokens [15].
- A separate evaluator pays mainly on tasks at the edge of what the model does reliably. In one small study a full harness cost about ten times the tokens with no gain on a frontier model (contested) [11][19].

**Guardrails and human attention**

- Enforce by hook, deny rule or CI only the rules with no exceptions, keep that set small, and promote a prose rule to a check once it is ignored repeatedly (established) [1][17].
- Enforce architecture with linters and structural tests whose failure messages state the fix (established) [12][14][24].
- Ask the human at irreversible or high-impact steps and decide alone on local, reversible work. Blanket approval is theatre: people accept about 93% of routine prompts (established) [1][6][8][17].
- Ask structured questions: a few described options plus free text (established) [5][21]. No primary source prescribes an options, risks and recommendation template; that format is this project's own.
- When output is wrong, fix the harness, not only the artefact (established) [14][30].

## What this means for us

Adopted in the setup:

- `CLAUDE.md` holds the working agreement and the rules, imports three short state files, and carries no layout or architecture overview.
- One item in Now. Task files only for work that touches a contract, a guarantee or a user-facing flow.
- Decision status tiers, with stop on conflict.
- Evidence behind every done, real-system checks first, every new check seen failing once in place of strict test-first, a verifier agent for significant work, three validation rounds at most.
- Two enforced process rules to start with. More are added only for rules with no exceptions, or after a prose rule is broken twice.
- Commit messages carry a `Verified:` line.
- Routine steps need no approval. The user is asked about the decisions that are theirs.

Deliberately our own, to revisit if it costs more than it gives:

- The seven-phase flow and the decision brief format come from the user's way of working, not from the literature.
- Guardrail checks are written before the implementation, although strict test-first is contested. Here the guardrails are the user's review surface, a different purpose from the one the study measured.

## Open questions

- How much the verifier agent adds on this project is unknown until the first validation round. Measure it by the findings per round that survive reproduction.
- Whether the three imported state files stay small as decisions accumulate. Revisit when `CLAUDE.md` plus its imports passes 200 lines.

## Sources

Numbers are those of the research run, so gaps mean a source was opened but is not cited here. The subagent could not open one source (Yegge's Beads post, HTTP 403).

1. Claude Code docs, "Best practices", https://code.claude.com/docs/en/best-practices (living page)
2. Claude Code docs, "How Claude remembers your project", https://code.claude.com/docs/en/memory (living page)
4. Claude Code docs, skills and features overview, https://code.claude.com/docs/en/skills (living page)
5. Claude Code docs, `/goal`, agent teams, AskUserQuestion, https://code.claude.com/docs/en/agent-sdk/user-input (living pages)
6. Claude platform docs, "Prompting best practices", https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices (living page)
7. Claude platform docs, "Prompting Claude Fable 5.2", https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-2 (undated)
8. Anthropic, "How we built Claude Code auto mode", https://www.anthropic.com/engineering/claude-code-auto-mode (2026-03-25)
9. Anthropic, "The components of agent memory", https://www.anthropic.com/engineering/components-of-agent-memory (2026-11-06)
10. Anthropic, "Effective harnesses for long-running agents", https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents (2025-11-26)
11. Anthropic, "Harness design for long-running application development", https://www.anthropic.com/engineering/harness-design-long-running-apps (2026-03-24)
12. OpenAI, "Harness engineering: leveraging Codex in an agent-first world", https://openai.com/index/harness-engineering/ (2026-02-11)
13. Böckeler, "Understanding Spec-Driven-Development: Kiro, spec-kit, and Tessl", https://martinfowler.com/articles/exploring-gen-ai/sdd-3-tools.html (2025-10-15)
14. Böckeler, "Harness engineering for coding agent users", https://martinfowler.com/articles/harness-engineering.html (2026-04-02)
15. Böckeler, "TDD inside the agent loop - theater or actual value?", https://martinfowler.com/articles/exploring-gen-ai/tdd-in-the-agent-loop.html (2026-08-10; small sample)
16. Anthropic, "The new rules of context engineering for Claude 5 generation models", https://claude.dev/blog/the-new-rules-of-context-engineering-for-claude-5-generation-models/ (2026-07-24)
17. Thoughtworks, "Engineering the harness: A practical pattern for reliable coding agents", https://thoughtworks.com/insights/blog/architecture/engineering-the-harness-a-practical-pattern-for-reliable-coding-agents (2026-09-29)
18. Anthropic, "How the Claude Code team does code review", https://claude.dev/blog/solving-code-review-at-anthropic/ (2026-10-15)
19. Naresh Jain, "Are Harnesses Worth the Cost?", https://martinfowler.com/articles/are-harnesses-worth-the-cost.html (2026-10-13; ten runs per arm, not Claude models)
20. Anthropic, "How the Claude Code team thinks about verification", https://claude.dev/blog/how-the-claude-code-team-thinks-about-verification/ (2026-11-16)
21. HumanLayer, 12-factor agents, factor 7, https://github.com/humanlayer/12-factor-agents/blob/main/content/factor-07-contact-humans-with-tools.md (2025)
22. HumanLayer, "Advanced Context Engineering for Coding Agents", https://github.com/humanlayer/advanced-context-engineering-for-coding-agents/blob/main/ace-fca.md (2025)
23. Simon Willison, "Agentic Engineering Patterns", https://simonwillison.net/guides/agentic-engineering-patterns/ (started 2026-02-23)
24. Simon Willison quoting Geoffrey Huntley, https://simonwillison.net/2026/Oct/13/geoffrey-huntley/ (2026-10-13)
25. Thoughtworks Technology Radar Vol. 35, https://www.thoughtworks.com/en-us/radar/techniques (2026-11-04)
26. Duncan Davidson, "Coding Agents Love Decision Records", https://oreilly.com/radar/coding-agents-love-decision-records/ (2026-10-02; one practitioner, no measurements)
28. Gloaguen et al., "Evaluating AGENTS.md", https://arxiv.org/abs/2602.11988 (2026-02-12)
30. Kief Morris, "Humans and Agents in Software Engineering Loops", https://martinfowler.com/articles/exploring-gen-ai/humans-and-agents.html (2026-03-04)
31. agents.md, https://agents.md/ (undated)
