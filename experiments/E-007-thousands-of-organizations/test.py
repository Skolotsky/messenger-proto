"""E-007: is the delivery design enough for thousands of organizations in one installation?

5,000 organizations in one PostgreSQL. Writers save changes the way Conversations would (a numbered log per
organization). Delivery workers find the organizations that have work with one question, take each with a
lock that other workers skip, and apply a share. Every writer and worker is its own process."""
import os, random, subprocess, sys, time
import psycopg

DSN = 'host=e007-pg user=postgres password=e007 dbname=postgres'
ORGS = 5000
OUT = []


def pg(autocommit=True):
    for _ in range(120):
        try:
            return psycopg.connect(DSN, autocommit=autocommit)
        except psycopg.OperationalError:
            time.sleep(0.5)
    raise SystemExit('PostgreSQL did not come up')


def say(s=''):
    print(s, flush=True)
    OUT.append(s)


def reset():
    c = pg()
    c.execute('drop table if exists change_log, org_counter, places, applied')
    c.execute('''create table change_log(org int, number bigint, saved_at double precision, primary key(org, number));
        create table org_counter(org int primary key, n bigint);
        create table places(org int primary key, last bigint);
        create table applied(org int, number bigint, t double precision, primary key(org, number))''')
    c.execute('insert into org_counter select g, 0 from generate_series(1, %s) g', (ORGS,))
    c.execute('insert into places select g, 0 from generate_series(1, %s) g', (ORGS,))
    return c


# ---- roles, each a process --------------------------------------------------------------------
def writer(seconds, rate, notify, seed):
    """Saves changes for random organizations: number and log entry in one transaction. rate 0 = flat out."""
    rng, c = random.Random(seed), pg(autocommit=False)
    end, n = time.monotonic() + seconds, 0
    while time.monotonic() < end:
        t = time.monotonic()
        org = rng.randint(2, ORGS)                    # organization 1 is kept for the backlog test
        with c.transaction():
            cur = c.cursor()
            num = cur.execute('update org_counter set n=n+1 where org=%s returning n', (org,)).fetchone()[0]
            cur.execute('insert into change_log values(%s,%s,extract(epoch from clock_timestamp()))', (org, num))
            if notify:
                cur.execute("select pg_notify('changes', %s)", (str(org),))
        n += 1
        if rate:
            time.sleep(max(0, 1 / rate - (time.monotonic() - t)))
    print(n)


def worker(seconds, share, cost_ms, naive):
    """Finds organizations with work by one question, takes each under a lock others skip, applies a share."""
    c = pg(autocommit=False)
    end, cost = time.monotonic() + seconds, cost_ms / 1000
    while time.monotonic() < end:
        if naive:                                     # the fault: ask every organization in turn, one by one
            orgs = list(range(1, ORGS + 1))
        else:
            orgs = [o for (o,) in c.execute('select c.org from org_counter c join places p using (org) where c.n > p.last limit 400').fetchall()]
            c.commit()
            random.shuffle(orgs)
        did = 0
        for org in orgs:
            with c.transaction():
                cur = c.cursor()
                r = cur.execute('select last from places where org=%s for update skip locked', (org,)).fetchone()
                if r is None:
                    continue                          # another worker has this organization right now
                rows = cur.execute('select number from change_log where org=%s and number>%s order by number limit %s', (org, r[0], share)).fetchall()
                if not rows:
                    continue
                if cost:
                    time.sleep(cost * len(rows))      # the work of applying, as in E-006
                cur.execute('insert into applied select %s, n, extract(epoch from clock_timestamp()) from unnest(%s::bigint[]) n', (org, [x[0] for x in rows]))
                cur.execute('update places set last=%s where org=%s', (rows[-1][0], org))
                did += len(rows)
        if not did:
            time.sleep(0.02)


def spawn(*args):
    return subprocess.Popen([sys.executable, 'test.py', '--role'] + [str(a) for a in args], stdout=subprocess.PIPE, text=True)


def pct(v, p):
    return v[min(len(v) - 1, int(len(v) * p))] if v else float('nan')


def latency(c, where='true'):
    v = sorted(x[0] for x in c.execute(f'select (a.t - l.saved_at) * 1000 from applied a join change_log l using (org, number) where {where}').fetchall())
    return f'median {pct(v, 0.5):.0f} ms, 19 in 20 within {pct(v, 0.95):.0f} ms, worst {v[-1]:.0f} ms' if v else 'no data'


