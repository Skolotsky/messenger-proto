# T-059 (first part): The two pages outside the product

- Traces to: brief (activity feed and delivery to independent systems, made observable), D-010, D-022, D-024, D-032, D-033, D-034; contracts `operations-and-control.md`, `feed-to-notification-sender.md`
- Started: 2026-11-22

## Outcome

Someone explaining the product live can run the demonstration and break its systems from the control panel, and watch what one person was actually sent in a separate notification app. Both run against typed fixtures until the demo program exists.

## Acceptance criteria

- [x] Each control of the panel calls the operation the contract names. Check: `control-panel.spec.ts`, "calls"
- [x] Delete and Delete all retired ask first; the current organization cannot be deleted, also when the server refuses. Check: `control-panel.spec.ts`, "organizations"
- [x] Every failure answer shows in place, next to the control that caused it. Check: `control-panel.spec.ts`, "failures"
- [x] Reset shows its progress and lands on the fresh organization. Check: `control-panel.spec.ts`, "reset"
- [x] The notification app lists what was sent, newest first, with delayed and changed entries, and never a held one. Check: `notification-app.spec.ts`
- [x] The empty state says nothing has been sent yet. Check: `notification-app.spec.ts`
- [x] Both pages work at 1280 and 400 px with no sideways scroll, light and dark. Check: both specs in both projects, screenshots reviewed

## Verification plan

- Automated: Playwright in Docker against the built pages, with the real HTTP calls answered by each page's fixture server, so paths, methods and bodies are what is asserted.
- Real system: not available; the demo program does not exist yet.
- Independent: at the T-059 milestone, with the real demo program.
- Not covered: the real demo program, real parts, the real shape of anything listed under "Shapes assumed".

## Shapes assumed

Each lives in the page's `src/api.ts`.

1. Both pages reach the parts at `/ops/<part>/...` on their own address. No route exists yet.
2. `demo/organizations` lists an organization a running Reset is seeding with `state: "preparing"`. The contract names only current and retired, and has no progress call. Without this state, Reset still works and shows its steps and elapsed time, but no counts.
3. `state` of a listed organization is `current` or `retired` (the contract's example shows only `retired`).
4. `DELETE demo/organizations/<id>` and `POST demo/organizations/delete-retired` answer any 2xx on success; their bodies are ignored.
5. A script step's `who` is a person's ID, or `demo` for something played on demand; `what` reads as the rest of a sentence after the name. "Play now" uses `play_situation`, the only operation there is, so a step with `situation: null` has no button.
6. A notification entry may carry an optional `edited: true`. The contract overwrites `text` on an edit and has no way to say it changed, so without this field the new text shows with no "Edited" mark.
7. The person named in a notification comes from `why`: the entry has no author field, and the avatar letter is the first letter of `why`.
8. The people list is `PersonView` from the schema; `is_agent` people are not offered in the notification app.

## Readings for the user

1. Reset and Rebuild act at once, without asking: neither destroys anything (Reset retires the old organization, which stays listed), and a presenter should not meet a dialog mid-demo. Only Delete asks.
2. Agents are left out of the notification app's person list, because a notification stands for a person's phone.
3. A deleted message's original text is not shown in the notification app, following D-004, although the sender still holds it.
4. While the sender is Off or Slow its list does not answer in time, so the app says so and keeps the last list seen.

## Requests for shared changes

1. `contracts/`, `operations-and-control.md`: state how a Reset's progress is read (or accept `preparing`, assumption 2).
2. `contracts/`, `NotificationEntry`: add `edited` (assumption 6), and consider an author field (assumption 7).
3. Compose or the demo program: serve both pages and route `/ops/` to the parts, then build with `VITE_API=network`. Add links between the product, the panel and the notification app once their addresses exist.
4. `./scripts/check` or `./scripts/verify`: include `tests/browser-outside/run` and the two pages' type checks. Not wired in, by this package's limits.

## Log

- 2026-11-22: started. The schema already defines the script, status, parked and notification shapes; the words in the contract stay the authority.
- 2026-11-22: both pages built. Found while reviewing: a poll that overtook the previous one would have hidden a slow sender forever; polls for one person no longer overlap, and a test holds this.

## Evidence

- `tests/browser-outside/run`: 96 tests pass at 1280 and 400 px (48 per width), in Docker.
- 20 planted faults, one at a time. Each was confirmed applied (exactly one match), type-checked and built, then failed the test meant for it: wrong commands, paths and receivers; Delete and Delete all retired not asking; Delete offered on the current organization; failures hidden; refusal reported as repeatable; no Reset progress; a silent part looking healthy; phone overflow; held entries shown; wrong order; missing delayed mark; deleted text kept; stale edit; failed read shown as empty; polls overtaking; no empty state. Files restored and the suite passed again.
- Screenshots reviewed: panel and notification list at 1280 and 400 px in light and dark; empty state at 400 px in dark.
- Not verified: anything against a real demo program or real parts; the assumed shapes above; real phones; screen readers.
