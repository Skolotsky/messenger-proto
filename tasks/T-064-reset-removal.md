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

## Evidence

_None yet._
