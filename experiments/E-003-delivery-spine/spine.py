#!/usr/bin/env python3
"""E-003: a throwaway delivery spine. One file, started in several roles as separate processes.

Roles: core, delivery, search, feed, notifier, proxy. Standard library and SQLite only.
This is not a draft of the product. Timings are scaled down (see TIMEOUT and HOLD).
"""
import argparse, json, os, random, sqlite3, sys, threading, time, traceback, uuid
import urllib.error, urllib.parse, urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

TIMEOUT = 1.0   # seconds a delivery call may take (proposal: 3 s)
HOLD = 1.0      # seconds a notification is held before sending (proposal: 5 s)
BREAK = set(filter(None, os.environ.get('BREAK', '').split(',')))   # deliberate faults in the design
PEOPLE = [f'p{i}' for i in range(12)]
# The wall clock on this machine steps backwards now and then (WSL2). CLOCK=wall reproduces what that does.
clock = time.time if os.environ.get('CLOCK') == 'wall' else time.monotonic


def db(path):
    c = sqlite3.connect(path, check_same_thread=False, isolation_level=None, timeout=15)
    for _ in range(200):                          # two threads opening a new file at once
        try:
            c.execute('PRAGMA journal_mode=WAL')
            break
        except sqlite3.OperationalError:
            time.sleep(0.02)
    c.execute('PRAGMA synchronous=NORMAL')
    return c


def call(url, obj=None, timeout=TIMEOUT):
    data = json.dumps(obj if obj is not None else {}).encode()
    req = urllib.request.Request(url, data=data, headers={'Content-Type': 'application/json'})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, json.loads(r.read() or b'{}')
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read() or b'{}')
        except Exception:
            return e.code, {}
    except Exception as e:
        return 0, {'error': repr(e)}


def serve(port, handler):
    class H(BaseHTTPRequestHandler):
        def do_POST(self):
            n = int(self.headers.get('Content-Length') or 0)
            obj = json.loads(self.rfile.read(n) or b'{}')
            try:
                st, out = handler(self.path, obj)
            except Exception as e:
                traceback.print_exc()
                st, out = 500, {'error': repr(e)}
            b = json.dumps(out).encode()
            try:
                self.send_response(st)
                self.send_header('Content-Length', str(len(b)))
                self.end_headers()
                self.wfile.write(b)
            except Exception:
                pass
        do_GET = do_POST

        def log_message(self, *a):
            pass

    class S(ThreadingHTTPServer):
        request_queue_size = 256
        daemon_threads = True

        def handle_error(self, *a):
            pass
    S(('127.0.0.1', port), H).serve_forever()


# ---------------------------------------------------------------------------------------------
# The feed's rules, simplified, as a pure function over a key-value store. The same function runs
# inside the feed process (on SQLite) and in the checker (on a dict), so "the live feed equals a
# clean replay of the log" can be tested. E-001 is the reference for the rules; this is a subset
# chosen so that order matters (read against delete, edit against delete, dial against arrival).

class SqlStore:
    def __init__(self, c): self.c = c
    def get(self, k, d=None):
        r = self.c.execute('select v from kv where k=?', (k,)).fetchone()
        return json.loads(r[0]) if r else d
    def put(self, k, v): self.c.execute('insert or replace into kv values(?,?)', (k, json.dumps(v, sort_keys=True)))
    def rm(self, k): self.c.execute('delete from kv where k=?', (k,))
    def scan(self, p):
        return [(k, json.loads(v)) for k, v in self.c.execute('select k,v from kv where k>=? and k<? order by k', (p, p + '￿'))]
    def dump(self): return dict(self.c.execute('select k,v from kv'))


class DictStore:
    def __init__(self): self.d = {}
    def get(self, k, d=None): return json.loads(self.d[k]) if k in self.d else d
    def put(self, k, v): self.d[k] = json.dumps(v, sort_keys=True)
    def rm(self, k): self.d.pop(k, None)
    def scan(self, p): return [(k, json.loads(self.d[k])) for k in sorted(self.d) if k.startswith(p)]
    def dump(self): return dict(self.d)


