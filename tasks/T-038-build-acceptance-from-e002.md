# T-038: Rules the prototype only approximates, as acceptance criteria for the build

- Phase: implement (acceptance criteria for the feed and conversation tasks)
- From: the independent verifier's review of E-002, 2026-11-22
- Decisions: D-001, D-003, D-004, D-011, D-013, D-015, D-016, D-020, D-023, D-024

## Outcome

The prototype fixes the look and the flows. It fakes the rules with a few hand-written lines, so these cases are wrong in it. The product must get each one right, each with a test. Do not copy the prototype's logic.

## Acceptance criteria

**Sorting after a change (D-013 point 3)**
1. Removing "Flags that name you" moves unread flagged rows to Needs you the next time the feed opens.
2. Turning "Any direct mention" off moves rows it brought into Focus back to Needs you.
3. A muted channel keeps only what names you and your own threads: no whole-channel mention, no flag that names nobody.
4. A thread set to Off shows nothing that does not name you, Info rows included.

**One read position (D-015)**
5. Reading a thread beside its channel marks its feed row read at every width.
6. Opening a channel reads the channel's row and clears its unread mark in the sidebar; its threads stay unread until opened (D-032).
7. Mark all read leaves no unread mark behind on a channel with no unread rows.
8. An arrival in a conversation you are watching, with its newest message visible, is read at once: no row, no count.
9. The "New" line is gone once a conversation has been read.
10. An on-screen deleted row no longer counts as unread.
11. Nothing becomes read in a background tab or an unfocused window (not exercised in the prototype).

**Arrivals (D-003, D-016, D-023)**
12. With Activity closed or a conversation open, arrivals still raise the count and the unread marks.
13. No arrival is ever lost: several batches queue, and a new message on a row marked Done brings the row back.
14. Arrivals reach their History tabs.
15. The arrivals control counts rows that changed, not messages.
16. Rows are newest first within a section, applied only when the person asks (D-003).
17. Kept collapses past a handful (D-003).

**Reactions (D-014)**
18. The grouped row always shows the true count per emoji.
19. What a waiting agent does with a reaction is the agent's own decision; the product attaches no meaning to an emoji (D-008).

**Search (D-024)**
20. The count is the true total, not the number shown.
21. Every message is searchable, in channels never opened too.
22. A hit on a thread reply opens that thread at that reply.
23. Text an agent is still writing is not searchable.

**Safety and writing**
24. Names, labels and messages are always escaped when shown. The prototype strips characters from the name field as a stopgap.
25. Saving an empty edit says what happens instead of doing nothing.
26. Edits and reactions made while offline have a defined waiting state; D-024 only defines it for new messages.

## Verification plan

One test per criterion at the product's boundary, plus a check in the running product through the browser.

## Log

- 2026-11-22: filed when the design phase closed.
