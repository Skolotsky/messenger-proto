# D-009: Which surfaces the first version includes

- Status: accepted; superseded in part (the notification tray: D-022)
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: D-001
- Evidence: R-002
- Visual: `docs/visuals/D-009-first-version-surfaces.html`

## Why this needs deciding now

D-001 says what the feed does. This decides how much product surrounds it in the first version: the screens and controls a person can actually use. It sets the size of the build, and the remaining product decisions (D-002 agents, D-003 feed detail, D-004 edits and deletes) are designed against these surfaces.

Not open here, because D-001 and the brief already require them: channels with messages; threads; mentions; reactions; editing and deleting a message; the feed with three buckets and a reason on every item; Flag and Info when writing; the channel dial and the thread dial; adding a Focus rule in one click; agents as participants in conversations; changes arriving live without a reload.

## Options

### A. Feed only

The required list above, plus a way to look at the product as different people (a person switcher, no sign-in). Nothing else.

- Upside: the smallest build; all effort goes to the feed and to delivery.
- Risk: three things D-001 promises cannot be shown. Without direct messages the first Focus starting rule has nothing to act on. Without a search box and a view of notifications, two of the three systems the brief names are invisible: nobody can see that an edit or a delete reached them.

### B. Feed plus what makes it observable

Everything in A, and:

- **One-to-one direct messages,** built as a two-person conversation on the same machinery as a channel.
- **A search box** with a plain results list, enough to see that a new, edited or deleted message is reflected.
- **A notification tray:** what the notification sender delivered to this person, as a visible list. It stands in for push and email.
- **A Focus rules list:** see, remove and preview the rules, including the count before saving.
- **Unread marks on channels** in the sidebar, so the product reads as a messenger and not only a feed.

- Upside: every promise in D-001 and every system in the brief can be seen working by a person, not only by a test.
- Risk: five more surfaces to build and keep correct under edits and deletes. Kept in check by making each one plain: no ranking in search, no settings in the tray.

### C. A fuller messenger

Everything in B, and: creating channels and inviting people, group direct messages or ad-hoc topics, profiles and presence, file attachments, formatted text, a small-screen layout.

- Upside: feels closest to a product someone could move a team onto.
- Risk: most of it does not exercise the feed or the delivery guarantees, and each surface multiplies the cases for edits, deletes and "nothing jumps". The bet in D-001 would be tested no better than in B.

## Best practice

A proof of concept includes what the hypothesis needs to be tested and observed, and fakes or omits the rest. Where a system exists only behind the scenes, give it the smallest visible surface that lets a person confirm it works.

## Recommendation

**B.** It is the smallest set in which everything D-001 promises, and all three systems in the brief, can be seen by a person using the product. A leaves two of those systems invisible; C spends effort where the bet is not.

Out of the first version under B, to be listed as known shortcuts: sign-in and accounts (a person switcher instead); creating or managing channels (they come seeded); group direct messages and ad-hoc topics; profiles, presence and typing indicators; files and formatted text; a small-screen layout; settings beyond the dials and Focus rules.

Locks in: the list of surfaces the other product decisions design against. A control to slow down or switch off the search index and the notification sender, to show the guarantees, is needed too, but belongs to the quality decisions (D-007) and is not decided here.

Reversibility: cheap. A surface can be added or dropped until it is built.

No second opinion was sought: cheap to reverse.

## Decision

Accepted by the user on 2026-11-22: **B, with two additions.**

- **A small-screen layout is in.** "Mobile is super important." Taken to mean the web product laid out for a phone-sized screen, not a separate app.
- **Creating channels is in.** "Otherwise it would look strange: entity which exists but can't be created." Taken at its plainest: a name and the people to add. Private channels, renaming and archiving stay out.

Both were listed under C and under "out of the first version" above; the user's choice overrides that. The rest of the out list stands: sign-in and accounts, group direct messages and ad-hoc topics, profiles, presence and typing indicators, files and formatted text, settings beyond the dials and Focus rules.

### Clarified by the user, 2026-11-22 (added after acceptance; the decision itself is unchanged)

- Web only, delivered as a PWA: a web app that can be installed on a phone or desktop and opens full-screen like an app. Taken to mean installable and laid out for a phone; working offline and real push notifications are not part of it (the notification tray stands in for push).
- The access model as a whole is out of scope: no private channels, roles or permissions. Every person can see and join every channel.

## Consequences

- Easier: every promise in D-001 and every system in the brief has a visible surface. The product can be shown on a phone, where a feed of "what needs you" is most natural.
- Harder: every surface has to work at two widths, and "nothing jumps" has to hold on the narrow one, where an expander or a deleted-message marker takes a larger share of the screen. Channel membership can now change while the product runs, so "everyone in the channel" for a flag is not a fixed list.
- Committed to: the surfaces in B, creating channels, and a layout that works on a phone-sized screen from the first user-facing task on (which makes T-004, a browser the agent can drive, more pressing).

## Follow-ups

- D-010 (the seeded team: how many people and agents, which roles, how busy) once this is accepted.
- Then D-002, D-003 and D-004 against the accepted surfaces.
