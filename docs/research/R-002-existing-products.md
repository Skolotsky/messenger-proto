# R-002: How do existing products solve what the brief asks for, and which decisions come first?

- Date: 2026-11-22
- Feeds: the first round of decisions (D-001 to D-009, listed as `open` in `docs/decisions/README.md`)
- Visual: `docs/visuals/R-002-decision-landscape.html`
- Method: five parallel web-research agents on a cheap model, one per topic, each required to give a URL per claim and to mark what it could not verify. Analysis by the main agent; eight load-bearing claims re-fetched by a separate checking agent (see "Verification").

## Answer

Every product studied has the same shape: the message is saved first, then every change (create, edit, delete, reaction) is published as an event, and the slow things (search, notifications, the feed, unread counts) consume those events from a queue, off the path of the person typing. What the brief calls "several independent systems" is the standard design, and the standard guarantee at such a boundary is "delivered at least once, possibly repeated, possibly out of order", made safe by receivers that ignore repeats and stale versions.

Where products genuinely differ, and where our decisions are: what the feed contains and how a person clears it; what happens to feed items and notifications when a message is edited or deleted; and how AI agents take part. In 2026 agents are ordinary conversation participants in Slack and Teams, with three additions: live status, streamed replies, and a "waiting on you" state. Structurally an agent is both a fourth receiver of message events and the heaviest source of edits.

## Findings

### The activity feed as a product

- **The feed is driven by subscription, not by channel traffic** (established). An item appears because you were mentioned, someone replied in a thread you started, replied to or follow, or someone reacted to your message. Slack, Teams, Linear and GitHub all work this way. [S1, S3, T1, L1, G1]
- **Filter by type, per-item read state, one main triage action** (established). Slack: tabs for Mentions, Threads, Reactions, plus Apps, Invitations in the 2026 redesign; "clear" hides an item and marks it read, and cleared items remain recoverable. Linear adds snooze; GitHub adds Done and Save. [S1, S2, L1, G1]
- **A cleared thread comes back when something new happens** (established). Slack: new replies reappear after clearing. GitHub: a new mention re-subscribes you. [S1, G1]
- **Grouping ("A, B and 3 others reacted") is a library pattern, not something product help pages document** (inference). Stream's feed model groups by target + type + time bucket and counts unread per group. [F1, F2]
- **"Seen" and "read" are different states** (established in feed libraries). Seen = the badge was cleared by opening the list; read = the person engaged with the item. [F2]
- **Edit and delete behaviour is the least documented and least consistent area** (contested). Mattermost: an edit that adds a mention does not notify. Zulip: it does ("the newly mentioned users will receive notifications just as if they had been mentioned in the original message"). Slack: its edit/delete help page says nothing about either case. Stream removes feed items on delete only when asked to. No product documents what its feed does when the source message is deleted. [M1, ZH1, SH1, F2] This is exactly the area the brief asks us to exercise, so we cannot copy an answer; we have to choose one.
- **Thread reply rules** (established, Slack): in channels and group DMs you are notified if you started the thread, replied to it, or were mentioned; you can follow or unfollow any thread. `@channel`/`@here` do not notify when used inside a thread. [S3, S4]
- **Slack separated "what appears in Activity" from "what gets pushed to a device" in its 2026 notification rebuild** (established). Intent is one decision, delivery another. [N1, N2]

### System shape

- **Persist, then publish, then consume asynchronously** (established). Zulip sends its event inside the database transaction that changes state. Slack runs push notifications and post-processing through a queue (Kafka in front of Redis). Discord indexes search from a queue and accepts a short delay before a message is searchable. [Z1, K1, DS1]
- **Clients need a catch-up rule, not just a live connection** (established). Discord: every event has a sequence number and a reconnecting client asks for everything after the last one it saw; if that fails it reloads. Zulip: same idea over long polling with `last_event_id`, and a full reload if the server has forgotten the client. Slack and Mattermost use WebSockets but do not document replay. [DG1, Z1]
- **Edits and deletes are events of their own** (established). Edit carries the full new message; delete carries only identifiers. Slack: `message_changed`, `message_deleted`. Discord: `MESSAGE_UPDATE`, `MESSAGE_DELETE`. [SE1, DG2]
- **IDs that sort by time** (established). Discord snowflakes; Slack's per-channel `ts`. [DG3, SE1]
- **Scale techniques (fan-out on write vs read, sharded channel servers, Cassandra/ScyllaDB) are irrelevant at our size** (inference). They answer "millions of users", which the brief does not ask.

