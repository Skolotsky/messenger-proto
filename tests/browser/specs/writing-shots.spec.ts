import { expect, test } from '@playwright/test';
import { isPhone, openConversation, post, start, unique, write } from './helpers';
import { failNext, hit, searchAnswer } from './writing-fixtures';

// Not a check: writes the pictures a person looks at. Runs only with CONTEXIA_SHOTS set.
const dir = process.env.CONTEXIA_SHOTS;

for (const theme of ['light', 'dark'] as const) {
  test(`writing and search pictures in ${theme}`, async ({ page, request }, info) => {
    test.skip(!dir, 'set CONTEXIA_SHOTS to write the pictures');
    const shot = (name: string) => page.screenshot({ path: `${dir}/${info.project.name}-${theme}-w${name}.png` });
    await page.addInitScript((t) => localStorage.setItem('contexia-theme', t), theme);
    await post(request, 'anna', '#launch', 'Can someone sign off the pricing page before noon?');

    await start(page);
    const pane = await openConversation(page, info, 'launch');
    const box = pane.getByRole('textbox', { name: 'Message' });
    await box.pressSequentially('Thanks @');
    await expect(page.getByRole('listbox', { name: 'Mention someone' })).toBeVisible();
    await shot('1-mentions');
    await box.fill('');

    const text = unique('Flagged for @Anna, please check the second tab');
    await box.fill(text);
    await pane.getByRole('button', { name: 'Flag', exact: true }).click();
    await box.press('Control+Enter');
    await expect(box).toHaveValue('');
    const found = pane.locator('.msg.mine', { hasText: text });
    const msg = pane.locator(`[data-msg="${await found.getAttribute('data-msg')}"]`);
    await msg.locator('.msg-self').first().hover();
    await shot('2-own-tools');
    await msg.getByRole('button', { name: 'Edit' }).click();
    await shot('3-edit');
    await failNext(page, 'edit', { refused: 'message deleted' });
    await msg.getByRole('textbox', { name: 'Edit your message' }).fill(`${text} now`);
    await msg.getByRole('button', { name: 'Save' }).click();
    await expect(msg.getByRole('alert')).toBeVisible();
    await shot('4-refused');
    await msg.getByRole('button', { name: 'Discard' }).click();
    await msg.locator('.msg-self').first().hover();
    await msg.getByRole('button', { name: 'Delete' }).click();
    await shot('5-delete-asks');
    await msg.getByRole('button', { name: 'Cancel' }).click();
    await write(pane, unique('One more line'));
    if (isPhone(info)) await pane.getByRole('button', { name: 'Back' }).click();

    await page.route('**/orgs/acme/people/marco/search*', (route) =>
      route.fulfill({
        json: searchAnswer(
          {
            through: 9, total: 37, next: 'c2',
            results: [
              hit({ message: 'a', text: 'Can someone sign off the pricing page?', match: [[25, 32]], edited: true, version: 2 }),
              hit({ message: 'b', author: 'lena', thread: 'root', text: 'The pricing tab changed again.', match: [[4, 11]] }),
            ],
          },
          'Search is catching up. The last 3 minutes may be missing.',
        ),
      }),
    );
    if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
    await page.getByRole('searchbox', { name: 'Search messages' }).fill('pricing');
    await expect(page.getByTestId('search-total')).toBeVisible();
    await shot('6-search');
    await page.keyboard.press('Escape');

    await page.getByRole('navigation', { name: 'Conversations' }).getByRole('button', { name: 'Create a channel' }).click();
    await page.getByRole('dialog').getByRole('textbox', { name: 'Name' }).fill('launch plans');
    await shot('7-new-channel');
  });
}
