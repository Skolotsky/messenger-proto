"""E-002 click-through: drives each page as a person would, at desktop and phone width, and keeps screenshots.
Run from the repo root: ./experiments/E-002-look-and-feel/run.sh"""
import json, sys
from pathlib import Path
from playwright.sync_api import sync_playwright

here = Path(__file__).parent
shots = here / "screenshots"; shots.mkdir(exist_ok=True)
PAGES = ["slack-like", "telegram-like", "own"]
SIZES = [("desktop", {"width": 1280, "height": 800}, 1), ("phone", {"width": 390, "height": 844}, 2)]
results, failures = [], []

def run(browser, slug, size, vp, scale):
    tag = f"{slug} @ {size}"
    ctx = browser.new_context(viewport=vp, device_scale_factor=scale, has_touch=size == "phone", color_scheme="light")
    pg = ctx.new_page(); errs = []
    pg.on("console", lambda m: errs.append(m.text) if m.type == "error" else None)
    pg.on("pageerror", lambda e: errs.append(str(e)))
    pg.goto(f"file://{here.resolve()}/{slug}.html")
    phone, tabs = size == "phone", slug == "telegram-like"
    def ok(name, cond, detail=""):
        results.append({"page": slug, "width": size, "check": name, "pass": bool(cond), "detail": str(detail)})
        if not cond: failures.append(f"{tag}: {name} {detail}")
    def vis(sel): return pg.locator(sel).locator("visible=true")
    def click(sel):  # a row is opened by its avatar corner, since the reason line is a control of its own
        if sel.endswith(" .row-main"): vis(sel).first.click(position={"x": 24, "y": 24})
        else: vis(sel).first.click()
    def shot(name): pg.screenshot(path=str(shots / f"{slug}-{size}-{name}.jpg"), type="jpeg", quality=80)
    def bucket(b):
        if tabs: click(f'[data-act="btab"][data-b="{b}"]')
        else: pg.locator(f'.bucket.b-{b}').scroll_into_view_if_needed()
    def top(sel): return pg.locator(sel).first.bounding_box()["y"]
    def demo(act):
        click(".demo-btn"); click(f'.demo [data-act="{act}"]')
        note = pg.locator(".demo-note").inner_text(); click(".demo-btn"); return note
    def feed():
        if phone and pg.locator(".has-conv").count(): click('[data-act="closeConv"]')
        click('[data-act="view"][data-v="feed"]')
    def close_conv():
        if phone and pg.locator(".has-conv").count(): click('[data-act="closeConv"]')
    def swipe(rid, dx):  # a finger drag across a row, started on its avatar
        bb = pg.locator(f'article[data-row="{rid}"] .row-main').bounding_box(); x, y = bb["x"] + 30, bb["y"] + 26
        pg.mouse.move(x, y); pg.mouse.down(); pg.mouse.move(x + dx / 2, y, steps=4); pg.mouse.move(x + dx, y, steps=4); pg.mouse.up()
    def no_overflow(name): ok(f"no sideways scroll: {name}", pg.evaluate("document.documentElement.scrollWidth <= innerWidth"))

    # 1. the feed: three buckets, a reason on every row, Focus with New and Kept
    shot("01-feed")
    for scheme in ["dark"]:
        pg.emulate_media(color_scheme=scheme); shot("01-feed-dark"); pg.emulate_media(color_scheme="light")
    seen = {}
    for b in ["focus", "needs", "fyi"]:
        bucket(b)
        rows = pg.locator(f".bucket.b-{b} .row")
        seen[b] = rows.count()
        reasons = pg.locator(f".bucket.b-{b} .row .row-reason").all_inner_texts()
        ok(f"{b}: every row gives its reason", len(reasons) == seen[b] and all(len(r.strip()) > 8 for r in reasons), reasons[:1])
    ok("three buckets hold rows", seen == {"focus": 5, "needs": 6, "fyi": 5}, seen)
    bucket("focus")
    ok("Focus has a Kept section with two rows", pg.locator(".kept-head").inner_text().startswith("Kept · 2") and pg.locator(".bucket.b-focus .row.kept").count() == 2)
    no_overflow("feed")

    # mark one row read without opening it (user, 2026-11-22)
    bucket("needs")
    if phone: pg.locator('article[data-row="r-triage"]').scroll_into_view_if_needed(); swipe("r-triage", 110)
    else: pg.locator('[data-row="r-triage"] .row-main').hover(); click('[data-row="r-triage"] .mark-read')
    r = pg.locator('[data-row="r-triage"]')
    ok("a row can be marked read without opening it", "read" in r.get_attribute("class").split() and "expanded" not in r.get_attribute("class") and pg.locator(".has-conv").count() == 0)
    bucket("focus")
    # 2. one row per conversation, expanded in place; reply and react from the row
    ok("#incidents is one row for six posts", "6 new" in pg.locator('[data-row="r-inc"]').inner_text() or pg.locator('[data-row="r-inc"] .row-badge').inner_text() == "6")
    if phone:  # a tap goes straight to the conversation; nothing expands
        click('[data-row="r-inc"] .row-main')
        ok("on a phone a tap opens the conversation at the first new message, and nothing expands", pg.locator(".has-conv").count() == 1 and pg.locator(".row-exp").count() == 0 and pg.locator(".conv .newline").is_visible())
        shot("02-row-expanded"); close_conv()
        ok("opening the conversation reads the row", "read" in pg.locator('article[data-row="r-inc"]').get_attribute("class").split())
        click('[data-row="r-anna"] .row-main'); pg.locator(".composer input").fill("Yes to 3 pm. Legal cleared the terms page yesterday."); pg.locator(".composer .reply button").click(); close_conv()
        ok("a reply in the conversation reads the row", "read" in pg.locator('article[data-row="r-anna"]').get_attribute("class").split())
        click('[data-row="r-rc"] .row-main'); t = pg.locator(".conv").inner_text()
        ok("agent flag asks for approval and shows it is waiting", "approval" in t and "Waiting on Maya" in t)
        pg.locator(".composer input").fill("Approved. Roll it out."); pg.locator(".composer .reply button").click()
        ok("replying releases the flagged row", "Continuing" in pg.locator(".conv").inner_text()); close_conv()
        ok("and the row is done", "done" in pg.locator('article[data-row="r-rc"]').get_attribute("class").split())
    else:
        click('[data-row="r-inc"] .row-main')
        ok("row expands in place with its six posts", pg.locator('[data-row="r-inc"] .row-exp .msg').count() == 6)
        ok("expanding marks the row read", "read" in pg.locator('[data-row="r-inc"]').get_attribute("class").split())
        click('[data-row="r-inc"] .row-main')
        click('[data-row="r-anna"] .row-main')
        pg.locator('[data-row="r-anna"] .reply input').fill("Yes to 3 pm. Legal cleared the terms page yesterday.")
        shot("02-row-expanded")
        pg.locator('[data-row="r-anna"] .reply button').click()
        ra = pg.locator('article[data-row="r-anna"]')
        ok("sending a reply from the row closes the row and shows the reply on it", ra.locator(".row-exp").count() == 0 and "Legal cleared" in ra.locator(".row-preview").inner_text() and "replied" in ra.inner_text().lower() and "read" in ra.get_attribute("class").split(), ra.inner_text()[:160])
        shot("02b-replied")

        # 4. an agent's flag asking for approval; react from the row releases it
        click('[data-row="r-rc"] .row-main')
        t = pg.locator('[data-row="r-rc"] .row-exp').inner_text()
        ok("agent flag asks for approval and shows it is waiting", "approval" in t and "Waiting on Maya" in t and pg.locator('[data-row="r-rc"] .tag.flag').count() > 0)
        click('[data-row="r-rc"] [data-act="react"][data-e="✅"]')
        ok("reacting from the row releases the flagged row", "done" in pg.locator('[data-row="r-rc"]').get_attribute("class").split() and "Continuing" in pg.locator('[data-row="r-rc"] .row-exp').inner_text())
        click('[data-row="r-rc"] .row-main')

    # 3. a flag that names the viewer, read and therefore kept; Done on it
    r = pg.locator('[data-row="r-go"]')
    ok("flagged row is kept, with the reason", "Flagged by Anna" in r.inner_text() and "kept" in r.get_attribute("class"))
    ok("the kept row shows the reaction on its message", "Marco" in pg.locator('[data-row="r-go"] .rxs').inner_text())
    click('[data-row="r-go"] .row-side [data-act="done"]')
    ok("Done releases it", "done" in r.get_attribute("class").split())

    # 4/5. agent result, Info progress, reaction as an answer, reactions on a row
    bucket("needs")
    ok("agent result is in Needs you", pg.locator('.bucket.b-needs [data-row="r-comp"] .tag.ag').count() == 1)
    ok("a reaction that is an answer names the person", "Marco reacted ✅" in pg.locator('.bucket.b-needs [data-row="r-mig"]').inner_text())
    ok("a row shows who reacted to its message", "Sam" in pg.locator('[data-row="r-triage"] .rxs').inner_text())
    bucket("fyi")
    ok("Info progress update sits in For your information", pg.locator('.bucket.b-fyi [data-row="r-metrics"] .tag.info').count() == 1)
    ok("grouped reactions show the split", "🎉 6 · ❤️ 2" in pg.locator('[data-row="r-beta"]').inner_text())
    shot("03-fyi")

    # 6. a message changes while on screen: nothing jumps
    bucket("needs")
    pg.locator('[data-row="r-export"]').scroll_into_view_if_needed()
    y = top('[data-row="r-sso"]'); h = pg.locator('[data-row="r-export"]').bounding_box()["height"]
    note = demo("playEdit")
    d = abs(top('[data-row="r-sso"]') - y); dh = abs(pg.locator('[data-row="r-export"]').bounding_box()["height"] - h)
    ok("edit to a longer text: the row below does not move", d < 0.5 and dh < 0.5, f"moved {d:.2f}px, row grew {dh:.2f}px; page says: {note}")
    ok("the edited row says so", "edited" in pg.locator('[data-row="r-export"] .row-time').inner_text())
    y = top('[data-row="r-mig"]')
    note = demo("playDelete")
    d = abs(top('[data-row="r-mig"]') - y)
    ok("delete: a marker holds the space, the row below does not move", d < 0.5 and "deleted this message" in pg.locator('[data-row="r-sso"]').inner_text(), f"moved {d:.2f}px; page says: {note}")
    shot("04-edited-and-deleted")
    # the longer text sits behind an expander that never opens by itself
    click('[data-row="r-export"] .row-main')
    if phone: ok("the conversation shows the full edit", "streaming writer" in pg.locator('[data-k="conv:t1"] .msg-body').inner_text())
    else:
        ok("expanded row shows the full edit", "streaming writer" in pg.locator('[data-row="r-export"] .row-exp .msg-body').inner_text())
        click('[data-row="r-export"] [data-act="openConv"]')
    # the same edit watched inside a conversation is checked on a fresh page in conv_edit below
    close_conv()

    # 7. "3 new" while the list stays still
    feed(); bucket("focus")
    n = pg.locator(".row").count(); y = top(".row")
    demo("playNew")
    ok('"3 new" appears and nothing in the list moves', vis(".newpill b").inner_text() == "3 new" and ("1 in Focus" in vis(".newpill").inner_text() or (phone and slug == "slack-like")) and vis(".newpill button").inner_text() == "Show" and pg.locator(".row").count() == n and abs(top(".row") - y) < 0.5)
    shot("05-three-new")
    click(".newpill button")
    shot("05b-new-applied")
    ok("the new rows come in when asked", pg.locator('[data-row="r-sam"]').count() == 1 and pg.locator(".newpill").count() == 0)

    # 8. an empty bucket
    bucket("needs"); click('.bucket.b-needs [data-act="markAll"]')
    click('[data-act="view"][data-v="history"]' if slug != "slack-like" else '[data-act="htab"][data-t="mentions"]')
    feed(); bucket("needs")
    ok("an emptied bucket shows an illustration and one line", pg.locator(".bucket.b-needs .empty svg").count() == 1 and "Nobody is waiting on you" in pg.locator(".bucket.b-needs .empty").inner_text())
    shot("06-empty-bucket")
    bucket("focus")
    ok("after leaving: done rows are gone, a read flag is under Kept", pg.locator('[data-row="r-go"]').count() == 0 and pg.locator('[data-row="r-rc"]').count() == 0 and pg.locator('[data-row="r-anna"]').count() == 0)

    # 9. the four history tabs
    for t, want in [("dms", "Anna Lindqvist"), ("mentions", "Tomás Rivera"), ("threads", "Launch checklist"), ("reactions", "Marco reacted")]:
        if slug != "slack-like" and t == "dms": click('[data-act="view"][data-v="history"]')
        click(f'[data-act="htab"][data-t="{t}"]')
        txt = pg.locator(".feed-scroll").inner_text()
        ok(f"history tab {t} lists entries", want in txt and pg.locator(".hrow").count() >= 4, pg.locator(".hrow").count())
        if t == "mentions":
            ok("the deleted mention is not in history; the edit shows its new text", "SSO" not in txt and "Update: I measured it" in txt)
            shot("07-history-mentions")
    no_overflow("history")

    # 11. a conversation with a deleted message that has replies; 10. the channel dial
    if slug == "slack-like" and phone: click('[data-act="view"][data-v="chats"]')
    elif slug != "slack-like": click('[data-act="view"][data-v="chats"]')
    click('[data-act="openConv"][data-c="ch-launch"]')
    ok("deleted message with replies keeps a marker and its two replies", pg.locator(".conv .msg.deleted .msg-replies .msg").count() == 2 and "Priya deleted this message" in pg.locator(".conv .msg.deleted").inner_text())
    shot("08-conversation")
    click('.conv-head .dial-btn')
    ok("channel dial offers four levels", pg.locator(".conv-head .pop .opt").count() == 4)
    shot("09-channel-dial")
    click('.conv-head .pop [data-l="focus"]')
    ok("channel dial changes in one click", "Focus" in pg.locator(".conv-head .dial-btn").inner_text())
    no_overflow("conversation")
    # sender's level when writing
    click('.composer [data-l="flag"]'); pg.locator(".composer input").fill("@Anna go for Thursday."); pg.locator(".composer .reply button").click()
    ok("a message can be sent flagged", pg.locator(".conv .msg.mine.is-flag").count() == 1)
    close_conv()

    # 10. the thread dial with Follow, and adding a Focus rule with its count
    if slug == "slack-like": click('[data-act="view"][data-v="feed"]')  # on a phone the tabs live under Activity
    else: click('[data-act="view"][data-v="history"]')
    click('[data-act="htab"][data-t="threads"]')
    click('.hrow [data-c="th-icons"]')
    ok("thread shows Following", vis('.conv-head [data-act="follow"]').inner_text() == "Following")
    click('.conv-head .dial-btn')
    ok("thread dial offers the four buckets-as-levels", pg.locator(".conv-head .pop .opt").count() == 4)
    shot("10-thread-dial")
    click('.conv-head .dial-btn')
    click('.conv-head [data-act="follow"]')
    ok("Unfollow sets the thread to Off", "Off" in pg.locator(".conv-head .dial-btn").inner_text() and vis('.conv-head [data-act="follow"]').inner_text() == "Follow")
    click('.conv-head [data-act="ruleFor"]')
    ok("adding a Focus rule shows its count first", "items in the last 7 days" in pg.locator(".sheet .rule-n").first.inner_text())
    shot("11-add-focus-rule")
    click('.sheet [data-act="addRule"]')
    pg.locator('.sheet [data-act="addRule"][data-kind="thread"]').click()
    click('.sheet [data-act="closeModal"]')
    ok("adding a thread to Focus sets the thread's own level", "Focus" in pg.locator(".conv-head .dial-btn").inner_text())
    close_conv()
    feed()
    click('[data-act="modal"][data-m="rules"]')
    click('.sheet .set-nav [data-s="people"]')
    ok("the rule is in its own list in settings", "Everything that reaches you from" in pg.locator(".sheet .set-body").inner_text())
    click('.sheet [data-act="closeModal"]')

    # 12. notifications, shown outside the product (D-022)
    ok("the product has no notification bell", pg.locator(".bell").count() == 0)
    click(".demo-btn"); click('.demo [data-act="modal"][data-m="tray"]')
    t = pg.locator(".sheet").inner_text()
    ok("tray lists notifications and shows the current truth", pg.locator(".sheet .tray-e").count() >= 10 and "Message deleted" in t and "For your information ·" not in t)
    shot("12-notification-tray")
    click('.sheet [data-act="closeModal"]'); click(".demo-btn")
    click('[data-act="modal"][data-m="rules"]'); click('.sheet [data-act="setSec"][data-s="notif"]'); click('.sheet [data-act="notify"][data-v="focus"]'); click('.sheet [data-act="closeModal"]')
    click(".demo-btn"); click('.demo [data-act="modal"][data-m="tray"]')
    ok('"Focus only", set in the product, narrows what the notification app receives', all("Focus" in x for x in pg.locator(".sheet .tray-e small").all_inner_texts()))
    click('.sheet [data-act="closeModal"]'); click(".demo-btn")

    # search (on the Slack-like phone layout the search box lives under Home)
    if slug == "slack-like" and phone: click('[data-act="view"][data-v="chats"]')
    vis('[data-search]').first.fill("embargo")
    ok("search finds a message", "embargo" in pg.locator("#sr").inner_text())
    vis('[data-search]').first.fill("SSO in Q1")
    ok("search does not find the deleted message", "Nothing found" in pg.locator("#sr").inner_text())
    ok("no script errors", not errs, errs)
    ctx.close()

