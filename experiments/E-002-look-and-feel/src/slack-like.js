/* Direction 1: the conventions of a work messenger. Sidebar of channels, dense list, activity as one view among others. */
window.Skin = { file: 'slack-like', title: 'Slack-like', buckets: 'stack', pillTop: true, threadBeside: true, shell(h) {
  const S = h.S;
  return `<div class="app sk v-${S.view}${S.conv ? ' has-conv' : ''}${S.actOpen === false ? ' act-closed' : ''}${S.thread ? ' has-thread' : ''}" style="--w-side:${S.w.side}px;--w-act:${S.w.act}px">
  <aside class="side"><div class="ws"><span class="org-mark">${h.org().name[0]}</span><span class="ws-t"><b>${h.org().name}</b><small${S.net === 'off' ? ' class="net-line"' : ''}>${S.net === 'off' ? 'Reconnecting… last update 10:42' : h.org().members + ' members'}</small></span></div>${h.search()}
    <button class="side-item${S.view !== 'chats' && S.actOpen !== false ? ' on' : ''}" data-act="actToggle" aria-pressed="${S.actOpen !== false}" data-tip="Open or close Activity">${h.ic('bell')}<span>Activity</span>${h.badge()}</button>
    <div class="side-scroll" data-scroll="side">${h.chats()}</div>${h.me()}${h.rz('side')}</aside>
  <main class="main"><header class="top"><h1><small class="org-line">${h.org().name}</small>Activity</h1>${h.net()}${h.pill()}<button class="icon-btn" data-act="modal" data-m="rules" aria-label="Feed and notification settings" data-tip="Feed and notification settings">${h.ic('rules')}</button><button class="icon-btn act-close" data-act="actToggle" aria-label="Close Activity" data-tip="Close Activity">×</button></header>
    <nav class="tabs" data-scroll="tabs"><button class="tab${S.view !== 'history' ? ' on' : ''}" data-act="view" data-v="feed">Feed ${h.badge()}</button><span class="tabs-sep">History</span>${h.histTabs(true)}</nav>
    ${S.view === 'history' ? h.history() : h.feed()}${h.rz('act')}</main>
  <section class="pane">${h.conv()}</section>${S.thread ? `<section class="pane tpane">${h.thread()}</section>` : ''}
  <nav class="bottom"><button class="nav${S.view !== 'chats' ? ' on' : ''}" data-act="view" data-v="feed">${h.ic('bell')}<span>Activity</span>${h.badge()}</button>${h.nav('chats', 'Channels', 'chat')}<button class="nav" data-act="modal" data-m="rules">${h.ic('rules')}<span>Settings</span></button></nav>
</div>`; } };
