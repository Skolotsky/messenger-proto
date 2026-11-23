import type { Bucket, FeedCounts, FeedPart, FeedRow } from '../api/types';

/**
 * What the person sees of the feed, kept apart from what is true.
 *
 * `live` is the truth, as the door last said it. `shown` is the list on screen, whose order and
 * sections change only when the person asks (Show) or comes back to the feed (D-003, D-023).
 * A change that fits in its place is applied to `shown` at once; anything that would move
 * something waits in `waiting` behind the Show control.
 */

export type Section = 'focus' | 'kept' | 'needs_you' | 'fyi';

export interface ShownRow {
  key: string;
  section: Section;
  row: FeedRow;
  /** Marked Done here: it stays, with Keep on offer, until the person leaves the feed (D-023, point 22). */
  done: boolean;
  /** Kept here, since this list was laid out: it stays in place and says so. */
  keptNow: boolean;
}

export interface FeedModel {
  phase: 'loading' | 'ready' | 'unavailable';
  notice: string | null;
  live: Record<string, FeedRow>;
  counts: FeedCounts | null;
  shown: ShownRow[];
  waiting: string[];
  /** Rises each time the list is laid out afresh. */
  layout: number;
}

export const emptyFeed: FeedModel = {
  phase: 'loading',
  notice: null,
  live: {},
  counts: null,
  shown: [],
  waiting: [],
  layout: 0,
};

export const rowKey = (r: { conversation: string; thread: string | null }) => `${r.conversation}|${r.thread ?? ''}`;

/** Kept rows you have read sit in Focus's Kept section, whichever bucket they came from (D-003, D-011). */
export const sectionOf = (row: FeedRow): Section => (row.kept && !row.unread ? 'kept' : row.bucket);

const ORDER: Section[] = ['focus', 'kept', 'needs_you', 'fyi'];

/** Newest activity first within a section (D-003; T-038 point 16). */
export function layOut(live: Record<string, FeedRow>): ShownRow[] {
  const rows = Object.entries(live).map(([key, row]) => ({
    key,
    row,
    section: sectionOf(row),
    done: false,
    keptNow: false,
  }));
  return rows.sort(
    (a, b) => ORDER.indexOf(a.section) - ORDER.indexOf(b.section) || b.row.newest - a.row.newest,
  );
}

function withoutKey(list: string[], key: string) {
  return list.includes(key) ? list.filter((k) => k !== key) : list;
}

function withKey(list: string[], key: string) {
  return list.includes(key) ? list : [...list, key];
}

/** A first reaction would add a line to the row, so it waits; later ones fit in the line (D-001). */
const reactionsFit = (old: FeedRow, next: FeedRow) => old.reactions.length > 0 || next.reactions.length === 0;

/** Something the person has not seen yet, which only Show may bring in. */
function hasNews(old: ShownRow | undefined, next: FeedRow): boolean {
  if (!next.unread) return false;
  if (!old) return true;
  return next.newest > old.row.newest || !reactionsFit(old.row, next);
}

function applyChanged(model: FeedModel, next: FeedRow): FeedModel {
  const key = rowKey(next);
  const live = { ...model.live, [key]: next };
  const at = model.shown.findIndex((s) => s.key === key);
  const old = at >= 0 ? model.shown[at] : undefined;
  const waiting = hasNews(old, next) ? withKey(model.waiting, key) : withoutKey(model.waiting, key);
  if (!old || next.newest > old.row.newest) return { ...model, live, waiting };
  // The same content, changed: it stays where it is, in its section, and shows the truth.
  const shown = [...model.shown];
  shown[at] = {
    ...old,
    row: {
      ...next,
      bucket: old.row.bucket,
      reactions: reactionsFit(old.row, next) ? next.reactions : old.row.reactions,
    },
    done: old.done && !next.kept,
    keptNow: old.keptNow && next.kept !== null,
  };
  return { ...model, live, waiting, shown };
}

/**
 * A row left the feed. On screen it stays, quiet, until the person moves on (D-003), unless it
 * was never seen and is out of view: then it goes without a trace (D-004).
 */
function applyRemoved(model: FeedModel, key: string, inView: (key: string) => boolean): FeedModel {
  const live = { ...model.live };
  delete live[key];
  const waiting = withoutKey(model.waiting, key);
  const at = model.shown.findIndex((s) => s.key === key);
  if (at < 0) return { ...model, live, waiting };
  const old = model.shown[at];
  if (old.row.unread && !inView(key)) {
    return { ...model, live, waiting, shown: model.shown.filter((s) => s.key !== key) };
  }
  const shown = [...model.shown];
  shown[at] = { ...old, row: { ...old.row, unread: false } };
  return { ...model, live, waiting, shown };
}

export type FeedEvent =
  | { type: 'loaded'; rows: FeedRow[]; counts: FeedCounts; notice: string | null }
  /** A fresh answer while the list is in view: treated as changes, so that nothing moves. */
  | { type: 'refreshed'; rows: FeedRow[]; counts: FeedCounts; notice: string | null; inView(key: string): boolean }
  | { type: 'notice'; notice: string | null | undefined }
  | { type: 'unavailable'; notice: string }
  | { type: 'parts'; parts: FeedPart[]; inView(key: string): boolean }
  | { type: 'deleted'; message: string; inView(key: string): boolean }
  | { type: 'show' }
  | { type: 'read'; key: string }
  | { type: 'keep'; key: string }
  | { type: 'done'; key: string }
  | { type: 'reacted'; key: string; emoji: string; me: string; on: boolean; release: boolean };

