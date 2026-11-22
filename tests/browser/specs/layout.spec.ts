import { expect, test, type Page } from '@playwright/test';
import { expectNoSidewaysScroll, isPhone, openConversation, post, start, unique } from './helpers';

const widthOf = async (page: Page, selector: string) => (await page.locator(selector).first().boundingBox())!.width;

async function dragHandle(page: Page, panel: string, dx: number) {
  const box = (await page.locator(`[data-rz="${panel}"]`).boundingBox())!;
  const x = box.x + box.width / 2;
  const y = box.y + box.height / 2;
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x + dx / 2, y);
  await page.mouse.move(x + dx, y);
  await page.mouse.up();
}

test.describe('wide', () => {
  test.beforeEach(({}, info) => test.skip(isPhone(info), 'panels are resized on a wide screen'));

  test('a panel is resized by dragging the line, and a double click puts it back', async ({ page }) => {
    await start(page);
    expect(await widthOf(page, '.side')).toBe(250);
    await dragHandle(page, 'side', 60);
    expect(await widthOf(page, '.side')).toBe(310);
    await page.locator('[data-rz="side"]').dblclick();
    expect(await widthOf(page, '.side')).toBe(250);
  });

  test('the size is remembered', async ({ page }) => {
    await start(page);
    await dragHandle(page, 'side', 30);
    await page.reload();
    await expect(page.locator('.side')).toBeVisible();
    expect(await widthOf(page, '.side')).toBe(280);
  });

  test('the line can be moved from the keyboard', async ({ page }) => {
    await start(page);
    const handle = page.getByRole('separator', { name: 'Resize the sidebar' });
    await handle.focus();
    await handle.press('ArrowRight');
    expect(await widthOf(page, '.side')).toBe(266);
    await handle.press('ArrowLeft');
    expect(await widthOf(page, '.side')).toBe(250);
  });

  test('no drag can push a panel out of the window', async ({ page, request }, info) => {
    const root = await post(request, 'anna', '#general', unique('Root for limits'));
    await post(request, 'lena', '#general', unique('Reply for limits'), root);
    await start(page);
    const pane = await openConversation(page, info, 'general');
    await pane.locator(`[data-msg="${root}"] .thread-link`).click();
    await expect(page.locator('.tpane')).toBeVisible();
    for (const [panel, dx] of [['side', 900], ['act', 900], ['thread', -900]] as const) {
      await dragHandle(page, panel, dx);
      await expectNoSidewaysScroll(page);
      const edge = await page.locator('.tpane').evaluate((n) => n.getBoundingClientRect().right);
      expect(edge, `after dragging ${panel}, the thread ends inside the window`).toBeLessThanOrEqual(1280);
      expect(await widthOf(page, '.pane'), 'the channel keeps its smallest width').toBeGreaterThanOrEqual(299);
    }
  });

  test('Activity can be closed, and reopened from the sidebar', async ({ page }) => {
    await start(page);
    await expect(page.locator('.main')).toBeVisible();
    await page.getByRole('button', { name: 'Close Activity' }).click();
    await expect(page.locator('.main')).toBeHidden();
    const toggle = page.locator('.side-item');
    await expect(toggle).toHaveAttribute('aria-pressed', 'false');
    await toggle.click();
    await expect(page.locator('.main')).toBeVisible();
  });

  for (const width of [800, 1000, 1240]) {
    test(`a window ${width} px wide fits a channel and a thread`, async ({ page, request }, info) => {
      const root = await post(request, 'anna', '#general', unique('Root for width'));
      await post(request, 'lena', '#general', unique('Reply for width'), root);
      await page.setViewportSize({ width, height: 700 });
      await start(page);
      const pane = await openConversation(page, info, 'general');
      await pane.locator(`[data-msg="${root}"] .thread-link`).click();
      await expect(page.locator('.tpane')).toBeVisible();
      await expectNoSidewaysScroll(page);
      const right = await page.locator('.tpane').evaluate((n) => n.getBoundingClientRect().right);
      expect(right).toBeLessThanOrEqual(width);
    });
  }
});

test('nothing scrolls sideways with a very long word in a message', async ({ page, request }, info) => {
  const word = `${'x'.repeat(600)}${Date.now().toString(36)}`;
  await post(request, 'anna', '#general', word);
  await start(page);
  const pane = await openConversation(page, info, 'general');
  await expect(pane.locator('.msg-body', { hasText: word })).toBeVisible();
  await expectNoSidewaysScroll(page);
  const overflow = await pane.locator('.conv-body').evaluate((n) => n.scrollWidth - n.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});
