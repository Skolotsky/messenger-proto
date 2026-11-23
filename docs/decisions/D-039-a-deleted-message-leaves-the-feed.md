# D-039: A deleted message always leaves the feed

- Status: accepted
- Level: product
- Date: 2026-11-23
- Decided by: user
- Depends on: D-003, D-004, D-011, D-016
- Supersedes in part: D-004 (a marker for a kept row stays until Done)

## Why this needs deciding now

A kept message that was deleted stayed in Kept, counted in its number and asked the person to press Done on something that no longer exists.

## Decision

A deleted message leaves the feed whether it was kept, unread or read. If its row is on screen at that moment, the row becomes a quiet marker in the same place, so nothing jumps. The marker has no Done button and does not count as kept. The next Refresh or reload removes it. A row stays only for other messages that still belong in it.

## Consequences

- Nobody has to close something that is gone.
- The feed's stored rows and contracts do not change: the screen does not lay out a row whose state is `retracted`.
- If a row was retracted by a reaction being taken back and that reaction returns, the row returns as it was.
