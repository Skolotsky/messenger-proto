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
- 2026-11-23: merged `main` at a27ef7c (T-052, T-053, T-054, T-056); every test now runs on the real feed.

## Findings

- **F-1, product defect in the feed (T-052), open.** When the first change of an organization reaches two feed copies at once, one copy answers 500 instead of 200 "already had it". Both read "no state yet", both pass the place check, the second waits in `createState`, gets the state the first just committed, and goes on to apply the change without checking its place again, so its journal insert hits the primary key (`server/internal/feed/service/take.go`, `takeIn`). Nothing is lost or applied twice, because the transaction rolls back. The contract counts a 500 as the feed being down for every organization, so the worker backs off for all of them. Seen in 60 of 60 rounds. Failing command: `tests/e2e/run real -v -run TestTheSameFirstChangeAtBothFeedCopies ./guardrails/two-copies/`

## Readings for the user

- G-013 "a 3 s delay": a delay of exactly 3 s equals the call limit, so it becomes an outage rather than a slow worker. The must-fail variant holds each delivery 2.9 s.
- G-013 "each receiver's own record": neither search nor the feed stores when it applied a change. The time is taken where the receiver confirms the delivery (a meter at its door), and the run also requires that the receiver reached the last change.
- G-025: the control panel's list of organizations is the operator's and is the stated exception, so it is not probed.
- G-027: "the fault tests' core scenarios" are read as: a burst with one organization moved; one copy each of workers, feed and Conversations killed; search and the sender down and back. Afterwards search equals the saved messages, the feed equals a clean replay, each due notification was handed over once, and screens equal a fresh load.
- The planted rule fault for the spine runs only on the stand-in, which uses the product's own rules package: the real feed has no switch for it from outside its process.

## Requests for shared changes

- `scripts/verify`: add `./guardrails/freshness` to the timed list (it runs for a minute), and add a must-fail step: `-tags t057faults -run Fails ./e2e/ ./guardrails/freshness/ ./guardrails/orgs-apart/ ./guardrails/two-copies/`. The spine's must-fail needs `CONTEXIA_E2E_FEED=standin`, otherwise it skips.
- `docs/guardrails.md` and `.claude/settings.json`: set G-013, G-025 and G-027 to enforced once F-1 is fixed and the tests pass (G-022).

## Evidence

Run on the real feed, merged at a27ef7c: `tests/e2e/run real -timeout 15m -tags t057faults -v ./e2e/... ./harness/router/ ./guardrails/freshness/ ./guardrails/orgs-apart/ ./guardrails/two-copies/`. It exits 1, for F-1 only.

- Spine: passes; the unread mention was sent about 5.3 s after its log timestamp.
- G-013: 3,000 changes in 60 s; 19 in 20 within 26 ms (search), 33 ms (feed), 8 ms (another screen). With each feed delivery held 2.9 s it fails at 55 s.
- G-025: passes for every door, live connections, calls naming the other's message, and 32 tables with row rules. The planted query finds 0 foreign rows with the rules on and 39 and 35 with them off.
- G-027: passes without faults, with a copy of each killed, and with search and the sender down and back. With the claim taken out it fails, with 1,057 repeats. `TestTheSameFirstChangeAtBothFeedCopies` fails: F-1.

Not verified:
- `./scripts/verify` was not run (brief for this task), so these tests have not run under its database settings.
- The spine's planted fault ran on the stand-in only.
- The door check of G-025 was seen failing by asking for the organization's own word, not through a planted leak in a product door.
- Search and the sender ran as one copy each in G-027.
- The independent verifier has not run.
