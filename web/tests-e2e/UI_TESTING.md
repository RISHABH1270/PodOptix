# PodOptix — UI Testing Guide

End-to-end tests for the PodOptix dashboard — real Chromium browser clicking through real user flows against a real Go backend.

---

## Tooling

| Piece              | What we use                                                   | Why |
|--------------------|---------------------------------------------------------------|-----|
| Test runner        | [**Playwright**](https://playwright.dev) `^1.63`               | Modern industry standard for E2E, made by Microsoft. Replaced Cypress for most teams in 2024+ |
| Browser            | **Chromium** (headless by default)                            | Ships with Playwright, ~150 MB one-time download |
| Language           | **TypeScript**                                                | Same language as the dashboard — one skillset |
| Assertions         | Playwright's built-in `expect()`                              | Auto-retries — no flaky sleeps |
| Test isolation     | Separate PostgreSQL DB + Redis index + ports                  | Same containers as prod, zero collision |

**Why Playwright, not Cypress or Selenium?**
- **Auto-wait built in** — `expect(page.getByRole('button')).toBeVisible()` retries until visible. No manual `sleep()`.
- **Multi-browser** — same tests run on Chromium, Firefox, WebKit
- **Fast** — headless, parallel-capable, ~2s per test
- **Great debugging** — screenshots, videos, traces on failure
- **First-class TypeScript** — no plugins needed

---

## What we test

| File                                      | What it verifies |
|-------------------------------------------|------------------|
| `tests-e2e/auth.spec.ts`                  | Register / login / logout / wrong password / protected route bounces to /login |
| `tests-e2e/clusters.spec.ts`              | Register cluster / list / delete / URL validation / edit |
| `tests-e2e/recommendations.spec.ts`       | Recalculate button enabled when connected · disabled + warning when disconnected |

Currently: **9 tests, ~17 seconds** total.

---

## Isolation strategy — why UI tests don't collide with anything

We use the **same PostgreSQL and Redis containers** as production and API tests, but with completely separate logical stores. Nothing overlaps:

|                | Production  | API tests (`tests/`) | UI tests (`tests-e2e/`) |
|----------------|-------------|----------------------|--------------------------|
| Server port    | `8080`      | `9090`               | `9091`                   |
| Vite port      | —           | —                    | `5174`                   |
| PostgreSQL DB  | `podoptix`  | `podoptix_test`      | `podoptix_ui_test`       |
| Redis index    | `0`         | `1`                  | `2`                      |

You can run **all three simultaneously** — dev app, API tests, UI tests — and they won't interfere.

**How the isolation actually works:**

Playwright's config passes different env vars to the same Go binary:

```typescript
// playwright.config.ts (excerpt)
env: {
  PORT:           '9091',
  DATABASE_URL:   'postgres://.../podoptix_ui_test?sslmode=disable',
  REDIS_URL:      'redis://localhost:6379/2',
  JWT_SECRET:     'ui-test-jwt-secret',
  ENCRYPTION_KEY: 'ui-test-32-byte-encryption-key!!',
}
```

The Go backend's `EnsureDatabase()` creates `podoptix_ui_test` on startup if it doesn't exist. Migrations run against it. It's exactly the same code path as production.

---

## The complete lifecycle of `npm run test:e2e`

```
1. npm script: test:e2e:reset
        │
        ├── docker exec podoptix-db psql -c "DROP DATABASE podoptix_ui_test"
        └── docker exec podoptix-redis redis-cli -n 2 FLUSHDB
        ↓
2. Playwright starts webServers in parallel:
        │
        ├── Go backend on :9091
        │      • EnsureDatabase → creates podoptix_ui_test fresh
        │      • SyncSchema → runs all 3 migrations
        │      • Opens pgxpool + Redis connection
        │      • Listens on :9091
        │
        └── Vite dev server on :5174 (proxies API calls to :9091)
        ↓
3. Playwright launches headless Chromium
        ↓
4. For each test file → for each test:
        │
        ├── Fresh browser context (isolated cookies, localStorage)
        ├── Navigate to http://localhost:5174/whatever
        ├── Interact (fill inputs, click buttons)
        ├── Assert (expect page/element states)
        └── On failure: capture screenshot + video + trace
        ↓
5. Playwright kills both webServers, exits
```

**Important gotcha we already solved:** Playwright's `globalSetup` runs *concurrently* with `webServer`, not before. So dropping the DB from a `globalSetup` script would nuke the DB *after* the backend created it → all queries fail. That's why we drop the DB **inside the npm script** (before Playwright even starts), not in a globalSetup hook.

---

## Prerequisites (one-time)

```bash
# 1. Docker services running
docker compose up -d

# 2. Install dashboard + Playwright deps
cd web
npm install

# 3. Download Chromium browser (~150 MB)
npx playwright install chromium
```

If your network intercepts SSL (corporate proxy blocks the download):

```bash
NODE_TLS_REJECT_UNAUTHORIZED=0 npx playwright install chromium
```

That flag only affects this one command's TLS check for the Playwright CDN. Doesn't affect anything else.

---

## Running tests

```bash
# Headless — fastest. Prints results, saves HTML report.
npm run test:e2e

# Interactive UI — beautiful test explorer with time-travel debugging
npm run test:e2e:ui

# Just reset the UI test DB + Redis (rarely needed manually)
npm run test:e2e:reset

# Open the HTML report from the last run (screenshots, videos, traces)
npm run test:e2e:report
```

### Filter to a subset

```bash
# only tests matching the pattern
npm run test:e2e -- --grep "register creates"

# only one file
npm run test:e2e -- tests-e2e/auth.spec.ts

# combine: file + grep
npm run test:e2e -- tests-e2e/clusters.spec.ts --grep "delete"
```

### Run headed (watch the browser)

```bash
npm run test:e2e -- --headed
```

Slower but great when writing new tests — you see exactly what Playwright is doing.

---

## Expected output

```
Running 9 tests using 1 worker

  ✓  1 [chromium] › auth.spec.ts:5 › Auth › register creates a user and redirects to /clusters (1.4s)
  ✓  2 [chromium] › auth.spec.ts:12 › Auth › register with duplicate email shows error (0.9s)
  ✓  3 [chromium] › auth.spec.ts:24 › Auth › login with wrong password shows error (1.2s)
  ✓  4 [chromium] › auth.spec.ts:37 › Auth › protected route without JWT redirects to /login (0.5s)
  ✓  5 [chromium] › clusters.spec.ts:5 › Clusters › register a cluster, see it in the list, then delete it (2.1s)
  ✓  6 [chromium] › clusters.spec.ts:32 › Clusters › POST /clusters rejects an invalid prometheus_url (1.0s)
  ✓  7 [chromium] › clusters.spec.ts:42 › Clusters › edit cluster updates the name (1.8s)
  ✓  8 [chromium] › recommendations.spec.ts:5 › Recommendations › recalculate is enabled when connected (1.3s)
  ✓  9 [chromium] › recommendations.spec.ts:20 › Recommendations › recalculate is disabled + warning shown when disconnected (1.2s)

  9 passed (16.5s)
```

---

## Debugging failures

Every failed test drops artifacts under `web/test-results/<test-name>/`:

| Artifact             | What it is                                                        |
|----------------------|-------------------------------------------------------------------|
| `test-failed-1.png`  | Screenshot at the moment of failure                               |
| `video.webm`         | Full screen recording of the test                                 |
| `trace.zip`          | Time-travel trace — every DOM state, network call, console log   |
| `error-context.md`   | Text summary — the DOM tree at failure, useful for AI/log grep    |

**Open the trace viewer** (best debugging tool):
```bash
npm run test:e2e:report
```

Or open a specific trace:
```bash
npx playwright show-trace web/test-results/<test-name>/trace.zip
```

---

## How the test file is structured

```typescript
import { test, expect } from '@playwright/test'
import { uniqueEmail, registerAndLogin } from './helpers'

test.describe('Auth', () => {

  test('register creates a user and redirects to /clusters', async ({ page }) => {
    const email = uniqueEmail('signup')      // signup-mu1b91qv-dwbr@ui.test
    await registerAndLogin(page, email)      // fills form, submits, waits for redirect
    await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
    await expect(page.getByText(email)).toBeVisible()  // sidebar shows email
  })

})
```

Key concepts:

- `test('name', async ({ page }) => { ... })` — every test gets a fresh `page` (fresh browser tab)
- `page.goto('/register')` — Playwright uses the `baseURL` from config (`http://localhost:5174`)
- `page.getByRole/getByPlaceholder/getByLabel/getByText` — semantic locators. Prefer these over CSS.
- `expect(locator).toBeVisible()` — auto-retries for up to 5 s until the assertion passes or times out

---

## Test helpers (`tests-e2e/helpers.ts`)

Shared utilities to avoid boilerplate:

```typescript
export const runId = Date.now().toString(36)                     // unique per Playwright run

export const uniqueEmail       = (label) => `${label}-${runId}-${randomHex(4)}@ui.test`
export const uniqueClusterName = (label) => `${label}-${runId}-${randomHex(4)}`

export async function registerAndLogin(page, email, password = 'password123') {
  await page.goto('/register')
  await page.getByPlaceholder('you@company.com').fill(email)
  await page.getByPlaceholder('At least 8 characters').fill(password)
  await page.getByRole('button', { name: /create account/i }).click()
  await expect(page).toHaveURL(/\/clusters$/)
}
```

**Why the unique names?** Tests run sequentially within a Playwright process but share `podoptix_ui_test`. Unique names prevent inter-test collisions (like registering the same email twice = 409 conflict).

---

## Adding a new test

1. Pick the right file (or create a new `.spec.ts` in `tests-e2e/`)
2. Use `test.describe()` to group related tests
3. Call `track()` isn't needed — Playwright numbers them automatically
4. Prefer `getByRole()` / `getByLabel()` / `getByPlaceholder()` locators over CSS selectors
5. Use helpers when you need auth: `await registerAndLogin(page, uniqueEmail('mytest'))`

Example — adding a "logout works" test:

```typescript
test('logout returns to /login and clears session', async ({ page }) => {
  await registerAndLogin(page, uniqueEmail('logout'))
  await page.getByTitle('Log out').click()
  await expect(page).toHaveURL(/\/login$/)

  // subsequent navigation to a protected page still redirects
  await page.goto('/clusters')
  await expect(page).toHaveURL(/\/login$/)
})
```

Run it:
```bash
npm run test:e2e -- --grep "logout works"
```

---

## Why we run tests sequentially (not parallel)

All UI tests share one `podoptix_ui_test` database. Playwright can run tests in parallel via `workers`, but parallel workers would race on:
- Unique constraints (cluster names, emails)
- Global state (recommendations for the same cluster ID)

With `workers: 1` and `fullyParallel: false`, tests run one after another. Fast enough — the whole suite is under 20 seconds.

If we ever need parallelism, the fix is to give each worker its own DB (`podoptix_ui_test_1`, `_2`, etc.) via env var templating. Not needed at current scale.

---

## Quick reference

| Command                                 | What it does                                  |
|-----------------------------------------|-----------------------------------------------|
| `npm run test:e2e`                      | Reset DB → run all UI tests headless          |
| `npm run test:e2e:ui`                   | Reset DB → open Playwright's interactive UI   |
| `npm run test:e2e:reset`                | Just drop `podoptix_ui_test` + flush Redis 2  |
| `npm run test:e2e:report`               | Open HTML report from last run                |
| `npm run test:e2e -- --headed`          | Watch tests in a visible browser window       |
| `npm run test:e2e -- --grep "pattern"`  | Filter tests by name                          |
| `npx playwright show-trace <trace.zip>` | Time-travel debugger for a specific run       |

---

## Key principle

Every test hits **real Chromium** → **real Vite** → **real Go backend** → **real PostgreSQL** → **real Redis**.
No API mocks. No component mocking. If a test passes here, that end-to-end user flow works in production.

The only "mock" anywhere is our local Prometheus container in `docker-compose.yml` — Prometheus isn't feasible to run against a real K8s cluster during CI.
