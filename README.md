# Contexia

A prototype of a real-time team messaging system, focused on the messaging experience, the activity feed, and reliable delivery of message changes to independent systems (search, notifications, the feed itself).

**Status:** the first version is built: the messaging product, the activity feed, reliable delivery to search and notifications, and a seeded demonstration.

## Run it

You need Docker and make. Nothing else is installed on the host.

```bash
make up
```

Then open:

| Address | What it is |
|---|---|
| http://127.0.0.1:18080/ | The product, as Maya, a member of the seeded team |
| http://127.0.0.1:18080/control/ | The control panel: play the team's situations, stop or slow search, the feed and the sender, Reset |
| http://127.0.0.1:18080/notifications/ | What each person's phone would have received |

A good first look: open Activity, press a row's reason line to see why it is there, play a situation from the control panel, then switch search or the feed off and watch the product say so while nothing is lost.

```bash
make down
```

If you ran an earlier build, remove its database once with `docker compose -f deploy/compose.yaml -f deploy/compose.ports.yaml down -v`.

`make verify` runs every check, including the failure tests, in about four minutes.

Known limits are in `docs/known-shortcuts.md`. Capacity is about 5,000 people in one organization, estimated by analysis and not measured.

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
