# Known shortcuts

Every place the design knowingly cuts a corner, with what the full answer would be. Started with the architecture draft (T-022); it grows with each implementation task. Product shortcuts are recorded in the decision that took them and repeated here in one line.

| Shortcut | Why it is acceptable now | The full answer |
|---|---|---|
| No sign-in, roles or private channels; a person switcher (D-009) | The bet is the feed, not access | Accounts, permissions checked at every door |
| Search and notifications are fakes; the tray stands for push (brief, D-009) | The brief allows it; their boundaries are real | A search engine and a push or email provider behind the same contracts |
| People and agents are scripted (D-002) | Agents are not the product | Teams connect their own |
| Acting on behalf of a person through the coding-agent connector (D-002) | Verification only; off unless switched on | Does not exist in a real deployment |
| Agents may read every channel (D-008, proposed) | No access model yet | Reading rights granted per agent |
| One database server with an area per part (D-005, proposed) | Independence is kept by rule and a check, not by hardware | A store per service |
| Ready for many teams, but none of the management is built: no creating or administering teams, no limits per team, one identity per team (D-026) | The first version runs one seeded team; a second exists for the tests | Team administration, limits, one identity across teams |
| One copy of each program and one database serve all teams (D-026) | Teams are independent, so this is a matter of spreading them | Several copies each taking a share of the teams; teams spread over several databases; a broker when balancing them becomes the work |
| One very large team still has one log and one feed | Teams of 30 | Split a team's log by channel; work large channels out when the feed is read |
| Log numbers handed out one at a time (D-005, proposed) | About 1.5 messages a second at a hundred times our size | Split the log by conversation, or read the database's replication log |
| The change log is never trimmed | Two weeks of a 36-member team is small | Keep a window, rebuild from snapshots |
| After a fenced change is fixed, the whole feed is rebuilt, not only the fenced part | Seconds at this size, and exact | Rebuild one conversation or one reader in place |
| A rebuild replays the whole log | Fast at this size (to be measured) | Snapshots and replay from a checkpoint |
| One copy of each part | A local demonstration | Several, with failover |
| Every post writes a feed row per affected reader | Channels of 30 | Work out large channels when the feed is read |
| No encryption between parts, no rate limits except on agents | Everything is local | Both |
| Edit history, undoing a delete, snooze, do-not-disturb, email, digests (D-003, D-004, D-012) | Outside the bet | Product work |
| Not offline; no real push (D-009) | Installable is enough for the first version | A service worker with a send queue; push |
