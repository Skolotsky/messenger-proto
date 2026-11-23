import { useCallback, useEffect, useMemo, useReducer, useRef, useState, useSyncExternalStore } from 'react';
import { act, DoorError, loadMessages, loadReplyText, loadStart, newId } from '../api/door';
import { goToEntry } from '../api/entry';
import { LiveLink } from '../api/live';
import { takePiece } from '../api/stream';
import type {
  ChatMessage,
  ConversationPart,
  ConversationSummary,
  Level,
  MessagesPage,
  PeopleAction,
  PersonView,
  ReplyInProgress,
  StartAnswer,
} from '../api/types';
import {
  applyToConversations,
  applyToLists,
  applyToReplies,
  applyToStart,
  dropOwnDeleted,
  listKey,
  mergePage,
  upsert,
  type Lists,
  type RepliesInProgress,
} from './lists';
import { Outbox, type OutItem, type OutResult } from './outbox';

interface State {
  phase: 'loading' | 'ready' | 'failed';
  start: StartAnswer | null;
  conversations: ConversationSummary[];
  lists: Lists;
  connected: boolean;
  /** When the connection was lost, while it is. */
  offlineSince: number | null;
  replies: RepliesInProgress;
  generation: number;
}

type Action =
  | { type: 'started'; start: StartAnswer }
  | { type: 'restarted'; start: StartAnswer }
  | { type: 'replyText'; reply: ReplyInProgress }
  | { type: 'agentStatus'; agent: string; state: string; on: string | null }
  | { type: 'unreadAgain'; conversation: string }
  | { type: 'failed' }
  | { type: 'list'; key: string; rows: Lists[string]; replace?: boolean }
  | { type: 'ownDeleted'; id: string }
  | { type: 'parts'; parts: ConversationPart[]; me: string; drop?: string[] }
  | { type: 'own'; key: string; message: ChatMessage }
  | { type: 'read'; conversation: string }
  | { type: 'connected'; connected: boolean }
  | { type: 'preference'; key: string; value: string }
  | { type: 'reload' };

const initial: State = {
  phase: 'loading',
  start: null,
  conversations: [],
  lists: {},
  connected: false,
  offlineSince: null,
  replies: {},
  generation: 0,
};

