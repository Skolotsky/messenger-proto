import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
  type ReactNode,
  type RefObject,
} from 'react';
import { createPortal } from 'react-dom';
import type { PersonView } from '../api/types';
import { loadSuggestions } from '../api/writing';
import { everyoneMatches, EVERYONE, findMentions, handleOf, matchPeople, tokenAt } from '../state/mentions';
import type { Session } from '../state/useSession';
import { Avatar } from './Avatar';

const MAX_SHOWN = 6;

/** A message's text with the mentions it really carries marked. Everything stays text (T-038, point 24). */
export function textWithMentions(
  text: string,
  mentions: string[],
  people: PersonView[],
  me: string | undefined,
): ReactNode {
  if (mentions.length === 0) return text;
  const out: ReactNode[] = [];
  let last = 0;
  for (const hit of findMentions(text, people)) {
    if (!mentions.includes(hit.id)) continue;
    out.push(text.slice(last, hit.index));
    out.push(
      <span key={hit.index} className={`at${hit.id === me ? ' me' : ''}`}>
        {text.slice(hit.index, hit.index + hit.length)}
      </span>,
    );
    last = hit.index + hit.length;
  }
  out.push(text.slice(last));
  return out;
}

type Item = { kind: 'person'; person: PersonView } | { kind: 'everyone' };

interface Options {
  session: Session;
  allowEveryone: boolean;
  value: string;
  onValue(value: string): void;
  box: RefObject<HTMLTextAreaElement>;
}

/** Suggestions after `@`, driven from the keyboard while typing (D-024). Used by the message box and the edit box. */
export function useMentionBox({ session, allowEveryone, value, onValue, box }: Options) {
  const me = session.me?.id;
  const [token, setToken] = useState<{ start: number; query: string; caret: number } | null>(null);
  const [dismissed, setDismissed] = useState<number | null>(null);
  const [active, setActive] = useState(0);
  const [answer, setAnswer] = useState<{ query: string; people: PersonView[]; everyone: boolean } | null>(null);
  const [place, setPlace] = useState<CSSProperties | null>(null);
  const caretAfter = useRef<number | null>(null);

  const sync = useCallback(() => {
    const el = box.current;
    if (!el || document.activeElement !== el) {
      setToken(null);
      return;
    }
    const caret = el.selectionStart;
    const found = el.selectionEnd === caret ? tokenAt(el.value, caret) : null;
    setToken((old) => {
      if (!found) return null;
      if (old && old.start === found.start && old.query === found.query && old.caret === caret) return old;
      return { ...found, caret };
    });
    if (!found) setDismissed(null);
  }, [box]);

  useLayoutEffect(() => {
    const el = box.current;
    if (el && caretAfter.current !== null) {
      el.setSelectionRange(caretAfter.current, caretAfter.current);
      caretAfter.current = null;
    }
    sync();
  }, [value, box, sync]);

  const query = token?.query ?? null;
  useEffect(() => {
    setActive(0);
    if (query === null || !me) return;
    let current = true;
    loadSuggestions(query, me)
      .then((a) => {
        if (current) setAnswer({ query, people: a.people, everyone: a.everyone });
      })
      .catch(() => {
        /* the people this screen already knows are enough to go on writing */
      });
    return () => {
      current = false;
    };
  }, [query, me]);

  const answered = answer && answer.query === query ? answer : null;
  const people = query === null ? [] : (answered?.people ?? matchPeople(query, session.peopleList));
  const everyone = query !== null && allowEveryone && (answered ? answered.everyone : everyoneMatches(query));
  const items: Item[] = [
    ...people
      .filter((p) => p.id !== me)
      .slice(0, MAX_SHOWN)
      .map((person): Item => ({ kind: 'person', person })),
    ...(everyone ? [{ kind: 'everyone' } as Item] : []),
  ];
  const open = token !== null && dismissed !== token.start && items.length > 0;
  const current = Math.min(active, items.length - 1);

  useLayoutEffect(() => {
    if (!open) {
      setPlace(null);
      return;
    }
    const measure = () => {
      const r = box.current?.getBoundingClientRect();
      if (!r) return;
      const width = Math.min(r.width, 380);
      const left = Math.max(8, Math.min(r.left, window.innerWidth - width - 8));
      setPlace(
        r.top > 290
          ? { position: 'fixed', left, width, bottom: window.innerHeight - r.top + 4 }
          : { position: 'fixed', left, width, top: r.bottom + 4 },
      );
    };
    measure();
    window.addEventListener('resize', measure);
    window.addEventListener('scroll', measure, true);
    return () => {
      window.removeEventListener('resize', measure);
      window.removeEventListener('scroll', measure, true);
    };
  }, [open, box]);

  const pick = (item: Item) => {
    if (!token) return;
    const handle = item.kind === 'everyone' ? EVERYONE : handleOf(item.person, session.peopleList);
    const inserted = `@${handle} `;
    caretAfter.current = token.start + inserted.length;
    onValue(value.slice(0, token.start) + inserted + value.slice(token.caret));
  };

  /** True when the key belonged to the list of suggestions. */
  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>): boolean => {
    if (!open || e.nativeEvent.isComposing) return false;
    switch (e.key) {
      case 'ArrowDown':
        setActive((current + 1) % items.length);
        break;
      case 'ArrowUp':
        setActive((current - 1 + items.length) % items.length);
        break;
      case 'Enter':
        if (e.ctrlKey || e.metaKey || e.shiftKey) return false;
        pick(items[current]);
        break;
      case 'Tab':
        pick(items[current]);
        break;
      case 'Escape':
        setDismissed(token.start);
        break;
      default:
        return false;
    }
    e.preventDefault();
    e.stopPropagation();
    return true;
  };

  const popup =
    open && place
      ? createPortal(
          <div className="mp-box" role="listbox" aria-label="Mention someone" style={{ ...place, zIndex: 50 }}>
            {items.map((item, i) => (
              <button
                key={item.kind === 'everyone' ? EVERYONE : item.person.id}
                type="button"
                role="option"
                aria-selected={i === current}
                className={i === current ? 'act' : undefined}
                onMouseDown={(e) => e.preventDefault()} // the caret stays in the box
                onClick={() => pick(item)}
              >
                {item.kind === 'everyone' ? (
                  <>
                    <span className="hash">@</span>
                    <b>channel</b>
                    <small>Everyone here. Not a mention of anyone in particular.</small>
                  </>
                ) : (
                  <>
                    <Avatar id={item.person.id} person={item.person} small />
                    <b>{item.person.name}</b>
                    {item.person.line && <small>{item.person.line}</small>}
                  </>
                )}
              </button>
            ))}
          </div>,
          document.body,
        )
      : null;

  return { onKeyDown, onCaret: sync, popup };
}
