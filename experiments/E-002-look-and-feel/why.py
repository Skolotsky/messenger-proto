"""Screenshots of the three ways to show why a row is in Focus and to change it. Run: ./scripts/browser experiments/E-002-look-and-feel/why.py"""
from pathlib import Path
from playwright.sync_api import sync_playwright
here = Path(__file__).parent; out = here / "screenshots"
with sync_playwright() as p:
    b = p.chromium.launch()
    for v in "abc":
        for name, vp, scale, clip in [("desktop", {"width": 1280, "height": 800}, 2, {"x": 250, "y": 0, "width": 503, "height": 640}), ("phone", {"width": 390, "height": 844}, 2, None)]:
            pg = b.new_page(viewport=vp, device_scale_factor=scale)
            errs = []; pg.on("pageerror", lambda e: errs.append(str(e)))
            pg.goto(f"file://{here.resolve()}/slack-like.html#why={v}")
            if v == "a": pg.locator('[data-row="r-anna"] .why-btn').click()
            if v == "b": pg.locator('[data-row="r-anna"] .row-main').click()
            if v == "c": pg.locator('[data-row="r-anna"] .why-btn').click()
            kw = {"clip": clip} if clip and v != "c" else {}
            pg.screenshot(path=str(out / f"why-{v}-{name}.png"), **kw)
            if name == "desktop":  # the same way on a row that is in Focus because of a channel dial, and after choosing
                if v == "c": pg.locator('.sheet [data-act="closeModal"]').click()
                if v == "b": pg.locator('[data-row="r-anna"] .row-main').click(); pg.locator('[data-row="r-inc"] .row-main').click()
                else:
                    if v == "a": pg.locator('[data-row="r-anna"] .why-btn').click()
                    pg.locator('[data-row="r-inc"] .why-btn').click()
                pg.screenshot(path=str(out / f"why-{v}-desktop-channel.png"), **kw)
            print(v, name, errs)
            pg.close()
    pg = b.new_page(viewport={"width": 1280, "height": 800}, device_scale_factor=2)
    pg.goto(f"file://{here.resolve()}/slack-like.html")
    pg.locator('.top [data-m="rules"]').click()
    pg.screenshot(path=str(out / "settings-desktop.png"))
    b.close()
