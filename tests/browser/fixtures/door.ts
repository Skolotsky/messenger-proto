import { readFileSync } from 'node:fs';
import { test as base, expect, type Page, type WebSocketRoute } from '@playwright/test';
import Ajv2020 from 'ajv/dist/2020.js';
import type {
  ConversationPart,
  FeedCounts,
  FeedPart,
  FeedRow,
  HistoryAnswer,
  HistoryEntry,
  HistoryTab,
  PeopleAction,
  PreviewAnswer,
  ScreenFeedAnswer,
  SettingsAnswer,
} from '../../../contracts/generated/types';

/**
 * The feed's answers for states the mock door cannot produce: kept, flagged, changed, retracted,
 * fenced and grouped rows, explanations with a change, settings. Every answer the screen is given
 * and every action it sends is validated against the contract's schema.
 */

const schema = JSON.parse(readFileSync(new URL('../../../contracts/schema/contexia.json', import.meta.url), 'utf8'));
const Ajv = (Ajv2020 as unknown as { default?: typeof Ajv2020 }).default ?? Ajv2020;
const ajv = new Ajv({ strict: false, allErrors: true, validateFormats: false });
ajv.addSchema(schema);

export function problems(name: string, value: unknown): string[] {
  const check = ajv.getSchema(`${schema.$id}#/$defs/${name}`);
  if (!check) throw new Error(`No schema named ${name}`);
  return check(value) ? [] : (check.errors ?? []).map((e) => `${name}${e.instancePath} ${e.message}`);
}

const HOUR = 3_600_000;
export const NOW = Date.now();
export const ago = (hours: number) => new Date(NOW - hours * HOUR).toISOString();

export const KEY = (conversation: string, thread: string | null = null) => `${conversation}|${thread ?? ''}`;

const defaultExplanation: FeedRow['explanation'] = {
  what: 'A message that mentions you.',
  decided_by: 'A direct mention lands in Needs you.',
  otherwise: 'With the channel muted it would still reach you.',
  change: null,
};

let number = 50_000;

export function row(over: Partial<FeedRow> & Pick<FeedRow, 'conversation'>): FeedRow {
  const newest = over.newest ?? ++number;
  return {
    thread: null,
    message: `msg_f${newest}`,
    bucket: 'needs_you',
    unread: true,
    kept: null,
    why: 'Anna mentioned you',
    newest,
    first_new: newest,
    author: 'anna',
    preview: 'Can you look at the launch checklist before noon?',
    at: ago(1),
    new_items: 1,
    reactions: [],
    state: 'none',
    fenced: false,
    explanation: defaultExplanation,
    ...over,
  };
}

export function entry(over: Partial<HistoryEntry> & Pick<HistoryEntry, 'message' | 'conversation'>): HistoryEntry {
  return {
    thread: null,
    author: 'anna',
    preview: 'Can you look at the launch checklist before noon?',
    at: ago(2),
    number: ++number,
    unread: false,
    by: null,
    emoji: null,
    level: null,
    kept: null,
    ...over,
  };
}

export const countsOf = (rows: FeedRow[]): FeedCounts => ({
  focus: rows.filter((r) => r.bucket === 'focus' && r.unread).length,
  needs_you: rows.filter((r) => r.bucket === 'needs_you' && r.unread).length,
  fyi: rows.some((r) => r.bucket === 'fyi' && r.unread),
  kept: rows.filter((r) => r.kept).length,
});

export const baseSettings = (): SettingsAnswer => ({
  focus_people: [
    { person: 'anna', items_last_7_days: 19 },
    { person: 'lena', items_last_7_days: 8 },
  ],
  removed_people: ['tomas'],
  auto_add: true,
  focus_only: false,
  release_on_act: true,
  rules: [{ type: 'item_kind', value: 'flags' }],
  removed_rules: [],
  levels: [{ conversation: '#incidents', thread: null, level: 'focus' }],
});

