import { test as plain } from '@playwright/test';
import { KEY, ago, busyFeed, expect, row, test } from '../fixtures/door';
import { doorAct, expectNoSidewaysScroll, isPhone, settled, start, unique } from './helpers';

const feedRows = (page: import('@playwright/test').Page) =>
  page.locator('.feed-scroll [data-row]').evaluateAll((els) => els.map((e) => (e as HTMLElement).dataset.row!));

test('three buckets in order, newest first inside each, Kept under Focus (D-003, T-038 16)', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  const headings = await page.locator('.bk-head h2').allTextContents();
  expect(headings).toEqual(['Focus', 'Needs you', 'For your information']);

  const order = await feedRows(page);
  expect(order.slice(0, 2), 'Focus, newest first').toEqual([KEY('dm:anna:marco'), KEY('#incidents')]);
  expect(order.slice(2, 4), 'Kept, inside Focus and below what is new').toEqual([KEY('#launch', 'msg_kept1'), KEY('#general', 'msg_kept2')]);
  expect(order.slice(4, 7), 'Needs you, newest first').toEqual([KEY('#launch', 'msg_n1'), KEY('#general', 'msg_n2'), KEY('#incidents', 'msg_n3')]);
  await expect(page.locator('.b-focus .kept-head')).toContainText('Kept · 2');
  await expect(page.locator('.b-focus > .bk-head .bk-count')).toHaveText('2');
  await expect(page.locator('.b-needs_you > .bk-head .bk-count')).toHaveText('3');
  await expect(page.locator('.b-fyi > .bk-head .bk-dot'), 'For your information shows a dot, not a number').toBeVisible();
  await expectNoSidewaysScroll(page);
});

test('every row says why it is there, and a flagged row says who flagged it', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  await expect(page.locator(`[data-row="${KEY('dm:anna:marco')}"] .why-btn`)).toContainText('direct message from Anna');
  const flagged = page.locator(`[data-row="${KEY('#launch', 'msg_kept1')}"]`);
  await expect(flagged.locator('.why-btn')).toContainText('Flagged by Anna');
  await expect(flagged.locator('.tag.flag')).toBeVisible();
  await expect(page.locator(`[data-row="${KEY('#general', 'msg_kept2')}"] .row-reason em`)).toHaveText('edited since you kept it');
});

test('Kept collapses past a handful and shows its true count (T-038 17)', async ({ page, door }) => {
  door.rows = Array.from({ length: 9 }, (_, i) =>
    row({ conversation: '#general', thread: `msg_k${i}`, message: `msg_k${i}`, bucket: 'focus', unread: false, kept: 'you', why: 'Kept by you', newest: 2000 - i }),
  );
  await start(page);
  const kept = page.locator('.kept-sec [data-row]');
  await expect(page.locator('.kept-head')).toContainText('Kept · 9');
  await expect(kept).toHaveCount(5);
  await page.getByRole('button', { name: 'Show all 9' }).click();
  await expect(kept).toHaveCount(9);
  await page.getByRole('button', { name: 'Show fewer' }).click();
  await expect(kept).toHaveCount(5);
});

test('an empty bucket says so in words, and Focus without rules offers the starting rules', async ({ page, door }) => {
  door.rows = [];
  door.settings = { ...door.settings, auto_add: false, focus_people: [], rules: [] };
  await start(page);
  await expect(page.locator('.b-needs_you .empty')).toContainText('Nobody is waiting on you.');
  await expect(page.locator('.b-fyi .empty')).toContainText('All caught up');
  const suggest = page.locator('.b-focus .suggest');
  await expect(suggest).toContainText('Focus is yours to fill');
  await suggest.locator('.rule', { hasText: 'Any direct mention' }).getByRole('button', { name: 'Add' }).click();
  await expect.poll(() => door.sent('focus_rule').length).toBe(1);
  expect(door.sent('focus_rule')[0]).toMatchObject({ rule: { type: 'item_kind', value: 'mentions' }, on: true });
});

