"""E-004, second part: many teams in one database. Does a numbered log per team keep one team from
holding up another, and is anything stepped over?"""
import random, threading, time
from test import connect, line, OUT

TEAMS = 8


def reset():
    with connect(True) as c:
        c.execute('drop table if exists change_log, counter, team_counter')
        c.execute('create table change_log(team int, number bigint, body text, primary key(team, number))')
        c.execute('create table counter(n bigint); insert into counter values(0)')
        c.execute('create table team_counter(team int primary key, n bigint)')
        c.execute('insert into team_counter select t, 0 from generate_series(0, %s) t', (TEAMS - 1,))


def number_one_log(cur, team):        # one numbering for everybody
    return cur.execute('update counter set n = n + 1 returning n').fetchone()[0]


def number_per_team(cur, team):       # each team has its own numbering
    return cur.execute('update team_counter set n = n + 1 where team = %s returning n', (team,)).fetchone()[0]


def run(numbering, each=200, slow_team=None, slow=0.3, hold_open=0.003):
    """Two writers per team. Optionally one team keeps every save open for a long time."""
    reset()
    took, written, seen, stop = {t: [] for t in range(TEAMS)}, {t: set() for t in range(TEAMS)}, {t: set() for t in range(TEAMS)}, threading.Event()
    lock = threading.Lock()

    def writer(team, seed):
        rng = random.Random(seed)
        n_saves = 12 if team == slow_team else each
        with connect() as c:
            for _ in range(n_saves):
                t0 = time.monotonic()
                with c.transaction():
                    cur = c.cursor()
                    n = numbering(cur, team)
                    cur.execute('insert into change_log values(%s, %s, %s)', (team, n, 'x'))
                    time.sleep(slow if team == slow_team else rng.uniform(0, hold_open))
                with lock:
                    took[team].append((time.monotonic() - t0) * 1000)
                    written[team].add(n)

    def follower(team):
        place = 0
        with connect(True) as c:
            while True:
                rows = [r[0] for r in c.execute('select number from change_log where team = %s and number > %s order by number', (team, place))]
                if rows:
                    seen[team].update(rows)
                    place = rows[-1]
                elif stop.is_set():
                    break
                time.sleep(0.001)
    per_team = numbering is number_per_team
    fs = [threading.Thread(target=follower, args=(t,)) for t in (range(TEAMS) if per_team else [])]
    ws = [threading.Thread(target=writer, args=(t, t * 10 + k)) for t in range(TEAMS) for k in range(2)]
    t0 = time.monotonic()
    for th in fs + ws:
        th.start()
    for th in ws:
        th.join()
    total = time.monotonic() - t0
    time.sleep(0.3)
    stop.set()
    for th in fs:
        th.join()
    others = sorted(x for t in range(TEAMS) if t != slow_team for x in took[t])
    skipped = sum(len(written[t] - seen[t]) for t in range(TEAMS)) if per_team else None
    return dict(median=others[len(others) // 2], p95=others[int(len(others) * 0.95)], worst=others[-1], skipped=skipped,
                rate=sum(len(v) for v in took.values()) / total)


if __name__ == '__main__':
    line('')
    line(f'{TEAMS} teams in one database, two writers each.')
    r = run(number_per_team)
    line(f'  a log per team, a follower per team: changes never seen: {r["skipped"]} of {TEAMS * 2 * 200}')
    line('')
    line('One team keeps every save open for 300 ms. How long does a save take in the OTHER teams?')
    for name, fn in [('one log for everybody', number_one_log), ('a log per team', number_per_team)]:
        r = run(fn, slow_team=0)
        line(f'  {name:24} | median {r["median"]:6.1f} ms, 19 in 20 within {r["p95"]:6.1f} ms, worst {r["worst"]:6.1f} ms')
    line('')
    line('All writers saving as fast as they can, nothing held open:')
    for name, fn in [('one log for everybody', number_one_log), ('a log per team', number_per_team)]:
        r = run(fn, each=400, hold_open=0)
        line(f'  {name:24} | {r["rate"]:6.0f} changes a second')
    open('results.txt', 'a').write('\n'.join(OUT) + '\n')
