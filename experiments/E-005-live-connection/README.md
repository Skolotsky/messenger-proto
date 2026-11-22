# E-005: Does a screen that follows the numbered-update contract end right after its connection is cut, refused, or loses single updates?

- Date: 2026-11-22
- Time box: one hour
- Feeds: D-005 (how changes reach a screen; accepted 2026-11-22: WebSocket with numbered updates), D-007 (the screen rows)
- Asked by the user on 2026-11-22, to close the "not tried yet" on the Conversations to Screen contract
- Visual: `docs/visuals/T-022-architecture-proposal.html`, sections 4 and 13

## Hypothesis

The contract is: every update has a number; a screen applies only the next one; after any gap it says the last number it has and gets the rest, or is told "too far behind" and loads the view fresh; a regular "the newest number is N" lets it notice an update lost at the end. We expected this to be enough for the screen to end equal to a fresh load after every kind of interruption, and expected the browser's simpler one-way stream to stop at six tabs.

## Method

A small test server (Python, aiohttp) produces a numbered stream of creates, edits and deletes at 400 a second and offers it over WebSocket. A page with about fifty lines of client code keeps a copy of the state. A real headless browser (Chromium, driven by Playwright) loads the page; the test cuts connections without a goodbye, makes the server refuse connections, and makes it lose single updates silently, then compares the page's state with the server's. Two client variants are broken on purpose. Everything runs in one container with no outside network.

Rerun, about two minutes: `./experiments/E-005-live-connection/run.sh`

Scaled: "the newest number is N" is sent every 0.5 s (proposal: about 20 s); 500 updates are kept ready for catch-up.

## What would change our mind

- The screen differing from a fresh load after any interruption: the contract needs another rule.
- WebSocket tabs failing to connect beyond some number on plain http: the choice against the one-way stream loses its reason.

## Result

Raw output: `results.txt`. 12 checks, none unexpected.

| Scenario | Outcome |
|---|---|
| Steady, 2,000 updates at 400 a second | equal to a fresh load; server to screen under 1 ms median, 5 ms worst (same machine) |
| Connection cut five times mid-burst | equal; reconnected 5 times, each time continuing from its number |
| Server refuses connections for 2 s mid-burst | equal; it had fallen more than 500 behind, was told so and loaded fresh once |
| One update in twenty lost silently | equal; it noticed each gap and asked again 26 times |
| The last update of a burst lost, then silence | equal; noticed through "the newest number is N" |
| Away while 800 updates pass | equal; one fresh load instead of a replay |
| Broken on purpose: applies whatever arrives | caught: 45 of 3,442 items wrong |
| Broken on purpose: reconnects without saying where it was | caught: 3,007 of 3,861 items wrong |
| Twelve tabs on plain http://127.0.0.1, WebSocket | 12 of 12 connected and current |
| Twelve tabs, the browser's one-way stream | 6 of 12 connected: the seventh tab hangs, as the browser documentation says |

## Conclusion

**The contract is sufficient**: under every interruption tried, the screen ended equal to a fresh load, and each rule in it is needed (the two broken clients end wrong). WebSocket has no tab limit on a local address; the one-way stream stops at six, which confirms the choice in D-005.

What this does not show: the real screens (this page holds a flat list, not a feed with held rows and an arrivals button); a phone going to sleep; a proxy between browser and server; speed over a real network; the fast path for text of a reply in progress on a screen.
