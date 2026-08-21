import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 20_000,
  retries: 0,
  reporter: 'line',
  use: { ...devices['Desktop Chrome'], baseURL: 'http://127.0.0.1:3081' },
  webServer: {
    command: 'pnpm --ignore-workspace dev --host 127.0.0.1',
    url: 'http://127.0.0.1:3081/',
    reuseExistingServer: true,
  },
})
