/**
 * OTA support archive browser evidence.
 *
 * Business value: a support operator can open a task-level rollout, download the
 * JSON handoff archive, and follow Ready Check links for representative failed devices.
 */

const { test, expect } = require('./fixtures');
const fs = require('fs');
const seedData = require('../lib/seed_data');

function expectOtaSupportArchive(bundle, source) {
  expect(bundle).toEqual(
    expect.objectContaining({
      task_id: source.taskId,
      package_id: source.packageId,
      generated_at: expect.any(String),
      total_rows: expect.any(Number),
      failed_count: expect.any(Number),
      failed_devices: expect.any(Array),
      failure_groups: expect.any(Array),
      next_actions: expect.any(Array),
      evidence_boundary: expect.any(Array),
      share_hint: expect.any(String)
    })
  );
  expect(Number.isFinite(Date.parse(bundle.generated_at))).toBe(true);
  expect(Number.isInteger(bundle.total_rows)).toBe(true);
  expect(Number.isInteger(bundle.failed_count)).toBe(true);
  expect(bundle.total_rows).toBeGreaterThan(0);
  expect(bundle.failed_count < 0).toBe(false);
  expect(bundle.total_rows).toBeGreaterThanOrEqual(bundle.failed_count);
  expect(Array.isArray(bundle.statistics)).toBe(true);
  expect(bundle.statistics.length).toBeGreaterThan(0);
  for (const row of bundle.statistics) {
    expect(row).toEqual(expect.objectContaining({
      status: expect.anything(),
      count: expect.anything()
    }));
    expect(Number.isInteger(Number(row.status))).toBe(true);
    expect(Number.isInteger(Number(row.count))).toBe(true);
    expect(Number(row.count)).toBeGreaterThan(0);
  }
  const statisticTotal = bundle.statistics.reduce((sum, row) => sum + Number(row.count), 0);
  const failedStatisticTotal = bundle.statistics
    .filter(row => String(row.status) === '5' || String(row.status).toLowerCase() === 'failed')
    .reduce((sum, row) => sum + Number(row.count), 0);
  expect(statisticTotal).toBe(bundle.total_rows);
  expect(failedStatisticTotal).toBe(bundle.failed_count);
  const groupedFailureTotal = bundle.failure_groups.reduce((sum, group) => sum + Number(group.count || 0), 0);
  expect(groupedFailureTotal).toBe(bundle.failed_count);
  expect(bundle.failed_devices.length).toBeLessThanOrEqual(Math.min(bundle.failed_count, 50));
  expect(bundle.next_actions.length).toBeGreaterThan(0);
  expect(bundle.evidence_boundary.length).toBeGreaterThan(0);
  expect(bundle.share_hint.trim()).not.toBe('');

  for (const group of bundle.failure_groups) {
    expect(group).toEqual(
      expect.objectContaining({
        reason: expect.any(String),
        count: expect.any(Number)
      })
    );
    expect(group.reason.trim()).not.toBe('');
    expect(group.count).toBeGreaterThan(0);
  }

  if (bundle.failed_count === 0) {
    expect(bundle.failed_devices).toEqual([]);
    expect(bundle.failure_groups).toEqual([]);
  } else {
    expect(bundle.failed_devices.length).toBeGreaterThan(0);
    expect(bundle.failure_groups.length).toBeGreaterThan(0);
  }

  for (const device of bundle.failed_devices) {
    expect(device).toEqual(
      expect.objectContaining({
        detail_id: expect.any(String),
        device_id: expect.any(String),
        failure_reason: expect.stringMatching(/\S/),
        ready_check_url: expect.stringMatching(/\S/)
      })
    );
    expect(device.detail_id.trim()).not.toBe('');
    expect(device.device_id.trim()).not.toBe('');
    expect(device.ready_check_url).toContain('tab=ready-check');
    expect(device.ready_check_url).toContain('source=ota');
    expect(device.ready_check_url).toContain('ota_task_id=' + encodeURIComponent(source.taskId));
    expect(device.ready_check_url).toContain('ota_detail_id=' + encodeURIComponent(device.detail_id));
  }
}

