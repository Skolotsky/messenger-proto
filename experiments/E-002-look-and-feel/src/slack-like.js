/* Direction 1: the conventions of a work messenger. Sidebar of channels, dense list, activity as one view among others. */
window.Skin = { file: 'slack-like', title: 'Slack-like', buckets: 'stack', pillTop: true, shell(h) {
  const S = h.S;
  return `<div class="app sk v-${S.view}${S.conv ? ' has-conv' : ''}${S.actOpen === false ? ' act-closed' : ''}">
  <aside class="side"><div class="ws"><b>Larkspur</b><small>Maya Okafor · Product</small></div>${h.search()}
    <button class="side-item${S.view !== 'chats' && S.actOpen !== false ? ' on' : ''}" data-act="actToggle" aria-pressed="${S.actOpen !== false}" title="Open or close Activity">${h.ic('bell')}<span>Activity</span>${h.badge()}</button>
    <div class="side-scroll" data-scroll="side">${h.chats()}</div>${h.me()}</aside>
  <main class="main"><header class="top"><h1>Activity</h1>${h.pill()}<button class="icon-btn" data-act="modal" data-m="rules" aria-label="Feed and notification settings" title="Feed and notification settings">${h.ic('rules')}</button>${h.bell()}<button class="icon-btn act-close" data-act="actToggle" aria-label="Close Activity" title="Close Activity">×</button></header>
    <nav class="tabs"><button class="tab${S.view !== 'history' ? ' on' : ''}" data-act="view" data-v="feed">Feed ${h.badge()}</button><span class="tabs-sep">History</span>${h.histTabs(true)}</nav>
    ${S.view === 'history' ? h.history() : h.feed()}</main>
  <section class="pane">${h.conv()}</section>
  <nav class="bottom"><button class="nav${S.view !== 'chats' ? ' on' : ''}" data-act="view" data-v="feed">${h.ic('bell')}<span>Activity</span>${h.badge()}</button>${h.nav('chats', 'Channels', 'chat')}<button class="nav" data-act="modal" data-m="rules">${h.ic('rules')}<span>Settings</span></button></nav>
</div>`; } };
