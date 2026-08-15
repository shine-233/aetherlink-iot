const fs = require('fs')
const path = require('path')
const { chromium } = require('@playwright/test')

const apiClient = require('../lib/api_client')

const previewBaseURL = process.env.NATIVE_PREVIEW_URL || 'http://127.0.0.1:9730'
const browserChannel = process.env.PLAYWRIGHT_BROWSER_CHANNEL || 'msedge'
const outputDir = path.resolve(__dirname, '..', 'output', 'playwright')
const authStatePath = path.resolve(__dirname, '..', 'e2e', '.auth', 'tenant-admin.json')

const uniqueName = `Public native browser smoke ${Date.now()}`
const rendererData = {
  version: 1,
  columns: 24,
  rowHeight: 60,
  widgets: [
    {
      id: 'public-smoke-text',
      x: 0,
      y: 0,
      w: 12,
      h: 2,
      type: 'text',
      config: { text: 'Public native smoke' }
    }
  ]
}

function responseCode(response) {
  return response && typeof response.code === 'number' ? response.code : null
}

function requireOK(response, label) {
  if (!response || response.code !== 200) {
    throw new Error(`${label} returned code ${responseCode(response)}`)
  }
  return response.data
}

function addPageDiagnostics(page) {
  const pageErrors = []
  const failedRequests = []
  page.on('pageerror', error => pageErrors.push(String(error && error.message ? error.message : error)))
  page.on('requestfailed', request => failedRequests.push(request.url()))
  return { pageErrors, failedRequests }
}

async function waitForVisibleText(page, text) {
  await page.waitForFunction(expected => document.body && document.body.innerText.includes(expected), text, {
    timeout: 30000
  })
}

function localStorageEntriesFromState() {
  if (!fs.existsSync(authStatePath)) throw new Error('tenant admin browser auth state is missing')
  const state = JSON.parse(fs.readFileSync(authStatePath, 'utf8'))
  return state.origins.flatMap(origin => origin.localStorage || []).map(entry => ({
    name: entry.name,
    value: entry.value
  }))
}

