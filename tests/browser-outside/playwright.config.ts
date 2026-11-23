import { defineConfig } from '@playwright/test';

// The pages are built for the network; each test answers their real requests itself.
export default defineConfig({
  testDir: './specs',
  // The real system is one shared installation, and its specs switch parts off and reset it.
  workers: process.env.CONTEXIA_REAL_URL ? 1 : 2,
  fullyParallel: true,
  forbidOnly: true,
  retries: 0,
  timeout: 30_000,
  expect: { timeout: 5_000 },
  reporter: [['list']],
  webServer: [
    { command: 'node serve.mjs', url: 'http://127.0.0.1:4190/alive', reuseExistingServer: false },
    { command: 'node front.mjs', url: 'http://127.0.0.1:4191/alive', reuseExistingServer: false },
  ],
  use: {
    baseURL: 'http://127.0.0.1:4190',
    locale: 'en-GB',
    timezoneId: 'UTC',
    trace: 'retain-on-failure',
  },
  projects: [
    { name: 'wide', use: { viewport: { width: 1280, height: 800 } } },
    { name: 'phone', use: { viewport: { width: 400, height: 800 }, hasTouch: true, isMobile: true } },
  ],
});
