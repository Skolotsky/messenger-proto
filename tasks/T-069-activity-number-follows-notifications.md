# T-069: The Activity number follows the notification setting

- Traces to: brief (activity feed, product craft), D-036, D-012, D-019
- Started: 2026-11-23

## Outcome

Someone on Focus only is called by Focus alone, and still sees a quiet dot when Needs you has something unread.

## Acceptance criteria

- [x] Focus and Needs you: the number is their sum. Check: `activity-count.spec.ts`, wide and phone.
- [x] Focus only: the number counts Focus, with no dot beside a number. Check: same spec.
- [x] Focus only with Focus empty and Needs you unread: a dot and no number. Check: same spec.
- [x] Changing the setting updates the mark at once. Check: same spec.
- [x] A person on Focus only never sees the larger number while the page loads. Check: same spec, with a delayed settings answer.

## Verification plan

- Automated: browser tests at both widths, seen failing first; `./scripts/verify`.
- Independent: no, a small change to one screen mark with no contract change.
- Not covered: a look at the running product by a person.

## Log

- 2026-11-23: decided with the user (D-036).

## Evidence

`activity-count.spec.ts` (6 tests at both widths): 8 of 12 runs failed before the change, all pass after. `./scripts/verify` passed all lanes. Not verified: a look at the running product by a person.
