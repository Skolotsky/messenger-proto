"""E-007, second part: the redesign after the independent check, at 300,000 organizations.

One row per organization holds the head of its log, the receiver's place, since when work is waiting, and a
short claim. An index covers only the organizations that have work, so finding work does not read the idle
ones. A worker claims a few organizations for a few seconds (a short transaction), delivers outside any
transaction, then notes the new place. A claim that runs out is taken over by another worker, so a change can
be delivered twice; the receiver ignores repeats, which is what makes that safe."""
import os, random, signal, subprocess, sys, time
import psycopg

DSN = 'host=e007-pg user=postgres password=e007 dbname=postgres'
ORGS = int(os.environ.get('ORGS', '300000'))
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


NOW = "extract(epoch from clock_timestamp())"


def reset(orgs=ORGS):
    c = pg()
    c.execute('drop table if exists change_log, org_log, delivered, applied')
    c.execute(f'''create table change_log(org int, number bigint, saved_at double precision, primary key(org, number));
        create table org_log(org int primary key, head bigint not null default 0, place bigint not null default 0,
                             waiting_since double precision, claim_until double precision not null default 0, tick_due double precision);
        create table delivered(org int, number bigint);                     -- every delivery attempt, repeats included
        create table applied(org int, number bigint, t double precision, primary key(org, number))''')
    c.execute('insert into org_log(org) select g from generate_series(1, %s) g', (orgs,))
    c.execute('create index org_work on org_log(waiting_since) where head > place')      # only organizations with work
    c.execute('create index org_fresh on org_log(waiting_since) where head > place and head - place <= 40')   # ... that one turn will clear
    c.execute('create index org_behind on org_log(waiting_since) where head - place > 40')                    # ... that are far behind
    c.execute('create index org_tick on org_log(tick_due) where tick_due is not null')   # only organizations awaiting a clock tick
    c.execute('analyze org_log')
    return c


def writer(seconds, rate, seed, lo, hi):
    rng, c = random.Random(seed), pg(autocommit=False)
    end, n = time.monotonic() + seconds, 0
    while time.monotonic() < end:
        t = time.monotonic()
        org = rng.randint(lo, hi)
        with c.transaction():
            cur = c.cursor()
            num = cur.execute(f'''update org_log set head = head + 1, tick_due = {NOW} + 6,
                                  waiting_since = case when head = place then {NOW} else waiting_since end
                                  where org = %s returning head''', (org,)).fetchone()[0]
            cur.execute(f'insert into change_log values(%s,%s,{NOW})', (org, num))
        n += 1
        if rate:
            time.sleep(max(0, 1 / rate - (time.monotonic() - t)))
    print(n)


def worker(seconds, share, cost_ms, lease):
    c = pg()
    end, cost = time.monotonic() + seconds, cost_ms / 1000
    while time.monotonic() < end:
        claim = lambda where, n: c.execute(f'''update org_log set claim_until = {NOW} + %s where org in (
                                   select org from org_log where {where} and claim_until < {NOW}
                                   order by waiting_since limit {n} for update skip locked)
                               returning org, place''', (lease,)).fetchall()
        if os.environ.get('LANES', '1') == '1':
            # Two lines: organizations one turn will clear go first, so fresh traffic never queues behind a
            # catch-up; every round also takes some that are far behind, so a backlog is never starved.
            claimed = claim('head > place and head - place <= 40', 6) + claim('head - place > 40', 2)
        else:
            claimed = claim('head > place', 8)
        if not claimed:
            time.sleep(0.02)
            continue
        for org, place in claimed:
            rows = [r[0] for r in c.execute('select number from change_log where org=%s and number>%s order by number limit %s', (org, place, share)).fetchall()]
            if rows:
                if cost:
                    time.sleep(cost * len(rows))                      # the call to the receiver
                c.execute('insert into delivered select %s, n from unnest(%s::bigint[]) n', (org, rows))
                c.execute(f'insert into applied select %s, n, {NOW} from unnest(%s::bigint[]) n on conflict do nothing', (org, rows))   # the receiver ignores a repeat
                place = rows[-1]
            c.execute(f'''update org_log set place = greatest(place, %s), claim_until = 0,
                          waiting_since = case when head > greatest(place, %s) then {NOW} else null end where org = %s''', (place, place, org))


def spawn(*args):
    return subprocess.Popen([sys.executable, 'test2.py', '--role'] + [str(a) for a in args], stdout=subprocess.PIPE, text=True)


def pct(v, p):
    return v[min(len(v) - 1, int(len(v) * p))] if v else float('nan')


def latency(c, where='true'):
    v = sorted(x[0] for x in c.execute(f'select (a.t - l.saved_at) * 1000 from applied a join change_log l using (org, number) where {where}').fetchall())
    return f'median {pct(v, 0.5):.0f} ms, 19 in 20 within {pct(v, 0.95):.0f} ms, worst {v[-1]:.0f} ms' if v else 'no data'


