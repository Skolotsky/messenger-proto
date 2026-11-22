# The seeded organization

Larkmere, an invented maker of clinic scheduling software, on the Tuesday morning two days before its 2.0 launch: 30 people, 6 agents, 20 channels, 23 direct conversations and two weeks of history (D-010). All people, companies and clinics are invented.

This folder is data. `server/internal/democontent` loads it and generates the history; it is a package, not a service.

```go
c, _ := democontent.Load(os.DirFS("demo/content"))
h, _ := c.Build(democontent.Options{})           // default seed and end; h.Changes go to a seed session
out, _ := c.Compile(c.Situations[0], h.Through, at, "p1") // the changes one situation writes
```

The same content, seed and end give the same bytes (`History.JSONL`). `Options.End` must be a Tuesday at 10:42 UTC; `LatestEnd(now)` gives the newest one, which moves the whole history by whole weeks so weekdays stay right.

## Files

| File | What it holds |
|---|---|
| `organization.json` | People, groups, channels, direct conversations, starting dials and Focus rules, the core cast, the default viewer (Maya) |
| `story/<channel>[.anything].txt` | Hand-written exchanges; `dm*.txt` holds direct ones |
| `agent-replies.json` | What people answer to each agent's routine posts; each line is used at most twice |
| `situations.json` | The situations of D-010 as replayable scripts with their feed outcomes |

The agents' routine work (builds, alerts, tickets, metrics, research progress, standup notes) is generated from a seed in `agents.go`.

## Story format

```
=== mid                         one exchange: early, mid, late, weekend, or d0..d15 for one day
tomas: Top-level message.       author id, then text; @Firstname is a mention
> marco: A reply in its thread. {+👍 tomas,nadia}
> tomas: Text. {edit: The text it is edited to a few minutes later.}
ewan: Posted, then taken back. {delete}
```

Annotations go at the end of a line: `{flag}`, `{info}`, `{+emoji people}`, `{edit: text}`, `{delete}`. For a direct conversation the header names the two people: `=== mid maya tomas`. Loading refuses unknown people, a mention or author outside the conversation, and unknown annotations. Days count from the first Monday: `d0` is 2 November, `d14` the last Monday, `d15` the last morning.

The generator may open a reply with `@Name` of the person it answers, so that threads carry mentions as real ones do. It leaves lines that already mention someone or that are edited later.

## What the history contains

Ordered `Change`s (`contracts/schema/contexia.json`), numbered from 1 with `previous` 0, for a seed session: who exists, channels, direct conversations, starting settings, then messages, edits, deletes, reactions and reads. A reply, reaction, Keep or Done is followed by the `reader.read` it implies (`docs/contracts/screen-to-conversations.md`). Maya last read on Monday evening, so Tuesday morning is unread for her. Seeded history creates no notifications (D-032).

## Situations

Each has `id`, `title`, `outcome` (these three are the contract's `Situation`), the line of D-010 it `covers`, its `viewer`, and `steps`. A step waits `after_s` seconds, then `who` does one of `post`, `reply`, `edit`, `delete`, `react`, `unreact`, `read`, `keep`, `done`, `dial`. `as` names a message for later steps, `to` refers to one, `in` is the conversation.

A step's `expect` lists feed outcomes at that moment: `for` a person, the row holding message `of` is `where`, one of `focus_new`, `focus_kept`, `needs_you`, `fyi`, `history` (left the feed) or `nothing`, and `why`, citing the decision.

A situation plays on top of the seeded settings, so an author who is someone's Focus person changes the outcome: Maya's Focus rule lifts Omar, Anna's lifts all mentions, Sam's lifts #customers.
