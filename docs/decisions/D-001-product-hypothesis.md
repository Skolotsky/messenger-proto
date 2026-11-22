# D-001: The product hypothesis the early access programme tests

- Status: proposed
- Level: product
- Date: 2026-11-22
- Decided by: user
- Depends on: none
- Evidence: R-002
- Visual: `docs/visuals/D-001-product-hypothesis.html`

## Why this needs deciding now

We are building a proof of concept to launch an early access programme (EAP). An EAP only teaches us something if it tests one clear bet. The bet also sets the scope: which surfaces we build, what the activity feed shows, and how far AI agents go. Every other product decision (D-002 to D-004, D-009) waits on it.

Given by the user on 2026-11-22, not open for decision here: the product is for small to mid-size startup teams building a product, with mixed roles (not only developers), most of whom use AI extensively.

## Options

### A. The feed is the short list of what is waiting on you

Bet: in a team where agents write as much as people, the scarce thing is human attention. A feed that separates "needs you" from "for your information", and keeps agent chatter out, lets a person clear what matters in a couple of minutes and trust that nothing was missed.

- Borrowed: Slack's item types (mentions, thread replies, reactions) and thread rules.
- Changed: two tiers decided by plain, visible rules (no AI ranking); "an agent is waiting on you" as its own item type; in-progress agent output never reaches the feed.
- Upside: it answers the question every feed has to answer (what deserves attention and what does not), and it stays inside the brief's focus.
- Risk: a wrong rule demotes something important and people stop trusting the feed. Reduced by demoting, never hiding, and by showing why each item is there.
- EAP signal: people clear the feed daily; the share of "needs you" items acted on is clearly higher than for "FYI".

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

## Recommendation

**A**, with two additions. From C: the feed always shows the current truth after an edit or delete, as a quality we hold ourselves to regardless of the bet. From B: only the "agent is waiting on you" item, with agents present as scripted participants in the seeded team.

Reason: it is the one option where the research points to a real gap (half of Slack's messages reportedly come from apps and agents, and no feed treats a waiting agent as a first-class item), it fits the brief's focus, and it gives a measurable signal.

Locks in: the feed has tiers set by explicit rules; agents exist as participants; no AI ranking or summarising in the PoC.

Reversibility: cheap today, moderate once the feed is built.

No second opinion was sought: nothing is built yet, so changing course costs little.

## Decision

_Open._

## Consequences

_Filled in when accepted._

## Follow-ups

- D-009 (which surfaces the PoC includes) and D-002 to D-004 open once this is accepted.
- The PoC must be able to explain, in the product and in writing: what earns a person's attention and what does not; what was borrowed from existing products and what was changed; what a stale, edited or deleted entry looks like; how a message reaches the feed; what is guaranteed; and where corners were knowingly cut. A "known shortcuts" list starts with the first implementation task.
