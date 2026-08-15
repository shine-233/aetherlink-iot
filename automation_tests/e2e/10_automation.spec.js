/**
 * 文件用途：用于提供自动化页面 E2E 检查的 Playwright 用户可见证据。
 * 核心逻辑：通过共享 fixture 登录指定角色，访问本地或预览路由，并断言页面内容、权限边界或种子状态。
 * 关键注意事项：只有结合真实本地账号、稳定种子数据和可见状态断言时，才可计入对应业务信心。
 * 重构建议：若要提升证据强度，应补充 API 状态校验、负向分支和稳定选择器，避免只保留路由冒烟。
 */

const { test, expect } = require('./fixtures');
const { isApiResponse, isGetResponse } = require('./helpers/api_response_matcher');
const seedData = require('../lib/seed_data');

function expectSceneDetailMatchesSeed(detail, scene) {
  expect(detail).toEqual(
    expect.objectContaining({
      info: expect.any(Object),
      actions: expect.any(Array)
    })
  );
  expect(String(detail.info.id || detail.info.ID)).toBe(String(scene.id));
  expect(detail.info.name || detail.info.Name).toBe(scene.name);
  expect(detail.actions.length).toBeGreaterThan(0);
}

function listRows(payload) {
  if (!payload) return [];
  if (Array.isArray(payload)) return payload;
  if (Array.isArray(payload.list)) return payload.list;
  if (Array.isArray(payload.data)) return payload.data;
  return [];
}

function pickId(row) {
  return row && (row.id || row.ID);
}

function expectApiSuccess(response, label) {
  expect(response, label).toEqual(expect.objectContaining({ code: 200 }));
  expect(response.data, label + ' data').not.toBeNull();
  expect(response.data, label + ' data').not.toBeUndefined();
  return response.data;
}

function flattenMenuPaths(nodes) {
  const paths = [];
  const visit = rows => {
    for (const row of rows || []) {
      const route = String(row.param1 || row.path || '').trim();
      if (route) paths.push(route);
      visit(row.children);
    }
  };
  visit(nodes);
  return paths;
}

