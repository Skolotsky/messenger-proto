import { expect, test, type Page } from '@playwright/test';
import { control, doorAct, openConversation, post, start, unique, write } from './helpers';
import { live, offline, resetDoor } from './live-helpers';

/** D-024, T-038 point 26: what is done without a connection stays in place, and goes out in order. */

test.afterEach(async ({ request }) => resetDoor(request));

function recordActions(page: Page) {
  const sent: { id: string; kind: string; message?: unknown }[] = [];
  page.on('request', (r) => {
    if (r.method() === 'POST' && r.url().endsWith('/people/actions')) sent.push(r.postDataJSON());
  });
  return sent;
}

const hover = async (msg: ReturnType<Page['locator']>) => msg.locator('.msg-self').first().hover();

test('a message, an edit and a reaction wait in place and go out in order, with their IDs', async ({ page, request }, info) => {
  const sent = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const own = unique('Mine to edit');
  await write(pane, own);
  const msg = pane.locator('.msg.mine', { hasText: own });
  await expect(msg).toBeVisible();
  const other = await post(request, 'anna', '#general', unique('Someone else'));
  const theirs = pane.locator(`[data-msg="${other}"]`);
  await expect(theirs).toBeVisible();

  await control(request, { down: true });
  await expect(offline(page)).toBeVisible();
  sent.length = 0;

  await hover(msg);
  await msg.getByRole('button', { name: 'Edit' }).click();
  const edit = msg.getByRole('textbox', { name: 'Edit your message' });
  await edit.fill(`${own} changed`);
  await edit.press('Control+Enter');
  await expect(msg.getByRole('status')).toContainText('Edit waiting');
  await expect(msg.locator('.msg-body')).toHaveText(`${own} changed`);

  await hover(theirs);
  await theirs.getByRole('button', { name: 'React', exact: true }).click();
  await page.getByRole('dialog').getByRole('button').filter({ hasText: '👍' }).first().click();
  await expect(theirs.getByRole('status')).toContainText('Reaction waiting');
  await expect(theirs.locator('.rx')).toContainText('👍');

  const text = unique('Written offline');
  const box = pane.getByRole('textbox', { name: 'Message' });
  await box.fill(text);
  await box.press('Control+Enter');
  await expect(box, 'the box is free again').toHaveValue('');
  const waiting = pane.locator('.msg', { hasText: text });
  await expect(waiting.getByRole('status')).toContainText('Waiting to send');
  expect(sent.filter((a) => a.kind !== 'read'), 'nothing is sent while offline').toEqual([]);

  await control(request, { down: false });
  await expect(live(page)).toBeVisible();
  await expect(waiting.getByRole('status')).toHaveCount(0);
  await expect(msg.getByRole('status')).toHaveCount(0);
  await expect(theirs.getByRole('status')).toHaveCount(0);

  const kinds = sent.filter((a) => a.kind !== 'read').map((a) => a.kind);
  expect(kinds, 'in the order made').toEqual(['edit', 'react', 'send']);
  expect(new Set(sent.map((a) => a.id)).size, 'each once').toBe(sent.length);
  await expect(pane.locator('.msg-body', { hasText: text })).toHaveCount(1);
});

test('what waited and is refused says so in place, with Discard and no Try again', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await control(request, { down: true });
  await expect(offline(page)).toBeVisible();
  const text = unique('Refused later');
  const box = pane.getByRole('textbox', { name: 'Message' });
  await box.fill(text);
  await box.press('Control+Enter');
  await expect(pane.locator('.msg', { hasText: text }).getByRole('status')).toBeVisible();
  await control(request, { refuse: 'not allowed here', down: false });
  const msg = pane.locator('.msg', { hasText: text });
  await expect(msg.getByRole('alert')).toContainText('Not sent: not allowed here');
  await expect(msg.getByRole('button', { name: 'Try again' })).toHaveCount(0);
  await msg.getByRole('button', { name: 'Discard' }).click();
  await expect(pane.locator('.msg', { hasText: text })).toHaveCount(0);
});

