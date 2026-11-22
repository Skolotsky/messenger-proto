import { expect, test, type Page } from '@playwright/test';
import { control, openConversation, post, settled, start, unique, write } from './helpers';

const NOW = new Date('2026-11-22T12:00:00Z');

function fixtureMessage(id: string, number: number, created_at: string) {
  return {
    message: {
      id, number, version: 1, conversation: '#launch', thread: null, author: 'anna', author_is_agent: false,
      participants: null, text: `Message ${id}`, mentions: [], level: null, reactions: [], stopped: false,
      created_at, edited_at: null, deleted: false,
    },
    thread_summary: null,
  };
}

test('a line separates the days, named as a person would', async ({ page }, info) => {
  await page.clock.setFixedTime(NOW);
  await page.route('**/orgs/acme/conversations/*/messages*', (route) =>
    route.fulfill({
      json: {
        number: 4, in_progress: [], read_up_to: null, has_before: false, has_after: false,
        messages: [
          fixtureMessage('old', 1, '2026-11-02T10:00:00Z'),
          fixtureMessage('week', 2, '2026-11-19T10:00:00Z'),
          fixtureMessage('yest', 3, '2026-11-21T10:00:00Z'),
          fixtureMessage('today1', 4, '2026-11-22T09:00:00Z'),
          fixtureMessage('today2', 5, '2026-11-22T10:00:00Z'),
        ],
      },
    }),
  );
  await start(page);
  const pane = await openConversation(page, info, 'launch');
  await expect(pane.locator('.dayline')).toHaveText(['2 November', 'Thursday', 'Yesterday', 'Today']);
  // Each line stands directly before the first message of its day.
  const next = await pane.locator('.dayline').evaluateAll((lines) => lines.map((l) => (l.nextElementSibling as HTMLElement).dataset.msg));
  expect(next).toEqual(['old', 'week', 'yest', 'today1']);
});

test('a sent message appears here and on another open screen', async ({ page, browser }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');

  const other = await browser.newContext({ viewport: { width: 1280, height: 800 }, serviceWorkers: 'block' });
  const otherPage = await other.newPage();
  await otherPage.addInitScript(() => localStorage.setItem('contexia-person', 'anna'));
  await otherPage.goto('http://127.0.0.1:4173/');
  await expect(otherPage.locator('.app[data-live="on"]')).toBeVisible();
  await otherPage.getByRole('navigation', { name: 'Conversations' }).getByRole('button', { name: 'general' }).click();
  const otherPane = otherPage.getByRole('region', { name: 'Conversation' });
  await expect(otherPane.locator('[data-msg]').first()).toBeVisible();

  const text = unique('Hello from the test');
  await write(pane, text);
  await expect(pane.locator('.msg-body', { hasText: text })).toBeVisible();
  await expect(otherPane.locator('.msg-body', { hasText: text })).toBeVisible();
  await other.close();
});

test('Enter starts a new line and Ctrl+Enter sends', async ({ page }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const box = pane.getByRole('textbox', { name: 'Message' });
  const first = unique('First line');
  await box.fill(first);
  await box.press('Enter');
  await box.pressSequentially('second line');
  await expect(box, 'Enter did not send').toHaveValue(`${first}\nsecond line`);
  await box.press('Control+Enter');
  await expect(box).toHaveValue('');
  await expect(pane.locator('.msg-body', { hasText: first })).toHaveText(`${first}\nsecond line`);
});

test('an arrival in a channel not open marks it unread, and opening clears the mark', async ({ page, request }, info) => {
  await start(page);
  await openConversation(page, info, 'general');
  if (info.project.name === 'phone') await page.getByRole('button', { name: 'Back' }).click();
  await post(request, 'anna', '#incidents', unique('Something arrived'));
  const row = page.getByRole('navigation', { name: 'Conversations' }).getByRole('button', { name: 'incidents' });
  await expect(row.getByRole('img', { name: 'unread' })).toBeAttached();
  await openConversation(page, info, 'incidents');
  await expect(row.getByRole('img', { name: 'unread' })).toHaveCount(0);
});

