'use strict'

/*
 * Purpose: inspect every canonical page against the currently running preview
 * proxy and backend, saving one real-browser screenshot plus console/network
 * evidence per route.
 *
 * This is deliberately separate from visual-page-sweep.js. That helper mocks
 * browser API responses for layout inspection; this script uses the live
 * AetherLink runtime and reports redirects, forbidden pages and optional
 * ThingsVis availability without turning them into false passes.
 */

const fs = require('fs')
const path = require('path')
const { chromium } = require('playwright')
const pageCoverage = require('../lib/page_coverage')
const apiClient = require('../lib/api_client')

const baseURL = String(process.env.LIVE_PAGE_BASE_URL || process.env.PREVIEW_URL || 'http://127.0.0.1:9725').replace(/\/$/, '')
const authStatePath = path.resolve(
  process.env.E2E_AUTH_STATE || path.join(__dirname, '..', 'e2e', '.auth', 'tenant-admin.json')
)
const outputDir = path.resolve(
  process.env.LIVE_PAGE_OUTPUT_DIR ||
    path.join(__dirname, '..', 'output', 'playwright', `live-page-audit-${new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14)}`)
)

const extraRoutes = [
  { route: '/', module: 'supplementary', name: 'Root redirect', priority: 'P1' },
  { route: '/first-device', module: 'supplementary', name: 'First device', priority: 'P0' },
  { route: '/terms', module: 'supplementary', name: 'Terms', priority: 'P2' },
  { route: '/privacy', module: 'supplementary', name: 'Privacy', priority: 'P2' },
  { route: '/login/code-login', module: 'auth', name: 'Code login', priority: 'P2' },
  { route: '/login/register-email', module: 'auth', name: 'Email register', priority: 'P2' },
  { route: '/login/register-super-admin', module: 'auth', name: 'Super-admin register', priority: 'P2' },
  { route: '/login/bind-wechat', module: 'auth', name: 'Bind WeChat', priority: 'P2' },
  { route: '/device/config', module: 'device', name: 'Device config bridge', priority: 'P2' },
  { route: '/tv-preview', module: 'visualization', name: 'Standalone ThingsVis preview', priority: 'P1' }
]

const routes = [
  ...pageCoverage.getCatalog().pages,
  ...extraRoutes
]

const queryOverrides = {
  '/device/config-detail': '?templateId=live-template',
  '/device/config-edit': '?templateId=live-template',
  '/device/details': '?d_id={deviceId}',
  '/device/details-child': '?d_id={deviceId}',
  '/device/grouping-details': '?groupId=live-group',
  '/device/service-details': '?id=live-service',
  '/device/share': '?id={deviceId}',
  // The authenticated standalone app uses the existing browser auth state.
  // Supplying a fake token here would overwrite the real token and create a
  // deterministic 401 that belongs to the audit fixture, not the page.
  '/device-details-app': '?d_id={deviceId}',
  '/visualization/native-board': '?id={boardId}',
  '/visualization/native-board-editor': '?id={boardId}',
  '/visualization/thingsvis-dashboards': '?projectId=qa-project',
  '/visualization/thingsvis-editor': '?id=qa-dashboard&projectId=qa-project',
  '/visualization/thingsvis-menu-dashboard': '?id=qa-dashboard',
  '/visualization/thingsvis-preview': '?id=qa-dashboard',
  '/tv-preview': '?id=qa-dashboard'
}

function routePath(item) {
  const suffix = queryOverrides[item.route]
  if (!suffix) return item.route
  const resolvedSuffix = suffix
    .replace('{deviceId}', runtimeFixture.deviceId || 'qa-device')
    .replace('{boardId}', runtimeFixture.boardId || 'qa-board')
  return `${item.route}${resolvedSuffix}`
}

function normalizedPath(value) {
  try {
    return new URL(value, baseURL).pathname.replace(/\/+$/, '') || '/'
  } catch {
    return String(value || '/').split('?')[0].replace(/\/+$/, '') || '/'
  }
}

function isThingsVisRoute(route) {
  return route.includes('thingsvis') || route === '/tv-preview'
}

function isAnonymousRoute(route) {
  return route === '/' || route.startsWith('/login') || route === '/terms' || route === '/privacy'
}

