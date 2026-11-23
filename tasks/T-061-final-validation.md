# T-061: Final validation

- Traces to: the brief, D-033, D-034, D-035, every contract and guardrail
- Finished: 2026-11-23

## Outcome

The first version runs from `make up` and holds its guarantees when its parts fail.

## Evidence

- `./scripts/verify` on the final code: all 11 lanes passed in 4 min 8 s. The main flow passed in a browser on the real system (3 passed, 1 skipped: the cross-page flow runs at the wide width only).
- A fresh verifier ran the real system. With search, the feed, the sender and delivery stopped, slowed and killed during about 400 messages: search equalled the truth, the feed equalled a rebuild, all 263 due notifications were sent once, and none of the 132 read or deleted inside the hold was sent. Healthy notifications took at most 6.0 s. 13,734 writes to a retired organization were refused.
- It found one blocker, two majors and two minors. All are fixed; each fix was seen failing first.

## Not verified

- The fixes after the verifier's report were not re-read by an independent reviewer; they passed the full suite.
- Firefox, Safari, a real phone, the install prompt, measured colour contrast.
- `make up` with an empty Docker build cache.
- Conversations killed during a delivery run.
- A real ten-minute backlog for G-006's 30-second catch-up (tested with about 130 changes).
- G-015 and G-019 as named guardrail tests on the real system (T-066).
- Capacity: about 5,000 people in one organization is an estimate by analysis, not measured.

## Calls made without asking

Listed under "Delegated" in `docs/decisions/README.md`.

## Still the user's

T-034 (review of the contracts and guardrails) and T-021 (nine small feed behaviours).
