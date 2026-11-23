# T-073: The product's worker leaves the other pages alone

- Traces to: D-009 (installable web app), D-022 and D-024 (the two pages outside the product), D-010 (control panel)
- Started: 2026-11-23

## Outcome

After someone has opened the product, `/control/`, `/notifications/` and the operator calls under `/ops/` still open as themselves. A browser that already holds the old worker recovers by itself on its next visit.

## Acceptance criteria

- [ ] With the worker in control, `/control/` and `/notifications/` show their own page and come from the network. Check: `tests/browser-outside/specs/service-worker.spec.ts`, both widths
- [ ] With the worker in control, an operator call under `/ops/` opened as a page shows its answer. Check: same spec
- [ ] The product's own address, with and without `?org=`, is still answered by the worker. Check: same spec
- [ ] A browser holding the old worker, which still takes the pages over, shows the right page after the new release without clearing anything. Check: same spec, plus once with a real build of the previous commit
- [ ] The same holds on the real server. Check: the spec against a private composed system

## Verification plan

- Automated: the built pages behind a small server laid out like the real one, with the release chosen per browser
- Real system: a private copy of the system, so the running demo is not touched
- Independent: no; a small routing fix whose proof is the browser test
- Not covered: browsers other than Chromium

## Log

- 2026-11-23: Started. The product has one address, so the worker's fallback is limited to it (an allowlist) rather than listing the pages to exclude: a page added later cannot be swallowed.
- 2026-11-23: Found on the way: the worker also answered a bare `/`, skipping the server's redirect to the current organization, so the app opened the default organization `acme`. The worker now answers only `/?org=…`, and `/` is no longer mapped to the stored page.
- 2026-11-23: `/ops/` with nothing after it is not a page or a call: the server answers it with the product, with or without a worker. Left as it is.

## Evidence

All criteria met.

- Before the fix: the spec failed at both widths on `/control/`, `/notifications/`, `/ops/feed/alive`, and on the entry address.
- After: `service-worker.spec.ts` 16 of 16 on the built pages.
- Old worker: 6 of 6 upgrading from a real build of commit e6f385e, with nothing cleared.
- Real server: 10 of 10 on a private composed system (the 6 others need the local front to switch release).
- `./scripts/check` passes. Browser lanes: wide 191 passed, phone 178 passed, 12 skipped between them; outside pages 112 passed.
- Not run: the full `./scripts/verify` (no server code changed); browsers other than Chromium.

