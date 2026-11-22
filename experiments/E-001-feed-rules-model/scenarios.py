"""E-001: the situations from D-010, plus edge cases, played through the model.

Each case states what the decision records say should happen and checks the model does it.
'basis' names the decision, or the ruling (Rnn) the model had to invent.
"""
import sys
from model import World, FOCUS, NEEDS, FYI, BUCKET, RULINGS

results = []


def case(name, got, expected, basis):
    ok = got == expected
    results.append((ok, name, got, expected, basis))


def team():
    w = World()
    for p in ["you", "anna", "marco", "priya", "sam", "lena"]:
        w.add_person(p)
    for a in ["release", "build", "research"]:
        w.add_agent(a)
    w.add_channel("#launch", ["you", "anna", "marco", "priya", "sam", "release"])
    w.add_channel("#product", ["you", "anna", "marco", "research"])
    w.add_channel("#incidents", ["you", "sam", "lena", "build"])
    w.add_channel("#infra", ["you", "sam", "lena", "marco", "build"])
    w.add_channel("#random", ["you", "anna", "marco", "priya", "sam", "lena"])
    return w


def where(w, u, m, kind="msg"):
    """Bucket name the row holding message m sits in for u, 'Earlier', or 'nothing'."""
    row = w._row_of(u, m, kind)
    if not row:
        return "nothing"
    b = w.row_bucket(u, row)
    if b is None:
        return "Earlier" + (" (marker)" if row.items[(m.id, kind)].retracted else "")
    s = BUCKET[b]
    if b == FOCUS:
        s += "/" + w.row_section(u, row)
    if row.items[(m.id, kind)].retracted:
        s += " (marker)"
    return s


def mine(w, u="you"):
    """Notifications sent to u under the accepted default (D-012, option B)."""
    return len([n for n in w.notifications if n[0] == u])


def tick(w, dt=1.0):
    w.now += dt


# ---------------------------------------------------------------- starting rules
w = team()
m = w.post("anna", w.dm("anna", "you"))
case("A person sends you a direct message", where(w, "you", m), "Focus/new", "D-001 starting rules")
m = w.post("release", w.dm("release", "you"))
case("An agent sends you a direct message, no flag", where(w, "you", m), "Needs you", "R04")
m = w.post("marco", "#product", mentions=["you"])
case("Someone mentions you in a channel", where(w, "you", m), "Needs you", "D-001 starting rules")
m = w.post("marco", "#random", group=True)
case("Someone mentions the whole channel", where(w, "you", m), "FYI", "D-001 starting rules")
m = w.post("marco", "#random")
case("Ordinary channel traffic not addressed to you", where(w, "you", m), "nothing", "D-001")
m = w.post("anna", "#product", mentions=["you"], level="info")
case("A courtesy mention marked Info", where(w, "you", m), "FYI", "D-001 Info")

# ---------------------------------------------------------------- flags (D-011)
w = team()
f = w.post("anna", "#launch", mentions=["you"], level="flag")
case("A person flags a message that mentions you", where(w, "you", f), "Focus/new", "D-001 flags")
w.read_row("you", ("th", f.id))
case("…you read it: it is kept for you automatically", where(w, "you", f), "Focus/kept", "D-011")
tick(w); w.post("you", "#launch", parent=f.id)
case("…you reply: the row is released", where(w, "you", f), "Earlier", "D-011")

f = w.post("anna", "#launch", mentions=["you"], level="flag"); w.read_row("you", ("th", f.id))
w.done("you", ("th", f.id))
case("A flag names you and you will not answer: Done releases it", where(w, "you", f), "Earlier", "D-011 (closes the hole)")

f = w.post("anna", "#launch", mentions=["you", "marco"], level="flag")
w.read_row("you", ("th", f.id)); w.read_row("marco", ("th", f.id))
tick(w); w.react("marco", f.id); w.post("marco", "#launch", parent=f.id)
case("A flag names two people; the other one reacts and replies: nothing changes for you", where(w, "you", f), "Focus/kept", "D-011 no shared state")
case("…and it is released for them", where(w, "marco", f), "Earlier", "D-011")
tick(w); w.react("you", f.id)
case("…you react: released for you", where(w, "you", f), "FYI", "D-011 (the row holds marco's unread reply, at the thread's level: mentioned)")

