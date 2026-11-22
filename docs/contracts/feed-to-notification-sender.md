# Activity feed to Notification sender

From D-018, D-012, D-007, D-022, D-034. Accepted 2026-11-22. Tried in E-003; the hold moved into the feed by D-034.

## 1. Parties

A delivery worker reading the feed's numbered log of notification notes (a log separate from its row updates, under the same edition) calls the notification sender (a stand-in behind a real boundary). The sender's list is shown in the separate mock notification app (D-022), which reads the sender directly. The product itself shows no notifications.

## 2. Events

Every call is made for one organization (D-026). It is in the address of every call a screen, an agent or the door makes (`/orgs/acme/…`) and in every item a worker delivers; a live connection is opened for one organization. All numbers, places and logs count within that organization. Nothing of another organization is ever sent or accepted.

```json
POST /notes
{ "org": "acme", "number": 1286, "previous": 1285, "edition": 7, "at": "2026-11-22T10:14:09.120Z", "kind": "send",
  "name": "acme / marco / #launch:msg_7Q2 / 4812", "row": { "conversation": "#launch", "thread": "msg_7Q2" }, "person": "marco", "bucket": "needs_you",
  "why": "Anna mentioned you in #launch", "text": "@Marco can you sign off the pricing page?", "due_at": "2026-11-22T10:14:03.120Z" }
{ "org": "acme", "number": 1301, "previous": 1300, "edition": 7, "at": "…", "kind": "update", "name": "acme / marco / #launch:msg_7Q2 / 4812", "text": "…new text…" }
{ "org": "acme", "number": 1317, "previous": 1316, "edition": 7, "at": "…", "kind": "update", "name": "acme / marco / #launch:msg_7Q2 / 4812", "now": "read" }

answers: 200 taken · 200 already had it · 409 { "i_am_at": 1285, "edition": 7 } · 422 I refuse this one · anything else, or 3 s of silence: down (for every organization)
```

`send` delivers a notification: the final text, its fixed name, and `due_at`, when it became due. With `history: true` it is listed and never delivered. `update` carries what is true now for a name: `text`, `now` (`read` or `deleted`), or both. Either note may carry `skip: true`, sent only by the delivery worker: "step over this parked note, unapplied".

A name is the organization, the person, the row and the number of the change that caused it; a thread's row is written `#launch:msg_7Q2`. A row that moves up into Focus while unread gets a second notification with its own name (D-012). The "Focus only" switch is applied by the feed.

## 3. Guarantees

- **The feed holds, the sender delivers (D-034).** When a row becomes due, the feed keeps a held entry with the row, in the same transaction. A read, a delete, a fence, a mention edited away, a reaction taken back or a dial change removes it, and nothing is written to this log for it.
- A clock tick from the change log releases every held entry more than 5 s old by the log's timestamps: the feed writes one `send`, with what is true at that moment. Only a tick releases: no other change does, and no machine's clock is read. So nothing is sent for something read or deleted within the hold, however late the feed or the sender are, and when a clock steps back.
- An entry read or deleted after its 5 s but before the tick that would release it is removed too: nothing is sent for something already read.
- The sender never judges and holds nothing: it delivers each name once and keeps its list of names. A name begins with the organization, so two organizations can never produce the same one. A name is sent once, also after restarts, after the feed is rebuilt (a new edition of that organization's log) and after the sender is wiped.
- The sender keeps a place per organization. A note from a higher edition resets that organization's place to 0; the names on the list stay; a note from a lower edition is answered `409` with the sender's edition. The worker keeps the edition with its place and restarts at 0 when it changes (D-032).
- One already sent cannot be recalled; an `update` makes its entry show the current truth (read, message deleted, the new text). A delete wins over anything later, and a newer text over an older one tried again later. An `update` for a name the sender does not know yet is remembered and takes effect when its `send` arrives.
- Every held entry that nothing removed is sent. When everything is healthy it goes out within 8 seconds of becoming due (5 s hold, the next tick about a second later, then delivery). Never lost; may be late. One handed over more than 10 minutes after `due_at` is marked "delayed", measured from the note's `due_at` to the later of the note's own time and the moment of hand-over.

## 4. Failure behaviour

- Sender down: notes wait in order; the same waiting and retrying as every other delivery.
- Feed down: no notes are written, so notifications wait until it is back.
- Sender wiped: the list of names survives, each with when it was sent and its "delayed" mark (D-031, D-032), as a real provider's delivery history would. The sender answers with its place and is refilled from the start of the feed's log; a name on the list is not sent again, and one that was never sent goes out, marked "delayed" if more than 10 minutes late.
- "I refuse this one" three times: the note is parked, shown, and can be sent again. Later notes of that organization carry on: no later note can release a hold, because the hold is in the feed (D-034). The worker steps over the parked note with `skip`; sent again later, it takes effect without moving the place. Other organizations are not affected.
- Seeded history writes no notes (D-032): the feed holds nothing for anything before the log's `seed.sealed` entry (D-033). A rebuild made because the rules changed marks its `send` notes `history: true`: listed, never delivered, and a name listed as history is never delivered later (D-034). An ordinary rebuild keeps "never lost".
- With a real push provider "sent once" cannot be promised across a crash between handing over and noting it; the rule there is "rather twice than never", passing the name as the provider's duplicate key where it takes one.

## 5. Evolution

Fields may be added to a note. A name never changes its meaning.

## 6. Enforced by

G-010, G-011, G-014, G-020, G-023, G-025, G-026, G-027.
