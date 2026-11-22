import { useEffect, useRef, useState, type ReactNode } from 'react';
import { act, newId } from '../api/door';
import type { ChannelDial, FocusRule, SettingsAnswer } from '../api/types';
import type { Feed } from '../state/useFeed';
import type { Session } from '../state/useSession';
import { Avatar } from './Avatar';
import { useThemePref } from './hooks';
import { Icon } from './Icon';

export type SettingsSection = 'account' | 'look' | 'writing' | 'feed' | 'people' | 'channels' | 'notifications';

const SECTIONS: [SettingsSection, string, boolean][] = [
  ['account', 'Account', false],
  ['look', 'Appearance', false],
  ['writing', 'Writing messages', false],
  ['feed', 'Activity feed', false],
  ['people', 'Focus people', true],
  ['channels', 'Channel levels', true],
  ['notifications', 'Notifications', false],
];

const DIALS: [ChannelDial, string, string][] = [
  ['focus', 'Focus', 'Every new post, and everything addressed to you, in Focus'],
  ['all', 'All activity', 'The same posts; the ones not addressed to you go to For your information'],
  ['default', 'Default', 'Only what is addressed to you, by the starting rules'],
  ['muted', 'Muted', 'Only what names you, and your own threads'],
];

const sameRule = (a: FocusRule, b: FocusRule) => a.type === b.type && a.value === b.value;
const FLAGS: FocusRule = { type: 'item_kind', value: 'flags' };
const MENTIONS: FocusRule = { type: 'item_kind', value: 'mentions' };

function Choice({ on, title, note, onPick }: { on: boolean; title: string; note: string; onPick(): void }) {
  return (
    <button className={`n-opt${on ? ' on' : ''}`} role="radio" aria-checked={on} onClick={onPick}>
      <span className="n-radio" />
      <span className="n-text">
        <b>{title}</b>
        <small>{note}</small>
      </span>
    </button>
  );
}

function Toggle({ on, title, note, onChange }: { on: boolean; title: string; note: string; onChange(on: boolean): void }) {
  return (
    <label className="set-sw">
      <input type="checkbox" checked={on} onChange={(e) => onChange(e.target.checked)} />
      <span>
        <b>{title}</b>
        <small>{note}</small>
      </span>
    </label>
  );
}

interface Props {
  session: Session;
  feed: Feed;
  section: SettingsSection;
  onSection(section: SettingsSection): void;
  onClose(): void;
}

