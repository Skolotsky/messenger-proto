"""E-006: the same delivery promises with Apache Kafka in the middle.

PostgreSQL holds the truth: a numbered change log per organization, written in the same transaction as
the change. A relay publishes each organization's log to one Kafka topic, in order, keyed by organization.
Two receivers read the topic as consumer groups: one that insists on order (like the feed) and one that
only ignores repeats (like search). Throwaway code; it says nothing about the product's language."""
import json, os, random, signal, subprocess, sys, threading, time, uuid, zlib
import psycopg
from confluent_kafka import Consumer, Producer, TopicPartition
from confluent_kafka.admin import AdminClient, NewTopic

DSN = 'host=e006-pg user=postgres password=e006 dbname=postgres'
KAFKA = 'e006-kafka:9092'
PARTS = 4
lane = lambda org: zlib.crc32(org.encode()) % PARTS      # which lane an organization's changes travel in


def pg(autocommit=True):
    for _ in range(120):
        try:
            return psycopg.connect(DSN, autocommit=autocommit)
        except psycopg.OperationalError:
            time.sleep(0.5)
    raise SystemExit('PostgreSQL did not come up')


# ---------------------------------------------------------------------------------------------
def relay(topic):
    """Publishes each organization's log in order, and notes how far it got only after Kafka confirmed."""
    c = pg()
    p = Producer({'bootstrap.servers': KAFKA, 'enable.idempotence': True, 'acks': 'all', 'linger.ms': 5,
                  'message.timeout.ms': 4000, 'socket.timeout.ms': 3000})
    nokey = os.environ.get('BREAK') == 'nokey'
    trusting = os.environ.get('BREAK') == 'trusting'  # the fault found on the first run: "sending finished" taken for "confirmed"
    failed = []
    report = lambda err, msg: failed.append(1) if err else None
    while True:
        busy = False
        for org, place in c.execute('select org, n from relay_place order by org').fetchall():
            rows = c.execute('select number, saved_at from change_log where org=%s and number>%s order by number limit 500', (org, place)).fetchall()
            if not rows:
                continue
            busy = True
            for number, at in rows:
                rec = json.dumps({'org': org, 'number': number, 'previous': number - 1, 'at': at})
                if nokey:
                    p.produce(topic, rec, partition=random.randrange(PARTS), on_delivery=report)   # the fault: no key, any lane
                else:
                    p.produce(topic, rec, key=org, partition=lane(org), on_delivery=report)
            left = p.flush(6)
            if left > 0 or (failed and not trusting):
                # Kafka did not confirm every one: note nothing, drop what is queued, publish this stretch again.
                # "Sending finished" is not confirmation: a change Kafka gave up on is reported as failed, not as left over.
                p.purge()
                p.flush(1)
                failed.clear()
                time.sleep(0.5)
                continue
            failed.clear()
            c.execute('update relay_place set n=%s where org=%s', (rows[-1][0], org))
        if not busy:
            time.sleep(0.01)


def receiver(topic, group, strict):
    """strict: takes only the next number of each organization (like the feed). Otherwise: ignores repeats (like search)."""
    c = pg(autocommit=False)
    stopped = set(filter(None, os.environ.get('STOPPED', '').split(',')))
    only = os.environ.get('ONLY')                     # a rebuild of one organization, read from the start
    cost = float(os.environ.get('APPLY_MS', '0')) / 1000
    k = Consumer({'bootstrap.servers': KAFKA, 'group.id': group, 'enable.auto.commit': False, 'auto.offset.reset': 'earliest',
                  'session.timeout.ms': 6000})
    k.subscribe([topic])
    if os.environ.get('FAIR') == '1' and strict:
        return fair_receiver(c, k, topic, stopped, cost)
    while True:
        try:
            msgs = k.consume(500, timeout=0.2)
        except Exception:
            time.sleep(0.5)
            continue
        good = [m for m in msgs if not m.error()]
        if not good:
            continue
        last = {}
        with c.transaction():
            cur = c.cursor()
            for m in good:
                v = json.loads(m.value())
                org, n = v['org'], v['number']
                if org in stopped or (only and org != only):
                    continue                          # set aside: the lane keeps moving
                if not strict:
                    cur.execute('insert into search_seen values(%s,%s,extract(epoch from clock_timestamp())) on conflict do nothing', (org, n))
                    continue
                if org not in last:
                    cur.execute('insert into feed_state values(%s,0) on conflict do nothing', (org,))
                    last[org] = cur.execute('select last from feed_state where org=%s for update', (org,)).fetchone()[0]
                if n <= last[org]:
                    continue                          # a repeat
                if n != last[org] + 1:
                    cur.execute('insert into feed_errors values(%s,%s,%s)', (org, n, last[org]))   # out of order
                    continue
                if cost:
                    time.sleep(cost)
                cur.execute('insert into feed_applied values(%s,%s,extract(epoch from clock_timestamp()))', (org, n))
                last[org] = n
            for org, n in last.items():
                cur.execute('update feed_state set last=%s where org=%s', (n, org))
        try:
            k.commit(asynchronous=False)              # only after the rows are saved: a crash before this repeats, never skips
        except Exception:
            pass                                      # Kafka is away: the rows are saved; the place is noted with a later batch


