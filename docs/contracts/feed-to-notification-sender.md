# Activity feed to Notification sender

From D-018, D-012, D-007, D-022. Accepted 2026-11-22. Tried in E-003.

## 1. Parties

A delivery worker reading the feed's own numbered log calls the notification sender (a stand-in behind a real boundary). The sender's list is shown in the separate mock notification app (D-022), which reads the sender directly. The product itself shows no notifications.

## 2. Events

Every call is made for one team (D-026). The team is in the address (`/teams/acme/…`) and in every item sent, and all numbers count within that team. Nothing of another team is ever sent or accepted.

```json
POST /notes
{ "team": "acme", "number": 1286, "previous": 1285, "edition": "feed-7c1", "at": "2026-11-22T10:14:03.120Z", "kind": "due",
  "name": "marco / #launch / 4812", "person": "marco", "bucket": "needs_you",
  "why": "Anna mentioned you in #launch", "text": "@Marco can you sign off the pricing page?" }
{ "team": "acme", "number": 1301, "previous": 1300, "edition": "feed-7c1", "at": "…", "kind": "changed",   "name": "marco / #launch / 4812", "text": "…new text…" }
{ "team": "acme", "number": 1317, "previous": 1316, "edition": "feed-7c1", "at": "…", "kind": "withdrawn", "name": "marco / #launch / 4812", "because": "read" }
{ "team": "acme", "number": 1318, "previous": 1317, "edition": "feed-7c1", "at": "…", "kind": "tick" }

answers: 200 taken · 200 already had it · 409 { "i_am_at": 1285 } · anything else, or 3 s of silence: down
```

A row that moves up into Focus while unread gets a second "due" with its own name (D-012). The "Focus only" switch is applied by the feed before it writes "due".

## 3. Guarantees

- The sender never judges: it sends what is due and nothing else.
- A name is sent once, also after restarts and after the feed is rebuilt (a new edition of its log).
- A notification is held for 5 seconds, measured by the timestamps in the notes, not by any machine's clock: it is sent only once a note stamped later than "due plus 5 s" has arrived. So nothing is sent for something read or deleted within the hold, however late the feed or the sender are.
- One already sent cannot be recalled; its entry then shows the current truth ("message deleted", read, the new text).
- Every notification that is due and not withdrawn within its hold is sent. When everything is healthy it goes out within 8 seconds of becoming due (5 s hold, a clock tick every second, delivery). Never lost; may be late. One sent more than 10 minutes late is marked "delayed".

## 4. Failure behaviour

- Sender down: notes wait in order; the same waiting and retrying as every other delivery.
- Feed down: no notes are written, so notifications wait until it is back.
- Sender wiped: it answers with its place and is refilled from the start of the feed's log. Anything due more than 10 minutes ago is listed as history without being sent again. This is possible because the stand-in's list is its own store; a real provider's delivery history is not ours to wipe.
- The sender does not refuse notes; an answer other than the three above counts as down.
- With a real push provider "sent once" cannot be promised across a crash between handing over and noting it; the rule there is "rather twice than never", passing the name as the provider's duplicate key where it takes one.

## 5. Evolution

Fields may be added to a note. A name never changes its meaning.

## 6. Enforced by

G-010, G-011, G-020, G-023, G-025, G-026.
