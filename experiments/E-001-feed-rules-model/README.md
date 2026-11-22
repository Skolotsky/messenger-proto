# E-001: Do the accepted feed rules hold together, and does Focus stay short in a busy team?

- Date: 2026-11-22
- Time box: one working session
- Feeds: the product decisions D-001, D-003, D-004 (are they complete and free of contradictions?); its findings became D-011, D-012 and D-013
- Visual: `docs/visuals/E-001-feed-rules-model.html`

## Hypothesis

1. The rules in D-001, D-003 and D-004 can be written down as one procedure that gives every person exactly one answer for every message, without the procedure having to invent behaviour the decisions do not state.
2. In a team of about 30 people and 6 agents (D-010), a person's Focus bucket stays an order of magnitude shorter than a single Slack-style list of the same activity, and agent traffic stays out of the top two buckets unless an agent is waiting on that person.

We expect 1 to be partly false: writing rules as code is how unstated cases surface. The point is to find them now.

## Method

A model of the rules in plain Python (standard library only), with no storage, network or interface. It is not a draft of the product and says nothing about the stack.

- `model.py`: the rules. People, agents, channels, direct conversations, threads, messages with mentions and a level (Flag, Info), reactions, edits, deletes; per person: Focus rules, channel dial, thread dial, feed rows, read, Keep and Done, the Earlier list, and a feed that does not move while it is viewed. Wherever the code had to choose a behaviour the decisions do not state, it calls `ruling(...)`, so every invented behaviour is named and counted.
- `scenarios.py`: the situations listed in D-010, plus edge cases, played step by step with the expected outcome asserted from the decision records.
- `simulate.py`: ten working days of a 30-person, 6-agent, 20-channel team with randomised but seeded traffic, checking invariants every 500 events and at the end, and checking every delete and every text-only edit against the D-004 table as it happens and measuring what each person's buckets hold at each visit. Variants change one rule at a time.

One command reruns everything: `./experiments/E-001-feed-rules-model/run.sh`

The traffic numbers (messages per person, how often people mention, flag, edit, delete, react) are assumptions, stated at the top of `simulate.py`. Results that depend on them are reported as comparisons between variants under the same traffic, not as predictions.

## What would change our mind

- A case where two accepted rules give different answers, or none does, for something that happens often in the simulated team: that rule needs a decision before building.
- Focus regularly holding more than a screenful (say more than 10 rows at a visit) under the starting rules: the starting rules or the dials need rethinking.
- Rows that can never leave a bucket: D-003 has a hole.
- An invariant from the decisions that cannot be kept (for example "what names you always gets through" against "Info never lands above For your information").

## Result

Run on 2026-11-22, Python 3.12, `./experiments/E-001-feed-rules-model/run.sh`, about 40 seconds. Raw output: `results.txt`, `results.json`. Two runs give identical output.

### Do the rules hold together?

- **92 scripted cases** behave as the decision records say (`scenarios.py`).
- **No invariant broken** in six variants and five further seeds, about 150,000 events in all: one item in one place; your own messages never; Info never above For your information; streaming text never; a deleted message never shown live; what names you always gets through; an open flag that names you always in Focus; an open flag for everyone present for everyone in scope; reactions never above For your information; no marker for something never seen; every delete and every text-only edit matches the D-004 table. One invariant was seen failing during development (an edit turning a read message into Info left it above For your information), which was a fault in the model, since fixed.
- **One hole: a flag nobody answers never leaves.** D-003 gives a flagged row one way out, the flag closing. In the base run 19 flags that named nobody and 18 that named a person were never answered in ten days. The first kind sits in Needs you for everyone in scope: the average person had 4.4 such rows by day ten (2.5 to 4.8 across seeds), and the Needs you list a person meets grows from 3.3 rows (days 1-2) to 7.2 (days 9-10). With the single change "a flag that names nobody leaves once read" it stays flat, 2.9 to 3.3. The second kind sits in Focus under "waiting on you": one person reached 17 (10 to 22 across seeds). The person has no way to say "not mine".
- **Behaviours the decisions do not state**, which the model had to invent. Each is named in `model.py` (`RULING_TEXT`) and counted. In the base run, most frequent first: where a read, still-open flag sits in Focus (R17, 560 times); whether any reply in the thread answers a flag (R07, 248); reactions to your message when the thread is off or the channel muted (R18, 172); new activity on a kept row (R16, 160); what "All activity" does with items addressed to you (R26, 91); whether deleting your own message removes you from its thread (R27, 42). Rarer ones: R03, R04, R05, R06, R08, R09, R10, R12, R13, R19, R22 to R25, R28 to R33.
- **One stated rule worth confirming.** D-001 says a muted channel lets through "only what names you". As written, that silences replies in threads you started or replied in there. It came up 306 times in the base run (N01).
- **Not modelled because nothing is decided:** which feed items send a notification, and where a sender sees the state of a flag they sent.