f = w.post("build", "#infra", level="flag")
case("An agent flags a message that names nobody: channel members get it in Needs you", where(w, "you", f), "Needs you", "D-001 flags")
case("…a person outside the channel gets nothing", where(w, "anna", f), "nothing", "D-001 + R22")
tick(w); w.post("lena", "#infra", parent=f.id)
case("…someone answers: it is still there for a person who had not read it", where(w, "you", f), "Needs you", "D-011 no shared state")
w.read_row("you", ("th", f.id))
case("…and leaves once you read it, answered or not", where(w, "you", f), "Earlier", "D-011")

f = w.post("anna", "#launch", group=True, level="flag")
case("A flag with only a whole-channel mention is treated as naming nobody", where(w, "you", f), "Needs you", "R03")

f = w.post("anna", w.dm("anna", "you"), level="flag")
case("A flag in a direct conversation is for the other person", where(w, "you", f), "Focus/new", "D-001 flags")
w.read_row("you", ("dm", w.dm("anna", "you")))
case("…read: kept for you", where(w, "you", f), "Focus/kept", "D-011")
tick(w); w.post("you", w.dm("anna", "you"))
case("…you write back: released", where(w, "you", f), "Earlier", "D-011")

f = w.post("anna", "#launch", mentions=["you"], level="flag"); w.read_row("you", ("th", f.id))
w.edit(f.id, level=None)
case("The sender removes the flag after you read it: what it kept is released", where(w, "you", f), "Earlier", "D-011")
f = w.post("anna", "#launch", mentions=["you"], level="flag")
w.edit(f.id, level=None)
case("The sender removes the flag before you read it: an ordinary mention", where(w, "you", f), "Needs you", "D-011")
f = w.post("anna", "#launch", mentions=["you"], level="flag"); w.read_row("you", ("th", f.id)); w.keep("you", ("th", f.id))
w.edit(f.id, level=None)
case("The flag is removed from a row you had also kept yourself: it stays kept", where(w, "you", f), "Focus/kept", "D-011")

# ---------------------------------------------------------------- agents
w = team()
root = w.post("you", "#launch", mentions=["release"])
s = w.post("release", "#launch", parent=root.id, streaming=True)
case("An agent's reply is still streaming in", where(w, "you", s), "nothing", "D-001")
p = w.post("release", "#launch", parent=root.id, level="info")
case("An agent posts a finished progress update (Info) in your thread", where(w, "you", p), "FYI", "D-001 Info")
w.finish_stream(s)
case("The streamed reply finishes: a result in your thread", where(w, "you", s), "Needs you", "D-001 thread dial")
a = w.post("release", "#launch", parent=root.id, mentions=["you"], level="flag")
case("The agent stops for your approval and flags you", where(w, "you", a), "Focus/new", "D-001 flags")
case("…the whole thread is one row, sitting in the highest bucket any of its items earns",
     len([r for r in w.people["you"].rows.values() if w.row_bucket("you", r)]), 1, "D-001 one row per conversation")
tick(w); w.react("you", a.id)
case("…you react to approve: released; the unread result keeps the row in Needs you",
     where(w, "you", s), "Needs you", "D-011 (observation: a row can move down a bucket)")

