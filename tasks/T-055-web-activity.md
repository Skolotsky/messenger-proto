# T-055: Web app: Activity

- Traces to: brief (activity feed, product craft), D-001, D-003, D-004, D-011, D-012, D-014, D-015, D-016, D-019 to D-023, D-032, D-034; T-038 points 12 to 18; contracts `screen-to-conversations.md`, `door-reads.md`
- Started: 2026-11-22

## Outcome

In the real web app, at 1280 and 400 px, light and dark, a person can work through Activity: three buckets and Kept, rows that explain themselves and can be changed from the row, arrivals that wait behind Show, history tabs, one settings screen, the full reaction picker, and on a phone a React button and two swipes.

## Acceptance criteria

Every check is in `tests/browser/specs/activity-*.spec.ts` and runs at both widths unless it says otherwise.

- [ ] Three buckets, newest first, with Kept in Focus, collapsed past five (D-003; T-038 16, 17). Check: `activity-feed`
- [ ] A row's reason opens what it is, what put it here, where it would go otherwise, and one button that acts at once, says so and offers Undo; the panel never reads the row (D-020, D-021). Check: `activity-reason`
- [ ] Arrivals wait behind Show, count rows and not messages, queue across batches, bring back a row marked Done, and raise the count while Activity is closed (D-023; T-038 12, 13, 15). Check: `activity-arrivals`
- [ ] Nothing being read moves on an arrival, an edit, a delete, a keep or a hover (under 0.5 px). Check: `activity-jumps`
- [ ] A click opens the conversation beside the feed (1280) or over it (400), at the first new message, and reads the row (D-023, point 21). Check: `activity-open`
- [ ] Four history tabs with unread counts; reading leaves an entry in place; arrivals reach them (D-016, D-019; T-038 14). Check: `activity-history`
- [ ] One settings screen: general rules, Focus people, channel levels, notifications, writing; what is removed can be added back (D-021, D-023). Check: `activity-settings`
- [ ] The full picker with search, on a message and on an unopened row; the true count per emoji (D-023, D-014; T-038 18). Check: `activity-reactions`
- [ ] On a phone: a visible React button, swipe right marks read (Done on a kept row), swipe left keeps (D-023). Check: `activity-phone`

## Verification plan

- Automated: Playwright through `web/app/scripts/browser-tests`, against the real mock door where it can produce the state, and typed fixtures valid against the schema where it cannot.
- Real system: the app used in a browser; screenshots at both widths and both themes.
- Independent: no; the milestone verifier comes at T-058.
- Not covered: the real feed service (T-052, T-058); offline and waiting states, mentions, search (T-056).

## Log

- 2026-11-22: started.

## Evidence

_To fill in._

## Readings for the user

_To fill in._

## Requests for shared changes

_None yet._
