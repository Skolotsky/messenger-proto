import { test as plain } from '@playwright/test';
import { KEY, busyFeed, expect, row, test } from '../fixtures/door';
import { isPhone, openConversation, post, start, unique } from './helpers';

const picker = (page: import('@playwright/test').Page) => page.getByRole('dialog', { name: 'Choose a reaction' });

test('the picker has search, and finds an emoji by name (D-023 point 2)', async ({ page, door }, info) => {
  door.rows = busyFeed();
  await start(page);
  const target = page.locator(`[data-row="${KEY('dm:anna:marco')}"]`);
  if (!isPhone(info)) await target.hover();
  await target.getByRole('button', { name: 'React' }).click();
  await expect(picker(page)).toBeVisible();
  expect(await picker(page).locator('.pk-grid button').count(), 'a full picker').toBeGreaterThan(50);
  await picker(page).getByRole('searchbox', { name: 'Search emoji' }).fill('rocket');
  await expect(picker(page).locator('.pk-grid button')).toHaveCount(1);
  await picker(page).getByRole('searchbox', { name: 'Search emoji' }).fill('zzzz');
  await expect(picker(page)).toContainText('No emoji by that name.');
  await page.keyboard.press('Escape');
  await expect(picker(page)).toBeHidden();
});

test('a row takes a reaction without being opened (D-023 points 3, 13), and that reads it (D-015)', async ({ page, door }, info) => {
  door.rows = busyFeed();
  await start(page);
  const target = page.locator(`[data-row="${KEY('#launch', 'msg_n1')}"]`);
  if (!isPhone(info)) await target.hover();
  await target.getByRole('button', { name: 'React' }).click();
  await picker(page).getByRole('button', { name: 'rocket ship launch' }).click();
  await expect.poll(() => door.sent('react').length).toBe(1);
  expect(door.sent('react')[0]).toMatchObject({ message: 'msg_n1', emoji: '🚀', on: true });
  await expect(target.locator('.rx', { hasText: '🚀' })).toBeVisible();
  await expect(target).toHaveClass(/(^|\s)read(\s|$)/);
  if (!isPhone(info)) await expect(page.getByRole('region', { name: 'Conversation' }).locator('.conv.none'), 'it did not open').toBeVisible();
});

test('reacting releases a kept row, unless the setting says only Done does (D-023 point 26)', async ({ page, door }, info) => {
  const kept = () => row({ conversation: '#launch', thread: 'msg_kr', message: 'msg_kr', bucket: 'focus', unread: false, kept: 'you', why: 'Kept by you', reactions: [{ emoji: '👍', people: ['lena'] }] });
  door.rows = [kept()];
  await start(page);
  const target = page.locator(`[data-row="${KEY('#launch', 'msg_kr')}"]`);
  await target.locator('.rx', { hasText: '👍' }).click();
  await expect(target.locator('.state')).toHaveText('Done');

  door.rows = [kept()];
  door.settings = { ...door.settings, release_on_act: false };
  await page.reload();
  await expect(page.locator('.app[data-live="on"]')).toBeVisible();
  await target.locator('.rx', { hasText: '👍' }).click();
  await expect.poll(() => door.sent('react').length).toBe(2);
  await expect(target.locator('.state')).toHaveCount(0);
  await expect(target.locator('.done-btn'), 'still kept').toBeVisible();
  void info;
});

test('a grouped row shows the true count per emoji (D-014, T-038 18)', async ({ page, door }) => {
  door.rows = [
    row({
      conversation: '#launch', thread: 'msg_g', message: 'msg_g', bucket: 'fyi', why: '✅ 4 · ❌ 1 on your message',
      reactions: [{ emoji: '✅', people: ['anna', 'lena', 'tomas', 'yuki'] }, { emoji: '❌', people: ['priya'] }],
    }),
  ];
  await start(page);
  const chips = page.locator(`[data-row="${KEY('#launch', 'msg_g')}"] .rx`);
  await expect(chips.nth(0)).toHaveAccessibleName('✅, 4 people');
  await expect(chips.nth(0)).toContainText('4');
  await expect(chips.nth(1)).toHaveAccessibleName('❌, 1 person');
  await expect(chips.nth(1), 'one person is named').toContainText('Priya');
});

plain('any message in a conversation takes a reaction from the full picker (D-023 point 1)', async ({ page, request }, info) => {
  const text = unique('React to me');
  const id = await post(request, 'anna', '#launch', text);
  await start(page);
  const pane = await openConversation(page, info, 'launch');
  const message = pane.locator(`[data-msg="${id}"]`);
  await message.scrollIntoViewIfNeeded();
  if (!isPhone(info)) await message.locator('.msg-self').hover();
  await message.getByRole('button', { name: 'React' }).click();
  await picker(page).getByRole('searchbox', { name: 'Search emoji' }).fill('coffee');
  await picker(page).getByRole('button', { name: 'coffee' }).click();
  const chip = message.locator('.rx', { hasText: '☕' });
  await expect(chip, 'the door saved it and the live update shows it').toBeVisible();
  await expect(chip).toHaveClass(/on/);
  await chip.click();
  await expect(chip, 'pressing your own reaction removes it').toHaveCount(0);
});
