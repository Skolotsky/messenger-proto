import type { ConversationSummary, PersonView } from '../api/types';
import type { Session } from '../state/useSession';
import { Avatar } from './Avatar';
import { conversationTitle } from './ConversationView';
import { timeOf } from './format';
import { useOfflineNotice } from './hooks';
import { Icon } from './Icon';
import type { Jump } from './jump';
import { NewConversationButton } from './NewConversation';
import { PersonMenu } from './PersonMenu';
import { SearchBox } from './SearchBox';

interface Props {
  session: Session;
  open: string | null;
  activityOpen: boolean;
  onToggleActivity(): void;
  onOpen(conversation: string): void;
  onSwitch(person: PersonView): void;
  onSettings(): void;
  /** Unread rows in Focus and Needs you. It stays here while Activity is closed (D-023, point 12). */
  attention: number;
  onSearchHit(jump: Jump): void;
}

/** Organization, Activity, channels, direct messages, and the person's own menu (D-019, D-023). */
export function Sidebar({ session, open, activityOpen, onToggleActivity, onOpen, onSwitch, onSettings, attention, onSearchHit }: Props) {
  const channels = session.conversations.filter((c) => c.type === 'channel');
  const directs = session.conversations.filter((c) => c.type === 'direct');
  const offline = useOfflineNotice(session.offlineSince);

  const item = (c: ConversationSummary) => {
    const title = conversationTitle(c, session);
    const other = c.people.find((p) => p !== session.me?.id) ?? c.people[0];
    return (
      <button
        key={c.conversation}
        className={`chat${c.conversation === open ? ' on' : ''}${c.unread ? ' unread' : ''}`}
        aria-current={c.conversation === open || undefined}
        onClick={() => onOpen(c.conversation)}
      >
        {c.type === 'channel' ? (
          <span className="hash" aria-hidden="true">
            #
          </span>
        ) : (
          <Avatar id={other} person={session.people.get(other)} small />
        )}
        <span className="chat-n">{c.type === 'channel' ? c.name : title}</span>
        {c.unread && <span className="dot" role="img" aria-label="unread" />}
      </button>
    );
  };

  return (
    <>
      <div className="ws">
        <span className="org-mark" aria-hidden="true">
          {Array.from(session.orgName)[0]}
        </span>
        <span className="ws-t">
          <b>{session.orgName}</b>
          {offline !== null ? (
            <small className="ws-off" role="status">
              Reconnecting · since {timeOf(new Date(offline).toISOString())}
            </small>
          ) : (
            <small>{session.peopleList.length} members</small>
          )}
        </span>
      </div>
      <SearchBox session={session} onHit={onSearchHit} />
      <button
        className={`side-item${activityOpen ? ' on' : ''}`}
        aria-pressed={activityOpen}
        title="Open or close Activity"
        onClick={onToggleActivity}
      >
        <Icon name="bell" />
        <span>Activity</span>
        {attention > 0 && <span className="badge">{attention}</span>}
      </button>
      <div className="side-scroll">
        <nav className="chats" aria-label="Conversations">
          <h3>
            Channels
            <NewConversationButton kind="channel" session={session} onOpen={onOpen} />
          </h3>
          {channels.map(item)}
          <h3>
            Direct messages
            <NewConversationButton kind="direct" session={session} onOpen={onOpen} />
          </h3>
          {directs.map(item)}
        </nav>
      </div>
      <PersonMenu session={session} onSwitch={onSwitch} onSettings={onSettings} />
    </>
  );
}
