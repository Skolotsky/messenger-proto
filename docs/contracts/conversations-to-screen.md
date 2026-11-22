# Conversations to Screen: live updates

From D-005 (WebSocket with numbered updates), D-007. Accepted 2026-11-22. Tried in E-005.

## 1. Parties

Conversations' live connection sends; an open screen receives. One connection per screen.

## 2. Events

Every call is made for one organization (D-026). It is in the address of every call a screen, an agent or the door makes (`/orgs/acme/…`) and in every item a worker delivers; a live connection is opened for one organization. All numbers, places and logs count within that organization. Nothing of another organization is ever sent or accepted.

A connection is opened at `/orgs/acme/live` for one person of one organization. Everything of one organization (every call of its screens, their connections, its agents) is served by one copy of Conversations, chosen from the organization in the address (D-029); if that changes, the connection is closed and the screen reconnects and resumes by number. Updates for what the screen shows, in two streams: `conversations` (messages, reactions, thread summaries, unread marks; numbered by the change log) and `feed` (the person's rows and counts; numbered by the feed's row log, with the feed's edition). One number is one update: whatever a change means for this screen travels together as a list of `parts`. A screen is sent only what concerns it, so the numbers it sees have gaps. Each update therefore also says which number was sent to this screen before it (`after`). Text of a reply being written arrives as pieces outside the numbering.

```json
screen → { "resume": { "reset": 2, "conversations": 4855, "feed": 97, "feed_edition": 7 } }      feed and feed_edition are null if the feed was unavailable at load
screen → { "watching": { "conversation": "#launch", "thread": null, "open": true } }      "open": false when it stops
server → { "stream": "conversations", "number": 4859, "after": 4855, "parts": [
            { "kind": "message.changed", "message": { "id": "msg_7Q2", "version": 2, "text": "…" } },
            { "kind": "thread.summary", "conversation": "#launch", "thread": "msg_7Q2", "replies": 4 },
            { "kind": "unread", "conversation": "#launch", "thread": "msg_7Q2", "unread": true } ] }
server → { "stream": "feed", "number": 103, "after": 97, "edition": 7, "parts": [
            { "kind": "row.changed", "row": { "conversation": "#launch", "thread": null, "bucket": "needs_you", "unread": true, "why": "Anna mentioned you" } } ] }
server → { "last_sent": { "conversations": 4859, "feed": 103, "feed_edition": 7 } }       every 5 s
server → { "too_far_behind": "feed" }                                   the screen loads that view fresh
server → { "reset": 3 }                                                 the organization was Reset: the screen loads everything fresh
server → { "piece": { "message": "msg_9A1", "at": 120, "text": "ready. " } }
```

## 3. Guarantees

- A screen takes its position in a stream only from its first full load (`screen-to-conversations.md`). Loading another view never moves it.
- A screen applies an update only if its `after` equals the last number the screen applied in that stream. A number it already has is ignored; any other mismatch makes it ask again from its own number. The same holds for the regular `last_sent` message.
- If the feed was rebuilt (a new edition), the screen is told to load the feed view fresh.
- After any interruption the screen ends equal to a freshly loaded one.
- When connected, an update reaches the screen within 1 second. After reconnecting, a screen has caught up within 2 seconds. An update lost on a live connection is noticed and fetched within 20 seconds.
- Up to 500 updates per stream are replayed; further behind, the screen is told to load fresh.
- Pieces of a reply carry their position; a screen that sees a gap, or loads while a reply is in progress, fetches the text so far (`GET /orgs/acme/replies/msg_9A1`).
- The closed list of update parts and their payloads is in `contracts/schema/`.

## 4. Failure behaviour

- Connection lost or refused: the screen shows "Reconnecting", keeps what it shows, tries again after 0.1 s, doubling up to 1 s, and on success sends its numbers.
- An update lost on a live connection: noticed by the next update's `after`, or by the `last_sent` message sent every 5 s, and asked for again.
- Nothing here can lose data: the connection only forwards what is saved elsewhere.
- What the copy did not save itself (a row the feed changed, a clock tick or an ended "watching" written by housekeeping, a save finished by the copy that served the organization before a move) reaches it by a nudge from whoever wrote it on Conversations' side (the delivery worker once the feed has confirmed, or housekeeping), sent through the organization's address, and by a check every 5 seconds of the organizations it has open screens for. A nudge may be lost; with every nudge lost and one update lost on the connection as well, a screen is still right within 5 + 5 seconds plus the fetch, inside the 20 seconds promised above.

## 5. Evolution

New kinds of part may be added; a screen ignores a part it does not know but still counts the update's number.

## 6. Enforced by

G-015, G-019, G-025, G-026, G-029, G-027.

Note: E-005 tried this contract with numbers that had no gaps; the `after` field was added on the verifier's finding that a filtered stream always has gaps, and is tried with G-015's test. `parts`, the per-person starting number and the edition in `last_sent` come from D-031.
