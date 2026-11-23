import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { DoorError, newId } from '../api/door';
import type { ChatMessage, MessageRow } from '../api/types';
import { mentionsIn } from '../state/mentions';
import type { OutItem } from '../state/outbox';
import type { Session } from '../state/useSession';
import { AutoTextarea } from './AutoTextarea';
import { Icon } from './Icon';
import { useMentionBox } from './Mentions';
import { WritingIcon } from './WritingIcon';

/** What went wrong with an action, and whether trying again can help (screen-to-conversations.md, section 4). */
export interface Problem {
  text: string;
  canRetry: boolean;
}

export function problemOf(e: unknown, what: string): Problem {
  return e instanceof DoorError && e.kind === 'refused'
    ? { text: `${what}: ${e.message}.`, canRetry: false }
    : { text: `${what}: no answer from the server.`, canRetry: true };
}

/** Said in the place where it happened. A refusal is final, so it offers no Try again. */
export function ActionProblem({
  problem,
  onRetry,
  onDiscard,
  waiting = false,
}: {
  problem: Problem;
  onRetry(): void;
  /** Left out where the form already has its own way out. */
  onDiscard?(): void;
  /** It is not over: the action goes out again by itself. */
  waiting?: boolean;
}) {
  const el = useRef<HTMLDivElement>(null);
  // The line appears under the message: at the bottom of a list it would otherwise be out of view.
  useEffect(() => el.current?.scrollIntoView({ block: 'nearest' }), [problem.text]);
  return (
    <div className={`send-state ${waiting ? 'waiting' : 'fail'}`} role="alert" ref={el}>
      {problem.text}{' '}
      {problem.canRetry && (
        <button type="button" className="link" onClick={onRetry}>
          Try again
        </button>
      )}
      {problem.canRetry && onDiscard && ' · '}
      {onDiscard && (
        <button type="button" className="link" onClick={onDiscard}>
          Discard
        </button>
      )}
    </div>
  );
}

const WAITING: Partial<Record<string, string>> = {
  send: 'Waiting to send',
  edit: 'Edit waiting to be saved',
  delete: 'Delete waiting to be saved',
  react: 'Reaction waiting to be saved',
};

const NOT_DONE: Partial<Record<string, string>> = {
  send: 'Not sent',
  edit: 'Not saved',
  delete: 'Not deleted',
  react: 'Reaction not saved',
};

/** What is waiting on this message, or did not go through, said under it (D-024; T-038 point 26). */
export function OutLines({ session, items }: { session: Session; items: OutItem[] }) {
  return (
    <>
      {items.map((item) => {
        const kind = item.action.kind;
        if (item.state === 'waiting' && item.reason) {
          return (
            <ActionProblem
              key={item.id}
              waiting
              problem={{ text: `${WAITING[kind]}: no answer from the server.`, canRetry: true }}
              onRetry={() => session.retryAction(item.id)}
              onDiscard={() => session.discardAction(item.id)}
            />
          );
        }
        if (item.state === 'waiting') {
          return (
            <div className="send-state waiting" role="status" key={item.id}>
              {WAITING[kind]}{' '}
              <button type="button" className="link" onClick={() => session.discardAction(item.id)}>
                Discard
              </button>
            </div>
          );
        }
        if (item.state === 'sending') return null;
        const refused = item.state === 'refused';
        return (
          <ActionProblem
            key={item.id}
            problem={{ text: `${NOT_DONE[kind]}: ${refused ? item.reason : 'no answer from the server'}.`, canRetry: !refused }}
            onRetry={() => session.retryAction(item.id)}
            onDiscard={() => session.discardAction(item.id)}
          />
        );
      })}
    </>
  );
}

/** Whether a conversation can have "channel" mentioned: a direct message cannot. */
export const allowsEveryone = (session: Session, conversation: string) =>
  session.conversations.find((c) => c.conversation === conversation)?.type !== 'direct';

/** Remove flag and Delete on one's own message (D-024). The same action ID on a retry, so it is saved once. */
export function useOwnMessage(session: Session, m: ChatMessage) {
  const [asking, setAsking] = useState(false);
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  const attempt = useRef<{ what: 'delete' | 'unflag'; id: string } | null>(null);

  const run = async (what: 'delete' | 'unflag') => {
    if (busy) return;
    if (attempt.current?.what !== what) attempt.current = { what, id: newId('act') };
    const actionId = attempt.current.id;
    setBusy(true);
    setProblem(null);
    try {
      if (what === 'delete') await session.remove(m, actionId);
      else await session.edit(m, { text: m.text, mentions: m.mentions, level: null, actionId });
      attempt.current = null;
      setAsking(false);
    } catch (e) {
      const found = problemOf(e, what === 'delete' ? 'Not deleted' : 'Flag not removed');
      if (!found.canRetry) attempt.current = null;
      setProblem(found);
    } finally {
      setBusy(false);
    }
  };

  return {
    asking,
    problem,
    busy,
    ask: () => {
      setProblem(null);
      setAsking(true);
    },
    confirmDelete: () => run('delete'),
    removeFlag: () => run('unflag'),
    retry: () => run(attempt.current?.what ?? 'unflag'),
    discard: () => {
      attempt.current = null;
      setProblem(null);
      setAsking(false);
    },
  };
}

export type OwnMessage = ReturnType<typeof useOwnMessage>;