### Does Focus stay short?

Base run: 13,466 messages in ten working days, 35% written by agents.

| Per person | Focus | Needs you | For your information | One single list |
|---|---|---|---|---|
| New rows per day | 4.4 | 15.5 | 17.8 | 37.7 |
| of which from agents | 0.2 | 2.0 | 2.9 | |
| Waiting at a visit, mean | 0.5 new, 0.7 waiting, 1.0 kept | 5.5 | 3.0 | |
| Waiting at a visit, 9 visits in 10 at most | 2 new | 14 | 7 | |

- Focus receives about one row in nine of what a single list would hold (4.3 to 4.6 against 36.9 to 38.5 across seeds).
- The on-call support person with two channels in Focus gets 17.5 Focus rows a day and meets 2.7 new ones at a visit (10 at most), because a burst of 15 alerts is one row.
- Agents write a third of the messages and cause 0.2 of the 4.4 new Focus rows a day. Without Info (agents post progress as ordinary messages) agent items in Needs you go from 1,074 to 4,738 over the run, and new Needs you rows per person per day from 15.5 to 18.6.
- With "any direct mention" as a starting Focus rule (the user's first proposal) Focus receives 7.1 rows a day instead of 4.4, and a person meets 1.1 new rows instead of 0.5.
- Needs you is the long bucket, not Focus. 43% of its items are replies in threads the person started, 36% replies in threads they only replied in, 15% mentions, 7% flags for everyone. At double the traffic a person meets 10 rows on average and 24 on a bad visit.

### Independent check

A fresh verifier agent, given only the decision records and the code, probed the model with its own scripts. It found six places where the model contradicted the decision text (a withdrawn flag on a message that still mentions you; an answered flag for everyone deleting a post that belonged in the feed anyway; a text-only edit re-delivering or moving a row; replying in a turned-off thread inside a muted channel; a mention inside a Focus channel landing in Needs you; replying not releasing a kept channel row), eleven behaviours decided without being marked, six "inventions" that the decisions do in fact state, weak invariants, and two skews in the simulation (one random stream shared by traffic and behaviour; replies to mentions only generated from Needs you). All were corrected before the numbers above were taken. The corrections were not re-verified by a second independent pass.

## Conclusion

**The foundation holds, with one hole to close and a short list to confirm.** The three buckets, the dials, Flag and Info, Keep and Done, and the edit and delete rules can be written as one procedure that gives every person one answer for every message, and its promises survive a simulated busy team. Hypothesis 2 holds under the assumed traffic: Focus stays short and agents stay out of it unless they are waiting on someone.

Hypothesis 1 was partly false, as expected. What needs the user before building:

1. A way out for a flag nobody answers (the hole).
2. Whether a muted channel should silence replies in your own threads.
3. Where a read, still-open flag sits in Focus, and whether it is counted.
4. What "All activity" does with items addressed to you (the record's sentence reads two ways).
5. Whether changing a dial or a rule re-sorts rows already in the feed.
6. Whether a reaction from anyone closes a flag for everyone.
7. What sends a notification (not modelled; needed before the delivery guarantees).

The rest of the invented behaviours are small and are listed for confirmation in one batch.

What this does not show:
- The traffic rates are assumptions. The comparisons between variants are sound; the absolute numbers are not predictions. Agents wrote 35% of messages here, below the 50% Salesforce reports for Slack.
- People in the simulation read and answer at fixed probabilities. Real people do not.
- Text rules judged by a model are not in it.
- Nothing about performance, storage or delivery: this is a model of rules, not of the system.
- The model was written and corrected by one agent and checked once by another. A behaviour both misread from the records would not have been caught.
