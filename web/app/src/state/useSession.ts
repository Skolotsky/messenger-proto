import { useCallback, useEffect, useMemo, useReducer, useRef } from 'react';
import { act, DoorError, loadMessages, loadStart, newId } from '../api/door';
import { connectLive } from '../api/live';
import type {
  ChatMessage,
  ConversationPart,
  ConversationSummary,
  Level,
  MessagesPage,
  PersonView,
  StartAnswer,
} from '../api/types';
import { applyToConversations, applyToLists, dropOwnDeleted, listKey, mergePage, upsert, type Lists } from './lists';

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
  | { type: 'list'; key: string; rows: Lists[string]; replace?: boolean }
  | { type: 'ownDeleted'; id: string }
  | { type: 'parts'; parts: ConversationPart[]; me: string; open: string | null }
  | { type: 'own'; key: string; message: ChatMessage }
  | { type: 'read'; conversation: string }
  | { type: 'connected'; connected: boolean }
  | { type: 'reload' };

const initial: State = { phase: 'loading', start: null, conversations: [], lists: {}, connected: false, generation: 0 };

function reduce(state: State, action: Action): State {
  switch (action.type) {
    case 'started':
      return { ...state, phase: 'ready', start: action.start, conversations: action.start.conversations };
    case 'failed':
      return { ...state, phase: 'failed' };
    case 'list':
      return {
        ...state,
        lists: {
          ...state.lists,
          [action.key]: action.replace ? action.rows : mergePage(state.lists[action.key], action.rows),
        },
      };
    case 'ownDeleted':
      return { ...state, lists: dropOwnDeleted(state.lists, action.id) };
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
    case 'reload':
      return { ...initial, generation: state.generation + 1 };
  }
}

export interface SendInput {
  conversation: string;
  thread: string | null;
  text: string;
  mentions?: string[];
  level: Level | null;
  /** The same IDs on a retry, so that a send whose answer was lost is saved once. */
  actionId: string;
  messageId: string;
}

export interface EditChange {
  text: string;
  mentions: string[];
  level: Level | null;
  /** The same ID on a retry, so that an edit whose answer was lost is saved once. */
  actionId: string;
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
  /** `around` opens at a position; `replace` shows exactly that page instead of joining it to what is held. */
  loadList(
    conversation: string,
    thread: string | null,
    options?: { around?: number; replace?: boolean },
  ): Promise<MessagesPage>;
  send(input: SendInput): Promise<void>;
  edit(message: ChatMessage, change: EditChange): Promise<void>;
  remove(message: ChatMessage, actionId: string): Promise<void>;
  createChannel(name: string, people: string[], actionId: string): Promise<string>;
  openDirect(person: string, actionId: string): Promise<string>;
  read(conversation: string, thread: string | null, upTo: number): void;
  setOpenConversation(conversation: string | null): void;
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

  const loadList = useCallback(
    async (conversation: string, thread: string | null, options?: { around?: number; replace?: boolean }) => {
      const page = await loadMessages(conversation, thread, options?.around);
      const replace = options?.replace || options?.around !== undefined;
      dispatch({ type: 'list', key: listKey(conversation, thread), rows: page.messages, replace });
      return page;
    },
    [],
  );

  const send = useCallback(
    async ({ conversation, thread, text, mentions = [], level, actionId, messageId }: SendInput) => {
      const saved = await act({
        kind: 'send',
        id: actionId,
        as: person,
        message: { id: messageId, conversation, thread, text, mentions, level },
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
        mentions,
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
    async (message: ChatMessage, { text, mentions, level, actionId }: EditChange) => {
      await act({ kind: 'edit', id: actionId, as: person, message: message.id, text, mentions, level });
      const changed = {
        ...message,
        text,
        mentions,
        level,
        version: message.version + 1,
        edited_at: new Date().toISOString(),
      };
      // A root message is in its channel's list and in its thread's: both change.
      const part = { kind: 'message.changed', message: changed } as unknown as ConversationPart;
      dispatch({ type: 'parts', parts: [part], me: person, open: openRef.current });
    },
    [person],
  );

  const remove = useCallback(
    async (message: ChatMessage, actionId: string) => {
      await act({ kind: 'delete', id: actionId, as: person, message: message.id });
      const part = {
        kind: 'message.deleted',
        message: {
          id: message.id,
          version: message.version + 1,
          conversation: message.conversation,
          thread: message.thread,
        },
      } as ConversationPart;
      dispatch({ type: 'parts', parts: [part], me: person, open: openRef.current });
      dispatch({ type: 'ownDeleted', id: message.id });
    },
    [person],
  );

  const addConversation = useCallback(
    (summary: ConversationSummary) => {
      const part = { kind: 'conversation.added', conversation_summary: summary } as ConversationPart;
      dispatch({ type: 'parts', parts: [part], me: person, open: openRef.current });
    },
    [person],
  );

  const createChannel = useCallback(
    async (name: string, people: string[], actionId: string) => {
      const saved = await act({ kind: 'create_channel', id: actionId, as: person, name, people });
      const conversation = saved.conversation ?? `#${name}`;
      addConversation({ conversation, type: 'channel', name, people: [person, ...people], unread: false, dial: 'default' });
      return conversation;
    },
    [person, addConversation],
  );

  const openDirect = useCallback(
    async (other: string, actionId: string) => {
      const saved = await act({ kind: 'open_direct', id: actionId, as: person, with: other });
      if (!saved.conversation) throw new DoorError('unreachable', 'No conversation in the answer.');
      const conversation = saved.conversation;
      addConversation({ conversation, type: 'direct', name: conversation, people: [person, other], unread: false, dial: 'default' });
      return conversation;
    },
    [person, addConversation],
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
    remove,
    createChannel,
    openDirect,
    read,
    setOpenConversation,
    retry,
  };
}