async function main() {
  fs.mkdirSync(outputDir, { recursive: true })

  let boardId = null
  let browser = null
  const result = {
    createCode: null,
    uiListVisible: false,
    uiPublishCode: null,
    uiPublishButtonDisabled: false,
    copyLinkWorks: false,
    sharedCode: null,
    publicPreviewCode: null,
    publicTextVisible: false,
    screenshots: {
      dashboardList: path.join(outputDir, 'native-publish-dashboard-list.png'),
      dashboardPublished: path.join(outputDir, 'native-publish-dashboard-published.png'),
      publicPreview: path.join(outputDir, 'native-publish-public-preview.png')
    },
    diagnostics: { dashboardPageErrors: 0, publicPageErrors: 0, publicFailedRequests: 0 }
  }

  try {
    await apiClient.login('tenant_admin')
    const createResponse = await apiClient.post(
      '/board',
      {
        name: uniqueName,
        home_flag: 'N',
        menu_flag: 'N',
        vis_type: 'native',
        config: JSON.stringify(rendererData)
      },
      'tenant_admin'
    )
    const createdBoard = requireOK(createResponse, 'create native board')
    boardId = createdBoard.id
    result.createCode = createResponse.code

    browser = await chromium.launch({ channel: browserChannel, headless: true })

    const authContext = await browser.newContext({
      viewport: { width: 1440, height: 1000 },
      permissions: ['clipboard-read', 'clipboard-write']
    })
    await authContext.addInitScript(entries => {
      for (const entry of entries) window.localStorage.setItem(entry.name, entry.value)
    }, localStorageEntriesFromState())

    const dashboardPage = await authContext.newPage()
    const dashboardDiagnostics = addPageDiagnostics(dashboardPage)
    const dashboardURL = new URL('/visualization/native-boards', previewBaseURL)
    await dashboardPage.goto(dashboardURL.toString(), { waitUntil: 'domcontentloaded', timeout: 30000 })
    const boardCard = dashboardPage.locator('[data-testid="native-board-item"]').filter({ hasText: uniqueName })
    await boardCard.waitFor({ state: 'visible', timeout: 30000 })
    result.uiListVisible = true
    await dashboardPage.screenshot({ path: result.screenshots.dashboardList, fullPage: true })

    const publishButton = boardCard.locator('[data-testid="native-board-publish-button"]')
    const publishResponsePromise = dashboardPage.waitForResponse(
      response =>
        response.request().method() === 'POST' &&
        response.url().includes(`/api/v1/board/${boardId}/publish`),
      { timeout: 30000 }
    )
    await publishButton.click()
    const publishResponse = await publishResponsePromise
    result.uiPublishCode = publishResponse.status()
    await dashboardPage.waitForTimeout(500)
    result.uiPublishButtonDisabled = await publishButton.isDisabled()
    await dashboardPage.screenshot({ path: result.screenshots.dashboardPublished, fullPage: true })

    await boardCard.locator('[data-testid="native-board-copy-link-button"]').click()
    const clipboardValue = await dashboardPage.evaluate(async () => {
      try {
        return await navigator.clipboard.readText()
      } catch {
        return ''
      }
    })
    result.copyLinkWorks = clipboardValue.includes('/tv-preview') && clipboardValue.includes('shareToken=')
    result.diagnostics.dashboardPageErrors = dashboardDiagnostics.pageErrors.length
    await authContext.close()

    const detailResponse = await apiClient.get(`/board/${boardId}`, {}, 'tenant_admin')
    const publishedBoard = requireOK(detailResponse, 'read published native board')
    if (!publishedBoard.share_token) throw new Error('published native board has no share token')

    const sharedResponse = await apiClient.getNoAuth(`/board/shared/${encodeURIComponent(publishedBoard.share_token)}`)
    const sharedBoard = requireOK(sharedResponse, 'read shared native board')
    result.sharedCode = sharedResponse.code

    const publicContext = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
    const publicPage = await publicContext.newPage()
    const publicDiagnostics = addPageDiagnostics(publicPage)
    const publicURL = new URL('/tv-preview', previewBaseURL)
    publicURL.searchParams.set('id', boardId)
    publicURL.searchParams.set('projectId', 'native-boards')
    publicURL.searchParams.set('provider', 'native')
    publicURL.searchParams.set('shareToken', publishedBoard.share_token)
    const publicResponse = await publicPage.goto(publicURL.toString(), {
      waitUntil: 'domcontentloaded',
      timeout: 30000
    })
    result.publicPreviewCode = publicResponse ? publicResponse.status() : null
    await waitForVisibleText(publicPage, 'Public native smoke')
    result.publicTextVisible = await publicPage.getByText('Public native smoke', { exact: true }).isVisible()
    await publicPage.screenshot({ path: result.screenshots.publicPreview, fullPage: true })
    result.diagnostics.publicPageErrors = publicDiagnostics.pageErrors.length
    result.diagnostics.publicFailedRequests = publicDiagnostics.failedRequests.length
    await publicContext.close()

    if (sharedBoard.id !== boardId) throw new Error('shared board id does not match created board')
    if (result.uiPublishCode !== 200) throw new Error(`UI publish returned HTTP ${result.uiPublishCode}`)
    if (!result.uiPublishButtonDisabled) throw new Error('published card did not disable the publish action')
    if (!result.publicTextVisible) throw new Error('public preview text is not visible')

    console.log(JSON.stringify(result, null, 2))
  } finally {
    if (browser) await browser.close().catch(() => {})
    if (boardId) {
      const deleteResponse = await apiClient.delete(`/board/${boardId}`, {}, 'tenant_admin')
      result.deleteCode = responseCode(deleteResponse)
    }
    apiClient.clearAllTokens()
  }
}

main().catch(error => {
  console.error(JSON.stringify({ ok: false, message: error.message }, null, 2))
  process.exitCode = 1
})
