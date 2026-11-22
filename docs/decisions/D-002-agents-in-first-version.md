# D-002: How real the AI agents are in the first version

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-009
- Evidence: R-002
- Visual: `docs/visuals/D-002-agents-in-first-version.html`

## Why this needs deciding now

D-001 made agents participants in conversations and gave them Flag and Info. What it left open is what drives them in the first version: a script or a real language model. That decides whether the product needs a model and its key to run at all, whether a demonstration is repeatable, and what the seeded team (D-010) can contain.

Not open here, taken from D-001 and from what Slack and Teams do today (R-002):

- An agent is a member of channels and threads, can be mentioned and messaged directly, and its messages carry a visible "agent" label.
- An agent shows a status while it works: working, waiting on someone, done.
- A reply appears as it is written. Text still streaming in never reaches the feed; a finished progress update is Info; a result carries no level; a stop for a human is flagged.
- In the product as launched, agents belong to the team that connects them. We provide the door they come through, not the agents themselves. What that door promises is D-008, a later decision.

## Options

### A. Scripted agents, through the same door a real one would use

The seeded team has a few agents that follow written scenarios: they post, work for a while, write a reply piece by piece, stop and flag someone, and continue when that person responds. They connect the same way a team's own agent would, so everything a real agent would exercise is exercised.

- Upside: the product runs with no model, no key and no cost; a demonstration plays out the same way every time; every agent behaviour D-001 describes can be shown on demand.
- Risk: an agent cannot answer something it has no script for. Mentioning one with a free question gets a scripted reply, which can look hollow. Reduced by saying so in the product ("demo agent") and listing it as a known shortcut.

### B. Agents driven by a real model

Each agent calls a language model and responds to whatever it is asked.

- Upside: feels real; closest to what early access teams will experience.
- Risk: the product does not run without a key and a network; replies vary, so a demonstration is not repeatable and a scenario such as "the agent stops and asks for approval" cannot be produced on demand; cost and model failures become our problem. The quality of the agent, not of the feed, is what people would judge.

### C. Scripted agents, plus one real assistant when a model is configured

A, and one general assistant that answers free questions through a real model if a key is present. Without a key it is simply absent.

- Upside: repeatable where it matters, real where someone pokes at it.
- Risk: two kinds of agent to build and explain. The model has to be integrated early, before the feed is proven.

## Best practice

Separate what the product owns from what it integrates. Chat products provide a door for agents (Slack's events and streaming methods, Teams' bot framework) and leave the agents to others. A proof of concept fakes what it does not own, behind the real boundary, so the fake can be replaced without touching the product.

## Recommendation

**A now, and C's assistant when the model arrives.** The model comes into the product anyway with the text rules, which are built last (D-001). At that point one real assistant is a small addition; before it, a model would be a dependency that proves nothing about the feed.

Locks in: the first version runs without a model; agents enter through a door a real agent could use; the seeded team's agents are scenarios.

Reversibility: cheap. Because scripted agents use the same door, a real one can replace or join them later.

No second opinion was sought: cheap to reverse.

## Decision

Accepted by the user on 2026-11-22: **A, extended, and without the later assistant.**

- "The core is A for sure. Agents themselves aren't part of the product." Someone looking at the product must be able to see it work with no real agent connected, so scripted demo agents are connected from the start.
- The same holds for people: the seeded team's people are scripted too.
- **A way in for a coding agent.** The user wants Claude Code to be able to take part from the early stages, both as an agent and on behalf of a person in the team, "to see from inside". Its purpose is verification: it lets the product be exercised as a participant would use it. It is a demonstration feature, not for production. The form is open ("MCP or something like that") and is the agent's to choose with the system shape.
- The recommendation's second half is dropped: no real assistant is planned. "It's not about 'assistant' but about helping me to do proper verification." The model still arrives with the text rules (D-001), for that purpose only.

## Consequences

- Easier: the first version runs with no model and no key. Checks can drive the product as a person or an agent would and observe what each participant receives, which is what the working agreement asks of verification.
- Harder: acting on behalf of a person, with no access model (D-009), is a door that must not exist in a real deployment. It has to be visibly a demonstration feature, off unless switched on, and on the list of known shortcuts.
- Committed to: scripted people and agents; agents entering through a door a real agent could use; the way in for a coding agent rides on the same doors as real participants (the agent door for agents, the same interface the web app uses for people), so what it verifies is the product and not a side path. It does not replace looking at the rendered page: layout, "nothing jumps" and the phone-sized screen still need a browser (T-004).

## Follow-ups

- D-010 (the seeded team) names the agents and their scenarios.
- D-008 (what the door promises an agent) stays with the later contract decisions.