export class FixtureDoor {
  rows: FeedRow[] = [];
  counts: FeedCounts | null = null;
  settings = baseSettings();
  history: Partial<Record<HistoryTab, HistoryEntry[]>> = {};
  preview: PreviewAnswer | null = { last_7_days: { moved: 14, from: 'focus', to: 'needs_you', stay: 3 } };
  actions: PeopleAction[] = [];
  refuseNext = false;
  errors: string[] = [];
  extraPeople: { id: string; name: string; is_agent: boolean; line: string | null; status: null }[] = [];
  private socket: WebSocketRoute | null = null;
  private feedNumber = 100;
  private conversationNumber = 900_000;

  async install(page: Page) {
    await page.route(/\/orgs\/acme\/people\/[^/]+\/start$/, async (route) => {
      const res = await route.fetch();
      const body = await res.json();
      body.people = [...body.people, ...this.extraPeople];
      await route.fulfill({ response: res, json: body });
    });
    await page.route(/\/orgs\/acme\/people\/[^/]+\/feed$/, (route) => {
      const answer = {
        edition: 1,
        through: number,
        number: this.feedNumber,
        rows: this.rows,
        counts: this.counts ?? countsOf(this.rows),
        notice: null,
      } as ScreenFeedAnswer;
      this.errors.push(...problems('ScreenFeedAnswer', answer));
      return route.fulfill({ json: answer });
    });
    await page.route(/\/orgs\/acme\/people\/[^/]+\/history\?/, (route) => {
      const tab = new URL(route.request().url()).searchParams.get('tab') as HistoryTab;
      const entries = this.history[tab] ?? [];
      const answer: HistoryAnswer = { tab, entries, unread: entries.filter((e) => e.unread).length, next: null };
      this.errors.push(...problems('HistoryAnswer', answer));
      return route.fulfill({ json: answer });
    });
    await page.route(/\/orgs\/acme\/people\/[^/]+\/settings$/, (route) => {
      this.errors.push(...problems('SettingsAnswer', this.settings));
      return route.fulfill({ json: this.settings });
    });
    await page.route(/\/orgs\/acme\/people\/[^/]+\/preview\?/, (route) =>
      this.preview
        ? route.fulfill({ json: this.preview })
        : route.fulfill({ status: 503, json: { not_available: 'the feed is catching up' } }),
    );
    await page.route(/\/orgs\/acme\/people\/actions$/, (route) => {
      const action = route.request().postDataJSON() as PeopleAction;
      this.errors.push(...problems('PeopleAction', action));
      this.actions.push(action);
      if (this.refuseNext) {
        this.refuseNext = false;
        return route.fulfill({ status: 409, json: { refused: 'message deleted' } });
      }
      return route.fulfill({ json: { saved: true, number: ++number } });
    });
    await page.routeWebSocket(/\/orgs\/acme\/live/, (ws) => {
      this.socket = ws;
      ws.onMessage(() => undefined);
    });
  }

  /** One numbered update of the feed stream, as the door would send it. */
  push(parts: FeedPart[]) {
    expect(this.socket, 'the screen has its live connection').not.toBeNull();
    const after = this.feedNumber;
    this.feedNumber += 1;
    const update = { stream: 'feed', number: this.feedNumber, after, edition: 1, parts };
    this.errors.push(...problems('LiveFeedUpdate', update));
    this.socket!.send(JSON.stringify(update));
  }

  /** A row that changed, with the counts the door would send beside it. */
  arrive(...rows: FeedRow[]) {
    for (const r of rows) {
      const at = this.rows.findIndex((x) => x.conversation === r.conversation && x.thread === r.thread);
      if (at >= 0) this.rows[at] = r;
      else this.rows.push(r);
    }
    this.push([...rows.map((r) => ({ kind: 'row.changed' as const, row: r })), { kind: 'counts', counts: countsOf(this.rows) }]);
  }

  pushConversation(parts: ConversationPart[]) {
    const after = this.conversationNumber;
    this.conversationNumber += 1;
    const update = { stream: 'conversations', number: this.conversationNumber, after, parts };
    this.errors.push(...problems('LiveConversationsUpdate', update));
    this.socket!.send(JSON.stringify(update));
  }

