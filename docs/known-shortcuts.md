# Known shortcuts

Every place the design knowingly cuts a corner, with what the full answer would be. Started with the architecture draft (T-022); it grows with each implementation task. Product shortcuts are recorded in the decision that took them and repeated here in one line.

| Shortcut | Why it is acceptable now | The full answer |
|---|---|---|
| No sign-in, roles or private channels; a person switcher (D-009) | The bet is the feed, not access | Accounts, permissions checked at every door |
| Search and notifications are stand-ins; a separate notification app stands for a phone (brief, D-022) | The brief allows it; their boundaries are real | A search engine and a push or email provider behind the same contracts |
| People and agents are scripted (D-002) | Agents are not the product | Teams connect their own |
| Agents may read every channel (D-008, proposed) | No access model yet | Reading rights granted per agent |
| One database server with an area per part (D-005, proposed) | Independence is kept by rule and a check, not by hardware | A store per service |
| Ready for many organizations, but none of the management is built: no creating or administering organizations, no limits per organization, one identity per organization (D-026) | The first version runs one seeded team; a second exists for the tests | Team administration, limits, one identity across organizations |
| One copy of each program and one database serve all organizations (D-026) | Organizations are independent, so this is a matter of spreading them | Several copies each taking a share of the organizations; organizations spread over several databases; a broker when balancing them becomes the work |
| One organization is handled one change at a time: about 5,000 people in one organization, estimated by analysis, not measured (D-030, D-034) | A seeded team of 36 | For larger organizations there is a path, with no number: conversations applied in parallel, grouped saves, big-channel items stored once, and batched calls to search. Beyond that, a log per conversation |
| Old data is never trimmed; no second database, no moving of an organization, one copy of each program (D-029) | One organization on one machine | Trimming per organization after a saved copy; a directory of databases; an entry point that routes by organization |
| Log numbers handed out one at a time (D-005, proposed) | About 1.5 messages a second at a hundred times our size | Split the log by conversation, or read the database's replication log |
| The change log is never trimmed | Two weeks of a 36-member team is small | Keep a window, rebuild from snapshots |
| After a fenced change is fixed, the whole feed is rebuilt, not only the fenced part | Seconds at this size, and exact | Rebuild one conversation or one reader in place |
| A rebuild replays the whole log | Fast at this size (to be measured) | Snapshots and replay from a checkpoint |
| One copy of each part | A local demonstration | Several, with failover |
| One PostgreSQL is a single point of failure. Not built | A local demonstration; nothing saved is lost while its disk survives, but everything waits while it is down | A standby that confirms each write and takes over automatically |
| Every post writes a feed row per affected reader | Channels of 30 | Work out large channels when the feed is read |
| No encryption between parts, no rate limits except on agents | Everything is local | Both |
| Edit history, undoing a delete, snooze, do-not-disturb, email, digests (D-003, D-004, D-012) | Outside the bet | Product work |
| An open screen asks every 10 s whether the feed is behind (every 5 s while the notice shows), so that cost grows with open screens | A few screens in a demonstration | A "behind" field on the regular `last_sent` message |
| Actions waiting while offline live in the page's memory; reloading the page loses them | The product says they are waiting; a reload is a deliberate act | Keep them in the browser's storage |
| Settings, dials, new channels and new direct messages are not queued offline | They say "Not saved" and change nothing | The same queue as messages |
| Not offline; no real push (D-009) | Installable is enough for the first version | A service worker with a send queue; push |
