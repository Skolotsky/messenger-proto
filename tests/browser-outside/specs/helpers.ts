import { expect, type Page, type Route } from '@playwright/test';

export interface Call { method: string; path: string; body: unknown }
export type Reply = { status: number; body?: unknown } | 'no-answer' | 'hold';
export type Handle = (req: { method: 'GET' | 'POST' | 'DELETE'; path: string; body?: unknown }) => { status: number; body: unknown };

/**
 * Answers the page's real requests to /ops with a fixture server, and records them.
 * `fail` replaces the answer for a method and path, so a test can plant any failure.
 */
export async function answerCalls(page: Page, handle: Handle) {
  const calls: Call[] = [];
  const fail = new Map<string, Reply>();
  const held: Route[] = [];
  await page.route('**/ops/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace(/^\/ops\//, '') + url.search;
    const method = request.method() as 'GET' | 'POST' | 'DELETE';
    const text = request.postData();
    const body = text ? JSON.parse(text) : undefined;
    calls.push({ method, path, body });
    const planted = fail.get(`${method} ${path}`);
    if (planted === 'no-answer') { await route.abort('failed'); return; }
    if (planted === 'hold') { held.push(route); return; }
    const answer = planted ?? handle({ method, path, body });
    await route.fulfill({ status: answer.status, contentType: 'application/json', body: answer.body === undefined ? '' : JSON.stringify(answer.body) });
  });
  return {
    calls,
    fail,
    /** Lets the held requests through to the fixture server. */
    async release() {
      for (const route of held.splice(0)) {
        const request = route.request();
        const url = new URL(request.url());
        const path = url.pathname.replace(/^\/ops\//, '') + url.search;
        fail.delete(`${request.method()} ${path}`);
        const text = request.postData();
        const answer = handle({ method: request.method() as 'POST', path, body: text ? JSON.parse(text) : undefined });
        await route.fulfill({ status: answer.status, contentType: 'application/json', body: JSON.stringify(answer.body) });
      }
    },
    /** Waits for a call other than a read. */
    async expectCall(method: string, path: string, body?: unknown) {
      await expect.poll(() => calls.filter((c) => c.method === method && c.path === path).length, { message: `${method} ${path}` }).toBeGreaterThan(0);
      if (body !== undefined) expect(calls.filter((c) => c.method === method && c.path === path).at(-1)!.body).toEqual(body);
    },
    writes: () => calls.filter((c) => c.method !== 'GET'),
  };
}

export async function expectNoSidewaysScroll(page: Page) {
  const over = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(over, 'the page scrolls sideways').toBeLessThanOrEqual(0);
}

export async function shot(page: Page, name: string, projectName: string) {
  if (!process.env.CONTEXIA_SHOTS) return;
  for (const theme of ['light', 'dark']) {
    await page.evaluate((t) => (window as unknown as { Theme: { set(v: string): void } }).Theme.set(t), theme);
    await page.screenshot({ path: `shots/${name}-${projectName}-${theme}.png`, fullPage: true });
  }
}
