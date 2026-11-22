# T-043: The build plan

- Phase: implement (the plan for all of it)
- Written 2026-11-22, for a new session to pick up
- Rests on: the brief; product decisions D-001 to D-004, D-009 to D-016, D-019 to D-024; technical decisions D-005, D-006, D-007, D-008, D-017, D-018, D-025, D-026, D-028, D-029, D-030; the eight contracts in `docs/contracts/`; guardrails G-003 to G-030; `tasks/T-038-build-acceptance-from-e002.md`
- Visual: `docs/visuals/T-043-build-plan.html`

## How to start (for the session that picks this up)

1. Read `CLAUDE.md`, this file, `docs/guardrails.md` and the contracts. The architecture on one page: `docs/visuals/T-022-architecture-proposal.html`; where state lives: `docs/visuals/D-005-where-state-lives.html`; how it grows: `docs/visuals/D-029-scaling-by-adding.html`.
2. Do **T-044** and **T-045** yourself, one after the other. Everything else stands on them and they define the shared files.
3. Then hand the packages of a wave to parallel subagents, one package each, following "Working in parallel" below. Merge, run the full verification, and only then open the next wave.
4. At each milestone, give the milestone's scope to a fresh `verifier` agent before calling it done.

Each package below is a board item. Before coding a package, its agent creates the package's own task file from `.claude/skills/implement/task-template.md`, copying the outcome, acceptance and tests from here and adding its log and evidence.

## Choices on internals (the agent's; change one only with a reason written in the package's log)

| Matter | Choice | Why |
|---|---|---|
| Toolchains | Go and Node run in Docker; nothing is installed on the host. `make` wraps everything | The host has Docker and make only; the user allowed downloads |
| Server | Go, one module in `server/`, one program per part under `server/cmd/`; standard `net/http`; `pgx` for PostgreSQL; no ORM; plain SQL migrations per part | Boring and well known; each part is one self-contained program (D-025) |
| WebSocket | `github.com/coder/websocket` | Small, maintained, context-aware |
| Property tests | `pgregory.net/rapid` | G-007 and G-030 are properties, not examples |
| Web app | TypeScript, React with Vite, installable through `vite-plugin-pwa`; plain CSS carried over from the prototype's tokens; no component kit | The most common stack for an agent to write correctly; the look already exists in E-002 |
| Browser tests | Playwright Test in its official Docker image, at 1280 and 400 px wide | D-009: every surface at two widths |
| Contracts in machine form | JSON Schema in `contracts/schema/` with examples in `contracts/examples/`; TypeScript types generated from them; Go types written by hand and tested against every example | D-025: the contracts are the source both sides are generated from or checked against. `docs/contracts/*.md` stay the authority in words |
| Time | Every program reads time through one injected clock. A test build can move it | The guardrails need 5 s, 60 s, 10 minutes and 7 days to pass in seconds |
| Deliberate faults | A test-only build switch per rule that a guardrail says "must fail without" | To see each guardrail test fail once for the right reason |
| Running | `make up` starts everything with Docker Compose; one organization, the seeded team | D-005: separate programs, one start command |

## Layout and who owns what

A package changes only the directories it owns. Shared files are changed only by the lead session, between waves.

```
server/cmd/{conversations,delivery,feed,search,sender,demo}/   one program each
server/internal/conversations/     T-046, T-053, T-054
server/internal/delivery/          T-047
server/internal/standins/          T-048
server/internal/feed/rules/        T-049   (pure: no database, no network)
server/internal/feed/              T-052
server/internal/shared/            T-044   SHARED: clock, calls between parts, database per organization, IDs, contract types
server/migrations/<part>/          the part's package
web/app/                           T-050, T-055, T-056, T-058
web/notification-app/, web/control-panel/   T-059
demo/content/                      T-051   (the seeded organization as data)
contracts/schema/, contracts/examples/      T-045   SHARED
tests/harness/                     T-045   SHARED: start the system, inject faults, move the clock, two organizations
tests/guardrails/<name>/           the package named for that guardrail below
tests/browser/                     T-050 onward
deploy/compose.yaml, Makefile, scripts/check, scripts/verify   T-044   SHARED
```

## The packages

Order on the board is priority. "Needs" is what must be merged first. Packages in the same wave do not need each other.

### Wave 0, in sequence, by the lead session