RANK = {'fyi': 1, 'needs': 2, 'focus': 3}


def ck_of(m):
    return 't:' + m['thread'] if m.get('thread') else m['conv']


def judge(st, m, p, part):
    """Where message m lands for reader p: (bucket or None, kept for them by a flag)."""
    if p == m['author'] or m.get('deleted'):
        return None, False
    conv, names, flag = m['conv'], p in m['mentions'], m['level'] == 'flag'
    dm = conv.startswith('dm:')
    if dm and p not in conv.split(':')[1:]:
        return None, False
    if m['level'] == 'info':
        return ('fyi' if (names or dm or p in part) else None), False
    if dm:
        return 'focus', flag
    if names:
        return ('focus' if flag else 'needs'), flag
    if m.get('thread'):
        return ('needs' if p in part else None), False
    dial = st.get(f'dial:{p}:{conv}', 'default')
    if flag and not m['mentions']:
        return (None if dial == 'muted' else 'needs'), False
    if dial == 'focus':
        return 'focus', False
    if dial == 'all':
        return 'fyi', False
    return None, False


def recompute(row):
    un = [i for i in row['items'] if not i['seen']]
    row['unread'] = bool(un)
    pool = un or row['items']
    row['bucket'] = max((i['b'] for i in pool), key=RANK.get) if pool else None


def withdraw(row, out, p):
    for name in row['due']:
        out.append(dict(kind='withdrawn', name=name, person=p))
    row['due'] = []


def add_item(st, out, p, ck, item, n):
    key = f'row:{p}:{ck}'
    row = st.get(key) or dict(items=[], unread=False, bucket=None, kept=False, due=[])
    oldb = row['bucket'] if row['unread'] else None
    row['items'].append(item)
    recompute(row)
    if not item['seen'] and item['b'] in ('needs', 'focus'):
        name = None
        if not row['due']:
            name = f'{p}/{ck}/{n}'                 # one notification per row (D-012)
        elif row['bucket'] == 'focus' and oldb != 'focus':
            name = f'{p}/{ck}/{n}/focus'           # once more when it moves up into Focus
        if name:
            row['due'].append(name)
            out.append(dict(kind='due', name=name, person=p))
    st.put(key, row)


def retract(st, out, p, ck, pred):
    key = f'row:{p}:{ck}'
    row = st.get(key)
    if not row:
        return
    keep = []
    for i in row['items']:
        if pred(i):
            if i['seen'] or row['kept']:           # a marker only where attention was spent (D-004)
                i['marker'], i['flag'] = True, False
                keep.append(i)
        else:
            keep.append(i)
    row['items'] = keep
    recompute(row)
    if not row['items']:
        withdraw(row, out, p)
        st.rm(key)
        return
    if not row['unread']:
        withdraw(row, out, p)
    st.put(key, row)


def mark_read(st, out, p, ck, n, release=False):
    st.put(f'read:{p}:{ck}', n)
    key = f'row:{p}:{ck}'
    row = st.get(key)
    if row:
        for i in row['items']:
            i['seen'] = True
            if release:
                i['flag'] = False
        recompute(row)
        withdraw(row, out, p)
        st.put(key, row)


