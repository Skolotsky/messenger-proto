# E-001: Do the accepted feed rules hold together, and does Focus stay short in a busy team?

_Three runs on 2026-11-22. The first found a hole and a list of unstated cases, which became D-011 and D-013. The second confirmed the hole closed. The third added D-012, D-014, D-015 and D-016. `results.txt` and `results.json` hold the third run; this note reports it and keeps the first run's key numbers as the evidence for those decisions._

- Date: 2026-11-22
- Time box: one working session
- Feeds: the product decisions D-001, D-003, D-004 and D-011 to D-016 (are they complete and free of contradictions?); its findings became D-011, D-012 and D-013
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

Third run, 2026-11-22, Python 3.12, `./experiments/E-001-feed-rules-model/run.sh`, about a minute. Two runs give identical output.

### Do the rules hold together?

- **123 scripted cases** behave as the decision records say.
- **No invariant broken** in five variants and five further seeds, about 130,000 events. Checked: one item in one place; your own messages never; Info never above For your information; streaming text never; a deleted message never shown live; what names you always arrives; a row kept by a flag only while that flag exists and names you; reactions never put a row in Focus; an answer by reaction always in Needs you; no marker for something never seen; no notification for a For your information item or an Info message; every delete matches the D-004 table; a text-only edit moves nothing; someone else's message never releases a row a flag kept for you; a dial or rule change never moves a row already read.
- **The invariants are weaker than that list suggests.** The second independent check changed the model in seven wrong ways in a copy (for example "Info notifies", "another person's reply clears your flag"), and the simulation's invariants caught none; the scripted cases caught five. Four invariants were added afterwards and not tested the same way. Treat the scripted cases as the stronger evidence.
- **The hole found in the first run is closed** (D-011). Then: a flag nobody answered never left, and the Needs you list a person met grew from 3.3 rows to 7.2 over ten days. Now it is 3.5 (days 1-2), 3.9 (days 5-6), 4.4 (days 9-10); across seeds its mean is 3.7 to 3.9 and nine visits in ten show 8 rows or fewer. The slight rise in this seed is within what other seeds show.

### Does Focus stay short?

Base run: 13,442 messages in ten working days, 35% written by agents.

| Per person | Focus | Needs you | For your information |
|---|---|---|---|
| Rows that first arrive there, per day | 4.4 | 19.8 | 14.6 |
| of which from agents | 0.2 | 2.2 | 2.9 |
| Waiting at a visit, mean | 0.5 new, 1.1 kept | 3.9 | 2.2 |
| Waiting at a visit, 9 visits in 10 at most | 1 new, 3 kept | 9 | 5 |

- A single list would get 38.8 rows a day; Focus gets about one in nine (4.2 to 4.5 against 37.3 to 38.4 across seeds).
- Kept in Focus, by the person or by a flag: 1.1 rows on average, 7 at most. This assumes a person presses Done within a day or two on a flag they will not answer.
- The on-call support person with two channels in Focus meets about 3 new Focus rows at a visit, because a burst of alerts is one row.
- Without Info (agents post progress as ordinary messages), rows arriving in Needs you from agents roughly double.
- With "any direct mention" as a starting Focus rule, Focus gets about 7 rows a day.

### Notifications (D-012)

Per person per day, counting one notification when a row becomes unread in a notifying bucket and one more if it moves up into Focus:

| Policy | Base | Across seeds | Half traffic | Double traffic |
|---|---|---|---|---|
| Everything | 41.2 | | 20.7 | 77.8 |
| Focus and Needs you (accepted default) | 29.3 | 27.6 to 29.0 | 14.6 | 54.3 |
| Focus only (the personal switch) | 5.3 | 5.0 to 5.2 | 2.7 | 10.0 |

**The default gives about 29 a day, not the "about 21" quoted when D-012 was put to the user.** That estimate counted rows by the bucket of their first item. A row that first arrives in For your information and later receives a reply still notifies, and a row the person has read notifies again on its next reply. With mentions starting in Focus, "Focus only" rises to 9.1.

### Reactions as answers (D-014)

