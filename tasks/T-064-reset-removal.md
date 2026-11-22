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
- 2026-11-22: `reset` removed (bd1da42). Retire, stop, delete per part, `seed.sealed` and G-031 built (f2a2b9f).
- 2026-11-22: A rule reading the log in order cannot tell a seeded change from a live one, so the log now says it: `seed.opened` starts a seeded history and `seed.sealed` ends it. A session without the opener writes no seal, so G-005 and G-003 needed no change. The rules decide from the log alone.

## Evidence

- Search for `reset`: what remains is the `demo-reset` operation (`demo:/reset`, the contract's own name) and the panel-width reset in the web app.
- G-031 (`tests/guardrails/retire-and-delete`): seen failing with deletes that removed nothing; with a delete that skips `sender.notifications` ("rows of the deleted organization are left in [sender.notifications]"); with a delete that ignores the organization ("the other organization changed"); and for a new table with no delete. Passes with the real delete, with a crash after each of the seven steps and two repeats.
- Retired, rule off in a scratch copy: "start: want 410 organization_gone, got 200" and "a retired organization's log moved from 4 to 5".
- Stopped, rule off in a scratch copy: "delivered after the stop: search 5 to 34, feed 5 to 80, sender 60 to 80".
- Suppression, rule off in a scratch copy: `TestNoNotesUpToTheSeal` failed with "a seeded mention wrote notes".
- Seeded history: the new history without its opener, with every number moved back by one, hashes exactly to the previous pin.
- Protected tests `saved-once`, `log-numbering`, `change-and-log-together`, `conversations-independent`: no line changed.
