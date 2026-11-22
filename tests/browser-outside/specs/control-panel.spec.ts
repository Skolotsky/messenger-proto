import { expect, test, type Page } from '@playwright/test';
import { createFixtureServer } from '../../../web/control-panel/src/fixtures';
import { LARKMERE_SITUATIONS } from '../../../web/control-panel/src/larkmere';
import { answerCalls, expectNoSidewaysScroll, shot } from './helpers';

const ORG = 'larkmere-7';

async function open(page: Page) {
  const server = createFixtureServer();
  const net = await answerCalls(page, server.handle);
  await page.goto('/panel/');
  await expect(page.getByTestId('script-state')).toHaveText('Running');
  await expect(page.getByTestId('org-larkmere-6')).toBeVisible();
  return { server, net };
}

const system = (page: Page, part: string) => page.getByTestId(`system-${part}`);

test.describe('calls', () => {
  test('pause and start call the script operation', async ({ page }) => {
    const { net } = await open(page);
    await page.getByRole('button', { name: 'Pause', exact: true }).click();
    await net.expectCall('POST', `demo/orgs/${ORG}/script`, { do: 'pause' });
    await expect(page.getByTestId('script-state')).toHaveText('Paused');
    await page.getByRole('button', { name: 'Start', exact: true }).click();
    await net.expectCall('POST', `demo/orgs/${ORG}/script`, { do: 'start' });
    await expect(page.getByTestId('script-state')).toHaveText('Running');
  });

  test('the pace calls the script operation with the chosen pace', async ({ page }) => {
    const { net } = await open(page);
    await page.getByLabel('Pace').selectOption('4');
    await net.expectCall('POST', `demo/orgs/${ORG}/script`, { do: 'pace', pace: 4 });
    await expect(page.getByLabel('Pace')).toHaveValue('4');
  });

  test('Play now plays that step and it moves to what happened', async ({ page }) => {
    const { net } = await open(page);
    const next = page.getByTestId('section-next');
    await expect(next).toContainText('turns a short question');
    await next.getByRole('button', { name: /Play now: Tomás/ }).click();
    await net.expectCall('POST', `demo/orgs/${ORG}/script`, { do: 'play_situation', situation: 'edited-to-another-length' });
    await expect(next).not.toContainText('turns a short question');
    await expect(page.getByTestId('section-happened')).toContainText('Tomás turns a short question into a long one');
  });

  test('every situation is offered and plays by its own id', async ({ page }) => {
    const { net } = await open(page);
    const section = page.getByTestId('section-situations');
    await expect(section.locator('button.situation')).toHaveCount(16);
    for (const s of [LARKMERE_SITUATIONS[0], LARKMERE_SITUATIONS[15]]) {
      await section.getByRole('button', { name: new RegExp(`^${s.title}`) }).click();
      await expect.poll(() => net.writes().filter((c) => (c.body as { situation?: string })?.situation === s.id).length).toBe(1);
    }
  });

  for (const [part, label] of [['search', 'Search index'], ['sender', 'Notification sender'], ['feed', 'Activity feed']]) {
    test(`the ${part} switch sets each mode on that part and shows what waits`, async ({ page }) => {
      const { net } = await open(page);
      const box = system(page, part);
      const path = `${part}/orgs/${ORG}/control/mode`;
      await box.getByRole('button', { name: 'Off' }).click();
      await net.expectCall('POST', path, { mode: 'off' });
      await expect(box.getByRole('button', { name: 'Off' })).toHaveAttribute('aria-pressed', 'true');
      await expect(page.getByTestId(`readout-${part}`)).toContainText('Down · 14 waiting');
      await box.getByRole('button', { name: 'Slow' }).click();
      await net.expectCall('POST', path, { mode: 'slow' });
      await expect(page.getByTestId(`readout-${part}`)).toContainText('Slow · 3 waiting');
      await box.getByRole('button', { name: 'Running' }).click();
      await net.expectCall('POST', path, { mode: 'running' });
      await expect(box.getByRole('button', { name: 'Running' })).toHaveAttribute('aria-pressed', 'true');
      const others = net.writes().filter((c) => c.path.endsWith('/control/mode') && c.path !== path);
      expect(others, `${label} must not switch another part`).toEqual([]);
    });

    test(`Rebuild of ${part} names that receiver`, async ({ page }) => {
      const { net } = await open(page);
      await page.getByRole('button', { name: `Rebuild ${label}` }).click();
      await net.expectCall('POST', `demo/orgs/${ORG}/rebuild`, { receiver: part, rules_changed: false });
    });
  }

  test('Try again retries that parked change on its receiver', async ({ page }) => {
    const { net } = await open(page);
    await expect(page.getByTestId('parked-search-4830')).toContainText('refused after 3 tries');
    await page.getByRole('button', { name: 'Try again: search change 4830' }).click();
    await net.expectCall('POST', `delivery/orgs/${ORG}/parked/4830/retry?receiver=search`);
    await expect(page.getByTestId('parked-search-4830')).toHaveCount(0);
    await expect(page.getByTestId('section-parked')).toContainText('Nothing is parked');
  });
});