def fold(st, n, kind, b, out):
    """Apply change number n to the feed's state; append notification notes to out."""
    if kind == 'message.created':
        m = b
        st.put('msg:' + m['id'], m)
        ck = ck_of(m)
        part = []
        if m.get('thread'):
            part = st.get('part:' + m['thread'])
            if part is None:
                root = st.get('msg:' + m['thread'])
                part = [root['author']] if root else []
        has = []
        for p in PEOPLE:
            bkt, flag = judge(st, m, p, part)
            if bkt:
                seen = st.get('watch:' + p) == ck  # arrived in front of them (D-015, rule 6)
                add_item(st, out, p, ck, dict(id=m['id'], msg=m['id'], n=n, b=bkt, seen=seen, flag=flag, marker=False), n)
                has.append(p)
        st.put('has:' + m['id'], has)
        if m.get('thread') and m['author'] not in part:
            st.put('part:' + m['thread'], part + [m['author']])
        elif m.get('thread'):
            st.put('part:' + m['thread'], part)
        mark_read(st, out, m['author'], ck, n, release=True)   # acting implies read
    elif kind == 'message.changed':
        old = st.get('msg:' + b['id'])
        if old is None:
            return
        m = b
        st.put('msg:' + m['id'], m)
        ck = ck_of(m)
        has = st.get('has:' + m['id'], [])
        who = set(old['mentions']) ^ set(m['mentions'])
        if old['level'] != m['level']:
            who |= set(has) | set(m['mentions'])
        part = st.get('part:' + m['thread'], []) if m.get('thread') else []
        for p in sorted(who):
            bkt, flag = judge(st, m, p, part)
            if p in has and not bkt:
                retract(st, out, p, ck, lambda i: i['id'] == m['id'])
                has.remove(p)
            elif p in has:
                row = st.get(f'row:{p}:{ck}')
                if row:
                    for i in row['items']:
                        if i['id'] == m['id'] and not i['marker']:
                            i['b'], i['flag'] = bkt, flag
                    recompute(row)
                    st.put(f'row:{p}:{ck}', row)
            elif bkt:                              # named by an edit: as if new (D-004)
                seen = st.get('watch:' + p) == ck
                add_item(st, out, p, ck, dict(id=m['id'], msg=m['id'], n=n, b=bkt, seen=seen, flag=flag, marker=False), n)
                has.append(p)
        st.put('has:' + m['id'], has)
        o, nw, a = set(map(tuple, old['reactions'])), set(map(tuple, m['reactions'])), m['author']
        for r, e in sorted(nw - o):
            if r != a:
                answer = r in m['mentions'] or (m['conv'].startswith('dm:') and r in m['conv'].split(':')[1:])
                seen = st.get('watch:' + a) == ck
                add_item(st, out, a, ck, dict(id=f"{m['id']}:r:{r}:{e}", msg=None, rmsg=m['id'], n=n,
                                              b='needs' if answer else 'fyi', seen=seen, flag=False, marker=False), n)
            mark_read(st, out, r, ck, n, release=True)
        for r, e in sorted(o - nw):
            retract(st, out, a, ck, lambda i, x=f"{m['id']}:r:{r}:{e}": i['id'] == x)
    elif kind == 'message.deleted':
        old = st.get('msg:' + b['id'])
        if old is None:
            return
        old['deleted'], old['version'], old['text'] = True, b['version'], ''
        st.put('msg:' + b['id'], old)
        ck = ck_of(old)
        for p in st.get('has:' + b['id'], []):
            retract(st, out, p, ck, lambda i: i['id'] == b['id'])
        retract(st, out, old['author'], ck, lambda i: i.get('rmsg') == b['id'])
        st.rm('has:' + b['id'])
    elif kind == 'reader.read':
        mark_read(st, out, b['person'], b['ck'], b['upto'])
    elif kind == 'reader.watch':
        st.put('watch:' + b['person'], b.get('ck'))
    elif kind == 'reader.dial':
        p = b['person']
        st.put(f"dial:{p}:{b['channel']}", b['level'])
        for key, row in st.scan(f'row:{p}:'):
            if key.split(':', 2)[2] != b['channel']:
                continue
            keep = []
            for i in row['items']:
                if i['seen'] or not i['msg'] or i['marker']:
                    keep.append(i)                  # read rows are never moved (D-013)
                    continue
                m = st.get('msg:' + i['msg'])
                bkt, flag = judge(st, m, p, [])
                if bkt:
                    i['b'], i['flag'] = bkt, flag
                    keep.append(i)
            row['items'] = keep
            recompute(row)
            if not row['items']:
                withdraw(row, out, p)
                st.rm(key)
            else:
                if not row['unread']:
                    withdraw(row, out, p)
                st.put(key, row)
    elif kind in ('reader.keep', 'reader.done'):
        key = f"row:{b['person']}:{b['ck']}"
        row = st.get(key)
        if row:
            row['kept'] = kind == 'reader.keep'
            st.put(key, row)


