# Operations and control: running the system

From D-005, D-006, D-007, D-010, D-022, D-024, D-029, D-031. Accepted 2026-11-22 with D-031.

## 1. Parties

The delivery workers, housekeeping, the control panel and the notification app call the parts below. The control panel and the notification app are outside the product (D-022, D-024). The demo program (scripted team) acts only through the people door and the agent door; seeding is its one exception, below.

## 2. Operations

Every call is made for one organization, except where it says "all".

```json
POST conversations/orgs/acme/nudge      {}                                → 204     "this organization has news"; no data; may be lost

GET  feed/orgs/acme/place                → { "through": 4861, "edition": 7 }
GET  search/orgs/acme/place              → { "through": 4855, "wipe": 1 }
GET  sender/orgs/acme/place              → { "through": 1285, "edition": 7 }

GET  delivery/orgs/acme/status           → { "receivers": [ { "name": "search", "place": 4855, "waiting": 6, "state": "running", "parked": 1 } ] }
                                            state: running · down · paused · stopped
GET  delivery/orgs/acme/parked           → { "parked": [ { "receiver": "search", "number": 4830, "reason": "refused", "tries": 3 } ] }
POST delivery/orgs/acme/parked/4830/retry?receiver=search        → 200

POST {feed|search|sender}/orgs/acme/control/mode    { "mode": "running" }   running · slow · off      (also /control/mode with no organization: all)
POST {feed|search|sender}/orgs/acme/control/wipe                            → 200; the sender keeps its list of names sent
POST demo/orgs/acme/rebuild              { "receiver": "feed", "rules_changed": false }     wipe, set its place to the start, refill
POST demo/reset                                                             seed a fresh organization, wait until level, make it current, retire the previous one → { "org": "acme-8", "name": "Larkmere" }
GET  demo/current                                                           the entry address's answer → { "org": "acme-8", "name": "Larkmere" }
GET  demo/organizations                                                     → { "organizations": [ { "org": "acme-7", "name": "Larkmere", "state": "retired", "created_at": "…", "approx_changes": 7956 } ] }
DELETE demo/organizations/acme-7                                            refused for the current organization: 409 { "refused": "current" }
POST demo/organizations/delete-retired

POST conversations/orgs/acme-7/retire                                       no new actions; repeating it changes nothing
POST delivery/orgs/acme-7/stop                                              delivers nothing more for it; repeating it changes nothing
DELETE {feed|search|sender|delivery|conversations}/orgs/acme-7              removes every row of that organization; repeating it changes nothing

GET  sender/orgs/acme/notifications?person=marco
→ { "notifications": [ { "name": "acme / marco / #launch:msg_7Q2 / 4812", "person": "marco", "state": "sent", "at": "…", "delayed": false, "why": "Anna mentioned you in #launch", "text": "…", "now": "read" } ] }

GET  demo/orgs/acme/script               → { "running": true, "pace": 1, "happened": […], "next": […], "situations": […] }
POST demo/orgs/acme/script               { "do": "start" }        start · pause · pace · play_situation (with "pace" or "situation")

POST conversations/orgs/acme/seed/open   { "name": "Larkmere" }   → 200 { "session": "s1" }     the display name is optional; without it, the organization's ID
POST conversations/orgs/acme/seed/s1     { "changes": [ … ] }     → 200 { "through": 1500 }
POST conversations/orgs/acme/seed/s1/seal                         → 200 { "through": 1500 }

POST delivery/orgs/acme/receivers/feed/restart { "edition": 8 }   set that receiver's place to the start
GET  {feed|search|sender}/orgs/acme/control/mode                  → { "mode": "running" }
GET  conversations/orgs/acme/people                               → the people, for the notification app and the control panel
```

## 3. Guarantees

- A nudge carries nothing and may be lost; the 5-second check covers it.
- `place` tells a worker where to continue. A wiped receiver answers with its start.
- Wipe and Rebuild touch only the organization named, unless the call says "all". Reset and Delete touch no organization but their own.
- `slow` answers after 10 s; `off` does not answer: both are the failures the guardrails inject.
- The notification app shows only what the sender holds for that organization and person.
- **Seeding** is the only way to write a past timestamp. A session opens only on an organization with an empty log, takes ordinary changes in batches, each stamped with its past time, never decreasing, and is sealed once. A seeded history begins with a `seed.opened` entry, and sealing it writes `seed.sealed` and answers with that entry's number as `through`. Afterwards the log continues from the seed's last number and time, and no seeded change causes a notification (D-032).
- Reset never reuses a number: it seeds a new organization from the same seed under the same display name, and only the demo program knows which organization is current (D-033). If that is slow, the next one is prepared in the background.
- Delete runs in order: retire, stop delivery, then the receivers and the feed, the log and originals last. Each step can be repeated, so a crash halfway is repaired by deleting again. It touches no other organization. The current organization cannot be deleted.

## 4. Failure behaviour

- A part that is off or unreachable: the control panel shows that part as down and how much is waiting; the other parts carry on.
- Opening a seed session on an organization that already has changes, or writing to a sealed one: `409 { "refused": "not empty" }`.
- Reset while the script is running: the script stops on the old organization and starts on the new one.

## 5. Evolution

Operations may be added. These are for operators and tests; no person's screen depends on them, except the nudge, which only ever saves waiting.

## 6. Enforced by

G-010, G-014, G-020, G-023, G-025, G-026, G-029, G-031.
