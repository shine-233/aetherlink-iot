/**
 * Cross-route closure evidence.
 *
 * This file only keeps route checks that also prove a real API-backed state is
 * visible in the browser. Pure route/403/exception smoke belongs to the route
 * contract and is intentionally not counted as business coverage here.
 */

const { test, expect } = require('./fixtures');
const seedData = require('../lib/seed_data');

function listFromResponse(response) {
  const data = response && response.data;
  if (Array.isArray(data)) return data;
  if (data && Array.isArray(data.list)) return data.list;
  return [];
}

function pickId(row) {
  return row && (row.id || row.ID);
}

function expectSuccess(response, label) {
  expect(response, label).toEqual(expect.objectContaining({ code: 200 }));
  return response.data;
}

function isGetResponse(response, pathname, query = {}) {
  const url = new URL(response.url());
  return response.request().method() === 'GET'
    && url.pathname.endsWith(pathname)
    && Object.entries(query).every(([key, value]) => url.searchParams.get(key) === String(value));
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

function menuPaths(response) {
  const data = expectSuccess(response, 'current user menu');
  expect(data).toEqual(expect.objectContaining({ list: expect.any(Array) }));
  return flattenMenuNodes(data.list).map(row => String(row.param1 || row.path || '').trim()).filter(Boolean);
}

async function expectMenuDeniedRoute(page, route) {
  await page.goto(route, { waitUntil: 'domcontentloaded' });
  await expect(page).toHaveURL(/\/403$/);
  await expect(page.getByRole('button', { name: /Logout|退出登录/i })).toBeVisible();
}

test.describe('route-backed business closure [14_route_coverage_closure]', () => {
  test.describe('tenant-admin stateful routes', () => {
    test.use({ role: 'tenant_admin' });

    test('tenant menu denies unregistered dashboard workspace routes', async ({ rolePage, api }) => {
      await api.login('tenant_admin');
      const paths = menuPaths(await api.get('/ui_elements/menu', {}, 'tenant_admin'));
      expect(paths).not.toContain('/dashboard/workspace');
      expect(paths).not.toContain('/dashboard/workbench');

      await expectMenuDeniedRoute(rolePage, '/dashboard/workspace');
      await expectMenuDeniedRoute(rolePage, '/dashboard/workbench');
    });

    test('seeded child-device detail route renders the selected device API state', async ({ rolePage, api }) => {
      await api.login('tenant_admin');
      const seed = await seedData.ensureDevice('tenant_admin');

      try {
        const detailResp = await api.get('/device/detail/' + seed.id, {}, 'tenant_admin');
        const detail = expectSuccess(detailResp, 'child-device detail');
        expect(detail).toEqual(expect.objectContaining({
          id: seed.id,
          name: expect.any(String),
          device_number: expect.any(String)
        }));
        expect(detail.name.trim()).not.toBe('');
        expect(detail.device_number.trim()).not.toBe('');

        await rolePage.goto('/device/details-child?d_id=' + encodeURIComponent(seed.id), {
          waitUntil: 'domcontentloaded'
        });
        await expect(rolePage).toHaveURL(/\/device\/details-child\?d_id=/);
        await expect(rolePage.getByText(detail.name, { exact: true }).first()).toBeVisible({ timeout: 15000 });
        await expect(rolePage.getByText(detail.device_number, { exact: true }).first()).toBeVisible();

        const editButton = rolePage.getByRole('button', { name: /Edit|编辑/i }).first();
        await editButton.click();
        const editModal = rolePage.locator('.n-modal').last();
        await expect(editModal).toBeVisible();
        const nameInput = editModal.locator('input').first();
        await expect(nameInput).toHaveValue(detail.name);
        const unsavedName = detail.name + '-unsaved-e2e';
        await nameInput.fill(unsavedName);

        const cancelRefreshPromise = rolePage.waitForResponse(response =>
          isGetResponse(response, '/device/detail/' + seed.id)
        );
        await editModal.getByRole('button', { name: /Cancel|取消/i }).click();
        const cancelRefresh = await cancelRefreshPromise;
        expect(cancelRefresh.status()).toBe(200);
        const cancelRefreshBody = await cancelRefresh.json();
        expect(cancelRefreshBody).toEqual(expect.objectContaining({
          code: 200,
          data: expect.objectContaining({ id: seed.id, name: detail.name })
        }));
        await expect(rolePage.getByText(unsavedName, { exact: true })).toHaveCount(0);
        await expect(rolePage.getByText(detail.name, { exact: true }).first()).toBeVisible();
      } finally {
        await seed.cleanup();
      }
    });

    test('personal-center renders the authenticated profile returned by the API', async ({ rolePage, api }) => {
      await api.login('tenant_admin');
      const profileResp = await api.get('/board/user/info', {}, 'tenant_admin');
      const profile = expectSuccess(profileResp, 'personal profile');
      const profileName = String(profile?.name || profile?.user_name || profile?.userName || '').trim();
      const profileEmail = String(profile?.email || profile?.user_email || '').trim();
      expect(profileName).not.toBe('');
      expect(profileEmail).not.toBe('');

      await rolePage.goto('/personal-center', { waitUntil: 'domcontentloaded' });
      await expect(rolePage).toHaveURL(/\/personal-center$/);
      await expect(rolePage.getByText(profileName, { exact: true }).first()).toBeVisible({ timeout: 15000 });
      await expect(rolePage.getByText(profileEmail, { exact: true }).first()).toBeVisible({ timeout: 15000 });
      const editButton = rolePage.getByTitle(/Edit|编辑/i).first();
      await editButton.click();
      const profileForm = rolePage.locator('.n-form').filter({ hasText: /Nickname|昵称/i }).first();
      const nameInput = profileForm.locator('.n-form-item').filter({ hasText: /Nickname|昵称/i }).locator('input').first();
      const emailInput = profileForm.locator('.n-form-item').filter({ hasText: /Email Address|Email|邮箱/i }).locator('input').first();
      await expect(nameInput).toBeVisible();
      await expect(emailInput).toBeDisabled();

      const profileEditor = profileForm.locator('xpath=..');
      const updatedProfileName = `${profileName.slice(0, 70)}-e2e-${Date.now()}`;
      let restorePayload = null;
      try {
        const updateRequestPromise = rolePage.waitForRequest(request => {
          const url = new URL(request.url());
          return request.method() === 'POST' && url.pathname.endsWith('/board/user/update');
        }, { timeout: 20000 });
        const updateResponsePromise = rolePage.waitForResponse(response => {
          const url = new URL(response.url());
          return response.request().method() === 'POST' && url.pathname.endsWith('/board/user/update');
        }, { timeout: 20000 });

        await nameInput.fill(updatedProfileName);
        await profileEditor.getByRole('button', { name: /Confirm|Save|确认|保存/i }).first().click();

        const updateRequest = await updateRequestPromise;
        const updatePayload = updateRequest.postDataJSON();
        expect(updatePayload).toEqual(expect.objectContaining({
          name: updatedProfileName,
          email: profileEmail
        }));
        restorePayload = { ...updatePayload, name: profileName };

        const updateResponse = await updateResponsePromise;
        expect(updateResponse.status()).toBe(200);
        expect(await updateResponse.json()).toEqual(expect.objectContaining({ code: 200 }));

        const updatedProfileResp = await api.get('/board/user/info', {}, 'tenant_admin');
        const updatedProfile = expectSuccess(updatedProfileResp, 'updated personal profile');
        expect(String(updatedProfile?.name || '').trim()).toBe(updatedProfileName);
        await expect(rolePage.getByText(updatedProfileName, { exact: true }).first()).toBeVisible({ timeout: 15000 });
        await expect(nameInput).toBeHidden();
      } finally {
        if (restorePayload) {
          const restoreResp = await api.post('/board/user/update', restorePayload, 'tenant_admin');
          expectSuccess(restoreResp, 'restore personal profile');
          const restoredProfileResp = await api.get('/board/user/info', {}, 'tenant_admin');
          const restoredProfile = expectSuccess(restoredProfileResp, 'restored personal profile');
          expect(String(restoredProfile?.name || '').trim()).toBe(profileName);
        }
      }

      await rolePage.reload({ waitUntil: 'domcontentloaded' });
      await expect(rolePage).toHaveURL(/\/personal-center$/);
      await expect(rolePage.getByText(profileName, { exact: true }).first()).toBeVisible();
      await expect(rolePage.getByText(profileEmail, { exact: true }).first()).toBeVisible();
    });

    test('seeded OpenAPI key appears in management/api and remains tenant-scoped', async ({ rolePage, api }) => {
      await api.login('tenant_admin');
      const tenantIdResp = await api.get('/user/detail', {}, 'tenant_admin');
      const tenantId = tenantIdResp.data && (tenantIdResp.data.tenant_id || tenantIdResp.data.tenantId);
      expect(String(tenantId || '')).not.toBe('');
      const seed = await seedData.ensureOpenApiKey('tenant_admin', tenantId);
      expect(seed.blocked, seed.reason || 'OpenAPI key endpoint unavailable').toBe(false);
      const seedName = String(seed.row?.name || '').trim();
      expect(seedName).not.toBe('');

      try {
        const listResp = await api.get('/open/keys', { page: 1, page_size: 100 }, 'tenant_admin');
        const rows = listFromResponse(listResp);
        const apiRow = rows.find(row => pickId(row) === seed.id || row.name === seedName);
        expect(apiRow).toEqual(expect.objectContaining({ name: seedName }));
        expect(String(apiRow.tenant_id || apiRow.tenantId)).toBe(String(tenantId));

        await rolePage.goto('/management/api', { waitUntil: 'domcontentloaded' });
        await expect(rolePage).toHaveURL(/\/management\/api$/);
        await expect(rolePage.getByText(apiRow.name, { exact: true }).first()).toBeVisible({ timeout: 15000 });
        await expect(rolePage.getByText(/Create API Key|创建 API Key|API 密钥/i).first()).toBeVisible();

        const apiKeyRow = rolePage.locator('.n-data-table-base-table-body .n-data-table-tr').filter({ hasText: apiRow.name });
        await expect(apiKeyRow).toHaveCount(1);
        await apiKeyRow.getByRole('button', { name: /Edit|编辑/i }).click();
        const editModal = rolePage.locator('.n-modal').last();
        await expect(editModal).toBeVisible();
        await expect(editModal.locator('input').first()).toHaveValue(apiRow.name);
        await editModal.getByRole('button', { name: /Cancel|取消/i }).click();
        await expect(editModal).toBeHidden();
        await expect(apiKeyRow).toContainText(apiRow.name);
      } finally {
        await seed.cleanup();
      }
    });

    test('system log path filter sends an exact API query and renders the empty result', async ({ rolePage, api }) => {
      await api.login('tenant_admin');
      const path = '/e2e/no-operation-log/' + Date.now();
      const apiResp = await api.get('/operation_logs', { page: 1, page_size: 10, path }, 'tenant_admin');
      expectSuccess(apiResp, 'filtered operation logs');
      expect(listFromResponse(apiResp)).toEqual([]);

      await rolePage.goto('/system-management-user/system-log', { waitUntil: 'domcontentloaded' });
      await expect(rolePage).toHaveURL(/\/system-management-user\/system-log$/);
      const pathInput = rolePage.locator('.n-form-item').filter({ hasText: /Request Path|请求路径/i }).locator('input').first();
      await pathInput.fill(path);
      const browserResponse = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return url.pathname.endsWith('/operation_logs') && url.searchParams.get('path') === path;
      });
      await rolePage.getByRole('button', { name: /Search|查询/i }).click();
      const response = await browserResponse;
      expect(response.status()).toBe(200);
      const body = await response.json();
      expect(body).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({
          total: 0,
          list: []
        })
      }));
      await expect(rolePage.locator('.n-data-table-empty')).toBeVisible({ timeout: 15000 });
    });
  });

  test.describe('super-admin read routes', () => {
    test.use({ role: 'super_admin' });

    test('super-admin menu denies routes without backend menu registration', async ({ rolePage, api }) => {
      await api.login('super_admin');
      const paths = menuPaths(await api.get('/ui_elements/menu', {}, 'super_admin'));
      expect(paths).not.toContain('/product/update-package');
      expect(paths).not.toContain('/system-management-user/equipment-map');

      await expectMenuDeniedRoute(rolePage, '/product/update-package');
      await expectMenuDeniedRoute(rolePage, '/system-management-user/equipment-map');
    });

    test('management/auth renders a menu element that is present in the API payload', async ({ rolePage, api }) => {
      await api.login('super_admin');
      const listResp = await api.get('/ui_elements', { page: 1, page_size: 10 }, 'super_admin');
      const rows = listFromResponse(listResp);
      expect(rows.length).toBeGreaterThan(0);
      const expected = rows.find(row =>
        String(row.element_code || '').trim() && String(row.param1 || '').trim()
      );
      expect(expected).toEqual(expect.objectContaining({
        element_code: expect.any(String),
        param1: expect.any(String)
      }));

      const browserListPromise = rolePage.waitForResponse(
        response => isGetResponse(response, '/ui_elements', { page: 1, page_size: 10 }),
        { timeout: 20000 }
      );
      await rolePage.goto('/management/auth', { waitUntil: 'domcontentloaded' });
      await expect(rolePage).toHaveURL(/\/management\/auth$/);
      const browserResponse = await browserListPromise;
      expect(browserResponse.status()).toBe(200);
      const browserRows = listFromResponse(await browserResponse.json());
      const browserRow = browserRows.find(row => String(pickId(row)) === String(pickId(expected)));
      expect(browserRow).toEqual(expect.objectContaining({
        element_code: expected.element_code,
        param1: expected.param1
      }));

      const renderedRow = rolePage.locator('.n-data-table-base-table-body .n-data-table-tr').filter({ hasText: expected.element_code });
      await expect(renderedRow).toHaveCount(1, { timeout: 15000 });
      await expect(renderedRow).toContainText(expected.element_code);
      await expect(renderedRow).toContainText(expected.param1);

      await renderedRow.getByRole('button', { name: /Edit|编辑/i }).click();
      const editModal = rolePage.locator('.n-modal').last();
      await expect(editModal).toBeVisible();
      const elementCodeInput = editModal.locator('.n-form-item').filter({ hasText: /Name|名称/i }).locator('input').first();
      const accessPathInput = editModal.locator('.n-form-item').filter({ hasText: /Access Path|访问路径/i }).locator('input').first();
      await expect(elementCodeInput).toHaveValue(expected.element_code);
      await expect(accessPathInput).toHaveValue(expected.param1);
      await editModal.getByRole('button', { name: /Cancel|取消/i }).click();
      await expect(editModal).toBeHidden();
      await expect(renderedRow).toContainText(expected.element_code);
      await expect(renderedRow).toContainText(expected.param1);
    });

  });
});
