# T-064: Remove the Reset number; retire and delete organizations

- Traces to: D-033, D-010, D-026, G-015, G-020, G-025, G-031
- Started: 2026-11-22

## Outcome

No contract, schema or product code knows a Reset number. A retired organization answers 410 and receives nothing more, and an organization can be deleted from every part, safely and repeatably.

## Acceptance criteria

- [ ] No `reset` or `stale_reset` in product code, contracts or schemas. Check: the search, shown in Evidence.
- [ ] A retired organization accepts no action and workers deliver nothing more for it; doors answer `410 { "organization_gone": true }`. Check: integration tests.
- [ ] `seed.sealed` closes a seed session, and the feed rules write no notes before it. Check: rule test.
- [ ] Deleting one of two organizations, also when interrupted and repeated, leaves the other's rows, log entries and notifications unchanged and removes every row of the deleted one from every part. Check: `tests/guardrails/retire-and-delete` (G-031), seen failing.
- [ ] `tests/guardrails/saved-once` changes only by losing the `reset` key from its test data. Check: its diff.
- [ ] `./scripts/check` and `./scripts/verify` pass. A fresh verifier checks this against D-033.

## Moved to later tasks, because the parts do not exist yet

- T-053: live connections closed with `organization_gone`.
- T-056: a screen that receives it returns to the entry address.
- T-059: the demo program's Reset, current organization and entry address; pressing Reset with a screen open lands it on the fresh organization; cleanup exercised in the real control panel; a verifier checks those against D-033.

## Log

- 2026-11-22: Started after the user accepted D-033 and its five readings.
- 2026-11-22: `reset` removed (commit bd1da42). Retire, stop, delete per part, `seed.sealed` and G-031 built; staged, not committed.
- 2026-11-22: **Blocked, two things need a decision.** (1) `tests/guardrails/log-numbering` (G-005, protected) counts the log as `1 + (writers-1)*savesEach`; the `seed.sealed` entry makes it one more, so the test fails until that line says `2 +`. Not edited: it needs the user's approval (G-022). (2) `core.hooksPath` in the shared git config was changed to an absolute path at 21:53:56 by something outside this task, so `./scripts/check` fails its "hook is switched on" line and no commit can pass. Not changed here.
- Open for T-052: the feed must build its engine with `rules.NewFromSeed`. A rule cannot tell a seeded change from a live one while reading in order, so the choice is made once, when the engine is created.

## Evidence

- Search for `reset`: what remains is the `demo-reset` operation (`demo:/reset`, the contract's own name) and the panel-width reset in the web app.
- G-031 (`tests/guardrails/retire-and-delete`): seen failing with deletes that removed nothing; with a delete that skips `sender.notifications` ("rows of the deleted organization are left in [sender.notifications]"); with a delete that ignores the organization ("the other organization changed"); and for a new table with no delete. Passes with the real delete, with a crash after each of the seven steps and two repeats.
- Rule test `TestNoNotesUpToTheSeal`: passes. It was not seen failing on its own.
- Seeded history: the old history with its `reset` keys stripped hashes exactly to the new pin.
- Protected tests `saved-once`, `log-numbering`, `change-and-log-together`, `conversations-independent`: no line changed.
- Run by hand on a composed system: every guardrail test passes except G-005 (302 changes, expected 301). A scratch copy with the count raised by one passed 5 of 5. Must-fail builds, server integration tests and browser tests (71 passed, 19 skipped) pass.
- **Not run:** `./scripts/verify` as a whole, because its first step is the failing check.
