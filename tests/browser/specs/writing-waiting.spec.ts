import { expect, test, type Locator, type Page } from '@playwright/test';
import { openConversation, post, start, unique, write } from './helpers';
import { resetDoor } from './live-helpers';
import { recordActions } from './writing-fixtures';

/**
 * screen-to-conversations.md section 4: with no answer the action waits and goes out by itself, in
 * order and with its ID, also while the live connection still looks up. A refusal stays final.
 */

test.afterEach(async ({ request }) => resetDoor(request));

const liveOn = (page: Page) => page.locator('.app[data-live="on"]');
const hover = (msg: Locator) => msg.locator('.msg-self').first().hover();

async function mine(pane: Locator, text: string) {
  await write(pane, text);
  const msg = pane.locator('.msg.mine', { hasText: text });
  await expect(msg.locator('.send-state')).toHaveCount(0);
  const id = await msg.getAttribute('data-msg');
  return pane.locator(`[data-msg="${id}"]`);
}

async function edit(msg: Locator, text: string) {
  await hover(msg);
  await msg.getByRole('button', { name: 'Edit' }).click();
  const box = msg.getByRole('textbox', { name: 'Edit your message' });
  await box.fill(text);
  await box.press('Control+Enter');
}

async function remove(msg: Locator) {
  await hover(msg);
  await msg.getByRole('button', { name: 'Delete' }).click();
  await msg.getByRole('group', { name: 'Delete this message?' }).getByRole('button', { name: 'Delete' }).click();
}

async function react(page: Page, msg: Locator) {
  await hover(msg);
  await msg.getByRole('button', { name: 'React', exact: true }).click();
  await page.getByRole('dialog').getByRole('button').filter({ hasText: '👍' }).first().click();
}

test('an edit, a delete and a reaction wait while the live state stays up, and go out by themselves', async ({ page, request }, info) => {
  const actions = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const toEdit = await mine(pane, unique('To edit'));
  const toDelete = await mine(pane, unique('To delete'));
  const changed = unique('Edited while unreachable');
  const theirId = await post(request, 'anna', '#general', unique('To react to'));
  const theirs = pane.locator(`[data-msg="${theirId}"]`);
  await expect(theirs).toBeVisible();

  await page.context().setOffline(true);
  await edit(toEdit, changed);
  await expect(toEdit.locator('.send-state')).toContainText('Edit waiting to be saved');
  await expect(toEdit.locator('.msg-body'), 'the edit shows while it waits').toHaveText(changed);
  await remove(toDelete);
  await expect(toDelete.locator('.send-state')).toContainText('Delete waiting to be saved');
  await react(page, theirs);
  await expect(theirs.locator('.send-state')).toContainText('Reaction waiting to be saved');
  await expect(theirs.locator('.rx')).toContainText('👍');
  await expect(liveOn(page), 'the live state never dropped').toBeVisible();

  // A sensible pause, and nothing overtakes the action at the head of the line.
  await page.waitForTimeout(2500);
  expect(actions.of('edit').length, 'no tight loop').toBeLessThanOrEqual(4);
  expect(actions.of('delete'), 'waits its turn').toEqual([]);
  expect(actions.of('react'), 'waits its turn').toEqual([]);

  await page.context().setOffline(false);
  await expect(toEdit.locator('.send-state')).toHaveCount(0, { timeout: 8000 });
  await expect(pane.locator(`[data-msg="${await theirs.getAttribute('data-msg')}"]`).locator('.send-state')).toHaveCount(0, { timeout: 8000 });
  await expect(toDelete).toHaveCount(0, { timeout: 8000 });
  await expect(toEdit.locator('.msg-body')).toHaveText(changed);
  await expect(theirs.locator('.rx')).toContainText('👍');

  const edits = actions.of('edit');
  expect(edits.length, 'it was tried again').toBeGreaterThan(1);
  expect(new Set(edits.map((a: { id: string }) => a.id)).size, 'with its original ID').toBe(1);
  expect(new Set(actions.of('delete').map((a: { id: string }) => a.id)).size).toBe(1);
  expect(new Set(actions.of('react').map((a: { id: string }) => a.id)).size).toBe(1);
});

test('a server error waits and is tried again with the same ID; a refusal on the retry is final', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const msg = await mine(pane, unique('Answered later'));
  let answer: 503 | 409 = 503;
  await page.route('**/orgs/acme/people/actions', (route) =>
    route.request().postDataJSON().kind !== 'edit'
      ? route.fallback()
      : answer === 503
        ? route.fulfill({ status: 503, json: { error: 'down' } })
        : route.fulfill({ status: 409, json: { refused: 'message deleted' } }),
  );
  await edit(msg, unique('Never saved'));
  await expect(msg.locator('.send-state')).toContainText('Edit waiting to be saved');
  answer = 409;
  await expect(msg.getByRole('alert')).toContainText('Not saved: message deleted', { timeout: 8000 });
  await expect(msg.getByRole('button', { name: 'Try again' })).toHaveCount(0);
  const edits = actions.of('edit');
  expect(edits.length).toBeGreaterThan(1);
  expect(new Set(edits.map((a: { id: string }) => a.id)).size).toBe(1);
  const count = edits.length;
  await page.waitForTimeout(1500);
  expect(actions.of('edit').length, 'a refusal is not tried again').toBe(count);
  await msg.getByRole('button', { name: 'Discard' }).click();
  await expect(msg.locator('.send-state')).toHaveCount(0);
});

test('a reply that stopped halfway says so, without moving anything', async ({ page, request }, info) => {
  const stoppedId = await post(request, 'anna', '#general', unique('A reply that gave up'));
  const plainId = await post(request, 'anna', '#general', unique('A finished message'));
  await page.route('**/orgs/acme/conversations/*/messages*', async (route) => {
    const response = await route.fetch();
    const body = await response.json();
    for (const r of body.messages ?? []) if (r.message.id === stoppedId) r.message.stopped = true;
    await route.fulfill({ response, json: body });
  });
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const stopped = pane.locator(`[data-msg="${stoppedId}"]`);
  const plain = pane.locator(`[data-msg="${plainId}"]`);
  await expect(stopped.locator('.msg-head .tag')).toHaveText('stopped');
  await expect(plain.locator('.msg-head .tag')).toHaveCount(0);
  const height = (m: Locator) => m.locator('.msg-head').evaluate((n) => n.getBoundingClientRect().height);
  expect(await height(stopped), 'the tag does not change the line').toBe(await height(plain));
  for (const scheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: scheme });
    const looks = await stopped.locator('.msg-head .tag').evaluate((n) => {
      const s = getComputedStyle(n);
      return { color: s.color, background: s.backgroundColor };
    });
    expect(looks.color, `readable in ${scheme}`).not.toBe(looks.background);
  }
});
