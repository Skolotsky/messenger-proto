// Every call the notification app makes, with its own small types. The shapes follow
// docs/contracts/operations-and-control.md and feed-to-notification-sender.md.

export interface Current { org: string; name: string }
export interface Person { id: string; name: string; is_agent: boolean; line: string | null; status: null }

export interface SenderEntry {
  name: string;
  person: string;
  /** The sender may hold other states; only "sent" and "history" are ever shown (D-034). */
  state: string;
  /** When it was sent; null for a history entry that was never delivered. */
  at: string | null;
  delayed: boolean;
  why: string;
  /** The current text: the sender overwrites it on an edit. */
  text: string;
  now: 'read' | 'deleted' | null;
  /** Assumed and optional: the contract has no way yet to say the text changed after sending. */
  edited?: boolean;
}

export interface Notification extends SenderEntry { state: 'sent' | 'history' }

export interface Request { path: string; timeoutMs: number }
export interface Answer { status: number; body: unknown }
export type Transport = (req: Request) => Promise<Answer>;

export class CallError extends Error {
  constructor(readonly status: number | null, readonly body: unknown) {
    super(status === null ? 'no answer' : `answered ${status}`);
  }
}

const CALL_MS = 3_000;

/** Calls go to /ops/<part>/...; the route in front of the parts is an assumption until the demo program exists. */
export const networkTransport: Transport = async ({ path, timeoutMs }) => {
  const stop = new AbortController();
  const timer = setTimeout(() => stop.abort(), timeoutMs);
  try {
    const res = await fetch(`/ops/${path}`, { signal: stop.signal });
    const text = await res.text();
    let body: unknown = null;
    try { body = text ? JSON.parse(text) : null; } catch { body = text; }
    return { status: res.status, body };
  } catch {
    throw new CallError(null, null);
  } finally {
    clearTimeout(timer);
  }
};

/** Only what was sent or listed as history, newest first; entries without a time keep the sender's order, after the rest. */
export function visible(entries: SenderEntry[]): Notification[] {
  const shown = entries.filter((e): e is Notification => e.state === 'sent' || e.state === 'history');
  const time = (e: Notification) => (e.at === null ? -Infinity : Date.parse(e.at));
  return shown
    .map((entry, index) => ({ entry, index }))
    .sort((a, b) => time(b.entry) - time(a.entry) || a.index - b.index)
    .map(({ entry }) => entry);
}

export function createApi(send: Transport) {
  const get = async <T>(path: string): Promise<T> => {
    const answer = await send({ path, timeoutMs: CALL_MS });
    if (answer.status >= 300) throw new CallError(answer.status, answer.body);
    return answer.body as T;
  };
  const org = (id: string) => `orgs/${encodeURIComponent(id)}`;
  return {
    current: () => get<Current>('demo/current'),
    people: async (id: string) => (await get<{ people: Person[] }>(`conversations/${org(id)}/people`)).people.filter((p) => !p.is_agent),
    notifications: async (id: string, person: string) =>
      visible((await get<{ notifications: SenderEntry[] }>(`sender/${org(id)}/notifications?person=${encodeURIComponent(person)}`)).notifications),
  };
}

export type Api = ReturnType<typeof createApi>;

export function explain(error: unknown, what: string): string {
  if (error instanceof CallError && error.status !== null) {
    const body = (error.body ?? {}) as Record<string, unknown>;
    if (error.status === 410 || body.organization_gone) return `${what}: this organization has been retired. Moving to the current one.`;
    if (typeof body.error === 'string') return `${what}: ${body.error}.`;
    return `${what}: the answer was ${error.status}.`;
  }
  return `${what}: no answer. It may be slow or off.`;
}
