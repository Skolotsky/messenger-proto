import type { Page, TestInfo } from '@playwright/test';
import { busyFeed, expect, test } from '../fixtures/door';
import { isPhone, start } from './helpers';

async function openSettings(page: Page, info: TestInfo, section?: string) {
  if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Settings' }).click();
  else await page.locator('.top').getByRole('button', { name: 'Settings' }).click();
  const dialog = page.getByRole('dialog', { name: 'Settings' });
  await expect(dialog).toBeVisible();
  if (section) await dialog.getByRole('navigation').getByRole('button', { name: section, exact: true }).click();
  return dialog;
}

test('one screen holds the general rules, people, channels, notifications and writing (D-023)', async ({ page, door }, info) => {
  door.rows = busyFeed();
  await start(page);
  const dialog = await openSettings(page, info);
  await expect(dialog.locator('.set-body > h2'), 'it opens on the section the person came from').toHaveText('Activity feed');
  for (const name of ['Account', 'Appearance', 'Writing messages', 'Activity feed', 'Focus people', 'Channel levels', 'Notifications']) {
    await expect(dialog.getByRole('navigation').getByRole('button', { name, exact: true })).toBeVisible();
  }
  await expect(dialog).toContainText('Flags that name you');
  await expect(dialog).toContainText('Any direct mention');
  await expect(dialog).toContainText('Replying or reacting marks a kept row done');
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
});

test('text rules appear nowhere (D-034)', async ({ page, door }, info) => {
  door.rows = busyFeed();
  await start(page);
  const dialog = await openSettings(page, info);
  for (const name of ['Activity feed', 'Focus people', 'Channel levels', 'Notifications']) {
    await dialog.getByRole('navigation').getByRole('button', { name, exact: true }).click();
    await expect(dialog).not.toContainText(/text rule|language model|anything about/i);
  }
});

test('simulated figures are not shown as if they were the person’s own (D-034)', async ({ page, door }, info) => {
  await start(page);
  const dialog = await openSettings(page, info, 'Notifications');
  await expect(dialog).toContainText('What should send you a notification?');
  await expect(dialog).not.toContainText(/about \d+ a day/);
  expect(door.errors).toEqual([]);
});

test('a person removed from Focus can be added back in the same place (D-021, D-023 point 15)', async ({ page, door }, info) => {
  await start(page);
  const dialog = await openSettings(page, info, 'Focus people');
  await expect(dialog).toContainText('Direct messages from · 2');
  await dialog.getByRole('button', { name: 'Remove Anna' }).click();
  await expect(dialog).toContainText('Direct messages from · 1');
  await expect.poll(() => door.sent('focus_person').length).toBe(1);
  expect(door.sent('focus_person')[0]).toMatchObject({ about: 'anna', on: false });
  await dialog.getByRole('button', { name: 'Add Anna back' }).click();
  await expect(dialog).toContainText('Direct messages from · 2');
  await expect.poll(() => door.sent('focus_person').length).toBe(2);
  expect(door.sent('focus_person')[1]).toMatchObject({ about: 'anna', on: true });
  await expect(dialog.getByRole('button', { name: 'Add Tomas back' }), 'someone removed earlier is listed too').toBeVisible();
});

test('switching off "add everyone" asks about those already added; keeping them is the default (D-021)', async ({ page, door }, info) => {
  await start(page);
  const dialog = await openSettings(page, info);
  await dialog.getByRole('checkbox', { name: /Add everyone who writes to me directly/ }).uncheck();
  const ask = dialog.getByRole('group', { name: 'Remove people already added?' });
  await expect(ask).toContainText('2 people are already in Focus and stay there');
  await expect(ask.getByRole('button', { name: 'Keep them' }), 'keeping them is the default').toBeFocused();
  await expect.poll(() => door.sent('focus_auto_add').length).toBe(1);
  expect(door.sent('focus_person'), 'nobody was removed by switching it off').toEqual([]);
  await ask.getByRole('button', { name: 'Keep them' }).click();
  await expect(ask).toBeHidden();
  expect(door.sent('focus_person')).toEqual([]);
});

