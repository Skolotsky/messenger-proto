# T-053: Conversations, live connection, door reads and housekeeping

- Traces to: brief (real-time experience; independent systems that can be slow or unavailable), D-005, D-007, D-015, D-029, D-031, D-032, D-033, D-034 (reading 1), D-035; G-015 and G-019 (server half), G-024, G-029 (housekeeping's questions)
- Started: 2026-11-22

## Outcome

An open screen stays current over one live connection and recovers from any interruption by number. The door answers feed, history, settings, preview and search, or says plainly why it cannot. Watching lapses by itself, and the log gets its clock ticks.

## Acceptance criteria

- [x] Two streams with `after`, `last_sent` every 5 s, resume, up to 500 replayed, `too_far_behind`. Check: `tests/guardrails/live-order`
- [x] Per-person filter equals the first load's rule for every log kind and person. Check: `TestLiveFilterMatchesTheLoad`, `TestEveryScreenFollowsItsOwnFilter`
- [x] A copy pushes only what it reads from the log, in number order, one reader per organization; a save only wakes it. Check: `TestTwoSavesReachingTheHubSwapped`, seen failing with `-tags fault_hub_handed`
- [x] A retired organization's connections are closed with `organization_gone`, also a new one. Check: `TestRetiredOrganizationClosesItsConnections`
- [x] Door reads through `call`, with notices when unavailable or more than 10 s behind, measured from the delivery place. Check: `TestDoorReadsAndTheirNotices` against recording stand-ins
- [x] G-024: watching per connection with leases. Check: `tests/guardrails/watching-lapses`, seen failing with `-tags fault_no_lease_check` and `-tags fault_no_watch_end`
- [x] Ticks timed from the previous tick, a busy organization still gets them, they stop 6 s after the last change. Check: `TestABusyOrganizationStillGetsTicks` (seen failing with `-tags fault_tick_from_newest`), `TestTicksStopSixSecondsAfterTheLastChange`
- [x] G-029 for housekeeping, judged by rows read at 50 and 500 idle organizations. Check: `tests/guardrails/idle-organizations-cost-nothing/housekeeping_test.go`, seen failing with `-tags fault_idle_scan`
- [x] The 5-second check and the nudge's receiving end. Check: `TestLostUpdateIsFetchedByLastSentWithEveryNudgeLost`, `TestFeedStreamFollowsAndANewEditionLoadsFresh`

## Verification plan

- Automated: the real Conversations on PostgreSQL with a controlled clock and a model screen that keeps the contract's rule; recording stand-ins for the feed and search.
- Real system: the composed program through `./scripts/verify`.
- Independent: at T-058 (Milestone 2), not in this task.
- Not covered: the browser half of G-015 and G-019 (T-056, T-058); the real feed service (T-052); two copies behind a real router (T-057).

## Requests for shared changes

1. **`scripts/verify`:** run the new must-fail builds: `go test -tags fault_hub_handed -run TestTwoSaves ./guardrails/live-order/`; `-tags fault_no_lease_check -run Frozen` and `-tags fault_no_watch_end -run Vanished` on `./guardrails/watching-lapses/`; `-tags fault_idle_scan -run TestHousekeeping` on `./guardrails/idle-organizations-cost-nothing/`; `-tags "integration fault_tick_from_newest" -run BusyOrganization` on `./internal/conversations`. Each must fail. Today they were run by hand.
2. **`deploy/compose.yaml`:** give `conversations` `CONTEXIA_CONVERSATIONS_URL` once a second copy exists, so housekeeping's nudge reaches the copy serving an organization. With one copy the local wake-up is enough.
3. **`server/go.mod`:** gained `github.com/coder/websocket v1.8.13`, the version `tests/go.mod` already uses and the plan's choice.
4. **`docs/guardrails.md`:** G-024 can move to `enforced` with its folder protected, and G-029's "Enforced by" can name the housekeeping file.
5. **T-054:** send pieces and agent status through `Store.Outside(org, conversation, message)`, which reaches the open screens of that conversation (or everyone for `""`).

## Readings for the user

1. **How far behind** is the age of the oldest change waiting for that receiver, by the log's timestamp against the clock. The contract's words ("newest change against its delivery place") would show a false notice after a quiet stretch, and none for one message stuck for hours.
2. **A retired or deleted organization on the live address** answers over the connection, because a browser cannot read the status of a refused upgrade.
3. **After `too_far_behind`** on the conversations stream the connection carries on from the person's newest number; on the feed stream it stops following until the screen resumes. Both are corrected by the `after` and `last_sent` rules.
4. **`read.moved`** carries a channel's new position; the screen works out which thread summaries that clears. Listing them would grow with the channel's history.
5. **A copy declared dead** is refused new watchings; its other saves are not fenced, since they come from a person's action with its own ID. It takes a new identity and drops its connections, so screens reconnect and say again what they have open.
6. **The preview** answers "not available" while the feed is more than 10 s behind. History and settings answer with what the feed has, since their answers have no field for a notice.
7. **An open connection accepts any origin**, because sign-in and the access model are outside the first version (D-009).

## Log

- 2026-11-22: one hub per organization with open screens, with a log reader and a feed reader. Resumes are carried out by the readers themselves, so a replay and the live tail cannot interleave. A screen that cannot keep up is dropped and resumes by number, so it never slows the organization.
- 2026-11-22: a connection's end now deletes what the database says it has open, not what memory says, because a start whose answer was lost with the connection would otherwise stay open and mark messages read for nobody.

## Evidence

Filled in with the final runs before this task is handed back.
