# D-023: How the feed is used day to day in the chosen layout

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-003, D-015, D-019, D-020, D-022
- Supersedes in part: D-003 (expanding a row in place and replying from the row are dropped at every width, point 21; reacting from the row stays, through a button)
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

13. **On a phone a row still takes a reaction from the feed**, through a visible React button on the row. "On mobile I still want to be able to put reaction right from activity feed."
14. **The two swipes on a phone are Keep and Mark read**, one in each direction. "Reaction shouldn't be on a swipe." As built: to the right marks read (Done on a kept row), to the left keeps; the row stays in place either way.
15. **What can be removed in settings can be added back in the same place.** "Here I can remove, but can't add them back. Please make it consistent." General rules are switches; a removed person, thread or sender stays listed under Removed with Add back.
16. **The Activity feed page of settings keeps the general rules; what was added one by one lives in its own lists**: Focus people, Focus threads. "So I can conveniently manage these lists separately, when the main activity settings page keeps general rules."
17. **Channel levels belong to Activity feed** in settings, as one more page under it, listing every channel with its level, changed in place.
18. **Whatever the reason panel on a row can change is also in settings**, and the panel links to that place. "Ensure the rule is in settings, not only here."

19. **Threads put in Focus are not listed in settings.** "I'm not sure I want focus threads in settings, it would be polluted over time." A thread is put in Focus, and taken out, on the thread itself: it is the thread's own level (D-001's thread dial), not a rule kept in a list. History → Threads shows each thread's level. This narrows points 16 and 18: people and channel levels are lists in settings, threads are not.

20. **No inline threads.** "Please don't do inlined threads. Do it Slack way. I hate inlined threads." A message with replies shows a link under it (who replied, how many, when last) and the thread opens on its own. A deleted message that has replies keeps its marker and that link (D-004).

21. **A feed row has no opened state, at any width. A click opens the conversation.** "There is no need to have expanded state in the activity feed if clicking doesn't close activity feed but just opens it alongside. So both for desktop and for mobile we don't need it for different reasons." On a wide screen the conversation opens beside the feed, at the first new message, with the cursor in the message box; on a phone it opens over the feed. This supersedes D-003's expanding in place and replying from the row at every width, and with it points 4, 5, 6 and the wide-screen half of point 9 above, which described the opened row. What stays on the row itself: React, Mark read, Keep (Done on a kept row), and the reason line (D-020).

22. **Done can be taken back.** "When it's done I should be able to click keep again, if I mistakenly clicked done." While a row marked Done is still in the feed (until the person leaves it, D-003), Keep is offered on it and puts it back as kept. On a phone the Keep swipe does the same.

23. **On a wide screen a thread opens beside its channel, not in its place.** "Threads should open alongside with the channel on a desktop, not replacing it. At least this is Slack pattern and I find it good enough." Activity gives up room for it while it is open. On a phone the thread opens in place of the channel, with a way back.
24. **Panels can be resized** by dragging the line between them. "It's basics."
25. **One settings button on Activity.** "Rules and feed settings open the same screen. A bit strange, let's leave just settings button."

26. **A setting decides whether a reply or a reaction marks a kept row done.** "There should be a setting which lets you mark these done only by explicit done, so reaction or reply wouldn't do that." On, as D-003 and D-011 have it: replying or reacting releases a kept or flagged row. Off: only Done does. It sits with the general rules of the Activity feed settings; on is the start.

27. **The time sits at the far right of every feed row, and nothing is reserved for actions.** "I don't like how the alignment looks like, time jumps between rows, let's use the common layout." Chosen from three variants ("A, time far right"; `screenshots/rows-a.png`, `rows-b.png`, `rows-c.png`). On a wide screen the actions float over the row's corner on hover; Done and the row's state sit at the bottom right. On a phone the React button and Done sit at the bottom right.

28. **Choosing another person is a real switch, one level down in the person's menu.** "It leaves me no choice, is it planned? Also picker of the user should be a different level of menu." The menu has one entry, Switch account; the people are under it. Choosing one asks, then switches, with a band across the top saying whose view it is and a way back. The mock-up holds only Maya's messages, so it switches the account and the band over her data; the product shows that person's own feed (D-002, D-010).
29. **The line under a name is the person's own words, not a role.** "'Product', 'founder' are just labels people put for their accounts, not some role system." It is edited in Account, means nothing to the product, and is not shown in the account list.

30. **Theme: System, Light or Dark.** The user asked that it match the system, have a switcher and follow best practice. System is the default and follows the device live. The switcher is in the person's menu and under Settings → Appearance. As built by the agent to that practice: the choice is remembered per device, applied before the first paint so nothing flashes, shared with the control panel and the notification app, and text meets 4.5:1 contrast in both themes. The sidebar stays dark in both, as in the messenger people know.

31. **The organization you are in is always shown; there is no switcher between organizations.** "Organization should be shown as well, cause such systems are always multi-tenant, so it matters in which organization I am. We'll not build the switcher between organizations, it's out of scope." It is named at the top of the sidebar, above the title on a phone's first screen, in the person's menu and on the Account page. It is a label, not a control. This answers the question D-026 left open: the product shows no second organization.

### Proposed by the agent, accepted with the design sign-off ("Yes, finalize", 2026-11-22)

- Lapsed with point 21: context above the new messages in an opened row, and how a long burst is shown there. The conversation shows the whole exchange with a "New" line at the first new message.
- **The arrivals button is called "Show"**, and the control names how many arrived in Focus and Needs you.
- **Which swipe is which:** the user set the pair; that right is read and left is keep is the agent's pick.
- **On a phone the reply box is not given the cursor automatically**, since that opens the keyboard.
- **The settings sections** are Account, Activity feed (with Focus people and Channel levels under it) and Notifications. There is no Sign out; Switch account stands in for sign-in (D-009).
- **The Notifications page** asks one question, "What should send you a notification?", with two choices and their daily figures from E-001. The user disliked the first form and did not understand the first Channels page; both were redrawn and not yet reviewed.

## Consequences

- Easier: one way to each thing, and a phone layout with two states, list and conversation.
- Harder: two interaction models for a row, by width. The settings screen becomes a surface of its own that every later setting must fit into.
- Committed to: the points above, as narrowed by point 21.

## Follow-ups

- What the prototype only approximates is in `tasks/T-038-build-acceptance-from-e002.md`.
- One word for one thing: the interface and the user say "organization", D-026 and the architecture say "team". To settle when the architecture is reviewed.
