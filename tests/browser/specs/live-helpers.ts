import { expect, type APIRequestContext, type Page } from '@playwright/test';
import { control } from './helpers';

/** What a person can see that the live connection keeps current. Two screens that are equal say the same. */
export async function screenState(page: Page) {
  return page.evaluate(() => {
    const text = (el: Element | null) => el?.textContent?.trim() ?? '';
    const messages = [...document.querySelectorAll('section[aria-label="Conversation"] [data-msg]')]
      .filter((m) => !(m.classList.contains('deleted') && !m.querySelector('.thread-link')))
      .map((m) => ({
        id: m.getAttribute('data-msg'),
        body: text(m.querySelector('.msg-body')),
        edited: m.querySelector('.edited') !== null,
        reactions: [...m.querySelectorAll('.rx')].map((r) => r.getAttribute('aria-label')).sort(),
        replies: text(m.querySelector('.thread-link b')),
        repliesUnread: m.querySelector('.thread-link .dot') !== null,
      }));
    const unread = [...document.querySelectorAll('nav[aria-label="Conversations"] .chat.unread .chat-n')].map(text).sort();
    // A row already read stays on screen until the person moves on (D-003): only what is new is compared.
    const rows = [...document.querySelectorAll('[data-row].unread')]
      .map((r) => ({
        key: r.getAttribute('data-row'),
        section: r.closest('[aria-label]')?.getAttribute('aria-label'),
        preview: text(r.querySelector('.row-preview')),
        count: text(r.querySelector('.row-count')),
        reactions: [...r.querySelectorAll('.rx')].map((x) => x.getAttribute('aria-label')).sort(),
      }))
      .sort((a, b) => String(a.key).localeCompare(String(b.key)));
    const buckets = [...document.querySelectorAll('.bk-count')].map(text);
    return { messages, unread, rows, buckets };
  });
}

type State = Awaited<ReturnType<typeof screenState>>;

/** Says where two screens differ, so that a failure can be read. */
function firstDifference(a: State, b: State): string | null {
  for (const part of ['unread', 'buckets'] as const) {
    if (JSON.stringify(a[part]) !== JSON.stringify(b[part])) return `${part}: ${JSON.stringify(a[part])} / fresh ${JSON.stringify(b[part])}`;
  }
  for (const part of ['messages', 'rows'] as const) {
    const left = a[part] as { id?: unknown; key?: unknown }[];
    const right = b[part] as { id?: unknown; key?: unknown }[];
    for (let i = 0; i < Math.max(left.length, right.length); i++) {
      if (JSON.stringify(left[i]) !== JSON.stringify(right[i])) {
        return `${part}[${i}] of ${left.length}/${right.length}: ${JSON.stringify(left[i])} / fresh ${JSON.stringify(right[i])}`;
      }
    }
  }
  return null;
}

/** Opens the same person's screen afresh, at the same conversation, and compares it with this one. */
export async function expectEqualsFreshLoad(page: Page, name: string) {
  const show = page.locator('.newpill').getByRole('button', { name: 'Show' });
  const fresh = await page.context().newPage();
  try {
    await fresh.goto(page.url());
    await expect(fresh.locator('.app[data-live="on"]')).toBeVisible();
    const phone = (page.viewportSize()?.width ?? 1280) <= 760;
    const openIn = async (p: Page) => {
      if (phone) await p.getByRole('navigation', { name: 'Main' }).getByRole('button', { name: 'Channels' }).click();
      await p.getByRole('navigation', { name: 'Conversations' }).getByRole('button', { name }).first().click();
      await expect(p.getByRole('region', { name: 'Conversation' }).locator('[data-msg]').first()).toBeVisible();
    };
    await openIn(fresh);
    await expect
      .poll(
        async () => {
          // Arrivals wait behind Show by design (D-023): the person asks for them, then the two compare.
          if (await show.isVisible()) await show.click({ force: true });
          const [mine, theirs] = await Promise.all([screenState(page), screenState(fresh)]);
          return firstDifference(mine, theirs) ?? 'equal';
        },
        { message: 'the screen ends equal to a freshly loaded one', timeout: 8000 },
      )
      .toBe('equal');
  } finally {
    await fresh.close();
  }
}

export const live = (page: Page) => page.locator('.app[data-live="on"]');
export const offline = (page: Page) => page.locator('.app[data-live="off"]');

/** Every test leaves the door's going-wrong switches as it found them. */
export async function resetDoor(request: APIRequestContext) {
  await control(request, { down: false, refuseLive: false, hold: false, dropNext: 0, feedDown: false, restore: true });
  await control(request, { release: 'order' });
}
