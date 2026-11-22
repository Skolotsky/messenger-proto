import { expect, test, type Locator } from '@playwright/test';
import { doorAct, isPhone, openConversation, post, settled, start, topOf, unique, write } from './helpers';
import { failNext, recordActions } from './writing-fixtures';

const askDelete = async (msg: Locator) => {
  await msg.locator('.msg-self').first().hover();
  await msg.getByRole('button', { name: 'Delete' }).click();
  return msg.getByRole('group', { name: 'Delete this message?' });
};

/** Held by its ID: while the question is shown, the message's text is not on screen. */
const byId = async (pane: Locator, text: string) => {
  const found = pane.locator('.msg.mine', { hasText: text });
  await expect(found).toBeVisible();
  return pane.locator(`[data-msg="${await found.getAttribute('data-msg')}"]`);
};

test('D-024: delete asks first, in the message\'s own space, and Cancel changes nothing', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const text = unique('Ask before deleting');
  await write(pane, text);
  const msg = await byId(pane, text);
  const ask = await askDelete(msg);
  await expect(ask).toContainText('cannot be undone');
  await expect(page.getByRole('dialog'), 'nothing covers the message').toHaveCount(0);
  expect(actions.of('delete'), 'asking sends nothing').toEqual([]);
  await ask.getByRole('button', { name: 'Cancel' }).click();
  await expect(msg.locator('.msg-body')).toHaveText(text);
  expect(actions.of('delete')).toEqual([]);
});

test('D-004: a deleted message without replies leaves nothing, also after a reload', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const text = unique('Delete me');
  await write(pane, text);
  const msg = await byId(pane, text);
  const id = await msg.getAttribute('data-msg');
  const ask = await askDelete(msg);
  await ask.getByRole('button', { name: 'Delete' }).click();
  await expect(pane.locator(`[data-msg="${id}"]`)).toHaveCount(0);
  expect(actions.of('delete').map((a) => a.message)).toEqual([id]);
  await start(page);
  const again = await openConversation(page, info, 'general');
  await expect(again.locator(`[data-msg="${id}"]`)).toHaveCount(0);
  await expect(again.getByText(text)).toHaveCount(0);
});

test('D-004: a deleted message with replies keeps its marker and its link to the thread', async ({ page, request }, info) => {
  const text = unique('Root with replies');
  const root = await post(request, 'marco', '#general', text);
  const reply = unique('A reply that stays');
  await post(request, 'anna', '#general', reply, root);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const msg = pane.locator(`[data-msg="${root}"]`);
  const ask = await askDelete(msg);
  await expect(ask, 'it says what happens to the replies').toContainText('its replies stay');
  await ask.getByRole('button', { name: 'Delete' }).click();
  await expect(msg.locator('.tomb')).toHaveText('This message was deleted.');
  await expect(msg).not.toContainText(text);
  await msg.getByRole('button', { name: /1 reply/ }).click();
  const thread = page.getByRole('region', { name: 'Thread' });
  await expect(thread.locator('.msg-body', { hasText: reply })).toBeVisible();
  await expect(thread.locator(`[data-msg="${root}"] .tomb`)).toBeVisible();
});

test('D-004: someone else\'s delete seen arriving holds its space until the person moves on', async ({ page, request }, info) => {
  const text = unique('Deleted while you watch');
  const id = await post(request, 'anna', '#general', text);
  await post(request, 'lena', '#general', unique('Below it'));
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const msg = pane.locator(`[data-msg="${id}"]`);
  await expect(msg).toBeVisible();
  const next = msg.locator('xpath=following-sibling::*[@data-msg][1]');
  await settled(page);
  const [before, height] = [await topOf(next), await msg.evaluate((n) => n.getBoundingClientRect().height)];

  await doorAct(request, { as: 'anna', kind: 'delete', message: id });
  await expect(msg.locator('.tomb')).toBeVisible();
  await expect(msg).not.toContainText(text);
  await settled(page);
  expect(Math.abs((await topOf(next)) - before), 'the next message did not move').toBeLessThan(0.5);
  expect(Math.abs((await msg.evaluate((n) => n.getBoundingClientRect().height)) - height), 'the marker has the height of the message').toBeLessThan(0.5);

  if (isPhone(info)) await pane.getByRole('button', { name: 'Back' }).click();
  await page.getByRole('navigation', { name: 'Conversations' }).getByRole('button', { name: 'launch' }).click();
  await expect(page.getByRole('region', { name: 'Conversation' }).locator('.conv-title b')).toHaveText('#launch');
  if (isPhone(info)) await page.getByRole('region', { name: 'Conversation' }).getByRole('button', { name: 'Back' }).click();
  const back = await openConversation(page, info, 'general');
  await expect(back.locator(`[data-msg="${id}"]`), 'moved on: nothing stays').toHaveCount(0);
});

test('a refused delete says so in place and offers Discard, not Try again', async ({ page }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const text = unique('Refused delete');
  await write(pane, text);
  const msg = await byId(pane, text);
  await failNext(page, 'delete', { refused: 'not your message' });
  const ask = await askDelete(msg);
  await ask.getByRole('button', { name: 'Delete' }).click();
  await expect(msg.getByRole('alert')).toContainText('Not deleted: not your message');
  await expect(msg.getByRole('button', { name: 'Try again' })).toHaveCount(0);
  await msg.getByRole('button', { name: 'Discard' }).click();
  await expect(msg.getByRole('alert')).toHaveCount(0);
  await expect(msg.locator('.msg-body')).toHaveText(text);
});

test('a delete with no answer offers Try again, with the same action ID', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const text = unique('Retry delete');
  await write(pane, text);
  const msg = await byId(pane, text);
  const id = await msg.getAttribute('data-msg');
  await failNext(page, 'delete', 'down');
  const ask = await askDelete(msg);
  await ask.getByRole('button', { name: 'Delete' }).click();
  await expect(msg.getByRole('alert')).toContainText('no answer');
  await msg.getByRole('button', { name: 'Try again' }).click();
  await expect(pane.locator(`[data-msg="${id}"]`)).toHaveCount(0);
  const deletes = actions.of('delete');
  expect(deletes).toHaveLength(2);
  expect(deletes[1].id).toBe(deletes[0].id);
});

test('a send refused says so and offers Discard; with no answer it also offers Try again', async ({ page }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const box = pane.getByRole('textbox', { name: 'Message' });
  await failNext(page, 'send', { refused: 'unknown conversation' });
  await box.fill(unique('Refused send'));
  await box.press('Control+Enter');
  const alert = pane.getByRole('alert');
  await expect(alert).toContainText('Not sent: unknown conversation');
  await expect(pane.getByRole('button', { name: 'Try again' })).toHaveCount(0);
  await pane.getByRole('button', { name: 'Discard' }).click();
  await expect(box).toHaveValue('');
  await expect(alert).toHaveCount(0);

  await page.unrouteAll();
  await failNext(page, 'send', 'down');
  const text = unique('Second try');
  await box.fill(text);
  await box.press('Control+Enter');
  await expect(pane.getByRole('alert')).toContainText('no answer');
  await pane.getByRole('button', { name: 'Try again' }).click();
  await expect(pane.locator('.msg-body', { hasText: text })).toBeVisible();
  await expect(box).toHaveValue('');
});
