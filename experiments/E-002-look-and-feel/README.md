# E-002: Which look and feel carries the three-bucket feed?

- Date: 2026-11-22
- Time box: one session
- Feeds: the look-and-feel direction (a decision record to be written from the user's choice); also asks the user to confirm one reading in D-016
- Task: `tasks/T-019-look-and-feel.md`

## Open the pages

`slack-like.html`, `telegram-like.html`, `own.html` in this folder. Each opens by double-click, needs no server, and resets on reload. All three show the same moment: Larkspur, a 30-person product company two days before its 2.0 launch, Tuesday 10:42, seen as Maya Okafor, the product manager.

The black **Demo** tab on the right edge is not part of the product. It stands in for the team acting while you watch: Tomás edits his message, Jonas deletes his, three new things arrive, you leave the feed and come back. It also switches theme and links the three versions.

## Hypothesis

The feed's rules (three buckets, a reason on every row, Kept, flags, one row per conversation, nothing jumps) can be carried by a familiar messenger layout, so the choice between directions is about emphasis and habit, not about whether the model fits. Expected: the Slack-like layout fits with least surprise, the Telegram-like one is best on a phone and strains on a desktop, and a layout built around the buckets shows the model most clearly at the cost of familiarity.

## Method

One shared script holds the content and all behaviour (`src/core.js`): the seeded rows, read and kept state, expanding, reply and react, dials, Focus rules, history tabs, the tray, and the edit, delete and arrival events. Each direction adds only a page shell and a stylesheet (`src/<name>.js`, `src/<name>.css`). So the three pages cannot differ in what the product does, only in how it is laid out. `build.py` inlines everything into three self-contained pages.

`check.py` drives each page in a headless browser (Chromium through Playwright, in Docker: `scripts/browser`) at 1280×800 and 390×844, clicking through every item of the task's list, and keeps screenshots. "Nothing jumps" is checked by measuring the position of the row or message below the changed one before and after.

One command reruns all of it, from the repo root: `./experiments/E-002-look-and-feel/run.sh`

These are mock-ups. Nothing here chooses a front-end stack or is the start of the product.

## What would change our mind

- A direction that cannot show one of the listed behaviours without bending a decision: that direction is out.
- A direction in which "nothing jumps" cannot be held at phone width: the same.
- If the user finds the unfamiliar direction harder to read at a glance than the familiar ones, familiarity wins over clarity of the model.

## Result

**Every listed behaviour works in all three directions at both widths.** 318 checks passed at the time of the comparison (53 per page and width); with the checks added after the user's review there are 337: `results.json`. No direction had to bend a decision. Screenshots of 15 states per page and width are in `screenshots/` (90 files; light theme, plus the opening screen in dark).

**Nothing jumps.** After the edit to five times the length and after the delete, the row below moved 0.00 px in the feed and the next message moved 0.00 px in the conversation, in every direction at both widths. The longer text waits behind "Show all" and opens only when asked. To see that the check can fail, the size lock was switched off once: the conversation checks then failed in all six runs (the next message moved 77 to 232 px) and the feed delete check failed in several (7 to 18 px). The feed's edit check did not fail without the lock, because a row's preview is cut to a fixed number of lines anyway; in the feed it is the line limit, not the lock, that holds the row still.

**How much one screen holds** (`measure.json`; opening state: 3 new rows in Focus, 2 kept, 6 in Needs you, 5 in For your information):

| | Rows fully on the first screen | Needs you starts after scrolling | Row height: Focus / Needs you / For your information |
|---|---|---|---|
| Slack-like, desktop and phone | 5 | 632 px (under one screen) | 100 / 100 / 100 px |
| Telegram-like, desktop and phone | 5, Focus only | never: it is behind a tab | 83 px in every bucket |
| The agent's own, desktop | 4 | 714 px (just past one screen) | 90 / 80 / 53 px |
| The agent's own, phone | 4 | 822 px (1.2 screens) | 112 / 80 / 53 px |

### What each direction does well for our model, and where it strains

**1. Slack-like** (dark sidebar, dense list, Activity as a view, conversation in a pane beside it)

- Does well: nothing to learn. Channels with their unread marks and dial levels stay on screen beside the feed, so it reads as a messenger. The conversation opens beside the feed on a desktop, so reading a row never loses your place. The four history tabs sit where people already look for them.
- Strains: the feed is one tab in a row of five, and only a small "History" label says the other four are something else; D-016 names exactly this risk. The three buckets are headings inside one long list and every row weighs the same, so For your information looks as heavy as Focus. On a phone the feed is not the first screen: channels and search live under Home and Activity is the second tab, which works against "some people will start from Focus".

**2. Telegram-like** (one list, round avatars, large targets, buckets as folders across the top, bubbles)

- Does well: best on a phone. One bucket per screen makes "one item, one place" literal, the counts on the folders are the summary, and an emptied bucket reads as an empty folder. The feed is the first screen. History is its own place in the bottom bar, clearly not the feed. Direct messages and agent exchanges read pleasantly as bubbles.
- Strains: the reason takes the line this style gives to the message, which leaves one line of about 35 characters for the text; almost every row has to be opened to be understood, and in our model opening is what makes a row leave. Needs you and For your information are invisible until their tab is tapped, so a flag for everyone can wait unseen behind a number. Expanding a row in place fights the habit that a tap opens the chat, and gives bubbles inside a list. The markers a work row needs (channel, flag, Info, agent) crowd the title at 390 px ("2.0 release can…"). On a desktop, bubbles waste width for long work messages and make a thread under a deleted message awkward.

**3. The agent's own** (a calm page; the reason leads every row; a row's size follows its bucket)

