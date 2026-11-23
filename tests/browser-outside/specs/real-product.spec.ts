import { expect, test, type APIRequestContext, type Locator, type Page } from '@playwright/test';

// T-058: the main flow of the web app on the real server, at both widths. Set CONTEXIA_REAL_URL
// to the system's one address, for example http://127.0.0.1:18080. Without it this is skipped.
// It reads the server's own answers beside the screen, so it fails when the two stop agreeing.
const real = process.env.CONTEXIA_REAL_URL;

type Row = { conversation: string; thread: string | null; preview: string; unread: boolean; kept: string | null };
type Feed = { rows: Row[]; counts: { focus: number; needs_you: number } };

const stamp = () => Date.now().toString(36);

class System {
  constructor(
    readonly request: APIRequestContext,
    readonly org: string,
  ) {}

  async get<T>(path: string): Promise<T> {
    const res = await this.request.get(`/orgs/${this.org}${path}`);
    expect(res.ok(), `the door answers ${path}`).toBeTruthy();
    return (await res.json()) as T;
  }

  feed = (who: string) => this.get<Feed>(`/people/${who}/feed`);

  async mode(part: 'feed' | 'search', mode: 'off' | 'running') {
    const res = await this.request.post(`/ops/${part}/orgs/${this.org}/control/mode`, { data: { mode } });
    expect(res.ok()).toBeTruthy();
  }

  async say(as: string, conversation: string, text: string, mentions: string[] = []) {
    const id = `${as}_${stamp()}_${Math.random().toString(36).slice(2, 8)}`;
    const res = await this.request.post(`/orgs/${this.org}/people/actions`, {
      data: { id: `act_${id}`, as, kind: 'send', message: { id: `msg_${id}`, conversation, thread: null, text, mentions, level: null } },
    });
    expect(res.ok(), 'the door saves the message').toBeTruthy();
  }
}

async function screen(page: Page, org: string, who: string) {
  await page.addInitScript((id) => localStorage.setItem('contexia-person', id), who);
  await page.goto(`/?org=${org}`);
  await expect(page.locator('[data-testid=feed] article').first()).toBeVisible({ timeout: 20_000 });
}

const isPhone = (page: Page) => (page.viewportSize()?.width ?? 1280) <= 760;
const mainNav = (page: Page) => page.getByRole('navigation', { name: 'Main' });

async function toChannels(page: Page) {
  if (isPhone(page)) await mainNav(page).getByRole('button', { name: 'Channels' }).click();
}

async function toActivity(page: Page) {
  if (!isPhone(page)) return;
  for (const back of await page.locator('.conv .back:visible').all()) await back.click();
  await mainNav(page).getByRole('button', { name: 'Activity' }).click();
}

async function openChannel(page: Page, name: string) {
  await toChannels(page);
  await page.locator('.chats .chat', { hasText: name }).first().click();
  await expect(page.locator('.pane:not(.tpane) .conv-title b')).toHaveText(`#${name}`);
  await expect(page.locator('.pane:not(.tpane) .msg').first()).toBeVisible();
}

async function write(box: Locator, text: string) {
  await box.fill(text);
  await box.press('Control+Enter');
}

const channelBox = (page: Page) => page.locator('.pane:not(.tpane) textarea[aria-label=Message]');
const message = (page: Page, text: string) => page.locator('.pane:not(.tpane) .msg', { hasText: text });

async function tool(page: Page, on: Locator, name: string) {
  if (!isPhone(page)) await on.hover();
  await on.getByRole('button', { name, exact: true }).first().click({ force: isPhone(page) });
}