# ---------------------------------------------------------------- threads
w = team()
t = w.post("you", "#product")
r = w.post("marco", "#product", parent=t.id)
case("A reply in a thread you started", where(w, "you", r), "Needs you", "D-001 thread dial")
t2 = w.post("anna", "#product")
r = w.post("marco", "#product", parent=t2.id)
case("A reply in someone else's thread you are not part of", where(w, "you", r), "nothing", "D-001 thread dial: Off")
w.set_thread_dial("you", t2.id, "fyi")
r = w.post("marco", "#product", parent=t2.id)
case("…after you click Follow", where(w, "you", r), "FYI", "D-001 Follow")
w.set_thread_dial("you", t.id, "off")
r = w.post("marco", "#product", parent=t.id)
case("A reply in your own thread after you turned it off", where(w, "you", r), "nothing", "D-001 Off")
r = w.post("marco", "#product", parent=t.id, mentions=["you"])
case("…a reply there that mentions you still arrives", where(w, "you", r), "Needs you", "D-001 names you gets through")
case("…and the thread stays off", w.people["you"].th_dial.get(t.id), "off", "D-001")
w.post("you", "#product", parent=t.id)
case("…you reply in it: the thread is back at Needs you", w.people["you"].th_dial.get(t.id), "needs", "D-001")
t3 = w.post("anna", "#product", mentions=["you"])
w.read_row("you", ("th", t3.id))
r = w.post("marco", "#product", parent=t3.id)
case("A reply under a message that mentioned you", where(w, "you", r), "FYI", "D-001 thread dial: mentioned")
w.set_thread_dial("you", t3.id, "focus")
r = w.post("marco", "#product", parent=t3.id)
case("A reply in a thread you put in Focus", where(w, "you", r), "Focus/new", "D-001 thread in Focus")
r = w.post("marco", "#product", parent=t3.id, level="info")
case("…an Info reply there still lands no higher than FYI", w._row_of("you", r).items[(r.id, "msg")].bucket, FYI, "D-001 Info")

# ---------------------------------------------------------------- channel dial
for level, expect_post, expect_reply in [("focus", "Focus/new", "nothing"), ("all", "FYI", "nothing"),
                                         ("default", "nothing", "nothing"), ("muted", "nothing", "nothing")]:
    w = team()
    w.set_channel_dial("you", "#incidents", level)
    posts = [w.post("build", "#incidents") for _ in range(6)]
    case(f"#incidents at {level}: a burst of six posts", where(w, "you", posts[0]), expect_post, "D-001 channel dial")
    if level in ("focus", "all"):
        case(f"#incidents at {level}: the burst is one row", len(w.people["you"].rows), 1, "D-001 one row per conversation")
    r = w.post("sam", "#incidents", parent=posts[0].id)
    case(f"#incidents at {level}: a reply in a thread you are not part of", where(w, "you", r), expect_reply, "D-001 channel in Focus")
    mm = w.post("sam", "#incidents", mentions=["you"])
    case(f"#incidents at {level}: a post that mentions you", where(w, "you", mm),
         "Focus/new" if level == "focus" else "Needs you", "D-001 names you gets through")

w = team()
w.set_channel_dial("you", "#launch", "focus")
t = w.post("you", "#launch"); r = w.post("anna", "#launch", parent=t.id)
case("Channel in Focus: a reply in your own thread there", where(w, "you", r), "Focus/new", "D-001 'everything addressed to you there'")
w.set_thread_dial("you", t.id, "fyi")
r = w.post("anna", "#launch", parent=t.id)
case("…but you set that thread to For your information yourself: the thread wins",
     w._row_of("you", r).items[(r.id, "msg")].bucket, FYI, "D-001 thread dial wins")
w = team()
w.set_channel_dial("you", "#launch", "muted")
t = w.post("you", "#launch"); r = w.post("anna", "#launch", parent=t.id)
case("Muted channel: a reply in your own thread there still reaches you", where(w, "you", r), "Needs you", "D-013")
t2 = w.post("anna", "#launch"); w.set_thread_dial("you", t2.id, "fyi")
r = w.post("marco", "#launch", parent=t2.id)
case("Muted channel: a reply in a thread you followed by hand", where(w, "you", r), "FYI", "D-001 thread dial wins")
i = w.post("anna", "#launch", mentions=["you"], level="info")
case("Muted channel: an Info message that mentions you", where(w, "you", i), "FYI", "R06")
w = team()
w.set_channel_dial("you", "#launch", "focus")
i = w.post("anna", "#launch", level="info")
case("Channel in Focus: an Info post there", where(w, "you", i), "FYI", "D-001 Info wins over Focus")

