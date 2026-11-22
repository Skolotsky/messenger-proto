import { KEY, busyFeed, entry, expect, row, test } from '../fixtures/door';
import { isPhone, post, start, unique } from './helpers';

const views = (page: import('@playwright/test').Page) => page.getByRole('navigation', { name: 'Activity views' });

test('four history tabs, each with its unread count (D-016, D-019)', async ({ page, door }) => {
  door.rows = busyFeed();
  door.history = {
    direct: [entry({ message: 'msg_h1', conversation: 'dm:anna:marco', unread: true }), entry({ message: 'msg_h2', conversation: 'dm:anna:marco', unread: true })],
    mentions: [entry({ message: 'msg_h3', conversation: '#launch', thread: 'msg_h3', unread: true })],
    threads: [entry({ message: 'msg_h4', conversation: '#launch', thread: 'msg_h4', level: 'fyi' })],
    reactions: [entry({ message: 'msg_h5', conversation: '#launch', by: 'lena', emoji: '👀' })],
  };
  await start(page);
  await expect(views(page).getByRole('button', { name: 'Direct messages, 2 unread' })).toBeVisible();
  await expect(views(page).getByRole('button', { name: 'Mentions, 1 unread' })).toBeVisible();
  await expect(views(page).getByRole('button', { name: 'Threads', exact: true })).toBeVisible();
  await expect(views(page).getByRole('button', { name: 'Reactions', exact: true })).toBeVisible();
  await expect(views(page).locator('.h-count').first(), 'the history count is quiet').not.toHaveClass(/badge/);

  await views(page).getByRole('button', { name: /^Threads/ }).click();
  await expect(page.locator('.hrow')).toContainText('For your information');
  await views(page).getByRole('button', { name: /^Reactions/ }).click();
  await expect(page.locator('.hrow')).toContainText('👀 on your message');
  await expect(page.locator('.hrow')).toContainText('Lena');
});

test('opening an entry reads it and it stays in the tab (D-016)', async ({ page, door, request }) => {
  const id = await post(request, 'anna', '#launch', unique('Mentioned yesterday'));
  door.history = { mentions: [entry({ message: id, conversation: '#launch', thread: id, unread: true, number: 777 })] };
  await start(page);
  await views(page).getByRole('button', { name: /^Mentions/ }).click();
  const item = page.locator(`[data-row="h:${id}"]`);
  await expect(item.locator('.dot')).toBeVisible();
  await item.locator('.row-hit').click();
  await expect.poll(() => door.sent('read').some((a) => a.thread === id && a.up_to === 777)).toBeTruthy();
  await expect(item).toBeAttached();
  await expect(item.locator('.dot')).toHaveCount(0);
});

test('an arrival reaches its tab, behind Show while the tab is open (T-038 14)', async ({ page, door }) => {
  door.rows = busyFeed();
  door.history = { mentions: [entry({ message: 'msg_old', conversation: '#launch', preview: 'The older mention' })] };
  await start(page);
  await views(page).getByRole('button', { name: /^Mentions/ }).click();
  await expect(page.locator('.hrow')).toHaveCount(1);

  door.history.mentions = [entry({ message: 'msg_newer', conversation: '#launch', preview: 'A newer mention', unread: true }), ...door.history.mentions!];
  door.push([
    { kind: 'row.changed', row: row({ conversation: '#launch', thread: 'msg_newer', message: 'msg_newer', newest: 7000 }) },
    { kind: 'history.changed', tab: 'mentions', unread: 1 },
  ]);
  await expect(page.locator('.top .newpill')).toContainText('1 new');
  await expect(page.locator('.hrow'), 'the list does not move').toHaveCount(1);
  await expect(views(page).getByRole('button', { name: 'Mentions, 1 unread' })).toBeVisible();
  await page.locator('.top .newpill').getByRole('button', { name: 'Show' }).click();
  await expect(page.locator('.hrow').first()).toContainText('A newer mention');
  await expect(page.locator('.hrow')).toHaveCount(2);
});

test('a row can be kept from a tab (D-016)', async ({ page, door }, info) => {
  door.history = { mentions: [entry({ message: 'msg_keep', conversation: '#launch', thread: 'msg_keep' })] };
  await start(page);
  await views(page).getByRole('button', { name: /^Mentions/ }).click();
  const item = page.locator('[data-row="h:msg_keep"]');
  if (!isPhone(info)) await item.hover();
  await item.getByRole('button', { name: 'Keep' }).click();
  await expect(item.getByRole('button', { name: 'Kept' })).toBeVisible();
  await expect.poll(() => door.sent('keep').length).toBe(1);
  expect(door.sent('keep')[0]).toMatchObject({ conversation: '#launch', thread: 'msg_keep' });
});

test('going to a history tab and back leaves the feed in order', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  await views(page).getByRole('button', { name: /^Mentions/ }).click();
  await expect(page.locator('.bk-head')).toHaveCount(0);
  await views(page).getByRole('button', { name: /^Feed/ }).click();
  await expect(page.locator('.bk-head')).toHaveCount(3);
  await expect(page.locator(`[data-row="${KEY('dm:anna:marco')}"]`)).toBeVisible();
});
