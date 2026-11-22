import { useCallback, useEffect, useRef, useState } from 'react';
import { explain, type Api, type Current, type Notification, type Person } from './api';

declare global {
  interface Window { Theme: { pref(): string; set(v: string): void } }
}

const POLL_MS = 2_000;
const DEFAULT_PERSON = 'maya';

const when = (iso: string) => new Date(iso).toLocaleString('en-GB', {
  weekday: 'short', hour: '2-digit', minute: '2-digit', timeZone: 'UTC',
}).replace(',', '');

function personFromAddress() {
  return new URLSearchParams(location.search).get('person');
}

function Entry({ n }: { n: Notification }) {
  const deleted = n.now === 'deleted';
  const read = n.now === 'read';
  const who = n.why.split(' ')[0] ?? '';
  return (
    <li className={`n${read ? ' is-read' : ''}${deleted ? ' is-deleted' : ''}`} data-testid="notification">
      <span className="avatar" aria-hidden="true">{who.charAt(0)}</span>
      <div className="body">
        <div className="n-head">
          <span className="app">Contexia</span>
          <time dateTime={n.at ?? undefined}>{n.at ? when(n.at) : 'Not sent'}</time>
        </div>
        <b className="why">{n.why}</b>
        {deleted
          ? <p className="text gone">Message deleted</p>
          : <p className="text">{n.text}</p>}
        {(n.delayed || read || n.edited || n.state === 'history') && (
          <div className="marks">
            {n.delayed && <span className="mark delayed">Delayed</span>}
            {n.edited && !deleted && <span className="mark">Edited</span>}
            {read && <span className="mark">Read</span>}
            {n.state === 'history' && <span className="mark">Listed only, not delivered</span>}
          </div>
        )}
      </div>
    </li>
  );
}

function ThemeChoice() {
  const [pref, setPref] = useState(() => window.Theme.pref());
  return (
    <label>Theme{' '}
      <select value={pref} onChange={(e) => { window.Theme.set(e.target.value); setPref(e.target.value); }}>
        <option value="system">System</option>
        <option value="light">Light</option>
        <option value="dark">Dark</option>
      </select>
    </label>
  );
}

export function App({ api }: { api: Api }) {
  const [current, setCurrent] = useState<Current | null>(null);
  const [people, setPeople] = useState<Person[]>([]);
  const [person, setPerson] = useState<string | null>(personFromAddress);
  const [list, setList] = useState<Notification[] | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const shownFor = useRef<string>('');
  const latest = useRef(0);
  const asking = useRef<string | null>(null);

  const load = useCallback(async () => {
    // A slow sender must surface as a problem, so a poll never overtakes the one still waiting for the same person.
    if (asking.current === (person ?? '')) return;
    asking.current = person ?? '';
    const ask = ++latest.current;
    try {
      let org: Current;
      try {
        org = await api.current();
      } catch (error) {
        if (ask === latest.current) setProblem(explain(error, 'The demo program'));
        return;
      }
      if (ask !== latest.current) return;
      setCurrent((c) => (c?.org === org.org ? c : org));
      let everyone: Person[];
      try {
        everyone = await api.people(org.org);
      } catch (error) {
        if (ask === latest.current) setProblem(explain(error, 'The list of people'));
        return;
      }
      if (ask !== latest.current) return;
      setPeople(everyone);
      const wanted = person && everyone.some((p) => p.id === person)
        ? person
        : everyone.find((p) => p.id === DEFAULT_PERSON)?.id ?? everyone[0]?.id ?? null;
      if (wanted !== person) { setPerson(wanted); return; }
      if (!wanted) { setProblem('This organization has no people yet.'); return; }
      const key = `${org.org}/${wanted}`;
      if (shownFor.current !== key) setList(null);
      try {
        const entries = await api.notifications(org.org, wanted);
        if (ask !== latest.current) return;
        shownFor.current = key;
        setList(entries);
        setProblem(null);
      } catch (error) {
        if (ask === latest.current) setProblem(explain(error, 'The notification sender'));
      }
    } finally {
      if (ask === latest.current) asking.current = null;
    }
  }, [api, person]);

  useEffect(() => {
    void load();
    const timer = setInterval(() => void load(), POLL_MS);
    return () => clearInterval(timer);
  }, [load]);

  const choose = (id: string) => {
    const url = new URL(location.href);
    url.searchParams.set('person', id);
    history.replaceState(null, '', url);
    setList(null);
    setProblem(null);
    setPerson(id);
  };

  const chosen = people.find((p) => p.id === person);
  const unread = list?.filter((n) => n.state === 'sent' && n.now === null).length ?? 0;

  return (
    <div className="wrap">
      <div className="top">
        <h1>Notification app</h1>
        <span className="tag">OUTSIDE THE PRODUCT</span>
        <label>Delivered to{' '}
          <select value={person ?? ''} onChange={(e) => choose(e.target.value)} disabled={people.length === 0}>
            {people.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
        </label>
        <ThemeChoice />
      </div>

      <div className="phone">
        <div className="bar">
          <span>{current?.name ?? 'Contexia'}</span>
          <span data-testid="count">{list && list.length > 0 ? `${unread} unread` : ''}</span>
        </div>
        {problem && (
          <p className="problem" role="alert" data-testid="problem">
            {problem}{list ? ' Showing the last list seen.' : ''}
          </p>
        )}
        {list === null && !problem && <p className="quiet" data-testid="loading">Loading…</p>}
        {list?.length === 0 && (
          <div className="empty" data-testid="empty">
            <b>Nothing has been sent yet</b>
            <p>{chosen ? `When something needs ${chosen.name.split(' ')[0]}, it appears here.` : 'When something is sent, it appears here.'} The team's earlier history sends nothing.</p>
          </div>
        )}
        {list && list.length > 0 && (
          <ul className="list">{list.map((n) => <Entry key={n.name} n={n} />)}</ul>
        )}
      </div>
    </div>
  );
}
