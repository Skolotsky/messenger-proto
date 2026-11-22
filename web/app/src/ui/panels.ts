import { useCallback, useEffect, useState } from 'react';
import { stored } from './hooks';

export type PanelKey = 'side' | 'act' | 'thread';
export type Widths = Record<PanelKey, number>;

export const DEFAULT_WIDTHS: Widths = { side: 250, act: 500, thread: 400 };
const MINIMUM: Widths = { side: 180, act: 340, thread: 320 };
const MAXIMUM: Widths = { side: 380, act: 720, thread: 640 };
const CHANNEL_MIN = 300;
const KEY = 'contexia-panels';

function load(): Widths {
  try {
    const saved = JSON.parse(stored.get(KEY) ?? '{}') as Partial<Widths>;
    const out = { ...DEFAULT_WIDTHS };
    for (const k of Object.keys(out) as PanelKey[]) {
      const v = saved[k];
      if (typeof v === 'number' && Number.isFinite(v)) out[k] = Math.min(MAXIMUM[k], Math.max(MINIMUM[k], v));
    }
    return out;
  } catch {
    return { ...DEFAULT_WIDTHS };
  }
}

export interface Shown {
  activity: boolean;
  thread: boolean;
}

/** A panel can grow only as far as the others keep their smallest size inside the window. */
export function clampWidth(key: PanelKey, value: number, widths: Widths, shown: Shown, viewport: number): number {
  const others =
    CHANNEL_MIN +
    (key === 'side' ? 0 : widths.side) +
    (shown.activity && key !== 'act' ? MINIMUM.act : 0) +
    (shown.thread && key !== 'thread' ? MINIMUM.thread : 0);
  const upper = Math.max(MINIMUM[key], Math.min(MAXIMUM[key], viewport - others));
  return Math.round(Math.max(MINIMUM[key], Math.min(upper, value)));
}

export function usePanels(shown: Shown) {
  const [widths, setWidths] = useState(load);
  useEffect(() => stored.set(KEY, JSON.stringify(widths)), [widths]);

  const resize = useCallback(
    (key: PanelKey, value: number) =>
      setWidths((w) => ({ ...w, [key]: clampWidth(key, value, w, shown, window.innerWidth) })),
    [shown],
  );
  const reset = useCallback((key: PanelKey) => setWidths((w) => ({ ...w, [key]: DEFAULT_WIDTHS[key] })), []);
  return { widths, resize, reset };
}
