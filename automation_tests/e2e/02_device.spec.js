/**
 * Device module E2E evidence. Each case exercises a seeded or API-backed
 * device state through a browser interaction and an observable result; pure
 * navigation-only checks are intentionally not represented here.
 */

const {
  test,
  expect,
  getStorageStatePath,
  storageStateExists,
  instrumentPageCoverage
} = require('./fixtures');
const config = require('../lib/network_runtime');
const seedData = require('../lib/seed_data');
const { skipWhenBlocked } = require('../lib/integration_blocked');
const fs = require('fs');
const {
  createTenantAdminAccount,
  cleanupDynamicAccounts
} = require('../tests/helpers/dynamic_accounts');

function createStorageState(token, userInfo) {
  const origin = new URL(config.frontendURL).origin;
  const normalizedUserInfo = {
    ...userInfo,
    roles: Array.isArray(userInfo.roles) && userInfo.roles.length ? userInfo.roles : [userInfo.authority].filter(Boolean)
  };

  return {
    cookies: [],
    origins: [
      {
        origin,
        localStorage: [
          { name: 'token', value: JSON.stringify(token) },
          { name: 'token_expires_in', value: JSON.stringify(String(Date.now() + 7200 * 1000)) },
          { name: 'userInfo', value: JSON.stringify(normalizedUserInfo) }
        ]
      }
    ]
  };
}

async function createRecipientBrowserContext(browser, api) {
  if (storageStateExists('tenant_admin_b')) {
    return {
      context: await browser.newContext({
        storageState: getStorageStatePath('tenant_admin_b'),
        baseURL: config.frontendURL
      }),
      accountKey: 'tenant_admin_b',
      cleanupAccounts: []
    };
  }

  const account = await createTenantAdminAccount(api.client, 'codex_e2e_share_recipient');
  const detailResp = await api.get('/user/detail', {}, account.accountKey);
  expect(detailResp.code).toBe(200);
  expect(detailResp.data).toEqual(expect.objectContaining({
    id: expect.any(String),
    email: account.email
  }));

  return {
    context: await browser.newContext({
      storageState: createStorageState(api.client.tokens[account.accountKey], detailResp.data),
      baseURL: config.frontendURL
    }),
    accountKey: account.accountKey,
    cleanupAccounts: [account]
  };
}

