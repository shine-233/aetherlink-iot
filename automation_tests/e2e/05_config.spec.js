/**
 * Device configuration browser closure. The flow starts from a real API seed,
 * verifies the detail response rendered by the browser, edits the persisted
 * name through the UI, and reads the result back through the API.
 */

const { test, expect } = require('./fixtures');
const seedData = require('../lib/seed_data');

function isDeviceConfigResponse(response, method, id) {
  try {
    const url = new URL(response.url());
    const expectedPath = id
      ? '/device_config/' + id
      : '/device_config';
    return response.request().method() === method && url.pathname.endsWith(expectedPath);
  } catch {
    return false;
  }
}

function escapeRegExp(value) {
  return String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

test.describe('device config module', () => {
  test.use({ role: 'tenant_admin' });

  test('seeded device config detail can be renamed through the browser and read back from the API', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureDeviceConfig('tenant_admin');
    let deleted = false;

    try {
      const persistedBefore = await api.get('/device_config/' + seed.id, {}, 'tenant_admin');
      expect(persistedBefore.code).toBe(200);
      expect(persistedBefore.data).toEqual(expect.objectContaining({
        id: seed.id,
        name: expect.stringMatching(/\S/),
        device_type: '1',
        protocol_type: 'MQTT',
        voucher_type: 'ACCESSTOKEN'
      }));

      const initialListPromise = rolePage.waitForResponse(
        response => isDeviceConfigResponse(response, 'GET'),
        { timeout: 20000 }
      );
      await rolePage.goto('/device/template', { waitUntil: 'domcontentloaded' });
      const initialListResponse = await initialListPromise;
      expect(initialListResponse.status()).toBe(200);
      const initialListBody = await initialListResponse.json();
      expect(initialListBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({ total: expect.any(Number), list: expect.any(Array) })
      }));

      const configNameInput = rolePage.getByPlaceholder(/config.*name/i);
      await configNameInput.fill(persistedBefore.data.name);
      const filteredListPromise = rolePage.waitForResponse(response => {
        if (!isDeviceConfigResponse(response, 'GET')) return false;
        return new URL(response.url()).searchParams.get('name') === persistedBefore.data.name;
      }, { timeout: 20000 });
      await rolePage.getByRole('button', { name: 'Search' }).click();
      const filteredListResponse = await filteredListPromise;
      expect(filteredListResponse.status()).toBe(200);
      const filteredListBody = await filteredListResponse.json();
      expect(filteredListBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({
          list: expect.arrayContaining([
            expect.objectContaining({
              id: seed.id,
              name: persistedBefore.data.name,
              device_type: persistedBefore.data.device_type,
              protocol_type: persistedBefore.data.protocol_type
            })
          ])
        })
      }));
      const seededConfigLink = rolePage.getByText(persistedBefore.data.name, { exact: true }).first();
      await expect(seededConfigLink).toBeVisible();

      const detailResponsePromise = rolePage.waitForResponse(
        response => isDeviceConfigResponse(response, 'GET', seed.id),
        { timeout: 20000 }
      );
      await seededConfigLink.click();
      await expect(rolePage).toHaveURL(new RegExp('/device/config-detail\\?id=' + escapeRegExp(seed.id)));
      const detailResponse = await detailResponsePromise;
      expect(detailResponse.status()).toBe(200);
      const detailBody = await detailResponse.json();
      expect(detailBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({ id: seed.id, name: persistedBefore.data.name })
      }));
      await expect(rolePage.getByText(persistedBefore.data.name, { exact: true }).first()).toBeVisible();

      const editLoadPromise = rolePage.waitForResponse(
        response => isDeviceConfigResponse(response, 'GET', seed.id),
        { timeout: 20000 }
      );
      await rolePage.getByRole('button', { name: /Edit|编辑/i }).click();
      await expect(rolePage).toHaveURL(new RegExp('/device/config-edit\\?id=' + escapeRegExp(seed.id)));
      await editLoadPromise;

      const nameInput = rolePage.getByPlaceholder(/Enter the device configuration name|请输入设备配置名称/i);
      await expect(nameInput).toHaveValue(persistedBefore.data.name);
      const updatedName = persistedBefore.data.name + '_browser_edit';
      await nameInput.fill(updatedName);

      const updateResponsePromise = rolePage.waitForResponse(
        response => isDeviceConfigResponse(response, 'PUT'),
        { timeout: 20000 }
      );
      const refreshedDetailPromise = rolePage.waitForResponse(
        response => isDeviceConfigResponse(response, 'GET', seed.id),
        { timeout: 20000 }
      );
      await rolePage.getByRole('button', { name: /Confirm|确认/i }).click();
      const updateResponse = await updateResponsePromise;
      expect(updateResponse.status()).toBe(200);
      const updateBody = await updateResponse.json();
      expect(updateBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({
          name: updatedName,
          protocol_type: persistedBefore.data.protocol_type,
          voucher_type: persistedBefore.data.voucher_type
        })
      }));

      await expect(rolePage).toHaveURL(new RegExp('/device/config-detail\\?id=' + escapeRegExp(seed.id)));
      const refreshedDetailResponse = await refreshedDetailPromise;
      expect(refreshedDetailResponse.status()).toBe(200);
      const refreshedDetailBody = await refreshedDetailResponse.json();
      expect(refreshedDetailBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({ id: seed.id, name: updatedName })
      }));
      await expect(rolePage.getByText(updatedName, { exact: true }).first()).toBeVisible();

      const persistedAfter = await api.get('/device_config/' + seed.id, {}, 'tenant_admin');
      expect(persistedAfter.code).toBe(200);
      expect(persistedAfter.data).toEqual(expect.objectContaining({
        id: seed.id,
        name: updatedName,
        device_type: persistedBefore.data.device_type,
        protocol_type: persistedBefore.data.protocol_type,
        voucher_type: persistedBefore.data.voucher_type
      }));

      const updatedListLoadPromise = rolePage.waitForResponse(
        response => isDeviceConfigResponse(response, 'GET'),
        { timeout: 20000 }
      );
      await rolePage.goto('/device/template', { waitUntil: 'domcontentloaded' });
      const updatedListLoad = await updatedListLoadPromise;
      expect(updatedListLoad.status()).toBe(200);
      const updatedNameInput = rolePage.getByPlaceholder(/config.*name/i);
      await updatedNameInput.fill(updatedName);
      const updatedSearchPromise = rolePage.waitForResponse(response => {
        if (!isDeviceConfigResponse(response, 'GET')) return false;
        return new URL(response.url()).searchParams.get('name') === updatedName;
      }, { timeout: 20000 });
      await rolePage.getByRole('button', { name: 'Search' }).click();
      const updatedSearchResponse = await updatedSearchPromise;
      expect(updatedSearchResponse.status()).toBe(200);
      const updatedSearchBody = await updatedSearchResponse.json();
      expect(updatedSearchBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({
          list: expect.arrayContaining([
            expect.objectContaining({ id: seed.id, name: updatedName })
          ])
        })
      }));
      await expect(rolePage.getByText(updatedName, { exact: true }).first()).toBeVisible();

      const deleteResponse = await api.delete('/device_config/' + seed.id, {}, 'tenant_admin');
      expect(deleteResponse.code).toBe(200);
      deleted = true;
      const deletedDetail = await api.get('/device_config/' + seed.id, {}, 'tenant_admin');
      expect([100000, 101001]).toContain(deletedDetail.code);
    } finally {
      if (!deleted) await seed.cleanup();
    }
  });
});
