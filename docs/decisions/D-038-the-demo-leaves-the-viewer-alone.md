# D-038: The scripted team never acts as the person being watched

- Status: accepted
- Level: product
- Date: 2026-11-23
- Decided by: user
- Depends on: D-002, D-010

## Why this needs deciding now

The script read and answered rows as Maya while the user watched as Maya, so rows looked read without anyone touching them.

## Decision

No live step acts as a situation's viewer. What a situation builds on (the viewer's own messages and levels) stands in the seeded history from Monday evening. Answering, reading and closing are left to the person.

## Consequences

Flags stay in Focus until the person acts. The deleted-message situation shows the marker for Lena, who read it, and nothing for Maya. Loading content refuses a step that acts as the viewer.