# ---------------------------------------------------------------------------------------------
# Core: the doors, the truth, and the change log written in the same transaction.

def core(a):
    c = db(a.dir + '/core.db')
    L = threading.Lock()
    c.executescript('''create table if not exists actions(id text primary key);
        create table if not exists messages(id text primary key, body text);
        create table if not exists change_log(number integer primary key autoincrement, kind text, body text, saved_at real);''')
    crash = float(os.environ.get('CRASH', '0'))
    busy = [0.0]

    def ticker():                                     # the log carries the clock: "it is now t, nothing earlier is missing"
        n = 0
        while True:
            time.sleep(0.1)
            with L:
                if clock() - busy[0] < HOLD + 1.5:
                    n += 1
                    c.execute('insert into change_log(kind,body,saved_at) values(?,?,?)', ('tick', json.dumps({'act': f'tick{n}-{uuid.uuid4().hex[:8]}'}), stamp()))
    threading.Thread(target=ticker, daemon=True).start()

    stamp_last = [c.execute('select coalesce(max(saved_at),0) from change_log').fetchone()[0]]

    def stamp():                                      # log stamps never go backwards, whatever the machine's clock does
        stamp_last[0] = max(stamp_last[0], clock())
        return stamp_last[0]

    def log(kind, body, act):
        c.execute('insert into change_log(kind,body,saved_at) values(?,?,?)', (kind, json.dumps(dict(body, act=act)), stamp()))

    def act(o):
        if c.execute('select 1 from actions where id=?', (o['id'],)).fetchone():
            return None, {'dup': True}              # a retry with the same ID changes nothing
        c.execute('insert into actions values(?)', (o['id'],))
        k = o['kind']
        if k == 'post':
            m = dict(id=o['msg'], version=1, conv=o['conv'], thread=o.get('thread'), author=o['author'], text=o['text'],
                     mentions=o['mentions'], level=o.get('level'), reactions=[], deleted=False)
            c.execute('insert into messages values(?,?)', (m['id'], json.dumps(m)))
            return ('message.created', m), {}
        if k in ('edit', 'delete', 'react'):
            r = c.execute('select body from messages where id=?', (o['msg'],)).fetchone()
            m = json.loads(r[0]) if r else None
            if not m or m['deleted']:
                return None, {'noop': True}
            ev = None
            if k == 'edit':
                m.update(text=o['text'], mentions=o['mentions'], level=o.get('level'))
                ev = 'message.changed'
            elif k == 'delete':
                m.update(deleted=True, text='', reactions=[])
                ev = 'message.deleted'
            else:
                pair = [o['person'], o['emoji']]
                if o['on'] and pair not in m['reactions']:
                    m['reactions'].append(pair)
                    ev = 'message.changed'
                elif not o['on'] and pair in m['reactions']:
                    m['reactions'].remove(pair)
                    ev = 'message.changed'
            if not ev:
                return None, {'noop': True}
            m['version'] += 1
            c.execute('update messages set body=? where id=?', (json.dumps(m), m['id']))
            body = m if ev == 'message.changed' else dict(id=m['id'], version=m['version'])
            return (ev, body), {}
        body = {f: v for f, v in o.items() if f not in ('id', 'kind')}
        if k == 'read':
            body['upto'] = c.execute('select coalesce(max(number),0) from change_log').fetchone()[0]
        return ('reader.' + k, body), {}

    def h(path, o):
        u = urllib.parse.urlparse(path)
        if u.path == '/ping':
            return 200, {}
        with L:
            if u.path == '/head':
                return 200, {'head': c.execute('select coalesce(max(number),0) from change_log').fetchone()[0]}
            if u.path == '/act':
                c.execute('begin immediate')
                try:
                    ev, out = act(o)
                    if ev and 'split_tx' in BREAK:   # the fault the outbox exists to prevent
                        c.execute('commit')
                        if random.random() < crash:
                            os._exit(1)
                        c.execute('begin immediate')
                    if ev:
                        log(ev[0], ev[1], o['id'])
                        busy[0] = clock()
                    c.execute('commit')
                    return 200, out
                except Exception:
                    c.execute('rollback')
                    raise
        return 404, {}
    serve(a.port, h)