- Does well: the reason is the first thing read on every row, in the bucket's colour, which is what "explained rules" asks for. Focus rows show three lines, so many can be dealt with without opening, and fewer rows are marked read by accident. For your information is a digest of single lines at about half the height of a Focus row. One sentence at the top gives all three counts. A flagged row and a kept row share one look, as D-011 intends. History is a separate place. On a desktop the feed is a centred page until a conversation opens beside it.
- Strains: it does not look like a messenger at first glance. Channels are a click away and their unread marks cannot be seen from the feed, which D-009 added so that the product "reads as a messenger and not only a feed". Cards cost height: on a phone Needs you starts 1.2 screens down, further than in the Slack-like version. Needs you rows came out only a little lighter than Focus rows (80 against 90 to 112 px), so the "size follows bucket" idea is clear for the bottom bucket and weak for the middle one. Three row styles are three things to keep still under edits and deletes.

## Conclusion

The hypothesis held: all three carry the model, and they differ in emphasis.

**Recommendation: build on the third direction, and take two things from the others.**

- From the Slack-like version, for desktop: a channel list beside the feed with unread marks and dial levels, so the product reads as a messenger; and its row density for Needs you.
- From the Telegram-like version, for the phone: the feed as the first screen with a bottom bar, large targets, and the three counts as tappable shortcuts to each bucket (the summary sentence can do this), so Needs you is one tap away and not 1.2 screens down.
- Not taken: buckets hidden behind tabs, one-line rows, bubbles for channel conversations, history tabs as siblings of the feed.

Reason: what we are testing in early access is whether people trust a feed that explains itself (D-001). The third direction is the only one where the explanation leads and where a row's weight says how much attention it asks for; its two real weaknesses (not reading as a messenger, a long scroll on a phone) are the two things the other directions do well and can be borrowed.

Confidence: moderate. It rests on one seeded moment, on the agent's own reading of the screens, and on no real users.

**For the user to confirm (D-016):** the pages show the history tabs as the replacement for the single Earlier list. A row that leaves a bucket is found in the tab for its type; there is no Earlier list anywhere.

### Outcome

The user chose the **Slack-like** direction on 2026-11-22, against the recommendation above ("People used to it. We can reuse habits."), and asked for unread counts and marks in the history tabs. Both are recorded in D-019 and the pages now show the counts and marks. The other two pages stay as a record.

### Follow-up asked by the user: why is this row here, and how do I stop it

Three variants live in the Slack-like page behind the address ending `#why=a`, `#why=b` or `#why=c` (without it the page is unchanged): (a) the reason line opens a small panel on the row; (b) the explanation sits inside the expanded row; (c) a "Why? Change…" link on every row opens a side panel with the whole path through the buckets. Each says what the item is, what put it in the bucket, where it would go otherwise, and offers to remove the deciding rule or change the deciding dial, with the count of what that would have moved. `why.py` takes the screenshots (`screenshots/why-*.png`). The page also gained one entry point for feed and notification settings (the sliders button beside the bell). Not covered by `check.py`.

### After the review

The pages now also show what the user decided next: variant (a) as the default (D-020), Focus people for direct messages (D-021), no notification bell, with the notification list reached from the Demo controls as a stand-in for the separate app (D-022), a mark-read button on rows (D-015), and Activity first on a phone (D-019). `check.py` covers these. A further proposal is shown and checked but not yet accepted (T-025): context above the new messages in an expanded row, a long burst showing its six newest messages with the earlier new ones one click above, and an arrow that opens the conversation at the first new message. After that the user set a run of interaction details, recorded in D-023 and shown in the Slack-like page (reactions on any message, a full picker, a phone layout where a tap opens the conversation and swipes give the quick actions, one settings screen, the person's own menu, a closable Activity). The other two pages share the behaviour but not the layout changes. 524 checks in all.

### What this does not show

- No person other than the agent has used the pages. The user's review is the real test.
- Dark theme was looked at on the opening screen only; the click-through ran in light.
- The checks were written by the same agent that built the pages; no independent verifier ran.
- Not shown, though decided: creating a channel (D-009), the person switcher, the control panel (D-010), text still streaming in from an agent, an edit that adds or removes a mention. "Keep" in a history tab only marks the entry.
- A phone was emulated by width and touch. No real device, no installed web app.
- Counts in "Add to Focus" are written by hand, not computed from the seeded messages.
