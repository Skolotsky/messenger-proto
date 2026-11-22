# E-007: Does delivery from the PostgreSQL logs grow by adding workers, with thousands of organizations?

- Date: 2026-11-22
- Time box: one hour
- Feeds: D-029 (scaling by adding), D-026
- Visual: `docs/visuals/D-029-scaling-by-adding.html`

## Hypothesis

With a numbered log per organization, delivery can be shared by any number of workers if each finds the organizations that have work with one question and takes an organization under a lock other workers skip. We expected: nothing delivered twice, capacity rising with workers once applying a change costs something, one organization's backlog not delaying others, and the cost of finding work not growing with the number of idle organizations. We suspected waking workers through the database (NOTIFY) on every save would hurt at a high save rate.

## Method

PostgreSQL 17 in Docker with 5,000 organizations. Writers save changes as Conversations would (a counter row per organization hands out the number; change and number in one transaction). Workers deliver by marking changes applied. Every writer and worker is its own process (Python, psycopg), so the client is not the limit. Rerun, about six minutes: `./experiments/E-007-thousands-of-organizations/run.sh`

## What would change our mind

- A change delivered twice or missed with several workers: the lock scheme is wrong.
- No gain from adding workers under load: delivery does not grow by adding.
- A large cost of NOTIFY: workers must be woken another way.

## Result

Raw output: `results.txt`.

| Question | Result |
|---|---|
| Saving, 16 writers flat out | 9,549 changes a second |
| The same, each save also waking workers with NOTIFY | 2,043 a second |
| 120,000 waiting changes delivered by 1, 2, 4, 8 workers (no work per change) | each delivered exactly once; about 17,000 to 19,000 a second regardless of workers (the database is the limit when applying costs nothing) |
| 600 saves a second, 1 ms of work per change, one worker | falls behind: 1.2 s median, 25.6 s for 19 in 20 |
| The same with four workers | keeps up: 9 ms median, 17 ms for 19 in 20, worst 45 ms |
| One organization gets 50,000 changes at once, the others at 400 saves a second, four workers | the others: 8 ms median, 16 ms for 19 in 20, worst 35 ms. The large one is applied by one worker at a time and needs about 50 s for its own backlog |
| Finding work when idle | one question: 1.0 ms. Asking each of 5,000 organizations: 719 ms a round |

## Conclusion

**Delivery grows by adding workers, safely.** Several workers never deliver a change twice, capacity rises with workers as soon as applying costs something, one organization's backlog stays its own, and looking for work costs a millisecond however many organizations are idle. **NOTIFY on every save must not be used**: it cut saving to a fifth.

A correction to earlier notes: E-004 reported about 1,000 saves a second. That was the limit of a single Python client; with separate processes the same PostgreSQL saved about 9,500.

What this does not show: more than one database, or the directory between them; the live connection or the agent door with several copies; the product's own code; real hardware (one laptop, PostgreSQL with default settings in Docker); 300,000 organizations (5,000 were used; the claim for more rests on nothing in the design depending on the total).