test('a send the door refuses stays in its place, says so, and offers Discard', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await control(request, { refuse: 'not allowed here' });
  const text = unique('Refused text');
  const box = pane.getByRole('textbox', { name: 'Message' });
  await box.fill(text);
  await box.press('Control+Enter');
  await expect(pane.getByRole('alert')).toContainText('Not sent: not allowed here');
  const msg = pane.locator('.msg', { hasText: text });
  await expect(msg.locator('.msg-body')).toHaveText(text);
  await expect(msg.getByRole('button', { name: 'Try again' }), 'trying again cannot help').toHaveCount(0);
  await msg.getByRole('button', { name: 'Discard' }).click();
  await expect(pane.locator('.msg-body', { hasText: text })).toHaveCount(0);
});

test('a send with the door down waits in its place, and goes through once when it is back', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  const box = pane.getByRole('textbox', { name: 'Message' });
  const text = unique('Written while down');
  try {
    await control(request, { down: true });
    await expect(page.locator('.app[data-live="off"]')).toBeVisible();
    await box.fill(text);
    await box.press('Control+Enter');
    await expect(box).toHaveValue('');
    await expect(pane.locator('.msg', { hasText: text }).getByRole('status')).toContainText('Waiting to send');
  } finally {
    await control(request, { down: false });
  }
  await expect(pane.locator('.msg', { hasText: text }).getByRole('status')).toHaveCount(0);
  await expect(pane.locator('.msg-body', { hasText: text })).toHaveCount(1);

  // The live connection came back by itself.
  await expect(page.locator('.app[data-live="on"]')).toBeVisible();
  const arrival = unique('After the outage');
  await post(request, 'anna', '#general', arrival);
  await expect(pane.locator('.msg-body', { hasText: arrival })).toBeVisible();
});

const readsOf = (page: Page) => {
  const reads: { conversation: string; thread: string | null }[] = [];
  page.on('request', (r) => {
    if (r.method() === 'POST' && r.url().endsWith('/people/actions') && r.postDataJSON().kind === 'read') reads.push(r.postDataJSON());
  });
  return reads;
};

test('nothing is read while the window is not in front of the person (D-015)', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await expect(pane.locator('[data-msg]').last()).toBeInViewport();
  await settled(page);
  const reads = readsOf(page);
  await page.evaluate(() => {
    document.hasFocus = () => false;
    window.dispatchEvent(new Event('blur'));
  });
  const text = unique('While away');
  await post(request, 'anna', '#general', text);
  await expect(pane.locator('.msg-body', { hasText: text })).toBeAttached();
  await settled(page);
  expect(reads, 'nothing was marked read while away').toEqual([]);

  await page.evaluate(() => {
    document.hasFocus = () => true;
    window.dispatchEvent(new Event('focus'));
  });
  await expect.poll(() => reads.length, { message: 'it is read on coming back' }).toBe(1);
});

test('on a phone, a channel covered by its thread is not read', async ({ page, request }, info) => {
  test.skip(info.project.name !== 'phone', 'on a wide screen the channel stays in view');
  const root = await post(request, 'anna', '#general', unique('Root'));
  await post(request, 'lena', '#general', unique('Reply'), root);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await pane.locator(`[data-msg="${root}"] .thread-link`).click();
  await expect(page.getByRole('region', { name: 'Thread' }).locator('[data-msg]')).toHaveCount(2);
  await settled(page);
  const reads = readsOf(page);
  const text = unique('Behind the thread');
  await post(request, 'priya', '#general', text);
  await expect(pane.locator('.msg-body', { hasText: text })).toBeAttached();
  await settled(page);
  expect(reads.filter((r) => r.thread === null), 'the channel was not read').toEqual([]);
});

test('a draft stays with its conversation', async ({ page }, info) => {
  await start(page);
  let pane = await openConversation(page, info, 'general');
  const draft = unique('A draft');
  await pane.getByRole('textbox', { name: 'Message' }).fill(draft);
  if (info.project.name === 'phone') await page.getByRole('button', { name: 'Back' }).click();
  pane = await openConversation(page, info, 'incidents');
  await expect(pane.getByRole('textbox', { name: 'Message' })).toHaveValue('');
  if (info.project.name === 'phone') await page.getByRole('button', { name: 'Back' }).click();
  pane = await openConversation(page, info, 'general');
  await expect(pane.getByRole('textbox', { name: 'Message' })).toHaveValue(draft);
});
