# D-026: The architecture is ready for many organizations in one installation (multi-tenant), and scales by organization

- Status: accepted; point 7 (how it scales) is worked out in D-029
- Level: system shape
- Date: 2026-11-22
- Decided by: user (the direction); the scope below is the agent's reading, open to objection
- Depends on: D-005, D-006, D-017, D-018, D-025
- Narrows: D-005 and D-017 ("the one log" becomes one log per organization), D-007 (Reset and a paused receiver are per organization), D-006 and G-009 (a fence limit, a parked change and a stopped feed are per organization)
- Evidence: E-004, second part (a numbered log per organization on PostgreSQL; the test code calls them teams)
- Visual: `docs/visuals/T-022-architecture-proposal.html`, section "D-026"

## Why this needs deciding now

The design assumed one organization per installation: one change log for everything. Asked when a message broker would be needed, the agent found that many organizations in one installation would let one organization's burst delay every other organization, and that the first remedy is a log per organization. The user then said: "Let's make architecture to be multitenant ready. and scalable therefore.", and clarified: "multitenant means multiple organizations. we don't have org switcher for the current scope." An organization is the tenant; the seeded team of about 30 people (D-010) lives inside one organization. Before any product code exists this costs a key on every table and contract; afterwards it means migrating all data and every boundary.

## Options

### A. Stay with one organization per installation

Nothing to do. Risk: a second organization needs a second installation, or a rebuild of the data model and every contract.

### B. Ready for many organizations: the organization is part of every key, and every organization has its own log

Nothing is built for managing organizations, but everything that would be expensive to change later is done from the start. Cost: an organization on every table, contract and test; tests need two organizations.

### C. Many organizations fully built

B plus creating and administering organizations, sign-in, limits per organization, several copies of each program actually running, organizations spread over several databases. Risk: most of it does not exercise the feed or the delivery guarantees the brief asks for ("focus on the messaging experience and activity feed rather than trying to build a complete product").

## Best practice

Make the tenant part of every key and every boundary from the first day, enforce separation in the data store itself, and keep tenants independent of each other, so that scaling is done by spreading tenants and never needs an order across them. Build the management of tenants when there is a second tenant.

## Recommendation

**B.** It is what "ready" means, it is cheap now, and E-004 shows the one mechanism it rests on works.

## Decision

Accepted by the user on 2026-11-22, as the direction: **B.** The scope, as the agent reads it:

1. **An organization is on everything.** Every message, reader fact, log entry, feed row, search entry, notification note, agent, and connection belongs to exactly one organization. PostgreSQL enforces it: a database session acts for one organization and cannot see or change another organization's rows.
2. **One numbered change log per organization.** Numbers are handed out one writer at a time within an organization. Organizations never wait for each other and have no common order.
3. **Delivery keeps a place per receiver and organization**, and takes the organizations in turn, so one organization's backlog cannot starve another. A receiver that is down is down for every organization; a refused change that is parked, a fence, and a feed that stops after five fences are per organization.
4. **The feed is worked out per organization**, and is rebuilt, fenced and reset per organization. Clock ticks are written per organization while that organization is active.
5. **Every contract is scoped by organization**: the organization is in the address and in every change, note, update and event; all numbers count within the organization.
6. **A person and an agent belong to one organization.** One identity across several organizations is not part of this.
7. **How it scales, which is why organizations are kept independent.** What is in place: every key, log, place and contract is per organization, and no question spans two. What that makes possible later, each needing a design of its own that is not done here: several copies of a program each taking a share of the organizations (needs each organization assigned to exactly one copy, because open connections, text of replies in progress and agents' limits live in a copy's memory); organizations spread over several databases (needs a directory saying which database holds which organization). A message broker is the standard way to get that assignment and balancing (`docs/visuals/D-005-message-broker.html`). What this does not give: one very large organization still has one log and one feed, and all organizations on one database share its capacity, about 850 to 1,030 saves a second in the test.
8. **Not built:** creating or administering organizations in the product, sign-in, limits per organization, several copies of a program actually running, more than one database. The first version runs one organization with the seeded team in it and has no organization switcher (the user, 2026-11-22); a second small organization exists only so that the tests can prove separation.

Evidence (E-004, second part, PostgreSQL 17, 8 organizations, two writers each): a follower per organization missed 0 of 3,200 changes. With one organization keeping every save open for 300 ms, saves in the other organizations took up to 2.6 s (19 in 20 within 317 ms) with one log for everybody, and up to 34 ms (19 in 20 within 24 ms) with a log per organization. Flat out, both forms saved about 850 to 1,030 changes a second: a log per organization buys independence, not more speed from one database.

## Consequences

- Easier: a second organization is data, not a redesign; scaling is spreading organizations; one organization's trouble stays in that organization.
- Harder: every table, call and test carries an organization; the delivery workers must be fair between organizations; tests run with two organizations.
- Committed to: the organization as part of every key and contract; a log per organization; separation enforced by the database.

## Follow-ups

- Contracts updated (the organization in every address and payload); guardrails G-025 (nothing crosses organizations) and G-026 (one organization cannot delay another) added; G-005, G-009, G-014 and G-020 read "per organization".
- Corrected after an independent check (2026-11-22): a receiver now has an answer for "this organization is stopped" that differs from "down"; refusals pause one organization only; a notification's name begins with the organization; the hold, the feed's own log and every place are stated per organization; clock ticks became one tick per quiet period (a tick a second per active organization would by itself use up the database at about a thousand active organizations); the scaling claims in point 7 say what further design they need. The sparser tick is not tried: E-003 used a tick every 0.1 s.
- Settled by the user on 2026-11-22: the product has no organization switcher in this scope; the second organization exists only for the tests.
