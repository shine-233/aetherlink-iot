/**
 * 文件用途：用于提供可视化路由就绪检查的 Playwright 用户可见证据。
 * 核心逻辑：通过共享 fixture 登录指定角色，访问本地或预览路由，并断言页面内容、权限边界或种子状态。
 * 关键注意事项：只有结合真实本地账号、稳定种子数据和可见状态断言时，才可计入对应业务信心。
 * 重构建议：若要提升证据强度，应补充 API 状态校验、负向分支和稳定选择器，避免只保留路由冒烟。
 */

const { test, expect } = require('./fixtures');
const { isApiResponse, isGetResponse } = require('./helpers/api_response_matcher');
const { skipWhenBlocked } = require('../lib/integration_blocked');

function uniqueName(prefix) {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 10000)}`;
}

function isNativeBoardResponse(response, method, boardId = '') {
  const url = new URL(response.url());
  const path = url.pathname;
  const isExpectedPath = boardId
    ? path.endsWith('/board/' + encodeURIComponent(boardId))
    : path.endsWith('/board');
  return response.request().method() === method && isExpectedPath;
}

function nativeInput(page, testId) {
  return page.getByTestId(testId).locator('input, textarea').first();
}

async function exchangeThingsVisToken(page) {
  return page.evaluate(async () => {
    const parseStoredValue = (raw) => {
      if (!raw) return null;
      try {
        const parsed = JSON.parse(raw);
        if (parsed && typeof parsed === 'object' && 'value' in parsed) return parsed.value;
        if (parsed && typeof parsed === 'object' && 'data' in parsed) return parsed.data;
        return parsed;
      } catch {
        return raw;
      }
    };

    const storageEntries = Object.keys(localStorage).map(key => ({
      key,
      value: parseStoredValue(localStorage.getItem(key))
    }));
    const tokenEntry = storageEntries.find(entry => /(^|[-_:])token$/i.test(entry.key) && typeof entry.value === 'string');
    const userInfoEntry = storageEntries.find(entry => {
      if (!/userinfo|user-info|user_info/i.test(entry.key)) return false;
      return entry.value && typeof entry.value === 'object';
    });

    const platformToken = tokenEntry && tokenEntry.value;
    const userInfo = userInfoEntry && userInfoEntry.value;
    if (!platformToken || !userInfo) {
      return { error: 'AetherLink login token or user info was not present in browser storage' };
    }

    const authority = userInfo.authority;
    const role = authority === 'SYS_ADMIN' ? 'SUPER_ADMIN' : authority === 'TENANT_ADMIN' ? 'TENANT_ADMIN' : 'EDITOR';
    const tenantId = userInfo.tenantId || userInfo.tenant_id || userInfo.spaceId || userInfo.userId || userInfo.id || '';
    const response = await fetch('/thingsvis-api/auth/sso', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        platform: 'aetherlink',
        platformToken,
        role,
        userInfo: {
          id: userInfo.userId || userInfo.id || '',
          email: userInfo.email || `${userInfo.userName || 'e2e'}@aetherlink.local`,
          name: userInfo.userName || userInfo.name || 'AetherLink E2E',
          tenantId
        }
      })
    });

    if (!response.ok) {
      return { error: await response.text(), status: response.status };
    }

    const data = await response.json();
    return { token: data.accessToken };
  });
}

async function thingsVisFetch(page, token, path, options = {}) {
  return page.evaluate(
    async ({ token: accessToken, path: requestPath, options: requestOptions }) => {
      const response = await fetch(`/thingsvis-api${requestPath}`, {
        method: requestOptions.method || 'GET',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${accessToken}`
        },
        body: requestOptions.body ? JSON.stringify(requestOptions.body) : undefined
      });
      const text = await response.text();
      let data = null;
      try {
        data = text ? JSON.parse(text) : null;
      } catch {
        data = text;
      }
      return { ok: response.ok, status: response.status, data };
    },
    { token, path, options }
  );
}

function flattenMenuNodes(nodes) {
  const flattened = [];
  const visit = rows => {
    for (const row of rows || []) {
      flattened.push(row);
      visit(row.children);
    }
  };
  visit(nodes);
  return flattened;
}

function thingsVisDashboardResponse(response, dashboardId) {
  return isGetResponse(response, '/thingsvis-api/dashboards/' + dashboardId);
}

