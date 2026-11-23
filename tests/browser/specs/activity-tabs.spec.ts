import { expect, test } from '../fixtures/door';
import { isPhone, openConversation, post, settled, start, unique } from './helpers';

test('choosing a tab moves no tab, and the tabs do not scroll up and down', async ({ page }, info) => {
  test.skip(isPhone(info), 'on a phone the row of tabs scrolls sideways');
  await start(page);
  const tabs = page.getByRole('navigation', { name: 'Activity views' });
  const places = () => tabs.getByRole('button').evaluateAll((els) => els.map((el) => Math.round((el.getBoundingClientRect().left + el.parentElement!.scrollLeft) * 10) / 10));
  await expect(tabs.locator('.h-count.wait')).toHaveCount(0); // the counts have arrived
  const before = await places();
  for (const name of [/^Direct/, /^Mentions/, /^Threads/, /^Reactions/, /^Feed/]) {
    await tabs.getByRole('button', { name }).click();
    await settled(page);
    expect(await places(), String(name)).toEqual(before);
  }
  const box = await tabs.evaluate((el) => [el.scrollHeight, el.clientHeight, getComputedStyle(el).overflowY]);
  expect(box[0]).toBe(box[1]);
  expect(box[2]).toBe('hidden');
});

test('pressing one resize handle lights that handle only', async ({ page }, info) => {
  test.skip(isPhone(info), 'a phone has no resize handles');
  await start(page);
  const side = page.getByRole('separator', { name: 'Resize the sidebar' });
  const other = page.getByRole('separator', { name: 'Resize Activity' });
  const line = (el: typeof side) => el.evaluate((node) => getComputedStyle(node, '::after').backgroundColor);
  const box = (await side.boundingBox())!;
  await page.mouse.move(box.x + box.width / 2, box.y + 200);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width / 2 + 12, box.y + 200);
  await expect.poll(() => line(side)).not.toBe('rgba(0, 0, 0, 0)');
  await expect.poll(() => line(other)).toBe('rgba(0, 0, 0, 0)');
  await page.mouse.up();
});

test('a direct message offers no thread, because direct conversations have none', async ({ page, request }, info) => {
  const text = unique('Direct, no thread');
  const id = await post(request, 'anna', 'dm:anna:marco', text);
  await start(page);
  const pane = await openConversation(page, info, 'Anna');
  const message = pane.locator(`[data-msg="${id}"]`);
  if (isPhone(info)) await message.locator('.msg-self').tap();
  else await message.locator('.msg-self').hover();
  await expect(message.getByRole('button', { name: 'React' }).first()).toBeVisible();
  await expect(message.getByRole('button', { name: 'Reply in thread' })).toHaveCount(0);
});
