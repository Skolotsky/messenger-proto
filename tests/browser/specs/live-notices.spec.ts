import { test as plain, expect } from '@playwright/test';
import { test as withFixtures, row } from '../fixtures/door';
import { control, doorAct, start, topOf, unique } from './helpers';
import { live, offline, resetDoor } from './live-helpers';

/** G-019, the screen's part: a surface behind says so in place, and says nothing when it is not. */

plain.afterEach(async ({ request }) => resetDoor(request));

/** One unread row of its own: the tests share a door, and earlier ones read what was there. */
const seedRow = (request: import('@playwright/test').APIRequestContext) =>
  doorAct(request, {
    as: 'anna',
    kind: 'send',
    message: { id: `msg_n_${unique('x').replace(/\W/g, '')}`, conversation: '#general', thread: null, text: unique('@Marco a row to hold still'), mentions: ['marco'], level: null },
  });

const feedTab = (page: import('@playwright/test').Page, name: RegExp) =>
  page.getByRole('navigation', { name: 'Activity views' }).getByRole('button', { name });

plain('"Reconnecting" appears past 10 s, in place, moves nothing, and goes when level', async ({ page, request }, info) => {
  await seedRow(request);
  await page.clock.install();
  await start(page);
  const firstRow = page.locator('[data-row]').first();
  await expect(firstRow).toBeVisible();
  const before = await topOf(firstRow);

  await control(request, { refuseLive: true, cut: true });
  await expect(offline(page)).toBeVisible();
  await page.clock.fastForward(5000);
  await expect(page.locator('.top-notice'), 'five seconds behind says nothing').toHaveCount(0);

  await page.clock.fastForward(6000);
  await expect(page.locator('.top-notice')).toContainText('Reconnecting · since');
  if (info.project.name === 'wide') await expect(page.locator('.ws-off')).toContainText('Reconnecting');
  expect(await topOf(firstRow), 'the list did not move').toBe(before);

  await control(request, { refuseLive: false });
  await expect
    .poll(async () => {
      await page.clock.fastForward(1100);
      return live(page).count();
    })
    .toBe(1);
  await expect(page.locator('.top-notice')).toHaveCount(0);
  await expect(page.locator('.ws-off')).toHaveCount(0);
});

plain('"Catching up" stands beside the title without moving the list, and goes when the feed is back', async ({ page, request }) => {
  await seedRow(request);
  await start(page);
  const firstRow = page.locator('[data-row]').first();
  await expect(firstRow).toBeVisible();
  const before = await topOf(firstRow);
  await control(request, { feedDown: true, beat: true });
  await feedTab(page, /^Mentions/).click();
  await feedTab(page, /^Feed/).click();
  await expect(page.locator('.top-notice')).toHaveText('Catching up');
  await expect(page.locator('[data-row]').first(), 'the rows it had stay').toBeVisible();
  expect(await topOf(page.locator('[data-row]').first())).toBe(before);

  await control(request, { feedDown: false, beat: true });
  await expect(page.locator('.top-notice')).toHaveCount(0);
});

withFixtures('a fenced row says it may be out of date', async ({ page, door }) => {
  door.rows = [row({ conversation: '#launch', thread: 'msg_fence', message: 'msg_fence', fenced: true }), row({ conversation: '#general', thread: 'msg_ok', message: 'msg_ok' })];
  await start(page);
  await expect(page.locator('[data-row="#launch|msg_fence"]')).toContainText('may be out of date');
  await expect(page.locator('[data-row="#general|msg_ok"]')).not.toContainText('may be out of date');
});
