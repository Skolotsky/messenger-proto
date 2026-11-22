# E-008: Two small simulations behind the scaling decisions

- Date: 2026-11-22
- Time box: half an hour
- Feeds: D-029 (corrections), D-030 (how one organization grows)
- Visual: `docs/visuals/D-029-scaling-by-adding.html`

## Hypothesis

A. The feed's result for one conversation depends only on that conversation's changes, in their order, and on reader-wide changes at their place. If so, the conversations of one organization can be applied side by side, which is what lets a large organization use more than one worker.

B. When many organizations have a backlog at once and workers serve "whoever has waited longest", fresh traffic waits a full round behind the catch-up. Taking organizations that one turn will clear first, with some of the backlog every round, should keep fresh traffic fast without starving the backlog.

## Method

Pure Python, no database and no processes; about half a minute. `python3 experiments/E-008-one-large-organization/sim.py`

A. The feed rules of E-003 (`spine.fold`) are replayed over 20 generated logs of 1,500 changes (12 people, about 38 conversations). Each log is replayed in 10 other orders that keep the order inside every conversation (a channel with its threads, or a direct conversation) and leave reader-wide changes in place. The result is compared with the original order. As a control the reader-wide changes are shuffled too.

B. A queue model: workers serve organizations in turns of at most 40 changes at 1 ms per change; 200 organizations start with 100 changes waiting; 2,000 fresh single changes arrive over 10 s. Two rules for choosing the next organization are compared. It is a model of the rule, not a measurement of the product.

## What would change our mind

- Any interleaving that changes the feed: conversations cannot be applied side by side, and a large organization needs a different design.
- No gain from the two-line rule in the model: keep the simpler rule.

## Result

Raw output: `results.txt`.

A. With reader-wide changes kept in place: **0 of 200** interleavings gave a different feed. With them shuffled as well: 200 of 200 differed.

B. A fresh change waits:

| Workers | Whoever has waited longest | Fresh first, some backlog every round |
|---|---|---|
| 2 (too few for the load) | 3.9 s median, 4.3 s for 19 in 20 | 1.8 s median, 2.6 s for 19 in 20 |
| 4 | 166 ms median, 2.1 s for 19 in 20 | 2 ms median, 30 ms for 19 in 20 |
| 8 | 1 ms median, 1.0 s for 19 in 20 | 1 ms median, 29 ms for 19 in 20 |

The backlog is finished at the same time under both rules (10 to 11 s).

## Conclusion

A. **Holds for the rules modelled**: the order between conversations does not matter, the place of reader-wide changes does. In this model "which conversation a person is watching" is one slot per person and therefore reader-wide; in the product, watching starts and ends per conversation and belongs to that conversation.

B. **The two-line rule is worth having**: with enough workers, fresh traffic is not held up by a wave of catching up, and the wave is not slowed.

What this does not show: the full feed rules (E-001's, with Focus rules and Focus people, which are reader-wide and would act as barriers); real costs; anything about a database.
