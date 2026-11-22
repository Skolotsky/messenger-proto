# Guardrails

What must stay true while we build. Each guardrail comes from a decision and names the check that enforces it. A guardrail without a working check is `pending`, and work that depends on it does not start. How guardrails are formed: the `guardrails` skill.

Types: `quality` (a measurable target) · `contract` (what one service promises another) · `invariant` (a rule the data or behaviour never breaks) · `architecture` (a structural rule about boundaries) · `process` (how we work)

| ID | Type | Guardrail | From | Enforced by | Status |
|----|------|-----------|------|-------------|--------|
| G-001 | process | The brief (`docs/brief.md`) changes only with the user's approval | setup | `permissions.ask` rule in `.claude/settings.json` (not yet seen working: T-003) | pending |
| G-002 | process | Every commit passes `./scripts/check` | setup | git pre-commit hook in `.githooks/` | enforced |
