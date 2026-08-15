/**
 * Apply-management permission evidence. These cases assert exact API and UI
 * authorization outcomes; they are boundaries, not marketplace business-flow
 * closure.
 */

const { test, expect } = require('./fixtures');

async function expectPermissionDenied(page, route) {
  await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 15000 });
  await expect(page).toHaveURL(/\/403$/);
  await expect(page.getByRole('button', { name: /Logout|退出登录/i })).toBeVisible({ timeout: 15000 });
  await expect(page.getByRole('button', { name: /Back to Home|返回首页/i })).toBeVisible();
  await expect(page.getByText(/404|Not Found/i)).toHaveCount(0);
}

test.describe('apply marketplace routes [15_apply_marketplace]', () => {
  test.describe('tenant-admin permission boundary', () => {
    test.use({ role: 'tenant_admin' });

    test('tenant admin is denied service-plugin APIs and both apply management routes', async ({ rolePage, api }) => {
      await api.login('tenant_admin');
      const serviceResp = await api.get('/service/list', { page: 1, page_size: 10 }, 'tenant_admin');
      expect(serviceResp.code).toBe(201001);
      expect(serviceResp.message).toEqual(expect.stringMatching(/permission|权限/i));

      for (const route of ['/apply/plugin', '/apply/service']) {
        await expectPermissionDenied(rolePage, route);
      }
    });
  });

  test.describe('super-admin API and legacy route boundary', () => {
    test.use({ role: 'super_admin' });

    test('super admin service-plugin API remains available while the legacy apply route is explicitly denied', async ({ rolePage, api }) => {
      await api.login('super_admin');
      const serviceResp = await api.get('/service/list', { page: 1, page_size: 10 }, 'super_admin');
      expect(serviceResp.code).toBe(200);
      expect(serviceResp.data).toEqual(expect.objectContaining({
        total: expect.any(Number),
        list: expect.any(Array)
      }));

      await expectPermissionDenied(rolePage, '/apply/service');
    });
  });
});
