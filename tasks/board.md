# Board

Work in flight. Order inside a section is priority. Finished work moves to `done.md`. Rules: CLAUDE.md, section "Tasks".

**Where we are:** The product scope is decided (2026-11-22). Accepted: D-001 the three-bucket feed; D-002 scripted participants; D-003 reading is enough, with Keep and Done; D-004 edits and deletes; D-009 the first-version surfaces; D-010 the large seeded team with a control panel; D-011 a flag has no state of its own; D-012 notifications follow the bucket; D-013 readings from E-001; D-014 reactions; D-015 what read means; D-016 history tabs. In progress: rerunning the E-001 model with the latest decisions. Next: the look-and-feel experiment in a separate session, then the technical decisions (D-005 to D-008), which the user holds until the product scope is final and the experiments are agreed. Text rules judged by a model are in scope but built last, after an experiment.

## Now

- **T-018** `experiment` Rerun E-001 with D-012, D-014 and D-015 in the model: measure how many rows "a reaction from someone you addressed" adds to Needs you and how many notifications a person gets a day; have a fresh verifier check the model again. Also covers the history tabs (D-016)

## Waiting on user

_Nothing._

## Next

- **T-004** `setup` Give the agent a browser it can drive, so interface work can be checked on the rendered page, including a phone-sized screen (D-009). None is connected (checked 2026-11-22). The user approved a headless browser in Docker on 2026-11-22; to be set up in the look-and-feel session, confirming the download once there
- **T-019** `experiment` E-002, look and feel: three clickable versions of the feed (Slack-like, Telegram-like, the agent's own) at desktop and phone width. To be done in a separate session, per the user. Detail: `tasks/T-019-look-and-feel.md`. Needs T-004 first

## Backlog

- **T-008** `research` Look at what Slack's Activity really does when a message is deleted or edited to add or remove a mention (undocumented; needs a real workspace). Informs D-004
- **T-003** `validate` First fresh session: confirm the setup loads. Check, in accept-edits mode so ordinary edits do not prompt: editing `docs/brief.md` still raises a permission prompt while editing another file does not; the `verifier` and `scout` agents launch; the overview page renders correctly in light and dark
