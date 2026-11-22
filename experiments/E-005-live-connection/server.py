"""E-005 test server: a numbered stream of updates over WebSocket (and, for one comparison, the browser's
one-way stream), with switches to cut connections, refuse them, and lose single updates silently."""
import asyncio, json, random, sys, time
from aiohttp import web

LIMIT = 500          # further behind than this: "too far behind, reload"
HEARTBEAT = 0.5      # seconds (proposal: about 20 s)


class S:
    log, state, queues, conns = [], {}, set(), set()
    drop, refuse_until, rng, nid = 0.0, 0.0, random.Random(1), 0


def produce():
    r, keys = S.rng.random(), list(S.state)
    if r < 0.5 or len(keys) < 5:
        S.nid += 1
        u = {'kind': 'set', 'key': f'm{S.nid}', 'value': {'text': f'text {S.nid}', 'v': 1}}
    elif r < 0.85:
        k = S.rng.choice(keys)
        u = {'kind': 'set', 'key': k, 'value': {'text': f'edited at {len(S.log)}', 'v': S.state[k]['v'] + 1}}
    else:
        u = {'kind': 'del', 'key': S.rng.choice(keys)}
    u['n'], u['t'] = len(S.log) + 1, time.time() * 1000
    S.log.append(u)
    if u['kind'] == 'set':
        S.state[u['key']] = u['value']
    else:
        S.state.pop(u['key'], None)
    for q in S.queues:
        q.put_nowait(u)


async def burst(count, rate):
    for _ in range(count):
        produce()
        await asyncio.sleep(1 / rate)


async def ws(request):
    if time.time() < S.refuse_until:
        return web.Response(status=503)
    w = web.WebSocketResponse()
    await w.prepare(request)
    tr = request.transport
    S.conns.add(tr)
    q, sent, tasks = asyncio.Queue(), [0], []
    S.queues.add(q)

    async def pump():
        while True:
            u = await q.get()
            if u['n'] <= sent[0]:
                continue
            sent[0] = u['n']
            if S.rng.random() >= S.drop:              # else: this update is lost on the way, silently
                await w.send_json(u)

    async def beat():
        while True:
            await asyncio.sleep(HEARTBEAT)
            await w.send_json({'head': len(S.log)})
    try:
        tasks.append(asyncio.create_task(beat()))
        async for msg in w:
            n = json.loads(msg.data)['resume']
            for t in tasks[1:]:
                t.cancel()
            del tasks[1:]
            if n < 0:                                  # a client that does not say where it is: live only
                n = len(S.log)
            if len(S.log) - n > LIMIT:
                await w.send_json({'too_far_behind': True})
                continue
            i = n
            while i < len(S.log):                      # everything after N, then live
                await w.send_json(S.log[i])
                i += 1
            sent[0] = i
            tasks.append(asyncio.create_task(pump()))
    except Exception:
        pass
    finally:
        for t in tasks:
            t.cancel()
        S.queues.discard(q)
        S.conns.discard(tr)
    return w


async def sse(request):
    r = web.StreamResponse(headers={'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache'})
    await r.prepare(request)
    q = asyncio.Queue()
    S.queues.add(q)
    try:
        await r.write(b'data: {"hello": true}\n\n')
        while True:
            u = await q.get()
            await r.write(f'id: {u["n"]}\ndata: {json.dumps(u)}\n\n'.encode())
    except Exception:
        pass
    finally:
        S.queues.discard(q)
    return r


async def control(request):
    o = await request.json()
    if 'burst' in o:
        asyncio.create_task(burst(o['burst'], o.get('rate', 400)))
    if 'drop' in o:
        S.drop = o['drop']
    if 'refuse' in o:
        S.refuse_until = time.time() + o['refuse']
    if o.get('cut') or 'refuse' in o:
        for tr in list(S.conns):
            tr.abort()                      # the connection dies without a goodbye
    return web.json_response({})


async def snapshot(request):
    return web.json_response({'state': S.state, 'n': len(S.log)})


def page(name):
    async def h(request):
        return web.FileResponse(name)
    return h


app = web.Application()
app.add_routes([web.get('/', page('client.html')), web.get('/sse.html', page('sse.html')), web.get('/ws', ws), web.get('/sse', sse),
                web.get('/snapshot', snapshot), web.post('/control', control)])
web.run_app(app, host='127.0.0.1', port=int(sys.argv[1]), print=None)