# ---------------------------------------------------------------- edits and deletes
w = team()
m = w.post("anna", "#launch")
w.edit(m.id, mentions=["you"])
case("An edit adds a mention of you", where(w, "you", m), "Needs you", "D-004")
w.edit(m.id, mentions=[])
case("…a second edit removes it before you read it: no trace", where(w, "you", m), "nothing", "D-004 never seen")
m = w.post("anna", "#launch", mentions=["you"]); w.read_row("you", ("th", m.id))
w.edit(m.id, mentions=[])
case("An edit removes your mention after you read it: marker in Earlier", where(w, "you", m), "Earlier (marker)", "D-004 read earlier")
m = w.post("anna", "#launch", mentions=["you"]); w.read_row("you", ("th", m.id))
w.edit(m.id)
case("A text-only edit of a row you read does not make it unread", where(w, "you", m), "Earlier", "D-004")
m = w.post("anna", "#launch", mentions=["you"])
w.delete(m.id)
case("A message that mentioned you is deleted before you saw it", where(w, "you", m), "nothing", "D-004 never seen")
m = w.post("anna", "#launch", mentions=["you"])
w.open_feed("you")
before = w.visible("you")
w.delete(m.id)
case("…deleted while its row is on your screen: a marker in the same space", where(w, "you", m), "Earlier (marker)", "D-004 on screen")
case("…and the list on screen has not changed", w.visible("you"), before, "D-003 list does not move")
w.leave_feed("you")
case("…after you move on it is gone", where(w, "you", m), "nothing", "D-004")
m = w.post("anna", "#launch", mentions=["you"]); w.read_row("you", ("th", m.id)); w.keep("you", ("th", m.id))
case("You keep a row", where(w, "you", m), "Focus/kept", "D-003 Keep")
w.delete(m.id)
case("…its message is deleted: the marker stays in Focus", where(w, "you", m), "Focus/kept (marker)", "D-004 kept")
w.done("you", ("th", m.id))
case("…until you mark it done", where(w, "you", m), "Earlier (marker)", "D-004")
f = w.post("anna", "#launch", mentions=["you"], level="flag")
w.delete(f.id)
case("A flag for you is withdrawn before you read it", where(w, "you", f), "nothing", "D-004 decision: never seen, no marker")
f = w.post("anna", "#launch", mentions=["you"], level="flag"); w.read_row("you", ("th", f.id))
w.delete(f.id)
case("A flagged message you had read is deleted: released, with a marker in Earlier",
     where(w, "you", f), "Earlier (marker)", "D-011 + D-004 read earlier")
t = w.post("anna", "#launch"); w.post("marco", "#launch", parent=t.id)
w.delete(t.id)
case("A deleted message with replies leaves a marker in the conversation", w.tombstones, 1, "D-004")
m = w.post("anna", "#launch", mentions=["you"], level="info")
w.edit(m.id, level=None)
case("An edit removes Info from a message that mentions you: it moves up, quietly",
     where(w, "you", m), "Needs you", "D-004 current truth")

# ---------------------------------------------------------------- reactions
w = team()
m = w.post("you", "#launch")
for p in ["anna", "marco", "priya"]:
    w.react(p, m.id)
row = w._row_of("you", m, "reaction")
case("Three people react to your message: one grouped row", (where(w, "you", m, "reaction"), row.items[(m.id, "reaction")].reason),
     ("FYI", "3 reacted to your message"), "D-001 grouped reactions")
w.unreact("anna", m.id)
case("…one reaction is taken back: the count follows", row.items[(m.id, "reaction")].reason, "2 reacted to your message", "D-004")
w.unreact("marco", m.id); w.unreact("priya", m.id)
case("…the last one goes before you saw the row: no trace", where(w, "you", m, "reaction"), "nothing", "D-004")

