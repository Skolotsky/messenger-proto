# T-050: Web app: the frame, the look, a conversation

- Traces to: brief (messaging experience, product craft), D-009, D-019, D-023, D-024, T-038 points 24 and 25; contracts `screen-to-conversations.md`, `conversations-to-screen.md`
- Started: 2026-11-22

## Outcome

An installable web app with the agreed Slack-like look: sidebar of channels and direct messages, a channel with day lines and a message box, a thread beside the channel, resizable panels, the person's own menu with a person switcher, at 1280 and 400 px, light and dark, working against the mock door.

## Acceptance criteria

Every check is in `tests/browser/` and runs at 1280 and 400 px unless it says otherwise.

- [ ] The organization, channels and direct messages (by the other person's name) are shown; on a phone the first screen is Activity with a bottom bar. Check: `frame.spec.ts`
- [ ] A conversation shows a line between days; a sent message appears, here and on another open screen; Ctrl+Enter sends and Enter starts a new line. Check: `conversation.spec.ts`
- [ ] A send that fails keeps the text in the box and says so. Check: `conversation.spec.ts`, door refused and down
- [ ] No inline threads: a message with replies has a link under it; the thread opens beside the channel (1280) or in its place with a way back (400). Check: `thread.spec.ts`
- [ ] Panels resize by dragging and by keyboard, a double click resets them, and the page never scrolls sideways. Check: `layout.spec.ts`
- [ ] Activity can be closed and reopened on a wide screen. Check: `layout.spec.ts`
- [ ] The person's menu holds the organization, theme and Switch account one level down; switching asks, then shows a band with a way back. Check: `menu.spec.ts`
- [ ] Theme: System, Light or Dark, remembered, applied before first paint. Check: `menu.spec.ts`
- [ ] T-038 point 24: names, labels and messages are always escaped. Check: `safety.spec.ts`
- [ ] T-038 point 25: saving an empty edit says what happens and changes nothing. Check: `safety.spec.ts`
- [ ] Nothing jumps: an arrival, opening a thread, a toolbar and a level button move nothing being read (under 0.5 px, measured as in E-002). Check: `jumps.spec.ts`
- [ ] Installable: a valid manifest with icons, and a service worker that takes control and never caches the door. Check: `install.spec.ts`

## Verification plan

- Automated: Playwright Test in its official image against the production build and the real mock door, at both widths: `web/app/scripts/browser-tests`.
- Real system: the app run and looked at in screenshots at both widths and both themes.
- Independent: no; the milestone verifier comes at T-058.
- Not covered: the real server (T-058); Activity, settings, mentions, delete, waiting and failed message states, the numbered catch-up (T-055, T-056).

## Log

- 2026-11-22: started.

## Evidence

_Not yet._

## Requests for shared changes

_None yet._
