# T-051: The seeded organization, as data

- Traces to: brief ("Seed enough data that the experience resembles a busy team"), D-002, D-010, D-001, D-011, D-012, D-014, D-016, D-021, D-032; `docs/contracts/operations-and-control.md` (the seed session), `docs/contracts/conversations-to-feed.md`
- Started: 2026-11-22

## Outcome

One invented organization, 30 people, 6 agents and 20 channels, with two weeks of history that reads like a busy product team, and the situations of D-010 as replayable scripts that name the feed outcome for whom. Data in `demo/content/` (format in its `README.md`) plus a small Go package, `server/internal/democontent`; no service.

## Acceptance criteria

- [x] The same seed gives the same content byte for byte; another seed differs. Check: `TestSameSeedSameBytes`, with the default history's SHA-256 pinned
- [x] Every situation in D-010 exists and names the feed outcome, for whom. Check: `TestEverySituationOfD010Exists`; all 39 named outcomes replayed against the E-001 rules model
- [x] The history as ordered changes validates against `Change`, with unbroken numbering and never-decreasing times. Check: `TestHistoryIsAValidLog`, `TestSituationsCompileToValidChanges`
- [x] Messages read as real work, varied, with believable proportions. Check: `TestProportions` (three seeds), `TestTextIsNotFiller`, and samples read by the author

## Verification plan

- Automated: Go tests in `server/internal/democontent`, through `./scripts/check`
- Real system: none; this package has no running program. T-059 plays it
- Independent: no; the lead validates at the milestone
- Not covered: playing the content through the real doors (T-059); the real feed's rules (T-049), which the situations are checked against only through the E-001 model

## Readings of the decisions

- D-010's "a flag that names nobody and closes when someone else answers" is superseded by D-011: a flag has no state. The situation shows that someone else's answer changes nothing for you, and the row leaves when you read it.
- E-002's names are not reused where they could be real: the company is Larkmere (was Larkspur), its clinics are newly invented, and two surnames changed (Marco Bellandi, Tomás Arrieta) plus Yuki Hayashida. First names, roles and the core cast are E-002's.
- Android, iOS, Safari and Windows are named as platforms clinics use. No company is named; `TestTextIsNotFiller` refuses a list of well-known ones.

## Requests for shared changes

1. **T-059 / Compose:** the server image is built with context `../server`, so `demo/content` is not inside it. The demo program needs it, for example a read-only mount at `/content` and `democontent.Load(os.DirFS("/content"))`.
2. **After T-049 merges:** add a test that replays `situations.json` through the real rules package and compares every `expect`. Today that comparison is a throwaway script over the E-001 model, which has no conversation-type Focus rule and predates D-021 to D-023.

## Log

- 2026-11-22: format, cast, story, agents' routine traffic, situations and tests written.
- 2026-11-22: reading samples found defects the tests had not: builds out of order, ticket numbers repeating, research progress going backwards, "We are frozen" before the freeze, a customer with tickets before it signed, a monitoring reply from the marketing lead. Fixed: exchanges can be pinned to a day, agents' numbers rise, each agent is answered by people who would answer it.
- 2026-11-22: replaying through the model with the seeded Focus rules showed a situation would have been wrong (Omar is Maya's Focus person, so his reply lands in Focus). The replier changed.

- 2026-11-22: Correction to commit a9b5542: its message says the replay test failed when the rule keeping a read flag in Focus was removed. That planted change only broke the build, and a corrected version of it changes no behaviour, because the code falls through to the same answer. Two faults that do change behaviour were then planted, and the replay test caught both (`"ask" is in focus_new, expected focus_kept`).

## Evidence

Default seed (20261117): 7,956 changes in the log, of which 1,564 messages (813 replies, 237 with a mention, 43 flags, 300 Info, 382 by agents), 29 edits, 8 deletes, 806 reactions added and 13 taken back. Two other seeds give 1,542 and 1,553 messages.

- Each check seen failing, by planting one fault at a time and restoring it: a skipped number and a backward time (`TestHistoryIsAValidLog`); an unknown level (schema); randomness not from the seed and an ignored seed (`TestSameSeedSameBytes`); a missing situation (`TestEverySituationOfD010Exists`); a line repeated and a real company name (`TestTextIsNotFiller`); no edits (`TestProportions`).
- Situations: 16 situations, 39 named outcomes, 0 differences from the E-001 model with the seeded dials and Focus rules. Planting Omar as the replier produced the expected difference.
- Feed at the end of history, by the E-001 model: Maya 2 new in Focus, 3 in Needs you, 16 in For your information; other core people 0 to 2, 1 to 5, and 4 to 19.
- Not verified: the real feed's rules (T-049), the seed session against a running Conversations (T-046, T-059), and how the volume feels on screen.
