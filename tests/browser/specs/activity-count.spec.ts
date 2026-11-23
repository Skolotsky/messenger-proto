import type { Page, TestInfo } from '@playwright/test';
import { expect, row, test } from '../fixtures/door';
import { isPhone, start } from './helpers';

const FOCUS = { conversation: '#launch', bucket: 'focus', why: 'In Focus: #launch' } as const;
const NEEDS = [
  { conversation: '#general', bucket: 'needs_you', why: 'Mentioned you' },
  { conversation: '#incidents', bucket: 'needs_you', why: 'Mentioned you' },
] as const;

/** Activity on the sidebar, or on the bottom bar of a phone. */
const entry = (page: Page, info: TestInfo) =>
  isPhone(info)
    ? page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: /Activity/ })
    : page.locator('.side-item', { hasText: 'Activity' });
const feedTab = (page: Page) => page.getByRole('navigation', { name: 'Activity views' }).getByRole('button', { name: /^Feed/ });

async function expectMark(page: Page, info: TestInfo, focus: string | null, needs: string | 'dot' | null) {
  for (const place of [entry(page, info), feedTab(page)]) {
    if (focus) await expect(place.locator('.m-focus')).toHaveText(focus);
    else await expect(place.locator('.m-focus')).toHaveCount(0);
    if (needs && needs !== 'dot') await expect(place.locator('.m-needs')).toHaveText(needs);
    else await expect(place.locator('.m-needs')).toHaveCount(0);
    await expect(place.locator('.dot')).toHaveCount(needs === 'dot' ? 1 : 0);
  }
}

test('with Focus and Needs you, each has its own number (D-037)', async ({ page, door }, info) => {
  door.rows = [row(FOCUS), ...NEEDS.map((r) => row(r))];
  await start(page);
  await expectMark(page, info, '1', '2');
});

test('with nothing in Focus, only the Needs you number shows (D-037)', async ({ page, door }, info) => {
  door.rows = NEEDS.map((r) => row(r));
  await start(page);
  await expectMark(page, info, null, '2');
});

test('with Focus only, the second number becomes a dot (D-037)', async ({ page, door }, info) => {
  door.rows = [row(FOCUS), ...NEEDS.map((r) => row(r))];
  door.settings = { ...door.settings, focus_only: true };
  await start(page);
  await expectMark(page, info, '1', 'dot');
});

test('with Focus only and Focus empty, Needs you is a quiet dot (D-037)', async ({ page, door }, info) => {
  door.rows = NEEDS.map((r) => row(r));
  door.settings = { ...door.settings, focus_only: true };
  await start(page);
  await expectMark(page, info, null, 'dot');
});

test('with Focus only and nothing unread that matters, there is no mark (D-037)', async ({ page, door }, info) => {
  door.rows = [row({ conversation: '#general', bucket: 'fyi', why: 'For your information' })];
  door.settings = { ...door.settings, focus_only: true };
  await start(page);
  await expectMark(page, info, null, null);
});

test('changing the setting changes the mark at once (D-037)', async ({ page, door }, info) => {
  door.rows = [row(FOCUS), ...NEEDS.map((r) => row(r))];
  await start(page);
  await expectMark(page, info, '1', '2');
  if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Settings' }).click();
  else await page.locator('.top').getByRole('button', { name: 'Settings' }).click();
  const dialog = page.getByRole('dialog', { name: 'Settings' });
  await dialog.getByRole('navigation').getByRole('button', { name: 'Notifications', exact: true }).click();
  await dialog.getByRole('radio', { name: /Focus only/ }).click();
  await page.keyboard.press('Escape');
  if (isPhone(info)) await entry(page, info).click();
  await expectMark(page, info, '1', 'dot');
});

test('someone on Focus only never sees the larger number while the page loads (D-037)', async ({ page, door }, info) => {
  door.rows = [row(FOCUS), ...NEEDS.map((r) => row(r))];
  door.settings = { ...door.settings, focus_only: true };
  let release = () => {};
  const held = new Promise<void>((done) => (release = done));
  await page.route(/\/people\/[^/]+\/settings$/, async (route) => {
    await held;
    await route.fallback();
  });
  await start(page);
  await expect(feedTab(page)).toBeVisible();
  await expect(entry(page, info).locator('.badge')).toHaveCount(0);
  release();
  await expectMark(page, info, '1', 'dot');
});
