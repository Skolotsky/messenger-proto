import { expect, test } from '@playwright/test';
import { openConversation, unique } from './helpers';
import { start } from './helpers';
import { recordActions } from './writing-fixtures';

const list = (page: import('@playwright/test').Page) => page.getByRole('listbox', { name: 'Mention someone' });

test('D-024: typing @ suggests people and "channel", and names nobody with it', async ({ page }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const box = pane.getByRole('textbox', { name: 'Message' });
  await box.pressSequentially('@');
  await expect(list(page).getByRole('option', { name: /Anna/ })).toBeVisible();
  await expect(list(page).getByRole('option', { name: /channel/ })).toContainText('Not a mention of anyone in particular');
  await expect(list(page).getByRole('option', { name: /Marco/ }), 'one does not mention oneself').toHaveCount(0);
});

test('D-024: arrows move, Enter and Tab pick, Esc closes and keeps the text', async ({ page }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const box = pane.getByRole('textbox', { name: 'Message' });
  await box.pressSequentially('@');
  const first = await list(page).getByRole('option').nth(0).locator('b').innerText();
  const second = await list(page).getByRole('option').nth(1).locator('b').innerText();
  await box.press('ArrowDown');
  await expect(list(page).getByRole('option').nth(1)).toHaveAttribute('aria-selected', 'true');
  await box.press('Enter');
  await expect(box, 'Enter picks and does not start a line').toHaveValue(`@${second.split(' ')[0]} `);
  await box.pressSequentially('@');
  await expect(list(page).getByRole('option').first().locator('b')).toHaveText(first);
  await box.press('Tab');
  await expect(box).toHaveValue(`@${second.split(' ')[0]} @${first.split(' ')[0]} `);
  await expect(box, 'Tab keeps the caret in the box').toBeFocused();
  await box.pressSequentially('@');
  await expect(list(page)).toBeVisible();
  await box.press('Escape');
  await expect(list(page)).toHaveCount(0);
  await expect(box).toHaveValue(`@${second.split(' ')[0]} @${first.split(' ')[0]} @`);
});

test('D-024: a mention in the middle of the text replaces only what was typed', async ({ page }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const box = pane.getByRole('textbox', { name: 'Message' });
  await box.fill('hello  there');
  await box.evaluate((el: HTMLTextAreaElement) => el.setSelectionRange(6, 6));
  await box.pressSequentially('@len');
  await expect(list(page).getByRole('option', { name: /Lena/ })).toBeVisible();
  await box.press('Enter');
  await expect(box).toHaveValue('hello @Lena  there');
  expect(await box.evaluate((el: HTMLTextAreaElement) => el.selectionStart), 'the caret stands after the name').toBe(12);
});

test('the mentions sent are the ones in the text, and are marked', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const box = pane.getByRole('textbox', { name: 'Message' });
  const tail = unique('tail');
  await box.fill(`@Anna and @lena, @channel: mail me a@yuki.co or @Marco or @Nobody ${tail}`);
  await box.press('Control+Enter');
  await expect(box).toHaveValue('');
  const sent = actions.of('send').at(-1)!;
  expect(sent.message.mentions, 'people and channel; not an address, oneself or a stranger').toEqual(['anna', 'lena', 'channel']);
  const body = pane.locator('.msg-body', { hasText: tail });
  await expect(body.locator('.at')).toHaveText(['@Anna', '@lena', '@channel']);
});

test('a direct message has no "channel" to mention', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'Anna');
  const box = pane.getByRole('textbox', { name: 'Message' });
  await box.pressSequentially('@');
  await expect(list(page).getByRole('option').first()).toBeVisible();
  await expect(list(page).getByRole('option', { name: /channel/ })).toHaveCount(0);
  await box.press('Escape');
  await box.fill(`@channel ${unique('direct')}`);
  await box.press('Control+Enter');
  await expect(box).toHaveValue('');
  expect(actions.of('send').at(-1)!.message.mentions).toEqual([]);
});