test.describe('device module', () => {
  test.use({ role: 'tenant_admin' });

  test('seeded device search matches API state and opens the selected device', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureDevice('tenant_admin');

    try {
      const detailResp = await api.get('/device/detail/' + seed.id, {}, 'tenant_admin');
      expect(detailResp.code).toBe(200);
      expect(detailResp.data).toEqual(expect.objectContaining({ id: seed.id, name: expect.any(String) }));
      const deviceName = detailResp.data.name;

      const listResp = await api.get('/device', { page: 1, page_size: 100, name: deviceName }, 'tenant_admin');
      expect(listResp.code).toBe(200);
      expect(seedData.listFromResponse(listResp)).toEqual(
        expect.arrayContaining([expect.objectContaining({ id: seed.id, name: deviceName })])
      );

      await rolePage.goto('/device/manage', { waitUntil: 'domcontentloaded' });
      await expect(rolePage).toHaveURL(/\/device\/manage$/);
      await expect(rolePage.getByText('Device Management').first()).toBeVisible();

      await rolePage.getByPlaceholder('Device Name').fill(deviceName);
      const filteredResponse = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return url.pathname.endsWith('/device') && url.searchParams.get('name') === deviceName;
      });
      await rolePage.getByRole('button', { name: 'Query' }).click();
      const browserResp = await filteredResponse;
      expect(browserResp.status()).toBe(200);
      const browserBody = await browserResp.json();
      expect(browserBody.code).toBe(200);
      expect(seedData.listFromResponse(browserBody)).toEqual(
        expect.arrayContaining([expect.objectContaining({ id: seed.id, name: deviceName })])
      );

      const deviceCard = rolePage.locator('.item-card').filter({ hasText: deviceName }).first();
      await expect(deviceCard).toBeVisible();
      await deviceCard.click();
      await expect(rolePage).toHaveURL(new RegExp('/device/details\\?d_id=' + seed.id));
    } finally {
      await seed.cleanup();
    }
  });

  test('manual add submits a real device and exposes it through list and detail APIs', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const suffix = (Date.now().toString(36) + Math.random().toString(36).slice(2, 5))
      .replace(/[^a-z0-9]/gi, '')
      .toUpperCase();
    const pid = ('E2E' + suffix).slice(0, 12).padEnd(12, '0');
    const deviceName = seedData.makeRunLabel('e2e_ui_device');
    let deviceId = '';

    try {
      await rolePage.goto('/device/manage?onboarding=first-device&add=manual', { waitUntil: 'domcontentloaded' });

      await rolePage.getByPlaceholder('Please input device name').fill(deviceName);
      // The manage-page search field also has a PID placeholder.  Scope the
      // assertion to the open add-device dialog and its real onboarding input.
      const addDialog = rolePage.getByRole('dialog');
      await addDialog.getByPlaceholder('Enter the 12-character device PID').fill(pid);

      const createResponse = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return response.request().method() === 'POST' && url.pathname.endsWith('/device');
      });
      await rolePage.getByRole('button', { name: /Save and Next/i }).click();

      const browserResp = await createResponse;
      expect(browserResp.status()).toBe(200);
      const browserBody = await browserResp.json();
      expect(browserBody.code).toBe(200);
      deviceId = seedData.pickId(browserBody.data);
      expect(deviceId).toEqual(expect.any(String));
      expect(browserResp.request().postDataJSON()).toEqual(
        expect.objectContaining({ name: deviceName, pid_number: pid })
      );

      const detailResp = await api.get('/device/detail/' + deviceId, {}, 'tenant_admin');
      expect(detailResp.code).toBe(200);
      expect(detailResp.data).toEqual(expect.objectContaining({ id: deviceId, name: deviceName }));

      const listResp = await api.get('/device', { page: 1, page_size: 100, name: deviceName }, 'tenant_admin');
      expect(listResp.code).toBe(200);
      expect(seedData.listFromResponse(listResp)).toEqual(
        expect.arrayContaining([expect.objectContaining({ id: deviceId, name: deviceName })])
      );

      await rolePage.goto('/device/manage', { waitUntil: 'domcontentloaded' });
      await rolePage.getByPlaceholder('Device Name').fill(deviceName);
      const filteredResponse = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return url.pathname.endsWith('/device') && url.searchParams.get('name') === deviceName;
      });
      await rolePage.getByRole('button', { name: 'Query' }).click();
      expect((await (await filteredResponse).json()).code).toBe(200);

      const deviceCard = rolePage.locator('.item-card').filter({ hasText: deviceName }).first();
      await expect(deviceCard).toBeVisible();
      await deviceCard.click();
      await expect(rolePage).toHaveURL(new RegExp('/device/details\\?d_id=' + deviceId));
    } finally {
      if (deviceId) {
        const deleteResp = await api.delete('/device/' + deviceId, {}, 'tenant_admin');
        expect(deleteResp.code).toBe(200);
      }
    }
  });

  test('created thing model is searchable in UI and matches API state', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const name = seedData.makeRunLabel('e2e_thing_model');
    const createResp = await api.post(
      '/device/template',
      { name, description: 'created by device browser E2E' },
      'tenant_admin'
    );
    expect(createResp.code).toBe(200);
    const templateId = seedData.pickId(createResp.data);
    expect(templateId).toEqual(expect.any(String));

    try {
      const listResp = await api.get('/device/template', { page: 1, page_size: 10, name }, 'tenant_admin');
      expect(listResp.code).toBe(200);
      expect(seedData.listFromResponse(listResp)).toEqual(
        expect.arrayContaining([expect.objectContaining({ id: templateId, name })])
      );

      await rolePage.goto('/device/thingsmodel', { waitUntil: 'domcontentloaded' });
      await expect(rolePage).toHaveURL(/\/device\/thingsmodel$/);
      await rolePage.getByPlaceholder('Enter Thing Model Name').fill(name);

      const filteredResponse = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return url.pathname.endsWith('/device/template') && url.searchParams.get('name') === name;
      });
      await rolePage.getByRole('button', { name: 'Search' }).click();
      const browserResp = await filteredResponse;
      expect(browserResp.status()).toBe(200);
      const browserBody = await browserResp.json();
      expect(browserBody.code).toBe(200);
      expect(seedData.listFromResponse(browserBody)).toEqual(
        expect.arrayContaining([expect.objectContaining({ id: templateId, name })])
      );
      await expect(rolePage.getByText(name).first()).toBeVisible();
    } finally {
      await api.delete('/device/template/' + templateId, {}, 'tenant_admin');
    }
  });

  test('created device group is searchable in UI and opens matching details', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const name = seedData.makeRunLabel('e2e_device_group');
    const createResp = await api.post('/device/group', { name }, 'tenant_admin');
    expect(createResp.code).toBe(200);

    const listResp = await api.get('/device/group', { page: 1, page_size: 20, name }, 'tenant_admin');
    expect(listResp.code).toBe(200);
    const group = seedData.listFromResponse(listResp).find(row => row && row.name === name);
    expect(group).toEqual(expect.objectContaining({ id: expect.any(String), name }));

    try {
      const detailResp = await api.get('/device/group/detail/' + group.id, {}, 'tenant_admin');
      expect(detailResp.code).toBe(200);
      expect(detailResp.data).toEqual(
        expect.objectContaining({ detail: expect.objectContaining({ id: group.id, name }) })
      );

      await rolePage.goto('/device/grouping', { waitUntil: 'domcontentloaded' });
      await expect(rolePage).toHaveURL(/\/device\/grouping$/);
      const searchInput = rolePage.getByPlaceholder('Please enter group name to search');
      const filteredResponse = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return url.pathname.endsWith('/device/group') && url.searchParams.get('name') === name;
      });
      await searchInput.fill(name);
      const browserResp = await filteredResponse;
      expect(browserResp.status()).toBe(200);
      const browserBody = await browserResp.json();
      expect(browserBody.code).toBe(200);
      expect(seedData.listFromResponse(browserBody)).toEqual(
        expect.arrayContaining([expect.objectContaining({ id: group.id, name })])
      );

      const groupRow = rolePage.getByRole('row').filter({ hasText: name.slice(0, 20) }).first();
      await expect(groupRow).toBeVisible();
      const browserDetailPromise = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return response.request().method() === 'GET'
          && response.status() === 200
          && url.pathname.endsWith('/device/group/detail/' + group.id);
      }, { timeout: 20000 });
      await groupRow.click();
      const browserDetail = await browserDetailPromise;
      const browserDetailBody = await browserDetail.json();
      expect(browserDetailBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({
          detail: expect.objectContaining({ id: group.id, name }),
          statistics: expect.objectContaining({
            device_total: expect.any(Number),
            online_total: expect.any(Number),
            offline_total: expect.any(Number),
            alarm_total: expect.any(Number)
          })
        })
      }));
      await expect(rolePage).toHaveURL(new RegExp('/device/grouping-details\\?id=' + group.id));
      await expect(rolePage.getByTestId('group-statistics-overview')).toBeVisible();
      await expect(rolePage.getByText(name, { exact: true }).first()).toBeVisible();
    } finally {
      await api.delete('/device/group/' + group.id, {}, 'tenant_admin');
    }
  });

  test('tenant admin service access stays empty when API denies plugin management', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const deniedResp = await api.get(
      '/service/list',
      { page: 1, page_size: 15, service_type: 2 },
      'tenant_admin'
    );
    expect(deniedResp.code).toBe(201001);
    expect(deniedResp.message).toContain('no permission to manage service plugins');

    const serviceResponse = rolePage.waitForResponse(response => {
      const url = new URL(response.url());
      return url.pathname.endsWith('/service/list') && url.searchParams.get('service_type') === '2';
    });
    await rolePage.goto('/device/service-access', { waitUntil: 'domcontentloaded' });
    const browserResp = await serviceResponse;
    expect(browserResp.status()).toBe(200);
    const browserBody = await browserResp.json();
    expect(browserBody).toEqual(expect.objectContaining({
      code: 201001,
      message: expect.stringContaining('no permission to manage service plugins')
    }));

    await expect(rolePage).toHaveURL(/\/device\/service-access$/);
    await expect(
      rolePage.getByText('No service access templates are available yet. Enable a service first, then create an access point for devices.')
    ).toBeVisible();
    await expect(rolePage.getByRole('button', { name: 'Open service catalog' })).toBeVisible();
  });

  test('share page distinguishes valid, invalid, and empty token states', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureRdiDevice('tenant_admin');
    let shareToken = '';

    try {
      const detailResp = await api.get('/device/detail/' + seed.id, {}, 'tenant_admin');
      expect(detailResp.code).toBe(200);
      const deviceName = detailResp.data.name;

      const shareResp = await api.post(
        '/rdi/devices/' + seed.id + '/share-token',
        { expires_in: 3600 },
        'tenant_admin'
      );
      expect(shareResp.code).toBe(200);
      shareToken = shareResp.data.token;

      // (a) Valid token: share page renders with device name visible
      const publicResp = await api.getNoAuth('/rdi/shared/' + encodeURIComponent(shareToken));
      expect(publicResp.code).toBe(200);
      expect(publicResp.data.device_id).toBe(seed.id);

      const validShareResponse = rolePage.waitForResponse(
        response => response.request().method() === 'POST' && response.url().includes('/rdi/share-tokens/') && response.url().endsWith('/accept'),
        { timeout: 20000 }
      );
      await rolePage.goto('/device/share?share_token=' + encodeURIComponent(shareToken), {
        waitUntil: 'domcontentloaded'
      });
      const validResponse = await validShareResponse;
      expect(validResponse.status()).toBe(200);
      const validBody = await validResponse.json();
      expect(validBody.code).toBe(200);
      await expect(rolePage.getByTestId('share-page')).toBeVisible();
      await expect(rolePage.getByTestId('share-success')).toBeVisible({ timeout: 15000 });
      await expect(rolePage.getByTestId('share-open-device')).toBeEnabled();

      // (b) Invalid token: share page renders error state
      const invalidToken = 'invalid-token-' + Date.now();
      const invalidPublicResp = await api.getNoAuth('/rdi/shared/' + encodeURIComponent(invalidToken));
      expect(invalidPublicResp.code).toBe(201001);

      const invalidShareResponse = rolePage.waitForResponse(
        response => response.request().method() === 'POST' && response.url().includes('/rdi/share-tokens/') && response.url().endsWith('/accept'),
        { timeout: 20000 }
      );
      await rolePage.goto('/device/share?share_token=' + encodeURIComponent(invalidToken), {
        waitUntil: 'domcontentloaded'
      });
      const invalidResponse = await invalidShareResponse;
      expect(invalidResponse.status()).toBe(200);
      const invalidBody = await invalidResponse.json();
      expect(invalidBody.code).toBe(201001);
      await expect(rolePage.getByTestId('share-error')).toBeVisible({ timeout: 15000 });

      // (c) Empty token: share page renders "Missing share token" empty state
      const emptyTokenAcceptRequests = [];
      rolePage.on('request', request => {
        if (request.method() === 'POST' && request.url().includes('/rdi/share-tokens/')) {
          emptyTokenAcceptRequests.push(request.url());
        }
      });
      await rolePage.goto('/device/share?share_token=', { waitUntil: 'domcontentloaded' });
      await expect(rolePage).toHaveURL(/\/device\/share/);
      await expect(rolePage.getByTestId('share-error')).toBeVisible();
      await expect(rolePage.getByText('Missing share token').first()).toBeVisible();
      await rolePage.getByTestId('share-retry').click();
      await expect(rolePage.getByTestId('share-error')).toBeVisible();
      await expect(rolePage.getByText('Missing share token').first()).toBeVisible();
      expect(emptyTokenAcceptRequests).toEqual([]);
    } finally {
      if (shareToken) {
        const revokeResp = await api.delete(
          '/rdi/devices/' + seed.id + '/share-tokens/' + encodeURIComponent(shareToken),
          {},
          'tenant_admin'
        );
        expect(revokeResp.code).toBe(200);
      }
      await seed.cleanup();
    }
  });

  test('Ready Check shows uniquely published MQTT telemetry evidence', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.createSimulationDevice('tenant_admin');
    const telemetryKey = 'codex_mqtt_e2e_' + Date.now();
    const telemetryValue = 'ready-check-' + Date.now();

    try {
      let telemetry;
      try {
        telemetry = await seedData.publishSimulatedTelemetryAndReadCurrent(
          seed.id,
          { [telemetryKey]: telemetryValue },
          'tenant_admin'
        );
      } catch (error) {
        const message = String(error && error.message || error);
        // The simulation endpoint publishes through the local MQTT broker. A broker
        // disconnect is surfaced by the API as a generic network Error/EOF without
        // repeating the MQTT port or protocol in the response message.
        const brokerUnavailable = /connect(?:ex|ion refused)|network error|closed network connection|connection reset|dial tcp|\bEOF\b/i.test(message);
        if (brokerUnavailable) {
          skipWhenBlocked(test, true, {
            category: 'runtime-external',
            seedable: false,
            reason: 'MQTT broker is unavailable for browser telemetry evidence: ' + message
          });
          return;
        }
        throw error;
      }
      expect(telemetry.rows).toEqual([
        expect.objectContaining({
          device_id: seed.id,
          key: telemetryKey,
          value: telemetryValue
        })
      ]);

      // 观察者必须在导航前注册（tab 挂载才发请求）；等待顺序对齐组件挂载
      // 顺序：先 /device/detail 门禁，再 ready-check tab 可见，最后证据响应。
      const browserDetailPromise = rolePage.waitForResponse(response => {
        try {
          const url = new URL(response.url());
          return response.request().method() === 'GET'
            && url.pathname.endsWith('/device/detail/' + seed.id);
        } catch {
          return false;
        }
      }, { timeout: 20000 });
      const evidenceResponses = Promise.all([
        rolePage.waitForResponse(
          response => response.url().includes('/device/' + seed.id + '/connection/diagnostics') && response.status() === 200,
          { timeout: 20000 }
        ),
        rolePage.waitForResponse(
          response => response.url().includes('/device/' + seed.id + '/onboarding/connection-guide') && response.status() === 200,
          { timeout: 20000 }
        )
      ]);

      await rolePage.goto(
        '/device/details?d_id=' + encodeURIComponent(seed.id) + '&tab=ready-check&onboarding=first-device',
        { waitUntil: 'domcontentloaded' }
      );

      // detail 业务失败时 visibleDetailComponents 为空、tab 不渲染，后续请求
      // 会等成超时；在此显式区分"读不一致空壳页"与"端点没回 200"。
      await test.step('device detail gate precedes ready-check mount', async () => {
        let browserDetail;
        try {
          browserDetail = await browserDetailPromise;
        } catch (error) {
          throw new Error('detail gate blocked (read-consistency): no /device/detail/' +
            seed.id + ' response; ' + String(error && error.message || error).split('\n')[0]);
        }
        const body = await browserDetail.json().catch(() => null);
        const receivedId = body && body.data && (body.data.id || body.data.ID);
        if (!body || body.code !== 200 || String(receivedId) !== String(seed.id)) {
          throw new Error('detail gate blocked (read-consistency): HTTP ' + browserDetail.status() +
            ' body=' + JSON.stringify(body).slice(0, 300) + ' device=' + seed.id);
        }
      });

      await expect(rolePage).toHaveURL(/\/device\/details\?d_id=.*tab=ready-check/);
      await expect(rolePage.getByTestId('device-ready-check')).toBeVisible({ timeout: 15000 });
      await evidenceResponses;
      const evidenceCenter = rolePage.getByTestId('device-ready-check-evidence-center');
      const loadEvidenceCenter = rolePage.getByTestId('device-ready-check-load-evidence-center');
      await expect
        .poll(
          async () => {
            if (await evidenceCenter.isVisible().catch(() => false)) return 'mounted';
            if (await loadEvidenceCenter.isVisible().catch(() => false)) return 'deferred';
            return 'pending';
          },
          { timeout: 15000 }
        )
        .toMatch(/mounted|deferred/);

      // The deferred placeholder and the real evidence center are mutually
      // exclusive. The viewport observer can mount the real center between the
      // poll and the click, detaching the placeholder button. Treat that
      // transition as success only when the real center is now visible; any
      // other click failure remains a real test failure.
      if (!(await evidenceCenter.isVisible().catch(() => false))) {
        await expect(loadEvidenceCenter).toBeVisible();
        await expect(loadEvidenceCenter).toBeEnabled();
        try {
          await loadEvidenceCenter.click({ timeout: 15000 });
        } catch (error) {
          if (!(await evidenceCenter.isVisible().catch(() => false))) throw error;
        }
      }
      await expect(evidenceCenter).toBeVisible({ timeout: 15000 });
      await expect(rolePage.getByTestId('device-ready-check-diagnostics')).toBeVisible();
      await expect(rolePage.getByTestId('device-ready-check-download-support-bundle')).toBeVisible();

      const downloadPromise = rolePage.waitForEvent('download');
      await rolePage.getByTestId('device-ready-check-download-support-bundle').click();
      const download = await downloadPromise;
      const filePath = await download.path();
      expect(filePath).toEqual(expect.any(String));

      const bundle = JSON.parse(fs.readFileSync(filePath, 'utf8'));
      expect(bundle).toEqual(
        expect.objectContaining({
          schema: 'aetherlink.ready-check.diagnostics.v1',
          device: expect.objectContaining({ id: seed.id }),
          telemetry: expect.objectContaining({
            latest: expect.stringContaining(telemetryKey),
            latestValue: telemetryValue,
            currentCount: expect.any(Number)
          })
        })
      );
      expect(bundle.telemetry.currentCount).toBeGreaterThan(0);
    } finally {
      await seed.cleanup();
    }
  });
});

