import { Page, expect } from '@playwright/test'

// Unique suffix per test run — avoids collisions between test files sharing the DB.
export const runId = Date.now().toString(36)

// Generates a unique email for a test — never collides across runs.
export const uniqueEmail = (label: string) => `${label}-${runId}-${Math.random().toString(36).slice(2, 6)}@ui.test`

// Generates a unique cluster name.
export const uniqueClusterName = (label: string) => `${label}-${runId}-${Math.random().toString(36).slice(2, 6)}`

// Registers a new user and returns to the /clusters page (logged in).
export async function registerAndLogin(page: Page, email: string, password = 'password123') {
  await page.goto('/register')
  await page.getByPlaceholder('you@company.com').fill(email)
  await page.getByPlaceholder('At least 8 characters').fill(password)
  await page.getByRole('button', { name: /create account/i }).click()
  await expect(page).toHaveURL(/\/clusters$/)
}
