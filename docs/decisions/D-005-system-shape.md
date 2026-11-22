# D-005: System shape and stack

- Status: accepted; superseded in part (the server language: D-025)
- Level: system shape
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001 to D-004, D-009 to D-016 (what the product demands)
- Evidence: R-002; the brief; second opinion and claim checks recorded in `tasks/T-022-architecture-draft.md`
- Visual: `docs/visuals/T-022-architecture-proposal.html`, section 5; walk-throughs in `docs/visuals/T-022-walkthroughs.html`

## Why this needs deciding now

The product scope is decided, and nothing can be built until we know which parts exist and how they reach each other. The brief allows the search index and the notification sender to be in-process fakes and asks that their boundaries be treated as real external systems that can be slow or unavailable. This decision sets how real those boundaries are, where the one original copy of the data lives, and how changes reach an open screen. D-006 to D-008, D-017 and D-018 all sit on it.

## Options

### Shape

**A. One program.** Search, notifications and the feed are modules of one process; "slow" and "down" are simulated by a switch. Least to build; the brief permits it. Risk: the boundary is a convention, a shortcut across it goes unnoticed, and nothing can really be stopped. Reversal: moderate.

**B. Parts that talk only over the network.** One codebase and one image. Conversations (the core), the feed, the fake search index, the fake notification sender and the scripted team each run as their own process under Docker Compose and reach each other only by HTTP. Upside: a receiver can really be stopped, slowed or wiped, and the failure tests do exactly that. Risk: five processes to start and watch; slower tests. Reversal: cheap toward A or C.

**C. B plus a message broker** (Kafka, NATS, Redis Streams) between the core and the receivers. What large products run. Risk: one more system to operate and explain; it still needs the save-and-log step to get changes in safely and receivers still see repeats. At about 1,500 messages a day it adds nothing we can show. Reversal: moderate.

### Where the truth is stored

**PostgreSQL in Docker**, with a separate schema per part and no part reading another's. Against **SQLite files**: nothing to run, but one writer at a time and no way to wake a delivery worker the moment something is saved (PostgreSQL's NOTIFY is delivered on commit; checked against its documentation 2026-11-22).

### How changes reach a screen

**WebSocket with our own numbered updates and "resume after N"**, falling back to a fresh load when too far behind. Against **server-sent events**: reconnection is built in, but outside HTTP/2 a browser allows six connections per site, so on plain `http://localhost` the seventh tab hangs (MDN, checked 2026-11-22). The user will open many tabs as different people. Against **long polling**: the same limit, more requests.

### Stack

Python 3.12 on the server (installed). The web app's toolkit is left open until the look-and-feel experiment (T-019) shows what the screens need; Node is not installed and could run only in Docker.

## Best practice

Every product studied in R-002 persists first, publishes each change as an event, and lets search, notifications and the feed consume asynchronously. At their scale that is a broker; at ours the standard form is a transactional outbox table and a worker per consumer. Clients pair a live connection with sequence numbers and a catch-up rule (Discord, Zulip). This proposal follows that practice; it departs only in not using a broker, for the reason under C.

## Recommendation

**B, PostgreSQL, WebSocket with numbered updates, Python on the server.** B is the cheapest shape in which an outage is real. The Compose mode with separate processes is the one the tests run; running everything in one process is a convenience only.

One detail the guarantee rests on: log numbers are assigned one at a time, in commit order, so a worker reading "after N" cannot step over a change that committed late (database sequences alone do not give this; sequence values are handed out before commit).

Locks in: a single source of truth with a numbered change log; HTTP between parts; one database server. Reversibility: the shape is cheap to change, the database and the log are expensive, the live connection is cheap.

Second opinion (a fresh agent, given the options without this recommendation): chose B, PostgreSQL and WebSocket; warned that "together or apart" must not become two modes of which one is untested, and that log numbering must follow commit order. Both are taken up above.

## Decision

Accepted by the user on 2026-11-22, in four parts.

- **The database: PostgreSQL.** Chosen by the user on 2026-11-22: "let's use PostgreSQL as an industry standard". The user added that it probably does not influence the architecture much; E-004 agrees (one implementation rule, no change of shape).
- **The shape: B, parts that talk only over the network and run as separate programs from one codebase, without a message broker.** Decided by the user on 2026-11-22: "ok let's do without message broker". The user first chose C ("I would choose a message broker. I think it's important part of scalable architecture"), asked for the case for it, and after reading the case and its limits (`docs/visuals/D-005-message-broker.html`) chose B, asking to be told when a broker would become necessary. That answer is on the same page and summarised below.
- **PostgreSQL runs in Docker** (confirmed by the user, 2026-11-22).
- **How changes reach a screen: WebSocket with numbered updates and "everything after number N".** "I agree with your recommendation" (2026-11-22).
- **The server is written in TypeScript**, against the agent's proposal of Python: "i don't like python. I'd rather choose typescript" (2026-11-22). The agent sees no risk to the goal in this: one language for server and web app, and the contracts can be shared as types. Consequences: Node is needed (not installed on this machine; it runs in Docker or is installed with the first implementation task); the experiments E-001 and E-003 to E-005 stay as throwaway Python, which is their purpose, and what they showed concerns the design, not the language; the PostgreSQL numbering rule (E-004) holds for any client.

When a broker becomes the right choice (any one of these): receivers owned by other teams or other codebases, or more than a handful of them; a receiver that needs several copies working side by side to keep up; sustained traffic in the high hundreds of changes a second, where one database is both the truth and what every receiver reads; systems that are not ours consuming the changes; more than one service producing changes; several live-connection servers that each need the stream. Because the save-and-log step, the numbered log and receivers that ignore repeats stay as they are, adding a broker later replaces only the delivery workers with a relay and readers.

Evidence since the proposal: E-003 built shape B small and broke it on purpose; it held. E-004 confirmed on PostgreSQL 17 that plain sequence numbering makes a worker miss about a fifth of the changes under concurrent writes, that numbering one writer at a time misses none at no measurable cost (about 1,000 changes a second either way), and that NOTIFY wakes a worker in about a millisecond.

## Consequences

- Easier: failure tests stop real processes; each part can be explained and replaced alone.
- Harder: starting and observing several processes; every call between parts needs a timeout and a failure answer.
- Committed to, if accepted: one codebase, HTTP boundaries, PostgreSQL, a numbered change log, numbered live updates.

## Follow-ups

- Experiment "the spine" (freshness under a burst; no change skipped under concurrent writers) before the first implementation task.
- Guardrails: no part reads another part's schema; every outbound call has a timeout.
- D-017, D-018, then D-006 to D-008.
