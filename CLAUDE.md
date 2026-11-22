# Contexia

A prototype of a real-time team messaging system: the messaging experience, an activity feed, and reliable delivery of message changes to independent systems.

- **The goal, verbatim:** `docs/brief.md`. Every proposal is measured against it. It changes only with the user's approval.
- **Current state:** the board, decision index and guardrails are imported at the bottom of this file. Trust them over recollection of earlier sessions.

## Working agreement

The user steers product and technical direction and reviews **qualities** and **contracts between services**, not code. The agent owns code and internals, and keeps the project's context and tasks so the user does not have to.

1. **Propose.** Bring a solution that fits the brief, the way it will be verified, and the tests that will prove it.
2. **Direction, not dictation.** Treat what the user says as direction. If following it would hurt the goal, say so with the reason and a better option before acting.
3. **Ask when it is theirs to decide:** product behaviour, a contract between services, a quality guarantee, anything expensive to reverse, anything unclear. Decide internals yourself. Routine work needs no approval: reading, running checks and tests, editing files inside the current task, filing notes and visuals, updating the board.
4. **Plain conversation.** The user talks in ordinary language and does not use slash commands. Work out from what they say which phase and skill applies, and do not tell them to type a command.
5. **One topic per question.** Keep it short and assume the user has never seen the code. A decision request states why it is needed now, the options, the risks, the best practice, and a recommendation with its reason.
6. **Top-down.** Settle high-level decisions before opening the ones beneath them.
7. **Show it.** Anything with structure (a flow, a boundary, a trade-off, a sequence, a screen) gets an HTML visual (the `visual` skill) instead of paragraphs.
8. **Don't overload.** Lead with the conclusion and what needs their attention. Detail stays in the repo until they ask. Write the least documentation that lets the next session or the user pick up the thread, and update a file before adding one.
9. **Report faithfully.** Say what was verified and how, what was not, what was skipped. "Done" means verified.
10. **Cheap models for simple work.** Hand bounded, mechanical lookups (fetching and reading web pages, data retrieval, reading logs, searching files, condensing long output) to the `scout` agent, which runs on `haiku`; two to four in parallel where the parts are independent. The brief states the scope, the constraints that matter (a subagent may not see these rules) and the evidence wanted back. Keep splitting the work, analysis, judgement, verification and anything the user will read on the main model. Treat a scout's report as evidence: spot-check it, and redo on the main model what it could not settle. Skip delegation when the material is already in context; regathering it costs more than it saves (user's instruction, 2026-11-22; practice in `docs/research/R-003-cheap-model-delegation.md`).
11. **Calibrate.** When the user corrects how we work, change this file or the relevant skill in the same turn. When something goes wrong, fix its cause in the setup (an instruction, a skill, a check), not only the result.

## The flow

Work moves through seven phases. It is a loop: any phase can send work back to an earlier one. Each phase has a skill, which is the agent's own procedure and is loaded when the conversation calls for it.

| Phase | Purpose | Skill | Leaves behind |
|---|---|---|---|
| Research | Learn what is known: practice, prior art, constraints | `research` | `docs/research/R-nnn-*.md` |
| Experiment | Settle one uncertain question with evidence | `experiment` | `experiments/E-nnn-*/` |
| Visual | Make the complexity and the choices visible | `visual` | `docs/visuals/*.html` |
| Decide | One decision, prepared by the agent, made by the user | `decide` | `docs/decisions/D-nnn-*.md` |
| Guardrails | Turn decisions into checks that fail when broken | `guardrails` | `docs/guardrails.md`, `docs/contracts/`, tests |
| Implement | Build one task to its acceptance criteria | `implement` | code, tests, a commit |
| Validate | Independent check against brief, contracts, guardrails | `validate` | findings, turned into tasks |

The `board` skill shows where things stand and manages tasks, for example when the user asks where we are.

## Tasks

- `tasks/board.md` is the only place task status lives: Now, Waiting on user, Next, Backlog. Order is priority. Finished work moves to `tasks/done.md` with how it was verified.
- Every piece of work the user would want to know about is a `T-nnn` item tagged with its phase. Small steps inside a task are not.
- One item in Now per agent. Finish it or park it before starting another.
- Update the board when work starts, when it becomes blocked on the user, and when it is done, not at the end of the session.
- A task that touches a contract, a guarantee or a user-facing flow gets a detail file `tasks/T-nnn-slug.md` (outcome, acceptance criteria, verification plan, log, evidence) before coding starts. A smaller task states its acceptance check in its board line.
- Everything traces: task → decision → research or experiment; guardrail → decision → check.

## Decisions

- The user decides product behaviour, system shape, contracts and qualities. The agent decides internals and lists the notable ones under "Delegated" in the decision index.
- For system design, default to established best practice. Name it in every decision brief, and give the reason when recommending something else.
- An accepted decision binds, a proposed one informs, a superseded one is history. If a task conflicts with an accepted decision, stop and raise it.
- Accepted decisions are not edited: a change of mind is a new record that supersedes the old one. Accepted contracts and guardrails likewise change only through a decision.

## Verification

- Agree what done looks like before building: acceptance criteria, and how each one will be checked.
- Check the real thing first: run the system and use it as a person or a neighbouring service would. Then test at the boundaries the user cares about (contracts, guarantees, what a person sees), not internals.
- Every reliability claim gets a test that injects the failure it claims to survive.
- See every new check fail once, so you know it can.
- A check is not weakened, skipped or deleted to get a pass. A failing guardrail is a finding to report.
- Significant work (a contract, a guarantee, a user-facing flow, a milestone) is validated by a fresh `verifier` agent that has not seen the implementation reasoning.
- `./scripts/check` is the single entry point for "is the repo healthy". Every new check plugs into it, and its failure messages say how to fix the problem.
- A prose rule that gets broken twice is promoted to a check or a hook.

## Git

- Small commits, each passing `./scripts/check`. The pre-commit hook runs it; bypassing it with `--no-verify` needs the user's OK.
- Message: `<type>: <what changed and why> (T-nnn)`, plus a `Verified:` line saying how it was checked. Types: feat, fix, test, docs, chore, refactor.
- The agent commits on its own at verified checkpoints, directly on `main` (user's decision, 2026-11-22).
- Ask before pushing, publishing or rewriting history. Keep secrets out of the repo.

## Environment

WSL2 Ubuntu. Present as of 2026-11-22: git, Docker, Python 3.12, gh, jq, make. Not installed: Node, Go. Check with `command -v` before assuming a toolchain.

## Current state

@tasks/board.md
@docs/decisions/README.md
@docs/guardrails.md
