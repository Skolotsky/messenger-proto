/* E-002 shared content and behaviour. The three directions differ only in their shell and styles.
   A mock-up: no server, no stack choice. Everything resets on reload. */
(function () {
'use strict';
const ME = 'maya';
const P = {
  maya: ['Maya Okafor', 'Product'], anna: ['Anna Lindqvist', 'Founder'], marco: ['Marco Bianchi', 'Engineering lead'],
  lena: ['Lena Fischer', 'Design'], tomas: ['Tomás Rivera', 'Backend'], priya: ['Priya Nair', 'Marketing'],
  sam: ['Sam Whitaker', 'Support lead'], jonas: ['Jonas Weber', 'Sales'], dele: ['Dele Adeyemi', 'Data'],
  ines: ['Inès Laurent', 'Frontend'], kofi: ['Kofi Mensah', 'Mobile'], yuki: ['Yuki Tanaka', 'QA'],
  omar: ['Omar Haddad', 'Finance and ops'], ruth: ['Ruth Mendel', 'Customer success'],
  ship: ['Shipwright', 'Release agent', 1], triage: ['Triage', 'Support agent', 1],
  field: ['Fieldnotes', 'Research agent', 1], ledger: ['Ledger', 'Metrics agent', 1]
};
const PID = Object.keys(P);
const pn = id => P[id][0], first = id => P[id][0].split(' ')[0], isAgent = id => !!P[id][2];
const esc = s => String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
const rich = s => esc(s).replace(/@([\wÀ-ÿ]+)/g, (x, n) => `<span class="at${n === 'Maya' ? ' me' : ''}">@${n}</span>`);
const IC = {
  bell: '<path d="M6 16v-5a6 6 0 1 1 12 0v5l1.5 2h-15z"/><path d="M10 20a2 2 0 0 0 4 0"/>',
  back: '<path d="M15 5l-7 7 7 7"/>', chev: '<path d="M6 9l6 6 6-6"/>',
  search: '<circle cx="11" cy="11" r="6"/><path d="M16 16l4 4"/>',
  feed: '<path d="M4 6h16M4 12h16M4 18h10"/>', clock: '<circle cx="12" cy="12" r="8"/><path d="M12 8v4l3 2"/>',
  chat: '<path d="M4 5h16v11H9l-5 4z"/>', home: '<path d="M4 11l8-7 8 7v9h-5v-6H9v6H4z"/>',
  check: '<path d="M5 12.5l4.5 4.5L19 7.5"/>',
  smile: '<circle cx="11" cy="13" r="7"/><path d="M8.5 14.5a3 3 0 0 0 5 0M9 11h.01M13 11h.01M19 3v4M17 5h4"/>',
  open: '<path d="M14 5h5v5"/><path d="M19 5l-8 8"/><path d="M10 6H7a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h9a2 2 0 0 0 2-2v-3"/>',
  rules: '<path d="M4 7h10M18 7h2M4 17h2M10 17h10"/><circle cx="16" cy="7" r="2"/><circle cx="8" cy="17" r="2"/>'
};
const ic = n => `<svg class="ic" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${IC[n]}</svg>`;
function av(id, cls) {
  if (id === '#') return `<span class="av ch ${cls || ''}" aria-hidden="true">#</span>`;
  const n = P[id][0].split(' ');
  const ini = isAgent(id) ? n[0].slice(0, 2) : n[0][0] + (n[1] ? n[1][0] : '');
  return `<span class="av ${isAgent(id) ? 'ag ' : ''}${cls || ''}" style="--h:${(PID.indexOf(id) * 47) % 360}" aria-hidden="true">${ini}</span>`;
}

/* ---------- the seeded moment: Larkspur, two days before the 2.0 launch, Tuesday 10:42, seen as Maya ---------- */
const CH = [ // name, dial, unread, people
  ['launch', 'all', 3, 31], ['incidents', 'focus', 6, 24], ['product', 'default', 3, 18], ['engineering', 'default', 2, 14],
  ['design', 'default', 2, 9], ['support', 'default', 1, 12], ['sales', 'default', 1, 8], ['general', 'default', 1, 36],
  ['releases', 'default', 1, 11], ['research', 'default', 1, 7], ['data', 'default', 1, 6], ['marketing', 'default', 1, 7],
  ['pricing', 'default', 0, 6], ['mobile', 'default', 0, 5], ['frontend', 'default', 0, 6], ['backend', 'default', 0, 7],
  ['qa', 'default', 0, 5], ['customers', 'default', 0, 10], ['ops', 'default', 0, 4], ['random', 'muted', 0, 33]
];
const DMS = ['anna', 'sam', 'marco', 'lena', 'tomas', 'ship', 'triage'];
const BG = { // ordinary traffic in conversations that carry no situation
  'ch-pricing': [['omar', 'Yesterday 14:10', 'Team plan stays at €79 per location through launch. We revisit in January.'], ['jonas', 'Yesterday 14:32', 'Good. Two pilots already have that number in their contracts.']],
  'ch-mobile': [['kofi', '09:05', 'iOS build 412 is on TestFlight. Android 388 follows after lunch.'], ['yuki', '09:40', 'Smoke run on 412 is clean.']],
  'ch-frontend': [['ines', '09:15', 'Storybook is updated for the new date picker.'], ['lena', '09:22', 'Looks right. The focus ring is finally visible on dark.']],
  'ch-backend': [['tomas', 'Yesterday 18:20', 'Migration 0412 adds the location index. Dry run took 41 seconds on a copy of prod.'], ['marco', 'Yesterday 18:31', 'Run it before the freeze tonight.']],
  'ch-qa': [['yuki', '08:50', 'Regression pass for rc3 starts as soon as Shipwright posts the build.'], ['kofi', '08:58', 'Ping me for anything on Android 12.']],
  'ch-customers': [['ruth', 'Yesterday 11:02', 'Brightwell renewed for 12 months, 9 locations.'], ['jonas', 'Yesterday 11:05', 'That is our biggest account now.']],
  'ch-ops': [['omar', 'Mon 16:45', 'Launch day lunch is ordered for 34 people.'], ['anna', 'Mon 16:50', 'Thank you. Make it 36, two contractors are in.']],
  'ch-random': [['kofi', '08:12', 'The coffee machine on the 2nd floor is fixed.'], ['priya', '08:15', 'Best news of launch week.']],
  'ch-releases': [['marco', 'Yesterday 17:10', 'Cutting 2.0.0-rc3 tonight. Thread below.'], ['yuki', 'Yesterday 17:14', 'QA is ready for it in the morning.']],
  'ch-research': [['dele', 'Mon 10:02', 'Interview notes from the six pilot clinics are in the shared folder.'], ['lena', 'Mon 10:20', 'Reading them before the empty-state work.']],
  'ch-design': [['lena', 'Yesterday 15:20', 'Empty states v3 is in the file. Thread below.'], ['ines', 'Yesterday 15:44', 'On it for web this afternoon.']],
  'ch-product': [['maya', 'Mon 11:00', 'Launch checklist for Thursday is in the doc. Thread below.'], ['dele', 'Mon 11:30', 'Dashboards will be ready for the Wednesday review.']],
  'ch-data': [['dele', 'Mon 09:12', 'Weekly refresh is running. Thread below.'], ['omar', 'Mon 09:40', 'I need the revenue one by Wednesday noon.']],
  'dm-marco': [['marco', 'Yesterday 17:40', 'Freeze is at 20:00 tonight, not 18:00.'], ['maya', 'Yesterday 17:42', 'Noted. I will tell marketing.']],
  'dm-lena': [['lena', 'Mon 15:02', 'Sent you the v3 file for the empty states.'], ['maya', 'Mon 15:30', 'Much better. One comment on the mascot.']],
  'dm-tomas': [['tomas', 'Mon 12:15', 'Do you have five minutes for the export question after standup?'], ['maya', 'Mon 12:16', 'Yes.']],
  'dm-ship': [['ship', 'Fri 16:00', 'Release notes draft for 1.9.4 is ready for your review.'], ['maya', 'Fri 16:20', 'Approved.']],
  'dm-triage': [['triage', 'Fri 09:00', 'Weekly summary: 212 tickets, 71 percent answered by macro.'], ['maya', 'Fri 09:05', 'Thanks.']]
};
const LONG_EDIT = '@Maya the export API can\'t ship both CSV and XLSX by Thursday. Which one matters for launch? Update: I measured it. CSV is done today. XLSX needs the streaming writer, which is two more days plus a QA pass, so it would land the Monday after launch. Brightwell and two other pilot clinics asked for XLSX by name; everyone else is fine with CSV. My suggestion: ship CSV on Thursday and announce XLSX for the week after.';

function seed() {
  const msgs = {}, convs = {};
  const M = (id, by, t, text, o) => { msgs[id] = Object.assign({ id, by, t, text, rx: [] }, o || {}); return id; };
  const C = (id, kind, title, o, list) => { convs[id] = Object.assign({ id, kind, title, msgs: list }, o || {}); list.forEach(function tag(m) { msgs[m].conv = id; (msgs[m].replies || []).forEach(tag); }); };
  C('dm-anna', 'dm', '', { with: 'anna' }, [
    M('a-1', 'anna', 'Yesterday 18:05', 'The board deck is due Friday. Can you own the roadmap slide?'),
    M('a0', 'maya', 'Yesterday 18:09', 'Yes. Draft by Wednesday night.'),
    M('a1', 'anna', '10:36', 'Can you join the pricing call with Brightwell at 3? They want to hear the roadmap from you, not from me.', { isNew: 1 }),
    M('a2', 'anna', '10:37', 'And did legal clear the new terms page? I don\'t want to launch without it.', { isNew: 1 }),
    M('a3', 'anna', '10:37', 'Brightwell will ask about SSO. Jonas wants to promise the first quarter.', { isNew: 1 }),
    M('a4', 'anna', '10:38', 'My view: we say "first half of next year" and nothing firmer.', { isNew: 1 }),
    M('a5', 'anna', '10:38', 'They also want the XLSX export. Tomás says it slips past Thursday?', { isNew: 1 }),
    M('a6', 'anna', '10:39', 'If it slips, I would rather tell them on the call than have them find out.', { isNew: 1 }),
    M('a7', 'anna', '10:39', 'The deck for the call is in the shared folder. Slide 4 is yours.', { isNew: 1 }),
    M('a8', 'anna', '10:40', 'Slide 6 has last quarter\'s numbers. Dele is refreshing them now.', { isNew: 1 }),
    M('a9', 'anna', '10:41', 'One more thing: the board wants a launch-day summary by Friday noon.', { isNew: 1 }),
    M('a10', 'anna', '10:41', 'Call me if 3 pm does not work. Otherwise see you there.', { isNew: 1 })]);
  C('dm-sam', 'dm', '', { with: 'sam' }, [
    M('s0', 'sam', 'Yesterday 12:20', 'Macros for the new booking flow are written. Want to skim them?'),
    M('s00', 'maya', 'Yesterday 13:02', 'Skimmed. The refund one is very clear.')]);
  C('th-rc', 'thread', '2.0 release candidate', { where: 'releases', tdial: 'fyi', tauto: 'you were mentioned in it' }, [
    M('rc1', 'marco', 'Yesterday 17:10', 'Cutting 2.0.0-rc3 tonight. @Shipwright please build it and run the full suite. @Maya signs off the beta rollout.'),
    M('rc2', 'ship', '08:02', 'Build started. 340 tests queued.', { info: 1 }),
    M('rc3', 'ship', '09:14', 'Progress: 212 of 340 tests passed, none failed.', { info: 1 }),
    M('rc4', 'ship', '10:21', '2.0.0-rc3 is built and all 340 tests pass. I have stopped before the rollout: I need your approval to release it to the 5% beta ring, @Maya.', { flag: 1, isNew: 1, status: 'Waiting on Maya' })]);
  C('ch-incidents', 'channel', '', { name: 'incidents' }, [
    M('i0', 'marco', 'Yesterday 09:30', 'Resolved: the calendar sync delay was a stuck worker. Post-mortem on Friday.'),
    M('i1', 'yuki', '10:02', 'Checkout is slow on staging and prod. p95 went from 400 ms to 2.9 s around 09:55.', { isNew: 1 }),
    M('i2', 'marco', '10:05', 'Looking. It lines up with the 09:50 config push.', { isNew: 1 }),
    M('i3', 'triage', '10:09', '9 customers reported failed bookings in the last 15 minutes. Their tickets are tagged checkout-slow.', { isNew: 1 }),
    M('i4', 'marco', '10:14', 'Found it: the push set the connection pool to 5. Reverting.', { isNew: 1 }),
    M('i5', 'tomas', '10:19', 'Revert is out. p95 is back to 430 ms.', { isNew: 1 }),
    M('i6', 'yuki', '10:24', 'Confirmed on prod. Leaving this open until the 11:00 check.', { isNew: 1, rx: [['🙏', ['marco', 'sam']]] })]);
  C('ch-launch', 'channel', '', { name: 'launch' }, [
    M('l1', 'priya', 'Yesterday 16:40', 'The press embargo lifts Thursday 09:00 CET. Three outlets are confirmed.'),
    M('l2', 'priya', 'Yesterday 17:02', '', { deleted: 1, replies: [
      M('l2a', 'marco', 'Yesterday 17:20', 'Thursday still works for engineering if rc3 is clean.'),
      M('l2b', 'lena', 'Yesterday 17:26', 'Design is done except the empty states.')] }),
    M('l3', 'anna', '09:12', '@Maya I need your go/no-go on the Thursday date by end of day. If we slip, I have to tell the press today.', { flag: 1, rx: [['👀', ['marco']]] }),
    M('l4', 'priya', '09:55', 'Launch post draft is in review. Screenshots are the last missing piece.')]);
  C('th-empty', 'thread', 'Onboarding empty states, v3', { where: 'design', tdial: 'fyi', tauto: 'you clicked Follow' }, [
    M('e1', 'lena', 'Yesterday 15:20', 'Empty states v3 is in the file. I dropped the mascot and kept one line of copy and one action per screen. The calendar screen now offers "Add your first location".', { edited: '08:47' }),
    M('e2', 'ines', 'Yesterday 15:44', 'The "no appointments yet" one is much calmer now.')]);
  C('ch-engineering', 'channel', '', { name: 'engineering' }, [
    M('g1', 'ines', '09:15', 'Storybook is updated for the new date picker.'),
    M('mig1', 'maya', '10:05', '@Marco can you confirm the migration runs before the code freeze tonight?', { rx: [['✅', ['marco']]] }),
    M('t1', 'tomas', '10:31', '@Maya the export API can\'t ship both CSV and XLSX by Thursday. Which one matters for launch?', { isNew: 1 }),
    M('g2', 'kofi', '10:39', 'Android build 388 is green.')]);
  C('th-check', 'thread', 'Launch checklist', { where: 'product', tdial: 'needs', tauto: 'you started this thread' }, [
    M('c1', 'maya', 'Mon 11:00', 'Launch checklist for Thursday is in the doc. Reply here when your line is done or blocked.'),
    M('c2', 'sam', 'Yesterday 10:12', 'Support macros: done.'),
    M('c3', 'priya', '09:48', 'Landing page copy: done. Waiting on the final screenshots from design.', { isNew: 1 }),
    M('c4', 'lena', '10:11', 'Screenshots by 14:00 today.', { isNew: 1 }),
    M('c5', 'priya', '10:33', 'Blocked: the pricing page still shows the old Team plan. Who owns that?', { isNew: 1 })]);
  C('ch-support', 'channel', '', { name: 'support' }, [
    M('u1', 'ruth', 'Yesterday 11:02', 'Brightwell renewed for 12 months.'),
    M('u2', 'triage', '09:58', '14 tickets since 09:00 mention double-booked slots, all from clinics with two locations. This looks like a product bug, not a support question. Someone from product or engineering should look.', { flag: 1, isNew: 1, rx: [['👀', ['sam']]] })]);
  C('ch-sales', 'channel', '', { name: 'sales' }, [
    M('j0', 'jonas', 'Yesterday 16:15', 'The Brightwell call moved to Tuesday 15:00.'),
    M('j1', 'jonas', '10:27', '@Maya can we promise Brightwell SSO in Q1? They will ask on the call today.', { isNew: 1 }),
    M('j2', 'ruth', '10:35', 'Their admin team is 4 people, for what it is worth.')]);
  C('th-comp', 'thread', 'Competitor pricing scan', { where: 'research', tdial: 'needs', tauto: 'you started this thread' }, [
    M('p1', 'maya', 'Mon 14:02', '@Fieldnotes read the public pricing pages of our 20 closest competitors and tell me who charges per location.'),
    M('p2', 'field', 'Mon 14:40', 'Progress: 12 of 20 pages read. Nothing blocking.', { info: 1 }),
    M('p3', 'field', '10:08', 'Done: 20 of 20 read. 7 charge per location, 11 per seat, 2 a flat fee. The median per-location price is €49 a month. The full table is in the doc linked at the top of this thread.', { isNew: 1, status: 'Done' })]);
  C('th-metrics', 'thread', 'Weekly metrics refresh', { where: 'data', tdial: 'fyi', tauto: 'you were mentioned in it' }, [
    M('w1', 'dele', 'Mon 09:12', 'Weekly refresh is running. @Maya the activation dashboard will use the new funnel definition.'),
    M('w2', 'ledger', '10:15', 'Progress: 4 of 7 dashboards rebuilt. Activation is next. No action needed.', { info: 1, isNew: 1, status: 'Working' })]);
  C('th-beta', 'thread', 'Your message in #general', { where: 'general', tdial: 'needs', tauto: 'you started this thread' }, [
    M('b1', 'maya', 'Yesterday 16:02', '2.0 beta is open to the whole team from today. Break it and tell me.', { rx: [['🎉', ['anna', 'marco', 'lena', 'sam', 'kofi', 'ines']], ['❤️', ['priya', 'ruth']]] })]);
  C('ch-general', 'channel', '', { name: 'general' }, [
    M('n0', 'anna', 'Yesterday 09:00', 'Two days to go. All-hands is Wednesday 16:00, short.'),
    M('n1', 'sam', '09:30', '@channel the support rota for launch week is pinned. Check your slot before Wednesday.', { isNew: 1 })]);
  C('th-icons', 'thread', 'Icon set for 2.0', { where: 'design', tdial: 'fyi', tauto: 'you clicked Follow' }, [
    M('k1', 'lena', 'Mon 13:30', 'Icon set for 2.0: outline, 1.5 px stroke. Objections by Wednesday.'),
    M('k2', 'kofi', 'Yesterday 10:10', 'The tab bar icons look thin on Android at 1.5.'),
    M('k3', 'lena', '09:41', 'Trying 1.75 for the tab bar only.', { isNew: 1 }),
    M('k4', 'ines', '09:52', 'Web is fine either way.', { isNew: 1 })]);
  C('ch-marketing', 'channel', '', { name: 'marketing' }, [
    M('q0', 'priya', 'Yesterday 15:00', 'Newsletter goes out Thursday 09:05, five minutes after the embargo.'),
    M('q1', 'priya', '10:18', 'The launch post is scheduled. Thanks @Maya for the copy review.', { info: 1, isNew: 1 })]);
  C('ch-data', 'channel', '', { name: 'data' }, [
    M('d0', 'dele', 'Mon 09:12', 'Weekly refresh is running. Thread below.'),
    M('d00', 'omar', 'Mon 09:40', 'I need the revenue one by Wednesday noon.')]);
  const R = (id, conv, bucket, items, by, t, reason, why, o) => Object.assign({ id, conv, bucket, items, by, t, reason, why, sec: 'new', read: false, kept: null, done: false, replies: [] }, o || {});
  const rows = [
    R('r-anna', 'dm-anna', 'focus', ['a1', 'a2', 'a3', 'a4', 'a5', 'a6', 'a7', 'a8', 'a9', 'a10'], 'anna', '10:41', 'In Focus: direct message from Anna', 'Anna is one of your Focus people: everyone who writes to you directly is added.', { rule: 'dm' }),
    R('r-rc', 'th-rc', 'focus', ['rc4'], 'ship', '10:21', 'In Focus: Shipwright flagged this for you', 'A flag that names you lands in Focus and is kept for you until you press Done, reply or react. Shipwright has stopped and is waiting for you.', { kept: 'flag', rule: 'flag' }),
    R('r-inc', 'ch-incidents', 'focus', ['i1', 'i2', 'i3', 'i4', 'i5', 'i6'], 'yuki', '10:24', 'In Focus: #incidents is set to Focus', 'You set this channel\'s dial to Focus, so every new post lands here, as one row however many there are.', { drive: 'channel', chan: 1 }),
    R('r-go', 'ch-launch', 'focus', ['l3'], 'anna', '09:12', 'Flagged by Anna', 'Anna flagged this and named you. You read it at 09:20, so it is kept for you until you press Done, reply or react. Nobody else\'s answer changes that.', { sec: 'kept', kept: 'flag', read: true, rule: 'flag' }),
    R('r-empty', 'th-empty', 'focus', ['e1'], 'lena', 'Yesterday', 'Kept by you', 'You kept this yesterday. It stays until you press Done, reply or react.', { sec: 'kept', kept: 'you', read: true, note: 'edited since you kept it' }),
    R('r-check', 'th-check', 'needs', ['c3', 'c4', 'c5'], 'priya', '10:33', 'Needs you: replies in a thread you started', 'Replies in a thread you started land in Needs you. The thread\'s dial is at Needs you, set automatically.', { drive: 'thread' }),
    R('r-export', 'ch-engineering', 'needs', ['t1'], 'tomas', '10:31', 'Needs you: Tomás mentioned you', 'A direct mention lands in Needs you. #engineering is at Default; a mention gets through at any level.'),
    R('r-sso', 'ch-sales', 'needs', ['j1'], 'jonas', '10:27', 'Needs you: Jonas mentioned you', 'A direct mention lands in Needs you. #sales is at Default.'),
    R('r-mig', 'ch-engineering', 'needs', ['mig1'], 'marco', '10:16', 'Needs you: Marco answered your mention', 'You mentioned Marco in this message, so his reaction counts as an answer and lands in Needs you with his name.', { title: 'Marco reacted ✅ to your message', sub: '#engineering', kind: 'rx' }),
    R('r-comp', 'th-comp', 'needs', ['p3'], 'field', '10:08', 'Needs you: reply in a thread you started', 'An agent\'s result carries no level, so it is sorted like anyone\'s reply: you started this thread.', { drive: 'thread' }),
    R('r-triage', 'ch-support', 'needs', ['u2'], 'triage', '09:58', 'Needs you: flagged for everyone in #support', 'A flag that names nobody lands in Needs you for everyone in the channel. It leaves once you have read it, like any row.'),
    R('r-metrics', 'th-metrics', 'fyi', ['w2'], 'ledger', '10:15', 'For your information: Ledger marked this Info', 'A progress update marked Info never lands higher than here and sends no notification.', { drive: 'thread' }),
    R('r-copy', 'ch-marketing', 'fyi', ['q1'], 'priya', '10:18', 'For your information: Priya marked this Info', 'It mentions you, but the sender\'s Info wins: a courtesy copy, one bucket down, no notification.'),
    R('r-icons', 'th-icons', 'fyi', ['k3', 'k4'], 'ines', '09:52', 'For your information: a thread you follow', 'You clicked Follow on this thread, so its replies land here.', { drive: 'thread' }),
    R('r-rota', 'ch-general', 'fyi', ['n1'], 'sam', '09:30', 'For your information: mention of the whole channel', 'A mention of the whole channel is not a mention of you.'),
    R('r-beta', 'th-beta', 'fyi', ['b1'], 'maya', 'Yesterday', 'For your information: reactions to your message', 'Reactions from people you did not address are grouped here and show the split by emoji.', { title: '🎉 6 · ❤️ 2 on your message', sub: '#general', kind: 'rx' })
  ];
  const T = (row, mid, bucket) => ({ row, mid, bucket });
  return {
    msgs, convs, rows, view: 'feed', btab: 'focus', htab: 'dms', conv: null, exp: new Set(), open: new Set(), lock: {}, menu: null, modal: null,
    pending: null, arrived: false, fyiSeen: false, level: '', focusOnly: false, demo: false, note: '', q: '', resort: false,
    dmAuto: true, dmPeople: ['anna', 'sam', 'marco', 'lena', 'tomas'], dmRemoved: [], dmAsk: false, off: {},
    rules: [{ id: 'flag', label: 'Flags that name you', n: 6, start: 1 }, { id: 'ch:incidents', label: 'Every new post in #incidents', n: 23, kind: 'channel', target: 'incidents' }],
    tray: [T('r-anna', 'a10', 'focus'), T('r-check', 'c5', 'needs'), T('r-export', 't1', 'needs'), T('r-sso', 'j1', 'needs'), T('r-inc', 'i6', 'focus'), T('r-rc', 'rc4', 'focus'), Object.assign(T('r-mig', 'mig1', 'needs'), { as: 'marco', lead: 'Reacted ✅ to your message: ', at: '10:16' }), T('r-comp', 'p3', 'needs'), T('r-triage', 'u2', 'needs'), T('r-go', 'l3', 'focus')],
    kept: new Set(), nid: 0
  };
}
let S = seed();
function conv(id) {
  if (!S.convs[id] && BG[id]) {
    const list = BG[id].map((x, i) => { const mid = id + '-' + i; S.msgs[mid] = { id: mid, by: x[0], t: x[1], text: x[2], rx: [], conv: id }; return mid; });
    S.convs[id] = id.startsWith('dm-') ? { id, kind: 'dm', title: '', with: id.slice(3), msgs: list } : { id, kind: 'channel', title: '', name: id.slice(3), msgs: list };
  }
  const c = S.convs[id];
  if (c && c.kind === 'channel' && !c.dial) { const ch = CH.find(x => x[0] === c.name); c.dial = ch ? ch[1] : 'default'; c.people = ch ? ch[3] : 5; }
  return c;
}
const HIST = {
  dms: [['dm-anna', 'anna', 'a10', '', 'Today 10:41'], ['dm-sam', 'sam', 's0', '', 'Yesterday'], ['dm-marco', 'marco', null, 'Freeze is at 20:00 tonight, not 18:00.', 'Yesterday'], ['dm-lena', 'lena', null, 'Sent you the v3 file for the empty states.', 'Monday'], ['dm-tomas', 'tomas', null, 'Do you have five minutes for the export question after standup?', 'Monday'], ['dm-ship', 'ship', null, 'Release notes draft for 1.9.4 is ready for your review.', 'Friday']],
  mentions: [['ch-engineering', 'tomas', 't1', '', 'Today 10:31', '#engineering'], ['ch-sales', 'jonas', 'j1', '', 'Today 10:27', '#sales'], ['th-rc', 'ship', 'rc4', '', 'Today 10:21', '#releases'], ['ch-marketing', 'priya', 'q1', '', 'Today 10:18', '#marketing'], ['ch-launch', 'anna', 'l3', '', 'Today 09:12', '#launch'], ['th-rc', 'marco', 'rc1', '', 'Yesterday', '#releases'], ['th-metrics', 'dele', 'w1', '', 'Monday', '#data']],
  threads: [['th-check', 'priya', 'c5', '', 'Today 10:33'], ['th-rc', 'ship', 'rc4', '', 'Today 10:21'], ['th-metrics', 'ledger', 'w2', '', 'Today 10:15'], ['th-comp', 'field', 'p3', '', 'Today 10:08'], ['th-icons', 'ines', 'k4', '', 'Today 09:52'], ['th-empty', 'ines', 'e2', '', 'Yesterday']],
  reactions: [['ch-engineering', 'marco', 'mig1', 'Marco reacted ✅ to your message', 'Today 10:16', '#engineering'], ['th-beta', 'anna', 'b1', '🎉 6 · ❤️ 2 on your message', 'Yesterday', '#general'], ['ch-pricing', 'omar', null, '✅ 4 · ❌ 1 on your message: "Vote: do we keep the Team plan name?"', 'Yesterday', '#pricing'], ['ch-product', 'lena', null, '👍 Lena, Tomás on your message: "Roadmap review moves to Wednesday 10:00."', 'Monday', '#product']]
};
const DIAL = { focus: ['Focus', 'Every new post, and everything addressed to you, in Focus'], all: ['All activity', 'The same posts; the ones not addressed to you go to For your information'], default: ['Default', 'Only what is addressed to you, by the starting rules'], muted: ['Muted', 'Only what names you, and your own threads'] };
const TDIAL = { focus: ['Focus', 'New replies land in Focus'], needs: ['Needs you', 'New replies land in Needs you'], fyi: ['For your information', 'What Follow sets'], off: ['Off', 'What Unfollow sets. What names you still gets through'] };
const BK = { focus: ['Focus', 'What matches your rules'], needs: ['Needs you', 'Directed at you'], fyi: ['For your information', 'Concerns you, not waiting on you'] };
const EMPTY = { focus: 'Nothing new in Focus.', needs: 'Nobody is waiting on you.', fyi: 'All caught up. What you read is in History.' };
const RCOUNT = { tomas: 7, anna: 19, marco: 11, lena: 8, priya: 9, sam: 6, jonas: 5, ship: 12, triage: 14, field: 4, ledger: 6 };
const CVOL = { engineering: 64, incidents: 23, launch: 41, support: 88, sales: 27, general: 52, product: 33, design: 29, marketing: 21, data: 12, releases: 18, research: 9 };

/* ---------- pieces ---------- */
const EMO = ['👍', '✅', '👀', '🎉'];
function toggleRx(m, e) { const x = m.rx.find(y => y[0] === e); if (x) { if (x[1].includes(ME)) { x[1] = x[1].filter(y => y !== ME); if (!x[1].length) m.rx = m.rx.filter(y => y !== x); } else x[1].push(ME); } else m.rx.push([e, [ME]]); }
function rxHTML(m) {
  if (!m.rx.length) return '';
  return '<div class="rxs">' + m.rx.map(([e, who]) => `<button class="rx${who.includes(ME) ? ' on' : ''}" data-act="reactMsg" data-m="${m.id}" data-e="${e}" title="${who.includes(ME) ? 'Remove your reaction' : 'React with ' + e}">${e} <i>${who.length > 2 ? who.length : who.map(w => w === ME ? 'You' : first(w)).join(', ')}</i></button>`).join('') + '</div>';
}
function msgHTML(id, ctx) {
  const m = S.msgs[id], k = ctx + ':' + id, L = S.lock[k] || {};
  const replies = (m.replies || []).map(x => msgHTML(x, ctx)).join('');
  const rep = replies ? `<div class="msg-replies">${replies}</div>` : '';
  if (m.deleted) {
    if (!replies && !L.self) return '';
    return `<div class="msg deleted" data-k="${k}"><div class="msg-self"${L.self ? ` style="height:${L.self}px"` : ''}><span class="tomb">${first(m.by)} deleted this message</span></div>${rep}</div>`;
  }
  const locked = m.edited && L.body && !S.open.has(k);
  const tags = (isAgent(m.by) ? '<span class="tag ag">agent</span>' : '') + (m.flag ? '<span class="tag flag">⚑ Flag</span>' : '') + (m.info ? '<span class="tag info">Info</span>' : '');
  const ed = m.edited ? `<span class="edited">edited</span>` + (locked ? `<button class="link" data-act="openMsg" data-k="${k}">Show all</button>` : '') : '';
  const tools = `<span class="msg-tools${S.rxFor === k ? ' open' : ''}"><button class="rx-add" data-act="rxFor" data-rk="${k}" aria-label="Add a reaction" title="Add a reaction">${ic('smile')}</button>${EMO.map(e => `<button class="rx-btn" data-act="reactMsg" data-m="${id}" data-e="${e}" aria-label="React ${e}">${e}</button>`).join('')}</span>`;
  return `<div class="msg${m.by === ME ? ' mine' : ''}${m.flag ? ' is-flag' : ''}${m.info ? ' is-info' : ''}" data-k="${k}"><div class="msg-self">${tools}${av(m.by)}<div class="msg-main"><div class="msg-head"><b>${pn(m.by)}</b>${tags}<time>${m.t}</time>${ed}</div><div class="msg-body${locked ? ' locked' : ''}"${locked ? ` style="max-height:${L.body}px"` : ''}>${rich(m.text)}</div>${rxHTML(m)}${m.status ? `<div class="status"><span class="st-dot"></span>${m.status}</div>` : ''}</div></div>${rep}</div>`;
}
function dialCtl(c, ctx) {
  const key = 'dial:' + c.id + ':' + ctx, open = S.menu === key;
  if (c.kind === 'channel') {
    const pop = open ? `<div class="pop"><div class="pop-h">What reaches your feed from #${c.name}</div>${Object.keys(DIAL).map(l => `<button class="opt${c.dial === l ? ' on' : ''}" data-act="setDial" data-c="${c.id}" data-l="${l}"><b>${DIAL[l][0]}</b><span>${DIAL[l][1]}</span></button>`).join('')}<div class="pop-f">A mention or a flag that names you gets through at every level.</div></div>` : '';
    return `<span class="dial"><button class="btn sm dial-btn" data-act="menu" data-key="${key}">#${c.name}: ${DIAL[c.dial][0]} ${ic('chev')}</button>${pop}</span>`;
  }
  if (c.kind === 'thread') {
    const on = c.tdial !== 'off';
    const pop = open ? `<div class="pop"><div class="pop-h">Where replies from this thread go</div>${Object.keys(TDIAL).map(l => `<button class="opt${c.tdial === l ? ' on' : ''}" data-act="setT" data-c="${c.id}" data-l="${l}"><b>${TDIAL[l][0]}</b><span>${TDIAL[l][1]}</span></button>`).join('')}<div class="pop-f">${c.tauto ? `Now: ${TDIAL[c.tdial][0]}${c.tset ? ', set by you' : ', set automatically because ' + c.tauto}.` : ''}</div></div>` : '';
    return `<span class="dial"><button class="btn sm${on ? ' on' : ''}" data-act="follow" data-c="${c.id}">${on ? 'Following' : 'Follow'}</button><button class="btn sm dial-btn" data-act="menu" data-key="${key}" aria-label="Thread level">${TDIAL[c.tdial][0]} ${ic('chev')}</button>${pop}</span>`;
  }
  return '';
}
/* ---- "why is this here, and I do not want it here": three ways to show it, picked with #why=a, b or c (E-002 follow-up) ---- */
const WHY = (location.hash.match(/why=([abc])/) || [])[1] || 'a'; // the user chose a (D-020)
const ruleOn = r => r.rule === 'dm' ? S.dmPeople.includes(conv(r.conv).with) : S.rules.some(x => x.id === r.rule);
function explain(r) {
  const c = conv(r.conv), rule = id => S.rules.find(x => x.id === id), acts = [];
  const off = S.off[r.id];
  let what = r.why, by = r.reason, alt = '';
  if (r.kept === 'you') { what = 'You pressed Keep on this row.'; by = 'Kept by you'; alt = 'It stays until you press Done, reply or react.'; }
  else if (r.rule === 'flag') {
    const x = rule('flag'); what = `${pn(r.by)} flagged this message and named you.`; by = 'Your rule "Flags that name you"'; alt = 'Without the rule it would be in Needs you, as a direct mention.';
    if (x) acts.push({ label: 'Stop putting flags in Focus', act: 'rmRuleId', d: `data-id="flag" data-rid="${r.id}" data-to="Needs you"`, note: `Removes the rule. ${x.n} items in the last 7 days would have gone to Needs you instead.` });
  } else if (r.rule === 'dm') { // Focus people: one entry per person who wrote to you directly (D-021)
    const p = c.with; what = `A direct message from ${pn(p)}, a person.`; by = `${first(p)} is one of your Focus people${S.dmAuto ? ', added the first time they wrote to you' : ''}`; alt = 'A direct message from someone who is not one of them lands in Needs you.';
    if (S.dmPeople.includes(p)) acts.push({ label: `Remove ${first(p)} from Focus`, act: 'rmPerson', d: `data-p="${p}" data-rid="${r.id}"`, note: `Only ${first(p)}; everyone else stays. ${RCOUNT[p] || 5} items in the last 7 days would have gone to Needs you instead.` });
  } else if (r.drive === 'channel') {
    what = `${r.items.length} new posts in #${c.name}. None of them names you.`; by = `You set #${c.name} to Focus`; alt = 'At Default they would not be in your feed at all.';
    acts.push({ label: `Set #${c.name} to All activity`, act: 'dialFrom', d: `data-c="${c.id}" data-l="all" data-rid="${r.id}" data-to="For your information"`, note: `${CVOL[c.name] || 20} posts in the last 7 days would have gone to For your information.` });
    acts.push({ label: `Set #${c.name} to Default`, act: 'dialFrom', d: `data-c="${c.id}" data-l="default" data-rid="${r.id}" data-to="nowhere in your feed"`, note: 'Only what names you, and your own threads, would reach you from it.' });
  }
  if (r.kept && !r.done) acts.unshift({ label: 'Done with this one', act: 'done', d: `data-id="${r.id}"`, note: 'Releases this row only. The rule stays.' });
  return { what, by, alt, acts, off };
}
function whyBody(r, full) {
  const e = explain(r), b = BK[r.bucket][0];
  const path = full ? `<ol class="why-path">${['focus', 'needs', 'fyi'].map(k => `<li class="${k === r.bucket ? 'hit' : ''}"><b>${BK[k][0]}</b><span>${k === r.bucket ? esc(e.by) + ' — decided here' : (['focus', 'needs', 'fyi'].indexOf(k) < ['focus', 'needs', 'fyi'].indexOf(r.bucket) ? 'no rule of yours matched' : k === 'needs' ? esc(e.alt || 'not reached') : 'not reached')}</span></li>`).join('')}</ol>` : '';
  const done = e.off ? `<p class="why-off">${e.off.msg ? esc(e.off.msg) + ` <button class="link" data-act="undoPerson" data-p="${e.off.p}" data-rid="${r.id}">Undo</button> · ` : `Changed. From the next time you open the feed, rows like this land in ${esc(e.off)}. `}<button class="link" data-act="modal" data-m="rules">Feed settings</button></p>` : '';
  const acts = e.off ? '' : e.acts.map(a => `<div class="why-act"><button class="btn sm" data-act="${a.act}" ${a.d}>${esc(a.label)}</button><small>${esc(a.note)}</small></div>`).join('');
  return `<div class="why-steps"><div><i>1</i><span><b>What it is</b>${esc(e.what)}</span></div><div><i>2</i><span><b>What put it in ${b}</b>${esc(e.by)}</span></div>${e.alt && !full ? `<div><i>3</i><span><b>Otherwise</b>${esc(e.alt)}</span></div>` : ''}</div>${path}${acts || done ? `<div class="why-acts"><b class="why-q">Not what you want in ${b}?</b>${acts}${done}</div>` : ''}`;
}
const CAP = 6; // a long burst shows its newest messages by the reply box; the earlier new ones are one click above them
const left = r => r.read ? 0 : r.items.length;
function ctxFor(r) { // context is what the new messages answer: the message before them in a direct conversation, or the start of the thread
  const c = conv(r.conv); if (c.kind === 'channel' || r.kind === 'rx') return '';
  const i = c.msgs.indexOf(r.items[0]), ids = [];
  if (c.kind === 'thread' && i > 1) ids.push(c.msgs[0]);
  if (i > 0) ids.push(c.msgs[i - 1]);
  const h = ids.filter(id => !S.msgs[id].deleted).map(id => msgHTML(id, 'ctx')).join('');
  return h ? `<div class="ctx"><span class="ctx-l">${c.kind === 'thread' ? 'In this thread' : 'Before'}${i > ids.length ? ` · <button class="link" data-act="openConv" data-c="${c.id}" data-id="${r.id}">${i - ids.length} earlier</button>` : ''}</span>${h}</div>` : '';
}
function rowHTML(r) {
  const c = conv(r.conv), ms = r.items.map(i => S.msgs[i]), last = ms[ms.length - 1], exp = S.exp.has(r.id);
  const k = 'row:' + last.id, L = S.lock[k] || {};
  const where = c.kind === 'dm' ? 'Direct message' : c.kind === 'channel' ? '#' + c.name : '#' + c.where;
  if (ms.every(m => m.deleted)) return `<article class="row gone b-${r.bucket}" data-row="${r.id}"><div class="row-main" style="height:${L.self || 64}px"><span class="tomb">${first(last.by)} deleted this message</span><small>${where} · it was here because ${first(last.by)} mentioned you</small></div></article>`;
  const title = r.title || (c.kind === 'dm' ? pn(c.with) : c.kind === 'channel' ? (r.chan ? '#' + c.name : pn(last.by)) : c.title);
  const sub = r.sub || (c.kind === 'dm' ? '' : c.kind === 'channel' ? (r.chan ? '' : '#' + c.name) : '#' + c.where);
  const n = left(r), cnt = n > 1 ? `<span class="row-count">${n} new</span>` : '';
  const marks = (ms.some(m => m.flag) ? '<span class="tag flag">⚑</span>' : '') + (ms.some(m => m.info) ? '<span class="tag info">Info</span>' : '') + (isAgent(r.by) ? '<span class="tag ag">agent</span>' : '');
  const state = r.done ? '<span class="state">Done</span>' : r.keptNow ? '<span class="state">Kept</span>' : '';
  const badge = r.read ? '' : `<span class="row-badge">${n > 1 ? n : ''}</span>`;
  const lockS = last.edited && L.body ? ` style="max-height:${L.body}px"` : '';
  let x = '';
  if (exp) {
    const mine = r.replies.map(i => msgHTML(i, 'exp')).join('');
    const kd = r.kept && !r.done ? `<button class="btn sm pri" data-act="done" data-id="${r.id}">Done</button>` : r.done ? '' : `<button class="btn sm" data-act="keep" data-id="${r.id}">Keep</button>`;
    x = `<div class="row-exp">${WHY === 'b' ? `<div class="why-inline">${whyBody(r)}</div>` : `<p class="why"><b>Why it is here.</b> ${esc(r.why)}</p>`}${ctxFor(r)}${ctxFor(r) ? '<div class="newline"><span>New</span></div>' : ''}${r.items.length > CAP && !r.all ? `<div class="more-new"><button class="btn sm" data-act="more" data-id="${r.id}">Show ${r.items.length - CAP} earlier new messages</button></div>` : ''}${(r.all ? r.items : r.items.slice(-CAP)).map(i => msgHTML(i, 'exp')).join('')}${mine}
      <div class="acts"><span class="quick"><small>React to the latest</small>${EMO.map(e => `<button class="rx-btn" data-act="react" data-id="${r.id}" data-e="${e}" aria-label="React ${e}">${e}</button>`).join('')}</span>${kd}<button class="btn sm" data-act="openConv" data-c="${r.conv}" data-id="${r.id}">Open conversation</button>${dialCtl(c, r.id)}<button class="btn sm" data-act="ruleFor" data-c="${r.conv}" data-by="${r.by}">Add to Focus…</button></div>
      <form class="reply" data-form="reply" data-id="${r.id}"><input name="t" autocomplete="off" aria-label="Reply" placeholder="Reply ${c.kind === 'dm' ? 'to ' + first(c.with) : 'in ' + where}…"><button class="btn pri">Send</button></form></div>`;
  }
  return `<article class="row b-${r.bucket} ${r.read ? 'read' : 'unread'}${exp ? ' expanded' : ''}${r.sec === 'kept' ? ' kept' : ''}${r.done ? ' done' : ''}" data-row="${r.id}">
    <div class="row-main" role="button" tabindex="0" data-act="toggle" data-id="${r.id}" aria-expanded="${exp}">${av(r.chan ? '#' : r.kind === 'rx' ? r.by : last.by)}
      <div class="row-text"><div class="row-head"><span class="row-title">${esc(title)}</span>${sub ? `<span class="row-sub">${sub}</span>` : ''}${marks}${cnt}<span class="row-time">${r.t}${last.edited ? ' · edited' : ''}</span></div>
        <div class="row-reason"><span class="why-dot"></span>${WHY === 'a' ? `<button class="why-btn" data-act="why" data-id="${r.id}" aria-expanded="${S.whyOpen === r.id}">${esc(r.reason)} ${ic('chev')}</button>` : esc(r.reason)}${r.note ? ` · <em>${r.note}</em>` : ''}${WHY === 'c' ? `<button class="why-btn c" data-act="whySheet" data-id="${r.id}">Why? Change…</button>` : ''}</div>
        <div class="row-preview" data-k="${k}"${lockS}><b>${last.by === ME ? 'You' : first(last.by)}:</b> ${rich(last.text)}</div>
        ${r.kind !== 'rx' && last.rx.length ? rxHTML(last) : ''}</div>
      <div class="row-side">${state}${badge}<span class="row-btns">${!r.read ? `<button class="mark-read" data-act="read" data-id="${r.id}" aria-label="Mark read without opening" title="Mark read without opening">${ic('check')}</button>` : ''}<button class="open-btn" data-act="openConv" data-c="${r.conv}" data-id="${r.id}" aria-label="Open the conversation" title="Open the conversation at the first new message">${ic('open')}</button></span>${r.sec === 'kept' && !r.done ? `<button class="btn sm pri" data-act="done" data-id="${r.id}">Done</button>` : ''}<span class="row-chev">${ic('chev')}</span></div></div>${WHY === 'a' && S.whyOpen === r.id ? `<div class="why-pop" data-stop="1"><h3>Why this is in ${BK[r.bucket][0]}</h3>${whyBody(r)}</div>` : ''}${x}</article>`;
}
const SUN = '<svg class="ill" viewBox="0 0 120 64" aria-hidden="true"><path d="M8 52h104" stroke="currentColor" stroke-width="2" stroke-linecap="round"/><path d="M36 52a24 24 0 0 1 48 0" fill="currentColor" opacity=".18"/><path d="M60 14v8M30 26l6 6M90 26l-6 6" stroke="currentColor" stroke-width="2" stroke-linecap="round"/><path d="M20 60h30M62 60h38" stroke="currentColor" stroke-width="2" stroke-linecap="round" opacity=".4"/></svg>';
function bucketHTML(b) {
  const rs = S.rows.filter(r => r.bucket === b), nw = rs.filter(r => r.sec !== 'kept'), kp = rs.filter(r => r.sec === 'kept');
  const n = nw.filter(r => !r.read).length;
  const cnt = b === 'fyi' ? (n ? '<span class="bk-dot" title="Something new"></span>' : '') : `<span class="bk-count">${n}</span>`;
  let body = nw.length ? nw.map(rowHTML).join('') : `<div class="empty">${SUN}<p>${EMPTY[b]}</p></div>`;
  if (kp.length) body += `<div class="kept-head"><b>Kept · ${kp.length}</b><small>Stays until you press Done, reply or react</small></div>` + kp.map(rowHTML).join('');
  return `<section class="bucket b-${b}" data-b="${b}"><header class="bk-head"><h2>${BK[b][0]}</h2>${cnt}<span class="bk-desc">${BK[b][1]}</span><span class="bk-acts">${b === 'focus' ? `<button class="btn sm" data-act="modal" data-m="rules">Rules · ${S.rules.length + S.dmPeople.length}</button>` : ''}<button class="btn sm" data-act="markAll" data-b="${b}">Mark all read</button></span></header>${body}</section>`;
}
const counts = () => ({ focus: S.rows.filter(r => r.bucket === 'focus' && r.sec !== 'kept' && !r.read).length, needs: S.rows.filter(r => r.bucket === 'needs' && !r.read).length, fyi: S.rows.filter(r => r.bucket === 'fyi' && !r.read).length });
function feed() {
  const bs = Skin.buckets === 'tabs' ? [S.btab] : ['focus', 'needs', 'fyi'];
  const pill = S.pending ? `<button class="newpill" data-act="applyNew">${S.pending.length} new</button>` : '';
  return `<div class="feed-wrap">${pill}<div class="feed-scroll" data-scroll="feed">${bs.map(bucketHTML).join('')}</div></div>`;
}
function bucketTabs() {
  const c = counts();
  return `<nav class="tabs btabs">${['focus', 'needs', 'fyi'].map(b => `<button class="tab b-${b}${S.btab === b ? ' on' : ''}" data-act="btab" data-b="${b}">${BK[b][0]}${b === 'fyi' ? (c.fyi ? '<span class="bk-dot"></span>' : '') : `<span class="bk-count">${c[b]}</span>`}</button>`).join('')}</nav>`;
}
const HT = { dms: 'Direct messages', mentions: 'Mentions', threads: 'Threads', reactions: 'Reactions' };
const hUnread = e => !!e[2] && !S.msgs[e[2]].deleted && S.rows.some(r => !r.read && r.items.includes(e[2]));
function histTabs(short) { // each tab counts its unread entries; the count is quiet, so the feed's badge stays the one that calls
  return Object.keys(HT).map(t => { const n = HIST[t].filter(hUnread).length; return `<button class="tab${S.view === 'history' && S.htab === t ? ' on' : ''}" data-act="htab" data-t="${t}" aria-label="${HT[t]}${n ? ', ' + n + ' unread' : ''}">${short && t === 'dms' ? 'DMs' : HT[t]}${n ? `<span class="h-count">${n}</span>` : ''}</button>`; }).join('');
}
function history() {
  const list = HIST[S.htab].filter(e => !e[2] || !S.msgs[e[2]].deleted).map(e => {
    const [cid, by, mid, text, when, where] = e, c = conv(cid), m = mid && S.msgs[mid];
    const unread = hUnread(e);
    const title = S.htab === 'threads' ? c.title : S.htab === 'reactions' ? '' : pn(by);
    const body = S.htab === 'reactions' ? esc(text) + (m ? `<span class="h-quote">${rich(m.text)}</span>` : '') : `${S.htab === 'threads' ? `<b>${first(by)}:</b> ` : ''}${rich(m ? m.text : text)}`;
    const extra = S.htab === 'threads' ? `<span class="row-sub">#${c.where} · ${TDIAL[c.tdial][0]}</span>` : where ? `<span class="row-sub">${where}</span>` : '';
    const kk = cid + ':' + (mid || text);
    return `<article class="row hrow${unread ? ' unread' : ''}"><div class="row-main" role="button" tabindex="0" data-act="openConv" data-c="${cid}">${av(by)}<div class="row-text"><div class="row-head">${unread ? '<span class="dot" title="Unread"></span>' : ''}${title ? `<span class="row-title">${esc(title)}</span>` : ''}${extra}${m && m.flag ? '<span class="tag flag">⚑</span>' : ''}${m && m.info ? '<span class="tag info">Info</span>' : ''}<span class="row-time">${when}${m && m.edited ? ' · edited' : ''}</span></div><div class="row-preview">${body}</div></div><div class="row-side"><button class="btn sm${S.kept.has(kk) ? ' on' : ''}" data-act="keepH" data-k="${esc(kk)}">${S.kept.has(kk) ? 'Kept' : 'Keep'}</button></div></div></article>`;
  }).join('');
  return `<div class="feed-wrap"><div class="feed-scroll" data-scroll="hist"><p class="hist-note">History, for finding things again. Newest first, read and unread together; a dot marks what you have not read yet. Nothing here is sorted by buckets, and nothing leaves by being read.</p>${list}</div></div>`;
}
function chats() {
  const dialTag = d => d === 'default' ? '' : `<span class="lvl l-${d}">${DIAL[d][0]}</span>`;
  const ch = CH.map(([n, d, u]) => { const c = S.convs['ch-' + n]; const dd = c && c.dial ? c.dial : d; const un = S.rows.some(r => r.conv === 'ch-' + n && !r.read) || (u && !S.convs['ch-' + n]); return `<button class="chat${un ? ' unread' : ''}${S.conv === 'ch-' + n ? ' on' : ''}" data-act="openConv" data-c="ch-${n}"><span class="hash">#</span><span class="chat-n">${n}</span>${dialTag(dd)}${un ? '<span class="dot"></span>' : ''}</button>`; }).join('');
  const dm = DMS.map(p => { const un = S.rows.some(r => r.conv === 'dm-' + p && !r.read); return `<button class="chat${un ? ' unread' : ''}${S.conv === 'dm-' + p ? ' on' : ''}" data-act="openConv" data-c="dm-${p}">${av(p, 'xs')}<span class="chat-n">${pn(p)}</span>${isAgent(p) ? '<span class="tag ag">agent</span>' : ''}${un ? '<span class="dot"></span>' : ''}</button>`; }).join('');
  return `<div class="chats"><h3>Channels <small>${CH.length}</small></h3>${ch}<h3>Direct messages</h3>${dm}</div>`;
}
function convPane() {
  if (!S.conv) return `<div class="conv none"><div class="empty">${SUN}<p>Open a row to read the conversation here, next to the feed.</p></div></div>`;
  const c = conv(S.conv);
  const title = c.kind === 'dm' ? pn(c.with) : c.kind === 'channel' ? '#' + c.name : c.title;
  const sub = c.kind === 'dm' ? P[c.with][1] + (isAgent(c.with) ? ' · agent' : '') : c.kind === 'channel' ? `Channel · ${c.people} people` : `Thread in #${c.where}`;
  let seenNew = false;
  const body = c.msgs.map(id => { const m = S.msgs[id]; let d = ''; if (m.isNew && !seenNew) { seenNew = true; d = '<div class="newline"><span>New</span></div>'; } return d + msgHTML(id, 'conv'); }).join('');
  const lv = l => `<button type="button" class="btn sm lv-${l}${S.level === l ? ' on' : ''}" data-act="level" data-l="${l}">${l === 'flag' ? '⚑ Flag' : 'Info'}</button>`;
  const by = c.kind === 'dm' ? c.with : S.msgs[c.msgs[c.msgs.length - 1]].by;
  return `<div class="conv k-${c.kind}"><header class="conv-head"><button class="icon-btn back" data-act="closeConv" aria-label="Back to the feed">${ic('back')}</button><div class="conv-title"><b>${esc(title)}</b><small>${sub}</small></div><div class="conv-acts">${dialCtl(c, 'conv')}<button class="btn sm" data-act="ruleFor" data-c="${c.id}" data-by="${by}">Add to Focus…</button></div></header>
    <div class="conv-body" data-scroll="conv">${body}</div>
    <form class="composer" data-form="send"><div class="levels">${lv('flag')}${lv('info')}<small>${S.level === 'flag' ? 'Needs attention. Stays with whoever you mention until they deal with it.' : S.level === 'info' ? 'For the record. Notifies nobody.' : ''}</small></div><div class="reply"><input name="t" autocomplete="off" aria-label="Message" placeholder="Message ${esc(title)}…"><button class="btn pri">Send</button></div></form></div>`;
}
function modalHTML() {
  const m = S.modal; if (!m) return '';
  let h = '';
  if (m.type === 'tray') {
    const es = S.tray.filter(e => !S.focusOnly || e.bucket === 'focus').map(e => {
      const msg = S.msgs[e.mid], r = S.rows.find(x => x.id === e.row), read = !r || r.read, c = conv(msg.conv);
      const where = c.kind === 'dm' ? 'Direct message' : c.kind === 'channel' ? '#' + c.name : '#' + c.where;
      const who = e.as || msg.by;
      return `<button class="tray-e${read ? ' read' : ''}" data-act="openConv" data-c="${msg.conv}" data-id="${e.row}">${av(who, 'xs')}<span class="tray-t"><b>${pn(who)}</b> <small>${where} · ${BK[e.bucket][0]} · ${e.at || msg.t}</small><span>${msg.deleted ? '<em>Message deleted</em>' : esc(e.lead || '') + rich(msg.text) + (msg.edited ? ' <em>(edited)</em>' : '')}</span></span></button>`;
    }).join('');
    h = `<div class="napp"><small>Not part of the product</small><h2>Notification app</h2><p class="sheet-note">A stand-in for the phone: what the notification sender delivered. In the real mock it is its own small web page, with a person to choose.</p><label class="switch">Delivered to <select disabled aria-label="Person"><option>Maya Okafor</option></select></label></div><p class="sheet-note">One per row, sent when a row becomes unread in Focus or Needs you${S.focusOnly ? ' (this person chose Focus only)' : ''}. For your information never notifies.</p>${es}`;
  } else if (m.type === 'rules') {
    const ppl = S.dmPeople;
    const dm = `<h3>Direct messages in Focus</h3><label class="switch"><input type="checkbox" data-act="dmAuto"${S.dmAuto ? ' checked' : ''}> Add everyone who writes to me directly</label>${S.dmAsk ? `<div class="ask"><p>New people will no longer be added. ${ppl.length} ${ppl.length === 1 ? 'person is' : 'people are'} already in Focus and stay there. Remove them too?</p><button class="btn sm pri" data-act="keepPeople">Keep them</button><button class="btn sm" data-act="rmAllPeople">Remove all ${ppl.length}</button></div>` : ''}${ppl.map(x => `<div class="rule person">${av(x, 'xs')}<span><b>${pn(x)}</b><small>${RCOUNT[x] || 5} items in the last 7 days</small></span><button class="btn sm" data-act="rmPerson" data-p="${x}">Remove</button></div>`).join('') || '<p class="sheet-note">Nobody. Direct messages land in Needs you.</p>'}${ppl.length > 1 && !S.dmAsk ? '<button class="link rm-all" data-act="rmAllPeople">Remove all</button>' : ''}`;
    h = `<h2>Feed and notifications</h2>${dm}<h3>Focus rules</h3><p class="sheet-note">Focus holds what matches these, and nothing else. Changes apply the next time you open the feed.</p>${S.rules.map((r, i) => `<div class="rule"><span><b>${esc(r.label)}</b><small>${r.n} items in the last 7 days${r.start ? ' · starting rule' : ''}</small></span><button class="btn sm" data-act="rmRule" data-i="${i}">Remove</button></div>`).join('')}
      <h3>Suggested</h3>${S.rules.some(r => r.id === 'mention') ? '' : `<div class="rule"><span><b>Any direct mention</b><small>Would have pulled in 23 items in the last 7 days. They are in Needs you today.</small></span><button class="btn sm pri" data-act="addRule" data-id="mention" data-label="Any direct mention" data-n="23">Add</button></div>`}<p class="sheet-note">Add a person, a channel or a thread from "Add to Focus…" wherever you see it.</p>
      <h3>Channel dials</h3><p class="sheet-note">Channels that are not at Default. Change a dial from the channel itself.</p>${CH.map(([n, d]) => [n, (S.convs['ch-' + n] && S.convs['ch-' + n].dial) || d]).filter(x => x[1] !== 'default').map(([n, d]) => `<div class="rule"><span><b>#${n}</b><small>${DIAL[d][0]}: ${DIAL[d][1]}</small></span><button class="btn sm" data-act="openConv" data-c="ch-${n}">Open</button></div>`).join('')}
      <h3 id="set-notif">Notifications</h3><p class="sheet-note">A notification is sent once per row, when it becomes unread in a bucket that notifies. There are no notification settings per channel: the dials above decide through the bucket.</p>
      <div class="rule"><span><b>Focus</b><small>Always notifies</small></span></div><div class="rule"><span><b>Needs you</b><small>${S.focusOnly ? 'Does not notify' : 'Notifies'}</small></span><label class="switch sw-in"><input type="checkbox" data-act="focusOnly"${S.focusOnly ? ' checked' : ''}> Focus only</label></div><div class="rule"><span><b>For your information</b><small>Never notifies</small></span></div>`;
  } else if (m.type === 'rule') {
    const c = conv(m.conv), opts = [];
    if (m.by && m.by !== ME) opts.push(['p:' + m.by, `Everything that reaches you from ${pn(m.by)}`, RCOUNT[m.by] || 5, 'person', m.by]);
    const chn = c.kind === 'channel' ? c.name : c.where;
    if (chn) opts.push(['ch:' + chn, `Every new post in #${chn}`, CVOL[chn] || 30, 'channel', chn]);
    if (c.kind === 'thread') opts.push(['th:' + c.id, `Every reply in "${c.title}"`, c.msgs.length, 'thread', c.id]);
    h = `<h2>Add to Focus</h2><p class="sheet-note">Each rule shows what it would have pulled into Focus over the last 7 days, before you save it.</p>${opts.map(o => { const has = S.rules.some(r => r.id === o[0]); return `<div class="rule"><span><b>${esc(o[1])}</b><small class="rule-n">${o[2]} items in the last 7 days${o[2] > 30 ? `, about ${Math.round(o[2] / 7)} a day. That would make Focus a long list.` : ''}</small></span><button class="btn sm pri" ${has ? 'disabled' : ''} data-act="addRule" data-id="${o[0]}" data-label="${esc(o[1])}" data-n="${o[2]}" data-kind="${o[3]}" data-target="${o[4]}">${has ? 'Added' : 'Add rule'}</button></div>`; }).join('')}`;
  }
  if (m.type === 'why') { const r = row(m.id); h = r ? `<h2>Why this is in ${BK[r.bucket][0]}</h2><p class="sheet-note">${esc(r.title || (conv(r.conv).kind === 'dm' ? pn(conv(r.conv).with) : conv(r.conv).title || '#' + conv(r.conv).name))} · ${r.t}. Every row is tested against the buckets from the top; the first that matches holds it.</p>${whyBody(r, true)}` : ''; }
  return `<div class="scrim" data-act="closeModal"><div class="sheet" role="dialog" aria-modal="true" data-stop="1"><button class="icon-btn close" data-act="closeModal" aria-label="Close">×</button>${h}</div></div>`;
}
const OTHER = { 'slack-like': 'Slack-like', 'telegram-like': 'Telegram-like', 'own': 'The agent\'s own' };
function demoHTML() {
  const b = (a, t) => `<button class="btn sm" data-act="${a}">${t}</button>`;
  const panel = S.demo ? `<div class="demo" data-stop="1"><h2>Demo controls</h2><p>Not part of the product. They stand in for the team acting while you watch.</p>
    ${b('playEdit', 'Tomás edits his message, longer')}${b('playDelete', 'Jonas deletes his message')}${b('playNew', 'Three new things arrive')}${b('leave', 'Leave the feed and come back')}<button class="btn sm" data-act="modal" data-m="tray">Open the notification app (outside the product)</button>${b('reset', 'Reset')}
    <p class="demo-note">${esc(S.note) || 'Tomás and Jonas are in Needs you.'}</p>
    <div class="demo-row">Theme ${['auto', 'light', 'dark'].map(t => `<button class="btn sm${(document.documentElement.dataset.theme || 'auto') === t ? ' on' : ''}" data-act="theme" data-t="${t}">${t}</button>`).join('')}</div>
    <div class="demo-row">Version ${Object.keys(OTHER).map(k => k === Skin.file ? `<b>${OTHER[k]}</b>` : `<a href="${k}.html">${OTHER[k]}</a>`).join(' · ')}</div></div>` : '';
  return `<button class="demo-btn" data-act="demo" aria-expanded="${S.demo}">Demo</button>${panel}`;
}
const h = {
  get S() { return S; }, feed, history, chats, conv: convPane, bucketTabs, histTabs, counts, ic, HT, BK,
  badge() { const c = counts(), n = c.focus + c.needs; return n ? `<span class="badge">${n}</span>` : ''; },
  bell() { return ''; },
  bellOld() { const n = S.tray.filter(e => { const r = S.rows.find(x => x.id === e.row); return r && !r.read && (!S.focusOnly || e.bucket === 'focus'); }).length; return `<button class="icon-btn bell" data-act="modal" data-m="tray" aria-label="Notifications">${ic('bell')}${n ? `<span class="badge">${n}</span>` : ''}</button>`; },
  search() { return `<div class="search">${ic('search')}<input type="search" data-search="1" aria-label="Search messages" placeholder="Search messages" value="${esc(S.q)}"><div class="sr" id="sr"></div></div>`; },
  nav(v, label, icon, extra) { return `<button class="nav${S.view === v ? ' on' : ''}" data-act="view" data-v="${v}">${ic(icon)}<span>${label}</span>${extra || ''}</button>`; }
};

/* ---------- behaviour ---------- */
const root = () => document.getElementById('app');
function render() {
  const keep = {}; document.querySelectorAll('[data-scroll]').forEach(e => keep[e.dataset.scroll] = e.scrollTop);
  root().innerHTML = Skin.shell(h) + modalHTML() + demoHTML();
  document.querySelectorAll('[data-scroll]').forEach(e => { if (keep[e.dataset.scroll]) e.scrollTop = keep[e.dataset.scroll]; });
  seeFyi();
}
function seeFyi() {
  if (S.view !== 'feed') return;
  const b = document.querySelector('.bucket.b-fyi'), sc = document.querySelector('.feed-scroll');
  if (b && sc && b.getBoundingClientRect().top < sc.getBoundingClientRect().bottom - 60) S.fyiSeen = true;
}
const row = id => S.rows.find(r => r.id === id);
function act(r) { r.read = true; if (r.kept) r.done = true; const m = S.msgs[r.items[r.items.length - 1]]; if (m.status === 'Waiting on Maya') m.status = 'Continuing'; }
function settle() { // the person leaves the feed: read rows go, kept rows move to Kept, rule and dial changes apply
  if (S.fyiSeen) S.rows.forEach(r => { if (r.bucket === 'fyi') r.read = true; });
  S.rows = S.rows.filter(r => {
    if (r.items.every(i => S.msgs[i].deleted)) return false;
    if (r.done) return false;
    if (r.read && r.kept) { r.bucket = 'focus'; r.sec = 'kept'; r.keptNow = false; if (r.kept === 'you') r.reason = 'Kept by you'; else r.reason = 'Flagged by ' + first(r.by); return true; }
    return !r.read;
  });
  S.rows = S.rows.filter(r => {
    if (r.read || r.sec === 'kept') return true;
    const c = conv(r.conv), info = r.items.every(i => S.msgs[i].info);
    if (r.drive === 'channel') { if (c.dial === 'all') { r.bucket = 'fyi'; r.reason = `For your information: #${c.name} is set to All activity`; } else if (c.dial === 'focus') { r.bucket = 'focus'; r.reason = `In Focus: #${c.name} is set to Focus`; } else return false; }
    if (r.drive === 'thread' && !info) { if (c.tdial === 'off') return false; if (c.tset) { r.bucket = c.tdial; r.reason = `${BK[c.tdial][0]}: you set this thread to ${TDIAL[c.tdial][0]}`; } }
    if (r.rule && !ruleOn(r) && r.bucket === 'focus' && !r.kept) { r.bucket = 'needs'; r.reason = 'Needs you: ' + r.reason.replace('In Focus: ', '') + (r.rule === 'dm' ? '' : ' (your Focus rule for it is off)'); r.why = r.rule === 'dm' ? `${first(r.by)} is not one of your Focus people, so a direct message lands in Needs you.` : r.why; }
    if (r.rule === 'dm' && ruleOn(r) && r.bucket === 'needs') { r.bucket = 'focus'; r.reason = 'In Focus: direct message from ' + first(r.by); }
    if (!info) { const hit = S.rules.find(x => (x.kind === 'person' && x.target === r.by) || (x.kind === 'channel' && (c.name === x.target) && r.bucket !== 'focus') || (x.kind === 'thread' && x.target === c.id) || (x.id === 'mention' && /mentioned you/.test(r.reason))); if (hit && r.bucket !== 'focus') { r.bucket = 'focus'; r.reason = `In Focus: your rule "${hit.label}"`; r.why = 'Your own Focus rule matched. You can remove it under Rules.'; } }
    return true;
  });
  S.exp.clear(); S.lock = {}; S.open.clear(); S.fyiSeen = false;
  const order = { focus: 0, needs: 1, fyi: 2 }; S.rows.sort((a, b) => order[a.bucket] - order[b.bucket]);
}
function snap(mid) { // remember the size of everything showing this message, so a change cannot move what is around it
  document.querySelectorAll(`[data-k$=":${mid}"]`).forEach(el => {
    const isRow = el.classList.contains('row-preview');
    const body = isRow ? el : el.querySelector('.msg-body'), self = isRow ? el.closest('.row-main') : el.querySelector('.msg-self');
    S.lock[el.dataset.k] = { body: body.getBoundingClientRect().height, self: self.getBoundingClientRect().height };
  });
}
function below(rid) { const el = document.querySelector(`[data-row="${rid}"]`); const nx = el && el.nextElementSibling; return nx ? nx.getBoundingClientRect().top : null; }
function onScreen(rid) { const el = document.querySelector(`[data-row="${rid}"]`); if (!el) return false; const b = el.getBoundingClientRect(); return b.bottom > 0 && b.top < innerHeight; }
function measured(rid, fn, what) { const a = below(rid); fn(); render(); const b = below(rid); S.note = a === null || b === null ? what + ' The row was not on screen.' : `${what} The row below moved by ${Math.abs(b - a).toFixed(1)} px.`; render(); }
const A = {
  toggle(d) { const r = row(d.id); if (S.exp.has(d.id)) S.exp.delete(d.id); else { S.exp.add(d.id); r.read = true; } },
  more(d) { row(d.id).all = true; },
  openConv(d) { if (S.conv !== d.c) S.lock = Object.fromEntries(Object.entries(S.lock).filter(([k]) => !k.startsWith('conv:'))); S.conv = d.c; conv(d.c); S.menu = null; S.modal = null; S.rows.forEach(r => { if (r.conv === d.c && (!d.id || r.id === d.id || conv(d.c).kind !== 'channel')) r.read = true; }); const sr = document.getElementById('sr'); if (sr) sr.innerHTML = ''; S.q = ''; },
  closeConv() { S.conv = null; S.menu = null; },
  read(d) { row(d.id).read = true; },
  keep(d) { const r = row(d.id); r.kept = r.kept || 'you'; r.read = true; r.keptNow = true; },
  done(d) { const r = row(d.id); r.done = true; r.read = true; },
  react(d) { const r = row(d.id); toggleRx(S.msgs[r.items[r.items.length - 1]], d.e); act(r); },
  reactMsg(d) { toggleRx(S.msgs[d.m], d.e); S.rxFor = null; S.rows.forEach(r => { if (r.items.includes(d.m)) act(r); }); },
  rxFor(d) { S.rxFor = S.rxFor === d.rk ? null : d.rk; },
  markAll(d) { S.rows.forEach(r => { if (r.bucket === d.b && r.sec !== 'kept') r.read = true; }); },
  view(d) { if (S.view === 'feed' && d.v !== 'feed') settle(); S.view = d.v; S.menu = null; },
  btab(d) { S.btab = d.b; if (d.b === 'fyi') S.fyiSeen = true; },
  htab(d) { if (S.view === 'feed') settle(); S.view = 'history'; S.htab = d.t; },
  menu(d) { S.menu = S.menu === d.key ? null : d.key; },
  setDial(d) { conv(d.c).dial = d.l; S.menu = null; S.note = 'Dial changed. Unread rows are re-sorted the next time the feed is opened.'; },
  setT(d) { const c = conv(d.c); c.tdial = d.l; c.tset = true; S.menu = null; },
  follow(d) { const c = conv(d.c); c.tdial = c.tdial === 'off' ? 'fyi' : 'off'; c.tset = true; S.menu = null; },
  modal(d) { S.modal = { type: d.m }; S.menu = null; },
  ruleFor(d) { S.modal = { type: 'rule', conv: d.c, by: d.by }; S.menu = null; },
  addRule(d) { S.rules.push({ id: d.id, label: d.label, n: +d.n, kind: d.kind, target: d.target }); if (d.kind === 'channel') { const c = conv('ch-' + d.target); if (c) c.dial = 'focus'; } },
  rmRule(d) { const r = S.rules.splice(+d.i, 1)[0]; if (r.kind === 'channel') { const c = conv('ch-' + r.target); if (c) c.dial = 'default'; } },
  closeModal() { S.modal = null; },
  why(d) { S.whyOpen = S.whyOpen === d.id ? null : d.id; },
  whySheet(d) { S.modal = { type: 'why', id: d.id }; },
  rmRuleId(d) { S.rules = S.rules.filter(x => x.id !== d.id); S.off[d.rid] = d.to; },
  rmPerson(d) { S.dmPeople = S.dmPeople.filter(x => x !== d.p); S.dmRemoved.push(d.p); if (d.rid) S.off[d.rid] = { p: d.p, msg: `${first(d.p)} is removed from Focus. Direct messages from ${first(d.p)} land in Needs you, starting the next time you open the feed.` }; },
  undoPerson(d) { if (!S.dmPeople.includes(d.p)) S.dmPeople.push(d.p); S.dmRemoved = S.dmRemoved.filter(x => x !== d.p); delete S.off[d.rid]; },
  dmAuto() { S.dmAuto = !S.dmAuto; S.dmAsk = !S.dmAuto && S.dmPeople.length > 0; },
  keepPeople() { S.dmAsk = false; },
  rmAllPeople() { S.dmRemoved.push(...S.dmPeople); S.dmPeople = []; S.dmAsk = false; },
  dialFrom(d) { conv(d.c).dial = d.l; S.rules = S.rules.filter(x => x.id !== 'ch:' + conv(d.c).name); S.off[d.rid] = d.to; },
  focusOnly() { S.focusOnly = !S.focusOnly; },
  openMsg(d) { S.open.add(d.k); },
  keepH(d) { if (S.kept.has(d.k)) S.kept.delete(d.k); else S.kept.add(d.k); },
  level(d) { S.level = S.level === d.l ? '' : d.l; },
  applyNew() {
    S.pending.forEach(p => { if (p.into) { const r = row(p.into); if (r) { r.items.push(...p.items); r.read = false; r.t = p.t; r.by = p.by; S.rows = [r, ...S.rows.filter(x => x !== r)]; return; } } S.rows.unshift(p.row); });
    const order = { focus: 0, needs: 1, fyi: 2 }; S.rows.sort((a, b) => (order[a.bucket] - order[b.bucket]) || ((a.sec === 'kept') - (b.sec === 'kept'))); S.pending = null;
  },
  demo() { S.demo = !S.demo; },
  playEdit() { const m = S.msgs.t1; if (m.edited) { S.note = 'Tomás has already edited it.'; return; } measured('r-export', () => { snap('t1'); m.text = LONG_EDIT; m.edited = '10:44'; }, 'Edited to five times the length.'); return true; },
  playDelete() {
    const m = S.msgs.j1; if (m.deleted) { S.note = 'Jonas has already deleted it.'; return; }
    const r = row('r-sso');
    if (r && !r.read && !onScreen('r-sso')) { m.deleted = true; S.rows = S.rows.filter(x => x !== r); S.note = 'Deleted before you had seen it: the row is gone without a trace and the count dropped.'; return; }
    measured('r-sso', () => { snap('j1'); m.deleted = true; }, 'Deleted. A marker holds its place until you move on.'); return true;
  },
  playNew() {
    if (S.arrived) { S.note = 'They have already arrived.'; return; } S.arrived = true;
    const mk = (id, by, t, text, cid) => { S.msgs[id] = { id, by, t, text, rx: [], conv: cid, isNew: 1 }; conv(cid).msgs.push(id); };
    mk('s1', 'sam', '10:43', 'Heads-up: Brightwell\'s admin just wrote in about double-booked slots. The same thing Triage flagged. It might come up on your 3 pm call.', 'dm-sam');
    mk('d1', 'dele', '10:43', '@Maya activation dropped 4 points after Friday\'s onboarding change. Worth a look before the go/no-go?', 'ch-data');
    mk('k5', 'kofi', '10:44', '1.75 works on Android. Ship it.', 'th-icons');
    const base = { sec: 'new', read: false, kept: null, done: false, replies: [] };
    if (S.dmAuto && !S.dmRemoved.includes('sam') && !S.dmPeople.includes('sam')) S.dmPeople.push('sam');
    const samIn = S.dmPeople.includes('sam');
    S.pending = [
      { row: Object.assign({ id: 'r-sam', conv: 'dm-sam', bucket: samIn ? 'focus' : 'needs', items: ['s1'], by: 'sam', t: '10:43', reason: (samIn ? 'In Focus' : 'Needs you') + ': direct message from Sam', why: samIn ? 'Sam is one of your Focus people.' : 'Sam is not one of your Focus people, so a direct message lands in Needs you.', rule: 'dm' }, base) },
      { row: Object.assign({ id: 'r-dele', conv: 'ch-data', bucket: 'needs', items: ['d1'], by: 'dele', t: '10:43', reason: 'Needs you: Dele mentioned you', why: 'A direct mention lands in Needs you. #data is at Default.' }, base, { replies: [] }) },
      { into: 'r-icons', items: ['k5'], t: '10:44', by: 'kofi', row: Object.assign({ id: 'r-icons', conv: 'th-icons', bucket: 'fyi', items: ['k5'], by: 'kofi', t: '10:44', reason: 'For your information: a thread you follow', why: 'You clicked Follow on this thread, so its replies land here.', drive: 'thread' }, base, { replies: [] }) }];
    S.tray.unshift({ row: 'r-sam', mid: 's1', bucket: samIn ? 'focus' : 'needs' }, { row: 'r-dele', mid: 'd1', bucket: 'needs' });
    S.note = 'Three arrived. Nothing in the list moved; they wait behind "3 new".';
  },
  leave() { settle(); S.note = 'You left and came back: read rows are in History, kept rows are under Kept, rule and dial changes are applied.'; },
  reset() { const t = S.demo; S = seed(); S.demo = t; },
  theme(d) { if (d.t === 'auto') delete document.documentElement.dataset.theme; else document.documentElement.dataset.theme = d.t; }
};
function submit(f) {
  const text = f.elements.t.value.trim(); if (!text) return;
  const id = 'me' + (++S.nid);
  if (f.dataset.form === 'reply') { const r = row(f.dataset.id); S.msgs[id] = { id, by: ME, t: 'now', text, rx: [], conv: r.conv }; conv(r.conv).msgs.push(id); r.replies.push(id); act(r); }
  else { const c = conv(S.conv); S.msgs[id] = { id, by: ME, t: 'now', text, rx: [], conv: c.id, flag: S.level === 'flag' ? 1 : 0, info: S.level === 'info' ? 1 : 0 }; c.msgs.push(id); S.level = ''; S.rows.forEach(r => { if (r.conv === c.id && r.read) act(r); }); }
  render();
  const b = document.querySelector('.conv-body'); if (b && f.dataset.form === 'send') b.scrollTop = b.scrollHeight;
}
function search(q) {
  S.q = q; const sr = document.getElementById('sr'); if (!sr) return;
  if (q.trim().length < 2) { sr.innerHTML = ''; return; }
  const hits = Object.values(S.msgs).filter(m => !m.deleted && m.conv && m.text.toLowerCase().includes(q.trim().toLowerCase())).slice(0, 7);
  sr.innerHTML = `<div class="sr-box">${hits.length ? hits.map(m => { const c = conv(m.conv); return `<button class="tray-e" data-act="openConv" data-c="${m.conv}">${av(m.by, 'xs')}<span class="tray-t"><b>${pn(m.by)}</b> <small>${c.kind === 'dm' ? 'Direct message' : '#' + (c.name || c.where)} · ${m.t}${m.edited ? ' · edited' : ''}</small><span>${rich(m.text)}</span></span></button>`; }).join('') : '<p class="sheet-note">Nothing found. Deleted messages are not searchable.</p>'}</div>`;
}
document.addEventListener('click', e => {
  const el = e.target.closest('[data-act]');
  if (!el) { if ((S.menu && !e.target.closest('.pop')) || (S.whyOpen && !e.target.closest('.why-pop'))) { S.menu = null; S.whyOpen = null; render(); } return; }
  if (el.dataset.act === 'closeModal' && el.classList.contains('scrim') && e.target.closest('[data-stop]')) return;
  if (el.tagName === 'INPUT' && el.type !== 'checkbox') return;
  const skip = A[el.dataset.act](el.dataset);
  if (!skip) render();
  if (el.dataset.act === 'openConv') { const nl = document.querySelector('.conv .newline'); if (nl) nl.scrollIntoView({ block: 'start' }); }
});
document.addEventListener('keydown', e => { if ((e.key === 'Enter' || e.key === ' ') && e.target.matches('[role="button"]')) { e.preventDefault(); e.target.click(); } if (e.key === 'Escape' && (S.modal || S.menu)) { S.modal = null; S.menu = null; render(); } });
document.addEventListener('submit', e => { e.preventDefault(); submit(e.target); });
document.addEventListener('input', e => { if (e.target.dataset.search) search(e.target.value); });
document.addEventListener('scroll', seeFyi, true);
window.E002 = { state: () => S };
render();
})();