# ---------------------------------------------------------------------------------------------
# A delivery worker: one per receiver, with its own place in the log.

class Worker(threading.Thread):
    def __init__(self, src, table, name, url, ddb, prefix=None, strict=False, extra=None):
        super().__init__(daemon=True)
        self.src, self.table, self.name, self.url, self.ddb = src, table, name, url, ddb
        self.prefix, self.strict, self.extra = prefix, strict, extra

    def run(self):
        src, d = db(self.src), db(self.ddb)
        d.execute('create table if not exists cursors(name text primary key, place int, last_sent int)')
        d.execute('create table if not exists parked(name text, number int, payload text, reason text)')
        d.execute('insert or ignore into cursors values(?,0,0)', (self.name,))
        setc = lambda p, l: d.execute('update cursors set place=?, last_sent=? where name=?', (p, l, self.name))
        getc = lambda: d.execute('select place,last_sent from cursors where name=?', (self.name,)).fetchone()
        refused_run, lasthb = 0, 0.0
        while True:
            place, last = getc()
            t = clock()
            src.execute('begin')
            rows = src.execute(f'select number,kind,body,saved_at from {self.table} where number>? order by number limit 200', (place,)).fetchall()
            ext = self.extra(src) if self.extra else {}
            src.execute('commit')
            if not rows:
                if clock() - lasthb > 0.1:       # "you have everything up to this moment"
                    wm = ext.pop('wm', t - 0.05)
                    st, out = call(self.url + '/heartbeat', dict(ext, through=last, wm=wm))
                    if st == 409:
                        setc(out['at'], out['at'])
                    lasthb = clock()
                time.sleep(0.01)
                continue
            ext.pop('wm', None)
            for number, kind, body, at in rows:
                if getc() != (place, last):          # an operator (or a test) moved the place
                    break
                if self.prefix and not kind.startswith(self.prefix):
                    place = number
                    setc(place, last)
                    continue
                payload = dict(ext, number=number, prev=last, kind=kind, body=json.loads(body), at=at)
                tries, back, rewind, parked = 0, 0.05, None, False
                while True:
                    st, out = call(self.url + '/changes', payload)
                    if st == 200:
                        refused_run = 0
                        break
                    if st == 409:                    # the receiver says where it really is
                        rewind = out['at']
                        break
                    if st == 422 and not self.strict:
                        tries += 1
                        if tries >= 3:
                            if refused_run >= 2:     # refusing everything: an outage, park nothing more
                                tries = 0
                            else:
                                d.execute('insert into parked values(?,?,?,?)', (self.name, number, json.dumps(payload), json.dumps(out)))
                                refused_run += 1
                                parked = True
                                break
                    time.sleep(back * (0.5 + random.random()))
                    back = min(back * 2, 1.0)
                if rewind is not None:
                    setc(rewind, rewind)
                    break
                place = number
                last = last if parked else number
                setc(place, last)


def delivery(a):
    ws = [Worker(a.dir + '/core.db', 'change_log', 'search', a.search_url, a.dir + '/delivery.db', prefix='message.'),
          Worker(a.dir + '/core.db', 'change_log', 'feed', a.feed_url, a.dir + '/delivery.db', strict=True)]
    for w in ws:
        w.start()
    for w in ws:
        w.join()


# ---------------------------------------------------------------------------------------------
# Search stand-in: newest version wins, a delete is remembered.

