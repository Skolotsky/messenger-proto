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

async function expectMark(page: Page, info: TestInfo, number: string | null, dot: boolean) {
  for (const place of [entry(page, info), feedTab(page)]) {
    if (number) await expect(place.locator('.badge')).toHaveText(number);
    else await expect(place.locator('.badge')).toHaveCount(0);
    await expect(place.locator('.dot')).toHaveCount(dot ? 1 : 0);
  }
}

test('with Focus and Needs you, the number is both (D-036)', async ({ page, door }, info) => {
  door.rows = [row(FOCUS), ...NEEDS.map((r) => row(r))];
  await start(page);
  await expectMark(page, info, '3', false);
});

test('with Focus only, the number is Focus and the dot does not sit beside it (D-036)', async ({ page, door }, info) => {
  door.rows = [row(FOCUS), ...NEEDS.map((r) => row(r))];
  door.settings = { ...door.settings, focus_only: true };
  await start(page);
  await expectMark(page, info, '1', false);
});

test('with Focus only and Focus empty, Needs you shows as a quiet dot (D-036)', async ({ page, door }, info) => {
  door.rows = NEEDS.map((r) => row(r));
  door.settings = { ...door.settings, focus_only: true };
  await start(page);
  await expectMark(page, info, null, true);
});

test('with Focus only and nothing unread that matters, there is no mark (D-036)', async ({ page, door }, info) => {
  door.rows = [row({ conversation: '#general', bucket: 'fyi', why: 'For your information' })];
  door.settings = { ...door.settings, focus_only: true };
  await start(page);
  await expectMark(page, info, null, false);
});

test('changing the setting changes the mark at once (D-036)', async ({ page, door }, info) => {
  door.rows = [row(FOCUS), ...NEEDS.map((r) => row(r))];
  await start(page);
  await expectMark(page, info, '3', false);
  if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Settings' }).click();
  else await page.locator('.top').getByRole('button', { name: 'Settings' }).click();
  const dialog = page.getByRole('dialog', { name: 'Settings' });
  await dialog.getByRole('navigation').getByRole('button', { name: 'Notifications', exact: true }).click();
  await dialog.getByRole('radio', { name: /Focus only/ }).click();
  await page.keyboard.press('Escape');
  if (isPhone(info)) await entry(page, info).click();
  await expectMark(page, info, '1', false);
});

test('someone on Focus only never sees the larger number while the page loads (D-036)', async ({ page, door }, info) => {
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
  await expectMark(page, info, '1', false);
});