function pickId(row) {
  return row && (row.id || row.ID || null);
}

function authorityHas(row, authority) {
  if (!row) return false;
  try {
    const values = JSON.parse(String(row.authority || '[]'));
    return Array.isArray(values) && values.includes(authority);
  } catch (_) {
    return String(row.authority || '').includes(authority);
  }
}

function uiElementRows(resp) {
  if (!resp || resp.code !== 200 || !resp.data) return [];
  const roots = Array.isArray(resp.data) ? resp.data : resp.data.list;
  if (!Array.isArray(roots)) return [];
  const rows = [];
  const visit = items => {
    for (const item of items) {
      if (!item || typeof item !== 'object') continue;
      rows.push(item);
      if (Array.isArray(item.children)) visit(item.children);
    }
  };
  visit(roots);
  return rows;
}

/**
 * The local seed database does not always carry the product menu entry. Create
 * only the route needed by this browser proof, then remove it in the caller's
 * finally block. The parent lookup keeps the fixture inside the normal product
 * menu hierarchy when that parent is available, while still working on a
 * minimal database where menus are rooted at `0`.
 */
async function createOtaRouteMenuFixture(api) {
  await api.login('super_admin');
  const suffix = Date.now();
  const parentCode = 'automation_product_' + suffix;
  const elementCode = 'automation_ota_support_' + suffix;
  let parentId = null;
  let parentCreated = false;
  let childId = null;
  try {
    const listResp = await api.get('/ui_elements', { page: 1, page_size: 500 }, 'super_admin');
    expect(listResp.code).toBe(200);

    const rows = uiElementRows(listResp);
    let parent = rows.find(row => authorityHas(row, 'TENANT_ADMIN') && (
      String(row.param1 || '').trim() === '/product'
      || String(row.route_path || '').trim() === 'view.product'
      || String(row.element_code || '').trim() === 'product'
    ));

    if (!parent) {
      const parentResp = await api.post('/ui_elements', {
        parent_id: '0',
        element_code: parentCode,
        element_type: 1,
        orders: 998,
        param1: '/product',
        param2: 'carbon:package',
        param3: 'self',
        authority: '["TENANT_ADMIN","SYS_ADMIN"]',
        description: 'OTA support parent fixture',
        remark: 'automation_ota_menu_fixture',
        multilingual: 'route.product',
        route_path: 'layout.base'
      }, 'super_admin');
      expect(parentResp.code, JSON.stringify(parentResp)).toBe(200);
      parentCreated = true;
      parentId = pickId(parentResp.data);

      const parentListResp = await api.get('/ui_elements', { page: 1, page_size: 500 }, 'super_admin');
      expect(parentListResp.code).toBe(200);
      parent = uiElementRows(parentListResp).find(row => row.element_code === parentCode);
      parentId = pickId(parent) || parentId;
      expect(parentId).toEqual(expect.any(String));
    } else {
      parentId = pickId(parent);
    }

    const createResp = await api.post('/ui_elements', {
      parent_id: parentId || '0',
      element_code: elementCode,
      element_type: 3,
      orders: 999,
      param1: '/product/update-ota',
      param2: 'carbon:package',
      param3: 'self',
      authority: '["TENANT_ADMIN","SYS_ADMIN"]',
      description: 'OTA support route fixture',
      remark: 'automation_ota_menu_fixture',
      multilingual: 'route.product_update-ota',
      route_path: 'view.product_update-ota'
    }, 'super_admin');
    expect(createResp.code, JSON.stringify(createResp)).toBe(200);
    childId = pickId(createResp.data);

    const persistedResp = await api.get('/ui_elements', { page: 1, page_size: 500 }, 'super_admin');
    expect(persistedResp.code).toBe(200);
    let created = uiElementRows(persistedResp).find(row => row.element_code === elementCode);
    if (!created) {
      // The paged endpoint returns only roots on this deployment; child IDs
      // are exposed by the nested menu endpoint instead.
      const menuResp = await api.get('/ui_elements/menu', {}, 'super_admin');
      expect(menuResp.code).toBe(200);
      created = uiElementRows(menuResp).find(row => row.element_code === elementCode);
    }
    childId = pickId(created) || childId;
    expect(childId, JSON.stringify({ createResp, persistedResp, created })).toEqual(expect.any(String));
    expect(created || createResp.data).toEqual(expect.objectContaining({
      element_code: elementCode,
      param1: '/product/update-ota',
      route_path: 'view.product_update-ota'
    }));

    return {
      id: childId,
      cleanup: async () => {
        const childDeleteResp = await api.delete('/ui_elements/' + childId, {}, 'super_admin');
        expect(childDeleteResp.code).toBe(200);
        if (parentCreated && parentId) {
          const parentDeleteResp = await api.delete('/ui_elements/' + parentId, {}, 'super_admin');
          expect(parentDeleteResp.code).toBe(200);
        }
      }
    };
  } catch (error) {
    if (childId) {
      await api.delete('/ui_elements/' + childId, {}, 'super_admin');
    }
    if (parentCreated && parentId) {
      const cleanupParentResp = await api.get('/ui_elements', { page: 1, page_size: 500 }, 'super_admin');
      const createdParent = uiElementRows(cleanupParentResp).find(row => row.element_code === parentCode);
      const cleanupParentId = pickId(createdParent) || parentId;
      if (cleanupParentId) {
        await api.delete('/ui_elements/' + cleanupParentId, {}, 'super_admin');
      }
    }
    throw error;
  }
}

