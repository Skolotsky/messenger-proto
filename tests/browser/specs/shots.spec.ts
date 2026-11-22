import { expect, test } from '@playwright/test';
import { isPhone, openConversation, post, start, unique } from './helpers';

// Not a check: writes the pictures a person looks at. Runs only with CONTEXIA_SHOTS set.
const dir = process.env.CONTEXIA_SHOTS;

for (const theme of ['light', 'dark'] as const) {
  test(`pictures in ${theme}`, async ({ page, request }, info) => {
    test.skip(!dir, 'set CONTEXIA_SHOTS to write the pictures');
    const shot = (name: string) => page.screenshot({ path: `${dir}/${info.project.name}-${theme}-${name}.png` });
    await page.addInitScript((t) => localStorage.setItem('contexia-theme', t), theme);

    const root = await post(request, 'anna', '#launch', `Can someone sign off the pricing page before ${unique('noon')}?`);
    await post(request, 'lena', '#launch', 'On it, I will look after lunch.', root);
    await post(request, 'priya', '#launch', 'The numbers on the second tab changed.\nPlease check them again.', root);

    await start(page);
    await shot('1-first');
    const pane = await openConversation(page, info, 'launch');
    await pane.locator(`[data-msg="${root}"]`).scrollIntoViewIfNeeded();
    await shot('2-channel');
    await pane.locator(`[data-msg="${root}"] .thread-link`).click();
    await expect(page.getByRole('region', { name: 'Thread' })).toContainText('second tab');
    await shot('3-thread');
    await page.getByRole('region', { name: 'Thread' }).getByRole('button', { name: isPhone(info) ? 'Back to the channel' : 'Close the thread' }).click();
    if (isPhone(info)) {
      await pane.getByRole('button', { name: 'Back' }).click();
      await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
      await shot('4-channels');
    }
    await page.getByRole('button', { name: 'Your account menu' }).click();
    await shot('5-menu');
    await page.locator('.me-pop').getByRole('button', { name: /Switch account/ }).click();
    await page.locator('.me-pop').getByRole('button', { name: 'Lena' }).click();
    await shot('6-asks');
    await page.getByRole('dialog').getByRole('button', { name: 'Switch' }).click();
    await expect(page.locator('.viewband')).toBeVisible();
    await shot('7-band');
  });
}
