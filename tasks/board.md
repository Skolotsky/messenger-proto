# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** The first version is built and validated (2026-11-23): the messaging product, the activity feed, reliable delivery to search and notifications, and the seeded demonstration. `make up` runs it; `make verify` passes on the final code. What was not verified, and the calls made without asking, are in `tasks/T-061-final-validation.md`.

## Now

_Nothing in progress._

## Waiting on user

- **T-034** `guardrails` Seven contracts (`docs/contracts/`) and guardrails G-003 to G-030 are written (G-027 to G-030 for growing by adding copies and databases and for one large organization, D-029 and D-030) (with the organization in every contract since D-026, and G-025, G-026 for keeping organizations apart and independent) from the accepted technical decisions; the user reviews the sentences in `docs/visuals/T-034-guardrails.html`. All product guardrails are `pending` until their tests exist. An independent verifier found four blockers and six larger gaps (delivery of notifications unprotected, a screen numbering that could not work with filtered updates, two boundaries without a contract, a guardrail whose check could pass while it was false); all answered, corrections not re-checked
- **T-021** `decide` Nine small behaviours the E-001 model still chooses on its own (list in `experiments/E-001-feed-rules-model/README.md`, "Still open after this run"). None blocks the look-and-feel work. The notification default was confirmed by the user; its figure of about 29 a day comes from a simulation with assumed traffic

## Next




## Backlog

- **T-066** `validate` Write G-015 and G-019's guardrail tests on the real system. Their behaviour is covered on the mock door and on the real main path
- **T-067** `decide` Put "the feed is behind" on `last_sent` instead of each screen asking every 10 s
- **T-062** `not planned` Text rules judged by a model: taken out of scope by the user (D-034)
- **T-060** `not planned` The coding-agent connector: taken out of scope by the user (D-034)
- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
