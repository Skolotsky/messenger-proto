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
- [ ] Press Reset with a screen open and it lands on the fresh organization; cleanup in the real panel. Check: `tests/browser-outside/specs/real-system.spec.ts`, at T-058 and T-061 with the real feed

## Verification plan

- Automated: G-020 on real Conversations, delivery, search and sender, with a stand-in feed that answers its contract; planted faults through the `demofaults` build tag.
- Real system: the composed system with the stand-in feed, driven through the one address.
- Independent: at T-058 and T-061.
- Not covered until T-052 lands: the real feed's wipe, edition and rebuild; notifications from real feed notes.

## Log

- 2026-11-23: the first rebuild check passed with a rebuild that never refilled, because delivery repairs a wiped receiver when the next change reaches it. The check now also runs on an idle organization, where that repair cannot hide the fault.

## Evidence

Filled in at the end of the task.
