# D-035: A reaction reads only up to its message; retries pause at most 20 seconds; levels change inside a conversation

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-006, D-007, D-014, D-015, D-023
- Narrows: "your own reaction ... marks the row read" in D-014, and the reaction part of acting implies read in D-015 and D-031
- Evidence: the T-046 and T-047 reports; the contract's 30-second catch-up against its 60-second retry pause; the T-055 report

## Why this needs deciding now

The feed service (T-052) builds on the first point and the delivery promise rests on the second. The third is a control D-001 and D-023 promise that no package was assigned to build.

## Decision

The user accepted all three recommendations on 2026-11-22 ("Accept all three").

1. **Reacting to an old message reads only up to that message.** Newer messages on the row stay unread, because D-015 says nothing is read while the person is not looking. A reply, Keep and Done still read the whole row.
2. **The pause between retries to a receiver that is down is at most 20 seconds**, down from 60, so a receiver coming back can be level within the 30 seconds G-006 promises. Pauses still grow 1, 2, 4 … and vary at random, and retrying never stops.
3. **The channel and thread level control inside a conversation** is built in the second half of T-056.

## Options considered

- Reaction: reading the whole row is simpler, but marks messages the person never saw as read.
- Retry: probing cheaply between tries keeps the 60-second pause, at the cost of a second kind of call. Capping the pause costs a few more failed calls during a long outage.
- Level control: leaving it in settings and the row's reason panel contradicts D-001 and D-023.

## Consequences

- A row can stay unread after its reader reacted to an older message in it.
- During a long outage each receiver gets at least three tries a minute, shared by all workers.

## Follow-ups

Reaction rule: built with this decision. Retry cap: built after T-064 merges, because T-064 is changing the delivery code. Level control: T-056, second half.