test.describe('reset', () => {
  test('shows its progress from the real answers, then lands on the fresh organization', async ({ page }) => {
    const { server, net } = await open(page);
    net.fail.set('POST demo/reset', 'hold');
    await page.getByRole('button', { name: 'Reset', exact: true }).click();
    const progress = page.getByTestId('reset-progress');
    await expect(progress).toContainText('Seeding a fresh organization');
    await expect(progress).toContainText('larkmere-7 keeps running');
    await expect(page.getByRole('button', { name: 'Pause' })).toBeDisabled();

    server.set({
      organizations: [
        { org: 'larkmere-8', name: 'Larkmere', state: 'preparing', created_at: '2026-11-17T10:42:30.000Z', approx_changes: 1240 },
        { org: ORG, name: 'Larkmere', state: 'current', created_at: '2026-11-17T10:02:00.000Z', approx_changes: 8124 },
      ],
      waiting: { search: 200, feed: 40 },
    });
    await expect(progress).toContainText('Seeding larkmere-8: 1,240 changes so far');
    await expect(progress).toContainText('Waiting for search and the feed: 240 changes to go');
    await expect(page.getByTestId('org-larkmere-8')).toContainText('Preparing');
    await expect(page.getByTestId('org-larkmere-8').getByRole('button')).toHaveCount(0);

    await net.release();
    await expect(progress).toHaveCount(0);
    await expect(page.locator('.org-now')).toContainText('larkmere-8');
    await expect(page.getByTestId('org-larkmere-8')).toContainText('Current');
    await expect(page.getByTestId(`org-${ORG}`)).toContainText('Retired');
  });
});

test.describe('organizations', () => {
  test('lists name, status, created and approximate size', async ({ page }) => {
    await open(page);
    const row = page.getByTestId('org-larkmere-6');
    await expect(row).toContainText('Larkmere');
    await expect(row).toContainText('Retired');
    await expect(row).toContainText('17 Nov');
    await expect(row).toContainText('about 7,956 changes');
  });

  test('Delete asks first: cancel calls nothing, confirming deletes that one', async ({ page }) => {
    const { net } = await open(page);
    const row = page.getByTestId('org-larkmere-6');
    await row.getByRole('button', { name: 'Delete larkmere-6' }).click();
    await expect(row.getByRole('alertdialog')).toContainText('cannot be undone');
    expect(net.writes()).toEqual([]);
    await row.getByRole('button', { name: 'Cancel' }).click();
    await expect(row.getByRole('alertdialog')).toHaveCount(0);
    expect(net.writes()).toEqual([]);

    await row.getByRole('button', { name: 'Delete larkmere-6' }).click();
    await row.getByRole('alertdialog').getByRole('button', { name: 'Delete larkmere-6' }).click();
    await net.expectCall('DELETE', 'demo/organizations/larkmere-6');
    await expect(row).toHaveCount(0);
    await expect(page.getByTestId('org-larkmere-5')).toBeVisible();
  });

  test('the current organization offers no Delete', async ({ page }) => {
    await open(page);
    const row = page.getByTestId(`org-${ORG}`);
    await expect(row).toContainText('Current');
    await expect(row).toContainText('Cannot be deleted');
    await expect(row.getByRole('button')).toHaveCount(0);
  });

  test('a server refusal for the current organization shows in its row', async ({ page }) => {
    const { net } = await open(page);
    net.fail.set('DELETE demo/organizations/larkmere-6', { status: 409, body: { refused: 'current' } });
    const row = page.getByTestId('org-larkmere-6');
    await row.getByRole('button', { name: 'Delete larkmere-6' }).click();
    await row.getByRole('alertdialog').getByRole('button', { name: 'Delete larkmere-6' }).click();
    const failure = page.getByTestId('failure-delete-larkmere-6');
    await expect(failure).toHaveText('Refused: this is the current organization, so it cannot be deleted.');
    await expect(row).toBeVisible();
  });

  test('Delete all retired asks first and keeps the current one', async ({ page }) => {
    const { net } = await open(page);
    await page.getByRole('button', { name: 'Delete all retired' }).click();
    const ask = page.getByRole('alertdialog', { name: 'Delete all retired organizations' });
    await expect(ask).toContainText('Delete all 2 retired');
    expect(net.writes()).toEqual([]);
    await ask.getByRole('button', { name: 'Cancel' }).click();
    expect(net.writes()).toEqual([]);

    await page.getByRole('button', { name: 'Delete all retired' }).click();
    await ask.getByRole('button', { name: 'Delete 2' }).click();
    await net.expectCall('POST', 'demo/organizations/delete-retired');
    await expect(page.getByTestId('org-larkmere-6')).toHaveCount(0);
    await expect(page.getByTestId('org-larkmere-5')).toHaveCount(0);
    await expect(page.getByTestId(`org-${ORG}`)).toBeVisible();
    await expect(page.getByRole('button', { name: 'Delete all retired' })).toBeDisabled();
  });
});

