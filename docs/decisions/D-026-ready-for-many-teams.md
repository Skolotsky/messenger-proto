# D-026: The architecture is ready for many teams in one installation, and scales by team

- Status: accepted
- Level: system shape
- Date: 2026-11-22
- Decided by: user (the direction); the scope below is the agent's reading, open to objection
- Depends on: D-005, D-006, D-017, D-018, D-025
- Narrows: D-005 (one change log becomes one per team), D-006 and G-009 (a fence limit, a parked change and a stopped feed are per team)
- Evidence: E-004, second part (a numbered log per team on PostgreSQL)
- Visual: `docs/visuals/T-022-architecture-proposal.html`, section "D-026"

## Why this needs deciding now

The design assumed one team per installation: one change log for everything. Asked when a message broker would be needed, the agent found that many teams in one installation would let one team's burst delay every other team, and that the first remedy is a log per team. The user then said: "Let's make architecture to be multitenant ready. and scalable therefore." Before any product code exists this costs a key on every table and contract; afterwards it means migrating all data and every boundary.

## Options

### A. Stay with one team per installation

Nothing to do. Risk: a second team needs a second installation, or a rebuild of the data model and every contract.

### B. Ready for many teams: the team is part of every key, and every team has its own log

Nothing is built for managing teams, but everything that would be expensive to change later is done from the start. Cost: a team on every table, contract and test; tests need two teams.

### C. Many teams fully built

B plus creating and administering teams, sign-in, limits per team, several copies of each program actually running, teams spread over several databases. Risk: most of it does not exercise the feed or the delivery guarantees the brief asks for ("focus on the messaging experience and activity feed rather than trying to build a complete product").

## Best practice

Make the tenant part of every key and every boundary from the first day, enforce separation in the data store itself, and keep tenants independent of each other, so that scaling is done by spreading tenants and never needs an order across them. Build the management of tenants when there is a second tenant.

## Recommendation

**B.** It is what "ready" means, it is cheap now, and E-004 shows the one mechanism it rests on works.

## Decision

Accepted by the user on 2026-11-22, as the direction: **B.** The scope, as the agent reads it:

1. **A team is on everything.** Every message, reader fact, log entry, feed row, search entry, notification note, agent, and connection belongs to exactly one team. PostgreSQL enforces it: a database session acts for one team and cannot see or change another team's rows.
2. **One numbered change log per team.** Numbers are handed out one writer at a time within a team. Teams never wait for each other and have no common order.
3. **Delivery keeps a place per receiver and team**, and takes the teams in turn, so one team's backlog cannot starve another. A receiver that is down is down for every team; a refused change that is parked, a fence, and a feed that stops after five fences are per team.
4. **The feed is worked out per team**, and is rebuilt, fenced and reset per team. Clock ticks are written per team while that team is active.
5. **Every contract is scoped by team**: the team is in the address and in every change, note, update and event; all numbers count within the team.
6. **A person and an agent belong to one team.** One identity across several teams is not part of this.
7. **How it scales, which is why teams are kept independent:** several copies of each program can each take a share of the teams; teams can be spread over several databases; a message broker becomes worth it when assigning and balancing teams is itself the work (`docs/visuals/D-005-message-broker.html`). What this does not give: one very large team still has one log and one feed.
8. **Not built:** creating or administering teams in the product, sign-in, limits per team, several copies of a program actually running, more than one database. The first version runs the seeded team; a second small team exists so that the tests can prove separation.

Evidence (E-004, second part, PostgreSQL 17, 8 teams, two writers each): a follower per team missed 0 of 3,200 changes. With one team keeping every save open for 300 ms, saves in the other teams took up to 2.6 s (19 in 20 within 317 ms) with one log for everybody, and up to 34 ms (19 in 20 within 24 ms) with a log per team. Flat out, both forms saved about 850 to 1,030 changes a second: a log per team buys independence, not more speed from one database.

## Consequences

- Easier: a second team is data, not a redesign; scaling is spreading teams; one team's trouble stays in that team.
- Harder: every table, call and test carries a team; the delivery workers must be fair between teams; tests run with two teams.
- Committed to: the team as part of every key and contract; a log per team; separation enforced by the database.

## Follow-ups

- Contracts updated (the team in every address and payload); guardrails G-025 (nothing crosses teams) and G-026 (one team cannot delay another) added; G-005, G-009, G-014 and G-020 read "per team".
- Open for the user: whether the product shows a second team (a switcher next to the person switcher), or the second team exists only for the tests.
