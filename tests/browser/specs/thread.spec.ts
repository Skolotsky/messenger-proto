import { expect, test } from '@playwright/test';
import { isPhone, openConversation, post, start, unique, write } from './helpers';

test('no inline threads: replies are behind a link under the message', async ({ page, request }, info) => {
  const rootText = unique('A root');
  const replyText = unique('A reply');
  const root = await post(request, 'anna', '#general', rootText);
  await post(request, 'lena', '#general', replyText, root);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const message = pane.locator(`[data-msg="${root}"]`);
  await expect(message.locator('.thread-link')).toContainText('1 reply');
  await expect(pane, 'the reply is not shown in the channel').not.toContainText(replyText);
});

test('a thread opens beside its channel on a wide screen', async ({ page, request }, info) => {
  test.skip(isPhone(info), 'wide layout');
  const root = await post(request, 'anna', '#general', unique('Root for beside'));
  const replyText = unique('Reply for beside');
  await post(request, 'lena', '#general', replyText, root);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await pane.locator(`[data-msg="${root}"] .thread-link`).click();

  const thread = page.getByRole('region', { name: 'Thread' });
  await expect(thread).toContainText(replyText);
  await expect(pane.locator(`[data-msg="${root}"]`), 'the channel stays').toBeVisible();
  const a = (await pane.boundingBox())!;
  const b = (await thread.boundingBox())!;
  expect(b.x, 'the thread is to the right of the channel').toBeGreaterThanOrEqual(a.x + a.width - 1);
  expect(b.x + b.width).toBeLessThanOrEqual(1280);
  expect(a.width, 'the channel keeps a readable width').toBeGreaterThanOrEqual(299);

  await thread.getByRole('button', { name: 'Close the thread' }).click();
  await expect(thread).toBeHidden();
});

test('on a phone a thread opens in place of the channel, with a way back', async ({ page, request }, info) => {
  test.skip(!isPhone(info), 'phone layout');
  const root = await post(request, 'anna', '#general', unique('Root on phone'));
  const replyText = unique('Reply on phone');
  await post(request, 'lena', '#general', replyText, root);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await pane.locator(`[data-msg="${root}"] .thread-link`).click();

  const thread = page.getByRole('region', { name: 'Thread' });
  await expect(thread).toContainText(replyText);
  const box = (await thread.boundingBox())!;
  expect(box.x).toBe(0);
  expect(box.width).toBe(400);
  await thread.getByRole('button', { name: 'Back to the channel' }).click();
  await expect(thread).toBeHidden();
  await expect(pane.locator(`[data-msg="${root}"]`)).toBeVisible();
});

test('any channel message can start a thread, and a reply updates the link', async ({ page, request }, info) => {
  const rootText = unique('No replies yet');
  const root = await post(request, 'anna', '#general', rootText);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const message = pane.locator(`[data-msg="${root}"]`);
  await expect(message.locator('.thread-link')).toHaveCount(0);
  await message.locator('.msg-self').hover();
  await message.getByRole('button', { name: 'Reply in thread' }).click();

  const thread = page.getByRole('region', { name: 'Thread' });
  await expect(thread).toContainText(rootText);
  const reply = unique('My reply');
  await write(thread, reply);
  await expect(thread.locator('.msg-body', { hasText: reply })).toBeVisible();
  await expect(pane, 'the reply stays out of the channel').not.toContainText(reply);
  if (isPhone(info)) await thread.getByRole('button', { name: 'Back to the channel' }).click();
  await expect(message.locator('.thread-link')).toContainText('1 reply');
});

test('a reply from someone else appears in the open thread', async ({ page, request }, info) => {
  const root = await post(request, 'anna', '#general', unique('Root for live'));
  await post(request, 'lena', '#general', unique('First'), root);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await pane.locator(`[data-msg="${root}"] .thread-link`).click();
  const thread = page.getByRole('region', { name: 'Thread' });
  await expect(thread.locator('[data-msg]')).toHaveCount(2);
  const text = unique('Arrives live');
  await post(request, 'priya', '#general', text, root);
  await expect(thread.locator('.msg-body', { hasText: text })).toBeVisible();
});
