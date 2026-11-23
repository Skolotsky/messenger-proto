import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { act, loadHistory, newId } from '../api/door';
import { ActivityMark } from './ActivityMark';
import { subscribeLive } from '../api/live';
import type { FeedRow, HistoryAnswer, HistoryEntry, HistoryTab, RowChange } from '../api/types';
import type { Section, ShownRow } from '../state/feedModel';
import type { Feed } from '../state/useFeed';
import type { Session } from '../state/useSession';
import { Avatar } from './Avatar';
import { conversationTitle } from './ConversationView';
import { FeedRowItem } from './FeedRowItem';
import { rowTime, timeOf } from './format';
import { useIsLooking, useOfflineNotice } from './hooks';
import { Icon } from './Icon';
import { BUCKET_NAME, ReasonPanel } from './ReasonPanel';
import type { SettingsSection } from './SettingsScreen';

export const HISTORY_TABS: { tab: HistoryTab; label: string; short: string }[] = [
  { tab: 'direct', label: 'Direct messages', short: 'DMs' },
  { tab: 'mentions', label: 'Mentions', short: 'Mentions' },
  { tab: 'threads', label: 'Threads', short: 'Threads' },
  { tab: 'reactions', label: 'Reactions', short: 'Reactions' },
];

const KEPT_HANDFUL = 5;

const EMPTY: Record<Exclude<Section, 'kept'>, string> = {
  focus: 'Nothing new in Focus.',
  needs_you: 'Nobody is waiting on you.',
  fyi: 'All caught up. What you read is in History.',
};

const TIP: Record<Exclude<Section, 'kept'>, string> = {
  focus: 'Focus: what matches your own rules. You decide what goes here.',
  needs_you: 'Needs you: directed at you. Mentions, replies in your threads, flags.',
  fyi: 'For your information: concerns you, but nobody is waiting on you. Never notifies.',
};

const THREAD_LEVEL: Record<string, string> = {
  focus: 'Focus',
  needs_you: 'Needs you',
  fyi: 'For your information',
  off: 'Off',
};

function Sun() {
  return (
    <svg className="ill" viewBox="0 0 120 64" aria-hidden="true">
      <path d="M8 52h104" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      <path d="M36 52a24 24 0 0 1 48 0" fill="currentColor" opacity=".18" />
      <path d="M60 14v8M30 26l6 6M90 26l-6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      <path d="M20 60h30M62 60h38" stroke="currentColor" strokeWidth="2" strokeLinecap="round" opacity=".4" />
    </svg>
  );
}

type View = 'feed' | HistoryTab;

interface HistoryState {
  answers: Partial<Record<HistoryTab, HistoryAnswer>>;
  /** The entries on screen for the open tab: new ones wait behind Show, like the feed. */
  shown: HistoryEntry[] | null;
  waiting: number;
  /** Where the open tab's next older page starts, once the person has asked for one. */
  paged: string | null | undefined;
}

/** One message can have several reactions, so a message is not enough to name an entry. */
const entryKey = (e: HistoryEntry) => `${e.message}|${e.by ?? ''}|${e.emoji ?? ''}`;

interface Props {
  session: Session;
  feed: Feed;
  onClose(): void;
  onOpenRow(row: FeedRow): void;
  onOpenEntry(entry: HistoryEntry): void;
  onSettings(section: SettingsSection): void;
  /** True while Activity is what the person has in front of them. */
  visible: boolean;
}

