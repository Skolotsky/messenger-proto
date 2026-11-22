# D-010: The seeded team

- Status: accepted
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

Accepted by the user on 2026-11-22: **C, a large team, plus a control panel.** The agent recommended B.

- "I think that focus mode pays off when it's 30 or more." The bet in D-001 is about attention under pressure, and a team of 12 does not put Focus under pressure.
- **A separate control panel** to start, stop and reset the demonstration. It shows plainly what has happened and what is going to happen, so the user can use it while explaining the product.

The agent's stated risk with C (nobody can follow the cast; most writing effort goes to noise) is handled this way: a core cast of about eight people and the agents carries the hand-written situations listed above, with the default viewer among them; the other people and channels produce generated background traffic. The situations to produce stay as listed under the recommendation.

## Consequences

- Easier: Focus, the channel dial and one row per conversation are shown under the load they exist for. The gap between Default and Muted on a busy channel is obvious.
- Harder: about 30 people, 6 agents and 20 channels have to stay believable. More history, so the first load and search have real volume to deal with.
- Committed to: the control panel as a surface of its own, outside the product's navigation. Reset returns the team to the same starting state every time, and the scripted activity follows a schedule that can be shown in advance, which is what makes "what is going to happen" possible. The panel also offers each situation on demand and the pace control. Controls to slow down or switch off search and notifications will live there too, when the quality decisions (D-007) define them.

## Follow-ups

- The names, the company and the wording of the seeded messages are the agent's to write (delegated), shown to the user in the running product.
- D-003 and D-004 use these situations as their examples.