# ---------------------------------------------------------------- reading, keeping, the stable list
w = team()
m = w.post("marco", "#product", mentions=["you"])
w.open_feed("you"); snap = w.visible("you")
w.read_row("you", ("th", m.id))
m2 = w.post("anna", "#product", mentions=["you"])
case("While the feed is open, a new mention does not change what is on screen", w.visible("you"), snap, "D-003")
case("…it is announced as '1 new'", w.people["you"].pending_new, 1, "D-003")
w.leave_feed("you")
case("A row you read is in Earlier on your next visit", where(w, "you", m), "Earlier", "D-003")
tick(w); r = w.post("marco", "#product", parent=m.id)
case("…and returns when something new happens in that conversation", where(w, "you", r), "FYI", "D-003 (the thread dial is 'mentioned', so FYI)")
w.keep("you", ("th", m.id))
case("You keep a row that came from another bucket: it lives in Focus", where(w, "you", m), "Focus/kept", "D-003")
tick(w); w.post("you", "#product", parent=m.id)
case("…replying to it releases it", where(w, "you", m), "Earlier", "D-003")

w = team()
w.add_rule("you", "mentions")
m = w.post("marco", "#product", mentions=["you"])
case("With the suggested rule 'any direct mention' added", where(w, "you", m), "Focus/new", "D-001 suggested rule")
w = team()
for _ in range(5):
    w.post("anna", "#launch", mentions=["you"])
w.post("anna", "#random"); w.post("marco", "#launch", mentions=["you"])
case("Before saving a rule for a person, the product can say how many items it would have pulled in",
     w.rule_preview("you", ("person", "anna")), 5, "D-001 count before saving + R05")

# ---------------------------------------------------------------- cases found by the independent check
w = team()
w.set_channel_dial("you", "#incidents", "focus")
t = w.post("sam", "#incidents"); w.read_row("you", ("ch", "#incidents"))
r = w.post("lena", "#incidents", parent=t.id, mentions=["you"])
case("Channel in Focus: a mention of you inside a thread you are not part of", where(w, "you", r), "Focus/new", "D-001 'everything addressed to you there'")
f = w.post("build", "#incidents", level="flag")
tick(w); w.post("sam", "#incidents", parent=f.id)
case("Channel in Focus: a flag for everyone is answered by someone else; the post still belongs in Focus",
     where(w, "you", f), "Focus/new", "D-001 channel in Focus brings every new top-level message")
w = team()
f = w.post("build", "#infra", level="flag"); w.read_row("you", ("th", f.id))
w.edit(f.id)
case("A flag for everyone you already read gets a typo fix: it does not come back", where(w, "you", f), "Earlier", "D-004 text-only edit")
w = team()
w.set_channel_dial("you", "#incidents", "focus")
p = w.post("build", "#incidents"); w.read_row("you", ("ch", "#incidents")); w.keep("you", ("ch", "#incidents"))
tick(w); w.post("you", "#incidents", parent=p.id)
case("You keep a channel row, then reply to a post in it: the row is released", where(w, "you", p), "Earlier", "D-003 reply releases a kept row")
w = team()
t = w.post("you", "#product"); w.post("marco", "#product", parent=t.id); w.delete(t.id)
r = w.post("anna", "#product", parent=t.id)
case("You delete the message that started a thread; a later reply still reaches you", where(w, "you", r), "Needs you", "R27")
w = team()
w.set_channel_dial("you", "#launch", "muted")
t = w.post("anna", "#launch"); w.set_thread_dial("you", t.id, "off")
w.post("you", "#launch", parent=t.id); r = w.post("marco", "#launch", parent=t.id)
case("Muted channel, a thread you turned off, then you reply in it: the next reply arrives", where(w, "you", r), "Needs you", "D-001 replying sets it back to Needs you; the thread dial wins")
w = team()
w.set_channel_dial("you", "#launch", "all")
m = w.post("anna", "#launch", mentions=["you"])
case("Channel at All activity: a post that mentions you lands as at Default", where(w, "you", m), "Needs you", "D-013")

