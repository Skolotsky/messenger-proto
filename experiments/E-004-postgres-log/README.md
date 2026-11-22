# E-004: On PostgreSQL, can a worker step over a change that was saved a moment late, and what does preventing it cost?

- Date: 2026-11-22
- Time box: one hour
- Feeds: D-005 (the database part, chosen by the user on 2026-11-22: PostgreSQL), D-006, D-007
- Visual: `docs/visuals/T-022-architecture-proposal.html`, section "Evidence"

## Hypothesis

Every receiver is fed by a worker that remembers "the highest number I have delivered" and asks the log for what comes after it. PostgreSQL hands out sequence numbers when a save starts, not when it finishes, so a save that finishes late can get a lower number than one already delivered, and the worker never sees it. E-003 could not test this (SQLite has one writer). We expected the loss to be real and the fix (hand out numbers one writer at a time) to cost little at our size.

## Method

PostgreSQL 17.12 in Docker; a Python client with the psycopg driver in a second container. 16 writers save 4,000 changes at once, each in its own transaction, taking its number as the last step before the commit; in the first test some keep the transaction open up to 3 ms longer. A reader follows the log the way a delivery worker would. Three ways of numbering are compared. Separately: how soon a listening worker has fetched a change when woken by NOTIFY sent in the same transaction.

Rerun, about two minutes: `./experiments/E-004-postgres-log/run.sh` (starts and removes its own PostgreSQL).

## What would change our mind

- No loss with a plain sequence: the rule is unnecessary.
- The fix costing most of the throughput: read the database's replication log instead, or number per conversation.
- NOTIFY wake-up in the hundreds of milliseconds: the 2-second freshness target needs another mechanism.

## Result

Raw output: `results.txt`. Three runs per numbering in the first test.

| How a change gets its number | Changes the reader never saw, of 4,000 | Saved per second, 16 writers flat out |
|---|---|---|
| A database sequence | 941, 948, 919 (and 813 with nothing held open) | 954 |
| A sequence, one writer at a time until its commit (advisory lock) | 0, 0, 0 | 1,071 |
| A one-row counter (its row lock lasts until commit) | 0, 0, 0 | 1,049 |

Save to "a worker has fetched it", woken by NOTIFY, 400 changes at 50 a second: median 1.0 ms, 19 in 20 within 1.3 ms, worst 7.4 ms.

## Conclusion

**The trap is real and large: with ordinary sequence numbering a worker silently misses about a fifth of the changes under concurrent writes. Numbering one writer at a time removes it completely and costs nothing measurable** (about 1,000 changes a second either way on this machine, against roughly 0.02 a second for the seeded team and 1.5 at a hundred times its size). NOTIFY wakes a worker within about a millisecond, so the 2-second target has three orders of magnitude to spare at the database.

The architecture does not change. One implementation rule becomes a guardrail: the log number is taken last in the transaction, one writer at a time, and a test with concurrent writers must fail without it.

What this does not show: behaviour with a database on another machine, under replication or failover; long transactions (they would hold up numbering for everyone, which is why numbering comes last); the alternative of reading only below the oldest open transaction, which was not tried.
