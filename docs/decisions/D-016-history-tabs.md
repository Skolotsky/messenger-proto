# D-016: Besides the feed, history tabs by type for finding things again

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-003, D-009
- Evidence: R-002 (Slack's Activity and Teams' feed filter by type: mentions, threads, reactions)
- Visual: none; the layout is a question for the look-and-feel experiment (E-002)

## Why this needs deciding now

The feed answers "what should I look at now" and lets rows leave once read. It does not answer "someone mentioned me yesterday, where was that?". The user raised this before the look-and-feel work, which has to give these views a place.

## Options

### A. The Earlier list only

Read rows collect in one Earlier list (D-003).

- Upside: nothing more to build.
- Risk: one undivided list of everything ever read is hard to search by eye.

### B. Separate tabs by type, apart from the feed (the user's proposal)

Direct messages, Mentions, Threads, Reactions.

- Upside: this is the part of Slack's Activity people already know how to use, kept for what it is good at: looking things up.
- Risk: four more views. If they look like buckets, people will not know which place is "the" feed.

## Best practice

Slack's Activity and Teams' feed are, in effect, these tabs (R-002). We moved the "what needs me" job to the three buckets; the lookup job still needs a home.

## Recommendation

**B**, as a lookup that is visibly not the feed.

## Decision

Accepted by the user on 2026-11-22: **B.** "Apart from the main activity feed there should be separate tabs: dms / mentions / threads / reactions. They are useful for discovery like 'someone mentioned me yesterday'. So it is not activity feed."

- Four tabs: **Direct messages, Mentions, Threads, Reactions.**
- They are history: newest first, read and unread together, and nothing leaves them by being read. No Keep-or-Done work happens there, though a row can be kept from a tab.
- They are not sorted by the buckets, dials or Focus rules. A mention of you is in Mentions whatever channel it came from.
- They show the current truth (D-004): a deleted message or a removed mention is not there.

The agent's reading, to confirm when the mock-ups are reviewed:
- The tabs take over the job of the single Earlier list from D-003. "Earlier" becomes "look in the tab for its type", and a row that has left a bucket is found there.
- Threads lists threads whose dial is not Off for the person. Reactions lists reactions to the person's messages, including the answers from D-014.

### Confirmed by the user, 2026-11-22 (added after acceptance; the decision is unchanged)

The agent's reading is confirmed ("Yes, replace it"): the history tabs replace the single Earlier list of D-003. A read row is found in the tab for its type. Two kinds of row have no tab and are found in their channel: a flag that names nobody, and posts from a channel set to Focus or All activity. Undoing an accidental read still works as in D-015: a read row stays in place, dimmed, until the person leaves the feed.

## Consequences

- Easier: the feed can be strict about letting rows go, because nothing is lost by it.
- Harder: four more views to keep true under edits and deletes, at two widths (D-009).
- Committed to: the four tabs as part of the first version, separate from the feed.

## Follow-ups

- E-002 decides where the tabs sit relative to the three buckets, on desktop and on a phone.
