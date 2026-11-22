import { useEffect, useMemo, useState, type CSSProperties } from 'react';
import type { PersonView } from '../api/types';
import { HOME_PERSON } from '../config';
import { useSession } from '../state/useSession';
import { Activity } from './Activity';
import { ConversationView } from './ConversationView';
import { stored, useIsPhone } from './hooks';
import { Icon } from './Icon';
import { usePanels } from './panels';
import { Resizer } from './Resizer';
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
  const isPhone = useIsPhone();
  const [phoneView, setPhoneView] = useState<'activity' | 'chats'>('activity');
  const [activityOpen, setActivityOpen] = useState(true);
  const [open, setOpen] = useState<string | null>(null);
  const [thread, setThread] = useState<string | null>(null);
  const [asking, setAsking] = useState<PersonView | null>(null);
  const shown = useMemo(() => ({ activity: activityOpen, thread: thread !== null }), [activityOpen, thread]);
  const { widths, resize, reset } = usePanels(shown);
  const { setOpenConversation } = session;

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
          />
          <Resizer panel="side" label="Resize the sidebar" width={widths.side} onResize={resize} onReset={reset} />
        </aside>
        <main className="main">
          <Activity orgName={session.orgName} onClose={() => setActivityOpen(false)} />
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
            </button>
            <button
              className={`nav${phoneView === 'chats' ? ' on' : ''}`}
              aria-current={phoneView === 'chats' || undefined}
              onClick={() => setPhoneView('chats')}
            >
              <Icon name="chat" />
              <span>Channels</span>
            </button>
            <button className="nav" disabled title="Settings are not built yet">
              <Icon name="rules" />
              <span>Settings</span>
            </button>
          </nav>
        )}
      </div>
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
  );
}
