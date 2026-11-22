# Decisions

What has been decided and what is still open, from the top down. One line per decision; the record holds the reasoning. How decisions are prepared and recorded: the `decide` skill.

Status: `open` (identified, not yet prepared) · `proposed` (brief ready, waiting for the user) · `accepted` · `superseded by D-nnn`

Line format: `- **D-nnn** [status] The decision as a statement, and what it commits us to → [record](D-nnn-slug.md)`

## Product: what we build and for whom

- **D-001** [accepted] The early access programme tests one bet: the feed sorts what deserves attention into three buckets (Focus by the person's own rules, Needs you, For your information), tuned by a dial per channel and per thread and by the sender's Flag or Info. Commits us to explained rules, one item in one place, no jumping on edit or delete, and model-judged text rules built last → [record](D-001-product-hypothesis.md)
- **D-002** [proposed] How real the AI agents are in the first version (recommended: scripted agents through the same door a real one would use; one real assistant when the model arrives with the text rules) → [record](D-002-agents-in-first-version.md)
- **D-003** [open] What the activity feed contains and how it is cleared: item types, grouping, read state, triage action. Evidence: R-002
- **D-004** [open] What an edit or a delete does to feed items and notifications that already exist. Evidence: R-002
- **D-009** [accepted] The first version is the feed plus what makes it observable: direct messages, a search box, a notification tray, a Focus rules list, unread marks, creating channels, and a layout that works on a phone-sized screen, delivered as an installable web app (PWA). Commits us to two widths for every surface; the access model, sign-in, group messages, ad-hoc topics, profiles, files and formatted text stay out → [record](D-009-first-version-surfaces.md)
- **D-010** [open] The seeded team: how many people and agents, which roles, how busy. Depends on D-002

## System shape: components, boundaries, stack

- **D-005** [open] System shape and stack: one deployable or several, the source of truth, how live updates and catch-up reach the screen. Evidence: R-002

## Contracts and qualities: what services promise each other

- **D-006** [open] The delivery promise to each receiver: loss, repeats, order across create, edit and delete. Evidence: R-002
- **D-007** [open] Quality targets: freshness, retry and give-up limits, what a person sees during an outage, how a receiver is rebuilt. Evidence: R-002
- **D-008** [open] The contract for agents as receivers of message events: what they are told, what they may read, what stops agents triggering each other. Depends on D-002. Evidence: R-002

## Delegated: notable choices the agent made on internals

_None yet._
