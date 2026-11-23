import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react';
import { newId } from '../api/door';
import type { Level } from '../api/types';
import { listKey } from '../state/lists';
import { mentionsIn } from '../state/mentions';
import type { Session } from '../state/useSession';
import { AutoTextarea } from './AutoTextarea';
import { useIsPhone } from './hooks';
import { useMentionBox } from './Mentions';
import { allowsEveryone } from './OwnMessage';

interface Props {
  session: Session;
  conversation: string;
  thread: string | null;
  placeholder: string;
  /** Called once a message has left the box. */
  onSent?(): void;
  /** Opens the setting for what Enter does. */
  onSendSettings?(): void;
}

const LEVEL_NOTE: Record<Level, string> = {
  flag: 'Needs attention. Stays with whoever you mention until they deal with it.',
  info: 'For the record. Notifies nobody.',
};

/** The message box, with `@` suggestions. What is sent stands in the list at once, waiting if need be (D-024). */
export function Composer({ session, conversation, thread, placeholder, onSent, onSendSettings }: Props) {
  const key = listKey(conversation, thread);
  const isPhone = useIsPhone();
  const [text, setText] = useState(() => session.drafts.get(key) ?? '');
  const [level, setLevel] = useState<Level | null>(null);
  const box = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (!isPhone) box.current?.focus({ preventScroll: true }); // on a phone the keyboard would open (D-023)
  }, [key, isPhone]);

  const change = (value: string) => {
    setText(value);
    if (value) session.drafts.set(key, value);
    else session.drafts.delete(key);
  };

  const allowEveryone = allowsEveryone(session, conversation);
  const mention = useMentionBox({ session, allowEveryone, value: text, onValue: change, box });

  const submit = () => {
    const body = text.trim();
    if (!body) return;
    const mentions = mentionsIn(body, session.peopleList, session.me?.id, allowEveryone);
    session.send({ conversation, thread, text: body, mentions, level, actionId: newId('act'), messageId: newId('msg') });
    change('');
    setLevel(null);
    onSent?.();
  };

  const onKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (mention.onKeyDown(e)) return;
    if (e.key !== 'Enter' || e.nativeEvent.isComposing) return;
    const sends = session.sendKey === 'enter' ? !e.shiftKey && !e.ctrlKey && !e.metaKey : e.ctrlKey || e.metaKey;
    if (sends) {
      e.preventDefault();
      submit();
    }
  };

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    submit();
  };

  const toggle = (value: Level) => setLevel((l) => (l === value ? null : value));
  const hint = session.sendKey === 'enter' ? 'Enter to send' : 'Ctrl+Enter to send';
  const tip =
    session.sendKey === 'enter'
      ? 'Enter sends. Shift+Enter starts a new line'
      : 'Ctrl+Enter sends. Enter starts a new line';

  return (
    <form className="composer" onSubmit={onSubmit}>
      <div className="cbox">
        <div className="reply">
          <AutoTextarea
            ref={box}
            value={text}
            aria-label="Message"
            placeholder={placeholder}
            onChange={(e) => change(e.target.value)}
            onSelect={mention.onCaret}
            onKeyDown={onKey}
          />
        </div>
        <div className="cbar">
          <div className="levels">
            <button
              type="button"
              className={`btn sm lv-flag${level === 'flag' ? ' on' : ''}`}
              aria-pressed={level === 'flag'}
              title="Flag: needs attention from whoever you mention"
              onClick={() => toggle('flag')}
            >
              Flag
            </button>
            <button
              type="button"
              className={`btn sm${level === 'info' ? ' on' : ''}`}
              aria-pressed={level === 'info'}
              title="Info: for the record, notifies nobody"
              onClick={() => toggle('info')}
            >
              Info
            </button>
            <small>{level ? LEVEL_NOTE[level] : ''}</small>
          </div>
          <button type="button" className="send-hint" title="Change what Enter does" onClick={onSendSettings}>
            {hint}
          </button>
          <button className="btn pri send" disabled={!text.trim()} title={tip}>
            Send
          </button>
        </div>
      </div>
      {mention.popup}
    </form>
  );
}
