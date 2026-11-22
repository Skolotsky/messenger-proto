"""E-001: ten working days of a 30-person, 6-agent, 20-channel team through the model.

Checks the invariants as it goes and measures what each person's buckets hold at each visit.
The traffic numbers below are ASSUMPTIONS, not measurements. Variants use the same seed and rates,
but the traffic is not identical between them: what people read changes who replies, and that
feeds back. Differences smaller than the spread across seeds, printed at the end, are noise.
"""
import heapq
import json
import random
import statistics
import sys
from collections import Counter, defaultdict, deque

import model
from model import World, FOCUS, NEEDS, FYI, BUCKET

ASSUME = dict(
    days=10,
    human_msgs_per_person_day=25,   # messages a person writes on a working day
    dm_share=0.15,                  # of those, in one-to-one direct conversations
    reply_share=0.60,               # of channel messages, replies in a thread
    mention_p=0.15,                 # a message mentions someone
    group_p=0.01,                   # a top-level message mentions the whole channel
    flag_p=0.02,                    # a person flags their message
    flag_named_share=0.75,          # of people's flags, name someone
    info_p=0.03,                    # a person marks their message Info
    edit_p=0.05, delete_p=0.01,
    reactions_per_msg=0.8,
    agent_tasks_per_day=60,         # a person asks an agent for something in a channel
    progress_per_task=(4, 8),       # finished progress updates per task
    task_stop_p=0.30,               # the agent stops for the person's approval
    flag_answer_p=0.90,             # a named person answers a flag
    support_posts_per_day=30, support_open_flag_p=0.15, open_flag_answer_p=0.60,
    incident_bursts=2, burst_size=15,
    visits_per_day=6,               # times a person opens the feed
    read_needs_p=0.90, view_fyi_p=0.70, reply_to_mention_p=0.30,
    keep_p=0.04, done_after_day_p=0.50,
    react_answer_p=0.30,            # a person addressed in a message answers with a reaction instead of words
    mention_react_p=0.20,           # a person merely mentioned reacts to acknowledge
    watch_minutes=3,                # after writing in a conversation, a person keeps it open in front of them
)

CHANNELS = [("#general", 30), ("#random", 30), ("#launch", 15), ("#product", 12), ("#engineering", 12),
            ("#incidents", 10), ("#support", 8), ("#design", 6), ("#sales", 6), ("#marketing", 6),
            ("#data", 5), ("#ops", 5), ("#mobile", 6), ("#backend", 7), ("#frontend", 6),
            ("#customers", 8), ("#hiring", 5), ("#billing", 4), ("#docs", 5), ("#ideas", 10)]
AGENTS = {"a-release": ["#launch", "#engineering"], "a-build": ["#backend", "#frontend", "#mobile"],
          "a-research": ["#product", "#customers"], "a-data": ["#data", "#sales", "#marketing"],
          "a-support": ["#support"], "a-alerts": ["#incidents"]}
PERSONAS = {"p02": "Product manager (default viewer)", "p01": "Founder", "p04": "Engineer", "p09": "Support, on call"}


def poisson(rng, lam):
    n, p, L = 0, 1.0, 2.718281828 ** -lam
    while True:
        p *= rng.random()
        if p <= L:
            return n
        n += 1


