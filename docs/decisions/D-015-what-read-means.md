# D-015: What "read" means, across the feed, conversations and devices

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-003, D-009
- Evidence: R-002 for seen versus read in feed libraries; the rest of "Best practice" is the agent's general knowledge of the named products and was not researched for this decision
- Visual: none

## Why this needs deciding now

D-003 made reading the way a row leaves the feed, and defined it for the feed alone: a row in Focus or Needs you is read when opened, and viewing For your information is enough. The user pointed out that this does not say when a message becomes read in general: in the conversation itself, on another device, when acting on it, or when it arrives while the person is looking. Because read removes rows, each of these has to have an answer.

## Options

### A. Read when opened (inboxes: email, GitHub, Linear)

- Upside: precise. Nothing is marked read that the person did not choose to open.
- Risk: effort per item.

### B. Read when it has been on screen (channels in Slack, Discord, Teams)

- Upside: effortless.
- Risk: false reads. In our feed a false read makes a row leave.

### C. Open for the feed's top buckets, on-screen elsewhere, over one shared read position

A for Focus and Needs you; B inside a conversation and for For your information; one "read up to here" position per conversation that every surface shares.

- Upside: precise where a mistake costs most, effortless where it costs little, and no surface disagrees with another.
- Risk: "on screen" needs a strict meaning, or things are marked read that nobody saw.

## Best practice

Chat products keep a read position per conversation, not a flag per message, and drive the sidebar, badges and notifications from it on every device. They separate seen (the list was opened, the badge clears) from read (the item was opened). They mark nothing read in a background tab or an unfocused window. All of them offer "mark as unread".

Where this decision differs: no "mark as unread". Keep (D-003) does that job and says what is meant.

## Recommendation

**C**, with the seven rules below.

## Decision

Accepted by the user on 2026-11-22 ("let's do as you say"): **C.**

1. **Opening stays the rule for Focus and Needs you.** A row is read when it is expanded in the feed or its conversation is opened. Read rows stay in place until the person leaves the feed and remain in Earlier (D-003), so a mistake can be undone.
2. **Read is a position per conversation.** A row is read when the person's position reaches its newest item. Expanding a row shows all its new items and moves the position to the end.
3. **One read state, shared.** Reading a thread in the channel marks its feed row read; reading the row in the feed clears the channel's unread mark. The same on every device.
4. **Acting implies read:** reply, react, Keep and Done.
5. **Only while the person is looking.** Nothing becomes read in a background tab or an unfocused window.
6. **Arrivals in a conversation the person is watching are read at once.** If something arrives while that conversation is open in front of them, with its newest message visible, it goes straight to Earlier: no feed row and no notification.
7. **No separate "mark as unread".** Keep is the way to say "I still need to deal with this".

Unchanged from D-003: in For your information, viewing the bucket is enough.

## Consequences

- Easier: the feed, the unread marks on channels (D-009) and notifications cannot disagree, because they read the same position.
- Harder: rule 6 depends on knowing that the newest message is really visible in a focused window; a loose test marks things read that nobody saw. Read position has to reach every open screen of the same person quickly.
- Committed to: a read position per person and conversation as the single source of read state; no mark-as-unread.

## Follow-ups

- D-012 (what sends a notification) builds on rules 3 and 6.
