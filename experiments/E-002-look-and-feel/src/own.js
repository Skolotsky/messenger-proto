/* Direction 3: built around the model itself. Weight follows the bucket: Focus rows are cards, Needs you rows are lines, For your information is a digest. */
window.Skin = { file: 'own', title: 'The agent\'s own', buckets: 'stack', shell(h) {
  const S = h.S, c = h.counts();
  const sum = `<b class="s-focus">${c.focus} in Focus</b> · <b class="s-needs">${c.needs} need you</b> · <span>${c.fyi ? c.fyi + ' for your information' : 'nothing else new'}</span>`;
  const head = S.view === 'feed' ? `<h1>Tuesday, 10:42</h1><p class="sum">${sum}</p>` : S.view === 'history' ? '<h1>History</h1><p class="sum">For finding things again. Not the feed.</p>' : '<h1>Channels and people</h1><p class="sum">Each channel shows its dial when it is not at Default.</p>';
  const body = S.view === 'feed' ? h.feed() : S.view === 'history' ? `<nav class="tabs">${h.histTabs()}</nav>${h.history()}` : `<div class="feed-wrap"><div class="feed-scroll" data-scroll="chats">${h.chats()}</div></div>`;
  return `<div class="app own v-${S.view}${S.conv ? ' has-conv' : ''}">
  <nav class="rail"><span class="logo" aria-hidden="true">L</span>${h.nav('feed', 'Feed', 'feed', h.badge())}${h.nav('history', 'History', 'clock')}${h.nav('chats', 'Channels', 'chat')}<span class="rail-gap"></span><button class="nav" data-act="modal" data-m="rules">${h.ic('rules')}<span>Rules</span></button></nav>
  <main class="main"><header class="top"><div class="top-t">${head}</div>${h.bell()}</header><div class="search-wrap">${h.search()}</div>${body}</main>
  <section class="pane">${h.conv()}</section>
</div>`; } };