def search(a):
    c = db(a.dir + '/search.db')
    L = threading.Lock()
    c.executescript('''create table if not exists docs(id text primary key, version int, text text, deleted int);
        create table if not exists applied(number int primary key, t real);''')
    ctl = {'slow': 0, 'refuse': None}
    guard = 'search_noguard' not in BREAK

    def h(path, o):
        if path == '/ping':
            return 200, {}
        if path == '/control':
            ctl.update(o)
            return 200, {}
        if path == '/heartbeat':
            return 200, {}
        if ctl['slow']:
            time.sleep(ctl['slow'])
        m = o['body']
        if ctl['refuse'] in ('*', m['id']):
            return 422, {'reason': 'told to refuse'}
        with L:
            r = c.execute('select version from docs where id=?', (m['id'],)).fetchone()
            if guard and r and r[0] >= m['version']:
                return 200, {'had': True}
            dele = o['kind'] == 'message.deleted'
            c.execute('begin immediate')
            c.execute('insert or replace into docs values(?,?,?,?)', (m['id'], m['version'], None if dele else m['text'], int(dele)))
            c.execute('insert or ignore into applied values(?,?)', (o['number'], clock()))
            c.execute('commit')
        return 200, {}
    serve(a.port, h)


# ---------------------------------------------------------------------------------------------
# Feed: takes only the change that follows the last one it applied; rows, its place and its own
# log of notification notes are saved in one step.

