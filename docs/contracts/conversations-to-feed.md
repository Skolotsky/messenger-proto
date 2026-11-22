# Conversations to Activity feed

From D-006, D-017, D-007. Accepted 2026-11-22. Tried in E-003.

## 1. Parties

The feed delivery worker (ours) calls the activity feed. The feed never calls Conversations and never reads its store.

## 2. Events

Every change, in log order: messages (created, changed, deleted; finished ones only), what readers did (read, watching, keep, done, dial, focus rule, focus person, focus only), and clock ticks. Each carries its number, the number before it, and the log's timestamp.

```json
POST /changes
{ "number": 4812, "previous": 4811, "at": "2026-11-22T10:14:03.120Z", "kind": "message.created",
  "message": { "id": "msg_7Q2", "version": 1, "conversation": "#launch", "thread": null, "author": "anna",
               "text": "@Marco can you sign off the pricing page?", "mentions": ["marco"], "level": null, "reactions": [] } }
{ "number": 4813, "previous": 4812, "at": "…", "kind": "reader.read", "person": "marco", "conversation": "#launch", "up_to": 4812 }
{ "number": 4814, "previous": 4813, "at": "…", "kind": "tick" }

POST /changes/batch   { "changes": [ …up to 100 of the above, in order… ] }      for a backlog

answers: 200 applied · 200 already had it · 409 { "i_am_at": 4811 } · anything else, or 3 s of silence: down
Applying a batch stops at the first change that is not answered 200, and answers with that one.
```

## 3. Guarantees

- Never lost; strictly in order. The feed takes only the change whose "previous" is the last one it applied, and saves its rows, its place and its notification notes in one step, so each change counts once.
- The feed's rows always equal what the rules give for the log up to its place. A feed rebuilt from the start of the log equals the live one.
- The feed holds nothing original: wiping it loses nothing.
- When healthy, a change is reflected within 2 seconds, 19 times in 20.
- Log timestamps never decrease. While there has been a change in the last 10 seconds, the log carries a clock tick every second.

## 4. Failure behaviour

- Out of sequence, or asked while idle: the feed answers with its place and the worker continues from there.
- Down: the worker waits and retries without limit. A person sees the rows the feed had, with "Catching up"; conversations, unread marks on channels and search are unaffected; notifications wait with the feed.
- A change the feed cannot apply: it is set aside and a fence goes up around the one conversation, or the one reader, it touches. Later changes inside the fence are set aside too; those rows are marked "may be out of date" and nothing new notifies from them. Everything outside stays exact. More than five fences: the feed stops. Repair: fix the fault, rebuild the feed.

## 5. Evolution

New kinds of change may be added; the feed applies what it knows and counts the rest. A change to the rules means a rebuild.

## 6. Enforced by

G-005, G-007, G-008, G-009, G-012, G-013, G-019, G-020, G-023, G-024.
