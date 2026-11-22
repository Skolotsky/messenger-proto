# T-047: Delivery workers

- Traces to: brief (reliable delivery to independent systems), D-005, D-006, D-007, D-026, D-028, D-029, G-012, G-014, G-023, G-026, G-027 (workers), G-029
- Started: 2026-11-22

## Outcome

One delivery worker per receiver feeds search and the feed from each organization's change log, and the notification sender from the feed's note log, so that a slow, stopped or refusing receiver delays nothing else and nothing saved is lost.

## Acceptance criteria

- [x] A stopped or slow receiver delays no other (G-012). Check: `tests/guardrails/receivers-independent`; must fail when the workers share one place.
- [x] A refused change is parked after three tries, shown, and can be sent again; three different refusals pause that organization only; a refused note stops that organization's notes until retried (G-014). Check: `tests/guardrails/parking`; must fail without parking.
- [x] Every call gives up within 3 s; retries after 1, 2, 4 … 60 s with jitter, kept once per receiver; no other client exists (G-023). Check: `tests/guardrails/calls-time-out`.
- [x] What happens in one organization does not delay another (G-026). Check: `tests/guardrails/orgs-independent`; must fail with a worker serving in order of arrival.
- [x] Two copies of the worker lose nothing and repeat nothing in a fault-free run (G-027, workers). Check: `tests/guardrails/receivers-independent`, two-copies test; must show repeats with the claim taken out.
- [x] Timer questions do not slow down beside idle organizations (G-029, the delivery questions). Check: `tests/guardrails/idle-organizations-cost-nothing`.
- [x] Reset number, feed `i_am_at`, search `wipe`, sender edition and the nudge behave as the contracts say. Check: tests in `tests/guardrails/parking` and `receivers-independent`.
- [x] The delivery operations (status, parked list, retry, receiver restart) answer as `contracts/operations.json` says. Check: same tests through the HTTP routes.

## Verification plan

- Automated: guardrail tests against real PostgreSQL through the `delivery`, `conversations` and `feed` logins, with in-process workers and recording receivers. Each is seen failing with its rule switched off (build tag `deliveryfaults`, env `CONTEXIA_DELIVERY_FAULT`).
- Real system: `./scripts/verify` starts the composed system; the delivery program runs its loops and answers its operations.
- Independent: at Milestone 1 (T-057), by the lead.
- Not covered: the real feed, search and sender (T-048, T-052 join in T-057).

## Requests for shared changes

1. `server/internal/shared/orgdb`: a list of databases on the directory (`All()`). A worker asks "which organizations have work?" once per database (D-029), which `PoolFor(org)` cannot express. Stand-in: `delivery.Databases` and `delivery.OneDatabase`. Needed for G-028 (T-057).
2. `deploy/compose*.yaml`, `scripts/verify`: nothing needed. The tests create their own databases, so the running delivery program never sees their organizations.
3. For T-046 and T-052 (no change asked, for information): a save may add its due row with `ON CONFLICT DO NOTHING`; the worker copes with a save caught during removal (`TestASaveCaughtMidRemovalIsNotStranded`). If a worker dies in that instant, the change waits for the organization's next write.
4. G-029's housekeeping questions (ticks, leases, abandoned replies, old events) are not in `tests/guardrails/idle-organizations-cost-nothing` yet: T-053 and T-054 own that code and should add them there.

## Contract problems found (followed as written, not reinterpreted)

1. **Retry pause against catch-up.** `conversations-to-search.md` and G-006 promise "level within 30 s of coming back" after a 10-minute outage, and also retries "up to 60 s". A receiver that returns just after a failed try is not tried for up to 60 s, so the 30 s cannot always hold. Needs a decision: cap the pause near 20 s, or probe cheaply between tries. Built: 60 s, as written.
2. **A wipe of an idle organization.** The worker learns of a wipe from the next answer or from asking the place, but G-029 forbids asking idle organizations. So a wiped index of an organization with no new change is refilled only by `receivers/{receiver}/restart` (Rebuild calls it) or its next change. A bare `control/wipe` does not refill by itself.
3. **Reset number on old entries.** Log entries keep the reset number they were saved under, yet a receiver refuses anything older than its current one. The worker therefore stamps the current number on every item it sends.
4. **Retry answers.** `operations.json` lists only 200 for `parked/{n}/retry`. The worker answers 200 also when the receiver refuses again or is down (the entry stays parked, with one more try), and 404 for a number that is not parked.

