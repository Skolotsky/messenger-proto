import { useRef, type KeyboardEvent, type PointerEvent } from 'react';
import type { PanelKey } from './panels';

interface Props {
  panel: PanelKey;
  label: string;
  width: number;
  /** The handle sits on the panel's left edge, so dragging left makes it wider. */
  fromRight?: boolean;
  onResize(panel: PanelKey, width: number): void;
  onReset(panel: PanelKey): void;
}

const STEP = 16;

/** The line between two panels: drag it, use the arrow keys, double-click to put it back (D-023). */
export function Resizer({ panel, label, width, fromRight, onResize, onReset }: Props) {
  const drag = useRef<{ x: number; width: number } | null>(null);
  const sign = fromRight ? -1 : 1;

  const down = (e: PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    e.currentTarget.setPointerCapture(e.pointerId);
    drag.current = { x: e.clientX, width };
    document.body.classList.add('resizing');
  };
  const move = (e: PointerEvent<HTMLDivElement>) => {
    if (drag.current) onResize(panel, drag.current.width + sign * (e.clientX - drag.current.x));
  };
  const up = () => {
    drag.current = null;
    document.body.classList.remove('resizing');
  };
  const key = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'ArrowLeft') onResize(panel, width - sign * STEP);
    else if (e.key === 'ArrowRight') onResize(panel, width + sign * STEP);
    else return;
    e.preventDefault();
  };

  return (
    <div
      className={`rz${fromRight ? ' rz-left' : ''}`}
      data-rz={panel}
      role="separator"
      aria-orientation="vertical"
      aria-label={label}
      aria-valuenow={width}
      tabIndex={0}
      title="Drag to resize. Double-click to reset"
      onPointerDown={down}
      onPointerMove={move}
      onPointerUp={up}
      onPointerCancel={up}
      onDoubleClick={() => onReset(panel)}
      onKeyDown={key}
    />
  );
}
