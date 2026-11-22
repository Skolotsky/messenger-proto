import { useEffect, useRef, useState } from 'react';
import type { PersonView } from '../api/types';
import type { Session } from '../state/useSession';
import { Avatar } from './Avatar';
import { useThemePref } from './hooks';
import { Icon } from './Icon';

const THEMES = [
  ['system', 'System'],
  ['light', 'Light'],
  ['dark', 'Dark'],
] as const;

/** The person's own menu at the bottom of the sidebar (D-023, points 11, 28, 30, 31). */
export function PersonMenu({ session, onSwitch }: { session: Session; onSwitch(person: PersonView): void }) {
  const [open, setOpen] = useState(false);
  const [people, setPeople] = useState(false);
  const [theme, setTheme] = useThemePref();
  const root = useRef<HTMLDivElement>(null);
  const me = session.me;

  useEffect(() => {
    if (!open) return;
    const outside = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    const esc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('pointerdown', outside);
    document.addEventListener('keydown', esc);
    return () => {
      document.removeEventListener('pointerdown', outside);
      document.removeEventListener('keydown', esc);
    };
  }, [open]);

  if (!me) return null;
  const humans = session.peopleList.filter((p) => !p.is_agent);

  return (
    <div className="me" ref={root}>
      <button
        className="me-btn"
        aria-expanded={open}
        aria-label="Your account menu"
        onClick={() => {
          setPeople(false);
          setOpen((o) => !o);
        }}
      >
        <Avatar id={me.id} person={me} />
        <span>
          <b>{me.name}</b>
          {me.line && <small>{me.line}</small>}
        </span>
        <Icon name="chev" />
      </button>
      {open && (
        <div className="pop me-pop" role="menu">
          {people ? (
            <>
              <button className="opt backlvl" onClick={() => setPeople(false)}>
                <Icon name="back" />
                <b>Switch account</b>
              </button>
              {humans.map((p) => (
                <button
                  key={p.id}
                  className={`opt who${p.id === me.id ? ' on' : ''}`}
                  aria-current={p.id === me.id || undefined}
                  onClick={() => {
                    setOpen(false);
                    if (p.id !== me.id) onSwitch(p);
                  }}
                >
                  <Avatar id={p.id} person={p} small />
                  <b>{p.name}</b>
                </button>
              ))}
              <div className="pop-f">
                There is no sign-in in the first version, so accounts are switched here and there is no Sign out.
              </div>
            </>
          ) : (
            <>
              <div className="me-org">
                <span className="org-mark" aria-hidden="true">
                  {Array.from(session.orgName)[0]}
                </span>
                <span>
                  <b>{session.orgName}</b>
                  <small>{session.peopleList.length} members</small>
                </span>
              </div>
              <button className="opt" disabled title="Settings are not built yet">
                <b>Settings</b>
                <span>Not built yet</span>
              </button>
              <div className="theme-row">
                <span>Theme</span>
                <span className="seg-ctl" role="group" aria-label="Theme">
                  {THEMES.map(([value, label]) => (
                    <button
                      key={value}
                      className={theme === value ? 'on' : ''}
                      aria-pressed={theme === value}
                      onClick={() => setTheme(value)}
                    >
                      {label}
                    </button>
                  ))}
                </span>
              </div>
              <button className="opt nextlvl" onClick={() => setPeople(true)}>
                <b>Switch account</b>
                <span>View the team as someone else</span>
                <Icon name="chev" />
              </button>
            </>
          )}
        </div>
      )}
    </div>
  );
}
