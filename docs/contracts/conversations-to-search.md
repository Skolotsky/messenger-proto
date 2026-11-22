# Conversations to Search index

From D-006, D-007, D-004. Accepted 2026-11-22. Tried in E-003.

## 1. Parties

The search delivery worker (ours) calls the search index (a stand-in behind a real boundary). The web app searches through Conversations' door, never directly.

## 2. Events

Every call is made for one organization (D-026). It is in the address of every call a screen, an agent or the door makes (`/orgs/acme/…`) and in every item a worker delivers; a live connection is opened for one organization. All numbers, places and logs count within that organization. Nothing of another organization is ever sent or accepted.

Finished messages only: created, changed, deleted. Every message of every channel, also channels a person never opened, and of direct conversations. A direct message carries its `participants`; only they can find it (D-031). A reply closed as stopped is indexed with its text so far. A change carries the whole current message and its version; a delete carries which message and its version.

```json
POST /changes
{ "org": "acme", "number": 4830, "kind": "message.changed",
  "message": { "id": "msg_7Q2", "version": 2, "conversation": "#launch", "thread": null, "edited": true, "participants": null, "author": "anna",
               "text": "@Marco @Lena can you sign off the pricing page?", "mentions": ["marco", "lena"],
               "level": null } }
{ "org": "acme", "number": 4855, "kind": "message.deleted", "message": { "id": "msg_7Q2", "version": 3 } }

Every answer carries the index's `wipe` number for this organization. A changed number sends the worker back to the start (D-032).

answers: 200 applied · 200 already had it · 422 I refuse this one · anything else, or 3 s of silence: down (for every organization)
```

## 3. Guarantees

- Never lost once saved. May arrive more than once. Sent in log order, but the index must not rely on order.
- The index applies a change only if its version is newer than what it has, and remembers a delete by its version, so a late edit cannot bring a message back.
- Once level, the index holds the current text of every message that is not deleted, and nothing else.
- When healthy, a change is searchable within 2 seconds, 19 times in 20. After an outage of 10 minutes the index is level again within 30 seconds of coming back.

## 4. Failure behaviour

- Down (no answer in 3 s, a connection error, a server error): the worker waits and tries again after 1, 2, 4 … up to 60 s, each pause varied at random so retries do not arrive in step, without limit, and skips nothing.
- "I refuse this one" three times: the change is parked for that organization, shown in the control panel with the reason and "try again", and the worker moves on. Three different changes of one organization refused in a row pause that organization only: the worker keeps its place, shows it, and carries on with the others.
- While the index is unreachable a person sees "Search is unavailable right now. Your messages are safe."; while it is more than 10 s behind, results carry "the last N minutes may be missing".
- A place is kept per organization; workers find work through an index of the organizations that have some, claim an organization for a few seconds and deliver outside any transaction, so any number can run (D-029). A claim that runs out is taken over, so a change can arrive twice; the version rule makes that harmless.
- How long to wait before the next try at an index that is down is kept once per receiver and shared by all workers, so more workers do not mean more retries. How often a change was refused is kept with the organization's place.
- Wiped, for one organization or for all: its `wipe` number rises, and the worker, seeing it on the next answer or when it asks for the place (`operations-and-control.md`), goes back to the start and refills it.

## 5. Evolution

Fields may be added to a message. The version rule and the meaning of a delete do not change.

## 6. Enforced by

G-006, G-012, G-013, G-014, G-019, G-020, G-023, G-025, G-026, G-027, G-029.
