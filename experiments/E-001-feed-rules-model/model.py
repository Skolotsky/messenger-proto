"""E-001: an executable model of the feed rules in D-001, D-003, D-004 and D-011 to D-016.

Throwaway. No storage, no network, no interface. Product code must not import it.
Every behaviour the decisions do not state goes through ruling(), so it is named and counted.
"""
from collections import Counter, defaultdict
from dataclasses import dataclass, field

FOCUS, NEEDS, FYI = 3, 2, 1
BUCKET = {FOCUS: "Focus", NEEDS: "Needs you", FYI: "FYI", 0: "-"}
LEVEL = {"focus": FOCUS, "needs": NEEDS, "fyi": FYI, "off": 0}

RULINGS = Counter()
RULING_TEXT = {
    "R03": "A flag with only a whole-channel mention counts as naming nobody",
    "R04": "A direct message from an agent lands in Needs you (the Focus starting rule says 'from people')",
    "R05": "A Focus rule for a person lifts what already reaches you from them; it does not pull in their other messages",
    "R06": "An Info message that mentions you in a muted channel still arrives, in For your information",
    "R12": "An edit that removes Info delivers the message as if new",
    "R13": "An edit that lowers a message you have not read moves it down quietly",
    "R16": "New activity on a kept row shows inside the Kept section; it does not move the row back to New",
    "R18": "Reactions to your message follow the thread: none when the thread is Off or the channel Muted",
    "R19": "Reading a conversation in the channel itself marks its feed row read",
    "R22": "A person who is not a member of a channel gets nothing from it except what names them",
    "R23": "Mark all read leaves kept rows in place",
    "R25": "Replying in a thread you set to For your information by hand leaves it there",
    "R27": "Deleting your own message does not remove you from its thread: later replies still reach you",
    "R30": "A whole-channel mention inside a thread reply reaches nobody extra",
    "R31": "If you remove a starting Focus rule, those items land in Needs you",
    "R32": "A flag that mentions only an agent counts as naming somebody, so no person receives it",
    "R34": "A reaction from an agent you addressed stays grouped in For your information (D-014 speaks of people)",
    "R35": "An answer by reaction reaches you from a muted channel, like a reply in your own thread; ordinary reactions from a muted channel do not",
    "R36": "An edit that newly names you sends a notification, like a new message would",
    "R37": "A row kept for you by a flag sits in Focus even if you removed the 'flags' starting rule",
    "R38": "New activity on a row sitting in Focus because it is kept notifies by the bucket the new item earns, not as Focus",
    "R40": "A row lifted into Focus by your own dial or rule change sends no notification",
    "R41": "In a channel you put in Focus, replies in a thread you were only mentioned in are lifted to Focus too",
    "R42": "A reply releases everything kept on that row; a reaction releases the flag on that message and your own Keep",
    "R43": "A reaction to a flag that names nobody is ordinary applause for its author, not an answer",
    "R44": "When a mention of you is edited away or deleted, the thread stops following you",
}
# Rulings accepted as assumptions in D-013 (or following directly from a decision). The rest are still open.
ACCEPTED = {"R03", "R04", "R05", "R06", "R12", "R13", "R16", "R18", "R19", "R22", "R23", "R25", "R27", "R30", "R31", "R32", "R36"}
NOTES = Counter()
NOTE_TEXT = {
}
QUIET = [False]


def ruling(code):
    if not QUIET[0]:
        RULINGS[code] += 1


def note(code):
    if not QUIET[0]:
        NOTES[code] += 1


@dataclass
class Msg:
    id: int
    conv: str                 # "#channel" or "dm:a|b"
    author: str
    ts: float
    parent: int | None = None  # thread root id, None for a top-level message
    mentions: set = field(default_factory=set)
    group: bool = False        # mentions the whole channel
    level: str | None = None   # "flag", "info" or None
    streaming: bool = False    # agent text still being written
    deleted: bool = False
    version: int = 1
    reactions: dict = field(default_factory=dict)   # user -> emoji


@dataclass
class Item:
    msg: int
    kind: str                  # "msg" or "reaction"
    bucket: int
    reason: str
    ts: float
    read: bool = False
    retracted: bool = False    # shown as a marker
    edited: bool = False
    flag: str | None = None    # "named" or "open" when the item is there because of a flag


@dataclass
class Row:
    key: tuple
    items: dict = field(default_factory=dict)   # (msg, kind) -> Item
    kept: bool = False         # kept by the person (D-003)
    flagged: set = field(default_factory=set)   # messages whose flag keeps this row for the person (D-011)
    hold: bool = False         # a marker that stays until Done (D-004: a row the person kept)
    last: float = 0.0


