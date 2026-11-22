# T-044: Foundation

- Traces to: brief (independent systems over the network), D-005, D-025, D-026, D-029, G-008, G-023, G-025, G-028
- Started: 2026-11-22
- Plan: `tasks/T-043-build-plan.md`

## Outcome

One command starts PostgreSQL and the six programs, each answering "alive". Every later package builds on the same shared pieces: the clock, the one client for calls between parts, and "the database for this organization".

## Acceptance criteria

- [x] `make up` starts PostgreSQL and conversations, delivery, feed, search, sender and demo, and each answers "alive". Check: `make verify`, smoke step.
- [x] A fresh clone runs `make up` and `make verify` with only Docker and make installed. Check: run from a `git clone` in the scratchpad, with Go and Node absent from the host.
- [x] Each part's login is refused on every other part's schema. Check: `tests/guardrails/parts-apart` (first half of G-008), seen failing with one shared login.
- [x] Calls between parts give up after at most 3 s, also against a server that accepts and never answers. Check: unit test of the shared client, seen failing without the limit.
- [x] Database access for an organization binds that organization to the session, and the binding does not leak to the next user of a pooled connection. Check: test against real PostgreSQL, seen failing without the binding.
- [x] Time is read through one injected clock that a test can move. Check: unit test.
- [x] `./scripts/check` stays fast and covers format, vet, unit tests and the existing repository checks; `./scripts/verify` runs the full suite. Check: plant a format error and a failing test, see each caught.

## Verification plan

- Automated: unit tests for the shared pieces; database tests on real PostgreSQL 17; the login half of G-008.
- Real system: `make up`, call each program's alive address, stop one and see it reported.
- Independent: no separate verifier; this task holds no product behaviour. Milestone 1 (T-057) verifies these pieces under load and faults.
- Not covered: the second half of G-008 (T-052); G-023's retry behaviour in workers (T-047); G-025 row rules on real tables (T-046, T-057); G-028 with two databases (T-057).

## Log

- 2026-11-22: The user asked for the whole plan to be implemented, T-044 to T-062, so T-045 does not wait for their T-034 review. A fresh verifier re-checks the T-034 corrections before the contracts become schemas.

- 2026-11-22: Internal choice: the delivery workers belong to the core store (`docs/visuals/D-005-where-state-lives.html`), so `delivery` has its own login, no schema of its own, and no table rights until T-047 grants them on the log tables only.
- 2026-11-22: `make` switches on the commit hook, because a fresh clone has it off and `./scripts/check` requires it.

## Evidence

2026-11-22, Go 1.24.13 and PostgreSQL 17 in Docker; the host has neither Go nor Node.

- Fresh `git clone`: `make up` started all parts, all six answered `/alive` on their ports, a stopped part stopped answering; `make verify` passed and left no containers behind.
- Seen failing, each planted in a throwaway clone: client without its time limit (test hangs and fails); organization bound to the session instead of the transaction (`still bound to "acme"`); feed given search's rights (`feed can [...] in schema search`); a format error and a failing unit test (`./scripts/check` fails, naming the fix).
- `./scripts/check` takes about 10 s; `./scripts/verify` about 30 s.

Not verified here: G-008 stays `pending` until its second half (T-052). Delivery's table-level rights on the log are granted and tested in T-046 and T-047.