function reduce(state: State, action: Action): State {
  switch (action.type) {
    case 'started':
      return { ...state, phase: 'ready', start: action.start, conversations: action.start.conversations };
    case 'restarted':
      return { ...state, start: action.start, conversations: action.start.conversations };
    case 'replyText':
      return { ...state, replies: { ...state.replies, [action.reply.message]: action.reply } };
    case 'agentStatus': {
      if (!state.start) return state;
      const status = { state: action.state, on: action.on } as PersonView['status'];
      const change = (p: PersonView) => (p.id === action.agent ? { ...p, status } : p);
      return { ...state, start: { ...state.start, person: change(state.start.person), people: state.start.people.map(change) } };
    }
    case 'unreadAgain':
      return {
        ...state,
        conversations: state.conversations.map((c) => (c.conversation === action.conversation ? { ...c, unread: true } : c)),
      };
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
      let { lists, conversations, start, replies } = state;
      for (const part of action.parts) {
        lists = applyToLists(lists, part);
        conversations = applyToConversations(conversations, part, action.me);
        replies = applyToReplies(replies, part);
        if (start) start = applyToStart(start, part);
      }
      // Created and deleted while away: the person never saw it, so it leaves no marker (D-004).
      for (const id of action.drop ?? []) lists = dropOwnDeleted(lists, id);
      return { ...state, lists, conversations, start, replies };
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
      return {
        ...state,
        connected: action.connected,
        offlineSince: action.connected ? null : (state.offlineSince ?? Date.now()),
      };
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
  mentions?: string[];
  level: Level | null;
  /** Chosen once: it waits, and is sent, with these (screen-to-conversations.md). */
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
  offlineSince: number | null;
  /** What the person did that the door has not confirmed, in the order they did it. */
  outbox: OutItem[];
  replies: RepliesInProgress;
  link: LiveLink | null;
  sendKey: 'enter' | 'ctrl_enter';
  drafts: Map<string, string>;
  /** `around` opens at a position; `replace` shows exactly that page instead of joining it to what is held. */
  loadList(
    conversation: string,
    thread: string | null,
    options?: { around?: number; replace?: boolean },
  ): Promise<MessagesPage>;
  /** The message stays in its place, waiting if need be; there is nothing to wait for here. */
  send(input: SendInput): void;
  edit(message: ChatMessage, change: EditChange): Promise<OutResult>;
  remove(message: ChatMessage, actionId: string): Promise<OutResult>;
  react(message: ChatMessage, emoji: string, on: boolean): void;
  /** A keep or done from a row: waits with the rest, and a failure comes to `onActionProblem`. */
  rowAction(action: PeopleAction): void;
  retryAction(id: string): void;
  discardAction(id: string): void;
  onActionProblem(listener: ((text: string) => void) | null): void;
  createChannel(name: string, people: string[], actionId: string): Promise<string>;
  openDirect(person: string, actionId: string): Promise<string>;
  read(conversation: string, thread: string | null, upTo: number): void;
  /** This row is open in front of the person, or no longer (D-015). */
  watch(conversation: string, thread: string | null, open: boolean): void;
  setPreference(key: string, value: string): Promise<void>;
  retry(): void;
}

export function useSession(person: string): Session {
  const [state, dispatch] = useReducer(reduce, initial);
  const drafts = useRef(new Map<string, string>());
  const readSent = useRef(new Map<string, number>());
  const stateRef = useRef(state);
  stateRef.current = state;
  const problemListener = useRef<((text: string) => void) | null>(null);
  const [link, setLink] = useState<LiveLink | null>(null);
  // What was loaded for each list, so that a fresh load shows the same place.
  const loaded = useRef(new Map<string, { conversation: string; thread: string | null; around?: number }>());
  // Messages that arrived since the connection was lost, until the screen is known to be level.
  const arrivedAway = useRef<Set<string> | null>(null);
  const fetchingReply = useRef(new Set<string>());
  // Updates that arrive while a list is on its way: its answer may be older than they are.
  const arriving = useRef(new Map<string, ConversationPart[]>());

  const [outbox] = useState(
    () =>
      new Outbox(act, (item, error) => {
        const reason = error.kind === 'refused' ? `${error.message}` : 'no answer from the server';
        const what = item.action.kind === 'read' ? 'Not marked read' : 'Not saved';
        if (item.action.kind === 'read') {
          const { conversation, thread } = item.action;
          readSent.current.delete(listKey(conversation, thread));
          if (thread === null) dispatch({ type: 'unreadAgain', conversation });
        }
        problemListener.current?.(`${what}: ${reason}.`);
      }),
  );
  const pending = useSyncExternalStore(outbox.subscribe, outbox.snapshot);

  useEffect(() => {
    let current: LiveLink | undefined;
    let cancelled = false;
    readSent.current.clear();

    const fetchReply = (message: string) => {
      if (fetchingReply.current.has(message)) return;
      fetchingReply.current.add(message);
      loadReplyText(message)
        .then((reply) => dispatch({ type: 'replyText', reply }))
        // Closed: the finished message or `reply.abandoned` is on its way.
        .catch(() => undefined)
        .finally(() => fetchingReply.current.delete(message));
    };

    loadStart(person)
      .then((start) => {
        if (cancelled) return;
        dispatch({ type: 'started', start });
        current = new LiveLink(person, {
          onParts: (parts) => {
            for (const held of arriving.current.values()) held.push(...parts);
            let drop: string[] | undefined;
            const away = arrivedAway.current;
            if (away) {
              for (const part of parts) {
                if (part.kind === 'message.created') away.add((part.message as unknown as ChatMessage).id);
                if (part.kind === 'message.deleted' && away.has(part.message.id)) (drop ??= []).push(part.message.id);
              }
            }
            dispatch({ type: 'parts', parts, me: person, drop });
          },
          onConnected: (connected) => {
            if (!connected) arrivedAway.current ??= new Set();
            dispatch({ type: 'connected', connected });
            outbox.setOnline(connected);
          },
          onLevel: () => {
            arrivedAway.current = null;
          },
          reloadConversations: async () => {
            const fresh = await loadStart(person);
            dispatch({ type: 'restarted', start: fresh });
            await Promise.all(
              [...loaded.current.entries()].map(async ([key, { conversation, thread, around }]) => {
                const page = await loadMessages(conversation, thread, around, person);
                dispatch({ type: 'list', key, rows: page.messages, replace: true });
                for (const reply of page.in_progress ?? []) dispatch({ type: 'replyText', reply });
              }),
            );
            return fresh.number;
          },
          onPiece: ({ message, at, text }) => {
            const held = stateRef.current.replies[message];
            const next = held ? takePiece(held, at, text) : 'gap';
            if (next === 'gap') fetchReply(message);
            else if (held && next !== held) {
              const reply = { ...held, ...next };
              // Pieces can come faster than the screen renders.
              stateRef.current = { ...stateRef.current, replies: { ...stateRef.current.replies, [message]: reply } };
              dispatch({ type: 'replyText', reply });
            }
          },
          onAgentStatus: ({ agent, state: agentState, on }) => dispatch({ type: 'agentStatus', agent, state: agentState, on }),
          onGone: () => void goToEntry(),
        });
        current.start(start.number);
        setLink(current);
      })
      .catch(() => {
        if (!cancelled) dispatch({ type: 'failed' });
      });
    return () => {
      cancelled = true;
      current?.stop();
      setLink(null);
    };
  }, [person, state.generation, outbox]);

  const loadList = useCallback(
    async (conversation: string, thread: string | null, options?: { around?: number; replace?: boolean }) => {
      const key = listKey(conversation, thread);
      loaded.current.set(key, { conversation, thread, around: options?.around });
      const held: ConversationPart[] = [];
      arriving.current.set(key, held);
      let page: MessagesPage;
      try {
        page = await loadMessages(conversation, thread, options?.around, person);
      } finally {
        if (arriving.current.get(key) === held) arriving.current.delete(key);
      }
      let list: Lists = { [key]: page.messages };
      for (const part of held) list = applyToLists(list, part);
      const replace = options?.replace || options?.around !== undefined;
      dispatch({ type: 'list', key, rows: list[key], replace });
      for (const reply of page.in_progress ?? []) dispatch({ type: 'replyText', reply });
      return page;
    },
    [person],
  );

  const send = useCallback(
    ({ conversation, thread, text, mentions = [], level, actionId, messageId }: SendInput) => {
      const draft: ChatMessage = {
        id: messageId,
        number: Number.MAX_SAFE_INTEGER,
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
      void outbox.add({
        action: { kind: 'send', id: actionId, as: person, message: { id: messageId, conversation, thread, text, mentions, level } },
        messageId,
        draft,
        // The answer is enough to show it, also if the live connection is away right now.
        onSaved: (saved) =>
          dispatch({ type: 'own', key: listKey(conversation, thread), message: { ...draft, number: saved.number ?? draft.number } }),
      });
    },
    [person, outbox],
  );

  const edit = useCallback(
    (message: ChatMessage, { text, mentions, level, actionId }: EditChange) =>
      outbox.add({
        action: { kind: 'edit', id: actionId, as: person, message: message.id, text, mentions, level },
        messageId: message.id,
        answersInPlace: true,
        onSaved: () => {
          const changed = { ...message, text, mentions, level, version: message.version + 1, edited_at: new Date().toISOString() };
          // A root message is in its channel's list and in its thread's: both change.
          const part = { kind: 'message.changed', message: changed } as unknown as ConversationPart;
          dispatch({ type: 'parts', parts: [part], me: person });
        },
      }),
    [person, outbox],
  );

  const remove = useCallback(
    (message: ChatMessage, actionId: string) =>
      outbox.add({
        action: { kind: 'delete', id: actionId, as: person, message: message.id },
        messageId: message.id,
        answersInPlace: true,
        onSaved: () => {
          const part = {
            kind: 'message.deleted',
            message: { id: message.id, version: message.version + 1, conversation: message.conversation, thread: message.thread },
          } as ConversationPart;
          dispatch({ type: 'parts', parts: [part], me: person });
          dispatch({ type: 'ownDeleted', id: message.id });
        },
      }),
    [person, outbox],
  );

  const react = useCallback(
    (message: ChatMessage, emoji: string, on: boolean) => {
      void outbox.add({
        action: { kind: 'react', id: newId('act'), as: person, message: message.id, emoji, on },
        messageId: message.id,
        onSaved: () => {
          // A channel the person is not in sends no live updates, so the answer is all there is.
          if (stateRef.current.conversations.some((c) => c.conversation === message.conversation)) return;
          const current = message.reactions.find((r) => r.emoji === emoji)?.people ?? [];
          const people = on ? [...new Set([...current, person])] : current.filter((p) => p !== person);
          const others = message.reactions.filter((r) => r.emoji !== emoji);
          const reactions = people.length ? [...others, { emoji, people }] : others;
          const part = { kind: 'reaction.changed', conversation: message.conversation, thread: message.thread, message: message.id, reactions } as ConversationPart;
          dispatch({ type: 'parts', parts: [part], me: person });
        },
      });
    },
    [person, outbox],
  );

  const rowAction = useCallback(
    (action: PeopleAction) => {
      void outbox.add({ action });
    },
    [outbox],
  );

  const retryAction = useCallback((id: string) => outbox.retry(id), [outbox]);
  const discardAction = useCallback((id: string) => outbox.discard(id), [outbox]);
  const onActionProblem = useCallback((listener: ((text: string) => void) | null) => {
    problemListener.current = listener;
  }, []);

  const addConversation = useCallback(
    (summary: ConversationSummary) => {
      const part = { kind: 'conversation.added', conversation_summary: summary } as ConversationPart;
      dispatch({ type: 'parts', parts: [part], me: person });
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
      void outbox.add({ action: { kind: 'read', id: newId('act'), as: person, conversation, thread, up_to: upTo } });
    },
    [person, outbox],
  );

  const watch = useCallback(
    (conversation: string, thread: string | null, open: boolean) => link?.watch(conversation, thread, open),
    [link],
  );

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
    offlineSince: state.offlineSince,
    outbox: pending,
    replies: state.replies,
    link,
    sendKey,
    drafts: drafts.current,
    loadList,
    send,
    edit,
    remove,
    react,
    rowAction,
    retryAction,
    discardAction,
    onActionProblem,
    createChannel,
    openDirect,
    read,
    watch,
    setPreference,
    retry,
  };
}
