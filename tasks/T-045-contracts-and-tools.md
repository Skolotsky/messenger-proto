# T-045: Contracts in machine form, and the tools packages build against

- Traces to: brief (independent systems), D-025, D-031, D-032, all contracts
- Started: 2026-11-22
- Plan: `tasks/T-043-build-plan.md`

## Outcome

Every operation and event of the eight contracts has a schema and a checked example, both languages agree with them, and each package can be built and tested without the others.

## Acceptance criteria

- [ ] Every operation and event has a schema and at least one example. Check: a coverage list in `contracts/` compared by `./scripts/check`.
- [ ] An example that breaks its schema fails `./scripts/check`. Check: plant one.
- [ ] Changing a schema without regenerating the TypeScript types fails `./scripts/check`. Check: plant one.
- [ ] Go types accept every example and round-trip it without losing a field. Check: Go test over `contracts/examples/`.
- [ ] The log writer produces a valid log for one or two organizations from a seed, the same each time. Check: every entry validates against the schema; same seed, same bytes.
- [ ] The mock door serves the people door and the live connection from fixtures. Check: its answers validate against the schemas.
- [ ] The harness starts and stops parts, repeats, loses and delays deliveries, moves the clock, and runs two organizations with the same names and numbers. Check: its own tests against a recording receiver.

## Verification plan

- Automated: schema validation of all examples, generated-file check, Go round trip, harness self-tests.
- Real system: the mock door run and called; the harness run against the composed foundation.
- Independent: yes, a fresh verifier reviews the schemas against the contracts and decisions before Wave 1.
- Not covered: real behaviour of the parts (their packages).

## Log

- 2026-11-22: Started after D-031 and D-032. Shapes the contracts leave to the schemas: screen update parts, feed row, history entry, settings, rule vocabulary.

## Evidence

_None yet._
