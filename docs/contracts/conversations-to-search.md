# Conversations to Search index

From D-006, D-007, D-004. Accepted 2026-11-22. Tried in E-003.

## 1. Parties

The search delivery worker (ours) calls the search index (a stand-in behind a real boundary). The web app searches through Conversations' door, never directly.

## 2. Events

Every call is made for one organization (D-026). The organization is in the address (`/orgs/acme/…`) and in every item sent, and all numbers count within that organization. Nothing of another organization is ever sent or accepted.

Finished messages only: created, changed, deleted. A change carries the whole current message and its version; a delete carries which message and its version.

```json
POST /changes
{ "org": "acme", "number": 4830, "kind": "message.changed",
  "message": { "id": "msg_7Q2", "version": 2, "conversation": "#launch", "author": "anna",
               "text": "@Marco @Lena can you sign off the pricing page?", "mentions": ["marco", "lena"],
               "level": null, "reactions": [] } }
{ "org": "acme", "number": 4855, "kind": "message.deleted", "message": { "id": "msg_7Q2", "version": 3 } }

answers: 200 applied · 200 already had it · 422 I refuse this one · anything else, or 3 s of silence: down
```

## 3. Guarantees

- Never lost once saved. May arrive more than once. Sent in log order, but the index must not rely on order.
- The index applies a change only if its version is newer than what it has, and remembers a delete by its version, so a late edit cannot bring a message back.
- Once level, the index holds the current text of every message that is not deleted, and nothing else.
- When healthy, a change is searchable within 2 seconds, 19 times in 20. After an outage of 10 minutes the index is level again within 30 seconds of coming back.

## 4. Failure behaviour

- Down (no answer in 3 s, a connection error, a server error): the worker waits and tries again after 1, 2, 4 … up to 60 s, each pause varied at random so retries do not arrive in step, without limit, and skips nothing.
- "I refuse this one" three times: the change is parked for that organization, shown in the control panel with the reason and "try again", and the worker moves on. Three different changes refused in a row count as down.
- While the index is unreachable a person sees "Search is unavailable right now. Your messages are safe."; while it is more than 10 s behind, results carry "the last N minutes may be missing".
- Wiped: the worker's place is set back to the start and the index refills.

## 5. Evolution

Fields may be added to a message. The version rule and the meaning of a delete do not change.

## 6. Enforced by

G-006, G-012, G-013, G-014, G-019, G-020, G-023, G-025, G-026.
