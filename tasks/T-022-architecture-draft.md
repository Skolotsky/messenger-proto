# T-022: A first draft of the overall architecture

- Phase: decide (prepares D-005 to D-008), with research and visuals as needed
- Asked by the user on 2026-11-22, to be done in a separate session from the one that prepared it
- Depends on: the brief; decisions D-001 to D-004 and D-009 to D-016; R-002; E-001
- Runs alongside T-019 (look and feel). Details that depend on the mock-ups come after them.

## Why now

The user held the technical decisions until the product scope was final and the experiments agreed. The product scope is now decided. On 2026-11-22 the user allowed a draft of the overall architecture to start before the look-and-feel experiment finishes: "details will come after visual experiments, but I guess we can already have a draft for the overall architecture". This task is that draft. It prepares decisions; it does not make them and it writes no product code.

## Outcome

The user can look at one picture of the system and a handful of walk-throughs, understand which parts exist, what each promises the others, and what happens when one of them is slow or down, and can then take the technical decisions one at a time with real options in front of them.

## What the product decisions demand of the architecture

Read the records themselves; this list is a map, not a substitute. Each point names where it comes from.

1. **Three independent receivers, plus agents.** Message changes must reach a search index, a notification sender and the activity feed, across boundaries treated as network boundaries, where any of them can be slow or unavailable (brief). Search and notifications may be local fakes behind real boundaries (brief). Agents receive message events too and are in effect a fourth receiver (D-002; D-008 is its contract).
2. **Every kind of change travels, not only new messages.** Create, edit of text, edit of mentions, edit of level (Flag, Info, none), delete, reaction added and removed (D-001, D-004, D-011, D-014). Search finds an edited message by its new text only and a deleted one not at all (D-004).
3. **The feed is state per reader, not a copy of messages.** Where a message lands depends on the reader's Focus rules, channel dial, thread dial and thread participation (D-001, D-013). It changes when the reader changes a rule or dial: unread rows re-sort (D-013). One row per conversation (D-001).
4. **Read is one position per person and conversation, shared by the feed, the channel unread marks and notifications, on every device** (D-015). What a retraction leaves behind depends on whether that reader had read the row at that moment (D-004). So the feed needs read state, and where read state lives is a design question.
5. **Notifications depend on the feed's state.** A notification is sent when a row becomes unread in Focus or Needs you, once per row, and once more if it moves up into Focus; nothing for Info, for streaming text, or for a conversation the person is watching (D-012, D-015). The brief lists the notification sender and the feed as independent systems that both receive messages. These two facts pull against each other, and the draft has to say how it reconciles them: who decides that a notification is due, and what the sender is promised.
6. **Kept is per reader, with two sources.** The reader's own Keep, and a flag that names them; removing the flag or deleting the message releases what the flag kept (D-003, D-011). A flag has no shared state anywhere (D-011).
7. **The screen never jumps, and it survives a dropped connection.** Changes reach every open screen live (D-009); the list does not move while viewed and new activity is announced, not inserted (D-003); edits and deletes keep their space (D-001, D-004). R-002 found that products pair a live connection with numbered updates and a catch-up rule.
8. **Agents come through a door a real agent could use** (D-002): they post, stream a reply, set Flag or Info, and receive events. Scripted agents, scripted people, and a coding agent joining as either, all use the same doors. Acting on behalf of a person is a demonstration feature that must be off unless switched on.
9. **A control panel outside the product** (D-010): start, stop, reset to the same starting state every time, a schedule that can be shown in advance, each seeded situation on demand, a pace control, and controls to slow down or switch off the search index and the notification sender.
10. **Text rules judged by a language model** arrive last (D-001): they may only promote an item into Focus, after it is already shown. The model is one more slow, unreliable system outside ours.
11. **A web app installable as a PWA, at phone and desktop width** (D-009). No access model, no sign-in: a person switcher (D-009).
12. **A busy seeded team**: about 30 people, 6 agents, 20 channels, two weeks of history (D-010).
13. **History tabs and search** read the current truth (D-016, D-004).

## What E-001 teaches the architecture

`experiments/E-001-feed-rules-model/` is a runnable statement of the feed rules, checked twice by independent agents, with 123 scripted cases. Use it as the reference for what the feed must compute and as a source of test cases. Product code must not import it.

