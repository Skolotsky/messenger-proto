import { useCallback, useEffect, useMemo, useReducer, useRef } from 'react';
import { act, loadMessages, loadStart, newId } from '../api/door';
import { connectLive } from '../api/live';
import type { ChatMessage, ConversationPart, ConversationSummary, Level, PersonView, StartAnswer } from '../api/types';
import { applyToConversations, applyToLists, listKey, mergePage, upsert, type Lists } from './lists';

interface State {
  phase: 'loading' | 'ready' | 'failed';
  start: StartAnswer | null;
  conversations: ConversationSummary[];
  lists: Lists;
  connected: boolean;
  generation: number;
}

type Action =
  | { type: 'started'; start: StartAnswer }
  | { type: 'failed' }
  | { type: 'list'; key: string; rows: Lists[string] }
  | { type: 'parts'; parts: ConversationPart[]; me: string; open: string | null }
  | { type: 'own'; key: string; message: ChatMessage }
  | { type: 'read'; conversation: string }
  | { type: 'connected'; connected: boolean }
  | { type: 'preference'; key: string; value: string }
  | { type: 'reload' };

const initial: State = { phase: 'loading', start: null, conversations: [], lists: {}, connected: false, generation: 0 };

function reduce(state: State, action: Action): State {
  switch (action.type) {
    case 'started':
      return { ...state, phase: 'ready', start: action.start, conversations: action.start.conversations };
    case 'failed':
      return { ...state, phase: 'failed' };
    case 'list':
      return { ...state, lists: { ...state.lists, [action.key]: mergePage(state.lists[action.key], action.rows) } };
    case 'parts': {
      let { lists, conversations } = state;
      for (const part of action.parts) {
        lists = applyToLists(lists, part);
        conversations = applyToConversations(conversations, part, action.me, action.open);
      }
      return { ...state, lists, conversations };
    }
    case 'own': {
      const rows = state.lists[action.key];
      return rows ? { ...state, lists: { ...state.lists, [action.key]: upsert(rows, action.message) } } : state;
    }
    case 'read':
      return {
        ...state,
        conversations: state.conversations.map((c) =>
          c.conversation === action.conversation && c.unread ? { ...c, unread: false } : c,
        ),
      };
    case 'connected':
      return { ...state, connected: action.connected };
    case 'preference': {
      if (!state.start) return state;
      const preferences = [
        ...state.start.preferences.filter((p) => p.key !== action.key),
        { key: action.key, value: action.value },
      ];
      return { ...state, start: { ...state.start, preferences } };
    }
    case 'reload':
      return { ...initial, generation: state.generation + 1 };
  }
}

export interface SendInput {
  conversation: string;
  thread: string | null;
  text: string;
  level: Level | null;
  /** The same IDs on a retry, so that a send whose answer was lost is saved once. */
  actionId: string;
  messageId: string;
}

export interface Session {
  phase: State['phase'];
  orgName: string;
  me: PersonView | null;
  people: Map<string, PersonView>;
  peopleList: PersonView[];
  conversations: ConversationSummary[];
  lists: Lists;
  connected: boolean;
  sendKey: 'enter' | 'ctrl_enter';
  drafts: Map<string, string>;
  loadList(conversation: string, thread: string | null): Promise<void>;
  send(input: SendInput): Promise<void>;
  edit(message: ChatMessage, text: string): Promise<void>;
  read(conversation: string, thread: string | null, upTo: number): void;
  setOpenConversation(conversation: string | null): void;
  setPreference(key: string, value: string): Promise<void>;
  retry(): void;
}

