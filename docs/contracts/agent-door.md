# The agent door

From D-008, D-002, D-001. Accepted 2026-11-22. Tried in E-003 with scripted agents.

## 1. Parties

An agent (scripted, a team's own, or a coding agent through a small connector) calls Conversations' door for agents. Conversations never calls an agent.

## 2. Operations

Every call is made for one organization (D-026). It is in the address of every call a screen, an agent or the door makes (`/orgs/acme/…`) and in every item a worker delivers; a live connection is opened for one organization. All numbers, places and logs count within that organization. Nothing of another organization is ever sent or accepted.

```json
GET  /orgs/acme/agents/release-agent/events?after=311&wait=25
→ [ { "org": "acme", "number": 312, "kind": "mentioned", "agent_chain": 0,
      "message": { "id": "msg_7Q2", "version": 1, "conversation": "#launch", "author": "anna", "text": "…" } } ]

POST /orgs/acme/agents/release-agent/messages          { "id": "a-91", "conversation": "#launch", "thread": "msg_7Q2", "text": "On it.", "level": "info", "because_of": 312 }
→ 200 { "message": "msg_8B3", "number": 4870 }
POST /orgs/acme/agents/release-agent/replies           { "id": "a-92", "conversation": "#launch", "thread": "msg_7Q2", "because_of": 312 }        "I am starting a reply"
→ 200 { "message": "msg_9A1", "number": 4871 }          later calls name the reply by the agent's ID, a-92; screens see msg_9A1
POST /orgs/acme/agents/release-agent/replies/a-92/pieces  { "at": 0, "text": "Build 2.4.1 " }
POST /orgs/acme/agents/release-agent/replies/a-92/finish  { "id": "a-92-done", "level": "flag", "mentions": ["marco"] }
POST /orgs/acme/agents/release-agent/direct            { "with": "marco" }        → 200 { "conversation": "dm:marco:release-agent" }, the same one each time
POST /orgs/acme/agents/release-agent/status            { "state": "waiting", "on": "marco" }
POST /orgs/acme/agents/release-agent/reactions         { "id": "a-93", "message": "msg_7Q2", "emoji": "eyes", "on": true }
PUT  /orgs/acme/agents/release-agent/subscriptions/%23launch      everything in that channel from now on; DELETE to stop
GET  /orgs/acme/agents/release-agent/conversations/%23launch/messages?around=4812      as the people door answers

answers: 200 · 200 already had it · 409 { "have": 12 } a piece is missing · 409 closed · 429 slow down · 410 { "too_far_behind": true, "oldest": 290 }
```

An agent is told what addresses it: a mention, a direct message, a reply in a thread it takes part in, a reaction to its message, and the edit or delete of any of these. On request, everything in a channel it is a member of.

## 3. Guarantees

- Each event carries the organization's reset number; after a Reset the agent starts again from the number it is told. A `reaction` event names who reacted, the emoji and whether it was added. Events are numbered per agent, kept 7 days, delivered in order, and may be read again; the agent ignores a number it has handled. An agent that asks for events older than 7 days is told so and from which number it can continue.
- Event kinds are named for what happened to the agent (`mentioned`, `direct_message`, `reply`, `reaction`, `changed`, `deleted`); the message inside has the same shape as everywhere else.
- Anything an agent posts with the same ID is saved once. A piece is identified by its reply and its position, so it needs no ID of its own; a status has none either, the latest one counts.
- Text of a reply in progress goes to open screens only. The log carries a text-free `reply.started`; search, the feed and notifications learn the text when it is finished, as one whole message.
- A reply with no new piece for 60 seconds, or older than 10 minutes, is closed as "stopped" with the text last saved: a finished message marked `stopped`, searchable, a For your information row for people in the thread, and no notification (D-031). The text so far is saved about once a second outside the log, so a crash loses at most the last second; a reply with no saved text is closed with no message (D-032).
- A piece that repeats text already received adds nothing.
- An agent is not told about another agent's message unless it is named in it. Each message counts the agents in a row behind it; after three, no further agent is told until a person writes.
- At most 20 messages a minute per agent. An organization's agents are served by one copy of Conversations (D-029), which keeps that count in memory; it starts fresh if the copy restarts.
- An agent may read any channel (the first version has no access model, D-009) and only its own direct conversations.

## 4. Failure behaviour

- An agent that is away, or never reads at all, reads on from its place when it returns. It cannot hold up people or other agents: nothing waits for an agent.
- "Slow down": the agent waits and sends the same thing again.
- Acting as a person is not part of this door: the coding-agent connector uses the door for people for that, off unless switched on when the system starts, with a banner while on.

## 5. Evolution

New kinds of event may be added; an agent ignores a kind it does not know.

## 6. Enforced by

G-003, G-016, G-017, G-018, G-025, G-026.
