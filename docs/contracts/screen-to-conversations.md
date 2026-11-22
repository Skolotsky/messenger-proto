# Screen to Conversations: what a person does

From D-005, D-006, D-017. Accepted 2026-11-22.

## 1. Parties

The web app (a person's screen), and the scripted team acting as people, call Conversations' door for people. Conversations answers.

## 2. Operations

Every call is made for one team (D-026). The team is in the address (`/teams/acme/…`) and in every item sent, and all numbers count within that team. Nothing of another team is ever sent or accepted.

Every action is one call carrying an ID the caller chose. Conversations saves it as a fact and writes it to the change log in the same step.

| Action | Says |
|---|---|
| send | a new message: conversation, optional thread, text, mentions, level (Flag, Info or none) |
| edit, delete | a change to the caller's own message |
| react | add or remove one emoji on a message |
| read | "I have read this conversation up to here" (opening a conversation, or Mark read on a row) |
| watching | "this conversation is open in front of me", sent again every 20 s while it is true, and "no longer" when it stops |
| keep, done | Keep a row; release it |
| dial | the level of a channel or of a thread |
| focus rule, focus person | add or remove a rule; remove a person from Focus or add them back; switch the option that adds everyone who writes directly |
| focus only | the personal notification switch (D-012) |

One question, answered through this door by the feed: "what would this change have moved in the last 7 days?" (D-020). It changes nothing.

```json
POST /people/actions
{ "id": "act_01JD3K7Q2", "as": "anna", "kind": "send",
  "message": { "id": "msg_7Q2", "conversation": "#launch", "thread": null,
               "text": "@Marco can you sign off the pricing page?", "mentions": ["marco"], "level": null } }
→ 200 { "saved": true, "number": 4812 }

{ "id": "act_01JD3K9R1", "as": "marco", "kind": "read", "conversation": "#launch", "up_to": 4812 }
{ "id": "act_01JD3KA07", "as": "marco", "kind": "watching", "conversation": "#launch" }
→ 409 { "refused": "unknown kind" }        for a kind Conversations does not know

GET /people/marco/preview?change=dial&channel=%23incidents&level=muted
→ 200 { "last_7_days": { "would_leave_needs_you": 14, "would_stay": 3 } }   or   503 { "not_available": "the feed is catching up" }
```

## 3. Guarantees

- "Saved" is answered only after the action and its log entry are saved together.
- The same ID again changes nothing and gets the same answer.
- A read position only moves forward.
- Watching lapses by itself: it counts only while it was reported within the last 60 seconds, by the log's own clock. A screen that vanishes, or Conversations restarting, therefore cannot leave a conversation "watched" (D-015: nothing is read while the person is not looking).
- Actions of one screen are saved in the order they were sent; an answer comes within 3 seconds or the screen treats the action as not sent.
- Undo is the opposite action with its own ID.

## 4. Failure behaviour

- No answer, or an error: the screen keeps the action, shows it as "not sent", and sends it again with the same ID.
- Conversations unreachable: the screen says "Reconnecting" and keeps what it shows.
- The question about the last 7 days may answer "not available right now" when the feed is behind; the action itself is never blocked by it.

## 5. Evolution

New kinds of action may be added. An existing kind keeps its meaning; fields are added, never repurposed. Conversations refuses a kind it does not know with a clear answer.

## 6. Enforced by

G-003, G-004, G-005, G-024, G-025, G-026.
