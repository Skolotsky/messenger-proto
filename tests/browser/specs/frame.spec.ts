import { expect, test } from '@playwright/test';
import { expectNoSidewaysScroll, isPhone, start } from './helpers';

test('the organization, channels and direct messages by the other person are shown', async ({ page }, info) => {
  await start(page);
  if (isPhone(info)) {
    await expect(page.locator('.org-line'), 'the organization is above the title on a phone').toHaveText('Acme');
    await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
  }
  await expect(page.locator('.ws-t b')).toHaveText('Acme');
  const nav = page.getByRole('navigation', { name: 'Conversations' });
  await expect(nav.getByRole('button', { name: 'launch' })).toBeVisible();
  await expect(nav.getByRole('button', { name: 'incidents' })).toBeVisible();
  await expect(nav.getByRole('button', { name: 'Anna' }), 'a direct message is named after the other person').toBeVisible();
  await expect(nav).not.toContainText('dm:');
  await expectNoSidewaysScroll(page);
});

test('on a phone the first screen is Activity, with Activity, Channels and Settings below', async ({ page }, info) => {
  test.skip(!isPhone(info), 'phone layout');
  await start(page);
  await expect(page.getByRole('heading', { name: 'Activity' })).toBeVisible();
  await expect(page.locator('.side')).toBeHidden();
  const bar = page.getByRole('navigation', { name: 'Main' });
  for (const label of ['Activity', 'Channels', 'Settings']) await expect(bar.getByRole('button', { name: label })).toBeVisible();
});

test('on a wide screen the sidebar, Activity and the conversation sit side by side', async ({ page }, info) => {
  test.skip(isPhone(info), 'wide layout');
  await start(page);
  const side = (await page.locator('.side').boundingBox())!;
  const main = (await page.locator('.main').boundingBox())!;
  const pane = (await page.locator('.pane').first().boundingBox())!;
  expect(side.x + side.width).toBeLessThanOrEqual(main.x + 1);
  expect(main.x + main.width).toBeLessThanOrEqual(pane.x + 1);
  await expect(page.getByRole('navigation', { name: 'Main' })).toBeHidden();
});
