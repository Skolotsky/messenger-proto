# T-045: Contracts in machine form, and the tools packages build against

- Traces to: brief (independent systems), D-025, D-031, D-032, all contracts
- Started: 2026-11-22
- Plan: `tasks/T-043-build-plan.md`

## Outcome

Every operation and event of the eight contracts has a schema and a checked example, both languages agree with them, and each package can be built and tested without the others.

## Acceptance criteria

- [x] Every operation and event has a schema and at least one example. Check: a coverage list in `contracts/` compared by `./scripts/check`.
- [x] An example that breaks its schema fails `./scripts/check`. Check: plant one.
- [x] Changing a schema without regenerating the TypeScript types fails `./scripts/check`. Check: plant one.
- [x] Go types accept every example and round-trip it without losing a field. Check: Go test over `contracts/examples/`.
- [x] The log writer produces a valid log for one or two organizations from a seed, the same each time. Check: every entry validates against the schema; same seed, same bytes.
- [x] The mock door serves the people door and the live connection from fixtures. Check: its answers validate against the schemas.
- [x] The harness starts and stops parts, repeats, loses and delays deliveries, moves the clock, and runs two organizations with the same names and numbers. Check: its own tests against a recording receiver.

## Verification plan

- Automated: schema validation of all examples, generated-file check, Go round trip, harness self-tests.
- Real system: the mock door run and called; the harness run against the composed foundation.
- Independent: yes, a fresh verifier reviews the schemas against the contracts and decisions before Wave 1.
- Not covered: real behaviour of the parts (their packages).

## Log

- 2026-11-22: Started after D-031 and D-032. Shapes the contracts leave to the schemas: screen update parts, feed row, history entry, settings, rule vocabulary.

- 2026-11-22: Correction to commit 411ad1f: its message lists "Go type losing a field" as caught, but that planted fault did not apply (the edit missed the aligned code), so the check passed without testing it. It was rerun with a fault that applied; the result is under Evidence.

- 2026-11-22: Harness built: clock a test build can move (`/test/clock`, absent from normal builds), troublemaker, recording receiver, start/stop/kill/freeze of real programs. Still to build after the schema review: the log writer and the mock door; two-organization support comes with the log writer.

- 2026-11-22: Independent review of the schemas: 12 blockers, 12 gaps, 6 minor, all applied. One ruling that follows from accepted decisions rather than a new one: a top-level message addressing you is the row of its own thread (D-001, E-001), and opening its channel reads it because it is shown there, while unseen replies stay unread (D-032). Agent status travels outside the numbering, latest wins, as the agent door already said.

- 2026-11-22: Third review: 7 blockers and 7 minor, all applied (read rule for a thread row, Undo for Done, the row in examples and history, reply positions). The review rounds stop here: remaining doubts are tested by building against the schemas.
- 2026-11-22: Added for Wave 1 (lead-session shared files): each part applies its own migrations (`shared/migrate`), and the log and delivery tables are fixed in `server/migrations/conversations/001_log.sql` and `feed/001_notes.sql`, so T-046, T-047 and T-052 agree on them. Test: delivery has exactly its rights; row rules hold.

## Evidence

Schemas (2026-11-22, after three independent reviews): 206 types, 182 examples, 54 operations. `./scripts/check` validates every example, refuses each example with a required field removed or an unknown field added, round-trips each through its Go type, and compares the generated TypeScript.

Seen failing, each planted in a throwaway clone: an example breaking its schema; a bad timestamp; a schema changed without regenerating; a type without an example; a Go type losing a field (`unknown field "connection"`, on the rerun described in the log).

Tools (2026-11-22): the log writer's 3,000 entries each validate against the schema, chain by number and time, use every change kind, repeat byte for byte from a seed, and match across two organizations except for the name. The mock door's answers and live updates validate against the schemas; it was run and called from the host (`./scripts/mock-door`). Log writer and mock door tests passed 25 runs in a row. `./scripts/verify` passes.

Not verified: the fixes from the third independent review were applied and pass the automated checks, but no reviewer has read them; real use by the packages and the Milestone 1 verifier are their next test. The mock door sorts rows by a deliberately plain rule, not the product's.
