import { ENTRY_URL, ORG } from '../config';

export type GoneState = 'here' | 'leaving' | 'failed';

let state: GoneState = 'here';
const listeners = new Set<() => void>();

const set = (next: GoneState) => {
  state = next;
  listeners.forEach((notify) => notify());
};

export const goneState = () => state;

export function subscribeGone(notify: () => void) {
  listeners.add(notify);
  return () => listeners.delete(notify);
}

/**
 * This organization was retired or deleted (D-033). The screen asks the entry address which
 * organization is current and opens it. The entry address itself belongs to the demo program.
 */
export async function goToEntry() {
  if (state === 'leaving') return;
  set('leaving');
  try {
    const res = await fetch(ENTRY_URL, { signal: AbortSignal.timeout(3000) });
    if (!res.ok) throw new Error(String(res.status));
    const { org } = (await res.json()) as { org: string };
    if (org === ORG) throw new Error('the entry address still names this organization');
    location.replace(`/?org=${encodeURIComponent(org)}`);
  } catch {
    set('failed');
  }
}
