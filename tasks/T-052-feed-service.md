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

## Evidence

Filled in with the final run; see the report of this task.

## Requests for shared changes

See the report of this task.

## Not checked

See the report of this task.
