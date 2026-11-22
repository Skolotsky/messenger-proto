# Decisions

What has been decided and what is still open, from the top down. One line per decision; the record holds the reasoning. How decisions are prepared and recorded: the `decide` skill.

Status: `open` (identified, not yet prepared) · `proposed` (brief ready, waiting for the user) · `accepted` · `superseded by D-nnn`

Line format: `- **D-nnn** [status] The decision as a statement, and what it commits us to → [record](D-nnn-slug.md)`

## Product: what we build and for whom

- **D-001** [accepted] The early access programme tests one bet: the feed sorts what deserves attention into three buckets (Focus by the person's own rules, Needs you, For your information), tuned by a dial per channel and per thread and by the sender's Flag or Info. Commits us to explained rules, one item in one place, no jumping on edit or delete, and model-judged text rules built last. Flag closing superseded by D-011; Muted and All activity narrowed by D-013 → [record](D-001-product-hypothesis.md)
- **D-002** [accepted] People and agents in the first version are scripted and enter through the doors real ones would use; no model is needed to run it. A coding agent can join as an agent or on behalf of a person, as a demonstration feature for verification → [record](D-002-agents-in-first-version.md)
- **D-003** [accepted] Reading a row is enough for it to leave the feed; a flag stays until it closes, and anything the person keeps stays in Focus until they mark it done. Commits us to Keep and Done, an Earlier list, replying in place, and a list that does not move while viewed. How a flagged row leaves is superseded by D-011 → [record](D-003-how-items-leave-the-feed.md)
- **D-004** [accepted] A deleted message leaves no trace in a conversation unless something depends on it (replies). In the feed, something taken back leaves a marker only where the person had already seen it, and one that stays for flagged or kept rows. An edit that adds a mention or flag reaches the person as new. The marker for a flagged row is superseded by D-011 → [record](D-004-edits-and-deletes.md)
- **D-009** [accepted] The first version is the feed plus what makes it observable: direct messages, a search box, a notification tray, a Focus rules list, unread marks, creating channels, and a layout that works on a phone-sized screen, delivered as an installable web app (PWA). Commits us to two widths for every surface; the access model, sign-in, group messages, ad-hoc topics, profiles, files and formatted text stay out → [record](D-009-first-version-surfaces.md)
- **D-010** [accepted] The seeded team is large: about 30 people, 6 agents and 20 channels, with a core cast carrying hand-written situations and the rest as background. A separate control panel starts, stops and resets the demonstration and shows what happened and what comes next → [record](D-010-seeded-team.md)
- **D-011** [accepted] A flag is a marker for each reader's feed, with no state of its own: one that names you is kept for you automatically until you press Done or respond; one that names nobody is an ordinary row in Needs you. Supersedes how a flag closes in D-001, D-003 and D-004. Commits us to no status on a flag anywhere → [record](D-011-a-flag-is-a-marker-for-the-readers-feed.md)
- **D-012** [accepted] Focus and Needs you send a notification, For your information never; one notification per row, not per message; one personal switch to "Focus only". Commits us to notifications following the bucket, with no notification settings per channel → [record](D-012-what-sends-a-notification.md)
- **D-013** [accepted] The readings E-001 needed: a muted channel does not silence your own threads; All activity leaves addressed items as at Default; a dial or rule change re-sorts unread rows; plus a list of smaller assumed behaviours that stand until objected to → [record](D-013-feed-rules-confirmed-after-e-001.md)
- **D-014** [accepted] A reaction from someone you addressed is an answer and lands in Needs you with their name; feed rows show the reactions on their message; the grouped row shows the split by emoji. No emoji has a built-in meaning → [record](D-014-reactions-in-the-feed.md)
- **D-015** [accepted] Read is one position per person and conversation, shared by the feed, channel unread marks and notifications on every device. Opening marks read in Focus and Needs you; acting implies read; nothing is read while the person is not looking; no mark-as-unread → [record](D-015-what-read-means.md)
- **D-016** [accepted] Besides the feed there are four history tabs for finding things again: Direct messages, Mentions, Threads, Reactions. They are not sorted by buckets or dials and nothing leaves them by being read → [record](D-016-history-tabs.md)

## System shape: components, boundaries, stack

- **D-005** [open] System shape and stack: one deployable or several, the source of truth, how live updates and catch-up reach the screen. Evidence: R-002

## Contracts and qualities: what services promise each other

- **D-006** [open] The delivery promise to each receiver: loss, repeats, order across create, edit and delete. Evidence: R-002
- **D-007** [open] Quality targets: freshness, retry and give-up limits, what a person sees during an outage, how a receiver is rebuilt. Evidence: R-002
- **D-008** [open] The contract for agents as receivers of message events: what they are told, what they may read, what stops agents triggering each other. Depends on D-002. Evidence: R-002

## Delegated: notable choices the agent made on internals

_None yet._
