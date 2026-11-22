# D-023: How the feed is used day to day in the chosen layout

- Status: accepted (the points under "Decided"); proposed (the points under "Still the agent's proposal")
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-003, D-015, D-019, D-020, D-022
- Supersedes in part: D-003 (expanding a row in place, and replying and reacting from the row, now hold on a wide screen only)
- Evidence: E-002 (all of it is shown in `experiments/E-002-look-and-feel/slack-like.html` and covered by `check.py`)
- Visual: `experiments/E-002-look-and-feel/screenshots/slack-like-*-18` to `-26`

## Why this needs deciding now

Using the Slack-like mock-up, the user corrected a run of interaction details one by one. They are small and belong together, so they are recorded together, as D-013 did for the feed rules. Each is the user's own statement.

## Options

Each point was tried in the mock-up and changed until the user was content with it; the earlier forms are in the experiment's git history.

## Best practice

Reuse the habits of the messenger people know (D-019), keep one way to do each thing, and on a phone prefer fewer states over more controls.

## Recommendation

As listed under "Decided"; where the agent had built something else first, the record says so.

## Decision

### Decided by the user, 2026-11-22

1. **Reactions are not limited to the latest message.** "I see messages and I want to be able to put reaction on any, not just on last." Every message in an opened row and in a conversation takes a reaction; the quick reactions stay as a shortcut for the latest.
2. **A full emoji picker, not only a few quick ones**, with search.
3. **A row takes a reaction without being opened.** "Even before I expanded item from the feed I should be able to react."
4. **Reactions and buttons sit on different lines** in an opened row, "to not mix them up".
5. **Opening a row makes it ready to answer:** its reply box is on screen and has the cursor.
6. **Sending a reply from a row closes the row.** "If user would want to stay in conversation they would go to the conversation." The closed row shows the reply and "Replied".
7. **One click from a row to the conversation**, at the first new message.
8. **Arrivals wait behind a visible control with a button**, placed in the free space beside the "Activity" title, not floating over the list. "No content layout shifts without user interactions." The agent first built a bare "3 new" pill over the list.
9. **On a phone a tap on a row opens the conversation. Nothing expands.** "Mobile needs more simple UX." This supersedes D-003's expanding in place, and replying and reacting from the row, for the phone-sized layout; they stay on a wide screen.
10. **All settings live in one settings screen**, reached from the person's own menu and opened on the section the person came from. "I don't want too big settings fragmentation. I expect all settings to be discoverable from overall user settings." The agent first built a separate feed panel.
11. **The sidebar holds the person's own avatar and menu**, "for typical things like logout, settings".
12. **Activity can be closed on a wide screen**: a close button on it, and clicking "Activity" in the sidebar opens and closes it. Its count stays on the sidebar while it is closed. Closing it counts as leaving the feed (D-003).

### Still the agent's proposal, shown in the mock-up, awaiting the user's word

- **Context in an opened row:** the message before the new ones in a direct conversation; the start of the thread and the reply before in a thread; none for a mention in a channel.
- **A long burst** shows its six newest messages by the reply box, with the earlier new ones one click above; opening reads the whole row (D-015). The number six is the agent's.
- **The arrivals button is called "Show"**, and the control names how many arrived in Focus and Needs you.
- **Swipe on a phone:** to the right marks the row read (Done on a kept row); to the left uncovers React and Keep. The row stays in place either way.
- **On a phone the reply box is not given the cursor automatically**, since that opens the keyboard.
- **The settings sections** are Account, Activity feed, Notifications, Channels. Sign out and switching person are named in the menu but not built: the first version has a person switcher in place of sign-in (D-009).

## Consequences

- Easier: one way to each thing, and a phone layout with two states, list and conversation.
- Harder: two interaction models for a row, by width. The settings screen becomes a surface of its own that every later setting must fit into.
- Committed to: the twelve points above.

## Follow-ups

- The proposal points need a yes or changes; then they move up.
