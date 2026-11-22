# D-007: Quality targets, and the failure that proves each

- Status: accepted
- Level: quality
- Date: 2026-11-22
- Decided by: agent, delegated by the user
- Depends on: D-005, D-006
- Evidence: R-002; the brief; second opinion and claim checks recorded in `tasks/T-022-architecture-draft.md`
- Visual: `docs/visuals/T-022-architecture-proposal.html`, section 9; walk-throughs in `docs/visuals/T-022-walkthroughs.html`

## Why this needs deciding now

A guarantee without numbers cannot be tested, and D-004, D-009 and D-010 each left a number to this decision: the hold before a notification is sent, and the controls that slow down or stop a receiver. These targets become the guardrails the build is held to.

## Options

The numbers are proposals; each is a dial the user can turn. Measured on one local machine with the seeded team.

| Promise | Proposed | Failure a test injects |
|---|---|---|
| A saved message is never lost on its way out | 0 lost | stop the core right after a save, mid-burst |
| No change is stepped over under concurrent writers | 0 skipped | many writers, one commit held open while later ones finish |
| Other open screens show a message | 1 s | none; measured at fast pace |
| Search and the feed reflect a change when healthy | 2 s, 19 times in 20 | a burst at ten times normal pace |
| Hold before a notification is sent; none goes out for something read or deleted within it | 5 s, measured by the log's own clock (clock ticks in the log; stamps never decrease) | read or delete 0.3 s after a mention while the feed is stopped or slower than the traffic; a backward step of the machine clock (E-003: 0 of 160 wrongly sent; 147 of 160 when counted from arrival) |
| A slow receiver delays nobody else | feed stays within 2 s | search answers after 10 s, or is stopped |
| A receiver that was down ends equal to the truth | 10 minutes of backlog cleared in 30 s | stop search during 10 minutes of edits and deletes; compare index with truth |
| Waiting and retrying | 3 s per call; again after 1, 2, 4 … 60 s with jitter; never give up on an outage | a receiver that accepts and never answers |
| A refused change does not block the rest and is not forgotten | parked after 3 tries, shown with "try again"; receiver not level while anything is parked | a fake told to refuse one delete: "index equals truth" fails while parked, passes after release |
| A broken receiver is not mistaken for a bad change | 3 different refusals in a row: pause | a fake told to refuse everything |
| The feed never skips | stops visibly on a change it cannot apply; the rest runs | feed it a change it rejects; stop the feed: conversations, unread marks and search keep working |
| A retry never posts twice | 1 message | drop the answer to a person's send and an agent's post |
| An agent misses nothing and holds up nobody | events kept 7 days, in order | an agent away ten minutes; an agent that never reads |
| A slow or failing model breaks nothing (built last) | the row stays | a model fake that times out |
| Repeats and late arrivals change nothing | identical end state | a proxy that repeats every delivery and shuffles nearby ones |
| A notification appears in the tray once, late if need be | 1; marked "delayed" past ten minutes | stop the sender for five minutes; stop it between sending and confirming; rebuild the feed while it runs |
| A receiver can be rebuilt from nothing | under 2 minutes at seeded size, equal to before (E-003 at full size: feed 80 s with batches of 100, 107 s without; search 21 s; "under 1 minute" as first proposed was not reached) | wipe the feed, then search, then the notification sender, while the team is active; the sender refills its tray without interrupting anyone |
| A screen that lost its connection ends right | caught up 2 s after reconnecting; a silently lost update healed within 20 s; reload past 500 missed updates | cut the connection for 30 s; swallow single updates; keep a screen away past the limit; compare with a fresh load |
| A surface that is behind says so | notice when more than 10 s behind | any outage above, on the rendered page |

What a person sees: search down, "Search is unavailable right now. Your messages are safe."; search behind, results with "the last N minutes may be missing"; feed behind, the rows it had with "Catching up"; notifications late, once, not for something deleted or read before they went out (a withdrawal that reaches the sender inside the hold cancels; after that the tray entry shows the current truth), marked "delayed" past ten minutes; core unreachable, "Reconnecting", the screen keeps what it shows, a new message waits as "not sent" and cannot post twice.

Alternatives on the two numbers most worth the user's attention:
- Hold before sending: 0 s (instant, but a typo fix or quick delete still notifies), **5 s**, 30 s (calmer, feels broken in a demonstration).
- Giving up: **never on an outage** (the log is kept, the receiver catches up), or after a fixed time (bounded backlog, but then a rebuild is the only repair).

## Best practice

Timeouts on every call, exponential backoff with jitter, a parking place for poison messages, replay from the source, and proving guarantees by injecting faults (R-002: AWS Builders' Library, Jepsen). Slack's Events API expects an answer in 3 seconds and gives up after three retries; we keep the 3 seconds and do not give up, because our receivers are ours to keep consistent.

## Recommendation

The table as proposed. Reset in the control panel restores a saved copy of each part taken after a rebuild (a rebuild at full size takes over a minute); Rebuild is its own control, and a test proves a rebuilt receiver equals the saved copy. Reversibility: cheap; these are settings and tests.

## Decision

**The table as proposed**, with the rebuild target at two minutes and Reset from a saved copy.

Why: each number is either the established figure (3 s per call, Slack's rule; retry with growing pauses and never giving up on an outage, because our receivers are ours to keep consistent) or was measured with a wide margin in E-003 to E-005 (2 s freshness against 7 to 12 ms measured; rebuild 80 s). The hold stays at 5 s: long enough for a typo fix or a quick delete, short enough that a demonstration does not look stuck. These are settings; the two a person will feel are the 5 s hold and the 10 s after which a surface says it is behind.

Decided by the agent on 2026-11-22, delegated by the user: "Ok, make a decision based on best practices and our product needs." The user had asked how many important decisions were left (three: D-006, D-007, D-008) and then gave this instruction; the agent read it as covering all three and said so. The user can overturn any of them; a change is a new record.

### Corrections, 2026-11-22 (added after acceptance; the decision is unchanged)

Found by the independent check of the contracts:
- The table row "The feed never skips: stops visibly on a change it cannot apply" predates the fence the user asked for and D-006 records. It reads: a change the feed cannot apply fences only the conversation or reader it touches; more than five fences stop the feed (G-009).
- "A silently lost update healed within 20 s" needs the screen to be told the last number sent every 10 seconds, not "about every 20"; the contract says 10.
- The log carries a clock tick every second while there is activity, which bounds the hold: a notification goes out within 8 seconds of becoming due when everything is healthy (G-010).
- "Watching" lapses after 60 seconds without renewal (G-024), so a vanished screen cannot leave messages read on arrival.
- Since D-026: Reset, rebuild, a parked change and a paused receiver are per organization; the clock tick is one per quiet period, 6 seconds after an organization's last change, not one a second.

## Consequences

- Easier: "done" for the delivery work is a list of tests that break things.
- Harder: the tests need real fault controls on each fake and a proxy that repeats and reorders.
- Committed to, if accepted: the targets as guardrails, each `pending` until its test exists and has been seen to fail once.

## Follow-ups

- Guardrails G-nnn, one per row, after acceptance.
- Experiments: the spine (the 2 s target); rebuild time for the seeded team.
- Product wording of the notices waits for T-019.
