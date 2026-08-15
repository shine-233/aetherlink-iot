const path = require('node:path')

async page => {
  const runtimeEnv = typeof process !== 'undefined' && process.env ? process.env : {}
  const projectRoot = runtimeEnv.AETHERLINK_PROJECT_ROOT || path.resolve(__dirname, '..', '..')
  const initialPageUrl = typeof page.url === 'function' ? page.url() : ''
  const queryOption = queryKey => {
    const match = initialPageUrl.match(new RegExp(`[?&]${queryKey}=([^&]*)`))
    return match ? decodeURIComponent(match[1]) : ''
  }
  const option = (environmentKey, queryKey, fallback) => runtimeEnv[environmentKey] || queryOption(queryKey) || fallback

  const allItems = [
    ["root", "/"],
    ["first-device", "/first-device"],
    ["terms", "/terms"],
    ["privacy", "/privacy"],
    ["403", "/403"],
    ["404", "/404"],
    ["500", "/500"],
    ["login", "/login"],
    ["login-pwd", "/login/pwd-login"],
    ["login-code", "/login/code-login"],
    ["login-register", "/login/register"],
    ["login-register-email", "/login/register-email"],
    ["login-register-super-admin", "/login/register-super-admin"],
    ["login-reset-pwd", "/login/reset-pwd"],
    ["login-bind-wechat", "/login/bind-wechat"],
    ["alarm-notification-group", "/alarm/notification-group"],
    ["alarm-notification-record", "/alarm/notification-record"],
    ["alarm-rdi-overview", "/alarm/rdi-overview"],
    ["alarm-warning-message", "/alarm/warning-message"],
    ["apply-plugin", "/apply/plugin"],
    ["apply-service", "/apply/service"],
    ["automation-linkage-edit", "/automation/linkage-edit"],
    ["automation-scene-edit", "/automation/scene-edit"],
    ["automation-scene-linkage", "/automation/scene-linkage"],
    ["automation-scene-manage", "/automation/scene-manage"],
    ["dashboard-workspace", "/dashboard/workspace"],
    ["dashboard-rdi-overview", "/dashboard/rdi-overview"],
    ["dashboard-workbench", "/dashboard/workbench"],
    ["device-command-center", "/device/command-center"],
    ["device-template-config", "/device/template"],
    ["device-config-bridge", "/device/config"],
    ["device-config-detail", "/device/config-detail?templateId=qa-template"],
    ["device-config-edit", "/device/config-edit?templateId=qa-template"],
    ["device-details", "/device/details?d_id=qa-device"],
    ["device-details-child", "/device/details-child?d_id=qa-device"],
    ["device-grouping", "/device/grouping"],
    ["device-grouping-details", "/device/grouping-details?groupId=qa-group"],
    ["device-manage", "/device/manage"],
    ["device-service-access", "/device/service-access"],
    ["device-service-details", "/device/service-details?id=qa-service"],
    ["device-share", "/device/share?id=qa-device"],
    ["device-shared-with-me", "/device/shared-with-me"],
    ["device-thingsmodel", "/device/thingsmodel"],
    ["device-details-app", "/device-details-app?d_id=qa-device&token=visual-token"],
    ["management-api", "/management/api"],
    ["management-auth", "/management/auth"],
    ["management-notification", "/management/notification"],
    ["management-role", "/management/role"],
    ["management-setting", "/management/setting"],
    ["management-user", "/management/user"],
    ["personal-center", "/personal-center"],
    ["product-update-ota", "/product/update-ota"],
    ["product-update-package", "/product/update-package"],
    ["system-equipment-map", "/system-management-user/equipment-map"],
    ["system-log", "/system-management-user/system-log"],
    ["native-boards", "/visualization/native-boards"],
    ["native-board", "/visualization/native-board?id=qa-board"],
    ["native-board-editor", "/visualization/native-board-editor?id=qa-board"],
    ["thingsvis-projects", "/visualization/thingsvis"],
    ["thingsvis-dashboards", "/visualization/thingsvis-dashboards?projectId=qa-project"],
    ["thingsvis-editor", "/visualization/thingsvis-editor?id=qa-dashboard&projectId=qa-project"],
    ["thingsvis-menu-dashboard", "/visualization/thingsvis-menu-dashboard?id=qa-dashboard"],
    ["thingsvis-preview", "/visualization/thingsvis-preview?id=qa-dashboard"],
    ["thingsvis-standalone-preview", "/tv-preview?id=qa-dashboard"]
  ].map(([key, path]) => ({ key, url: `http://127.0.0.1:9725${path}` }))
  const rangeStart = Number(option('VISUAL_PAGE_START', 'visualStart', 0))
  const rangeEnd = Number(option('VISUAL_PAGE_END', 'visualEnd', allItems.length))
  const items = allItems.slice(rangeStart, rangeEnd)

  // The real backend is intentionally not started for this visual-only pass.
  // Keep the preview proxy and production API contracts untouched; fulfil only
  // the browser requests needed to let each page render its actual layout.
  const projectFixture = {
    id: 'qa-project',
    name: 'Visual QA project',
    description: 'Temporary visual inspection fixture',
    thumbnail: null,
    tenantId: 'qa-tenant',
    createdById: 'visual-user',
    createdAt: '2026-01-01T00:00:00.000Z',
    updatedAt: '2026-01-01T00:00:00.000Z',
    _count: { dashboards: 1 }
  }
  const dashboardFixture = {
    id: 'qa-dashboard',
    name: 'Visual QA dashboard',
    thumbnail: null,
    version: 1,
    canvasConfig: { mode: 'fixed', width: 1920, height: 1080, background: '#f5f7fa' },
    nodes: [],
    dataSources: [],
    variables: [],
    isPublished: false,
    publishedAt: null,
    shareToken: null,
    projectId: 'qa-project',
    createdById: 'visual-user',
    createdAt: '2026-01-01T00:00:00.000Z',
    updatedAt: '2026-01-01T00:00:00.000Z'
  }
  const dashboardSummary = {
    id: dashboardFixture.id,
    name: dashboardFixture.name,
    thumbnail: null,
    version: dashboardFixture.version,
    isPublished: false,
    publishedAt: null,
    shareToken: null,
    homeFlag: false,
    projectId: dashboardFixture.projectId,
    createdAt: dashboardFixture.createdAt,
    updatedAt: dashboardFixture.updatedAt,
    project: { id: projectFixture.id, name: projectFixture.name },
    createdBy: { id: 'visual-user', name: 'Visual QA' }
  }

  const pathOf = value => value.replace(/^[a-z]+:\/\/[^/]+/i, '').split('?')[0] || '/'

  const explicitRoutePathByPath = {
    '/personal-center': 'layout.base$view.personal-center'
  }

  const homeMenuItem = {
    id: 'visual-home',
    parent_id: '0',
    title: 'home',
    multilingual: 'home',
    param2: 'mdi:home-outline',
    element_code: 'home',
    param1: '/home',
    param3: '0',
    orders: 0,
    element_type: 3,
    authority: '[]',
    description: 'home',
    remark: '',
    route_path: 'layout.base$view.home',
    children: []
  }

  const dynamicPageItems = items
    .map(item => {
      const path = pathOf(item.url)
      if (
        path === '/' ||
        path === '/first-device' ||
        path === '/terms' ||
        path === '/privacy' ||
        path === '/403' ||
        path === '/404' ||
        path === '/500' ||
        path.startsWith('/login') ||
        path === '/device/config' ||
        path === '/device-details-app' ||
        path === '/tv-preview'
      ) {
        return null
      }

      const aliases = {
        '/device/template': 'device_config',
        '/device/thingsmodel': 'device_template',
        '/automation/scene-manage': 'automation_scene-manage'
      }
      const elementCode = aliases[path] || path.replace(/^\//, '').replaceAll('/', '_')
      return {
        id: `visual-${elementCode}`,
        parent_id: '',
        title: elementCode,
        multilingual: elementCode,
        param2: 'mdi:view-dashboard-outline',
        element_code: elementCode,
        param1: path,
        param3: '0',
        orders: 1,
        element_type: 3,
        authority: '[]',
        description: elementCode,
        remark: '',
        route_path: explicitRoutePathByPath[path] || null,
        children: []
      }
    })
    .filter(Boolean)

  // The production menu is hierarchical: a first-level module owns the page
  // routes below it. Keeping that shape matters because the route adapter
  // assigns layout.base to the module and view.* to its children. Returning
  // every page as a top-level item makes the adapter interpret
  // layout.base$view.* as a layout name and sends the browser to /403.
  const topLevelPages = []
  const dynamicGroups = dynamicPageItems.reduce((groups, page) => {
    const pathSegments = pathOf(page.param1).split('/').filter(Boolean)
    const groupCode = pathSegments[0]
    if (!groupCode) return groups

    // A single-segment route such as /personal-center is already a leaf.
    // Wrapping it in a same-named group creates duplicate route names in the
    // dynamic adapter and makes the permission guard fall back to /403.
    if (pathSegments.length === 1) {
      page.parent_id = '0'
      topLevelPages.push(page)
      return groups
    }

    let group = groups.get(groupCode)
    if (!group) {
      group = {
        id: `visual-group-${groupCode}`,
        parent_id: '0',
        title: groupCode,
        multilingual: groupCode,
        param2: 'mdi:view-dashboard-outline',
        element_code: groupCode,
        param1: `/${groupCode}`,
        param3: '0',
        orders: 1,
        element_type: 1,
        authority: '[]',
        description: groupCode,
        remark: '',
        route_path: null,
        children: []
      }
      groups.set(groupCode, group)
    }

    page.parent_id = group.id
    group.children.push(page)
    return groups
  }, new Map())

  const dynamicMenuItems = [homeMenuItem, ...topLevelPages, ...Array.from(dynamicGroups.values())]

  const backendEnvelope = data => ({ code: 200, message: 'ok', data })
  const emptyPage = () => ({ total: 0, list: [] })
  const emptyMonthlyTrend = {
    year: new Date().getFullYear(),
    months: Array.from({ length: 12 }, (_, index) => ({ month: index + 1, count: 0 }))
  }
  const emptyNotificationEmail = {
    id: 'visual-email',
    config: 'null',
    email_config: {},
    notice_type: 'EMAIL',
    status: 'CLOSED',
    remark: ''
  }
  const emptyNotificationSms = {
    id: 'visual-sms',
    config: 'null',
    sme_config: {},
    notice_type: 'SMS',
    status: 'CLOSED',
    remark: ''
  }
  const emptyDeviceDetail = {
    id: 'qa-device',
    name: 'Visual QA device',
    voucher: '',
    tenant_id: 'qa-tenant',
    is_enabled: 'enabled',
    activate_flag: 'inactive',
    created_at: '2026-01-01T00:00:00.000Z',
    update_at: '2026-01-01T00:00:00.000Z',
    device_number: 'QA-DEVICE',
    product_id: '',
    parent_id: '',
    label: '',
    location: '',
    sub_device_addr: '',
    current_version: '',
    additional_info: '{}',
    protocol_config: '{}',
    device_config_name: '',
    remark1: '',
    remark2: '',
    remark3: '',
    device_config_id: '',
    batch_number: '',
    activate_at: '',
    is_online: 0
  }
  const emptyNativeBoard = {
    id: 'qa-board',
    name: 'Visual QA native board',
    tenant_id: 'qa-tenant',
    created_at: '2026-01-01T00:00:00.000Z',
    updated_at: '2026-01-01T00:00:00.000Z',
    home_flag: 'N',
    config: JSON.stringify({ version: 1, columns: 24, rowHeight: 60, widgets: [] }),
    description: 'Temporary visual inspection board',
    remark: null,
    menu_flag: 'N',
    vis_type: 'native'
  }
  const platformData = pathname => {
    const normalizedPathname = pathname.replace(/\/+$/, '') || '/'

    if (normalizedPathname === '/logo') {
      return {
        list: [
          {
            id: 'visual-branding',
            system_name: 'AetherLink IoT',
            logo_background: '',
            logo_loading: '',
            logo_cache: '',
            home_background: ''
          }
        ],
        total: 1
      }
    }
    if (normalizedPathname === '/ui_elements/menu') return { list: dynamicMenuItems, home: 'home' }
    if (normalizedPathname === '/user/detail') {
      return { id: 'visual-user', userId: 'visual-user', userName: 'Visual QA', authority: 'SYS_ADMIN', roles: ['SYS_ADMIN'] }
    }
    if (normalizedPathname === '/tenant/setup-state') {
      return { has_admin: true, has_tenant_admin: true, has_tenant: true, entry: 'login', next_step: 'login' }
    }
    if (normalizedPathname === '/sys_function') return []
    if (normalizedPathname === '/sys_version') {
      return { current_version: '', latest_version: '', version: '', build: '' }
    }
    if (normalizedPathname === '/alarm/info/history/monthly') return emptyMonthlyTrend
    if (normalizedPathname === '/alarm/info/history' || normalizedPathname === '/alarm/info') return emptyPage()
    if (normalizedPathname === '/alarm/device/counts') return emptyPage()
    if (normalizedPathname === '/board/tenant/device/info') return { total: 0, online: 0, offline: 0, list: [] }
    if (normalizedPathname === '/board') return { total: 0, list: [] }
    if (normalizedPathname === '/board/qa-board') return emptyNativeBoard
    if (normalizedPathname === '/command/datas/jobs') return emptyPage()
    if (normalizedPathname === '/command/datas/saved-filters') return emptyPage()
    if (normalizedPathname === '/device') return emptyPage()
    if (normalizedPathname === '/device/group/tree') return []
    if (normalizedPathname === '/device/group') return emptyPage()
    if (normalizedPathname === '/device/template') return emptyPage()
    if (normalizedPathname.startsWith('/device/template/detail/')) return { id: 'qa-template', name: 'Visual QA template', label: '', thing_model: { properties: [], events: [], services: [] } }
    if (normalizedPathname === '/device_config') return emptyPage()
    if (normalizedPathname.startsWith('/device_config/')) return { protocol_config: '{}', device_template_id: '' }
    if (normalizedPathname.startsWith('/device/detail/')) return emptyDeviceDetail
    if (
      normalizedPathname === '/telemetry/datas/current' ||
      normalizedPathname.startsWith('/telemetry/datas/current/')
    ) {
      return []
    }
    if (normalizedPathname === '/notification/services/config/EMAIL') return emptyNotificationEmail
    if (normalizedPathname === '/notification/services/config/SME_CODE') return emptyNotificationSms
    if (normalizedPathname === '/notification/e-mail/templates') return emptyPage()
    if (normalizedPathname === '/message_push/config') return { url: '' }
    if (normalizedPathname === '/open/keys') return emptyPage()
    if (normalizedPathname === '/rdi/shared-with-me/devices') return emptyPage()
    if (normalizedPathname === '/scene') return emptyPage()
    if (normalizedPathname === '/service/plugin/select') return { protocol: [], service: [] }
    if (normalizedPathname === '/ui_elements') return emptyPage()
    if (normalizedPathname.includes('/counts')) return emptyPage()
    if (normalizedPathname.includes('/list') || normalizedPathname.includes('/page') || normalizedPathname.includes('/user')) return emptyPage()
    return {}
  }

  const fulfillPlatform = async route => {
    const request = route.request()
    const pathname = pathOf(request.url()).replace(/^\/api\/v1/, '').replace(/^\/proxy-default/, '') || '/'
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(backendEnvelope(platformData(pathname)))
    })
  }

  const fulfillThingsVis = async route => {
    const request = route.request()
    const pathname = pathOf(request.url()).replace(/^\/thingsvis-api/, '') || '/'
    let data = {}
    if (pathname === '/projects') {
      data = { data: [projectFixture], meta: { page: 1, limit: 20, total: 1, totalPages: 1 } }
    } else if (pathname === '/projects/qa-project') {
      data = projectFixture
    } else if (pathname === '/dashboards') {
      data = { data: [dashboardSummary], meta: { page: 1, limit: 20, total: 1, totalPages: 1 } }
    } else if (pathname === '/dashboards/home') {
      data = { data: null }
    } else if (pathname.endsWith('/thumbnail')) {
      data = { thumbnail: null }
    } else if (pathname.startsWith('/dashboards/')) {
      data = dashboardFixture
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(data) })
  }

  await page.unroute('**/api/v1/**').catch(() => {})
  await page.unroute('**/proxy-default/**').catch(() => {})
  await page.unroute('**/thingsvis-api/**').catch(() => {})
  await page.route('**/api/v1/**', fulfillPlatform)
  await page.route('**/proxy-default/**', fulfillPlatform)
  await page.route('**/thingsvis-api/**', fulfillThingsVis)

  const archiveStamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14)
  const screenshotRoot = option(
    'VISUAL_OUTPUT_DIR',
    'visualOutputDir',
    path.join(projectRoot, 'verification', `visual-page-sweep-${archiveStamp}`, 'frontend-playwright')
  )
  const authState = () => {
    localStorage.setItem('token', JSON.stringify('visual-token'))
    localStorage.setItem('token_expires_in', JSON.stringify(Date.now() + 3600000))
    localStorage.setItem(
      'userInfo',
      JSON.stringify({
        authority: 'SYS_ADMIN',
        id: 'visual-user',
        userId: 'visual-user',
        userName: 'Visual QA',
        roles: ['SYS_ADMIN']
      })
    )
  }
  const clearAuth = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('token_expires_in')
    localStorage.removeItem('userInfo')
  }

  const consoleErrors = []
  const failedRequests = []
  page.on('console', message => {
    if (message.type() === 'error') consoleErrors.push(message.text())
  })
  page.on('pageerror', error => consoleErrors.push(`pageerror: ${error.message}`))
  page.on('requestfailed', request => {
    failedRequests.push(`${request.url()}: ${request.failure()?.errorText || 'failed'}`)
  })

  const results = []
  // Start on a same-origin document so the first auth fixture operation does
  // not touch localStorage from about:blank (which Edge rejects).
  await page.goto('http://127.0.0.1:9725/login', { waitUntil: 'domcontentloaded', timeout: 15000 }).catch(() => {})

  for (const item of items) {
    const isLogin = item.url.includes('/login')
    if (isLogin) await page.evaluate(clearAuth)
    else await page.evaluate(authState)

    const errorStart = consoleErrors.length
    const failedStart = failedRequests.length
    let navigationError = ''
    try {
      await page.goto(item.url, { waitUntil: 'domcontentloaded', timeout: 15000 })
      await page.waitForTimeout(650)
    } catch (error) {
      navigationError = String(error?.message || error)
    }

    let title = ''
    let body = ''
    try {
      title = await page.title()
      body = (await page.locator('body').innerText()).replace(/\s+/g, ' ').trim().slice(0, 260)
    } catch (error) {
      navigationError = navigationError || String(error?.message || error)
    }

    const screenshot = `${screenshotRoot}/${item.key}.png`
    let screenshotError = ''
    try {
      await page.screenshot({ path: screenshot, fullPage: true, scale: 'css', type: 'png' })
    } catch (error) {
      screenshotError = String(error?.message || error)
    }

    results.push({
      key: item.key,
      requestedUrl: item.url,
      finalUrl: page.url(),
      title,
      body,
      consoleErrors: consoleErrors.length - errorStart,
      failedRequests: failedRequests.length - failedStart,
      navigationError,
      screenshotError
    })
  }

  await page.evaluate(authState)
  return results
}
