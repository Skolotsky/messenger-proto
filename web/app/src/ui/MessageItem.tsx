import { useLayoutEffect, useRef, useState } from 'react';
import type { MessageRow } from '../api/types';
import type { Session } from '../state/useSession';
import { Avatar } from './Avatar';
import { timeOf, whenOf } from './format';
import { Icon } from './Icon';
import { textWithMentions } from './Mentions';
import { ActionProblem, DeleteAsk, EditBox, OwnTools, StableBody, useOwnMessage } from './OwnMessage';

interface Props {
  row: MessageRow;
  session: Session;
  inThread: boolean;
  isThreadOpen: boolean;
  onOpenThread?(id: string): void;
}

export function MessageItem({ row, session, inThread, isThreadOpen, onOpenThread }: Props) {
  const { message: m, thread_summary: summary } = row;
  const [editing, setEditing] = useState(false);
  const own = useOwnMessage(session, m);
  const self = useRef<HTMLDivElement>(null);
  const height = useRef(0);
  const author = session.people.get(m.author);
  const mine = m.author === session.me?.id;
  const replies = summary?.replies ?? 0;
  const classes = ['msg', mine && 'mine', m.level === 'flag' && 'is-flag', m.level === 'info' && 'is-info', m.deleted && 'deleted']
    .filter(Boolean)
    .join(' ');

  // A marker has the height of what it replaces, so nothing below it moves (D-004).
  useLayoutEffect(() => {
    if (!m.deleted && !editing && !own.asking && self.current) height.current = self.current.getBoundingClientRect().height;
  });

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
        <div className="msg-self" style={height.current ? { height: height.current } : undefined}>
          <span className="tomb">This message was deleted.</span>
        </div>
        {threadLink}
      </div>
    );
  }

  return (
    <div className={classes} data-msg={m.id}>
      <div className="msg-self" ref={self}>
        {!editing && !own.asking && (
          <span className="msg-tools">
            {!inThread && m.thread === null && (
              <button aria-label="Reply in thread" title="Reply in thread" onClick={() => onOpenThread?.(m.id)}>
                <Icon name="chat" />
              </button>
            )}
            {mine && <OwnTools own={own} flagged={m.level === 'flag'} onEdit={() => setEditing(true)} />}
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
          ) : own.asking ? (
            <DeleteAsk own={own} hasReplies={replies > 0} />
          ) : (
            <StableBody text={m.text}>{textWithMentions(m.text, m.mentions, session.peopleList, session.me?.id)}</StableBody>
          )}
          {own.problem && <ActionProblem problem={own.problem} onRetry={own.retry} onDiscard={own.discard} />}
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
