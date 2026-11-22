# D-011: A flag is a marker for each reader's feed, with no state of its own

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-003, D-004
- Supersedes in part: D-001 (when a flag closes), D-003 (how a flagged row leaves), D-004 (the marker for a flagged row)
- Evidence: E-001
- Visual: `docs/visuals/E-001-feed-rules-model.html` (the finding that led here)

## Why this needs deciding now

E-001 found a hole in the accepted rules: a flagged row could only leave when the flag "closed", so a flag nobody answered stayed in people's feeds for good. In the simulated team the Needs you list a person met grew from 3.3 rows to 7.2 in ten days for that reason alone, and one person had 17 unanswered flags in Focus. The person had no way to say "not mine". The same experiment asked where a read but unanswered flag sits in Focus, and whether one person's reaction closes a flag for everyone.

## Options

### A. Keep a shared state and add ways out

The flag stays open or closed for everyone, as in D-001. Add: a flag that names nobody leaves once read; a flag that names you can be marked Done by you, and the sender sees that; a "Waiting on you" section in Focus. This was the agent's recommendation.

- Upside: the sender can see who has dealt with it.
- Risk: three additions on top of an entity that already has a lifecycle: who it is open for, what closes it, for whom.

### B. No shared state: a flag only changes how the message behaves in each reader's feed (the user's proposal)

A flag is not open or closed. For each reader it does one thing, and each reader finishes with it on their own.

- Upside: nothing new to learn. A flag that names you is the same thing as a row you kept, only kept for you by the sender. The hole and both open questions disappear.
- Risk: the sender cannot see whether anyone has dealt with it, and one person's answer does not clear it from anyone else's feed.

## Best practice

Prefer the model with fewer states when both meet the need. Shared, multi-party state (open for whom, closed by whom) is where chat products accumulate edge cases, and E-001 showed ours already had them before anything was built.

## Recommendation

The agent recommended A and, after the user's proposal, recommends **B**: it removes the cause of the hole and does not add a way round it.

## Decision

Accepted by the user on 2026-11-22: **B.** "Everyone has to close it for themselves, even other's reply can't change its state for you. So flag doesn't have a state overall. It's just a marker for your feed behaviour." And on where a read, unanswered flag sits: "we can treat it as 'to be handled' automatically without the person having chosen to, so we don't invent a new entity."

| A flagged message… | Lands in | Stays until |
|---|---|---|
| names you, or is a direct message to you | Focus, kept for you automatically | you press Done, or you reply or react to it |
| names nobody | Needs you, for everyone in the thread or channel | you read it, like any other row |

- **Kept by a flag** looks and behaves like a row you kept yourself (D-003): it sits in the Kept section of Focus once read, with the reason "Flagged by Anna" in place of "Kept by you". There is no separate section for it.
- **A flag that names nobody is not kept automatically.** One flagged post in a 30-person channel would otherwise sit in 30 people's Focus until each pressed Done. Anyone who wants to hold on to it can keep it.
- **Someone else's answer changes nothing for you.** Nor does their reaction.
- **The sender sees no status.** There is no "answered by" or "still waiting on". The sender sees replies, as with any message.
- **If the flag is removed, what it kept is released** (the user's correction to the agent's first version). A row that was kept only because of the flag stops being kept: if you had read it, it goes to Earlier; if not, it is an ordinary row in whichever bucket the message now earns. A row you also kept yourself stays kept. Deleting the message removes its flag too, so the same holds, with a marker in place of the text where D-004 gives one.
- Agents are unaffected: an agent that stopped for a person waits for that person's reply, as for any message.

What this replaces:
- D-001: the "Closes when" column of the flag table, "the sender can withdraw a flag at any time" as a change of state, and "'everyone in the thread', for a flag that mentions nobody" as the set of people it closes for. Where a flag lands is unchanged.
- D-003: the table row "is flagged for you: leaves when the flag closes", and "for a flag, that reply or reaction is also what closes it".
- D-004: "Kept, or flagged for the person: the marker stays in Focus until the person marks it done" now holds only for rows the person kept themselves.

## Consequences

- Easier: one idea, kept, with two sources. Nothing about a flag has to be agreed between people or delivered to several feeds at once: each feed can settle it alone.
- Harder: a sender who needs to know that something was handled has to read the replies. A flag for everyone that someone already answered still shows for the others until they read it.
- Committed to: no status on a flag anywhere in the product; a row records whether it is kept by its reader, by a flag, or both.

## Follow-ups

- E-001's model is updated to this decision and rerun.
- D-013 records the other points E-001 raised.
