#!/usr/bin/env python3
"""E-003 harness: starts the spine as separate processes, drives a scripted team through the core's
door, injects failures, then compares every receiver with the truth. Usage: run.py [scenario ...]"""
import glob, json, os, random, shutil, sqlite3, subprocess, sys, tempfile, threading, time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import spine
from spine import HOLD, PEOPLE, call

CH = [f'#c{i}' for i in range(int(os.environ.get('E003_CHANNELS', '4')))]
ROWS = []          # (scenario, check, ok, expected_ok, detail)
NOTES = []         # (scenario, measurement, value)


class Sys:
    PORT = {'core': 0, 'search': 1, 'feed': 2, 'notifier': 3, 'px-search': 11, 'px-feed': 12, 'px-notifier': 13}

    def __init__(self, name, env=None, chaos=False):
        self.name, self.chaos = name, chaos
        self.dir = tempfile.mkdtemp(prefix=f'e003-{name}-')
        import socket
        while True:                                   # a block of ports nobody else on this machine uses
            self.base = random.randrange(11000, 31000, 20)
            try:
                for off in self.PORT.values():
                    with socket.socket() as k:
                        k.bind(('127.0.0.1', self.base + off))
                break
            except OSError:
                continue
        self.env = dict(os.environ, **(env or {}))
        self.procs = {}

    def url(self, role): return f'http://127.0.0.1:{self.base + self.PORT[role]}'
    def target(self, role): return self.url('px-' + role) if self.chaos else self.url(role)

    def start(self, role, wait=True):
        cmd = [sys.executable, HERE + '/spine.py']
        if role.startswith('px-'):
            cmd += ['proxy', '--port', str(self.base + self.PORT[role]), '--target', self.url(role[3:]), '--seed', str(self.PORT[role])]
        elif role == 'delivery':
            cmd += ['delivery', '--dir', self.dir, '--search-url', self.target('search'), '--feed-url', self.target('feed')]
        else:
            cmd += [role, '--dir', self.dir, '--port', str(self.base + self.PORT[role])]
            if role == 'feed':
                cmd += ['--notif-url', self.target('notifier')]
        log = open(f'{self.dir}/{role}.log', 'a')
        self.procs[role] = subprocess.Popen(cmd, env=self.env, stdout=log, stderr=log)
        if role != 'delivery' and wait:
            for _ in range(200):
                if call(self.url(role) + '/ping', timeout=0.5)[0] == 200:
                    return
                time.sleep(0.02)
            raise RuntimeError(f'{role} did not start; see {self.dir}/{role}.log')

    def up(self):
        for r in ['search', 'notifier'] + (['px-search', 'px-feed', 'px-notifier'] if self.chaos else []) + ['feed', 'core', 'delivery']:
            self.start(r)
        return self

    def kill(self, role):
        p = self.procs.pop(role)
        p.kill()
        p.wait()

    def wipe(self, role, files):
        self.kill(role)
        for f in files:
            for g in glob.glob(f'{self.dir}/{f}*'):
                os.remove(g)
        self.start(role)

    def down(self, expect_errors=False):
        for r in list(self.procs):
            self.kill(r)
        errs = sum(open(f).read().count('Traceback') for f in glob.glob(self.dir + '/*.log'))
        if errs and not expect_errors:
            NOTES.append((self.name, 'tracebacks in process logs', f'{errs} (kept in {self.dir})'))
        else:
            shutil.rmtree(self.dir, ignore_errors=True)

    def q(self, file, sql, args=()):
        c = sqlite3.connect(f'file:{self.dir}/{file}?mode=ro', uri=True, timeout=15)
        try:
            return c.execute(sql, args).fetchall()
        finally:
            c.close()

    def w(self, file, sql, args=()):
        c = sqlite3.connect(f'{self.dir}/{file}', timeout=15, isolation_level=None)
        try:
            c.execute(sql, args)
        finally:
            c.close()