function dashboardCard(page, dashboardId) {
  return page.locator(
    `[data-testid="thingsvis-dashboard-card"][data-dashboard-id="${dashboardId}"]`
  );
}

async function openDashboardMenuModal(page, dashboardId, dashboardName) {
  const card = dashboardCard(page, dashboardId);
  await expect(card).toContainText(dashboardName, { timeout: 20000 });
  await card.getByTestId('thingsvis-dashboard-menu').click();

  const modal = page.locator('.n-modal').filter({
    hasText: /System menu configuration|系统菜单配置/i
  }).last();
  await expect(modal).toBeVisible({ timeout: 15000 });
  await expect(modal.locator('input').first()).toHaveValue(dashboardName);
  return modal;
}

async function enableDashboardMenu(modal, menuName) {
  const enabledSwitch = modal.getByRole('switch');
  const currentState = await enabledSwitch.getAttribute('aria-checked');
  expect(currentState).toMatch(/^(true|false)$/);
  if (currentState !== 'true') {
    await enabledSwitch.click();
  }
  await expect(enabledSwitch).toHaveAttribute('aria-checked', 'true');
  await modal.getByPlaceholder(/Enter menu name|请输入菜单名称/i).fill(menuName);
}

async function ensureThingsVisDashboard(page) {
  await page.goto('/visualization/thingsvis', { waitUntil: 'domcontentloaded' });
  const tokenResult = await exchangeThingsVisToken(page);
  if (!tokenResult.token) {
    return {
      blocked: true,
      integrationBlocked: !tokenResult.status || tokenResult.status >= 500,
      reason: tokenResult.error || 'ThingsVis SSO token unavailable'
    };
  }

  const projectName = uniqueName('e2e-thingsvis-project');
  const projectResp = await thingsVisFetch(page, tokenResult.token, '/projects', {
    method: 'POST',
    body: { name: projectName, description: 'automation seeded project' }
  });
  if (!projectResp.ok || !projectResp.data || !projectResp.data.id) {
    return {
      blocked: true,
      integrationBlocked: projectResp.status >= 500,
      reason: `ThingsVis project create failed: ${projectResp.status}`
    };
  }

  const dashboardName = uniqueName('e2e-thingsvis-dashboard');
  const dashboardResp = await thingsVisFetch(page, tokenResult.token, '/dashboards', {
    method: 'POST',
    body: {
      name: dashboardName,
      projectId: projectResp.data.id,
      canvasConfig: { mode: 'fixed', width: 1920, height: 1080, background: '#ffffff' },
      nodes: [],
      dataSources: [],
      variables: []
    }
  });
  if (!dashboardResp.ok || !dashboardResp.data || !dashboardResp.data.id) {
    await thingsVisFetch(page, tokenResult.token, `/projects/${projectResp.data.id}`, { method: 'DELETE' });
    return {
      blocked: true,
      integrationBlocked: dashboardResp.status >= 500,
      reason: `ThingsVis dashboard create failed: ${dashboardResp.status}`
    };
  }

  return {
    blocked: false,
    token: tokenResult.token,
    project: projectResp.data,
    dashboard: dashboardResp.data,
    cleanup: async () => {
      const dashboardDelete = await thingsVisFetch(
        page,
        tokenResult.token,
        `/dashboards/${dashboardResp.data.id}`,
        { method: 'DELETE' }
      );
      expect(dashboardDelete.ok, 'delete seeded ThingsVis dashboard').toBe(true);
      const projectDelete = await thingsVisFetch(
        page,
        tokenResult.token,
        `/projects/${projectResp.data.id}`,
        { method: 'DELETE' }
      );
      expect(projectDelete.ok, 'delete seeded ThingsVis project').toBe(true);
    }
  };
}

function skipThingsVisIntegrationIfUnavailable(testInfo, seed, label) {
  if (seed.integrationBlocked) {
    skipWhenBlocked(testInfo, true, {
      reason: `${label}: ${seed.reason}`,
      category: 'runtime-external',
      seedable: false,
    });
  }
  expect(seed.blocked, seed.reason || `${label} fixture must be available`).toBe(false);
}

