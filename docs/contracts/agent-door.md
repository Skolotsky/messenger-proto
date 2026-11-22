# The agent door

From D-008, D-002, D-001. Accepted 2026-11-22. Tried in E-003 with scripted agents.

## 1. Parties

An agent (scripted, a team's own, or a coding agent through a small connector) calls Conversations' door for agents. Conversations never calls an agent.

## 2. Operations

```json
GET  /agents/release-agent/events?after=311&wait=25
→ [ { "number": 312, "kind": "mentioned", "agent_chain": 0,
      "message": { "id": "msg_7Q2", "version": 1, "conversation": "#launch", "author": "anna", "text": "…" } } ]

POST /agents/release-agent/messages          { "id": "a-91", "thread": "msg_7Q2", "text": "On it.", "level": "info", "because_of": 312 }
POST /agents/release-agent/replies           { "id": "a-92", "thread": "msg_7Q2", "because_of": 312 }        "I am starting a reply"
POST /agents/release-agent/replies/a-92/pieces  { "at": 0, "text": "Build 2.4.1 " }
POST /agents/release-agent/replies/a-92/finish  { "id": "a-92-done", "level": "flag", "mentions": ["marco"] }
POST /agents/release-agent/status            { "state": "waiting", "on": "marco" }

answers: 200 · 200 already had it · 409 { "have": 12 } a piece is missing · 409 closed · 429 slow down · 410 { "too_far_behind": true, "oldest": 290 }
```

An agent is told what addresses it: a mention, a direct message, a reply in a thread it takes part in, a reaction to its message, and the edit or delete of any of these. On request, everything in a channel it is a member of.

## 3. Guarantees

- Events are numbered per agent, kept 7 days, delivered in order, and may be read again; the agent ignores a number it has handled. An agent that asks for events older than 7 days is told so and from which number it can continue.
- Event kinds are named for what happened to the agent (`mentioned`, `direct_message`, `reply`, `reaction`, `changed`, `deleted`); the message inside has the same shape as everywhere else.
- Anything an agent posts with the same ID is saved once. A piece is identified by its reply and its position, so it needs no ID of its own; a status has none either, the latest one counts.
- Text of a reply in progress goes to open screens only. Search, the feed and notifications learn of the reply when it is finished, as one whole message.
- A reply with no new piece for 60 seconds, or older than 10 minutes, is closed as "stopped" with the text so far.
- A piece that repeats text already received adds nothing.
- An agent is not told about another agent's message unless it is named in it. Each message counts the agents in a row behind it; after three, no further agent is told until a person writes.
- At most 20 messages a minute per agent.
- An agent may read any channel (the first version has no access model, D-009) and only its own direct conversations.

## 4. Failure behaviour

- An agent that is away, or never reads at all, reads on from its place when it returns. It cannot hold up people or other agents: nothing waits for an agent.
- "Slow down": the agent waits and sends the same thing again.
- Acting as a person is not part of this door: the coding-agent connector uses the door for people for that, off unless switched on when the system starts, with a banner while on.

## 5. Evolution

New kinds of event may be added; an agent ignores a kind it does not know.

## 6. Enforced by

G-003, G-016, G-017, G-018.