# ---------------------------------------------------------------- dial and rule changes re-sort unread rows (D-013)
w = team()
m = w.post("marco", "#product", mentions=["you"]); m2 = w.post("anna", "#product", mentions=["you"])
w.read_row("you", ("th", m2.id))
w.add_rule("you", "mentions")
case("You add the rule 'any direct mention': an unread mention already in the feed moves to Focus", where(w, "you", m), "Focus/new", "D-013")
case("…a read one stays where it was", where(w, "you", m2), "Earlier", "D-013")
w = team()
w.set_channel_dial("you", "#incidents", "all")
p = w.post("build", "#incidents"); mm = w.post("sam", "#incidents", mentions=["you"])
w.set_channel_dial("you", "#incidents", "muted")
case("You mute a channel: its unread, unaddressed row leaves", where(w, "you", p), "nothing", "D-013")
case("…what names you stays", where(w, "you", mm), "Needs you", "D-001 names you gets through")
w = team()
t = w.post("anna", "#product"); w.set_thread_dial("you", t.id, "fyi"); r = w.post("marco", "#product", parent=t.id)
w.set_thread_dial("you", t.id, "focus")
case("You raise a followed thread to Focus: its unread reply moves up", where(w, "you", r), "Focus/new", "D-013")

# ---------------------------------------------------------------- reactions as answers (D-014)
w = team()
m = w.post("you", "#launch", mentions=["marco"], level="flag")
w.react("anna", m.id, "tada"); w.react("marco", m.id, "check")
row = w._row_of("you", m, "answer:marco")
case("You flag Marco; he reacts: an answer, in Needs you, with his name",
     (where(w, "you", m, "answer:marco"), row.items[(m.id, "answer:marco")].reason), ("Needs you", "marco reacted check to your message"), "D-014")
case("…Anna's reaction on the same message stays grouped in For your information, not counting Marco",
     row.items[(m.id, "reaction")].reason, "1 reacted to your message", "D-014")
w.unreact("marco", m.id)
case("…Marco takes his reaction back before you saw it", (m.id, "answer:marco") in row.items, False, "D-004")
w = team()
m = w.post("you", w.dm("you", "anna"))
w.react("anna", m.id, "+1")
case("Anna reacts to your direct message: an answer", where(w, "you", m, "answer:anna"), "Needs you", "D-014")
w = team()
w.set_channel_dial("you", "#launch", "focus")
m = w.post("you", "#launch", mentions=["marco"]); w.react("marco", m.id)
case("An answer by reaction in a channel you put in Focus still lands in Needs you", where(w, "you", m, "answer:marco"), "Needs you", "D-014 reactions never land in Focus")
w = team()
m = w.post("you", "#launch", mentions=["release"]); w.react("release", m.id, "eyes")
case("An agent you mentioned reacts: grouped, in For your information", where(w, "you", m, "reaction"), "FYI", "R34")

# ---------------------------------------------------------------- read (D-015) and notifications (D-012)
w = team()
t = w.post("you", "#product"); w.watch("you", ("th", t.id), w.now + 5)
r = w.post("marco", "#product", parent=t.id)
case("A reply arrives in a thread open in front of you: read at once, no feed row", where(w, "you", r), "Earlier", "D-015")
case("…and no notification", mine(w), 0, "D-015 + D-012")
w = team()
t = w.post("you", "#product")
r1 = w.post("marco", "#product", parent=t.id); r2 = w.post("anna", "#product", parent=t.id)
case("Two replies in your thread before you read it: one notification, not two", mine(w), 1, "D-012 one per row")
f = w.post("anna", "#product", parent=t.id, mentions=["you"], level="flag")
case("…then a flag for you in the same unread thread: the row moves up to Focus and notifies once more", mine(w), 2, "D-012")
w.read_row("you", ("th", t.id)); w.post("marco", "#product", parent=t.id)
case("…you read it, then a new reply: a new notification", mine(w), 3, "D-012")
w = team()
m = w.post("you", "#launch"); w.react("anna", m.id); w.post("marco", "#random", group=True)
p = w.post("anna", "#product", mentions=["you"], level="info")
case("Reactions, a whole-channel mention and an Info mention: no notification", w.notifications, [], "D-012 FYI never; D-001 Info none")
w = team()
m = w.post("anna", "#launch"); w.edit(m.id, mentions=["you"])
case("An edit adds a mention of you: it notifies like a new message", mine(w), 1, "R36")

