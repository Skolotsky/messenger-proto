# T-065: The hold moves into the feed; scope and claims narrowed

- Traces to: D-034, D-012, D-018, G-010, G-011, G-014, G-027, G-029
- Started 2026-11-22 beside T-064's last delivery fix (the user: "eliminate bottlenecks when you can"). Its delivery part follows that merge. Comes before T-052.

## Outcome

The feed holds a notification for 5 s by the log's timestamps and the sender only delivers. The connector, text rules, two-database testing and the 100,000 figure are gone from everything current.

## Acceptance criteria

- [ ] Contracts, schemas, guardrails, board and build plan agree with D-034. Check: a fresh verifier.
- [ ] Held entries live in the feed's tables, saved with the row; ticks are timed from the previous tick. Check: rule and delivery tests, including a busy organization whose changes are under 1 s apart.
- [ ] Nothing is sent for something read, deleted, fenced, edited away, un-reacted or re-dialled within the hold, with the feed late and with a backward clock step. Check: G-011's cases moved to the feed, each seen failing with its rule off.
- [ ] Once per name after restarts, rebuilds and a wiped sender; within 8 s when healthy; "delayed" after 10 minutes; an update before its send is remembered; `history: true` is listed and never delivered. Check: G-010, each must-fail variant kept.
- [ ] A parked note holds nothing back. Check: G-014.
- [ ] G-029 uses 50 and 500 idle organizations and judges rows read. The lint protecting the database function runs in `./scripts/check`, seen failing.
- [ ] Searches for `stopped_at`, the removed note kinds, G-028 and "100,000" show only history. Check: shown in Evidence.
- [ ] Simulated figures appear nowhere as evidence. Check: search of the README, product and validation text.
- [ ] The pause between retries is at most 20 s (D-035). Check: a test of the pause, seen failing at 60 s.
- [ ] `./scripts/check` and `./scripts/verify` pass.

## Log

- 2026-11-22: D-034 and its five readings accepted by the user.
- 2026-11-22: code half built. Notes are `send` and `update`; held entries live with the row in the rules; only a log tick releases; the sender delivers once per name. A parked note is stepped over (`skip`) and applied on retry. A parked retry holds a claim. Retries pause at most 20 s.
- Open for others: the tick writer in `server/internal/conversations/save.go` still pushes `tick_due_at` forward on every save, so a busy organization would get no tick (T-053). The feed service must store held entries with its rows and call `Fence` (T-052). `scripts/lint-database-access` still has to be wired into `./scripts/check`.

- 2026-11-22: Merged by the lead with G-010, G-011, G-014, G-023, G-029 and G-031 reworded as D-034 requires, and the database lint in `./scripts/check`. The hold through the running feed and the 8 s promise end to end wait for T-052 and T-053.

## Evidence

Each seen failing with its rule off, then passing:

- G-011, hold counted from arrival: sent for something read within the hold (feed stopped, feed slower, clock stepped back).
- G-011, any change releases: sent in "conversations out of step".
- G-011, a read leaves the entry: sent in the feed-stopped, busy-organization, reaction and dial cases.
- G-010, no list of names: a name sent 5 times. A sender that sends nothing: names sent 0 times.
- G-014, a parked note holds the rest back (`notes-stop`): later notes never arrive.
- G-029, `scan-all`: 1,070 to 2,090 rows read beside 500 idle organizations, against at most 80 allowed.
- G-031, retry without a claim: stop answers 200 with the retry inside the receiver's call.
- D-035, pause at 60 s: "a pause of 1m0s".
- `scripts/lint-database-access`: fails on a planted `pgxpool.New` outside `orgdb`.

G-030 passes unedited, and still fails with an organization-wide change moved.
