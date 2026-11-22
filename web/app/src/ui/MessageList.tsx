import { Fragment, useCallback, useEffect, useLayoutEffect, useRef, type ReactNode } from 'react';
import type { MessageRow } from '../api/types';
import { dayKey, dayLabel } from './format';

interface Props {
  rows: MessageRow[];
  renderRow(row: MessageRow): ReactNode;
  onPinnedChange(pinned: boolean): void;
  /** Shown after the first row, for a thread: how many replies follow its root. */
  afterFirst?: ReactNode;
}

const NEAR_BOTTOM = 8;

/**
 * The scrolling list of a conversation. Nothing the person is reading moves: while they are
 * away from the bottom, the message at the top of the view keeps its exact place through
 * arrivals and through a change of the panel's width. At the bottom, the list stays there.
 */
export function MessageList({ rows, renderRow, onPinnedChange, afterFirst }: Props) {
  const scroller = useRef<HTMLDivElement>(null);
  const inner = useRef<HTMLDivElement>(null);
  const pinned = useRef(true);
  const anchor = useRef<{ id: string; top: number } | null>(null);

  const capture = useCallback(() => {
    const box = scroller.current;
    if (!box) return;
    const top = box.getBoundingClientRect().top;
    for (const el of box.querySelectorAll<HTMLElement>('[data-msg]')) {
      const rect = el.getBoundingClientRect();
      if (rect.bottom > top + 1) {
        anchor.current = { id: el.dataset.msg!, top: rect.top - top };
        return;
      }
    }
  }, []);

  const hold = useCallback(() => {
    const box = scroller.current;
    if (!box) return;
    if (pinned.current) {
      box.scrollTop = box.scrollHeight;
      return;
    }
    const held = anchor.current;
    if (!held) return;
    const el = box.querySelector<HTMLElement>(`[data-msg="${CSS.escape(held.id)}"]`);
    if (!el) return;
    const delta = el.getBoundingClientRect().top - box.getBoundingClientRect().top - held.top;
    if (Math.abs(delta) > 0.1) box.scrollTop += delta;
  }, []);

  useLayoutEffect(hold, [rows, hold]);

  useEffect(() => {
    const observer = new ResizeObserver(hold);
    if (scroller.current) observer.observe(scroller.current);
    if (inner.current) observer.observe(inner.current);
    return () => observer.disconnect();
  }, [hold]);

  const onScroll = () => {
    const box = scroller.current;
    if (!box) return;
    const isPinned = box.scrollHeight - box.scrollTop - box.clientHeight < NEAR_BOTTOM;
    if (isPinned !== pinned.current) {
      pinned.current = isPinned;
      onPinnedChange(isPinned);
    }
    capture();
  };

  let lastDay: number | null = null;
  return (
    <div className="conv-body" ref={scroller} onScroll={onScroll} data-testid="messages">
      <div ref={inner}>
        {rows.map((row, index) => {
          const day = dayKey(row.message.created_at);
          const line = day !== lastDay ? <DayLine iso={row.message.created_at} /> : null;
          lastDay = day;
          return (
            <Fragment key={row.message.id}>
              {line}
              {renderRow(row)}
              {index === 0 && afterFirst}
            </Fragment>
          );
        })}
      </div>
    </div>
  );
}

function DayLine({ iso }: { iso: string }) {
  return (
    <div className="dayline" role="separator">
      <span>{dayLabel(iso)}</span>
    </div>
  );
}
