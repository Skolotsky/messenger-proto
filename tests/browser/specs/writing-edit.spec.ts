import { expect, test, type Locator, type Page } from '@playwright/test';
import { doorAct, isPhone, openConversation, post, settled, start, topOf, unique, write } from './helpers';
import { failNext, recordActions } from './writing-fixtures';

async function mine(page: Page, pane: Locator, text: string) {
  await write(pane, text);
  const found = pane.locator('.msg.mine', { hasText: text }).last();
  await expect(found).toBeVisible();
  return pane.locator(`[data-msg="${await found.getAttribute('data-msg')}"]`);
}

const tool = async (msg: Locator, name: string) => {
  await msg.locator('.msg-self').first().hover();
  await msg.getByRole('button', { name }).click();
};

test('an edit happens in place and says "edited"; mentions follow the new text', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const text = unique('Ask @Anna about it');
  const msg = await mine(page, pane, text);
  expect(actions.of('send').at(-1)!.message.mentions).toEqual(['anna']);

  await tool(msg, 'Edit');
  const box = msg.getByRole('textbox', { name: 'Edit your message' });
  await expect(box, 'the cursor is in the text').toBeFocused();
  const changed = text.replace('@Anna', '@Lena');
  await box.fill(changed);
  await box.press('Control+Enter');
  const edited = pane.locator('.msg.mine', { hasText: changed });
  await expect(edited.locator('.edited')).toHaveText('edited');
  await expect(edited.locator('.at')).toHaveText(['@Lena']);
  const sent = actions.of('edit').at(-1)!;
  expect(sent.mentions, 'the removed name is gone, the new one is there').toEqual(['lena']);
  expect(sent.text).toBe(changed);
});

test('@ suggestions work in the edit box, and Esc closes them before it cancels the edit', async ({ page }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const msg = await mine(page, pane, unique('Before'));
  await tool(msg, 'Edit');
  const box = msg.getByRole('textbox', { name: 'Edit your message' });
  await box.press('End');
  await box.pressSequentially(' @');
  const list = page.getByRole('listbox', { name: 'Mention someone' });
  await expect(list).toBeVisible();
  await box.press('Escape');
  await expect(list).toHaveCount(0);
  await expect(box, 'the first Esc only closed the list').toBeVisible();
  await box.press('Escape');
  await expect(box, 'the second Esc cancels the edit').toHaveCount(0);
});

test('T-038.25: an empty edit is refused, says so, sends nothing and changes nothing', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const text = unique('Keep me');
  const msg = await mine(page, pane, text);
  await tool(msg, 'Edit');
  const box = msg.getByRole('textbox', { name: 'Edit your message' });
  await box.fill('   ');
  await box.press('Control+Enter');
  await expect(msg.getByRole('alert')).toContainText('cannot be empty');
  await expect(msg.getByRole('button', { name: 'Try again' }), 'trying again cannot help').toHaveCount(0);
  expect(actions.of('edit')).toEqual([]);
  await msg.getByRole('button', { name: 'Cancel' }).click();
  await expect(msg.locator('.msg-body')).toHaveText(text);
});

test('D-024: removing a flag is an edit, keeping the text and the mentions', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const text = unique('@Anna please look');
  const box = pane.getByRole('textbox', { name: 'Message' });
  await box.fill(text);
  await pane.getByRole('button', { name: 'Flag', exact: true }).click();
  await box.press('Control+Enter');
  await expect(box).toHaveValue('');
  const msg = pane.locator('.msg.mine', { hasText: text });
  await expect(msg.locator('.tag.flag')).toBeVisible();
  await tool(msg, 'Remove the flag');
  await expect(msg.locator('.tag.flag')).toHaveCount(0);
  await expect(msg.locator('.edited')).toBeVisible();
  const sent = actions.of('edit').at(-1)!;
  expect([sent.text, sent.mentions, sent.level]).toEqual([text, ['anna'], null]);
  await expect(msg.getByRole('button', { name: 'Remove the flag' }), 'no flag, no such tool').toHaveCount(0);
});

test('a refused edit says so in place and offers Discard, not Try again', async ({ page }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const text = unique('Refused edit');
  const msg = await mine(page, pane, text);
  await failNext(page, 'edit', { refused: 'message deleted' });
  await tool(msg, 'Edit');
  const box = msg.getByRole('textbox', { name: 'Edit your message' });
  await box.fill(`${text} changed`);
  await box.press('Control+Enter');
  await expect(msg.getByRole('alert')).toContainText('Not saved: message deleted');
  await expect(msg.getByRole('button', { name: 'Try again' })).toHaveCount(0);
  await msg.getByRole('button', { name: 'Discard' }).click();
  await expect(box).toHaveCount(0);
  await expect(msg.locator('.msg-body')).toHaveText(text);
});

