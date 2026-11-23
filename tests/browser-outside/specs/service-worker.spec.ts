import { expect, test, type BrowserContext, type Page } from '@playwright/test';

// The product's worker has scope /, so it sees every navigation on the address (T-073).
// With CONTEXIA_REAL_URL these run on a real system; the release tests need the local front.
const real = process.env.CONTEXIA_REAL_URL;
const front = real || 'http://127.0.0.1:4191'; // the lane passes an empty value when there is none

test.use({ baseURL: front, serviceWorkers: 'allow' });

const pages = [
  { path: '/control/', title: 'Contexia control panel' },
  { path: '/notifications/', title: 'Contexia notifications' },
];

async function workerInControl(page: Page) {
  await page.goto('/');
  await page.evaluate(() => navigator.serviceWorker.ready);
  await expect.poll(() => page.evaluate(() => navigator.serviceWorker.controller !== null), { timeout: 15_000 }).toBe(true);
}

const release = (context: BrowserContext, name: 'old' | 'new') =>
  context.addCookies([{ name: 'release', value: name, url: front }]);

for (const { path, title } of pages) {
  test(`with the worker installed, ${path} opens as itself, from the network`, async ({ page }) => {
    await workerInControl(page);
    const response = await page.goto(path);
    await expect(page).toHaveTitle(title);
    expect(response!.fromServiceWorker(), 'the worker left this page alone').toBe(false);
  });

  test(`a browser holding the old worker recovers ${path} by itself`, async ({ context, page }) => {
    test.skip(!!real, 'the release is chosen by the local front');
    await release(context, 'old');
    await workerInControl(page);
    await page.goto(path);
    await expect(page, 'the old worker really does take the page over').toHaveTitle('Contexia');

    await release(context, 'new');
    await page.goto(path);
    await expect(page).toHaveTitle(title, { timeout: 15_000 });
    const again = await page.reload();
    await expect(page).toHaveTitle(title);
    expect(again!.fromServiceWorker()).toBe(false);
  });
}

test('with the worker installed, an operator call opened as a page shows its answer', async ({ page }) => {
  await workerInControl(page);
  const response = await page.goto('/ops/feed/alive');
  expect(response!.fromServiceWorker()).toBe(false);
  expect(response!.headers()['content-type']).toContain('application/json');
  await expect(page.locator('body')).toContainText('alive');
});

test('the entry address still asks the server for the current organization', async ({ page }) => {
  await workerInControl(page);
  await page.goto('/');
  expect(new URL(page.url()).searchParams.get('org')).toMatch(/^larkmere-\d+$/);
});

test('a browser holding the old worker gets the entry address back by itself', async ({ context, page }) => {
  test.skip(!!real, 'the release is chosen by the local front');
  await release(context, 'old');
  await workerInControl(page);
  await page.goto('/');
  expect(new URL(page.url()).searchParams.get('org'), 'the old worker really does skip the server').toBeNull();

  await release(context, 'new');
  await page.goto('/');
  await expect.poll(() => new URL(page.url()).searchParams.get('org'), { timeout: 15_000 }).toMatch(/^larkmere-\d+$/);
});

test('the product at an organization is still answered by the worker', async ({ page }) => {
  await workerInControl(page);
  const response = await page.goto('/?org=larkmere-1');
  expect(response!.fromServiceWorker()).toBe(true);
  await expect(page).toHaveTitle('Contexia');
});
