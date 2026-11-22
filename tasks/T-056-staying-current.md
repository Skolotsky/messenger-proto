# T-056: Web app: staying current and going wrong well (second half)

- Traces to: brief (real-time messaging, reliability, product craft), D-001, D-015, D-023, D-024, D-031, D-032, D-033, D-035, G-015, G-019; T-038 point 26; contracts `conversations-to-screen.md`, `screen-to-conversations.md`
- Started: 2026-11-22

## Outcome

A person's screen stays right through anything the connection does and ends equal to a fresh load. What they do without a connection waits in its place and goes out in order. A channel's and a thread's level change from inside the conversation.

## Acceptance criteria

Every browser check runs at 1280 and 400 px against the production build and the mock door.

- [x] Numbered updates: apply only after the last number, ignore what is held, ask again otherwise; `last_sent`; `too_far_behind`; new edition; only a full load sets a position. Check: `live-rules`, `live-catch-up`
- [x] Cut, refused, single lost updates, more than 500 away, another view opened with updates in flight, all nudges lost, retired organization: each ends equal to a fresh load. Check: `live-catch-up`
- [x] Reconnect 0.1 s doubling to 1 s, varied, never above its step, with an injected random source. Check: `live-rules`
- [x] Pieces of a reply, gaps fetched, `reply.abandoned`. Check: `live-catch-up`
- [x] A message, edit and reaction wait in place and go out in order with their IDs; a refusal says so with Discard; Try again only where it can help; a failed Mark read shows as failed. Check: `live-waiting`
- [x] `organization_gone` and a 404 for the organization go to the entry address and load the current one. Check: `live-catch-up`
- [x] "Reconnecting", "Catching up", "may be out of date" in place, past 10 s, moving nothing. Check: `live-notices`
- [x] The level control in a conversation; `read` for the right row; `watching` over the connection. Check: `level-control`
- [x] The mock door follows the contract and its answers fit the schema. Check: `tests/harness/mockdoor`

## Verification plan

- Automated: Playwright through `web/app/scripts/browser-tests`; Go tests of the mock door against the schema.
- Real system: screenshots at both widths and themes.
- Independent: no; the milestone verifier comes at T-058.
- Not covered: the real server (T-053, T-058); Firefox and Safari; a real phone.

## Log

- 2026-11-22: started.
- Internal: the stream rules are a small pure file (`api/stream.ts`), the connection is `LiveLink`, waiting actions are `state/outbox.ts`. What is unsaved is laid over the list on screen and never written into it.
- Updates that arrive while a list is loading are held and applied to its answer, because the answer can be older than they are.
- Found by planted faults: three checks passed with the fault in. A reversed `message.created` hid wrong-order reactions; reopening a conversation hid an update lost to a moved position; a later door answer hid an ignored `organization_gone`. All three checks were rewritten and now catch their fault. The mock door gained a switch that keeps `last_sent` back, so it cannot heal a fault by luck.
- Found by looking: the level control's arrow pointed sideways.

## Evidence

- `web/app/scripts/browser-tests`, both widths, Chromium, production build, mock door: 334 passed, 40 skipped (the other width's tests and the picture writers). The run showed 336 with a throwaway picture check at each width, since removed.
- `go test ./harness/mockdoor/`: passes, with live messages and answers checked against the schema.
- Seen failing, at 1280 px, one planted fault at a time, each confirmed applied and compiling (22): applies whatever arrives; reconnects without its numbers; a view load moves the position; `last_sent` ignored; `too_far_behind` ignored; new edition ignored; `organization_gone` ignored; 410 and 404 ignored; reply gap not fetched; `reply.abandoned` ignored; pause above its step; no variation; waiting actions newest first; refusal offers Try again; failed send does not hold back the next; failed Mark read silent; waiting not shown; notice at once; Catching up never clears; watching not said again; Follow sets the wrong level; thread read as the channel.
- Which fault fails which G-015 scenario: applying whatever arrives fails single lost update and wrong order; reconnecting without numbers fails cut, refused and more than 500 away. Another view in flight, all nudges lost and a retired organization fail for their own specific fault (a view moving the position, `last_sent` ignored, `organization_gone` ignored), not for those two.
- Looked at: wide and phone, light and dark: waiting message, Reconnecting notice, level menu.
- Not verified: the real server; Firefox and Safari; a real phone; planted faults at 400 px; contrast was not measured.

## Known gaps

- **Agent status is kept current but nothing on screen shows it yet**, and it has no browser check.
- `preference`, `profile_line`, `people.added` and `person.added` parts are applied, without a browser check of their own.
- Waiting actions live in memory: reloading the page while offline loses them, with no warning.
- Settings, dials, creating a channel, opening a direct message and Keep from a history tab are not queued. Without a connection they say "Not saved" and change nothing.
- The history tabs still refresh on any row change when the door sends no `history.changed` (T-055's stopgap).

## Readings for the user

1. **"Reconnecting" appears after 10 s without a connection**, not at once (G-019, D-007), so a short blip shows nothing. A message written meanwhile shows "Waiting to send" at once.
2. **A failed new message holds back later ones** until Try again or Discard, so messages cannot overtake each other. Reactions, edits and reads do not hold anything back.
3. **A waiting action can be discarded** while it waits.
4. **A message created and deleted while you were away leaves no marker**, because you never saw it (D-004). One you had on screen keeps its marker until you move on.
5. **A failed Mark read** says so at the top of the feed, and the row is unread again.
6. **A thread's automatic level** is shown as Needs you if you wrote in it, For your information if you are mentioned, otherwise Off (D-001).
7. **A thread's page always includes its root**, also when opened around a reply further down.

## Requests for shared changes

1. Contract: state the body of a 404 for a deleted organization. The screen recognises it by the word "organization" in `refused`, because a 404 can also mean an unknown person.
2. Contract: state that a thread's page around a position always includes its root.
3. Contract: say whether a person viewing a channel they are not in gets live updates. The mock sends none.
4. T-059: serve the entry address. The screen asks `VITE_ENTRY_URL` (default `/demo/current`) and opens `/?org=<org>`. `web/app/vite.config.ts` proxies `/demo` to the door for now.
5. T-053: send `unread` parts. The screen no longer guesses a mark from an arriving message.
