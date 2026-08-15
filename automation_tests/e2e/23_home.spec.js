/**
 * Home first-device workbench browser evidence.
 *
 * The page must consume the live first-device API response and refresh that
 * response from an operator action; static route rendering is not sufficient.
 */

const { test, expect } = require('./fixtures');
const seedData = require('../lib/seed_data');

test.describe('home first-device workbench [23_home]', () => {
  test.use({ role: 'tenant_admin' });

  test('home renders live first-device state and refreshes the workbench from the browser', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureDevice('tenant_admin');
    expect(String(seed.id || '').trim(), 'seeded device must have an id').not.toBe('');

    try {
      const firstDeviceResp = await api.get('/device', { page: 1, page_size: 1 }, 'tenant_admin');
      expect(firstDeviceResp.code).toBe(200);
      const firstDevice = seedData.listFromResponse(firstDeviceResp)[0];
      // The home contract is "the first device returned by the live API".
      // ensureDevice() makes sure that a usable device exists, but it may
      // legitimately reuse an existing fixture; it does not own the ordering
      // of every device already visible to this tenant.  Do not conflate the
      // setup fixture with page-one ordering.
      expect(firstDevice).toEqual(expect.objectContaining({
        id: expect.any(String),
        name: expect.any(String)
      }));

      const initialDeviceResponse = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return url.pathname.endsWith('/device') &&
          url.searchParams.get('page') === '1' &&
          url.searchParams.get('page_size') === '1';
      });
      await rolePage.goto('/home?onboarding=first-device', { waitUntil: 'domcontentloaded' });
      const initialBrowserResp = await initialDeviceResponse;
      expect(initialBrowserResp.status()).toBe(200);
      const initialBody = await initialBrowserResp.json();
      expect(initialBody.code).toBe(200);
      expect(seedData.listFromResponse(initialBody)).toEqual(
        expect.arrayContaining([expect.objectContaining({ id: firstDevice.id, name: firstDevice.name })])
      );

      await expect(rolePage).toHaveURL(/\/home\?onboarding=first-device/);
      await expect(rolePage.getByText('First device verification').first()).toBeVisible({ timeout: 15000 });
      await expect(rolePage.getByText('Minimal loop').first()).toBeVisible();
      await expect(rolePage.getByText(firstDevice.name).first()).toBeVisible();

      const refreshResponse = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return url.pathname.endsWith('/device') &&
          url.searchParams.get('page') === '1' &&
          url.searchParams.get('page_size') === '1';
      });
      await rolePage.getByRole('button', { name: 'Refresh', exact: true }).first().click();
      const refreshedBrowserResp = await refreshResponse;
      expect(refreshedBrowserResp.status()).toBe(200);
      const refreshedBody = await refreshedBrowserResp.json();
      expect(refreshedBody.code).toBe(200);
      expect(seedData.listFromResponse(refreshedBody)).toEqual(
        expect.arrayContaining([expect.objectContaining({ id: firstDevice.id })])
      );
      await expect(rolePage.getByText(firstDevice.name).first()).toBeVisible();
    } finally {
      await seed.cleanup();
    }
  });
});
