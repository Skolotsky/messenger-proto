import { call } from './door';
import type { ScreenSearchAnswer, SuggestAnswer } from './types';

export const loadSuggestions = (query: string, as: string) =>
  call<SuggestAnswer>(`/suggest?q=${encodeURIComponent(query)}&as=${encodeURIComponent(as)}`);

export function searchMessages(person: string, query: string, after: string | null) {
  const page = after ? `&after=${encodeURIComponent(after)}` : '';
  return call<ScreenSearchAnswer>(`/people/${encodeURIComponent(person)}/search?q=${encodeURIComponent(query)}${page}`);
}
