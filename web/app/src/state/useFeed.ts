import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from 'react';
import { act, DoorError, loadFeed, loadSettings, newId } from '../api/door';
import { subscribeLive } from '../api/live';
import type { FeedRow, PeopleAction, RowAct, SettingsAnswer } from '../api/types';
import { emptyFeed, feedReduce, rowKey, waitingBy, type FeedModel, type Section, type ShownRow } from './feedModel';
import type { Session } from './useSession';

const inViewOf = (key: string) => {
  const box = document.querySelector('.feed-scroll');
  const el = box?.querySelector(`[data-row="${CSS.escape(key)}"]`);
  if (!box || !el) return false;
  const a = box.getBoundingClientRect();
  const b = el.getBoundingClientRect();
  return b.bottom > a.top && b.top < a.bottom;
};

/** What a row's one change, or its Undo, says to the door. */
export function rowActToAction(change: RowAct, as: string): PeopleAction {
  return { ...change, id: newId('act'), as } as PeopleAction;
}

export interface Feed {
  model: FeedModel;
  waitingCount: number;
  waitingBy: ReturnType<typeof waitingBy>;
  /** Unread rows in Focus and Needs you: the one number that calls (D-019). */
  attention: number;
  problem: string | null;
  clearProblem(): void;
  settings: SettingsAnswer | null;
  saveSetting(action: RowAct | Record<string, unknown>, change: (s: SettingsAnswer) => SettingsAnswer): Promise<void>;
  reloadSettings(): void;
  show(): void;
  /** The person came back to the feed: the list is laid out from the truth (D-003, D-013). */
  enter(): void;
  markRead(row: FeedRow): void;
  markAllRead(section: Section): void;
  keep(row: FeedRow): void;
  done(row: FeedRow): void;
  react(row: FeedRow, emoji: string): void;
  perform(change: RowAct): Promise<void>;
}

const say = (e: unknown) =>
  e instanceof DoorError && e.kind === 'refused' ? `Not saved: ${e.message}.` : 'Not saved: no answer from the server.';

