# Conversations' door reading from the feed and the search index

From D-005, D-017, D-020, D-016, D-007. Accepted 2026-11-22. Not tried in an experiment.

## 1. Parties

Conversations' door for people (and its live connection) calls the activity feed and the search index to answer a screen. The feed and the index never call back. The screen never calls either directly.

## 2. Operations

Every call is made for one organization (D-026). It is in the address of every call a screen, an agent or the door makes (`/orgs/acme/…`) and in every item a worker delivers; a live connection is opened for one organization. All numbers, places and logs count within that organization. Nothing of another organization is ever sent or accepted.

All are questions; none changes anything.

```json
GET feed/orgs/acme/people/marco/rows                     → { "edition": 7, "through": 4861, "number": 103, "rows": [ { "conversation": "#launch", "thread": null, "bucket": "needs_you", "unread": true, "kept": false, "why": "Anna mentioned you", "newest": 4812 } ], "counts": { "focus": 1, "needs_you": 4, "fyi": true } }
GET feed/orgs/acme/people/marco/history?tab=mentions     → entries with an unread mark each, and the tab's unread count
GET feed/orgs/acme/people/marco/settings                 → Focus people, removed people, each thread's and channel's level as it is in effect
GET feed/orgs/acme/people/marco/preview?change=…         → what the change would have moved in the last 7 days
GET feed/orgs/acme/updates?after=103&edition=7                        → the organization's row changes after 103, a page at a time, each `{ number, person, after, parts }` with `after` the number before it for that reader; or { "new_edition": 8 }
GET search/orgs/acme/query?q=pricing&as=marco&after=…     → { "through": 4855, "total": 37, "next": "…", "results": [ { "message": "msg_7Q2", "version": 2, "conversation": "#launch", "thread": null, "author": "anna", "edited": true, "text": "…", "match": [[34, 41]] } ] }
```

`through` is the number up to which the answering part has applied everything. `number` in the rows answer is the last number of the feed's row log for this person. The feed keeps two numbered logs under one edition: row updates (read here) and notification notes (`feed-to-notification-sender.md`). The door measures how far behind an answer is from that receiver's delivery place in the log, which also passes changes the receiver is not sent, so a quiet channel never looks late.

## 3. Guarantees

- An answer includes everything up to `through`, except rows under a fence, and possibly later changes (the feed may work on conversations side by side, D-030).
- A search answer holds only what `as` may see: every channel message, and a direct message only if `as` is one of its participants (D-031). It gives the true total, not the number shown, marks where the text matched, says whether a message was edited, and names the thread of a reply so the screen can open it there (D-024; T-038, points 20 to 22).
- The feed's row updates are numbered by its own log and carry its edition; after a rebuild the edition changes and the door tells screens to load the feed fresh.
- The door adds nothing and remembers nothing: what it shows is what the part answered, or a notice.
- The feed never calls Conversations. The door learns that the feed has news from Conversations' own delivery worker, which nudges the organization's copy when the feed has confirmed a change; the nudge carries no data and may be lost. The door also asks every 5 seconds for each organization it has open screens for. It asks by organization, never per person and never with a call held open, so any copy of the feed can answer, and the cost follows the organizations that have screens open and the changes they have.

## 4. Failure behaviour

- No answer within 3 seconds, or an error: the door answers the screen at once with a notice instead of the content ("Search is unavailable right now. Your messages are safe." / the rows it last showed, with "Catching up"). Sending, reading and channel unread marks do not depend on these calls.
- More than 10 seconds behind (the newest change the receiver is sent against its delivery place, by the log's timestamps): the answer is shown with "the last N minutes may be missing" or "Catching up".
- A fenced conversation or reader is reported in the rows as "may be out of date".

## 5. Evolution

Fields may be added. `through`, `edition` and `number` keep their meaning.

## 6. Enforced by

G-008, G-012, G-019, G-023, G-025, G-026, G-029, G-027.