def feed(a):
    path = a.dir + '/feed.db'
    c = db(path)
    L = threading.Lock()
    c.executescript('''create table if not exists kv(k text primary key, v text);
        create table if not exists meta(k text primary key, v);
        create table if not exists feedlog(number integer primary key autoincrement, kind text, body text, saved_at real);
        create table if not exists applied(number int primary key, t real);
        create table if not exists fences(scope text primary key, number int, reason text);
        create table if not exists set_aside(number int primary key, scope text);
        create table if not exists aside_msgs(id text primary key, scope text);''')
    if not c.execute("select 1 from meta where k='edition'").fetchone():
        c.executemany('insert into meta values(?,?)', [('edition', uuid.uuid4().hex), ('last', 0), ('wm', 0.0)])
    st = SqlStore(c)
    strict = 'feed_nonstrict' not in BREAK
    slow = float(os.environ.get('SLOW_FEED', '0'))
    meta = lambda k: c.execute('select v from meta where k=?', (k,)).fetchone()[0]
    # A change the feed cannot apply is fenced off with what it touches (one conversation, or one
    # reader); everything else carries on. Past FENCE_LIMIT fences it is not one bad change: stop.
    poison = set(filter(None, os.environ.get('FEED_POISON', '').split(',')))   # faults planted by the test
    FENCE_LIMIT = 5
    fencing = 'feed_stops' not in BREAK

    def scope_of(o):
        k, b = o['kind'], o['body']
        if k == 'message.created':
            return 'conv:' + ck_of(b)
        if k in ('message.changed', 'message.deleted'):
            m = st.get('msg:' + b['id'])
            if m:
                return 'conv:' + ck_of(m)
            r = c.execute('select scope from aside_msgs where id=?', (b['id'],)).fetchone()
            return r[0] if r else None
        if k.startswith('reader.'):
            return 'reader:' + b['person']
        return None

    def planted(o):
        k, b = o['kind'], o['body']
        return ((k == 'message.changed' and ('all' in poison or 'msg:' + b['id'] in poison))
                or (k == 'reader.dial' and 'reader:' + b['person'] in poison))

    def h(path, o):
        if path == '/ping':
            return 200, {}
        with L:
            last = meta('last')
            if path == '/heartbeat':
                if o['through'] != last:
                    return (409, {'at': last}) if strict else (200, {})
                return 200, {}
            if o['number'] <= last:
                return 200, {'had': True}
            if strict and o['prev'] != last:
                return 409, {'at': last}
            if slow:
                time.sleep(slow)
            sc = scope_of(o)
            fenced = bool(sc and c.execute('select 1 from fences where scope=?', (sc,)).fetchone())
            b = o['body']
            if not fenced and o['kind'] == 'message.created' and b.get('thread') and \
                    c.execute('select 1 from aside_msgs where id=?', (b['thread'],)).fetchone():
                fenced = None                          # a reply under a message that was set aside: fence its thread too
            c.execute('begin immediate')
            try:
                if fenced or fenced is None:
                    raise RuntimeError('inside a fence')
                if planted(o):
                    raise RuntimeError('fault planted by the test')
                out = []
                fold(st, o['number'], o['kind'], o['body'], out)
                if o['kind'] == 'tick':
                    out.append(dict(kind='tick', name=''))
                shut = {x[0][7:] for x in c.execute("select scope from fences where scope like 'reader:%'")}
                out = [e for e in out if not (e['kind'] == 'due' and e.get('person') in shut)]
                for e in out:
                    c.execute('insert into feedlog(kind,body,saved_at) values(?,?,?)', (e['kind'], json.dumps(e), o['at']))
                c.execute("update meta set v=? where k='last'", (o['number'],))
                c.execute("update meta set v=max(v,?) where k='wm'", (o['at'],))
                c.execute('insert or ignore into applied values(?,?)', (o['number'], clock()))
                c.execute('commit')
            except Exception as ex:
                c.execute('rollback')
                if not fencing or sc is None:
                    raise
                if not fenced and c.execute('select count(*) from fences').fetchone()[0] >= FENCE_LIMIT:
                    raise                              # too many: stop rather than fence everything
                c.execute('begin immediate')
                c.execute('insert or ignore into fences values(?,?,?)', (sc, o['number'], repr(ex)))
                c.execute('insert into set_aside values(?,?)', (o['number'], sc))
                if o['kind'] == 'message.created':
                    c.execute('insert or ignore into aside_msgs values(?,?)', (b['id'], sc))
                c.execute("update meta set v=? where k='last'", (o['number'],))
                c.execute("update meta set v=max(v,?) where k='wm'", (o['at'],))
                c.execute('insert or ignore into applied values(?,?)', (o['number'], clock()))
                c.execute('commit')
        return 200, {}

    def extra(src):
        return dict(edition=src.execute("select v from meta where k='edition'").fetchone()[0],
                    wm=src.execute("select v from meta where k='wm'").fetchone()[0])
    Worker(path, 'feedlog', 'notifier', a.notif_url, a.dir + '/feed-delivery.db', strict=True, extra=extra).start()
    serve(a.port, h)


# ---------------------------------------------------------------------------------------------
# Notification sender stand-in. Three ways of holding before sending, to compare:
#   event     : send once HOLD has passed since the thing happened (the naive reading)
#   arrival   : send once HOLD has passed since the note arrived here
#   watermark : send once a note or clock tick stamped later than the event + HOLD has arrived through the log

