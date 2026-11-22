# D-010: The seeded team

- Status: proposed
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-002, D-009
- Evidence: R-002
- Visual: `docs/visuals/D-010-seeded-team.html`

## Why this needs deciding now

The brief asks that the experience "resembles a busy team rather than an empty demo". D-002 settled that the people and agents in it are scripted. This decides who they are and how busy they are, because the remaining product decisions (feed detail, edits and deletes) are best designed against concrete situations, and the first thing built will need this data.

Not open here: the team keeps living while someone watches, since scripted people and agents post, reply, react, edit and delete on their own (D-002). The audience is a small to mid-size startup with mixed roles and heavy AI use (D-001).

## Options

### A. A handful: 6 people, 2 agents, 4 channels

- Upside: quick to write; every character is memorable.
- Risk: it does not feel busy, a feed with three buckets has little to sort, and the case D-001 is built for (agents writing as much as people) barely shows.

### B. A mid-size startup: 12 people, 3 agents, 9 channels

A product company two weeks before a launch. About two weeks of history, roughly 1,500 messages, and live activity at a pace the viewer can pause, slow down or speed up.

- People, by role: founder, product manager, designer, three engineers, marketing, sales, customer support, operations and finance, data analyst, a part-time contractor.
- Agents: a release agent (ships builds, stops for approval), a support agent (sorts incoming customer issues, flags the ones needing a person), a research agent (posts findings and progress while it works).
- Channels: general, product, engineering, design, launch, support, incidents, sales, random. Plus one-to-one direct messages.
- The default viewer is the product manager, the role that sits between everyone. The switcher shows the same moment as the founder, an engineer or support.
- Upside: busy enough that sorting matters; mixed roles; agents produce a visible share of the traffic; every channel dial level and each bucket has a natural example.
- Risk: more to write, and seeded text that reads as filler would undermine the craft. Reduced by writing the situations by hand and generating only the background chatter.

### C. A large team: 30 or more people, 6 agents, 20 channels

- Upside: the feed is under real pressure.
- Risk: beyond the audience D-001 names; nobody can follow the cast; most of the writing effort goes to noise.

## Best practice

Demo data is designed, not generated: start from the situations the product has to show, and write the smallest believable world in which each one happens naturally. Keep a way to replay a situation on demand.

## Recommendation

**B.** It matches the audience, and it is the smallest team in which all the situations below arise without looking staged.

Situations the seeded team must produce, each replayable on demand:

- a flag that names you, from a person and from an agent waiting for approval; a flag that names nobody and closes when someone else answers
- a direct mention, and a mention added by a later edit; a mention removed by an edit
- a message that reached your feed and is then edited to a different length, and one that is then deleted
- a pile of reactions on your message; a reaction taken back
- replies in a thread you started, in one you follow, and in one you turned off
- a burst in the incidents channel, seen with the dial at Focus, Default and Muted
- an agent's progress updates as Info, its result, and its stop for a human
- a courtesy mention marked Info

Locks in: the cast and situations the other product decisions are illustrated with. The pace control exists as a demonstration tool.

Reversibility: cheap. Names, numbers and situations can change at any time.

No second opinion was sought: cheap to reverse.

## Decision

_Open._

## Consequences

_Filled in when accepted._

## Follow-ups

- The names, the company and the wording of the seeded messages are the agent's to write (delegated), shown to the user in the running product.
- D-003 and D-004 use these situations as their examples.
