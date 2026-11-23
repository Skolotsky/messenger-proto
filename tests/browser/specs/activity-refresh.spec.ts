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

test('a kept message that is deleted is a marker with no Done, not counted, and gone on Refresh', async ({ page, door }) => {
  const gone = row({ conversation: '#marketing', bucket: 'needs_you', unread: false, kept: 'you', why: 'Kept by you' });
  const other = row({ conversation: '#launch', bucket: 'needs_you', unread: false, kept: 'you', why: 'Kept by you' });
  door.rows = [gone, other];
  await start(page);
  const marker = page.locator(`[data-row="${KEY('#marketing')}"]`);
  await expect(page.locator('.kept-head')).toContainText('Kept · 2');

  door.push([{ kind: 'row.changed', row: { ...gone, state: 'retracted', preview: '', reactions: [] } }]);
  await expect(marker).toContainText('deleted this message');
  await expect(marker.getByRole('button', { name: 'Done' }), 'nothing is left to close').toHaveCount(0);
  await expect(page.locator('.kept-head'), 'only what is still kept is counted').toContainText('Kept · 1');

  await page.locator('.top').getByRole('button', { name: 'Refresh' }).click();
  await expect(marker).toHaveCount(0);
  await expect(page.locator(`[data-row="${KEY('#launch')}"]`)).toBeVisible();
});

test('a kept message that was deleted is not there on a fresh load, and alone leaves no Kept heading', async ({ page, door }) => {
  door.rows = [row({ conversation: '#marketing', bucket: 'needs_you', unread: false, kept: 'you', state: 'retracted', preview: '' })];
  await start(page);
  await expect(page.locator('.bucket, section[aria-label="Focus"]').first()).toBeVisible();
  await expect(page.locator(`[data-row="${KEY('#marketing')}"]`)).toHaveCount(0);
  await expect(page.locator('.kept-head')).toHaveCount(0);
});

test('the Kept heading goes when its only row is deleted, and the marker stays', async ({ page, door }) => {
  const only = row({ conversation: '#marketing', bucket: 'needs_you', unread: false, kept: 'you' });
  door.rows = [only];
  await start(page);
  await expect(page.locator('.kept-head')).toContainText('Kept · 1');
  door.push([{ kind: 'row.changed', row: { ...only, state: 'retracted', preview: '', reactions: [] } }]);
  await expect(page.locator(`[data-row="${KEY('#marketing')}"]`)).toContainText('deleted this message');
  await expect(page.locator('.kept-head')).toHaveCount(0);
});
