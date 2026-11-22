# D-014: A reaction from someone you addressed is an answer; feed rows show reactions

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-003, D-011
- Evidence: E-001 (how reactions behave in the model); no research on reaction use was done, the use cases below are from the agent's general knowledge of team chat
- Visual: none

## Why this needs deciding now

The user raised it: reactions are a large part of how teams collaborate, and the accepted rules treat every reaction as applause. A reaction to your message lands in For your information as "3 reacted", whatever it was and whoever it came from. Since D-011 a flag has no status of its own, so a reaction is also the only quick way a team can show "I have this".

## Options

How reactions are used, against the rules as they stood:

| Use | Example | Fits? |
|---|---|---|
| Applause | 🎉 on a launch announcement | Yes: For your information, grouped |
| Answering a request | You flag Marco "need your sign-off"; he reacts ✅ | No: his answer lands at the bottom as "1 reacted", unnamed |
| Acknowledging | 👍 or 👀 on a question in a direct conversation | No: the same |
| Voting | "✅ for option A, ❌ for B" | Partly: "5 reacted", not the split |
| Claiming work | An agent asks "who can take this?"; Lena reacts 👀 | No: everyone else still has the row and cannot see it is taken without opening it |
| Agent status | An agent reacts 👀 when it starts, ✅ when done | Yes |
| Your own quick reply | You react 👍 instead of writing "ok" | Yes: it releases your row |

### A. Leave reactions as they are

- Upside: nothing to build.
- Risk: answers given by reaction are buried, and a team cannot see at a glance that something is taken.

### B. Give specific emoji a meaning (✅ means done, 👀 means taken)

- Upside: the product could act on them.
- Risk: teams differ, and it is a hidden rule. It also brings back a shared status, which D-011 removed.

### C. Three changes that do not interpret emoji

1. A reaction from someone you addressed in that message is an answer: it lands in Needs you and names the person.
2. A feed row shows the reactions already on its message.
3. The grouped row shows the split by emoji.

- Upside: a plain, explainable rule based on who reacted, not on what the emoji means.
- Risk: change 1 adds rows to Needs you, the longest bucket. To be measured in the model.

## Best practice

Products show reactions on the message and list them in the activity feed without ranking them (Slack's Reactions filter, Teams' Reactions filter; R-002). None was found to treat a reaction as an answer. C goes beyond established practice on change 1; the reason is that our feed sorts by whether something is directed at you, and a reaction from the person you asked is.

## Recommendation

**C.**

## Decision

Accepted by the user on 2026-11-22 ("overall agree"): **C.**

1. **A reaction from someone you addressed is an answer.** If your message mentions a person, is flagged for them, or was sent to them in a direct conversation, their reaction to it lands in Needs you, like a reply in your thread, and says who and what: "Marco reacted ✅ to your message". Reactions from anyone else stay grouped in For your information.
2. **Feed rows show the reactions on their message**: "👀 Lena". No bucket changes. This is how a team shows that something is taken, now that a flag has no status.
3. **The grouped row shows the split**: "✅ 4 · ❌ 1 on your message".

Unchanged: reactions never land in Focus; your own reaction counts as acting, marks the row read and releases a row kept for you (D-003, D-011); no emoji has a built-in meaning.

## Consequences

- Easier: asking by flag or mention and answering by reaction works end to end, for people and for agents waiting on approval.
- Harder: a reaction is now sorted by who made it and whom the message addressed, so the feed needs both for every reaction. Rows have to stay current as reactions come and go, without jumping.
- Committed to: the three changes above.

## Follow-ups

- Measure in E-001 how many rows change 1 adds to Needs you.