@dataclass
class Person:
    rules: set = field(default_factory=lambda: {"dm", "flags"})   # starting Focus rules (D-001)
    ch_dial: dict = field(default_factory=dict)   # channel -> focus|all|default|muted
    th_dial: dict = field(default_factory=dict)   # thread root -> focus|needs|fyi|off (explicit only)
    rows: dict = field(default_factory=dict)
    view: dict | None = None                      # snapshot while the feed is open
    view_ts: float = 0.0
    pending_new: int = 0
    dm_people: set = field(default_factory=set)   # D-021: whose direct messages land in Focus
    dm_removed: set = field(default_factory=set)  # D-021: taken out by hand, never added back by writing again
    only_done: bool = False                       # D-023 point 26: a reply or reaction does not release a kept row


class World:
    def __init__(self):
        self.kind = {}                 # id -> "person" | "agent"
        self.channels = {}             # "#name" -> set of members
        self.msgs = {}
        self.threads = defaultdict(list)   # root id -> reply ids
        self.people = {}               # id -> Person
        self.next_id = 1
        self.now = 0.0
        self.on_deliver = None         # hook for measurements
        self.tombstones = 0            # markers left in conversations (deleted message with replies)
        self.self_check = False        # compare delete and text-only edit against the D-004 table
        self.watching = {}             # person -> (row key, until): a conversation open in front of them (D-015)
        self.read_at_once = 0
        self.notified = Counter()      # (person, policy) -> notifications; policies A, B, C of D-012
        self.notifications = []        # (person, message id, bucket) under the accepted default, B
        self.violations = []

    # ---------------------------------------------------------------- setup
    def add_person(self, pid):
        self.kind[pid] = "person"
        self.people[pid] = Person()

    def add_agent(self, aid):
        self.kind[aid] = "agent"

    def add_channel(self, name, members):
        self.channels[name] = set(members)

    @staticmethod
    def dm(a, b):
        return "dm:" + "|".join(sorted([a, b]))

    # ------------------------------------------------------------ thread level
    def auto_level(self, u, root, before=None):
        """D-001 thread dial, automatic part: started or replied -> needs; mentioned -> fyi; else off."""
        ids = [root] + self.threads[root]
        mentioned = False
        for i in ids:
            if before is not None and i >= before:
                continue
            m = self.msgs[i]
            if m.author == u:
                if m.deleted:
                    ruling("R27")
                return "needs"
            if u in m.mentions and not m.deleted:
                mentioned = True
            elif u in getattr(m, "ever_mentioned", ()):
                ruling("R44")
        return "fyi" if mentioned else "off"

    def thread_level(self, u, m):
        """Level that applies to a plain reply m for person u, and whether it was set by hand."""
        P = self.people[u]
        root = m.parent
        if root in P.th_dial:
            return P.th_dial[root], True
        lvl = self.auto_level(u, root, before=m.id)
        if lvl != "off" and not m.conv.startswith("dm:"):
            dial = P.ch_dial.get(m.conv, "default")
            if dial == "focus":
                if lvl == "fyi":
                    ruling("R41")
                lvl = "focus"       # D-001: a channel in Focus brings replies in your threads there
            # D-013: a muted channel does not silence replies in your own threads
        return lvl, False

    # ---------------------------------------------------------------- flags
    def flag_recipients(self, m):
        """Who a flag names: the other person in a direct conversation, or the people mentioned."""
        if m.level != "flag":
            return set()
        named = set(m.mentions)
        if m.conv.startswith("dm:"):
            named |= set(m.conv[3:].split("|")) - {m.author}
        return named

    def _note_flag(self, m):
        named = self.flag_recipients(m)
        if m.group and not named:
            ruling("R03")
        if named and all(self.kind.get(x) == "agent" for x in named):
            ruling("R32")

    # ------------------------------------------------------------- classify
    def classify(self, u, m):
        """Where message m lands for person u right now: (bucket, reason, row key, flag kind) or None."""
        if m.deleted or m.streaming or m.author == u:
            return None
        P = self.people[u]
        is_dm = m.conv.startswith("dm:")
        in_dm = is_dm and u in m.conv[3:].split("|")
        if is_dm and not in_dm:
            return None
        dial = "default" if is_dm else P.ch_dial.get(m.conv, "default")
        member = in_dm or u in self.channels[m.conv]
        named = u in m.mentions
        is_reply = m.parent is not None
        root = m.parent or m.id
        who = m.author
        cands = []   # (bucket, reason, row key, flag kind)

        lvl, by_hand = (self.thread_level(u, m) if is_reply else ("off", False))

        if m.level == "flag":
            if u in self.flag_recipients(m):
                if "flags" not in P.rules:
                    ruling("R31")
                cands.append((FOCUS if "flags" in P.rules else NEEDS, f"Flagged for you by {who}", ("th", root), "named", "flag"))
            elif not self.flag_recipients(m):
                in_scope = (lvl != "off") if is_reply else (member and dial != "muted")
                if in_scope:
                    where = "the thread" if is_reply else m.conv
                    cands.append((NEEDS, f"Flagged for everyone in {where} by {who}", ("th", root), "open", "flag"))

        if in_dm:
            if self.kind[who] == "person":
                if "dm" not in P.rules:
                    ruling("R31")
                if "dm" in P.rules and who not in P.dm_removed:
                    P.dm_people.add(who)      # D-021: the option adds everyone who writes to you directly
                cands.append((FOCUS if who in P.dm_people else NEEDS, f"Direct message from {who}", ("dm", m.conv), None, "dm"))
            else:
                ruling("R04")
                cands.append((NEEDS, f"Direct message from agent {who}", ("dm", m.conv), None, "dm"))

        if named:
            cands.append((FOCUS if "mentions" in P.rules else NEEDS, f"{who} mentioned you", ("th", root), None, "mention"))

        if m.group and is_reply:
            ruling("R30")
        if m.group and not is_reply and member and dial != "muted":
            cands.append((FYI, f"{who} mentioned everyone in {m.conv}", ("th", root), None, "group"))

        if is_reply and lvl != "off":
            text = {"focus": "Thread is in Focus", "needs": "Reply in your thread", "fyi": "Reply in a thread you follow"}[lvl]
            cands.append((LEVEL[lvl], text, ("th", root), None, "reply"))

        if not is_reply and not is_dm:
            if member and dial == "focus":
                cands.append((FOCUS, f"{m.conv} is in Focus", ("ch", m.conv), None, "channel"))
            elif member and dial == "all":
                cands.append((FYI, f"{m.conv} is set to All activity", ("ch", m.conv), None, "channel"))
            elif not member and not named:
                ruling("R22")

        if not is_dm and member and dial == "focus":
            # D-001: a channel in Focus brings "everything addressed to you there (mentions, replies in
            # your threads, flags)". A thread level set by hand still wins for plain replies.
            cands = [c if (c[0] == FOCUS or (c[4] == "reply" and by_hand))
                     else (FOCUS, c[1] + f" · {m.conv} is in Focus", c[2], c[3], c[4]) for c in cands]
        # D-013: at All activity, what is addressed to you lands as at Default

        if not cands:
            return None
        # highest bucket wins; among equals prefer the addressed reason over the channel-wide one
        cands.sort(key=lambda c: (c[0], c[2][0] != "ch", c[3] is not None), reverse=True)
        bucket, reason, key, flag = cands[0][:4]
        flag = next((c[3] for c in cands if c[3]), None)
        if any(c[2][0] != "ch" for c in cands):
            key = next(c[2] for c in cands if c[2][0] != "ch")
        if in_dm:
            key = ("dm", m.conv)      # a direct conversation is one row, replies included

        if ("person", who) in P.rules and bucket < FOCUS:
            ruling("R05")
            bucket, reason = FOCUS, reason + f" · {who} is in your Focus"
        if m.level == "info" and bucket > FYI:
            bucket = FYI          # D-001: Info never lands higher than For your information
            reason += " · marked Info"
            if named and dial == "muted":
                ruling("R06")
        elif m.level == "info" and named and dial == "muted":
            ruling("R06")
        return bucket, reason, key, flag

    # --------------------------------------------------------------- deliver
    def _unread_bucket(self, u, row, skip=None):
        """Highest bucket among the row's unread items that could themselves have notified (not silent Info)."""
        live = [i.bucket for i in row.items.values() if i is not skip and not i.read and not i.retracted
                and not (i.kind == "msg" and self.msgs[i.msg].level == "info")] if row else []
        return max(live) if live else 0

    def watch(self, u, key, until):
        self.watching[u] = (key, until)

    def _notify(self, u, m, it, prev):
        """D-012: a row notifies when it becomes unread in a bucket that notifies, once; again if it moves up into Focus."""
        if m.level == "info" and it.kind == "msg":
            return                      # D-001: Info sends no notification
        row = self._row_of(u, m, it.kind)
        if row and (row.kept or row.flagged) and it.bucket < FOCUS:
            ruling("R38")
        for policy, least in (("A", FYI), ("B", NEEDS), ("C", FOCUS)):
            if it.bucket >= least and (prev < least or (it.bucket == FOCUS and prev < FOCUS)):
                self.notified[(u, policy)] += 1
                if policy == "B":
                    self.notifications.append((u, m.id, it.bucket, it.kind))

    def _deliver(self, u, m, cls, kind="msg", as_new=True):
        bucket, reason, key, flag = cls
        P = self.people[u]
        row = P.rows.get(key)
        new_row = row is None or self.row_bucket(u, row) is None
        prev = self._unread_bucket(u, row)
        if row is None:
            row = P.rows[key] = Row(key)
        if row.kept:
            ruling("R16")
        it = Item(m.id, kind, bucket, reason, self.now, flag=flag)
        row.items[(m.id, kind)] = it
        if flag == "named":
            row.flagged.add(m.id)      # D-011: a flag that names you keeps the row for you
            if "flags" not in P.rules:
                ruling("R37")
        row.last = self.now
        w = self.watching.get(u)
        if w and w[0] == key and self.now <= w[1]:
            it.read = True             # D-015: it arrived in a conversation open in front of them
            self.read_at_once += 1
            return it
        self._notify(u, m, it, prev)
        if P.view is not None:
            P.pending_new += 1
        if self.on_deliver:
            self.on_deliver(u, m, it, new_row)
        return it

    def _drop_item(self, u, row, it):
        row.items.pop((it.msg, it.kind), None)
        if not row.items and not row.kept:
            self.people[u].rows.pop(row.key, None)

    def _settle(self, u, key):
        """Nothing to do eagerly: where a row sits is derived. Kept for readability."""
        return

    # ------------------------------------------------------------ row status
    def row_bucket(self, u, row):
        """Bucket the row sits in now, or None when it belongs in Earlier."""
        if row.kept or row.hold or row.flagged:
            return FOCUS
        live = [i for i in row.items.values() if not i.read and not i.retracted]
        return max(i.bucket for i in live) if live else None

    def row_section(self, u, row):
        flag_read = row.flagged and all(row.items[(f, "msg")].read for f in row.flagged if (f, "msg") in row.items)
        if row.kept or flag_read:
            return "kept"          # new activity on a kept row shows inside Kept (R16)
        if any(not i.read and not i.retracted for i in row.items.values()):
            return "new"
        return "kept"              # read, and kept for you by a flag, or a marker held until Done

    def feed(self, u):
        """Rows per bucket, newest activity first. Focus is split into its sections."""
        P = self.people[u]
        out = {FOCUS: [], NEEDS: [], FYI: []}
        for row in P.rows.values():
            b = self.row_bucket(u, row)
            if b:
                out[b].append(row)
        for b in out:
            out[b].sort(key=lambda r: -r.last)
        return out

    def earlier(self, u):
        P = self.people[u]
        return [r for r in P.rows.values() if self.row_bucket(u, r) is None]

    def counts(self, u):
        f = self.feed(u)
        sec = Counter(self.row_section(u, r) for r in f[FOCUS])
        return {"focus_new": sec["new"], "focus_kept": sec["kept"],
                "needs": len(f[NEEDS]), "fyi": len(f[FYI])}

    # ---------------------------------------------------------------- events
    def post(self, author, conv, parent=None, mentions=(), group=False, level=None, streaming=False):
        m = Msg(self.next_id, conv, author, self.now, parent, set(mentions) - {author}, group, level, streaming)
        self.next_id += 1
        self.msgs[m.id] = m
        if parent is not None:
            self.threads[parent].append(m.id)
        if streaming:
            return m
        self._after_post(m)
        return m

    def finish_stream(self, m):
        # it arrives now: give it its place in the order, after replies made while it was streaming
        old = m.id
        del self.msgs[old]
        m.id = self.next_id
        self.next_id += 1
        self.msgs[m.id] = m
        if m.parent is not None:
            self.threads[m.parent] = [m.id if i == old else i for i in self.threads[m.parent] if i != old] + [m.id]
        m.streaming = False
        m.ts = self.now
        self._after_post(m)

    def _after_post(self, m):
        if m.level == "flag":
            self._note_flag(m)
        author = m.author
        root = m.parent or m.id
        kept_before = {u: {(r.key, f) for r in P.rows.values() for f in r.flagged}
                       for u, P in self.people.items() if u != m.author} if self.self_check else {}
        # the author acted: their own row for this conversation is read, a kept row is released,
        # and a thread they had turned off is on again (D-001, D-003)
        if author in self.people:
            P = self.people[author]
            if m.parent is not None and P.th_dial.get(root) == "off":
                P.th_dial[root] = "needs"     # D-001: replying in a thread you turned off sets it back to Needs you
            elif m.parent is not None and P.th_dial.get(root) == "fyi":
                ruling("R25")
            keys = []
            if m.parent is not None:
                keys.append(("th", root))
                r0 = self._row_of(author, self.msgs[root])
                if r0:
                    keys.append(r0.key)       # the message replied to may sit in a channel row
            if m.conv.startswith("dm:"):
                keys.append(("dm", m.conv))
            for key in keys:
                if key in P.rows:
                    for it in P.rows[key].items.values():
                        it.read = True
                    if P.rows[key].kept or len(P.rows[key].flagged) > 1:
                        ruling("R42")
                    if not P.only_done:           # D-023 point 26: with the setting on, only Done releases
                        P.rows[key].kept = P.rows[key].hold = False
                        P.rows[key].flagged.clear()   # D-011: replying releases what a flag kept
        for u in self.people:
            cls = self.classify(u, m)
            if cls:
                self._deliver(u, m, cls)
        for u, before in kept_before.items():
            now = {(r.key, f) for r in self.people[u].rows.values() for f in r.flagged}
            if not before <= now:
                self.violations.append(f"{u}: a message from {m.author} released a row a flag kept for them (D-011)")

    def react(self, u, mid, emoji="+1"):
        m = self.msgs[mid]
        if m.deleted:
            return
        m.reactions[u] = emoji
        # the person reacting acted on their own row
        if u in self.people:
            row = self._row_of(u, m)
            if row:
                for it in row.items.values():
                    if it.msg == mid:
                        it.read = True
                if row.kept:
                    ruling("R42")
                if not self.people[u].only_done:  # D-023 point 26
                    row.kept = row.hold = False
                    row.flagged.discard(mid)  # D-011: reacting releases what the flag kept
        a = m.author
        if a == u or a not in self.people:
            return
        P = self.people[a]
        root = m.parent or m.id
        is_dm = m.conv.startswith("dm:")
        key = ("dm", m.conv) if is_dm else ("th", root)
        if m.level == "flag" and not self.flag_recipients(m):
            ruling("R43")
        if self._addressed(m, u):
            # D-014: a reaction from someone you addressed is an answer; it lands in Needs you and names them
            if self.kind[u] == "agent":
                ruling("R34")
            elif P.th_dial.get(root) == "off":
                ruling("R18")
                return
            else:
                if not is_dm and P.ch_dial.get(m.conv, "default") == "muted":
                    ruling("R35")
                row = P.rows.get(key)
                if row:
                    row.items.pop((mid, f"answer:{u}"), None)
                self._deliver(a, m, (NEEDS, f"{u} reacted {emoji} to your message", key, None), kind=f"answer:{u}")
                return
        if not is_dm and P.ch_dial.get(m.conv, "default") == "muted" or P.th_dial.get(root) == "off":
            ruling("R18")
            return
        row = P.rows.get(key)
        it = row.items.get((mid, "reaction")) if row else None
        n = self._applause(m)
        if it:
            it.reason = f"{n} reacted to your message"
            wt = self.watching.get(a)
            if wt and wt[0] == key and self.now <= wt[1]:
                it.read, it.retracted = True, False              # D-015: the conversation is open in front of them
                self.read_at_once += 1
                return
            returned = self.row_bucket(a, row) is None
            prev = self._unread_bucket(a, row, skip=it)
            it.read, it.retracted, it.ts = False, False, self.now   # D-003: a row returns, unread, on new activity
            row.last = self.now
            self._notify(a, m, it, prev)
            if self.on_deliver and returned:
                self.on_deliver(a, m, it, True)
        else:
            self._deliver(a, m, (FYI, f"{n} reacted to your message", key, None), kind="reaction")

    def _addressed(self, m, u):
        """Did message m address u: a mention, or the other side of a direct conversation."""
        return u in m.mentions or (m.conv.startswith("dm:") and u in m.conv[3:].split("|") and u != m.author)

    def _applause(self, m):
        """Reactions counted in the grouped row: everyone except the author and people whose reaction is an answer."""
        return len([x for x in m.reactions if x != m.author
                    and not (self._addressed(m, x) and self.kind[x] == "person")])

    def unreact(self, u, mid):
        m = self.msgs[mid]
        if u not in m.reactions:
            return
        del m.reactions[u]
        a = m.author
        if a not in self.people:
            return
        row = self._row_of(a, m, kind=f"answer:{u}")
        if row:
            self._retract(a, row, row.items[(mid, f"answer:{u}")])   # D-004: taken back
            return
        row = self._row_of(a, m, kind="reaction")
        if not row:
            return
        it = row.items[(mid, "reaction")]
        n = self._applause(m)
        if n == 0:
            self._retract(a, row, it)      # D-004: when the last reaction goes, the row is taken back
        else:
            it.reason = f"{n} reacted to your message"

    def _row_of(self, u, m, kind="msg"):
        P = self.people[u]
        for row in P.rows.values():
            if (m.id, kind) in row.items:
                return row
        return None

    def _signature(self):
        return {(u, r.key, k): (i.bucket, i.read, i.retracted, r.kept, r.hold)
                for u, P in self.people.items() for r in P.rows.values() for k, i in r.items.items()}

    def edit(self, mid, mentions=None, level="same", group=None):
        m = self.msgs[mid]
        if m.deleted:
            return
        old_mentions, old_level, old_group = set(m.mentions), m.level, m.group
        old_recipients = self.flag_recipients(m)
        new_mentions = old_mentions if mentions is None else set(mentions) - {m.author}
        new_level = old_level if level == "same" else level
        new_group = old_group if group is None else group
        m.version += 1
        if (new_mentions, new_level, new_group) == (old_mentions, old_level, old_group):
            # D-004: an edit that only changes the text updates the row in place
            before = self._signature() if self.self_check else None
            for u in self.people:
                row = self._row_of(u, m)
                if row and not row.items[(mid, "msg")].retracted:
                    row.items[(mid, "msg")].edited = True
            if self.self_check and self._signature() != before:
                self.violations.append(f"text-only edit of {mid} moved, unread or created a row")
            return
        m.ever_mentioned = set(getattr(m, "ever_mentioned", set())) | old_mentions
        m.mentions, m.level, m.group = new_mentions, new_level, new_group
        new_recipients = self.flag_recipients(m)
        if m.level == "flag":
            self._note_flag(m)
        for u in self.people:
            row = self._row_of(u, m)
            it = row.items[(mid, "msg")] if row else None
            if it and it.retracted:
                it = None
            cls = self.classify(u, m)
            if row and u not in new_recipients:
                row.flagged.discard(mid)        # D-011: if the flag is removed, what it kept is released
            newly_named = (u in m.mentions and u not in old_mentions) or (u in new_recipients and u not in old_recipients)
            as_new = newly_named or (old_level == "info" and m.level != "info")   # D-004, D-013
            if it is None and cls:
                if row and (mid, "msg") in row.items:
                    row.items.pop((mid, "msg"))
                self._deliver(u, m, cls)        # D-004: as if named from the start
            elif it and not cls:
                self._retract(u, row, it)
            elif it and cls:
                bucket, reason, key, flag = cls
                it.edited = True
                it.flag = flag
                if flag == "named":
                    row.flagged.add(mid)
                if as_new:
                    prev = self._unread_bucket(u, row, skip=it)
                    it.read = False
                    it.bucket, it.reason, it.ts = bucket, reason, self.now
                    row.last = self.now
                    ruling("R36" if newly_named else "R12")
                    self._notify(u, m, it, prev)
                else:
                    if bucket < it.bucket and not it.read:
                        ruling("R13")
                    it.bucket, it.reason = bucket, reason

    def delete(self, mid):
        m = self.msgs[mid]
        if m.deleted:
            return
        expect = {}
        if self.self_check:
            for u in self.people:
                row = self._row_of(u, m)
                if row and not row.items[(mid, "msg")].retracted:
                    it = row.items[(mid, "msg")]
                    if row.kept:
                        expect[u] = "held marker"
                    elif self._on_screen(u, row, it) or it.read:
                        expect[u] = "marker"
                    else:
                        expect[u] = "nothing"
        m.deleted = True
        if m.parent is None and self.threads[mid]:
            self.tombstones += 1            # D-004: the marker stays so the thread keeps its top
        for u, P in self.people.items():
            for row in list(P.rows.values()):
                for k, it in list(row.items.items()):
                    if it.msg == mid:
                        row.flagged.discard(mid)    # D-011: deleting the message removes its flag too
                        self._retract(u, row, it)
        for u, want in expect.items():
            row = self._row_of(u, m)
            it = row.items.get((mid, "msg")) if row else None
            got = "nothing" if not it else ("held marker" if it.retracted and (row.hold or row.kept) else "marker" if it.retracted else "live")
            if got != want:
                self.violations.append(f"delete of {mid}: {u} should see {want}, sees {got}")

    def _retract(self, u, row, it):
        """D-004, option C: what is left when something aimed at a person is taken back."""
        if it.retracted:
            return
        if row.kept:
            it.retracted, it.read = True, True
            row.hold = True                 # marker stays in Focus until Done
        elif self._on_screen(u, row, it):
            it.retracted = True             # marker in the same space until the person moves on
        elif it.read:
            it.retracted = True             # marker in the Earlier list
        else:
            self._drop_item(u, row, it)     # never seen: nothing (D-004, also for a flag never read)
        it.flag = None

    def _on_screen(self, u, row, it):
        P = self.people[u]
        return P.view is not None and it.ts <= P.view_ts and any(row.key in keys for keys in P.view.values())

    # ------------------------------------------------------- the person acts
    def open_feed(self, u):
        P = self.people[u]
        f = self.feed(u)
        P.view = {b: [r.key for r in rows] for b, rows in f.items()}
        P.view_ts = self.now
        P.pending_new = 0
        return f

    def visible(self, u):
        """What is on screen while the feed is open: the snapshot, in its original order (D-003)."""
        P = self.people[u]
        return {b: [k for k in keys] for b, keys in P.view.items()}

    def leave_feed(self, u):
        P = self.people[u]
        P.view = None
        P.pending_new = 0
        for row in list(P.rows.values()):
            for it in list(row.items.values()):
                if it.retracted and not it.read and not row.hold:
                    self._drop_item(u, row, it)   # was on screen unread; gone after moving on

    def read_row(self, u, key):
        row = self.people[u].rows.get(key)
        if row:
            for it in row.items.values():
                it.read = True

    def view_bucket(self, u, bucket=FYI):
        """D-003: in For your information, viewing the bucket is enough."""
        for row in self.feed(u)[bucket]:
            self.read_row(u, row.key)

    def mark_all_read(self, u, bucket):
        ruling("R23")
        for row in self.feed(u)[bucket]:
            self.read_row(u, row.key)

    def read_conversation(self, u, root):
        if ("th", root) in self.people[u].rows:
            ruling("R19")
            self.read_row(u, ("th", root))

    def keep(self, u, key):
        row = self.people[u].rows.get(key)
        if row:
            row.kept = True
            for it in row.items.values():
                it.read = True          # D-015: acting implies read

    def done(self, u, key):
        row = self.people[u].rows.get(key)
        if row:
            row.kept = row.hold = False
            row.flagged.clear()
            for it in row.items.values():
                it.read = True

    # ------------------------------------------------------- Focus people (D-021)
    def remove_dm_person(self, u, who):
        P = self.people[u]
        P.dm_people.discard(who); P.dm_removed.add(who)
        self.resort(u)

    def add_dm_person(self, u, who):
        P = self.people[u]
        P.dm_removed.discard(who); P.dm_people.add(who)
        self.resort(u)

    def dm_option_off(self, u, remove_all=False):
        """Switching the option off stops adding; those already added stay unless the person says otherwise."""
        P = self.people[u]
        P.rules.discard("dm")
        if remove_all:
            P.dm_removed |= P.dm_people; P.dm_people = set()
        self.resort(u)

    def resort(self, u):
        """D-013: a dial or rule change re-sorts unread rows already in the feed; read ones stay."""
        P = self.people[u]
        QUIET[0] = True
        read_before = {(r.key, k): (i.bucket, i.reason) for r in P.rows.values() for k, i in r.items.items() if i.read}
        lifted = False
        try:
            for row in list(P.rows.values()):
                for it in list(row.items.values()):
                    if it.read or it.retracted:
                        continue
                    m = self.msgs[it.msg]
                    if it.kind == "msg":
                        cls = self.classify(u, m)
                        if cls:
                            lifted = lifted or (cls[0] == FOCUS and it.bucket < FOCUS)
                            it.bucket, it.reason = cls[0], cls[1]
                        else:
                            row.flagged.discard(it.msg)
                            self._drop_item(u, row, it)
                    elif it.kind == "reaction":
                        root = m.parent or m.id
                        if (not m.conv.startswith("dm:") and P.ch_dial.get(m.conv, "default") == "muted") \
                                or P.th_dial.get(root) == "off":
                            self._drop_item(u, row, it)     # R18: reactions follow the dial
        finally:
            QUIET[0] = False
        if lifted:
            ruling("R40")
        read_after = {(r.key, k): (i.bucket, i.reason) for r in P.rows.values() for k, i in r.items.items() if i.read}
        if self.self_check and read_before != read_after:
            self.violations.append(f"{u}: a dial or rule change moved a row they had already read (D-013)")

    def set_channel_dial(self, u, ch, level):
        self.people[u].ch_dial[ch] = level
        self.resort(u)

    def set_thread_dial(self, u, root, level):
        self.people[u].th_dial[root] = level
        self.resort(u)

    def add_rule(self, u, rule):
        self.people[u].rules.add(rule)
        self.resort(u)

    def remove_rule(self, u, rule):
        self.people[u].rules.discard(rule)
        self.resort(u)

    def history(self, u, tab):
        """D-016: the history tabs. Read and unread together, newest first, the current truth, no buckets or dials."""
        out = []
        P = self.people[u]
        QUIET[0] = True
        try:
            for m in self.msgs.values():
                if m.deleted or m.streaming:
                    continue
                dm_mine = m.conv.startswith("dm:") and u in m.conv[3:].split("|")
                if m.conv.startswith("dm:") and not dm_mine:
                    continue
                if tab == "reactions":
                    if m.author == u and any(x != u for x in m.reactions):
                        out.append(m.id)
                elif m.author == u:
                    continue
                elif tab == "mentions" and u in m.mentions:
                    out.append(m.id)
                elif tab == "dms" and dm_mine:
                    out.append(m.id)
                elif tab == "threads" and m.parent is not None:
                    lvl = P.th_dial.get(m.parent) or self.auto_level(u, m.parent, before=m.id)
                    if lvl != "off":
                        out.append(m.id)
        finally:
            QUIET[0] = False
        return sorted(out, reverse=True)

    def rule_preview(self, u, rule, since=0.0):
        """D-001: how many items the rule would have pulled in recently."""
        P = self.people[u]
        saved = set(P.rules)
        n = 0
        QUIET[0] = True
        for m in self.msgs.values():
            if m.ts < since:
                continue
            P.rules = set(saved)
            before = self.classify(u, m)
            P.rules = saved | {rule}
            after = self.classify(u, m)
            if after and after[0] == FOCUS and (not before or before[0] < FOCUS):
                n += 1
        P.rules = saved
        QUIET[0] = False
        return n

    # ------------------------------------------------------------ invariants
    def check(self):
        """Things the decisions say must always hold. Returns a list of violations found now.
        Violations caught as they happen (self-checks) accumulate in self.violations."""
        QUIET[0] = True
        try:
            return self._check()
        finally:
            QUIET[0] = False

    def _check(self):
        bad = []
        for u, P in self.people.items():
            seen = set()
            for row in P.rows.values():
                b = self.row_bucket(u, row)
                if b == FOCUS and not (row.kept or row.hold or row.flagged) and all(
                        i.kind != "msg" for i in row.items.values() if not i.read and not i.retracted):
                    bad.append(f"{u}: a row sits in Focus because of reactions alone (D-014)")
                for fid in row.flagged:
                    fm = self.msgs[fid]
                    if fm.deleted or fm.level != "flag" or u not in self.flag_recipients(fm):
                        bad.append(f"{u}: a row is still kept by flag {fid}, which is gone (D-011)")
                for it in row.items.values():
                    m = self.msgs[it.msg]
                    k = (it.msg, it.kind)
                    if k in seen:
                        bad.append(f"{u}: message {it.msg} appears in two rows (one item, one place)")
                    seen.add(k)
                    if it.kind == "msg" and m.author == u:
                        bad.append(f"{u}: own message {it.msg} is in their feed")
                    if it.kind == "reaction" and it.bucket != FYI:
                        bad.append(f"{u}: a reaction row sits above FYI")
                    if it.kind.startswith("answer:") and it.bucket != NEEDS:
                        bad.append(f"{u}: an answer by reaction is not in Needs you (D-014)")
                    if it.kind.startswith("answer:") and not it.retracted and it.kind[7:] not in m.reactions:
                        bad.append(f"{u}: an answer by reaction is shown but the reaction is gone")
                    if m.level == "info" and it.kind == "msg" and it.bucket > FYI and not it.retracted:
                        bad.append(f"{u}: Info message {it.msg} sits above FYI")
                    if m.streaming:
                        bad.append(f"{u}: streaming message {it.msg} reached the feed")
                    if m.deleted and not it.retracted:
                        bad.append(f"{u}: deleted message {it.msg} still shows as live")
                    if it.kind == "msg" and not it.read and not it.retracted and "mentioned you" in it.reason \
                            and u not in m.mentions:
                        bad.append(f"{u}: unread row says 'mentioned you' but message {it.msg} no longer does")
                    if it.retracted and not it.read and not self._on_screen(u, row, it) and not row.hold and not row.kept:
                        bad.append(f"{u}: a marker is left for something they never saw ({it.msg})")
            for m in self.msgs.values():
                if m.deleted or m.streaming or m.author == u:
                    continue
                if m.conv.startswith("dm:") and u not in m.conv[3:].split("|"):
                    continue
                if u in m.mentions and (m.id, "msg") not in seen:
                    bad.append(f"{u}: message {m.id} mentions them but never reached the feed (names you gets through)")
                if m.level == "flag" and u in self.flag_recipients(m) and (m.id, "msg") not in seen:
                    bad.append(f"{u}: flag {m.id} names them but never reached the feed")
                if m.level == "flag" and not self.flag_recipients(m):
                    cls = self.classify(u, m)
                    if cls and cls[3] == "open" and (m.id, "msg") not in seen:
                        bad.append(f"{u}: flag for everyone {m.id} is in scope but never reached their feed")
        for (u, mid, bucket, kind) in self.notifications:
            if bucket < NEEDS:
                bad.append(f"{u}: a notification was sent for a For your information item (D-012)")
            if kind == "msg" and mid in self.msgs and self.msgs[mid].level == "info" and self.msgs[mid].version == 1:
                bad.append(f"{u}: a notification was sent for an Info message (D-001)")
        return bad
