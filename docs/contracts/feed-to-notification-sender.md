# Activity feed to Notification sender

From D-018, D-012, D-007, D-022. Accepted 2026-11-22. Tried in E-003.

## 1. Parties

A delivery worker reading the feed's own numbered log calls the notification sender (a stand-in behind a real boundary). The sender's list is shown in the separate mock notification app (D-022), which reads the sender directly. The product itself shows no notifications.

## 2. Events

Every call is made for one organization (D-026). It is in the address of every call a screen, an agent or the door makes (`/orgs/acme/…`) and in every item a worker delivers; a live connection is opened for one organization. All numbers, places and logs count within that organization. Nothing of another organization is ever sent or accepted.

```json
POST /notes
{ "org": "acme", "number": 1286, "previous": 1285, "edition": "feed-7c1", "at": "2026-11-22T10:14:03.120Z", "kind": "due",
  "name": "acme / marco / #launch / 4812", "person": "marco", "bucket": "needs_you",
  "why": "Anna mentioned you in #launch", "text": "@Marco can you sign off the pricing page?" }
{ "org": "acme", "number": 1301, "previous": 1300, "edition": "feed-7c1", "at": "…", "kind": "changed",   "name": "acme / marco / #launch / 4812", "text": "…new text…" }
{ "org": "acme", "number": 1317, "previous": 1316, "edition": "feed-7c1", "at": "…", "kind": "withdrawn", "name": "acme / marco / #launch / 4812", "because": "read" }
{ "org": "acme", "number": 1318, "previous": 1317, "edition": "feed-7c1", "at": "…", "kind": "tick" }

answers: 200 taken · 200 already had it · 409 { "i_am_at": 1285 } · anything else, or 3 s of silence: down
```

A row that moves up into Focus while unread gets a second "due" with its own name (D-012). The "Focus only" switch is applied by the feed before it writes "due".

## 3. Guarantees

- The sender never judges: it sends what is due and nothing else.
- A name begins with the organization, so two organizations can never produce the same one. A name is sent once, also after restarts and after the feed is rebuilt (a new edition of that organization's log).
- The sender keeps a place per organization.
- A notification is held for 5 seconds, measured by the timestamps in the notes, not by any machine's clock: it is sent only once a note of the same organization stamped later than "due plus 5 s" has arrived. The feed passes the clock on while that organization has a notification on hold. So nothing is sent for something read or deleted within the hold, however late the feed or the sender are.
- One already sent cannot be recalled; its entry then shows the current truth ("message deleted", read, the new text).
- Every notification that is due and not withdrawn within its hold is sent. When everything is healthy it goes out within 8 seconds of becoming due (5 s hold, then the next change or the clock tick at 6 s, then delivery). Never lost; may be late. One sent more than 10 minutes late is marked "delayed".

## 4. Failure behaviour

- Sender down: notes wait in order; the same waiting and retrying as every other delivery.
- Feed down: no notes are written, so notifications wait until it is back.
- Sender wiped: it answers with its place and is refilled from the start of the feed's log. Anything due more than 10 minutes ago is listed as history without being sent again. This is possible because the stand-in's list is its own store; a real provider's delivery history is not ours to wipe.
- The sender does not refuse notes; an answer other than the three above counts as down.
- With a real push provider "sent once" cannot be promised across a crash between handing over and noting it; the rule there is "rather twice than never", passing the name as the provider's duplicate key where it takes one.

## 5. Evolution

Fields may be added to a note. A name never changes its meaning.

## 6. Enforced by

G-010, G-011, G-020, G-023, G-025, G-026, G-027.
