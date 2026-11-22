#!/usr/bin/env python3
"""E-008: two small simulations behind the scaling decisions. No database, no processes; a few seconds.

A. Can the changes of one organization be applied side by side, conversation by conversation, without
   changing the feed's result? The feed rules of E-003 are replayed over the same changes in many different
   interleavings that keep the order inside each conversation.
B. When many organizations have a backlog at once, how long does fresh traffic wait, by the rule used to
   choose whom to serve next? A queue model, not a measurement."""
import heapq, json, os, random, sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'E-003-delivery-spine'))
import spine

PEOPLE, CH = spine.PEOPLE, ['#c0', '#c1', '#c2', '#c3']


def make_log(n, seed):
    """A change log as Conversations would write it, made in memory (the generator of E-003's driver)."""
    r, msgs, log = random.Random(seed), [], []
    for i in range(1, n * 3):
        if len(log) >= n:
            break
        p, x = r.choice(PEOPLE), r.random()
        num = len(log) + 1
        if x < 0.42 or len(msgs) < 5:
            q, thread, mentions = r.random(), None, []
            tops = [m for m in msgs[-40:] if m['conv'].startswith('#') and not m['deleted']]
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
            m = dict(id=f'm{i}', version=1, conv=conv, thread=thread, author=p, text=f'text {i}', mentions=mentions,
                     level='flag' if lv < 0.07 else 'info' if lv < 0.14 else None, reactions=[], deleted=False)
            msgs.append(m)
            log.append((num, 'message.created', json.loads(json.dumps(m))))
            continue
        live = [m for m in msgs if not m['deleted']]
        if not live:
            continue
        m = r.choice(live[-15:]) if r.random() < 0.4 else r.choice(live)
        if x < 0.50:
            o = r.choice([y for y in PEOPLE if y != m['author']])
            if m['conv'].startswith('#'):
                m['mentions'].remove(o) if o in m['mentions'] else m['mentions'].append(o)
            if r.random() < 0.2:
                m['level'] = None if m['level'] == 'flag' else 'flag'
            m['text'], m['version'] = f'edited {i}', m['version'] + 1
            log.append((num, 'message.changed', json.loads(json.dumps(m))))
        elif x < 0.55:
            m['deleted'], m['version'] = True, m['version'] + 1
            log.append((num, 'message.deleted', dict(id=m['id'], version=m['version'])))
        elif x < 0.67:
            pair = [p, r.choice('👍✅👀')]
            if pair in m['reactions']:
                m['reactions'].remove(pair)
            else:
                m['reactions'].append(pair)
            m['version'] += 1
            log.append((num, 'message.changed', json.loads(json.dumps(m))))
        elif x < 0.90:
            log.append((num, 'reader.read', dict(person=p, ck=spine.ck_of(m), upto=num - 1)))
        elif x < 0.903:
            log.append((num, 'reader.watch', dict(person=p, ck=spine.ck_of(m) if r.random() < 0.7 else None)))
        elif x < 0.96:
            log.append((num, 'reader.dial', dict(person=p, channel=r.choice(CH), level=r.choice(['default', 'muted', 'focus', 'all']))))
        else:
            log.append((num, 'reader.keep' if r.random() < 0.6 else 'reader.done', dict(person=p, ck=spine.ck_of(m))))
    return log


def lane_of(kind, b, where):
    """Which conversation a change belongs to: a channel together with its threads, or a direct conversation.
    None: a change about a reader as a whole, which every conversation has to see in the same place."""
    if kind == 'message.created':
        where[b['id']] = b['conv']
        return b['conv']
    if kind in ('message.changed', 'message.deleted'):
        return where[b['id']]
    if kind == 'reader.dial':
        return b['channel']
    if kind == 'reader.watch':
        return None                      # in this model "watching" is one slot per person: it spans conversations
    ck = b['ck']
    return where.get(ck[2:], ck) if ck.startswith('t:') else ck


