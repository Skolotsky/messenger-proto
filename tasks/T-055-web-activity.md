# T-055: Web app: Activity

- Traces to: brief (activity feed, product craft), D-001, D-003, D-004, D-011, D-012, D-014, D-015, D-016, D-019 to D-023, D-032, D-034; T-038 points 12 to 18; contracts `screen-to-conversations.md`, `door-reads.md`
- Started: 2026-11-22

## Outcome

In the real web app, at 1280 and 400 px, light and dark, a person can work through Activity: three buckets and Kept, rows that explain themselves and can be changed from the row, arrivals that wait behind Show, history tabs, one settings screen, the full reaction picker, and on a phone a React button and two swipes.

## Acceptance criteria

Every check is in `tests/browser/specs/activity-*.spec.ts` and runs at both widths unless it says otherwise.

- [x] Three buckets, newest first, with Kept in Focus, collapsed past five (D-003; T-038 16, 17). Check: `activity-feed`
- [x] A row's reason opens what it is, what put it here, where it would go otherwise, and one button that acts at once, says so and offers Undo; the panel never reads the row (D-020, D-021). Check: `activity-reason`
- [x] Arrivals wait behind Show, count rows and not messages, queue across batches, bring back a row marked Done, and raise the count while Activity is closed (D-023; T-038 12, 13, 15). Check: `activity-arrivals`
- [x] Nothing being read moves on an arrival, an edit, a delete, a keep or a hover (under 0.5 px). Check: `activity-jumps`
- [x] A click opens the conversation beside the feed (1280) or over it (400), at the first new message, and reads the row (D-023, point 21). Check: `activity-open`
- [x] Four history tabs with unread counts; reading leaves an entry in place; arrivals reach them (D-016, D-019; T-038 14). Check: `activity-history`
- [x] One settings screen: general rules, Focus people, channel levels, notifications, writing; what is removed can be added back (D-021, D-023). Check: `activity-settings`
- [x] The full picker with search, on a message and on an unopened row; the true count per emoji (D-023, D-014; T-038 18). Check: `activity-reactions`
- [x] On a phone: a visible React button, swipe right marks read (Done on a kept row), swipe left keeps (D-023). Check: `activity-phone`

## Verification plan

- Automated: Playwright through `web/app/scripts/browser-tests`, against the real mock door where it can produce the state, and typed fixtures valid against the schema where it cannot.
- Real system: the app used in a browser; screenshots at both widths and both themes.
- Independent: no; the milestone verifier comes at T-058.
- Not covered: the real feed service (T-052, T-058); offline and waiting states, mentions, search (T-056).

## Log

- 2026-11-22: started.
- Internal: the feed keeps the truth (`live`) apart from the list on screen (`shown`), in `web/app/src/state/feedModel.ts`. Whole-pixel lines and `overflow-anchor: none`, so the position hold is exact and does not depend on one browser's scroll anchoring.
- Connection code: `api/live.ts` gained `subscribeLive` and one line calling its listeners. `useSession.ts` gained `setPreference` outside the connection part. After a reconnect the feed is fetched again and applied in place; T-056 replaces this with the numbered catch-up.
- Found by planted faults: two checks passed with the fault in. `/read/` also matched `unread`, and Chromium's scroll anchoring hid a missing position hold. Both checks were fixed and then caught their fault.
- Found by looking: a refused Keep still said Kept, the error line pushed the list down 60 px, and on a phone the history Keep button touched the time. All fixed.

## Evidence

- `web/app/scripts/browser-tests`: all specs at 1280 and 400 px, Chromium, production build, real mock door plus fixtures validated against `contracts/schema/contexia.json`. Every action the screen sent in a fixture test was validated too.
- Seen failing: 21 planted faults, one at a time, each confirmed applied and built: arrivals applied at once; messages counted instead of rows; Done row not brought back; deleted row keeps its text; first reaction applied in place; row height not held; scroll position not held; asking why reads the row; no Undo; Kept never collapses; For your information read without being viewed; opening does not read; no New line; history arrival moves the list; removed people not listed; switching off removes everyone; wrong grouped count; reacting does not read; swipes swapped; refused Keep still says Kept; error line pushes the list.
- Looked at: wide and phone, light and dark: dense feed, reason panel, arrivals, picker, history, settings, people, channel levels, empty buckets, a long name and a long message.
- Not verified: the real feed service; Firefox and Safari; a real phone (touch was emulated through Chromium's input pipeline); contrast was not measured.

## Known gaps

- **The thread's level and the channel's level cannot be changed from the conversation.** D-023 points 17 and 19 and D-001 place them there, and the settings text says a thread is put in Focus on the thread itself. Channel levels work in settings and from a row's reason; a thread's level can only change from a row's reason. No package names this control.
- A Mark read that fails is not shown as failed: `session.read` retries at the next opening, and the row stays quiet until the feed reloads. Waiting and failed states are T-056.

## Readings for the user

1. Notifications settings leave out the "about 29 a day" figures that D-023 and the prototype show, because D-034 says simulated numbers must not appear in the product as evidence.
2. A first reaction on a row on screen waits behind Show, because it would add a line and move the list. Later reactions update in place.
3. An edit that gets longer is clipped to the row's height. D-001's expander lapsed with D-023 point 21, since a click opens the conversation.
4. An unread row deleted while in view becomes a marker until the person moves on; out of view it goes without a trace (D-004).
5. A row that is kept and read sits in Focus's Kept section whichever bucket the door reports (D-003, D-011).
6. Show also counts as moving on for For your information rows already viewed: they are marked read.

## Requests for shared changes

1. Contract: say what form `emoji` takes. The examples use names (`eyes`, `tada`), the picker sends the character. A name is shown as plain text today.
2. Plan: name a package for the level control on a channel and on a thread (see Known gaps).
3. Mock door, if it should show these states: keep and done do not imply read; no `history.changed` (the screen refreshes the tabs on any row change as a stopgap); the Reactions tab is always empty; settings are fixed; a direct row can name an author who is not in that conversation (`dm:anna:marco` by Yuki).
4. `tests/browser/package.json` gains `ajv` for validating fixtures.
