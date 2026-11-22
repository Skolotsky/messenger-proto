import { expect, type APIRequestContext, type Locator, type Page, type TestInfo } from '@playwright/test';

export const DOOR = 'http://127.0.0.1:8090';
let counter = 0;

/** Text no other test has used, so that tests sharing one door cannot see each other's messages. */
export const unique = (label: string) => `${label} ${Date.now().toString(36)}-${++counter}`;
const id = (prefix: string) => `${prefix}_t${Date.now().toString(36)}${++counter}`;

export const isPhone = (info: TestInfo) => info.project.name === 'phone';

export async function control(request: APIRequestContext, body: Record<string, unknown>) {
  const res = await request.post(`${DOOR}/mock/control`, { data: body });
  expect(res.ok(), 'the mock door takes the control call').toBeTruthy();
}

export async function doorAct(request: APIRequestContext, body: Record<string, unknown>) {
  const res = await request.post(`${DOOR}/orgs/acme/people/actions`, { data: { id: id('act'), ...body } });
  expect(res.ok(), `the door saves ${String(body.kind)}`).toBeTruthy();
  return res.json();
}

/** Someone else writes in a conversation, through the door. Returns the message's ID. */
export async function post(
  request: APIRequestContext,
  as: string,
  conversation: string,
  text: string,
  thread: string | null = null,
) {
  const messageId = id('msg');
  await doorAct(request, {
    as,
    kind: 'send',
    message: { id: messageId, conversation, thread, text, mentions: [], level: null },
  });
  return messageId;
}

export async function start(page: Page) {
  await page.goto('/');
  await expect(page.locator('.app[data-live="on"]')).toBeVisible();
}

/** Opens a conversation from the sidebar, going through the bottom bar on a phone. */
export async function openConversation(page: Page, info: TestInfo, name: string) {
  if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
  await page.getByRole('navigation', { name: 'Conversations' }).getByRole('button', { name }).first().click();
  const pane = page.getByRole('region', { name: 'Conversation' });
  await expect(pane.locator('[data-msg]').first()).toBeVisible();
  return pane;
}

export async function write(pane: Locator, text: string) {
  const box = pane.getByRole('textbox', { name: 'Message' });
  await box.fill(text);
  await box.press('Control+Enter');
  await expect(box).toHaveValue('');
}

export const topOf = (el: Locator) => el.evaluate((node) => node.getBoundingClientRect().top);

/** Lets the browser finish a frame, so that a measurement sees the settled layout. */
export const settled = (page: Page) =>
  page.evaluate(() => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done))));

/** No page-level sideways scroll, and nothing wider than the window. */
export async function expectNoSidewaysScroll(page: Page) {
  const over = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(over, 'the page does not scroll sideways').toBeLessThanOrEqual(0);
}
