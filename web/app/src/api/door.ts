import { ORG } from '../config';
import type { ActionSaved, MessagesPage, PeopleAction, StartAnswer } from './types';

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

async function call<T>(path: string, init?: RequestInit): Promise<T> {
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

export function loadMessages(conversation: string, thread: string | null) {
  const query = thread ? `?thread=${encodeURIComponent(thread)}` : '';
  return call<MessagesPage>(`/conversations/${encodeURIComponent(conversation)}/messages${query}`);
}

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
