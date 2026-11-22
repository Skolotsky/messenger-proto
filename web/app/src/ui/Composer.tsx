import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from 'react';
import { DoorError, newId } from '../api/door';
import type { Level } from '../api/types';
import { listKey } from '../state/lists';
import type { Session } from '../state/useSession';
import { AutoTextarea } from './AutoTextarea';
import { useIsPhone } from './hooks';

interface Props {
  session: Session;
  conversation: string;
  thread: string | null;
  placeholder: string;
}

const LEVEL_NOTE: Record<Level, string> = {
  flag: 'Needs attention. Stays with whoever you mention until they deal with it.',
  info: 'For the record. Notifies nobody.',
};

/**
 * The message box. Seams for T-056: `@` suggestions, and the waiting and failed states of a
 * message in the list. Until then a send that fails keeps its text here and says so.
 */
export function Composer({ session, conversation, thread, placeholder }: Props) {
  const key = listKey(conversation, thread);
  const isPhone = useIsPhone();
  const [text, setText] = useState(() => session.drafts.get(key) ?? '');
  const [level, setLevel] = useState<Level | null>(null);
  const [sending, setSending] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  const box = useRef<HTMLTextAreaElement>(null);
  // The same text sent again keeps its IDs, so a send whose answer was lost is saved once.
  const attempt = useRef<{ body: string; actionId: string; messageId: string } | null>(null);

  useEffect(() => {
    if (!isPhone) box.current?.focus({ preventScroll: true }); // on a phone the keyboard would open (D-023)
  }, [key, isPhone]);

  const change = (value: string) => {
    setText(value);
    setProblem(null);
    if (value) session.drafts.set(key, value);
    else session.drafts.delete(key);
  };

  const submit = async () => {
    const body = text.trim();
    if (!body || sending) return;
    const signature = JSON.stringify([body, level]);
    if (attempt.current?.body !== signature) {
      attempt.current = { body: signature, actionId: newId('act'), messageId: newId('msg') };
    }
    const { actionId, messageId } = attempt.current;
    setSending(true);
    setProblem(null);
    try {
      await session.send({ conversation, thread, text: body, level, actionId, messageId });
      attempt.current = null;
      change('');
      setLevel(null);
    } catch (e) {
      const refused = e instanceof DoorError && e.kind === 'refused';
      if (refused) attempt.current = null;
      setProblem(
        refused
          ? `Not sent: ${(e as DoorError).message}. Your text is still here.`
          : 'Not sent: no answer from the server. Your text is still here, try again.',
      );
    } finally {
      setSending(false);
    }
  };

  const onKey = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key !== 'Enter' || e.nativeEvent.isComposing) return;
    const sends = session.sendKey === 'enter' ? !e.shiftKey && !e.ctrlKey && !e.metaKey : e.ctrlKey || e.metaKey;
    if (sends) {
      e.preventDefault();
      void submit();
    }
  };

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    void submit();
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
            <small role={problem ? 'alert' : undefined} className={problem ? 'err' : undefined}>
              {problem ?? (level ? LEVEL_NOTE[level] : '')}
            </small>
          </div>
          <span className="send-hint">{hint}</span>
          <button className="btn pri send" disabled={sending || !text.trim()} title={tip}>
            Send
          </button>
        </div>
      </div>
    </form>
  );
}
