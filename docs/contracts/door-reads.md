# Conversations' door reading from the feed and the search index

From D-005, D-017, D-020, D-016, D-007. Accepted 2026-11-22. Not tried in an experiment.

## 1. Parties

Conversations' door for people (and its live connection) calls the activity feed and the search index to answer a screen. The feed and the index never call back. The screen never calls either directly.

## 2. Operations

Every call is made for one team (D-026). The team is in the address (`/teams/acme/…`) and in every item sent, and all numbers count within that team. Nothing of another team is ever sent or accepted.

All are questions; none changes anything.

```json
GET feed/people/marco/rows                     → { "edition": "feed-7c1", "through": 4861, "number": 103, "rows": [ { "conversation": "#launch", "bucket": "needs_you", "unread": true, "kept": false, "why": "Anna mentioned you", "newest": 4812 } ], "counts": { "focus": 1, "needs_you": 4, "fyi": true } }
GET feed/people/marco/history?tab=mentions     → entries with an unread mark each, and the tab's unread count
GET feed/people/marco/settings                 → Focus people, removed people, each thread's and channel's level as it is in effect
GET feed/people/marco/preview?change=…         → what the change would have moved in the last 7 days
GET feed/people/marco/updates?after=97&edition=feed-7c1&wait=25   → row changes after 97, each with the number before it; or { "new_edition": "feed-9d4" }
GET search/query?q=pricing&as=marco            → { "through": 4855, "results": [ { "message": "msg_7Q2", "version": 2, "text": "…" } ] }
```

`through` is the number of the last change the answering part has applied. The door compares it with the head of the log to know how far behind the answer is.

## 3. Guarantees

- An answer reflects the log up to `through`, exactly.
- The feed's row updates are numbered by its own log and carry its edition; after a rebuild the edition changes and the door tells screens to load the feed fresh.
- The door adds nothing and remembers nothing: what it shows is what the part answered, or a notice.

## 4. Failure behaviour

- No answer within 3 seconds, or an error: the door answers the screen at once with a notice instead of the content ("Search is unavailable right now. Your messages are safe." / the rows it last showed, with "Catching up"). Sending, reading and channel unread marks do not depend on these calls.
- More than 10 seconds behind (head of the log against `through`, by the log's timestamps): the answer is shown with "the last N minutes may be missing" or "Catching up".
- A fenced conversation or reader is reported in the rows as "may be out of date".

## 5. Evolution

Fields may be added. `through`, `edition` and `number` keep their meaning.

## 6. Enforced by

G-008, G-012, G-019, G-023, G-025, G-026.
