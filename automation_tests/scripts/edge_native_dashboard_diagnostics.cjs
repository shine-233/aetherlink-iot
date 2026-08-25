const fs = require('fs')
const path = require('path')
const { chromium } = require('@playwright/test')

const previewBaseURL = process.env.NATIVE_PREVIEW_URL || 'http://127.0.0.1:9730'
const browserChannel = process.env.PLAYWRIGHT_BROWSER_CHANNEL || 'chromium'
const authStatePath = path.resolve(__dirname, '..', 'e2e', '.auth', 'tenant-admin.json')
const screenshotPath = path.resolve(__dirname, '..', 'output', 'playwright', 'native-dashboard-diagnostics.png')

function authEntries() {
  const state = JSON.parse(fs.readFileSync(authStatePath, 'utf8'))
  return state.origins.flatMap(origin => origin.localStorage || []).map(entry => ({ name: entry.name, value: entry.value }))
}

async function main() {
  const browser = await chromium.launch({ channel: browserChannel, headless: true })
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  await context.addInitScript(entries => {
    for (const entry of entries) window.localStorage.setItem(entry.name, entry.value)
  }, authEntries())
  const page = await context.newPage()
  const responses = []
  const pageErrors = []
  const consoleErrors = []
  let menuSummary = null
  page.on('response', response => {
    if (response.url().includes('/api/') || response.url().includes('/assets/')) {
      responses.push({ url: new URL(response.url()).pathname, status: response.status() })
    }
  })
  page.on('response', async response => {
    if (!response.url().includes('/api/v1/ui_elements/menu')) return
    try {
      const body = await response.json()
      const walk = (items, depth = 0) =>
        (Array.isArray(items) ? items : []).flatMap(item => [
          {
            depth,
            title: item.title,
            element_code: item.element_code,
            param1: item.param1,
            route_path: item.route_path,
            children: Array.isArray(item.children) ? item.children.length : 0
          },
          ...walk(item.children, depth + 1)
        ])
      menuSummary = walk(body?.data?.list || body?.data || body?.list || body)
    } catch {
      menuSummary = { parseFailed: true }
    }
  })
  page.on('pageerror', error => pageErrors.push(String(error.message || error)))
  page.on('console', message => {
    if (message.type() === 'error') consoleErrors.push(message.text())
  })

  const url = new URL('/visualization/thingsvis-dashboards', previewBaseURL)
  url.searchParams.set('projectId', 'native-boards')
  url.searchParams.set('provider', 'native')
  const response = await page.goto(url.toString(), { waitUntil: 'domcontentloaded', timeout: 30000 })
  await page.waitForTimeout(5000)
  fs.mkdirSync(path.dirname(screenshotPath), { recursive: true })
  await page.screenshot({ path: screenshotPath, fullPage: true })
  const state = await page.evaluate(() => ({
    href: window.location.href,
    title: document.title,
    appLength: document.querySelector('#app')?.innerHTML.length || 0,
    cardCount: document.querySelectorAll('[data-testid="thingsvis-dashboard-card"]').length,
    bodyText: document.body?.innerText?.slice(0, 1200) || '',
    localStorageKeys: Object.keys(window.localStorage)
  }))
  console.log(JSON.stringify({
    httpStatus: response && response.status(),
    state,
    pageErrors,
    consoleErrors,
    menuSummary,
    responses: responses.slice(-80),
    screenshotPath
  }, null, 2))
  await browser.close()
}

main().catch(error => {
  console.error(JSON.stringify({ ok: false, message: error.message }, null, 2))
  process.exitCode = 1
})
