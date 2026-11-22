# D-008: The contract for agents as receivers and senders

- Status: proposed
- Level: contract
- Date: 2026-11-22
- Decided by: user (not yet decided)
- Depends on: D-002, D-005, D-006
- Evidence: R-002; the brief; second opinion and claim checks recorded in `tasks/T-022-architecture-draft.md`
- Visual: `docs/visuals/T-022-architecture-proposal.html`, section 10; walk-throughs in `docs/visuals/T-022-walkthroughs.html`

## Why this needs deciding now

D-002 committed to agents entering through a door a real agent could use, and to a coding agent joining for verification. The scripted agents of the seeded team (D-010) are the first users of that door, so it has to be defined before they are written.

## Options

**A. We call the agent.** The agent hosts a URL; we push events and expect an answer in three seconds (Slack's Events API). Best known. An agent on a developer's machine has no reachable URL, so the coding agent of D-002 could not use it.

**B. The agent calls us and reads from its own place.** The agent asks for events after the last number it handled and waits for the answer (long poll; a WebSocket can be added). Slack offers the same as Socket Mode, where the app opens the connection and acknowledges each event (checked 2026-11-22). Works from anywhere; an agent that was away reads on; it is the catch-up rule the screens use. Risk: an agent hears only when it asks, and a slow one keeps events in store; we keep 7 days.

## Best practice

Chat products deliver events to agents across a boundary with acknowledgement and retries, grant reading rights explicitly, keep in-progress output silent, and cap streams at ten minutes (R-002). No vendor documents a rule against agents triggering each other.

## Recommendation

**B**, with:

- **Told:** what addresses the agent (a mention, a direct message, a reply in a thread it takes part in, a reaction to its message, and edits or deletes of these). On request, everything in a channel it is a member of.
- **May read:** any channel, because the first version has no access model (D-009), and only its own direct conversations. A known shortcut.
- **Sends:** a message with a level; a streamed reply (start, pieces, finish with level and mentions); a status (working, waiting on someone, done). Everything it sends carries its own ID, so a repeat does not post twice.
- **Streaming:** pieces travel a best-effort path to open screens and are kept as "text so far"; only the start and the finished message go through the change log. A reply with no piece for a minute, or older than ten minutes, is closed as "stopped".
- **Agents setting each other off:** an agent is not told about another agent's message unless it is named; each message counts the agents in a row behind it, and after three no further agent is told until a person writes; 20 messages a minute per agent.
- **A coding agent:** an MCP connector over this door. Acting as a person uses the door the web app uses, is off unless switched on at start, and shows a banner.

Reversibility: cheap to moderate; A can be added beside B.

Tried with scripted agents in E-003 (2026-11-22): 183 events to three agents, one of them away for 3 s and all of them crashing now and then, none missed and in order; 46 repeated replies saved once; 59 streamed replies never seen unfinished by a receiver; 10 abandoned ones closed as "stopped"; an agent loop stopped at three; a too-fast agent slowed down and retried. An agent told to slow down must wait and send the same thing again; that is part of the contract.

Second opinion: chose B; noted that through MCP nothing is pushed, so "reads from its place" is a call the agent makes, and that agent posts need an ID against double posting. Both are in the proposal.

## Decision

Not yet taken.

## Consequences

- Easier: scripted agents, a real agent and a coding agent use one door; an agent's outage needs no handling on our side beyond keeping its events.
- Harder: the chain counter must travel with every message.
- Committed to, if accepted: the events and calls above as a contract.

## Follow-ups

- Contract `docs/contracts/agent-door.md` after acceptance.
- Small product question: whether a reply closed as "stopped" reaches anyone's feed.
