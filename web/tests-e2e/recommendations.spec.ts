import { test, expect } from '@playwright/test'
import { uniqueEmail, uniqueClusterName, registerAndLogin } from './helpers'

test.describe('Recommendations', () => {
  test('recalculate is enabled when connected', async ({ page }) => {
    await registerAndLogin(page, uniqueEmail('recon'))

    await page.goto('/clusters/new')
    await page.getByPlaceholder('production-us-east').fill(uniqueClusterName('reco'))
    await page.getByPlaceholder('https://prometheus.your-cluster.example.com').fill('http://localhost:9090')
    await page.getByPlaceholder('Bearer token for Prometheus authentication').fill('t')
    await page.getByRole('button', { name: /register cluster$/i }).click()

    // connected → Recalculate button is enabled
    const btn = page.getByRole('button', { name: /^recalculate$/i })
    await expect(btn).toBeEnabled()
    await expect(page.getByText(/cluster is disconnected/i)).toHaveCount(0)
  })

  test('recalculate is disabled + warning shown when disconnected', async ({ page }) => {
    await registerAndLogin(page, uniqueEmail('recdis'))

    // register a valid URL that will fail to connect
    await page.goto('/clusters/new')
    await page.getByPlaceholder('production-us-east').fill(uniqueClusterName('discon'))
    await page.getByPlaceholder('https://prometheus.your-cluster.example.com').fill('http://127.0.0.1:1')
    await page.getByPlaceholder('Bearer token for Prometheus authentication').fill('t')
    await page.getByRole('button', { name: /register cluster$/i }).click()

    // status should be disconnected → banner visible, button disabled
    await expect(page.getByText(/cluster is disconnected/i)).toBeVisible()
    await expect(page.getByRole('button', { name: /^recalculate$/i })).toBeDisabled()
  })
})
