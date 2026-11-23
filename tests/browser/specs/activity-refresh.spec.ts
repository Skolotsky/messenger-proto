import { KEY, expect, row, test } from '../fixtures/door';
import { expectNoSidewaysScroll, start } from './helpers';

test('Refresh clears what was read from the feed, and only when pressed', async ({ page, door }) => {
  door.rows = [
    row({ conversation: '#launch', bucket: 'focus', why: 'In Focus: #launch' }),
    row({ conversation: '#general', bucket: 'needs_you', why: 'Mentioned you' }),
  ];
  await start(page);
  const read = page.locator(`[data-row="${KEY('#launch')}"]`);
  const other = page.locator(`[data-row="${KEY('#general')}"]`);
  await expect(read).toBeVisible();
  await page.getByRole('button', { name: 'Mark all read' }).first().click();
  await expect(read).not.toHaveClass(/unread/);
  await expect(read, 'nothing leaves until the person asks').toBeVisible();

  await page.locator('.top').getByRole('button', { name: 'Refresh' }).click();
  await expect(read).toHaveCount(0);
  await expect(other).toBeVisible();
  await expectNoSidewaysScroll(page);
});

test('Refresh is there on a history tab too', async ({ page }) => {
  await start(page);
  await page.getByRole('navigation', { name: 'Activity views' }).getByRole('button', { name: /^Mentions/ }).click();
  const answer = page.waitForResponse((r) => r.url().includes('/history?') && r.url().includes('tab=mentions'));
  await page.locator('.top').getByRole('button', { name: 'Refresh' }).click();
  expect((await answer).ok()).toBe(true);
});