export function useSession(person: string): Session {
  const [state, dispatch] = useReducer(reduce, initial);
  const openRef = useRef<string | null>(null);
  const drafts = useRef(new Map<string, string>());
  const readSent = useRef(new Map<string, number>());

  useEffect(() => {
    let live: (() => void) | undefined;
    let cancelled = false;
    readSent.current.clear();
    loadStart(person)
      .then((start) => {
        if (cancelled) return;
        dispatch({ type: 'started', start });
        live = connectLive(person, start.reset, start.number, {
          onParts: (parts) => dispatch({ type: 'parts', parts, me: person, open: openRef.current }),
          onReset: () => dispatch({ type: 'reload' }),
          onConnected: (connected) => dispatch({ type: 'connected', connected }),
        });
      })
      .catch(() => {
        if (!cancelled) dispatch({ type: 'failed' });
      });
    return () => {
      cancelled = true;
      live?.();
    };
  }, [person, state.generation]);

  const loadList = useCallback(async (conversation: string, thread: string | null) => {
    const page = await loadMessages(conversation, thread);
    dispatch({ type: 'list', key: listKey(conversation, thread), rows: page.messages });
  }, []);

  const send = useCallback(
    async ({ conversation, thread, text, level, actionId, messageId }: SendInput) => {
      const saved = await act({
        kind: 'send',
        id: actionId,
        as: person,
        message: { id: messageId, conversation, thread, text, mentions: [], level },
      });
      // The answer is enough to show it, also if the live connection is away right now.
      const message: ChatMessage = {
        id: messageId,
        number: saved.number ?? Number.MAX_SAFE_INTEGER,
        version: 1,
        conversation,
        thread,
        author: person,
        author_is_agent: false,
        participants: null,
        text,
        mentions: [],
        level,
        reactions: [],
        stopped: false,
        created_at: new Date().toISOString(),
        edited_at: null,
        deleted: false,
      };
      dispatch({ type: 'own', key: listKey(conversation, thread), message });
    },
    [person],
  );

  const edit = useCallback(
    async (message: ChatMessage, text: string) => {
      await act({
        kind: 'edit',
        id: newId('act'),
        as: person,
        message: message.id,
        text,
        mentions: message.mentions,
        level: message.level,
      });
      const changed = { ...message, text, version: message.version + 1, edited_at: new Date().toISOString() };
      // A root message is in its channel's list and in its thread's: both change.
      const part = { kind: 'message.changed', message: changed } as unknown as ConversationPart;
      dispatch({ type: 'parts', parts: [part], me: person, open: openRef.current });
    },
    [person],
  );

  const read = useCallback(
    (conversation: string, thread: string | null, upTo: number) => {
      const key = listKey(conversation, thread);
      if ((readSent.current.get(key) ?? 0) >= upTo) return;
      readSent.current.set(key, upTo);
      if (thread === null) dispatch({ type: 'read', conversation });
      act({ kind: 'read', id: newId('act'), as: person, conversation, thread, up_to: upTo }).catch(() => {
        // Not read after all: it is said again the next time the conversation is opened.
        readSent.current.delete(key);
      });
    },
    [person],
  );

  const setOpenConversation = useCallback((conversation: string | null) => {
    openRef.current = conversation;
  }, []);

  const setPreference = useCallback(
    async (key: string, value: string) => {
      await act({ kind: 'preference', id: newId('act'), as: person, key, value });
      dispatch({ type: 'preference', key, value });
    },
    [person],
  );

  const retry = useCallback(() => dispatch({ type: 'reload' }), []);

  const start = state.start;
  const people = useMemo(() => new Map((start?.people ?? []).map((p) => [p.id, p])), [start]);
  const sendKey = start?.preferences.find((p) => p.key === 'send_key')?.value === 'enter' ? 'enter' : 'ctrl_enter';

  return {
    phase: state.phase,
    orgName: start?.org_name ?? '',
    me: start?.person ?? null,
    people,
    peopleList: start?.people ?? [],
    conversations: state.conversations,
    lists: state.lists,
    connected: state.connected,
    sendKey,
    drafts: drafts.current,
    loadList,
    send,
    edit,
    read,
    setOpenConversation,
    setPreference,
    retry,
  };
}
