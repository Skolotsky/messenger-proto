"""Screenshots of the row layout variants. Run: ./scripts/browser experiments/E-002-look-and-feel/rowv.py"""
from pathlib import Path
from playwright.sync_api import sync_playwright
here = Path(__file__).parent; out = here / "screenshots"
with sync_playwright() as p:
    b = p.chromium.launch()
    for v in ["now", "a", "b", "c"]:
        pg = b.new_page(viewport={"width": 1280, "height": 860}, device_scale_factor=2)
        pg.goto(f"file://{here.resolve()}/slack-like.html" + ("" if v == "now" else f"#row={v}"))
        pg.locator('article[data-row="r-rc"] .row-main').hover(); pg.wait_for_timeout(400)
        pg.screenshot(path=str(out / f"rows-{v}.png"), clip={"x": 250, "y": 90, "width": 502, "height": 700})
        pg.close()
    b.close()
