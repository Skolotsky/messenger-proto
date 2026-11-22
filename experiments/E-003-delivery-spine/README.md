# E-003: Does the proposed delivery design keep its promises when its parts are killed, slowed, repeated and wiped?

- Date: 2026-11-22
- Time box: one working session
- Feeds: D-005, D-006, D-007, D-017, D-018 (all still to be decided by the user). Asked for by the user before reading the proposal.
- Visual: `docs/visuals/E-003-delivery-spine.html`

## Hypothesis

The design in `docs/visuals/T-022-architecture-proposal.html` (save and log in one step; a worker per receiver reading the log from its own place; receivers that ignore repeats and older versions; a feed worked out from the log; the feed deciding notifications and the sender holding them briefly) loses nothing, repeats nothing visibly and ends equal to the truth under every failure D-007 lists. We expected the mechanics to hold and the notification hold to be the weak point, because the independent check of the proposal called it a race.

## Method

A throwaway spine in plain Python (standard library, SQLite), started as six kinds of separate processes that talk only over HTTP on this machine: core, delivery workers, a search stand-in, the feed, a notification sender stand-in, and a troublemaker that sits between a worker and a receiver. A scripted team of 12 posts, replies in threads, mentions, flags, edits, deletes, reacts, reads, watches and turns dials, at 120 to 300 actions a second. The harness kills processes for real, then compares every receiver with the truth.

- `spine.py`: the parts. The feed's rules are a simplified subset of E-001's, chosen so that order matters (read against delete, edit against delete, dial against arrival). The same rule function runs inside the feed and in the checker.
- `run.py`: eleven scenarios and seven checks. "The feed equals a clean replay of the log" compares the live feed, entry by entry, with one computed afresh from the core's log.
- Four variants are broken on purpose, to see each check fail once.

One command reruns everything, in about six minutes: `./experiments/E-003-delivery-spine/run.sh`

Timings are scaled down: a delivery call may take 1 s (proposal: 3 s) and a notification is held 1 s (proposal: 5 s).

## What would change our mind

- A saved change missing from a receiver after any failure, or a receiver that differs from the truth once level: D-006's promise does not hold as designed.
- The feed differing from a clean replay: D-017 (the feed is worked out, never typed in) is not achievable this way.
- A stopped or slow receiver delaying another: the worker-per-receiver shape is wrong.
- Notifications going out for things read or deleted within the hold: D-018 needs a different mechanism.

## Result

Final run, 2026-11-22, Python 3.12, WSL2, 16 cores. Raw output: `results.txt`, `results.json`. **110 checks, none unexpected** in the default run (75 at first; the fence and agent scenarios were added later the same day), plus 7 in the separate full-size run.

### What held

| Scenario | Outcome |
|---|---|
| Healthy burst, 2,941 changes at 300 actions a second | every check holds; save to search 6 ms median, 11 ms for 19 in 20; save to feed 7 ms and 12 ms; a notification leaves 30 ms (median) after its hold ends |
| Every delivery disturbed (15% late stale copies, 15% lost answers, 20% delivered twice) on all three hops | every check holds; level after 47 s |
| Search stopped 5 s mid-burst | every check holds; the feed stayed at 7 ms median, 18 ms worst; search cleared 661 waiting changes 0.6 s after it came back |
| Search answering after 1.5 s (over the limit) for 4 s | every check holds; feed unaffected (17 ms worst) |
| Search refuses one delete | parked; "search equals the truth" fails while parked, as it must; holds after "try again" |
| Search refuses everything | the worker pauses with 2 parked and 111 waiting, then recovers; every check holds |
| Core killed five times mid-burst | no acknowledged action lost, none saved twice; every check holds |
| Delivery, feed, sender and search each killed mid-burst; the feed worker's place moved forward by 40 by mistake | the feed answers with its real place and the worker goes back; every check holds |
| Feed wiped mid-burst | rebuilt while new changes kept arriving, level 1.4 s after the wipe; equal to a clean replay; no notification repeated |
| Then the sender wiped | tray refilled from the feed's log: 565 entries before, the same 565 after |
| Then search wiped | 2,439 changes refilled in 1.6 s; equal to the truth |
| The feed cannot apply one edit and one dial change (faults planted) | only that conversation and that reader are fenced: 51 rows inside, the other 452 rows and all 910 notification notes outside equal a clean replay; 38 of 1,772 changes set aside; after the fault is removed and the feed rebuilt, everything equals a clean replay and no notification repeats |
| The feed cannot apply any edit | after five fences it stops instead of fencing everything; whole again after the fix and a rebuild |
| Scripted agents at the agent door (three agents; one away for 3 s; each "crashing" now and then before noting its place; ordinary traffic meanwhile) | 183 events: none missed, in order; 183 replies saved, 46 repeats recognised by their ID, none missing; 59 replies written piece by piece, none sent to a receiver unfinished, all whole in search; a piece sent twice appears once; 10 abandoned replies closed as "stopped" with the text so far; two agents answering each other stopped after three messages; 20 of 45 accepted from an agent posting too fast; an event reaches a waiting agent in 10 ms (median); all seven receiver checks hold with agents in the mix |
| Rebuild at the seeded team's size, run separately: 36 participants, 20 channels, 48,800 changes, 20,900 messages | equal to a clean replay; **feed rebuilt in 107 s one change per call, 80 s with up to 100 changes per call**; search refilled in 21 s |

