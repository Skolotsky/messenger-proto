# D-025: The server is written in Go; the web app in TypeScript; the contracts are the shared source

- Status: accepted
- Level: system shape
- Date: 2026-11-22
- Decided by: user
- Depends on: D-005
- Supersedes in part: D-005 (the server language, TypeScript)
- Evidence: none specific; the experiments E-003 to E-005 concern the design and hold for any language
- Visual: `docs/visuals/T-022-architecture-proposal.html`, section 5

## Why this needs deciding now

D-005 set TypeScript for the server, the user's choice over the agent's proposal of Python. Before any product code exists, the user asked which suits the backend better, TypeScript or Go, noting that with an agent writing the code "one language for everything" counts for less. Changing later would mean rewriting the server.

## Options

### A. TypeScript on the server and in the web app

One language and toolchain; the contracts can be shared as types. Risk: the server is a set of network programs with background workers on a single-threaded runtime; types vanish at run time, so every boundary needs separate validation; a large dependency tree per part.

### B. Go on the server, TypeScript in the web app

Each part is one self-contained program. Concurrency (delivery workers, the live connection, agents waiting for events, retries with timeouts) is what the language and its standard library are built for; failures are handled where they occur; the race detector catches a class of concurrency faults in exactly the code the guardrails protect; what is decoded at a boundary is checked. Cost: two toolchains (Go for the server, Node only to build the web app) and a step that generates the web app's types from the contracts.

## Best practice

Choose the language by what the system does. Small network services with background workers and strict failure handling are a common use of Go; a browser application is written in TypeScript. Where two languages meet, let a written contract be the source and generate each side's types from it, so neither can drift.

## Recommendation

**B.** The argument for A was one language with shared types. With an agent writing the code the first half weighs little, and the second half is kept, in a stricter form, by generating from the contracts.

## Decision

Accepted by the user on 2026-11-22: **B.** "In an era of AI i don't see why i need to use typescript over go", and after the comparison: "Ok, let's then choose go."

## Consequences

- Easier: one program per part with nothing else to ship; concurrency faults found by tooling; boundary data checked when decoded.
- Harder: two toolchains; a generation step between the contracts and the web app's types.
- Committed to: Go for Conversations, the delivery workers, the feed, the stand-ins, the scripted team and control panel's server side; TypeScript in the browser; the contracts under `docs/contracts/` as the source both sides are generated from or checked against.
- Unchanged: everything else in D-005 (separate programs over the network, no broker, PostgreSQL in Docker, WebSocket with numbered updates); the contracts and guardrails; the experiments stay throwaway Python.

## Follow-ups

- Go and Node are not installed on this machine; both are installed, or run in Docker, with the first implementation task.
- The first implementation task sets up the generation of the web app's types from the contracts and a check that fails when they disagree.
