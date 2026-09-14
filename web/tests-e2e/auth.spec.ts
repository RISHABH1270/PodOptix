import { test, expect } from '@playwright/test'
import { uniqueEmail, registerAndLogin } from './helpers'

test.describe('Auth', () => {
  test('register creates a user and redirects to /clusters', async ({ page }) => {
    const email = uniqueEmail('signup')
    await registerAndLogin(page, email)
    await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
    await expect(page.getByText(email)).toBeVisible() // shown in sidebar
  })

  test('register with duplicate email shows error', async ({ page }) => {
    const email = uniqueEmail('dup')
    await registerAndLogin(page, email)

    // register again with the same email
    await page.goto('/register')
    await page.getByPlaceholder('you@company.com').fill(email)
    await page.getByPlaceholder('At least 8 characters').fill('password123')
    await page.getByRole('button', { name: /create account/i }).click()
    await expect(page.getByText(/already exists/i)).toBeVisible()
  })

  test('login with wrong password shows error', async ({ page }) => {
    const email = uniqueEmail('wrongpw')
    await registerAndLogin(page, email)

    // log out, try wrong password
    await page.getByTitle('Log out').click()
    await expect(page).toHaveURL(/\/login$/)
    await page.getByPlaceholder('you@company.com').fill(email)
    await page.getByPlaceholder('••••••••').fill('WRONG-password')
    await page.getByRole('button', { name: /sign in/i }).click()
    await expect(page.getByText(/invalid email or password/i)).toBeVisible()
  })

  test('protected route without JWT redirects to /login', async ({ page, context }) => {
    await context.clearCookies()
    await page.goto('/clusters')
    await expect(page).toHaveURL(/\/login$/)
  })
})
