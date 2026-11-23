import { DoorError } from '../api/door';
import type { ActionSaved, ChatMessage, PeopleAction } from '../api/types';

export type OutState = 'sending' | 'waiting' | 'failed' | 'refused';

export interface OutItem {
  id: string;
  action: PeopleAction;
  state: OutState;
  reason: string | null;
  /** The message this is about, where it shows its state. */
  messageId: string | null;
  /** A new message: what stands in its place until it is saved. */
  draft: ChatMessage | null;
  onSaved(saved: ActionSaved): void;
}

export interface OutInput {
  action: PeopleAction;
  messageId?: string | null;
  draft?: ChatMessage | null;
  onSaved?(saved: ActionSaved): void;
  /** The caller shows what goes wrong while the connection is up, and offers its own Try again. */
  answersInPlace?: boolean;
}

export type OutResult = 'saved' | 'waiting';

interface Waiter {
  resolve(result: OutResult): void;
  reject(error: unknown): void;
}

const SHOWN_ON_MESSAGE = new Set<PeopleAction['kind']>(['send', 'edit', 'delete', 'react']);

/** These wait for an answer and go out again by themselves (screen-to-conversations.md, section 4). */
const WAITS_FOR_AN_ANSWER = new Set<PeopleAction['kind']>(['send', 'edit', 'delete', 'react']);
const RETRY_MS = [1000, 2000, 4000];

const rowOf = (a: PeopleAction) =>
  a.kind === 'read' ? `${a.conversation}|${a.thread ?? ''}` : null;

/**
 * What the person did and the door has not confirmed yet. Actions are sent one at a time, in the
 * order they were made, each with its ID. One with no answer stays first in line and holds back
 * what came after it, so that messages cannot overtake each other.
 */
export class Outbox {
  items: OutItem[] = [];
  private online = false;
  private running = false;
  /** How many times in a row the action at the head of the line had no answer. */
  private misses = 0;
  private pause: ReturnType<typeof setTimeout> | null = null;
  private readonly waiters = new Map<string, Waiter>();
  private readonly listeners = new Set<() => void>();

  constructor(
    private readonly post: (action: PeopleAction) => Promise<ActionSaved>,
    private readonly onProblem: (item: OutItem, error: DoorError) => void,
  ) {}

  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  snapshot = () => this.items;

  private changed(items: OutItem[]) {
    this.items = items;
    this.listeners.forEach((l) => l());
  }

  private patch(id: string, change: Partial<OutItem>) {
    this.changed(this.items.map((i) => (i.id === id ? { ...i, ...change } : i)));
  }

  private remove(id: string) {
    this.changed(this.items.filter((i) => i.id !== id));
  }

  add(input: OutInput): Promise<OutResult> {
    const { action } = input;
    const row = rowOf(action);
    if (row) {
      // One position per row: a newer read replaces a read still waiting.
      const held = this.items.find((i) => i.state === 'waiting' && rowOf(i.action) === row);
      if (held) this.changed(this.items.filter((i) => i !== held));
    }
    // Behind an action with no answer, this one waits its turn and says so.
    const direct = this.online && this.misses === 0;
    const item: OutItem = {
      id: action.id,
      action,
      state: direct ? 'sending' : 'waiting',
      reason: null,
      messageId: input.messageId ?? null,
      draft: input.draft ?? null,
      onSaved: input.onSaved ?? (() => undefined),
    };
    this.changed([...this.items, item]);
    const result = new Promise<OutResult>((resolve, reject) => {
      if (input.answersInPlace && direct) this.waiters.set(item.id, { resolve, reject });
      else resolve(direct ? 'saved' : 'waiting');
    });
    void this.pump();
    return result;
  }

  setOnline(online: boolean) {
    this.online = online;
    this.endPause();
    if (!online) {
      // Whatever has no answer yet waits for the connection, and its message says so.
      this.misses = 0;
      this.changed(
        this.items.map((i) => (i.state === 'sending' || i.state === 'waiting' ? { ...i, state: 'waiting', reason: null } : i)),
      );
      for (const waiter of this.waiters.values()) waiter.resolve('waiting');
      this.waiters.clear();
      return;
    }
    this.changed(this.items.map((i) => (i.state === 'failed' ? { ...i, state: 'waiting', reason: null } : i)));
    void this.pump();
  }

