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
    def click(sel): vis(sel).first.click()
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

    # 2. one row per conversation, expanded in place; reply and react from the row
    ok("#incidents is one row for six posts", "6 new" in pg.locator('[data-row="r-inc"]').inner_text() or pg.locator('[data-row="r-inc"] .row-badge').inner_text() == "6")
    click('[data-row="r-inc"] .row-main')
    ok("row expands in place with its six posts", pg.locator('[data-row="r-inc"] .row-exp .msg').count() == 6)
    ok("expanding marks the row read", "read" in pg.locator('[data-row="r-inc"]').get_attribute("class").split())
    click('[data-row="r-inc"] .row-main')
    click('[data-row="r-anna"] .row-main')
    pg.locator('[data-row="r-anna"] .reply input').fill("Yes to 3 pm. Legal cleared the terms page yesterday.")
    pg.locator('[data-row="r-anna"] .reply button').click()
    ok("reply from the row appears in the row", "Legal cleared" in pg.locator('[data-row="r-anna"] .row-exp').inner_text())
    shot("02-row-expanded")
    click('[data-row="r-anna"] .row-main')

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
    body = pg.locator('[data-row="r-export"] .row-exp .msg-body')
    ok("expanded row shows the full edit", "streaming writer" in body.inner_text())
    click('[data-row="r-export"] [data-act="openConv"]')
    # the same edit watched inside a conversation is checked on a fresh page in conv_edit below
    close_conv()

    # 7. "3 new" while the list stays still
    feed(); bucket("focus")
    n = pg.locator(".row").count(); y = top(".row")
    demo("playNew")
    ok('"3 new" appears and nothing in the list moves', vis(".newpill").inner_text() == "3 new" and pg.locator(".row").count() == n and abs(top(".row") - y) < 0.5)
    shot("05-three-new")
    click(".newpill")
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
    click('.sheet [data-act="closeModal"]'); close_conv()
    feed()
    click('[data-act="modal"][data-m="rules"]')
    ok("the rule is in the Focus rules list", pg.locator(".sheet .rule").count() >= 5)
    click('.sheet [data-act="closeModal"]')

    # 12. the notification tray
    click('[data-act="modal"][data-m="tray"]')
    t = pg.locator(".sheet").inner_text()
    ok("tray lists notifications and shows the current truth", pg.locator(".sheet .tray-e").count() >= 10 and "Message deleted" in t and "For your information" not in t.split("Notify me")[1])
    shot("12-notification-tray")
    pg.locator('.sheet input[type="checkbox"]').check()
    ok('"Focus only" narrows the tray', all("Focus" in x for x in pg.locator(".sheet .tray-e small").all_inner_texts()))
    click('.sheet [data-act="closeModal"]')

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
            except Exception as e:
                failures.append(f"{slug} @ {size}: stopped: {str(e).splitlines()[0][:300]}")
    b.close()
(here / "results.json").write_text(json.dumps(results, indent=1, ensure_ascii=False) + "\n")
passed = sum(r["pass"] for r in results)
print(f"{passed} of {len(results)} checks passed across {len(PAGES)} pages and {len(SIZES)} widths")
for f in failures: print("FAIL", f)
sys.exit(1 if failures else 0)
