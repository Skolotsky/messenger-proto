import { expect, test, type Page, type TestInfo } from '@playwright/test';
import { doorAct, isPhone, post, start, unique } from './helpers';
import { hit, messagesPage, recordActions, row, schemaProblems, searchAnswer } from './writing-fixtures';

async function box(page: Page, info: TestInfo) {
  if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
  return page.getByRole('searchbox', { name: 'Search messages' });
}
const results = (page: Page) => page.getByRole('region', { name: 'Search results' });

async function answerWith(page: Page, answer: (url: URL) => unknown) {
  await page.route('**/orgs/acme/people/marco/search*', (route) =>
    route.fulfill({ json: answer(new URL(route.request().url())) }),
  );
}

test('the fixtures are held to the contract: a broken answer is noticed', () => {
  expect(schemaProblems('ScreenSearchAnswer', { answer: null, notice: null })).toEqual([]);
  const broken = { answer: { through: 1, total: 1, next: null, results: [{ ...hit({}), match: 'x' }] }, notice: null };
  expect(schemaProblems('ScreenSearchAnswer', broken).length).toBeGreaterThan(0);
  expect(schemaProblems('ScreenSearchAnswer', { answer: null }).length).toBeGreaterThan(0);
});

test('T-038.20: the count is the true total, and paging brings the rest', async ({ page }, info) => {
  const asked: (string | null)[] = [];
  await answerWith(page, (url) => {
    const after = url.searchParams.get('after');
    asked.push(after);
    return after
      ? searchAnswer({ through: 9, total: 37, next: null, results: [hit({ message: 'm3', text: 'third pricing', match: [[6, 13]] })] })
      : searchAnswer({
          through: 9, total: 37, next: 'cursor 2',
          results: [hit({ message: 'm1', text: 'first pricing', match: [[6, 13]] }), hit({ message: 'm2', text: 'second pricing', match: [[7, 14]] })],
        });
  });
  await start(page);
  await (await box(page, info)).fill('pricing');
  await expect(page.getByTestId('search-total')).toHaveText('37 messages');
  await expect(results(page).locator('[data-hit-id]')).toHaveCount(2);
  await results(page).getByRole('button', { name: 'Show more (35 left)' }).click();
  await expect(results(page).locator('[data-hit-id]')).toHaveCount(3);
  await expect(page.getByTestId('search-total'), 'still the total, not what is shown').toHaveText('37 messages');
  expect(asked).toEqual([null, 'cursor 2']);
  await expect(results(page).getByRole('button', { name: /Show more/ }), 'nothing more to bring').toHaveCount(0);
});

test('paging that fails says so, keeps the results and can be tried again', async ({ page }, info) => {
  let failing = true;
  await page.route('**/orgs/acme/people/marco/search*', (route) => {
    const after = new URL(route.request().url()).searchParams.get('after');
    if (after && failing) {
      failing = false;
      return route.fulfill({ status: 503, json: {} });
    }
    return route.fulfill({
      json: after
        ? searchAnswer({ through: 9, total: 2, next: null, results: [hit({ message: 'm2' })] })
        : searchAnswer({ through: 9, total: 2, next: 'c2', results: [hit({ message: 'm1' })] }),
    });
  });
  await start(page);
  await (await box(page, info)).fill('pricing');
  await results(page).getByRole('button', { name: /Show more/ }).click();
  await expect(results(page).getByRole('alert')).toHaveText('Could not load more.');
  await expect(results(page).locator('[data-hit-id]')).toHaveCount(1);
  await results(page).getByRole('button', { name: 'Try again' }).click();
  await expect(results(page).locator('[data-hit-id]')).toHaveCount(2);
});

test('match marks count code points, "edited" shows, and the rule is stated', async ({ page }, info) => {
  const text = '🎉🎉 ship the pricing page';
  await answerWith(page, () =>
    searchAnswer({ through: 9, total: 1, next: null, results: [hit({ text, edited: true, version: 2, match: [[3, 7], [12, 19]] })] }),
  );
  await start(page);
  await (await box(page, info)).fill('pricing');
  const entry = results(page).locator('[data-hit-id]');
  await expect(entry.locator('mark')).toHaveText(['ship', 'pricing']);
  await expect(entry.locator('.sr-text')).toHaveText(text);
  await expect(entry.locator('small')).toContainText('edited');
  await expect(results(page)).toContainText('An edited message is found by its new text. A deleted message is not found.');
});

