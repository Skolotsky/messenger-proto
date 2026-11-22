import { expect, test, type Page, type TestInfo } from '@playwright/test';
import { isPhone, start } from './helpers';
import { failNext, recordActions } from './writing-fixtures';

const sidebar = (page: Page) => page.getByRole('navigation', { name: 'Conversations' });

async function toChannels(page: Page, info: TestInfo) {
  if (isPhone(info)) await page.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
}

const fresh = () => `room-${Date.now().toString(36)}-${Math.floor(Math.random() * 1e6).toString(36)}`;

test('D-024: a channel is created with a name and people, and opens', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  await toChannels(page, info);
  await sidebar(page).getByRole('button', { name: 'Create a channel' }).click();
  const dialog = page.getByRole('dialog', { name: 'Create a channel' });
  await expect(dialog.getByRole('button', { name: 'Create', exact: true }), 'a name is needed').toBeDisabled();
  const name = fresh();
  await dialog.getByRole('textbox', { name: 'Name' }).fill(`#${name.toUpperCase().replace(/-/g, ' ')}`);
  await expect(dialog, 'it says what the name will be').toContainText(`#${name}`);
  await dialog.getByRole('textbox', { name: 'Find a person' }).fill('len');
  await dialog.getByRole('button', { name: 'Lena' }).click();
  await dialog.getByRole('textbox', { name: 'Find a person' }).fill('');
  await dialog.getByRole('button', { name: 'Anna' }).click();
  await dialog.getByRole('button', { name: 'Create', exact: true }).click();
  await expect(dialog).toHaveCount(0);
  const pane = page.getByRole('region', { name: 'Conversation' });
  await expect(pane.locator('.conv-title b')).toHaveText(`#${name}`);
  await expect(pane.locator('.conv-title small')).toHaveText('3 members');
  const sent = actions.of('create_channel').at(-1)!;
  expect([sent.name, [...sent.people].sort()]).toEqual([name, ['anna', 'lena']]);
  if (isPhone(info)) await pane.getByRole('button', { name: 'Back' }).click();
  await expect(sidebar(page).getByRole('button', { name, exact: true })).toHaveCount(1);
});

test('a taken channel name is refused in place, keeps what was typed, and offers no Try again', async ({ page }, info) => {
  await start(page);
  await toChannels(page, info);
  await sidebar(page).getByRole('button', { name: 'Create a channel' }).click();
  const dialog = page.getByRole('dialog', { name: 'Create a channel' });
  await dialog.getByRole('textbox', { name: 'Name' }).fill('general');
  await dialog.getByRole('button', { name: 'Create', exact: true }).click();
  await expect(dialog.getByRole('alert')).toContainText('Not created: name taken');
  await expect(dialog.getByRole('button', { name: 'Try again' })).toHaveCount(0);
  await expect(dialog.getByRole('textbox', { name: 'Name' })).toHaveValue('general');
  await dialog.getByRole('button', { name: 'Discard' }).click();
  await expect(dialog).toHaveCount(0);
});

test('creating a channel with no answer offers Try again, with the same action ID', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  await toChannels(page, info);
  await failNext(page, 'create_channel', 'down');
  await sidebar(page).getByRole('button', { name: 'Create a channel' }).click();
  const dialog = page.getByRole('dialog', { name: 'Create a channel' });
  const name = fresh();
  await dialog.getByRole('textbox', { name: 'Name' }).fill(name);
  await dialog.getByRole('button', { name: 'Create', exact: true }).click();
  await expect(dialog.getByRole('alert')).toContainText('no answer');
  await dialog.getByRole('button', { name: 'Try again' }).click();
  await expect(dialog).toHaveCount(0);
  const sent = actions.of('create_channel');
  expect(sent).toHaveLength(2);
  expect(sent[1].id).toBe(sent[0].id);
});

test('D-009: a new direct message opens, and a second start reaches the same conversation', async ({ page }, info) => {
  const actions = recordActions(page);
  await start(page);
  await toChannels(page, info);
  const open = async (who: string) => {
    await sidebar(page).getByRole('button', { name: 'New direct message' }).click();
    const dialog = page.getByRole('dialog', { name: 'New direct message' });
    await expect(dialog.getByRole('button', { name: /Marco/ }), 'not with oneself').toHaveCount(0);
    await dialog.getByRole('button', { name: who }).click();
    await expect(dialog).toHaveCount(0);
    const pane = page.getByRole('region', { name: 'Conversation' });
    await expect(pane.locator('.conv-title b')).toHaveText(who);
    await expect(pane.locator('.conv-title small')).toHaveText('Direct message');
    if (isPhone(info)) await pane.getByRole('button', { name: 'Back' }).click();
  };
  await open('Yuki');
  await open('Yuki');
  expect(actions.of('open_direct').map((a) => a.with)).toEqual(['yuki', 'yuki']);
  await expect(sidebar(page).getByRole('button', { name: 'Yuki' }), 'one conversation in the list').toHaveCount(1);
});

test('a direct message refused says so in place', async ({ page }, info) => {
  await start(page);
  await toChannels(page, info);
  await failNext(page, 'open_direct', { refused: 'unknown person' });
  await sidebar(page).getByRole('button', { name: 'New direct message' }).click();
  const dialog = page.getByRole('dialog', { name: 'New direct message' });
  await dialog.getByRole('button', { name: 'Priya' }).click();
  await expect(dialog.getByRole('alert')).toContainText('unknown person');
  await expect(dialog.getByRole('button', { name: 'Try again' })).toHaveCount(0);
  await dialog.getByRole('button', { name: 'Discard' }).click();
  await expect(dialog).toHaveCount(0);
});