test.describe('failures show in place', () => {
  const broken = { status: 503, body: { error: 'not available' } };

  test('start or pause', async ({ page }) => {
    const { net } = await open(page);
    net.fail.set(`POST demo/orgs/${ORG}/script`, broken);
    await page.getByRole('button', { name: 'Pause', exact: true }).click();
    await expect(page.getByTestId('failure-toggle')).toHaveText('Pause failed: not available.');
    await expect(page.getByTestId('script-state')).toHaveText('Running');
  });

  test('the pace', async ({ page }) => {
    const { net } = await open(page);
    net.fail.set(`POST demo/orgs/${ORG}/script`, broken);
    await page.getByLabel('Pace').selectOption('2');
    await expect(page.getByTestId('failure-pace')).toContainText('Change the pace failed');
    await expect(page.getByLabel('Pace')).toHaveValue('1');
  });

  test('Reset', async ({ page }) => {
    const { net } = await open(page);
    net.fail.set('POST demo/reset', 'no-answer');
    await page.getByRole('button', { name: 'Reset', exact: true }).click();
    await expect(page.getByTestId('failure-reset')).toContainText('Reset: no answer');
    await expect(page.getByTestId('reset-progress')).toHaveCount(0);
    await expect(page.locator('.org-now')).toContainText(ORG);
  });

  test('Play now, inside its own row', async ({ page }) => {
    const { net } = await open(page);
    net.fail.set(`POST demo/orgs/${ORG}/script`, broken);
    await page.getByRole('button', { name: /Play now: Marco/ }).click();
    const failure = page.getByTestId('failure-next-direct-mention');
    await expect(failure).toContainText('Play now failed');
    await expect(page.locator('.steps > li', { has: failure })).toContainText('Marco');
  });

  test('a situation, under its button', async ({ page }) => {
    const { net } = await open(page);
    net.fail.set(`POST demo/orgs/${ORG}/script`, { status: 404, body: { error: 'no such situation' } });
    const s = LARKMERE_SITUATIONS[3];
    await page.getByTestId('section-situations').getByRole('button', { name: new RegExp(`^${s.title}`) }).click();
    await expect(page.getByTestId(`failure-situation-${s.id}`)).toHaveText('Play the situation failed: no such situation.');
  });

  test('a mode switch, in that system, leaving the switch as it was', async ({ page }) => {
    const { net } = await open(page);
    net.fail.set(`POST search/orgs/${ORG}/control/mode`, 'no-answer');
    const box = system(page, 'search');
    await box.getByRole('button', { name: 'Off' }).click();
    await expect(box.getByTestId('failure-mode-search')).toContainText('no answer');
    await expect(box.getByRole('button', { name: 'Running' })).toHaveAttribute('aria-pressed', 'true');
    await expect(system(page, 'feed').getByRole('alert')).toHaveCount(0);
  });

  test('Rebuild, in that system', async ({ page }) => {
    const { net } = await open(page);
    net.fail.set(`POST demo/orgs/${ORG}/rebuild`, broken);
    await page.getByRole('button', { name: 'Rebuild Activity feed' }).click();
    await expect(system(page, 'feed').getByTestId('failure-rebuild-feed')).toContainText('Rebuild Activity feed failed');
  });

  test('Try again, in its row, and the change stays parked', async ({ page }) => {
    const { net } = await open(page);
    net.fail.set(`POST delivery/orgs/${ORG}/parked/4830/retry?receiver=search`, broken);
    await page.getByRole('button', { name: 'Try again: search change 4830' }).click();
    await expect(page.getByTestId('parked-search-4830').getByRole('alert')).toContainText('Try again failed');
    await expect(page.getByTestId('parked-search-4830')).toBeVisible();
  });

  test('Delete, saying that it can be repeated', async ({ page }) => {
    const { net } = await open(page);
    net.fail.set('DELETE demo/organizations/larkmere-5', broken);
    const row = page.getByTestId('org-larkmere-5');
    await row.getByRole('button', { name: 'Delete larkmere-5' }).click();
    await row.getByRole('alertdialog').getByRole('button', { name: 'Delete larkmere-5' }).click();
    await expect(page.getByTestId('failure-delete-larkmere-5')).toContainText('Deleting again is safe.');
  });

  test('Delete all retired', async ({ page }) => {
    const { net } = await open(page);
    net.fail.set('POST demo/organizations/delete-retired', broken);
    await page.getByRole('button', { name: 'Delete all retired' }).click();
    await page.getByRole('alertdialog').getByRole('button', { name: 'Delete 2' }).click();
    await expect(page.getByTestId('failure-delete-retired')).toContainText('Deleting again is safe.');
    await expect(page.getByTestId('org-larkmere-6')).toBeVisible();
  });

  test('a part that stops answering is shown as down, with what waits', async ({ page }) => {
    const { server, net } = await open(page);
    server.set({ waiting: { feed: 21 } });
    net.fail.set(`GET feed/orgs/${ORG}/control/mode`, 'no-answer');
    await expect(page.getByTestId('readout-feed')).toHaveText('Down · 21 waiting');
    await expect(page.getByTestId('failure-read-feed')).toBeVisible();
    await expect(page.getByTestId('readout-search')).toContainText('waiting');
    await expect(system(page, 'search').getByTestId('failure-read-search')).toHaveCount(0);
  });

  test('delivery that stops answering keeps the last counts and says so', async ({ page }) => {
    const { net } = await open(page);
    await expect(page.getByTestId('readout-feed')).toContainText('0 waiting');
    net.fail.set(`GET delivery/orgs/${ORG}/status`, 'no-answer');
    await expect(page.getByTestId('failure-delivery')).toContainText('last ones seen');
  });

  test('the demo program not answering at the start is said plainly', async ({ page }) => {
    const server = createFixtureServer();
    const net = await answerCalls(page, server.handle);
    net.fail.set('GET demo/current', 'no-answer');
    await page.goto('/panel/');
    await expect(page.getByTestId('failure-demo')).toContainText('not answering');
    await expect(page.getByRole('button', { name: 'Reset', exact: true })).toBeDisabled();
  });
});

