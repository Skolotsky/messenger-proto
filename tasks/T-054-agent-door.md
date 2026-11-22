# T-054: Conversations, the agent door

- Traces to: brief (agents exercising the feed through real doors), D-002, D-008, D-031, D-032, D-033, D-034; G-016, G-017, G-018, part of G-029
- Started: 2026-11-22

## Outcome

Agents work through Conversations' agent door: they read their own numbered events, post, stream replies, react, open direct conversations, subscribe to channels, set a status and read conversations.

## Acceptance criteria

- [x] Every operation of `agent-door` answers in the contract's form, and everything it logs matches the schema. Check: `TestEveryAgentOperationAnswersItsContract`
- [x] An agent is told what addresses it, and a channel it subscribed to from then on. Check: `TestAnAgentIsToldWhatAddressesIt`, `TestASubscriptionTellsEverythingFromNowOn`
- [x] Events are numbered per agent, in order, re-readable; an agent away or crashing misses none; one that never reads delays nobody. Check: `TestAnAgentThatIsAwayOrCrashesMissesNothing`
- [x] Past 7 days: `410 too_far_behind` with where to continue, before and after trimming. Check: `TestAfterSevenDaysAnAgentIsToldItIsTooFarBehind`
- [x] Long polling answers on arrival, ends after its wait, and hears a retirement. Check: `TestALongPoll*`, `TestAWaitingAgentHearsThatTheOrganizationIsGone`
- [x] A retired organization answers 410 on every operation, a deleted one 404. Check: `TestRetiredAnswersGoneAndDeletedAnswersNotFound`
- [x] Text in progress stays off the log; the finished reply is one message. Check: `TestTextInProgressStaysOffTheLog`
- [x] Pieces by position in code points; a repeat adds nothing; a gap answers `409 { have }`. Check: `TestPiecesAreIdentifiedByPosition`
- [x] 60 s of silence or 10 minutes closes a reply as stopped; no saved text writes `reply.abandoned` and no message. Check: `TestSilenceClosesAReplyAsStopped`, `TestAReplyOlderThanTenMinutesIsClosed`, `TestAReplyWithNoSavedTextLeavesNoMessage`
- [x] A crash loses at most what was not yet saved. Check: `TestACrashLosesAtMostWhatWasNotYetSaved`, and by hand with SIGKILL
- [x] Three agents in a row, then none is told until a person writes; also across rows. Check: `TestAgentsStopAtThreeInARow`, `TestTheCountFollowsAnAnswerIntoAnotherRow`
- [x] 20 messages a minute per agent; a resend is neither counted nor saved twice. Check: `TestAnAgentIsHeldToTwentyMessagesAMinute`, `TestResendingASavedMessageIsNotCountedOrSavedAgain`
- [x] Status: latest wins, outside the numbering, on `PersonView.status`; one function hands pieces and status to the live connection. Check: `TestEveryAgentOperationAnswersItsContract`, `TestAnAgentsPushReachesTheLiveConnection`
- [x] The door's timed questions read no idle organization, at 50 and 500. Check: `TestTheAgentDoorsTimedQuestionsSkipIdleOrganizations`

## Verification plan

- Automated: the door's real routes on a database of their own, with a moved clock (`tests/guardrails/agent-events`, `reply-in-progress`, `agent-limits`); each planted fault under `-tags agentfaults`.
- Real system: the real program on PostgreSQL with the real clock, killed with SIGKILL mid-reply and restarted.
- Independent: not in this task; the door is covered by the milestones' verifiers.
- Not covered: pieces and status reaching a real screen (T-053 calls `SetAgentPush`); the door under two copies; scripted agents (T-059).

## Readings for the user

Each is where the contract is silent; the choice stands until objected to.

1. **A new message in a subscribed channel has event kind `reply`**, also at top level (`message.thread` is null). The contract's six kinds have none for it and the schema allows no other.
2. **`@channel` from a person tells the agents in that channel, as `mentioned`.** From an agent it tells no agent, since an agent must be named.
3. **"A thread it takes part in"** means the agent wrote its root or a reply. Being mentioned in a thread is a `mentioned` event and does not by itself make it a participant.
4. **Another agent's direct message follows the same rule as a channel:** the agent is told only if named in it. This is the contract's sentence read literally.
5. **Agents in a row are counted in the message's own row** (a thread, or a channel's top level). An answer naming via `because_of` an event of another row carries that event's count, so a loop cannot escape by changing rows.
6. **An agent's reaction counts as one more agent** behind that message, and tells nobody once the message's count is three. The contract only caps messages; without this two agents could loop on reactions.
7. **`oldest` in `too_far_behind` is the oldest number still kept:** the agent continues with `after = oldest - 1`.
8. **`continue_from`** is set when a page (100 events) was cut, to the last number in it; otherwise null.
9. **The limit of 20 counts posts and reply starts.** A finish, a piece, a reaction and a status do not count. A message told to slow down keeps its ID unspent.
10. **A repeated piece does not count as a new piece**, so it cannot keep a reply open past 60 s.
11. **A piece or finish that arrives after the deadline but before housekeeping** closes the reply at once and answers `409 closed`.
12. **Subscribing to a channel the agent is not a member of answers 404**, and so does a status naming an unknown person. Reading another's direct conversation answers `409 not a participant`.
13. **A reaction that changes nothing** answers 200 with the log's current number, since `AgentPosted.number` is required.
14. **An edit that newly names an agent** reaches it as `mentioned` (D-004); otherwise an agent that was told, or is addressed now, gets `changed` or `deleted`.
15. **Seeded history tells no agent.** Their events start with the first live change.
16. **A reply in progress when its organization is retired is dropped** (D-033): no stopped message.

