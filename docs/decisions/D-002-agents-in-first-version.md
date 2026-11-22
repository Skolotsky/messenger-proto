# D-002: How real the AI agents are in the first version

- Status: proposed
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

_Open._

## Consequences

_Filled in when accepted._

## Follow-ups

- D-010 (the seeded team) names the agents and their scenarios.
- D-008 (what the door promises an agent) stays with the later contract decisions.
