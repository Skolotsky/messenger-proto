import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// Relative paths, so the page can be served from any folder. /ops is where the operator
// calls will be routed when the demo program exists (an assumption: see src/api.ts).
const ops = process.env.CONTEXIA_OPS ?? 'http://127.0.0.1:8099';

export default defineConfig({
  base: './',
  plugins: [react()],
  server: { port: 5181, proxy: { '/ops': { target: ops, changeOrigin: true } } },
});
