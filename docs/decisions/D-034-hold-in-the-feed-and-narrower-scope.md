# D-034: The notification hold moves into the feed; scope and claims are narrowed

- Status: accepted
- Level: contract
- Date: 2026-11-22
- Decided by: user
- Depends on: D-012, D-014, D-018, D-029, D-030, D-031, D-032, D-033
- Supersedes: the hold in D-018; "a refused note stops that organization's notes" in D-032; "a fence writes `withdrawn`" in D-031; the coding-agent connector and acting as a person in D-002 and D-008; text rules judged by a model in D-001; the "about 100,000 people" figure in D-030
- Evidence: the contracts, the feed rules and the delivery workers as built, read against this decision on 2026-11-22

## Why this needs deciding now

The hold splits one judgement across two parts: the feed decides what is due, and the sender decides when. Moving it into the feed leaves the sender only delivering. T-052 builds the feed service on whichever form is chosen, so this comes first. The other points remove work and claims the project should not carry.

## Decision

The user's decision, 2026-11-22.

### 1. The 5-second hold moves into the feed

D-012's behaviour is unchanged.

- When a row becomes due, the feed keeps a held entry in its own tables, in the same transaction as the row.
- A read, a delete or a fence removes the held entry. Nothing is written to the note log for it.
- A clock tick from the log releases entries more than 5 s old by the log's timestamps: the feed writes one `send` note with the final text and the same fixed name. A machine's clock is never used.
- After sending, an `update` note carries the current truth ("read", "message deleted", new text) for the notification app's list. An update for a name the sender does not know yet is remembered, not lost.
- The sender only delivers, once per name, and keeps its list of sent names. The hold, its clock, and the `due`, `changed`, `withdrawn` and `clock` notes are removed from it.
- A parked note no longer stops that organization's notes, because no later note can release a hold. `stopped_at` is removed.
- Every promise stays: nothing sent for something read or deleted within the hold, including when the feed is late or fenced and when the clock steps back; within 8 s when healthy; once per name after restarts, rebuilds and a wiped sender; "delayed" after 10 minutes.
- G-010 and G-011 keep what they promise. Their hold cases move to the feed and keep their must-fail variants.

### 2. Out of scope

The coding-agent connector (T-060) and text rules judged by a model (T-062) move to Backlog as "not planned", and T-061 no longer needs them. The connector and "acting as a person" leave the agent-door contract. Scripted agents and the agent door stay. Text rules do not appear in the product's interface.

### 3. No scale testing embedded in the product

- G-028 and the two-database part of T-057 are removed. The written analysis in D-029 is enough. The one function that picks an organization's database stays.
- G-029's test uses 50 and 500 idle organizations instead of 20,000, and checks that rows read do not rise.
- G-027 (two copies) stays: it tests failure recovery, not size, and it already found a real defect.

### 4. The capacity statement

The "100,000 people" figure is withdrawn. The only capacity statement is "about 5,000 people in one organization, estimated by analysis, not measured". Larger organizations have a path with no number: conversations applied in parallel, grouped saves, big-channel items stored once, and batched calls to search. The D-030 record and `tasks/done.md` stay as history.

### 5. Simulated figures are never validation

"29 notifications a day" and similar E-001 numbers may appear only as a sanity check from a simulation with assumed traffic. Nothing in the README, the product or the final validation presents them as evidence that people want this feed.

## Readings the user confirmed ("Accept all five", 2026-11-22)

1. **Ticks are timed from the previous tick:** about one a second while an organization is active, until a tick stamped at least 6 s after the last change that was not a tick. Timed from the newest entry, a busy organization would never get a tick and no hold would ever be released. This costs one small log entry per second per active organization.
2. **More than read, delete and fence cancels a hold:** a mention edited away, a reaction taken back and a dial change also remove the held entry, with nothing written to the note log.
3. **A rebuild after a rule change** marks its `send` notes `history: true`: the sender lists them and never delivers them (D-032).
4. **Superseded parts** are those named at the top of this record.
5. **The check protecting the database function stays** as a plain lint in `./scripts/check`, with no guardrail and no second database.

Consequences the user did not object to: the `text` rule type leaves the schema; G-029 drops its timing limit and judges rows read only; `docs/visuals/T-034-guardrails.html` is corrected for the withdrawn figure.

## Consequences

- Easier: the sender is a plain deliverer, and a parked note holds nothing back.
- Harder: the feed owns held entries, which must rebuild exactly from the log.
- Committed to: a tick about once a second per active organization.

## Follow-ups

T-065 applies this after T-064 merges and before T-052: contracts, schemas, guardrails, board and build plan agree with it; each moved hold test is seen failing with its rule off; searches for `stopped_at`, the removed note kinds, G-028 and "100,000" show only history; a fresh verifier checks it against this decision.
