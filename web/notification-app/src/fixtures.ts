// A stand-in for the three calls this page makes, at the HTTP level, used until the demo program exists.
// The browser tests answer the page's real requests with this same server.
import type { Answer, SenderEntry } from './api';
import { LARKMERE_PEOPLE } from './larkmere';

const ORG = 'larkmere-7';
const at = (hour: number, minute: number, second = 0) => new Date(Date.UTC(2026, 10, 17, hour, minute, second)).toISOString();
const entry = (row: string, number: number, sent: string | null, why: string, text: string, more: Partial<SenderEntry> = {}): SenderEntry => ({
  name: `${ORG} / maya / ${row} / ${number}`, person: 'maya', state: 'sent', at: sent, delayed: false, why, text, now: null, ...more,
});

// Maya's list after the demonstration has run for a while, with the texts of the seeded situations.
// Everyone else has been sent nothing, which is the true starting state (D-032).
export const MAYA: SenderEntry[] = [
  entry('#product:msg_d1', 8131, at(10, 44, 12), 'Marco mentioned you in #product',
    '@Maya the recurring editor is in rc.7. Can you try moving a series across the clock change before we call it done?'),
  entry('#engineering:msg_e4', 8127, at(10, 43, 20), 'Tomás mentioned you in #engineering',
    "@Maya the export API can't ship both CSV and XLSX by Thursday. Which one matters for launch? Update: I measured it. CSV is done today. XLSX needs the streaming writer, which is two more days plus a QA pass, so it would land the Monday after launch.",
    { edited: true }),
  entry('#marketing:msg_v2', 8119, at(10, 42, 41), 'Priya mentioned you in #marketing',
    '@Maya @Lena the launch video goes out with the old logo unless someone stops it today. Can one of you look at the last frame?',
    { now: 'deleted' }),
  entry('#releases:msg_r8', 8102, at(10, 41, 5), 'Deployer is waiting for your approval in #releases',
    "@Maya 2.0.0-rc.8 is ready for production, but the release notes still say 'practitioner' in four places and the content check is set to block. Approve as it is, or wait for a fix?",
    { delayed: true }),
  entry('#launch:msg_a1', 8096, at(10, 39, 30), 'Anna flagged a decision for you in #launch',
    '@Maya the press embargo lifts at 09:00 on Thursday. Do we hold the pricing page until then, or publish it tonight with the rest of the site? I need your call before Priya schedules it.',
    { now: 'read' }),
];

export function createFixtureServer(lists: Record<string, SenderEntry[]> = { maya: MAYA }) {
  function handle(req: { path: string }): Answer {
    const [path, query = ''] = req.path.split('?');
    if (path === 'demo/current') return { status: 200, body: { org: ORG, name: 'Larkmere' } };
    if (/^conversations\/orgs\/[^/]+\/people$/.test(path)) return { status: 200, body: { people: LARKMERE_PEOPLE } };
    if (/^sender\/orgs\/[^/]+\/notifications$/.test(path)) {
      const person = new URLSearchParams(query).get('person') ?? '';
      return { status: 200, body: { notifications: lists[person] ?? [] } };
    }
    return { status: 404, body: { error: `no fixture for ${path}` } };
  }
  return { handle };
}