- The feed's answer for one message and one reader needs: the message's current version, the reader's rules and dials, who took part in the thread, and the reader's read state. Any design has to say where each of those lives and how the feed gets them.
- The same situations the model plays (`scenarios.py`) are candidates for the product's own behaviour tests.
- Its README lists nine behaviours still chosen by the model alone. They do not block the draft. If one turns out to change the design, raise it.
- Numbers for sizing arguments (assumed traffic, proportions only): a person gets about 4 new Focus rows, 20 Needs you rows and 15 For your information rows a day; about 29 notifications a day under the accepted default.

## Established practice to start from

From R-002, to be named in each decision brief with the reason if the draft recommends otherwise:

- Save the message and a record of the change in one step, then deliver from that record (transactional outbox).
- Deliver at least once; receivers ignore repeats and stale versions. Nobody promises exactly-once across a network.
- A version per message that rises with every change; a delete is a marker that wins over any late edit.
- One cursor or queue per receiver, so a slow one does not hold back the others. Timeouts, backoff with jitter, a parking place for changes that keep failing, a way to rebuild a receiver from the source.
- A live connection with numbered updates, and a catch-up rule after a drop.
- What real boundaries promise: Slack's Events API (answer within 3 seconds, 3 retries, a unique ID per event).

## What to produce

1. **A system map** (visual): the parts, the boundaries between them, what moves across each arrow, and the failure path. Plain language, no class or file names.
2. **Walk-throughs** (visuals, steps over time), at least:
   - a message is sent and reaches search, notifications, the feed and an agent;
   - a message is edited so that it gains a mention, and then deleted;
   - a receiver is down for a while and recovers, with what a person sees meanwhile;
   - the same change is delivered twice and out of order;
   - a screen loses its connection and catches up;
   - a person changes a dial and their unread rows re-sort;
   - an agent streams a reply and then stops with a flag.
3. **The technical decisions, prepared as proposals** with real options, the established practice, a recommendation and its reversibility, top-down:
   - D-005 system shape and stack (one deployable or several; the source of truth; how live updates and catch-up reach the screen). Note the environment: Docker and Python 3.12 are present, Node and Go are not; check before assuming.
   - D-006 the delivery promise to each receiver (loss, repeats, order).
   - D-007 quality targets (freshness, retry and give-up limits, what a person sees during an outage, how a receiver is rebuilt), each with the failure a test will inject.
   - D-008 the contract for agents as receivers and senders.
   - Any further decision the draft finds it needs, registered as `open` in the index. Expect at least: where read state and per-reader feed state live; how the notification sender learns that a notification is due (point 5 above).
4. **For each part, a short answer to "what would change at a hundred times this size"**, even though we do not build that. One or two sentences each. The point is to know which choices are deliberate simplifications.
5. **A list of experiments** the draft cannot do without, each with the question it settles. Candidates from R-002: can a polling relay keep search and the feed within a second or two; should streamed agent text pass through the reliable path per chunk, coalesced, or only when finished.
6. **A "known shortcuts" list**, started here: every place the draft knowingly cuts a corner, with what the full answer would be.
7. **What waits for the look-and-feel experiment** (T-019), listed so nothing is decided twice: anything shaped by how the screens turn out.

## Boundaries

- No product code, no dependencies installed, no stack chosen by doing. An experiment needs its own task and the user's go-ahead.
- The user decides system shape, contracts between parts and qualities. Prepare one decision at a time, top-down, and ask one question at a time. Nothing in D-005 to D-008 is accepted by this task.
- The brief does not change. If a product decision turns out to be impossible or very costly to honour, say so with the reason and the options; do not design around it silently.
- Contracts between parts are written as contracts (what is sent, what is promised, what happens on failure), in `docs/contracts/` once a decision accepts them, not before.

## Acceptance criteria

1. The system map and at least the seven walk-throughs exist under `docs/visuals/`, self-contained, and each was looked at rendered if a browser is available to the session (say so if not).
2. D-005 to D-008 each have a record with status `proposed`, real alternatives, the established practice named, a recommendation, and reversibility. Further decisions found are in the index as `open`.
3. Every point in "What the product decisions demand" is traceable to a part of the draft, or is listed as unresolved with the reason.
4. Every reliability claim in the draft names the failure a test will inject to prove it.
5. The larger-scale notes, the experiment list, the known-shortcuts list and the waits-for-look-and-feel list exist.
6. A fresh `verifier` agent, given the brief, the decision records and the draft but not the drafting session's reasoning, has checked points 3 and 4 and its findings are answered.
7. `./scripts/check` passes; the board and the decision index are current.