export function Activity({ session, feed, onClose, onOpenRow, onOpenEntry, onSettings, visible }: Props) {
  const [view, setView] = useState<View>('feed');
  const [why, setWhy] = useState<{ key: string; anchor: DOMRect } | null>(null);
  const [made, setMade] = useState<Record<string, RowChange | null>>({});
  const [keptOpen, setKeptOpen] = useState(false);
  const [history, setHistory] = useState<HistoryState>({ answers: {}, shown: null, waiting: 0, paged: undefined });
  const scroller = useRef<HTMLDivElement>(null);
  const anchor = useRef<{ key: string; top: number } | null>(null);
  const looking = useIsLooking();
  const fyiSeen = useRef(false);
  const fyiHead = useRef<HTMLElement | null>(null);
  const person = session.me?.id ?? null;
  const viewRef = useRef(view);
  viewRef.current = view;
  const { model } = feed;
  const { enter, markRead } = feed;
  const offline = useOfflineNotice(session.offlineSince);

  /* ---------- history ---------- */
  const refreshHistory = useCallback(
    async (tabs: HistoryTab[]) => {
      if (!person) return;
      const answers = await Promise.all(tabs.map((t) => loadHistory(person, t).catch(() => null)));
      setHistory((h) => {
        const next = { ...h.answers };
        answers.forEach((a) => {
          if (a) next[a.tab] = a;
        });
        const open = viewRef.current;
        if (open === 'feed') return { ...h, answers: next };
        const answer = next[open];
        if (!answer) return { ...h, answers: next };
        if (!h.shown) return { ...h, answers: next, shown: answer.entries, waiting: 0 };
        // In place: what is there shows the truth. What is new waits (D-003).
        const known = new Set(h.shown.map(entryKey));
        const fresh = new Map(answer.entries.map((e) => [entryKey(e), e]));
        // An entry older than this answer's first page is simply not in it; it was not deleted.
        const oldest = answer.next ? Math.min(...answer.entries.map((e) => e.number)) : -Infinity;
        const shown = h.shown.map((e) => fresh.get(entryKey(e)) ?? (e.number < oldest ? e : { ...e, unread: false, preview: '' }));
        return { ...h, answers: next, shown, waiting: answer.entries.filter((e) => !known.has(entryKey(e))).length };
      });
    },
    [person],
  );

  useEffect(() => {
    if (session.phase === 'ready') void refreshHistory(HISTORY_TABS.map((t) => t.tab));
  }, [session.phase, feed.freshLoads, refreshHistory]);

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const off = subscribeLive((message) => {
      if (!('stream' in message) || message.stream !== 'feed') return;
      const said = message.parts.filter((p) => p.kind === 'history.changed');
      if (said.length) {
        setHistory((h) => {
          const answers = { ...h.answers };
          for (const p of said) {
            if (p.kind === 'history.changed' && answers[p.tab]) answers[p.tab] = { ...answers[p.tab]!, unread: p.unread };
          }
          return { ...h, answers };
        });
      }
      const open = viewRef.current;
      const tabs = said.flatMap((p) => (p.kind === 'history.changed' ? [p.tab] : []));
      // The mock door does not say which tab changed, so a row change refreshes them all.
      const needed = said.length ? tabs.filter((t) => t === open) : message.parts.some((p) => p.kind === 'row.changed' || p.kind === 'row.removed') ? HISTORY_TABS.map((t) => t.tab) : [];
      if (!needed.length) return;
      clearTimeout(timer);
      timer = setTimeout(() => void refreshHistory(needed), 250);
    });
    return () => {
      off();
      clearTimeout(timer);
    };
  }, [refreshHistory]);

  const go = (next: View) => {
    if (next === view) return;
    if (view === 'feed') leaveFeed();
    setWhy(null);
    if (next === 'feed') {
      setHistory((h) => ({ ...h, shown: null, waiting: 0, paged: undefined }));
      enter();
    } else {
      setHistory((h) => ({ ...h, shown: h.answers[next]?.entries ?? null, waiting: 0, paged: undefined }));
      void refreshHistory([next]);
    }
    setView(next);
    scroller.current?.scrollTo({ top: 0 });
  };

  /* ---------- leaving the feed ---------- */
  // For your information is read by being viewed, and that takes effect on leaving it (D-003, D-015).
  const leaveFeed = useCallback(() => {
    if (fyiSeen.current) {
      for (const s of model.shown) if (s.section === 'fyi' && s.row.unread) markRead(s.row);
    }
    fyiSeen.current = false;
    setMade({});
  }, [model.shown, markRead]);

  const wasVisible = useRef(visible);
  useEffect(() => {
    if (wasVisible.current && !visible && viewRef.current === 'feed') leaveFeed();
    if (!wasVisible.current && visible && viewRef.current === 'feed') enter();
    wasVisible.current = visible;
  }, [visible, leaveFeed, enter]);

  useEffect(() => {
    const el = fyiHead.current;
    const root = scroller.current;
    if (!el || !root || !visible || !looking || view !== 'feed') return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) fyiSeen.current = true;
      },
      { root },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [visible, looking, view, model.layout]);

  /* ---------- nothing moves ---------- */
  const capture = useCallback(() => {
    const box = scroller.current;
    if (!box) return;
    const top = box.getBoundingClientRect().top;
    for (const el of box.querySelectorAll<HTMLElement>('[data-row]')) {
      const rect = el.getBoundingClientRect();
      if (rect.bottom > top + 1) {
        anchor.current = { key: el.dataset.row!, top: rect.top - top };
        return;
      }
    }
  }, []);

  useLayoutEffect(() => {
    const box = scroller.current;
    const held = anchor.current;
    if (!box || !held || box.scrollTop === 0) return;
    const el = box.querySelector<HTMLElement>(`[data-row="${CSS.escape(held.key)}"]`);
    if (!el) return;
    const delta = el.getBoundingClientRect().top - box.getBoundingClientRect().top - held.top;
    if (Math.abs(delta) > 0.1) box.scrollTop += delta;
  }, [model.shown]);

  const showNew = () => {
    if (view === 'feed') {
      if (fyiSeen.current) leaveFeed();
      feed.show();
    } else {
      setHistory((h) => ({ ...h, shown: h.answers[view]?.entries ?? h.shown, waiting: 0, paged: undefined }));
    }
    anchor.current = null;
    scroller.current?.scrollTo({ top: 0 }); // you asked for them: go to where they are
  };

  /** The person asks for the list as it is now: what they read leaves, what waits comes in. */
  const refresh = async () => {
    if (view !== 'feed') await refreshHistory([view]);
    showNew();
  };

  const loadOlder = async () => {
    if (!person || view === 'feed') return;
    const after = history.paged === undefined ? history.answers[view]?.next : history.paged;
    if (!after) return;
    const page = await loadHistory(person, view, after).catch(() => null);
    if (!page) return;
    setHistory((h) => {
      if (viewRef.current !== page.tab) return h;
      const have = new Set((h.shown ?? []).map(entryKey));
      return { ...h, shown: [...(h.shown ?? []), ...page.entries.filter((e) => !have.has(entryKey(e)))], paged: page.next };
    });
  };

  const closeWhy = useCallback(() => setWhy(null), []);

  const sections = useMemo(() => {
    const by: Record<Section, ShownRow[]> = { focus: [], kept: [], needs_you: [], fyi: [] };
    for (const s of model.shown) by[s.section].push(s);
    return by;
  }, [model.shown]);

  const whyRow = why ? model.shown.find((s) => s.key === why.key) : undefined;
  const counts = model.counts;
  const noFocusRules =
    feed.settings !== null &&
    !feed.settings.auto_add &&
    feed.settings.rules.length === 0 &&
    feed.settings.focus_people.length === 0;

  const rowProps = (s: ShownRow) => ({
    shown: s,
    session,
    feed,
    whyOpen: why?.key === s.key,
    onOpen: onOpenRow,
    onWhy: (row: ShownRow, box: DOMRect) => setWhy((w) => (w?.key === row.key ? null : { key: row.key, anchor: box })),
  });

  const bucket = (b: Exclude<Section, 'kept'>) => {
    const rows = sections[b];
    const kept = b === 'focus' ? sections.kept : [];
    const n = rows.filter((s) => s.row.unread).length;
    const live = b === 'fyi' ? null : (counts?.[b] ?? n);
    const collapsed = kept.length > KEPT_HANDFUL && !keptOpen;
    return (
      <section className={`bucket b-${b}`} aria-label={BUCKET_NAME[b]} key={b} ref={b === 'fyi' ? fyiHead : undefined}>
        <header className="bk-head">
          <h2 title={TIP[b]}>{BUCKET_NAME[b]}</h2>
          {b === 'fyi' ? (
            counts?.fyi ? <span className="bk-dot" role="img" aria-label="Something new" /> : null
          ) : (
            <span className="bk-count">{live}</span>
          )}
          <span className="bk-acts">
            <button
              className="btn sm"
              disabled={n === 0}
              title="Mark every new row here as read. Kept rows stay."
              onClick={() => feed.markAllRead(b)}
            >
              Mark all read
            </button>
          </span>
        </header>
        {rows.length > 0 ? (
          rows.map((s) => <FeedRowItem key={`${model.layout}:${s.key}`} {...rowProps(s)} />)
        ) : b === 'focus' && noFocusRules ? (
          <FocusSuggestions feed={feed} />
        ) : kept.length === 0 ? (
          <div className="empty">
            <Sun />
            <p>{EMPTY[b]}</p>
          </div>
        ) : null}
        {kept.length > 0 && (
          <div className="kept-sec">
            <div className="kept-head">
              <b>Kept · {kept.length}</b>
              <small>
                {feed.settings?.release_on_act === false
                  ? 'Stays until you press Done'
                  : 'Stays until you press Done, reply or react'}
              </small>
              {kept.length > KEPT_HANDFUL && (
                <button className="link" aria-expanded={!collapsed} onClick={() => setKeptOpen((o) => !o)}>
                  {collapsed ? `Show all ${kept.length}` : 'Show fewer'}
                </button>
              )}
            </div>
            {(collapsed ? kept.slice(0, KEPT_HANDFUL) : kept).map((s) => (
              <FeedRowItem key={`${model.layout}:${s.key}`} {...rowProps(s)} />
            ))}
          </div>
        )}
      </section>
    );
  };

  const waitingCount = view === 'feed' ? feed.waitingCount : history.waiting;
  const waitingNote =
    view === 'feed'
      ? [
          feed.waitingBy.focus ? `${feed.waitingBy.focus} in Focus` : '',
          feed.waitingBy.needs_you ? `${feed.waitingBy.needs_you} need you` : '',
        ]
          .filter(Boolean)
          .join(' · ')
      : '';

  return (
    <>
      <header className="top">
        <h1>
          <small className="org-line">{session.orgName}</small>
          Activity
        </h1>
        {offline !== null ? (
          <span className="top-notice" role="status">
            Reconnecting · since {timeOf(new Date(offline).toISOString())}
          </span>
        ) : (
          model.phase === 'ready' &&
          model.notice && (
            <span className="top-notice" role="status">
              {model.notice}
            </span>
          )
        )}
        {waitingCount > 0 && (
          <div className="newpill" role="status">
            <span>
              <b>{waitingCount} new</b>
              {waitingNote && <small>{waitingNote}</small>}
            </span>
            <button onClick={showNew}>Show</button>
          </div>
        )}
        <button className="icon-btn" aria-label="Refresh" title="Refresh: clear what you have read and show what is new" onClick={() => void refresh()}>
          <Icon name="refresh" />
        </button>
        <button className="icon-btn" aria-label="Settings" title="Settings" onClick={() => onSettings('feed')}>
          <Icon name="rules" />
        </button>
        <button className="icon-btn act-close" onClick={onClose} aria-label="Close Activity" title="Close Activity">
          <Icon name="close" />
        </button>
      </header>
      <nav className="tabs" aria-label="Activity views">
        <button className={`tab${view === 'feed' ? ' on' : ''}`} aria-current={view === 'feed' || undefined} onClick={() => go('feed')}>
          <span className="tab-label" data-text="Feed">Feed</span>
          <ActivityMark marks={feed.marks} reserve />
        </button>
        <span className="tabs-sep">History</span>
        {HISTORY_TABS.map(({ tab, label, short }) => {
          const n = history.answers[tab]?.unread ?? 0;
          return (
            <button
              key={tab}
              className={`tab${view === tab ? ' on' : ''}`}
              aria-current={view === tab || undefined}
              aria-label={`${label}${n ? `, ${n} unread` : ''}`}
              title={`History of your ${label.toLowerCase()}. Nothing leaves it by being read.`}
              onClick={() => go(tab)}
            >
              <span className="tab-label" data-text={short}>{short}</span>
              {/* Until its count is known the tab keeps the room for it, so the tabs do not move on load. */}
              {/* The space stays when there is no count, so a count coming or going moves no tab. */}
              {history.answers[tab] ? (n > 0 ? <span className="h-count">{n}</span> : <span className="h-count none" aria-hidden="true" />) : <span className="h-count wait" aria-hidden="true" />}
            </button>
          );
        })}
      </nav>
      <div className="feed-wrap">
        {feed.problem && (
          <div className="feed-problem" role="alert">
            <span>{feed.problem}</span>
            <button className="link" onClick={feed.clearProblem}>
              Dismiss
            </button>
          </div>
        )}
        <div className="feed-scroll" ref={scroller} onScroll={capture} data-testid="feed">
          {view === 'feed' ? (
            model.phase === 'loading' ? (
              <div className="empty start" aria-busy="true" />
            ) : model.phase === 'unavailable' ? (
              <div className="empty start" role="status">
                <p>
                  <b>{model.notice ?? 'Catching up'}</b>
                  <br />
                  Your feed will be here in a moment. Your messages are safe.
                </p>
              </div>
            ) : (
              (['focus', 'needs_you', 'fyi'] as const).map(bucket)
            )
          ) : (
            <HistoryList
              session={session}
              tab={view}
              entries={history.shown ?? history.answers[view]?.entries ?? null}
              hasMore={Boolean(history.paged === undefined ? history.answers[view]?.next : history.paged)}
              onMore={() => void loadOlder()}
              onOpen={(entry) => {
                setHistory((h) => ({
                  ...h,
                  shown: h.shown?.map((e) => (sameRow(e, entry) ? { ...e, unread: false } : e)) ?? h.shown,
                }));
                onOpenEntry(entry);
              }}
              onKeep={(entry) => {
                if (!person) return;
                setHistory((h) => ({
                  ...h,
                  shown: h.shown?.map((e) => (sameRow(e, entry) ? { ...e, kept: e.kept ?? 'you', unread: false } : e)) ?? h.shown,
                }));
                act({ kind: 'keep', id: newId('act'), as: person, conversation: entry.conversation, thread: entry.thread }).catch(() => undefined);
              }}
            />
          )}
        </div>
      </div>
      {whyRow && why && (
        <ReasonPanel
          key={why.key}
          shown={whyRow}
          anchor={why.anchor}
          session={session}
          feed={feed}
          made={made[why.key] ?? null}
          onMade={(change) => setMade((m) => ({ ...m, [why.key]: change }))}
          onSettings={(s) => {
            setWhy(null);
            onSettings(s);
          }}
          onClose={closeWhy}
        />
      )}
    </>
  );
}

