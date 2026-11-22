// Every call the control panel makes, with its own small types. The shapes follow
// docs/contracts/operations-and-control.md; aligning them with the generated types is an edit here.

export type ReceiverName = 'search' | 'sender' | 'feed';
export type Mode = 'running' | 'slow' | 'off';
export type ReceiverState = 'running' | 'down' | 'paused' | 'stopped';

export interface Current { org: string; name: string }
export interface Person { id: string; name: string; is_agent: boolean; line: string | null; status: null }
export interface Situation { id: string; title: string; outcome: string }
export interface ScriptStep { at: string; who: string; what: string; situation: string | null }
export interface Script { running: boolean; pace: number; happened: ScriptStep[]; next: ScriptStep[]; situations: Situation[] }
export type ScriptCommand =
  | { do: 'start' }
  | { do: 'pause' }
  | { do: 'pace'; pace: number }
  | { do: 'play_situation'; situation: string };
export interface ReceiverStatus { name: ReceiverName; place: number; waiting: number; state: ReceiverState; parked: number }
export interface ParkedChange { receiver: ReceiverName; number: number; reason: string; tries: number }
// Assumed: "preparing" marks the organization a running Reset is seeding. The contract names only current and retired.
export type OrgState = 'current' | 'retired' | 'preparing';
export interface Organization { org: string; name: string; state: OrgState; created_at: string; approx_changes: number }

export interface Request { method: 'GET' | 'POST' | 'DELETE'; path: string; body?: unknown; timeoutMs: number }
export interface Answer { status: number; body: unknown }
export type Transport = (req: Request) => Promise<Answer>;

/** A call that was refused, failed, or got no answer. */
export class CallError extends Error {
  constructor(readonly status: number | null, readonly body: unknown) {
    super(status === null ? 'no answer' : `answered ${status}`);
  }
}

// The contract's limit for a call between parts (G-023).
const CALL_MS = 3_000;
// Reset and Rebuild answer when the work is done, which takes as long as seeding or refilling takes.
const LONG_MS = 180_000;

/** Calls go to /ops/<part>/...; the route in front of the parts is an assumption until the demo program exists. */
export const networkTransport: Transport = async ({ method, path, body, timeoutMs }) => {
  const stop = new AbortController();
  const timer = setTimeout(() => stop.abort(), timeoutMs);
  try {
    const res = await fetch(`/ops/${path}`, {
      method,
      signal: stop.signal,
      headers: body === undefined ? undefined : { 'content-type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const text = await res.text();
    let parsed: unknown = null;
    try { parsed = text ? JSON.parse(text) : null; } catch { parsed = text; }
    return { status: res.status, body: parsed };
  } catch {
    throw new CallError(null, null);
  } finally {
    clearTimeout(timer);
  }
};

export function createApi(send: Transport) {
  const call = async <T>(method: Request['method'], path: string, body?: unknown, timeoutMs = CALL_MS): Promise<T> => {
    const answer = await send({ method, path, body, timeoutMs });
    if (answer.status >= 300) throw new CallError(answer.status, answer.body);
    return answer.body as T;
  };
  const org = (id: string) => `orgs/${encodeURIComponent(id)}`;
  return {
    current: () => call<Current>('GET', 'demo/current'),
    organizations: async () => (await call<{ organizations: Organization[] }>('GET', 'demo/organizations')).organizations,
    people: async (id: string) => (await call<{ people: Person[] }>('GET', `conversations/${org(id)}/people`)).people,
    script: (id: string) => call<Script>('GET', `demo/${org(id)}/script`),
    command: (id: string, command: ScriptCommand) => call<unknown>('POST', `demo/${org(id)}/script`, command),
    status: async (id: string) => (await call<{ receivers: ReceiverStatus[] }>('GET', `delivery/${org(id)}/status`)).receivers,
    parked: async (id: string) => (await call<{ parked: ParkedChange[] }>('GET', `delivery/${org(id)}/parked`)).parked,
    retry: (id: string, change: ParkedChange) =>
      call<unknown>('POST', `delivery/${org(id)}/parked/${change.number}/retry?receiver=${change.receiver}`),
    mode: async (part: ReceiverName, id: string) => (await call<{ mode: Mode }>('GET', `${part}/${org(id)}/control/mode`)).mode,
    setMode: (part: ReceiverName, id: string, mode: Mode) => call<unknown>('POST', `${part}/${org(id)}/control/mode`, { mode }),
    rebuild: (id: string, receiver: ReceiverName) =>
      call<unknown>('POST', `demo/${org(id)}/rebuild`, { receiver, rules_changed: false }, LONG_MS),
    reset: () => call<Current>('POST', 'demo/reset', undefined, LONG_MS),
    deleteOrganization: (id: string) => call<unknown>('DELETE', `demo/organizations/${encodeURIComponent(id)}`, undefined, LONG_MS),
    deleteRetired: () => call<unknown>('POST', 'demo/organizations/delete-retired', undefined, LONG_MS),
  };
}

export type Api = ReturnType<typeof createApi>;

/** What to tell the operator, in place, about a call that failed. */
export function explain(error: unknown, action: string): string {
  if (!(error instanceof CallError)) return `${action} failed.`;
  if (error.status === null) return `${action}: no answer within the time limit. The part may be off or unreachable.`;
  const body = (error.body ?? {}) as Record<string, unknown>;
  if (error.status === 409 && body.refused === 'current') return 'Refused: this is the current organization, so it cannot be deleted.';
  if (error.status === 410 || body.organization_gone) return `${action}: this organization has been retired. Moving to the current one.`;
  if (typeof body.refused === 'string') return `${action} refused: ${body.refused}.`;
  if (typeof body.error === 'string') return `${action} failed: ${body.error}.`;
  return `${action} failed (answer ${error.status}).`;
}
