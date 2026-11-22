import { defineConfig } from '@playwright/test';

// One worker: the tests share one mock door, and some of them take it down on purpose.
export default defineConfig({
  testDir: './specs',
  workers: 1,
  fullyParallel: false,
  forbidOnly: true,
  retries: 0,
  timeout: 30_000,
  expect: { timeout: 5_000 },
  reporter: [['list']],
  use: {
    baseURL: 'http://127.0.0.1:4173',
    locale: 'en-GB',
    serviceWorkers: 'block',
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'wide', use: { viewport: { width: 1280, height: 800 } } },
    { name: 'phone', use: { viewport: { width: 400, height: 800 }, hasTouch: true, isMobile: true } },
  ],
});