def conv_edit(browser, slug, size, vp, scale):
    """The same edit and delete, watched inside the conversation instead of the feed."""
    ctx = browser.new_context(viewport=vp, device_scale_factor=scale, has_touch=size == "phone")
    pg = ctx.new_page(); pg.goto(f"file://{here.resolve()}/{slug}.html")
    def check(conv, mid, nxt, act, name):
        pg.evaluate(f"(() => {{ const b=document.createElement('button'); b.dataset.act='openConv'; b.dataset.c='{conv}'; b.hidden=true; document.body.appendChild(b); b.click(); b.remove(); }})()")
        y = pg.locator(f'[data-k="conv:{nxt}"]').bounding_box()["y"]
        pg.locator(".demo-btn").click(); pg.locator(f'.demo [data-act="{act}"]').click(); pg.locator(".demo-btn").click()
        d = abs(pg.locator(f'[data-k="conv:{nxt}"]').bounding_box()["y"] - y)
        results.append({"page": slug, "width": size, "check": name, "pass": d < 0.5, "detail": f"moved {d:.2f}px"})
        if d >= 0.5: failures.append(f"{slug} @ {size}: {name} moved {d:.2f}px")
    check("ch-engineering", "t1", "g2", "playEdit", "in the conversation, an edit does not move the next message")
    has = pg.locator('.conv [data-act="openMsg"]').count() == 1
    results.append({"page": slug, "width": size, "check": "the longer edit waits behind an expander", "pass": has, "detail": ""})
    if not has: failures.append(f"{slug} @ {size}: no expander in conversation")
    pg.screenshot(path=str(shots / f"{slug}-{size}-13-edit-in-conversation.jpg"), type="jpeg", quality=80)
    if has:
        pg.locator('.conv [data-act="openMsg"]').click()
        full = "week after" in pg.locator('[data-k="conv:t1"] .msg-body').inner_text() and pg.locator('.conv .msg-body.locked').count() == 0
        results.append({"page": slug, "width": size, "check": "the expander opens the full text when asked", "pass": full, "detail": ""})
        if not full: failures.append(f"{slug} @ {size}: expander did not open")
    check("ch-sales", "j1", "j2", "playDelete", "in the conversation, a delete does not move the next message")
    ctx.close()

