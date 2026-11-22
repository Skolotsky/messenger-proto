# D-001: The product hypothesis the early access programme tests

- Status: accepted; superseded in part (when a flag closes: D-011; the Muted and All activity levels are narrowed by D-013)
- Level: product
- Date: 2026-11-22 (proposed, shaped in discussion and accepted the same day)
- Decided by: user
- Depends on: none
- Evidence: R-002
- Visual: `docs/visuals/D-001-product-hypothesis.html`

## Why this needs deciding now

We are building a proof of concept to launch a closed early access programme (EAP). An EAP only teaches us something if it tests one clear bet. The bet also sets the scope: which surfaces we build, what the activity feed shows, and how far AI agents go. Every other product decision (D-002 to D-004, D-009) waits on it.

Given by the user on 2026-11-22, not open for decision here: the product is for small to mid-size startup teams building a product, with mixed roles (not only developers), most of whom use AI extensively.

## Options

### A. The feed is the short list of what deserves your attention, in three buckets you can shape

Bet: in a team where agents write as much as people, the scarce thing is human attention. People will trust a feed that sorts by plain, visible rules, and will shape the top of it themselves.

The first version of this option had two tiers. The user leaned to A and added the third, Focus, on 2026-11-22. As it now stands:

- **Focus** is the top bucket and belongs to the person. It holds what matches their own rules. Some people will use it as the one place to start. Rules are of two kinds:
  - exact rules: a person, a direct conversation, a channel, a thread, a kind of item. Added in one click from wherever the thing is. Before a rule is saved, the product shows how many items it would have pulled in recently.
  - what focusing a conversation means: a channel in Focus brings every new top-level message there, plus everything addressed to you there (mentions, replies in your threads, flags). Replies in threads you are not part of stay out; a thread is focused on its own, and then every new reply lands in Focus. Reactions never land in Focus. This follows from why people focus a channel: being on call, a crunch period, a low-volume channel where every post matters, being the one who answers. All of them want every post, not only what names them.
  - text rules, judged by a language model: "anything about pricing or contracts". The model may only move an item up into Focus, never hide or lower one, and it runs after the item is already in the feed, so a slow or unavailable model delays a promotion and breaks nothing.
- **Needs you** is the system's judgement that something is directed at you. Fixed rules, the same for everyone, each item saying why it is there.
- **For your information** is what concerns you without waiting on you.
- **One item, one place.** Focus wins over Needs you, which wins over For your information. Every item says why it is where it is ("In Focus: direct message from Anna").
- **The sender can set a level on a message: Flag, nothing, or Info.** The same control for people and agents; almost every message carries neither. A flag says "this needs attention", which is stronger than a mention and not always a request. Who it is for follows from the message itself:

  | A flagged message… | Is for | Lands in | Closes when |
  |---|---|---|---|
  | in a direct conversation | the other person | Focus | they respond |
  | that mentions people | each person mentioned | Focus | each responds, for themselves |
  | that mentions nobody | everyone in the thread, or in the channel for a top-level message | Needs you | anyone responds |

  The sender can withdraw a flag at any time. Agents flag whenever they stop and wait for a human (an approval, a choice, a clarification) and leave everything else unflagged; people flag when they choose to. An AI suggesting a flag to a person is a later idea, out of scope. What counts as responding is settled in D-003.

  Info says "for the record, do not disturb anyone". An Info message never lands higher than For your information, even when it mentions you and even in a channel or thread you put in Focus, and it sends no notification. A mention inside it is a polite copy. Agents use it for finished progress updates; people use it for notes and courtesy mentions. The sender's Info wins over the reader's Focus rule: the message stays visible one bucket down, so nothing is hidden.

  For agents this gives four cases: text still streaming in is not a finished message and never reaches the feed; a finished progress update is Info; a result carries no level; a stop for a human is flagged.
- **One row per conversation.** In Focus, new items from the same channel or thread collapse into one entry ("#incidents · 6 new") that expands, and the count updates in place. A focused channel must not turn a short list into a long one.
- **Each channel has one dial**, changed in one click from the channel, so the fixed rules can be tuned without a grid of switches:

  | Level | What reaches your feed from the channel |
  |---|---|
  | Focus | Every new top-level message and everything addressed to you, in Focus. The same thing as a "this channel" Focus rule |
  | All activity | The same items, with the unaddressed ones in For your information |
  | Default | The starting rules below |
  | Muted | Only what names you: direct mentions and flags that mention you |

  Whatever the level, a direct mention or a flag that names you always gets through, and every item's reason includes the channel's level ("#launch is set to All activity").