  sent<K extends PeopleAction['kind']>(kind: K) {
    return this.actions.filter((a): a is Extract<PeopleAction, { kind: K }> => a.kind === kind);
  }
}

/** A test with a fixture door; everything it served and received must have been valid. */
export const test = base.extend<{ door: FixtureDoor }>({
  door: async ({ page }, use) => {
    const door = new FixtureDoor();
    await door.install(page);
    await use(door);
    expect(door.errors, 'every fixture and every action is valid against the schema').toEqual([]);
  },
});

export { expect };

/** A busy morning, for the checks that need a full list. */
export function busyFeed(): FeedRow[] {
  const change = (about: string, name: string): FeedRow['explanation'] => ({
    what: `A direct message from ${name}, a person.`,
    decided_by: `${name} is one of your Focus people, added the first time they wrote to you.`,
    otherwise: 'A direct message from someone who is not one of them lands in Needs you.',
    change: {
      label: `Remove ${name} from Focus`,
      do: { kind: 'focus_person', about, on: false },
      undo: { kind: 'focus_person', about, on: true },
      moved_last_7_days: 19,
    },
  });
  return [
    row({ conversation: 'dm:anna:marco', bucket: 'focus', why: 'In Focus: direct message from Anna', preview: 'Can you join the pricing call at 3? They want to hear the roadmap from you.', new_items: 4, newest: 1200, first_new: 1190, at: ago(0.2), explanation: change('anna', 'Anna') }),
    row({ conversation: '#incidents', bucket: 'focus', author: 'lena', why: 'In Focus: #incidents is set to Focus', preview: 'Revert is out. p95 is back to 430 ms.', new_items: 6, newest: 1190, at: ago(0.4) }),
    row({ conversation: '#launch', thread: 'msg_kept1', message: 'msg_kept1', bucket: 'focus', unread: false, kept: 'flag', author: 'anna', why: 'Flagged by Anna', preview: 'I need your go/no-go on the Thursday date by end of day.', newest: 1100, at: ago(3), reactions: [{ emoji: '👀', people: ['lena'] }] }),
    row({ conversation: '#general', thread: 'msg_kept2', message: 'msg_kept2', bucket: 'focus', unread: false, kept: 'you', author: 'priya', why: 'Kept by you', state: 'changed', preview: 'Empty states v3 is in the file. One line of copy and one action per screen.', newest: 1090, at: ago(26) }),
    row({ conversation: '#launch', thread: 'msg_n1', message: 'msg_n1', author: 'tomas', why: 'Tomas mentioned you', preview: "The export API can't ship both CSV and XLSX by Thursday. Which one matters for launch?", newest: 1180, at: ago(0.5) }),
    row({ conversation: '#general', thread: 'msg_n2', message: 'msg_n2', author: 'priya', why: 'Reply in a thread you started', preview: 'Blocked: the pricing page still shows the old plan. Who owns that?', new_items: 3, newest: 1170, at: ago(0.7) }),
    row({ conversation: '#incidents', thread: 'msg_n3', message: 'msg_n3', author: 'yuki', why: 'Yuki reacted ✅ to your message', preview: 'Can you confirm the migration runs before the code freeze tonight?', newest: 1160, at: ago(1), reactions: [{ emoji: '✅', people: ['yuki'] }] }),
    row({ conversation: '#general', thread: 'msg_f1', message: 'msg_f1', bucket: 'fyi', author: 'lena', why: 'A thread you follow', preview: 'Trying 1.75 for the tab bar only.', newest: 1150, at: ago(1.2) }),
    row({ conversation: '#launch', thread: 'msg_f2', message: 'msg_f2', bucket: 'fyi', author: 'priya', why: 'Reactions to your message', preview: 'The 2.0 beta is open to the whole team from today.', newest: 1140, at: ago(27), reactions: [{ emoji: '🎉', people: ['anna', 'lena', 'tomas', 'yuki', 'priya', 'marco'] }, { emoji: '❌', people: ['tomas'] }] }),
  ];
}