- Answers by reaction are 13% of the items landing in Needs you: 10% in direct conversations and 3% in channels. In a direct conversation every reaction from the other person is an answer by the rule, and the simulation lets people react to direct messages as often as to channel messages, so the 10% is probably high.
- Why items land in Needs you: replies in threads the person started 44%, replies in threads they only replied in 30%, answers by reaction 13%, flags for everyone 9%, a mention and nothing else 4%. Most mentions come inside threads the person is already in.

### Read at once (D-015, rule 6)

460 arrivals were read at once because the person had written in that conversation within the last three minutes. That is a rough stand-in for "has it open in front of them" and says little about how often the rule will apply.

### Still open after this run

Behaviours the model has to choose that no decision states. In the base run, most frequent first:

- R38 (149): new activity on a row that sits in Focus because it is kept notifies by the bucket the new item earns, not as Focus.
- R42 (56): a reply releases everything kept on that row; a reaction releases the flag on that message and the person's own Keep.
- R43 (31): a reaction to a flag that names nobody is ordinary applause for its author, not an answer.
- R35 (9): an answer by reaction reaches you from a muted channel; ordinary reactions from a muted channel do not.
- R41 (5): in a channel put in Focus, replies in a thread you were only mentioned in are lifted to Focus too.
- R44 (2): when a mention of you is edited away or deleted, the thread stops following you.
- R34, R37, R40 did not come up in the base run: an agent's reaction stays in For your information; a row kept by a flag sits in Focus even without the "flags" starting rule; a row lifted into Focus by your own rule change does not notify.
- D-016's Threads tab is modelled as "threads whose dial is not Off", which is the agent's reading recorded in D-016, to be confirmed.

Not modelled: the notification tray's contents, search, a read position inside a row (a row is read or not), showing reactions on a row, and the phone-sized screen.

### Fourth run: Focus people and "only Done releases" (T-039)

The model now has D-021 (a list of people that fills itself, removal, Add back, the option off) and D-023 point 26 (only Done releases).

- 140 scripted cases pass, 17 of them new. Four injected faults in the new rules were each caught by them.
- No invariant broke in the five variants and five further seeds.
- **The notification figures stand:** about 29 a day by default (27.6 to 29.0 across seeds), about 5 with Focus only (5.0 to 5.2).
- Not measured: how long Kept grows with "only Done releases" on. The user is content with the Kept design for this stage (2026-11-22), so the measurement was dropped. The setting is off at the start.

### Independent checks

Two fresh verifier agents, each given only the decision records and the code, probed the model with their own scripts.

- The first (after run one) found six places where the model contradicted the decisions, eleven behaviours decided without being marked, six "inventions" the decisions did state, weak invariants, and two skews in the simulation. All corrected before run one's numbers were taken.
- The second (after the third version) found six more contradictions (removing Info was not "as if new"; Keep did not mark read; a flagged direct message that mentions anyone was not kept; a reply inside a direct conversation reached nobody; a finished stream ignored replies made while it streamed; later reactions ignored "watching"), an undercount of the "everything" notification policy, a misleading breakdown of Needs you, eleven more unmarked behaviours, and the weakness of the invariants described above. All corrected or listed above before these numbers were taken.
- The corrections made after the second check have not been checked by a third.

## Conclusion

**The rules hold together as one procedure, and Focus stays short under the assumed traffic.** Two rounds of independent checking each found real faults in the model, which is a reason to trust the decisions more than the model: every fault was the model departing from a decision, not two decisions contradicting each other. The one contradiction-level problem in the decisions themselves, the flag that could never leave, was found in run one and is closed.

What the user should know before building:

1. The notification default is about 29 a day in this team, not 21. "Focus only" is about 5.
2. Needs you is the long bucket and thread replies are three quarters of it.
3. Nine small behaviours are still the model's choice (listed above).

What this does not show:
- The traffic rates are assumptions. Variants share a seed and rates but not identical traffic, because what people read changes who replies; differences inside the spread across seeds are noise. Agents wrote 35% of messages here, below the 50% Salesforce reports for Slack.
- People in the simulation read, reply and react at fixed probabilities.
- Text rules judged by a model are not in it.
- Nothing about performance, storage or delivery: this is a model of rules, not of the system.
- A behaviour that the modelling agent and both verifiers misread the same way would not have been caught.
