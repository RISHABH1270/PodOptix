import { defineConfig, devices } from '@playwright/test'

// UI test isolation strategy — completely separate from production and API tests:
//   Backend port:  9091   (production 8080, API tests 9090)
//   Postgres DB:   podoptix_ui_test  (production 'podoptix', API tests 'podoptix_test')
//   Redis index:   2      (production 0, API tests 1)
//   Vite port:     5174   (dev 5173)
//
// This means you can run: prod app + API tests + UI tests all at the same time.
export default defineConfig({
  testDir: './tests-e2e',
  timeout: 30_000,
  expect: { timeout: 5_000 },

  // Sequential — all tests share one DB. Simpler than coordinating unique names per test.
  fullyParallel: false,
  workers: 1,

  reporter: [['list'], ['html', { open: 'never' }]],

  use: {
    baseURL: 'http://localhost:5174',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },

  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],

  // Auto-start backend + frontend with isolated env before running tests.
  webServer: [
    {
      // Go backend on :9091 with its own DB + Redis index
      command: 'go run ./cmd/hub',
      cwd: '..',
      port: 9091,
      timeout: 60_000,
      reuseExistingServer: false, // always fresh servers — avoids stale state issues
      env: {
        PORT:           '9091',
        DATABASE_URL:   'postgres://postgres:password@localhost:5432/podoptix_ui_test?sslmode=disable',
        REDIS_URL:      'redis://localhost:6379/2',
        JWT_SECRET:     'ui-test-jwt-secret-key-please-change',
        ENCRYPTION_KEY: 'ui-test-32-byte-encryption-key!!',
      },
    },
    {
      // Vite dev server on :5174, proxying to the :9091 backend
      command: 'npm run dev',
      port: 5174,
      timeout: 30_000,
      reuseExistingServer: false, // always fresh servers — avoids stale state issues
      env: {
        VITE_PORT:     '5174',
        VITE_API_PORT: '9091',
      },
    },
  ],
})
