import { test, expect } from '@playwright/test'
import { uniqueEmail, uniqueClusterName, registerAndLogin } from './helpers'

test.describe('Clusters', () => {
  test('register a cluster, see it in the list, then delete it', async ({ page }) => {
    await registerAndLogin(page, uniqueEmail('clusters'))
    const name = uniqueClusterName('prod')

    // register — point at the local Prometheus we run in docker-compose so status = connected
    await page.getByRole('link', { name: /register cluster/i }).click()
    await expect(page).toHaveURL(/\/clusters\/new$/)
    await page.getByPlaceholder('production-us-east').fill(name)
    await page.getByPlaceholder('https://prometheus.your-cluster.example.com').fill('http://localhost:9090')
    await page.getByPlaceholder('Bearer token for Prometheus authentication').fill('no-token-needed-for-local')
    await page.getByRole('button', { name: /register cluster$/i }).click()

    // lands on cluster detail
    await expect(page).toHaveURL(/\/clusters\/[a-f0-9-]+$/)
    await expect(page.getByRole('heading', { name })).toBeVisible()
    await expect(page.getByText('connected').first()).toBeVisible()

    // go back to list — cluster row is there
    await page.getByRole('link', { name: /back to clusters/i }).click()
    await expect(page.getByRole('link', { name: new RegExp(name) })).toBeVisible()

    // delete via row action (confirm dialog)
    page.on('dialog', (d) => d.accept())
    await page.getByTitle('Delete').last().click()
    await expect(page.getByRole('link', { name: new RegExp(name) })).toHaveCount(0)
  })

  test('POST /clusters rejects an invalid prometheus_url', async ({ page }) => {
    await registerAndLogin(page, uniqueEmail('badurl'))
    await page.goto('/clusters/new')
    await page.getByPlaceholder('production-us-east').fill(uniqueClusterName('bad'))
    await page.getByPlaceholder('https://prometheus.your-cluster.example.com').fill('not-a-url')
    await page.getByPlaceholder('Bearer token for Prometheus authentication').fill('x')
    await page.getByRole('button', { name: /register cluster$/i }).click()
    await expect(page.getByText(/must be a full URL/i)).toBeVisible()
  })

  test('edit cluster updates the name', async ({ page }) => {
    await registerAndLogin(page, uniqueEmail('edit'))
    const original = uniqueClusterName('orig')
    const updated  = uniqueClusterName('edited')

    // register a cluster
    await page.goto('/clusters/new')
    await page.getByPlaceholder('production-us-east').fill(original)
    await page.getByPlaceholder('https://prometheus.your-cluster.example.com').fill('http://localhost:9090')
    await page.getByPlaceholder('Bearer token for Prometheus authentication').fill('t')
    await page.getByRole('button', { name: /register cluster$/i }).click()
    await expect(page.getByRole('heading', { name: original })).toBeVisible()

    // edit — first textbox is the pre-filled Cluster name field
    await page.getByRole('link', { name: /^edit$/i }).click()
    await expect(page).toHaveURL(/\/edit$/)
    const nameField = page.locator('input[type="text"]').first()
    await nameField.fill(updated)
    await page.getByRole('button', { name: /save changes/i }).click()

    // back on cluster detail with new name
    await expect(page.getByRole('heading', { name: updated })).toBeVisible()
  })
})
