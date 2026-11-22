#!/usr/bin/env python3
"""Export E-001's scripted cases as data, so the Go rules can replay them.

Run from the repository root:
    python3 server/internal/feed/rules/testdata/export_e001.py

It runs experiments/E-001-feed-rules-model/scenarios.py unchanged, records every
call made on a World, and at every case() stores the model's whole observable
state. The Go test replays the calls as log changes and compares that state, so
each case checks more than the single value the scenario asserted.

A marker the model keeps only because the row is on screen is left out of the
state: what is on screen is the web app's, and the log does not carry it. Those
cases are listed in the task file.
"""
import json
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parents[5]
E001 = ROOT / "experiments" / "E-001-feed-rules-model"
sys.path.insert(0, str(E001))
sys.dont_write_bytecode = True

import model  # noqa: E402

worlds = []  # one trace per World
cases = []
current = None


def mid(i):
    return f"m{i}"


def key_str(key):
    return f"{key[0]}:{key[1]}" if key[0] != "th" else f"th:{mid(key[1])}"


def record(w, op, **args):
    args["op"] = op
    args["now"] = w.now
    w.trace["ops"].append(args)


def snapshot(w):
    out = {}
    for u, P in sorted(w.people.items()):
        rows = []
        for row in P.rows.values():
            items = []
            for (m, kind), it in sorted(row.items.items(), key=lambda kv: (kv[0][0], kv[0][1])):
                if it.retracted and not it.read and w._on_screen(u, row, it) and not row.hold and not row.kept:
                    continue
                items.append({"msg": mid(m), "kind": kind, "bucket": it.bucket, "read": it.read, "retracted": it.retracted})
            if not items and not row.kept:
                continue
            b = w.row_bucket(u, row)
            where = "Earlier" if b is None else model.BUCKET[b]
            if b == model.FOCUS:
                where += "/" + w.row_section(u, row)
            rows.append({"key": key_str(row.key), "where": where, "kept": row.kept, "hold": row.hold,
                         "flagged": sorted(mid(f) for f in row.flagged), "items": items})
        rows.sort(key=lambda r: r["key"])
        out[u] = {
            "rows": rows,
            "thread_dials": {mid(k): v for k, v in sorted(P.th_dial.items())},
            "focus_people": sorted(P.dm_people),
            "notifications": [[mid(n[1]), n[2], n[3]] for n in w.notifications if n[0] == u],
            "history": {tab: [mid(i) for i in w.history(u, tab)] for tab in ("dms", "mentions", "threads", "reactions")},
        }
    return out


def wrap(name, translate):
    original = getattr(model.World, name)

    def wrapper(self, *a, **k):
        if getattr(self, "_inside", False):
            return original(self, *a, **k)
        self._inside = True
        try:
            before = translate(self, *a, **k)
            result = original(self, *a, **k)
            after = before(result) if callable(before) else None
            if after is not None:
                op, args = after
                record(self, op, **args)
            return result
        finally:
            self._inside = False
    setattr(model.World, name, wrapper)


def t_post(w, author, conv, parent=None, mentions=(), group=False, level=None, streaming=False):
    def done(m):
        return ("post", dict(id=mid(m.id), author=author, conv=conv, parent=mid(parent) if parent else None,
                             mentions=sorted(m.mentions), group=group, level=level, streaming=streaming))
    return done


def t_finish(w, m):
    old = m.id
    def done(_):
        return ("finish", dict(old=mid(old), id=mid(m.id), author=m.author, conv=m.conv,
                               parent=mid(m.parent) if m.parent else None, mentions=sorted(m.mentions),
                               group=m.group, level=m.level))
    return done


def t_edit(w, i, mentions=None, level="same", group=None):
    def done(_):
        m = w.msgs[i]
        return ("edit", dict(id=mid(i), mentions=sorted(m.mentions), group=m.group, level=m.level))
    return done


def simple(op, fields):
    def t(w, *a, **k):
        args = dict(zip(fields, a))
        args.update(k)
        for f in ("mid", "root"):
            if f in args and isinstance(args[f], int):
                args[f] = mid(args[f])
        if "key" in args:
            args["key"] = key_str(args["key"])
        def done(_):
            return (op, args)
        return done
    return t


