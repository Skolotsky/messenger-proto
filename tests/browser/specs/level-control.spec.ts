import { expect, test, type Page } from '@playwright/test';
import { control, doorAct, openConversation, post, start, unique, write } from './helpers';
import { live, offline, resetDoor } from './live-helpers';

/** D-001, D-023, D-035: a channel's and a thread's level, changed from the conversation; read and watching. */

test.afterEach(async ({ request }) => resetDoor(request));

function actions(page: Page) {
  const sent: Record<string, unknown>[] = [];
  page.on('request', (r) => {
    if (r.method() === 'POST' && r.url().endsWith('/people/actions')) sent.push(r.postDataJSON());
  });
  return sent;
}

const setLevelBack = (request: Parameters<typeof doorAct>[0], conversation: string, thread: string | null) =>
  doorAct(request, { as: 'marco', kind: 'dial', conversation, thread, level: thread ? 'auto' : 'default' });

test('the channel level is one menu in the conversation, and it stays after a reload', async ({ page, request }, info) => {
  const sent = actions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  try {
    await pane.getByRole('button', { name: /^Channel level/ }).click();
    const menu = pane.getByRole('menu', { name: 'Level of this channel' });
    await expect(menu.getByRole('menuitemradio')).toHaveText([/^Focus/, /^All activity/, /^Default/, /^Muted/]);
    await menu.getByRole('menuitemradio', { name: /^Muted/ }).click();
    await expect(menu).toHaveCount(0);
    await expect(pane.getByRole('button', { name: 'Channel level: Muted' })).toBeVisible();
    expect(sent.filter((a) => a.kind === 'dial')).toMatchObject([{ conversation: '#general', thread: null, level: 'muted' }]);

    await page.reload();
    await expect(live(page)).toBeVisible();
    const again = await openConversation(page, info, 'general');
    await expect(again.getByRole('button', { name: 'Channel level: Muted' })).toBeVisible();
  } finally {
    await setLevelBack(request, '#general', null);
  }
});

test('Follow and Unfollow are one click on a thread, with all four levels beside them', async ({ page, request }, info) => {
  const root = await post(request, 'anna', '#general', unique('A thread that is not mine'));
  await post(request, 'lena', '#general', unique('A reply'), root);
  const sent = actions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  try {
    await pane.locator(`[data-msg="${root}"] .thread-link`).click();
    const thread = page.getByRole('region', { name: 'Thread' });
    await thread.getByRole('button', { name: 'Follow', exact: true }).click();
    await expect(thread.getByRole('button', { name: 'Unfollow' })).toBeVisible();
    await expect(thread.getByRole('button', { name: 'Thread level: For your information' })).toBeVisible();
    expect(sent.filter((a) => a.kind === 'dial').pop()).toMatchObject({ conversation: '#general', thread: root, level: 'fyi' });

    await thread.getByRole('button', { name: 'Unfollow' }).click();
    await expect(thread.getByRole('button', { name: 'Follow', exact: true })).toBeVisible();
    expect(sent.filter((a) => a.kind === 'dial').pop()).toMatchObject({ thread: root, level: 'off' });

    await thread.getByRole('button', { name: /^Thread level/ }).click();
    const menu = thread.getByRole('menu', { name: 'Level of this thread' });
    await expect(menu.getByRole('menuitemradio')).toHaveText([/^Focus/, /^Needs you/, /^For your information/, /^Off/]);
    await menu.getByRole('menuitemradio', { name: /^Focus/ }).click();
    await expect(thread.getByRole('button', { name: 'Thread level: Focus' })).toBeVisible();
    expect(sent.filter((a) => a.kind === 'dial').pop()).toMatchObject({ thread: root, level: 'focus' });
  } finally {
    await setLevelBack(request, '#general', root);
  }
});

test('a thread you wrote in starts at Needs you, by its automatic level', async ({ page, request }, info) => {
  const root = await post(request, 'anna', '#general', unique('Ask me'));
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await pane.locator(`[data-msg="${root}"]`).locator('.msg-self').first().hover();
  await pane.locator(`[data-msg="${root}"]`).getByRole('button', { name: 'Reply in thread' }).click();
  const thread = page.getByRole('region', { name: 'Thread' });
  await expect(thread.getByRole('button', { name: 'Follow', exact: true })).toBeVisible();
  await write(thread, unique('My answer'));
  await expect(thread.getByRole('button', { name: 'Thread level: Needs you' })).toBeVisible();
  await expect(thread.getByRole('button', { name: 'Unfollow' })).toBeVisible();
});

test('a level the door does not save says so in place', async ({ page }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await page.route('**/orgs/acme/people/actions', (route) =>
    route.request().postDataJSON().kind === 'dial' ? route.fulfill({ status: 503, body: 'down' }) : route.continue(),
  );
  await pane.getByRole('button', { name: /^Channel level/ }).click();
  await pane.getByRole('menuitemradio', { name: /^Muted/ }).click();
  await expect(pane.locator('.lv-err')).toHaveText('Not saved');
  await expect(pane.getByRole('button', { name: 'Channel level: Default' }), 'it did not change').toBeVisible();
});

test('reading sends the right row: the channel for the channel, the thread for the thread', async ({ page, request }, info) => {
  const root = await post(request, 'anna', '#general', unique('Root'));
  const reply = await post(request, 'lena', '#general', unique('Reply'), root);
  const sent = actions(page);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await expect.poll(() => sent.filter((a) => a.kind === 'read' && a.conversation === '#general' && a.thread === null).length).toBeGreaterThan(0);
  await pane.locator(`[data-msg="${root}"] .thread-link`).click();
  await expect(page.getByRole('region', { name: 'Thread' }).locator(`[data-msg="${reply}"]`)).toBeVisible();
  await expect.poll(() => sent.filter((a) => a.kind === 'read' && a.thread === root).length, 'the thread is read as its own row').toBeGreaterThan(0);
});

test('watching is said over the live connection, ends when the view closes, and is said again after a reconnect', async ({ page, request }, info) => {
  const said: { conversation: string; thread: string | null; open: boolean }[] = [];
  await page.routeWebSocket(/\/live/, (ws) => {
    const server = ws.connectToServer();
    ws.onMessage((m) => {
      const parsed = JSON.parse(String(m));
      if (parsed.watching) said.push(parsed.watching);
      server.send(m);
    });
    server.onMessage((m) => ws.send(m));
  });
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await expect.poll(() => said.at(-1)).toEqual({ conversation: '#general', thread: null, open: true });

  // An arrival on a row being watched is read at once: no mark, no row.
  const text = unique('Watched arrival');
  await post(request, 'anna', '#general', text);
  await expect(pane.locator('.msg-body', { hasText: text })).toBeVisible();
  await expect(page.locator('.chat.unread', { hasText: 'general' })).toHaveCount(0);

  const count = said.length;
  await control(request, { cut: true });
  await expect(offline(page)).toBeVisible();
  await expect(live(page)).toBeVisible();
  await expect.poll(() => said.slice(count).some((w) => w.conversation === '#general' && w.open), 'said again on the new connection').toBe(true);

  if (info.project.name === 'phone') await pane.getByRole('button', { name: 'Back' }).click();
  else await page.getByRole('navigation', { name: 'Conversations' }).getByRole('button', { name: 'launch' }).first().click();
  await expect.poll(() => said.some((w) => w.conversation === '#general' && !w.open), 'and it ends').toBe(true);
});