def why_and_people(browser, slug, size, vp, scale):
    """Why a row is in Focus (D-020) and how direct messages enter and leave Focus (D-021)."""
    ctx = browser.new_context(viewport=vp, device_scale_factor=scale, has_touch=size == "phone")
    pg = ctx.new_page(); pg.goto(f"file://{here.resolve()}/{slug}.html")
    def ok(name, cond, detail=""):
        results.append({"page": slug, "width": size, "check": name, "pass": bool(cond), "detail": str(detail)})
        if not cond: failures.append(f"{slug} @ {size}: {name} {detail}")
    def vis(sel): return pg.locator(sel).locator("visible=true")
    def goto_feed():
        vis('[data-act="view"][data-v="feed"]').first.click()
    def needs():
        if slug == "telegram-like": vis('[data-act="btab"][data-b="needs"]').first.click()
    pg.locator('article[data-row="r-anna"] .why-btn').click()
    t = pg.locator(".why-pop").inner_text()
    ok("the reason line opens the explanation without reading the row", "A direct message from Anna Lindqvist" in t and "Focus people" in t and "unread" in pg.locator('[data-row="r-anna"]').get_attribute("class").split())
    pg.screenshot(path=str(shots / f"{slug}-{size}-15-why.jpg"), type="jpeg", quality=80)
    ok("the panel says where the rule lives in settings", pg.locator('.why-pop [data-act="modal"][data-s="people"]').count() == 1)
    pg.locator('.why-pop [data-act="rmPerson"]').click()
    t = pg.locator(".why-pop").inner_text()
    ok("removing a person from the row acts at once, informs and offers undo", "Anna is removed from Focus" in t and pg.locator('.why-pop [data-act="undoPerson"]').count() == 1 and pg.locator(".scrim").count() == 0)
    ok("the row does not move while it is looked at", pg.locator('.bucket.b-focus [data-row="r-anna"]').count() == 1)
    pg.screenshot(path=str(shots / f"{slug}-{size}-16-person-removed.jpg"), type="jpeg", quality=80)
    vis('[data-act="modal"][data-m="rules"]').first.click()
    pg.locator('.sheet .set-nav [data-s="people"]').click()
    names = pg.locator(".sheet .rule.person:not(.gone-rule) b").all_inner_texts()
    ok("only that person left the Focus people", "Anna Lindqvist" not in names and len(names) == 4, names)
    pg.locator('.sheet .set-nav [data-s="feed"]').click()
    pg.locator('.sheet [data-act="dmAuto"]').uncheck()
    ok("switching the option off asks about people already added", "already in Focus and stay there" in pg.locator(".sheet .ask").inner_text())
    pg.screenshot(path=str(shots / f"{slug}-{size}-17-people-settings.jpg"), type="jpeg", quality=80)
    pg.locator('.sheet [data-act="keepPeople"]').click()
    pg.locator('.sheet .set-nav [data-s="people"]').click()
    ok("keeping them leaves the list as it was", pg.locator(".sheet .rule.person:not(.gone-rule)").count() == 4 and pg.locator(".sheet .ask").count() == 0)
    pg.locator('.sheet [data-act="rmPerson"][data-p="sam"]').click()
    ok("people can be removed one by one in settings", pg.locator(".sheet .rule.person:not(.gone-rule)").count() == 3)
    pg.locator('.sheet [data-act="closeModal"]').first.click()
    # with the option off and Sam removed, Sam's next message is not added back
    pg.locator(".demo-btn").click(); pg.locator('.demo [data-act="playNew"]').click(); pg.locator('.demo [data-act="leave"]').click(); pg.locator(".demo-btn").click()
    if slug == "telegram-like": needs()
    ok("after leaving the feed, Anna's message is in Needs you", pg.locator('.bucket.b-needs [data-row="r-anna"]').count() == 1)
    vis(".newpill button").first.click()
    ok("a removed person is not added back when they write again", pg.locator('.bucket.b-needs [data-row="r-sam"]').count() == 1)
    ctx.close()