const sameRow = (a: HistoryEntry, b: HistoryEntry) => a.conversation === b.conversation && a.thread === b.thread;

function FocusSuggestions({ feed }: { feed: Feed }) {
  const add = (action: Record<string, unknown>, change: Parameters<Feed['saveSetting']>[1]) => {
    feed.saveSetting(action, change).catch(() => undefined);
  };
  return (
    <div className="suggest">
      <p>
        <b>Focus is yours to fill.</b> Nothing lands here until you say what should. Three rules most people start with:
      </p>
      <div className="rule">
        <span>
          <b>Direct messages from people</b>
          <small>Everyone who writes to you directly.</small>
        </span>
        <button className="btn sm pri" onClick={() => add({ kind: 'focus_auto_add', on: true }, (s) => ({ ...s, auto_add: true }))}>
          Add
        </button>
      </div>
      <div className="rule">
        <span>
          <b>Flags that name you</b>
          <small>When someone, or an agent, flags a message for you.</small>
        </span>
        <button
          className="btn sm pri"
          onClick={() =>
            add({ kind: 'focus_rule', rule: { type: 'item_kind', value: 'flags' }, on: true }, (s) => ({
              ...s,
              rules: [...s.rules, { type: 'item_kind', value: 'flags' }],
            }))
          }
        >
          Add
        </button>
      </div>
      <div className="rule">
        <span>
          <b>Any direct mention</b>
          <small>They are in Needs you today.</small>
        </span>
        <button
          className="btn sm pri"
          onClick={() =>
            add({ kind: 'focus_rule', rule: { type: 'item_kind', value: 'mentions' }, on: true }, (s) => ({
              ...s,
              rules: [...s.rules, { type: 'item_kind', value: 'mentions' }],
            }))
          }
        >
          Add
        </button>
      </div>
    </div>
  );
}

