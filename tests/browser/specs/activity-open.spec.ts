import { KEY, busyFeed, expect, row, test } from '../fixtures/door';
import { isPhone, post, start, unique } from './helpers';

const hit = (page: import('@playwright/test').Page, key: string) => page.locator(`[data-row="${key}"] .row-hit`);

test('a click opens the conversation beside the feed (wide) or over it (phone), and reads the row', async ({ page, door, request }, info) => {
  const id = await post(request, 'anna', '#launch', unique('Row to open'));
  door.rows = [row({ conversation: '#launch', thread: id, message: id, newest: 900000 })];
  await start(page);
  const key = KEY('#launch', id);
  await hit(page, key).click({ position: { x: 24, y: 24 } });
  const conversation = page.getByRole('region', { name: 'Conversation' });
  await expect(conversation).toBeVisible();
  const thread = page.getByRole('region', { name: 'Thread' });
  await expect(thread).toBeVisible();
  await expect.poll(() => door.sent('read').some((a) => a.conversation === '#launch' && a.thread === id && a.up_to === 900000), 'the row is read up to its newest').toBeTruthy();

  if (isPhone(info)) {
    const box = (await thread.boundingBox())!;
    expect(box.x).toBe(0);
    expect(box.width).toBe(400);
  } else {
    await expect(page.locator('.main'), 'the feed stays open beside it (or gives room to the thread)').toBeAttached();
  }
});

test('a row that was not a thread opens its conversation at the first new message, under a New line', async ({ page, door, request }) => {
  const ids: string[] = [];
  for (let i = 0; i < 30; i++) ids.push(await post(request, 'anna', '#general', unique(`Earlier ${i}`)));
  const messages = await (await request.get('http://127.0.0.1:8090/orgs/acme/conversations/%23general/messages')).json();
  const numberOf = (id: string) => messages.messages.find((m: { message: { id: string } }) => m.message.id === id).message.number as number;
  const firstNew = numberOf(ids[10]);
  door.rows = [row({ conversation: '#general', thread: null, message: ids[29], newest: numberOf(ids[29]), first_new: firstNew, new_items: 20, why: 'In Focus: #general is set to Focus', bucket: 'focus' })];
  await start(page);
  await hit(page, KEY('#general')).click({ position: { x: 24, y: 24 } });
  const pane = page.getByRole('region', { name: 'Conversation' });
  const line = pane.locator('.newline');
  await expect(line).toBeVisible();
  await expect(line).toBeInViewport();
  const after = pane.locator('.newline + *');
  await expect(after).toContainText('Earlier 10');
});

test('on a wide screen the cursor is in the message box; on a phone it is not (D-023)', async ({ page, door, request }, info) => {
  const id = await post(request, 'anna', '#launch', unique('Cursor row'));
  door.rows = [row({ conversation: '#launch', thread: id, message: id })];
  await start(page);
  await hit(page, KEY('#launch', id)).click({ position: { x: 24, y: 24 } });
  const box = page.getByRole('region', { name: 'Thread' }).getByRole('textbox', { name: 'Message' });
  await expect(box).toBeVisible();
  if (isPhone(info)) await expect(box).not.toBeFocused();
  else await expect(box).toBeFocused();
});

test('a read row stays in place, quiet, until the person moves on (D-003)', async ({ page, door, request }, info) => {
  test.skip(isPhone(info), 'on a phone the conversation covers the feed');
  const id = await post(request, 'anna', '#launch', unique('Stays quietly'));
  door.rows = [...busyFeed(), row({ conversation: '#launch', thread: id, message: id, newest: 1300 })];
  await start(page);
  const target = page.locator(`[data-row="${KEY('#launch', id)}"]`);
  await target.locator('.row-hit').click({ position: { x: 24, y: 24 } });
  await expect(target).toHaveClass(/read/);
  await expect(target).toBeVisible();
});

test('the buttons on a row do not open it', async ({ page, door }, info) => {
  door.rows = busyFeed();
  await start(page);
  const target = page.locator(`[data-row="${KEY('dm:anna:marco')}"]`);
  await target.hover();
  await target.getByRole('button', { name: 'React' }).click();
  await page.keyboard.press('Escape');
  if (!isPhone(info)) await expect(page.locator('.conv.none')).toBeVisible();
  expect(door.sent('read')).toEqual([]);
});