**T-044 Foundation.** Outcome: `make up` starts PostgreSQL and six empty programs that answer "alive"; `make test` and `make verify` run. Includes: the layout above; Docker images for Go and Node; Compose file; one database login and schema per part (G-008); the shared pieces: the clock, the one client for calls between parts with its 3 s limit (G-023), "the database for this organization" with the organization bound to the session (G-025), IDs; `./scripts/check` stays fast (format, vet, unit tests, generated files up to date, the existing repository checks) and `./scripts/verify` runs the full suite (Compose, guardrail tests, browser tests).
- Acceptance: a fresh clone runs `make up` and `make verify` with nothing installed but Docker and make; each part's login is refused on the other parts' schemas (first half of G-008's test, seen failing with one shared login).

**T-045 Contracts in machine form, and the stand-ins the packages build against.** Needs T-044. Outcome: every operation and event of the eight contracts has a schema and at least one example; TypeScript types are generated; Go types pass every example; and three tools exist so that packages can work without each other:
- a **log writer** that produces a valid change log for one or two organizations from a seeded script (the driver of E-003, as a Go test helper);
- a **mock door** that serves the people door and the live connection from fixtures, for the web app;
- the **test harness**: start and stop parts, the troublemaker (repeats, lost answers, late copies), move the clock, two organizations with the same names and numbers.
- Acceptance: an example that breaks its schema fails `./scripts/check`; changing a schema without regenerating fails it too.

### Wave 1, six packages in parallel (all need T-045 only)

**T-046 Conversations: saving and the log.** Tables for people, channels, messages, reactions, what readers did; one row per organization with the newest number, places, claims and due ticks; the change log per organization; the people door's actions (send, edit, delete, react, read, keep, done, dial, Focus rule and person, Focus only, preference, create channel, add people); questions (a conversation around a position with thread summaries, `@` suggestions).
- Guardrails owned: G-003 (saved once), G-004 (change and log together; the database rule), G-005 (numbering), first half of G-025 (row rules).
- Acceptance: each guardrail test seen failing with its rule switched off, then passing; every action kind has an example-driven test including its refusal answers.

**T-047 Delivery workers.** Find work through the index, claim, deliver outside a transaction, note the place; turns (fresh first, some backlog each round); outage against refusal, parking and release; the pause per receiver; batches; the nudge after "applied". Built against the log writer and a recording receiver.
- Guardrails owned: G-012 (independent receivers), G-014 (parking), G-023 (calls time out), G-026 (organizations independent), G-027 for workers (two copies), G-029 (idle organizations cost nothing).
- Acceptance: as above; the recording receiver counts repeats.

**T-048 The two stand-ins.** Search: version rule, delete remembered, query with total, match marks, "edited", thread; a `through` in every answer. Notification sender: names, the list of what was sent, the hold by clock notes, "delayed", its list for the notification app. Both with Running, Slow and Off switches for tests.
- Guardrails owned: G-006 (search equals truth), G-010 (sent exactly once), G-011 (the hold).
- Acceptance: as above, driven by the log writer through the troublemaker.

**T-049 The feed's rules, as a pure package.** No database, no network: given a change and what is known, what happens to which reader's rows, counts and notification notes. From D-001, D-003, D-004, D-011 to D-016, D-020, D-021, D-023 and T-038's points 1 to 19.
- Tests: one table test per rule, each citing its decision; E-001's 123 cases exported as data and replayed, every difference explained in the log (a decision wins over the model, which lacks D-021 and later decisions); the property of G-030 (conversations interleaved differently give the same result, organization-wide changes left in place); the nine open behaviours of T-021 implemented as the model has them and listed.
- Guardrails owned: G-030.

**T-050 Web app: the frame, the look, a conversation.** Build setup, installable; the prototype's tokens and components carried over (`experiments/E-002-look-and-feel/src/`, Slack-like); sidebar, channel and thread view, message box; against the mock door. Two widths.
- Tests: browser tests for T-038 points 24 to 25 and the layout rules of D-019 and D-023 (thread beside the channel, resizable panels, day lines, no inline threads); "nothing jumps" measured as in E-002.

**T-051 The seeded organization, as data.** One organization, about 30 people, 6 agents, 20 channels, two weeks of history; the hand-written situations of D-010 as replayable scripts; background traffic generated from a seed. Data and a small format only; no program.
- Acceptance: the same seed gives the same content; every situation listed in D-010 exists and names the feed outcome it should produce.

### Wave 2, five packages in parallel

**T-052 The feed as a service.** Needs T-049. Takes changes in order; rows, counts, its place and notification notes saved in one step; fences and the stop after five; editions and rebuild; clock notes; the questions the door asks (rows, history tabs with unread counts, settings in effect, "what would this have moved", row changes after a number).
- Guardrails owned: G-007 (equals a clean replay), G-009 (fence), second half of G-008 (never calls Conversations; rebuilt with Conversations unreachable), G-027 for the feed.

**T-053 Conversations: live connection, reading through the door, housekeeping.** Needs T-046. The WebSocket with numbered updates, `after`, `last_sent`, resume, "too far behind"; the door's questions to the feed and to search with notices when behind; the 5-second check; watching per connection with leases; clock ticks; the nudge's receiving end.
- Guardrails owned: G-024 (watching), the server half of G-015 and G-019.
- Acceptance (D-033): live connections of a retired organization are closed with `organization_gone`.
- Acceptance (architecture note, 2026-11-22): a copy of Conversations pushes to screens only what it reads from the organization's change log, in number order, with one reader per organization. The handler that saved a change only wakes that reader and never hands it the update. Reason: two saves commit in order but can reach the hub swapped, and a screen ignores a lower number, which loses a message silently. G-015's test gets a case that forces two saves of one organization to reach the hub out of order; it must fail for a hub that sends what handlers hand it.

**T-054 Conversations: the agent door.** Needs T-046. Events per agent kept 7 days; posts with IDs; replies in progress (start, pieces, finish, closed as "stopped"); the limits.
- Guardrails owned: G-016, G-017, G-018.

**T-055 Web app: Activity.** Needs T-050. The three buckets, rows with reasons and the reason panel, Kept, arrivals behind "Show", history tabs with counts, the settings screen (general rules, Focus people, channel levels, notifications, writing), phone layout with swipes. Against the mock door.
- Tests: browser tests for D-020, D-021, D-023 and T-038 points 12 to 18.
- Also (gap found in T-050): the full reaction picker of D-023, on a message and on an unopened row.

**T-056 Web app: writing, going wrong, search, staying current.** Needs T-050. Mentions, edit in place, delete that asks; offline, waiting and failed actions with IDs and order; search results; the live client: numbered updates, resume, reload when too far behind.
- Guardrails owned: the screen half of G-015; tests for D-024 and T-038 points 20 to 26.
- Acceptance (D-033): a screen that receives `organization_gone` goes back to the entry address and loads the current organization.
- Acceptance (D-035): the channel and thread level control inside a conversation, as D-001 and D-023 describe it.
- Also (gap found in T-050): creating a channel and starting a new direct message (D-009, D-024).
- Acceptance (architecture note, 2026-11-22): the reconnect pause (0.1 s doubling to 1 s) gets random variation, so a whole organization does not reconnect in the same instant when a copy dies. The varied pause never exceeds its step, so the contract's 1 s ceiling and the 2 s catch-up still hold. Test with a seeded random source.

### Wave 3, joining up

**T-057 Milestone 1: the spine runs end to end.** Needs T-046, T-047, T-048, T-052. Everything composed; a message, an edit and a delete travel to search, the feed and the sender. Run every guardrail test owned so far against the composed system, with two copies of the workers and the feed (G-027) and two organizations (G-025).
- Then: independent verifier on the delivery guarantees.

**T-058 Milestone 2: a person can use it.** Needs T-053, T-055, T-056, T-057. The web app on the real programs at both widths; G-015 and G-019 in a browser; all of T-038.
- Then: independent verifier on the user-facing flows.

**T-059 The demonstration.** Needs T-051, T-053, T-054. The scripted team playing through the two doors; the control panel (start, pause, reset, pace, what happened and what comes next, each situation on demand, Running, Slow and Off for search, sender and feed with how much is waiting); the notification app; Reset from saved copies and Rebuild (G-020).
- Guardrails owned: G-020.
- Acceptance (D-033): Reset seeds a fresh organization, waits until search and the feed are level, makes it current and retires the previous one; the entry address gives the current organization; with a screen open, pressing Reset lands it on the fresh one; the control panel lists organizations and deletes one or all retired, asking first; a fresh verifier checks this against D-033.

**T-060 A coding agent can join.** Not planned (D-034): moved to Backlog.

### Wave 4

**T-061 Milestone 3: validation.** Needs all above. A fresh verifier checks the running product against the brief, every contract and every guardrail; findings become tasks. The known-shortcuts list is brought up to date; a short "how to run and what to look at" is written.

**T-062 Text rules judged by a model.** Not planned (D-034): moved to Backlog. Text rules do not appear in the product's interface.

## Who builds `operations-and-control.md` (D-032)

The nudge: T-047 sends, T-053 receives. Place and control (mode, wipe): T-048 for search and the sender, T-052 for the feed. Delivery status, parked list, retry, restart: T-047. Seed session, people list, replies' text so far, open direct: T-046 (saved text of a reply: T-054). Rebuild, Reset, script: T-059. The notification app's list: T-048.

## Verification strategy

**What "done" means for a package:** every acceptance point checked; its guardrail tests seen failing once and then passing; `./scripts/check` and its own tests pass, with the full `./scripts/verify` once at the end (T-061); evidence written in its task file; the real thing run and looked at.

| Level | What it proves | How | When it runs |
|---|---|---|---|
| 1. Rules | The feed sorts as decided | Table tests per rule citing the decision; E-001's cases replayed; properties (G-007, G-030) | `./scripts/check`, on every commit |
| 2. One part against its contract | Each part keeps its side, also on the failure paths | The part alone, the other side played by the log writer, the mock door or a recording receiver; every schema example is a test | `./scripts/check` where fast, else `./scripts/verify` |
| 3. Guardrails | The promises hold when things break | `tests/guardrails/<name>` against the composed system: programs killed, receivers stopped or slowed, deliveries repeated or late, the clock moved, two organizations with the same names. Each has a variant with its rule switched off that must fail | The package's own guardrails at merge; all of them in `./scripts/verify` once, in T-061 |
| 4. Browser | A person sees what was decided, at two widths | Playwright: the flows of D-020 to D-024 and T-038; nothing moves that should not; notices appear when a part is behind | `./scripts/verify` |
| 5. The real thing | It works when used | Start it; use it as a person in a browser and as an agent through the door; stop a receiver from the control panel and watch | By hand, at the end of each package that has something to see |
| 6. Independent | The author is not fooling themselves | A fresh `verifier` agent with the scope and the yardsticks, not the reasoning | Once, at the end (T-061), covering D-033, D-034 and the whole system |

Rules for every test: seeded randomness; time only through the clock; no sleeping and hoping; a failing test says what to do. Timed guardrails (G-012, G-013, G-026) run for about a minute at 50 actions a second with the seeded team's size, never at large sizes (CLAUDE.md, Verification). A guardrail test is never weakened to pass; a failing one is reported.

Guardrail to package: G-003, G-004, G-005: T-046 · G-006, G-010, G-011: T-048 · G-007, G-009: T-052 · G-008: T-044 and T-052 · G-012, G-014, G-023, G-026, G-029: T-047 · G-013: T-057 · G-015, G-019: T-053 and T-056, proven in T-058 · G-016, G-017, G-018: T-054 · G-020: T-059 · G-024: T-053 · G-025: T-046, proven for every door in T-057 · G-027: T-057 · G-030: T-049.

## Working in parallel

- **One package, one subagent, one worktree.** Start each with the Agent tool on the main model, in its own git worktree. It commits there; the lead merges into `main` when the package's own checks pass. Between milestones a merge runs `./scripts/check` and the tests of what it touched; the full `./scripts/verify` and the independent verifier run once, at the end, in T-061 (user's instruction, 2026-11-23: "doing all the expensive verifications at the end").
- **A package edits only what it owns** (table above). If it needs a change to a shared file (`server/internal/shared/`, `contracts/`, `tests/harness/`, Compose, Makefile, the check scripts), it writes the request in its task file and carries on against its own stand-in; the lead makes the shared change between waves.
- **The brief for a subagent** holds: its section of this plan; the contracts and guardrails it touches, by path; the directories it owns; "write the guardrail tests first and see them fail"; "do not edit outside your directories"; what to report back (what is true now, how it was checked, what was not checked, requests for shared changes). A subagent does not see `CLAUDE.md` rules unless told, so the brief repeats the ones that matter. Each brief names the agent's own scratch folder: the session scratchpad is shared, and generic file names overwrote each other in Wave 1.
- **Packages meet only through the contracts.** That is what makes the waves parallel: the log writer stands in for Conversations, the recording receiver for the feed and the stand-ins, the mock door for the whole server. Joining happens in T-057 and T-058, where differences show up as failing tests and are fixed against the contract, not by adjusting the other package.
- **Board:** one item in Now per agent. The lead keeps the board; subagents report, they do not edit it.

