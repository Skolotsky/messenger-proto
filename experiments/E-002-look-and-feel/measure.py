"""How much of the feed one screen holds in each direction. Run: ./scripts/browser experiments/E-002-look-and-feel/measure.py"""
import json
from pathlib import Path
from playwright.sync_api import sync_playwright
here = Path(__file__).parent
JS = """() => {
  const sc = document.querySelector('.feed-scroll').getBoundingClientRect();
  const rows = [...document.querySelectorAll('.row')].filter(r => { const b = r.getBoundingClientRect(); return b.top >= sc.top - 1 && b.bottom <= sc.bottom + 1; });
  const head = b => { const e = document.querySelector('.bucket.b-' + b + ' .bk-head'); return e ? Math.round(e.getBoundingClientRect().top - sc.top) : null; };
  const pv = document.querySelector('[data-row="r-rc"] .row-preview'); const r = document.createRange(); r.selectNodeContents(pv);
  const lines = Math.round(pv.getBoundingClientRect().height / parseFloat(getComputedStyle(pv).lineHeight));
  const rowH = b => { const e = document.querySelector('.bucket.b-' + b + ' .row'); return e ? Math.round(e.getBoundingClientRect().height) : null; };
  return { rows_on_first_screen: rows.length, feed_height: Math.round(sc.height), needs_starts_at: head('needs'), fyi_starts_at: head('fyi'), preview_lines: lines, preview_width: Math.round(pv.getBoundingClientRect().width), first_row_height: {focus: rowH('focus'), needs: rowH('needs'), fyi: rowH('fyi')} };
}"""
out = {}
with sync_playwright() as p:
    b = p.chromium.launch()
    for slug in ["slack-like", "telegram-like", "own"]:
        for name, vp in [("desktop", {"width": 1280, "height": 800}), ("phone", {"width": 390, "height": 844})]:
            pg = b.new_page(viewport=vp); pg.goto(f"file://{here.resolve()}/{slug}.html")
            out[f"{slug} {name}"] = pg.evaluate(JS); pg.close()
    b.close()
(here / "measure.json").write_text(json.dumps(out, indent=1) + "\n")
for k, v in out.items(): print(k, v)
