import { expect, test, type Page } from '@playwright/test';
import type { SenderEntry } from '../../../web/notification-app/src/api';
import { createFixtureServer, MAYA } from '../../../web/notification-app/src/fixtures';
import { answerCalls, expectNoSidewaysScroll, shot } from './helpers';

const ORG = 'larkmere-7';
const listPath = (person: string) => `sender/orgs/${ORG}/notifications?person=${person}`;
const base: SenderEntry = { name: 'n', person: 'maya', state: 'sent', at: '2026-11-17T10:00:00.000Z', delayed: false, why: 'Anna mentioned you in #launch', text: 'Original text', now: null };

async function open(page: Page, lists?: Record<string, SenderEntry[]>, address = '/notifications/') {
  const server = createFixtureServer(lists);
  const net = await answerCalls(page, server.handle);
  await page.goto(address);
  return { server, net };
}

const entries = (page: Page) => page.getByTestId('notification');

test('lists what was sent to the chosen person, newest first, whatever order the sender gives', async ({ page }) => {
  const shuffled = [MAYA[2], MAYA[0], MAYA[4], MAYA[1], MAYA[3]];
  const { net } = await open(page, { maya: shuffled });
  await expect(entries(page)).toHaveCount(5);
  await net.expectCall('GET', listPath('maya'));
  const whys = await page.locator('.why').allTextContents();
  expect(whys).toEqual(MAYA.map((n) => n.why));
  await expect(entries(page).first()).toContainText('Tue 10:44');
  await expect(entries(page).first()).toContainText('recurring editor');
});

test('choosing a person reads that person from the sender and keeps it in the address', async ({ page }) => {
  const { net } = await open(page, { maya: MAYA, tomas: [{ ...base, name: 't1', person: 'tomas', why: 'Marco replied in your thread in #engineering', text: 'Pool setting is back to forty.' }] });
  await expect(entries(page)).toHaveCount(5);
  await page.getByLabel('Delivered to').selectOption({ label: 'Tomás Arrieta' });
  await net.expectCall('GET', listPath('tomas'));
  await expect(entries(page)).toHaveCount(1);
  await expect(entries(page).first()).toContainText('Pool setting is back to forty.');
  expect(new URL(page.url()).searchParams.get('person')).toBe('tomas');
});

test('agents are not offered: they have no phone', async ({ page }) => {
  await open(page);
  await expect(entries(page)).toHaveCount(5);
  const options = await page.getByLabel('Delivered to').locator('option').allTextContents();
  expect(options).toHaveLength(30);
  expect(options).toContain('Maya Okafor');
  expect(options).not.toContain('Scribe');
});

test('a delayed notification is marked, and only that one', async ({ page }) => {
  await open(page, { maya: [{ ...base, name: 'late', delayed: true, at: '2026-11-17T10:05:00.000Z' }, { ...base, name: 'on-time' }] });
  await expect(entries(page)).toHaveCount(2);
  await expect(entries(page).nth(0).locator('.mark.delayed')).toHaveText('Delayed');
  await expect(entries(page).nth(1).locator('.mark.delayed')).toHaveCount(0);
});

test('read is shown as the current truth', async ({ page }) => {
  await open(page, { maya: [{ ...base, now: 'read' }] });
  await expect(entries(page).first().locator('.mark')).toHaveText('Read');
  await expect(entries(page).first()).toContainText('Original text');
  await expect(page.getByTestId('count')).toHaveText('0 unread');
});

test('a deleted message no longer shows its text', async ({ page }) => {
  await open(page, { maya: [{ ...base, now: 'deleted' }] });
  await expect(entries(page).first()).toContainText('Message deleted');
  await expect(entries(page).first()).not.toContainText('Original text');
});

test('an edit shows the new text, and the entry changes while it is on screen', async ({ page }) => {
  const list = [{ ...base }];
  await open(page, { maya: list });
  await expect(entries(page).first()).toContainText('Original text');
  list[0] = { ...base, text: 'The new text after the edit', edited: true };
  await expect(entries(page).first()).toContainText('The new text after the edit');
  await expect(entries(page).first()).not.toContainText('Original text');
  await expect(entries(page).first().locator('.mark')).toHaveText('Edited');
});

