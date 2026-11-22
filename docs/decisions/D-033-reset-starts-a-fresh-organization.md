# D-033: Reset starts a fresh organization; the Reset number is removed from the product

- Status: accepted
- Level: contract
- Date: 2026-11-22
- Decided by: user
- Depends on: D-006, D-007, D-010, D-024, D-026, D-029, D-032
- Supersedes: the Reset parts of D-032 (the reset number, and restoring saved copies); Reset from a saved copy in D-007
- Narrows: D-006 (a retired organization), D-026 (the demo program may create, retire and delete organizations), G-025 (an operator list of organizations)
- Evidence: the footprint of `reset` across contracts and code, 2026-11-22; organizations are already fully separate (D-026)

## Why this needs deciding now

A demo button must not shape the contracts. `reset` is in every delivered item, every receiver and the live protocol only because Reset rewinds a log and reuses its numbers. Organizations are already fully separate, so Reset can start a fresh one and never reuse a number. T-052, T-053 and T-054 would otherwise build on it.

## Decision

The user's decision, 2026-11-22. D-010 stands: Reset still returns the team to the same starting state.

1. **Reset.** The demo program seeds a new organization from the same seed (an internal ID such as `acme-7`, the same display name), waits until search and the feed are level, makes it current and retires the previous one. Seeding creates no notifications. If this makes Reset slow, the next organization is prepared in the background so the switch is immediate.
2. **Current** is known only to the demo program. The web app's entry address gets it from there. No product program knows about "current" or about demos.
3. **A retired organization** accepts no new actions, and workers deliver nothing more for it. Doors and the agent door answer `410 { "organization_gone": true }`, and live connections are closed with the same message. A screen that receives it goes back to the entry address and loads the current organization. This is the only new product behaviour, and it is generic.
4. **Cleanup in the control panel.** A list of organizations (name, current or retired, created, approximate size), with Delete for one and Delete all retired, each asking first. The current organization cannot be deleted. Delete removes that organization's data from every part: delivery stopped first, then the receivers and the feed, the log and originals last. It is safe to repeat after a crash halfway and touches no other organization.
5. **Removed:** `reset` everywhere, `stale_reset`, the live `{ "reset": n }` message, and the saved copies that existed only for Reset. `wipe` and `edition` stay: rebuilding a receiver is a real production need.

## Readings the user confirmed ("Accept all five", 2026-11-22)

1. Retirement is the one deliberate exception to "nothing lost" (D-006, G-006, G-010): what was still undelivered, including notifications on hold, is dropped. Those guarantees apply until an organization is retired.
2. D-026 is narrowed: only the demo program and the control panel create, retire and delete organizations. The product still has no organization switcher or administration.
3. Reset from a saved copy in D-007 is superseded. Rebuild and its two-minute target stay.
4. G-025 allows the control panel's operator list: name, status, created time and approximate size, and no content.
5. One `seed.sealed` log entry closes a seed session. The feed writes no notifications for anything before it (D-032).

## Consequences

- Easier: delivered items, receivers and the live protocol lose a field and a failure case; numbers are never reused.
- Harder: retired organizations accumulate until deleted, so cleanup is a real operation across every part.
- `410` now has two bodies on the agent door (`organization_gone`, `too_far_behind`), so a client reads the body.

## Follow-ups

T-064 removes `reset`, builds retire and delete, and adds G-031. The demo program, the control panel and the screen's return to the entry address do not exist yet: pressing Reset with a screen open and cleanup in the real control panel are acceptance points of T-059, with T-056 handling `organization_gone` on the screen and T-053 on live connections.