export function useFeed(session: Session): Feed {
  const [model, dispatch] = useReducer(feedReduce, emptyFeed);
  const [problem, setProblem] = useState<string | null>(null);
  const [settings, setSettings] = useState<SettingsAnswer | null>(null);
  const person = session.me?.id ?? null;
  const ready = session.phase === 'ready';
  const modelRef = useRef(model);
  modelRef.current = model;
  const settingsRef = useRef(settings);
  settingsRef.current = settings;
  const wasConnected = useRef(false);

  const load = useCallback(
    async (mode: 'lay-out' | 'in-place') => {
      if (!person) return;
      try {
        const answer = await loadFeed(person);
        if (!answer.counts) {
          dispatch({ type: 'unavailable', notice: answer.notice ?? 'Catching up' });
        } else if (mode === 'lay-out') {
          dispatch({ type: 'loaded', rows: answer.rows, counts: answer.counts, notice: answer.notice });
        } else {
          dispatch({ type: 'refreshed', rows: answer.rows, counts: answer.counts, notice: answer.notice, inView: inViewOf });
        }
      } catch {
        dispatch({ type: 'unavailable', notice: 'Catching up' });
      }
    },
    [person],
  );

  const reloadSettings = useCallback(() => {
    if (person) loadSettings(person).then(setSettings, () => undefined);
  }, [person]);

  useEffect(() => {
    if (!ready || !person) return;
    void load('lay-out');
    reloadSettings();
  }, [ready, person, load, reloadSettings]);

  useEffect(
    () =>
      subscribeLive((message) => {
        if (!('stream' in message)) return;
        if (message.stream === 'feed') {
          dispatch({ type: 'parts', parts: message.parts, inView: inViewOf });
          if (message.parts.some((p) => p.kind === 'settings.changed')) reloadSettings();
        } else if (message.stream === 'conversations') {
          for (const part of message.parts) {
            if (part.kind === 'message.deleted') dispatch({ type: 'deleted', message: part.message.id, inView: inViewOf });
          }
        }
      }),
    [reloadSettings],
  );

  // Back on the connection: what was missed is applied without moving the list. T-056 replaces
  // this with the numbered catch-up.
  useEffect(() => {
    if (session.connected && !wasConnected.current && modelRef.current.phase !== 'loading') void load('in-place');
    wasConnected.current = session.connected;
  }, [session.connected, load]);

  const send = useCallback(
    async (action: PeopleAction) => {
      try {
        setProblem(null);
        await act(action);
      } catch (e) {
        setProblem(say(e));
        void load('in-place');
        throw e;
      }
    },
    [load],
  );

  const quiet = useCallback(
    (action: PeopleAction) => {
      send(action).catch(() => undefined);
    },
    [send],
  );

  const { read } = session;

  const markRead = useCallback(
    (row: FeedRow) => {
      dispatch({ type: 'read', key: rowKey(row) });
      read(row.conversation, row.thread, row.newest);
    },
    [read],
  );

  const markAllRead = useCallback(
    (section: Section) => {
      // Kept rows stay kept; this is for what is new in the section (D-003).
      for (const s of modelRef.current.shown) {
        if (s.section === section && s.row.unread) markRead(s.row);
      }
    },
    [markRead],
  );

  const keep = useCallback(
    (row: FeedRow) => {
      if (!person) return;
      dispatch({ type: 'keep', key: rowKey(row) });
      quiet({ kind: 'keep', id: newId('act'), as: person, conversation: row.conversation, thread: row.thread });
    },
    [person, quiet],
  );

  const done = useCallback(
    (row: FeedRow) => {
      if (!person) return;
      dispatch({ type: 'done', key: rowKey(row) });
      quiet({ kind: 'done', id: newId('act'), as: person, conversation: row.conversation, thread: row.thread });
    },
    [person, quiet],
  );

  const react = useCallback(
    (row: FeedRow, emoji: string) => {
      if (!person) return;
      const on = !row.reactions.some((r) => r.emoji === emoji && r.people.includes(person));
      const release = (settingsRef.current?.release_on_act ?? true) && row.kept !== null;
      dispatch({ type: 'reacted', key: rowKey(row), emoji, me: person, on, release });
      quiet({ kind: 'react', id: newId('act'), as: person, message: row.message, emoji, on });
    },
    [person, quiet],
  );

  const perform = useCallback(
    async (change: RowAct) => {
      if (!person) return;
      await send(rowActToAction(change, person));
      if (change.kind === 'keep') dispatch({ type: 'keep', key: rowKey(change) });
      if (change.kind === 'done') dispatch({ type: 'done', key: rowKey(change) });
      if (change.kind !== 'keep' && change.kind !== 'done') reloadSettings();
    },
    [person, send, reloadSettings],
  );

  const saveSetting = useCallback(
    async (action: RowAct | Record<string, unknown>, change: (s: SettingsAnswer) => SettingsAnswer) => {
      if (!person) return;
      const before = settingsRef.current;
      if (before) setSettings(change(before));
      try {
        await send({ ...action, id: newId('act'), as: person } as PeopleAction);
      } catch (e) {
        setSettings(before);
        throw e;
      }
    },
    [person, send],
  );

  const show = useCallback(() => dispatch({ type: 'show' }), []);
  const enter = useCallback(() => void load('lay-out'), [load]);
  const clearProblem = useCallback(() => setProblem(null), []);

  const counted = useMemo(() => waitingBy(model), [model]);
  const counts = model.counts;

  return {
    model,
    waitingCount: model.waiting.length,
    waitingBy: counted,
    attention: counts ? counts.focus + counts.needs_you : 0,
    problem,
    clearProblem,
    settings,
    saveSetting,
    reloadSettings,
    show,
    enter,
    markRead,
    markAllRead,
    keep,
    done,
    react,
    perform,
  };
}

export type { ShownRow };
