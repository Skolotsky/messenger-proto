# T-059 (second part): The demo program

- Traces to: brief (a busy team, delivery to independent systems made observable), D-002, D-010, D-022, D-024, D-033, D-034, G-020, G-031; contract `operations-and-control.md`
- Started: 2026-11-23

## Outcome

`make up` gives one address with the product, the control panel and the notification app. A scripted team plays through the real doors, Reset lands everyone on a fresh organization with the same starting state, and a presenter can break, rebuild and clean up the systems from the panel.

## Acceptance criteria

- [x] The team acts only through the people and agent doors; all 16 situations play and write what their scripts say. Check: `TestTheTeamPlaysEverySituationThroughTheDoors`
- [x] Start, pause, pace and Play now behave, and a step repeated after an outage keeps its ID. Check: `server/internal/demo/script_test.go`
- [x] Two Resets give equal starting states under different organizations; the previous one answers 410. Check: `TestTwoResetsGiveEqualStartingStates`
- [x] Nothing becomes current before search and the feed are level. Check: `TestResetWaitsUntilLevel`
- [x] Rebuild of each receiver equals what it held, active or idle, and touches no other organization. Check: `TestRebuildEqualsWhatItHeldAndTouchesNoOther`, `TestRebuildRefillsAnIdleOrganizationByItself`
- [x] Delete removes a retired organization, can be repeated, and refuses the current one. Check: `TestCleanupLeavesTheCurrentAndRemovesRetired`
- [x] Only a rebuild after a rule change tells the feed so. Check: `TestOnlyARuleChangeMarksTheRebuildAsHistory`
- [x] Press Reset with a screen open and it lands on the fresh organization; cleanup in the real panel. Check: `CONTEXIA_REAL_URL=<address> tests/browser-outside/run --project=wide real-system`

## Verification plan

- Automated: G-020 on the real Conversations, delivery, search, sender and feed; planted faults through the `demofaults` build tag.
- Real system: the composed system, driven through the one address and in a browser.
- Independent: at T-058 and T-061.
- Not covered: the phone width of the cross-page flow; a real push provider; `make up` on the default ports (run on another project and port, beside other sessions).

## Readings for the user

1. "Level" for Reset means search and the feed have reached the seal's number with nothing parked; the sender is not waited for, since a seed writes no notes.
2. The next organization is always prepared in the background, so Reset is immediate. It is listed as "Preparing", cannot be deleted, and is never shown to anyone until it is current.
3. A seed a crash left half done is retired on the next start and can then be deleted.
4. "Play now" works while the script is paused and leaves it paused. A situation played on demand comes off the schedule unless it has already begun.
5. After the schedule finishes the script stops; Start plays it again from the first situation. The script's position is in memory: restarting the demo program starts it again, paused.
6. Repeating Delete on an organization already gone answers 200.
7. The product opens as Maya, the content's default viewer, in the `make up` build.
8. The list of organizations uses the key `id`, not `org`: it is operator bookkeeping about organizations and holds none of their content (D-033, reading 4), so G-031's fixture needs no row for it.

## Requests for shared changes

1. `docs/guardrails.md`: set G-020 to `enforced` and add its protection rule to `.claude/settings.json`.
2. `scripts/verify`: run the must-fail set: `go test -tags demofaults -run Fails ./guardrails/rebuild/`.
3. `contracts/` and `operations-and-control.md`: accept `preparing` as an organization state.
4. `tasks/board.md`: the outside pages' assumptions 1 to 5 and 8 are now real; 6 and 7 (`edited`, an author field) stay open.

## Log

- 2026-11-23: the first rebuild check passed with a rebuild that never refilled, because delivery repairs a wiped receiver when the next change reaches it. The check now also runs on an idle organization, where that repair cannot hide the fault.
- 2026-11-23: one run saw the previous organization still accepting an action right after Reset. Its cause was not established. Reset now repeats the retire call for up to 10 s before answering, and the background loop keeps trying; the test passed in every run since.
- 2026-11-23: the stand-in feed was replaced by the real one when T-052 merged.

## Evidence

- `tests/guardrails/rebuild`: 7 tests pass on all real parts.
- Planted faults, each caught for its reason: a different seed per Reset (states differ), Reset not waiting for level, a rebuild wiping every organization, a rebuild that never refills. A wrong text in the door play was caught by the play-through test.
- Browser, on the composed system with the real feed: Reset with a screen open moved the screen to the fresh organization; Delete all retired removed the old one. With the entry route broken on purpose the same test failed ("the open screen moves by itself").
- Composed system: a situation played at normal pace reached the notification app in 7 s; a feed rebuild took under 2 s and raised the edition with the same counts; Reset from a prepared organization took 11 ms.
- Not verified: the phone width of the cross-page flow, and the plain `make up` ports.