# ---------------------------------------------------------------- history tabs (D-016)
w = team()
w.set_channel_dial("you", "#launch", "muted")
m1 = w.post("anna", "#launch", mentions=["you"]); w.read_row("you", ("th", m1.id)); w.open_feed("you"); w.leave_feed("you")
m2 = w.post("marco", "#product", mentions=["you"]); m3 = w.post("marco", "#product", mentions=["you"]); w.delete(m3.id)
d = w.post("anna", w.dm("anna", "you"))
t = w.post("you", "#product"); r = w.post("marco", "#product", parent=t.id); w.react("anna", t.id)
case("Mentions tab: read and unread together, from any channel, without the deleted one", w.history("you", "mentions"), [m2.id, m1.id], "D-016")
case("Direct messages tab", w.history("you", "dms"), [d.id], "D-016")
case("Threads tab: replies in threads you follow", w.history("you", "threads"), [r.id], "D-016")
case("Reactions tab: your messages that got reactions", w.history("you", "reactions"), [t.id], "D-016")

# ---------------------------------------------------------------- cases found by the second independent check
w = team()
m = w.post("anna", "#product", mentions=["you"], level="info"); w.read_row("you", ("th", m.id))
w.edit(m.id, level=None)
case("An edit removes Info from a mention you had read: it comes back as new, and notifies", (where(w, "you", m), mine(w)), ("Needs you", 1), "D-013 as if new")
w = team()
m = w.post("anna", "#product", mentions=["you"]); w.keep("you", ("th", m.id))
case("Keeping a row marks it read", w._row_of("you", m).items[(m.id, "msg")].read, True, "D-015 acting implies read")
w = team()
f = w.post("anna", w.dm("anna", "you"), mentions=["release"], level="flag"); w.read_row("you", ("dm", w.dm("anna", "you")))
case("A flagged direct message that also mentions an agent is still kept for you", where(w, "you", f), "Focus/kept", "D-011")
w = team()
d = w.post("anna", w.dm("anna", "you")); r = w.post("anna", w.dm("anna", "you"), parent=d.id)
case("A reply under a message in a direct conversation reaches you", where(w, "you", r), "Focus/new", "D-001")
w = team()
t = w.post("anna", "#launch"); s = w.post("release", "#launch", parent=t.id, streaming=True)
w.post("you", "#launch", parent=t.id); w.finish_stream(s)
case("An agent's reply finishes streaming after you replied in the thread: it reaches you", where(w, "you", s), "Needs you", "D-001")
w = team()
m = w.post("you", "#launch"); w.watch("you", ("th", m.id), w.now + 5)
w.react("anna", m.id); w.react("priya", m.id)
case("Two reactions arrive while the thread is open in front of you: both read at once", where(w, "you", m, "reaction"), "Earlier", "D-015")
w = team()
t = w.post("you", "#product"); w.post("marco", "#product", parent=t.id, level="info"); w.post("marco", "#product", parent=t.id)
case("An Info reply, then an ordinary one, in your unread thread: the ordinary one notifies", mine(w), 1, "D-012 + D-001")
w = team()
m = w.post("marco", "#product", mentions=["you"]); w.read_row("you", ("th", m.id)); w.open_feed("you"); w.leave_feed("you")
w.add_rule("you", "mentions")
case("A rule change leaves a row you already read where it was", where(w, "you", m), "Earlier", "D-013")

