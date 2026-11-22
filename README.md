# Contexia

A prototype of a real-time team messaging system, focused on the messaging experience, the activity feed, and reliable delivery of message changes to independent systems (search, notifications, the feed itself).

**Status:** foundation built. `make up` starts the empty programs; `make verify` runs the full suite. Only Docker and make are needed.

## How this repo is organised

| Path | What it holds |
|---|---|
| `docs/brief.md` | The goal, verbatim |
| `docs/decisions/` | Decision records: what was chosen, the alternatives, and why |
| `docs/contracts/` | Contracts between services |
| `docs/guardrails.md` | Qualities and invariants, each tied to the check that enforces it |
| `docs/research/`, `experiments/` | Evidence behind the decisions |
| `docs/visuals/` | Diagrams and explainers; open the HTML files in a browser |
| `tasks/` | The board (now, next, backlog) and the log of finished work |
| `CLAUDE.md`, `.claude/` | Working agreement and tooling for AI-assisted development |

## Checks

`./scripts/check` is the single entry point that says whether the repo is healthy. A pre-commit hook runs it; after cloning, enable the hook with:

```bash
git config core.hooksPath .githooks
```
