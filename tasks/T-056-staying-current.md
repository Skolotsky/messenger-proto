# T-056: Web app: staying current and going wrong well (second half)

- Traces to: brief (real-time messaging, reliability, product craft), D-001, D-015, D-023, D-024, D-031, D-032, D-033, D-035, G-015, G-019; T-038 point 26; contracts `conversations-to-screen.md`, `screen-to-conversations.md`
- Started: 2026-11-22

## Outcome

A person's screen stays right through anything the connection does and ends equal to a fresh load. What they do without a connection waits in its place and goes out in order. A channel's and a thread's level change from inside the conversation.

## Acceptance criteria

Every browser check runs at 1280 and 400 px against the production build and the mock door.

- [ ] Numbered updates: apply only after the last number, ignore what is held, ask again otherwise; `last_sent`; `too_far_behind`; new edition; only a full load sets a position. Check: `live-rules`, `live-catch-up`
- [ ] Cut, refused, single lost updates, more than 500 away, another view opened with updates in flight, all nudges lost, retired organization: each ends equal to a fresh load. Check: `live-catch-up`
- [ ] Reconnect 0.1 s doubling to 1 s, varied, never above its step, with an injected random source. Check: `live-rules`
- [ ] Pieces of a reply, gaps fetched, `reply.abandoned`. Check: `live-catch-up`
- [ ] A message, edit and reaction wait in place and go out in order with their IDs; a refusal says so with Discard; Try again only where it can help; a failed Mark read shows as failed. Check: `live-waiting`
- [ ] `organization_gone` and a 404 for the organization go to the entry address and load the current one. Check: `live-catch-up`
- [ ] "Reconnecting", "Catching up", "may be out of date" in place, past 10 s, moving nothing. Check: `live-notices`
- [ ] The level control in a conversation; `read` for the right row; `watching` over the connection. Check: `level-control`
- [ ] The mock door follows the contract and its answers fit the schema. Check: `tests/harness/mockdoor`

## Verification plan

- Automated: Playwright through `web/app/scripts/browser-tests`; Go tests of the mock door against the schema.
- Real system: screenshots at both widths and themes.
- Independent: no; the milestone verifier comes at T-058.
- Not covered: the real server (T-053, T-058); Firefox and Safari; a real phone.

## Log

- 2026-11-22: started.

## Evidence

Filled in below when the checks have run.
