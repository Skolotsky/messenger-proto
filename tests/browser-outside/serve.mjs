// Serves the built pages: /panel and /notifications make real calls, /panel-fixtures and
// /notifications-fixtures are the default builds that answer themselves.
import { createReadStream, existsSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { extname, join, normalize } from 'node:path';

const web = new URL('../../web/', import.meta.url).pathname;
const roots = {
  panel: join(web, 'control-panel/dist-network'),
  notifications: join(web, 'notification-app/dist-network'),
  'panel-fixtures': join(web, 'control-panel/dist'),
  'notifications-fixtures': join(web, 'notification-app/dist'),
};
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' };

createServer((req, res) => {
  const path = new URL(req.url, 'http://x').pathname;
  if (path === '/alive') { res.end('ok'); return; }
  const [, top, ...rest] = path.split('/');
  const root = roots[top];
  const file = root && join(root, normalize(rest.join('/') || 'index.html'));
  if (!file || !file.startsWith(root) || !existsSync(file) || !statSync(file).isFile()) {
    res.statusCode = 404;
    res.end();
    return;
  }
  res.setHeader('content-type', types[extname(file)] ?? 'application/octet-stream');
  createReadStream(file).pipe(res);
}).listen(4190, '127.0.0.1');