- **Each thread has a dial too, and its levels are the buckets themselves.** The question is always "where should replies from this thread go?":

  | Level | Where new replies land | Set automatically when |
  |---|---|---|
  | Focus | Focus | never; the person chooses it |
  | Needs you | Needs you | you started the thread or replied in it |
  | For your information | For your information | you were mentioned in it, or you clicked Follow |
  | Off | nowhere | every other thread; also what Unfollow sets |

  Follow and Unfollow are one-click shortcuts for "For your information" and "Off"; a small menu beside the button offers all four levels, which most people will never open. Rules that go with it: what names you gets through even at Off; the thread's dial wins over the channel's for replies, so a followed thread in a muted channel still reaches you; replying in a thread you turned off sets it back to Needs you; being mentioned does not override an explicit Off, though the mention itself arrives; "everyone in the thread", for a flag that mentions nobody, means everyone whose dial for it is not Off; Info replies still land no higher than For your information. A thread is one row in whichever bucket it lands in.
- **Nothing jumps.** An edit or a delete never changes the size or position of what is on screen. A longer edit sits behind an expander, a shorter one keeps its space, a deleted message leaves a marker in its place, and emptied space is filled with care (an illustration, not a gap). The feed still always shows the current truth. Detail belongs to D-004.

Starting rules, so that Focus is useful before anyone configures anything (recommended; the user proposed "any direct message" and "any direct mention", see the note below):

| Bucket | What lands there from day one |
|---|---|
| Focus (your rules, both removable) | Direct messages from people, one to one. Flagged messages that mention you or are sent to you directly, from a person or an agent |
| Needs you (fixed) | Direct mentions of you. Replies in threads you started or replied in (the thread dial's automatic level). Flagged messages that mention nobody, in your threads and channels |
| For your information (fixed) | Reactions to your messages, grouped. Mentions of a whole channel or group. Replies in threads you follow or were mentioned in. Messages marked Info that concern you |
| Never in the feed | Agent text that is still streaming in. Ordinary channel traffic you were not addressed in |

Note on the difference from the user's proposal: direct mentions start in Needs you, not Focus. Mentions are frequent and often a courtesy ("thanks @sam"), and with them in Focus the Needs you bucket is left almost empty and Focus becomes Needs you under another name. "Any direct mention" is offered as a suggested rule, one click to add.

- Borrowed: Slack's item types (mentions, thread replies, reactions) and its thread rules; the idea of a personal priority view (Slack's VIP filter, Linear's Priority tab, Gmail's priority inbox); Slack's "agent is waiting" status; Teams' rule that in-progress agent output notifies nobody.
- Changed: buckets by purpose instead of filters by type; the top bucket is the person's own and its rules are visible; a flagged message, from a waiting agent or a person, is a first-class item; every item explains its place; nothing jumps on edit or delete.
- Upside: it answers the question every feed has to answer (what deserves attention and what does not) and gives the person the last word.
- Risks:
  - A wrong rule puts something important low and people stop trusting the feed. Reduced by lowering, never hiding, and by the reason shown on each item.
  - A broad rule ("everything in #launch") turns Focus into an unread list. Reduced by the count shown before saving.
  - Flags are overused, like the high-importance flag in email, or a flag with no mention reaches a whole large channel. Reduced by: a flag closes when answered; one with no mention lands a bucket lower and closes for everyone at the first response; the sender can withdraw it; the person can remove the starting rule.
  - Text rules are not deterministic and a wrong call in the top bucket costs the most trust. Reduced by promote-only, by showing which rule matched, and by an experiment before they are built. The user judges the gain larger than the risk (2026-11-22).
- EAP signals: people clear the feed daily; the share of people who add a Focus rule; the share who start from Focus; items in higher buckets are acted on clearly more often than those below.

### B. The workspace where people and agents work as teammates

Bet: teams want one place where agents are assigned work, report progress, hand off and ask for approval, and the feed is the view of that work.

- Upside: the most novel option and the strongest AI story.
- Risk: the result depends on how good the agents are, not on our product. It needs believable agents to show at all, and it pulls effort away from messaging, the feed and reliable delivery, which the brief puts first.
- EAP signal: teams route real work through agents in the product.

### C. A Slack-style feed, done carefully

Bet: none beyond execution. The feed copies Slack's model and the effort goes into it being correct and never stale under edits, deletes and outages.

