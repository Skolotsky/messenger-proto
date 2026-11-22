# Conversations to Activity feed

From D-006, D-017, D-007. Accepted 2026-11-22. Tried in E-003.

## 1. Parties

The feed delivery worker (ours) calls the activity feed. The feed never calls Conversations and never reads its store.

## 2. Events

Every call is made for one organization (D-026). It is in the address of every call a screen, an agent or the door makes (`/orgs/acme/…`) and in every item a worker delivers; a live connection is opened for one organization. All numbers, places and logs count within that organization. Nothing of another organization is ever sent or accepted.

Every change, in log order, with no number skipped. Each carries `org`, `number`, `previous`, `at` and `kind`. The feed gets everything it needs from these (it never calls Conversations):

| Kind | Payload |
|---|---|
| `message.created`, `message.changed` | the whole finished message (`README.md`). A reply closed as stopped is a `message.created` with `stopped: true` and no mentions (D-031): a For your information row for people in the thread |
| `message.deleted` | `message: { id, version, conversation, thread }` |
| `reply.started` | `message, conversation, thread, author`. No text. The feed counts it and does nothing |
| `reaction.added`, `reaction.removed` | `person, emoji, message, conversation, thread`. Does not change the message's version |
| `person.joined` | `person, name, is_agent` |
| `channel.created` | `conversation, name, people` |
| `channel.people_added` | `conversation, people` |
| `direct.created` | `conversation, people` (two) |
| `reader.read` | `person, conversation, thread, up_to` |
| `reader.watching_started`, `reader.watching_ended` | `person, conversation, thread, connection` |
| `reader.keep`, `reader.done` | `person, conversation, thread` |
| `reader.dial` | `person, conversation, thread, level` |
| `reader.focus_rule` | `person, rule, on` |
| `reader.focus_person` | `person, about, on` |
| `reader.focus_auto_add` | `person, on` |
| `reader.focus_only` | `person, on` |
| `reader.release_setting` | `person, on` (D-023, point 26) |
| `reader.preference`, `reader.profile_line` | `person`, no effect on the feed: counted and ignored |
| `tick` | nothing |

```json
POST /changes
{ "org": "acme", "number": 4812, "previous": 4811, "at": "2026-11-22T10:14:03.120Z", "kind": "message.created",
  "message": { "id": "msg_7Q2", "version": 1, "conversation": "#launch", "thread": null, "author": "anna", "author_is_agent": false,
               "text": "@Marco can you sign off the pricing page?", "mentions": ["marco"], "level": null, "reactions": [], "stopped": false } }
{ "org": "acme", "number": 4813, "previous": 4812, "at": "…", "kind": "reader.read", "person": "marco", "conversation": "#launch", "thread": null, "up_to": 4812 }
{ "org": "acme", "number": 4814, "previous": 4813, "at": "…", "kind": "tick" }

POST /changes/batch   { "changes": [ …up to 100 of the above, of one organization, in order… ] }      for a backlog

answers: 200 applied · 200 already had it · 200 { "fenced": true } taken and set aside · 409 { "i_am_at": 4811 } · 423 { "organization_stopped": "acme" } · anything else, or 3 s of silence: down (for every organization)
Applying a batch stops at the first change that is not answered 200, and answers with that one.
```

## 3. Guarantees

- Never lost; strictly in order. The feed takes only the change whose "previous" is the last one it applied, and saves its rows, its place and its notification notes in one step, so each change counts once.
- The feed's rows always equal what the rules give for the log up to its place. A feed rebuilt from the start of the log equals the live one.
- The feed holds nothing original: wiping it loses nothing.
- At this boundary nothing changes with the size of an organization: the feed takes changes strictly in order and confirms each. Inside, it may then work on different conversations of one organization side by side (D-030). It keeps the order inside a conversation (a channel with its threads, or a direct conversation), and treats an organization-wide change (a Focus rule, a Focus person or the option that fills that list, "Focus only", the release setting, a person joining, a clock tick) as a point that everything before it must have reached first. Every other change belongs to exactly one conversation. "Its place", and the `through` it reports, is the last change up to which everything has been applied.
- When healthy, a change is reflected within 2 seconds, 19 times in 20.
- Timestamps never decrease within an organization's log. After an organization's last change, its log gets a clock tick about 1 second after the newest entry, until a tick is stamped at least 6 seconds after the last change that was not a tick (D-031). Housekeeping looks for ticks that are due every 0.5 seconds, through an index holding only organizations inside that window, so idle organizations cost nothing. So anything is known to be more than 5 seconds old within about 6.5 seconds of happening, whatever arrived in between.
- The feed's rows, its place, its own log of notification notes and that log's edition are all per organization. One organization's feed can be rebuilt without touching another's.

## 4. Failure behaviour

- Out of sequence: the feed answers with its place and the worker continues from there. A worker that is idle or starting asks for it (`operations-and-control.md`).
- A place is kept per organization. A worker finds work through an index that holds only the organizations that have some, so idle organizations cost nothing. It claims a few organizations for a few seconds, delivers a share of each outside any database transaction, and notes the new place. Organizations that one turn will clear go first; every round also takes some that are far behind. Any number of workers and of feed copies can run side by side (D-029).
- A claim that runs out is taken over by another worker, so a change can reach the feed twice. That is safe because the feed takes only the change that follows its last one; the claim saves work, it is not what keeps delivery correct.
- Workers are not woken through the database on each save; they ask every few tens of milliseconds.
- Down: the worker waits and retries without limit. A person sees the rows the feed had, with "Catching up"; conversations, unread marks on channels and search are unaffected; notifications wait with the feed.
- A change the feed cannot apply: it is set aside and a fence goes up around the one conversation, or the one reader, it touches. Later changes inside the fence are set aside too; those rows are marked "may be out of date" and nothing new notifies from them. Raising a fence writes `withdrawn` (`because: "fenced"`) for every notification on hold that the fence covers. A set-aside change counts as applied for `through` and for clock notes, so one fence cannot freeze the organization's notifications. Everything outside stays exact. More than five fences in one organization: the feed stops for that organization only and answers "organization stopped" for it; the worker keeps that organization's place, shows it, and carries on with the others. Repair: fix the fault, rebuild that organization's feed.

## 5. Evolution

New kinds of change may be added; the feed applies what it knows and counts the rest. A change to the rules means a rebuild.

## 6. Enforced by

G-005, G-007, G-008, G-009, G-012, G-013, G-019, G-020, G-023, G-024, G-025, G-026, G-027, G-028, G-029, G-030.