wrap("post", t_post)
wrap("finish_stream", t_finish)
wrap("edit", t_edit)
wrap("delete", simple("delete", ["mid"]))
wrap("react", simple("react", ["person", "mid", "emoji"]))
wrap("unreact", simple("unreact", ["person", "mid"]))
wrap("read_row", simple("read", ["person", "key"]))
wrap("keep", simple("keep", ["person", "key"]))
wrap("done", simple("done", ["person", "key"]))
wrap("watch", simple("watch", ["person", "key", "until"]))
wrap("open_feed", simple("open_feed", ["person"]))
wrap("leave_feed", simple("leave_feed", ["person"]))
wrap("set_channel_dial", simple("channel_dial", ["person", "conv", "level"]))
wrap("set_thread_dial", simple("thread_dial", ["person", "root", "level"]))
wrap("remove_dm_person", simple("remove_person", ["person", "who"]))
wrap("add_dm_person", simple("add_person", ["person", "who"]))


def t_option_off(w, u, remove_all=False):
    people = sorted(w.people[u].dm_people) if remove_all else []
    def done(_):
        return ("auto_add_off", dict(person=u, remove=people))
    return done


wrap("dm_option_off", t_option_off)


def rule_args(rule):
    if isinstance(rule, tuple):
        return {"type": rule[0], "value": rule[1]}
    return {"type": "item_kind", "value": rule}


def t_add_rule(w, u, rule):
    return lambda _: ("add_rule", dict(person=u, rule=rule_args(rule)))


def t_remove_rule(w, u, rule):
    return lambda _: ("remove_rule", dict(person=u, rule=rule_args(rule)))


wrap("add_rule", t_add_rule)
wrap("remove_rule", t_remove_rule)

original_preview = model.World.rule_preview


def preview(self, u, rule, since=0.0):
    n = original_preview(self, u, rule, since)
    record(self, "preview", person=u, rule=rule_args(rule), expected=n)
    return n


model.World.rule_preview = preview

original_setattr = model.Person.__setattr__


def person_setattr(self, name, value):
    original_setattr(self, name, value)
    if name == "only_done" and value and worlds:
        w = worlds[-1]
        for u, P in w.people.items():
            if P is self:
                record(w, "only_done", person=u)


model.Person.__setattr__ = person_setattr

original_init = model.World.__init__


def world_init(self):
    original_init(self)
    self.trace = {"people": [], "agents": [], "channels": {}, "ops": []}
    worlds.append(self)


model.World.__init__ = world_init
original_add_person, original_add_agent, original_add_channel = model.World.add_person, model.World.add_agent, model.World.add_channel


def add_person(self, p):
    original_add_person(self, p)
    self.trace["people"].append(p)


def add_agent(self, a):
    original_add_agent(self, a)
    self.trace["agents"].append(a)


def add_channel(self, name, members):
    original_add_channel(self, name, members)
    self.trace["channels"][name] = sorted(members)


model.World.add_person, model.World.add_agent, model.World.add_channel = add_person, add_agent, add_channel

# Run the scenarios as they are, with case() also taking a snapshot of the world in play.
source = (E001 / "scenarios.py").read_text()
head, tail = source.split("# ---------------------------------------------------------------- report")
namespace = {"__name__": "scenarios"}
exec(compile(head.split("# ---------------------------------------------------------------- starting rules")[0], "scenarios", "exec"), namespace)


def case(name, got, expected, basis):
    w = worlds[-1]
    model.QUIET[0] = True
    snap = snapshot(w)
    model.QUIET[0] = False
    w.trace["ops"].append({"op": "case", "index": len(cases), "state": snap})
    cases.append({"name": name, "basis": basis, "ok": got == expected, "got": repr(got), "expected": repr(expected)})


namespace["case"] = case
exec(compile("# ----" + head.split("# ---------------------------------------------------------------- starting rules")[1], "scenarios", "exec"), namespace)

bad = [c["name"] for c in cases if not c["ok"]]
if bad:
    sys.exit(f"the model itself fails: {bad}")
out = {"source": "experiments/E-001-feed-rules-model/scenarios.py", "cases": cases,
       "worlds": [w.trace for w in worlds]}
path = pathlib.Path(__file__).parent / "e001_cases.json"
path.write_text(json.dumps(out, separators=(",", ":"), ensure_ascii=False, sort_keys=True) + "\n")
print(f"{len(cases)} cases in {len(worlds)} worlds written to {path} ({path.stat().st_size // 1024} KiB)")