test.describe('automation module', () => {
  test.describe.configure({ timeout: 60000 });
  test.use({ role: 'tenant_admin' });

  test('scene manage search submits a seeded scene name', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const scene = await seedData.ensureScene('tenant_admin');

    try {
      expect(scene.blocked, scene.reason || 'scene seed must be available').toBe(false);
      expect(['string', 'number']).toContain(typeof scene.id);
      expect(String(scene.id).trim()).not.toBe('');

      await rolePage.goto('/automation/scene-manage', { waitUntil: 'domcontentloaded' });
      const searchInput = rolePage.locator('input[placeholder*="Scene Name" i]').first();
      await expect(searchInput).toBeVisible({ timeout: 15000 });

      const filteredResponse = rolePage.waitForResponse(
        response => isGetResponse(response, '/scene', { name: scene.name }),
        { timeout: 20000 }
      );
      await searchInput.fill(scene.name);
      // "Clear search" also contains the word Search.  Anchor the accessible
      // name so the business action cannot become ambiguous as the form grows.
      await rolePage.getByRole('button', { name: /^(Search|\u641c\u7d22|\u67e5\u8be2)$/i }).click();

      const browserResponse = await filteredResponse;
      expect(browserResponse.status()).toBe(200);
      const browserBody = await browserResponse.json();
      const browserRows = listRows(expectApiSuccess(browserBody, 'browser filtered scene list'));
      const browserRow = browserRows.find(row => String(pickId(row)) === String(scene.id));
      expect(browserRow).toEqual(expect.objectContaining({ name: scene.name }));
      await expect(rolePage.getByText(scene.name).first()).toBeVisible({ timeout: 15000 });

      const detail = expectApiSuccess(
        await api.get('/scene/detail/' + scene.id, {}, 'tenant_admin'),
        'seeded scene detail'
      );
      expectSceneDetailMatchesSeed(detail, scene);

      const listData = expectApiSuccess(
        await api.get('/scene', { page: 1, page_size: 20, name: scene.name }, 'tenant_admin'),
        'seeded scene list'
      );
      const matchedRow = listRows(listData).find(row => String(pickId(row)) === String(scene.id));
      expect(matchedRow).toEqual(expect.any(Object));
      expect(matchedRow.name || matchedRow.Name).toBe(scene.name);
    } finally {
      await scene.cleanup();
    }
  });

  test('scene edit echoes a seeded scene and matches the detail API', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const scene = await seedData.ensureScene('tenant_admin');
    const previewDescription = 'E2E scene preview ' + Date.now();

    try {
      expect(scene.blocked, scene.reason || 'scene seed must be available').toBe(false);
      const directDetail = expectApiSuccess(
        await api.get('/scene/detail/' + scene.id, {}, 'tenant_admin'),
        'direct seeded scene detail'
      );
      expectSceneDetailMatchesSeed(directDetail, scene);
      expect(directDetail.info.description).toBe('automation seed');

      const browserDetailResponse = rolePage.waitForResponse(
        response => isGetResponse(response, '/scene/detail/' + scene.id),
        { timeout: 20000 }
      );
      await rolePage.goto('/automation/scene-edit?id=' + scene.id, { waitUntil: 'domcontentloaded' });
      await expect(rolePage).toHaveURL(new RegExp('/automation/scene-edit\\?id=' + scene.id));

      const browserResponse = await browserDetailResponse;
      expect(browserResponse.status()).toBe(200);
      const browserDetail = expectApiSuccess(await browserResponse.json(), 'browser seeded scene detail');
      expectSceneDetailMatchesSeed(browserDetail, scene);
      expect(browserDetail.actions).toEqual(directDetail.actions);

      const nameInput = rolePage.locator(
        'input[placeholder*="Scene Name" i], input[placeholder*="场景名称"]'
      ).first();
      await expect(nameInput).toHaveValue(scene.name, { timeout: 15000 });

      const descriptionInput = rolePage.locator(
        'textarea[placeholder*="description" i], textarea[placeholder*="描述"]'
      ).first();
      await expect(descriptionInput).toBeVisible({ timeout: 15000 });
      await descriptionInput.fill(previewDescription);

      const dryRunResponsePromise = rolePage.waitForResponse(
        response => isApiResponse(response, 'POST', '/scene/dry-run'),
        { timeout: 20000 }
      );
      await rolePage.getByTestId('automation-dry-run-request-backend').click();
      const dryRunResponse = await dryRunResponsePromise;
      expect(dryRunResponse.status()).toBe(200);
      const dryRunRequest = dryRunResponse.request().postDataJSON();
      expect(dryRunRequest).toMatchObject({
        id: scene.id,
        name: scene.name,
        description: previewDescription
      });
      expect(dryRunRequest.actions).toEqual(expect.any(Array));
      expect(dryRunRequest.actions.length).toBeGreaterThan(0);
      const dryRunData = expectApiSuccess(await dryRunResponse.json(), 'browser scene dry-run');
      expect(dryRunData.can_save).toBe(true);

      const detailAfterPreview = expectApiSuccess(
        await api.get('/scene/detail/' + scene.id, {}, 'tenant_admin'),
        'scene detail after non-persisting dry-run'
      );
      expectSceneDetailMatchesSeed(detailAfterPreview, scene);
      expect(detailAfterPreview.info.description).toBe(directDetail.info.description);
    } finally {
      await scene.cleanup();
    }
  });

  test('scene linkage search finds a seeded automation and matches its detail API', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const automation = await seedData.ensureSceneAutomation('tenant_admin');

    try {
      expect(automation.blocked, automation.reason || 'scene automation seed must be available').toBe(false);
      const directDetail = expectApiSuccess(
        await api.get('/scene_automations/detail/' + automation.id, {}, 'tenant_admin'),
        'seeded scene automation detail'
      );
      expect(directDetail).toMatchObject({
        id: automation.id,
        name: automation.name,
        enabled: 'N'
      });
      expect(directDetail.trigger_condition_groups).toEqual(expect.any(Array));
      expect(directDetail.trigger_condition_groups.length).toBeGreaterThan(0);
      expect(directDetail.actions).toEqual(expect.any(Array));
      expect(directDetail.actions.length).toBeGreaterThan(0);

      await rolePage.goto('/automation/scene-linkage', { waitUntil: 'domcontentloaded' });
      const searchInput = rolePage.locator(
        'input[placeholder*="Scene Linkage Name" i], input[placeholder*="场景联动名称"]'
      ).first();
      await expect(searchInput).toBeVisible({ timeout: 15000 });

      const filteredResponse = rolePage.waitForResponse(
        response => isGetResponse(response, '/scene_automations/list', { name: automation.name }),
        { timeout: 20000 }
      );
      await searchInput.fill(automation.name);
      await rolePage.getByRole('button', { name: /Search|\u641c\u7d22|\u67e5\u8be2/i }).click();

      const browserResponse = await filteredResponse;
      expect(browserResponse.status()).toBe(200);
      const browserBody = await browserResponse.json();
      const browserRows = listRows(expectApiSuccess(browserBody, 'browser filtered scene automation list'));
      const browserRow = browserRows.find(row => String(pickId(row)) === String(automation.id));
      expect(browserRow).toEqual(
        expect.objectContaining({
          name: directDetail.name,
          enabled: directDetail.enabled
        })
      );
      await expect(rolePage.getByText(automation.name).first()).toBeVisible({ timeout: 15000 });
    } finally {
      await automation.cleanup();
    }
  });

  test('automation editor pre-validates, saves, and keeps the updated rule after refresh', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const automation = await seedData.ensureSceneAutomation('tenant_admin');
    const updatedName = automation.name ? automation.name.slice(0, 30) + '_ui' : '';

    try {
      expect(automation.blocked, automation.reason || 'scene automation seed must be available').toBe(false);
      expect(updatedName).not.toBe('');

      const editableSeedPayload = {
        ...automation.payload,
        id: automation.id,
        trigger_condition_groups: [[{
          trigger_conditions_type: '10',
          trigger_source: automation.deviceId,
          trigger_param_type: 'status',
          trigger_param: 'On-line',
          trigger_operator: '=',
          trigger_value: 'online'
        }]]
      };
      const editableSeed = expectApiSuccess(
        await api.put('/scene_automations', editableSeedPayload, 'tenant_admin'),
        'frontend-editable automation seed'
      );
      expect(String(editableSeed.scene_automation_id)).toBe(String(automation.id));

      const menuData = expectApiSuccess(
        await api.get('/ui_elements/menu', {}, 'tenant_admin'),
        'tenant-admin menu'
      );
      expect(menuData.list).toEqual(expect.any(Array));
      expect(flattenMenuPaths(menuData.list)).toContain('/automation/scene-linkage');

      const detailLoadPromise = rolePage.waitForResponse(
        response => isGetResponse(response, '/scene_automations/detail/' + automation.id),
        { timeout: 20000 }
      );
      await rolePage.goto('/automation/linkage-edit?id=' + automation.id, {
        waitUntil: 'domcontentloaded'
      });
      await expect(rolePage).toHaveURL(new RegExp('/automation/linkage-edit\\?id=' + automation.id));

      const loadedDetailResponse = await detailLoadPromise;
      expect(loadedDetailResponse.status()).toBe(200);
      const loadedDetail = expectApiSuccess(
        await loadedDetailResponse.json(),
        'browser automation detail'
      );
      expect(loadedDetail).toMatchObject({
        id: automation.id,
        name: automation.name,
        enabled: 'N'
      });

      const nameInput = rolePage.locator('input[placeholder*="Scene Linkage Name" i]').first();
      await expect(nameInput).toHaveValue(automation.name, { timeout: 15000 });
      await nameInput.fill(updatedName);
      await rolePage.getByTestId('automation-dry-run-refresh-local').click();
      await expect(rolePage.getByTestId('automation-dry-run-condition-count')).toContainText(/1/);
      await expect(rolePage.getByTestId('automation-dry-run-action-count')).toContainText(/1/);

      const previewResponsePromise = rolePage.waitForResponse(
        response => isApiResponse(response, 'POST', '/scene_automations/dry-run'),
        { timeout: 20000 }
      );
      await rolePage.getByTestId('automation-dry-run-request-backend').click();
      const previewResponse = await previewResponsePromise;
      expect(previewResponse.status()).toBe(200);
      const previewRequest = previewResponse.request().postDataJSON();
      expect(previewRequest).toMatchObject({ id: automation.id, name: updatedName });
      expect(previewRequest.trigger_condition_groups.length).toBeGreaterThan(0);
      expect(previewRequest.actions.length).toBeGreaterThan(0);
      const previewData = expectApiSuccess(await previewResponse.json(), 'browser automation dry-run');
      expect(previewData.can_save).toBe(true);

      const saveDryRunPromise = rolePage.waitForResponse(
        response => isApiResponse(response, 'POST', '/scene_automations/dry-run'),
        { timeout: 20000 }
      );
      await rolePage.getByRole('button', {
        name: /Save Scene Linkage|\u4fdd\u5b58\u573a\u666f\u8054\u52a8/i
      }).click();
      const saveDryRunResponse = await saveDryRunPromise;
      expect(saveDryRunResponse.status()).toBe(200);
      expectApiSuccess(await saveDryRunResponse.json(), 'save-gate automation dry-run');

      const confirmation = rolePage.locator('.n-dialog').filter({
        hasText: /confirm whether to save|\u786e\u8ba4\u662f\u5426\u4fdd\u5b58/i
      }).last();
      await expect(confirmation).toBeVisible({ timeout: 15000 });

      const updateResponsePromise = rolePage.waitForResponse(
        response => isApiResponse(response, 'PUT', '/scene_automations'),
        { timeout: 20000 }
      );
      await confirmation.getByRole('button', { name: /Confirm|\u786e\u5b9a|\u786e\u8ba4/i }).click();
      const updateResponse = await updateResponsePromise;
      expect(updateResponse.status()).toBe(200);
      const updateRequest = updateResponse.request().postDataJSON();
      expect(updateRequest).toMatchObject({ id: automation.id, name: updatedName });
      expect(updateRequest.trigger_condition_groups.length).toBeGreaterThan(0);
      expect(updateRequest.actions.length).toBeGreaterThan(0);
      const updateData = expectApiSuccess(await updateResponse.json(), 'browser automation update');
      expect(String(updateData.scene_automation_id)).toBe(String(automation.id));

      await expect(rolePage).toHaveURL(/\/automation\/scene-linkage$/, { timeout: 15000 });

      const persistedDetail = expectApiSuccess(
        await api.get('/scene_automations/detail/' + automation.id, {}, 'tenant_admin'),
        'updated automation detail'
      );
      expect(persistedDetail).toMatchObject({ id: automation.id, name: updatedName, enabled: 'N' });

      const persistedList = expectApiSuccess(
        await api.get('/scene_automations/list', { page: 1, page_size: 20, name: updatedName }, 'tenant_admin'),
        'updated automation list'
      );
      expect(listRows(persistedList)).toEqual(
        expect.arrayContaining([expect.objectContaining({ id: automation.id, name: updatedName })])
      );

      await rolePage.reload({ waitUntil: 'domcontentloaded' });
      const searchInput = rolePage.locator(
        'input[placeholder*="Scene Linkage Name" i], input[placeholder*="\u573a\u666f\u8054\u52a8\u540d\u79f0"]'
      ).first();
      await expect(searchInput).toBeVisible({ timeout: 15000 });
      const refreshedSearchPromise = rolePage.waitForResponse(
        response => isGetResponse(response, '/scene_automations/list', { name: updatedName }),
        { timeout: 20000 }
      );
      await searchInput.fill(updatedName);
      await rolePage.getByRole('button', { name: /Search|\u641c\u7d22|\u67e5\u8be2/i }).click();
      const refreshedSearch = await refreshedSearchPromise;
      expect(refreshedSearch.status()).toBe(200);
      const refreshedRows = listRows(
        expectApiSuccess(await refreshedSearch.json(), 'refreshed automation search')
      );
      expect(refreshedRows).toEqual(
        expect.arrayContaining([expect.objectContaining({ id: automation.id, name: updatedName })])
      );
      await expect(rolePage.getByText(updatedName, { exact: true }).first()).toBeVisible({ timeout: 15000 });
    } finally {
      await automation.cleanup();
    }
  });
});