def bursts(browser, slug, size, vp, scale):
    """Ten messages from one person: one row, newest by the reply box, with context, and one click to the conversation (proposal, 2026-11-22)."""
    ctx = browser.new_context(viewport=vp, device_scale_factor=scale, has_touch=size == "phone")
    pg = ctx.new_page(); pg.goto(f"file://{here.resolve()}/{slug}.html")
    def ok(name, cond, detail=""):
        results.append({"page": slug, "width": size, "check": name, "pass": bool(cond), "detail": str(detail)})
        if not cond: failures.append(f"{slug} @ {size}: {name} {detail}")
    if size == "phone":  # no expanding on a phone: a tap opens the conversation, a swipe gives the quick actions
        def drag(rid, dx, release=True):
            pg.evaluate("id => document.querySelector(`article[data-row='${id}']`).scrollIntoView({block: 'center'})", rid)
            bb = pg.locator(f'article[data-row="{rid}"] .row-main').bounding_box(); x, y = bb["x"] + 30, bb["y"] + 26
            pg.mouse.move(x, y); pg.mouse.down(); pg.mouse.move(x + dx / 2, y, steps=4); pg.mouse.move(x + dx, y, steps=4)
            if release: pg.mouse.up()
        ok("on a phone a row shows one button, React; mark read and open are the swipe and the tap", pg.locator(".row-btns .mark-read, .row-btns .open-btn").locator("visible=true").count() == 0 and pg.locator('article[data-row="r-inc"] .row-btns .rx-pick').is_visible())
        pg.locator('article[data-row="r-inc"] .row-btns .rx-pick').click()
        pg.screenshot(path=str(shots / f"{slug}-{size}-21b-react-from-row.jpg"), type="jpeg", quality=80)
        pg.locator('.picker [data-e="🔥"]').first.click()
        ok("the React button on a row reacts from the feed, without opening anything", "You" in pg.locator('article[data-row="r-inc"] .rxs').inner_text() and pg.locator(".has-conv").count() == 0)
        pg.locator('article[data-row="r-anna"] .row-main').click(position={"x": 24, "y": 24})
        ok("a tap opens the conversation at the first new message", pg.locator(".has-conv").count() == 1 and pg.locator(".conv .newline").is_visible() and pg.locator(".row-exp").count() == 0)
        pg.locator('[data-act="closeConv"]').locator("visible=true").first.click()
        if slug == "telegram-like": pg.locator('[data-act="btab"][data-b="needs"]').click()
        pg.evaluate("document.querySelector(`article[data-row='r-check']`).scrollIntoView({block: 'center'})")
        y0 = pg.locator('article[data-row="r-export"]').bounding_box()["y"]
        drag("r-check", 100, release=False)
        pg.screenshot(path=str(shots / f"{slug}-{size}-22-swipe-right.jpg"), type="jpeg", quality=80)
        pg.mouse.up()
        r = pg.locator('article[data-row="r-check"]')
        ok("a swipe to the right marks the row read without opening it, and the row stays in place", "read" in r.get_attribute("class").split() and pg.locator(".has-conv").count() == 0 and abs(pg.locator('article[data-row="r-export"]').bounding_box()["y"] - y0) < 0.5)
        y1 = pg.locator('article[data-row="r-sso"]').bounding_box()["y"]
        pg.evaluate("document.querySelector(`article[data-row='r-export']`).scrollIntoView({block: 'center'})"); y1 = pg.locator('article[data-row="r-sso"]').bounding_box()["y"]
        drag("r-export", -110, release=False)
        pg.screenshot(path=str(shots / f"{slug}-{size}-23-swipe-left.jpg"), type="jpeg", quality=80)
        pg.mouse.up()
        ex = pg.locator('article[data-row="r-export"]')
        ok("a swipe to the left keeps the row, without opening it, and the row stays in place", "kept" in ex.inner_text().lower() and "read" in ex.get_attribute("class").split() and pg.locator(".has-conv").count() == 0 and abs(pg.locator('article[data-row="r-sso"]').bounding_box()["y"] - y1) < 0.5)
        ok("no reaction hides behind a swipe", pg.locator(".sw-under button").count() == 0)
        ctx.close(); return
    row = pg.locator('article[data-row="r-anna"]')
    ok("ten messages are one row that says so", pg.locator('article[data-row="r-anna"]').count() == 1 and ("10 new" in row.inner_text() or row.locator(".row-badge").inner_text() == "10"), row.inner_text()[:60])
    row.locator(".row-main").click(position={"x": 24, "y": 24})
    box = row.locator(".reply input").bounding_box()
    ok("an opened row has its reply box on screen", box["y"] >= 0 and box["y"] + box["height"] <= vp["height"], box)
    if size == "desktop": ok("and the cursor is in it", pg.evaluate("document.activeElement === document.querySelector('article[data-row=\"r-anna\"] .reply input')"))
    ok("expanding shows the six newest messages, after the one before the burst", row.locator(".row-exp > .msg").count() == 6 and "Call me if 3 pm" in row.locator(".row-exp > .msg").last.inner_text() and row.locator(".ctx .msg").count() == 1 and "Draft by Wednesday night" in row.locator(".ctx").inner_text())
    ok("the four earlier new ones are one click above, and the row is read", "Show 4 earlier new messages" in row.locator(".more-new").inner_text() and "read" in row.get_attribute("class").split())
    pg.screenshot(path=str(shots / f"{slug}-{size}-18-ten-messages.jpg"), type="jpeg", quality=80)
    row.locator('[data-act="more"]').click()
    ok("showing the earlier ones gives all ten in order", row.locator(".row-exp > .msg").count() == 10 and "pricing call" in row.locator(".row-exp > .msg").first.inner_text())
    if slug == "telegram-like": pg.locator('[data-act="btab"][data-b="needs"]').click()
    t = pg.locator('article[data-row="r-check"]')
    t.locator(".row-main").click(position={"x": 24, "y": 24})
    ok("a thread row shows the message that started the thread and the reply before the new ones", t.locator(".ctx .msg").count() == 2 and "Launch checklist for Thursday" in t.locator(".ctx").inner_text())
    if slug == "telegram-like": pg.locator('[data-act="btab"][data-b="focus"]').click()
    # react to any message in an expanded row, not only the latest
    if slug == "telegram-like": pg.locator('[data-act="btab"][data-b="focus"]').click()
    inc = pg.locator('article[data-row="r-inc"]'); inc.locator(".row-main").click(position={"x": 24, "y": 24})
    second = inc.locator(".row-exp > .msg").nth(1)
    second.locator(".msg-self").hover()
    if size == "phone": second.locator(".rx-pick").click(); pg.locator('.picker [data-e="👀"]').first.click()
    else: second.locator('.msg-tools [data-e="👀"]').click()
    ok("a reaction can be put on any message in the row", "You" in inc.locator(".row-exp > .msg").nth(1).locator(".rxs").inner_text() and "You" not in inc.locator(".row-exp > .msg").last.inner_text())
    inc.locator(".row-exp > .msg").nth(1).locator(".rxs .rx").click()
    ok("clicking your reaction takes it back", inc.locator(".row-exp > .msg").nth(1).locator(".rxs").count() == 0)
    inc.locator(".row-exp > .msg").nth(2).locator(".msg-self").hover()
    pg.screenshot(path=str(shots / f"{slug}-{size}-19-react-to-any.jpg"), type="jpeg", quality=80)
    # the full picker, from a message
    inc.locator(".row-exp > .msg").nth(2).locator(".rx-pick").click()
    ok("the full emoji picker opens from a message, with a search box", pg.locator(".picker .pk-grid button").count() >= 60 and pg.locator(".picker input").count() == 1)
    pg.screenshot(path=str(shots / f"{slug}-{size}-20-emoji-picker.jpg"), type="jpeg", quality=80)
    pg.locator(".picker input").fill("rocket")
    ok("searching the picker narrows it", pg.locator(".picker .pk-grid button").locator("visible=true").count() == 1)
    pg.locator('.picker [data-e="🚀"]').click()
    ok("an emoji from the picker lands on that message", "🚀" in inc.locator(".row-exp > .msg").nth(2).locator(".rxs").inner_text() and pg.locator(".picker").count() == 0)
    # react from a row that was never expanded
    pg.reload()
    if slug == "telegram-like": pg.locator('[data-act="btab"][data-b="needs"]').click()
    tr = pg.locator('article[data-row="r-triage"]'); tr.locator(".row-main").hover()
    tr.locator(".row-btns .rx-pick").click()
    pg.screenshot(path=str(shots / f"{slug}-{size}-21-react-from-row.jpg"), type="jpeg", quality=80)
    pg.locator('.picker [data-e="👍"]').first.click()
    ok("a collapsed row takes a reaction without being expanded", "You" in tr.locator(".rxs").inner_text() and tr.locator(".row-exp").count() == 0 and "read" in tr.get_attribute("class").split())
    pg.reload()
    pg.locator('article[data-row="r-anna"] .open-btn').click()
    ok("one click on the row opens the conversation at the first new message", pg.locator(".conv .newline").count() == 1 and pg.locator(".conv .newline").is_visible() and pg.locator('article[data-row="r-anna"] .row-exp').count() == 0)
    ctx.close()

