# D-029: The system grows by adding copies and databases; nothing but a small directory is shared between organizations

- Status: accepted
- Level: system shape
- Date: 2026-11-22
- Decided by: user (the requirement); the mechanisms are the agent's, open to objection
- Depends on: D-005, D-026, D-028
- Narrows: D-026 (point 7, how it scales), D-007 and G-024 (how "watching" lapses)
- Evidence: E-007 (5,000 organizations on PostgreSQL), E-006 (last part), E-004
- Visual: `docs/visuals/D-029-scaling-by-adding.html`

## Why this needs deciding now

Asked why the design has no cache, the agent answered "nothing needs it at this size". The user: "but we are multitenant. so we may have thousands of organizations", and then: "it isn't about concrete number. It is about scaling, so we are sure that we would work even for 300k orgs just adding more instances/shards/etc". D-026 made every key and contract per organization but left open how programs and databases are multiplied ("prepared, needs more design"). This record closes that: for every part it says what is added to grow, and removes the things that could not be grown.

## Options

### A. Leave it at "prepared" (D-026)

Per-organization keys and logs only. Risk: the first attempt to run a second copy or a second database meets undesigned questions (who serves which organization, how workers avoid each other, what wakes whom), and three things in the current design do not grow at all (below).

### B. Design the growth now, build only what is free

Fix a rule every part obeys, name what is added to grow each part, and build in the first version the pieces that cost nothing extra and would be expensive to retrofit. A second database, moving organizations and trimming old data stay unbuilt.

### C. Build it: several copies, several databases, a pub/sub system

Proves it by running it. Risk: operational work that shows nothing about the feed or the delivery guarantees the brief asks for.

## Best practice

Scale a multi-tenant system by keeping tenants independent and making every component either stateless (its state in a store keyed by tenant) or reachable by tenant, so capacity is added by adding copies and by spreading tenants over stores; keep the one global thing, the tenant directory, small and read-mostly. Avoid mechanisms whose cost grows with the total (a global lock, a global wake-up, polling every tenant).

## Recommendation

**B.**

## Decision

Accepted on the user's requirement, 2026-11-22. The rule: **every piece of state belongs to one organization, and every program either keeps its state in PostgreSQL or keeps only memory about organizations routed to it.** From that:

1. **Databases.** Organizations are spread over several PostgreSQL databases. A small directory says which database holds an organization; programs look it up and remember it. Conversations' area and the feed's area may sit on different servers. No question ever spans two organizations, so nothing spans two databases.
2. **Programs without memory grow by adding copies**: the door for people's plain calls, the delivery workers, the feed, the connectors to search and to the notification sender. Workers ask one question per database, "which organizations have work?", and take each organization under a lock that other copies skip. Two copies can therefore never deliver the same change at the same time, and no copy has to be told which organizations are its own.
3. **Programs with memory are reached by organization**: the live connection, the agent door and text of replies in progress. Everything a screen or an agent of one organization does goes to one copy of Conversations, chosen from the organization in the address. That copy made the saves, so it knows at once what to push; nothing has to be passed between copies. When copies are added, some organizations move; their screens and agents reconnect and catch up by number, which they can already do.
4. **Waking up.** Workers are not woken through the database on every save. They ask the one question every few tens of milliseconds.
5. **Housekeeping runs per database** under a lock one copy holds at a time: the clock tick for an organization gone quiet, "watching" that lapsed, replies that were abandoned, agents' events older than 7 days.
6. **"Watching" is renewed without writing to the log.** A screen still reports every 20 s; Conversations only notes the time. One "no longer watching" is written to the log when 60 s pass without a report. (As first written, every renewal was a log entry: three a minute for every person with a conversation open, more than all other changes together.)
7. **Old data is trimmed per organization** once a saved copy of each receiver exists for it: the change log, the IDs of handled actions, agents' events, sent notifications. A rebuild then starts from the saved copy. Not built; without it these grow without bound.

Built in the first version, because it is free now and expensive later: the organization in every key and address; a log per organization; workers that find work with one question and take organizations under a skipped lock, run as two copies in the tests; housekeeping under a lock; every database access going through "the database for this organization", which today returns the only one; watching renewed off the log. Not built: a second database and the directory behind it, moving an organization, trimming, the entry point that routes by organization (one copy needs none).