## Risks, and what to do

- **The feed's rules are the largest single piece (T-049)** and everything visible depends on them. It starts in wave 1 with nothing in its way; if it is late, T-052 and T-055 still proceed against fixtures.
- **Guardrail tests are protected once they have passed** (user's decision, 2026-11-22). Writing one the first time is free. When a package has seen its guardrail test fail and then pass, the lead sets that guardrail to `enforced` in `docs/guardrails.md` and adds `Edit(/tests/guardrails/<name>/**)` to `permissions.ask` in `.claude/settings.json`; `./scripts/check` fails if an enforced guardrail's folder is unprotected. From then on, changing that test asks the user.
- **Two packages disagreeing about a contract.** The schemas and examples of T-045 are the tie-breaker; if they are unclear, fix them there, with the words in `docs/contracts/` as authority.
- **Scope.** The three measures of D-030, a second copy of Conversations in production form, trimming, and an entry point that routes by organization are not built. The tests for G-027 use the harness's small router. Capacity is settled by the written analysis in D-029 and D-034: nothing tests scale inside the product.

## Out of this plan

Sign-in and permissions; creating organizations; real push; edit history; snooze; anything in `docs/known-shortcuts.md` marked as not built.

## Log

- 2026-11-22: plan written. Nothing started.

## Evidence

_None yet._
