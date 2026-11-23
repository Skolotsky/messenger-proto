import { useLayoutEffect, useRef, useState } from 'react';
import type { MessageRow } from '../api/types';
import { withPending } from '../state/outbox';
import type { Session } from '../state/useSession';
import { AgentState } from './AgentState';
import { Avatar } from './Avatar';
import { timeOf, whenOf } from './format';
import { Icon } from './Icon';
import { usePicker } from './ReactionPicker';
import { textWithMentions } from './Mentions';
import { ActionProblem, DeleteAsk, EditBox, OutLines, OwnTools, StableBody, useOwnMessage } from './OwnMessage';

interface Props {
  row: MessageRow;
  session: Session;
  inThread: boolean;
  isThreadOpen: boolean;
  onOpenThread?(id: string): void;
  /** This is the agent's newest message here, so its current state stands beside its name. */
  showAgentState?: boolean;
}

export function MessageItem({ row, session, inThread, isThreadOpen, onOpenThread, showAgentState = false }: Props) {
  const { thread_summary: summary } = row;
  const pending = session.outbox.filter((i) => i.messageId === row.message.id);
  const m = withPending(row.message, pending, session.me?.id);
  const unsaved = pending.some((i) => i.action.kind === 'send');
  const deleting = pending.some((i) => i.action.kind === 'delete');
  const [editing, setEditing] = useState(false);
  const own = useOwnMessage(session, m);
  const self = useRef<HTMLDivElement>(null);
  const height = useRef(0);
  const author = session.people.get(m.author);
  const mine = m.author === session.me?.id;
  const replies = summary?.replies ?? 0;
  const openPicker = usePicker();
  const myId = session.me?.id;
  // One action with its own ID (D-023); it shows at once, marked as waiting if it has to wait.
  const react = (emoji: string) => {
    if (!myId || unsaved) return;
    session.react(m, emoji, !m.reactions.some((r) => r.emoji === emoji && r.people.includes(myId)));
  };
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
        {summary.unread && <span className="dot" role="img" aria-label="unread replies" />}
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
        {!editing && !own.asking && !unsaved && !deleting && (
          <span className="msg-tools">
            <button
              aria-label="React"
              title="React"
              onClick={(e) => {
                const box = e.currentTarget.getBoundingClientRect();
                openPicker({
                  label: `React to ${mine ? 'your' : `${(author?.name ?? m.author).split(' ')[0]}'s`} message`,
                  x: box.left + box.width / 2,
                  y: box.bottom,
                  choose: react,
                });
              }}
            >
              <Icon name="smile" />
            </button>
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
            {showAgentState && <AgentState person={author} session={session} />}
            {m.level === 'flag' && <span className="tag flag">Flag</span>}
            {m.level === 'info' && <span className="tag info">Info</span>}
            <time dateTime={m.created_at}>{timeOf(m.created_at)}</time>
            {m.edited_at && <span className="edited">edited</span>}
          </div>
          {editing ? (
            <EditBox session={session} row={{ ...row, message: m }} onClose={() => setEditing(false)} />
          ) : own.asking ? (
            <DeleteAsk own={own} hasReplies={replies > 0} />
          ) : (
            <StableBody text={m.text}>{textWithMentions(m.text, m.mentions, session.peopleList, session.me?.id)}</StableBody>
          )}
          {own.problem && <ActionProblem problem={own.problem} onRetry={own.retry} onDiscard={own.discard} />}
          <OutLines session={session} items={pending} />
          {m.reactions.length > 0 && (
            <div className="rxs">
              {m.reactions.map((r) => (
                <button
                  key={r.emoji}
                  className={`rx${session.me && r.people.includes(session.me.id) ? ' on' : ''}`}
                  title={r.people.map((p) => session.people.get(p)?.name ?? p).join(', ')}
                  aria-label={`${r.emoji}, ${r.people.length} ${r.people.length === 1 ? 'person' : 'people'}`}
                  onClick={() => react(r.emoji)}
                >
                  {r.emoji} <i>{r.people.length}</i>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
      {threadLink}
    </div>
  );
}