function classifyResult(item, finalPath, body, evidence) {
  const bodyText = String(body || '')
  const lowerBody = bodyText.toLowerCase()
  if (evidence.screenshotError || evidence.navigationError) return 'runtime-error'
  if (finalPath === '/403' || /无权限|没有权限|forbidden|permission denied/i.test(bodyText)) return 'forbidden'

  if (isThingsVisRoute(item.route)) {
    const externalFailureBody = /unable to load|failed to load|database error|login expired|external.{0,20}(blocked|unavailable)/i.test(bodyText)
    if (
      externalFailureBody ||
      evidence.httpErrors.some(error => error.status >= 500) ||
      /optional.{0,20}(disabled|未启用)|external.{0,20}(blocked|unavailable)|thingsvis.{0,30}(disabled|unavailable)|服务未启用|外部服务不可用/i.test(lowerBody)
    ) return 'optional-disabled'
    if (evidence.httpErrors.length || evidence.failedRequests.length) return 'external-blocked'
  }

  if (/unable to load|failed to load|database error|login expired/i.test(bodyText)) return 'runtime-error'
  if (evidence.pageErrors.length || evidence.consoleErrors.length || evidence.failedRequests.length) return 'runtime-error'
  if (finalPath !== normalizedPath(item.route)) return 'redirected'
  return 'passed'
}

function createHtml(report) {
  const rows = report.pages.map(page => {
    const errorCount = page.consoleErrors + page.pageErrors + page.failedRequests + page.httpErrors.length
    return `<tr><td>${page.name}</td><td>${page.requestedPath}</td><td>${page.finalUrl}</td><td>${page.status}</td><td>${errorCount}</td><td>${page.body.slice(0, 220).replace(/[&<>]/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' })[char])}</td><td><a href="screenshots/${page.screenshotFile}">screenshot</a></td></tr>`
  }).join('\n')
  return `<!doctype html><html><head><meta charset="utf-8"><title>Live page audit</title><style>body{font-family:Arial,sans-serif;margin:20px;background:#f5f7fa;color:#222}table{width:100%;border-collapse:collapse;background:#fff}th,td{padding:8px;border-bottom:1px solid #ddd;text-align:left;font-size:12px}th{background:#1f6feb;color:#fff;position:sticky;top:0}.passed{color:#137333}.runtime-error{color:#b3261e}</style></head><body><h1>Live page audit</h1><p>Base URL: ${report.baseURL}</p><p>Canonical pages: ${report.canonicalPageCount}; supplementary routes: ${report.supplementaryRouteCount}; total inspected: ${report.pages.length}</p><p>Summary: ${JSON.stringify(report.summary)}</p><table><thead><tr><th>Page</th><th>Requested</th><th>Final URL</th><th>Status</th><th>Errors</th><th>Body</th><th>Artifact</th></tr></thead><tbody>${rows}</tbody></table></body></html>`
}

const runtimeFixture = { boardId: '', deviceId: '' }

async function prepareFixture() {
  await apiClient.login('tenant_admin')
  const boardResponse = await apiClient.post('/board', {
    name: `live-page-audit-${Date.now()}`,
    config: JSON.stringify({ version: 1, columns: 24, rowHeight: 60, widgets: [] }),
    home_flag: 'N',
    menu_flag: 'N',
    vis_type: 'native'
  }, 'tenant_admin')
  if (boardResponse && boardResponse.code === 200) {
    runtimeFixture.boardId = String(boardResponse.data?.id || '').trim()
  }

  const deviceResponse = await apiClient.get('/device', { page: 1, page_size: 20 }, 'tenant_admin')
  const deviceRows = Array.isArray(deviceResponse?.data?.list)
    ? deviceResponse.data.list
    : Array.isArray(deviceResponse?.data)
      ? deviceResponse.data
      : []
  runtimeFixture.deviceId = String(deviceRows[0]?.id || deviceRows[0]?.device_id || '').trim()
}

async function cleanupFixture() {
  if (runtimeFixture.boardId) {
    await apiClient.delete(`/board/${runtimeFixture.boardId}`, {}, 'tenant_admin')
  }
}