def direct_worker():
    """No Kafka: the design of D-005, D-026 and D-028. A worker reads each organization's log in PostgreSQL from
    its own place and takes the organizations in turn, a share at a time. FAIR=0 works through one organization's
    whole backlog before the next, which is the fault the in-turn rule exists to prevent."""
    c = pg(autocommit=False)
    cost = float(os.environ.get('APPLY_MS', '0')) / 1000
    share = 40 if os.environ.get('FAIR', '1') == '1' else 1000000
    while True:
        orgs = [o for (o,) in c.execute('select org from org_counter order by org').fetchall()]
        c.commit()
        busy = False
        for org in orgs:
            with c.transaction():
                cur = c.cursor()
                cur.execute('insert into feed_state values(%s,0) on conflict do nothing', (org,))
                last = cur.execute('select last from feed_state where org=%s for update', (org,)).fetchone()[0]
                rows = cur.execute('select number from change_log where org=%s and number>%s order by number limit %s', (org, last, share)).fetchall()
                for (n,) in rows:
                    if cost:
                        time.sleep(cost)
                    cur.execute('insert into feed_applied values(%s,%s,extract(epoch from clock_timestamp()))', (org, n))
                    last = n
                cur.execute('update feed_state set last=%s where org=%s', (last, org))
            busy |= bool(rows)
        if not busy:
            time.sleep(0.01)


def fair_receiver(c, k, topic, stopped, cost, share=40, cap=60000):
    """Takes the organizations in turn: what was read waits in a queue per organization, each gets a share
    per round, and the place noted in Kafka is the oldest record of each lane that is not yet applied."""
    from collections import deque
    queues, newest = {}, {}                           # org -> deque of (lane, offset, number); lane -> newest offset read
    while True:
        waiting = sum(len(q) for q in queues.values())
        if waiting < cap:
            try:
                msgs = k.consume(500, timeout=0.02 if waiting else 0.2)
            except Exception:
                msgs = []
                time.sleep(0.5)
            for m in msgs:
                if m.error():
                    continue
                v = json.loads(m.value())
                newest[m.partition()] = m.offset()
                if v['org'] not in stopped:
                    queues.setdefault(v['org'], deque()).append((m.partition(), m.offset(), v['number']))
        if not any(queues.values()):
            continue
        with c.transaction():
            cur = c.cursor()
            for org, q in queues.items():
                if not q:
                    continue
                cur.execute('insert into feed_state values(%s,0) on conflict do nothing', (org,))
                last = cur.execute('select last from feed_state where org=%s for update', (org,)).fetchone()[0]
                for _ in range(min(share, len(q))):   # this organization's share of the round
                    lane_, off, n = q.popleft()
                    if n <= last:
                        continue
                    if n != last + 1:
                        cur.execute('insert into feed_errors values(%s,%s,%s)', (org, n, last))
                        continue
                    if cost:
                        time.sleep(cost)
                    cur.execute('insert into feed_applied values(%s,%s,extract(epoch from clock_timestamp()))', (org, n))
                    last = n
                cur.execute('update feed_state set last=%s where org=%s', (last, org))
        oldest = {}
        for q in queues.values():
            if q:                                     # offsets rise within a queue, so its head is its oldest
                lane_, off, n = q[0]
                oldest[lane_] = min(off, oldest.get(lane_, off))
        place = [TopicPartition(topic, ln, oldest.get(ln, newest[ln] + 1)) for ln in newest]
        try:
            k.commit(offsets=place, asynchronous=False)
        except Exception:
            pass


