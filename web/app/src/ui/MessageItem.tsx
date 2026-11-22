import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { DoorError } from '../api/door';
import type { MessageRow, PersonView } from '../api/types';
import type { Session } from '../state/useSession';
import { AutoTextarea } from './AutoTextarea';
import { Avatar } from './Avatar';
import { timeOf, whenOf } from './format';
import { Icon } from './Icon';

interface Props {
  row: MessageRow;
  session: Session;
  inThread: boolean;
  isThreadOpen: boolean;
  onOpenThread?(id: string): void;
}

/** Marks a mention only where the message really names that person; everything stays text. */
function withMentions(text: string, mentions: string[], people: Map<string, PersonView>, me: string | undefined) {
  const names = new Map<string, string>();
  for (const id of mentions) {
    const name = people.get(id)?.name;
    if (name) names.set(name.toLowerCase(), id);
  }
  if (mentions.includes('channel')) names.set('channel', 'channel');
  if (names.size === 0) return text;
  const out: ReactNode[] = [];
  let last = 0;
  for (const match of text.matchAll(/@([\p{L}\p{N}_-]+)/gu)) {
    const id = names.get(match[1].toLowerCase());
    if (!id) continue;
    out.push(text.slice(last, match.index));
    out.push(
      <span key={match.index} className={`at${id === me ? ' me' : ''}`}>
        {match[0]}
      </span>,
    );
    last = match.index + match[0].length;
  }
  out.push(text.slice(last));
  return out;
}

export function MessageItem({ row, session, inThread, isThreadOpen, onOpenThread }: Props) {
  const { message: m, thread_summary: summary } = row;
  const [editing, setEditing] = useState(false);
  const author = session.people.get(m.author);
  const mine = m.author === session.me?.id;
  const replies = summary?.replies ?? 0;
  const classes = ['msg', mine && 'mine', m.level === 'flag' && 'is-flag', m.level === 'info' && 'is-info', m.deleted && 'deleted']
    .filter(Boolean)
    .join(' ');

  const threadLink =
    !inThread && replies > 0 && summary ? (
      <button
        className={`thread-link${isThreadOpen ? ' open' : ''}`}
        onClick={() => onOpenThread?.(m.id)}
        aria-expanded={isThreadOpen}
      >
        {summary.people.slice(0, 3).map((p) => (
          <Avatar key={p} id={p} person={session.people.get(p)} small />
        ))}
        <b>
          {replies} {replies === 1 ? 'reply' : 'replies'}
        </b>
        {summary.last_reply_at && <small>Last reply {whenOf(summary.last_reply_at)}</small>}
        <span className="tl-go">View thread ›</span>
      </button>
    ) : null;

  if (m.deleted) {
    return (
      <div className={classes} data-msg={m.id}>
        <div className="msg-self">
          <span className="tomb">This message was deleted.</span>
        </div>
        {threadLink}
      </div>
    );
  }

  return (
    <div className={classes} data-msg={m.id}>
      <div className="msg-self">
        {!editing && (
          <span className="msg-tools">
            {!inThread && m.thread === null && (
              <button aria-label="Reply in thread" title="Reply in thread" onClick={() => onOpenThread?.(m.id)}>
                <Icon name="chat" />
              </button>
            )}
            {mine && (
              <button aria-label="Edit" title="Edit" onClick={() => setEditing(true)}>
                <Icon name="pencil" />
              </button>
            )}
          </span>
        )}
        <Avatar id={m.author} person={author} />
        <div className="msg-main">
          <div className="msg-head">
            <b>{author?.name ?? m.author}</b>
            {m.level === 'flag' && <span className="tag flag">Flag</span>}
            {m.level === 'info' && <span className="tag info">Info</span>}
            <time dateTime={m.created_at}>{timeOf(m.created_at)}</time>
            {m.edited_at && <span className="edited">edited</span>}
          </div>
          {editing ? (
            <EditBox session={session} row={row} onClose={() => setEditing(false)} />
          ) : (
            <div className="msg-body">{withMentions(m.text, m.mentions, session.people, session.me?.id)}</div>
          )}
          {m.reactions.length > 0 && (
            <div className="rxs">
              {m.reactions.map((r) => (
                <span
                  key={r.emoji}
                  className={`rx${session.me && r.people.includes(session.me.id) ? ' on' : ''}`}
                  title={r.people.map((p) => session.people.get(p)?.name ?? p).join(', ')}
                >
                  {r.emoji} <i>{r.people.length}</i>
                </span>
              ))}
            </div>
          )}
        </div>
      </div>
      {threadLink}
    </div>
  );
}

/**
 * Edit in place: the smallest form T-038 point 25 needs. Seam for T-056, which adds mentions,
 * Remove flag, Delete and the waiting state.
 */
function EditBox({ session, row, onClose }: { session: Session; row: MessageRow; onClose(): void }) {
  const m = row.message;
  const [text, setText] = useState(m.text);
  const [problem, setProblem] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const box = useRef<HTMLTextAreaElement>(null);

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
      setProblem('A message cannot be empty. Nothing was changed: write something, or press Cancel.');
      return;
    }
    if (body === m.text) {
      onClose();
      return;
    }
    setSaving(true);
    setProblem(null);
    try {
      await session.edit(m, body);
      onClose();
    } catch (e) {
      setSaving(false);
      setProblem(
        e instanceof DoorError && e.kind === 'refused'
          ? `Not saved: ${e.message}.`
          : 'Not saved: no answer from the server. Your change is still here, try again.',
      );
    }
  };

  const onKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
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
          onKeyDown={onKey}
        />
        <button className="btn sm pri" disabled={saving}>
          Save
        </button>
        <button type="button" className="btn sm" onClick={onClose}>
          Cancel
        </button>
      </form>
      {problem && (
        <div className="send-state fail" role="alert">
          {problem}
        </div>
      )}
    </>
  );
}