def state(c):
    saved = c.execute('select coalesce(sum(head),0) from org_log where head > 0').fetchone()[0]
    done = c.execute('select count(*) from applied').fetchone()[0]
    attempts = c.execute('select count(*) from delivered').fetchone()[0]
    return saved, done, attempts


def wait_level(c, timeout):
    t0 = time.monotonic()
    while time.monotonic() - t0 < timeout:
        saved, done, _ = state(c)
        if saved == done:
            return time.monotonic() - t0
        time.sleep(0.5)
    return None


def timed(c, sql, n=200):
    t0 = time.monotonic()
    for _ in range(n):
        c.execute(sql).fetchall()
    return (time.monotonic() - t0) / n * 1000


def part4(lanes):
    os.environ['LANES'] = '1' if lanes else '0'
    how = 'fresh traffic first, the backlog alongside' if lanes else 'strictly whoever has waited longest'
    c = reset()
    big = ORGS                                                         # the largest ID, which the first design starved
    c.execute(f'insert into change_log select o, g, {NOW} from generate_series(100001, 102000) o, generate_series(1, 100) g')
    c.execute(f'update org_log set head = 100, waiting_since = {NOW} where org between 100001 and 102000')
    c.execute(f'insert into change_log select %s, g, {NOW} from generate_series(1, 50000) g', (big,))
    c.execute(f'update org_log set head = 50000, waiting_since = {NOW} where org = %s', (big,))
    t0 = time.monotonic()
    ws = [spawn('worker', 200, 40, 1, 5) for _ in range(4)]
    wr = [spawn('writer', 30, 50, i, 1, 90000) for i in range(8)]
    for w in wr:
        w.communicate()
    say(f'    {how}:')
    say(f'        organizations with ordinary traffic, save to applied: {latency(c, "org <= 90000")}')
    lv = wait_level(c, 170)
    saved, done, attempts = state(c)
    last_small = c.execute('select max(t) - min(t) from applied where org between 100001 and 102000').fetchone()[0]
    for w in ws:
        w.kill()
    say(f'        the 2,000 with 100 waiting each all done after {(last_small or -1):.0f} s; everything, the 50,000 included, after {time.monotonic() - t0:.0f} s | saved {saved}, applied {done}')


def main():
    say('E-007, second part: the redesign, measured.')
    say()
    say('1 · What looking for work costs when nothing is happening, by number of organizations in the database:')
    for n in (5000, 50000, 300000):
        c = reset(n)
        scan = timed(c, 'select c.org from org_log c where c.head > c.place + 0 limit 400', 20)   # "+ 0" keeps the index out: the first design
        idx = timed(c, f'select org from org_log where head > place and claim_until < {NOW} order by waiting_since limit 8')
        tick = timed(c, f'select org from org_log where tick_due is not null and tick_due < {NOW} limit 50')
        say(f'    {n:7} organizations | first design (reads every row): {scan:6.1f} ms | with an index of organizations that have work: {idx:5.2f} ms | "who needs a clock tick": {tick:5.2f} ms')

    say()
    say(f'2 · {ORGS} organizations, 600 saves a second for 30 s spread over all of them, 1 ms of work per change:')
    for workers in (1, 4):
        c = reset()
        ws = [spawn('worker', 50, 40, 1, 5) for _ in range(workers)]
        wr = [spawn('writer', 30, 75, i, 1, ORGS) for i in range(8)]
        for w in wr:
            w.communicate()
        lv = wait_level(c, 15)
        saved, done, attempts = state(c)
        for w in ws:
            w.kill()
        say(f'    {workers} worker{"s" if workers > 1 else " "} | saved {saved}, applied {done}, delivery attempts {attempts} | save to applied: {latency(c)}')

    say()
    say('3 · The same with four workers, two of them killed mid-way (their claims run out after 5 s and are taken over):')
    c = reset()
    ws = [spawn('worker', 70, 40, 1, 5) for _ in range(4)]
    wr = [spawn('writer', 30, 75, i, 1, ORGS) for i in range(8)]
    time.sleep(10)
    for w in ws[:2]:
        w.send_signal(signal.SIGKILL)
    for w in wr:
        w.communicate()
    lv = wait_level(c, 30)
    saved, done, attempts = state(c)
    for w in ws[2:]:
        w.kill()
    say(f'    saved {saved}, applied {done} (each once: the receiver ignores a repeat), delivery attempts {attempts}, so {attempts - done} repeats | level {"%.0f s after the last save" % lv if lv is not None else "NOT REACHED"}')

    say()
    say('4 · A wave of catching up: 2,000 organizations each have 100 changes waiting, one more has 50,000, and 400 saves a second go on elsewhere (4 workers):')
    part4(False)
    part4(True)
    open('results-redesign.txt', 'w').write('\n'.join(OUT) + '\n')


if __name__ == '__main__':
    if len(sys.argv) > 1 and sys.argv[1] == '--role':
        a = sys.argv[3:]
        if sys.argv[2] == 'writer':
            writer(float(a[0]), float(a[1]), int(a[2]), int(a[3]), int(a[4]))
        else:
            worker(float(a[0]), int(a[1]), float(a[2]), float(a[3]))
    else:
        main()