def totals(c):
    saved = c.execute('select coalesce(sum(n),0) from org_counter').fetchone()[0]
    done = c.execute('select count(*) from applied').fetchone()[0]
    return saved, done


def drain(c, workers, share=40, cost=0.0, timeout=240):
    t0 = time.monotonic()
    ws = [spawn('worker', timeout, share, cost, 0) for _ in range(workers)]
    while time.monotonic() - t0 < timeout:
        saved, done = totals(c)
        if done == saved:
            break
        time.sleep(0.25)
    for w in ws:
        w.kill()
    return time.monotonic() - t0, totals(c)


def main():
    c = reset()
    say(f'{ORGS} organizations in one PostgreSQL 17, every writer and worker its own process.')

    say()
    say('1 · How fast can changes be saved? 16 writers flat out for 15 s, spread over all organizations:')
    for notify in (0, 1):
        reset()
        ws = [spawn('writer', 15, 0, notify, i) for i in range(16)]
        n = sum(int(w.communicate()[0]) for w in ws)
        say(f'    {"waking the workers through the database on every save (NOTIFY)" if notify else "without waking anybody":66} | {n / 15:6.0f} saves a second')

    say()
    say('2 · How fast can they be delivered? 120,000 waiting changes spread over all organizations, nothing else going on:')
    for workers in (1, 2, 4, 8):
        c = reset()
        c.execute('insert into change_log select 1 + (g %% %s), 1 + g / %s, extract(epoch from clock_timestamp()) from generate_series(0, 119999) g', (ORGS, ORGS))
        c.execute('update org_counter set n = 24')
        took, (saved, done) = drain(c, workers)
        say(f'    {workers} worker{"s" if workers > 1 else " "} | {done / took:6.0f} changes a second | delivered {done} of {saved}, each exactly once')

    say()
    say('3 · Steady traffic: 600 saves a second for 30 s over all organizations, each change costing a worker 1 ms to apply:')
    for workers in (1, 4):
        c = reset()
        ws = [spawn('worker', 45, 40, 1, 0) for _ in range(workers)]
        wr = [spawn('writer', 30, 75, 0, i) for i in range(8)]
        n = sum(int(w.communicate()[0]) for w in wr)
        time.sleep(8)
        saved, done = totals(c)
        for w in ws:
            w.kill()
        say(f'    {workers} worker{"s" if workers > 1 else " "} | saved {saved} ({n / 30:.0f} a second), delivered {done} | save to delivered: {latency(c)}')

    say()
    say('4 · One organization gets 50,000 changes at once while the others carry on at 400 saves a second (1 ms per change, 4 workers):')
    c = reset()
    ws = [spawn('worker', 60, 40, 1, 0) for _ in range(4)]
    wr = [spawn('writer', 30, 50, 0, i) for i in range(8)]
    time.sleep(5)
    c.execute('insert into change_log select 1, g, extract(epoch from clock_timestamp()) from generate_series(1, 50000) g')
    c.execute('update org_counter set n=50000 where org=1')
    for w in wr:
        w.communicate()
    say(f'    the other organizations, save to delivered: {latency(c, "org <> 1")}')
    t0 = time.monotonic()
    while totals(c)[0] != totals(c)[1] and time.monotonic() - t0 < 40:
        time.sleep(0.5)
    saved, done = totals(c)
    for w in ws:
        w.kill()
    say(f'    in the end delivered {done} of {saved}')

    say()
    say('5 · What looking for work costs when nothing is happening:')
    c = reset()
    t0 = time.monotonic()
    for _ in range(50):
        c.execute('select c.org from org_counter c join places p using (org) where c.n > p.last limit 400').fetchall()
    one = (time.monotonic() - t0) / 50 * 1000
    t0 = time.monotonic()
    for org in range(1, ORGS + 1):
        c.execute('select number from change_log where org=%s and number>%s order by number limit 40', (org, 0)).fetchall()
    each = (time.monotonic() - t0) * 1000
    say(f'    one question "which organizations have work?": {one:.1f} ms    asking each of {ORGS} organizations in turn: {each:.0f} ms a round')
    open('results.txt', 'w').write('\n'.join(OUT) + '\n')


if __name__ == '__main__':
    if len(sys.argv) > 1 and sys.argv[1] == '--role':
        a = sys.argv[3:]
        if sys.argv[2] == 'writer':
            writer(float(a[0]), float(a[1]), a[2] == '1', int(a[3]))
        else:
            worker(float(a[0]), int(a[1]), float(a[2]), a[3] == '1')
    else:
        main()