async function requireThingsVisCompatRoute(page, testInfo) {
  await page.goto('/visualization/thingsvis?provider=native', { waitUntil: 'domcontentloaded' });
  const nativeBoardsHeading = page.getByText('Native boards', { exact: true });
  await page.waitForFunction(
    () => Boolean(document.body?.innerText?.trim()) || ['/403', '/404'].includes(window.location.pathname),
    undefined,
    { timeout: 10000 }
  ).catch(() => {});
  const nativeBoardsVisible = await nativeBoardsHeading.isVisible({ timeout: 3000 }).catch(() => false);
  if (!nativeBoardsVisible) {
    // Dynamic auth mode can remove the optional compatibility entry before
    // the page component mounts. In that case the permission guard lands on
    // the explicit status route, so inspect the URL as the stable signal
    // instead of relying only on localized error-page text.
    const errorRoute = new URL(page.url()).pathname;
    const isBlockedRoute = ['/403', '/404'].includes(errorRoute);
    const notFound = page.getByText(/Page not found|ERROR 404|未找到页面/i).first();
    const backToHome = page.getByRole('link', { name: /Back to Home|返回首页/i }).first();
    if (
      isBlockedRoute ||
      (await notFound.isVisible({ timeout: 2000 }).catch(() => false)) ||
      (await backToHome.isVisible({ timeout: 2000 }).catch(() => false))
    ) {
      skipWhenBlocked(testInfo, true, {
        reason: 'ThingsVis compatibility routes are disabled in the current frontend build (VITE_ENABLE_THINGSVIS_COMPAT is not Y)',
        category: 'config',
        seedable: false,
      });
      return;
    }
  }
  await expect(nativeBoardsHeading).toBeVisible({ timeout: 20000 });
}

