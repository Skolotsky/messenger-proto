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
| react | add or remove one emoji on a message |
| read | "I have read this conversation up to here" (opening a conversation, or Mark read on a row) |
| watching | "this conversation is open in front of me", and "no longer" when it stops. Said over the screen's live connection; only the start and the end are saved as changes |
| keep, done | Keep a row; release it |
| dial | the level of a channel or of a thread |
| focus rule, focus person | add or remove a rule; remove a person from Focus or add them back; switch the option that adds everyone who writes directly |
| focus only | the personal notification switch (D-012) |
| preference | a setting that only changes the person's own screens, such as which key sends a message (D-024). It follows the person to every screen; no receiver acts on it |

Questions, which change nothing: "what would this change have moved in the last 7 days?" (D-020, answered by the feed); the people and channels to suggest after `@` (D-024); a conversation's messages around a position, with each message's thread summary (D-023).

```json
POST /orgs/acme/people/actions
{ "id": "act_01JD3K7Q2", "as": "anna", "kind": "send",
  "message": { "id": "msg_7Q2", "conversation": "#launch", "thread": null,
               "text": "@Marco can you sign off the pricing page?", "mentions": ["marco"], "level": null } }
→ 200 { "saved": true, "number": 4812 }

{ "id": "act_01JD3K9R1", "as": "marco", "kind": "read", "conversation": "#launch", "up_to": 4812 }
{ "id": "act_01JD3KA07", "as": "marco", "kind": "watching", "conversation": "#launch" }
→ 409 { "refused": "unknown kind" }        for a kind Conversations does not know
→ 409 { "refused": "message deleted" }     an edit or reaction on a message that is gone; trying again cannot help

GET /orgs/acme/people/marco/preview?change=dial&channel=%23incidents&level=muted
→ 200 { "last_7_days": { "would_leave_needs_you": 14, "would_stay": 3 } }   or   503 { "not_available": "the feed is catching up" }
```

## 3. Guarantees

- "Saved" is answered only after the action and its log entry are saved together.
- The same ID again changes nothing and gets the same answer.
- A read position only moves forward.
- Watching lasts only as long as the connection it was said on. The connection is checked every 20 seconds and counts as gone after 40 without an answer; Conversations then writes "no longer watching" to the log. Each copy of Conversations also holds a short lease in the database; if a copy dies, housekeeping ends every watching that copy had started. Either way a vanished screen is "no longer watching" within 60 seconds (D-015: nothing is read while the person is not looking), and keeping it alive writes nothing per screen (D-029).
- Actions of one screen are saved in the order they were sent; an answer comes within 3 seconds or the screen treats the action as not sent.
- Undo is the opposite action with its own ID.

## 4. Failure behaviour

- No answer, or a server error: the action waits. A new message stays in its place marked as waiting; an edit or a reaction shows as waiting on its message. Waiting actions are sent in the order they were made, each with its ID, when the connection is back (D-024; T-038, point 26).
- A refusal is final: the screen says so in place and offers Discard; "Try again" is offered only where trying again can help (no answer, server error).
- Conversations unreachable: the screen says "Reconnecting" and keeps what it shows.
- The question about the last 7 days may answer "not available right now" when the feed is behind; the action itself is never blocked by it.

## 5. Evolution

New kinds of action may be added. An existing kind keeps its meaning; fields are added, never repurposed. Conversations refuses a kind it does not know with a clear answer.

## 6. Enforced by

G-003, G-004, G-005, G-024, G-025, G-026.
