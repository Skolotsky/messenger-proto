/* Direction 2: the chat list is the main surface. Buckets are folders across the top, rows are large, the conversation is bubbles. */
window.Skin = { file: 'telegram-like', title: 'Telegram-like', buckets: 'tabs', shell(h) {
  const S = h.S, title = { feed: 'Feed', history: 'History', chats: 'Chats' }[S.view];
  const tabs = S.view === 'feed' ? h.bucketTabs() : S.view === 'history' ? `<nav class="tabs btabs">${h.histTabs(true)}</nav>` : '';
  return `<div class="app tg v-${S.view}${S.conv ? ' has-conv' : ''}">
  <section class="list"><header class="top"><h1>${title}</h1><button class="icon-btn" data-act="modal" data-m="rules" aria-label="Focus rules">${h.ic('rules')}</button>${h.bell()}</header>
    <div class="search-wrap">${h.search()}</div>${tabs}
    ${S.view === 'feed' ? h.feed() : S.view === 'history' ? h.history() : `<div class="feed-wrap"><div class="feed-scroll" data-scroll="chats">${h.chats()}</div></div>`}
    <nav class="bottom">${h.nav('feed', 'Feed', 'feed', h.badge())}${h.nav('history', 'History', 'clock')}${h.nav('chats', 'Chats', 'chat')}</nav></section>
  <section class="pane">${h.conv()}</section>
</div>`; } };