test.describe('ota support archive module', () => {
  test.use({ role: 'tenant_admin' });

  test('OTA task detail downloads support archive with task-level counts and conditional Ready Check handoff fields', async ({
    rolePage,
    api
  }) => {
    await api.login('tenant_admin');
    const source = await seedData.ensureOtaTaskSupportBundleSource('tenant_admin');
    let menuFixture = null;
    try {
      expect(source.blocked, source.reason || 'OTA task support-bundle source is required').not.toBe(true);

      const apiBundleResp = await api.get('/ota/task/' + source.taskId + '/support-bundle', {}, 'tenant_admin');
      expect(apiBundleResp.code).toBe(200);
      expectOtaSupportArchive(apiBundleResp.data, source);

      // The browser's dynamic route table is loaded from UI elements. Seed the
      // route with a super-admin API token before opening the tenant page.
      menuFixture = await createOtaRouteMenuFixture(api);

      const pageUrl =
        '/product/update-ota' +
        '?ota_package_id=' + encodeURIComponent(source.packageId) +
        '&source=ready-check' +
        '&ota_task_id=' + encodeURIComponent(source.taskId);
      await rolePage.goto(pageUrl, { waitUntil: 'domcontentloaded' });

      await expect(rolePage).toHaveURL(/\/product\/update-ota/);
      await expect(rolePage.getByTestId('ota-download-task-support-bundle')).toBeVisible({ timeout: 15000 });

      const supportResponsePromise = rolePage.waitForResponse(
        response => response.url().includes('/ota/task/' + source.taskId + '/support-bundle') && response.status() === 200,
        { timeout: 20000 }
      );
      const downloadPromise = rolePage.waitForEvent('download');
      await rolePage.getByTestId('ota-download-task-support-bundle').click();
      const download = await downloadPromise;
      const supportResponse = await supportResponsePromise;
      const supportBody = await supportResponse.json();
      expect(supportBody.code).toBe(200);
      expectOtaSupportArchive(supportBody.data, source);

      const filePath = await download.path();
      expect(download.suggestedFilename()).toBe('aetherlink-ota-task-' + source.taskId + '-support-bundle.json');
      expect(filePath).toEqual(expect.any(String));

      const browserBundle = JSON.parse(fs.readFileSync(filePath, 'utf8'));
      expectOtaSupportArchive(browserBundle, source);
      expect(browserBundle).toEqual(supportBody.data);
    } finally {
      if (menuFixture && menuFixture.cleanup) await menuFixture.cleanup();
      if (source.cleanup) await source.cleanup();
    }
  });
});