test('a long name and a long message stay inside their row', async ({ page, door }) => {
  door.extraPeople = [{ id: 'maxi', name: 'Maximiliana Alexandra von Hohenzollern-Sigmaringen', is_agent: false, line: null, status: null }];
  door.rows = [
    row({ conversation: '#launch', thread: 'msg_l', message: 'msg_l', author: 'maxi', why: 'Maximiliana Alexandra von Hohenzollern-Sigmaringen mentioned you in a very long reason that must not push anything', preview: 'word '.repeat(200) + 'x'.repeat(300), at: ago(30) }),
  ];
  await start(page);
  const target = page.locator(`[data-row="${KEY('#launch', 'msg_l')}"]`);
  await expect(target).toBeVisible();
  await expectNoSidewaysScroll(page);
  const box = (await target.boundingBox())!;
  const time = (await target.locator('.row-time').boundingBox())!;
  expect(time.x + time.width, 'the time stays inside the row').toBeLessThanOrEqual(box.x + box.width);
  expect(box.height, 'the message is clamped, not a wall of text').toBeLessThan(170);
});

test('a fenced row says it may be out of date', async ({ page, door }, info) => {
  door.rows = [row({ conversation: '#launch', thread: 'msg_fence', message: 'msg_fence', fenced: true })];
  await start(page);
  await expect(page.locator('.row-reason .fenced')).toHaveText('may be out of date');
  test.skip(isPhone(info), 'the wording is the same');
});

plain('through the real mock door: a mention waits behind Show, then lands in Needs you with its reason', async ({ page, request }) => {
  await start(page);
  await expect(page.locator('.bk-head h2').first()).toHaveText('Focus');
  const text = unique('Can you check this');
  const id = `msg_real${Date.now().toString(36)}`;
  await doorAct(request, { as: 'anna', kind: 'send', message: { id, conversation: '#launch', thread: null, text, mentions: ['marco'], level: null } });
  const control = page.locator('.top .newpill');
  await expect(control).toBeVisible();
  await expect(page.locator('.feed-scroll'), 'it waits').not.toContainText(text);
  await control.getByRole('button', { name: 'Show' }).click();
  const landed = page.locator(`.b-needs_you [data-row="${KEY('#launch', id)}"]`);
  await expect(landed).toContainText(text);
  await expect(landed.locator('.why-btn')).toContainText('mentioned you');
});

test('For your information is read by viewing it, on leaving the feed; Needs you is not (D-003, D-015)', async ({ page, door }, info) => {
  door.rows = [
    row({ conversation: '#launch', thread: 'msg_nr', message: 'msg_nr', bucket: 'needs_you', newest: 1500 }),
    row({ conversation: '#general', thread: 'msg_fy', message: 'msg_fy', bucket: 'fyi', why: 'A thread you follow', newest: 1400 }),
  ];
  await start(page);
  await expect(page.locator('.b-fyi [data-row]')).toBeInViewport();
  expect(door.sent('read'), 'nothing is read while the list is on screen').toEqual([]);
  await page.getByRole('navigation', { name: 'Activity views' }).getByRole('button', { name: /^Mentions/ }).click();
  await expect.poll(() => door.sent('read').map((a) => a.thread)).toEqual(['msg_fy']);
  void info;
});

test('For your information far below the view is not read by leaving', async ({ page, door }) => {
  door.rows = [
    ...Array.from({ length: 30 }, (_, i) => row({ conversation: '#launch', thread: `msg_long${i}`, message: `msg_long${i}`, newest: 2000 - i })),
    row({ conversation: '#general', thread: 'msg_far', message: 'msg_far', bucket: 'fyi', newest: 100 }),
  ];
  await start(page);
  await expect(page.locator('.b-fyi [data-row]')).not.toBeInViewport();
  await page.getByRole('navigation', { name: 'Activity views' }).getByRole('button', { name: /^Mentions/ }).click();
  await settled(page);
  expect(door.sent('read')).toEqual([]);
});