def replay(order):
    st, notes = spine.DictStore(), []
    for n, kind, b in order:
        out = []
        spine.fold(st, n, kind, json.loads(json.dumps(b)), out)
        notes += [(e['kind'], e['name']) for e in out]
    per = {}
    for kind, name in notes:                    # notes in order per conversation; the order between conversations is free
        per.setdefault(name.split('/')[1], []).append((kind, name))
    return st.dump(), per


def shuffle_lanes(log, rng, respect_barriers=True):
    """Another order of the same changes: the order inside each conversation is kept; changes of different
    conversations are interleaved at random. Reader-wide changes stay where they are, as barriers."""
    where, out, block = {}, [], []

    def flush():
        lanes = {}
        for item in block:
            lanes.setdefault(item[3], []).append(item[:3])
        keys = [k for k in lanes for _ in lanes[k]]
        rng.shuffle(keys)
        for k in keys:
            out.append(lanes[k].pop(0))
        block.clear()
    for n, kind, b in log:
        lane = lane_of(kind, b, where)
        if lane is None and respect_barriers:
            flush()
            out.append((n, kind, b))
        else:
            block.append((n, kind, b, lane if lane is not None else '*'))
    flush()
    return out


def part_a():
    print('A · Applying one organization\'s conversations side by side')
    for label, respect in (('reader-wide changes kept in place', True), ('reader-wide changes shuffled too (broken on purpose)', False)):
        differ = total = 0
        for seed in range(20):
            log = make_log(1500, seed)
            base = replay(log)
            for k in range(10):
                total += 1
                if replay(shuffle_lanes(log, random.Random(seed * 100 + k), respect)) != base:
                    differ += 1
        lanes = len({lane_of(k, b, w) for w in [{}] for n, k, b in make_log(1500, 0)})
        print(f'    {label}: {differ} of {total} interleavings gave a different feed '
              f'(20 logs of 1,500 changes, about {lanes} conversations each, 10 interleavings per log)')


def part_b():
    """Workers take organizations one after another; serving an organization's turn takes 1 ms per change, at most
    40 changes a turn. 200 organizations start with 100 changes waiting each; fresh single changes arrive for
    other organizations at a steady rate. How long does a fresh change wait?"""
    print('B · A wave of catching up, and fresh traffic arriving meanwhile (a queue model)')
    for rule in ('whoever has waited longest', 'fresh traffic first, some of the backlog every round'):
        for workers in (2, 4, 8):
            rng = random.Random(1)
            backlog = [[0.0, 100] for _ in range(200)]             # [waiting since, changes left]
            arrivals = sorted(rng.uniform(0, 10) for _ in range(2000))   # 200 fresh changes a second for 10 s
            fresh, waits, free = [], [], [0.0] * workers
            heapq.heapify(free)
            ai, round_no = 0, 0
            while ai < len(arrivals) or fresh or any(b[1] for b in backlog):
                t = heapq.heappop(free)
                while ai < len(arrivals) and arrivals[ai] <= t:
                    fresh.append(arrivals[ai]); ai += 1
                behind = [b for b in backlog if b[1]]
                if not fresh and not behind:
                    t = arrivals[ai]                               # idle until the next arrival
                    heapq.heappush(free, t)
                    continue
                round_no += 1
                take_backlog = bool(behind) and (not fresh or (
                    min(b[0] for b in behind) < fresh[0] if rule.startswith('whoever') else round_no % 4 == 0))
                if take_backlog:
                    b = min(behind, key=lambda z: z[0])
                    n = min(40, b[1]); b[1] -= n; b[0] = t + n * 0.001
                    heapq.heappush(free, t + n * 0.001)
                else:
                    waits.append(t + 0.001 - fresh.pop(0))
                    heapq.heappush(free, t + 0.001)
            waits.sort()
            done = max(free)
            print(f'    {rule:52} | {workers} workers | a fresh change waits: median {waits[len(waits)//2]*1000:7.0f} ms, '
                  f'19 in 20 within {waits[int(len(waits)*0.95)]*1000:7.0f} ms | the whole backlog is done after {done:5.1f} s')


if __name__ == '__main__':
    part_a()
    print()
    part_b()