async function inspectPage(page, item) {
  const requestedPath = routePath(item)
  const requestedUrl = `${baseURL}${requestedPath}`
  const consoleErrors = []
  const pageErrors = []
  const failedRequests = []
  const httpErrors = []

  page.on('console', message => {
    if (message.type() === 'error') consoleErrors.push(message.text())
  })
  page.on('pageerror', error => pageErrors.push(error.message))
  page.on('requestfailed', request => {
    failedRequests.push({ url: request.url(), error: request.failure()?.errorText || 'failed' })
  })
  page.on('response', response => {
    if (response.status() >= 400) httpErrors.push({ url: response.url(), status: response.status() })
  })

  let navigationError = ''
  try {
    await page.goto(requestedUrl, { waitUntil: 'domcontentloaded', timeout: 20000 })
    await page.waitForTimeout(600)
    await page.waitForLoadState('networkidle', { timeout: 2500 }).catch(() => {})
  } catch (error) {
    navigationError = error.message
  }

  let title = ''
  let body = ''
  try {
    title = await page.title()
    body = (await page.locator('body').innerText()).replace(/\s+/g, ' ').trim().slice(0, 1200)
  } catch (error) {
    navigationError = navigationError || error.message
  }

  const screenshotFile = `${item.module}-${item.route.replace(/^\//, '').replace(/[^a-zA-Z0-9_-]+/g, '_') || 'root'}.png`
  let screenshotError = ''
  try {
    await page.screenshot({ path: path.join(outputDir, 'screenshots', screenshotFile), fullPage: true, type: 'png' })
  } catch (error) {
    screenshotError = error.message
  }

  const finalUrl = page.url()
  const finalPath = normalizedPath(finalUrl)
  const evidence = { consoleErrors, pageErrors, failedRequests, httpErrors, navigationError, screenshotError }
  return {
    route: item.route,
    name: item.name,
    module: item.module,
    priority: item.priority,
    requestedPath,
    requestedUrl,
    finalUrl,
    title,
    body,
    status: classifyResult(item, finalPath, body, evidence),
    screenshotFile,
    ...evidence
  }
}

async function main() {
  if (!fs.existsSync(authStatePath)) throw new Error(`auth state not found: ${authStatePath}`)
  fs.mkdirSync(path.join(outputDir, 'screenshots'), { recursive: true })
  await prepareFixture()

  const browser = await chromium.launch({
    channel: process.env.BROWSER_CHANNEL || 'msedge',
    headless: process.env.LIVE_PAGE_HEADED !== '1'
  })
  const pages = []
  try {
    for (const item of routes) {
      // Keep every route in an isolated browser context. Some legacy pages
      // intentionally clear or replace auth state (for example the standalone
      // device-details app); reusing one context would make later unrelated
      // pages appear to have 401/runtime failures.
      const context = await browser.newContext({
        viewport: { width: 1440, height: 900 },
        ...(isAnonymousRoute(item.route) ? {} : { storageState: authStatePath })
      })
      const page = await context.newPage()
      try {
        pages.push(await inspectPage(page, item))
      } finally {
        await page.close().catch(() => {})
        await context.close().catch(() => {})
      }
    }
  } finally {
    await browser.close().catch(() => {})
    await cleanupFixture().catch(error => pages.push({ route: '<fixture>', name: 'fixture cleanup', status: 'runtime-error', body: error.message }))
  }

  const summary = pages.reduce((counts, page) => {
    counts[page.status] = (counts[page.status] || 0) + 1
    return counts
  }, {})
  const report = {
    generatedAt: new Date().toISOString(),
    baseURL,
    authStatePath,
    canonicalPageCount: pageCoverage.getCatalog().pages.length,
    supplementaryRouteCount: extraRoutes.length,
    fixture: { boardId: runtimeFixture.boardId || null, deviceId: runtimeFixture.deviceId || null },
    summary,
    pages
  }
  fs.writeFileSync(path.join(outputDir, 'live-page-audit.json'), JSON.stringify(report, null, 2), 'utf8')
  fs.writeFileSync(path.join(outputDir, 'live-page-audit.html'), createHtml(report), 'utf8')
  console.log(JSON.stringify({ outputDir, summary, total: pages.length }, null, 2))
}

main().catch(error => {
  console.error(error.stack || error.message)
  process.exitCode = 1
})
