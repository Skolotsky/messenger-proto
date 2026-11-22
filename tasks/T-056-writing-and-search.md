# T-056: Web app: writing and search (first half)

- Traces to: brief (messages, replies, mentions, editing and deleting; product craft), D-004, D-009, D-019, D-023, D-024, D-031, D-032; T-038 points 20 to 25; contracts `screen-to-conversations.md`, `door-reads.md`
- Started: 2026-11-22

## Outcome

A person can write with `@` suggestions, edit and delete their own messages in place, find any message with search and land on it, create a channel, start a direct message, and is told in place when the door refuses something. The live client's numbering, resume and catch-up, reconnect variation, the offline queue and `organization_gone` are the second half and wait for T-064.

## Acceptance criteria

Every check runs in Playwright at 1280 and 400 px against the production build and the mock door.

- [x] `@` suggests people and "channel", from the keyboard; the `mentions` sent match the text. Check: `writing-mentions.spec.ts`
- [x] Edit in place: nothing jumps, an empty edit is refused, removing a flag is an edit, a retry keeps its ID. Check: `writing-edit.spec.ts`
- [x] Delete asks first in the message's space; without replies nothing stays, with replies the marker and link stay; a delete seen arriving holds its space until the person moves on. Check: `writing-delete.spec.ts`
- [x] A channel is created with a name and people; a direct message is started once per pair. Check: `writing-new.spec.ts`
- [x] A refusal says so in place with Discard and no Try again; no answer offers Try again with the same ID. Check: the writing specs
- [x] T-038.20: the count is the true total, with paging. Check: `search-results.spec.ts`
- [x] T-038.21: a message in a channel never opened, or one the person is not in, is found and opens. Check: `search-results.spec.ts`
- [x] T-038.22: a hit on a reply opens that thread at that reply. Check: `search-results.spec.ts`
- [x] Marks use code points, "edited" shows, and "unavailable" and "may be missing" are said in place. Check: `search-results.spec.ts`

## Verification plan

- Automated: Playwright through `web/app/scripts/browser-tests`; answers the mock door cannot produce are fixtures checked against the schema at run time (`writing-fixtures.ts`, which has its own test that a broken answer is noticed).
- Real system: screenshots at both widths and both themes.
- Independent: no; the milestone verifier comes at T-058.
- Not covered: the real server (T-058); T-038.23 (the search index's and agent door's side); T-038.26 and the waiting state (second half).

## Log

- 2026-11-22: started.
- Seam for the second half: `live.ts` and the connection effect in `useSession.ts` are untouched and nothing is built on `reset`. A failed send stays in the message box until the waiting and failed states of a message exist.
- Internal: mentions are worked out from the text when sending or saving, so removing a name removes the mention. Search and suggestion failures never block writing.
- Found by looking at the phone picture: the "Not saved" line appeared below the edge of the list. It is now brought into view, with a check.
- Found by a planted fault: the first in-view check passed without the fix and failed with it by 0.3 px. The check now sets up the edge case, and the line keeps an 8 px margin.

## Evidence

- `web/app/scripts/browser-tests`, all specs: 164 passed, 16 skipped (the other width's tests and the picture writers), Chromium, production build, mock door, 1280 and 400 px.
- Seen failing: 21 planted faults, one at a time, each confirmed applied and compiling, each failing its check: mentions never sent; edit keeps old mentions; "channel" in a direct message; Enter does not pick; empty edit sent; Remove flag keeps the flag; edit retry with a new ID; refusal offers Try again; problem line out of view; arriving edit not held; delete does not ask; own delete leaves a marker; marker without the height; held marker survives moving on; count is what is shown; marks in UTF-16; reply hit opens the channel; read from an older page; unavailable shown as no results; older answer replaces a newer; hit outside one's channels does not open.
- Looked at: wide light (search, edit, new channel), wide dark (mentions, delete asking, search), phone light (own tools, refused edit), phone dark (search, new channel).
- Not verified: the real server; Firefox and Safari; a real phone.

## Readings for the user

1. **Your own longer edit shows in full** and moves what is below it, because you did it. Only a change arriving from elsewhere on a message in view holds its space behind "Show all" (D-004, as in the prototype).
2. **Your own delete without replies goes at once.** Someone else's, seen arriving, holds a marker until you move on (D-004).
3. **A search hit in a channel you are not in opens that channel** (D-009: everyone can see every channel). The message box stays there; whether a non-member may post is the door's answer.
4. **`@channel` is not offered in a direct message** and does not count there, since it would name nobody.
5. **You cannot mention yourself:** your own name is not suggested and does not count.
6. A first name is the mention; two people with the same first name are told apart by the full name with a hyphen (`@Anna-Lindqvist`).

## Requests for shared changes

1. Mock door: `around`, paging of search, and a thread hit are not produced, so those specs use fixtures. A delete of a reply sends no `thread.summary`, so a channel's reply count stays stale until reload; T-053 should confirm the real door sends it.
2. Mock door: `create_channel` adds the creator without removing a duplicate. The screen sends only the other people, which is right for both the mock and the real door.
3. Contract example: the search notice is "Catching up", while G-019 says "the last N minutes may be missing". The screen shows the door's words as they are, so T-053 should send the G-019 wording.
4. `GET /messages` takes `as`, which the screen does not send yet; thread summaries' `unread` depends on it. For the second half or T-053.
