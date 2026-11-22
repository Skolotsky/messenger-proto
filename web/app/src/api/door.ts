import { ORG } from '../config';
import type {
  ActionSaved,
  HistoryAnswer,
  HistoryTab,
  MessagesPage,
  PeopleAction,
  PreviewAnswer,
  RowAct,
  ScreenFeedAnswer,
  SettingsAnswer,
  StartAnswer,
} from './types';

/** Why an action did not go through. A refusal is final; anything else may be tried again. */
export class DoorError extends Error {
  constructor(
    readonly kind: 'refused' | 'unreachable',
    message: string,
  ) {
    super(message);
  }
}

const LIMIT_MS = 3000; // the screen treats an action unanswered after 3 s as not sent (screen-to-conversations.md)

export async function call<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`/orgs/${encodeURIComponent(ORG)}${path}`, { ...init, signal: AbortSignal.timeout(LIMIT_MS) });
  } catch {
    throw new DoorError('unreachable', 'No answer from the server.');
  }
  if (res.ok) return (await res.json()) as T;
  if (res.status >= 400 && res.status < 500) {
    const body = (await res.json().catch(() => null)) as { refused?: string } | null;
    throw new DoorError('refused', body?.refused ?? `Refused (${res.status}).`);
  }
  throw new DoorError('unreachable', `Server error (${res.status}).`);
}

export const loadStart = (person: string) => call<StartAnswer>(`/people/${encodeURIComponent(person)}/start`);

export function loadMessages(conversation: string, thread: string | null, around?: number) {
  const params = new URLSearchParams();
  if (thread) params.set('thread', thread);
  if (around !== undefined) params.set('around', String(around));
  const query = params.size ? `?${params}` : '';
  return call<MessagesPage>(`/conversations/${encodeURIComponent(conversation)}/messages${query}`);
}

const personPath = (person: string) => `/people/${encodeURIComponent(person)}`;

export const loadFeed = (person: string) => call<ScreenFeedAnswer>(`${personPath(person)}/feed`);

export const loadHistory = (person: string, tab: HistoryTab) =>
  call<HistoryAnswer>(`${personPath(person)}/history?tab=${tab}`);

export const loadSettings = (person: string) => call<SettingsAnswer>(`${personPath(person)}/settings`);

/** What a change would have moved in the last 7 days. May be unavailable; the action never waits for it. */
export const loadPreview = (person: string, change: RowAct) =>
  call<PreviewAnswer>(`${personPath(person)}/preview?change=${encodeURIComponent(JSON.stringify(change))}`);

export const act = (action: PeopleAction) =>
  call<ActionSaved>('/people/actions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(action),
  });

/** An ID the caller chooses, so that sending the same action again changes nothing. */
export function newId(prefix: 'act' | 'msg'): string {
  const bytes = crypto.getRandomValues(new Uint8Array(12));
  return `${prefix}_${Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')}`;
}
