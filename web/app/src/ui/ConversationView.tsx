import { useEffect, useState } from 'react';
import type { ConversationSummary } from '../api/types';
import { listKey } from '../state/lists';
import type { Session } from '../state/useSession';
import { Composer } from './Composer';
import { useIsLooking } from './hooks';
import { Icon } from './Icon';
import { MessageItem } from './MessageItem';
import { MessageList } from './MessageList';

export function conversationTitle(c: ConversationSummary, session: Session): string {
  if (c.type === 'channel') return `#${c.name}`;
  const other = c.people.find((p) => p !== session.me?.id) ?? c.people[0];
  return session.people.get(other)?.name ?? other;
}

interface Props {
  session: Session;
  conversation: ConversationSummary;
  /** The root message when this view is a thread. */
  thread: string | null;
  openThread: string | null;
  onOpenThread(id: string): void;
  onClose(): void;
  /** False while something covers this view: what is covered is not being read (D-015). */
  active?: boolean;
}

/** One conversation or one thread: its header, its messages, its message box. */
export function ConversationView({
  session,
  conversation,
  thread,
  openThread,
  onOpenThread,
  onClose,
  active = true,
}: Props) {
  const id = conversation.conversation;
  const key = listKey(id, thread);
  const rows = session.lists[key];
  const [failed, setFailed] = useState(false);
  const [pinned, setPinned] = useState(true);
  const looking = useIsLooking();
  const { loadList, read } = session;
  const title = conversationTitle(conversation, session);

  useEffect(() => {
    setFailed(false);
    setPinned(true);
    loadList(id, thread).catch(() => setFailed(true));
  }, [id, thread, loadList]);

  // Read means seen: this window is in front of the person and the newest message is in view (D-015).
  const newest = rows?.[rows.length - 1]?.message;
  useEffect(() => {
    if (active && looking && pinned && newest) read(id, thread, newest.number);
  }, [active, looking, pinned, newest, id, thread, read]);

  const replies = thread && rows ? rows.length - 1 : 0;
  const subtitle = thread
    ? title
    : conversation.type === 'channel'
      ? `${conversation.people.length} members`
      : 'Direct message';

  return (
    <div className={`conv k-${thread ? 'thread side-thread' : conversation.type}`}>
      <header className="conv-head">
        <button className="icon-btn back" onClick={onClose} aria-label={thread ? 'Back to the channel' : 'Back'}>
          <Icon name="back" />
        </button>
        <div className="conv-title">
          <b>{thread ? 'Thread' : title}</b>
          <small>{subtitle}</small>
        </div>
        {thread && (
          <button className="icon-btn th-close" onClick={onClose} aria-label="Close the thread" title="Close the thread">
            <Icon name="close" />
          </button>
        )}
      </header>
      {failed ? (
        <div className="conv-body">
          <div className="empty" role="alert">
            <p>This conversation could not be loaded.</p>
            <button
              className="btn"
              onClick={() => {
                setFailed(false);
                loadList(id, thread).catch(() => setFailed(true));
              }}
            >
              Try again
            </button>
          </div>
        </div>
      ) : !rows ? (
        <div className="conv-body" aria-busy="true" />
      ) : rows.length === 0 ? (
        <div className="conv-body">
          <div className="empty start">
            <p>
              <b>This is the start of {title}.</b>
            </p>
          </div>
        </div>
      ) : (
        <MessageList
          key={key}
          rows={rows}
          onPinnedChange={setPinned}
          afterFirst={
            thread ? (
              <div className="dayline replies-line">
                <span>
                  {replies} {replies === 1 ? 'reply' : 'replies'}
                </span>
              </div>
            ) : undefined
          }
          renderRow={(row) => (
            <MessageItem
              row={row}
              session={session}
              inThread={thread !== null}
              isThreadOpen={openThread === row.message.id}
              onOpenThread={onOpenThread}
            />
          )}
        />
      )}
      <Composer
        key={key}
        session={session}
        conversation={id}
        thread={thread}
        placeholder={thread ? 'Reply in thread…' : `Message ${title}…`}
      />
    </div>
  );
}