test('in a long message the match is in what is shown', async ({ page }, info) => {
  const text = `${'word '.repeat(200)}needle ${'word '.repeat(200)}`;
  const from = text.indexOf('needle');
  await answerWith(page, () =>
    searchAnswer({ through: 9, total: 1, next: null, results: [hit({ text, match: [[from, from + 6]] })] }),
  );
  await start(page);
  await (await box(page, info)).fill('needle');
  await expect(results(page).locator('mark')).toHaveText('needle');
  const shown = await results(page).locator('.sr-text').innerText();
  expect(shown.length).toBeLessThan(200);
  expect(shown.startsWith('…') && shown.endsWith('…')).toBeTruthy();
});

test('a message is shown as text in the results, marks included', async ({ page }, info) => {
  const text = '<img src=x onerror="window.__hit=1"> pricing';
  await answerWith(page, () =>
    searchAnswer({ through: 9, total: 1, next: null, results: [hit({ text, match: [[5, 8]] })] }),
  );
  await start(page);
  await (await box(page, info)).fill('pricing');
  await expect(results(page).locator('.sr-text')).toHaveText(text);
  await expect(results(page).locator('img')).toHaveCount(0);
  expect(await page.evaluate(() => (window as unknown as { __hit?: number }).__hit)).toBeUndefined();
});

test('search that is unavailable says so in place and claims no results', async ({ page }, info) => {
  await answerWith(page, () => searchAnswer(null, 'Search is unavailable right now. Your messages are safe.'));
  await start(page);
  await (await box(page, info)).fill('pricing');
  await expect(results(page).getByRole('status')).toHaveText('Search is unavailable right now. Your messages are safe.');
  await expect(page.getByTestId('search-total')).toHaveCount(0);
  await expect(results(page)).not.toContainText('Nothing found');
});

test('search with no answer at all says the same', async ({ page }, info) => {
  await page.route('**/orgs/acme/people/marco/search*', (route) => route.fulfill({ status: 503, json: {} }));
  await start(page);
  await (await box(page, info)).fill('pricing');
  await expect(results(page).getByRole('status')).toContainText('unavailable');
  await expect(page.getByTestId('search-total')).toHaveCount(0);
});

test('search that is behind shows the results and says what may be missing', async ({ page }, info) => {
  const notice = 'Search is catching up. The last 3 minutes may be missing.';
  await answerWith(page, () => searchAnswer({ through: 9, total: 1, next: null, results: [hit({})] }, notice));
  await start(page);
  await (await box(page, info)).fill('pricing');
  await expect(results(page).locator('.lag')).toHaveText(notice);
  await expect(page.getByTestId('search-total')).toHaveText('1 message');
  await expect(results(page).locator('[data-hit-id]')).toHaveCount(1);
});

test('an answer to an older question never replaces a newer one', async ({ page }, info) => {
  let releaseOld!: () => void;
  const held = new Promise<void>((done) => (releaseOld = done));
  await page.route('**/orgs/acme/people/marco/search*', async (route) => {
    const q = new URL(route.request().url()).searchParams.get('q');
    if (q === 'old') await held;
    await route.fulfill({
      json: searchAnswer({ through: 9, total: 1, next: null, results: [hit({ message: `m-${q}`, text: `about ${q}`, match: [[6, 9]] })] }),
    });
  });
  await start(page);
  const input = await box(page, info);
  const oldAsked = page.waitForRequest((r) => r.url().includes('q=old'));
  await input.fill('old');
  await oldAsked;
  await input.fill('new');
  await expect(results(page).locator('[data-hit-id="m-new"]')).toBeVisible();
  const oldAnswered = page.waitForResponse((r) => r.url().includes('q=old'));
  releaseOld();
  await oldAnswered;
  await page.evaluate(() => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done))));
  await expect(results(page).locator('[data-hit-id="m-new"]')).toBeVisible();
  await expect(results(page).locator('[data-hit-id="m-old"]')).toHaveCount(0);
});

test('T-038.21: found by the real door, in a channel never opened; edited by new text; deleted not found', async ({ page, request }, info) => {
  const word = `zq${Date.now().toString(36)}`;
  const id = await post(request, 'marco', '#incidents', `before ${word}`);
  await start(page);
  const input = await box(page, info);
  await input.fill(word);
  const entry = results(page).locator(`[data-hit-id="${id}"]`);
  await expect(entry.locator('mark')).toHaveText(word);
  await expect(page.getByTestId('search-total')).toHaveText('1 message');

  const changed = `${word}x`;
  await doorAct(request, { as: 'marco', kind: 'edit', message: id, text: `after ${changed}`, mentions: [], level: null });
  await input.fill(changed);
  await expect(results(page).locator(`[data-hit-id="${id}"] small`)).toContainText('edited');
  await input.fill(`${word}-gone`);
  await expect(results(page)).toContainText('Nothing found');

  await doorAct(request, { as: 'marco', kind: 'delete', message: id });
  await input.fill(changed);
  await expect(page.getByTestId('search-total')).toHaveText('0 messages');
  await expect(results(page)).toContainText('Nothing found');
});

