// A stand-in for the operator calls, at the HTTP level, used until the demo program exists.
// The browser tests answer the page's real requests with this same server.
import type { Answer, Mode, Organization, ParkedChange, ReceiverName, ReceiverStatus, Request, Script, ScriptStep } from './api';
import { LARKMERE_PEOPLE, LARKMERE_SITUATIONS } from './larkmere';

const at = (minute: number, second = 0) => new Date(Date.UTC(2026, 10, 17, 10, minute, second)).toISOString();
const step = (minute: number, who: string, what: string, situation: string | null = null): ScriptStep => ({ at: at(minute), who, what, situation });

function freshScript(): Script {
  return {
    running: true,
    pace: 1,
    happened: [
      step(41, 'anna', 'sent Maya three direct messages about the 3 pm call'),
      step(38, 'priya', 'replied in the launch checklist thread in #launch'),
      step(35, 'yuki', 'confirmed the booking slowdown in #incidents'),
      step(31, 'ship', 'posted the rc.8 build result in #releases'),
    ],
    next: [
      step(43, 'marco', 'mentions Maya in #product about the recurring editor', 'direct-mention'),
      step(44, 'tomas', 'turns a short question in #engineering into a long one', 'edited-to-another-length'),
      step(46, 'jonas', 'mentions Maya in #sales, then takes it back', 'mention-removed-by-edit'),
      step(48, 'ship', 'stops and waits for Maya to approve rc.8', 'flag-names-you-from-an-agent'),
      step(52, 'felix', 'starts a burst of updates in #incidents', 'incident-burst-at-three-dials'),
    ],
    situations: LARKMERE_SITUATIONS,
  };
}

const RECEIVERS: ReceiverName[] = ['search', 'sender', 'feed'];

export function createFixtureServer() {
  let generation = 7;
  let script = freshScript();
  const modes: Record<ReceiverName, Mode> = { search: 'running', sender: 'running', feed: 'running' };
  const waiting: Record<ReceiverName, number> = { search: 0, sender: 0, feed: 0 };
  let parked: ParkedChange[] = [{ receiver: 'search', number: 4830, reason: 'refused', tries: 3 }];
  let organizations: Organization[] = [
    { org: 'larkmere-7', name: 'Larkmere', state: 'current', created_at: at(2), approx_changes: 8124 },
    { org: 'larkmere-6', name: 'Larkmere', state: 'retired', created_at: '2026-11-17T09:12:00.000Z', approx_changes: 7956 },
    { org: 'larkmere-5', name: 'Larkmere', state: 'retired', created_at: '2026-11-16T16:40:00.000Z', approx_changes: 7911 },
  ];

  const currentOrg = () => organizations.find((o) => o.state === 'current')!;
  const ok = (body: unknown = {}): Answer => ({ status: 200, body });

  const statusOf = (name: ReceiverName): ReceiverStatus => ({
    name,
    place: 8124 - waiting[name],
    waiting: waiting[name],
    state: modes[name] === 'off' ? 'down' : 'running',
    parked: parked.filter((p) => p.receiver === name).length,
  });

  function command(body: { do: string; pace?: number; situation?: string }): Answer {
    if (body.do === 'start') script = { ...script, running: true };
    else if (body.do === 'pause') script = { ...script, running: false };
    else if (body.do === 'pace' && body.pace) script = { ...script, pace: body.pace };
    else if (body.do === 'play_situation') {
      const situation = script.situations.find((s) => s.id === body.situation);
      if (!situation) return { status: 404, body: { error: 'no such situation' } };
      const played: ScriptStep = { at: at(42, 10), who: 'demo', what: `played: ${situation.title}`, situation: situation.id };
      script = { ...script, happened: [played, ...script.happened], next: script.next.filter((n) => n.situation !== situation.id) };
    } else return { status: 400, body: { error: 'unknown command' } };
    return ok();
  }

  function reset(): Answer {
    generation += 1;
    organizations = organizations.map((o) => (o.state === 'current' ? { ...o, state: 'retired' as const } : o));
    organizations = [{ org: `larkmere-${generation}`, name: 'Larkmere', state: 'current', created_at: at(42, 30), approx_changes: 7980 },
      ...organizations.filter((o) => o.state !== 'preparing')];
    script = freshScript();
    return ok({ org: `larkmere-${generation}`, name: 'Larkmere' });
  }

  /** Answers one request the way the parts would. */
  function handle(req: Pick<Request, 'method' | 'path' | 'body'>): Answer {
    const [path, query = ''] = req.path.split('?');
    const key = `${req.method} ${path}`;
    if (key === 'GET demo/current') { const c = currentOrg(); return ok({ org: c.org, name: c.name }); }
    if (key === 'GET demo/organizations') return ok({ organizations });
    if (key === 'POST demo/reset') return reset();
    if (key === 'POST demo/organizations/delete-retired') {
      organizations = organizations.filter((o) => o.state !== 'retired');
      return ok();
    }
    let m = /^DELETE demo\/organizations\/([^/]+)$/.exec(key);
    if (m) {
      const target = decodeURIComponent(m[1]);
      if (target === currentOrg().org) return { status: 409, body: { refused: 'current' } };
      organizations = organizations.filter((o) => o.org !== target);
      return ok();
    }
    m = /^(GET|POST) demo\/orgs\/[^/]+\/script$/.exec(key);
    if (m) return m[1] === 'GET' ? ok(script) : command(req.body as { do: string });
    if (/^POST demo\/orgs\/[^/]+\/rebuild$/.test(key)) {
      const receiver = (req.body as { receiver: ReceiverName }).receiver;
      waiting[receiver] = 0;
      return ok();
    }
    if (/^GET conversations\/orgs\/[^/]+\/people$/.test(key)) return ok({ people: LARKMERE_PEOPLE });
    if (/^GET delivery\/orgs\/[^/]+\/status$/.test(key)) return ok({ receivers: RECEIVERS.map(statusOf) });
    if (/^GET delivery\/orgs\/[^/]+\/parked$/.test(key)) return ok({ parked });
    m = /^POST delivery\/orgs\/[^/]+\/parked\/(\d+)\/retry$/.exec(key);
    if (m) {
      const number = Number(m[1]);
      const receiver = new URLSearchParams(query).get('receiver');
      parked = parked.filter((p) => !(p.number === number && p.receiver === receiver));
      return ok();
    }
    m = /^(GET|POST) (search|sender|feed)\/orgs\/[^/]+\/control\/mode$/.exec(key);
    if (m) {
      const part = m[2] as ReceiverName;
      if (m[1] === 'GET') return ok({ mode: modes[part] });
      modes[part] = (req.body as { mode: Mode }).mode;
      waiting[part] = modes[part] === 'running' ? 0 : modes[part] === 'slow' ? 3 : 14;
      return ok();
    }
    return { status: 404, body: { error: `no fixture for ${key}` } };
  }

  return {
    handle,
    /** For tests: put the state into a given shape. */
    set(change: { waiting?: Partial<Record<ReceiverName, number>>; parked?: ParkedChange[]; organizations?: Organization[]; script?: Partial<Script> }) {
      Object.assign(waiting, change.waiting);
      if (change.parked) parked = change.parked;
      if (change.organizations) organizations = change.organizations;
      if (change.script) script = { ...script, ...change.script };
    },
  };
}
