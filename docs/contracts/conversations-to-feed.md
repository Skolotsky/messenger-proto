# Conversations to Activity feed

From D-006, D-017, D-007. Accepted 2026-11-22. Tried in E-003.

## 1. Parties

The feed delivery worker (ours) calls the activity feed. The feed never calls Conversations and never reads its store.

## 2. Events

Every call is made for one organization (D-026). It is in the address of every call a screen, an agent or the door makes (`/orgs/acme/…`) and in every item a worker delivers; a live connection is opened for one organization. All numbers, places and logs count within that organization. Nothing of another organization is ever sent or accepted.

Every change, in log order: messages (created, changed, deleted; finished ones only), what readers did (read, watching, keep, done, dial, focus rule, focus person, focus only), and clock ticks. Each carries its number, the number before it, and the log's timestamp.

```json
POST /changes
{ "org": "acme", "number": 4812, "previous": 4811, "at": "2026-11-22T10:14:03.120Z", "kind": "message.created",
  "message": { "id": "msg_7Q2", "version": 1, "conversation": "#launch", "thread": null, "author": "anna",
               "text": "@Marco can you sign off the pricing page?", "mentions": ["marco"], "level": null, "reactions": [] } }
{ "org": "acme", "number": 4813, "previous": 4812, "at": "…", "kind": "reader.read", "person": "marco", "conversation": "#launch", "up_to": 4812 }
{ "org": "acme", "number": 4814, "previous": 4813, "at": "…", "kind": "tick" }

POST /changes/batch   { "changes": [ …up to 100 of the above, in order… ] }      for a backlog

answers: 200 applied · 200 already had it · 409 { "i_am_at": 4811 } · 423 { "organization_stopped": "acme" } · anything else, or 3 s of silence: down (for every organization)
Applying a batch stops at the first change that is not answered 200, and answers with that one.
```

## 3. Guarantees

- Never lost; strictly in order. The feed takes only the change whose "previous" is the last one it applied, and saves its rows, its place and its notification notes in one step, so each change counts once.
- The feed's rows always equal what the rules give for the log up to its place. A feed rebuilt from the start of the log equals the live one.
- The feed holds nothing original: wiping it loses nothing.
- The feed may apply the changes of different conversations of one organization side by side (D-030). It keeps the order inside a conversation (a channel with its threads, or a direct conversation), and applies a reader-wide change (a Focus rule, a Focus person or the option that fills that list, "Focus only") only after everything before it and before anything after it. Every other change belongs to exactly one conversation.
- When healthy, a change is reflected within 2 seconds, 19 times in 20.
- Timestamps never decrease within an organization's log. When an organization goes quiet, its log gets one clock tick 6 seconds after its last change, so that everything before it is known to be more than 5 seconds old; there is no tick while changes keep coming, since they carry the time themselves.
- The feed's rows, its place, its own log of notification notes and that log's edition are all per organization. One organization's feed can be rebuilt without touching another's.

## 4. Failure behaviour

- Out of sequence, or asked while idle: the feed answers with its place and the worker continues from there.
- A place is kept per organization. A worker finds work through an index that holds only the organizations that have some, so idle organizations cost nothing. It claims a few organizations for a few seconds, delivers a share of each outside any database transaction, and notes the new place. Organizations that one turn will clear go first; every round also takes some that are far behind. Any number of workers and of feed copies can run side by side (D-029).
- A claim that runs out is taken over by another worker, so a change can reach the feed twice. That is safe because the feed takes only the change that follows its last one; the claim saves work, it is not what keeps delivery correct.
- Workers are not woken through the database on each save; they ask every few tens of milliseconds.
- Down: the worker waits and retries without limit. A person sees the rows the feed had, with "Catching up"; conversations, unread marks on channels and search are unaffected; notifications wait with the feed.
- A change the feed cannot apply: it is set aside and a fence goes up around the one conversation, or the one reader, it touches. Later changes inside the fence are set aside too; those rows are marked "may be out of date" and nothing new notifies from them. Everything outside stays exact. More than five fences in one organization: the feed stops for that organization only and answers "organization stopped" for it; the worker keeps that organization's place, shows it, and carries on with the others. Repair: fix the fault, rebuild that organization's feed.

## 5. Evolution

New kinds of change may be added; the feed applies what it knows and counts the rest. A change to the rules means a rebuild.

## 6. Enforced by

G-005, G-007, G-008, G-009, G-012, G-013, G-019, G-020, G-023, G-024, G-025, G-026, G-027, G-028, G-029, G-030.
