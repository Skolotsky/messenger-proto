import { useLayoutEffect, useRef, useState, type MouseEvent, type PointerEvent } from 'react';
import type { FeedRow, Reaction } from '../api/types';
import type { ShownRow } from '../state/feedModel';
import type { Feed } from '../state/useFeed';
import type { Session } from '../state/useSession';
import { Avatar } from './Avatar';
import { conversationTitle } from './ConversationView';
import { rowTime } from './format';
import { useIsPhone } from './hooks';
import { Icon } from './Icon';
import { usePicker } from './ReactionPicker';

const SWIPE_AT = 70;
const firstName = (name: string) => name.split(' ')[0];

interface Props {
  shown: ShownRow;
  session: Session;
  feed: Feed;
  whyOpen: boolean;
  onWhy(row: ShownRow, anchor: DOMRect): void;
  onOpen(row: FeedRow): void;
}

/** Who reacted, as the row shows it: names while they fit, then the true count (D-014; T-038 point 18). */
export function reactionLabel(r: Reaction, session: Session) {
  if (r.people.length > 2) return String(r.people.length);
  return r.people
    .map((p) => (p === session.me?.id ? 'You' : firstName(session.people.get(p)?.name ?? p)))
    .join(', ');
}

export function FeedRowItem({ shown, session, feed, whyOpen, onWhy, onOpen }: Props) {
  const { row, done, keptNow } = shown;
  const isPhone = useIsPhone();
  const openPicker = usePicker();
  const main = useRef<HTMLDivElement>(null);
  const height = useRef<number | null>(null);
  const width = useRef<number | null>(null);
  const drag = useRef<{ x: number; y: number; dx: number; on: boolean } | null>(null);
  const swallow = useRef(false);
  const [offset, setOffset] = useState(0);

  // Nothing jumps (D-001, D-004): content that changes in place keeps the row's exact height.
  const content = `${row.preview}|${row.state}|${row.reactions.length}|${row.why}|${done}|${keptNow}`;
  useLayoutEffect(() => {
    const el = main.current;
    if (!el) return;
    const w = el.offsetWidth;
    if (width.current !== null && width.current !== w) {
      el.style.height = ''; // the panel was resized: the text wraps anew
      height.current = null;
    }
    width.current = w;
    if (height.current === null) height.current = el.offsetHeight;
    else if (el.offsetHeight !== height.current) el.style.height = `${height.current}px`;
  }, [content]);

  useLayoutEffect(() => {
    const el = main.current;
    if (!el) return;
    const observer = new ResizeObserver(() => {
      if (el.offsetWidth === width.current) return;
      width.current = el.offsetWidth;
      el.style.height = '';
      height.current = el.offsetHeight;
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const conversation = session.conversations.find((c) => c.conversation === row.conversation);
  const where = conversation ? conversationTitle(conversation, session) : row.conversation;
  const isDirect = conversation?.type === 'direct';
  const isChannelRow = !isDirect && row.thread === null;
  const author = session.people.get(row.author);
  const authorName = row.author === session.me?.id ? 'You' : (author?.name ?? row.author);
  const title = isDirect ? where : isChannelRow ? where : authorName;
  const sub = isDirect || isChannelRow ? '' : where;
  const keptNowOrBefore = row.kept !== null && !done;
  const retracted = row.state === 'retracted';
  const stateWord = done ? 'Done' : keptNow ? 'Kept' : null;

  const stop = (e: MouseEvent) => e.stopPropagation();

  const pick = (e: MouseEvent<HTMLButtonElement>) => {
    e.stopPropagation();
    const box = e.currentTarget.getBoundingClientRect();
    openPicker({
      label: `React to ${row.author === session.me?.id ? 'your' : `${firstName(author?.name ?? row.author)}'s`} message`,
      x: box.left + box.width / 2,
      y: box.bottom,
      choose: (emoji) => feed.react(row, emoji),
    });
  };

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    swallow.current = false;
    if (!isPhone || retracted || (e.target as HTMLElement).closest('button:not(.row-hit)')) return;
    drag.current = { x: e.clientX, y: e.clientY, dx: 0, on: false };
  };
  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    const d = drag.current;
    if (!d) return;
    d.dx = e.clientX - d.x;
    if (!d.on && Math.abs(d.dx) > 8 && Math.abs(d.dx) > Math.abs(e.clientY - d.y)) {
      d.on = true;
      e.currentTarget.setPointerCapture(e.pointerId);
    }
    if (d.on) setOffset(Math.max(-150, Math.min(120, d.dx)));
  };
  const endDrag = () => {
    const d = drag.current;
    drag.current = null;
    if (!d?.on) return;
    swallow.current = true;
    setOffset(0);
    if (d.dx > SWIPE_AT) {
      if (keptNowOrBefore) feed.done(row);
      else if (row.unread) feed.markRead(row);
    } else if (d.dx < -SWIPE_AT && !keptNowOrBefore) {
      feed.keep(row);
    }
  };

  if (retracted) {
    const who = row.author === session.me?.id ? 'You' : firstName(author?.name ?? row.author);
    return (
      <article className={`row gone b-${row.bucket}`} data-row={shown.key}>
        <div className="row-main" ref={main}>
          <span className="tomb">{row.preview === '' ? `${who} deleted this message` : `${who} took this back`}</span>
          <small>
            {where}
            {row.kept === 'you' || row.kept === 'both' ? ' · you had kept it' : ''}
          </small>
          {keptNowOrBefore && (
            <span className="row-side">
              <button className="btn sm pri" onClick={() => feed.done(row)}>
                Done
              </button>
            </span>
          )}
        </div>
      </article>
    );
  }

  const classes = [
    'row',
    `b-${row.bucket}`,
    row.unread ? 'unread' : 'read',
    shown.section === 'kept' && 'kept',
    done && 'done',
    (stateWord || keptNowOrBefore) && 'has-side',
    whyOpen && 'why-on',
  ]
    .filter(Boolean)
    .join(' ');

  const label = `${row.unread ? 'Unread: ' : ''}${title}${sub ? ` in ${sub}` : ''}. ${row.why}. Open the conversation.`;

  return (
    <article className={classes} data-row={shown.key}>
      {isPhone && (
        <div className="sw-under" aria-hidden="true">
          <span className="sw-l">
            <Icon name="check" /> {keptNowOrBefore ? 'Done' : 'Mark read'}
          </span>
          <span className="sw-k">
            {keptNowOrBefore ? 'Kept' : done ? 'Keep again' : 'Keep'} <Icon name="pin" />
          </span>
        </div>
      )}
      <div
        className="row-main"
        ref={main}
        style={offset ? { transform: `translateX(${offset}px)`, transition: 'none' } : undefined}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
      >
        <button
          className="row-hit"
          aria-label={label}
          onClick={() => {
            if (swallow.current) {
              swallow.current = false;
              return;
            }
            onOpen(row);
          }}
        />
        {isChannelRow ? (
          <span className="av ch" aria-hidden="true">
            #
          </span>
        ) : (
          <Avatar id={row.author} person={author} />
        )}
        <div className="row-text">
          <div className="row-head">
            <span className="row-title">{title}</span>
            {sub && <span className="row-sub">{sub}</span>}
            {(row.kept === 'flag' || row.kept === 'both') && (
              <span className="tag flag" title="Flagged for you">
                Flag
              </span>
            )}
            {row.unread && row.new_items > 1 && <span className="row-count">{row.new_items} new</span>}
            <span className="row-time">
              {rowTime(row.at)}
              {row.state === 'changed' && ' · edited'}
            </span>
          </div>
          <div className="row-reason">
            <span className="why-dot" />
            <button
              className="why-btn"
              aria-expanded={whyOpen}
              aria-haspopup="dialog"
              title="Why is this here?"
              onClick={(e) => {
                e.stopPropagation();
                onWhy(shown, e.currentTarget.getBoundingClientRect());
              }}
            >
              <span>{row.why}</span>
              <Icon name="chev" />
            </button>
            {row.kept && row.state === 'changed' && <em>edited since you kept it</em>}
            {row.fenced && <em className="fenced">may be out of date</em>}
          </div>
          <div className="row-preview">
            {isChannelRow && (
              <b>{row.author === session.me?.id ? 'You' : firstName(author?.name ?? row.author)}: </b>
            )}
            {row.preview}
          </div>
          {row.reactions.length > 0 && (
            <div className="rxs row-rxs">
              {row.reactions.map((r) => (
                <button
                  key={r.emoji}
                  className={`rx${session.me && r.people.includes(session.me.id) ? ' on' : ''}`}
                  title={r.people.map((p) => session.people.get(p)?.name ?? p).join(', ')}
                  aria-label={`${r.emoji}, ${r.people.length} ${r.people.length === 1 ? 'person' : 'people'}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    feed.react(row, r.emoji);
                  }}
                >
                  {r.emoji} <i>{reactionLabel(r, session)}</i>
                </button>
              ))}
            </div>
          )}
        </div>
        <div className="row-side" onClick={stop}>
          {stateWord && <span className="state">{stateWord}</span>}
          <span className="row-btns">
            <button className="rx-pick" aria-label="React" title="React without opening" onClick={pick}>
              <Icon name="smile" />
            </button>
            {row.unread && (
              <button className="mark-read" aria-label="Mark read" title="Mark read without opening" onClick={() => feed.markRead(row)}>
                <Icon name="check" />
              </button>
            )}
            {!keptNowOrBefore && (
              <button
                className="keep-btn"
                aria-label={done ? 'Keep again' : 'Keep'}
                title={done ? 'Keep again: undo Done' : 'Keep until you press Done'}
                onClick={() => feed.keep(row)}
              >
                <Icon name="pin" />
              </button>
            )}
          </span>
          {keptNowOrBefore && (
            <button className="btn sm pri done-btn" title="Done: release this row" onClick={() => feed.done(row)}>
              Done
            </button>
          )}
        </div>
      </div>
    </article>
  );
}