def run(name, seed=7, scale=1.0, mentions_focus=False, info_used=True, check_every=500, only_done=False, done_p=None, dm_removed=0):
    rng = random.Random(seed)             # traffic: who writes what, where, when
    rv = random.Random(seed * 1000 + 1)   # behaviour at a visit, kept apart so a rule change does not reshuffle traffic
    model.RULINGS.clear(); model.NOTES.clear()
    w = World()
    w.self_check = True
    people = [f"p{i:02d}" for i in range(1, 31)]
    for p in people:
        w.add_person(p)
        w.people[p].only_done = only_done
        if dm_removed:                    # D-021: each person has taken this many colleagues out of Focus
            w.people[p].dm_removed = set(random.Random(f"{seed}{p}").sample([q for q in people if q != p], dm_removed))
        if mentions_focus:
            w.add_rule(p, "mentions")
    for a in AGENTS:
        w.add_agent(a)
    for ch, size in CHANNELS:
        members = set(people) if size >= 30 else set(rng.sample(people, size))
        if ch in ("#launch", "#product"):
            members |= {"p01", "p02"}
        if ch in ("#support", "#incidents"):
            members |= {"p09"}
        if ch == "#engineering":
            members |= {"p04"}
        w.add_channel(ch, members | {a for a, chs in AGENTS.items() if ch in chs})
    # a few dials set by hand, as people would
    w.set_channel_dial("p09", "#incidents", "focus")
    w.set_channel_dial("p09", "#support", "focus")
    w.set_channel_dial("p02", "#launch", "all")
    for p in ("p04", "p05", "p06"):
        w.set_channel_dial(p, "#random", "muted")
    weight = {ch: size * rng.uniform(0.5, 1.5) for ch, size in CHANNELS}
    roots = defaultdict(lambda: deque(maxlen=30))
    A = ASSUME

    # measurements
    arrivals = defaultdict(Counter)       # person -> Counter[(bucket, 'agent'|'person')] of new rows
    items_by = Counter()                  # (bucket, author kind) of items
    needs_why = Counter()                 # why an item landed in Needs you
    visits = defaultdict(list)            # person -> list of counts dicts
    violations = []
    events = [0]

    def on_deliver(u, m, it, new_row):
        k = w.kind[m.author]
        items_by[(it.bucket, k)] += 1
        if it.bucket == NEEDS:
            why = it.reason.split(" · ")[0]
            own = m.parent is not None and not m.conv.startswith("dm:") and it.kind == "msg" \
                and w.auto_level(u, m.parent, before=m.id) == "needs"
            if " reacted " in why:
                why = "Reaction from someone you addressed, in a direct conversation" if m.conv.startswith("dm:") \
                    else "Reaction from someone you addressed, in a channel"
            elif own or why == "Reply in your thread":
                # counted as a thread reply even when it also mentions the person: it would be here anyway
                why = "Reply in a thread you started" if w.msgs[m.parent].author == u else "Reply in a thread you only replied in"
            elif "mentioned you" in why:
                why = "Mentioned you, nothing else"
            elif why.startswith("Flagged for everyone"):
                why = "Flag that names nobody"
            elif why.startswith("Direct message from agent"):
                why = "Direct message from an agent"
            elif why.startswith("Flagged for you") or why.startswith("Direct message"):
                why = "Other"
            needs_why[why] += 1
        if new_row:
            arrivals[u][(it.bucket, k)] += 1
    w.on_deliver = on_deliver

    heap, seq = [], [0]

    def at(t, fn, *args):
        seq[0] += 1
        heapq.heappush(heap, (t, seq[0], fn, args))

    def persons_in(ch, minus=()):
        return sorted(x for x in w.channels[ch] if w.kind[x] == "person" and x not in minus)

    def thread_people(root):
        return {w.msgs[i].author for i in [root] + w.threads[root] if w.kind[w.msgs[i].author] == "person"}

    def reply_later(t, who, conv, root):
        def go():
            if root in w.msgs and not w.msgs[root].deleted:
                w.post(who, conv, parent=root)
                w.watch(who, ("dm", conv) if conv.startswith("dm:") else ("th", root), w.now + A["watch_minutes"])
        at(t, go)

    def human_msg(t):
        a = rng.choice(people)
        parent, group = None, False
        if rng.random() < A["dm_share"]:
            other = rng.choice([p for p in people if p != a])
            conv, pool = w.dm(a, other), [other]
        else:
            mine = sorted(ch for ch in w.channels if a in w.channels[ch])
            conv = rng.choices(mine, [weight[c] for c in mine])[0]
            pool = persons_in(conv, minus=(a,))
            if roots[conv] and rng.random() < A["reply_share"]:
                parent = rng.choice(list(roots[conv])[-10:])
                if w.msgs[parent].deleted:
                    parent = None
            group = parent is None and rng.random() < A["group_p"]
        is_dm = conv.startswith("dm:")
        mentions = []
        if pool and not is_dm and rng.random() < A["mention_p"]:
            near = sorted(p for p in (thread_people(parent) if parent else []) if p != a)
            mentions = [rng.choice(near or pool)]
        level = None
        r = rng.random()
        if r < A["flag_p"]:
            level = "flag"
            if not is_dm and not mentions and pool and rng.random() < A["flag_named_share"]:
                mentions = [rng.choice(pool)]
        elif r < A["flag_p"] + A["info_p"]:
            level = "info"
        m = w.post(a, conv, parent=parent, mentions=mentions, group=group, level=level)
        root = parent or m.id
        w.watch(a, ("dm", conv) if (is_dm and parent is None) else ("th", root), t + A["watch_minutes"])
        for p in mentions:
            if level != "flag" and w.kind[p] == "person" and rng.random() < A["mention_react_p"]:
                at(t + rng.uniform(1, 120), lambda p=p: w.react(p, m.id, "eyes"))
        if parent is None and not is_dm:
            roots[conv].append(m.id)
        for _ in range(poisson(rng, A["reactions_per_msg"])):
            if pool:
                at(t + rng.uniform(1, 60), lambda who=rng.choice(pool): w.react(who, m.id))
        if rng.random() < A["edit_p"]:
            k = rng.random()
            def ed():
                if k < 0.80:
                    w.edit(m.id)
                elif k < 0.90 and pool:
                    w.edit(m.id, mentions=set(m.mentions) | {rng.choice(pool)})
                elif k < 0.95:
                    w.edit(m.id, mentions=[])
                else:
                    w.edit(m.id, level=rng.choice([None, "info"]))
            at(t + rng.uniform(1, 10), ed)
        if rng.random() < A["delete_p"]:
            at(t + rng.uniform(1, 30), lambda: w.delete(m.id))
        if level == "flag":
            named = sorted(p for p in w.flag_recipients(m) if w.kind[p] == "person")
            for p in named:
                if rng.random() < A["flag_answer_p"]:
                    if rng.random() < A["react_answer_p"]:
                        at(t + rng.uniform(5, 180), lambda p=p: w.react(p, m.id, "check"))
                    elif is_dm:
                        at(t + rng.uniform(5, 180), lambda p=p: w.post(p, conv))
                    else:
                        reply_later(t + rng.uniform(5, 180), p, conv, root)
            if not named and pool and rng.random() < A["open_flag_answer_p"]:
                reply_later(t + rng.uniform(10, 240), rng.choice(pool), conv, root)

    def agent_task(t):
        agent = rng.choice([a for a in AGENTS if a not in ("a-support", "a-alerts")])
        ch = rng.choice(AGENTS[agent])
        req = rng.choice(persons_in(ch))
        root = w.post(req, ch, mentions=[agent])
        roots[ch].append(root.id)
        tt = t
        for _ in range(rng.randint(*A["progress_per_task"])):
            tt += rng.uniform(2, 15)
            at(tt, lambda: w.post(agent, ch, parent=root.id, level="info" if info_used else None))

        def result(t2):
            s = [None]
            at(t2, lambda: s.__setitem__(0, w.post(agent, ch, parent=root.id, streaming=True,
                                                    mentions=[req] if rng.random() < 0.5 else [])))
            at(t2 + rng.uniform(0.5, 2), lambda: w.finish_stream(s[0]))
        if rng.random() < A["task_stop_p"]:
            tt += rng.uniform(2, 10)
            at(tt, lambda: w.post(agent, ch, parent=root.id, mentions=[req], level="flag"))
            if rng.random() < A["flag_answer_p"]:
                ta = tt + rng.uniform(5, 180)
                reply_later(ta, req, ch, root.id)
                result(ta + rng.uniform(2, 20))
        else:
            result(tt + rng.uniform(2, 10))

    def support_post(t):
        r = rng.random()
        if r < A["support_open_flag_p"]:
            m = w.post("a-support", "#support", level="flag")
            if rng.random() < A["open_flag_answer_p"]:
                reply_later(t + rng.uniform(10, 240), rng.choice(persons_in("#support")), "#support", m.id)
        else:
            m = w.post("a-support", "#support", level="info" if (r < 0.75 and info_used) else None)
        roots["#support"].append(m.id)

    def burst(t):
        for i in range(A["burst_size"]):
            def go(i=i):
                m = w.post("a-alerts", "#incidents", mentions=["p09"] if i == 0 else [], level="flag" if i == 0 else None)
                roots["#incidents"].append(m.id)
                if i == 0:
                    reply_later(w.now + rng.uniform(1, 10), "p09", "#incidents", m.id)
                elif rng.random() < 0.3:
                    reply_later(w.now + rng.uniform(1, 20), rng.choice(persons_in("#incidents")), "#incidents", m.id)
            at(t + i * rng.uniform(0.5, 2.5), go)

    def visit(t, u):
        f = w.open_feed(u)
        c = w.counts(u); c["day"] = int(t // 1440)
        visits[u].append(c)
        just_read = []

        def maybe_reply(row):
            mention = next((i for i in row.items.values() if not i.read and "mentioned you" in i.reason), None)
            if mention and rv.random() < A["reply_to_mention_p"]:
                mm = w.msgs[mention.msg]
                reply_later(t + rv.uniform(1, 5), u, mm.conv, mm.parent or mm.id)
        for row in f[FOCUS]:
            if w.row_section(u, row) == "new":
                maybe_reply(row)
                w.read_row(u, row.key); just_read.append(row)
        for row in f[NEEDS]:
            if rv.random() < A["read_needs_p"]:
                maybe_reply(row)
                w.read_row(u, row.key); just_read.append(row)
        if rv.random() < A["view_fyi_p"]:
            w.view_bucket(u, FYI)
        for row in just_read:
            if rv.random() < A["keep_p"]:
                w.keep(u, row.key)
        for row in list(w.people[u].rows.values()):
            # a row kept by the person, or by a flag they have not answered, is marked done after a day or so
            if (row.kept or row.flagged) and row.last < t - 1440 and rv.random() < (A["done_after_day_p"] if done_p is None else done_p):
                w.done(u, row.key)
        w.leave_feed(u)

    for d in range(A["days"]):
        start, end = d * 1440 + 540, d * 1440 + 1080
        for _ in range(int(len(people) * A["human_msgs_per_person_day"] * scale)):
            at(rng.uniform(start, end), human_msg, )
        for _ in range(int(A["agent_tasks_per_day"] * scale)):
            at(rng.uniform(start, end - 60), agent_task)
        for _ in range(int(A["support_posts_per_day"] * scale)):
            at(rng.uniform(start, end), support_post)
        for p in people:
            for _ in range(A["visits_per_day"]):
                at(rv.uniform(start, end), visit, p)
    for _ in range(A["incident_bursts"]):
        d = rng.randrange(A["days"])
        at(d * 1440 + rng.uniform(560, 1000), burst)

    while heap:
        t, _, fn, args = heapq.heappop(heap)
        w.now = t
        if fn in (human_msg, agent_task, support_post, burst, visit):
            fn(t, *args)
        else:
            fn(*args)
        events[0] += 1
        if events[0] % check_every == 0:
            violations.extend(w.check())
    violations.extend(w.check())
    violations.extend(w.violations)

    # ---- summarise
    def stat(xs):
        xs = sorted(xs)
        return {"mean": round(statistics.mean(xs), 1), "median": xs[len(xs) // 2], "p90": xs[int(len(xs) * 0.9)], "max": xs[-1]}

    keys = ["focus_new", "focus_kept", "needs", "fyi"]
    at_visit = {k: stat([c[k] for u in people for c in visits[u]]) for k in keys}
    needs_trend = {f"days {a + 1}-{b}": round(statistics.mean(c["needs"] for u in people for c in visits[u] if a <= c["day"] < b), 1)
                   for a, b in ((0, 2), (4, 6), (8, 10))}
    per_day = {}
    for label, group in [("all", people)] + [(p, [p]) for p in PERSONAS]:
        row = {}
        for b in (FOCUS, NEEDS, FYI):
            for k in ("person", "agent"):
                row[f"{BUCKET[b]} from {k}s"] = round(sum(arrivals[u][(b, k)] for u in group) / len(group) / A["days"], 1)
        row["single list"] = round(sum(sum(arrivals[u].values()) for u in group) / len(group) / A["days"], 1)
        row["Focus"] = round(row["Focus from persons"] + row["Focus from agents"], 1)
        per_day[label] = row
    persona_visit = {p: {k: stat([c[k] for c in visits[p]]) for k in keys} for p in PERSONAS}
    msgs = list(w.msgs.values())
    by_agents = sum(1 for m in msgs if w.kind[m.author] == "agent")
    end_state = {
        "rows kept by a flag at the end (per person, mean)": round(statistics.mean(
            sum(1 for r in w.people[u].rows.values() if r.flagged) for u in people), 1),
        "rows in Kept at the end (per person, mean)": round(statistics.mean(
            sum(1 for r in w.people[u].rows.values() if r.kept or r.flagged) for u in people), 1),
        "rows in Kept at the end (largest for one person)": max(
            sum(1 for r in w.people[u].rows.values() if r.kept or r.flagged) for u in people),
        "rows kept by the person at the end (per person, mean)": round(statistics.mean(
            sum(1 for r in w.people[u].rows.values() if r.kept) for u in people), 1),
        "markers left in conversations": w.tombstones,
    }
    return {
        "variant": name, "messages": len(msgs), "share written by agents": round(by_agents / len(msgs), 2),
        "events": events[0], "violations": len(violations), "violation samples": sorted(set(v.split(":", 1)[1].strip()[:90] for v in violations))[:8],
        "rows at a visit": at_visit, "Needs you at a visit over time": needs_trend, "new rows per person per day": per_day, "personas at a visit": persona_visit,
        "items by bucket and author": {f"{BUCKET[b]} / {k}": n for (b, k), n in sorted(items_by.items(), reverse=True)},
        "why items land in Needs you": dict(needs_why.most_common()),
        "notifications per person per day": {p: round(sum(w.notified[(u, p)] for u in people) / len(people) / A["days"], 1) for p in "ABC"},
        "read at once while watching": w.read_at_once,
        "end of run": end_state, "rulings": dict(sorted(model.RULINGS.items())), "notes": dict(model.NOTES),
    }


VARIANTS = [
    ("base: the accepted rules", {}),
    ("mentions start in Focus", {"mentions_focus": True}),
    ("agents do not use Info", {"info_used": False}),
    ("half the traffic", {"scale": 0.5}),
    ("double the traffic", {"scale": 2.0}),
]

if __name__ == "__main__":
    out = []
    for name, kw in VARIANTS:
        r = run(name, **kw)
        out.append(r)
        v = r["rows at a visit"]
        d = r["new rows per person per day"]["all"]
        print(f"\n== {name}")
        print(f"   {r['messages']} messages, {int(r['share written by agents'] * 100)}% by agents, {r['events']} events, invariant violations: {r['violations']}")
        for s in r["violation samples"]:
            print("     !", s)
        print("   rows waiting when a person opens the feed (mean / p90 / max):")
        for k, label in [("focus_new", "Focus, new"), ("focus_kept", "Focus, kept (by you or a flag)"),
                         ("needs", "Needs you"), ("fyi", "For your information")]:
            print(f"     {label:32s} {v[k]['mean']:5} / {v[k]['p90']:3} / {v[k]['max']:3}")
        print(f"   new rows per person per day: Focus {d['Focus']} (from agents {d['Focus from agents']}), "
              f"Needs you {round(d['Needs you from persons'] + d['Needs you from agents'], 1)} (from agents {d['Needs you from agents']}), "
              f"FYI {round(d['FYI from persons'] + d['FYI from agents'], 1)} (from agents {d['FYI from agents']}); one single list: {d['single list']}")
        n = r["notifications per person per day"]
        print(f"   notifications per person per day: everything {n['A']}, Focus and Needs you {n['B']} (accepted default), Focus only {n['C']}")
        print(f"   arrivals read at once because the person had just written in that conversation (a rough stand-in for 'watching'): {r['read at once while watching']}")
        print("   Needs you at a visit, mean, as the run goes on:", ", ".join(f"{k}: {v}" for k, v in r["Needs you at a visit over time"].items()))
        for k, n in r["end of run"].items():
            print(f"   {k}: {n}")
    base = out[0]
    print("\n== personas, base rules: new rows per day, and Focus rows at a visit (mean / max)")
    for p, label in PERSONAS.items():
        d = base["new rows per person per day"][p]
        pv = base["personas at a visit"][p]
        print(f"   {label:34s} Focus {d['Focus']:5}  Needs you {round(d['Needs you from persons'] + d['Needs you from agents'], 1):5}  "
              f"FYI {round(d['FYI from persons'] + d['FYI from agents'], 1):5}  single list {d['single list']:5}   "
              f"Focus new at a visit {pv['focus_new']['mean']} / {pv['focus_new']['max']}")
    print("\n== why items land in Needs you, base rules (items, not rows)")
    total = sum(base["why items land in Needs you"].values())
    for why, n in base["why items land in Needs you"].items():
        print(f"   {why:40s} {n:6d}  {round(100 * n / total)}%")
    print("\n== behaviours the model had to invent, and how often each came up in the base run")
    for code, n in sorted(base["rulings"].items(), key=lambda x: -x[1]):
        if code not in model.ACCEPTED:
            print(f"   {code} x{n:6d}  STILL OPEN  {model.RULING_TEXT[code]}")
    for code, n in sorted(base["rulings"].items(), key=lambda x: -x[1]):
        if code in model.ACCEPTED:
            print(f"   {code} x{n:6d}  accepted in D-013  {model.RULING_TEXT[code]}")

    print("\n== how much the base numbers move with the random seed (five other seeds)")
    seeds = [run("base", seed=s) for s in (1, 2, 3, 4, 5)]
    def rng_of(f):
        xs = [f(r) for r in seeds]
        return f"{min(xs)} to {max(xs)}"
    print("   new Focus rows per person per day:      ", rng_of(lambda r: r["new rows per person per day"]["all"]["Focus"]))
    print("   one single list, rows per person per day:", rng_of(lambda r: r["new rows per person per day"]["all"]["single list"]))
    print("   Focus, new, at a visit (mean):           ", rng_of(lambda r: r["rows at a visit"]["focus_new"]["mean"]))
    print("   Needs you at a visit (mean):             ", rng_of(lambda r: r["rows at a visit"]["needs"]["mean"]))
    print("   Needs you at a visit (p90):              ", rng_of(lambda r: r["rows at a visit"]["needs"]["p90"]))
    print("   Focus, kept, at a visit (mean):          ", rng_of(lambda r: r["rows at a visit"]["focus_kept"]["mean"]))
    print("   Focus, kept, at a visit (max):           ", rng_of(lambda r: r["rows at a visit"]["focus_kept"]["max"]))
    print("   notifications a day, accepted default:   ", rng_of(lambda r: r["notifications per person per day"]["B"]))
    print("   notifications a day, Focus only:         ", rng_of(lambda r: r["notifications per person per day"]["C"]))
    print("   invariant violations:                    ", rng_of(lambda r: r["violations"]))
    out.append({"seeds": seeds})
    json.dump(out, open("results.json", "w"), indent=1)
    sys.exit(1 if any(r["violations"] for r in out[:-1] + seeds) else 0)
