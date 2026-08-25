/**
 * Management E2E coverage must prove authenticated API state, rendered browser
 * state, or an exact permission rejection. Route visibility by itself is not
 * treated as business evidence.
 */

const { test, expect } = require('./fixtures');
const { isGetResponse } = require('./helpers/api_response_matcher');

function expectApiSuccess(resp, label) {
  expect(resp, label).toEqual(expect.objectContaining({ code: 200 }));
  expect(resp.data, label + ' data').not.toBeNull();
  expect(resp.data, label + ' data').not.toBeUndefined();
  return resp.data;
}

function expectListPayload(resp, label) {
  const data = expectApiSuccess(resp, label);
  expect(data, label + ' payload').toEqual(
    expect.objectContaining({
      list: expect.any(Array),
    }),
  );
  expect(data.list.length, label + ' list length').toBeGreaterThan(0);
  return data.list;
}

function flattenMenuNodes(nodes) {
  const flattened = [];
  const visit = (items) => {
    if (!Array.isArray(items)) return;
    items.forEach((item) => {
      if (!item || typeof item !== 'object') return;
      flattened.push(item);
      visit(item.children);
    });
  };
  visit(nodes);
  return flattened;
}

function parseConfigObject(value, label) {
  if (value == null || value === '' || value === 'null') return {};
  expect(value, label).toEqual(expect.any(String));
  const parsed = JSON.parse(value);
  expect(parsed, label + ' parsed').toEqual(expect.any(Object));
  return parsed;
}

function formInputByLabel(page, label) {
  return page.locator('.n-form-item').filter({ hasText: label }).locator('input').first();
}