test('T-038.21: a hit in a channel the person is not in opens that channel', async ({ page, request }, info) => {
  const name = `outside-${Date.now().toString(36)}`;
  await doorAct(request, { as: 'yuki', kind: 'create_channel', name, people: ['anna'] });
  const word = `zr${Date.now().toString(36)}`;
  const id = await post(request, 'yuki', `#${name}`, `only here ${word}`);
  await start(page);
  const input = await box(page, info);
  await expect(page.getByRole('navigation', { name: 'Conversations' }).getByRole('button', { name })).toHaveCount(0);
  await input.fill(word);
  await results(page).locator(`[data-hit-id="${id}"]`).click();
  const pane = page.getByRole('region', { name: 'Conversation' });
  await expect(pane.locator('.conv-title b')).toHaveText(`#${name}`);
  await expect(pane.locator(`[data-msg="${id}"]`)).toBeInViewport();
});

test('T-038.22: a hit on a reply opens that thread at that reply', async ({ page, request }, info) => {
  const word = `zs${Date.now().toString(36)}`;
  const root = await post(request, 'anna', '#general', unique('The root'));
  for (let i = 0; i < 30; i++) await post(request, 'anna', '#general', `Reply ${i} of many`, root);
  const reply = await post(request, 'lena', '#general', `the one ${word}`, root);
  for (let i = 0; i < 30; i++) await post(request, 'anna', '#general', `Later reply ${i}`, root);
  await start(page);
  await (await box(page, info)).fill(word);
  await expect(results(page).locator(`[data-hit-id="${reply}"] small`)).toContainText('in a thread');
  await results(page).locator(`[data-hit-id="${reply}"]`).click();
  await expect(results(page)).toHaveCount(0);
  const thread = page.getByRole('region', { name: 'Thread' });
  const target = thread.locator(`[data-msg="${reply}"]`);
  await expect(target).toBeInViewport({ ratio: 1 });
  await expect(target).toHaveAttribute('data-hit', '');
  await expect(thread.locator(`[data-msg="${root}"]`), 'it is that thread').toHaveCount(1);
});

test('a hit in an older page opens around it, reads nothing past it, and "Go to the latest" comes back', async ({ page }, info) => {
  const actions = recordActions(page);
  const older = Array.from({ length: 41 }, (_, i) => row(`old${i}`, 100 + i));
  const latest = Array.from({ length: 41 }, (_, i) => row(`new${i}`, 400 + i));
  const arounds: (string | null)[] = [];
  await page.route('**/orgs/acme/conversations/*/messages*', (route) => {
    const around = new URL(route.request().url()).searchParams.get('around');
    arounds.push(around);
    return route.fulfill({
      json: around ? messagesPage(older, { has_before: true, has_after: true }) : messagesPage(latest, { has_before: true }),
    });
  });
  await answerWith(page, () =>
    searchAnswer({ through: 9, total: 1, next: null, results: [hit({ message: 'old20', number: 120, text: 'the pricing page', match: [[4, 11]] })] }),
  );
  await start(page);
  await (await box(page, info)).fill('pricing');
  await results(page).locator('[data-hit-id="old20"]').click();
  const pane = page.getByRole('region', { name: 'Conversation' });
  const target = pane.locator('[data-msg="old20"]');
  await expect(target).toBeInViewport({ ratio: 1 });
  await expect(target).toHaveAttribute('data-hit', '');
  expect(arounds).toContain('120');
  await expect(pane.getByRole('status')).toContainText('older messages');
  expect(actions.of('read'), 'nothing is marked read from a page that does not reach the newest').toEqual([]);

  await pane.getByRole('button', { name: 'Go to the latest' }).click();
  await expect(pane.locator('[data-msg="new40"]')).toBeVisible();
  await expect(pane.locator('[data-msg="old20"]'), 'no gap stays in the list').toHaveCount(0);
  await expect(pane.getByRole('button', { name: 'Go to the latest' })).toHaveCount(0);
  await expect.poll(() => actions.of('read').at(-1)?.up_to).toBe(440);
});

test('a hit whose message has gone says so', async ({ page }, info) => {
  await page.route('**/orgs/acme/conversations/*/messages*', (route) =>
    route.fulfill({ json: messagesPage([row('other', 90)]) }),
  );
  await answerWith(page, () =>
    searchAnswer({ through: 9, total: 1, next: null, results: [hit({ message: 'vanished', number: 91 })] }),
  );
  await start(page);
  await (await box(page, info)).fill('pricing');
  await results(page).locator('[data-hit-id="vanished"]').click();
  await expect(page.getByRole('region', { name: 'Conversation' }).getByRole('status')).toContainText('no longer here');
});
