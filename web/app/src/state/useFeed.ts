import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from 'react';
import { act, DoorError, loadFeed, loadSettings, newId } from '../api/door';
import { subscribeLive } from '../api/live';
import { NOTICE_AFTER_MS } from '../config';
import type { FeedRow, PeopleAction, RowAct, SettingsAnswer } from '../api/types';
import { emptyFeed, feedReduce, rowKey, waitingBy, type FeedModel, type Section, type ShownRow } from './feedModel';
import type { Session } from './useSession';

const BEHIND_AFTER_MS = NOTICE_AFTER_MS + 500;

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

export interface ActivityMarks {
  focus: number;
  needs: number;
  needsQuiet: boolean;
}

export interface Feed {
  model: FeedModel;
  waitingCount: number;
  waitingBy: ReturnType<typeof waitingBy>;
  /** The mark on Activity: unread Focus and Needs you, the second a quiet dot under Focus only (D-037). */
  marks: ActivityMarks;
  /** Rises with every full load: what hangs on the feed (history tabs, settings) loads again. */
  freshLoads: number;
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
  const [settingsFailed, setSettingsFailed] = useState(false);
  const person = session.me?.id ?? null;
  const ready = session.phase === 'ready';
  const modelRef = useRef(model);
  modelRef.current = model;
  const settingsRef = useRef(settings);
  settingsRef.current = settings;
  const { link } = session;
  const [freshLoads, setFreshLoads] = useState(0);

  const load = useCallback(
    async (mode: 'lay-out' | 'in-place', takePosition = false) => {
      if (!person) return;
      try {
        const answer = await loadFeed(person);
        if (!answer.counts) {
          dispatch({ type: 'unavailable', notice: answer.notice ?? 'Catching up' });
          link?.feedAway();
          return;
        }
        if (mode === 'lay-out') {
          dispatch({ type: 'loaded', rows: answer.rows, counts: answer.counts, notice: answer.notice });
        } else {
          dispatch({ type: 'refreshed', rows: answer.rows, counts: answer.counts, notice: answer.notice, inView: inViewOf });
        }
        // Only a full load sets the stream's position; opening Activity again does not.
        if (takePosition && answer.number !== null && answer.edition !== null) {
          link?.feedLoaded(answer.number, answer.edition);
          setFreshLoads((n) => n + 1);
        }
      } catch {
        dispatch({ type: 'unavailable', notice: 'Catching up' });
        link?.feedAway();
      }
    },
    [person, link],
  );

  // Nothing tells an open screen that the feed fell behind, so it asks: once when something happened
  // and 10 s have passed, and every 5 s while it is saying so (D-007, G-019). Only the notice is taken
  // from the answer: the rows keep following the numbered stream.
  const connected = useRef(session.connected);
  connected.current = session.connected;
  const checkBehind = useCallback(async () => {
    if (!person || !connected.current) return;
    try {
      const answer = await loadFeed(person);
      if (!answer.counts) {
        dispatch({ type: 'notice', notice: answer.notice ?? 'Catching up' });
        link?.feedAway();
      } else {
        dispatch({ type: 'notice', notice: answer.notice });
      }
    } catch {
      dispatch({ type: 'notice', notice: 'Catching up' });
      link?.feedAway();
    }
  }, [person, link]);

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    let latest = 0;
    // Asked once the newest change is old enough to count as late, and again for anything newer.
    const due = () => {
      const left = latest + BEHIND_AFTER_MS - Date.now();
      if (left > 0) {
        timer = setTimeout(due, left);
        return;
      }
      timer = undefined;
      void checkBehind();
    };
    const off = subscribeLive((message) => {
      if (!('stream' in message) || message.stream !== 'conversations') return;
      latest = Date.now();
      if (timer === undefined) timer = setTimeout(due, BEHIND_AFTER_MS);
    });
    return () => {
      off();
      clearTimeout(timer);
    };
  }, [checkBehind]);

  const saying = Boolean(model.notice);
  useEffect(() => {
    if (!saying) return;
    const timer = setInterval(() => void checkBehind(), 5000);
    return () => clearInterval(timer);
  }, [saying, checkBehind]);

  // A mention in a channel the person is not in reaches them only through the feed, so no update
  // announces it: a slow regular question covers that case.
  useEffect(() => {
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') void checkBehind();
    }, NOTICE_AFTER_MS);
    return () => clearInterval(timer);
  }, [checkBehind]);

  const reloadSettings = useCallback(() => {
    if (!person) return;
    loadSettings(person).then(
      (answer) => {
        setSettings(answer);
        setSettingsFailed(false);
      },
      () => setSettingsFailed(true),
    );
  }, [person]);

  useEffect(() => {
    if (!ready || !person || !link) return;
    // The first load, and any fresh one the connection asks for (too far behind, a new edition,
    // the feed back after being unavailable): what is on screen stays where it is.
    link.setFeedLoader(() => load(modelRef.current.phase === 'ready' ? 'in-place' : 'lay-out', true));
    link.loadFeed();
    reloadSettings();
    return () => link.setFeedLoader(null);
  }, [ready, person, link, load, reloadSettings]);

  useEffect(() => {
    if (freshLoads > 1) reloadSettings();
  }, [freshLoads, reloadSettings]);

  const { onActionProblem } = session;
  useEffect(() => {
    onActionProblem((text) => {
      setProblem(text);
      void load('in-place');
    });
    return () => onActionProblem(null);
  }, [onActionProblem, load]);

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

  const { read, rowAction } = session;

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
      rowAction({ kind: 'keep', id: newId('act'), as: person, conversation: row.conversation, thread: row.thread });
    },
    [person, rowAction],
  );

  const done = useCallback(
    (row: FeedRow) => {
      if (!person) return;
      dispatch({ type: 'done', key: rowKey(row) });
      rowAction({ kind: 'done', id: newId('act'), as: person, conversation: row.conversation, thread: row.thread });
    },
    [person, rowAction],
  );

  const react = useCallback(
    (row: FeedRow, emoji: string) => {
      if (!person) return;
      const on = !row.reactions.some((r) => r.emoji === emoji && r.people.includes(person));
      const release = (settingsRef.current?.release_on_act ?? true) && row.kept !== null;
      dispatch({ type: 'reacted', key: rowKey(row), emoji, me: person, on, release });
      rowAction({ kind: 'react', id: newId('act'), as: person, message: row.message, emoji, on });
    },
    [person, rowAction],
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
  // Until the setting is known nothing is counted, so Focus only never flashes the larger number.
  // If it cannot be loaded, the default applies: better to call than to hide.
  const focusOnly = settings?.focus_only ?? false;
  const known = settings !== null || settingsFailed;
  const marks: ActivityMarks = {
    focus: counts && known ? counts.focus : 0,
    needs: counts && known ? counts.needs_you : 0,
    needsQuiet: focusOnly,
  };

  return {
    model,
    waitingCount: model.waiting.length,
    waitingBy: counted,
    marks,
    freshLoads,
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
