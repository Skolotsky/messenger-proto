# Operations and control: running the system

From D-005, D-006, D-007, D-010, D-022, D-024, D-029, D-031. Accepted 2026-11-22 with D-031.

## 1. Parties

The delivery workers, housekeeping, the control panel and the notification app call the parts below. The control panel and the notification app are outside the product (D-022, D-024). The demo program (scripted team) acts only through the people door and the agent door; seeding is its one exception, below.

## 2. Operations

Every call is made for one organization, except where it says "all".

```json
POST conversations/orgs/acme/nudge      {}                                → 204     "this organization has news"; no data; may be lost

GET  feed/orgs/acme/place                → { "through": 4861, "edition": "feed-7c1" }
GET  search/orgs/acme/place              → { "through": 4855 }
GET  sender/orgs/acme/place              → { "through": 1285, "edition": "feed-7c1" }

GET  delivery/orgs/acme/status           → { "receivers": [ { "name": "search", "place": 4855, "waiting": 6, "state": "running", "parked": 1 } ] }
                                            state: running · down · paused · stopped
GET  delivery/orgs/acme/parked           → [ { "receiver": "search", "number": 4830, "reason": "refused", "tries": 3 } ]
POST delivery/orgs/acme/parked/4830/retry?receiver=search        → 200

POST {feed|search|sender}/orgs/acme/control/mode    { "mode": "running" }   running · slow · off      (also /control/mode with no organization: all)
POST {feed|search|sender}/orgs/acme/control/wipe                            → 200; the sender keeps its list of names sent
POST demo/orgs/acme/rebuild              { "receiver": "feed" }             wipe, set its place to the start, refill
POST demo/orgs/acme/reset                                                   restores this organization's saved copies

GET  sender/orgs/acme/notifications?person=marco
→ [ { "name": "acme / marco / #launch / 4812", "state": "sent", "at": "…", "delayed": false, "why": "Anna mentioned you in #launch", "text": "…", "now": "read" } ]

GET  demo/orgs/acme/script               → { "running": true, "pace": 1, "happened": […], "next": […], "situations": […] }
POST demo/orgs/acme/script               { "do": "start" }        start · pause · pace · play_situation (with "pace" or "situation")

POST conversations/orgs/acme/seed        { "changes": [ … ] }     → 200 { "through": 1500 }
```

## 3. Guarantees

- A nudge carries nothing and may be lost; the 5-second check covers it.
- `place` tells a worker where to continue. A wiped receiver answers with its start.
- Wipe, Rebuild and Reset touch only the organization named, unless the call says "all".
- `slow` answers after 10 s; `off` does not answer: both are the failures the guardrails inject.
- The notification app shows only what the sender holds for that organization and person.
- **Seeding** is the only way to write a past timestamp. It is accepted only by an organization with an empty log, as ordinary changes in one save per batch, each stamped with its past time, never decreasing. Afterwards the log continues from the seed's last number and time.

## 4. Failure behaviour

- A part that is off or unreachable: the control panel shows that part as down and how much is waiting; the other parts carry on.
- Seeding an organization that already has changes: `409 { "refused": "not empty" }`.
- Reset while the script is running: the script is paused first.

## 5. Evolution

Operations may be added. These are for operators and tests; no person's screen depends on them, except the nudge, which only ever saves waiting.

## 6. Enforced by

G-010, G-014, G-020, G-023, G-025, G-026, G-029.
