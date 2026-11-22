import type { Page } from '@playwright/test';
import { KEY, busyFeed, expect, row, test } from '../fixtures/door';
import { isPhone, settled, start, topOf } from './helpers';

test.beforeEach(({}, info) => {
  test.skip(!isPhone(info), 'the phone gestures');
});

/** A real touch drag, through the browser's own input pipeline. */
async function swipe(page: Page, selector: string, dx: number) {
  const box = (await page.locator(selector).boundingBox())!;
  const y = box.y + box.height / 2;
  const x0 = box.x + box.width / 2;
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x: x0, y }] });
  for (let i = 1; i <= 6; i++) {
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x: x0 + (dx * i) / 6, y }] });
  }
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
  await cdp.detach();
  await settled(page);
}

const key = KEY('#launch', 'msg_n1');
const sel = `[data-row="${key}"] .row-main`;

test('every row has a visible React button', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  const target = page.locator(`[data-row="${key}"]`);
  await expect(target.getByRole('button', { name: 'React' })).toBeVisible();
  await target.getByRole('button', { name: 'React' }).click();
  await expect(page.getByRole('dialog', { name: 'Choose a reaction' })).toBeVisible();
});

test('swiping right marks read and the row stays in place', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  const target = page.locator(`[data-row="${key}"]`);
  const before = await topOf(target);
  await swipe(page, sel, 110);
  await expect(target).toHaveClass(/read/);
  await expect.poll(() => door.sent('read').length).toBe(1);
  expect(door.sent('read')[0]).toMatchObject({ conversation: '#launch', thread: 'msg_n1' });
  expect(door.sent('keep')).toEqual([]);
  expect(Math.abs((await topOf(target)) - before)).toBeLessThan(0.5);
  await expect(page.locator('.pane').first(), 'a swipe does not open the conversation').toBeHidden();
});

test('swiping left keeps, and the row stays in place', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  const target = page.locator(`[data-row="${key}"]`);
  const before = await topOf(target);
  await swipe(page, sel, -120);
  await expect(target.locator('.state')).toHaveText('Kept');
  await expect.poll(() => door.sent('keep').length).toBe(1);
  expect(door.sent('read')).toEqual([]);
  expect(Math.abs((await topOf(target)) - before)).toBeLessThan(0.5);
});

test('on a kept row, swiping right is Done, and Keep takes it back (D-023 point 22)', async ({ page, door }) => {
  const keptKey = KEY('#launch', 'msg_kd');
  door.rows = [row({ conversation: '#launch', thread: 'msg_kd', message: 'msg_kd', bucket: 'focus', unread: false, kept: 'you', why: 'Kept by you' })];
  await start(page);
  const target = page.locator(`[data-row="${keptKey}"]`);
  await swipe(page, `[data-row="${keptKey}"] .row-main`, 110);
  await expect(target.locator('.state')).toHaveText('Done');
  await expect.poll(() => door.sent('done').length).toBe(1);
  await swipe(page, `[data-row="${keptKey}"] .row-main`, -120);
  await expect(target.locator('.state')).toHaveText('Kept');
  await expect.poll(() => door.sent('keep').length).toBe(1);
});

test('a short drag does nothing; a vertical one scrolls', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  await swipe(page, sel, 30);
  expect(door.sent('read')).toEqual([]);
  expect(door.sent('keep')).toEqual([]);
  await expect(page.locator(`[data-row="${key}"]`)).toHaveClass(/unread/);
});

test('a tap opens the conversation over the feed (D-023 point 9)', async ({ page, door }) => {
  door.rows = [row({ conversation: 'dm:anna:marco', bucket: 'focus', why: 'In Focus: direct message from Anna' })];
  await start(page);
  await page.locator(`[data-row="${KEY('dm:anna:marco')}"] .row-hit`).tap({ position: { x: 24, y: 24 } });
  const pane = page.getByRole('region', { name: 'Conversation' });
  await expect(pane).toBeVisible();
  const box = (await pane.boundingBox())!;
  expect(box.width).toBe(400);
  await pane.getByRole('button', { name: 'Back' }).click();
  await expect(page.getByRole('heading', { name: 'Activity' })).toBeVisible();
});
