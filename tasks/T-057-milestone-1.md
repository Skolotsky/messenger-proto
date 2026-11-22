# T-057: Milestone 1, the spine runs end to end

- Traces to: brief (messages and their edits and deletes reach search, the notification sender and the feed), D-005, D-006, D-007, D-018, D-026, D-029, D-033, D-034; G-013, G-025, G-027
- Started: 2026-11-23

## Outcome

The real programs run together. What a person does at the door reaches search, the feed and the sender, with two organizations kept apart and with two copies of the workers, the feed and Conversations.

## Acceptance criteria

- [ ] A message, an edit, a delete, a mention read in the hold and a mention left unread reach each receiver as the contracts say; nothing is sent for the read one and the unread one is sent once within 8 s. Check: `tests/e2e`, `TestTheSpine`
- [ ] G-013: at 50 actions a second for 60 s, 19 in 20 changes are reflected by search and the feed within 2 s and on another open screen within 1 s. Fails with a delayed worker. Check: `tests/guardrails/freshness`
- [ ] G-025: two organizations with the same people, channels, message IDs and numbers see nothing of each other at any door. Check: `tests/guardrails/orgs-apart`
- [ ] G-027: two copies lose nothing, apply nothing twice, and deliver nothing twice without faults; an organization moved mid-burst ends equal to a fresh load. Fails with the claim taken out. Check: `tests/guardrails/two-copies`
- [ ] All of the above run on the real feed. Check: `tests/e2e/run real`

## Verification plan

- Automated: `tests/e2e/run standin|real` starts each real program as its own process on a private PostgreSQL.
- Real system: the same programs the product runs; the feed is the real one once T-052 is merged.
- Independent: a fresh verifier on the delivery guarantees, by the lead, after the real-feed run.
- Not covered: the web app (T-058), the demo program, a second database (G-028).

## Log

- 2026-11-23: harness, router and stand-in feed built while T-052 is in progress.

## Readings for the user

_Filled in as they arise._

## Requests for shared changes

_Filled in as they arise._

## Evidence

_Filled in before the task moves to done._
