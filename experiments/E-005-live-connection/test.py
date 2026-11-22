"""E-005: does a screen that follows the numbered-update contract end equal to a fresh load after
connections are cut, refused, or lose single updates? Driven in a real headless browser."""
import json, subprocess, sys, time, urllib.request
from playwright.sync_api import sync_playwright

PORT, ROWS, NOTES = 8765, [], []
BASE = f'http://127.0.0.1:{PORT}'


def ctl(**o):
    req = urllib.request.Request(BASE + '/control', data=json.dumps(o).encode(), headers={'Content-Type': 'application/json'})
    urllib.request.urlopen(req, timeout=5).read()


def truth():
    return json.loads(urllib.request.urlopen(BASE + '/snapshot', timeout=5).read())


def settle(page, timeout=20):
    """Wait until nothing new is produced and the screen has stopped changing; return (screen, truth)."""
    t0, prev = time.time(), None
    while time.time() - t0 < timeout:
        tr, sc = truth(), page.evaluate('snap()')
        cur = (tr['n'], sc['last'])
        if prev == cur and (sc['last'] == tr['n'] or time.time() - t0 > 6):
            return sc, tr
        prev = cur
        time.sleep(0.7)
    return page.evaluate('snap()'), truth()


def row(scenario, name, ok, detail, expected=True):
    ROWS.append((scenario, name, ok, expected, detail))


def equal(scenario, page, expected=True):
    sc, tr = settle(page)
    diff = sum(1 for k in set(sc['state']) | set(tr['state']) if sc['state'].get(k) != tr['state'].get(k))
    st = sc['stats']
    row(scenario, 'the screen equals a fresh load', diff == 0 and sc['last'] == tr['n'], 
        f"{diff} of {len(tr['state'])} items differ; screen at {sc['last']}, server at {tr['n']}; "
        f"reconnected {st['reconnects']}, asked again {st['resumes']}, reloaded {st['reloads']}, repeats ignored {st['repeatsIgnored']}", expected)
    return sc


def pct(v, p):
    v = sorted(v)
    return v[min(len(v) - 1, int(len(v) * p))] if v else float('nan')


def main():
    srv = subprocess.Popen([sys.executable, 'server.py', str(PORT)])
    for _ in range(100):
        try:
            truth()
            break
        except Exception:
            time.sleep(0.1)
    with sync_playwright() as p:
        b = p.chromium.launch()
        ctx = b.new_context()
        page = ctx.new_page()
        page.goto(BASE + '/')

        S = 'steady: 2,000 updates at 400 a second'
        ctl(burst=2000, rate=400)
        sc = equal(S, page)
        lat = sc['stats']['lat']
        NOTES.append((S, 'server to screen', f'median {pct(lat, 0.5):.0f} ms, 19 in 20 within {pct(lat, 0.95):.0f} ms, worst {max(lat):.0f} ms'))

        S = 'the connection cut five times mid-burst'
        ctl(burst=2000, rate=400)
        for _ in range(5):
            time.sleep(0.8)
            ctl(cut=True)
        equal(S, page)

        S = 'the server refuses connections for 2 s mid-burst'
        ctl(burst=1600, rate=400)
        time.sleep(1)
        ctl(refuse=2)
        equal(S, page)

        S = 'one update in twenty lost silently on a live connection'
        ctl(drop=0.05)
        ctl(burst=2000, rate=400)
        time.sleep(5.5)
        ctl(drop=0)
        equal(S, page)

        S = 'the last update of a burst lost silently, then silence'
        ctl(burst=50, rate=400)
        time.sleep(0.11)
        ctl(drop=1.0)
        time.sleep(0.2)
        ctl(drop=0)
        equal(S, page)

        S = 'away while 800 updates pass (more than the 500 kept ready)'
        before = page.evaluate('snap()')['stats']['reloads']
        page.evaluate('pause()')
        ctl(burst=800, rate=800)
        time.sleep(1.5)
        page.evaluate('resume()')
        sc = equal(S, page)
        row(S, 'it reloaded once instead of replaying', sc['stats']['reloads'] - before == 1, f"{sc['stats']['reloads'] - before} reloads")

        for mode, what, fault in [('careless', 'a screen that applies whatever arrives, one update in twenty lost', 'drop'),
                                  ('noresume', 'a screen that reconnects without saying where it was, connection cut', 'cut')]:
            S = 'broken on purpose: ' + what
            bad = ctx.new_page()
            bad.goto(BASE + '/?mode=' + mode)
            time.sleep(0.5)
            if fault == 'drop':
                ctl(drop=0.05)
            ctl(burst=1200, rate=400)
            if fault == 'cut':
                for _ in range(3):
                    time.sleep(0.8)
                    ctl(cut=True)
            time.sleep(3.5)
            ctl(drop=0)
            equal(S, bad, expected=False)
            bad.close()
        equal('the good screen, after sharing those faults', page)

        S = 'twelve tabs open at once on plain http://127.0.0.1'
        tabs = [ctx.new_page() for _ in range(12)]
        for t in tabs:
            t.goto(BASE + '/')
        ctl(burst=400, rate=400)
        time.sleep(3)
        tr = truth()
        ok = sum(1 for t in tabs if t.evaluate('snap()')['last'] == tr['n'])
        row(S, 'WebSocket: every tab is connected and current', ok == 12, f'{ok} of 12')
        for t in tabs:
            t.close()
        tabs = [ctx.new_page() for _ in range(12)]
        for t in tabs:
            try:
                t.goto(BASE + '/sse.html', timeout=4000)
            except Exception:
                pass
        time.sleep(3)
        conn = 0
        for t in tabs:
            try:
                conn += bool(t.evaluate('window.connected === true'))
            except Exception:
                pass
        row(S, "the browser's one-way stream: every tab is connected", conn == 12, f'{conn} of 12 (the browser allows six per site)', expected=False)
        b.close()
    srv.kill()
    lines, unexpected = [], 0
    for sc_, name, ok, exp, detail in ROWS:
        verdict = ('holds' if ok else 'BROKEN') if exp else ('caught' if not ok else 'NOT CAUGHT')
        if ok != exp:
            unexpected += 1
            verdict += ' (unexpected)'
        lines.append(f'{verdict:22} | {sc_} | {name} | {detail}')
    lines += [''] + [f'measured | {a} | {b_} | {c}' for a, b_, c in NOTES]
    lines += ['', f'{len(ROWS)} checks, {unexpected} unexpected. Heartbeat every 0.5 s (proposal: about 20 s).']
    print('\n'.join(lines))
    open('results.txt', 'w').write('\n'.join(lines) + '\n')
    sys.exit(1 if unexpected else 0)


if __name__ == '__main__':
    main()
