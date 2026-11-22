# D-027: A message broker (Apache Kafka) carries the changes from Conversations to the receivers

- Status: superseded by D-028
- Level: system shape
- Date: 2026-11-22
- Decided by: user (a broker, and why); the choice of Apache Kafka follows from the user's stated reason and is the agent's, open to objection
- Depends on: D-005, D-006, D-017, D-018, D-026
- Supersedes in part: D-005 (the shape "no message broker": the agent's own delivery workers, their places and the parking place)
- Evidence: R-002; E-003 (the rules inside the receivers); E-006 (to be run: the broker in the middle)
- Visual: `docs/visuals/D-005-message-broker.html`

## Why this needs deciding now

D-005 chose separate programs without a broker, after the user first chose one and then decided against it for a first version. With D-026 the architecture became ready for many organizations, and the user then stated what the result is for: "I need to show the solution which can go then into production alongside with other systems. So we can use a broker and I can mention that we could overall omit it for the system, but I chose it for the tooling standardization and keeping in mind that real features would be shipped alongside with many other services of other teams and features." The agent had named exactly this as the case in which to choose a broker now. Nothing is built yet, so the change costs documents and one experiment.

## Options

### A. Stay without a broker (D-005 as accepted)

Our own delivery workers read each organization's log in PostgreSQL. Fewest moving parts; tested in E-003. Risk: other teams' services would need access to our database or a worker written by us for each of them; the tooling for watching, replaying and parking is home-made.

### B. Apache Kafka

A relay publishes each organization's log into a Kafka topic, keyed by organization; every receiver reads the topic from its own place as a consumer group. Upside: the standard place for other services to attach, with standard tooling (lag, replay, dead-letter topics, connectors). Risk: one more system that must be up; order holds per lane and several organizations share a lane, so keeping one organization from delaying another is our work on top (see Consequences).

### C. NATS JetStream

Lighter, with cheap subjects per organization. Risk: less widely known, which weakens the stated reason; it does not remove the lane question either unless every receiver keeps a reader per organization.

## Best practice

Every product in R-002 saves first and publishes each change to a queue that search, notifications and the feed consume; Slack's write-up puts Kafka at the front of that path. Where many services of several teams consume the same events, a broker is the established shared boundary. What a broker does not replace is established too: saving the change and its log entry in one transaction and publishing afterwards (transactional outbox), and receivers that ignore repeats.

## Recommendation

**B.** The user's reason is standard tooling and other teams' services, and Kafka is the standard.

## Decision

Accepted by the user on 2026-11-22: **a message broker**, for the reason quoted above. The agent stated the cost before (one more system to run and explain; the delivery layer tested in E-003 is replaced) and builds around the broker from here. **Apache Kafka** is the agent's choice on the user's reason.

What changes:
- The agent's own delivery workers, their places per receiver and organization, and the parking place are replaced by: one relay that publishes each organization's log to Kafka in order; a consumer group per receiver; a side topic for changes a receiver refuses.
- The stream of changes becomes a contract in its own right, the one other teams' services attach to.
- The search index and the notification sender stay stand-ins behind a network boundary; a small connector of ours reads the stream and calls each.

What stays exactly as decided: PostgreSQL as the truth with a numbered log per organization (D-005, D-026), the change and its log entry saved in one transaction, receivers that ignore repeats and older versions (D-006), the feed worked out from the stream and fenced when it cannot apply a change (D-006, D-017), the feed deciding notifications and the hold by the log's own clock (D-018), the numbers (D-007), the agent door (D-008), the live connection (D-005), Go (D-025).

How it will be presented, in the user's words: the system could do without a broker; it is chosen for standard tooling and because real features ship alongside many other services of other teams.

## Consequences

- Easier: another team's service attaches to the stream without touching our database or our code; lag, replay and parking use standard tools.
- Harder: Kafka must be running in every environment and test; a relay stands between PostgreSQL and Kafka; order holds per lane, and organizations sharing a lane share its queue, so a long backlog of one delays its lane-mates by the time it takes to work through it, and a stopped organization must be set aside by the receiver so the lane keeps moving.
- Committed to: Kafka between the logs and the receivers; the stream of changes as a public contract.

## Follow-ups

- E-006: the broker in the middle (relay killed, broker stopped, a receiver killed and rebuilt, order per organization, how much one organization's backlog delays a lane-mate, freshness).
- Contracts and guardrails rewritten around the stream after E-006; the review page redrawn.
