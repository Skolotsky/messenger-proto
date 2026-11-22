# D-022: Notifications are shown in a separate mock notification app, not inside the product

- Status: accepted
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-009, D-012
- Supersedes in part: D-009 (the notification tray as a surface of the product)
- Evidence: E-002 (in the Slack-like page the tray read as a second copy of Focus and Needs you; the user asked "what's different between notifications and feed?")
- Visual: none

## Why this needs deciding now

D-009 put a notification tray inside the product to stand in for push, so that the notification sender, one of the three systems in the brief, could be seen working. In the chosen layout it sits beside the feed and shows nearly the same rows. The architecture draft (T-022) needs to know where the notification sender's output is seen.

## Options

### A. Keep the tray inside the product

- Upside: one window.
- Risk: it looks like a second feed, and it presents something as part of our interface that in reality belongs to the phone.

### B. Move the list into the demonstration control panel

- Upside: clearly not the product.
- Risk: mixes "what a person receives" with the controls for running the demonstration.

### C. A separate mock application: a simple web view showing the notifications for a selected person (the user's proposal)

- Upside: the boundary is real: the sender delivers to something outside the product, as it would to a phone. It can be watched side by side with the product.
- Risk: one more small thing to build and keep true under edits and deletes.

## Best practice

Fake an external system behind its real boundary, and give it the smallest visible surface that lets a person confirm it works (the same practice D-009 cites).

## Recommendation

The agent suggested B. After the user's proposal it recommends **C**: it is the more honest boundary.

## Decision

Accepted by the user on 2026-11-22: **C.** "We can make separate mock-application with simple web-view which shows notifications for selected person, cause notifications are not part of your ui."

- The product has no bell and no tray.
- What notifies stays a setting in the product (D-012): the personal switch to "Focus only" lives with the feed settings, which the user asked to be easy to reach from the feed and close to each other.
- What D-004 and D-012 say about a tray entry (an edit shows the new text, a delete shows "message deleted", a read row is marked read) now describes the mock application's list.

## Consequences

- Easier: the feed is the only place in the product that asks for attention. The notification sender gets a receiver that is plainly outside the product.
- Harder: a second small application with a person selector.
- Committed to: no notification list in the product's interface.

## Follow-ups

- T-022 (architecture): the notification sender's receiver is this mock application; D-018 and the delivery contract should name it.
- In E-002's mock-up the list is reached from the Demo controls and labelled as outside the product; the person selector is drawn but fixed.
