import { Fragment, useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import type { MessageRow } from '../api/types';
import type { Jump } from './jump';
import { dayKey, dayLabel } from './format';

interface Props {
  rows: MessageRow[];
  renderRow(row: MessageRow): ReactNode;
  onPinnedChange(pinned: boolean): void;
  /** Shown after the first row, for a thread: how many replies follow its root. */
  afterFirst?: ReactNode;
  /** Opened from a feed row: start at this number, under a New line (D-023, point 21). */
  newFrom?: number | null;
  /** A search hit to bring into view, once. */
  target?: Jump | null;
  onLanded?(nonce: number): void;
  /** Under the last message: replies being written. */
  footer?: ReactNode;
}

const NEAR_BOTTOM = 8;

/**
 * The scrolling list of a conversation. Nothing the person is reading moves: while they are
 * away from the bottom, the message at the top of the view keeps its exact place through
 * arrivals and through a change of the panel's width. At the bottom, the list stays there.
 */
export function MessageList({ rows, renderRow, onPinnedChange, afterFirst, newFrom = null, target, onLanded, footer }: Props) {
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

  // Held once, for this opening: a message arriving later must not move the line.
  const [newLine] = useState(() => rows.find((r) => r.message.number >= (newFrom ?? Infinity))?.message.id ?? null);

  useLayoutEffect(() => {
    const box = scroller.current;
    if (!newLine || !box) return;
    const line = box.querySelector<HTMLElement>('.newline');
    if (!line) return;
    box.scrollTop += line.getBoundingClientRect().top - box.getBoundingClientRect().top - 8;
    pinned.current = box.scrollHeight - box.scrollTop - box.clientHeight < NEAR_BOTTOM;
    onPinnedChange(pinned.current);
    capture();
  }, [newLine, capture, onPinnedChange]);

  useLayoutEffect(hold, [rows, hold]);

  // Opens at the hit: it stands in the middle of the view, and the list stops following the bottom.
  const landed = useRef<number | null>(null);
  useLayoutEffect(() => {
    const box = scroller.current;
    if (!box || !target || landed.current === target.nonce) return;
    const el = box.querySelector<HTMLElement>(`[data-msg="${CSS.escape(target.message)}"]`);
    if (!el) return;
    landed.current = target.nonce;
    box.querySelector('[data-hit]')?.removeAttribute('data-hit');
    const rect = el.getBoundingClientRect();
    const view = box.getBoundingClientRect();
    box.scrollTop += rect.top - view.top - (box.clientHeight - rect.height) / 2;
    el.setAttribute('data-hit', '');
    const isPinned = box.scrollHeight - box.scrollTop - box.clientHeight < NEAR_BOTTOM;
    pinned.current = isPinned;
    onPinnedChange(isPinned);
    capture();
    onLanded?.(target.nonce);
  }, [rows, target, capture, onPinnedChange, onLanded]);

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
              {row.message.id === newLine && (
                <div className="newline" role="separator">
                  <span>New</span>
                </div>
              )}
              {renderRow(row)}
              {index === 0 && afterFirst}
            </Fragment>
          );
        })}
        {footer}
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