function recount(counts: FeedCounts | null, before: FeedRow | undefined, after: FeedRow | undefined): FeedCounts | null {
  if (!counts) return counts;
  const next = { ...counts };
  const tally = (row: FeedRow | undefined, by: 1 | -1) => {
    if (!row) return;
    if (row.unread && row.bucket === 'focus') next.focus = Math.max(0, next.focus + by);
    if (row.unread && row.bucket === 'needs_you') next.needs_you = Math.max(0, next.needs_you + by);
    if (row.kept) next.kept = Math.max(0, next.kept + by);
  };
  tally(before, -1);
  tally(after, 1);
  return next;
}

function changeRow(model: FeedModel, key: string, change: (row: FeedRow) => FeedRow, mark: Partial<ShownRow>): FeedModel {
  const before = model.live[key];
  const after = before ? change(before) : undefined;
  const live = after ? { ...model.live, [key]: after } : model.live;
  const shown = model.shown.map((s) => (s.key === key ? { ...s, ...mark, row: change(s.row) } : s));
  let counts = recount(model.counts, before, after);
  if (counts) counts = { ...counts, fyi: Object.values(live).some((r) => r.bucket === 'fyi' && r.unread) };
  return { ...model, live, shown, counts, waiting: after?.unread ? model.waiting : withoutKey(model.waiting, key) };
}

export function feedReduce(model: FeedModel, event: FeedEvent): FeedModel {
  switch (event.type) {
    case 'loaded': {
      const live = Object.fromEntries(event.rows.map((r) => [rowKey(r), r]));
      return {
        phase: 'ready',
        notice: event.notice,
        live,
        counts: event.counts,
        shown: layOut(live),
        waiting: [],
        layout: model.layout + 1,
      };
    }
    case 'refreshed': {
      let next: FeedModel = { ...model, phase: 'ready', notice: event.notice };
      const keys = new Set(event.rows.map(rowKey));
      for (const key of Object.keys(model.live)) if (!keys.has(key)) next = applyRemoved(next, key, event.inView);
      for (const row of event.rows) next = applyChanged(next, row);
      return { ...next, counts: event.counts };
    }
    case 'notice':
      return model.notice === (event.notice ?? null) ? model : { ...model, notice: event.notice ?? null };
    case 'unavailable':
      return { ...model, phase: model.phase === 'loading' ? 'unavailable' : model.phase, notice: event.notice };
    case 'parts': {
      let next = model;
      for (const part of event.parts) {
        if (part.kind === 'row.changed') next = applyChanged(next, part.row);
        else if (part.kind === 'row.removed') next = applyRemoved(next, rowKey(part.row), event.inView);
        else if (part.kind === 'counts') next = { ...next, counts: part.counts };
      }
      return next;
    }
    case 'deleted': {
      // The text is gone everywhere at once, also from a row that stays on screen as a marker (D-004).
      const hit = model.shown.find((s) => s.row.message === event.message);
      if (!hit) return model;
      if (hit.row.unread && !event.inView(hit.key)) return applyRemoved(model, hit.key, event.inView);
      const shown = model.shown.map((s) =>
        s === hit ? { ...s, row: { ...s.row, state: 'retracted' as const, preview: '', reactions: [] } } : s,
      );
      return { ...model, shown };
    }
    case 'show': {
      return { ...model, shown: layOut(model.live), waiting: [], layout: model.layout + 1 };
    }
    case 'read':
      return changeRow(model, event.key, (r) => ({ ...r, unread: false }), {});
    case 'keep':
      return changeRow(
        model,
        event.key,
        (r) => ({ ...r, unread: false, kept: r.kept === 'flag' ? 'both' : (r.kept ?? 'you') }),
        { done: false, keptNow: true },
      );
    case 'done':
      return changeRow(model, event.key, (r) => ({ ...r, unread: false, kept: null }), { done: true, keptNow: false });
    case 'reacted': {
      const { emoji, me, on, release } = event;
      const toggled = (r: FeedRow): FeedRow => {
        const others = r.reactions.filter((x) => x.emoji !== emoji);
        const current = r.reactions.find((x) => x.emoji === emoji)?.people ?? [];
        const people = on ? [...new Set([...current, me])] : current.filter((p) => p !== me);
        const reactions = current.length || !people.length ? r.reactions.map((x) => (x.emoji === emoji ? { emoji, people } : x)).filter((x) => x.people.length) : [...others, { emoji, people }];
        // Acting implies read (D-015); by the person's setting it also releases a kept row (D-023, point 26).
        return { ...r, reactions, unread: false, kept: release ? null : r.kept };
      };
      const wasKept = model.live[event.key]?.kept ?? model.shown.find((s) => s.key === event.key)?.row.kept;
      return changeRow(model, event.key, toggled, release && wasKept ? { done: true, keptNow: false } : {});
    }
  }
}

/** How many of the waiting rows land where. The control names Focus and Needs you (D-023). */
export function waitingBy(model: FeedModel): Record<Bucket, number> {
  const out: Record<Bucket, number> = { focus: 0, needs_you: 0, fyi: 0 };
  for (const key of model.waiting) {
    const row = model.live[key];
    if (row) out[row.bucket] += 1;
  }
  return out;
}