test('"Remove all" after asking removes each person', async ({ page, door }, info) => {
  await start(page);
  const dialog = await openSettings(page, info);
  await dialog.getByRole('checkbox', { name: /Add everyone who writes to me directly/ }).uncheck();
  await dialog.getByRole('button', { name: 'Remove all 2' }).click();
  await expect.poll(() => door.sent('focus_person').length).toBe(2);
  expect(door.sent('focus_person').every((a) => a.on === false)).toBeTruthy();
});

test('a general rule is a switch, and what is off can be turned on again', async ({ page, door }, info) => {
  await start(page);
  const dialog = await openSettings(page, info);
  const flags = dialog.getByRole('checkbox', { name: /Flags that name you/ });
  await expect(flags).toBeChecked();
  await flags.uncheck();
  await expect.poll(() => door.sent('focus_rule').length).toBe(1);
  expect(door.sent('focus_rule')[0]).toMatchObject({ rule: { type: 'item_kind', value: 'flags' }, on: false });
  await flags.check();
  await expect.poll(() => door.sent('focus_rule').length).toBe(2);
  expect(door.sent('focus_rule')[1]).toMatchObject({ on: true });
});

test('every channel is listed with its level, changed in place (D-023 point 17)', async ({ page, door }, info) => {
  await start(page);
  const dialog = await openSettings(page, info, 'Channel levels');
  const incidents = dialog.getByRole('combobox', { name: 'Level for #incidents' });
  await expect(incidents).toHaveValue('focus');
  const launch = dialog.getByRole('combobox', { name: 'Level for #launch' });
  await expect(launch).toHaveValue('default');
  await launch.selectOption('muted');
  await expect.poll(() => door.sent('dial').length).toBe(1);
  expect(door.sent('dial')[0]).toMatchObject({ conversation: '#launch', thread: null, level: 'muted' });
  await expect(dialog).toContainText('Changed by you · 2');
});

test('notifications ask one question with two choices (D-012)', async ({ page, door }, info) => {
  await start(page);
  const dialog = await openSettings(page, info, 'Notifications');
  await expect(dialog.getByRole('radio', { name: /Focus and Needs you/ })).toHaveAttribute('aria-checked', 'true');
  await dialog.getByRole('radio', { name: /Focus only/ }).click();
  await expect(dialog.getByRole('radio', { name: /Focus only/ })).toHaveAttribute('aria-checked', 'true');
  await expect.poll(() => door.sent('focus_only').length).toBe(1);
  expect(door.sent('focus_only')[0]).toMatchObject({ on: true });
});

test('the release setting sends its action (D-023 point 26)', async ({ page, door }, info) => {
  await start(page);
  const dialog = await openSettings(page, info);
  await dialog.getByRole('checkbox', { name: /Replying or reacting/ }).uncheck();
  await expect.poll(() => door.sent('release_setting').length).toBe(1);
  expect(door.sent('release_setting')[0]).toMatchObject({ on: false });
});

test('which key sends is a setting, and the message box follows it', async ({ page, door }, info) => {
  await start(page);
  const dialog = await openSettings(page, info, 'Writing messages');
  await dialog.getByRole('radio', { name: /^Enter sends/ }).click();
  await expect(dialog.getByRole('radio', { name: /^Enter sends/ })).toHaveAttribute('aria-checked', 'true');
  await expect.poll(() => door.sent('preference').length).toBe(1);
  expect(door.sent('preference')[0]).toMatchObject({ key: 'send_key', value: 'enter' });
});

test('a setting that is refused goes back and says so', async ({ page, door }, info) => {
  await start(page);
  const dialog = await openSettings(page, info);
  const flags = dialog.getByRole('checkbox', { name: /Flags that name you/ });
  door.refuseNext = true;
  await flags.click(); // not uncheck(): a refused change puts the switch back at once, which is the point
  await expect(dialog.getByRole('alert')).toContainText('did not go through');
  await expect(flags, 'the switch shows the truth again').toBeChecked();
});