/** The one settings screen (D-023, points 10, 15 to 19, 26). */
export function SettingsScreen({ session, feed, section, onSection, onClose }: Props) {
  const root = useRef<HTMLDivElement>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [asking, setAsking] = useState(false);
  const s = feed.settings;
  const { reloadSettings } = feed;

  useEffect(() => {
    reloadSettings();
    root.current?.focus({ preventScroll: true });
  }, [reloadSettings]);

  useEffect(() => {
    const esc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', esc);
    return () => document.removeEventListener('keydown', esc);
  }, [onClose]);

  const save = (action: Record<string, unknown>, change: (x: SettingsAnswer) => SettingsAnswer) => {
    setProblem(null);
    feed.saveSetting(action, change).catch(() => setProblem('That did not go through. Nothing changed. Try again.'));
  };

  const setRule = (rule: FocusRule, on: boolean) =>
    save({ kind: 'focus_rule', rule, on }, (x) => ({
      ...x,
      rules: on ? [...x.rules.filter((r) => !sameRule(r, rule)), rule] : x.rules.filter((r) => !sameRule(r, rule)),
      removed_rules: on ? x.removed_rules.filter((r) => !sameRule(r, rule)) : [...x.removed_rules.filter((r) => !sameRule(r, rule)), rule],
    }));

  const setPerson = (about: string, on: boolean) =>
    save({ kind: 'focus_person', about, on }, (x) => {
      const known = x.focus_people.find((p) => p.person === about);
      return {
        ...x,
        focus_people: on
          ? known
            ? x.focus_people
            : [...x.focus_people, { person: about, items_last_7_days: 0 }]
          : x.focus_people.filter((p) => p.person !== about),
        removed_people: on ? x.removed_people.filter((p) => p !== about) : [...x.removed_people.filter((p) => p !== about), about],
      };
    });

  const removeAll = () => {
    for (const p of s?.focus_people ?? []) setPerson(p.person, false);
    setAsking(false);
  };

  const nameOf = (id: string) => session.people.get(id)?.name ?? id;

  const personRow = (id: string, note: string, action: ReactNode) => (
    <div className="rule person" key={id}>
      <Avatar id={id} person={session.people.get(id)} small />
      <span>
        <b>{nameOf(id)}</b>
        <small>{note}</small>
      </span>
      {action}
    </div>
  );

  let body: ReactNode = <p className="sheet-note">Loading…</p>;
  if (section === 'account') body = <AccountPage session={session} />;
  else if (section === 'look') body = <AppearancePage />;
  else if (section === 'writing') body = <WritingPage session={session} />;
  else if (s && section === 'feed') {
    const has = (rule: FocusRule) => s.rules.some((r) => sameRule(r, rule));
    const changed = session.conversations.filter((c) => c.type === 'channel' && levelOf(s, c.conversation, c.dial) !== 'default').length;
    body = (
      <>
        <p className="set-q">What goes to Focus?</p>
        <p className="sheet-note">Focus holds what matches these rules and lists, and nothing else. Changes apply the next time you open the feed.</p>
        <h3>General rules</h3>
        <Toggle
          on={s.auto_add}
          title="Add everyone who writes to me directly"
          note="Their direct messages land in Focus. Off: nobody new is added."
          onChange={(on) => {
            save({ kind: 'focus_auto_add', on }, (x) => ({ ...x, auto_add: on }));
            setAsking(!on && s.focus_people.length > 0);
          }}
        />
        {asking && (
          <div className="ask" role="group" aria-label="Remove people already added?">
            <p>
              New people will no longer be added. {s.focus_people.length} {s.focus_people.length === 1 ? 'person is' : 'people are'} already in Focus and
              stay there. Remove them too?
            </p>
            <button className="btn sm pri" autoFocus onClick={() => setAsking(false)}>
              Keep them
            </button>
            <button className="btn sm" onClick={removeAll}>
              Remove all {s.focus_people.length}
            </button>
          </div>
        )}
        <Toggle on={has(FLAGS)} title="Flags that name you" note="A flag that mentions you or is sent to you directly." onChange={(on) => setRule(FLAGS, on)} />
        <Toggle on={has(MENTIONS)} title="Any direct mention" note="Off: mentions land in Needs you." onChange={(on) => setRule(MENTIONS, on)} />
        <h3>Kept rows</h3>
        <Toggle
          on={s.release_on_act}
          title="Replying or reacting marks a kept row done"
          note="Off: a kept or flagged row stays until you press Done yourself."
          onChange={(on) => save({ kind: 'release_setting', on }, (x) => ({ ...x, release_on_act: on }))}
        />
        <h3>What you added to Focus</h3>
        <button className="set-link" onClick={() => onSection('people')}>
          <span>
            <b>People · {s.focus_people.length}</b>
            <small>Whose direct messages land in Focus</small>
          </span>
          <Icon name="chev" />
        </button>
        <p className="sheet-note">Threads are not listed here. A thread is put in Focus, and taken out, on the thread itself; History, Threads shows each thread's level.</p>
        <h3>Channels</h3>
        <button className="set-link" onClick={() => onSection('channels')}>
          <span>
            <b>Channel levels · {changed} changed</b>
            <small>How much of each channel reaches your feed</small>
          </span>
          <Icon name="chev" />
        </button>
      </>
    );
  } else if (s && section === 'people') {
    const everything = s.rules.filter((r) => r.type === 'person');
    const removedEverything = s.removed_rules.filter((r) => r.type === 'person');
    body = (
      <>
        <button className="link set-back" onClick={() => onSection('feed')}>
          ‹ Activity feed
        </button>
        <p className="sheet-note">Direct messages from these people land in Focus. Remove someone here, or from the reason line on one of their rows.</p>
        <h3>Direct messages from · {s.focus_people.length}</h3>
        {s.focus_people.length === 0 && <p className="sheet-note">Nobody. Direct messages land in Needs you.</p>}
        {s.focus_people.map((p) =>
          personRow(
            p.person,
            `${p.items_last_7_days} ${p.items_last_7_days === 1 ? 'item' : 'items'} in the last 7 days`,
            <button className="btn sm" aria-label={`Remove ${nameOf(p.person)}`} onClick={() => setPerson(p.person, false)}>
              Remove
            </button>,
          ),
        )}
        {s.focus_people.length > 1 && (
          <button className="link rm-all" onClick={removeAll}>
            Remove all
          </button>
        )}
        {s.removed_people.length > 0 && (
          <>
            <h3>Removed · {s.removed_people.length}</h3>
            <p className="sheet-note">Not added again when they write.</p>
            {s.removed_people.map((id) =>
              personRow(
                id,
                'Their direct messages land in Needs you',
                <button className="btn sm pri" aria-label={`Add ${nameOf(id)} back`} onClick={() => setPerson(id, true)}>
                  Add back
                </button>,
              ),
            )}
          </>
        )}
        {(everything.length > 0 || removedEverything.length > 0) && <h3>Everything from · {everything.length}</h3>}
        {everything.map((r) =>
          personRow(
            r.value,
            'Everything that reaches you from them',
            <button className="btn sm" aria-label={`Stop lifting everything from ${nameOf(r.value)}`} onClick={() => setRule(r, false)}>
              Remove
            </button>,
          ),
        )}
        {removedEverything.map((r) =>
          personRow(
            r.value,
            'Removed',
            <button className="btn sm pri" aria-label={`Lift everything from ${nameOf(r.value)} again`} onClick={() => setRule(r, true)}>
              Add back
            </button>,
          ),
        )}
      </>
    );
  } else if (s && section === 'channels') {
    const channels = session.conversations.filter((c) => c.type === 'channel');
    const list = (level: (d: ChannelDial) => boolean) => channels.filter((c) => level(levelOf(s, c.conversation, c.dial)));
    const row = (c: (typeof channels)[number]) => {
      const dial = levelOf(s, c.conversation, c.dial);
      return (
        <div className="rule ch-row" key={c.conversation}>
          <span>
            <b>#{c.name}</b>
            <small>{DIALS.find((d) => d[0] === dial)?.[2]}</small>
          </span>
          <select
            aria-label={`Level for #${c.name}`}
            value={dial}
            onChange={(e) => {
              const next = e.target.value as ChannelDial;
              save({ kind: 'dial', conversation: c.conversation, thread: null, level: next }, (x) => ({
                ...x,
                levels: [...x.levels.filter((l) => !(l.conversation === c.conversation && l.thread === null)), { conversation: c.conversation, thread: null, level: next }],
              }));
            }}
          >
            {DIALS.map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </div>
      );
    };
    const changed = list((d) => d !== 'default');
    const standard = list((d) => d === 'default');
    body = (
      <>
        <button className="link set-back" onClick={() => onSection('feed')}>
          ‹ Activity feed
        </button>
        <p className="set-q">How much of each channel reaches your feed?</p>
        <p className="sheet-note">Every channel has one level. A mention or a flag that names you gets through at every level.</p>
        <div className="lv-legend">
          {DIALS.map(([value, label, note]) => (
            <div key={value}>
              <b>{label}</b>
              <span>{note}</span>
            </div>
          ))}
        </div>
        {changed.length > 0 && (
          <>
            <h3>Changed by you · {changed.length}</h3>
            {changed.map(row)}
          </>
        )}
        <h3>At Default · {standard.length}</h3>
        {standard.map(row)}
      </>
    );
  } else if (s && section === 'notifications') {
    body = (
      <>
        <p className="set-q">What should send you a notification?</p>
        <div role="radiogroup" aria-label="What sends a notification" className="n-group">
          <Choice
            on={!s.focus_only}
            title="Focus and Needs you"
            note="Everything directed at you: what your Focus rules catch, mentions, replies in your threads, flags."
            onPick={() => save({ kind: 'focus_only', on: false }, (x) => ({ ...x, focus_only: false }))}
          />
          <Choice
            on={s.focus_only}
            title="Focus only"
            note="Only what you chose for Focus yourself, and flags that name you. Mentions and replies wait in the feed."
            onPick={() => save({ kind: 'focus_only', on: true }, (x) => ({ ...x, focus_only: true }))}
          />
        </div>
        <h3>Always</h3>
        <ul className="set-list">
          <li>For your information never sends a notification.</li>
          <li>One notification per conversation, not one per message.</li>
          <li>Nothing is sent for a conversation you are looking at.</li>
          <li>There is no separate notification setting per channel: a channel's level decides what reaches Focus and Needs you, and that decides what notifies.</li>
        </ul>
      </>
    );
  }

  const title = SECTIONS.find(([k]) => k === section)?.[1];
  const parentOn = section === 'people' || section === 'channels';

  return (
    <div className="scrim center" onClick={onClose}>
      <div
        className="sheet settings"
        role="dialog"
        aria-modal="true"
        aria-label="Settings"
        tabIndex={-1}
        ref={root}
        onClick={(e) => e.stopPropagation()}
      >
        <button className="icon-btn close" aria-label="Close settings" onClick={onClose}>
          <Icon name="close" />
        </button>
        <nav className="set-nav" aria-label="Settings sections">
          <h2>Settings</h2>
          {SECTIONS.map(([key, label, sub]) => (
            <button
              key={key}
              className={`set-item${sub ? ' sub' : ''}${key === section ? ' on' : ''}${key === 'feed' && parentOn ? ' parent' : ''}`}
              aria-current={key === section || undefined}
              onClick={() => onSection(key)}
            >
              {label}
            </button>
          ))}
        </nav>
        <div className="set-body">
          <h2>{title}</h2>
          {problem && (
            <p className="err" role="alert">
              {problem}
            </p>
          )}
          {body}
        </div>
      </div>
    </div>
  );
}

function levelOf(s: SettingsAnswer, conversation: string, fallback: ChannelDial): ChannelDial {
  const entry = s.levels.find((l) => l.conversation === conversation && l.thread === null);
  return (entry?.level as ChannelDial | undefined) ?? fallback;
}

function AccountPage({ session }: { session: Session }) {
  const me = session.me;
  const [line, setLine] = useState(me?.line ?? '');
  const [state, setState] = useState<'idle' | 'saved' | 'failed'>('idle');
  if (!me) return null;
  const save = () => {
    if (line === (me.line ?? '')) return;
    act({ kind: 'profile_line', id: newId('act'), as: me.id, text: line }).then(
      () => setState('saved'),
      () => setState('failed'),
    );
  };
  return (
    <>
      <div className="rule person">
        <Avatar id={me.id} person={me} />
        <span>
          <b>{me.name}</b>
          <small>{session.orgName}</small>
        </span>
      </div>
      <label className="fld">
        <span>The line under your name</span>
        <input
          value={line}
          maxLength={80}
          onChange={(e) => {
            setLine(e.target.value);
            setState('idle');
          }}
          onBlur={save}
          onKeyDown={(e) => {
            if (e.key === 'Enter') save();
          }}
        />
      </label>
      <p className="sheet-note" role="status">
        {state === 'saved'
          ? 'Saved.'
          : state === 'failed'
            ? 'Not saved: no answer from the server.'
            : 'Your own words. It means nothing to the product: there are no roles or permissions in the first version.'}
      </p>
      <p className="sheet-note">To see the team as someone else, use Switch account in your menu.</p>
    </>
  );
}

function AppearancePage() {
  const [theme, setTheme] = useThemePref();
  return (
    <>
      <p className="set-q">Which theme?</p>
      <div role="radiogroup" aria-label="Theme" className="n-group">
        <Choice on={theme === 'system'} title="System" note="Follows your device and changes with it." onPick={() => setTheme('system')} />
        <Choice on={theme === 'light'} title="Light" note="Always light." onPick={() => setTheme('light')} />
        <Choice on={theme === 'dark'} title="Dark" note="Always dark." onPick={() => setTheme('dark')} />
      </div>
      <p className="sheet-note">Remembered on this device, so a phone and a laptop can differ.</p>
    </>
  );
}

function WritingPage({ session }: { session: Session }) {
  const enterSends = session.sendKey === 'enter';
  const [problem, setProblem] = useState(false);
  const choose = (value: 'enter' | 'ctrl_enter') => {
    setProblem(false);
    session.setPreference('send_key', value).catch(() => setProblem(true));
  };
  return (
    <>
      <p className="set-q">While writing or editing a message, what does Enter do?</p>
      <div role="radiogroup" aria-label="What Enter does" className="n-group">
        <Choice on={!enterSends} title="Enter starts a new line" note="Ctrl+Enter sends the message, or saves the edit." onPick={() => choose('ctrl_enter')} />
        <Choice on={enterSends} title="Enter sends" note="Shift+Enter starts a new line." onPick={() => choose('enter')} />
      </div>
      {problem && (
        <p className="err" role="alert">
          That did not go through. Nothing changed. Try again.
        </p>
      )}
      <p className="sheet-note">Esc leaves an edit without saving. On a phone the keyboard's Enter always starts a new line; the Send button sends.</p>
    </>
  );
}
