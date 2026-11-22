import { KEY, busyFeed, countsOf, expect, row, test } from '../fixtures/door';
import { isPhone, settled, start, topOf } from './helpers';

const pill = (page: import('@playwright/test').Page) => page.locator('.top .newpill');
const order = (page: import('@playwright/test').Page) =>
  page.locator('.feed-scroll [data-row]').evaluateAll((els) => els.map((e) => (e as HTMLElement).dataset.row!));

test('an arrival waits behind Show and the list does not change (D-023 point 8)', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  const before = await order(page);
  await expect(pill(page)).toHaveCount(0);

  door.arrive(row({ conversation: '#general', thread: 'msg_new1', message: 'msg_new1', author: 'yuki', why: 'Yuki mentioned you', preview: 'Fresh arrival', newest: 3000 }));
  await expect(pill(page)).toContainText('1 new');
  await expect(page.locator('.feed-scroll'), 'not in the list yet').not.toContainText('Fresh arrival');
  expect(await order(page)).toEqual(before);

  await pill(page).getByRole('button', { name: 'Show' }).click();
  await expect(page.locator('.feed-scroll')).toContainText('Fresh arrival');
  await expect(pill(page)).toHaveCount(0);
  const after = await order(page);
  expect(after.indexOf(KEY('#general', 'msg_new1')), 'newest first in Needs you').toBe(after.indexOf(KEY('#launch', 'msg_n1')) - 1);
});

test('the control counts rows that changed, not messages, and names Focus and Needs you (T-038 15)', async ({ page, door }, info) => {
  door.rows = busyFeed();
  await start(page);
  const same = (newest: number, n: number) => row({ conversation: '#general', thread: 'msg_burst', message: `msg_burst${n}`, why: 'Anna mentioned you', newest, new_items: n, preview: `Burst ${n}` });
  door.arrive(same(3001, 1));
  door.arrive(same(3002, 2));
  door.arrive(same(3003, 3));
  door.arrive(row({ conversation: 'dm:anna:marco', bucket: 'focus', why: 'In Focus: direct message from Anna', newest: 3004, new_items: 5, preview: 'And another' }));
  await expect(pill(page).locator('b')).toHaveText('2 new');
  if (!isPhone(info)) await expect(pill(page).locator('small')).toHaveText('1 in Focus · 1 need you');
});

test('no arrival is lost: batches queue, and all are there after Show (T-038 13)', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  for (let i = 0; i < 3; i++) {
    door.arrive(row({ conversation: '#general', thread: `msg_q${i}`, message: `msg_q${i}`, preview: `Queued ${i}`, newest: 3100 + i }));
    await expect(pill(page).locator('b')).toHaveText(`${i + 1} new`);
  }
  await pill(page).getByRole('button', { name: 'Show' }).click();
  for (let i = 0; i < 3; i++) await expect(page.locator('.feed-scroll')).toContainText(`Queued ${i}`);
});

test('a new message on a row marked Done brings the row back (T-038 13)', async ({ page, door }, info) => {
  const kept = row({ conversation: '#launch', thread: 'msg_d', message: 'msg_d', bucket: 'focus', unread: false, kept: 'you', why: 'Kept by you', preview: 'The old text', newest: 1000 });
  door.rows = [kept];
  await start(page);
  const target = page.locator(`[data-row="${KEY('#launch', 'msg_d')}"]`);
  await target.getByRole('button', { name: 'Done', exact: true }).last().click();
  await expect(target.locator('.state')).toHaveText('Done');
  await expect.poll(() => door.sent('done').length).toBe(1);

  door.arrive({ ...kept, kept: null, unread: true, newest: 1500, first_new: 1500, preview: 'Back again', why: 'Reply in a thread you started', bucket: 'needs_you' });
  await expect(pill(page)).toContainText('1 new');
  await expect(target, 'still the old row until asked').toContainText('The old text');
  await pill(page).getByRole('button', { name: 'Show' }).click();
  const back = page.locator(`.b-needs_you [data-row="${KEY('#launch', 'msg_d')}"]`);
  await expect(back).toContainText('Back again');
  await expect(back).toHaveClass(/unread/);
  test.skip(isPhone(info), 'the same on a phone');
});

test('with Activity closed, or another view open, arrivals still raise the count (T-038 12)', async ({ page, door }, info) => {
  door.rows = busyFeed();
  await start(page);
  const before = countsOf(door.rows);
  const expected = String(before.focus + before.needs_you + 1);
  if (isPhone(info)) {
    await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
  } else {
    await page.getByRole('button', { name: 'Close Activity' }).click();
    await expect(page.locator('.main')).toBeHidden();
  }
  door.arrive(row({ conversation: '#general', thread: 'msg_c', message: 'msg_c', preview: 'While away', newest: 3200 }));
  const badge = isPhone(info)
    ? page.getByRole('navigation', { name: 'Main' }).locator('.badge')
    : page.locator('.side-item .badge');
  await expect(badge).toHaveText(expected);
});

test('coming back to the feed lays it out afresh, with no Show needed (D-003)', async ({ page, door }, info) => {
  door.rows = busyFeed();
  await start(page);
  if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
  else await page.getByRole('button', { name: 'Close Activity' }).click();
  door.arrive(row({ conversation: '#general', thread: 'msg_back', message: 'msg_back', preview: 'Arrived while away', newest: 3300 }));
  if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Activity' }).click();
  else await page.getByRole('button', { name: 'Open or close Activity' }).or(page.locator('.side-item')).first().click();
  await expect(page.locator('.feed-scroll')).toContainText('Arrived while away');
  await expect(pill(page)).toHaveCount(0);
});

test('a row read elsewhere dims in place and does not move', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  const key = KEY('#launch', 'msg_n1');
  const target = page.locator(`[data-row="${key}"]`);
  const before = await topOf(target);
  const current = door.rows.find((r) => r.message === 'msg_n1')!;
  door.push([{ kind: 'row.changed', row: { ...current, unread: false } }]);
  await expect(target).toHaveClass(/(^|\s)read(\s|$)/);
  await settled(page);
  expect(Math.abs((await topOf(target)) - before)).toBeLessThan(0.5);
  await expect(pill(page), 'reading elsewhere is not an arrival').toHaveCount(0);
});
