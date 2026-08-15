/**
 * Standalone device-details application boundary evidence. The route contract
 * uses the real `d_id` query parameter and must render the same device returned
 * by the authenticated detail and telemetry APIs. The page is read-only, so it
 * is not counted as an operator business-flow closure case.
 */

const { test, expect } = require('./fixtures');
const seedData = require('../lib/seed_data');

function isDeviceDetailResponse(response, deviceId) {
  const url = new URL(response.url());
  return response.request().method() === 'GET'
    && url.pathname.endsWith('/device/detail/' + deviceId);
}

function isTelemetryCurrentResponse(response, deviceId) {
  const url = new URL(response.url());
  return response.request().method() === 'GET'
    && url.pathname.endsWith('/telemetry/datas/current/' + deviceId);
}

test.describe('standalone device details app route [19_device_details_app]', () => {
  test.use({ role: 'tenant_admin' });

  test('seeded device opens in the standalone app with matching detail API identity and status', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureDevice('tenant_admin');

    try {
      const apiDetail = await api.get('/device/detail/' + seed.id, {}, 'tenant_admin');
      expect(apiDetail.code).toBe(200);
      expect(apiDetail.data).toEqual(expect.objectContaining({
        id: seed.id,
        name: expect.stringMatching(/\S/),
        device_number: expect.stringMatching(/\S/),
        is_online: expect.any(Number)
      }));

      const browserDetailPromise = rolePage.waitForResponse(
        response => isDeviceDetailResponse(response, seed.id),
        { timeout: 20000 }
      );
      const browserTelemetryPromise = rolePage.waitForResponse(
        response => isTelemetryCurrentResponse(response, seed.id),
        { timeout: 20000 }
      );
      await rolePage.goto('/device-details-app?d_id=' + encodeURIComponent(seed.id), {
        waitUntil: 'domcontentloaded'
      });
      const browserDetail = await browserDetailPromise;
      expect(browserDetail.status()).toBe(200);
      const browserBody = await browserDetail.json();
      expect(browserBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({
          id: seed.id,
          name: apiDetail.data.name,
          device_number: apiDetail.data.device_number,
          is_online: apiDetail.data.is_online
        })
      }));
      const browserTelemetry = await browserTelemetryPromise;
      expect(browserTelemetry.status()).toBe(200);
      const telemetryBody = await browserTelemetry.json();
      expect(telemetryBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.any(Array)
      }));

      await expect(rolePage).toHaveURL(new RegExp('/device-details-app\\?d_id=' + seed.id));
      await expect(rolePage.getByRole('heading', { name: apiDetail.data.name })).toBeVisible();
      await expect(rolePage.getByText(apiDetail.data.is_online === 1 ? /Online|在线/i : /Offline|离线/i).first()).toBeVisible();
      await expect(rolePage.getByTestId('device-details-app')).toBeVisible();
      await expect(rolePage.locator('.device-details-app')).toBeVisible();
      await expect(rolePage.getByText(/403|404|Not Found|Back to Home/i)).toHaveCount(0);

      const [refreshedDetail] = await Promise.all([
        rolePage.waitForResponse(
          response => isDeviceDetailResponse(response, seed.id),
          { timeout: 20000 }
        ),
        rolePage.reload({ waitUntil: 'domcontentloaded' })
      ]);
      expect(refreshedDetail.status()).toBe(200);
      const refreshedBody = await refreshedDetail.json();
      expect(refreshedBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({
          id: seed.id,
          name: apiDetail.data.name,
          device_number: apiDetail.data.device_number,
          is_online: apiDetail.data.is_online
        })
      }));
      await expect(rolePage.getByRole('heading', { name: apiDetail.data.name })).toBeVisible({ timeout: 20000 });
      await expect(rolePage.getByText(/403|404|Not Found|Back to Home/i)).toHaveCount(0);
    } finally {
      await seed.cleanup();
    }
  });
});
