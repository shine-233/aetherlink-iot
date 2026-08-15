/**
 * SYS_ADMIN native-board tenant-context regression.
 *
 * The backend deliberately requires an explicit tenant for SYS_ADMIN board
 * writes. The list-page tenant filter is the user's selected context and the
 * create modal must carry that same context into the POST payload.
 */

const { test, expect } = require('./fixtures');

function nativeInput(page, testId) {
  return page.getByTestId(testId).locator('input, textarea').first();
}

function isBoardCreateResponse(response) {
  return response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith('/board');
}

test.describe('SYS_ADMIN native board tenant context [23_native_board_super_admin]', () => {
  test.use({ role: 'super_admin' });

  test('uses the selected tenant filter as the create context', async ({ rolePage, api }, testInfo) => {
    await api.login('super_admin');
    const usersResp = await api.get('/user', { page: 1, page_size: 1000 }, 'super_admin');
    expect(usersResp.code).toBe(200);
    const tenant = (usersResp.data?.list || []).find(row => row.authority === 'TENANT_ADMIN' && row.tenant_id);
    expect(tenant, 'SYS_ADMIN needs a selectable tenant context').toBeTruthy();
    const tenantId = String(tenant.tenant_id).trim();
    const boardName = `e2e-super-admin-native-${Date.now()}`;
    let boardId = '';

    try {
      await rolePage.goto('/visualization/native-boards', { waitUntil: 'domcontentloaded' });
      const tenantFilter = rolePage.getByTestId('native-board-tenant-filter');
      await expect(tenantFilter).toBeVisible({ timeout: 20000 });
      await tenantFilter.click();
      const option = rolePage.locator('.n-base-select-menu').getByText(tenantId, { exact: false }).first();
      await expect(option).toBeVisible({ timeout: 10000 });
      await option.click();

      await expect(tenantFilter).toContainText(tenantId, { timeout: 10000 });
      await rolePage.getByTestId('native-board-create-button').click();
      const modalTenant = rolePage.getByTestId('native-board-tenant-select');
      await expect(modalTenant).toContainText(tenantId, { timeout: 10000 });
      await rolePage.screenshot({ path: testInfo.outputPath('native-board-super-admin-selected-tenant.png'), fullPage: true });
      await nativeInput(rolePage, 'native-board-name').fill(boardName);

      const createResponsePromise = rolePage.waitForResponse(isBoardCreateResponse, { timeout: 20000 });
      await rolePage.getByTestId('native-board-submit').click();
      const createResponse = await createResponsePromise;
      expect(createResponse.status()).toBe(200);
      const payload = createResponse.request().postDataJSON();
      expect(payload).toEqual(expect.objectContaining({ name: boardName, tenant_id: tenantId, vis_type: 'native' }));
      const body = await createResponse.json();
      expect(body).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({ name: boardName, tenant_id: tenantId, vis_type: 'native' })
      }));
      boardId = body.data.id;
      await expect(rolePage).toHaveURL(/\/visualization\/native-board\?id=/, { timeout: 20000 });
      await rolePage.screenshot({ path: testInfo.outputPath('native-board-super-admin-viewer.png'), fullPage: true });
    } finally {
      if (boardId) await api.delete('/board/' + boardId, {}, 'super_admin');
    }
  });
});