def notifier(a):
    c = db(a.dir + '/notif.db')
    L = threading.Lock()
    c.executescript('''create table if not exists n(name text primary key, person text, due_at real, arrived_at real, withdrawn_at real, state text);
        create table if not exists tray_log(name text, sent_at real, due_at real);
        create table if not exists meta(k text primary key, v);''')
    c.executemany('insert or ignore into meta values(?,?)', [('edition', ''), ('last', 0), ('wm', 0.0)])
    mode = os.environ.get('NOTIFY_MODE', 'watermark')
    dedupe = 'notif_nodedupe' not in BREAK
    meta = lambda k: c.execute('select v from meta where k=?', (k,)).fetchone()[0]
    setm = lambda k, v: c.execute('update meta set v=? where k=?', (v, k))

    def send(name, due_at):
        c.execute("update n set state='sent' where name=?", (name,))
        c.execute('insert into tray_log values(?,?,?)', (name, clock(), due_at))

    def h(path, o):
        if path == '/ping':
            return 200, {}
        with L:
            if o.get('edition') != meta('edition'):  # the feed was rebuilt: its log starts again
                setm('edition', o['edition']); setm('last', 0); setm('wm', 0.0)
            last = meta('last')
            if path == '/heartbeat':
                if o['through'] != last:
                    return 409, {'at': last}
                return 200, {}
            if o['number'] <= last:
                return 200, {'had': True}
            if o['prev'] != last:
                return 409, {'at': last}
            e, now = o['body'], clock()
            c.execute('begin immediate')
            setm('wm', max(meta('wm'), o['at']))
            r = c.execute('select state,due_at from n where name=?', (e['name'],)).fetchone()
            if e['kind'] == 'tick':
                pass
            elif e['kind'] == 'due':
                if r is None:
                    c.execute("insert into n values(?,?,?,?,null,'pending')", (e['name'], e['person'], o['at'], now))
                    if mode == 'event' and now >= o['at'] + HOLD:
                        send(e['name'], o['at'])
                elif not dedupe and r[0] != 'pending':
                    c.execute('insert into tray_log values(?,?,?)', (e['name'], now, o['at']))   # a sender with no memory
            else:
                if r is None:
                    c.execute("insert into n values(?,?,null,?,?,'cancelled')", (e['name'], e['person'], now, o['at']))
                elif r[0] == 'pending' and mode == 'watermark' and o['at'] >= r[1] + HOLD:
                    send(e['name'], r[1])             # by the log's own clock the hold had already ended
                    c.execute("update n set state='sent_then_withdrawn', withdrawn_at=? where name=?", (o['at'], e['name']))
                elif r[0] == 'pending':
                    c.execute("update n set state='cancelled', withdrawn_at=? where name=?", (o['at'], e['name']))
                elif r[0] == 'sent':
                    c.execute("update n set state='sent_then_withdrawn', withdrawn_at=? where name=?", (o['at'], e['name']))
            setm('last', o['number'])
            c.execute('commit')
        return 200, {}

    def timer():
        while True:
            time.sleep(0.02)
            with L:
                wm, now = meta('wm'), clock()
                for name, due_at, arrived in c.execute("select name,due_at,arrived_at from n where state='pending'").fetchall():
                    ready = {'event': now >= due_at + HOLD, 'arrival': now >= arrived + HOLD, 'watermark': wm >= due_at + HOLD}[mode]
                    if ready:
                        c.execute('begin immediate')
                        send(name, due_at)
                        c.execute('commit')
    threading.Thread(target=timer, daemon=True).start()
    serve(a.port, h)


# ---------------------------------------------------------------------------------------------
# A troublemaker between a worker and a receiver: repeats, loses answers, delivers copies late.

def proxy(a):
    rng = random.Random(a.seed)
    L = threading.Lock()

    def h(path, o):
        if path == '/ping':
            return 200, {}
        url = a.target + path
        if path != '/changes':
            return call(url, o)
        with L:
            r, delay = rng.random(), rng.uniform(0.05, 1.5)
        if r < 0.15:                                  # this copy turns up late; the worker will retry
            threading.Timer(delay, lambda: call(url, o)).start()
            return 503, {}
        if r < 0.30:                                  # delivered, but the answer is lost
            call(url, o)
            return 503, {}
        if r < 0.50:                                  # delivered twice
            call(url, o)
        return call(url, o)
    serve(a.port, h)


if __name__ == '__main__':
    ap = argparse.ArgumentParser()
    ap.add_argument('role')
    ap.add_argument('--dir')
    ap.add_argument('--port', type=int, default=0)
    ap.add_argument('--search-url')
    ap.add_argument('--feed-url')
    ap.add_argument('--notif-url')
    ap.add_argument('--target')
    ap.add_argument('--seed', type=int, default=1)
    a = ap.parse_args()
    {'core': core, 'delivery': delivery, 'search': search, 'feed': feed, 'notifier': notifier, 'proxy': proxy}[a.role](a)
