import { expect, test, type APIRequestContext, type Locator, type Page, type TestInfo } from '@playwright/test';
import { doorAct, isPhone, openConversation, post, settled, start, topOf, unique } from './helpers';

// "Nothing jumps", measured as in E-002: the position of what is being read, before and after.
const TOLERANCE = 0.5;

/** Opens a channel long enough to scroll, scrolled to its middle. Returns a message in view. */
async function reading(page: Page, request: APIRequestContext, info: TestInfo) {
  const ids: string[] = [];
  for (let i = 0; i < 40; i++) ids.push(await post(request, 'anna', '#incidents', unique(`Filler ${i}`)));
  await start(page);
  const pane = await openConversation(page, info, 'incidents');
  const list = pane.getByTestId('messages');
  await expect(pane.locator(`[data-msg="${ids[39]}"]`)).toBeAttached();
  // Half of the fillers end up above the view and half below it.
  await pane.locator(`[data-msg="${ids[20]}"]`).evaluate((m) => {
    const el = m.closest('[data-testid="messages"]')!;
    el.scrollTop += m.getBoundingClientRect().top - el.getBoundingClientRect().top;
  });
  await settled(page);
  const target = await list.evaluate((el) => {
    const box = el.getBoundingClientRect();
    const seen = [...el.querySelectorAll<HTMLElement>('[data-msg]')].find((m) => {
      const r = m.getBoundingClientRect();
      return r.top > box.top + 60 && r.bottom < box.bottom - 60;
    });
    return seen!.dataset.msg!;
  });
  return { pane, list, ids, watched: pane.locator(`[data-msg="${target}"]`) };
}

async function expectStill(page: Page, watched: Locator, before: number, what: string) {
  await settled(page);
  const moved = Math.abs((await topOf(watched)) - before);
  expect(moved, `${what}: the message being read moved ${moved.toFixed(2)} px`).toBeLessThan(TOLERANCE);
}

test('an arrival while reading older messages moves nothing', async ({ page, request }, info) => {
  const { pane, watched } = await reading(page, request, info);
  const before = await topOf(watched);
  const text = unique('Arrived below');
  await post(request, 'lena', '#incidents', text);
  await expect(pane.locator('.msg-body', { hasText: text })).toBeAttached();
  await expectStill(page, watched, before, 'arrival');
});

test('a message above the view growing moves nothing', async ({ page, request }, info) => {
  const { pane, list, ids, watched } = await reading(page, request, info);
  // The oldest of the fillers is above the view: make it several lines long.
  const above = ids[0];
  expect(await pane.locator(`[data-msg="${above}"]`).evaluate((n, box) => n.getBoundingClientRect().bottom < (box as Element).getBoundingClientRect().top, await list.elementHandle())).toBeTruthy();
  const before = await topOf(watched);
  const heightBefore = await pane.locator(`[data-msg="${above}"]`).evaluate((n) => n.getBoundingClientRect().height);
  const long = Array.from({ length: 12 }, (_, i) => `Line ${i} of a much longer edit`).join('\n');
  await doorAct(request, { as: 'anna', kind: 'edit', message: above, text: long, mentions: [], level: null });
  await expect(pane.locator(`[data-msg="${above}"] .msg-body`)).toContainText('Line 11');
  const heightAfter = await pane.locator(`[data-msg="${above}"]`).evaluate((n) => n.getBoundingClientRect().height);
  expect(heightAfter, 'the edit really made it taller').toBeGreaterThan(heightBefore + 100);
  await expectStill(page, watched, before, 'an edit above the view');
});

test('opening and closing a thread moves nothing in the channel', async ({ page, request }, info) => {
  test.skip(isPhone(info), 'on a phone the thread takes the whole screen');
  const { pane, watched } = await reading(page, request, info);
  const id = await watched.getAttribute('data-msg');
  const before = await topOf(watched);
  await post(request, 'lena', '#incidents', unique('A reply'), id);
  await expect(watched.locator('.thread-link')).toBeVisible();
  await expectStill(page, watched, before, 'a reply appearing under a message in view');

  const held = await topOf(watched);
  await watched.locator('.thread-link').click();
  const thread = page.getByRole('region', { name: 'Thread' });
  await expect(thread).toBeVisible();
  await expectStill(page, watched, held, 'opening a thread');
  await thread.getByRole('button', { name: 'Close the thread' }).click();
  await expect(thread).toBeHidden();
  await expectStill(page, watched, held, 'closing a thread');
  await expect(pane).toBeVisible();
});

test('a toolbar on hover moves nothing', async ({ page, request }, info) => {
  test.skip(isPhone(info), 'a phone has no hover');
  const { list, watched } = await reading(page, request, info);
  const before = await topOf(watched);
  const height = await list.evaluate((el) => el.scrollHeight);
  await watched.locator('.msg-self').hover();
  await expect(watched.locator('.msg-tools')).toHaveCSS('opacity', '1');
  await expectStill(page, watched, before, 'hover');
  expect(await list.evaluate((el) => el.scrollHeight)).toBe(height);
});

test('choosing a level in the message box moves nothing', async ({ page, request }, info) => {
  const { pane, watched } = await reading(page, request, info);
  const before = await topOf(watched);
  const composer = pane.locator('.composer');
  const height = (await composer.boundingBox())!.height;
  await composer.getByRole('button', { name: 'Flag' }).click();
  await expect(composer.getByRole('button', { name: 'Flag' })).toHaveAttribute('aria-pressed', 'true');
  await expectStill(page, watched, before, 'choosing Flag');
  expect((await composer.boundingBox())!.height).toBe(height);
});

test('at the bottom, an arrival stays in view', async ({ page, request }, info) => {
  await start(page);
  const pane = await openConversation(page, info, 'launch');
  const text = unique('Newest message');
  await post(request, 'anna', '#launch', text);
  await expect(pane.locator('.msg-body', { hasText: text })).toBeInViewport();
});
