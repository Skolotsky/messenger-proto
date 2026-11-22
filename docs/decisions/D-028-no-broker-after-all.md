# D-028: No message broker: back to delivery from the PostgreSQL logs, ready for many organizations

- Status: accepted
- Level: system shape
- Date: 2026-11-22
- Decided by: user
- Depends on: D-005, D-006, D-026
- Supersedes: D-027 (a message broker, Apache Kafka)
- Evidence: E-006 (Kafka in the middle, built small and broken on purpose); E-003, E-004
- Visual: `docs/visuals/D-005-message-broker.html`

## Why this needs deciding now

D-027 put Apache Kafka between Conversations' logs and the receivers, for standard tooling and because the system is to ship alongside other teams' services. E-006 then tried exactly that. Seeing the first results, the user asked: "tell me if kafka made everything worse? If yes then let's go back to previous architecture before kafka." The contracts and guardrails had not yet been rewritten for Kafka, so going back costs this record.

## Options

### A. Keep Kafka in the middle (D-027)

Standard attachment point and tooling for other teams. Measured cost in E-006: save to receiver 121 ms median against 7 ms; a new way to lose data (a relay that takes "sending finished" for "confirmed" lost 393 of about 7,000 changes when Kafka was stopped and restarted); with a plain consumer, one organization's 20,000 waiting changes delayed every other organization by 8 s median and up to 23 s, also in other lanes; keeping organizations from delaying each other needs a hand-made consumer that takes them in turn (which brought it back to 54 ms median, 2 s worst). One more stateful system in every run.

### B. Back to delivery from the logs, and Kafka only as an outward stream for other teams

Our own receivers are fed as before; a relay additionally publishes the stream of changes to Kafka for outside consumers, so nothing of ours depends on it. The agent's recommendation.

### C. Back to delivery from the logs, no Kafka at all

The design as accepted in D-005 and D-026: a delivery worker per receiver reading each organization's log in PostgreSQL, taking the organizations in turn. Fewest parts; when a broker becomes right stays recorded.

## Best practice

Add a broker when there is someone to attach to it. Until then the same pattern (save and log in one step, a place per reader, receivers that ignore repeats) runs on the database alone, and adding a broker later replaces one layer.

## Recommendation

The agent recommended B. It answers the user's stated reason for D-027 without putting Kafka in the path of our own receivers.

## Decision

Accepted by the user on 2026-11-22: **C.** "Ok, let's then get rid of kafka and rollback to the previous architecture. Just remember about multitenancy."

So the design is again: separate programs that talk only over the network (D-005); PostgreSQL in Docker as the truth; a numbered change log per organization (D-026); a delivery worker per receiver that keeps a place per receiver and organization and takes the organizations in turn; receivers that ignore repeats and older versions; a parking place for a change a receiver refuses; the feed worked out from the log; no message broker. Everything D-026 decided about organizations stands. The contracts under `docs/contracts/` and the guardrails describe this design and were not changed by D-027.

What is kept from E-006: the measurements above as the record of why; the rule that a publisher advances only on confirmed delivery, for the day a broker is added; and a direct measurement, made for this decision, that our own worker taking organizations in turn keeps one organization's backlog from delaying another (see E-006, last part).

## Consequences

- Easier: one stateful system (PostgreSQL); the isolation between organizations is a property of the design, not of a hand-made consumer on top of a broker.
- Harder: another team's service has no standard place to attach to until a broker, or an outward stream as in B, is added.
- Committed to: no message broker in the first version. When one becomes right, and what it would cost, is in `docs/visuals/D-005-message-broker.html`.

## Follow-ups

- None for the contracts or guardrails: they already describe this design.