test('never shows a held or withdrawn entry; a history entry is marked as not delivered', async ({ page }) => {
  await open(page, { maya: [
    { ...base, name: 'held', state: 'held', at: null, why: 'Held why', text: 'Held text' },
    { ...base, name: 'withdrawn', state: 'withdrawn', why: 'Withdrawn why', text: 'Withdrawn text' },
    { ...base, name: 'history', state: 'history', at: null, why: 'History why', text: 'History text' },
    { ...base, name: 'sent', why: 'Sent why', text: 'Sent text' },
  ] });
  await expect(entries(page)).toHaveCount(2);
  await expect(page.locator('body')).not.toContainText(/held/i);
  await expect(page.locator('body')).not.toContainText('Withdrawn');
  await expect(entries(page).nth(0)).toContainText('Sent text');
  await expect(entries(page).nth(1)).toContainText('Listed only, not delivered');
  await expect(entries(page).nth(1)).toContainText('Not sent');
  await expect(page.getByTestId('count')).toHaveText('1 unread');
});

test('with only held entries the page is honestly empty', async ({ page }) => {
  await open(page, { maya: [{ ...base, state: 'held', at: null }] });
  await expect(page.getByTestId('empty')).toContainText('Nothing has been sent yet');
  await expect(entries(page)).toHaveCount(0);
});

test('the empty state says nothing has been sent yet', async ({ page }, info) => {
  await open(page, {});
  const empty = page.getByTestId('empty');
  await expect(empty).toContainText('Nothing has been sent yet');
  await expect(empty).toContainText('When something needs Maya');
  await expectNoSidewaysScroll(page);
  await shot(page, 'notifications-empty', info.project.name);
});

test('a sender that stops answering is said in place, and the last list stays', async ({ page }) => {
  const { net } = await open(page);
  await expect(entries(page)).toHaveCount(5);
  net.fail.set('GET ' + listPath('maya'), { status: 503, body: { error: 'down' } });
  await expect(page.getByTestId('problem')).toContainText('The notification sender: down. Showing the last list seen.');
  await expect(entries(page)).toHaveCount(5);
  net.fail.delete('GET ' + listPath('maya'));
  await expect(page.getByTestId('problem')).toHaveCount(0);
});

test('a sender that never answers still surfaces as a problem', async ({ page }) => {
  const { net } = await open(page);
  await expect(entries(page)).toHaveCount(5);
  net.fail.set('GET ' + listPath('maya'), 'hold');
  await expect(page.getByTestId('problem')).toContainText('no answer', { timeout: 9_000 });
  await expect(entries(page)).toHaveCount(5);
});

test('with no sender at the start it says so rather than claiming nothing was sent', async ({ page }) => {
  const server = createFixtureServer();
  const net = await answerCalls(page, server.handle);
  net.fail.set('GET ' + listPath('maya'), 'no-answer');
  await page.goto('/notifications/?person=maya');
  await expect(page.getByTestId('problem')).toContainText('no answer');
  await expect(page.getByTestId('empty')).toHaveCount(0);
});

test('the demo program not answering is said in place', async ({ page }) => {
  const server = createFixtureServer();
  const net = await answerCalls(page, server.handle);
  net.fail.set('GET demo/current', { status: 503, body: {} });
  await page.goto('/notifications/');
  await expect(page.getByTestId('problem')).toContainText('The demo program');
});

test('fits the width, in both themes', async ({ page }, info) => {
  await open(page);
  await expect(entries(page)).toHaveCount(5);
  await expectNoSidewaysScroll(page);
  await shot(page, 'notifications', info.project.name);
  for (const theme of ['light', 'dark']) {
    await page.getByLabel('Theme').selectOption(theme);
    await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
    const background = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    expect(background).toBe(theme === 'dark' ? 'rgb(16, 17, 18)' : 'rgb(233, 231, 225)');
    await expectNoSidewaysScroll(page);
  }
});

test('the default build runs from its fixtures with no server at all', async ({ page }) => {
  const outside: string[] = [];
  page.on('request', (r) => { if (new URL(r.url()).pathname.startsWith('/ops/')) outside.push(r.url()); });
  await page.goto('/notifications-fixtures/');
  await expect(entries(page)).toHaveCount(5);
  expect(outside).toEqual([]);
});