# ---------------------------------------------------------------------------------------------
class World:
    def __init__(self, name, orgs, env=None):
        self.topic, self.orgs, self.procs, self.env = f'changes-{name}-{uuid.uuid4().hex[:6]}', orgs, {}, dict(os.environ, **(env or {}))
        self.c = pg()
        self.c.execute('drop table if exists change_log, org_counter, relay_place, feed_state, feed_applied, feed_errors, search_seen')
        self.c.execute('''create table change_log(org text, number bigint, saved_at double precision, primary key(org, number));
            create table org_counter(org text primary key, n bigint); create table relay_place(org text primary key, n bigint);
            create table feed_state(org text primary key, last bigint); create table feed_applied(org text, number bigint, t double precision, primary key(org, number));
            create table feed_errors(org text, number bigint, last bigint); create table search_seen(org text, number bigint, t double precision, primary key(org, number))''')
        for o in orgs:
            self.c.execute('insert into org_counter values(%s,0)', (o,))
            self.c.execute('insert into relay_place values(%s,0)', (o,))
        admin = AdminClient({'bootstrap.servers': KAFKA})
        for _ in range(60):
            try:
                fs = admin.create_topics([NewTopic(self.topic, PARTS, 1)])
                fs[self.topic].result(10)
                break
            except Exception:
                time.sleep(1)
        self.acked, self.lock, self.save_ms = 0, threading.Lock(), []

    def start(self, role, env=None, group=None):
        args = {'relay': ['relay', self.topic], 'feed': ['receiver', self.topic, group or 'feed-' + self.topic, '1'],
                'search': ['receiver', self.topic, 'search-' + self.topic, '0'], 'direct': ['direct', self.topic]}[role.split(':')[0]]
        self.procs[role] = subprocess.Popen([sys.executable, 'test.py', '--role'] + args, env=dict(self.env, **(env or {})))

    def kill(self, role):
        p = self.procs.pop(role)
        p.send_signal(signal.SIGKILL)
        p.wait()

    def save(self, conn, org):
        t0 = time.monotonic()
        with conn.transaction():
            cur = conn.cursor()
            n = cur.execute('update org_counter set n=n+1 where org=%s returning n', (org,)).fetchone()[0]
            cur.execute('insert into change_log values(%s,%s,extract(epoch from clock_timestamp()))', (org, n))
        with self.lock:
            self.acked += 1
            self.save_ms.append((time.monotonic(), (time.monotonic() - t0) * 1000))

    def writers(self, seconds, rate, orgs=None):
        """One writer per organization at `rate` saves a second."""
        def w(org):
            conn = pg(autocommit=False)
            end = time.monotonic() + seconds
            while time.monotonic() < end:
                t = time.monotonic()
                self.save(conn, org)
                time.sleep(max(0, 1 / rate - (time.monotonic() - t)))
        ts = [threading.Thread(target=w, args=(o,)) for o in (orgs or self.orgs)]
        for t in ts:
            t.start()
        return ts

    def level(self, timeout=90, skip=(), feed_only=False):
        t0 = time.monotonic()
        while time.monotonic() - t0 < timeout:
            heads = dict(self.c.execute('select org, n from org_counter').fetchall())
            feed = dict(self.c.execute('select org, last from feed_state').fetchall())
            srch = dict(self.c.execute('select org, count(*) from search_seen group by org').fetchall())
            if all(feed.get(o, 0) == h and (feed_only or srch.get(o, 0) == h) for o, h in heads.items() if o not in skip):
                return time.monotonic() - t0
            time.sleep(0.2)
        return None

    def check(self, skip=()):
        heads = dict(self.c.execute('select org, n from org_counter').fetchall())
        fa = dict(self.c.execute('select org, count(*) from feed_applied group by org').fetchall())
        ss = dict(self.c.execute('select org, count(*) from search_seen group by org').fetchall())
        errs = self.c.execute('select count(*) from feed_errors').fetchone()[0]
        total = sum(h for o, h in heads.items())
        bad = [o for o, h in heads.items() if o not in skip and (fa.get(o, 0) != h or ss.get(o, 0) != h)]
        return dict(ok=not bad and total == self.acked, total=total, acked=self.acked, out_of_order=errs,
                    feed=sum(fa.values()), search=sum(ss.values()), behind=bad)

    def latency(self, table, org=None, t0=None, t1=None):
        q = f'select (r.t - l.saved_at) * 1000 from {table} r join change_log l using (org, number) where true'
        a = []
        if org:
            q += ' and org = %s'; a.append(org)
        if t0:
            q += ' and l.saved_at between %s and %s'; a += [t0, t1]
        return sorted(x[0] for x in self.c.execute(q, a).fetchall())

    def now(self):
        return self.c.execute('select extract(epoch from clock_timestamp())::float8').fetchone()[0]

    def down(self):
        for r in list(self.procs):
            self.kill(r)


