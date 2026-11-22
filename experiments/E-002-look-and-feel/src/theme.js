/* Theme: System (the default), Light or Dark. Runs before the first paint so a stored choice never flashes the wrong theme. */
(function () {
  var K = 'contexia-theme', mem = null, d = document.documentElement, mq = matchMedia('(prefers-color-scheme: dark)');
  function read() { try { return localStorage.getItem(K); } catch (e) { return mem; } }
  function write(v) { mem = v; try { if (v === 'system') localStorage.removeItem(K); else localStorage.setItem(K, v); } catch (e) {} }
  function pref() { var v = read(); return v === 'light' || v === 'dark' ? v : 'system'; }
  function resolved() { var p = pref(); return p === 'system' ? (mq.matches ? 'dark' : 'light') : p; }
  function apply() {
    var p = pref(); if (p === 'system') delete d.dataset.theme; else d.dataset.theme = p;
    var m = document.querySelector('meta[name="theme-color"]'); if (m) m.content = resolved() === 'dark' ? '#17181a' : '#ffffff';
  }
  function changed() { apply(); window.dispatchEvent(new Event('themechange')); }
  window.Theme = { pref: pref, resolved: resolved, set: function (v) { write(v); changed(); } };
  apply();
  mq.addEventListener('change', changed); // follows the device live while on System
  window.addEventListener('storage', function (e) { if (e.key === K) changed(); }); // and other open windows
})();
