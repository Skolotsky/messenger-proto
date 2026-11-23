# T-058: The web app on the real server

- Traces to: brief (the messaging experience and the activity feed), D-008, D-015, D-016, D-019 to D-024, G-015, G-019, T-038; contracts `screen-to-conversations.md`, `conversations-to-screen.md`, `door-reads.md`, `agent-door.md`
- Started: 2026-11-23

## Outcome

A person can use the web app on the real programs at 1280 and 400 px, and an agent's current state is visible beside its name.

## Acceptance criteria

- [x] The main flow works on the real system at both widths: buckets, reason and Undo, opening a row, Keep and Done, history tabs and counts, sending, thread, edit, reaction, delete, mention, search, channel, direct message, agent state, a situation reaching the feed and the notification app, search and feed off, Reset. Check: `CONTEXIA_REAL_URL=http://127.0.0.1:18080 tests/browser-outside/run real-product`
- [x] The spec fails when the app and server disagree. Check: with history entries identified by message alone, it failed on "the Direct messages tab shows the server's entries"
- [x] An agent's state stands beside its name and follows it live. Check: the same spec; a played situation showed "Working"
- [ ] Every point of T-038 checked on real data. Status per point under Evidence

## Verification plan

- Automated: `real-product.spec.ts` on the composed system; the mock-door browser tests around the changed code; `./scripts/check`.
- Real system: driven in a headless browser as Maya, Marco, Lena, Greta and Hana, with search and the feed off, Conversations stopped and restarted, and Reset with a screen open.
- Independent: not here, on the user's instruction. The lead runs `./scripts/verify` and the verifier once at the end.
- Not covered: see Evidence.

## Readings for the user

1. **The screen asks whether the feed is behind.** Nothing in the contract tells an open screen that the feed fell behind, so it asks: about 10 s after a change it hears of, every 10 s while visible, and every 5 s while the notice shows. This is one read per open screen, which grows with screens. `door-reads.md` wants the door's own checking to be per organization, so a small "behind" field on `last_sent` would be the better contract. It needs a decision.
2. **A question waits 4 s, an action 3 s.** The door waits up to 3 s for the feed or index and then answers with a notice; with the same limit on the screen, that notice was never heard.
3. **Where an agent's state shows:** on its newest message in the open conversation, on the reply it is writing, in the sidebar and the header of its direct conversation. Older messages show none, because the state is about now.
4. **The scripted agents now report a state:** waiting on the person a flag names, done after their last step in a situation, otherwise working. A lost status does not hold up the script.
5. **A grouped reaction row can read "🎉 2" beside a chip "🎉 3".** The rules leave out of the row's line a person whose reaction is an answer (D-014), while the chip counts everyone. Left as decided; T-038 point 18 could be read as wanting the two equal.
6. **History tabs load older entries** with a "Show older" button, since the server answers 50 at a time.
7. **The seeded history ends on 17 November**, so its rows read "Tuesday" or older beside anything new.

## Log

- 2026-11-23: the first version of the spec passed with the history defect planted, because it visited the Reactions tab last. It now visits it first and asks the server after the click.

## Evidence

Defects found on the real system and fixed:

1. History entries were identified by message alone, and one message has several reactions: rows from Reactions stayed behind in other tabs (71 rows shown in a tab of 44).
2. History showed only the first 50 entries, with no way to reach the rest.
3. An open screen never said "Catching up" when the feed stopped.
4. The door's notice could not reach the screen, because both gave up at 3 s.
5. A reaction in a channel the person is not in vanished once saved, since such a channel sends no live updates.
6. Agent state was delivered but not shown, and the scripted agents never reported one.

Results:

- `real-product` spec: 2 passed (both widths), on a fresh database.
- Mock-door browser tests for activity, live, conversation, thread and new messages: 203 passed, 19 skipped (the skips were not examined). `go test ./internal/demo/` passes.
- Screenshots looked at: feed at both widths, reason panel, history, thread at both widths, search, direct message, agent state.

T-038 on real data:

- Checked: 1, 5, 6, 7, 8, 9, 10, 12, 13, 14, 15, 17, 20, 21, 22, 24, 25.
- Seen but not asserted on its own: 16 (the newest row came first after Show).
- See reading 5: 18.
- Not checked on the real system: 2, 3, 4 (rule tests only), 11 (background tab), 19, 23 (the script does not stream replies, so text in progress was never on screen), 26 (mock door only).

Not verified:

- G-015 and G-019 have no guardrail test on the real system from this task; the spec covers their main path only.
- The control panel's Off, Running and Play buttons were not clicked: the same calls were made directly. The panel page was opened but its screenshot was not reviewed.
- Stopping and restarting Conversations was run at 1280 px only.
- `./scripts/verify` and the independent verifier were not run, on the user's instruction.
