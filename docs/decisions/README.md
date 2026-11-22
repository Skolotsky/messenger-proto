# Decisions

What has been decided and what is still open, from the top down. One line per decision; the record holds the reasoning. How decisions are prepared and recorded: the `decide` skill.

Status: `open` (identified, not yet prepared) · `proposed` (brief ready, waiting for the user) · `accepted` · `superseded by D-nnn`

Line format: `- **D-nnn** [status] The decision as a statement, and what it commits us to → [record](D-nnn-slug.md)`

## Product: what we build and for whom

- **D-001** [proposed] The product hypothesis the early access programme tests: the feed sorts what deserves attention into three buckets (Focus by the person's own rules, Needs you, For your information), tuned by one dial per channel and by the sender's Flag or Info. Settled in discussion; waiting for the user to accept the record. Sets the scope of every other product decision → [record](D-001-product-hypothesis.md)
- **D-002** [open] The role of AI agents: whether they are full participants, how status and streamed replies appear, what agent activity reaches the feed, scripted or real model. Evidence: R-002
- **D-003** [open] What the activity feed contains and how it is cleared: item types, grouping, read state, triage action. Evidence: R-002
- **D-004** [open] What an edit or a delete does to feed items and notifications that already exist. Evidence: R-002
- **D-009** [open] Which surfaces the PoC includes (channels, threads, direct messages, switching person) and how busy the seeded team is. Depends on D-001

## System shape: components, boundaries, stack

- **D-005** [open] System shape and stack: one deployable or several, the source of truth, how live updates and catch-up reach the screen. Evidence: R-002

## Contracts and qualities: what services promise each other

- **D-006** [open] The delivery promise to each receiver: loss, repeats, order across create, edit and delete. Evidence: R-002
- **D-007** [open] Quality targets: freshness, retry and give-up limits, what a person sees during an outage, how a receiver is rebuilt. Evidence: R-002
- **D-008** [open] The contract for agents as receivers of message events: what they are told, what they may read, what stops agents triggering each other. Depends on D-002. Evidence: R-002

## Delegated: notable choices the agent made on internals

_None yet._