- Upside: the smallest scope and the most time for reliability.
- Risk: an EAP would teach us nothing, and a team has no reason to switch from what it already uses.
- EAP signal: none specific.

## Best practice

A proof of concept tests one falsifiable hypothesis with a signal that can be observed in early use, and cuts everything that does not serve it. R-002 shows the feed mechanics are common ground across products, so the bet should sit where products differ: what reaches a person, and how agents take part.

Where A as shaped goes beyond that practice: text rules judged by a model add a second bet (people want smart filtering and will trust it). The agent recommended building exact rules first and deciding on text rules after an experiment. The user chose to keep text rules in the scope and to place their experiment and build after everything else, so the first bet can be shown without them.

## Recommendation

**A as described above**, with the starting rules in the table.

Locks in: three buckets with one item in one place; Focus is defined only by the person's rules; fixed, explained rules for the other two; the sender's level on a message (Flag, Info); one dial per channel and one per thread; one row per conversation; agents are participants in conversations; text rules judged by a model are in scope, built last, after an experiment; nothing jumps on edit or delete.

Order of building: the three buckets with exact rules and flags first; text rules last.

Reversibility: cheap today, moderate once the feed is built. The starting rules stay cheap to change.

No second opinion was sought: nothing is built yet, so changing course costs little.

## Decision

Accepted by the user on 2026-11-22: **option A as described above.**

The user's own shaping, beyond the agent's first proposal:
- A third bucket, Focus, on top: "a way for the user to form what's really important for their attention", by exact rules that are easy to add and, optionally, by text rules judged by a language model.
- Text rules stay in scope against the agent's advice to decide after an experiment: "everyone wants some smart filtering and llm is the smartest way currently... gains are bigger than risks, at least from the product perspective". Their experiment and build come after everything else. Promote-only and after the item is shown were agreed.
- No content jumping on edit or delete: expander for a changed size, a marker for a deleted message, emptied space filled with care.
- Senders set a level on a message. The agent proposed a mark naming a person; the user simplified it to a flag that reaches whoever the message mentions, or everyone in scope when it mentions nobody, chose the term "flag" ("not everything that needs attention means request"), and added Info so a person, like an agent, can post without disturbing anyone.
- The fixed rules are "good as defaults", tuned by a dial per channel and per thread. The thread dial came from the user's point that a person must be able to follow a thread that is not theirs and leave one that is.

Agreed from the agent's side: direct mentions start in Needs you, not Focus; a flag that mentions nobody lands a bucket lower and closes at the first response; what names you gets through any dial level; one row per conversation; a count before a Focus rule is saved.

Set aside: an end date on a Focus rule ("let's not overcomplicate yet"). Out of scope for the first version: ad-hoc topics.

## Consequences

- Easier: every later product decision has a yardstick (does it help a person trust the three buckets?). "What deserves attention" has an explicit, explainable answer in the product.
- Harder: the feed is no longer a list of events. An item's bucket depends on the reader's rules, two dials, the sender's level and the state of a flag, and it can change after it appeared. Edits, deletes, a closed flag and a late promotion all have to update an item in place without jumping.
- Committed to: three buckets; Focus owned by the person; fixed, explained rules for the other two; Flag and Info; channel and thread dials; one row per conversation; agents as participants; text rules judged by a model built last after an experiment.

## Follow-ups

- Later surface, out of scope for the first version (user, 2026-11-22): **ad-hoc topics**, for gathering a handful of people around one subject, the role group direct messages play in Slack. A topic behaves like a channel but is not listed among channels, has no channel identity (a title for the subject, no name), is archived automatically after a period without activity, and has its own notification defaults. Recorded so D-009 and the data model do not rule it out.

- D-009 (which surfaces the PoC includes) and D-002 to D-004 open once this is accepted. D-002 inherits flags and "agents are participants"; D-003 inherits the buckets and rules; D-004 inherits "nothing jumps", plus the open question of how long a deleted message's marker stays and whether it counts as needing you.
- An experiment on text rules, run after the rest is built: can a plain-text rule sort our seeded messages well enough and fast enough? It needs a real model, which becomes a system-shape question at that point.
- The PoC must be able to explain, in the product and in writing: what earns a person's attention and what does not; what was borrowed from existing products and what was changed; what a stale, edited or deleted entry looks like; how a message reaches the feed; what is guaranteed; and where corners were knowingly cut. A "known shortcuts" list starts with the first implementation task.
