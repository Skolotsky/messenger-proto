# D-013: The feed rules E-001 found unstated or ambiguous

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-003, D-011
- Evidence: E-001
- Visual: `docs/visuals/E-001-feed-rules-model.html`

## Why this needs deciding now

Writing the accepted rules as a runnable model (E-001) surfaced cases the decisions leave open, state in two ways, or state in a way that surprises. Each had to be given some behaviour for the model to run. The frequent ones need the user's call before anything is built on them.

## Options

Each point was put to the user with the behaviour the record implies, the alternative, how often it came up in the simulated team, and a recommendation (the table in the visual). They are small and independent, so they are recorded together.

## Best practice

Close ambiguities in rules before building on them, and record the reading that was chosen next to the rule.

## Recommendation

As listed under Decision; the agent's recommendation was accepted on each point.

## Decision

Accepted by the user on 2026-11-22.

1. **A muted channel does not silence your own threads.** Replies in threads you started or replied in there still reach you, in Needs you. D-001's Muted row ("only what names you") is narrowed by this. It came up 306 times in the simulated ten days. The user answered "yes" to a line that held both the question and the recommendation; this records the recommendation, and is the first thing to revisit if that reading was wrong.
2. **At "All activity", what is addressed to you lands as at Default.** The level only adds the channel's other top-level posts, in For your information. D-001's sentence ("the same items, with the unaddressed ones in For your information") could also be read as lifting addressed items to Focus; it does not.
3. **Changing a dial or a Focus rule re-sorts what is already in the feed.** Unread rows move, or leave, the next time the feed is opened. Read rows stay where they are.
4. **No separate "waiting" section in Focus, and a reaction closes nothing for other people.** Both are settled by D-011.

Assumed by the agent in the model and shown to the user for objection, none raised so far. They stand until someone objects:

- A flag with only a whole-channel mention counts as naming nobody. A flag that mentions only an agent reaches no person.
- A direct message from an agent lands in Needs you, since the Focus starting rule says "from people".
- A Focus rule for a person lifts what already reaches you from them; it does not pull in their other messages.
- If you remove a starting Focus rule, those items land in Needs you.
- New activity on a kept row shows inside the Kept section.
- Reactions to your message follow the thread: none when the thread is Off or the channel Muted.
- Deleting your own message does not remove you from its thread.
- Replying in a thread you set to For your information by hand leaves it there.
- A whole-channel mention inside a thread reply reaches nobody extra.
- An edit that removes Info delivers the message as if new; one that lowers a message you have not read moves it down quietly.
- Reading a conversation in the channel itself marks its feed row read. Mark all read leaves kept rows in place.
- A person who is not a member of a channel gets nothing from it except what names them.

### Confirmed by the user, 2026-11-22 (added after acceptance; the decision is unchanged)

The reading in point 1 (a muted channel does not silence your own threads) and the list of smaller assumed behaviours were shown to the user again and confirmed ("ok").

## Consequences

- Easier: the rules have one reading. The model and the decisions agree.
- Harder: point 3 means a rule or dial change has to be applied to existing unread rows, not only to what arrives next.
- Committed to: the readings above.

## Follow-ups

- D-012 (what sends a notification) is the remaining product decision E-001 raised.
- Worth watching in early access, not a decision now: Needs you is the long bucket, and about a third of its items are replies in threads the person only replied in once.