def kafka(cmd):
    open('.ack', 'w').close()
    open('.ctl', 'w').write(cmd)
    for _ in range(300):
        if open('.ack').read().strip() == cmd:
            return
        time.sleep(0.1)
    raise SystemExit('the host did not ' + cmd + ' Kafka')


def pct(v, p):
    return v[min(len(v) - 1, int(len(v) * p))] if v else float('nan')


def fmt(v):
    return f'median {pct(v, 0.5):.0f} ms, 19 in 20 within {pct(v, 0.95):.0f} ms, worst {v[-1]:.0f} ms ({len(v)} changes)' if v else 'no data'


OUT = []


def say(s=''):
    print(s, flush=True)
    OUT.append(s)


def verdict(name, r, expect_ok=True, extra=''):
    word = ('holds' if r['ok'] else 'BROKEN') if expect_ok else ('caught' if not r['ok'] else 'NOT CAUGHT')
    say(f"{word:10} | {name} | saved {r['acked']}, in the logs {r['total']}, the ordered receiver applied {r['feed']}, the other saw {r['search']}; {r['out_of_order']} arrivals out of order were refused{extra}")
    return r['ok'] == expect_ok


def setup():
    orgs = [f'org{i}' for i in range(40)]
    A = orgs[0]
    B = next(o for o in orgs[1:] if lane(o) == lane(A))          # shares a lane with A
    C = next(o for o in orgs[1:] if lane(o) != lane(A))          # travels in another lane
    return A, B, C


def world(name, orgs, env=None):
    w = World(name, orgs, env)
    for r in ('relay', 'feed', 'search'):
        w.start(r)
    time.sleep(8)                                                 # the consumer groups settle
    return w


def s_healthy(A, B, C):
    w = world('healthy', [A, B, C])
    for t in w.writers(20, 100): t.join()
    w.level()
    ok = verdict('healthy: three organizations at 100 saves a second each for 20 s', w.check())
    say(f'           save to applied by the ordered receiver: {fmt(w.latency("feed_applied"))}')
    say(f'           save to seen by the other receiver:      {fmt(w.latency("search_seen"))}')
    w.down()
    return ok


def s_relay_killed(A, B, C):
    w = world('relaykill', [A, B, C])
    ts = w.writers(15, 100)
    for _ in range(5):
        time.sleep(2); w.kill('relay'); w.start('relay')
    for t in ts: t.join()
    w.level()
    ok = verdict('the relay killed five times mid-burst', w.check())
    w.down()
    return ok


def s_kafka_down(A, B, C, brk=None):
    w = world('kafkadown', [A, B, C], env={'BREAK': brk} if brk else None)
    ts = w.writers(24, 100)
    time.sleep(5)
    t0 = time.monotonic(); kafka('stop'); time.sleep(8); kafka('start'); t1 = time.monotonic()
    for t in ts: t.join()
    lv = w.level(40 if brk else 120)
    if brk:
        ok = verdict('broken on purpose: a relay that takes "sending finished" for "confirmed", Kafka stopped for 8 s', w.check(), expect_ok=False)
    else:
        during = sorted(ms for at, ms in w.save_ms if t0 <= at <= t1)
        ok = verdict('Kafka stopped for 8 s mid-burst', w.check(), extra=f'; level {lv:.0f} s after the last save' if lv is not None else '; NOT LEVEL')
        say(f'           while Kafka was down, {len(during)} saves were accepted; a save took median {pct(during, 0.5):.0f} ms, worst {during[-1]:.0f} ms')
    w.down()
    return ok