# ---------------------------------------------------------------- Focus people (D-021)
w = team(); m = w.post("anna", w.dm("anna", "you"))
case("D-021: someone who writes to you directly is added to your Focus people", ("anna" in w.people["you"].dm_people, where(w, "you", m)), (True, "Focus/new"), "D-021")
w = team(); w.post("anna", w.dm("anna", "you")); w.dm_option_off("you"); a = w.post("anna", w.dm("anna", "you")); b = w.post("marco", w.dm("marco", "you"))
case("D-021: option off: someone already added stays in Focus", where(w, "you", a), "Focus/new", "D-021")
case("D-021: option off: someone new is not added", where(w, "you", b), "Needs you", "D-021")
w = team(); w.post("anna", w.dm("anna", "you")); w.post("marco", w.dm("marco", "you")); w.dm_option_off("you", remove_all=True); a = w.post("anna", w.dm("anna", "you"))
case("D-021: option off and remove all: nobody is left", (w.people["you"].dm_people, where(w, "you", a)), (set(), "Needs you"), "D-021")
w = team(); w.post("anna", w.dm("anna", "you")); w.post("marco", w.dm("marco", "you")); w.remove_dm_person("you", "anna"); a = w.post("anna", w.dm("anna", "you")); b = w.post("marco", w.dm("marco", "you"))
case("D-021: a person removed is not added back when they write again", where(w, "you", a), "Needs you", "D-021")
case("D-021: removing one person leaves the others", where(w, "you", b), "Focus/new", "D-021")
w = team(); w.post("anna", w.dm("anna", "you")); w.remove_dm_person("you", "anna"); w.add_dm_person("you", "anna"); a = w.post("anna", w.dm("anna", "you"))
case("D-021: Add back returns them to Focus", where(w, "you", a), "Focus/new", "D-021")
w = team(); a = w.post("release", w.dm("release", "you"))
case("D-021: an agent is not added to Focus people", ("release" in w.people["you"].dm_people, where(w, "you", a)), (False, "Needs you"), "D-021, D-013")
w = team(); a = w.post("anna", w.dm("anna", "you")); w.remove_dm_person("you", "anna")
case("D-021: an unread row moves to Needs you once the person is removed", where(w, "you", a), "Needs you", "D-021, D-013")
w = team(); a = w.post("anna", w.dm("anna", "you")); w.read_row("you", ("dm", w.dm("anna", "you"))); w.keep("you", ("dm", w.dm("anna", "you"))); w.remove_dm_person("you", "anna")
case("D-021: removing a person does not release a row you kept", w.people["you"].rows[("dm", w.dm("anna", "you"))].kept, True, "D-021, D-003")

# ---------------------------------------------------------------- only Done releases (D-023 point 26)
def only_done():
    w = team(); w.people["you"].only_done = True
    return w
w = only_done(); f = w.post("anna", "#launch", mentions=["you"], level="flag"); w.post("you", "#launch", parent=f.id); r = w._row_of("you", f)
case("Only Done: a reply leaves a flag kept", bool(r.flagged), True, "D-023 p26")
w = only_done(); f = w.post("anna", "#launch", mentions=["you"], level="flag"); w.react("you", f.id); r = w._row_of("you", f)
case("Only Done: a reaction leaves a flag kept", bool(r.flagged), True, "D-023 p26")
case("Only Done: acting still reads the row", all(i.read for i in r.items.values() if i.msg == f.id), True, "D-015")
w.done("you", r.key)
case("Only Done: Done releases it", (bool(r.flagged), r.kept), (False, False), "D-023 p26")
w = only_done(); m = w.post("anna", "#launch"); t = w.post("marco", "#launch", mentions=["you"]); k = w._row_of("you", t); w.keep("you", k.key); w.react("you", t.id)
case("Only Done: a reaction leaves your own Keep", k.kept, True, "D-023 p26")
w = only_done(); f = w.post("anna", "#launch", mentions=["you"], level="flag"); r = w._row_of("you", f); w.edit(f.id, level=None)
case("Only Done: a flag the sender removes is still released", bool(r.flagged), False, "D-011")
w = team(); f = w.post("anna", "#launch", mentions=["you"], level="flag"); w.post("you", "#launch", parent=f.id); r = w._row_of("you", f)
case("Setting off (the start): a reply releases the flag", bool(r.flagged), False, "D-011")

# ---------------------------------------------------------------- report
bad = [r for r in results if not r[0]]
width = max(len(r[1]) for r in results)
for ok, name, got, expected, basis in results:
    mark = "ok  " if ok else "FAIL"
    print(f"{mark} {name}\n       -> {got}   [{basis}]" + ("" if ok else f"\n       expected {expected}"))
print(f"\n{len(results) - len(bad)} of {len(results)} cases behave as the decisions say")
print("invented behaviours exercised by the scenarios:", ", ".join(sorted(RULINGS)))
sys.exit(1 if bad else 0)