What does not grow this way, stated plainly: one organization larger than one database or one worker can carry (an organization's log is applied by one worker at a time: 50,000 waiting changes took about 50 s in E-007); the directory and the entry point, which must be kept available by ordinary means; the search engine and the push provider, which scale on their own side.

Evidence (E-007, 5,000 organizations in one PostgreSQL 17, every writer and worker a separate process, on this laptop):
- Saving: 9,549 changes a second; 2,043 when every save also wakes the workers through the database. (The "about 1,000 a second" quoted from E-004 was the limit of that test's single client program, not of PostgreSQL.)
- Delivering with several workers: 120,000 changes delivered, each exactly once, with 1, 2, 4 and 8 workers.
- Adding workers adds capacity: at 600 saves a second with 1 ms of work per change, one worker fell behind (25 s for 19 in 20); four kept up (9 ms median, 17 ms for 19 in 20).
- One organization with 50,000 waiting changes did not delay the others (8 ms median, 35 ms worst).
- Finding work: one question took 1 ms; asking each of 5,000 organizations took 719 ms a round.

### Corrections after the independent check, 2026-11-22 (the decision stands; these mechanisms replace the ones above where they differ)

A fresh verifier tried to break the claim and did, in four places. The answers, by analysis, with the small evidence named:

1. **Finding work no longer reads idle organizations.** As first written, "which organizations have work?" read every organization's row (the verifier measured 1 ms at 5,000 organizations, 52 ms at 300,000, asked many times a second). Now one row per organization holds the newest number, each receiver's place, since when work waits and a short claim, and an index covers only the organizations that have work. Cost is independent of the idle ones (E-007, second part: 0.13 ms at 5,000, 50,000 and 300,000).
2. **Workers claim, deliver outside any transaction, then note the place.** A claim lasts a few seconds; one that runs out is taken over. A change can therefore be delivered twice. Delivery stays correct because receivers ignore repeats (D-006), not because of the claim; point 2 above overstated the lock.
3. **Turns.** Organizations that one turn will clear are served first; every round also takes some that are far behind. Strictly "whoever has waited longest" made fresh traffic wait behind a wave of catching up (seen in E-007; the two-line rule is modelled in E-008).
4. **Everything of an organization goes to its one copy of Conversations, plain calls included.** Point 2 above wrongly listed the door for people's plain calls among the programs multiplied freely.
5. **What a copy did not save itself reaches it by a nudge and a regular check.** A row the feed changed, a tick or an ended "watching" written by housekeeping, a save finished by the copy that served the organization before a move: the writer sends a nudge through the organization's address, which may be lost, and every copy checks every 10 seconds the organizations it has open screens for. The cost follows organizations with open screens. No call is held open per person at the feed.
6. **"Watching" lasts as long as the connection it was said on.** The connection is checked every 20 s; start and end are logged; nothing is written in between. Each copy holds a short lease in the database; housekeeping ends the watchings of a copy whose lease ran out. This replaces point 6 above (a time noted per report).
7. **Housekeeping asks through indexes too** (who needs a clock tick: 0.12 ms at every size), and its lock passes to another copy within seconds if the holder dies.
8. **The pause before retrying a receiver that is down is kept once per receiver**, so more workers do not mean more retries; how often a change was refused is kept with the organization's place.
9. **A program that asks the wrong database** is told "not here" and looks the organization up again.
10. Guardrails: G-027 now covers two copies of Conversations and an organization moved mid-burst, and counts repeats; G-029 holds every timer question to a cost that does not rise with idle organizations.

Withdrawn: "looking for work costs a millisecond however many organizations are idle" (measured at one size only); "can therefore never deliver the same change at the same time"; "about 50 s" for the large organization's backlog (that run had not finished). Not measured, and not to be measured on the user's machine: 300,000 organizations under load. The claim that the design grows by adding rests on the analysis: after these corrections no question, lock, timer or table in it depends on the total number of organizations.

One organization growing large is decided separately in D-030.

## Consequences

- Easier: capacity is added by adding copies and databases; no new kind of system (a cache, a broker) is needed for it.
- Harder: every database access must go through the directory; background jobs need a lock; the tests run with two copies.
- Committed to: the rule above; no mechanism whose cost grows with the number of organizations.

## Follow-ups

- Contracts: watching (screen to Conversations), workers and wake-ups (to search, to the feed), one copy per organization (live connection, agent door).
- Guardrails: G-027 (two copies of every program without memory: nothing lost, nothing twice), G-028 (two organizations on two databases: everything holds); G-024's mechanism.
- A cache: still none. What thousands of organizations need first is in this record; a shared in-memory store would only be needed if screens of one organization were served by several copies, which point 3 avoids.
