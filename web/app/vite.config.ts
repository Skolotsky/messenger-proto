import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

// The app calls the door on its own address, so the same build works behind any host.
const door = process.env.CONTEXIA_DOOR ?? 'http://127.0.0.1:8090';
const proxy = { '/orgs': { target: door, ws: true, changeOrigin: true } };

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['icon.svg'],
      manifest: {
        name: 'Contexia',
        short_name: 'Contexia',
        description: 'Team messaging with an activity feed that sorts what deserves your attention.',
        start_url: '/',
        scope: '/',
        display: 'standalone',
        background_color: '#17181a',
        theme_color: '#1d2b2a',
        icons: [
          { src: 'icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: 'icon-512.png', sizes: '512x512', type: 'image/png' },
          { src: 'icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        // Installable, not offline (D-009): only the app's own files are cached, never an answer from the door.
        globPatterns: ['**/*.{js,css,html,svg,png}'],
        navigateFallbackDenylist: [/^\/orgs/],
        runtimeCaching: [],
      },
    }),
  ],
  server: { port: 5173, proxy },
  preview: { port: 4173, proxy },
});
