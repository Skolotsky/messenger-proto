import { doorAct, control, openConversation, post, start, unique } from './helpers';
import { expect, test } from '@playwright/test';
import { expectEqualsFreshLoad, live, offline, resetDoor } from './live-helpers';

/** The screen half of G-015: after any interruption the screen ends equal to a freshly loaded one. */

test.describe.configure({ timeout: 60_000 });
test.afterEach(async ({ request }) => resetDoor(request));

const say = (request: Parameters<typeof post>[0], text: string) => post(request, 'anna', '#general', text);
const body = (page: import('@playwright/test').Locator, text: string) => page.locator('.msg-body', { hasText: text });

test('G-015: a cut connection resumes from its numbers and ends equal to a fresh load', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const edited = await post(request, 'anna', '#general', unique('To be edited'));
  await expect(pane.locator(`[data-msg="${edited}"]`)).toBeVisible();

  await control(request, { refuseLive: true, cut: true });
  await expect(offline(page)).toBeVisible();
  const missed = unique('Said while cut');
  await say(request, missed);
  await doorAct(request, { as: 'anna', kind: 'edit', message: edited, text: 'Edited while cut', mentions: [], level: null });
  await doorAct(request, { as: 'lena', kind: 'react', message: edited, emoji: '🎉', on: true });
  await control(request, { refuseLive: false });

  await expect(live(page)).toBeVisible();
  await expect(body(pane, missed)).toBeVisible({ timeout: 2000 });
  await expect(pane.locator(`[data-msg="${edited}"] .msg-body`)).toHaveText('Edited while cut');
  await expectEqualsFreshLoad(page, 'general');
});

test('G-015: a refused connection keeps trying, and catches up within 2 s of being let in', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await control(request, { refuseLive: true, cut: true });
  await expect(offline(page)).toBeVisible();
  const missed = unique('While refused');
  await say(request, missed);
  await expect(body(pane, missed)).toHaveCount(0);
  await control(request, { refuseLive: false });
  await expect(body(pane, missed)).toBeVisible({ timeout: 2000 });
  await expectEqualsFreshLoad(page, 'general');
});

test('G-015: a single lost update is noticed by the next one, and asked for again', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await control(request, { dropNext: 1 });
  const lost = unique('Lost on the way');
  await say(request, lost);
  const next = unique('The one after');
  await say(request, next);
  await expect(body(pane, next)).toBeVisible();
  await expect(body(pane, lost), 'what was lost came back').toBeVisible();
  const texts = await pane.locator('.msg-body').allTextContents();
  expect(texts.indexOf(lost), 'and in its place').toBeLessThan(texts.indexOf(next));
  await expectEqualsFreshLoad(page, 'general');
});

test('G-015: with every nudge lost, a lost update is healed by the regular last_sent', async ({ page, request }, info) => {
  test.setTimeout(45_000);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await control(request, { dropNext: 1 });
  const lost = unique('Nothing follows this');
  await say(request, lost);
  // Nothing else arrives: only the door's message every 5 s can tell the screen.
  await expect(body(pane, lost)).toBeVisible({ timeout: 20_000 });
  await expectEqualsFreshLoad(page, 'general');
});

test('G-015: updates arriving in the wrong order end in the right one', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await control(request, { hold: true });
  const texts = [unique('First'), unique('Second'), unique('Third')];
  for (const t of texts) await say(request, t);
  await control(request, { release: 'reverse' });
  for (const t of texts) await expect(body(pane, t)).toBeVisible();
  const shown = await pane.locator('.msg-body').allTextContents();
  expect(texts.map((t) => shown.indexOf(t))).toEqual([...texts.map((t) => shown.indexOf(t))].sort((a, b) => a - b));
  await expectEqualsFreshLoad(page, 'general');
});

test('G-015: another view opened with updates in flight does not move the position', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await control(request, { hold: true });
  const inFlight = unique('In flight');
  await say(request, inFlight);
  const elsewhere = unique('In flight elsewhere');
  await post(request, 'anna', '#launch', elsewhere);
  // Opening another conversation loads a view newer than the screen's position.
  if (info.project.name === 'phone') await pane.getByRole('button', { name: 'Back' }).click();
  const other = await openConversation(page, info, 'launch');
  await expect(body(other, elsewhere), 'the fresh view has it').toBeVisible();
  await control(request, { release: 'order' });
  if (info.project.name === 'phone') await other.getByRole('button', { name: 'Back' }).click();
  const back = await openConversation(page, info, 'general');
  await expect(body(back, inFlight)).toBeVisible();
  // Only updates count: one more proves the stream is still applied from the right place.
  const after = unique('After the view');
  await say(request, after);
  await expect(body(back, after)).toBeVisible();
  await expectEqualsFreshLoad(page, 'general');
});

