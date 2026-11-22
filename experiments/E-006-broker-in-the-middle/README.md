# E-006: Do the delivery promises hold with Apache Kafka in the middle, and what does it cost?

- Date: 2026-11-22
- Time box: one working session
- Feeds: D-027 (a message broker, since superseded) and D-028 (no broker, decided on these results); D-026 (many organizations)
- Visual: `docs/visuals/D-005-message-broker.html`

## Hypothesis

With a relay publishing each organization's log from PostgreSQL to one Kafka topic, keyed by organization, and each receiver reading it as a consumer group, the promises of D-006 still hold under the faults of D-007: nothing lost, repeats harmless, order per organization. We expected correctness to carry over and expected a cost in speed and in the independence of organizations, because Kafka keeps order per lane and several organizations share a lane.

## Method

PostgreSQL 17 and Apache Kafka 3.9 (one node, KRaft) in Docker; a Python client (confluent-kafka, psycopg) in a third container. PostgreSQL holds the truth: a numbered change log per organization. A relay process publishes it; a receiver that insists on order (like the feed) and one that only ignores repeats (like search) read the topic and keep their state in PostgreSQL. Three organizations, two of which share a lane of the topic. Processes are killed for real and Kafka is stopped and started for real. Two variants are broken on purpose. A last part runs the design without Kafka: our own worker reading each organization's log and taking the organizations in turn.

Rerun, about nine minutes: `./experiments/E-006-broker-in-the-middle/run.sh` (a scenario can be named to run it alone).

## What would change our mind

- A change missing or applied twice after any fault: Kafka in the middle breaks D-006.
- One organization's backlog delaying another beyond the 2 s target with no remedy: Kafka conflicts with D-026.
- No measurable cost against the direct design: there would be no reason against it.

## Result

Raw output: `results.txt`.

### Correctness, with Kafka

| Fault | Outcome |
|---|---|
| Healthy, three organizations at 100 saves a second each | every change at both receivers, once, in order |
| Relay killed five times mid-burst | holds |
| Kafka stopped for 8 s mid-burst | holds; meanwhile about 2,760 saves were accepted, a save taking 1 ms (worst 17 ms): people keep working |
| The ordered receiver killed three times; the other stopped for 8 s | holds; the ordered receiver was as fresh as before while the other was stopped |
| An organization stopped at the receiver while it gets 20,000 changes | its lane-mate is unaffected (122 ms median) |
| One organization wiped at the receiver and rebuilt from the start of the topic | all of its changes back in 0.5 s, reading past 20,800 records |
| Broken on purpose: the relay takes "sending finished" for "confirmed", Kafka stopped for 8 s | caught: several hundred changes never arrive (393 of 7,061 on the run that found it), and everything after the gap is refused |
| Broken on purpose: the relay publishes without the organization as key | caught: the ordered receiver applies almost nothing |

### Cost against the direct design

| | Without Kafka | With Kafka |
|---|---|---|
| Save to receiver, healthy | 7 ms median, 12 ms for 19 in 20 (E-003) | 121 ms median, 216 ms for 19 in 20 |
| One organization gets 20,000 changes at once; another organization's changes are applied after (1 ms of work per change) | worker taking organizations in turn: 25 ms median, 57 ms for 19 in 20, worst 61 ms | plain consumer: 8 s median, 22.7 s for 19 in 20, worst 23.8 s, also for an organization in another lane; hand-made consumer taking organizations in turn: 54 ms median, 0.5 s for 19 in 20, worst 2.1 s |
| The same without the in-turn rule (broken on purpose) | 8 s median, 25.5 s worst | as the plain consumer |
| Stateful systems to run | PostgreSQL | PostgreSQL and Kafka, with a relay between |

## Conclusion

**The promises hold with Kafka in the middle, but only with two things built on top, and at a cost.** A relay must advance only on confirmed delivery (the first relay written here did not, and lost changes when Kafka was restarted). A receiver must take organizations in turn by hand, or one organization's backlog delays all the others for many seconds, which D-026 forbids. Delivery is about fifteen times slower than direct, still far inside the 2-second target.

**The design without Kafka keeps one organization from delaying another by construction**: 25 ms against a backlog of 20,000, and 8 s when the in-turn rule is taken out. This is the first measurement of that property; before, it was designed and not tried.

On these results the user chose to go back to the design without a broker (D-028).

What this does not show: Kafka with several nodes or tuned for latency (the client and broker ran with near-default settings, one node); consumers per lane in parallel, which would confine a backlog to its own lane; real apply costs (1 ms was assumed for the backlog scenarios); more than three organizations; the product's own Go code. One run per scenario.
