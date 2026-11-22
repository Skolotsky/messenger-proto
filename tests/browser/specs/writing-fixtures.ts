import { expect, type Page, type Request } from '@playwright/test';
import { readFileSync } from 'node:fs';

type Schema = Record<string, any>;
const root = JSON.parse(
  readFileSync(new URL('../../../contracts/schema/contexia.json', import.meta.url), 'utf8'),
) as Schema;

function problems(schema: Schema | boolean, value: unknown, at: string): string[] {
  if (schema === true) return [];
  if (schema === false) return [`${at}: not allowed`];
  if (schema.$ref) return problems(root.$defs[String(schema.$ref).replace('#/$defs/', '')], value, at);
  const out: string[] = [];
  if (schema.oneOf) {
    const fits = (schema.oneOf as Schema[]).filter((s) => problems(s, value, at).length === 0).length;
    if (fits !== 1) out.push(`${at}: fits ${fits} of the oneOf choices`);
  }
  if (schema.allOf) for (const s of schema.allOf as Schema[]) out.push(...problems(s, value, at));
  if ('const' in schema && value !== schema.const) out.push(`${at}: must be ${schema.const}`);
  if (schema.enum && !schema.enum.includes(value)) out.push(`${at}: not one of ${schema.enum}`);
  if (schema.type) {
    const kind = value === null ? 'null' : Array.isArray(value) ? 'array' : Number.isInteger(value) ? 'integer' : typeof value;
    const types: string[] = [].concat(schema.type);
    if (!types.includes(kind) && !(kind === 'integer' && types.includes('number'))) out.push(`${at}: is ${kind}, must be ${types}`);
  }
  if (typeof value === 'number' && schema.minimum !== undefined && value < schema.minimum) out.push(`${at}: below ${schema.minimum}`);
  if (Array.isArray(value)) {
    if (schema.minItems !== undefined && value.length < schema.minItems) out.push(`${at}: too few items`);
    const first: Schema[] = schema.prefixItems ?? [];
    value.forEach((v, i) => {
      const s = i < first.length ? first[i] : schema.items;
      if (s !== undefined) out.push(...problems(s, v, `${at}[${i}]`));
    });
  } else if (value && typeof value === 'object') {
    const o = value as Record<string, unknown>;
    for (const r of (schema.required ?? []) as string[]) if (!(r in o)) out.push(`${at}.${r}: missing`);
    for (const [k, v] of Object.entries(o)) {
      if (schema.properties?.[k]) out.push(...problems(schema.properties[k], v, `${at}.${k}`));
      else if (schema.additionalProperties === false) out.push(`${at}.${k}: not in the schema`);
    }
  }
  return out;
}

export const schemaProblems = (definition: string, value: unknown) => problems(root.$defs[definition], value, definition);

/** A fixture has to be a valid answer of the contract, or the test proves nothing. */
export function valid<T>(definition: string, value: T): T {
  expect(schemaProblems(definition, value), `the fixture is a valid ${definition}`).toEqual([]);
  return value;
}

export function hit(over: Record<string, unknown>) {
  return {
    message: 'msg_hit', number: 100, created_at: '2026-11-22T10:14:03.120Z', version: 1, conversation: '#launch',
    thread: null, author: 'anna', edited: false, text: 'the pricing page', match: [[4, 11]], ...over,
  };
}

export const searchAnswer = (answer: Record<string, unknown> | null, notice: string | null = null) =>
  valid('ScreenSearchAnswer', { answer, notice });

export function row(id: string, number: number, over: Record<string, unknown> = {}, summary: unknown = null) {
  return {
    message: {
      id, number, version: 1, conversation: '#launch', thread: null, author: 'anna', author_is_agent: false,
      participants: null, text: `Message ${id}`, mentions: [], level: null, reactions: [], stopped: false,
      created_at: '2026-11-22T09:00:00.000Z', edited_at: null, deleted: false, ...over,
    },
    thread_summary: summary,
  };
}

export const messagesPage = (messages: unknown[], over: Record<string, unknown> = {}) =>
  valid('MessagesAnswer', { number: 500, messages, in_progress: [], read_up_to: null, has_before: false, has_after: false, ...over });

/** The actions this page sends, as they leave it. */
export function recordActions(page: Page) {
  const sent: Record<string, any>[] = [];
  page.on('request', (r: Request) => {
    if (r.method() === 'POST' && r.url().endsWith('/people/actions')) sent.push(r.postDataJSON());
  });
  return { sent, of: (kind: string) => sent.filter((a) => a.kind === kind) };
}

/** Answers the next action of a kind with a refusal, or with no answer at all. */
export async function failNext(page: Page, kind: string, how: { refused: string } | 'down') {
  let done = false;
  await page.route('**/orgs/acme/people/actions', async (route) => {
    const body = route.request().postDataJSON();
    if (done || body.kind !== kind) return route.fallback();
    done = true;
    if (how === 'down') return route.fulfill({ status: 503, json: { error: 'down' } });
    return route.fulfill({ status: 409, json: how });
  });
}
