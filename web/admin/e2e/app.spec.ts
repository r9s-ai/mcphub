import { expect, test, type Page } from '@playwright/test'

const overview = { connect_total: 1, connect_online: 1, component_total: 1, component_online: 1, component_offline: 0, last_updated: '2026-08-21T08:00:00Z' }
const component = { id: 'docs', connect_id: 'laptop-1', name: 'Documentation', transport: 'streamable-http', upstream_url: 'http://127.0.0.1:8765/mcp', status: 'online', last_heartbeat: '2026-08-21T08:00:00Z', registered_at: '2026-08-21T07:00:00Z', public_url: 'https://mcp.example.com/mcp/demo/docs' }
const connect = { id: 'laptop-1', name: 'Developer Laptop', version: '0.1.0', status: 'online', connected_at: '2026-08-21T07:00:00Z', last_heartbeat: '2026-08-21T08:00:00Z', components: [component] }

async function mockAdmin(page: Page) {
  await page.route('**/api/admin/**', async route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (request.headers().authorization !== 'Bearer correct-token') return route.fulfill({ status: 401, body: 'admin authentication required' })
    if (path === '/api/admin/overview') return route.fulfill({ json: overview })
    if (path === '/api/admin/connects') return route.fulfill({ json: { connects: [connect] } })
    if (path === '/api/admin/components') return route.fulfill({ json: { components: [component] } })
    if (path === '/api/admin/components/docs') return route.fulfill({ json: component })
    if (path === '/api/admin/groups') return route.fulfill({ json: { groups: [{ id: 'default', name: 'Default', description: '默认工具', is_default: true }] } })
    if (path === '/api/admin/groups/default/tools') return route.fulfill({ json: { tools: [{ component: 'docs', name: 'search_docs', enabled: true }] } })
    if (path.includes('/api/admin/tokens/')) return request.method() === 'GET' ? route.fulfill({ json: { tenant_id: 'demo', default_group_id: 'default', group_ids: ['default'] } }) : route.fulfill({ status: 204 })
    if (path.includes('/catalog/components/')) return route.fulfill({ status: 202 })
    return route.fulfill({ status: 404 })
  })
}

test('落地页突出连接治理与聚合动态发现', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: /连接本地 MCP/ })).toBeVisible()
  await expect(page.getByRole('heading', { name: '聚合动态发现' })).toBeVisible()
  await expect(page.getByText('mcphub_search', { exact: true })).toBeVisible()
  await expect(page.getByText('4 个稳定元工具')).toBeVisible()
  await expect(page.getByRole('link', { name: /进入管理中心/ }).first()).toHaveAttribute('href', '/admin')
  await page.setViewportSize({ width: 390, height: 844 })
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth)).toBe(390)
})

test('Admin Token 登录后可访问五个业务菜单', async ({ page }) => {
  await mockAdmin(page)
  await page.goto('/admin/components/docs')
  await expect(page).toHaveURL(/\/admin\/login\?redirect=/)
  await page.getByLabel('Admin Token').fill('correct-token')
  await page.getByRole('button', { name: /进入管理中心/ }).click()
  await expect(page).toHaveURL(/\/admin\/components\/docs$/)
  await expect(page.getByRole('heading', { level: 1, name: 'Documentation' })).toBeVisible()
  for (const label of ['概览', '连接实例', 'MCP 服务', '工具分组', 'Token 授权']) await expect(page.getByRole('link', { name: label })).toBeVisible()
  await page.getByRole('link', { name: '工具分组' }).click()
  await expect(page.getByText('按需发现，而不是一次加载全部工具')).toBeVisible()
  await expect(page.getByText('docs.search_docs')).toBeVisible()
})