test('what went wrong is brought into view, also when the message ends at the edge of the list', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const text = unique('At the edge');
  const msg = await mine(page, pane, text);
  await post(request, 'anna', '#general', unique('Written below it'));
  await expect(msg.locator('xpath=following-sibling::*[@data-msg]').first()).toBeAttached();
  await failNext(page, 'edit', { refused: 'message deleted' });
  await tool(msg, 'Edit');
  const box = msg.getByRole('textbox', { name: 'Edit your message' });
  await box.fill(`${text} changed`);
  // The list is scrolled so that the message's last pixel is the last one in view.
  await msg.evaluate((node) => {
    const list = node.closest('.conv-body') as HTMLElement;
    list.scrollTop += node.getBoundingClientRect().bottom - list.getBoundingClientRect().bottom;
  });
  await settled(page);
  await box.press('Control+Enter');
  await expect(msg.getByRole('alert')).toContainText('Not saved');
  await expect(msg.getByRole('alert')).toBeInViewport({ ratio: 1 });
  await settled(page);
  await expect(msg.getByRole('alert'), 'and it stays there').toBeInViewport({ ratio: 1 });
});

test('an edit with no answer offers Try again, with the same action ID', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const text = unique('Retry edit');
  const msg = await mine(page, pane, text);
  await failNext(page, 'edit', 'down');
  await tool(msg, 'Edit');
  const box = msg.getByRole('textbox', { name: 'Edit your message' });
  await box.fill(`${text} changed`);
  await box.press('Control+Enter');
  await expect(msg.getByRole('alert')).toContainText('no answer');
  await msg.getByRole('button', { name: 'Try again' }).click();
  await expect(pane.locator('.msg.mine', { hasText: `${text} changed` }).locator('.edited')).toBeVisible();
  const edits = actions.of('edit');
  expect(edits).toHaveLength(2);
  expect(edits[1].id, 'saved once, whichever answer was lost').toBe(edits[0].id);
});

test('D-004: a longer edit arriving on screen moves nothing and waits behind "Show all"', async ({ page, request }, info) => {
  const first = unique('Short');
  const id = await post(request, 'marco', '#general', first);
  await post(request, 'anna', '#general', unique('Next one'));
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const msg = pane.locator(`[data-msg="${id}"]`);
  await expect(msg).toBeVisible();
  const next = msg.locator('xpath=following-sibling::*[@data-msg][1]');
  await settled(page);
  const [before, height] = [await topOf(next), await msg.evaluate((n) => n.getBoundingClientRect().height)];

  const long = `${first} ${Array.from({ length: 40 }, (_, i) => `word${i}`).join(' ')}`;
  await doorAct(request, { as: 'marco', kind: 'edit', message: id, text: long, mentions: [], level: null });
  await expect(msg.locator('.edited')).toBeVisible();
  await settled(page);
  expect(Math.abs((await topOf(next)) - before), 'the next message did not move').toBeLessThan(0.5);
  expect(Math.abs((await msg.evaluate((n) => n.getBoundingClientRect().height)) - height)).toBeLessThan(0.5);

  await msg.getByRole('button', { name: 'Show all' }).click();
  await expect(msg.getByRole('button', { name: 'Show all' })).toHaveCount(0);
  expect(await msg.evaluate((n) => n.getBoundingClientRect().height), 'the person asked to see it').toBeGreaterThan(height + 20);
});

test('D-004: a shorter edit arriving on screen keeps its space', async ({ page, request }, info) => {
  const first = `${unique('Long')} ${'many words here '.repeat(30)}`;
  const id = await post(request, 'marco', '#general', first);
  await post(request, 'anna', '#general', unique('Next two'));
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const msg = pane.locator(`[data-msg="${id}"]`);
  await expect(msg).toBeVisible();
  const next = msg.locator('xpath=following-sibling::*[@data-msg][1]');
  await settled(page);
  const before = await topOf(next);
  await doorAct(request, { as: 'marco', kind: 'edit', message: id, text: 'Short now', mentions: [], level: null });
  await expect(msg.locator('.msg-body')).toContainText('Short now');
  await settled(page);
  expect(Math.abs((await topOf(next)) - before)).toBeLessThan(0.5);
  await expect(msg.getByRole('button', { name: 'Show all' }), 'nothing is hidden').toHaveCount(0);
});

test('own message tools fit beside the name on a phone', async ({ page }, info) => {
  test.skip(!isPhone(info), 'phone only');
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const box = pane.getByRole('textbox', { name: 'Message' });
  const text = unique('Flagged on a phone');
  await box.fill(text);
  await pane.getByRole('button', { name: 'Flag', exact: true }).click();
  await box.press('Control+Enter');
  const msg = pane.locator('.msg.mine', { hasText: text });
  const tools = await msg.locator('.msg-tools').boundingBox();
  const time = await msg.locator('.msg-head time').boundingBox();
  expect(time!.x + time!.width, 'the time is not under the tools').toBeLessThanOrEqual(tools!.x);
});