def s_receiver_killed(A, B, C):
    w = world('receiverkill', [A, B, C])
    ts = w.writers(16, 100)
    for _ in range(3):
        time.sleep(3); w.kill('feed'); w.start('feed')
    for t in ts: t.join()
    w.level(120)
    ok = verdict('the ordered receiver killed three times mid-burst', w.check())
    w.down()
    return ok


def s_other_stopped(A, B, C):
    w = world('otherstopped', [A, B, C])
    ts = w.writers(20, 100)
    time.sleep(5)
    s0 = w.now(); w.kill('search'); time.sleep(8); s1 = w.now(); w.start('search')
    for t in ts: t.join()
    w.level(120)
    ok = verdict('the other receiver stopped for 8 s mid-burst', w.check())
    say(f'           the ordered receiver meanwhile: {fmt(w.latency("feed_applied", t0=s0, t1=s1))}')
    w.down()
    return ok


def s_backlog(A, B, C, fair):
    how = 'a receiver that takes organizations in turn' if fair else 'a plain receiver'
    w = world('lanes', [A, B, C], env={'APPLY_MS': '1', 'FAIR': '1' if fair else '0'})
    ts = w.writers(40, 20, [B, C])
    time.sleep(4)
    t0 = w.now()
    w.c.execute("insert into change_log select %s, g, extract(epoch from clock_timestamp()) from generate_series(1, 20000) g", (A,))
    w.c.execute('update org_counter set n=20000 where org=%s', (A,)); w.acked += 20000
    for t in ts: t.join()
    t1 = w.now()
    w.level(180)
    ok = verdict(f'{how}: {A} gets 20,000 changes at once while {B} and {C} carry on', w.check())
    say(f'           {B} (same lane as the backlog):  {fmt(w.latency("feed_applied", B, t0, t1))}')
    say(f'           {C} (another lane):              {fmt(w.latency("feed_applied", C, t0, t1))}')
    if fair:                                          # and killed while the queues are full: nothing lost
        pass
    w.down()
    return ok


def s_fair_killed(A, B, C):
    w = world('fairkill', [A, B, C], env={'APPLY_MS': '1', 'FAIR': '1'})
    ts = w.writers(30, 20, [B, C])
    time.sleep(3)
    w.c.execute("insert into change_log select %s, g, extract(epoch from clock_timestamp()) from generate_series(1, 20000) g", (A,))
    w.c.execute('update org_counter set n=20000 where org=%s', (A,)); w.acked += 20000
    for _ in range(3):
        time.sleep(5); w.kill('feed'); w.start('feed')
    for t in ts: t.join()
    w.level(180)
    ok = verdict('the in-turn receiver killed three times with its queues full', w.check())
    w.down()
    return ok


def s_stopped_org(A, B, C):
    w = world('stopped', [A, B, C], env={'APPLY_MS': '1', 'STOPPED': A})
    ts = w.writers(20, 20, [B, C])
    time.sleep(4)
    t0 = w.now()
    w.c.execute("insert into change_log select %s, g, extract(epoch from clock_timestamp()) from generate_series(1, 20000) g", (A,))
    w.c.execute('update org_counter set n=20000 where org=%s', (A,)); w.acked += 20000
    for t in ts: t.join()
    t1 = w.now()
    w.level(120, skip=(A,))
    r = w.check(skip=(A,))
    ok = not r['behind']
    say(f"{'holds' if ok else 'BROKEN':10} | {A} is stopped at the ordered receiver and gets 20,000 changes; its lane-mate must not wait | {r['out_of_order']} arrivals out of order were refused")
    say(f'           {B} (same lane as the stopped one): {fmt(w.latency("feed_applied", B, t0, t1))}')
    w.kill('feed')                                    # then rebuild one organization from the start of the topic
    w.c.execute('delete from feed_state where org=%s', (B,)); w.c.execute('delete from feed_applied where org=%s', (B,))
    head = w.c.execute('select n from org_counter where org=%s', (B,)).fetchone()[0]
    t0 = time.monotonic()
    w.start('feed:rebuild', env={'ONLY': B, 'APPLY_MS': '0', 'STOPPED': ''}, group='rebuild-' + uuid.uuid4().hex[:6])
    while (w.c.execute('select last from feed_state where org=%s', (B,)).fetchone() or [0])[0] != head and time.monotonic() - t0 < 120:
        time.sleep(0.1)
    cnt = w.c.execute('select count(*) from feed_applied where org=%s', (B,)).fetchone()[0]
    say(f"{'holds' if cnt == head else 'BROKEN':10} | {B} wiped at the ordered receiver and rebuilt by reading the topic from the start | {cnt} of {head} changes back in {time.monotonic() - t0:.1f} s, reading past {w.acked} records")
    w.down()
    return ok and cnt == head


