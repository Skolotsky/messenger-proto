"""E-004: the change log on PostgreSQL. Can a reader that remembers "the highest number I have seen"
step over a change that was saved a moment late, and what does preventing it cost?"""
import json, random, statistics, threading, time
import psycopg

DSN = 'host=e004-pg user=postgres password=e004 dbname=postgres'
OUT = []


def connect(autocommit=False):
    for _ in range(60):
        try:
            return psycopg.connect(DSN, autocommit=autocommit)
        except psycopg.OperationalError:
            time.sleep(0.5)
    raise SystemExit('PostgreSQL did not come up')


def reset():
    with connect(True) as c:
        c.execute('drop table if exists change_log, counter; drop sequence if exists log_seq')
        c.execute('create sequence log_seq')
        c.execute('create table change_log(number bigint primary key, body text, saved_at timestamptz)')
        c.execute('create table counter(n bigint); insert into counter values(0)')


# How a writer gets the number for its change. All three run inside the writer's transaction.
def number_sequence(cur):            # the obvious way: a database sequence
    return cur.execute("select nextval('log_seq')").fetchone()[0]


def number_locked_sequence(cur):     # one writer at a time from here until its commit
    cur.execute('select pg_advisory_xact_lock(4242)')
    return cur.execute("select nextval('log_seq')").fetchone()[0]


def number_counter_row(cur):         # a one-row counter: the row lock lasts until commit
    return cur.execute('update counter set n = n + 1 returning n').fetchone()[0]


def burst(numbering, writers=16, each=250, hold_open=0.003):
    """Writers save changes at once; some keep the save open a little. A reader follows the log."""
    reset()
    seen, stop, written = [], threading.Event(), []
    lock = threading.Lock()

    def writer(seed):
        rng = random.Random(seed)
        with connect() as c:
            for _ in range(each):
                with c.transaction():
                    cur = c.cursor()
                    cur.execute("select 'the message itself is saved here'")
                    n = numbering(cur)                       # numbering comes last, just before the commit
                    cur.execute('insert into change_log values(%s, %s, clock_timestamp())', (n, 'x'))
                    if hold_open:
                        time.sleep(rng.uniform(0, hold_open))  # the save is still open
                with lock:
                    written.append(n)

    def reader():
        place = 0
        with connect(True) as c:
            while True:
                rows = [r[0] for r in c.execute('select number from change_log where number > %s order by number', (place,))]
                if rows:
                    seen.extend(rows)
                    place = rows[-1]
                elif stop.is_set():
                    break
                time.sleep(0.001)
    rd = threading.Thread(target=reader)
    rd.start()
    ws = [threading.Thread(target=writer, args=(i,)) for i in range(writers)]
    t0 = time.monotonic()
    for w in ws:
        w.start()
    for w in ws:
        w.join()
    took = time.monotonic() - t0
    time.sleep(0.3)
    stop.set()
    rd.join()
    skipped = len(set(written) - set(seen))
    gaps = max(written) - len(written)
    return skipped, len(written), gaps, len(written) / took


def wake_latency(n=400, rate=50):
    """Save a change and NOTIFY in the same transaction; a listener fetches what is new. Save to seen."""
    reset()
    lat, sent = [], {}
    done = threading.Event()

    def listener():
        place = 0
        with connect(True) as c:
            c.execute('listen changes')
            ready.set()
            while len(lat) < n:
                for _ in c.notifies(timeout=2, stop_after=1):
                    pass
                for (num,) in c.execute('select number from change_log where number > %s order by number', (place,)):
                    place = num
                    lat.append((time.monotonic() - sent[num]) * 1000)
                if done.is_set() and len(lat) >= len(sent):
                    break
    ready = threading.Event()
    th = threading.Thread(target=listener)
    th.start()
    ready.wait()
    with connect() as c:
        for i in range(n):
            with c.transaction():
                cur = c.cursor()
                num = number_locked_sequence(cur)
                cur.execute('insert into change_log values(%s, %s, clock_timestamp())', (num, 'x'))
                cur.execute('notify changes')
                sent[num] = time.monotonic()                # just before the commit
            time.sleep(1 / rate)
    done.set()
    th.join()
    lat.sort()
    return lat[len(lat) // 2], lat[int(len(lat) * 0.95)], lat[-1], len(lat)


def line(s):
    print(s, flush=True)
    OUT.append(s)


if __name__ == '__main__':
    with connect(True) as c:
        line(c.execute('select version()').fetchone()[0].split(',')[0])
    line('')
    line('A reader follows the log while 16 writers save 4,000 changes, some keeping the save open up to 3 ms:')
    for name, fn in [('a database sequence', number_sequence), ('a sequence, one writer at a time until commit', number_locked_sequence),
                     ('a one-row counter', number_counter_row)]:
        res = [burst(fn) for _ in range(3)]
        line(f'  {name:46} | changes the reader never saw: {[r[0] for r in res]} of {res[0][1]} | numbers left unused: {[r[2] for r in res]}')
    line('')
    line('Cost of numbering one writer at a time: 16 writers saving as fast as they can, nothing held open:')
    for name, fn in [('a database sequence', number_sequence), ('a sequence, one writer at a time until commit', number_locked_sequence),
                     ('a one-row counter', number_counter_row)]:
        r = burst(fn, hold_open=0)
        line(f'  {name:46} | {r[3]:6.0f} changes a second | never seen: {r[0]}')
    line('')
    p50, p95, worst, n = wake_latency()
    line(f'Save to "a worker has fetched it", woken by NOTIFY, {n} changes at 50 a second: median {p50:.1f} ms, 19 in 20 within {p95:.1f} ms, worst {worst:.1f} ms')
    open('results.txt', 'w').write('\n'.join(OUT) + '\n')