function HistoryList({
  session,
  tab,
  entries,
  hasMore,
  onMore,
  onOpen,
  onKeep,
}: {
  session: Session;
  tab: HistoryTab;
  entries: HistoryEntry[] | null;
  hasMore: boolean;
  onMore(): void;
  onOpen(entry: HistoryEntry): void;
  onKeep(entry: HistoryEntry): void;
}) {
  if (!entries) return <div className="empty start" aria-busy="true" />;
  return (
    <>
      {entries.length === 0 && (
        <div className="empty">
          <Sun />
          <p>Nothing here yet.</p>
        </div>
      )}
      {entries.map((e) => {
        const conversation = session.conversations.find((c) => c.conversation === e.conversation);
        const where = conversation ? conversationTitle(conversation, session) : e.conversation;
        const author = session.people.get(e.author);
        const by = e.by ? session.people.get(e.by) : undefined;
        const name = author?.name ?? e.author;
        const gone = e.preview === '';
        return (
          <article className={`row hrow${e.unread ? ' unread' : ''}`} key={entryKey(e)} data-row={`h:${e.message}`}>
            <div className="row-main">
              <button
                className="row-hit"
                disabled={gone}
                aria-label={`${e.unread ? 'Unread: ' : ''}${name} in ${where}. Open the conversation.`}
                onClick={() => onOpen(e)}
              />
              <Avatar id={e.by ?? e.author} person={by ?? author} />
              <div className="row-text">
                <div className="row-head">
                  {e.unread && <span className="dot" role="img" aria-label="unread" />}
                  <span className="row-title">{tab === 'reactions' && by ? by.name : name}</span>
                  <span className="row-sub">
                    {where}
                    {tab === 'threads' && e.level ? ` · ${THREAD_LEVEL[e.level] ?? e.level}` : ''}
                  </span>
                  <span className="row-time">{rowTime(e.at)}</span>
                </div>
                <div className="row-preview">
                  {gone ? (
                    <span className="tomb">This message was deleted.</span>
                  ) : (
                    <>
                      {tab === 'reactions' && e.emoji && <b>{e.emoji} on your message: </b>}
                      {e.preview}
                    </>
                  )}
                </div>
              </div>
              {!gone && (
                <div className="row-side">
                  <button
                    className={`btn sm${e.kept ? ' on' : ''}`}
                    aria-pressed={!!e.kept}
                    disabled={!!e.kept}
                    title={e.kept ? 'Kept. Press Done in the feed to release it.' : 'Keep this in Focus'}
                    onClick={() => onKeep(e)}
                  >
                    {e.kept ? 'Kept' : 'Keep'}
                  </button>
                </div>
              )}
            </div>
          </article>
        );
      })}
      {hasMore && (
        <div className="sr-more">
          <button className="btn sm" onClick={onMore}>
            Show older
          </button>
        </div>
      )}
    </>
  );
}
