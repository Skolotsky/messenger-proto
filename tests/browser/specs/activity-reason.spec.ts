import { KEY, busyFeed, expect, row, test } from '../fixtures/door';
import { settled, start, topOf } from './helpers';

const annaRow = KEY('dm:anna:marco');

async function openWhy(page: import('@playwright/test').Page, key: string) {
  await page.locator(`[data-row="${key}"] .why-btn`).click();
  const panel = page.getByRole('dialog', { name: /Why this is in/ });
  await expect(panel).toBeVisible();
  return panel;
}

test('the panel says what it is, what put it here, and where it would go otherwise (D-020)', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  const panel = await openWhy(page, annaRow);
  await expect(panel).toContainText('What it is');
  await expect(panel).toContainText('A direct message from Anna, a person.');
  await expect(panel).toContainText('What put it in Focus');
  await expect(panel).toContainText('Anna is one of your Focus people');
  await expect(panel).toContainText('Otherwise');
  await expect(panel).toContainText('lands in Needs you');
  await expect(panel.getByRole('button', { name: 'Remove Anna from Focus' })).toBeVisible();
  await expect(panel, 'how many it would have moved, and where').toContainText('14 items in the last 7 days would have gone to Needs you instead of Focus.');
});

test('asking why reads nothing and moves nothing (D-020)', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  const target = page.locator(`[data-row="${annaRow}"]`);
  const before = await topOf(target);
  await openWhy(page, annaRow);
  await settled(page);
  expect(door.sent('read'), 'opening the reason is not reading').toEqual([]);
  await expect(target).toHaveClass(/unread/);
  expect(await topOf(target)).toBeCloseTo(before, 0);
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog', { name: /Why this is in/ })).toBeHidden();
  await expect(target).toHaveClass(/unread/);
});

test('the one button acts at once, says what it did and offers Undo; the row stays (D-020, D-021)', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  const target = page.locator(`[data-row="${annaRow}"]`);
  const before = await topOf(target);
  const panel = await openWhy(page, annaRow);
  await panel.getByRole('button', { name: 'Remove Anna from Focus' }).click();

  await expect(panel.getByRole('status')).toContainText('Remove Anna from Focus: done.');
  await expect.poll(() => door.sent('focus_person').length, 'it acts without asking').toBe(1);
  expect(door.sent('focus_person')[0]).toMatchObject({ about: 'anna', on: false });
  expect(await topOf(target), 'the row does not move while looked at').toBeCloseTo(before, 0);

  await panel.getByRole('button', { name: 'Undo' }).click();
  await expect.poll(() => door.sent('focus_person').length).toBe(2);
  expect(door.sent('focus_person')[1]).toMatchObject({ about: 'anna', on: true });
  await expect(panel.getByRole('button', { name: 'Remove Anna from Focus' }), 'the button is back').toBeVisible();
});

test('a refused change says so and changes nothing', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  const panel = await openWhy(page, annaRow);
  door.refuseNext = true;
  await panel.getByRole('button', { name: 'Remove Anna from Focus' }).click();
  await expect(panel.getByRole('alert')).toContainText('did not go through');
  await expect(panel.getByRole('button', { name: 'Undo' })).toHaveCount(0);
});

test('without a preview the button still works (screen-to-conversations.md)', async ({ page, door }) => {
  door.rows = busyFeed();
  door.preview = null;
  await start(page);
  const panel = await openWhy(page, annaRow);
  await expect(panel, 'the count the row already carries is shown').toContainText('19 items in the last 7 days');
  await panel.getByRole('button', { name: 'Remove Anna from Focus' }).click();
  await expect.poll(() => door.sent('focus_person').length).toBe(1);
});

test('the panel links to the place in settings that holds the rule (D-023, point 18)', async ({ page, door }) => {
  door.rows = busyFeed();
  await start(page);
  const panel = await openWhy(page, annaRow);
  await panel.getByRole('button', { name: 'Manage this in settings' }).click();
  const settings = page.getByRole('dialog', { name: 'Settings' });
  await expect(settings.locator('.set-body > h2')).toHaveText('Focus people');
  await expect(settings).toContainText('Anna');
});

test('a channel dial on a row changes the channel, and Undo puts it back', async ({ page, door }) => {
  door.rows = [
    row({
      conversation: '#incidents',
      bucket: 'focus',
      why: 'In Focus: #incidents is set to Focus',
      explanation: {
        what: '6 new posts in #incidents. None of them names you.',
        decided_by: '#incidents is set to Focus',
        otherwise: 'At Default they would not be in your feed at all.',
        change: {
          label: 'Set #incidents to All activity',
          do: { kind: 'dial', conversation: '#incidents', thread: null, level: 'all' },
          undo: { kind: 'dial', conversation: '#incidents', thread: null, level: 'focus' },
          moved_last_7_days: 23,
        },
      },
    }),
  ];
  await start(page);
  const panel = await openWhy(page, KEY('#incidents'));
  await panel.getByRole('button', { name: 'Set #incidents to All activity' }).click();
  await expect.poll(() => door.sent('dial').length).toBe(1);
  expect(door.sent('dial')[0]).toMatchObject({ conversation: '#incidents', thread: null, level: 'all' });
  await panel.getByRole('button', { name: 'Undo' }).click();
  await expect.poll(() => door.sent('dial').length).toBe(2);
  expect(door.sent('dial')[1]).toMatchObject({ level: 'focus' });
});
