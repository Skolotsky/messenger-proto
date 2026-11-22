import { useCallback, useEffect, useMemo, useState, type CSSProperties } from 'react';
import type { FeedRow, HistoryEntry, PersonView } from '../api/types';
import { HOME_PERSON } from '../config';
import { useFeed } from '../state/useFeed';
import { useSession } from '../state/useSession';
import { Activity } from './Activity';
import { ConversationView } from './ConversationView';
import { stored, useIsPhone } from './hooks';
import { Icon } from './Icon';
import { usePanels } from './panels';
import { PickerProvider } from './ReactionPicker';
import { Resizer } from './Resizer';
import { SettingsScreen, type SettingsSection } from './SettingsScreen';
import { Sidebar } from './Sidebar';

const PERSON_KEY = 'contexia-person';

export function App() {
  const [person, setPerson] = useState(() => stored.get(PERSON_KEY) ?? HOME_PERSON);
  const choose = (id: string) => {
    stored.set(PERSON_KEY, id === HOME_PERSON ? null : id);
    setPerson(id);
  };
  // A new person is a new session: nothing of the previous view is carried over.
  return <Shell key={person} person={person} onChoose={choose} />;
}

function Shell({ person, onChoose }: { person: string; onChoose(id: string): void }) {
  const session = useSession(person);
  const feed = useFeed(session);
  const isPhone = useIsPhone();
  const [phoneView, setPhoneView] = useState<'activity' | 'chats'>('activity');
  const [activityOpen, setActivityOpen] = useState(true);
  const [open, setOpen] = useState<string | null>(null);
  const [thread, setThread] = useState<string | null>(null);
  const [asking, setAsking] = useState<PersonView | null>(null);
  const [settings, setSettings] = useState<SettingsSection | null>(null);
  // Where a conversation opened from a feed row begins (D-023, point 21).
  const [newFrom, setNewFrom] = useState<{ thread: string | null; number: number | null } | null>(null);
  const shown = useMemo(() => ({ activity: activityOpen, thread: thread !== null }), [activityOpen, thread]);
  const { widths, resize, reset } = usePanels(shown);
  const { setOpenConversation } = session;
  const closeSettings = useCallback(() => setSettings(null), []);

  useEffect(() => setOpenConversation(open), [open, setOpenConversation]);

  const conversation = session.conversations.find((c) => c.conversation === open) ?? null;
  const viewing = person !== HOME_PERSON;
  const home = session.people.get(HOME_PERSON);

  if (session.phase === 'loading') {
    return <div className="boot" aria-busy="true" />;
  }
  if (session.phase === 'failed') {
    return (
      <div className="boot">
        <div className="empty" role="alert">
          <p>
            <b>Contexia could not reach the server.</b>
          </p>
          <button className="btn" onClick={session.retry}>
            Try again
          </button>
        </div>
      </div>
    );
  }

  const openConversation = (id: string | null) => {
    setOpen(id);
    setThread(null);
    setNewFrom(null);
  };
  /** A click on a row: its conversation, or its thread beside the channel, at the first new message. */
  const openRow = (row: FeedRow) => {
    setOpen(row.conversation);
    setThread(row.thread);
    setNewFrom(row.unread ? { thread: row.thread, number: row.first_new } : null);
    feed.markRead(row);
  };
  const openEntry = (entry: HistoryEntry) => {
    setOpen(entry.conversation);
    setThread(entry.thread);
    setNewFrom(entry.unread ? { thread: entry.thread, number: entry.number } : null);
    if (entry.unread) session.read(entry.conversation, entry.thread, entry.number);
  };
  const classes = [
    'app sk',
    `v-${phoneView}`,
    conversation && 'has-conv',
    !activityOpen && 'act-closed',
    conversation && thread && 'has-thread',
  ]
    .filter(Boolean)
    .join(' ');
  const style = {
    '--w-side': `${widths.side}px`,
    '--w-act': `${widths.act}px`,
    '--w-thread': `${widths.thread}px`,
  } as CSSProperties;

  return (
    <PickerProvider>
    <div className={viewing ? 'root viewing' : 'root'}>
      {viewing && (
        <div className="viewband" role="status">
          <b>Viewing as {session.me?.name}</b>
          <span className="vb-note">You act as this person until you switch back.</span>
          <button className="btn sm" onClick={() => onChoose(HOME_PERSON)}>
            Back to {home?.name ?? HOME_PERSON}
          </button>
        </div>
      )}
      <div className={classes} style={style} data-live={session.connected ? 'on' : 'off'}>
        <aside className="side">
          <Sidebar
            session={session}
            open={open}
            activityOpen={activityOpen}
            onToggleActivity={() => setActivityOpen((o) => !o)}
            onOpen={openConversation}
            onSwitch={setAsking}
            onSettings={() => setSettings('account')}
            attention={feed.attention}
          />
          <Resizer panel="side" label="Resize the sidebar" width={widths.side} onResize={resize} onReset={reset} />
        </aside>
        <main className="main">
          <Activity
            session={session}
            feed={feed}
            visible={isPhone ? phoneView === 'activity' : activityOpen}
            onClose={() => setActivityOpen(false)}
            onOpenRow={openRow}
            onOpenEntry={openEntry}
            onSettings={setSettings}
          />
          <Resizer panel="act" label="Resize Activity" width={widths.act} onResize={resize} onReset={reset} />
        </main>
        <section className="pane" aria-label="Conversation">
          {conversation ? (
            <ConversationView
              key={conversation.conversation}
              session={session}
              conversation={conversation}
              thread={null}
              active={!(isPhone && thread)}
              openThread={thread}
              onOpenThread={setThread}
              onClose={() => openConversation(null)}
              newFrom={newFrom && newFrom.thread === null ? newFrom.number : null}
            />
          ) : (
            <div className="conv none">
              <div className="empty">
                <p>
                  {activityOpen
                    ? 'Choose a channel or a person on the left to read the conversation here.'
                    : 'Choose a channel or a person on the left. Activity is closed.'}
                </p>
              </div>
            </div>
          )}
        </section>
        {conversation && thread && (
          <section className="pane tpane" aria-label="Thread">
            <Resizer
              panel="thread"
              label="Resize the thread"
              width={widths.thread}
              fromRight
              onResize={resize}
              onReset={reset}
            />
            <ConversationView
              key={thread}
              session={session}
              conversation={conversation}
              thread={thread}
              openThread={null}
              onOpenThread={setThread}
              onClose={() => setThread(null)}
              newFrom={newFrom && newFrom.thread === thread ? newFrom.number : null}
            />
          </section>
        )}
        {isPhone && (
          <nav className="bottom" aria-label="Main">
            <button
              className={`nav${phoneView === 'activity' ? ' on' : ''}`}
              aria-current={phoneView === 'activity' || undefined}
              onClick={() => setPhoneView('activity')}
            >
              <Icon name="bell" />
              <span>Activity</span>
              {feed.attention > 0 && <span className="badge">{feed.attention}</span>}
            </button>
            <button
              className={`nav${phoneView === 'chats' ? ' on' : ''}`}
              aria-current={phoneView === 'chats' || undefined}
              onClick={() => setPhoneView('chats')}
            >
              <Icon name="chat" />
              <span>Channels</span>
            </button>
            <button className="nav" onClick={() => setSettings('feed')}>
              <Icon name="rules" />
              <span>Settings</span>
            </button>
          </nav>
        )}
      </div>
      {settings && (
        <SettingsScreen session={session} feed={feed} section={settings} onSection={setSettings} onClose={closeSettings} />
      )}
      {asking && (
        <div className="scrim center" onClick={() => setAsking(null)}>
          <div
            className="sheet dialog"
            role="dialog"
            aria-modal="true"
            aria-label="Switch account"
            onClick={(e) => e.stopPropagation()}
          >
            <h2>Switch to {asking.name}?</h2>
            <p>You will see the team as {asking.name} and act as them until you switch back.</p>
            <div className="dlg-acts">
              <button className="btn" onClick={() => setAsking(null)}>
                Cancel
              </button>
              <button
                className="btn pri"
                autoFocus
                onClick={() => {
                  onChoose(asking.id);
                  setAsking(null);
                }}
              >
                Switch
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
    </PickerProvider>
  );
}
