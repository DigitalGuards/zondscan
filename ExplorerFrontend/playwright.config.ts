import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  timeout: 30_000,
  use: {
    baseURL: 'http://127.0.0.1:18090',
    viewport: { width: 1440, height: 1000 },
    timezoneId: 'Europe/Amsterdam',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    launchOptions: { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH },
  },
  webServer: [
    {
      command: 'node e2e/fixture-api.mjs',
      url: 'http://127.0.0.1:18091/health',
      reuseExistingServer: !process.env.CI,
    },
    {
      command: process.env.E2E_SERVER_COMMAND || 'npm run dev -- --hostname 127.0.0.1 --port 18090',
      url: 'http://127.0.0.1:18090/settings',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      env: {
        HANDLER_URL: 'http://127.0.0.1:18091',
        NEXT_PUBLIC_HANDLER_URL: 'http://127.0.0.1:18091',
        NEXT_TELEMETRY_DISABLED: '1',
      },
    },
  ],
});
