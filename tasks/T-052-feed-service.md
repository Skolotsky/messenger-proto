# T-052: The activity feed as a running service

- Traces to: brief (the activity feed; independent systems that can be slow or down), D-006, D-017, D-018, D-026, D-029, D-030, D-031, D-032, D-033, D-034, D-035, G-007, G-008, G-009, G-027, G-030, G-031
- Started: 2026-11-22

## Outcome

`server/cmd/feed` takes the log's changes strictly in order and saves its rows, counts, place, held notifications and notes in one transaction, so each change counts once. It answers the door's questions from its own store, never calls Conversations, and can be wiped and rebuilt from the log for one organization or all.

## Acceptance criteria

- [x] Only the change that follows the last one is taken; the contract's answers (`applied`, already had it, `fenced`, `i_am_at`, `organization_stopped`). Check: G-007 with repeats, lost answers, late copies and a misplaced worker
- [x] A restarted or second copy produces exactly what one uninterrupted copy would: rows, counts, tabs, settings, held entries, notes. Check: G-007 (restart), G-027 (`feed-two-copies`, taking turns and racing, one copy killed)
- [x] A save that dies before its commit counts for nothing. Check: `TestFailsNothingWhenASaveDiesBeforeItsCommit` (build tag `feedfaults`)
- [x] Notes numbered per organization and edition without gaps, `note_heads` and `note_due` in the same transaction; a rebuild after a rule change marks its sends as history. Check: `DiffNotes` in G-007, `TestRebuildAfterARuleChangeMarksHistory`
- [x] A change that cannot be applied fences that reader in that conversation or the conversation, removes held entries, sets later changes aside; more than five fences stop that organization only; a fault on a reader-wide change stops it. Check: G-009
- [x] Editions, wipe, place, mode (running, slow, off), for one organization and all. Check: `TestControlTouchesOneOrganization`, `TestSlowModeAndAllOrganizations`
- [x] `DELETE /orgs/{org}` removes every row of the new tables. Check: G-031 on a throwaway copy with the fixture lines below; each new table left out is reported
- [x] The door's questions: rows, history tabs with unread counts, settings, preview, updates with `after` per reader. Check: `TestAnswersFitTheirSchemas`, `DiffUpdates` (a screen playing the updates ends equal to the rows answer)
- [x] The feed never calls Conversations: a lint, and the real program rebuilt and answering with Conversations stopped. Check: `feed-apart`

## Verification plan

- Automated: the four guardrail folders, each with must-fail variants under the build tag `feedfaults`.
- Real system: `TestRealDeliveryReachesTheFeed` (real Conversations, delivery worker and feed) and one mention followed through to the sender by hand.
- Independent: none here; the feed is in the scope of Milestone 1 (T-057).
- Not covered: see "Not checked".

## How it stores what the rules decide

`feed_journal` is the feed's copy of each change it took, with what became of it (applied, counted, set aside, fenced). The rules' state in memory is only a cache of it: a restarted or second copy replays the journal, and a copy that is behind catches up inside the transaction that holds the organization's row lock. Rows, counts, held entries and row updates are saved beside it for the door. Everything is removed by a wipe.

## Readings for the user

1. **A rebuild after a rule change needs a way to reach the feed.** The contract's wipe has an empty body. The feed accepts an optional `{"rules_changed": true}` on it. See "Requests for shared changes".
2. **What counts as "from before the rebuild"** is everything up to the feed's place when it was wiped. A notification held at that moment and released afterwards is sent as usual: it had not gone out.
3. **The history mark survives later ordinary rebuilds**, because a name listed as history must never be delivered later (D-034).
4. **A fence on one reader** sets aside that reader's own actions in that conversation. Other people's messages still reach their rows, marked "may be out of date", and nothing new notifies from them. An `update` for a notification already sent still goes out, since it only tells the truth about it.
5. **A text the note log cannot store** (a NUL character) is treated as a change the feed cannot apply, so it fences that conversation instead of stopping delivery.
6. **A change the feed cannot even read**, with no conversation named, stops that organization: no smaller fence can hold it.
7. **The sixth fence stops the organization and is not counted.** The place stays before it, so a rebuild after the repair takes it again.
8. **`preview` measures its 7 days back from the last change the feed took**, since the feed reads no clock for what it decides.

## Log

