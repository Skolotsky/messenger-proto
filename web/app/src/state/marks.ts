export interface Piece {
  text: string;
  marked: boolean;
}

/** Sorted, within the text, and joined where they touch. Positions count code points, end exclusive. */
function clean(spans: number[][], length: number): [number, number][] {
  const sorted = spans
    .map(([from, to]): [number, number] => [Math.max(0, from), Math.min(length, to)])
    .filter(([from, to]) => from < to)
    .sort((a, b) => a[0] - b[0]);
  const out: [number, number][] = [];
  for (const span of sorted) {
    const last = out[out.length - 1];
    if (last && span[0] <= last[1]) last[1] = Math.max(last[1], span[1]);
    else out.push([...span]);
  }
  return out;
}

/**
 * Splits a text at its match marks. Around a long text it keeps a window from just before the
 * first match, so that the match is always in what is shown.
 */
export function markedPieces(text: string, spans: number[][], window = 160): Piece[] {
  const points = Array.from(text);
  const marks = clean(spans, points.length);
  const start = marks.length && points.length > window ? Math.max(0, marks[0][0] - 40) : 0;
  const end = Math.min(points.length, start + window);
  const piece = (from: number, to: number, marked: boolean): Piece => ({ text: points.slice(from, to).join(''), marked });
  const out: Piece[] = [];
  if (start > 0) out.push({ text: '…', marked: false });
  let at = start;
  for (const [from, to] of marks) {
    if (to <= start || from >= end) continue;
    const a = Math.max(from, start);
    const b = Math.min(to, end);
    if (a > at) out.push(piece(at, a, false));
    out.push(piece(a, b, true));
    at = b;
  }
  if (at < end) out.push(piece(at, end, false));
  if (end < points.length) out.push({ text: '…', marked: false });
  return out;
}