export function OwnTools({ own, flagged, onEdit }: { own: OwnMessage; flagged: boolean; onEdit(): void }) {
  return (
    <>
      {flagged && (
        <button aria-label="Remove the flag" title="Remove the flag" onClick={own.removeFlag}>
          <WritingIcon name="unflag" />
        </button>
      )}
      <button aria-label="Edit" title="Edit" onClick={onEdit}>
        <Icon name="pencil" />
      </button>
      <button aria-label="Delete" title="Delete" onClick={own.ask}>
        <WritingIcon name="trash" />
      </button>
    </>
  );
}

/** Asks inside the message's own space, so nothing covers what is being deleted (D-024). */
export function DeleteAsk({ own, hasReplies }: { own: OwnMessage; hasReplies: boolean }) {
  return (
    <div className="del-ask" role="group" aria-label="Delete this message?">
      <span>
        Delete this message? It disappears for everyone{hasReplies ? '; its replies stay' : ''}. This cannot be undone.
      </span>
      <button className="btn sm danger" disabled={own.busy} onClick={own.confirmDelete}>
        Delete
      </button>
      <button className="btn sm" onClick={own.discard}>
        Cancel
      </button>
    </div>
  );
}

function isOnScreen(node: HTMLElement | null): boolean {
  const box = node?.closest('.conv-body');
  if (!node || !box) return false;
  const a = node.getBoundingClientRect();
  const b = box.getBoundingClientRect();
  return a.bottom > b.top && a.top < b.bottom;
}

/**
 * The body of a message. A change that arrives while it is on screen keeps the space it had: a
 * shorter text keeps its height, a longer one waits behind "Show all" (D-004). The person's own
 * save comes through the edit box, which remounts this, so their own edit shows in full.
 */
export function StableBody({ text, children }: { text: string; children: ReactNode }) {
  const el = useRef<HTMLDivElement>(null);
  const measured = useRef(0);
  const [seen, setSeen] = useState(text);
  const [lock, setLock] = useState<number | null>(null);
  const [over, setOver] = useState(false);

  if (text !== seen) {
    setSeen(text);
    // Only what the person can see keeps its space; above the view the list holds its place itself.
    if (lock === null && measured.current > 0 && isOnScreen(el.current)) setLock(measured.current);
  }

  useLayoutEffect(() => {
    const node = el.current;
    if (!node) return;
    if (lock === null) {
      measured.current = node.getBoundingClientRect().height;
      setOver(false);
    } else {
      setOver(node.scrollHeight > lock + 1);
    }
  }, [lock, text]);

  useEffect(() => {
    const node = el.current;
    if (!node || lock !== null) return;
    const observer = new ResizeObserver(() => {
      measured.current = node.getBoundingClientRect().height;
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, [lock]);

  return (
    <div
      ref={el}
      className={`msg-body${lock !== null ? ' locked' : ''}`}
      style={lock !== null ? { height: lock, overflow: 'hidden' } : undefined}
    >
      {children}
      {lock !== null && over && (
        <button type="button" className="link show-all" onClick={() => setLock(null)}>
          Show all
        </button>
      )}
    </div>
  );
}

/** Edit in place: mentions follow the text, an empty edit is refused and says so, a retry keeps its ID. */
export function EditBox({ session, row, onClose }: { session: Session; row: MessageRow; onClose(): void }) {
  const m = row.message;
  const [text, setText] = useState(m.text);
  const [problem, setProblem] = useState<Problem | null>(null);
  const [saving, setSaving] = useState(false);
  const box = useRef<HTMLTextAreaElement>(null);
  const attempt = useRef<{ body: string; id: string } | null>(null);
  const allowEveryone = allowsEveryone(session, m.conversation);
  const mention = useMentionBox({
    session,
    allowEveryone,
    value: text,
    onValue: (value) => {
      setText(value);
      setProblem(null);
    },
    box,
  });

  useEffect(() => {
    const el = box.current;
    if (el) {
      el.focus({ preventScroll: true });
      el.setSelectionRange(el.value.length, el.value.length);
    }
  }, []);

  const save = async () => {
    const body = text.trim();
    if (!body) {
      setProblem({ text: 'A message cannot be empty. Nothing was changed: write something, or press Cancel.', canRetry: false });
      return;
    }
    if (body === m.text) {
      onClose();
      return;
    }
    if (attempt.current?.body !== body) attempt.current = { body, id: newId('act') };
    setSaving(true);
    setProblem(null);
    try {
      await session.edit(m, {
        text: body,
        mentions: mentionsIn(body, session.peopleList, session.me?.id, allowEveryone),
        level: m.level,
        actionId: attempt.current.id,
      });
      onClose();
    } catch (e) {
      setSaving(false);
      const found = problemOf(e, 'Not saved');
      if (!found.canRetry) attempt.current = null;
      setProblem(found);
    }
  };

  const onKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (mention.onKeyDown(e)) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
      return;
    }
    if (e.key !== 'Enter' || e.nativeEvent.isComposing) return;
    const saves = session.sendKey === 'enter' ? !e.shiftKey && !e.ctrlKey && !e.metaKey : e.ctrlKey || e.metaKey;
    if (saves) {
      e.preventDefault();
      void save();
    }
  };

  return (
    <>
      <form
        className="edit"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <AutoTextarea
          ref={box}
          value={text}
          aria-label="Edit your message"
          onChange={(e) => {
            setText(e.target.value);
            setProblem(null);
          }}
          onSelect={mention.onCaret}
          onKeyDown={onKey}
        />
        <button className="btn sm pri" disabled={saving}>
          Save
        </button>
        <button type="button" className="btn sm" onClick={onClose}>
          Cancel
        </button>
      </form>
      {mention.popup}
      {problem && (
        <ActionProblem problem={problem} onRetry={() => void save()} onDiscard={text.trim() ? onClose : undefined} />
      )}
    </>
  );
}
