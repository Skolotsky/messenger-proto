import type { PersonView } from '../api/types';

/** The word in a mention of everyone. It names nobody (D-001, D-024). */
export const EVERYONE = 'channel';

const fold = (s: string) => s.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase();
const firstName = (p: PersonView) => p.name.trim().split(/\s+/)[0];
const fullHandle = (p: PersonView) => p.name.trim().split(/\s+/).join('-');

/** What is typed after `@` for a person: the first name, or the full name when two people share it. */
export function handleOf(person: PersonView, people: PersonView[]): string {
  const first = fold(firstName(person));
  const shared = people.some((p) => p.id !== person.id && fold(firstName(p)) === first);
  return shared ? fullHandle(person) : firstName(person);
}

function directory(people: PersonView[]): Map<string, string> {
  const out = new Map<string, string>();
  for (const p of people) {
    out.set(fold(handleOf(p, people)), p.id);
    out.set(fold(fullHandle(p)), p.id);
  }
  return out;
}

export interface MentionHit {
  index: number;
  length: number;
  id: string;
}

/** Every mention in a text. "@Marco-thanks" is Marco; an address such as a@b.co is nobody. */
export function findMentions(text: string, people: PersonView[]): MentionHit[] {
  const known = directory(people);
  const out: MentionHit[] = [];
  for (const match of text.matchAll(/(?<![\p{L}\p{N}_])@([\p{L}\p{N}_-]+)/gu)) {
    let token = match[1];
    for (;;) {
      const key = fold(token);
      const id = key === EVERYONE ? EVERYONE : known.get(key);
      if (id) {
        out.push({ index: match.index, length: token.length + 1, id });
        break;
      }
      const cut = token.lastIndexOf('-');
      if (cut <= 0) break;
      token = token.slice(0, cut);
    }
  }
  return out;
}

/** The `mentions` of a message, from its text. One's own name and, in a direct message, "channel" do not count. */
export function mentionsIn(text: string, people: PersonView[], me: string | undefined, allowEveryone: boolean): string[] {
  const out = new Set<string>();
  for (const { id } of findMentions(text, people)) {
    if (id === me || (id === EVERYONE && !allowEveryone)) continue;
    out.add(id);
  }
  return [...out];
}

/** The text after an `@` the caret is in, or null. */
export function tokenAt(text: string, caret: number): { start: number; query: string } | null {
  const match = /(?:^|[^\p{L}\p{N}_])@([\p{L}\p{N}_-]*)$/u.exec(text.slice(0, caret));
  return match ? { start: caret - match[1].length - 1, query: match[1] } : null;
}

/** People whose first or any other name starts with what was typed. */
export function matchPeople(query: string, people: PersonView[]): PersonView[] {
  const q = fold(query);
  return people.filter((p) => fold(p.name).split(/\s+/).some((word) => word.startsWith(q)));
}

export const everyoneMatches = (query: string) => EVERYONE.startsWith(fold(query));