## Log

- 2026-11-22: started. The code was drafted before the guardrail tests, so each test's failure was seen by switching its rule off (`deliveryfaults` build), not by running it before the code existed.
- 2026-11-22: the tests found three real defects: two copies claimed the same organization (the claim was not rechecked on the locked row); the planner could turn the "per due row" lookups back into whole-table reads (fenced with OFFSET 0); a fake that never noticed callers hanging up.
- 2026-11-22: the planted `arrival-order` fault at first did not make G-026 fail, because it still ran 16 organizations at once. The fault was corrected to serve one at a time; the check was not changed, and it then failed (20.6 s against 2 s).
- 2026-11-22: a full verify caught one more real defect, intermittently: a copy that read "healthy" just before another noted an outage made an extra try during the shared wait. A copy now reads the receiver's state again after claiming. The retry test then passed 15 times of 15 and the full verify passed.
- 2026-11-22: the coordinator said the user will remove the `reset` number in a separate task after this merges. Nothing new was built on it; the places that use it are listed under "Where `reset` is used".
- Choice on internals: the poll is a wall-clock pace, the one wait not on the injected clock, so a test clock cannot stop delivery; every decision (claims, retries) reads the injected clock. Tries of a change below three are kept as rows of the parked table with `tries < 3`, because the fixed tables have no other durable place; every reader filters them out.

## Where `reset` is used (for the removal task)

- `server/internal/delivery/receiver.go`: `staleReset` answer and `classify`; `place.Reset` and `client.place`; `stamped`; the `reset` parameter of `forSearch`.
- `server/internal/delivery/turn.go`: `spot.orgReset`, `spot.reset`; the comparison and `resync` branch in `pass`; both queries in `load`; `stamped` and `forSearch` calls in the three `deliver*` functions; the `staleReset` case in each; all of `resync`.
- `server/internal/delivery/ops.go`: `retryChange` and `retryNote` read it and send it.
- Tables it touches (not ours to edit): `org_log.reset`, `delivery_places.reset`, `note_heads.reset`, `note_places.reset`.
- `tests/guardrails/receivers-independent/rig/rig.go`: `SaveNote`, `AddOrg` (`Reset: 1`), `Receiver.resets`, `SetReset`, `resetOf`, the place answer, the `stale_reset` answer in `apply`.
- `tests/guardrails/receivers-independent/receivers_test.go`: `SetReset` in `TestASaveCaughtMidRemovalIsNotStranded`, and its hand-written body.
- `tests/guardrails/parking/parking_test.go`: `TestAResetIsFollowed`; `SetReset` in `TestTheFeedsPlaceWinsWhenOutOfSequence`. There `SetReset` is only used to set the receiver's place, so it needs a replacement, not just deletion.
- `tests/guardrails/orgs-independent/orgs_test.go`: the `reset` field of the backlog's bodies.

## Evidence

- `./scripts/verify`: all passed (every guardrail folder, harness, server integration tests).
- Each rule switched off (`-tags deliveryfaults`, `CONTEXIA_DELIVERY_FAULT`), each test failed for its reason: `no-parking`, `no-pause`, `notes-pass` (G-014); `no-time-limit`, `retry-storm` (G-023); `shared-place` (G-012); `arrival-order` (G-026: 20.1 s against 2 s); `no-claim` (G-027: 14, 112 and 550 repeats); `scan-all` (G-029: 40,070 rows read); `eager-remove` (lost wake-up).
- Healthy measurements: G-012 other receiver 19 in 20 within 22 to 28 ms; G-026 quiet organization within about 32 ms, saves under 5 ms; G-029 rows read the same at 1,000 and 20,000 idle organizations.
- Real program, on PostgreSQL with no receivers running: answers alive, status shows the feed `down` with its place kept and 2 waiting, search moved past two ticks, no panic.
- Not checked: the real feed, search and sender (T-048, T-052; joined in T-057); G-012's "reading and unread marks answer as before" (needs Conversations, T-046 and T-053); G-029's housekeeping questions (T-053, T-054); G-026's "must fail with one log for everybody" (the tables make that form impossible to switch on; the arrival-order variant was run); two databases (G-028, T-057).
