import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { EMOJI } from './emoji';
import { useIsPhone } from './hooks';

interface Request {
  label: string;
  x: number;
  y: number;
  choose(emoji: string): void;
}

const PickerContext = createContext<(request: Request) => void>(() => undefined);

/** Opens the one picker of the app, from a message or from a row that was not opened. */
export const usePicker = () => useContext(PickerContext);

export function PickerProvider({ children }: { children: ReactNode }) {
  const [request, setRequest] = useState<Request | null>(null);
  const open = useCallback((r: Request) => setRequest(r), []);
  const close = useCallback(() => setRequest(null), []);
  return (
    <PickerContext.Provider value={open}>
      {children}
      {request && <Picker request={request} onClose={close} />}
    </PickerContext.Provider>
  );
}

const WIDTH = 308;
const HEIGHT = 360;

function Picker({ request, onClose }: { request: Request; onClose(): void }) {
  const [query, setQuery] = useState('');
  const isPhone = useIsPhone();
  const root = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const opener = useRef(document.activeElement as HTMLElement | null);

  useEffect(() => {
    if (!isPhone) input.current?.focus({ preventScroll: true }); // on a phone the keyboard would cover it
    const back = opener.current;
    const outside = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) onClose();
    };
    const esc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        onClose();
      }
    };
    document.addEventListener('pointerdown', outside);
    document.addEventListener('keydown', esc, true);
    return () => {
      document.removeEventListener('pointerdown', outside);
      document.removeEventListener('keydown', esc, true);
      if (back?.isConnected) back.focus({ preventScroll: true });
    };
  }, [isPhone, onClose]);

  const sections = useMemo(() => {
    const q = query.trim().toLowerCase();
    return Object.entries(EMOJI)
      .map(([name, list]) => [name, q ? list.filter(([, words]) => words.includes(q)) : list] as const)
      .filter(([, list]) => list.length > 0);
  }, [query]);

  const style = isPhone
    ? undefined
    : {
        left: Math.max(8, Math.min(request.x - WIDTH / 2, innerWidth - WIDTH - 8)),
        top: Math.max(8, Math.min(request.y + 10, innerHeight - HEIGHT - 8)),
      };

  return (
    <div className="picker" role="dialog" aria-label="Choose a reaction" style={style} ref={root}>
      <div className="pk-head">
        <small>{request.label}</small>
        <input
          ref={input}
          type="search"
          value={query}
          placeholder="Search emoji"
          aria-label="Search emoji"
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>
      <div className="pk-body">
        {sections.map(([name, list]) => (
          <div className="pk-sec" key={name}>
            <h4>{name}</h4>
            <div className="pk-grid">
              {list.map(([emoji, words]) => (
                <button
                  key={emoji}
                  aria-label={words}
                  title={words.split(' ')[0]}
                  onClick={() => {
                    request.choose(emoji);
                    onClose();
                  }}
                >
                  {emoji}
                </button>
              ))}
            </div>
          </div>
        ))}
        {sections.length === 0 && <p className="pk-none">No emoji by that name.</p>}
      </div>
    </div>
  );
}