test('G-015: more than 500 updates away, the screen is told and loads fresh', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await control(request, { refuseLive: true, cut: true });
  await expect(offline(page)).toBeVisible();
  await control(request, { burst: { as: 'anna', conversation: '#general', count: 510 } });
  const last = unique('After the burst');
  await say(request, last);
  await control(request, { refuseLive: false });
  await expect(live(page)).toBeVisible();
  await expect(body(pane, last), 'the whole view was loaded fresh').toBeVisible();
  await expectEqualsFreshLoad(page, 'general');
  const next = unique('Applied after the reload');
  await say(request, next);
  await expect(body(pane, next), 'and the stream goes on from the new position').toBeVisible();
});

test('G-015: a new edition of the feed loads the feed fresh', async ({ page, request }, info) => {
  await start(page);
  await openConversation(page, info, 'general');
  const feedLoads: string[] = [];
  page.on('request', (r) => {
    if (/\/people\/marco\/feed$/.test(r.url())) feedLoads.push(r.url());
  });
  await control(request, { newEdition: true, beat: true });
  await expect.poll(() => feedLoads.length, { message: 'the feed is loaded again' }).toBeGreaterThan(0);
  await post(request, 'anna', '#general', unique('After the rebuild'));
  await expectEqualsFreshLoad(page, 'general');
});

test('D-033: a retired organization sends the screen to the entry address, into the current one', async ({ page, request }, info) => {
  await start(page);
  await control(request, { retire: true });
  await expect(page).toHaveURL(/\?org=acme-2/);
  await expect(live(page)).toBeVisible();
  const said = unique('In the new organization');
  await request.post('http://127.0.0.1:8090/orgs/acme-2/people/actions', {
    data: { id: `act_${said}`, as: 'anna', kind: 'send', message: { id: `msg_${Date.now()}`, conversation: '#general', thread: null, text: said, mentions: [], level: null } },
  });
  if (info.project.name === 'phone') await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
  await expect(page.locator('.chat.unread', { hasText: 'general' }), 'and it is live there').toBeVisible();
});

/** With the connection away, only a door answer can tell the screen. Opening a conversation asks the door. */
async function askTheDoor(page: import('@playwright/test').Page, phone: boolean) {
  if (phone) {
    await page.getByRole('region', { name: 'Conversation' }).getByRole('button', { name: 'Back' }).click();
    await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
  }
  await page.getByRole('navigation', { name: 'Conversations' }).getByRole('button', { name: 'launch' }).first().click();
}

test('D-033: a door answer of 410 goes to the entry address too', async ({ page, request }, info) => {
  await start(page);
  await openConversation(page, info, 'general');
  await control(request, { refuseLive: true, cut: true });
  await expect(offline(page)).toBeVisible();
  await control(request, { retire: true });
  await askTheDoor(page, info.project.name === 'phone');
  await expect(page).toHaveURL(/\?org=acme-2/);
  await control(request, { refuseLive: false });
  await expect(live(page)).toBeVisible();
});

test('D-033: a 404 for the organization is treated the same way', async ({ page, request }, info) => {
  await start(page);
  await openConversation(page, info, 'general');
  await control(request, { refuseLive: true, cut: true });
  await expect(offline(page)).toBeVisible();
  await control(request, { retire: true, forget: true });
  await askTheDoor(page, info.project.name === 'phone');
  await expect(page).toHaveURL(/\?org=acme-2/);
  await control(request, { refuseLive: false });
  await expect(live(page)).toBeVisible();
});

test('a reply being written arrives in pieces, heals a gap, and closes', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const message = `msg_reply_${Date.now()}`;
  await control(request, { reply: { start: true, message, conversation: '#general', author: 'lena' } });
  const reply = pane.locator(`[data-reply="${message}"] .msg-body`);
  await expect(pane.locator(`[data-reply="${message}"]`)).toBeVisible();
  await control(request, { reply: { message, piece: 'Ready ' } });
  await expect(reply).toHaveText('Ready');
  await control(request, { reply: { message, piece: 'to ', silent: true } });
  await control(request, { reply: { message, piece: 'ship.' } });
  await expect(reply, 'the gap is fetched from the text so far').toHaveText('Ready to ship.');
  await control(request, { reply: { message, finish: true } });
  await expect(pane.locator(`[data-reply="${message}"]`)).toHaveCount(0);
  await expect(pane.locator(`[data-msg="${message}"] .msg-body`)).toHaveText('Ready to ship.');

  const dropped = `${message}_b`;
  await control(request, { reply: { start: true, message: dropped, conversation: '#general', author: 'lena' } });
  await expect(pane.locator(`[data-reply="${dropped}"]`)).toBeVisible();
  await control(request, { reply: { message: dropped, abandon: true } });
  await expect(pane.locator(`[data-reply="${dropped}"]`), 'reply.abandoned takes it away').toHaveCount(0);
});
