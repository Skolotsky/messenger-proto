# Contracts

One file per boundary between services. A contract is what the user reviews instead of code, so it is written in plain language first and schemas second.

Each contract states:

1. **Parties**: who calls or publishes, who receives.
2. **Operations or events**: name, payload shape, one realistic example.
3. **Guarantees**: delivery, ordering, idempotency, and any latency or freshness target, with numbers.
4. **Failure behaviour**: what happens when the other side is slow, down or rejects the request, and what the caller sees.
5. **Evolution**: how the contract may change without breaking the other side.
6. **Enforced by**: the guardrail IDs from `docs/guardrails.md` whose checks hold this contract.

An accepted contract changes only through a decision (the `decide` skill). Note the decision ID next to the change.

## Terms every contract shares (D-031)

- **Row:** a conversation plus an optional thread, `{ "conversation": "#launch", "thread": "msg_7Q2" }`; `thread` is `null` for the channel-level row (posts that reach you through the channel's dial) or the direct conversation. A top-level message that addresses you (a mention, a flag) is the row of its own thread, with `thread` set to its own ID, so it and its later replies are one row (D-001, as in E-001). Read, keep, done, dials, feed rows and notification names all use it.
- **Read position:** one per person and row. Reading a thread moves only that thread's position; opening a channel moves only the channel's, which covers its top-level messages, because those are what it shows. A thread's row is unread while its root is past both the channel's and the thread's position, or a reply is past the thread's. So opening a channel reads a mention shown there and leaves unseen replies unread, and opening the thread reads all of it. The channel's sidebar mark ignores a top-level message already read through its own thread. A reply marks its thread, never its channel in the sidebar (D-032).
- **IDs:** an action ID is unique per organization and caller. The same ID with the same body gets the first answer; with a different body it is refused (`409 { "refused": "id reused" }`). A message ID is unique per organization.
- **Text positions** count Unicode code points, from 0.
- **Message:** `{ id, number, version, conversation, thread, author, author_is_agent, participants, text, mentions, level, reactions: [{ emoji, people }], stopped, created_at, edited_at, deleted }`. `number` is the log number of its `message.created` and never changes; a streamed reply takes the number of its finished message. `participants` is the two people of a direct message, otherwise `null`. `version` counts edits and the delete; a reaction does not change it. The exact form of this and every payload, and where a partial form is allowed, is in `contracts/schema/`.
- **A deleted message** keeps its identity and has empty `text`, `mentions` and `reactions`, so it can leave a marker and nothing of what it said.
- Exact answers and status codes per operation are in `contracts/operations.json`.
- **Edition:** a rising whole number per organization, raised by a feed rebuild.
- **A retired organization** (D-033) accepts no new actions and workers deliver nothing more for it. Doors and the agent door answer `410 { "organization_gone": true }`, and live connections are closed with the same message. What was still undelivered is dropped: this is the one deliberate exception to "nothing is lost". Numbers are never reused: Reset starts a new organization. A deleted organization answers 404; a screen treats a 404 for its organization like `organization_gone`.