### Reliable delivery

- **Transactional outbox is the established answer to "saved the message but failed to tell the others"** (established). The event is written in the same transaction as the message, and a relay delivers it afterwards. Relay by polling (simple) or by reading the database log (faster, more to operate). [O1, O2, O3]
- **At-least-once delivery plus receivers that ignore repeats** (established). "Exactly once" across a network boundary is not something any product promises; Slack, GitHub and Stripe all tell receivers to expect repeats. [O4, E1, GH1]
- **Order holds only per key, and redelivery can still reorder** (established). The usual guard: each message carries a version that increases on every change; a receiver applies a change only if it is newer; a delete is a marker that wins over any late edit. (The guard itself is common practice and my synthesis, not a quoted source; Elasticsearch's external versioning is the same idea for search.) [P1, ES1]
- **One slow receiver must not hold back the others** (established practice, my synthesis): one cursor or queue per receiver, timeouts on every call, exponential backoff with jitter, a parking place for events that keep failing, and a way to replay or rebuild a receiver from the source. [A1, AZ1]
- **What real boundaries promise** (established). Slack Events API: receiver must answer within 3 seconds; up to 3 retries (immediately, ~1 min, ~5 min); every event has a unique `event_id`; delivery is switched off if over 95% of attempts fail in an hour. GitHub: 10-second timeout and no automatic retry at all. [E1, GH1]
- **Notifications need their own rule for change** (inference). A notification already sent cannot be recalled; one not yet sent can be cancelled if the message was deleted or the mention removed. A short send delay makes this possible. No primary source found for how Slack does it.
- **Guarantees are proven by breaking things** (established): kill the relay mid-batch, take a receiver down, duplicate and reorder deliveries, then check nothing was lost and every receiver ends equal to the source. [J1]

### AI bots and agents

- **Agents moved from a side panel into ordinary conversations** (established, 2026). Slack (June 2026): "Agent conversations now look & feel the same as a regular direct message"; new apps can only use this experience. Teams (June 2026): agents take part "across chats, channels, and meetings" by @mention. Linear: agents "appear as users that can be mentioned, assigned". [AI1, AI5, AI8]
- **Three things an agent has that a person does not** (established):
  1. *Status.* Slack: `processing`, `active`, `suspended`, `closed`; "an agent that has posted a question and is waiting on the answer is `suspended`", including waiting for an approval. [AI2]
  2. *Streamed replies.* Slack `chat.startStream`/`appendStream`/`stopStream`; Teams streaming. Both cap a stream at 10 minutes. [AI3, AI6]
  3. *An AI label and feedback buttons* on the result. Teams has an explicit "AI generated" label; Slack labels the app as an agent. [AI6, AI2]
- **In-progress agent output is deliberately silent** (established, Teams): streamed updates "don't create notifications or trigger the unread message badge"; only the finished message counts. [AI6]
- **Volume** (vendor's own figure, Nov 2026): Salesforce says half of Slack messages come from apps and agents. Treat as marketing, but the direction is clear: without a noise rule, automation floods a feed. Slack's 2026 Activity has a separate Apps filter; Teams caps apps at 20 feed notifications per minute per user. [AI4, S2, AI7]
- **Agents receive message events through the same kind of boundary as any other external system** (established): Slack's Events API, with the 3-second answer and retries above. [E1]
- **What an agent may read is granted explicitly** (established): scopes in Slack, with user consent for private conversations; per-resource consent in Teams. [AI9, AI7]
- **AI on the feed itself is modest so far** (established): Slack offers daily channel recaps and thread summaries on paid plans. No product found that ranks or rewrites the activity feed with AI. [AI10]
- **Loop prevention (agents triggering agents) has no official rule** in Slack or Teams docs (not found).

## What this means for us

1. **The backbone is not really a choice.** Persist, outbox, per-receiver delivery, version-guarded idempotent receivers. Recommending anything else would need a reason. The choices are the numbers and promises on top: how fresh, how long we retry, what a person sees while a receiver is down.
2. **The product decisions are where we show craft.** What the feed contains, how it is cleared, how items group, and above all what an edit or a delete does to items and notifications already produced. Products disagree or stay silent here.
3. **An agent fits the brief twice** (inference). As a *fourth independent receiver* of message events it exercises the delivery guarantees with a realistic consumer. As a *producer* it stresses the edit path: a streamed reply is one message edited many times, so we must decide whether in-progress text travels through the reliable pipeline or only the finished message does (Teams' answer: only the finished one notifies).
4. **"Waiting on you" is a new kind of feed item** (inference). An agent paused for an approval is the most actionable thing a feed can show, and no existing feed treats it as a first-class type. This is the clearest place to go beyond a Slack copy.
5. **Order of decisions.** Scope and audience first, then the role of agents (it changes the data model: participant types, message states), then the feed, then edit/delete semantics, then system shape, then the guarantees.

## Open questions

- What Slack's Activity does today when the source message is deleted or edited to add or remove a mention. Not documented; needs a look in a real workspace. Informs D-004, does not block it.
- Whether streamed agent text should pass through the outbox per chunk, be coalesced, or bypass it until final. Candidate for an experiment once the stack is chosen.
- Polling relay latency on our stack: is "searchable and in the feed within a second or two" reachable without reading the database log? Candidate for an experiment.
- Could not verify: Discord Inbox details (help page blocked), Stripe's retry schedule, any primary source for cancelling pending notifications.

## Verification

A second cheap agent re-fetched eight pages and compared them with the claims. Its quotes pass through a summarising fetch tool, so they are near-verbatim, not checked by eye in a browser.

- Confirmed: Salesforce "fifty percent of Slack messages are from apps and agents" [AI4]; Teams streamed updates create no notification or badge, 10-minute cap [AI6]; Slack thread reply rule [S3]; Slack streaming methods [AI3]; Slack Events API 3-second answer and retry schedule [E1]; Zulip notifies on an edit that adds a mention [ZH1].
- Corrected: a research agent reported that Slack maps old notification settings at read time instead of migrating them. The article describes a migration and backfill. The claim is dropped; the Activity/push separation stands [N1].
- Confirmed absent: Slack's edit/delete help page says nothing about notifications [SH1].
- Not re-checked: everything else. Findings rest on the research agents' reading of the cited pages. Many sources are from 2026 and newer than the main agent's own knowledge, so they could not be cross-checked from memory.

## Sources

Fetched 2026-11-22. Date shown is the page's own where visible.

Feed and notifications
- [S1] [Slack help: Activity](https://slack.com/help/articles/19693583638803), Slack, ©2026. Item types, clearing, cleared items.
- [S2] [Slack help: updated Activity](https://slack.com/help/articles/46751260742035), Slack, rolling out from Jan 2026. New filters (Apps, Invitations, Reminders), saved filter tabs.
- [S3] [Follow and unfollow threads](https://slack.com/help/articles/115000769927-Follow-and-unfollow-threads), Slack, ©2026. Who is notified of a reply.
- [S4] [Notify a channel or workspace](https://slack.com/help/articles/202009646-Notify-a-channel-or-workspace), Slack, ©2026. `@here`/`@channel` rules.
- [T1] [Explore the Activity feed in Teams](https://support.microsoft.com/en-us/office/explore-the-activity-feed-in-microsoft-teams-91c635a1-644a-4c60-9c98-233db3e13a56), Microsoft, ©2026. Item types, filters, 30-day retention.
- [L1] [Linear Inbox](https://linear.app/docs/inbox), Linear, undated. Subscription model, snooze.
- [G1] [About notifications](https://docs.github.com/en/subscriptions-and-notifications/concepts/about-notifications), GitHub, undated. Reasons, Done/Save.
- [F1] [Scalable activity feed architecture](https://getstream.io/blog/scalable-activity-feed-architecture/), Stream, 2026-02-11. Fan-out trade-offs, aggregation keys.
- [F2] [Notification feeds](https://getstream.io/activity-feeds/docs/react/notification-feeds.md), Stream, undated. Seen vs read, grouping, removal on delete.
- [N1] [How Slack rebuilt notifications](https://slack.engineering/how-slack-rebuilt-notifications/), Slack Engineering, 2026-03-19. Activity vs push separation.
- [N2] [Slack's new notification system](https://www.infoq.com/news/2026/04/slack-new-notification-system/), InfoQ, 2026-04-21. Secondary confirmation.
- [ZH1] [Edit a message](https://zulip.com/help/edit-a-message), Zulip, undated. Edit-to-add-mention notifies.
- [SH1] [Edit or delete messages](https://slack.com/help/articles/202395258-Edit-or-delete-messages), Slack, undated. Silent on notifications.
- [M1] [Mention people](https://docs.mattermost.com/end-user-guide/collaborate/mention-people.html), Mattermost, undated. Edit-to-add-mention does not notify.

System shape
- [Z1] [Zulip events system](https://zulip.readthedocs.io/en/stable/subsystems/events-system.html), Zulip, undated. Event on commit, `last_event_id`, reload fallback.
- [K1] [Scaling Slack's job queue](https://slack.engineering/scaling-slacks-job-queue/), Slack Engineering, 2017-12-06. Queue behind every message post.
- [DS1] [How Discord indexes billions of messages](https://discord.com/blog/how-discord-indexes-billions-of-messages), Discord, 2017-03-16. Queue-fed search, accepted delay.
- [DG1] [Discord gateway](https://docs.discord.com/developers/events/gateway), Discord, undated. Sequence numbers, resume and replay.
- [DG2] [Discord gateway events](https://docs.discord.com/developers/topics/gateway-events), Discord, undated. Update and delete payloads.
- [DG3] [Discord API reference](https://docs.discord.com/developers/reference), Discord, undated. Snowflake IDs.
- [SE1] [Slack message event](https://docs.slack.dev/reference/events/message), Slack, undated. `ts`, `message_changed`, `message_deleted`.
- [RT1] [Real-time messaging](https://slack.engineering/real-time-messaging/), Slack Engineering, 2023-04-11. Gateway and channel servers.

Reliable delivery
- [O1] [Transactional outbox](https://microservices.io/patterns/data/transactional-outbox.html), microservices.io, undated.
- [O2] [Polling publisher](https://microservices.io/patterns/data/polling-publisher.html), microservices.io, undated. Ordering caveat.
- [O3] [Debezium outbox event router](https://debezium.io/documentation/reference/stable/transformations/outbox-event-router.html), Debezium, undated.
- [O4] [Idempotent consumer](https://microservices.io/patterns/communication-style/idempotent-consumer.html), microservices.io, undated.
- [P1] [Pub/Sub ordering](https://docs.cloud.google.com/pubsub/docs/ordering), Google Cloud, undated. Per-key order, at-least-once.
- [ES1] [Elasticsearch versioning support](https://www.elastic.co/blog/elasticsearch-versioning-support), Elastic, 2013-06-24 (old; principle only).
- [A1] [Timeouts, retries and backoff with jitter](https://builder.aws.com/content/3EumjoZascWd1oZiEgL8ORlv3qE/timeouts-retries-and-backoff-with-jitter), AWS Builders' Library, 2019-12-05.
- [AZ1] [Circuit breaker pattern](https://learn.microsoft.com/en-us/azure/architecture/patterns/circuit-breaker), Microsoft, updated 2026-11-09.
- [E1] [Slack Events API](https://docs.slack.dev/apis/events-api/), Slack, undated. 3-second answer, retries, `event_id`.
- [GH1] [Handling failed webhook deliveries](https://docs.github.com/en/webhooks/using-webhooks/handling-failed-webhook-deliveries), GitHub, undated.
- [J1] [Jepsen: NATS 2.12.1](https://jepsen.io/analyses/nats-2.12.1), Jepsen, 2025-12-08. Why queues are tested under failure.

AI agents
- [AI1] [Agent messages tab](https://docs.slack.dev/changelog/2026/06/30/agent-messages-tab/), Slack changelog, 2026-06-30.
- [AI2] [agents.sessions.setStatus](https://docs.slack.dev/reference/methods/agents.sessions.setStatus) and [Developing agents](https://docs.slack.dev/ai/developing-agents), Slack, undated.
- [AI3] [Chat streaming](https://docs.slack.dev/changelog/2025/10/7/chat-streaming), Slack changelog, 2025-10-07.
- [AI4] [Slack stats](https://salesforce.com/news/stories/slack-stats), Salesforce, 2026-11-03. Vendor figures.
- [AI5] [Build collaborative agents where work happens](https://devblogs.microsoft.com/microsoft365dev/build-collaborative-agents-where-work-happens/), Microsoft, 2026-06-02.
- [AI6] [Teams streaming UX](https://learn.microsoft.com/en-us/microsoftteams/platform/bots/streaming-ux), Microsoft, 2026-10-28.
- [AI7] [Teams activity feed notification best practices](https://learn.microsoft.com/en-us/Graph/teams-activity-feed-notifications-best-practices), Microsoft, 2024-11-07.
- [AI8] [Agents in Linear](https://linear.app/docs/agents-in-linear.md), Linear, undated.
- [AI9] [Slack MCP server and Real-time Search API](https://docs.slack.dev/changelog/2026/02/17/slack-mcp), Slack changelog, 2026-02-17.
- [AI10] [Slack AI features guide](https://slack.com/help/articles/25076892548883), Slack, undated.
