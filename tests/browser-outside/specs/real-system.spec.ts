import { expect, test } from '@playwright/test';

// The final acceptance of T-059 (D-033), on a running system: set CONTEXIA_REAL_URL to its one
// address, for example http://127.0.0.1:18080. Without it these tests are skipped.
const real = process.env.CONTEXIA_REAL_URL;

const orgOf = (url: string) => new URL(url).searchParams.get('org');

test.describe('the real system', () => {
  test.skip(!real, 'needs CONTEXIA_REAL_URL');
  test.use({ baseURL: real });
  test.setTimeout(180_000);

  test('Reset with a screen open lands it on the fresh organization, and cleanup removes the old one', async ({ context, page }, info) => {
    test.skip(info.project.name !== 'wide', 'one width is enough for a flow across pages');

    await page.goto('/');
    await expect.poll(() => orgOf(page.url()), { timeout: 150_000, message: 'the entry address opens the current organization' }).toMatch(/^larkmere-\d+$/);
    const before = orgOf(page.url())!;

    const panel = await context.newPage();
    await panel.goto('/control/');
    await expect(panel.locator('.org-now code')).toHaveText(before, { timeout: 30_000 });
    await panel.getByRole('button', { name: 'Reset', exact: true }).click();

    await expect.poll(() => orgOf(page.url()), { timeout: 150_000, message: 'the open screen moves by itself' }).not.toBe(before);
    const after = orgOf(page.url())!;
    await expect(panel.locator('.org-now code')).toHaveText(after, { timeout: 30_000 });

    const oldRow = panel.locator('tr, li').filter({ hasText: before }).filter({ hasText: 'Retired' }).first();
    await expect(oldRow).toBeVisible({ timeout: 10_000 });
    await expect(panel.getByRole('button', { name: `Delete ${after}` })).toHaveCount(0);

    await panel.getByRole('button', { name: 'Delete all retired' }).click();
    await panel.getByRole('alertdialog').getByRole('button', { name: /^Delete \d+$/ }).click();
    await expect(panel.locator('tr, li').filter({ hasText: before })).toHaveCount(0, { timeout: 60_000 });
    await expect(panel.locator('tr, li').filter({ hasText: after }).first()).toBeVisible();
  });
});
