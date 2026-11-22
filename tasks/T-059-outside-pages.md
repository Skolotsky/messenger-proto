# T-059 (first part): The two pages outside the product

- Traces to: brief (activity feed and delivery to independent systems, made observable), D-010, D-022, D-024, D-032, D-033, D-034; contracts `operations-and-control.md`, `feed-to-notification-sender.md`
- Started: 2026-11-22

## Outcome

Someone explaining the product live can run the demonstration and break its systems from the control panel, and watch what one person was actually sent in a separate notification app. Both run against typed fixtures until the demo program exists.

## Acceptance criteria

- [ ] Each control of the panel calls the operation the contract names. Check: `control-panel.spec.ts`, "calls"
- [ ] Delete and Delete all retired ask first; the current organization cannot be deleted, also when the server refuses. Check: `control-panel.spec.ts`, "organizations"
- [ ] Every failure answer shows in place, next to the control that caused it. Check: `control-panel.spec.ts`, "failures"
- [ ] Reset shows its progress and lands on the fresh organization. Check: `control-panel.spec.ts`, "reset"
- [ ] The notification app lists what was sent, newest first, with delayed and changed entries, and never a held one. Check: `notification-app.spec.ts`
- [ ] The empty state says nothing has been sent yet. Check: `notification-app.spec.ts`
- [ ] Both pages work at 1280 and 400 px with no sideways scroll, light and dark. Check: both specs in both projects, screenshots reviewed

## Verification plan

- Automated: Playwright in Docker against the built pages, with the real HTTP calls answered by each page's fixture server, so paths, methods and bodies are what is asserted.
- Real system: not available; the demo program does not exist yet.
- Independent: at the T-059 milestone, with the real demo program.
- Not covered: the real demo program, real parts, the real shape of anything listed under "Shapes assumed".

## Shapes assumed

## Readings for the user

## Requests for shared changes

## Log

- 2026-11-22: started. The schema already defines the script, status, parked and notification shapes; the words in the contract stay the authority.

## Evidence