### Each check seen to fail

| Broken on purpose | Caught by |
|---|---|
| Search without the version rule, under late copies | 17 of 664 messages differ from the truth |
| A feed that accepts any newer change, with its place moved | 139 of 1,559 feed entries differ |
| A sender with no list of what it sent, with the feed rebuilt | 171 repeats among 709 sent |
| The log entry saved in a second step, the core dying in between | 4 of 635 messages have a change missing from the log |

### Findings that change the proposal

**1. The notification hold must be measured by the log's own clock.** 160 mentions were read or deleted 0.3 s later, inside the hold. Sent anyway:

| How the hold is measured | Feed down for 3 s | Feed slower than the traffic |
|---|---|---|
| From when it happened, on the sender's clock | 88 of 160 | 159 of 160 |
| From when the note reached the sender | 8 of 160 | 147 of 160 |
| By the log's clock: sent only once a later entry or clock tick, stamped past the end of the hold, has arrived through the log | 0 of 160 | 0 of 160 |

The proposal's wording ("a withdrawal that reaches the sender inside the hold cancels") is the second row. The third needs one addition: the core writes a clock tick into the log about ten times a second while there is activity, and the feed passes it on. With it, "not sent if read or deleted within the hold" holds however late any part is, and the tray can be refilled exactly.

**2. Timestamps in the log must never go backwards.** This machine's wall clock steps back by about 0.8 s every so often (measured: one step of 0.811 s in 25 s; WSL2 time sync). Before the core forced its stamps to be non-decreasing, 4 of 617 notifications in a healthy run, and 2 of 494 in a kill run, went out wrongly. Receivers compare log stamps only with log stamps, never with their own clock.

**3. A receiver that insists on order needs a way to say where it is, also when idle.** A wiped sender was not noticed while nothing new arrived, until the worker's idle heartbeat was answered with "I am at 0".

**4. A change the feed cannot apply can be contained** (added after the user asked for a smaller blast radius, 2026-11-22). The first design stopped the whole feed. Now the change is set aside and a fence goes up around the one conversation, or the one reader, it touches; later changes inside the fence wait with it; everything outside stays exact (table above). The price, measured: inside a fence nothing new notifies, and one notification that was already due went out although the person had answered within the hold, because the feed could no longer see that. Past five fences the feed stops: that is a broken feed, not one bad change. Repair is fixing the fault and rebuilding the feed from the log.

**5. Rebuilding the feed at full size takes more than a minute in this build.** 80 to 107 seconds for 48,800 changes, against the "under a minute" proposed in D-007. The product will differ (TypeScript, PostgreSQL, the full rules), in which direction is not known. Two consequences are proposed: the target becomes "under two minutes at seeded size", and Reset in the control panel restores a saved copy of each receiver made once from a rebuild, with the rebuild kept as its own button and a test that the copy equals a rebuild. One unplanned observation while adding batch delivery: a slip in the test code made the feed fail on every change, and the fence rule did what it should: three fences, then a stop, no wrong rows.

Not design findings: three false failures came from the harness itself (two threads creating one SQLite file, ports already used by other programs on this machine, a test fault overwritten by the worker) and were fixed in the harness.

## Conclusion

**The design holds.** Nothing was lost, repeated visibly or left different from the truth under any injected failure, a stopped or slow receiver delayed nobody else, and the feed and the tray can be wiped and rebuilt to the same state. Confidence is high for the mechanism and low for the absolute speeds.

It changed the proposal in three places (above), the first of which decides whether D-018's promise about notifications is true.

What this does not show:
- **PostgreSQL.** SQLite has one writer, so the hazard of log numbers being handed out before a save finishes cannot occur here. `run.py numbering` is only a model of it (a careless reader misses about half the changes; handing numbers out one save at a time misses none). The real check needs PostgreSQL, which is not on this machine and would have to be downloaded.
- **Speed in the real stack.** 6 to 12 ms is local HTTP with SQLite and a worker that looks every 10 ms. It says the 2 s target has a wide margin, not what the product will measure.
- **The rules.** The feed rules here are a subset; E-001 is the evidence for the rules. Equality with a clean replay shows delivery faults do not change the outcome, not that the outcome is right.
- **Screens.** No live connection, no browser, no catch-up after a dropped connection.
- **Real agents, the fast path for a reply in progress as seen on a screen, the model-judged rules, and anything larger than the seeded team.** The agent door was tried with scripted agents only, which is what the first version has (D-002).
- One run per scenario, fixed seeds; the troublemaker's faults are random but seeded.