test.describe('the web app on the real server', () => {
  test.skip(!real, 'needs CONTEXIA_REAL_URL');
  test.use({ baseURL: real, serviceWorkers: 'block', actionTimeout: 10_000 });
  test.setTimeout(240_000);

  test('a person uses it end to end, and the screen agrees with the server', async ({ browser, request }, info) => {
    await expect.poll(async () => (await request.get('/demo/current')).status(), { timeout: 150_000 }).toBe(200);
    // A fresh organization each time: the walk reads rows, so it needs unread ones to start from.
    const reset = await request.post('/demo/reset', { timeout: 150_000 });
    expect(reset.ok(), 'Reset gives a fresh organization').toBeTruthy();
    const { org } = (await reset.json()) as { org: string };
    const system = new System(request, org);
    const use = info.project.use;
    const context = () => browser.newContext({ baseURL: real, viewport: use.viewport, hasTouch: use.hasTouch, isMobile: use.isMobile, serviceWorkers: 'block', locale: 'en-GB' });
    const opened = async () => {
      const p = await (await context()).newPage();
      p.setDefaultTimeout(10_000); // a step that cannot act fails here instead of hanging the run
      return p;
    };
    const page = await opened();
    const other = await opened();
    const mark = stamp();
    await screen(page, org, 'maya');
    await screen(other, org, 'marco');

    await test.step('the three buckets show the server\'s rows and counts', async () => {
      const feed = await system.feed('maya');
      await expect(page.locator('section.b-focus .bk-count')).toHaveText(String(feed.counts.focus));
      await expect(page.locator('section.b-needs_you .bk-count')).toHaveText(String(feed.counts.needs_you));
      await expect(page.locator('[data-testid=feed] article')).toHaveCount(feed.rows.length);
    });

    await test.step('a row\'s reason, its button and Undo change the setting and put it back', async () => {
      const before = await system.get('/people/maya/settings');
      await page.locator('section.b-needs_you .row .why-btn').first().click();
      const reason = page.getByRole('dialog', { name: /Why this is in/ });
      await reason.locator('.why-act button').click();
      await expect(reason.getByRole('button', { name: 'Undo' })).toBeVisible();
      await expect.poll(() => system.get('/people/maya/settings')).not.toEqual(before);
      await reason.getByRole('button', { name: 'Undo' }).click();
      await expect.poll(() => system.get('/people/maya/settings')).toEqual(before);
      await page.keyboard.press('Escape');
    });

    await test.step('opening a row reads it on the server', async () => {
      const row = page.locator('section.b-focus .row.unread').first();
      const key = (await row.getAttribute('data-row'))!;
      await row.locator('.row-hit').click({ position: { x: 4, y: 4 } });
      await expect(page.locator('.pane .conv-title').first()).toBeVisible();
      await expect
        .poll(async () => (await system.feed('maya')).rows.find((r) => `${r.conversation}|${r.thread ?? ''}` === key)?.unread ?? false)
        .toBe(false);
      await toActivity(page);
    });

    await test.step('Keep and Done are saved on the server', async () => {
      const row = page.locator('section.b-needs_you .row.unread').first();
      const key = (await row.getAttribute('data-row'))!;
      const kept = async () => (await system.feed('maya')).rows.find((r) => `${r.conversation}|${r.thread ?? ''}` === key)?.kept ?? null;
      if (isPhone(page)) {
        // The phone's Keep is a swipe to the left (D-023).
        const main = row.locator('.row-main');
        const box = (await main.boundingBox())!;
        const at = (x: number) => ({ clientX: box.x + x, clientY: box.y + 30, pointerId: 1, bubbles: true });
        await main.dispatchEvent('pointerdown', at(300));
        await main.dispatchEvent('pointermove', at(150));
        await main.dispatchEvent('pointerup', at(150));
      } else {
        await row.hover();
        await row.locator('.keep-btn').click();
      }
      await expect.poll(kept).toBe('you');
      const shown = page.locator(`[data-row="${key}"]`);
      await shown.locator('.done-btn').click({ force: isPhone(page) });
      await expect.poll(kept).toBeNull();
    });

    await test.step('each history tab shows the server\'s entries and count', async () => {
      // Reactions first: one message has several entries there, and a list that confuses them leaves rows behind in the next tab.
      const tabs = [['reactions', 'Reactions'], ['direct', 'Direct messages'], ['mentions', 'Mentions'], ['threads', 'Threads']] as const;
      for (const [tab, label] of tabs) {
        const button = page.getByRole('navigation', { name: 'Activity views' }).getByRole('button', { name: new RegExp(`^${label}`) });
        await button.click(); // leaving the feed reads what was seen in For your information, so the server is asked after this
        await expect
          .poll(async () => {
            const answer = await system.get<{ unread: number; entries: unknown[] }>(`/people/maya/history?tab=${tab}`);
            return {
              rows: (await page.locator('.hrow').count()) === answer.entries.length,
              count: (await button.getAttribute('aria-label')) === (answer.unread ? `${label}, ${answer.unread} unread` : label),
            };
          }, { message: `the ${label} tab shows the server's entries and count` })
          .toEqual({ rows: true, count: true });
      }
      await page.getByRole('navigation', { name: 'Activity views' }).getByRole('button', { name: /^Feed/ }).click();
    });

    await test.step('a message, its reply, edit, reaction and delete reach a second screen', async () => {
      await openChannel(page, 'general');
      await openChannel(other, 'general');
      await write(channelBox(page), `hello ${mark}`);
      await expect(message(other, `hello ${mark}`)).toBeVisible({ timeout: 3000 });

      await tool(page, message(page, `hello ${mark}`), 'Reply in thread');
      const reply = page.locator('.tpane textarea[aria-label=Message]');
      await write(reply, `reply ${mark}`);
      await expect(page.locator('.tpane .msg', { hasText: `reply ${mark}` })).toBeVisible();
      await expect(message(other, `hello ${mark}`).locator('.thread-link')).toContainText('1 reply');
      if (isPhone(page)) await page.locator('.tpane .back').click();

      await tool(page, message(page, `hello ${mark}`), 'Edit');
      const edit = page.getByRole('textbox', { name: 'Edit your message' });
      await edit.fill(`changed ${mark}`);
      await edit.press('Control+Enter');
      await expect(message(other, `changed ${mark}`).locator('.edited')).toBeVisible();

      await tool(other, message(other, `changed ${mark}`), 'React');
      await other.getByRole('dialog', { name: 'Choose a reaction' }).locator('button').nth(2).click();
      await expect(message(page, `changed ${mark}`).locator('.rx')).toHaveCount(1);

      await write(channelBox(page), `remove ${mark}`);
      await expect(message(other, `remove ${mark}`)).toBeVisible();
      await tool(page, message(page, `remove ${mark}`), 'Delete');
      await page.getByRole('group', { name: 'Delete this message?' }).getByRole('button', { name: 'Delete' }).click();
      await expect(message(other, `remove ${mark}`)).toHaveCount(0);
    });

    await test.step('a mention is suggested and reaches the person\'s feed', async () => {
      const box = channelBox(page);
      await box.fill('');
      await box.pressSequentially('@Lena');
      await box.press('Enter');
      await expect(box).toHaveValue('@Lena Fischer '.startsWith(await box.inputValue()) ? await box.inputValue() : /^@Lena/);
      await box.pressSequentially(`look ${mark}`);
      await box.press('Control+Enter');
      await expect
        .poll(async () => (await system.feed('lena')).rows.some((r) => r.preview.includes(`look ${mark}`)), { timeout: 5000 })
        .toBe(true);
    });

    await test.step('search finds a reply and opens its thread', async () => {
      if (isPhone(page)) {
        await page.locator('.pane:not(.tpane) .back').click();
        await toChannels(page);
      }
      const search = page.getByRole('searchbox', { name: 'Search messages' });
      await expect(async () => {
        await search.fill('');
        await search.fill(`reply ${mark}`);
        await expect(page.getByTestId('search-total')).toHaveText('1 message', { timeout: 1500 });
      }).toPass({ timeout: 10_000 });
      await page.locator('[data-hit-id]').first().click();
      await expect(page.locator('.tpane .msg', { hasText: `reply ${mark}` })).toBeVisible();
      if (isPhone(page)) {
        await page.locator('.tpane .back').click();
        await page.locator('.pane:not(.tpane) .back').click();
      }
      await search.fill('').catch(() => undefined);
    });

    await test.step('a new channel and a direct message work for both sides', async () => {
      await toChannels(page);
      await page.getByRole('button', { name: 'Create a channel' }).click();
      const dialog = page.getByRole('dialog', { name: 'Create a channel' });
      await dialog.locator('input').first().fill(`room ${mark}`);
      await dialog.getByRole('button', { name: 'Marco Bellandi' }).click();
      await dialog.getByRole('button', { name: 'Create', exact: true }).click();
      await expect(page.locator('.pane:not(.tpane) .conv-title b')).toHaveText(`#room-${mark}`);
      await write(channelBox(page), `first ${mark}`);
      await expect(message(page, `first ${mark}`)).toBeVisible();
      if (isPhone(other)) await other.locator('.pane:not(.tpane) .back').click();
      await toChannels(other);
      await expect(other.locator('.chats .chat', { hasText: `room-${mark}` })).toHaveCount(1);
      if (isPhone(page)) await page.locator('.pane:not(.tpane) .back').click();
      await toChannels(page);
      await page.getByRole('button', { name: 'New direct message' }).click();
      await page.getByRole('dialog', { name: 'New direct message' }).getByRole('button', { name: 'Hana Novak' }).click();
      await expect(page.locator('.pane:not(.tpane) .conv-title b')).toHaveText('Hana Novak');
      await write(channelBox(page), `direct ${mark}`);
      await expect
        .poll(async () => (await system.feed('hana')).rows.some((r) => r.preview.includes(`direct ${mark}`)), { timeout: 5000 })
        .toBe(true);
      if (isPhone(page)) await page.locator('.pane:not(.tpane) .back').click();
    });

    await test.step('an agent\'s state stands beside its name and follows it live', async () => {
      await openChannel(page, 'releases');
      const status = (state: string, on: string | null) =>
        request.post(`/orgs/${org}/agents/ship/status`, { data: { state, on } });
      expect((await status('working', null)).ok()).toBeTruthy();
      await expect(page.locator('.pane:not(.tpane) [data-agent-state=ship]')).toHaveText('Working');
      expect((await status('waiting', 'maya')).ok()).toBeTruthy();
      await expect(page.locator('.pane:not(.tpane) [data-agent-state=ship]')).toHaveText('Waiting on you');
      if (isPhone(page)) await page.locator('.pane:not(.tpane) .back').click();
    });

    await test.step('a scripted situation reaches the feed and the notification app', async () => {
      await toActivity(page); // what arrives before the feed is back in view is simply laid out, with no Show
      const played = await request.post(`/demo/orgs/${org}/script`, { data: { do: 'play_situation', situation: 'direct-mention' } });
      expect(played.ok()).toBeTruthy();
      await expect(page.locator('.newpill')).toBeVisible({ timeout: 40_000 });
      const notifications = await (await context()).newPage();
      await notifications.goto('/notifications/');
      await notifications.getByLabel('Delivered to').selectOption('maya');
      await expect(notifications.locator('.list li').first()).toBeVisible({ timeout: 40_000 });
      await notifications.close();
      await page.locator('.newpill button').click();
    });

    await test.step('with search and the feed off the screen says so, and stops saying so', async () => {
      await system.mode('search', 'off');
      try {
        await toChannels(page);
        const search = page.getByRole('searchbox', { name: 'Search messages' });
        await search.fill('pricing');
        await expect(page.locator('.sr-off')).toContainText('Search is unavailable', { timeout: 8000 });
        await search.fill('');
        await page.keyboard.press('Escape');
      } finally {
        await system.mode('search', 'running');
      }
      await toActivity(page);
      await system.mode('feed', 'off');
      try {
        await system.say('marco', '#general', `@Maya while away ${mark}`, ['maya']);
        await expect(page.locator('.top-notice')).toHaveText('Catching up', { timeout: 30_000 });
      } finally {
        await system.mode('feed', 'running');
      }
      await expect(page.locator('.top-notice')).toHaveCount(0, { timeout: 30_000 });
    });

    await test.step('Reset moves the open screen to the fresh organization', async () => {
      const res = await request.post('/demo/reset', { timeout: 150_000 });
      expect(res.ok()).toBeTruthy();
      const { org: fresh } = (await res.json()) as { org: string };
      await expect.poll(() => new URL(page.url()).searchParams.get('org'), { timeout: 20_000 }).toBe(fresh);
      await expect(page.locator('[data-testid=feed] article').first()).toBeVisible({ timeout: 20_000 });
    });
  });
});
