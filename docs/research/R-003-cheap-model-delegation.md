# R-003: How should simple work be delegated to subagents on a cheap model?

- Date: 2026-11-22
- Feeds: working agreement item 10 in `CLAUDE.md`, the `scout` agent (T-006)
- Method: web research by a subagent. The Claude Code pages were read only partly (about the first 100k characters), so more guidance may exist there. Figures were not re-checked against the pages.

## Answer

Use a cheap model for bounded, mechanical lookups and keep everything that needs judgement on the main model: splitting the work, briefing, judging results, verification, design. Judge the saving by cost per finished task, not price per token, and do not delegate what is already in context.

## Findings

- **Good fits for a cheap model:** searching, extraction, collecting files, checking links, condensing test or log output (established) [1][2].
- **Keep decomposition, briefing and judging on the strong model** (established). One cited study put delegation reliability near 0.85 on the strongest models and near 0.45 on the fast tier [2]. No primary study was found on cheap-model failures with subtle bugs or design.
- **Ask for short summaries back, and limit the subagent's tools** (established) [1].
- **Restate constraints in the brief:** project rules mostly do not reach subagents (established) [3]. Asking for file and line, quoted lines and an explicit "not found" is widely repeated but unmeasured (inference).
- **Treat results as evidence, not conclusions.** Delegating a task does not make the answer smarter; a higher error rate is tolerable only under supervision (contested) [1][2].
- **Two to four agents per wave;** keep tasks that share files together (contested, one practitioner's heuristic) [4].
- **Cost:** a fresh subagent regathers context the main conversation already has, and its prompt cache lasts 5 minutes against about an hour for the main session, so small lookups can cost more delegated than inline (established) [1][5].
- **Price per token is not cost per task.** In one reported comparison the cheaper model used 1.9 times the tokens and cost more per task at a lower completion rate; in another, cheap workers under a strong orchestrator kept 96% of the score at 46% of the cost (vendor-reported, different model families) [2].

## What this means for us

- A `scout` agent pinned to `haiku` with read-only tools, so the model choice is built in. A per-call model override would also work but depends on remembering it.
- The built-in Explore agent runs on a strong model by default in this setup, so it is not the cheap option; use `scout` for cheap lookups.
- The main agent spot-checks scout reports and never passes them to the user unread.

## Open questions

- No measurement exists for this project. If cost matters later, compare usage per agent type after a week of work.
- Whether `haiku` is reliable enough on long logs here is untested; escalate when a scout reports "not found" for something that should exist.

## Sources

1. Claude Code docs, "Best practices", https://code.claude.com/docs/en/best-practices (living page)
2. Arize, "How cheap models changed multi-agent economics", https://arize.com/blog/how-cheap-models-changed-multi-agent-economics/ (2026-08)
3. Claude Code docs, "Sub-agents" and "Model configuration", https://code.claude.com/docs/en/sub-agents , https://code.claude.com/docs/en/model-config (living pages)
4. Rahul Garg, "Orchestrator tax", https://martinfowler.com/articles/orchestrator-tax.html (2026-07-16)
5. Claude Code docs, "Costs", https://code.claude.com/docs/en/costs (living page)