def s_no_key(A, B, C):
    w = world('nokey', [A, B, C], env={'BREAK': 'nokey'})
    for t in w.writers(10, 100): t.join()
    time.sleep(6)
    ok = verdict('broken on purpose: the relay publishes without the organization as key (any lane)', w.check(), expect_ok=False)
    w.down()
    return ok


def s_backlog_direct(A, B, C, fair):
    how = 'no Kafka, our own worker taking organizations in turn' if fair else 'no Kafka, broken on purpose: a worker that finishes one organization before the next'
    w = World('direct', [A, B, C], env={'APPLY_MS': '1', 'FAIR': '1' if fair else '0'})
    w.start('direct')
    time.sleep(1)
    ts = w.writers(40, 20, [B, C])
    time.sleep(4)
    t0 = w.now()
    w.c.execute("insert into change_log select %s, g, extract(epoch from clock_timestamp()) from generate_series(1, 20000) g", (A,))
    w.c.execute('update org_counter set n=20000 where org=%s', (A,)); w.acked += 20000
    for t in ts: t.join()
    t1 = w.now()
    lv = w.level(180, feed_only=True)
    lat = w.latency('feed_applied', B, t0, t1) + w.latency('feed_applied', C, t0, t1)
    lat.sort()
    within = pct(lat, 0.95) < 2000 and lv is not None
    word = ('holds' if within else 'BROKEN') if fair else ('caught' if not within else 'NOT CAUGHT')
    applied = w.c.execute('select count(*) from feed_applied').fetchone()[0]
    say(f"{word:10} | {how}: {A} gets 20,000 changes at once; the others' changes must be applied within 2 s (19 in 20) | saved {w.acked}, applied {applied}")
    say(f'           {B}: {fmt(w.latency("feed_applied", B, t0, t1))}')
    say(f'           {C}: {fmt(w.latency("feed_applied", C, t0, t1))}')
    w.down()
    return within == fair and applied == w.acked


SCENARIOS = [('healthy', s_healthy), ('relay-killed', s_relay_killed), ('kafka-down', s_kafka_down), ('receiver-killed', s_receiver_killed),
             ('other-stopped', s_other_stopped), ('backlog-plain', lambda *a: s_backlog(*a, fair=False)), ('backlog-in-turn', lambda *a: s_backlog(*a, fair=True)),
             ('in-turn-killed', s_fair_killed), ('stopped-org', s_stopped_org),
             ('trusting-relay', lambda *a: s_kafka_down(*a, brk='trusting')), ('no-key', s_no_key),
             ('direct-in-turn', lambda *a: s_backlog_direct(*a, fair=True)), ('direct-one-at-a-time', lambda *a: s_backlog_direct(*a, fair=False))]


def main():
    A, B, C = setup()
    names = sys.argv[1:] or [n for n, _ in SCENARIOS]
    say(f'Lanes: {A} and {B} share one; {C} travels in another. In the backlog scenarios applying a change costs the ordered receiver 1 ms.')
    ok = True
    for n, fn in SCENARIOS:
        if n in names:
            ok &= fn(A, B, C)
    say()
    say('all as expected' if ok else 'SOMETHING UNEXPECTED')
    if not sys.argv[1:]:
        open('results.txt', 'w').write('\n'.join(OUT) + '\n')
    sys.exit(0 if ok else 1)


if __name__ == '__main__':
    if len(sys.argv) > 1 and sys.argv[1] == '--role':
        if sys.argv[2] == 'relay':
            relay(sys.argv[3])
        elif sys.argv[2] == 'direct':
            direct_worker()
        else:
            receiver(sys.argv[3], sys.argv[4], sys.argv[5] == '1')
    else:
        main()