## Verification plan

- Trace each of the thirteen demands to the draft in a table; the verifier repeats this independently.
- Play five of E-001's scripted cases by hand through the walk-throughs (an edit that adds a mention; a delete of a kept row; a flag removed after it was read; a reaction from someone addressed; a dial change with unread rows) and confirm the draft has a place where each rule is applied.
- For each receiver: state what it holds, how it would be rebuilt from nothing, and which test shows that.
- The user reviews the system map first, then each decision in order.

## Notes for the session that picks this up

- Use the `decide`, `visual` and `research` skills. Hand lookups to the `scout` agent; keep the analysis on the main model (CLAUDE.md, rule 10).
- R-002's sources were read by cheap agents and only partly re-checked. If a design choice rests on a specific claim there (a retry schedule, a product's behaviour), re-check it.
- The user is not a spectator of code: lead with the picture and the question, keep detail in the repo.
- Ask before pushing, installing or publishing anything.

## Log

- 2026-11-22: task prepared. Not started.
- 2026-11-22: drafted in a separate session. The user asked mid-way for the proposal as one HTML page with schemes, snippets and the technical decisions; the system map, the decision options and the lists (experiments, shortcuts, waits, demand trace) are therefore one page, `docs/visuals/T-022-architecture-proposal.html`, with the walk-throughs beside it. No separate draft document was written.
- Second opinion (fresh agent, options without recommendation): agreed on every choice (separate programs over HTTP, PostgreSQL, a worker per receiver in log order, the core owning reader facts, the feed deciding notifications, WebSocket, agents reading from their own place, streamed text off the log). Its additions, all taken up: test the separate-programs mode; log numbers must follow commit order; the screen shows the person's own action without waiting for the feed; say plainly that the sender stands behind the feed; notification names and a sent list so a rebuild never re-sends; model verdicts saved as facts; an ID on agent posts; a timeout that closes a dead stream.
- Claims re-checked against primary pages by a scout: the six-connection limit of server-sent events outside HTTP/2 (MDN, confirmed); Slack Socket Mode, app opens the connection and acknowledges each event (confirmed); PostgreSQL NOTIFY is delivered on commit (confirmed); sequence values are not rolled back and leave gaps (confirmed). That sequence order can differ from commit order is not stated on the pages read; it follows from values being handed out before commit and is to be shown by the "spine" experiment's concurrent-writer test.

- Independent verifier (fresh agent; brief, product decisions and the proposal, no drafting reasoning): criteria 3 and 4 not met at first. Nine findings, all answered in the pages and records: (1) the feed dropped an out-of-order change as "not newer": it now takes only the next number and answers anything else with its position; a "withdrawn" before its "due" is remembered. (2) "At most once" for notifications holds only because the tray is the fake's own store: said so, with the rule for a real provider. (3) The sender had no rebuild: it refills from the feed's log in a quiet mode; reset empties every part; a rebuilt feed starts a new edition of its log. (4) "Never for something deleted or read meanwhile" was a race against the hold: reworded; watching a conversation is now a fact on the log, so D-015 rule 6 does not depend on the hold. (5) D-012's second notification on moving into Focus and the "Focus only" switch were missing: added. (6) A parked change broke "equal to the truth": release path and test added. (7) Five demands only gestured at: a table added (kept row deleted, flag removed after read, reaction as answer, control panel, seed, history tabs). (8) Ten claims without an injected failure: rows added to D-007. (9) This log entry was missing.

- 2026-11-22, later: the user decided D-005 (shape B without a broker after first choosing one, PostgreSQL in Docker, WebSocket, TypeScript), D-017 and D-018, asked for a smaller blast radius when the feed chokes on a change (fencing, tried in E-003), and asked for every experiment needed to settle the design (E-004 PostgreSQL numbering, E-005 the screen, agents and a full-size rebuild in E-003). The look-and-feel decisions D-019 to D-023 were checked against the design and fit. The user then delegated D-006 to D-008 to the agent. Task closed.

## Evidence

- Rendered with `./scripts/browser` (headless Chromium) at 1100px light and 400px dark: neither page scrolls sideways; the map, D-006 and D-008 sections were looked at as screenshots.
- `./scripts/check` passes.
- Demand trace: the collapsed table at the end of the proposal page. Demand 11 (installable web app at two widths) is marked not designed yet; it waits for T-019.
- Independent verifier on criteria 3 and 4: findings and answers in the log. The corrections were not re-checked by a second verifier.
