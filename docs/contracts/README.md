# Contracts

One file per boundary between services. A contract is what the user reviews instead of code, so it is written in plain language first and schemas second.

Each contract states:

1. **Parties**: who calls or publishes, who receives.
2. **Operations or events**: name, payload shape, one realistic example.
3. **Guarantees**: delivery, ordering, idempotency, and any latency or freshness target, with numbers.
4. **Failure behaviour**: what happens when the other side is slow, down or rejects the request, and what the caller sees.
5. **Evolution**: how the contract may change without breaking the other side.
6. **Enforced by**: the guardrail IDs from `docs/guardrails.md` whose checks hold this contract.

An accepted contract changes only through a decision (the `decide` skill). Note the decision ID next to the change.
