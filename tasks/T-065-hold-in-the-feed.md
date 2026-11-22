# T-065: The hold moves into the feed; scope and claims narrowed

- Traces to: D-034, D-012, D-018, G-010, G-011, G-014, G-027, G-029
- Not started: waits for T-064 to merge. Comes before T-052.

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
- [ ] `./scripts/check` and `./scripts/verify` pass.

## Log

- 2026-11-22: D-034 and its five readings accepted by the user.

## Evidence

_None yet._