test('fits the width, in both themes', async ({ page }, info) => {
  const { server } = await open(page);
  server.set({ waiting: { search: 14, feed: 3 } });
  await expect(page.getByTestId('readout-search')).toContainText('14');
  await expectNoSidewaysScroll(page);
  await page.getByRole('button', { name: 'Delete all retired' }).click();
  await expectNoSidewaysScroll(page);
  await shot(page, 'panel', info.project.name);
  for (const theme of ['light', 'dark']) {
    await page.getByLabel('Theme').selectOption(theme);
    await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
    const background = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    expect(background).toBe(theme === 'dark' ? 'rgb(20, 21, 22)' : 'rgb(244, 244, 241)');
    await expectNoSidewaysScroll(page);
  }
});

test('the default build runs from its fixtures with no server at all', async ({ page }) => {
  const outside: string[] = [];
  page.on('request', (r) => { if (new URL(r.url()).pathname.startsWith('/ops/')) outside.push(r.url()); });
  await page.goto('/panel-fixtures/');
  await expect(page.getByTestId('section-situations').locator('button.situation')).toHaveCount(16);
  await expect(page.getByTestId('section-next')).toContainText('Marco');
  await page.getByRole('button', { name: 'Pause', exact: true }).click();
  await expect(page.getByTestId('script-state')).toHaveText('Paused');
  expect(outside).toEqual([]);
});