- 2026-11-22: read the contracts, decisions and rules. Chose a journal with the rules' state as a cache, over serialising the rules' state.
- 2026-11-22: **order of work.** The service and the tests were written side by side, not test first. Each check was then seen failing with its rule off (Evidence).
- 2026-11-22: the checks found five real faults. In the rules: a reaction did not reach other readers' rows; an edit missed rows showing the message through a reaction; "turn this thread off" on a direct conversation sent a thread level with no thread. In the service: a copy that caught up kept its old idea of the store, and failed on the next write.
- 2026-11-22: two must-fail variants passed at first because the test was too weak (a wipe hid a skipped change; two racing copies never had to learn from each other). Both tests were strengthened.

- 2026-11-23: merged `main` (T-053, T-054). By the coordinator's instruction the full `./scripts/verify` is left to Milestone 1.

- 2026-11-23: Merged by the lead. G-031's protected fixture gained one row for each of the feed's seven tables through the edit tool's protected-test gate; no assertion changed. The wipe contract gained the optional `rules_changed`.

## Evidence

Run after merging `main`:

- `./scripts/check`: run by the pre-commit hook on this file's commit.
- G-007, G-009, G-008 second half (`feed-apart`), G-027 for the feed, G-030: pass.
- Seen failing with the rule off (`-tags feedfaults`): any newer change accepted (26 differences); a fence leaving held entries (a notification sent from inside it); changes inside a fence applied; more than five fences not stopping; a copy not catching up from the journal (76 differences). G-030 with `g030fault` fails.
- A save dying before its commit, every third save: the feed still equals a clean fold. This one is an injected failure with no variant that breaks the transaction.
- G-031 on a throwaway copy with the fixture lines: passes, and each of the seven new tables left out of delete was reported (before the merge). The other three `retire-and-delete*` folders pass. The protected test fails on this branch until the lines are applied: "the fixture has no row in" one of the new tables.
- Rebuild of the seeded organization (7,957 changes) from nothing, through the feed's door in batches of 100: 905 ms.
- Real programs: a mention reached the feed in 0.06 s and the sender in 5.46 s, once; a mention read within the hold was not sent.

## Requests for shared changes

1. **G-031 fixture**, in `fill` of `tests/guardrails/retire-and-delete/retire_delete_test.go`, before the `search.state` line:

```go
`INSERT INTO feed.feed_state (org, edition, through) VALUES ($1, 1, 2)`,
`INSERT INTO feed.feed_journal (org, number, disposition, body) VALUES ($1, 1, 'applied', '{}')`,
`INSERT INTO feed.feed_rows (org, person, row_key, section, last, body) VALUES ($1, 'marco', '#launch/th/msg_1', 0, 1, '{}')`,
`INSERT INTO feed.feed_readers (org, person, number, counts) VALUES ($1, 'marco', 1, '{}')`,
`INSERT INTO feed.feed_held (org, name, person, conversation, message, item, at) VALUES ($1, 'n1', 'marco', '#launch', 'msg_1', 'msg', '2026-11-22T10:00:00.000Z')`,
`INSERT INTO feed.feed_row_updates (org, number, person, after, parts) VALUES ($1, 1, 'marco', 0, '[]')`,
`INSERT INTO feed.feed_fences (org, conversation, person, number, reason) VALUES ($1, '#launch', '', 1, 'test')`,
```

2. **`scripts/verify`:** run `-tags feedfaults -run Fails ./guardrails/feed-equals-replay/ ./guardrails/feed-fence/ ./guardrails/feed-two-copies/`.
3. **Contract:** give the wipe an optional `rules_changed` (`operations-and-control.md`, `contracts/operations.json`, schema). T-059's rebuild must send it.
4. **`docs/guardrails.md`, `.claude/settings.json`:** G-007 and G-009 can be set to `enforced`; G-008 should also name `tests/guardrails/feed-apart`, G-027 `tests/guardrails/feed-two-copies`.

## Not checked

- The full `./scripts/verify`, including browser and timed guardrails (Milestone 1).
- G-027 with two feed copies behind the real delivery workers and the harness router: the copies here are driven by the test.
- The door (T-053) reading this feed through a screen.
- The time a cold copy takes to replay the seeded organization, on its own.
- The journal is never trimmed: it grows with the log.
- The three rule fixes have no table test of their own; the replay and schema checks found them and now hold them.