  retry(id: string) {
    this.endPause();
    this.patch(id, { state: 'waiting', reason: null });
    void this.pump();
  }

  discard(id: string) {
    this.waiters.get(id)?.resolve('saved');
    this.waiters.delete(id);
    if (this.next()?.id === id) {
      this.misses = 0;
      this.endPause();
    }
    this.remove(id);
    void this.pump();
  }

  private endPause() {
    if (this.pause !== null) clearTimeout(this.pause);
    this.pause = null;
  }

  private next(): OutItem | null {
    for (const item of this.items) {
      if (item.state === 'sending' || item.state === 'waiting') return item;
    }
    return null;
  }

  private async pump() {
    if (this.running) return;
    this.running = true;
    try {
      for (;;) {
        if (!this.online || this.pause !== null) return;
        const item = this.next();
        if (!item) return;
        // Tried again, it keeps saying that it waits, so the line under the message does not flicker.
        if (item.state !== 'sending' && !item.reason) this.patch(item.id, { state: 'sending' });
        try {
          const saved = await this.post(item.action);
          item.onSaved(saved);
          this.misses = 0;
          this.remove(item.id);
          this.settle(item.id, 'resolve', 'saved');
        } catch (e) {
          if (!this.items.some((i) => i.id === item.id)) continue; // discarded on the way
          const error = e instanceof DoorError ? e : new DoorError('unreachable', 'No answer from the server.');
          this.failed(item, error);
        }
      }
    } finally {
      this.running = false;
    }
  }

  private settle(id: string, how: 'resolve' | 'reject', value: unknown) {
    const waiter = this.waiters.get(id);
    if (!waiter) return false;
    this.waiters.delete(id);
    if (how === 'resolve') waiter.resolve(value as OutResult);
    else waiter.reject(value);
    return true;
  }

  private failed(item: OutItem, error: DoorError) {
    const refused = error.kind === 'refused';
    if (!refused && !this.online) {
      this.patch(item.id, { state: 'waiting' });
      this.settle(item.id, 'resolve', 'waiting');
      return;
    }
    if (!refused && WAITS_FOR_AN_ANSWER.has(item.action.kind)) {
      this.waitForAnswer(item, error);
      return;
    }
    this.misses = 0;
    // The form that made it is still open and says it there.
    if (this.waiters.has(item.id)) {
      this.remove(item.id);
      this.settle(item.id, 'reject', error);
      return;
    }
    if (item.messageId && SHOWN_ON_MESSAGE.has(item.action.kind)) {
      this.patch(item.id, { state: refused ? 'refused' : 'failed', reason: error.message });
      return;
    }
    this.remove(item.id);
    this.onProblem(item, error);
  }

  /** No answer with the connection up: it stays first in line and is sent again, with its ID, after a pause. */
  private waitForAnswer(item: OutItem, error: DoorError) {
    const wait = RETRY_MS[Math.min(this.misses, RETRY_MS.length - 1)];
    this.misses += 1;
    this.changed(
      this.items.map((i) => {
        if (i.id === item.id) return { ...i, state: 'waiting' as const, reason: error.message };
        return i.state === 'sending' ? { ...i, state: 'waiting' as const } : i;
      }),
    );
    for (const id of [...this.waiters.keys()]) this.settle(id, 'resolve', 'waiting');
    this.pause = setTimeout(() => {
      this.pause = null;
      void this.pump();
    }, wait * (0.8 + Math.random() * 0.2));
  }
}

/** A message as the person expects it, with what they did that is not saved yet. */
export function withPending(message: ChatMessage, items: OutItem[], me: string | undefined): ChatMessage {
  let shown = message;
  for (const { action, messageId, state } of items) {
    if (messageId !== message.id || state === 'refused' || state === 'failed') continue;
    if (action.kind === 'edit') {
      shown = { ...shown, text: action.text, mentions: action.mentions, level: action.level, edited_at: shown.edited_at ?? new Date().toISOString() };
    } else if (action.kind === 'react' && me) {
      const current = shown.reactions.find((r) => r.emoji === action.emoji)?.people ?? [];
      const people = action.on ? [...new Set([...current, me])] : current.filter((p) => p !== me);
      const others = shown.reactions.filter((r) => r.emoji !== action.emoji);
      shown = { ...shown, reactions: people.length ? [...others, { emoji: action.emoji, people }] : others };
    }
  }
  return shown;
}
