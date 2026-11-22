# Screen to Conversations: what a person does

From D-005, D-006, D-017, and the screens decided in D-009, D-020, D-021, D-023, D-024. Accepted 2026-11-22.

## 1. Parties

The web app (a person's screen), and the scripted team acting as people, call Conversations' door for people. Conversations answers.

## 2. Operations

Every call is made for one organization (D-026). It is in the address of every call a screen, an agent or the door makes (`/orgs/acme/…`) and in every item a worker delivers; a live connection is opened for one organization. All numbers, places and logs count within that organization. Nothing of another organization is ever sent or accepted.

Every action is one call carrying an ID the caller chose. Conversations saves it as a fact and writes it to the change log in the same step.

| Action | Says |
|---|---|
| send | a new message: conversation, optional thread (any channel message can start one), text of several lines, mentions (people, or "channel" for everyone, which names nobody), level (Flag, Info or none) |
| edit, delete | a change to the caller's own message; removing its flag is an edit |
| create channel, add people | a new channel with a name and people; more people later (D-009, D-024) |
| open direct | the direct conversation with one other person, created if there is none. Opening it twice gives the same conversation |
| react | add or remove one emoji on a message |
| read | "I have read this row up to here" (opening a conversation or thread, or Mark read on a row) |
| watching | "this row is open in front of me", and "no longer" when it stops. Not a call: said only over the live connection (`conversations-to-screen.md`); only the start and the end are saved as changes |
| keep, done | Keep a row; release it. Both name the row |
| dial | the level of a channel or of a thread |
| focus rule, focus person | add or remove a rule; remove a person from Focus or add them back; switch the option that adds everyone who writes directly |
| focus only | the personal notification switch (D-012) |
| release setting | whether a reply or a reaction releases a kept row (D-023, point 26). The feed acts on it |
| profile line | the person's own line under their name (D-023, point 29). No receiver acts on it |
| preference | a setting that only changes the person's own screens, such as which key sends a message (D-024). It follows the person to every screen; no receiver acts on it |

Questions, which change nothing. The first full load (`start` for the `conversations` stream, `feed` for the `feed` stream) answers with `number`, the last number of that stream for this person, read together with the content and, for the feed, the edition. Only that load sets the screen's position. Later views never move it: the screen keeps applying updates, by message version. If the feed is unavailable, `feed` answers with a notice and no number, and the screen loads it when it returns.

| Question | Answers with |
|---|---|
| `GET /orgs/acme/people/marco/start` | the person, their preferences and profile line, channels and direct conversations with unread marks, `number` for the `conversations` stream |
| `GET /orgs/acme/conversations/%23launch/messages?around=4812` (also `&thread=msg_7Q2`) | messages around a position, each with its thread summary (D-023), `number` |
| `GET /orgs/acme/people/marco/feed` | the feed's rows and counts, `edition`, `number` for the `feed` stream, or a notice |
| `GET /orgs/acme/people/marco/history?tab=mentions` | history entries with unread marks and the tab's count (D-016) |
| `GET /orgs/acme/people/marco/settings` | Focus people, removed people, levels in effect |
| `GET /orgs/acme/people/marco/preview?change=…` | what the change would have moved in the last 7 days (D-020) |
| `GET /orgs/acme/people/marco/search?q=pricing` | the search answer of `door-reads.md`, or a notice |
| `GET /orgs/acme/suggest?q=mar` | people and channels to suggest after `@` (D-024) |

```json
POST /orgs/acme/people/actions
{ "id": "act_01JD3K7Q2", "as": "anna", "kind": "send",
  "message": { "id": "msg_7Q2", "conversation": "#launch", "thread": null,
               "text": "@Marco can you sign off the pricing page?", "mentions": ["marco"], "level": null } }
→ 200 { "saved": true, "number": 4812 }

{ "id": "act_01JD3K9R1", "as": "marco", "kind": "read", "conversation": "#launch", "thread": null, "up_to": 4812 }
{ "id": "act_01JD3KB22", "as": "marco", "kind": "keep", "conversation": "#launch", "thread": "msg_7Q2" }
→ 409 { "refused": "unknown kind" }        for a kind Conversations does not know
→ 409 { "refused": "message deleted" }     an edit or reaction on a message that is gone; trying again cannot help

GET /orgs/acme/people/marco/preview?change=<the change as URL-encoded JSON>
→ 200 { "last_7_days": { "moved": 14, "from": "needs_you", "to": "fyi", "stay": 3 } }   or   503 { "not_available": "the feed is catching up" }
```

## 3. Guarantees

- "Saved" is answered only after the action and its log entry are saved together.
- The same ID again changes nothing and gets the same answer (scope: `README.md`, "IDs").
- A read position only moves forward.
- Acting implies read (D-014, D-015, D-035): a reply, Keep or Done writes its fact and a `reader.read` of the whole row, up to its newest message, in one step. A reaction reads only up to the message reacted to, so newer messages on the row stay unread. The answer carries the number of the fact.
- An arrival on a row someone is watching: Conversations writes that person's `reader.read` in the same step as the arrival.
- Watching is per connection and belongs to its row: "connection X of Marco is watching #launch". A person counts as watching a conversation while at least one such entry is open, so the start on a new connection and the end of an old one can arrive in either order.
- It lasts only as long as its connection. The connection is checked every 20 seconds and counts as gone after 40 without an answer; Conversations then writes the end to the log. Each copy of Conversations holds a lease in the database, renewed every 5 seconds and valid for 15; a start is saved only in a step that also confirms the copy's lease is valid. Housekeeping, every 5 seconds, declares a copy whose lease ran out dead and ends every watching it had started; a copy declared dead cannot save anything further under that identity. So a vanished screen is "no longer watching" within about 40 seconds and the screens of a dead copy within about 25, both inside 60 (D-015: nothing is read while the person is not looking), and keeping a watching alive writes nothing per screen (D-029).
- Actions of one screen are saved in the order they were sent; an answer comes within 3 seconds or the screen treats the action as not sent.
- Undo is the opposite action with its own ID.

## 4. Failure behaviour

- `410 { "organization_gone": true }`: the organization was retired (D-033). The screen goes back to the entry address and loads the current organization.
- No answer, or a server error: the action waits. A new message stays in its place marked as waiting; an edit or a reaction shows as waiting on its message. Waiting actions are sent in the order they were made, each with its ID, when the connection is back (D-024; T-038, point 26).
- A refusal is final: the screen says so in place and offers Discard; "Try again" is offered only where trying again can help (no answer, server error).
- Conversations unreachable: the screen says "Reconnecting" and keeps what it shows.
- The question about the last 7 days may answer "not available right now" when the feed is behind; the action itself is never blocked by it.

## 5. Evolution

New kinds of action may be added. An existing kind keeps its meaning; fields are added, never repurposed. Conversations refuses a kind it does not know with a clear answer.

## 6. Enforced by

G-003, G-004, G-005, G-024, G-025, G-026.
