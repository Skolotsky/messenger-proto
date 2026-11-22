import { expect, test } from '@playwright/test';
import { start } from './helpers';

test.use({ serviceWorkers: 'allow' });

const pngSize = (b: Buffer) => [b.readUInt32BE(16), b.readUInt32BE(20)];

test('the manifest makes the app installable, with its own icons', async ({ page, request }) => {
  await start(page);
  const href = await page.locator('link[rel="manifest"]').getAttribute('href');
  const res = await request.get(new URL(href!, page.url()).href);
  expect(res.ok()).toBeTruthy();
  const manifest = await res.json();
  expect(manifest.name).toBe('Contexia');
  expect(manifest.display).toBe('standalone');
  expect(manifest.start_url).toBe('/');
  for (const size of [192, 512]) {
    const icon = manifest.icons.find((i: { sizes: string }) => i.sizes === `${size}x${size}`);
    expect(icon, `an icon of ${size} px`).toBeTruthy();
    const file = await request.get(new URL(icon.src, page.url()).href);
    expect(file.ok()).toBeTruthy();
    expect(pngSize(await file.body()), 'the file is really that size').toEqual([size, size]);
  }
});

test('a service worker takes control, and never stores an answer from the door', async ({ page }) => {
  await start(page);
  await page.evaluate(() => navigator.serviceWorker.ready);
  await page.reload();
  await expect.poll(() => page.evaluate(() => navigator.serviceWorker.controller !== null)).toBe(true);
  await expect(page.locator('.app')).toBeVisible();
  const cached = await page.evaluate(async () => {
    const urls: string[] = [];
    for (const name of await caches.keys()) {
      for (const req of await (await caches.open(name)).keys()) urls.push(req.url);
    }
    return urls;
  });
  expect(cached.length, 'the app files are stored').toBeGreaterThan(0);
  expect(cached.filter((u) => u.includes('/orgs/'))).toEqual([]);
});
