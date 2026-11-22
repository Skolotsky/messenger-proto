import { expect, test, type Page, type TestInfo } from '@playwright/test';
import { isPhone, openConversation, start, unique, write } from './helpers';

async function openMenu(page: Page, info: TestInfo) {
  if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
  await page.getByRole('button', { name: 'Your account menu' }).click();
  return page.locator('.me-pop');
}

test('the menu names the organization and the people are one level down', async ({ page }, info) => {
  await start(page);
  const menu = await openMenu(page, info);
  await expect(menu.locator('.me-org')).toContainText('Acme');
  await expect(menu.getByRole('button', { name: 'Lena' }), 'people are not on the first level').toHaveCount(0);
  await menu.getByRole('button', { name: /Switch account/ }).click();
  await expect(menu.getByRole('button', { name: 'Lena' })).toBeVisible();
  await expect(menu, 'an agent is not an account to switch to').not.toContainText('Release-agent');
  await menu.getByRole('button', { name: 'Switch account' }).click();
  await expect(menu.locator('.me-org')).toBeVisible();
});

test('switching asks first, then shows whose view it is, with a way back', async ({ page }, info) => {
  await start(page);
  const menu = await openMenu(page, info);
  await menu.getByRole('button', { name: /Switch account/ }).click();
  await menu.getByRole('button', { name: 'Lena' }).click();

  const dialog = page.getByRole('dialog', { name: 'Switch account' });
  await expect(dialog).toContainText('Switch to Lena?');
  await dialog.getByRole('button', { name: 'Cancel' }).click();
  await expect(page.locator('.viewband'), 'Cancel changes nothing').toHaveCount(0);

  const again = await openMenu(page, { ...info, project: { ...info.project, name: 'wide' } } as TestInfo);
  await again.getByRole('button', { name: /Switch account/ }).click();
  await again.getByRole('button', { name: 'Lena' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Switch' }).click();

  const band = page.locator('.viewband');
  await expect(band).toContainText('Viewing as Lena');
  await page.reload();
  await expect(band, 'the view survives a reload').toContainText('Viewing as Lena');

  // What is written now is written as Lena.
  const pane = await openConversation(page, info, 'general');
  const text = unique('Written as Lena');
  await write(pane, text);
  await expect(pane.locator('.msg', { hasText: text }).locator('.msg-head b')).toHaveText('Lena');

  await band.getByRole('button', { name: 'Back to Marco' }).click();
  await expect(band).toHaveCount(0);
});

test('the theme is chosen in the menu, remembered, and applied before the first paint', async ({ page }, info) => {
  await start(page);
  const menu = await openMenu(page, info);
  const bg = () => page.evaluate(() => getComputedStyle(document.body).backgroundColor);
  await menu.getByRole('button', { name: 'Light' }).click();
  await expect.poll(bg).toBe('rgb(255, 255, 255)');
  await menu.getByRole('button', { name: 'Dark' }).click();
  await expect.poll(bg).toBe('rgb(23, 24, 26)');

  // Reads the theme from the document's first moment, before any of the app's code has run.
  await page.addInitScript(() => {
    document.addEventListener('readystatechange', () => {
      if (document.readyState === 'interactive') {
        (window as unknown as { firstTheme: string }).firstTheme = document.documentElement.dataset.theme ?? 'none';
      }
    }, { once: true });
  });
  await page.reload();
  expect(await page.evaluate(() => (window as unknown as { firstTheme: string }).firstTheme)).toBe('dark');
  await expect.poll(bg).toBe('rgb(23, 24, 26)');
});

test('System follows the device', async ({ page }, info) => {
  await page.emulateMedia({ colorScheme: 'dark' });
  await start(page);
  const bg = () => page.evaluate(() => getComputedStyle(document.body).backgroundColor);
  await expect.poll(bg).toBe('rgb(23, 24, 26)');
  await page.emulateMedia({ colorScheme: 'light' });
  await expect.poll(bg).toBe('rgb(255, 255, 255)');
  const menu = await openMenu(page, info);
  await expect(menu.getByRole('button', { name: 'System' })).toHaveAttribute('aria-pressed', 'true');
});
