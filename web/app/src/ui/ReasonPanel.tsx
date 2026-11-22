import { useEffect, useRef, useState } from 'react';
import { loadPreview } from '../api/door';
import type { Bucket, RowAct, RowChange } from '../api/types';
import type { ShownRow } from '../state/feedModel';
import type { Feed } from '../state/useFeed';
import type { Session } from '../state/useSession';
import { useIsPhone } from './hooks';
import { Icon } from './Icon';
import type { SettingsSection } from './SettingsScreen';

export const BUCKET_NAME: Record<Bucket, string> = {
  focus: 'Focus',
  needs_you: 'Needs you',
  fyi: 'For your information',
};

const WIDTH = 380;

/** The place in settings that holds what this change touches (D-023, point 18). */
function settingsPlace(change: RowAct | undefined): SettingsSection {
  if (change?.kind === 'focus_person') return 'people';
  if (change?.kind === 'dial' && change.thread === null) return 'channels';
  return 'feed';
}

function previewLine(moved: number | null | undefined, from?: Bucket | null, to?: Bucket | null) {
  if (moved === null || moved === undefined) return null;
  const items = `${moved} ${moved === 1 ? 'item' : 'items'} in the last 7 days`;
  if (to) return `${items} would have gone to ${BUCKET_NAME[to]}${from ? ` instead of ${BUCKET_NAME[from]}` : ''}.`;
  return `${items} would have moved.`;
}

interface Props {
  shown: ShownRow;
  anchor: DOMRect;
  session: Session;
  feed: Feed;
  /** A change already made from this row in this visit, so that Undo is still there. */
  made: RowChange | null;
  onMade(change: RowChange | null): void;
  onSettings(section: SettingsSection): void;
  onClose(): void;
}

/** Why a row is where it is, and the one thing to change about it. It never reads the row (D-020). */
export function ReasonPanel({ shown, anchor, session, feed, made, onMade, onSettings, onClose }: Props) {
  const { row } = shown;
  const e = row.explanation;
  const change = e.change;
  const isPhone = useIsPhone();
  const root = useRef<HTMLDivElement>(null);
  const opener = useRef(document.activeElement as HTMLElement | null);
  const [preview, setPreview] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  const person = session.me?.id;
  const bucket = BUCKET_NAME[row.bucket];

  useEffect(() => {
    root.current?.focus({ preventScroll: true });
    const back = opener.current;
    const outside = (ev: PointerEvent) => {
      const target = ev.target as HTMLElement;
      if (!root.current?.contains(target) && !target.closest?.('.why-btn')) onClose();
    };
    const esc = (ev: KeyboardEvent) => {
      if (ev.key === 'Escape') onClose();
    };
    document.addEventListener('pointerdown', outside);
    document.addEventListener('keydown', esc);
    return () => {
      document.removeEventListener('pointerdown', outside);
      document.removeEventListener('keydown', esc);
      if (back?.isConnected) back.focus({ preventScroll: true });
    };
  }, [onClose]);

  // The count is a courtesy: when the feed cannot answer, the button still works (screen-to-conversations.md).
  useEffect(() => {
    if (!change || !person) return;
    let live = true;
    setPreview(previewLine(change.moved_last_7_days));
    loadPreview(person, change.do).then(
      ({ last_7_days: p }) => {
        if (live) setPreview(previewLine(p.moved, p.from, p.to));
      },
      () => undefined,
    );
    return () => {
      live = false;
    };
  }, [change, person]);

  const run = async (what: RowAct, after: RowChange | null) => {
    setBusy(true);
    setProblem(null);
    try {
      await feed.perform(what);
      onMade(after);
    } catch {
      setProblem('That did not go through. Nothing changed. Try again.');
    } finally {
      setBusy(false);
    }
  };

  const style = isPhone
    ? undefined
    : (() => {
        const left = Math.max(8, Math.min(anchor.left, innerWidth - WIDTH - 8));
        const below = innerHeight - anchor.bottom;
        return below > 300
          ? { left, top: anchor.bottom + 6 }
          : { left, bottom: Math.max(8, innerHeight - anchor.top + 6) };
      })();

  return (
    <>
      {isPhone && <div className="why-scrim" onClick={onClose} />}
      <div className="why-pop" role="dialog" aria-label={`Why this is in ${bucket}`} tabIndex={-1} ref={root} style={style}>
        <div className="why-top">
          <h3>Why this is in {bucket}</h3>
          <button className="icon-btn why-x" aria-label="Close" onClick={onClose}>
            <Icon name="close" />
          </button>
        </div>
        <div className="why-steps">
          <div>
            <i>1</i>
            <span>
              <b>What it is</b>
              {e.what}
            </span>
          </div>
          <div>
            <i>2</i>
            <span>
              <b>What put it in {bucket}</b>
              {e.decided_by}
            </span>
          </div>
          <div>
            <i>3</i>
            <span>
              <b>Otherwise</b>
              {e.otherwise}
            </span>
          </div>
        </div>
        <div className="why-acts">
          {made ? (
            <p className="why-off" role="status">
              <b>{made.label}: done.</b> This row stays where it is; the change applies the next time you open the feed.{' '}
              <button className="link" disabled={busy} onClick={() => void run(made.undo, null)}>
                Undo
              </button>
            </p>
          ) : (
            change && (
              <>
                <b className="why-q">Not what you want in {bucket}?</b>
                <div className="why-act">
                  <button className="btn sm" disabled={busy} onClick={() => void run(change.do, change)}>
                    {change.label}
                  </button>
                  {preview && <small>{preview}</small>}
                </div>
              </>
            )
          )}
          {problem && (
            <p className="err" role="alert">
              {problem}
            </p>
          )}
          <button className="link" onClick={() => onSettings(settingsPlace((made ?? change)?.do))}>
            Manage this in settings
          </button>
        </div>
      </div>
    </>
  );
}
