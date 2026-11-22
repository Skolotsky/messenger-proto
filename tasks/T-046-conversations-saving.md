# T-046: Conversations, saving and the log

- Traces to: brief (messages, replies, mentions, reactions, edits and deletes reaching independent systems), D-005, D-006, D-015, D-017, D-026, D-031, D-032; G-003, G-004, G-005, first half of G-025
- Started: 2026-11-22

## Outcome

Conversations saves every person's action and writes it to the numbered change log in one transaction, and answers the people door's actions and its questions that need neither the feed nor search.

## Acceptance criteria

- [x] Every `PeopleAction` kind is saved from its contract example, its answer has the contract's form, and every log entry matches the schema. Check: `TestEveryActionExample`
- [x] Every refusal answer of each kind. Check: `TestRefusals`
- [x] The same ID with the same body gets the first answer and changes nothing; with another body `id reused`; IDs are per caller; a refusal is remembered. Check: `TestSameIDGetsTheFirstAnswer`
- [x] Acting implies read: a reply, reaction, Keep or Done writes its fact and a `reader.read` of the whole row in one step; the answer carries the fact's number. Check: `TestActingImpliesRead`
- [x] An arrival on a watched row is read in the same step, for that row only. Check: `TestWatchedArrivalIsReadInTheSameStep`
- [x] Questions: start, messages with thread summaries and read position, suggest, people. Check: `TestQuestions`, `TestStartNumberIsThePersonsOwn`
- [x] Seed session: open, batches with past times, seal; refused when not empty, sealed, out of order or going back in time; the log continues afterwards. Check: `TestSeedSession`
- [x] Timestamps never decrease in a log, also when the clock steps back. Check: `TestTimeNeverGoesBack`
- [x] G-003, seen failing without action IDs. Check: `tests/guardrails/saved-once`, `-tags fault_no_action_ids`
- [x] G-004, seen failing with the rule off. Check: `tests/guardrails/change-and-log-together`, `-tags fault_no_log_rule`
- [x] G-005, seen failing with a plain sequence. Check: `tests/guardrails/log-numbering`, `-tags fault_plain_sequence`
- [x] First half of G-025, seen failing with row rules off. Check: `TestOrganizationsAreKeptApartByRowRules`, `-tags "integration fault_no_row_rules"`

## Verification plan

- Automated: integration tests of the part alone on PostgreSQL (`server/internal/conversations`, tag `integration`); the three guardrail tests on PostgreSQL, G-003 against the real program built and killed with SIGKILL.
- Real system: the composed program through `./scripts/verify`, and a by-hand pass through its HTTP door.
- Independent: not in this task; the delivery guarantees get a verifier at T-057.
- Not covered: the live connection, watching over it, the door's reads from feed and search, housekeeping and ticks (T-053); the agent door (T-054). G-025 for every door is proven in T-057.

## Requests for shared changes

1. **`contracts/operations.json`, `messages`: add the optional query `as` (person).** `MessagesAnswer.read_up_to` and `thread_summary.unread` belong to a person, but the operation names none. The server accepts `as`; without it both are empty. The web app must send it.
2. **A way to tell the feed where seeded history ends.** `feed-to-notification-sender.md` says seeded history writes no "due" (D-032), but no change, place or operation tells the feed which numbers were seeded. Conversations knows (`seed_sessions`). This needs a contract decision before T-052 and T-059 meet; nothing was invented here.
3. **`orgdb`: a read-only snapshot transaction.** `InOrg` sets the organization first, so a question cannot then ask for repeatable read. Questions hold `FOR SHARE` on the organization's log row instead, so a save waits for a running question. A snapshot variant would remove that wait.
4. **`tests/guardrails/orgs-apart`** is not among this package's folders. The first half of G-025 is `TestOrganizationsAreKeptApartByRowRules` in `server/internal/conversations`; T-057 should move or call it from the guardrail's named folder.
5. **`tests/go.mod`** gained nothing; no dependency changes were needed.

## Contract notes

- **Reacting to a plain channel message reads the whole channel row.** The contract says a reaction reads "the whole row, up to its newest message". For a top-level message that does not address the person, that row is the channel, so a reaction on an old message marks every newer top-level message read. Followed as written; it sits uneasily with D-015 ("nothing is read while the person is not looking"). Worth a product look.
- **`org_name` has no source.** Nothing in the contracts sets an organization's display name; `start` answers with its ID.
- **What creates an organization.** Only the seed session creates its `org_log` row; the contracts name no other way.

## Log

- 2026-11-22: choices on internals. One step is one transaction holding the `org_log` row; every change goes through one `project` function, for actions and seeding alike. The G-004 rule is a deferred constraint trigger: the entry at the head must have been written by this transaction. Refusals are remembered with the action ID and add nothing to the log. An action that changes nothing (a repeated reaction, a read that does not move, an open direct that exists, add people already there) answers `saved` with `number: null`.
- Readings where the contract is silent: a reply to a deleted root is allowed while live replies keep it visible, refused otherwise; direct conversations have no threads and only their two people can write in them; writing in a direct conversation counts as acting on its row; the channel-level dial accepts only the four channel levels; channel membership is not enforced for writing, as the access model is out of scope (D-009); mentions must name known people or "channel".

## Evidence

- `./scripts/check` and `./scripts/verify`: all passed on 2026-11-22.
- G-003 passed: 204 actions acknowledged through 5 SIGKILLs of the real program, each in the store and the log once. With `-tags fault_no_action_ids` it failed: a resent action answered `message id taken` instead of its first answer.
- G-004 passed. With `-tags fault_no_log_rule` it failed: transactions changing `messages` and `reader_reads` without a log entry were not refused at commit. A split write with the rule off is reported by the check "every saved change is in the log".
- G-005 passed with 16 writers and one transaction held open. With `-tags fault_plain_sequence` it failed: the follower stepped over 34 committed changes.
- G-025, first half, passed on all 16 tables. With `-tags "integration fault_no_row_rules"` it failed: a query bound to one organization saw the other's rows.
- By hand: the real program in Compose, driven over HTTP through seed, send, resend, reply, react, edit, open direct, start, messages (channel and thread), suggest, people and a refusal; the log held the expected nine entries and `delivery_due` had its search and feed rows.
- Not verified: anything needing T-047 onward (a worker actually consuming the log), the live connection, and G-025 through every door. The G-004 rule proves that a transaction wrote a log entry, not that it wrote one for each fact it changed.
