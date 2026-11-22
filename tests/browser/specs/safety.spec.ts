import { expect, test } from '@playwright/test';
import { doorAct, isPhone, openConversation, post, start, unique, write } from './helpers';

const ATTACK = '<img src=x onerror="window.__hit=1"><script>window.__hit=1</script><b>bold</b>';
const hit = (page: import('@playwright/test').Page) => page.evaluate(() => (window as unknown as { __hit?: number }).__hit);

test('T-038.24: a message is shown as text, never as markup', async ({ page, request }, info) => {
  const text = `${unique('Attack')} ${ATTACK}`;
  await post(request, 'anna', '#general', text);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const body = pane.locator('.msg-body', { hasText: 'onerror' }).last();
  await expect(body).toHaveText(text);
  await expect(body.locator('img, script, b')).toHaveCount(0);
  expect(await hit(page)).toBeUndefined();
});

test('T-038.24: a channel name is shown as text', async ({ page, request }, info) => {
  const name = `x<img src=y onerror="window.__hit=1">${Date.now().toString(36)}`;
  await doorAct(request, { as: 'marco', kind: 'create_channel', name, people: ['anna'] });
  await start(page);
  if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
  const row = page.getByRole('navigation', { name: 'Conversations' }).locator('.chat-n', { hasText: name });
  await expect(row).toHaveText(name);
  await row.click();
  await expect(page.getByRole('region', { name: 'Conversation' }).locator('.conv-title b')).toHaveText(`#${name}`);
  await expect(page.locator('img')).toHaveCount(0);
  expect(await hit(page)).toBeUndefined();
});

test('T-038.24: names, the organization and a profile line are shown as text', async ({ page }, info) => {
  const evil = `Eve<img src=z onerror="window.__hit=1">`;
  await page.route('**/orgs/acme/people/marco/start', async (route) => {
    const res = await route.fetch();
    const start = await res.json();
    start.org_name = `Org${evil}`;
    start.person.line = `Line${evil}`;
    for (const p of start.people) if (p.id === 'anna') p.name = evil;
    await route.fulfill({ response: res, json: start });
  });
  await start(page);
  if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
  await expect(page.locator('.ws-t b')).toHaveText(`Org${evil}`);
  await expect(page.locator('.me-btn small')).toHaveText(`Line${evil}`);
  const nav = page.getByRole('navigation', { name: 'Conversations' });
  await expect(nav.locator('.chat-n', { hasText: evil })).toHaveText(evil);
  await page.getByRole('button', { name: 'Your account menu' }).click();
  await page.locator('.me-pop').getByRole('button', { name: /Switch account/ }).click();
  await expect(page.locator('.me-pop .opt.who', { hasText: evil }).locator('b')).toHaveText(evil);
  await expect(page.locator('img')).toHaveCount(0);
  expect(await hit(page)).toBeUndefined();
});

test('T-038.25: saving an empty edit says what happens and changes nothing', async ({ page }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const text = unique('Keep this text');
  await write(pane, text);
  // Held by ID: while it is edited, the message's text is in a box and no longer matches by text.
  const message = pane.locator(`[data-msg="${await pane.locator('.msg', { hasText: text }).getAttribute('data-msg')}"]`);
  await message.locator('.msg-self').hover();
  await message.getByRole('button', { name: 'Edit' }).click();

  const box = message.getByRole('textbox', { name: 'Edit your message' });
  await expect(box).toHaveValue(text);
  await box.fill('   \n ');
  await message.getByRole('button', { name: 'Save' }).click();

  const said = message.getByRole('alert');
  await expect(said).toContainText('cannot be empty');
  await expect(said).toContainText('Nothing was changed');
  await expect(box, 'the box stays open').toBeVisible();

  await box.fill('x');
  await expect(said, 'the notice goes once there is text').toHaveCount(0);
  await message.getByRole('button', { name: 'Cancel' }).click();
  await expect(message.locator('.msg-body')).toHaveText(text);
  await expect(message.locator('.edited')).toHaveCount(0);

  await page.reload();
  const again = await openConversation(page, info, 'general');
  await expect(again.locator('.msg', { hasText: text }).locator('.edited'), 'the door was not told').toHaveCount(0);
});

test('an edit with text is saved and says edited', async ({ page }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const text = unique('Before the edit');
  await write(pane, text);
  const id = await pane.locator('.msg', { hasText: text }).getAttribute('data-msg');
  const message = pane.locator(`[data-msg="${id}"]`);
  await message.locator('.msg-self').hover();
  await message.getByRole('button', { name: 'Edit' }).click();
  const after = unique('After the edit');
  await message.getByRole('textbox', { name: 'Edit your message' }).fill(after);
  await message.getByRole('button', { name: 'Save' }).click();
  const same = pane.locator(`[data-msg="${id}"]`);
  await expect(same.locator('.msg-body')).toHaveText(after);
  await expect(same.locator('.edited')).toBeVisible();
});