test.describe('device share flow', () => {
  test('share link acceptance closes the loop when a recipient tenant is available', async ({ browser, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureRdiDevice('tenant_admin');
    const deviceId = seed.id;
    let shareToken = '';
    expect(deviceId).toEqual(expect.any(String));
    try {
      const shareResp = await api.post(
        '/rdi/devices/' + deviceId + '/share-token',
        { expires_in: 7 * 24 * 60 * 60 },
        'tenant_admin'
      );

      expect(shareResp.code).toBe(200);
      expect(shareResp.data).toEqual(expect.objectContaining({
        token: expect.stringMatching(/\S/),
        share_path: expect.stringContaining('/device/share?share_token=')
      }));
      shareToken = shareResp.data.token;
      expect(shareResp.data.share_path).toContain(encodeURIComponent(shareToken));

      const publicResp = await api.getNoAuth('/rdi/shared/' + shareToken);
      expect(publicResp.code).toBe(200);
      expect(publicResp.data.device_id).toBe(deviceId);

      const { context, accountKey, cleanupAccounts } = await createRecipientBrowserContext(browser, api);
      try {
        const page = await context.newPage();
        // This page belongs to a recipient context created inside the test,
        // so it does not pass through the rolePage fixture automatically.
        // Instrument it explicitly so the real shared-with-me navigation is
        // represented in the page-coverage artifact.
        instrumentPageCoverage(page);
        const acceptResponsePromise = page.waitForResponse(response =>
          response.request().method() === 'POST' &&
          response.url().includes('/rdi/share-tokens/' + encodeURIComponent(shareToken) + '/accept')
        );

        await page.goto('/device/share?share_token=' + encodeURIComponent(shareToken), {
          waitUntil: 'domcontentloaded'
        });

        const acceptResponse = await acceptResponsePromise;
        expect(acceptResponse.status()).toBe(200);
        const acceptBody = await acceptResponse.json();
        expect(acceptBody).toEqual(expect.objectContaining({
          code: 200,
          data: expect.objectContaining({
            device: expect.objectContaining({ device_id: deviceId }),
            shared_with_me: true
          })
        }));
        await expect(page.getByTestId('share-page')).toBeVisible();
        await expect(page.getByTestId('share-success')).toBeVisible({ timeout: 15000 });
        await expect(page.getByTestId('share-open-shared-with-me')).toBeVisible();

        const sharedListResp = await api.get(
          '/rdi/shared-with-me/devices',
          { page: 1, page_size: 20 },
          accountKey
        );
        expect(sharedListResp.code).toBe(200);
        const sharedRow = seedData.listFromResponse(sharedListResp).find(
          row => row && row.device && row.device.device_id === deviceId
        );
        expect(sharedRow).toEqual(expect.objectContaining({
          device: expect.objectContaining({
            device_id: deviceId,
            device_name: expect.stringMatching(/\S/)
          })
        }));

        const repeatAcceptPromise = page.waitForResponse(response =>
          response.request().method() === 'POST' &&
          response.url().includes('/rdi/share-tokens/' + encodeURIComponent(shareToken) + '/accept')
        );
        await page.getByTestId('share-refresh').click();
        const repeatAcceptBody = await (await repeatAcceptPromise).json();
        expect(repeatAcceptBody).toEqual(expect.objectContaining({
          code: 200,
          data: expect.objectContaining({
            device: expect.objectContaining({ device_id: deviceId }),
            already_accepted: true,
            shared_with_me: true
          })
        }));
        await expect(page.getByTestId('share-success')).toHaveAttribute('data-already-accepted', 'true');

        await page.getByTestId('share-open-shared-with-me').click();
        await expect(page).toHaveURL(/\/device\/shared-with-me/);
        await expect(page.getByTestId('shared-with-me-page')).toBeVisible();
        await expect(page.getByText(sharedRow.device.device_name, { exact: true }).first()).toBeVisible();
      } finally {
        await context.close();
        await cleanupDynamicAccounts(api.client, cleanupAccounts);
      }
    } finally {
      if (shareToken) {
        const revokeResp = await api.delete(
          '/rdi/devices/' + deviceId + '/share-tokens/' + encodeURIComponent(shareToken),
          {},
          'tenant_admin'
        );
        expect(revokeResp.code).toBe(200);
      }
      await seed.cleanup();
    }
  });
});
