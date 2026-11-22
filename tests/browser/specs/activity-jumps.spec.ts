import type { Locator, Page } from '@playwright/test';
import { KEY, busyFeed, expect, row, test, type FixtureDoor } from '../fixtures/door';
import { isPhone, settled, start, topOf } from './helpers';

// "Nothing jumps", measured as in E-002: the position of what is being read, before and after.
const TOLERANCE = 0.5;

const WATCHED = KEY('#general', 'msg_watch');

/** A list long enough to scroll, scrolled so that the watched row is in the middle of the view. */
async function reading(page: Page, door: FixtureDoor) {
  door.rows = [
    ...busyFeed(),
    ...Array.from({ length: 14 }, (_, i) =>
      row({ conversation: '#general', thread: `msg_fill${i}`, message: `msg_fill${i}`, author: 'priya', why: 'Priya mentioned you', preview: `Filler ${i}`, newest: 900 - i }),
    ),
    row({ conversation: '#general', thread: 'msg_watch', message: 'msg_watch', author: 'tomas', why: 'Tomas mentioned you', preview: 'The row being read. It has a sentence long enough to fill its line.', newest: 893 }),
  ];
  await start(page);
  const watched = page.locator(`[data-row="${WATCHED}"]`);
  const scroller = page.getByTestId('feed');
  await watched.evaluate((el) => {
    const box = el.closest('.feed-scroll')!;
    box.scrollTop += el.getBoundingClientRect().top - box.getBoundingClientRect().top - 160;
  });
  await settled(page);
  expect(await scroller.evaluate((el) => el.scrollTop), 'the list really is scrolled').toBeGreaterThan(100);
  return { watched, scroller };
}

async function expectStill(page: Page, watched: Locator, before: number, what: string) {
  await settled(page);
  const moved = Math.abs((await topOf(watched)) - before);
  expect(moved, `${what}: the row being read moved ${moved.toFixed(2)} px`).toBeLessThan(TOLERANCE);
}

const rowOf = (door: FixtureDoor, key: string) => door.rows.find((r) => KEY(r.conversation, r.thread) === key)!;

test('an arrival moves nothing', async ({ page, door }) => {
  const { watched } = await reading(page, door);
  const before = await topOf(watched);
  door.arrive(row({ conversation: '#launch', thread: 'msg_arr', message: 'msg_arr', preview: 'Arrives above', newest: 5000 }));
  await expect(page.locator('.top .newpill')).toBeVisible();
  await expectStill(page, watched, before, 'arrival');
});

test('the arrivals control appearing moves nothing below the title', async ({ page, door }) => {
  await reading(page, door);
  const head = await page.locator('.tabs').boundingBox();
  door.arrive(row({ conversation: '#launch', thread: 'msg_arr2', message: 'msg_arr2', newest: 5001 }));
  await expect(page.locator('.top .newpill')).toBeVisible();
  expect((await page.locator('.tabs').boundingBox())!.y).toBeCloseTo(head!.y, 1);
});

test('an edit to a much longer text keeps the row exactly its height (D-001)', async ({ page, door }) => {
  const { watched } = await reading(page, door);
  const before = await topOf(watched);
  const height = (await watched.boundingBox())!.height;
  const target = rowOf(door, WATCHED);
  door.push([{ kind: 'row.changed', row: { ...target, state: 'changed', preview: 'Much longer now. '.repeat(40) } }]);
  await expect(watched).toContainText('Much longer now');
  await expectStill(page, watched, before, 'a longer edit');
  expect((await watched.boundingBox())!.height, 'the row kept its height').toBeCloseTo(height, 0);
});

test('an edit to a shorter text keeps the row exactly its height (D-001)', async ({ page, door }) => {
  const { watched } = await reading(page, door);
  const next = page.locator(`[data-row="${KEY('#general', 'msg_fill0')}"]`);
  const nextTop = await topOf(next);
  const height = (await watched.boundingBox())!.height;
  const target = rowOf(door, WATCHED);
  door.push([{ kind: 'row.changed', row: { ...target, state: 'changed', preview: 'Ok.' } }]);
  await expect(watched).toContainText('Ok.');
  await settled(page);
  expect((await watched.boundingBox())!.height).toBeCloseTo(height, 0);
  expect(Math.abs((await topOf(next)) - nextTop)).toBeLessThan(TOLERANCE);
});

test('a row above being removed moves nothing being read', async ({ page, door }) => {
  const { watched } = await reading(page, door);
  const before = await topOf(watched);
  // Scrolled past: this one was never seen and is out of view, so it goes (D-004).
  door.push([{ kind: 'row.removed', row: { conversation: 'dm:anna:marco', thread: null } }]);
  await expect(page.locator(`[data-row="${KEY('dm:anna:marco')}"]`)).toHaveCount(0);
  await expectStill(page, watched, before, 'a row above leaving');
});

test('a deleted row in view becomes a marker of the same height, with no text left (D-004)', async ({ page, door }) => {
  const { watched } = await reading(page, door);
  const before = await topOf(watched);
  const height = (await watched.boundingBox())!.height;
  const target = rowOf(door, WATCHED);
  door.pushConversation([{ kind: 'message.deleted', message: { id: target.message, version: 2, conversation: target.conversation, thread: target.thread } }]);
  await expect(watched).toContainText('deleted this message');
  await expect(watched).not.toContainText('The row being read');
  await expectStill(page, watched, before, 'a delete');
  expect((await watched.boundingBox())!.height).toBeCloseTo(height, 0);
});

test('a first reaction on a row waits behind Show, because it would add a line', async ({ page, door }) => {
  const { watched } = await reading(page, door);
  const before = await topOf(watched);
  const target = rowOf(door, WATCHED);
  door.push([{ kind: 'row.changed', row: { ...target, reactions: [{ emoji: '👀', people: ['lena'] }] } }]);
  await expect(page.locator('.top .newpill')).toBeVisible();
  await expect(watched.locator('.row-rxs')).toHaveCount(0);
  await expectStill(page, watched, before, 'a first reaction');
});

test('Keep and Mark read change the row in place', async ({ page, door }, info) => {
  test.skip(isPhone(info), 'on a phone these are swipes: activity-phone.spec');
  const { watched } = await reading(page, door);
  const before = await topOf(watched);
  await watched.hover();
  await watched.getByRole('button', { name: 'Mark read' }).click();
  await expect(watched).toHaveClass(/read/);
  await expectStill(page, watched, before, 'Mark read');
  await watched.hover();
  await watched.getByRole('button', { name: 'Keep', exact: true }).click();
  await expect(watched.locator('.state')).toHaveText('Kept');
  await expectStill(page, watched, before, 'Keep');
});

test('hovering a row moves nothing and takes no room', async ({ page, door }, info) => {
  test.skip(isPhone(info), 'a phone has no hover');
  const { watched } = await reading(page, door);
  const before = await topOf(watched);
  const text = watched.locator('.row-preview');
  const place = () => text.evaluate((n) => { const r = n.getBoundingClientRect(); return [r.left, r.width]; });
  const sideways = await place();
  await watched.locator('.row-main').hover();
  await expect(watched.locator('.row-btns')).toHaveCSS('opacity', '1');
  await expectStill(page, watched, before, 'hover');
  expect(await place(), 'the text keeps its place and width').toEqual(sideways);
});