class Driver:
    """A scripted team: posts, thread replies, mentions, flags, edits, deletes, reactions, reads, dials."""

    def __init__(self, sy, seed=1):
        self.sy, self.rng, self.L = sy, random.Random(seed), threading.Lock()
        self.msgs, self.n, self.acked, self.failed, self.threads = [], 0, [], 0, []

    def gen(self):
        with self.L:
            r = self.rng
            self.n += 1
            aid, p, x = f'a{self.n}', r.choice(PEOPLE), r.random()
            if x < 0.42 or len(self.msgs) < 5:
                mid, q, thread, mentions = f'm{self.n}', r.random(), None, []
                tops = [m for m in self.msgs[-40:] if m['conv'].startswith('#')]
                if q < 0.10:
                    conv = 'dm:' + ':'.join(sorted([p, r.choice([y for y in PEOPLE if y != p])]))
                elif q < 0.45 and tops:
                    par = r.choice(tops)
                    thread, conv = par['thread'] or par['id'], par['conv']
                else:
                    conv = r.choice(CH)
                if conv.startswith('#') and r.random() < 0.3:
                    mentions = r.sample([y for y in PEOPLE if y != p], r.choice([1, 1, 2]))
                lv = r.random()
                level = 'flag' if lv < 0.07 else 'info' if lv < 0.14 else None
                self.msgs.append(dict(id=mid, conv=conv, thread=thread, author=p, mentions=mentions, level=level))
                return dict(id=aid, kind='post', msg=mid, conv=conv, thread=thread, author=p, text=f'text {self.n}', mentions=mentions, level=level)
            m = r.choice(self.msgs[-15:]) if r.random() < 0.4 else r.choice(self.msgs)
            if x < 0.50:
                ment, o = list(m['mentions']), r.choice([y for y in PEOPLE if y != m['author']])
                if m['conv'].startswith('#'):
                    ment.remove(o) if o in ment else ment.append(o)
                level = m['level']
                if r.random() < 0.2:
                    level = None if level == 'flag' else 'flag'
                m['mentions'], m['level'] = ment, level
                return dict(id=aid, kind='edit', msg=m['id'], text=f'edited {self.n}', mentions=ment, level=level)
            if x < 0.55:
                self.msgs.remove(m)
                return dict(id=aid, kind='delete', msg=m['id'])
            if x < 0.67:
                return dict(id=aid, kind='react', msg=m['id'], person=p, emoji=r.choice('👍✅👀'), on=r.random() < 0.8)
            ck = spine.ck_of(m)
            if x < 0.90:
                return dict(id=aid, kind='read', person=p, ck=ck)
            if x < 0.93:
                return dict(id=aid, kind='watch', person=p, ck=ck if r.random() < 0.7 else None)
            if x < 0.96:
                return dict(id=aid, kind='dial', person=p, channel=r.choice(CH), level=r.choice(['default', 'muted', 'focus', 'all']))
            return dict(id=aid, kind='keep' if r.random() < 0.6 else 'done', person=p, ck=ck)

    def send(self, a):
        t0 = spine.clock()
        while call(self.sy.url('core') + '/act', a, timeout=2)[0] != 200:
            if spine.clock() - t0 > 60:
                self.failed += 1
                return False
            time.sleep(0.05)                          # same ID again: the core saves it once
        with self.L:
            self.acked.append(a['id'])
        return True

    def run(self, count, rate, threads=6):
        def w():
            gap = threads / rate
            for _ in range(count // threads):
                t = spine.clock()
                a = self.gen()
                self.send(a)
                if random.random() < 0.05:            # a screen that did not hear "saved" and sends again
                    call(self.sy.url('core') + '/act', a, timeout=2)
                time.sleep(max(0, gap - (spine.clock() - t)))
        self.threads = [threading.Thread(target=w) for _ in range(threads)]
        for t in self.threads:
            t.start()
        return self

    def join(self):
        for t in self.threads:
            t.join()


def quiesce(sy, timeout=120):
    """Wait until every receiver has everything and no notification is still held."""
    t0, ok = spine.clock(), 0
    while spine.clock() - t0 < timeout:
        try:
            head = call(sy.url('core') + '/head')[1]['head']
            cur = {n: p for n, p in sy.q('delivery.db', 'select name,place from cursors')}
            fm = dict(sy.q('feed.db', 'select k,v from meta'))
            fhead = sy.q('feed.db', 'select coalesce(max(number),0) from feedlog')[0][0]
            nm = dict(sy.q('notif.db', 'select k,v from meta'))
            pend = sy.q('notif.db', "select count(*) from n where state='pending'")[0][0]
            level = (cur.get('search') == head and cur.get('feed') == head and fm['last'] == head
                     and nm['last'] == fhead and (nm['edition'] == fm['edition'] or fhead == 0) and pend == 0)
            ok = ok + 1 if level else 0
            if ok >= 3:
                return spine.clock() - t0
        except Exception:
            ok = 0
        time.sleep(0.2)
    state = 'unknown'
    try:
        state = f'head {head}, places {cur}, feed at {fm["last"]}, feed notes {fhead}, sender at {nm["last"]}, held {pend}'
    except Exception:
        pass
    ROWS.append((sy.name, 'every receiver became level', False, True, state))
    open(f'{sy.dir}/delivery.log', 'a').write('Traceback: kept because the system did not become level\n')
    return None


def check(sy, drv, scenario, expect=None, only=None):
    """Compare the receivers with the truth. expect: {check name: False} for checks that must fail."""
    expect = expect or {}
    acts = {r[0] for r in sy.q('core.db', 'select id from actions')}
    log = sy.q('core.db', "select number,kind,body,saved_at from change_log where kind!='tick' order by number")
    ids = [json.loads(b)['act'] for _, _, b, _ in log]
    R = {}
    if drv:
        R['every acknowledged action is saved, once'] = (set(drv.acked) <= acts and len(ids) == len(set(ids)) and not drv.failed,
                                                         f'{len(drv.acked)} acknowledged, {len(log)} changes in the log')
    truth = {i: json.loads(b) for i, b in sy.q('core.db', 'select id,body from messages')}
    created = {json.loads(b)['id'] for _, k, b, _ in log if k == 'message.created'}
    vers = {}
    for _, k, b, _ in log:
        if k.startswith('message.'):
            m = json.loads(b)
            vers[m['id']] = max(vers.get(m['id'], 0), m['version'])
    lost = sum(1 for i, m in truth.items() if i not in created or vers.get(i) != m['version'])
    R['every saved change is in the log'] = (lost == 0, f'{lost} of {len(truth)} messages have a change missing from the log')
    docs = {i: (v, t, d) for i, v, t, d in sy.q('search.db', 'select id,version,text,deleted from docs')}
    bad = sum(1 for i in docs if i not in truth)
    for i, m in truth.items():
        d = docs.get(i)
        if not d or d[0] != m['version'] or bool(d[2]) != m['deleted'] or (not m['deleted'] and d[1] != m['text']):
            bad += 1
    R['search equals the truth'] = (bad == 0, f'{bad} of {len(truth)} messages differ')
    ref, reflog = spine.DictStore(), []
    for n, k, b, at in log:
        out = []
        spine.fold(ref, n, k, json.loads(b), out)
        reflog += [(e['kind'], e['name'], at) for e in out]
    live = dict(sy.q('feed.db', 'select k,v from kv'))
    livelog = [(k, json.loads(b)['name']) for k, b in sy.q('feed.db', "select kind,body from feedlog where kind!='tick' order by number")]
    diff = sum(1 for k in set(live) | set(ref.d) if live.get(k) != ref.d.get(k))
    R['the feed equals a clean replay of the log'] = (diff == 0 and livelog == [(k, n) for k, n, _ in reflog],
                                                      f'{diff} of {len(ref.d)} entries differ; {len(livelog)} notes against {len(reflog)}')
    due, wd = {}, {}
    for k, n, at in reflog:
        (due if k == 'due' else wd).setdefault(n, at)
    tray = [r[0] for r in sy.q('notif.db', 'select name from tray_log')]
    dup = len(tray) - len(set(tray)) + sum(1 for n in set(tray) if n not in due)
    viol = [n for n in set(tray) if n in wd and wd[n] - due.get(n, 0) < HOLD]
    missing = [n for n in due if n not in wd and n not in tray]
    R['a notification is sent once'] = (dup == 0, f'{dup} repeats among {len(tray)} sent')
    R['none sent for what was read or deleted within the hold'] = (not viol, f'{len(viol)} of {len(set(tray))} sent')
    R['no due notification is missing'] = (not missing, f'{len(missing)} missing of {len(due)} due')
    for name, (ok, detail) in R.items():
        if only and name not in only:
            continue
        ROWS.append((scenario, name, ok, expect.get(name, True), detail))
    return dict(due=due, wd=wd, tray=set(tray), log=log)


def pct(v, p):
    v = sorted(v)
    return v[min(len(v) - 1, int(len(v) * p))] if v else float('nan')


def latency(sy, file, t0=None, t1=None, kinds=None):
    saved = {n: at for n, k, at in sy.q('core.db', "select number,kind,saved_at from change_log where kind!='tick'")
             if (not kinds or k.startswith(kinds)) and (t0 is None or t0 <= at <= t1)}
    return [(t - saved[n]) * 1000 for n, t in sy.q(file, 'select number,t from applied') if n in saved]


def fmt(v):
    return f'median {pct(v, 0.5):.0f} ms, 19 in 20 within {pct(v, 0.95):.0f} ms, worst {max(v):.0f} ms ({len(v)} changes)' if v else 'no data'


def at(delay, fn):
    t = threading.Timer(delay, fn)
    t.start()
    return t


# ---------------------------------------------------------------------------------------------

def s_healthy():
    S = 'healthy burst'
    sy = Sys('healthy').up()
    drv = Driver(sy).run(3000, 300)
    drv.join()
    NOTES.append((S, 'time to level after the last action', f'{quiesce(sy):.1f} s (includes the {HOLD:.0f} s hold)'))
    c = check(sy, drv, S)
    NOTES.append((S, 'save to search', fmt(latency(sy, 'search.db', kinds='message.'))))
    NOTES.append((S, 'save to feed', fmt(latency(sy, 'feed.db'))))
    late = [(s - d - HOLD) * 1000 for s, d in sy.q('notif.db', 'select sent_at,due_at from tray_log')]
    NOTES.append((S, 'notification sent after its hold ended', fmt(late)))
    NOTES.append((S, 'load', f'{len(c["log"])} changes at about 300 actions a second; {len(c["due"])} notifications due, {len(c["tray"])} sent'))
    sy.down()


def s_chaos():
    S = 'repeats, lost answers and late copies on every hop'
    sy = Sys('chaos', chaos=True).up()
    drv = Driver(sy, 2).run(1500, 150)
    drv.join()
    q = quiesce(sy)
    time.sleep(2)                                     # let the last late copies land
    check(sy, drv, S)
    NOTES.append((S, 'faults', 'each delivery: 15% arrive late as a stale copy, 15% lose their answer, 20% are delivered twice'))
    NOTES.append((S, 'time to level', f'{q:.1f} s' if q else 'did not level'))
    sy.down()


def s_search_down():
    S = 'search stopped for 5 s mid-burst'
    sy = Sys('searchdown').up()
    drv = Driver(sy, 3).run(2400, 200)
    time.sleep(3)
    t0 = spine.clock(); sy.kill('search')
    time.sleep(5)
    t1 = spine.clock(); sy.start('search')
    drv.join()
    quiesce(sy)
    check(sy, drv, S)
    NOTES.append((S, 'feed freshness during the outage', fmt(latency(sy, 'feed.db', t0, t1))))
    back = [t for n, t in sy.q('search.db', 'select number,t from applied')
            if n in {n for n, a in sy.q('core.db', 'select number,saved_at from change_log') if t0 <= a <= t1}]
    NOTES.append((S, 'search caught up with the backlog', f'{max(back) - t1:.1f} s after it came back ({len(back)} changes)' if back else 'no data'))
    sy.down()


def s_search_slow():
    S = 'search answers after 1.5 s (longer than the 1 s limit) for 4 s'
    sy = Sys('searchslow').up()
    drv = Driver(sy, 4).run(2000, 200)
    time.sleep(3)
    t0 = spine.clock(); call(sy.url('search') + '/control', {'slow': 1.5})
    time.sleep(4)
    t1 = spine.clock(); call(sy.url('search') + '/control', {'slow': 0})
    drv.join()
    quiesce(sy)
    check(sy, drv, S)
    NOTES.append((S, 'feed freshness meanwhile', fmt(latency(sy, 'feed.db', t0, t1))))
    sy.down()


def release_parked(sy):
    c = sqlite3.connect(f'{sy.dir}/delivery.db', timeout=15, isolation_level=None)
    n = 0
    for rowid, payload in c.execute('select rowid,payload from parked order by number').fetchall():
        if call(sy.url('search') + '/changes', json.loads(payload))[0] == 200:
            c.execute('delete from parked where rowid=?', (rowid,))
            n += 1
    c.close()
    return n


def s_park():
    S = 'search refuses one delete, then refuses everything'
    sy = Sys('park').up()
    drv = Driver(sy, 5).run(300, 150)
    drv.join()
    core = sy.url('core') + '/act'
    call(core, dict(id='x1', kind='post', msg='mX', conv='#c0', author='p0', text='sent by mistake', mentions=[], level=None))
    call(sy.url('search') + '/control', {'refuse': 'mX'})
    call(core, dict(id='x2', kind='delete', msg='mX'))
    drv.run(120, 120).join()
    quiesce(sy)
    only = ['search equals the truth']
    NOTES.append((S, 'parked after the refused delete', str(sy.q('delivery.db', 'select count(*) from parked')[0][0])))
    check(sy, None, S + ' · while parked', expect={only[0]: False}, only=only)
    call(sy.url('search') + '/control', {'refuse': None})
    release_parked(sy)
    check(sy, None, S + ' · after "try again"', only=only)
    call(sy.url('search') + '/control', {'refuse': '*'})
    drv.run(120, 120).join()
    time.sleep(3)
    parked = sy.q('delivery.db', 'select count(*) from parked')[0][0]
    head = call(sy.url('core') + '/head')[1]['head']
    place = sy.q('delivery.db', "select place from cursors where name='search'")[0][0]
    ROWS.append((S + ' · refusing everything', 'the worker pauses and parks no more than two', parked <= 2 and place < head, True,
                 f'{parked} parked, {head - place} changes waiting'))
    call(sy.url('search') + '/control', {'refuse': None})
    quiesce(sy)
    release_parked(sy)
    check(sy, drv, S + ' · after recovery')
    sy.down()


def s_kill_core():
    S = 'the core killed five times mid-burst'
    sy = Sys('killcore').up()
    drv = Driver(sy, 6).run(2000, 200)
    for _ in range(5):
        time.sleep(1.5)
        sy.kill('core')
        sy.start('core')
    drv.join()
    quiesce(sy)
    check(sy, drv, S)
    sy.down()


def jump_feed_place(sy, by=40):
    """An operator's mistake: the feed worker's place is moved forward while the worker is stopped."""
    sy.kill('delivery')
    before = sy.q('delivery.db', "select place from cursors where name='feed'")[0][0]
    sy.w('delivery.db', "update cursors set place=place+?, last_sent=last_sent+? where name='feed'", (by, by))
    sy.start('delivery')
    NOTES.append((sy.name, 'feed place moved by mistake', f'from {before} to {before + by}'))


def s_kill_others():
    S = 'delivery, feed, sender and search each killed mid-burst; the feed\'s place moved by mistake'
    sy = Sys('killothers').up()
    drv = Driver(sy, 7).run(2400, 200)
    for role, pause in [('delivery', 0.7), ('feed', 1.0), ('notifier', 1.0), ('search', 0.7)]:
        time.sleep(1.5)
        sy.kill(role)
        time.sleep(pause)
        sy.start(role)
    time.sleep(1)
    jump_feed_place(sy)
    drv.join()
    quiesce(sy)
    check(sy, drv, S)
    sy.down()


def s_rebuild():
    S = 'the feed wiped mid-burst and rebuilt'
    sy = Sys('rebuild').up()
    drv = Driver(sy, 8).run(2500, 250)
    time.sleep(5)
    t0 = spine.clock()
    sy.wipe('feed', ['feed.db', 'feed-delivery.db'])
    drv.join()
    quiesce(sy)
    c = check(sy, drv, S)
    done = [t for n, t in sy.q('feed.db', 'select number,t from applied')]
    behind = len([1 for n, _, _, a in c['log'] if a <= t0])
    when = f'{max(0, sorted(done)[behind - 1] - t0):.1f} s after the wipe' if behind and len(done) >= behind else f'NOT LEVEL: {len(done)} applied'
    NOTES.append((S, 'rebuild', f'{behind} earlier changes replayed while new ones kept arriving; level again {when}'))
    S2 = 'then the notification sender wiped and refilled'
    before = set(c['tray'])
    sy.wipe('notifier', ['notif.db'])
    quiesce(sy)
    c2 = check(sy, drv, S2, only=['a notification is sent once', 'none sent for what was read or deleted within the hold', 'no due notification is missing'])
    gone = before - c2['tray']
    NOTES.append((S2, 'tray before and after', f'{len(before)} before, {len(c2["tray"])} after; {len(gone)} entries not refilled '
                  f'({sum(1 for n in gone if n in c["wd"])} of them were sent and read later), {len(c2["tray"] - before)} new'))
    S3 = 'then search wiped and its place set back to the start'
    sy.wipe('search', ['search.db'])
    sy.w('delivery.db', "update cursors set place=0, last_sent=0 where name='search'")
    t0 = spine.clock()
    q = quiesce(sy)
    check(sy, drv, S3, only=['search equals the truth'])
    NOTES.append((S3, 'refilled', f'{len(c["log"])} changes in {q:.1f} s'))
    sy.down()


def s_hold():
    """Read or delete 0.3 s after a mention, while the feed is down or too slow: is the notification still sent?"""
    for cond, env, kill in [('the feed down for 3 s', {}, True), ('the feed slower than the traffic', {'SLOW_FEED': '0.03'}, False)]:
        for mode in ['event', 'arrival', 'watermark']:
            S = f'hold "{mode}" · {cond}'
            sy = Sys('hold', env=dict(env, NOTIFY_MODE=mode)).up()
            core, cases = sy.url('core') + '/act', 240

            def post(a):
                while call(core, a, timeout=2)[0] != 200:
                    time.sleep(0.05)

            def case(i):
                A, B = PEOPLE[i % 12], PEOPLE[(i + 1) % 12]
                post(dict(id=f'r{i}', kind='post', msg=f'root{i}', conv=CH[i % 4], author=A, text='root', mentions=[], level=None))
                post(dict(id=f'm{i}', kind='post', msg=f'rep{i}', conv=CH[i % 4], thread=f'root{i}', author=A, text='mention', mentions=[B], level=None))
                time.sleep(0.3)
                if i % 3 == 0:
                    post(dict(id=f'f{i}', kind='read', person=B, ck=f't:root{i}'))
                elif i % 3 == 1:
                    post(dict(id=f'f{i}', kind='delete', msg=f'rep{i}'))
            ts = [at(i * 6 / cases, lambda i=i: case(i)) for i in range(cases)]
            if kill:
                time.sleep(2)
                sy.kill('feed')
                time.sleep(3)
                sy.start('feed')
            for t in ts:
                t.join()
            time.sleep(1)
            quiesce(sy, 180)
            c = check(sy, None, S, only=['__none__'])
            viol = [n for n in c['tray'] if n in c['wd'] and c['wd'][n] - c['due'][n] < HOLD]
            missing = [n for n in c['due'] if n not in c['wd'] and n not in c['tray']]
            quick = sum(1 for n in c['due'] if n in c['wd'] and c['wd'][n] - c['due'][n] < HOLD)
            ROWS.append((S, 'none sent for what was read or deleted within the hold', not viol, mode == 'watermark' or not viol,
                         f'{len(viol)} sent wrongly out of {quick} that were read or deleted in time; {len(missing)} due ones missing'))
            sy.down()


def mutation(name, brk, check_name, run, chaos=False, env=None):
    S = f'broken on purpose: {name}'
    sy = Sys('mut', env=dict(env or {}, BREAK=brk), chaos=chaos).up()
    drv = run(sy)
    drv.join()
    quiesce(sy, 60)
    time.sleep(2 if chaos else 0)
    check(sy, drv, S, expect={check_name: False}, only=[check_name])
    sy.down()


def check_outside(sy, scenario):
    """With fences up: is everything outside them still exactly what a clean replay gives?"""
    fences = [x[0] for x in sy.q('feed.db', 'select scope from fences')]
    convs = {f[5:] for f in fences if f.startswith('conv:')}
    readers = {f[7:] for f in fences if f.startswith('reader:')}
    log = sy.q('core.db', "select number,kind,body,saved_at from change_log where kind!='tick' order by number")
    ref, reflog = spine.DictStore(), []
    for n, k, b, at in log:
        out = []
        spine.fold(ref, n, k, json.loads(b), out)
        reflog += [(e['kind'], e['name']) for e in out]
    outside = lambda p, ck: p not in readers and ck not in convs
    rows = lambda d: {k: v for k, v in d.items() if k.startswith('row:') and outside(*k.split(':', 2)[1:])}
    live = dict(sy.q('feed.db', 'select k,v from kv'))
    a, b = rows(live), rows(ref.dump())
    diff = sum(1 for k in set(a) | set(b) if a.get(k) != b.get(k))
    inside = sum(1 for k in ref.d if k.startswith('row:') and not outside(*k.split(':', 2)[1:]))
    note_out = lambda nm: outside(*nm.split('/')[:2])
    livelog = [(k, json.loads(x)['name']) for k, x in sy.q('feed.db', "select kind,body from feedlog where kind!='tick' order by number")]
    la, lb = [x for x in livelog if note_out(x[1])], [x for x in reflog if note_out(x[1])]
    aside = sy.q('feed.db', 'select count(*) from set_aside')[0][0]
    ROWS.append((scenario, 'outside the fences, every row equals a clean replay', diff == 0, True,
                 f'{diff} of {len(b)} rows differ; {inside} rows are inside the fences; fenced: {", ".join(sorted(fences))}; {aside} changes set aside of {len(log)}'))
    ROWS.append((scenario, 'outside the fences, the notification notes equal a clean replay', la == lb, True, f'{len(la)} notes against {len(lb)}'))
    return fences


def s_fence():
    S = 'the feed cannot apply one edit and one dial change'
    sy = Sys('fence', env={'FEED_POISON': 'msg:mP,reader:p3'}).up()
    core = sy.url('core') + '/act'
    drv = Driver(sy, 11).run(300, 150)
    drv.join()
    call(core, dict(id='q1', kind='post', msg='rootP', conv='#c0', author='p0', text='root', mentions=[], level=None))
    call(core, dict(id='q2', kind='post', msg='mP', conv='#c0', thread='rootP', author='p0', text='reply', mentions=['p1'], level=None))
    call(core, dict(id='q3', kind='edit', msg='mP', text='reply, edited', mentions=['p1', 'p2'], level=None))   # the feed chokes on this
    call(core, dict(id='q4', kind='post', msg='mP2', conv='#c0', thread='rootP', author='p1', text='answer', mentions=[], level=None))
    call(core, dict(id='q5', kind='dial', person='p3', channel='#c1', level='muted'))                           # and on this
    drv.run(1500, 200).join()
    quiesce(sy)
    fences = check_outside(sy, S + ' · while fenced')
    ROWS.append((S + ' · while fenced', 'only what the two changes touch is fenced', sorted(fences) == ['conv:t:rootP', 'reader:p3'], True, ', '.join(sorted(fences))))
    check(sy, drv, S + ' · while fenced', expect={'the feed equals a clean replay of the log': False},
          only=['the feed equals a clean replay of the log', 'a notification is sent once', 'search equals the truth'])
    sy.env.pop('FEED_POISON')                         # the fault in the feed is fixed
    sy.wipe('feed', ['feed.db', 'feed-delivery.db'])  # and the feed is rebuilt from the log
    quiesce(sy)
    names = ['every acknowledged action is saved, once', 'every saved change is in the log', 'search equals the truth',
             'the feed equals a clean replay of the log', 'a notification is sent once', 'no due notification is missing']
    c = check(sy, drv, S + ' · after the fix and a rebuild', only=names)
    convs = {f[5:] for f in fences if f.startswith('conv:')}
    readers = {f[7:] for f in fences if f.startswith('reader:')}
    viol = [n for n in c['tray'] if n in c['wd'] and c['wd'][n] - c['due'][n] < HOLD]
    stray = [n for n in viol if n.split('/')[0] not in readers and n.split('/')[1] not in convs]
    ROWS.append((S + ' · after the fix and a rebuild', 'a notification sent although the row was read in time can only come from inside a fence',
                 not stray, True, f'{len(viol)} such notifications, {len(viol) - len(stray)} of them from inside the fences'))

    S = 'the feed cannot apply any edit'
    sy2 = Sys('fenceall', env={'FEED_POISON': 'all'}).up()
    drv = Driver(sy2, 12).run(600, 200)
    drv.join()
    time.sleep(2)
    nf = sy2.q('feed.db', 'select count(*) from fences')[0][0]
    head = call(sy2.url('core') + '/head')[1]['head']
    last = dict(sy2.q('feed.db', 'select k,v from meta'))['last']
    ROWS.append((S, 'after five fences the feed stops instead of fencing everything', nf == 5 and last < head, True, f'{nf} fences, {head - last} changes waiting'))
    sy2.env.pop('FEED_POISON')
    sy2.wipe('feed', ['feed.db', 'feed-delivery.db'])
    quiesce(sy2)
    check(sy2, drv, S + ' · after the fix and a rebuild')
    sy.down(expect_errors=True)
    sy2.down(expect_errors=True)


def s_agents():
    """Scripted agents only: they are told what addresses them, go away, crash and repeat themselves."""
    S = 'scripted agents at the agent door'
    sy = Sys('agents').up()
    core = sy.url('core')
    drv = Driver(sy, 21).run(1200, 150)                # ordinary traffic meanwhile
    stop, t0 = threading.Event(), spine.clock()
    got, place, repeats, lat = {a: [] for a in spine.AGENTS}, {a: 0 for a in spine.AGENTS}, [0], []
    full, cut_short = {}, {}                            # what a streamed reply should end up as
    pieces = [f'piece {k}. ' for k in range(5)]

    slowed = [0]

    def act(o):
        while True:
            st, out = call(core + '/agent/act', o, timeout=2)
            if st != 429:
                break
            slowed[0] += 1                              # told to slow down: wait and send the same thing again
            time.sleep(0.3)
        if out.get('dup'):
            repeats[0] += 1
        return st, out

    def respond(name, e, rng):
        m, n = e['message'], e['number']
        rid, thread = f'{name}-re-{n}', e['message']['thread'] or e['message']['id']
        if m['text'] == 'loop':                         # two agents that always answer each other
            other = 'a1' if name == 'a0' else 'a0'
            act(dict(id=rid, agent=name, kind='post', msg=rid, conv=m['conv'], thread=thread, text='loop', mentions=[other], cause=n))
        elif m['author'] not in PEOPLE:
            return
        elif n % 3 == 0:                                # a reply written piece by piece, ending with a flag
            act(dict(id=rid + '-start', agent=name, kind='start', msg=rid, conv=m['conv'], thread=thread, cause=n))
            dies = name == 'a2' and n % 6 == 0
            at = 0
            for k, p in enumerate(pieces):
                if dies and k == 2:
                    cut_short[rid] = ''.join(pieces[:2])
                    return                              # the agent dies mid-reply and never finishes
                act(dict(agent=name, kind='piece', msg=rid, at=at, text=p))
                if k == 1:
                    act(dict(agent=name, kind='piece', msg=rid, at=at, text=p))   # the same piece again
                at += len(p)
                time.sleep(0.02)
            act(dict(id=rid + '-finish', agent=name, kind='finish', msg=rid, level='flag', mentions=[m['author']]))
            full[rid] = ''.join(pieces)
        else:
            act(dict(id=rid, agent=name, kind='post', msg=rid, conv=m['conv'], thread=thread, text='done', mentions=[], level='info', cause=n))

    def agent(name, away=None):
        rng = random.Random(name)
        while not stop.is_set():
            if away and away[0] < spine.clock() - t0 < away[1]:
                time.sleep(0.1)                         # this agent is simply not there for a while
                continue
            st, out = call(f'{core}/agent/events?agent={name}&after={place[name]}&wait=0.5', timeout=2)
            if st != 200:
                time.sleep(0.05)
                continue
            for e in out['events']:
                got[name].append(e['number'])
                if name == 'a0':
                    lat.append((spine.clock() - e['t']) * 1000)
                respond(name, e, rng)
                if rng.random() < 0.2 and e['number'] not in (cut_short and [int(k.split('-re-')[1]) for k in cut_short]):
                    break                               # crashes before noting how far it got: reads this event again
                place[name] = e['number']
    ts = [threading.Thread(target=agent, args=('a0',)), threading.Thread(target=agent, args=('a1', (2, 5))), threading.Thread(target=agent, args=('a2',))]
    for t in ts:
        t.start()
    post = lambda o: call(core + '/act', o, timeout=2)
    time.sleep(0.5)
    post(dict(id='hl', kind='post', msg='loopstart', conv='#c0', author='p0', text='loop', mentions=['a0'], level=None))
    for i in range(180):                                # people mention agents, a1 also while it is away
        ag = ['a0', 'a2', 'a1'][i % 3]
        post(dict(id=f'h{i}', kind='post', msg=f'ask{i}', conv=CH[i % 4], author=PEOPLE[i % 12], text=f'question {i}', mentions=[ag], level=None))
        time.sleep(0.04)
    drv.join()
    time.sleep(6)                                       # a1 is back and has caught up; abandoned replies are closed
    stop.set()
    for t in ts:
        t.join()
    time.sleep(spine.RATE_WINDOW + 0.5)
    accepted = sum(1 for i in range(45) if call(core + '/agent/act', dict(id=f'rate{i}', agent='a2', kind='post', msg=f'rate{i}', conv='#c3',
                                                                           text='burst', mentions=[], level='info'))[0] == 200)
    quiesce(sy)
    ev = {a: [n for (n,) in sy.q('core.db', 'select number from agent_events where agent=? order by number', (a,))] for a in spine.AGENTS}
    ok = all(sorted(set(got[a])) == ev[a] and got[a] == sorted(got[a]) for a in spine.AGENTS)
    ROWS.append((S, 'every agent received every event addressed to it, in order', ok, True,
                 '; '.join(f'{a}: {len(ev[a])} events, read {len(got[a])} times' for a in spine.AGENTS) + ' (a1 was away for 3 s)'))
    truth = {i: json.loads(b) for i, b in sy.q('core.db', 'select id,body from messages')}
    answered = [f'{a}-re-{n}' for a in spine.AGENTS for n in ev[a]]
    human = [rid for rid in answered if rid in truth]
    missing = [a for a in ('a0', 'a1', 'a2') for n in ev[a]
               if json.loads(sy.q('core.db', 'select body from agent_events where agent=? and number=?', (a, n))[0][0])['message']['author'] in PEOPLE
               and f'{a}-re-{n}' not in truth]
    ROWS.append((S, 'a reply sent again after a crash is saved once, and none is missing', not missing and repeats[0] > 0, True,
                 f'{len(human)} replies saved, {repeats[0]} repeats recognised by their ID, {len(missing)} missing'))
    log = sy.q('core.db', "select kind,body from change_log where kind!='tick' order by number")
    streamed = {json.loads(b)['id'] for k, b in log if k == 'stream.started'}
    early = [json.loads(b)['id'] for k, b in log if k.startswith('message.') and json.loads(b).get('id') in streamed and not json.loads(b).get('finished', True)]
    docs = dict(sy.q('search.db', 'select id,text from docs'))
    wrong = [i for i in streamed if docs.get(i) != truth[i]['text'] or not truth[i]['finished']]
    ROWS.append((S, 'text in progress reached no receiver; the finished reply reached search whole', not early and not wrong and len(streamed) > 0, True,
                 f'{len(streamed)} replies written piece by piece; {len(early)} unfinished ones sent to receivers; {len(wrong)} differ in search'))
    bad_full = [i for i, t in full.items() if truth[i]['text'] != t or truth[i].get('stopped')]
    ROWS.append((S, 'a piece sent twice appears once', not bad_full and len(full) > 0, True, f'{len(full)} finished replies, {len(bad_full)} with wrong text'))
    bad_cut = [i for i, t in cut_short.items() if truth[i]['text'] != t or not truth[i].get('stopped')]
    ROWS.append((S, 'a reply abandoned mid-way is closed as "stopped" with the text so far', not bad_cut and len(cut_short) > 0, True,
                 f'{len(cut_short)} abandoned, {len(bad_cut)} not closed properly'))
    loop = [m for m in truth.values() if m.get('thread') == 'loopstart' and m['author'] in spine.AGENTS]
    ROWS.append((S, 'two agents that always answer each other stop after three messages', len(loop) == spine.CHAIN_CAP, True, f'{len(loop)} agent messages in the thread'))
    ROWS.append((S, 'an agent posting too fast is told to slow down', accepted == spine.RATE, True, f'{accepted} of 45 accepted in one window (limit {spine.RATE})'))
    NOTES.append((S, 'event written to received by a waiting agent', fmt(lat)))
    NOTES.append((S, 'agents told to slow down while catching up', f'{slowed[0]} times; each waited and sent again'))
    check(sy, drv, S + ' · and the receivers')
    sy.down()


def s_rebuild_full():
    """Not in the default run: E003_PEOPLE=36 E003_CHANNELS=20 run.py rebuild-full. The seeded team's size."""
    n = int(os.environ.get('E003_CHANGES', '50000'))
    S = f'rebuild at the seeded team\'s size: {len(PEOPLE)} participants, {len(CH)} channels'
    sy = Sys('full').up()
    drv = Driver(sy, 31).run(n, 700, threads=12)
    drv.join()
    quiesce(sy, 600)
    head = call(sy.url('core') + '/head')[1]['head']
    real = sy.q('core.db', "select count(*) from change_log where kind!='tick'")[0][0]
    rows = sy.q('feed.db', "select count(*) from kv where k like 'row:%'")[0][0]
    t0 = spine.clock()
    sy.wipe('feed', ['feed.db', 'feed-delivery.db'])
    while dict(sy.q('feed.db', 'select k,v from meta')).get('last') != head:
        time.sleep(0.2)
    took = spine.clock() - t0
    NOTES.append((S, 'feed rebuilt from nothing', f'{real} changes ({rows} rows for {len(PEOPLE)} readers) in {took:.1f} s: {real / took:.0f} changes a second, ' + ('up to 100 changes per call' if os.environ.get('BATCH') == '1' else 'one change per call')))
    t0 = spine.clock()
    sy.wipe('search', ['search.db'])
    sy.w('delivery.db', "update cursors set place=0, last_sent=0 where name='search'")
    while sy.q('delivery.db', "select place from cursors where name='search'")[0][0] != head:
        time.sleep(0.2)
    NOTES.append((S, 'search refilled from nothing', f'{spine.clock() - t0:.1f} s'))
    quiesce(sy, 600)
    check(sy, drv, S)
    sy.down()


def s_mutations():
    mutation('search without the version rule, under late copies', 'search_noguard', 'search equals the truth',
             lambda sy: Driver(sy, 2).run(1500, 150), chaos=True)

    def jump(sy):
        d = Driver(sy, 7).run(1000, 200)
        at(2.5, lambda: jump_feed_place(sy))
        return d
    mutation('a feed that accepts any newer change, with its place moved by mistake', 'feed_nonstrict', 'the feed equals a clean replay of the log', jump)

    def wipe(sy):
        d = Driver(sy, 8).run(1500, 250)
        at(4, lambda: sy.wipe('feed', ['feed.db', 'feed-delivery.db']))
        return d
    mutation('a sender that keeps no list of what it sent, with the feed rebuilt', 'notif_nodedupe', 'a notification is sent once', wipe)

    def crashy(sy):
        stop = threading.Event()

        def nurse():
            while not stop.is_set():
                if sy.procs['core'].poll() is not None:
                    sy.start('core', wait=False)
                time.sleep(0.05)
        threading.Thread(target=nurse, daemon=True).start()
        d = Driver(sy, 6).run(1500, 200)
        orig = d.join
        d.join = lambda: (orig(), stop.set())
        return d
    mutation('the log entry saved in a second step, with the core dying in between', 'split_tx', 'every saved change is in the log',
             crashy, env={'CRASH': '0.004'})


def s_numbering_model():
    """A model of the numbering hazard, not PostgreSQL: numbers are taken when a save starts and
    become visible when it finishes. A reader that remembers 'the highest number seen' steps over a
    save that finishes late. Handing out numbers one save at a time removes it."""
    S = 'numbering model (a model, not PostgreSQL)'
    for serial in (False, True):
        visible, L, gate, seq, seen, stop = set(), threading.Lock(), threading.Lock(), [0], set(), threading.Event()

        def writer(rng):
            for _ in range(300):
                if serial:
                    gate.acquire()
                with L:
                    seq[0] += 1
                    n = seq[0]
                time.sleep(rng.uniform(0, 0.002))     # the save is still open
                with L:
                    visible.add(n)
                if serial:
                    gate.release()

        def reader():
            place = 0
            while not stop.is_set() or place < seq[0]:
                with L:
                    new = sorted(n for n in visible if n > place)
                if new:
                    seen.update(new)
                    place = new[-1]
                elif stop.is_set():
                    break
                time.sleep(0.0005)
        ws = [threading.Thread(target=writer, args=(random.Random(i),)) for i in range(8)]
        rd = threading.Thread(target=reader)
        rd.start()
        for w in ws:
            w.start()
        for w in ws:
            w.join()
        time.sleep(0.05)
        stop.set()
        rd.join()
        skipped = seq[0] - len(seen)
        ROWS.append((S, 'numbers handed out one save at a time: nothing stepped over' if serial else 'numbers taken before the save finishes: nothing stepped over',
                     skipped == 0, serial, f'{skipped} of {seq[0]} changes never seen by the reader'))


SCENARIOS = {'healthy': s_healthy, 'chaos': s_chaos, 'search-down': s_search_down, 'search-slow': s_search_slow, 'park': s_park,
             'kill-core': s_kill_core, 'kill-others': s_kill_others, 'rebuild': s_rebuild, 'hold': s_hold,
             'fence': s_fence, 'agents': s_agents, 'mutations': s_mutations, 'numbering': s_numbering_model}
EXTRA = {'rebuild-full': s_rebuild_full}      # run by name, with the sizes in the environment

if __name__ == '__main__':
    random.seed(1)
    names = sys.argv[1:] or list(SCENARIOS)
    t0 = spine.clock()
    for n in names:
        print(f'[{spine.clock() - t0:5.0f} s] {n}', flush=True)
        {**SCENARIOS, **EXTRA}[n]()
    lines, unexpected = [], 0
    for sc, name, ok, exp, detail in ROWS:
        verdict = ('holds' if ok else 'BROKEN') if exp else ('caught' if not ok else 'NOT CAUGHT')
        if ok != exp:
            unexpected += 1
            verdict += ' (unexpected)'
        lines.append(f'{verdict:22} | {sc} | {name} | {detail}')
    lines.append('')
    lines += [f'measured | {sc} | {what} | {val}' for sc, what, val in NOTES]
    lines.append('')
    lines.append(f'{len(ROWS)} checks, {unexpected} unexpected. Timings scaled down: calls limited to {spine.TIMEOUT:.0f} s, hold {HOLD:.0f} s.')
    print('\n'.join(lines))
    if len(names) == len(SCENARIOS):
        open(HERE + '/results.txt', 'w').write('\n'.join(lines) + '\n')
        json.dump(dict(rows=ROWS, notes=NOTES), open(HERE + '/results.json', 'w'), indent=1, ensure_ascii=False)
    sys.exit(1 if unexpected else 0)
