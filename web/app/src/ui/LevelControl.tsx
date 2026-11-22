import { useEffect, useRef, useState } from 'react';
import type { ChannelDial, ConversationSummary, DialLevel, MessageRow, SettingsAnswer } from '../api/types';
import type { Feed } from '../state/useFeed';
import type { Session } from '../state/useSession';
import { Icon } from './Icon';

type ThreadLevel = 'focus' | 'needs_you' | 'fyi' | 'off';

const CHANNEL_LEVELS: [ChannelDial, string, string][] = [
  ['focus', 'Focus', 'Every new post, and everything addressed to you, in Focus'],
  ['all', 'All activity', 'The same posts; the ones not addressed to you go to For your information'],
  ['default', 'Default', 'Only what is addressed to you, by the starting rules'],
  ['muted', 'Muted', 'Only what names you, and your own threads'],
];

const THREAD_LEVELS: [ThreadLevel, string, string][] = [
  ['focus', 'Focus', 'New replies land in Focus'],
  ['needs_you', 'Needs you', 'New replies land in Needs you'],
  ['fyi', 'For your information', 'New replies land in For your information'],
  ['off', 'Off', 'Only what names you gets through'],
];

/** The thread's automatic level, while the person has not chosen one (D-001). */
export function automaticLevel(rows: MessageRow[] | undefined, me: string | undefined): ThreadLevel {
  if (!rows || !me) return 'off';
  if (rows.some((r) => r.message.author === me)) return 'needs_you';
  if (rows.some((r) => r.message.mentions.includes(me))) return 'fyi';
  return 'off';
}

function withLevel(s: SettingsAnswer, conversation: string, thread: string | null, level: DialLevel): SettingsAnswer {
  return {
    ...s,
    levels: [...s.levels.filter((l) => !(l.conversation === conversation && l.thread === thread)), { conversation, thread, level }],
  };
}

interface Props {
  session: Session;
  feed: Feed;
  conversation: ConversationSummary;
  thread: string | null;
  rows: MessageRow[] | undefined;
}

/** One click from the conversation: the channel's dial, or the thread's with Follow and Unfollow (D-001, D-035). */
export function LevelControl({ session, feed, conversation, thread, rows }: Props) {
  const [open, setOpen] = useState(false);
  const [problem, setProblem] = useState(false);
  const box = useRef<HTMLDivElement>(null);
  const id = conversation.conversation;

  useEffect(() => {
    if (!open) return;
    const away = (e: PointerEvent) => {
      if (!box.current?.contains(e.target as Node)) setOpen(false);
    };
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('pointerdown', away);
    document.addEventListener('keydown', key);
    return () => {
      document.removeEventListener('pointerdown', away);
      document.removeEventListener('keydown', key);
    };
  }, [open]);

  if (conversation.type === 'direct') return null;

  const entry = feed.settings?.levels.find((l) => l.conversation === id && l.thread === thread);
  const level: DialLevel = entry?.level ?? (thread ? automaticLevel(rows, session.me?.id) : conversation.dial);
  const choices = thread ? THREAD_LEVELS : CHANNEL_LEVELS;
  const label = choices.find(([value]) => value === level)?.[1] ?? 'Default';

  const choose = async (next: DialLevel) => {
    setOpen(false);
    setProblem(false);
    try {
      await feed.saveSetting({ kind: 'dial', conversation: id, thread, level: next }, (s) => withLevel(s, id, thread, next));
    } catch {
      setProblem(true);
    }
  };

  const menu = (
    <div className="lv-menu" role="menu" aria-label={thread ? 'Level of this thread' : 'Level of this channel'}>
      {choices.map(([value, name, note]) => (
        <button key={value} role="menuitemradio" aria-checked={value === level} onClick={() => void choose(value)}>
          <b>{name}</b>
          <small>{note}</small>
        </button>
      ))}
    </div>
  );

  return (
    <div className="lv-ctl" ref={box}>
      {problem && (
        <span className="lv-err" role="alert">
          Not saved
        </span>
      )}
      {thread ? (
        <>
          <button className="btn sm lv-follow" onClick={() => void choose(level === 'off' ? 'fyi' : 'off')}>
            {level === 'off' ? 'Follow' : 'Unfollow'}
          </button>
          <button
            className="btn sm lv-more"
            aria-haspopup="menu"
            aria-expanded={open}
            aria-label={`Thread level: ${label}`}
            title={`Thread level: ${label}`}
            onClick={() => setOpen((o) => !o)}
          >
            <Icon name="chev" />
          </button>
        </>
      ) : (
        <button
          className="btn sm lv-dial"
          aria-haspopup="menu"
          aria-expanded={open}
          aria-label={`Channel level: ${label}`}
          title="How much of this channel reaches your feed"
          onClick={() => setOpen((o) => !o)}
        >
          <span>{label}</span>
          <Icon name="chev" />
        </button>
      )}
      {open && menu}
    </div>
  );
}
