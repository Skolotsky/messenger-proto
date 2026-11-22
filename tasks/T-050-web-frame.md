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
- Readings: T-038 point 25 needs an edit box, while editing belongs to T-056, so the smallest edit in place is built here (save, cancel, the empty case); T-056 extends `EditBox`. Opening a conversation sends `read` only while the window is in front, the newest message is in view and nothing covers it (D-015). `watching` is not sent yet (T-053, T-056). A send that fails keeps its text in the box with the same IDs; the waiting and failed message states are T-056.
- Internal: lines in the message list are a whole 22 px (the prototype's 1.45 gives 21.75), because a scroll position is whole pixels and holding a message in place was otherwise off by 0.5 px, which the "nothing jumps" check caught.
- Seams: `Activity.tsx` (T-055); `live.ts` `receive` for the numbered catch-up, `Composer` for `@` suggestions, `EditBox` for delete and Remove flag (T-056). Settings are shown disabled.

## Evidence

- `web/app/scripts/browser-tests`: 73 passed, 19 skipped (the other width's tests and the picture writer), Chromium, production build, real mock door, at 1280 and 400 px.
- Seen failing: 18 planted faults, one at a time, each caught by its spec: message, channel and person names as markup; empty edit silent; list not holding position; toolbar taking room; thread replacing the channel; replies inline; no day lines; Enter sends; failed send loses text; read while away; read while covered; drag dead; panel dragged out of the window; switch without asking; theme after first paint; service worker storing door answers. A first toolbar fault was missed, so the check now also measures the text's left edge and width.
- Looked at: wide light (thread, band), wide dark (channel, menu), phone light (first screen, channel, switch dialog), phone dark (thread, menu).
- Not verified: the real server; Firefox and Safari; a real phone or an actual install prompt (manifest and service worker are checked); contrast was not measured.

## Requests for shared changes

1. `scripts/verify`, with the browser tests: `web/app/scripts/browser-tests`
2. `scripts/check`, new section (fast, no browser):
   ```bash
   section "Web app"
   TOOL_DIR=web/app ./scripts/tool npm ci --no-audit --no-fund >/dev/null 2>&1
   if TOOL_DIR=web/app ./scripts/tool npm run typecheck >/dev/null 2>&1; then pass "web app types"; else fail "web app types. Run: TOOL_DIR=web/app ./scripts/tool npm run typecheck"; fi
   ```
3. `contracts/generate.mjs`: the generated `Message` is `{ [k: string]: unknown }` because of the schema's `allOf`. It should carry the fields; `web/app/src/api/types.ts` holds a hand-written `ChatMessage` until then.
4. Mock door, needed by T-056: (a) the first live update has `after: 0` or the previous global entry instead of the screen's own position, and `last_sent.conversations` is 0 until something was sent, so the contract's `after` rule would reject them; (b) updates go to every connection, unfiltered, including another person's `read.moved`; (c) it sends no `unread` part, so `lists.ts` derives the mark from arrivals as a stopgap; (d) `around` and `limit` are ignored.
5. Contract wording: the values of the `send_key` preference are not stated; the example has `enter`. Read here as: `enter` means Enter sends, anything else Ctrl+Enter.
6. Plan gap: no package names the reaction picker (D-023), creating a channel, or starting a new direct message (D-009, D-024).
