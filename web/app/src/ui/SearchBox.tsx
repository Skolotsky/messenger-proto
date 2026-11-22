import { useEffect, useRef, useState } from 'react';
import type { SearchResult } from '../api/types';
import { searchMessages } from '../api/writing';
import { markedPieces } from '../state/marks';
import type { Session } from '../state/useSession';
import { Avatar } from './Avatar';
import { whenOf } from './format';
import type { Jump } from './jump';
import { WritingIcon } from './WritingIcon';

const UNAVAILABLE = 'Search is unavailable right now. Your messages are safe.';
const WAIT_MS = 180;

interface Found {
  query: string;
  results: SearchResult[];
  total: number;
  next: string | null;
  notice: string | null;
}

type View = { status: 'idle' } | { status: 'loading' } | { status: 'unavailable'; notice: string } | ({ status: 'ready' } & Found);

/** Where a message is, in words a person would use. */
function placeOf(hit: SearchResult, session: Session): string {
  const c = session.conversations.find((x) => x.conversation === hit.conversation);
  if (c?.type === 'direct') {
    const other = c.people.find((p) => p !== session.me?.id) ?? c.people[0];
    return session.people.get(other)?.name ?? other;
  }
  return c ? `#${c.name}` : hit.conversation;
}

/** The search box, its results, and what it says when the index is behind or away (D-024; T-038, 20 to 22). */
export function SearchBox({ session, onHit }: { session: Session; onHit(jump: Jump): void }) {
  const person = session.me?.id ?? '';
  const [query, setQuery] = useState('');
  const [open, setOpen] = useState(false);
  const [view, setView] = useState<View>({ status: 'idle' });
  const [more, setMore] = useState<'idle' | 'loading' | 'failed'>('idle');
  const asked = useRef(0);
  const nonce = useRef(0);
  const root = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const q = query.trim();
    const ask = ++asked.current;
    setMore('idle');
    if (!q) {
      setView({ status: 'idle' });
      return;
    }
    setView((v) => (v.status === 'ready' ? v : { status: 'loading' }));
    const timer = setTimeout(() => {
      searchMessages(person, q, null)
        .then(({ answer, notice }) => {
          if (ask !== asked.current) return;
          if (!answer) setView({ status: 'unavailable', notice: notice ?? UNAVAILABLE });
          else setView({ status: 'ready', query: q, results: answer.results, total: answer.total, next: answer.next, notice });
        })
        .catch(() => {
          if (ask === asked.current) setView({ status: 'unavailable', notice: UNAVAILABLE });
        });
    }, WAIT_MS);
    return () => clearTimeout(timer);
  }, [query, person]);

  useEffect(() => {
    if (!open) return;
    const away = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('pointerdown', away);
    return () => document.removeEventListener('pointerdown', away);
  }, [open]);

  const loadMore = async () => {
    if (view.status !== 'ready' || !view.next) return;
    const ask = asked.current;
    setMore('loading');
    try {
      const { answer, notice } = await searchMessages(person, view.query, view.next);
      if (ask !== asked.current) return;
      if (!answer) {
        setMore('failed');
        return;
      }
      setView((v) => {
        if (v.status !== 'ready') return v;
        const known = new Set(v.results.map((r) => r.message));
        return {
          ...v,
          results: [...v.results, ...answer.results.filter((r) => !known.has(r.message))],
          total: answer.total,
          next: answer.next,
          notice,
        };
      });
      setMore('idle');
    } catch {
      if (ask === asked.current) setMore('failed');
    }
  };

  const openHit = (hit: SearchResult) => {
    setOpen(false);
    onHit({
      conversation: hit.conversation,
      thread: hit.thread,
      message: hit.message,
      number: hit.number,
      nonce: ++nonce.current,
    });
  };

  return (
    <div
      className="search"
      ref={root}
      onKeyDown={(e) => {
        if (e.key === 'Escape') setOpen(false);
      }}
    >
      <WritingIcon name="search" />
      <input
        type="search"
        aria-label="Search messages"
        placeholder="Search messages"
        autoComplete="off"
        value={query}
        onFocus={() => setOpen(true)}
        onChange={(e) => {
          setQuery(e.target.value);
          setOpen(true);
        }}
      />
      {open && view.status !== 'idle' && (
        <div className="sr">
          <div className="sr-box" role="region" aria-label="Search results">
            {view.status === 'loading' && <p className="sheet-note sr-note">Searching…</p>}
            {view.status === 'unavailable' && (
              <p className="sr-note sr-off" role="status">
                {view.notice}
              </p>
            )}
            {view.status === 'ready' && (
              <>
                <div className="sr-h">
                  <b data-testid="search-total">
                    {view.total} {view.total === 1 ? 'message' : 'messages'}
                  </b>
                  {view.notice && (
                    <span className="lag" role="status">
                      {view.notice}
                    </span>
                  )}
                </div>
                {view.results.length === 0 && <p className="sheet-note sr-note">Nothing found.</p>}
                {view.results.map((hit) => (
                  <button key={hit.message} className="tray-e" data-hit-id={hit.message} onClick={() => openHit(hit)}>
                    <Avatar id={hit.author} person={session.people.get(hit.author)} small />
                    <span className="tray-t">
                      <b>{session.people.get(hit.author)?.name ?? hit.author}</b>{' '}
                      <small>
                        {placeOf(hit, session)}
                        {hit.thread ? ' · in a thread' : ''} · {whenOf(hit.created_at)}
                        {hit.edited ? ' · edited' : ''}
                      </small>
                      <span className="sr-text">
                        {markedPieces(hit.text, hit.match).map((p, i) =>
                          p.marked ? <mark key={i}>{p.text}</mark> : <span key={i}>{p.text}</span>,
                        )}
                      </span>
                    </span>
                  </button>
                ))}
                {view.next && (
                  <div className="sr-more">
                    {more === 'failed' && (
                      <span className="err" role="alert">
                        Could not load more.
                      </span>
                    )}
                    <button className="btn sm" disabled={more === 'loading'} onClick={() => void loadMore()}>
                      {more === 'failed' ? 'Try again' : `Show more (${view.total - view.results.length} left)`}
                    </button>
                  </div>
                )}
                <p className="sr-f">An edited message is found by its new text. A deleted message is not found.</p>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
