# Conversations to Screen: live updates

From D-005 (WebSocket with numbered updates), D-007. Accepted 2026-11-22. Tried in E-005.

## 1. Parties

Conversations' live connection sends; an open screen receives. One connection per screen.

## 2. Events

Every call is made for one team (D-026). The team is in the address (`/teams/acme/…`) and in every item sent, and all numbers count within that team. Nothing of another team is ever sent or accepted.

Updates for what the screen shows, in two streams: `conversations` (messages, reactions, thread summaries, unread marks; numbered by the change log) and `feed` (the person's rows and counts; numbered by the feed's own log, with the feed's edition). A screen is sent only what concerns it, so the numbers it sees have gaps. Each update therefore also says which number was sent to this screen before it (`after`). Text of a reply being written arrives as pieces outside the numbering.

```json
screen → { "resume": { "conversations": 4855, "feed": 97, "feed_edition": "feed-7c1" } }
server → { "stream": "conversations", "number": 4859, "after": 4855, "kind": "message.changed", "message": { "id": "msg_7Q2", "version": 2, "text": "…" } }
server → { "stream": "feed", "number": 103, "after": 97, "edition": "feed-7c1", "kind": "row.changed", "row": { "conversation": "#launch", "bucket": "needs_you", "unread": true, "why": "Anna mentioned you" } }
server → { "last_sent": { "conversations": 4859, "feed": 103 } }       every 10 s
server → { "too_far_behind": "feed" }                                   the screen loads that view fresh
server → { "piece": { "message": "msg_9A1", "at": 120, "text": "ready. " } }
```

## 3. Guarantees

- A screen applies an update only if its `after` equals the last number the screen applied in that stream. A number it already has is ignored; any other mismatch makes it ask again from its own number. The same holds for the regular `last_sent` message.
- If the feed was rebuilt (a new edition), the screen is told to load the feed view fresh.
- After any interruption the screen ends equal to a freshly loaded one.
- When connected, an update reaches the screen within 1 second. After reconnecting, a screen has caught up within 2 seconds. An update lost on a live connection is noticed and fetched within 20 seconds.
- Up to 500 updates per stream are replayed; further behind, the screen is told to load fresh.
- Pieces of a reply carry their position; a screen that sees a gap fetches the text so far.

## 4. Failure behaviour

- Connection lost or refused: the screen shows "Reconnecting", keeps what it shows, tries again after 0.1 s, doubling up to 1 s, and on success sends its numbers.
- An update lost on a live connection: noticed by the next update's `after`, or by the `last_sent` message sent every 10 s, and asked for again.
- Nothing here can lose data: the connection only forwards what is saved elsewhere.

## 5. Evolution

New kinds of update may be added; a screen ignores a kind it does not know but still counts its number.

## 6. Enforced by

G-015, G-019, G-025, G-026.

Note: E-005 tried this contract with numbers that had no gaps; the `after` field was added on the verifier's finding that a filtered stream always has gaps, and is tried with G-015's test.