def chrome(browser, slug, size, vp, scale):
    """Slack-like only: one settings screen, the person's own menu, closing and reopening Activity (user, 2026-11-22)."""
    if slug != "slack-like": return
    ctx = browser.new_context(viewport=vp, device_scale_factor=scale, has_touch=size == "phone")
    pg = ctx.new_page(); pg.goto(f"file://{here.resolve()}/{slug}.html")
    def ok(name, cond, detail=""):
        results.append({"page": slug, "width": size, "check": name, "pass": bool(cond), "detail": str(detail)})
        if not cond: failures.append(f"{slug} @ {size}: {name} {detail}")
    def vis(sel): return pg.locator(sel).locator("visible=true")
    vis('[data-act="modal"][data-m="rules"]').first.click()
    body = lambda: pg.locator(".settings .set-body").inner_text().lower()
    ok("settings is one screen, opened on Activity feed when reached from the feed", pg.locator(".settings .set-item").count() == 5 and pg.locator(".settings .set-item.on").inner_text() == "Activity feed")
    ok("the Activity feed page keeps the general rules and links to the lists", pg.locator(".settings .set-sw").count() == 3 and "people · 5" in body() and "channel levels · 3 changed" in body())
    ok("threads are not a list in settings; the page says where they are managed", pg.locator('.settings [data-s="threads"]').count() == 0 and "on the thread itself" in body())
    pg.screenshot(path=str(shots / f"{slug}-{size}-24-settings.jpg"), type="jpeg", quality=80)
    pg.locator('.settings [data-act="ruleToggle"][data-id="flag"]').uncheck()
    ok("a general rule is switched off and on in the same place", not pg.locator('.settings [data-act="ruleToggle"][data-id="flag"]').is_checked())
    pg.locator('.settings [data-act="ruleToggle"][data-id="flag"]').check()
    ok("and back on", pg.locator('.settings [data-act="ruleToggle"][data-id="flag"]').is_checked())
    pg.locator('.settings .set-link[data-s="people"]').click()
    ok("Focus people is its own list", pg.locator(".settings .set-item.on").inner_text() == "Focus people" and pg.locator(".settings .rule.person:not(.gone-rule)").count() == 5)
    pg.locator('.settings [data-act="rmPerson"][data-p="sam"]').click()
    ok("a removed person stays listed with a way back", "Sam Whitaker" in pg.locator(".settings .rule.person.gone-rule").inner_text())
    pg.screenshot(path=str(shots / f"{slug}-{size}-24b-settings-people.jpg"), type="jpeg", quality=80)
    pg.locator('.settings .gone-rule [data-act="undoPerson"]').click()
    ok("adding the person back restores them", pg.locator(".settings .rule.person:not(.gone-rule)").count() == 5 and pg.locator(".settings .gone-rule").count() == 0)
    pg.locator('.settings [data-s="notif"]').click()
    ok("notifications are a section of the same screen: one question, two choices", pg.locator(".settings .n-opt").count() == 2 and pg.locator(".settings .n-opt.on b").inner_text() == "Focus and Needs you")
    pg.locator('.settings [data-v="focus"]').click()
    ok("choosing Focus only selects it", pg.locator(".settings .n-opt.on b").inner_text() == "Focus only")
    pg.screenshot(path=str(shots / f"{slug}-{size}-24c-settings-notifications.jpg"), type="jpeg", quality=80)
    pg.locator('.settings [data-v="all"]').click()
    pg.locator('.settings .set-nav [data-s="channels"]').click()
    ok("channel levels sit under Activity feed", "sub" in pg.locator(".settings .set-item.on").get_attribute("class") and pg.locator(".settings .set-item.on").inner_text() == "Channel levels")
    ok("channels lists every channel with its level, changed ones first", pg.locator(".settings .ch-row").count() == 20 and "changed by you · 3" in body())
    pg.locator('.settings select[data-dial="ch-product"]').select_option("muted")
    ok("a channel's level is changed in place", "changed by you · 4" in body())
    pg.screenshot(path=str(shots / f"{slug}-{size}-24d-settings-channels.jpg"), type="jpeg", quality=80)
    pg.locator('.settings select[data-dial="ch-incidents"]').select_option("default")
    pg.locator('.settings select[data-dial="ch-incidents"]').select_option("focus")
    ok("a channel can be taken out of Focus and put back in the same place", pg.locator('.settings select[data-dial="ch-incidents"]').input_value() == "focus")
    pg.locator('.settings [data-act="closeModal"]').click()
    if size == "phone": ctx.close(); return
    pg.locator(".me-btn").click()
    ok("the sidebar has the person's own menu, leading to settings", "Maya Okafor" in pg.locator(".me-btn").inner_text() and pg.locator('.me-pop [data-s="account"]').count() == 1)
    pg.screenshot(path=str(shots / f"{slug}-{size}-25-own-menu.jpg"), type="jpeg", quality=80)
    pg.locator('.me-pop [data-s="account"]').click()
    ok("from there settings opens on Account", pg.locator(".settings .set-item.on").inner_text() == "Account")
    pg.locator('.settings [data-act="closeModal"]').click()
    pg.locator('.side [data-act="openConv"][data-c="ch-launch"]').click()
    w0 = pg.locator(".pane").bounding_box()["width"]
    pg.locator(".act-close").click()
    ok("Activity has a close button; the conversation takes the room", pg.locator(".main").is_hidden() and pg.locator(".pane").bounding_box()["width"] > w0 + 300)
    ok("the count stays on the sidebar while Activity is closed", pg.locator(".side-item .badge").is_visible())
    pg.screenshot(path=str(shots / f"{slug}-{size}-26-activity-closed.jpg"), type="jpeg", quality=80)
    pg.locator(".side-item").click()
    ok("clicking Activity in the sidebar opens it again", pg.locator(".main").is_visible())
    pg.locator(".side-item").click()
    ok("and clicking it once more closes it", pg.locator(".main").is_hidden())
    ctx.close()

