import { expect, test } from '../fixtures/door';
import { isPhone, openConversation, start } from './helpers';

test('the send hint opens the setting for what Enter does', async ({ page }, info) => {
  test.skip(isPhone(info), 'a phone has no Enter key hint');
  await start(page);
  const pane = await openConversation(page, info, 'launch');
  await pane.getByRole('button', { name: /Enter to send/ }).click();
  const settings = page.getByRole('dialog', { name: 'Settings' });
  await expect(settings.getByRole('radiogroup', { name: 'What Enter does' })).toBeVisible();
});
