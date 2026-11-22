"""Quick look: screenshot each page at both widths. Usage: ./scripts/browser experiments/E-002-look-and-feel/look.py OUTDIR [js-to-run]"""
import sys
from pathlib import Path
from playwright.sync_api import sync_playwright
here = Path(__file__).parent
out = Path(sys.argv[1]); out.mkdir(parents=True, exist_ok=True)
js = sys.argv[2] if len(sys.argv) > 2 else None
with sync_playwright() as p:
    b = p.chromium.launch()
    for slug in ["slack-like", "telegram-like", "own"]:
        for name, vp, scale in [("desktop", {"width": 1280, "height": 800}, 1), ("phone", {"width": 390, "height": 844}, 2)]:
            ctx = b.new_context(viewport=vp, device_scale_factor=scale, has_touch=name == "phone")
            pg = ctx.new_page()
            errs = []
            pg.on("console", lambda m: errs.append(m.text) if m.type == "error" else None)
            pg.on("pageerror", lambda e: errs.append(str(e)))
            pg.goto(f"file://{here.resolve()}/{slug}.html")
            if js: pg.evaluate(js)
            pg.wait_for_timeout(100)
            pg.screenshot(path=str(out / f"{slug}-{name}.png"))
            print(slug, name, "errors:", errs, "overflow:", pg.evaluate("document.documentElement.scrollWidth - innerWidth"))
            ctx.close()
    b.close()
