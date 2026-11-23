// The built pages at the addresses the real front serves them (server/internal/demo/front.go),
// so the product's service worker sees the other pages on its own origin.
// A browser with the cookie release=old gets the previous worker: the same build with the
// navigation rule that release shipped. Cookies keep parallel tests from affecting each other.
import { createReadStream, existsSync, readFileSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { extname, join, normalize } from 'node:path';

const web = new URL('../../web/', import.meta.url).pathname;
const app = join(web, 'app/dist-outside');
const mounts = {
  control: join(web, 'control-panel/dist-network'),
  notifications: join(web, 'notification-app/dist-network'),
};
const types = {
  '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml',
  '.png': 'image/png', '.webmanifest': 'application/manifest+json',
};

function previousWorker() {
  const now = readFileSync(join(app, 'sw.js'), 'utf8');
  const old = now
    .replace(/\{(?:allow|deny)list:\[.*?\]\}/, '{denylist:[/^\\/orgs/,/^\\/demo/]}')
    .replace(/directoryIndex:null,?/, '');
  if (old === now && !now.includes('denylist:[/^\\/orgs/,/^\\/demo/]')) throw new Error('front: no navigation rule found in sw.js');
  return `${old}\n// previous release\n`;
}

function sendPage(res, root, path) {
  const name = normalize(path).replace(/^\/+/, '');
  const file = join(root, name);
  const found = name && file.startsWith(root) && existsSync(file) && statSync(file).isFile();
  const chosen = found ? file : join(root, 'index.html');
  res.setHeader('content-type', types[extname(chosen)] ?? 'application/octet-stream');
  res.setHeader('cache-control', 'no-cache');
  createReadStream(chosen).pipe(res);
}

createServer((req, res) => {
  const { pathname: path, searchParams } = new URL(req.url, 'http://x');
  if (path === '/alive') { res.end('ok'); return; }
  if (path === '/' && !searchParams.get('org')) { // the entry: sends a screen to the current organization
    res.writeHead(302, { location: '/?org=larkmere-1', 'cache-control': 'no-store' });
    res.end();
    return;
  }
  if (path === '/sw.js' && /(?:^|;\s*)release=old/.test(req.headers.cookie ?? '')) {
    res.setHeader('content-type', 'text/javascript');
    res.setHeader('cache-control', 'no-cache');
    res.end(previousWorker());
    return;
  }
  if (path.startsWith('/ops/')) {
    res.setHeader('content-type', 'application/json');
    res.end(JSON.stringify({ part: 'feed', status: 'alive' }));
    return;
  }
  const [, top, ...rest] = path.split('/');
  if (mounts[top]) {
    if (path === `/${top}`) { res.writeHead(301, { location: `/${top}/` }); res.end(); return; }
    sendPage(res, mounts[top], rest.join('/'));
    return;
  }
  sendPage(res, app, path);
}).listen(4191, '127.0.0.1');
