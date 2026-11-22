# D-017: Conversations owns what a reader did; the feed is worked out from it

- Status: accepted; narrowed by D-026 (the log is per organization)
- Level: system shape
- Date: 2026-11-22
- Decided by: user
- Depends on: D-005; D-003, D-004, D-013, D-015, D-020, D-021, D-023 (what a reader can do)
- Evidence: E-003 (the feed rebuilt equal to a clean replay, also at the seeded team's size; a fenced feed made whole by a rebuild); second opinion recorded in `tasks/T-022-architecture-draft.md`
- Visual: `docs/visuals/T-022-architecture-proposal.html`, section 6

## Why this needs deciding now

Where a message lands depends on the reader: what they read, their dials, Focus rules and Focus people, what they kept. Some part has to hold the original of those. It decides whether the feed can be wiped and rebuilt, and whether "had they read it when it was deleted" (D-004) has one answer.

## Options

### A. Conversations owns the facts; the feed is worked out

Every reader action (read up to here, watching a conversation, Keep, Done, a dial, a Focus rule, a Focus person removed or added back, the option that fills that list, the "Focus only" switch, Undo) is saved in Conversations like a message and travels on the same numbered log. The feed computes rows from the log and holds nothing original. Upside: reads and deletes are in one order, so D-004 has exactly one answer, on every rebuild; channel unread marks and the feed use the same read position (D-015); the feed can be wiped, fenced and rebuilt. Risk: what a person does in the feed goes round through the log before the feed knows it; the screen shows the person's own action at once. Reversal: expensive.

### B. The feed owns reader state

The screen writes reads, dials and Keep straight to the feed; Conversations knows only messages. Upside: the feed answers at once. Risk: the feed holds originals, so wiping it loses what people did; read state has two homes; the order of "read" against "deleted" depends on which call arrived first. Reversal: expensive.

## Best practice

Keep one source of truth and derive views from an ordered log of changes; a derived view can then be rebuilt and compared (event-carried state, projections). This follows it.

## Recommendation

**A.** Things that are worked out and not typed in (a thread's effective level, the reason on a row, the counts, the history tabs, the self-filling list of Focus people from D-021, the answer to "what would this change have moved", D-020) are answered by the feed from its own copy.

## Decision

Accepted by the user on 2026-11-22: **A.** ("Yes, Conversations.")

## Consequences

- Easier: one answer to every "what happened first"; rebuild, fence and reset all rest on it; a second device sees the same state.
- Harder: every reader action is a round trip through the log; the feed keeps its own copy of everything it judges.
- Committed to: reader actions as facts in Conversations on the one log; a feed with no originals.

## Follow-ups

- Contract: Screen to Conversations lists the reader actions (`docs/contracts/`, after D-006).
- Guardrail: the feed never writes to, or reads from, Conversations' store; a rebuilt feed equals the live one.
- E-001's model does not yet have Focus people (D-021).