## Requests for shared changes

0. **Blocking the merge: add the door's seven tables to G-031's fixture.** `tests/guardrails/retire-and-delete` requires a fixture row in every table with an `org` column, and it is an enforced test this package may not edit (G-022), so on this branch `TestDeleteRemovesOneOrganizationAndNoOther` fails with "the fixture has no row in conversations.agent_*". The delete code already covers the tables. Add these lines to `fill`, before the `feed.note_heads` row; changing this test needs the user's approval:

   ```go
   `INSERT INTO conversations.agent_heads (org, agent, head) VALUES ($1, 'marco', 1)`,
   `INSERT INTO conversations.agent_events (org, agent, number, at, message, body) VALUES ($1, 'marco', 1, ` + at + `, 'msg_1', '{"kind": "mentioned"}')`,
   `INSERT INTO conversations.agent_subscriptions (org, agent, conversation) VALUES ($1, 'marco', '#launch')`,
   `INSERT INTO conversations.agent_chains (org, message, chain) VALUES ($1, 'msg_1', 1)`,
   `INSERT INTO conversations.agent_replies (org, agent, id, message, conversation, started_at, last_piece_at, due_at, text)
   	VALUES ($1, 'marco', 'a-1', 'msg_2', '#launch', ` + at + `, ` + at + `, ` + at + `, 'the same words')`,
   `INSERT INTO conversations.agent_status (org, agent, state) VALUES ($1, 'marco', 'working')`,
   `INSERT INTO conversations.agent_org_due (org, reply_due_at, events_due_at) VALUES ($1, ` + at + `, ` + at + `)`,
   ```

1. **`scripts/verify`: run the new must-fail tests:** `-tags agentfaults -run Fails ./guardrails/agent-events/ ./guardrails/reply-in-progress/ ./guardrails/agent-limits/ ./guardrails/idle-organizations-cost-nothing/`. Until then they run only by hand.
2. **`docs/guardrails.md`: set G-016, G-017 and G-018 to `enforced`** and add their `permissions.ask` rules.
3. **`orgdb`: a list of all databases.** The door's housekeeping takes its pools from `main`, as delivery has its own `All()`. G-028's lint will need one shared way.
4. **T-053:** call `Store.SetAgentPush` from the live connection. `conversation` is empty for a status (everyone in the organization); for a piece it names the conversation, and a direct conversation's pieces must reach only its two people. `GET /orgs/{org}/replies/{message}` is registered here, so T-053 must not add it.
5. **`server/cmd/conversations/main.go`** now starts the door's housekeeping (one line), beside whatever T-053 starts.

## Contract notes

- **`GET /orgs/{org}/replies/{message}` names no reader,** so the text of a reply in progress in a direct conversation can be fetched by anyone who knows its message ID. D-031 says only participants find a direct message. It needs an `as`, as `messages` has.
- **`finish` carries no length.** An agent that crashed nothing but talks to a restarted server can finish a reply missing up to its last second of text, without knowing. An `at` on finish, answered `409 { have }`, would close this.
- **The event kinds lack one for "new in a channel you follow"** (reading 1).

## Log

- 2026-11-22: choices on internals. Events are written in the step that saves the change, so an agent cannot miss one and no second reader of the log is needed. A reply's newest text is in memory, saved about once a second to `agent_replies`, and restored from there after a restart. Lock order: the reply, then the organization's log row; a question holding the log row reads an atomic copy of the text. Housekeeping finds organizations through `agent_org_due`, which holds times only, on partial indexes. Those two questions run with sequential scans off, because on a small table PostgreSQL would otherwise read every row, idle organizations included.
- 2026-11-22: a test of mine miscounted a code-point position (3 for 2); the door was right and the test was corrected.

## Evidence

- G-016, G-017, G-018 and the door's part of G-029 pass, three runs in a row; G-016 to G-018 also under the race detector.
- Seen failing with the rule off (`-tags agentfaults`), each for its own reason: G-016 a dropped `reply` event, no `too_far_behind` (answered 200 with old events); G-017 text on the log, no timeout (0 messages after 61 s), a repeat added again (`héllo🙂héllo🙂…`); G-018 no chain cap (12 posts, not 3), no rate limit (45 of 45 accepted); G-029 a question reading idle organizations (55 rows with 5 due).
- By hand, the real program with the real clock: a long poll answered in 0.017 s; killed with SIGKILL mid-reply, the log held none of the text, the saved text survived, the unsaved part was lost, a later piece answered `409 { have: 6 }`, and its own housekeeping closed the reply as stopped.
- **`./scripts/verify` on this branch FAILS**, at G-031 only, for the fixture in request 0. With those lines applied to a throwaway copy of the branch, the full `./scripts/verify` passed (all guardrails, must-fail runs, integration, timed guardrails, 96 browser tests in the last suite shown). The enforced test in this branch is unchanged.
- Not verified: pieces and status on a real screen (needs T-053); the door with two copies; the new must-fail tests inside `./scripts/verify` (request 1).
