/** Tenant dashboard authorization boundaries. */

const { test, expect } = require('./fixtures');

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
  expect(response, 'current user menu').toEqual(expect.objectContaining({ code: 200 }));
  expect(response.data).toEqual(expect.objectContaining({ list: expect.any(Array) }));
  return flattenMenuNodes(response.data.list).map(row => String(row.param1 || row.path || '').trim()).filter(Boolean);
}

async function expectDashboardDenied(page, route) {
  await page.goto(route, { waitUntil: 'domcontentloaded' });
  await expect(page).toHaveURL(/\/403$/);
  await expect(page.getByRole('button', { name: /Logout|退出登录/i })).toBeVisible({ timeout: 15000 });
  await expect(page.getByRole('button', { name: /Back to Home|返回首页/i })).toBeVisible();
  await expect(page.getByText(/404|Not Found/i)).toHaveCount(0);
}

test.describe('dashboard module', () => {
  test.use({ role: 'tenant_admin' });

  test('tenant admin receives an explicit 403 boundary for the dashboard root', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const paths = menuPaths(await api.get('/ui_elements/menu', {}, 'tenant_admin'));
    expect(paths).not.toContain('/dashboard');

    await expectDashboardDenied(rolePage, '/dashboard');
  });

  test('tenant admin receives an explicit 403 boundary for the dashboard RDI route', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const paths = menuPaths(await api.get('/ui_elements/menu', {}, 'tenant_admin'));
    expect(paths).not.toContain('/dashboard/rdi-overview');

    await expectDashboardDenied(rolePage, '/dashboard/rdi-overview');
  });
});