def hist_unread(browser, slug, size, vp, scale):
    """History tabs carry an unread count, and unread entries a mark; opening one clears both (user, 2026-11-22)."""
    ctx = browser.new_context(viewport=vp, device_scale_factor=scale, has_touch=size == "phone")
    pg = ctx.new_page(); pg.goto(f"file://{here.resolve()}/{slug}.html")
    def ok(name, cond, detail=""):
        results.append({"page": slug, "width": size, "check": name, "pass": bool(cond), "detail": str(detail)})
        if not cond: failures.append(f"{slug} @ {size}: {name} {detail}")
    def vis(sel): return pg.locator(sel).locator("visible=true")
    if slug != "slack-like": vis('[data-act="view"][data-v="history"]').first.click()
    counts = {t: (vis(f'[data-act="htab"][data-t="{t}"] .h-count').first.inner_text() if vis(f'[data-act="htab"][data-t="{t}"] .h-count').count() else "0") for t in ["dms", "mentions", "threads", "reactions"]}
    ok("history tabs show their unread counts", counts == {"dms": "1", "mentions": "4", "threads": "5", "reactions": "2"}, counts)
    if slug == "slack-like" and size == "desktop": ok("all five Activity tabs fit without scrolling", pg.evaluate("(() => { const t = document.querySelector('.main .tabs'); return t.scrollWidth <= t.clientWidth; })()"))
    vis('[data-act="htab"][data-t="mentions"]').first.click()
    ok("unread history entries carry a mark, read ones do not", pg.locator(".hrow.unread .dot").count() == 4 and pg.locator(".hrow:not(.unread) .dot").count() == 0, pg.locator(".hrow.unread .dot").count())
    pg.screenshot(path=str(shots / f"{slug}-{size}-14-history-unread.jpg"), type="jpeg", quality=80)
    vis('.hrow.unread .row-main').first.click()
    if size == "phone": vis('[data-act="closeConv"]').first.click()
    ok("opening an entry clears its mark and lowers the count; the entry stays", vis('[data-act="htab"][data-t="mentions"] .h-count').first.inner_text() == "3" and pg.locator(".hrow").count() == 7, pg.locator(".hrow").count())
    ctx.close()

with sync_playwright() as p:
    b = p.chromium.launch()
    for slug in PAGES:
        for size, vp, scale in SIZES:
            try:
                run(b, slug, size, vp, scale)
                conv_edit(b, slug, size, vp, scale)
                hist_unread(b, slug, size, vp, scale)
                chrome(b, slug, size, vp, scale)
                bursts(b, slug, size, vp, scale)
                why_and_people(b, slug, size, vp, scale)
            except Exception as e:
                failures.append(f"{slug} @ {size}: stopped: {str(e).splitlines()[0][:300]}")
    b.close()
(here / "results.json").write_text(json.dumps(results, indent=1, ensure_ascii=False) + "\n")
passed = sum(r["pass"] for r in results)
print(f"{passed} of {len(results)} checks passed across {len(PAGES)} pages and {len(SIZES)} widths")
for f in failures: print("FAIL", f)
sys.exit(1 if failures else 0)