test('a send with no answer offers Try again with the same IDs, and holds back what came after', async ({ page }, info) => {
  const sent = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  let failing = true;
  await page.route('**/orgs/acme/people/actions', (route) => {
    const kind = route.request().postDataJSON().kind;
    return failing && kind === 'send' ? route.fulfill({ status: 503, body: 'down' }) : route.continue();
  });
  const first = unique('First');
  const second = unique('Second');
  const box = pane.getByRole('textbox', { name: 'Message' });
  await box.fill(first);
  await box.press('Control+Enter');
  const msg = pane.locator('.msg', { hasText: first });
  await expect(msg.getByRole('alert')).toContainText('no answer');
  await box.fill(second);
  await box.press('Control+Enter');
  await expect(pane.locator('.msg', { hasText: second }), 'it stands below the first').toBeVisible();
  expect(sent.filter((a) => a.kind === 'send').length, 'it does not overtake').toBe(1);

  failing = false;
  await msg.getByRole('button', { name: 'Try again' }).click();
  await expect(msg.getByRole('alert')).toHaveCount(0);
  await expect(pane.locator('.msg', { hasText: second }).locator('.send-state')).toHaveCount(0);
  const sends = sent.filter((a) => a.kind === 'send');
  expect(sends).toHaveLength(3);
  expect(sends[1].id, 'the same action').toBe(sends[0].id);
  expect((sends[1].message as { id: string }).id).toBe((sends[0].message as { id: string }).id);
  const texts = await pane.locator('.msg-body').allTextContents();
  expect(texts.indexOf(first)).toBeLessThan(texts.indexOf(second));
});

test('a reaction that fails says so on its message, and Discard takes it back', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const id = await post(request, 'anna', '#general', unique('React to this'));
  const msg = pane.locator(`[data-msg="${id}"]`);
  await expect(msg).toBeVisible();
  await page.route('**/orgs/acme/people/actions', (route) =>
    route.request().postDataJSON().kind === 'react' ? route.fulfill({ status: 409, json: { refused: 'message deleted' } }) : route.continue(),
  );
  await hover(msg);
  await msg.getByRole('button', { name: 'React', exact: true }).click();
  await page.getByRole('dialog').getByRole('button').filter({ hasText: '👍' }).first().click();
  await expect(msg.getByRole('alert')).toContainText('Reaction not saved: message deleted');
  await expect(msg.locator('.rx'), 'a refused reaction is not shown as made').toHaveCount(0);
  await msg.getByRole('button', { name: 'Discard' }).click();
  await expect(msg.getByRole('alert')).toHaveCount(0);
});

/** On a phone Mark read is a swipe to the right (D-023). */
async function markRead(page: Page, row: ReturnType<Page['locator']>, phone: boolean) {
  if (!phone) {
    await row.hover();
    await row.getByRole('button', { name: 'Mark read' }).click();
    return;
  }
  const box = (await row.locator('.row-main').boundingBox())!;
  const y = box.y + box.height - 8; // on the preview: a swipe does not start on a button
  const x = box.x + box.width / 2;
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y }] });
  for (let i = 1; i <= 6; i++) await cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x: x + (120 * i) / 6, y }] });
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
  await cdp.detach();
}

test('a Mark read that fails shows as failed, and the row is unread again', async ({ page, request }, info) => {
  const text = unique('Mark me read');
  await doorAct(request, { as: 'anna', kind: 'send', message: { id: `msg_mr_${Date.now()}`, conversation: '#general', thread: null, text: `@Marco ${text}`, mentions: ['marco'], level: null } });
  await start(page);
  const row = page.locator('[data-row].unread', { hasText: text });
  await expect(row).toBeVisible();
  await page.route('**/orgs/acme/people/actions', (route) =>
    route.request().postDataJSON().kind === 'read' ? route.fulfill({ status: 503, body: 'down' }) : route.continue(),
  );
  await markRead(page, row, info.project.name === 'phone');
  await expect(page.locator('.feed-problem')).toContainText('Not marked read');
  await expect(page.locator('[data-row].unread', { hasText: text }), 'it says unread again').toBeVisible();
});