test.describe('ThingsVis visualization business routes [11_visualization]', () => {
  test.use({ role: 'tenant_admin' });

  test('native board CRUD is persisted by the local provider across list viewer and editor routes', async ({ rolePage, api }) => {
    const boardName = uniqueName('e2e-native-board');
    const updatedBoardName = boardName + '-updated';
    let boardId = '';

    try {
      await rolePage.goto('/visualization/native-boards', { waitUntil: 'domcontentloaded' });
      await expect(rolePage.getByTestId('native-board-create-button')).toBeVisible({ timeout: 20000 });

      await rolePage.getByTestId('native-board-create-button').click();
      await nativeInput(rolePage, 'native-board-name').fill(boardName);
      await nativeInput(rolePage, 'native-board-description').fill('native route-flow fixture');

      const createResponsePromise = rolePage.waitForResponse(
        response => isNativeBoardResponse(response, 'POST'),
        { timeout: 20000 }
      );
      const viewerRoutePromise = rolePage.waitForURL(/\/visualization\/native-board\?id=/, { timeout: 20000 });
      await rolePage.getByTestId('native-board-submit').click();
      const createResponse = await createResponsePromise;
      await viewerRoutePromise;
      expect(createResponse.status()).toBe(200);
      const createdBody = await createResponse.json();
      expect(createdBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({ name: boardName, vis_type: 'native' })
      }));

      boardId = new URL(rolePage.url()).searchParams.get('id') || '';
      expect(boardId).toEqual(expect.stringMatching(/\S/));
      await expect(rolePage.locator('.local-visualization-viewer')).toBeVisible({ timeout: 20000 });
      await expect(rolePage.getByText('Unable to load dashboard', { exact: true })).toHaveCount(0);

      const editorDetailPromise = rolePage.waitForResponse(
        response => isNativeBoardResponse(response, 'GET', boardId),
        { timeout: 20000 }
      );
      await rolePage.goto('/visualization/native-board-editor?id=' + encodeURIComponent(boardId), {
        waitUntil: 'domcontentloaded'
      });
      const editorDetail = await editorDetailPromise;
      expect(editorDetail.status()).toBe(200);
      await expect(nativeInput(rolePage, 'board-name')).toHaveValue(boardName, { timeout: 20000 });

      await nativeInput(rolePage, 'board-name').fill(updatedBoardName);
      await rolePage.getByTestId('add-widget').click();
      await expect(rolePage.getByTestId('widget-editor')).toHaveCount(1, { timeout: 10000 });

      const updateResponsePromise = rolePage.waitForResponse(
        response => isNativeBoardResponse(response, 'PUT'),
        { timeout: 20000 }
      );
      const savedViewerRoutePromise = rolePage.waitForURL(/\/visualization\/native-board\?id=/, { timeout: 20000 });
      await rolePage.getByTestId('save-board').click();
      const updateResponse = await updateResponsePromise;
      await savedViewerRoutePromise;
      expect(updateResponse.status()).toBe(200);
      const updatePayload = updateResponse.request().postDataJSON();
      expect(updatePayload).toEqual(expect.objectContaining({
        id: boardId,
        name: updatedBoardName,
        vis_type: 'native'
      }));
      const updatedBody = await updateResponse.json();
      expect(updatedBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({ id: boardId, name: updatedBoardName, vis_type: 'native' })
      }));

      const persisted = await api.get('/board/' + boardId, {}, 'tenant_admin');
      expect(persisted.code).toBe(200);
      expect(persisted.data).toEqual(expect.objectContaining({ id: boardId, name: updatedBoardName, vis_type: 'native' }));
      const persistedConfig = JSON.parse(persisted.data.config);
      expect(persistedConfig.widgets).toHaveLength(1);

      const listResponsePromise = rolePage.waitForResponse(
        response => isNativeBoardResponse(response, 'GET') && new URL(response.url()).searchParams.get('vis_type') === 'native',
        { timeout: 20000 }
      );
      await rolePage.goto('/visualization/native-boards', { waitUntil: 'domcontentloaded' });
      const listResponse = await listResponsePromise;
      expect(listResponse.status()).toBe(200);
      const card = rolePage.locator('[data-testid="native-board-item"]').filter({ hasText: updatedBoardName }).first();
      await expect(card).toBeVisible({ timeout: 20000 });
      await card.click();
      await expect(rolePage).toHaveURL(new RegExp('/visualization/native-board\\?id=' + boardId), { timeout: 20000 });
    } finally {
      if (boardId) {
        await api.delete('/board/' + boardId, {}, 'tenant_admin');
      }
    }
  });

  test('native board flows through the local provider on all ThingsVis compatibility routes', async ({ rolePage, api }, testInfo) => {
    const boardName = uniqueName('e2e-local-compat-board');
    const menuName = uniqueName('E2E Local Compat Menu');
    let boardId = '';
    let menuSaved = false;

    await requireThingsVisCompatRoute(rolePage, testInfo);

    const created = await api.post('/board', {
      name: boardName,
      config: JSON.stringify({ version: 1, columns: 24, rowHeight: 60, widgets: [] }),
      home_flag: 'N',
      menu_flag: 'N',
      vis_type: 'native'
    }, 'tenant_admin');
    expect(created.code).toBe(200);
    expect(created.data).toEqual(expect.objectContaining({ name: boardName, vis_type: 'native' }));
    boardId = created.data.id;

    try {
      const listResponsePromise = rolePage.waitForResponse(
        response => isNativeBoardResponse(response, 'GET') && new URL(response.url()).searchParams.get('vis_type') === 'native',
        { timeout: 20000 }
      );
      await rolePage.goto(
        '/visualization/thingsvis-dashboards?projectId=native-boards&provider=native',
        { waitUntil: 'domcontentloaded' }
      );
      const listResponse = await listResponsePromise;
      expect(listResponse.status()).toBe(200);
      const card = dashboardCard(rolePage, boardId);
      await expect(card).toContainText(boardName, { timeout: 20000 });
      await expect(card.locator('a')).toHaveAttribute(
        'href',
        `/tv-preview?id=${encodeURIComponent(boardId)}&projectId=native-boards&provider=native`
      );

      const editorDetailPromise = rolePage.waitForResponse(
        response => isNativeBoardResponse(response, 'GET', boardId),
        { timeout: 20000 }
      );
      await rolePage.goto(
        '/visualization/thingsvis-editor?id=' + encodeURIComponent(boardId) + '&projectId=native-boards&provider=native',
        { waitUntil: 'domcontentloaded' }
      );
      const editorDetail = await editorDetailPromise;
      expect(editorDetail.status()).toBe(200);
      await expect(rolePage.getByRole('status')).toContainText('Edit native dashboard', { timeout: 20000 });
      const nativeEditorRoutePromise = rolePage.waitForURL(
        /\/visualization\/native-board-editor\?id=/,
        { timeout: 20000 }
      );
      await rolePage.getByRole('button', { name: /^Edit$/ }).click();
      await nativeEditorRoutePromise;
      await expect(rolePage.getByTestId('add-widget')).toBeVisible({ timeout: 20000 });
      await expect(rolePage.locator('.local-visualization-viewer')).toBeVisible({ timeout: 20000 });

      const previewDetailPromise = rolePage.waitForResponse(
        response => isNativeBoardResponse(response, 'GET', boardId),
        { timeout: 20000 }
      );
      await rolePage.goto(
        '/visualization/thingsvis-preview?id=' + encodeURIComponent(boardId) + '&projectId=native-boards&provider=native',
        { waitUntil: 'domcontentloaded' }
      );
      const previewDetail = await previewDetailPromise;
      expect(previewDetail.status()).toBe(200);
      await expect(rolePage.locator('.local-visualization-viewer')).toBeVisible({ timeout: 20000 });

      await rolePage.goto(
        '/visualization/thingsvis-dashboards?projectId=native-boards&provider=native',
        { waitUntil: 'domcontentloaded' }
      );
      const modal = await openDashboardMenuModal(rolePage, boardId, boardName);
      await enableDashboardMenu(modal, menuName);
      const savePromise = rolePage.waitForResponse(
        response => isApiResponse(response, 'PUT', '/dashboard-menu/' + boardId),
        { timeout: 20000 }
      );
      await modal.getByRole('button', { name: /Save menu|淇濆瓨鑿滃崟/i }).click();
      const saveResponse = await savePromise;
      expect(saveResponse.status()).toBe(200);
      expect(await saveResponse.json()).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({ dashboard_id: boardId, dashboard_name: boardName, menu_name: menuName, enabled: true })
      }));
      menuSaved = true;

      const persistedMenu = await api.get('/dashboard-menu/' + boardId, {}, 'tenant_admin');
      expect(persistedMenu.code).toBe(200);
      expect(persistedMenu.data).toEqual(expect.objectContaining({
        dashboard_id: boardId,
        dashboard_name: boardName,
        menu_name: menuName,
        enabled: true
      }));

      const menuDetailPromise = rolePage.waitForResponse(
        response => isNativeBoardResponse(response, 'GET', boardId),
        { timeout: 20000 }
      );
      await rolePage.goto(
        '/visualization/thingsvis-menu-dashboard?id=' + encodeURIComponent(boardId) + '&projectId=native-boards&provider=native',
        { waitUntil: 'domcontentloaded' }
      );
      const menuDetail = await menuDetailPromise;
      expect(menuDetail.status()).toBe(200);
      await expect(rolePage.locator('.local-visualization-viewer')).toBeVisible({ timeout: 20000 });
    } finally {
      if (menuSaved) {
        const deletedMenu = await api.delete('/dashboard-menu/' + boardId, {}, 'tenant_admin');
        expect(deletedMenu.code).toBe(200);
      }
      if (boardId) {
        const deletedBoard = await api.delete('/board/' + boardId, {}, 'tenant_admin');
        expect(deletedBoard.code).toBe(200);
      }
    }
  });

  test('seeded ThingsVis project and dashboard render across project list editor preview and menu routes', async ({ rolePage }, testInfo) => {
    const seed = await ensureThingsVisDashboard(rolePage);
    skipThingsVisIntegrationIfUnavailable(testInfo, seed, 'ThingsVis project/dashboard service');

    try {
      const projectState = await thingsVisFetch(rolePage, seed.token, `/projects/${seed.project.id}`);
      expect(projectState.ok).toBe(true);
      expect(projectState.data).toEqual(expect.objectContaining({
        id: seed.project.id,
        name: seed.project.name
      }));
      const dashboardState = await thingsVisFetch(rolePage, seed.token, `/dashboards/${seed.dashboard.id}`);
      expect(dashboardState.ok).toBe(true);
      expect(dashboardState.data).toEqual(expect.objectContaining({
        id: seed.dashboard.id,
        name: seed.dashboard.name,
        projectId: seed.project.id
      }));

      await rolePage.goto('/visualization/thingsvis', { waitUntil: 'domcontentloaded' });
      const projectLink = rolePage.getByText(seed.project.name, { exact: true }).first();
      await expect(projectLink).toBeVisible({ timeout: 20000 });
      await Promise.all([
        rolePage.waitForURL(
          new RegExp('/visualization/thingsvis-dashboards\\?projectId=' + seed.project.id),
          { timeout: 20000 }
        ),
        projectLink.click()
      ]);
      await expect(rolePage.getByText(seed.project.name, { exact: true }).first()).toBeVisible({ timeout: 20000 });
      const card = dashboardCard(rolePage, seed.dashboard.id);
      await expect(card).toContainText(seed.dashboard.name, { timeout: 20000 });

      const [previewPage] = await Promise.all([
        rolePage.context().waitForEvent('page', { timeout: 20000 }),
        card.locator('a').click()
      ]);
      const previewResponsePromise = previewPage.waitForResponse(
        response => thingsVisDashboardResponse(response, seed.dashboard.id),
        { timeout: 20000 }
      );
      await previewPage.waitForLoadState('domcontentloaded');
      const previewResponse = await previewResponsePromise;
      expect(previewResponse.status()).toBe(200);
      expect(await previewResponse.json()).toEqual(expect.objectContaining({
        id: seed.dashboard.id,
        name: seed.dashboard.name,
        projectId: seed.project.id
      }));
      await expect(previewPage).toHaveURL(
        new RegExp('/tv-preview\\?id=' + seed.dashboard.id),
        { timeout: 20000 }
      );
      await expect(previewPage).toHaveTitle(
        new RegExp(seed.dashboard.name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')),
        { timeout: 20000 }
      );
      await previewPage.close();

      // The generated protected route and the standalone constant route point
      // to the same viewer component. Exercise both real route entries so the
      // coverage alias cannot hide a route-specific guard or layout regression.
      const generatedPreviewResponse = rolePage.waitForResponse(
        response => thingsVisDashboardResponse(response, seed.dashboard.id),
        { timeout: 20000 }
      );
      await rolePage.goto(
        '/visualization/thingsvis-preview?id=' + encodeURIComponent(seed.dashboard.id),
        { waitUntil: 'domcontentloaded' }
      );
      const generatedPreview = await generatedPreviewResponse;
      expect(generatedPreview.status()).toBe(200);
      expect(await generatedPreview.json()).toEqual(expect.objectContaining({
        id: seed.dashboard.id,
        name: seed.dashboard.name,
        projectId: seed.project.id
      }));
      await expect(rolePage.locator('iframe.thingsvis-frame')).toBeVisible({ timeout: 20000 });

      const editorDetailPromise = rolePage.waitForResponse(
        response => thingsVisDashboardResponse(response, seed.dashboard.id),
        { timeout: 20000 }
      );
      await card.getByTestId('thingsvis-dashboard-edit').click();
      await expect(rolePage).toHaveURL(
        new RegExp('/visualization/thingsvis-editor\\?id=' + seed.dashboard.id),
        { timeout: 20000 }
      );
      const editorDetail = await editorDetailPromise;
      expect(editorDetail.status()).toBe(200);
      expect(await editorDetail.json()).toEqual(expect.objectContaining({
        id: seed.dashboard.id,
        name: seed.dashboard.name,
        projectId: seed.project.id
      }));
      await expect(rolePage.getByText(seed.dashboard.name, { exact: true }).first()).toBeVisible({ timeout: 20000 });

      const menuDashboardPromise = rolePage.waitForResponse(
        response => thingsVisDashboardResponse(response, seed.dashboard.id),
        { timeout: 20000 }
      );
      await rolePage.goto('/visualization/thingsvis-menu-dashboard?id=' + encodeURIComponent(seed.dashboard.id), {
        waitUntil: 'domcontentloaded'
      });
      const menuDashboardResponse = await menuDashboardPromise;
      expect(menuDashboardResponse.status()).toBe(200);
      expect(await menuDashboardResponse.json()).toEqual(expect.objectContaining({
        id: seed.dashboard.id,
        name: seed.dashboard.name
      }));
      await expect(rolePage.getByText(seed.dashboard.name, { exact: true }).first()).toBeVisible({ timeout: 20000 });
    } finally {
      await seed.cleanup();
    }
  });

  // API 边界 + 浏览器闭环：验证 dashboard-menu API 拒绝无主仪表盘后，浏览器侧的可视化入口不崩溃。
  test('dashboard menu API rejects missing dashboard ownership and the visualization route stays stable', async ({ rolePage, api }, testInfo) => {
    await api.login('tenant_admin');
    const seed = await ensureThingsVisDashboard(rolePage);
    skipThingsVisIntegrationIfUnavailable(testInfo, seed, 'ThingsVis negative-menu service');
    const menuName = uniqueName('E2E Unmirrored Menu');

    try {
      await rolePage.goto(
        '/visualization/thingsvis-dashboards?projectId=' + encodeURIComponent(seed.project.id),
        { waitUntil: 'domcontentloaded' }
      );
      const modal = await openDashboardMenuModal(
        rolePage,
        seed.dashboard.id,
        seed.dashboard.name
      );
      await enableDashboardMenu(modal, menuName);

      const rejectedSavePromise = rolePage.waitForResponse(
        response => isApiResponse(response, 'PUT', '/dashboard-menu/' + seed.dashboard.id),
        { timeout: 20000 }
      );
      await modal.getByRole('button', { name: /Save menu|保存菜单/i }).click();
      const rejectedSave = await rejectedSavePromise;
      expect(rejectedSave.status()).toBe(200);
      const rejectedRequest = rejectedSave.request().postDataJSON();
      expect(rejectedRequest).toMatchObject({
        menu_name: menuName,
        dashboard_name: seed.dashboard.name,
        enabled: true
      });
      const rejectedBody = await rejectedSave.json();
      expect(rejectedBody.code).toBe(201001);
      expect(rejectedBody.message).toMatch(/dashboard not found|no permission/i);

      await expect(rolePage).toHaveURL(
        new RegExp('/visualization/thingsvis-dashboards\\?projectId=' + seed.project.id)
      );
      await expect(modal).toBeVisible();
      await expect(
        rolePage.getByText(/Failed to save menu configuration|菜单配置保存失败/i).last()
      ).toBeVisible({ timeout: 15000 });

      const fetchResp = await api.get('/dashboard-menu/' + seed.dashboard.id, {}, 'tenant_admin');
      expect(fetchResp.code).toBe(200);
      expect(fetchResp.data).toBeNull();
    } finally {
      await seed.cleanup();
    }
  });

  test('dashboard menu persists for a real ThingsVis dashboard when the mirror is available', async ({ rolePage, api }, testInfo) => {
    await api.login('tenant_admin');
    const dashboardId = String(process.env.THINGSVIS_MIRRORED_DASHBOARD_ID || '').trim();
    if (!dashboardId) {
      skipWhenBlocked(testInfo, true, {
        reason: 'THINGSVIS_MIRRORED_DASHBOARD_ID is not configured for the optional ThingsVis/local mirror integration',
        category: 'runtime-external',
        seedable: false,
      });
    }
    const menuName = uniqueName('E2E ThingsVis Menu');

    await rolePage.goto('/visualization/thingsvis', { waitUntil: 'domcontentloaded' });
    const tokenResult = await exchangeThingsVisToken(rolePage);
    if (!tokenResult.token && (!tokenResult.status || tokenResult.status >= 500)) {
      skipWhenBlocked(testInfo, true, {
        reason: `ThingsVis mirror SSO service unavailable: ${tokenResult.error || tokenResult.status}`,
        category: 'runtime-external',
        seedable: false,
      });
    }
    expect(tokenResult.token, tokenResult.error || 'ThingsVis SSO token must be available').toEqual(expect.any(String));
    const dashboardState = await thingsVisFetch(rolePage, tokenResult.token, '/dashboards/' + dashboardId);
    expect(dashboardState.ok, 'mirrored ThingsVis dashboard must be readable').toBe(true);
    expect(dashboardState.data).toEqual(expect.objectContaining({
      id: dashboardId,
      name: expect.stringMatching(/\S/),
      projectId: expect.stringMatching(/\S/)
    }));

    const originalMenuResponse = await api.get('/dashboard-menu/' + dashboardId, {}, 'tenant_admin');
    expect(originalMenuResponse.code).toBe(200);
    const originalMenu = originalMenuResponse.data;
    if (originalMenu !== null) {
      expect(originalMenu).toEqual(expect.objectContaining({
        dashboard_id: dashboardId,
        dashboard_name: expect.stringMatching(/\S/),
        menu_name: expect.stringMatching(/\S/),
        sort: expect.any(Number),
        enabled: expect.any(Boolean)
      }));
    }

    try {
      await rolePage.goto(
        '/visualization/thingsvis-dashboards?projectId=' + encodeURIComponent(dashboardState.data.projectId),
        { waitUntil: 'domcontentloaded' }
      );
      const modal = await openDashboardMenuModal(
        rolePage,
        dashboardId,
        dashboardState.data.name
      );
      await enableDashboardMenu(modal, menuName);

      const savePromise = rolePage.waitForResponse(
        response => isApiResponse(response, 'PUT', '/dashboard-menu/' + dashboardId),
        { timeout: 20000 }
      );
      await modal.getByRole('button', { name: /Save menu|保存菜单/i }).click();
      const saveResponse = await savePromise;
      expect(saveResponse.status()).toBe(200);
      expect(saveResponse.request().postDataJSON()).toMatchObject({
        menu_name: menuName,
        dashboard_name: dashboardState.data.name,
        enabled: true
      });
      const saveResp = await saveResponse.json();
      expect(saveResp.code).toBe(200);
      expect(saveResp.data.dashboard_id).toBe(dashboardId);
      expect(saveResp.data.dashboard_name).toBe(dashboardState.data.name);
      expect(saveResp.data.menu_name).toBe(menuName);
      expect(saveResp.data.enabled).toBe(true);

      const fetchResp = await api.get('/dashboard-menu/' + dashboardId, {}, 'tenant_admin');
      expect(fetchResp.code).toBe(200);
      expect(fetchResp.data.dashboard_id).toBe(dashboardId);
      expect(fetchResp.data.menu_name).toBe(menuName);
      expect(fetchResp.data.enabled).toBe(true);

      const menuResp = await api.get('/ui_elements/menu', {}, 'tenant_admin');
      expect(menuResp.code).toBe(200);
      expect(menuResp.data.list).toEqual(expect.any(Array));
      const dynamicMenu = flattenMenuNodes(menuResp.data.list).find(
        row => row.param1 === '/home/dashboard/' + dashboardId
      );
      expect(dynamicMenu).toEqual(expect.objectContaining({
        description: menuName,
        route_path: 'view.visualization_thingsvis-menu-dashboard'
      }));

      const viewerResponsePromise = rolePage.waitForResponse(
        response => thingsVisDashboardResponse(response, dashboardId),
        { timeout: 20000 }
      );
      await rolePage.getByText(menuName, { exact: true }).click();
      await expect(rolePage).toHaveURL(
        new RegExp('/home/dashboard/' + dashboardId + '$'),
        { timeout: 20000 }
      );
      const viewerResponse = await viewerResponsePromise;
      expect(viewerResponse.status()).toBe(200);
      expect(await viewerResponse.json()).toEqual(expect.objectContaining({
        id: dashboardId,
        name: dashboardState.data.name
      }));
      await expect(
        rolePage.getByText(dashboardState.data.name, { exact: true }).first()
      ).toBeVisible({ timeout: 20000 });
    } finally {
      if (originalMenu === null) {
        const deleteResp = await api.delete('/dashboard-menu/' + dashboardId, {}, 'tenant_admin');
        expect(deleteResp.code).toBe(200);
      } else {
        const restoreResp = await api.put('/dashboard-menu/' + dashboardId, {
          menu_name: originalMenu.menu_name,
          dashboard_name: originalMenu.dashboard_name,
          sort: originalMenu.sort,
          enabled: originalMenu.enabled
        }, 'tenant_admin');
        expect(restoreResp.code).toBe(200);
      }
      const removedResp = await api.get('/dashboard-menu/' + dashboardId, {}, 'tenant_admin');
      expect(removedResp.code).toBe(200);
      if (originalMenu === null) {
        expect(removedResp.data).toBeNull();
      } else {
        expect(removedResp.data).toEqual(expect.objectContaining({
          dashboard_id: originalMenu.dashboard_id,
          dashboard_name: originalMenu.dashboard_name,
          menu_name: originalMenu.menu_name,
          sort: originalMenu.sort,
          enabled: originalMenu.enabled
        }));
      }
    }
  });
});
