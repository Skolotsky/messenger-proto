# D-021: Direct messages reach Focus through a list of people that fills itself

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001, D-013, D-020
- Supersedes in part: D-001 (the starting Focus rule "Direct messages from people, one to one" as a single rule)
- Evidence: E-002 (shown in `experiments/E-002-look-and-feel/slack-like.html`: the reason line on Anna's row, and the settings sheet)
- Visual: `experiments/E-002-look-and-feel/screenshots/slack-like-desktop-16-person-removed.jpg`, `slack-like-desktop-17-people-settings.jpg`

## Why this needs deciding now

With D-020 a person can say "not this in Focus" from a row. For a direct message, the only thing to switch off was the whole starting rule, which throws out every direct conversation because of one. The user said it "should work carefully" and gave the shape.

## Options

### A. One rule, on or off

- Upside: nothing to build.
- Risk: one unwanted sender costs every direct message its place in Focus.

### B. A rule plus exceptions ("direct messages, except from Anna")

- Upside: narrow.
- Risk: a second kind of thing, the exception, beside rules.

### C. A list of people, filled automatically (the user's proposal)

- Upside: only one kind of thing, a person in Focus, which D-001 already has as a rule. Removing one person is removing one entry.
- Risk: the list grows without the person seeing it. Reduced by showing it in settings with a count per person.

## Best practice

Make the narrow action the default and the broad one deliberate; confirm what affects many things, and for a single reversible thing act at once and offer undo.

## Recommendation

The agent had built A in the mock-up. After the user's proposal it recommends **C**.

## Decision

Accepted by the user on 2026-11-22: **C**, in their words:

- "When this option is enabled then everyone once written to you are added to focus."
- "When you disable this option it doesn't mean everyone leaves it automatically. By default this setting will just stop adding new people to focus, also it would ask you if you want to remove all already added people from focus. You can always remove them all or one by one later in settings."
- "From the message the button also will by default remove this person from focus. It will inform you, but not ask, it comes from your intent."

So:

| Where | Action | What happens |
|---|---|---|
| Settings | The option "Add everyone who writes to me directly" is on (the starting state) | A person is added to the Focus people the first time they write to you directly |
| Settings | The option is switched off | New people are no longer added. People already added stay. The product asks once whether to remove them too; keeping them is the default |
| Settings | Remove one person, or all | Done at once |
| A row (D-020) | "Remove Anna from Focus" | Removes that person only, at once, says so, offers Undo. Nothing is asked |

The agent's readings, shown in the mock-up and open to objection:

1. A Focus person means direct messages from that person land in Focus. It does not lift their mentions or replies elsewhere; that is the separate person rule of D-001 and D-013, added by hand.
2. A person who was removed is not added back when they write again, whatever the option says. Only the user adds them back.
3. Undo is offered wherever nothing was asked.
4. Agents are not added: a direct message from an agent lands in Needs you (D-013).
5. The other starting rule, "Flags that name you", stays a single rule.
6. A row already on screen does not move; the change applies the next time the feed is opened (D-013).

## Consequences

- Easier: "not this" from a direct message costs one click and one person.
- Harder: the product keeps, per reader, the people added, the people removed, and the state of the option. The seeded team has to start with a believable list.
- Committed to: Focus people as a per-reader list; the option; ask-before-many, act-at-once-for-one.

## Follow-ups

- The architecture draft (T-022, D-017) should count Focus people among "what a reader did".
- E-001's model treats direct messages as one rule; it needs this before its numbers are quoted again.
