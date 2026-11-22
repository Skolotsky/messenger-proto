# T-048: The two stand-ins (search index and notification sender)

- Traces to: brief (independent systems over the network), D-006, D-007, D-018, D-026, D-031, D-032, G-006, G-010, G-011
- Started: 2026-11-22

## Outcome

Two stand-ins behind real network boundaries. The search index holds exactly the current text of every message that is not deleted. The notification sender sends each named notification exactly once, after the 5-second hold by the notes' own clock.

## Acceptance criteria

- [ ] Search: version rule, delete remembered, `wipe` number, true total, match marks in code points, edited, thread, `through`, direct messages only to participants, stale-reset, refusal. Check: `tests/guardrails/search-equals-truth`
- [ ] G-006 seen failing without the version rule, then passing. Check: same folder, build tag `standinfaults`
- [ ] Sender: names, list surviving a wipe with time and delayed mark, delayed over 10 minutes, editions, history after a rule change, list for the app. Check: `tests/guardrails/notification-once`
- [ ] G-010 seen failing without the list of names and for a sender that sends nothing, then passing. Check: same folder, tag `standinfaults`
- [ ] G-011 seen failing when the hold counts from arrival and when any note moves the clock, then passing. Check: `tests/guardrails/notification-hold`, tag `standinfaults`
- [ ] Both: place, mode (running, slow, off), wipe, per organization and for all. Check: the two contract tests

## Verification plan

- Automated: each stand-in over HTTP on PostgreSQL with its own login, driven by the log writer through the troublemaker, with a controlled clock.
- Real system: the composed `sender` program killed mid-burst; `./scripts/verify`.
- Independent: no, the milestone verifier (T-057) covers it.
- Not covered: the real delivery worker and feed (T-047, T-052); notes are derived from the log by the test.

## Requests for shared changes

1. `scripts/verify`: also run the "must fail" tests: `go test -tags standinfaults ./guardrails/search-equals-truth/ ./guardrails/notification-once/ ./guardrails/notification-hold/`. Today they run only by hand.
2. Move `tests/guardrails/notification-once/rig` to `tests/harness/` when convenient. It serves three guardrails, and it sits in one guardrail's folder only because this package may not edit the harness.
3. T-059 needs an operation to restore a stand-in from its saved copy. None exists in `operations.json` (contract problem 2).

## Contract problems found (followed the words; not reinterpreted)

1. **A rebuild after a rule change has no signal on the wire.** The contract says the sender lists earlier notifications as history, but no note says so. The only vocabulary is `withdrawn` with `because: "rule_changed"`, so the sender treats that as "history, never sent". T-052 must write it directly after the `due`, before any clock note that would release it. Needs the lead's confirmation.
2. **Reset cannot reach the stand-ins.** They learn a higher reset number from the first item carrying it, and nothing tells them to restore a saved copy.
3. **A search change does not say which wipe it belongs to**, so a copy in flight during a wipe cannot be recognized. Handled inside: after a wipe `through` follows the replay until it has passed the old place. A second worker's stale copy arriving mid-replay can still overstate `through` briefly.
4. **A withdrawal stamped after the hold, arriving before any clock note passed it.** The contract's two sentences pull apart. Followed "due and not withdrawn within its hold is sent": it goes out and its entry shows the truth, as in E-003.

## Log

- 2026-11-22: started. Choices on internals: modes live in the program's memory, so two copies must each be told (matters for G-027); `off` drops the connection when the caller gives up or the mode changes, because a handler that just returns would answer an empty 200; a delete blanks the text of a sent entry (D-004); a `history` entry's `at` is when it fell due.
- 2026-11-22: first test run hung: the troublemaker does not pass on the caller's giving up, so a request held by `off` never ended. Fixed in the stand-in by the mode-change release above.

## Evidence

- Seen failing with each rule off (`-tags standinfaults`), each for the right reason:
  - G-006 without the version rule: 16 differences, for example a deleted message back in the index.
  - G-010 without the list of names: a name sent 3 times. Sender that sends nothing: names sent 0 times.
  - G-011 hold counted from arrival: a notification sent for something read within the hold. Any note moving the clock: the same, in the "out of step" case.
- Then passing: the three guardrail folders, `./scripts/check`, `./scripts/verify` (all passed).
- Real program: `sender` killed twice mid-burst and restarted, run three times, each name sent once; the test fails if the burst ended before the kill.
- Not checked: the real delivery worker and feed (T-047, T-052; the test plays the worker and derives the notes); the 8-second promise end to end (needs the feed's clock notes); the 30 s catch-up at a real ten-minute backlog (about 130 changes here); two copies (G-027); answers against the JSON schema (decoded into the Go contract types only).
