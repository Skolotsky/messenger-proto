import type { ChatMessage, ConversationPart, ConversationSummary, MessageRow, ThreadSummary } from '../api/types';

export type Lists = Record<string, MessageRow[]>;

export const listKey = (conversation: string, thread: string | null) =>
  thread ? `${conversation}|${thread}` : conversation;

const byNumber = (a: MessageRow, b: MessageRow) => a.message.number - b.message.number;

/** Puts a message in a list. An older version never replaces a newer one. */
export function upsert(rows: MessageRow[], message: ChatMessage, summary?: ThreadSummary | null): MessageRow[] {
  const at = rows.findIndex((r) => r.message.id === message.id);
  if (at < 0) return [...rows, { message, thread_summary: summary ?? null }].sort(byNumber);
  const old = rows[at];
  if (message.version < old.message.version) return rows;
  const next = [...rows];
  next[at] = { message, thread_summary: summary === undefined ? old.thread_summary : summary };
  return next.sort(byNumber);
}

/** Merges a loaded page with what arrived while it was on its way. */
export function mergePage(current: MessageRow[] | undefined, page: MessageRow[]): MessageRow[] {
  let rows = page;
  for (const row of current ?? []) {
    // A marker held on screen goes when the person moves on (D-004).
    if (row.message.deleted && !row.thread_summary?.replies) continue;
    const loaded = page.find((r) => r.message.id === row.message.id);
    if (!loaded || row.message.version > loaded.message.version) rows = upsert(rows, row.message, row.thread_summary);
  }
  return [...rows].sort(byNumber);
}

function mapMessage(lists: Lists, id: string, change: (row: MessageRow) => MessageRow | null): Lists {
  let next = lists;
  for (const [key, rows] of Object.entries(lists)) {
    const at = rows.findIndex((r) => r.message.id === id);
    if (at < 0) continue;
    const changed = change(rows[at]);
    const copy = [...rows];
    if (changed) copy[at] = changed;
    else copy.splice(at, 1);
    next = { ...next, [key]: copy };
  }
  return next;
}

/** The person's own delete: without replies nothing stays, not even for them (D-004, D-024). */
export function dropOwnDeleted(lists: Lists, id: string): Lists {
  return mapMessage(lists, id, (row) => (row.thread_summary?.replies ? row : null));
}

/** Applies one live part to the loaded lists. A part this screen does not know changes nothing. */
export function applyToLists(lists: Lists, part: ConversationPart): Lists {
  switch (part.kind) {
    case 'message.created': {
      const message = part.message as unknown as ChatMessage;
      const key = listKey(message.conversation, message.thread);
      const rows = lists[key];
      return rows ? { ...lists, [key]: upsert(rows, message) } : lists;
    }
    case 'message.changed': {
      const message = part.message as unknown as ChatMessage;
      return mapMessage(lists, message.id, (row) =>
        message.version >= row.message.version ? { ...row, message } : row,
      );
    }
    case 'message.deleted': {
      const gone = part.message;
      return mapMessage(lists, gone.id, (row) => {
        if (gone.version < row.message.version) return row;
        // D-004: it keeps its space for whoever has it on screen; without replies it is gone at the next load.
        return {
          ...row,
          message: { ...row.message, version: gone.version, deleted: true, text: '', mentions: [], reactions: [] },
        };
      });
    }
    case 'reaction.changed':
      return mapMessage(lists, part.message, (row) => ({
        ...row,
        message: { ...row.message, reactions: part.reactions },
      }));
    case 'thread.summary':
      return mapMessage(lists, part.thread, (row) => ({
        ...row,
        thread_summary: {
          replies: part.replies,
          last_reply_at: part.last_reply_at,
          people: part.people,
          unread: row.thread_summary?.unread ?? false,
        },
      }));
    default:
      return lists;
  }
}

/**
 * The sidebar's unread marks. The contract's `unread` part is the truth. The mock door does not
 * send it, so a top-level arrival from someone else also marks a conversation not being looked at.
 */
export function applyToConversations(
  conversations: ConversationSummary[],
  part: ConversationPart,
  me: string,
  open: string | null,
): ConversationSummary[] {
  const mark = (id: string, unread: boolean) =>
    conversations.map((c) => (c.conversation === id && c.unread !== unread ? { ...c, unread } : c));
  switch (part.kind) {
    case 'unread':
      return part.thread === null ? mark(part.conversation, part.unread) : conversations;
    case 'message.created': {
      const m = part.message as unknown as ChatMessage;
      return m.thread === null && m.author !== me && m.conversation !== open
        ? mark(m.conversation, true)
        : conversations;
    }
    case 'conversation.added': {
      const summary = part.conversation_summary;
      const known = conversations.some((c) => c.conversation === summary.conversation);
      return known || !summary.people.includes(me) ? conversations : [...conversations, summary];
    }
    default:
      return conversations;
  }
}
