# D-030: A single organization grows too: conversations side by side, saves in small groups, large channels kept once

- Status: accepted
- Level: system shape
- Date: 2026-11-22
- Decided by: agent, on the user's request for analytical decisions
- Depends on: D-026, D-029, D-017
- Evidence: analysis below; E-008 (simulation A)
- Visual: `docs/visuals/D-029-scaling-by-adding.html`, section "How large can one organization be"

## Why this needs deciding now

D-029 grows the system by spreading organizations, and named one limit it does not lift: one very large organization. The user asked: "what is the approximate current cap for organization size knowing typical DB ability to hold the load? how could we make it scalable too? is it complicated to put into architecture as well?", and then asked for analytical decisions instead of measurements on their machine.

## The cap today, by analysis

Assumptions, stated so they can be challenged:
- A person causes about 150 changes a working day (E-001's traffic gives about 37 messages a person a day; reactions, reads and settings make up the rest). The busiest second is about four times the working-day average: **about 0.02 changes a second per person**.
- A typical production PostgreSQL (16 to 32 cores, fast disks, a second copy confirming each write) commits a small transaction in **1 to 3 ms**.

Three things in an organization happen one at a time:

| One at a time | Why | Rate |
|---|---|---|
| Saving a change | the organization's next number is handed out under a lock held until the save is confirmed | 300 to 1,000 a second |
| The feed applying a change | one change after another per organization, each a transaction of 2 to 20 row writes (2 to 5 ms) | 200 to 500 a second |
| A message that concerns a whole channel (a mention of everyone, a flag that names nobody, members who set the channel to Focus or All activity) | one row is written per member | 0.2 to 0.5 s for 10,000 members, during which that organization's feed waits |

These rates are estimates from the assumptions above, not measurements.

## Options

### A. Leave it and state the cap

Nothing to do. A very large organization is not served well.

### B. Lift the cap inside the parts, keeping one log per organization

Three measures, none of which changes a contract between parts:
1. **The feed applies an organization's conversations side by side.** Order is kept inside a conversation (a channel with its threads, or a direct conversation); changes about a reader as a whole (Focus rules, Focus people, "Focus only") stay at their place in the log and everything before them is finished first. The feed's capacity for one organization then grows with workers.
2. **Saves of one organization are written in small groups.** One copy of Conversations serves an organization (D-029), so it can collect the saves that arrive within a few milliseconds, hand out their numbers together and confirm them with one write. One write then carries tens of saves.
3. **What concerns a whole large channel is kept once, not once per member.** In a channel above 500 members, an item that reaches a person only because they are a member (they are not named, it is not their thread) is stored once for the channel. A reader's feed and count combine their own rows with the channel items after their read position.

After these, about **100,000 people** in one organization (2,000 changes a second at the busiest time, 15 million a day): the limits left are the organization's log being read by one delivery claim at a time, which reading can sustain, and the organization living in one database.

### C. A log per conversation

Every channel and every direct conversation gets its own numbered log; an organization is then nothing that happens one at a time. No cap from the design short of one conversation and one database. Cost: every contract's numbers become per conversation; a screen follows many numbers instead of two; the feed needs every reader's settings kept by version so that a replay gives the same result; taking turns needs two levels (organizations, then conversations). A redesign of contracts and guardrails.

## Best practice

Keep the unit of strict order as small as the product's rules need, and no smaller than can be explained and tested. Systems of this kind order by conversation at very large size, and fan out to readers when read for very large audiences. Lift a limit where it binds, in the smallest step that removes it.

## Recommendation and decision

**B**, decided by the agent on the user's request for analytical decisions (2026-11-22). It lifts the cap roughly tenfold without changing any contract, and each measure can be added when an organization of that size exists.

Decided with it, to be true from the first version:
- **The rule that makes B and C possible** is a guardrail now (G-030): the feed's result for a conversation depends only on that conversation's changes, in their order, and on reader-wide changes at their place in the log. E-008 replayed 200 different interleavings of conversations against the modelled rules: none changed the feed; with the reader-wide changes moved, all did.
- **Which changes are reader-wide** is part of the feed contract: Focus rules, Focus people and their option, "Focus only". Everything else belongs to one conversation: messages, reactions, read positions, watching, Keep and Done, a channel's or thread's level, membership of a channel.
- **The feed's rows may be "for one reader" or "for a whole channel"** in its store, so measure 3 needs no change of shape later.

Not built in the first version: the three measures themselves. With a seeded team of 36 the feed applies one change at a time; the guardrail keeps the door open.

C is the step after B, for organizations beyond about 100,000 people. It is not adopted because it changes every contract for a size no first version meets. Is it complicated? B: no, it is inside the feed and inside Conversations. C: yes, moderately, for the reasons above.

### Corrections after an independent check, 2026-11-22 (the decision stands)

- **"None of which changes a contract" was too strong.** Measure 1 keeps the delivery boundary as it is (the feed takes changes strictly in order and confirms each; inside, it works on conversations side by side), but it needed one contract to be made stricter: the notification sender's clock now moves only on clock notes, which the feed writes when everything of the organization up to that time has been applied. Otherwise a later note from one conversation could release a notification before another conversation's withdrawal had been written. The feed-to-sender contract and G-011 say so.
- **Two more things happen one at a time** and belong in the table: the feed's own numbered log of notes (one number hand-out per organization, a few hundred a second, enough for about 100,000 people; grouped like saves beyond that), and delivery to search and to the sender one change per call (lifted by handing over several changes per call, as the feed contract already allows for a backlog).
- **Clock ticks are organization-wide**, like the reader-wide changes: everything before a tick is finished before it counts.
- **About 100,000 people is also where one copy of Conversations holding every connection of one organization ends.** Beyond it an organization's screens would need several copies and something shared between them; that is the moment for a shared in-memory store.
- **A flag for everyone in a very large channel still means one notification per member.** Measure 3 keeps the feed from writing a row per member; the notifications are the product's rule (D-012) and are spread out by the sender.
- **The simulation is weaker evidence than first stated.** It models a subset of the rules with none of the product's reader-wide kinds, replays one after another and not at the same time, and at first compared the notification notes without their order. It was tightened (longer stretches between organization-wide changes; notes compared in order per conversation) and still shows no difference, but the proof has to be G-030's test on the product's own rules.

## Consequences

- Easier: a clear answer to "how large can one organization be", with the next two steps named.
- Harder: the feed's rules must keep the independence of conversations; a new rule that needs something from another conversation has to be made reader-wide on purpose.
- Committed to: G-030; the list of reader-wide changes; the two kinds of row.

## Follow-ups

- Guardrail G-030; the feed contract names reader-wide changes.
- E-001's rules (Focus rules, a person rule that "lifts what already reaches you from them") must be checked against the independence rule when they become the product's tests; a person rule is reader-wide, which fits.
