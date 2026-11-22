import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import {
  explain, type Api, type Current, type Mode, type Organization, type ParkedChange, type Person,
  type ReceiverName, type ReceiverStatus, type Script, type ScriptCommand, type ScriptStep,
} from './api';

declare global {
  interface Window { Theme: { pref(): string; set(v: string): void } }
}

const POLL_MS = 1_500;
const PACES = [0.5, 1, 2, 4];
const SYSTEMS: { part: ReceiverName; label: string }[] = [
  { part: 'search', label: 'Search index' },
  { part: 'sender', label: 'Notification sender' },
  { part: 'feed', label: 'Activity feed' },
];
const MODES: { mode: Mode; label: string }[] = [
  { mode: 'running', label: 'Running' },
  { mode: 'slow', label: 'Slow' },
  { mode: 'off', label: 'Off' },
];

const clock = (iso: string) => new Date(iso).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', timeZone: 'UTC' });
const day = (iso: string) => new Date(iso).toLocaleString('en-GB', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit', timeZone: 'UTC' });

interface View {
  current: Current | null;
  script: Script | null;
  receivers: ReceiverStatus[] | null;
  parked: ParkedChange[] | null;
  organizations: Organization[] | null;
  modes: Partial<Record<ReceiverName, Mode>>;
  people: Person[];
  /** Which reads failed on the latest attempt; the last known values stay on screen. */
  down: Record<string, boolean>;
}

const EMPTY: View = { current: null, script: null, receivers: null, parked: null, organizations: null, modes: {}, people: [], down: {} };

function useView(api: Api) {
  const [view, setView] = useState<View>(EMPTY);
  const running = useRef(false);
  const again = useRef(false);
  const peopleOf = useRef<string | null>(null);

  const load = useCallback(async () => {
    if (running.current) { again.current = true; return; }
    running.current = true;
    try {
      do {
        again.current = false;
        const down: Record<string, boolean> = {};
        const read = async <T,>(key: string, fn: () => Promise<T>): Promise<T | undefined> => {
          try { return await fn(); } catch { down[key] = true; return undefined; }
        };
        const current = await read('demo', api.current);
        const organizations = await read('organizations', api.organizations);
        if (!current) {
          setView((v) => ({ ...v, organizations: organizations ?? v.organizations, down }));
          continue;
        }
        const id = current.org;
        const [script, receivers, parked, search, sender, feed, people] = await Promise.all([
          read('script', () => api.script(id)),
          read('delivery', () => api.status(id)),
          read('parked', () => api.parked(id)),
          read('mode-search', () => api.mode('search', id)),
          read('mode-sender', () => api.mode('sender', id)),
          read('mode-feed', () => api.mode('feed', id)),
          peopleOf.current === id ? Promise.resolve(undefined) : read('people', () => api.people(id)),
        ]);
        if (people) peopleOf.current = id;
        setView((v) => {
          const moved = v.current?.org !== id;
          return {
            current,
            organizations: organizations ?? v.organizations,
            script: script ?? (moved ? null : v.script),
            receivers: receivers ?? (moved ? null : v.receivers),
            parked: parked ?? (moved ? null : v.parked),
            modes: {
              search: search ?? (moved ? undefined : v.modes.search),
              sender: sender ?? (moved ? undefined : v.modes.sender),
              feed: feed ?? (moved ? undefined : v.modes.feed),
            },
            people: people ?? v.people,
            down,
          };
        });
      } while (again.current);
    } finally {
      running.current = false;
    }
  }, [api]);

  useEffect(() => {
    void load();
    const timer = setInterval(() => void load(), POLL_MS);
    return () => clearInterval(timer);
  }, [load]);

  return { view, load };
}

/** Runs one control's call, tracks that it is busy, and keeps its failure beside it. */
function useActions(after: () => Promise<void>) {
  const [busy, setBusy] = useState<Record<string, boolean>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const run = useCallback(async (key: string, label: string, fn: () => Promise<unknown>) => {
    setBusy((b) => ({ ...b, [key]: true }));
    setErrors(({ [key]: _gone, ...rest }) => rest);
    let succeeded = false;
    try {
      await fn();
      succeeded = true;
    } catch (error) {
      setErrors((e) => ({ ...e, [key]: explain(error, label) }));
    } finally {
      setBusy(({ [key]: _gone, ...rest }) => rest);
    }
    await after();
    return succeeded;
  }, [after]);
  return { busy, errors, run };
}

function Failure({ id, text }: { id: string; text?: string }) {
  if (!text) return null;
  return <p className="failure" role="alert" data-testid={`failure-${id}`}>{text}</p>;
}

function Section({ title, children, id, aside }: { title: string; id: string; children: ReactNode; aside?: ReactNode }) {
  return (
    <section aria-labelledby={`h-${id}`} data-testid={`section-${id}`}>
      <div className="section-head"><h2 id={`h-${id}`}>{title}</h2>{aside}</div>
      {children}
    </section>
  );
}

function ThemeChoice() {
  const [pref, setPref] = useState(() => window.Theme.pref());
  return (
    <label className="field">Theme
      <select value={pref} onChange={(e) => { window.Theme.set(e.target.value); setPref(e.target.value); }}>
        <option value="system">System</option>
        <option value="light">Light</option>
        <option value="dark">Dark</option>
      </select>
    </label>
  );
}

function StepLine({ step, names, children }: { step: ScriptStep; names: Map<string, string>; children?: ReactNode }) {
  const who = step.who === 'demo' ? 'On demand' : names.get(step.who) ?? step.who;
  return (
    <li>
      <time dateTime={step.at}>{clock(step.at)}</time>
      <span><b>{who}</b> {step.what}</span>
      {children}
    </li>
  );
}

function systemLine(status: ReceiverStatus | undefined, mode: Mode | undefined, deliveryDown: boolean, partDown: boolean) {
  if (!status) return { tone: 'off', text: deliveryDown ? 'Delivery is not answering, so how much is waiting is unknown.' : 'Loading…' };
  const waiting = `${status.waiting.toLocaleString('en-GB')} waiting`;
  const parked = status.parked > 0 ? ` · ${status.parked} parked` : '';
  if (status.state === 'down' || mode === 'off' || partDown) return { tone: 'off', text: `Down · ${waiting}${parked}` };
  if (status.state === 'paused') return { tone: 'slow', text: `Paused · ${waiting}${parked}` };
  if (status.state === 'stopped') return { tone: 'off', text: `Stopped · ${waiting}${parked}` };
  if (mode === 'slow') return { tone: 'slow', text: `Slow · ${waiting}${parked}` };
  if (status.waiting === 0 && status.parked === 0) return { tone: 'ok', text: 'Up to date · 0 waiting' };
  return { tone: status.parked > 0 ? 'slow' : 'ok', text: `Catching up · ${waiting}${parked}` };
}

function ResetProgress({ api, previous }: { api: Api; previous: string }) {
  const [elapsed, setElapsed] = useState(0);
  const [fresh, setFresh] = useState<Organization | null>(null);
  const [behind, setBehind] = useState<ReceiverStatus[] | null>(null);

  useEffect(() => {
    let live = true;
    const started = Date.now();
    const tick = async () => {
      setElapsed(Math.floor((Date.now() - started) / 1000));
      try {
        const preparing = (await api.organizations()).find((o) => o.state === 'preparing' && o.org !== previous) ?? null;
        if (!live) return;
        setFresh(preparing);
        if (preparing) {
          const receivers = await api.status(preparing.org);
          if (live) setBehind(receivers.filter((r) => r.name !== 'sender'));
        }
      } catch { /* the progress is a courtesy; the Reset call itself reports failure */ }
    };
    void tick();
    const timer = setInterval(() => void tick(), 500);
    return () => { live = false; clearInterval(timer); };
  }, [api, previous]);

  const waitingOn = behind?.reduce((n, r) => n + r.waiting, 0);
  const stage = behind ? 2 : 1;
  const steps = [
    fresh ? `Seeding ${fresh.org}: ${fresh.approx_changes.toLocaleString('en-GB')} changes so far` : 'Seeding a fresh organization',
    behind ? `Waiting for search and the feed: ${waitingOn!.toLocaleString('en-GB')} changes to go` : 'Waiting for search and the feed',
    'Making it current and retiring the previous one',
  ];
  return (
    <div className="progress" role="status" data-testid="reset-progress">
      <b>Preparing a fresh organization · {elapsed} s</b>
      <ol>
        {steps.map((text, i) => (
          <li key={text} className={i + 1 < stage ? 'done' : i + 1 === stage ? 'active' : ''}>{text}</li>
        ))}
      </ol>
      <span className="muted">{previous} keeps running until the new one is ready.</span>
    </div>
  );
}

/** A delete that failed partway can be repeated (the contract); a refusal cannot. */
const deleteFailure = (text?: string) => (text && !text.startsWith('Refused') ? `${text} Deleting again is safe.` : text);

function Organizations({ view, actions, api }: { view: View; actions: ReturnType<typeof useActions>; api: Api }) {
  const [asking, setAsking] = useState<string | null>(null);
  const list = view.organizations;
  const currentId = view.current?.org;
  const isCurrent = (o: Organization) => o.org === currentId || o.state === 'current';
  const retired = (list ?? []).filter((o) => !isCurrent(o) && o.state === 'retired');

  const remove = async (o: Organization) => {
    setAsking(null);
    await actions.run(`delete-${o.org}`, `Delete ${o.org}`, () => api.deleteOrganization(o.org));
  };
  const removeRetired = async () => {
    setAsking(null);
    await actions.run('delete-retired', 'Delete all retired', api.deleteRetired);
  };

  return (
    <Section title="Organizations" id="organizations" aside={
      <button className="btn" disabled={retired.length === 0 || !!actions.busy['delete-retired']}
        onClick={() => setAsking('all')}>Delete all retired</button>
    }>
      {asking === 'all' && (
        <div className="confirm" role="alertdialog" aria-label="Delete all retired organizations">
          <span>Delete all {retired.length} retired organizations and everything in them? This cannot be undone.</span>
          <span className="confirm-actions">
            <button className="btn" onClick={() => setAsking(null)}>Cancel</button>
            <button className="btn danger" onClick={() => void removeRetired()}>Delete {retired.length}</button>
          </span>
        </div>
      )}
      <Failure id="delete-retired" text={deleteFailure(actions.errors['delete-retired'])} />
      {view.down.organizations && <Failure id="organizations" text="The list of organizations is not answering. Showing the last one seen." />}
      {list && (
        <ul className="orgs">
          {list.map((o) => {
            const current = isCurrent(o);
            return (
              <li key={o.org} data-testid={`org-${o.org}`}>
                <div className="org-line">
                  <span className="org-name"><b>{o.name}</b> <code>{o.org}</code></span>
                  <span className={`badge ${current ? 'current' : o.state}`}>{current ? 'Current' : o.state === 'preparing' ? 'Preparing' : 'Retired'}</span>
                  <span className="muted">Created {day(o.created_at)}</span>
                  <span className="muted">about {o.approx_changes.toLocaleString('en-GB')} changes</span>
                  <span className="org-action">
                    {current
                      ? <span className="muted">Cannot be deleted</span>
                      : o.state === 'retired'
                        ? <button className="btn" disabled={!!actions.busy[`delete-${o.org}`]} aria-label={`Delete ${o.org}`}
                            onClick={() => setAsking(o.org)}>{actions.busy[`delete-${o.org}`] ? 'Deleting…' : 'Delete'}</button>
                        : null}
                  </span>
                </div>
                {asking === o.org && (
                  <div className="confirm" role="alertdialog" aria-label={`Delete ${o.org}`}>
                    <span>Delete {o.org} and everything in it from every part? This cannot be undone.</span>
                    <span className="confirm-actions">
                      <button className="btn" onClick={() => setAsking(null)}>Cancel</button>
                      <button className="btn danger" onClick={() => void remove(o)}>Delete {o.org}</button>
                    </span>
                  </div>
                )}
                <Failure id={`delete-${o.org}`} text={deleteFailure(actions.errors[`delete-${o.org}`])} />
              </li>
            );
          })}
        </ul>
      )}
    </Section>
  );
}

export function App({ api }: { api: Api }) {
  const { view, load } = useView(api);
  const actions = useActions(load);
  const { busy, errors, run } = actions;
  const [resetting, setResetting] = useState<string | null>(null);
  const org = view.current?.org;
  const script = view.script;
  const names = new Map(view.people.map((p) => [p.id, p.name]));
  const paces = script && !PACES.includes(script.pace) ? [...PACES, script.pace].sort((a, b) => a - b) : PACES;
  const locked = !org || !!resetting;

  const command = (key: string, label: string, c: ScriptCommand) => {
    if (org) void run(key, label, () => api.command(org, c));
  };
  const reset = async () => {
    if (!org) return;
    setResetting(org);
    await run('reset', 'Reset', api.reset);
    setResetting(null);
  };

  return (
    <>
      <header>
        <div className="title">
          <h1>Control panel</h1>
          <span className="tag">OUTSIDE THE PRODUCT</span>
          {view.current && <span className="org-now">{view.current.name} <code>{view.current.org}</code></span>}
        </div>
        <div className="controls">
          <span className={`state ${script?.running ? 'run' : 'pause'}`} data-testid="script-state">
            {script ? (script.running ? 'Running' : 'Paused') : '…'}
          </span>
          <button className="btn pri" disabled={locked || !script || !!busy.toggle}
            onClick={() => command('toggle', script?.running ? 'Pause' : 'Start', { do: script?.running ? 'pause' : 'start' })}>
            {script?.running ? 'Pause' : 'Start'}
          </button>
          <label className="field">Pace
            <select value={script?.pace ?? 1} disabled={locked || !script}
              onChange={(e) => command('pace', 'Change the pace', { do: 'pace', pace: Number(e.target.value) })}>
              {paces.map((p) => <option key={p} value={p}>{p}×</option>)}
            </select>
          </label>
          <button className="btn" disabled={locked} onClick={() => void reset()}>{resetting ? 'Resetting…' : 'Reset'}</button>
          <ThemeChoice />
        </div>
      </header>

      <div className="notices">
        {view.down.demo && <Failure id="demo" text="The demo program is not answering. Showing the last state seen." />}
        <Failure id="toggle" text={errors.toggle} />
        <Failure id="pace" text={errors.pace} />
        <Failure id="reset" text={errors.reset} />
        {resetting && <ResetProgress api={api} previous={resetting} />}
      </div>

      <main>
        <div className="column">
          <Section title="What comes next" id="next">
            {view.down.script && <Failure id="script" text="The script is not answering. Showing the last state seen." />}
            <ul className="steps">
              {script?.next.map((s, i) => (
                <StepLine key={`${s.at}-${i}`} step={s} names={names}>
                  {s.situation && (
                    <button className="btn small" disabled={locked || !!busy[`next-${s.situation}`]}
                      aria-label={`Play now: ${names.get(s.who) ?? s.who} ${s.what}`}
                      onClick={() => command(`next-${s.situation}`, 'Play now', { do: 'play_situation', situation: s.situation! })}>
                      Play now
                    </button>
                  )}
                  {s.situation && <Failure id={`next-${s.situation}`} text={errors[`next-${s.situation}`]} />}
                </StepLine>
              ))}
              {script && script.next.length === 0 && <li className="muted">The schedule is finished. Reset to run it again.</li>}
            </ul>
          </Section>
          <Section title="What happened" id="happened">
            <ul className="steps">
              {script?.happened.map((s, i) => <StepLine key={`${s.at}-${i}`} step={s} names={names} />)}
              {script && script.happened.length === 0 && <li className="muted">Nothing has happened yet.</li>}
            </ul>
          </Section>
        </div>

        <Section title="Situations on demand" id="situations">
          <ul className="situations">
            {script?.situations.map((s) => (
              <li key={s.id}>
                <button className="situation" disabled={locked || !!busy[`situation-${s.id}`]} title={s.outcome}
                  onClick={() => command(`situation-${s.id}`, 'Play the situation', { do: 'play_situation', situation: s.id })}>
                  <b>{s.title}</b>
                  <span>{s.outcome}</span>
                </button>
                <Failure id={`situation-${s.id}`} text={errors[`situation-${s.id}`]} />
              </li>
            ))}
          </ul>
        </Section>

        <div className="column">
          <Section title="The three systems" id="systems">
            {view.down.delivery && view.receivers && <Failure id="delivery" text="Delivery is not answering. The waiting counts are the last ones seen." />}
            {SYSTEMS.map(({ part, label }) => {
              const status = view.receivers?.find((r) => r.name === part);
              const mode = view.modes[part];
              const line = systemLine(status, mode, !!view.down.delivery, !!view.down[`mode-${part}`]);
              return (
                <div className="system" key={part} data-testid={`system-${part}`}>
                  <div className="system-head">
                    <b>{label}</b>
                    <button className="btn small" disabled={locked || !!busy[`rebuild-${part}`]} aria-label={`Rebuild ${label}`}
                      onClick={() => org && void run(`rebuild-${part}`, `Rebuild ${label}`, () => api.rebuild(org, part))}>
                      {busy[`rebuild-${part}`] ? 'Rebuilding…' : 'Rebuild'}
                    </button>
                  </div>
                  <div className="seg" role="group" aria-label={label}>
                    {MODES.map((m) => (
                      <button key={m.mode} aria-pressed={mode === m.mode} disabled={locked || !!busy[`mode-${part}`]}
                        onClick={() => org && void run(`mode-${part}`, `Set ${label} to ${m.label}`, () => api.setMode(part, org, m.mode))}>
                        {m.label}
                      </button>
                    ))}
                  </div>
                  <div className={`readout ${line.tone}`} data-testid={`readout-${part}`}>{line.text}</div>
                  {view.down[`mode-${part}`] && <Failure id={`read-${part}`} text={`${label} is not answering, so its switch may be out of date.`} />}
                  <Failure id={`mode-${part}`} text={errors[`mode-${part}`]} />
                  <Failure id={`rebuild-${part}`} text={errors[`rebuild-${part}`]} />
                </div>
              );
            })}
            <p className="note">Nothing is lost while a system is slow or off: its changes wait in order and are delivered when it returns.</p>
          </Section>
          <Section title="Parked changes" id="parked">
            {view.down.parked && <Failure id="parked" text="The list of parked changes is not answering. Showing the last one seen." />}
            {view.parked?.length === 0 && <p className="muted">Nothing is parked.</p>}
            <ul className="steps">
              {view.parked?.map((p) => {
                const key = `retry-${p.receiver}-${p.number}`;
                return (
                  <li key={key} data-testid={`parked-${p.receiver}-${p.number}`}>
                    <span><b>{SYSTEMS.find((s) => s.part === p.receiver)?.label ?? p.receiver}</b> change {p.number}: {p.reason} after {p.tries} tries
                      <Failure id={key} text={errors[key]} />
                    </span>
                    <button className="btn small" disabled={locked || !!busy[key]} aria-label={`Try again: ${p.receiver} change ${p.number}`}
                      onClick={() => org && void run(key, 'Try again', () => api.retry(org, p))}>Try again</button>
                  </li>
                );
              })}
            </ul>
          </Section>
        </div>

        <div className="wide"><Organizations view={view} actions={actions} api={api} /></div>
      </main>
    </>
  );
}
