/**
 * Write-adjacent E2E evidence. Seeded API records are matched against the
 * browser list so a visible row cannot pass without a real backend record.
 */

const { test, expect } = require('./fixtures');
const seedData = require('../lib/seed_data');

function isDeviceConfigListResponse(response) {
  try {
    const url = new URL(response.url());
    return response.request().method() === 'GET'
      && url.pathname.endsWith('/device_config');
  } catch {
    return false;
  }
}

function isDeviceConfigDetailResponse(response, id) {
  try {
    const url = new URL(response.url());
    return response.request().method() === 'GET'
      && url.pathname.endsWith('/device_config/' + id);
  } catch {
    return false;
  }
}

test.describe('safe write-adjacent flows', () => {
  test.use({ role: 'tenant_admin' });

  test('seeded device template is searchable in the UI and matches the list/detail APIs', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureDeviceConfig('tenant_admin');

    try {
      const detailResponse = await api.get('/device_config/' + seed.id, {}, 'tenant_admin');
      expect(detailResponse.code).toBe(200);
      expect(detailResponse.data).toEqual(expect.objectContaining({
        id: seed.id,
        name: expect.stringMatching(/\S/)
      }));
      const name = detailResponse.data.name;

      const initialResponsePromise = rolePage.waitForResponse(
        isDeviceConfigListResponse,
        { timeout: 20000 }
      );
      await rolePage.goto('/device/config', { waitUntil: 'domcontentloaded' });
      const initialResponse = await initialResponsePromise;
      expect(initialResponse.status()).toBe(200);

      const nameInput = rolePage.getByPlaceholder(/Enter Config Name|请输入配置名称/i);
      await nameInput.fill(name);
      const filteredResponsePromise = rolePage.waitForResponse(response => {
        if (!isDeviceConfigListResponse(response)) return false;
        return new URL(response.url()).searchParams.get('name') === name;
      }, { timeout: 20000 });
      await rolePage.getByRole('button', { name: /Search|搜索/i }).click();
      const filteredResponse = await filteredResponsePromise;
      expect(filteredResponse.status()).toBe(200);
      const body = await filteredResponse.json();
      expect(body).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({ total: expect.any(Number), list: expect.any(Array) })
      }));
      expect(body.data.list).toEqual(expect.arrayContaining([
        expect.objectContaining({ id: seed.id, name })
      ]));

      await expect(rolePage.getByText(name, { exact: true }).first()).toBeVisible();
      await expect(rolePage).toHaveURL(/\/device\/template$/);

      const detailLoadPromise = rolePage.waitForResponse(
        response => isDeviceConfigDetailResponse(response, seed.id),
        { timeout: 20000 }
      );
      await rolePage.getByText(name, { exact: true }).first().click();
      await expect(rolePage).toHaveURL(
        new RegExp('/device/config-detail\\?id=' + seed.id),
        { timeout: 20000 }
      );
      const browserDetailResponse = await detailLoadPromise;
      expect(browserDetailResponse.status()).toBe(200);
      const browserDetail = await browserDetailResponse.json();
      expect(browserDetail).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({ id: seed.id, name })
      }));
      await expect(rolePage.getByText(name, { exact: true }).first()).toBeVisible({ timeout: 15000 });
    } finally {
      await seed.cleanup();
    }
  });
});
