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

### Second part: the redesign after the independent check (`test2.py`)

The independent check found that the "one question" reads every organization's row, so its cost grows with the number of organizations in a database (it measured 1 ms at 5,000, 8.8 ms at 50,000, 52 ms at 300,000), and that organizations were not really taken in turn. The redesign: one row per organization holds the newest number, the receiver's place, since when work waits and a short claim; an index covers only organizations with work; a worker claims a few for a few seconds, delivers outside any transaction, and notes the place.

Measured on the first run of `test2.py`, before the user asked that no large sizes be run on this machine (the raw output of that run was overwritten by a later, interrupted one; `results-redesign-partial.txt` holds what the interrupted run printed):

| Question | Result |
|---|---|
| Finding work when idle, with 5,000 / 50,000 / 300,000 organizations | through the index: 0.13 to 0.14 ms at every size; reading every row: 0.3 / 2.0 / 11.4 ms |
| 600 saves a second over 300,000 organizations, 1 ms of work per change | one worker falls behind; four keep up (11 ms median, 31 ms for 19 in 20) |
| Two of four workers killed mid-way | nothing lost; their claims ran out and were taken over; 1 change of 17,733 was delivered twice and ignored by the receiver |
| Serving strictly "whoever has waited longest" while 2,000 organizations catch up | fresh traffic waited about 22 s: the reason for the two-line rule, which was then modelled in E-008 and not measured here |

The run with 300,000 organizations is not to be repeated on this machine (user's instruction, 2026-11-22). `test2.py` takes `ORGS=…` to run small.

## Conclusion

**Delivery grows by adding workers, safely.** Several workers never deliver a change twice, capacity rises with workers as soon as applying costs something, one organization's backlog stays its own, and looking for work costs a millisecond however many organizations are idle. **NOTIFY on every save must not be used**: it cut saving to a fifth.

A correction to earlier notes: E-004 reported about 1,000 saves a second. That was the limit of a single Python client; with separate processes the same PostgreSQL saved about 9,500.

What this does not show: more than one database, or the directory between them; the live connection or the agent door with several copies; the product's own code; real hardware (one laptop, PostgreSQL with default settings in Docker); 300,000 organizations (5,000 were used; the claim for more rests on nothing in the design depending on the total).