test.describe('management module', () => {
  test.describe.configure({ timeout: 45000 });
  test.use({ role: 'super_admin' });

  test('super-admin tenant list in the browser matches authenticated user API state', async ({ rolePage, api, data }) => {
    await api.login('super_admin');
    const tenantAdminAccount = data.account('tenant_admin');
    expect(tenantAdminAccount).toEqual(
      expect.objectContaining({
        email: expect.stringContaining('@'),
      }),
    );

    const userRows = expectListPayload(
      await api.get('/user', { page: 1, page_size: 10, email: tenantAdminAccount.email }, 'super_admin'),
      'filtered super admin user list',
    );
    const tenantAdminUser = userRows.find((row) => row.email === tenantAdminAccount.email);
    expect(tenantAdminUser, 'tenant admin user row visible to super admin').toEqual(
      expect.objectContaining({
        authority: 'TENANT_ADMIN',
        email: tenantAdminAccount.email,
      }),
    );

    await rolePage.goto('/management/user', { waitUntil: 'domcontentloaded' });
    const emailInput = formInputByLabel(rolePage, /Email|\u90ae\u7bb1/i);
    await expect(emailInput).toBeVisible();

    const filteredResponse = rolePage.waitForResponse(
      (response) => isGetResponse(response, '/user', { email: tenantAdminAccount.email }),
      { timeout: 20000 },
    );
    await emailInput.fill(tenantAdminAccount.email);
    await rolePage.getByRole('button', { name: /Search|\u641c\u7d22|\u67e5\u8be2/i }).click();

    const browserResponse = await filteredResponse;
    expect(browserResponse.status()).toBe(200);
    const browserRows = expectListPayload(await browserResponse.json(), 'browser filtered user list');
    const browserTenantAdmin = browserRows.find((row) => row.email === tenantAdminAccount.email);
    expect(browserTenantAdmin).toEqual(
      expect.objectContaining({
        authority: tenantAdminUser.authority,
        email: tenantAdminUser.email,
      }),
    );

    await expect(rolePage).toHaveURL(/\/management\/user$/);
    await expect(rolePage.getByText('User Management').first()).toBeVisible();
    await expect(rolePage.getByText(tenantAdminUser.email).first()).toBeVisible();
    await expect(rolePage.locator('table')).toBeVisible();
  });

  test('super-admin role API access is separated from its unassigned UI route permission', async ({ rolePage, api }) => {
    await api.login('super_admin');

    const roleData = expectApiSuccess(
      await api.get('/role', { page: 1, page_size: 20 }, 'super_admin'),
      'super admin role list',
    );
    expect(roleData).toEqual(
      expect.objectContaining({
        total: expect.any(Number),
        list: expect.any(Array),
      }),
    );

    const menuRows = expectListPayload(
      await api.get('/ui_elements/menu', {}, 'super_admin'),
      'super admin menu',
    );
    const menuNodes = flattenMenuNodes(menuRows);
    const hasRoleRoute = menuNodes.some((node) =>
      node.element_code === 'management_role'
      || node.param1 === '/management/role'
      || node.route_path === 'view.management_role'
    );
    expect(hasRoleRoute, 'management role route assignment').toBe(false);

    const protectedRoleRequests = [];
    rolePage.on('request', (request) => {
      const url = new URL(request.url());
      // Keep the API transport prefix in the predicate so the browser route
      // itself (/management/role) cannot be mistaken for an API request.
      if (/(?:\/api\/v1|\/proxy-default)\/role$/.test(url.pathname)) {
        protectedRoleRequests.push(request.url());
      }
    });
    await rolePage.goto('/management/role', { waitUntil: 'domcontentloaded' });

    await expect(rolePage).toHaveURL(/\/403$/);
    expect(protectedRoleRequests, 'permission guard must stop the role-list request').toEqual([]);
    await expect(rolePage.getByRole('button', { name: /Logout|\u9000\u51fa\u767b\u5f55/i })).toBeVisible();
    await expect(rolePage.getByText(/404|Not Found|\u672a\u627e\u5230/i)).toHaveCount(0);
    await expect(rolePage.getByRole('button', { name: /Add Role|\u65b0\u589e\u89d2\u8272/i })).toHaveCount(0);
    await expect(rolePage.locator('table')).toHaveCount(0);
  });

  test('notification email form matches the persisted service configuration returned by API', async ({ rolePage, api }) => {
    await api.login('super_admin');

    const directData = expectApiSuccess(
      await api.get('/notification/services/config/EMAIL', {}, 'super_admin'),
      'email notification configuration',
    );
    expect(directData).toEqual(
      expect.objectContaining({
        notice_type: 'EMAIL',
        status: expect.stringMatching(/^(OPEN|CLOSE)$/),
      }),
    );
    const directConfig = parseConfigObject(directData.config, 'email notification config JSON');

    const browserRequest = rolePage.waitForResponse(
      (response) => isGetResponse(response, '/notification/services/config/EMAIL'),
      { timeout: 20000 },
    );
    await rolePage.goto('/management/notification', { waitUntil: 'domcontentloaded' });
    const browserResponse = await browserRequest;
    expect(browserResponse.status()).toBe(200);

    const browserData = expectApiSuccess(await browserResponse.json(), 'browser email notification configuration');
    expect(browserData).toEqual(
      expect.objectContaining({
        notice_type: 'EMAIL',
        status: directData.status,
        config: directData.config,
      }),
    );
    const browserConfig = parseConfigObject(browserData.config, 'browser email notification config JSON');
    expect(browserConfig.host).toBe(directConfig.host);
    expect(browserConfig.port).toBe(directConfig.port);
    expect(browserConfig.from_email).toBe(directConfig.from_email);
    expect(browserConfig.ssl).toBe(directConfig.ssl);

    await expect(rolePage).toHaveURL(/\/management\/notification$/);
    await expect(
      formInputByLabel(rolePage, /Send Mail Server|\u53d1\u9001\u90ae\u4ef6\u670d\u52a1\u5668/i),
    ).toHaveValue(String(directConfig.host || ''));
    await expect(
      formInputByLabel(rolePage, /Send Port|\u53d1\u9001\u7aef\u53e3/i),
    ).toHaveValue(directConfig.port == null ? '' : String(directConfig.port));
    await expect(
      formInputByLabel(rolePage, /Sender Email|\u53d1\u9001\u4eba\u90ae\u4ef6/i),
    ).toHaveValue(String(directConfig.from_email || ''));
    await expect(
      rolePage.locator('.n-form-item').filter({ hasText: /SSL/i }).getByRole('checkbox'),
    ).toBeChecked({ checked: directConfig.ssl === true });

    const statusItem = rolePage.locator('.n-form-item').filter({
      hasText: /Enable\/Disable Service|\u5f00\u542f\/\u5173\u95ed\u670d\u52a1/i,
    });
    await expect(statusItem.getByRole('switch')).toHaveAttribute(
      'aria-checked',
      directData.status === 'OPEN' ? 'true' : 'false',
    );
  });

  test.describe('service details route', () => {
    // The route is tenant-admin scoped in the platform menu.  Keep the
    // super-admin permission-boundary cases above separate from this real
    // browser business flow instead of asserting a 403 page by accident.
    test.use({ role: 'tenant_admin' });

    test('renders the selected plugin access list from API', async ({ rolePage, api }) => {
      await api.login('tenant_admin');

    const services = expectListPayload(
      // Plugin catalog management is SYS_ADMIN-only; the selected access
      // list itself is tenant-scoped and is verified below with the browser
      // role.  Use the two explicit contracts instead of assuming one role
      // can do both operations.
      await api.get('/service/list', { page: 1, page_size: 20 }, 'super_admin'),
      'service plugin list',
    );
    const service = services.find(
      (row) => row && row.id && row.name
        && String(row.service_identifier || '').trim().toUpperCase() === 'HTTP',
    );
    expect(service, 'registered HTTP service plugin required for the local SVCR contract').toEqual(
      expect.objectContaining({
        id: expect.any(String),
        name: expect.stringMatching(/\S/),
        service_identifier: expect.stringMatching(/^HTTP$/i),
      }),
    );

    const query = {
      page: 1,
      page_size: 10,
      service_plugin_id: service.id,
    };
    const directData = expectApiSuccess(
      await api.get('/service/access/list', query, 'tenant_admin'),
      'selected service access list',
    );
    expect(directData).toEqual(
      expect.objectContaining({
        total: expect.any(Number),
        list: expect.any(Array),
      }),
    );

    const browserRequest = rolePage.waitForResponse(
      (response) => isGetResponse(response, '/service/access/list', query),
      { timeout: 20000 },
    );
    await rolePage.goto(
      `/device/service-details?id=${encodeURIComponent(service.id)}`
        + `&service_type=${encodeURIComponent(service.service_type || '')}`
        + `&service_name=${encodeURIComponent(service.name)}`
        + `&service_identifier=${encodeURIComponent(service.service_identifier || '')}`,
      { waitUntil: 'domcontentloaded' },
    );
    const browserResponse = await browserRequest;
    expect(browserResponse.status()).toBe(200);
    const browserData = expectApiSuccess(await browserResponse.json(), 'browser service access list');
    expect(browserData.total).toBe(directData.total);
    expect(browserData.list).toEqual(expect.any(Array));
    expect(browserData.list.length).toBe(directData.list.length);

    await expect(rolePage).toHaveURL(/\/device\/service-details\?id=/);
    await expect(rolePage.getByText(service.name, { exact: true }).first()).toBeVisible();

    // HTTP SVCR is a platform-local form contract. The optional adapter is
    // required only for automatic discovery and protocol traffic, not for
    // opening the access-point form.
    const formProbe = await api.get(
      '/service/access/voucher/form',
      { service_plugin_id: service.id },
      'tenant_admin',
    );
    expect(formProbe.code, 'HTTP SVCR form is a local platform contract').toBe(200);
    expect(Array.isArray(formProbe.data), 'voucher form response data must be an array').toBe(true);
    const addAccessButton = rolePage.getByRole('button', { name: /New Access|新建接入|新增接入/i }).first();
    await expect(addAccessButton).toBeVisible();
    await addAccessButton.click();
    await expect(rolePage.locator('.n-modal').last()).toBeVisible({ timeout: 15000 });
    if (directData.list.length === 0) {
      await expect(rolePage.locator('.service-access-empty')).toBeVisible({ timeout: 15000 });
    } else {
      await expect(rolePage.locator('.n-data-table-base-table-body .n-data-table-tr')).toHaveCount(
        directData.list.length,
      );
    }
    });
  });
});
