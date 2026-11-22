# D-024: How you write, what going wrong looks like, search results, and the two pages outside the product

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-004, D-009, D-010, D-019, D-022, D-023
- Evidence: E-002 (all of it is shown in `experiments/E-002-look-and-feel/`: `slack-like.html`, `control-panel.html`, `notification-app.html`, and covered by `check.py`)
- Visual: `experiments/E-002-look-and-feel/screenshots/slack-like-desktop-30` to `-36`, `control-panel-desktop.jpg`, `notification-app-desktop.jpg`

## Why this needs deciding now

After the feed was settled (D-019 to D-023), four things the brief asks for by name had no design: creating, editing and deleting a message; what a person sees when a system is slow or unavailable; search; and the surfaces that make the notification sender and the demonstration visible. The agent built one proposal for each into the prototype; the user tried them and accepted all four.

## Options

One proposal was put forward for each, as below. The alternatives the agent weighed and dropped: a confirmation dialog over the page for delete (it covers what is being deleted); delete without asking plus undo (undo of a delete is out, D-004); a banner across the top for offline (it pushes the page down, which breaks "no layout shift without the person's action"); hiding a message that failed to send (the person would believe it was sent).

## Best practice

Say what happened in the place where it happened; never move what a person is looking at; never let a person believe something was sent or found when it was not.

## Recommendation

The four proposals. Reversibility: cheap now, moderate once built.

## Decision

Accepted by the user on 2026-11-22: "Yes, accept all."

### Your own message

- Typing `@` suggests people, and "channel" as a mention of everyone that names nobody (D-001).
- Your own message offers Edit and Delete, and Remove flag when it carries one.
- Editing happens in place. The message then says "edited" (D-004: no edit history).
- Delete asks first, inside the message's own space, and says it cannot be undone. After it, a message without replies leaves nothing; one with replies leaves its marker (D-004).

### When things go wrong

- **Offline:** a notice beside the "Activity" title and one line in the sidebar say since when. Nothing on screen moves.
- **Written while offline:** the message stays in its place, marked as waiting, and is sent when the connection is back.
- **A send that fails:** the message stays in its place with "Try again" and "Discard".
- **Back online:** what arrived meanwhile waits behind the arrivals button (D-023), like any arrival.
- **Search behind:** the results say that the index is catching up and that recent changes may be missing.

How long before a notice appears, and what "behind" means in seconds, are not decided here: they belong to the quality targets (D-007, proposed).

### Search results

The results say how many messages were found, mark the match, show "edited" where it applies, and state the rule: an edited message is found by its new text, a deleted one is not found (D-004).

### Outside the product

- **Control panel (D-010):** start, pause and reset; the pace; what happened; what comes next, with "Play now"; each situation on demand; and for the search index, the notification sender and the feed a switch between Running, Slow and Off with how much is waiting.
- **Notification app (D-022):** what the sender delivered to a chosen person, each entry in the current state of its message, read when the row was read anywhere.

### Added by the user, 2026-11-22 (after acceptance; the decision is unchanged)

- **Enter and Ctrl+Enter.** "During editing I would like to use enter for new line and ctrl+enter for send (and configurable in settings of course)." Messages are multi-line. By default Enter starts a new line and Ctrl+Enter sends or saves; a setting, "Writing messages", switches to Enter sends with Shift+Enter for a new line. The agent's reading: it holds for the message box and for editing alike. The hint beside Send names the key and opens that setting ("this should be clickable then and lead to the settings").
- **The mention suggestions are driven from the keyboard** while typing: arrows move, Enter or Tab picks, Esc closes.
- **Every action has a tooltip** saying what it means, Flag and Info first of all.
- **A box that holds text is as tall as its text** and shows no scrollbar until it reaches its limit (reported by the user as a bug in the edit box).
- **A row of tabs that does not fit scrolls sideways by dragging.**
- **A line between days in a conversation**, "so I can visually distinguish today's messages from older ones".
- **What a bucket means is a tooltip on its name**, not a line of text beside it ("these explanations may be just tooltips").
- **An agent is marked by a robot on the corner of its avatar**, not by a text label. This is the visible agent mark D-002 asks for, in another form.

### Added at the design sign-off, 2026-11-22 (the user: "Yes, finalize")

Accepted with the sign-off: an agent's answer appears as it is written and reaches no feed until finished; a channel is created from a "+" with a name and people; an empty Focus offers three rules to start with. Found missing by the independent review and added: any channel message can start a thread (Reply in thread); a draft survives anything that redraws the screen and stays with its conversation.

## Consequences

- Easier: every part of the brief now has a designed surface, and the reliability story can be shown by one person with three windows: the product, the control panel, the notification app.
- Harder: a message has two more visible states, waiting and failed, that the system has to be able to tell apart truthfully. "Search is behind" requires the product to know how far behind each receiver is.
- Committed to: the forms above. The mock-ups of the two outside pages are connected to nothing and are a picture of them, not a start.

## Follow-ups

- The architecture decisions (T-022: D-005 to D-008, D-017, D-018) have to make these states real: knowing that a send failed, how far a receiver is behind, and what the notification app is fed.
- Not designed and left for the build: an agent's reply appearing as it is written, creating a channel, the person switcher, an empty Focus offering suggested rules.
