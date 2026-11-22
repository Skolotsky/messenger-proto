import { KEY, ago, busyFeed, entry, expect, row, test } from '../fixtures/door';
import { isPhone, start } from './helpers';

// Not a check: writes the pictures a person looks at. Runs only with CONTEXIA_SHOTS set.
const dir = process.env.CONTEXIA_SHOTS;

for (const theme of ['light', 'dark'] as const) {
  test(`Activity pictures in ${theme}`, async ({ page, door }, info) => {
    test.skip(!dir, 'set CONTEXIA_SHOTS to write the pictures');
    const shot = (name: string) => page.screenshot({ path: `${dir}/activity-${info.project.name}-${theme}-${name}.png` });
    await page.addInitScript((t) => localStorage.setItem('contexia-theme', t), theme);
    door.extraPeople = [{ id: 'maxi', name: 'Maximiliana Alexandra von Hohenzollern-Sigmaringen', is_agent: false, line: null, status: null }];
    door.rows = [
      ...busyFeed(),
      row({ conversation: '#general', thread: 'msg_long', message: 'msg_long', author: 'maxi', why: 'Maximiliana Alexandra von Hohenzollern-Sigmaringen mentioned you', preview: 'A long message about the rollout plan, which goes on for quite a while. '.repeat(8), newest: 1175, at: ago(0.6) }),
      row({ conversation: '#launch', thread: 'msg_gone', message: 'msg_gone', bucket: 'focus', unread: false, kept: 'you', state: 'retracted', preview: '', author: 'lena', why: 'Kept by you', newest: 1080, at: ago(28) }),
    ];
    door.history = {
      mentions: [
        entry({ message: 'msg_h1', conversation: '#launch', thread: 'msg_h1', unread: true, author: 'tomas', preview: "The export API can't ship both CSV and XLSX by Thursday." }),
        entry({ message: 'msg_h2', conversation: '#general', thread: 'msg_h2', author: 'priya', preview: 'Thanks for the copy review.', at: ago(26) }),
      ],
      threads: [entry({ message: 'msg_h3', conversation: '#general', thread: 'msg_h3', level: 'fyi', author: 'lena', preview: 'Trying 1.75 for the tab bar only.' })],
    };
    await start(page);
    await page.waitForLoadState('load');
    await expect(page.locator(`[data-row="${KEY('dm:anna:marco')}"]`)).toBeVisible();
    await shot('1-feed');

    await page.locator(`[data-row="${KEY('dm:anna:marco')}"] .why-btn`).click();
    await expect(page.getByRole('dialog', { name: /Why this is in/ })).toContainText('14 items');
    await shot('2-reason');
    await page.keyboard.press('Escape');

    door.arrive(row({ conversation: '#general', thread: 'msg_a', message: 'msg_a', preview: 'Arrived', newest: 3000 }));
    await expect(page.locator('.top .newpill')).toBeVisible();
    await shot('3-arrivals');

    const target = page.locator(`[data-row="${KEY('#launch', 'msg_n1')}"]`);
    await target.scrollIntoViewIfNeeded();
    if (!isPhone(info)) await target.hover();
    await target.getByRole('button', { name: 'React' }).click();
    await expect(page.getByRole('dialog', { name: 'Choose a reaction' })).toBeVisible();
    await shot('4-picker');
    await page.keyboard.press('Escape');

    await page.getByRole('navigation', { name: 'Activity views' }).getByRole('button', { name: /^Mentions/ }).click();
    await expect(page.locator('.hrow').first()).toBeVisible();
    await shot('5-history');

    if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Settings' }).click();
    else await page.locator('.top').getByRole('button', { name: 'Settings' }).click();
    const settings = page.getByRole('dialog', { name: 'Settings' });
    await expect(settings).toContainText('General rules');
    await shot('6-settings');
    await settings.getByRole('navigation').getByRole('button', { name: 'Focus people', exact: true }).click();
    await expect(settings).toContainText('Removed');
    await shot('7-people');
    await settings.getByRole('navigation').getByRole('button', { name: 'Channel levels', exact: true }).click();
    await shot('8-channels');
    await settings.getByRole('button', { name: 'Close settings' }).click();

    door.rows = [];
    await page.getByRole('navigation', { name: 'Activity views' }).getByRole('button', { name: /^Feed/ }).click();
    await expect(page.locator('.b-needs_you .empty')).toBeVisible();
    await shot('9-empty');
  });
}
