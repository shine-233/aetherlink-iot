/**
 * Data and RDI visibility E2E evidence. Business cases use seeded records or
 * tenant-scoped API responses and verify that the browser renders the same state.
 */

const { test, expect } = require('./fixtures');
const fs = require('fs');
const seedData = require('../lib/seed_data');

function waitForReadyCheckEvidenceResponses(rolePage, deviceId) {
  return Promise.all([
    rolePage.waitForResponse(
      response => response.url().includes('/device/' + deviceId + '/connection/diagnostics') && response.status() === 200,
      { timeout: 20000 }
    ),
    rolePage.waitForResponse(
      response => response.url().includes('/device/' + deviceId + '/onboarding/connection-guide') && response.status() === 200,
      { timeout: 20000 }
    )
  ]);
}

function expectReadyCheckDeepLink(bundle, key, deviceId) {
  const link = bundle.deepLinks.find(item => item.key === key);
  expect(link).toEqual(
    expect.objectContaining({
      key,
      url: expect.stringContaining('/device/details')
    })
  );
  expect(link.url).toContain('d_id=' + encodeURIComponent(deviceId));
  return link;
}

function expectReadyCheckBackendStep(bundle, key) {
  const step = bundle.backendNextSteps.find(item => item.key === key);
  expect(step).toEqual(
    expect.objectContaining({
      key,
      title: expect.any(String),
      description: expect.any(String),
      status: expect.stringMatching(/^(done|warning|todo)$/)
    })
  );
  return step;
}

function pickId(row) {
  return row && (row.id || row.ID);
}

function listFromResponse(response) {
  const data = response && response.data;
  if (Array.isArray(data)) return data;
  if (data && Array.isArray(data.list)) return data.list;
  return [];
}

async function waitForTwinDesired(api, deviceId, twinKey) {
  // 10s 窗口：给主从读复制留出收敛时间，避免把可见性延迟误判为写丢失。
  const deadline = Date.now() + 10000;
  let lastResponse = null;
  while (Date.now() < deadline) {
    lastResponse = await api.get('/device/twin/' + deviceId, {}, 'tenant_admin');
    const rows = lastResponse && lastResponse.data && Array.isArray(lastResponse.data.rows)
      ? lastResponse.data.rows
      : [];
    if (lastResponse.code === 200 && rows.some(row => row && row.key === twinKey)) {
      return lastResponse;
    }
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  return lastResponse;
}

function isDeviceDetailResponse(response, deviceId) {
  try {
    const url = new URL(response.url());
    return response.request().method() === 'GET'
      // The dev Vite proxy uses /proxy-default while the preview proxy uses
      // /api/v1.  Match the backend resource suffix so the assertion follows
      // the actual transport without accepting an unrelated page response.
      && url.pathname.endsWith('/device/detail/' + deviceId);
  } catch {
    return false;
  }
}

// 区分两类快照失败：读到他人设备遥测（data 非空）vs 端点非 200。
function snapshotFailureSummary(body) {
  if (!body || body.code !== 200) {
    return 'non-200 telemetry snapshot: ' + JSON.stringify(body).slice(0, 300);
  }
  return 'received rows for this device: ' + JSON.stringify(body.data).slice(0, 300);
}

function requiredNumericField(payload, keys) {
  const key = keys.find(candidate => Object.prototype.hasOwnProperty.call(payload, candidate));
  expect(key).not.toBeUndefined();
  const value = Number(payload[key]);
  expect(Number.isFinite(value)).toBe(true);
  return value;
}

async function expectStatisticValue(page, label, value) {
  // Sidebar menu items reuse labels such as "Devices"; bind the assertion to
  // the rendered dashboard main region before walking to the statistic card.
  const labelNode = page.locator('main').getByText(label, { exact: true }).first();
  await expect(labelNode).toBeVisible({ timeout: 15000 });
  const statistic = labelNode.locator(
    'xpath=ancestor::*[contains(concat(" ", normalize-space(@class), " "), " n-statistic ")][1]'
  );
  await expect(statistic).toContainText(String(value));
}

test.describe('data and RDI visibility module', () => {
  test.use({ role: 'tenant_admin' });

  test('new device telemetry tab renders the same empty snapshot returned by the API', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.createSimulationDevice('tenant_admin');
    expect(seed.id).toEqual(expect.any(String));

    try {
      const persistedDetail = await api.get('/device/detail/' + seed.id, {}, 'tenant_admin');
      expect(persistedDetail.code).toBe(200);
      expect(persistedDetail.data).toEqual(expect.objectContaining({
        id: seed.id,
        name: expect.stringMatching(/\S/),
        device_number: expect.stringMatching(/\S/),
        is_online: expect.any(Number)
      }));
      const telemetryBefore = await api.get(
        '/telemetry/datas/current/' + seed.id,
        {},
        'tenant_admin'
      );
      expect(telemetryBefore, snapshotFailureSummary(telemetryBefore))
        .toEqual(expect.objectContaining({ code: 200, data: [] }));

      const browserDetailPromise = rolePage.waitForResponse(
        response => isDeviceDetailResponse(response, seed.id),
        { timeout: 20000 }
      );
      await rolePage.goto('/device/details?d_id=' + encodeURIComponent(seed.id), {
        waitUntil: 'domcontentloaded'
      });
      const browserDetail = await browserDetailPromise;
      expect(browserDetail.status()).toBe(200);
      const browserBody = await browserDetail.json();
      expect(browserBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({
          id: seed.id,
          name: persistedDetail.data.name,
          device_number: persistedDetail.data.device_number,
          is_online: persistedDetail.data.is_online
        })
      }));

      await expect(rolePage).toHaveURL(/\/device\/details\?d_id=/);
      await expect(rolePage.locator('.device-details-title')).toHaveText(persistedDetail.data.name);
      await expect(rolePage.locator('.device-details-meta')).toContainText(seed.id);
      await expect(
        rolePage.getByText(persistedDetail.data.is_online === 1 ? /Online/i : /Offline/i).first()
      ).toBeVisible();

      const telemetryTab = rolePage
        .locator('.device-details-tabs .n-tabs-tab')
        .filter({ hasText: /^(Telemetry|遥测)$/i })
        .first();
      await expect(telemetryTab).toBeVisible({ timeout: 15000 });
      const telemetryResponsePromise = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return response.request().method() === 'GET'
          && url.pathname.endsWith('/telemetry/datas/current/' + seed.id);
      }, { timeout: 20000 });
      await telemetryTab.click();
      const telemetryResponse = await telemetryResponsePromise;
      expect(telemetryResponse.status()).toBe(200);
      const telemetryBody = await telemetryResponse.json();
      expect(telemetryBody, snapshotFailureSummary(telemetryBody))
        .toEqual(expect.objectContaining({ code: 200, data: [] }));
      // Naive UI marks the active tab with its class/data-name rather than
      // aria-selected; assert the rendered state that the user actually sees.
      await expect(telemetryTab).toHaveClass(/n-tabs-tab--active/);
      await expect(rolePage).toHaveURL(/[?&]tab=telemetry(?:&|$)/);
      await expect(rolePage.locator('.device-details-tabs .n-tab-pane:visible')).toHaveCount(1);
      await expect(
        rolePage.getByText(/No telemetry has been reported yet|设备还没有上报遥测/)
      ).toBeVisible();
    } finally {
      await seed.cleanup();
    }
  });

  test('device twin tab downloads platform-visible evidence bundle for a seeded desired state', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    // The shared RDI seed is intentionally reduced to the four customer tabs;
    // use a non-RDI device so the twin workbench is a real visible capability.
    const seed = await seedData.createSimulationDevice('tenant_admin');
    const twinKey = 'e2e_twin_' + Date.now();
    let desiredId = null;

    try {
      const desiredResp = await api.put('/device/twin/' + seed.id + '/desired', {
        source: 'telemetry',
        key: twinKey,
        desired: { target: 'browser-evidence', run: twinKey }
      }, 'tenant_admin');
      expect(desiredResp.code).toBe(200);
      desiredId = desiredResp.data && (desiredResp.data.id || desiredResp.data.ID);
      expect(desiredId).toEqual(expect.any(String));

      // The write endpoint is synchronous, but keep an explicit read-after-
      // write oracle so a transient database visibility gap is reported at
      // the API boundary instead of being misread as a browser rendering bug.
      const seededTwinState = await waitForTwinDesired(api, seed.id, twinKey);
      expect(seededTwinState.code).toBe(200);
      expect(seededTwinState.data.rows).toEqual(
        expect.arrayContaining([expect.objectContaining({ key: twinKey, label: twinKey })])
      );

      const twinResponsePromise = rolePage.waitForResponse(response => {
        try {
          const url = new URL(response.url());
          return response.request().method() === 'GET'
            && url.pathname.endsWith('/device/twin/' + seed.id);
        } catch {
          return false;
        }
      }, { timeout: 20000 });
      await rolePage.goto(
        '/device/details?d_id=' + encodeURIComponent(seed.id) + '&tab=device-twin',
        { waitUntil: 'domcontentloaded' }
      );

      const twinResponse = await twinResponsePromise;
      expect(twinResponse.status()).toBe(200);
      const twinBody = await twinResponse.json();
      expect(twinBody).toEqual(expect.objectContaining({ code: 200, data: expect.any(Object) }));
      expect(twinBody.data.rows).toEqual(
        expect.arrayContaining([expect.objectContaining({ key: twinKey, label: twinKey })])
      );

      await expect(rolePage).toHaveURL(/\/device\/details\?d_id=.*tab=device-twin/);
      await expect(rolePage.getByTestId('device-twin-confirmation')).toBeVisible({
        timeout: 15000
      });
      await expect(rolePage.getByTestId('device-twin-download-evidence-bundle')).toBeVisible();

      const downloadPromise = rolePage.waitForEvent('download');
      await rolePage.getByTestId('device-twin-download-evidence-bundle').click();
      const download = await downloadPromise;
      const filePath = await download.path();
      expect(download.suggestedFilename()).toMatch(/^device-twin-evidence-[a-zA-Z0-9_-]+-\d{4}-\d{2}-\d{2}T.*\.json$/);
      expect(filePath).toEqual(expect.any(String));

      const bundle = JSON.parse(fs.readFileSync(filePath, 'utf8'));
      expect(bundle).toEqual(expect.objectContaining({
        schema_version: 'twin-lite-evidence-v1',
        device_id: seed.id,
        status: expect.any(String),
        next_action: expect.any(String),
        evidence_boundary: expect.any(String),
        scope: expect.objectContaining({
          source: 'device-twin-workbench',
          platform_visible_evidence_only: true
        }),
        summary: expect.objectContaining({
          desiredCount: expect.any(Number),
          reportedCount: expect.any(Number),
          matchedCount: expect.any(Number),
          deltaCount: expect.any(Number),
          unavailableCount: expect.any(Number)
        }),
        rows: expect.any(Array)
      }));
      expect(bundle.rows).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            key: twinKey,
            label: twinKey,
            source: 'telemetry',
            desired: expect.objectContaining({
              target: 'browser-evidence',
              run: twinKey
            }),
            comparable: true,
            status: 'pending'
          })
        ])
      );
    } finally {
      if (desiredId) {
        await api.delete('/expected/data/' + desiredId, {}, 'tenant_admin');
      }
      await seed.cleanup();
    }
  });

  test('first-device Ready Check downloads diagnostics bundle with connection-guide evidence', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    // The existing fixture device is an RDI device in this database and is
    // intentionally reduced to the four customer tabs. Ready Check is a
    // general device capability, so create a non-RDI simulation device for
    // this browser workflow.
    const seed = await seedData.createSimulationDevice('tenant_admin');
    expect(seed.id).toEqual(expect.any(String));

    try {
      // The Ready Check collector runs on component mount; the evidence
      // center itself is deferred, so register the network observers before
      // navigation rather than after the requests have already completed.
      const evidenceResponses = waitForReadyCheckEvidenceResponses(rolePage, seed.id);
      await rolePage.goto(
        '/device/details?d_id=' + encodeURIComponent(seed.id) + '&tab=ready-check&onboarding=first-device',
        { waitUntil: 'domcontentloaded' }
      );

      await expect(rolePage).toHaveURL(/\/device\/details\?d_id=.*tab=ready-check/);
      await expect(rolePage.getByTestId('device-ready-check')).toBeVisible({ timeout: 15000 });
      await evidenceResponses;
      // The detailed evidence center is intentionally deferred until the user asks for it.
      // Exercise that visible workflow before asserting controls inside the deferred child.
      // Scrolling the placeholder into view can itself trigger IntersectionObserver and replace
      // the button, so dispatch the click on the currently mounted node without auto-scrolling.
      await rolePage
        .getByRole('button', { name: 'Load evidence center', exact: true })
        .evaluate(button => button.click());
      await expect(rolePage.getByTestId('device-ready-check-diagnostics')).toBeVisible();
      await expect(rolePage.getByTestId('device-ready-check-download-support-bundle')).toBeVisible();

      const downloadPromise = rolePage.waitForEvent('download');
      await rolePage.getByTestId('device-ready-check-download-support-bundle').click();
      const download = await downloadPromise;
      const filePath = await download.path();
      expect(download.suggestedFilename()).toMatch(/^aetherlink-ready-check-[a-zA-Z0-9._-]+-diagnostics\.json$/);
      expect(filePath).toEqual(expect.any(String));

      const bundle = JSON.parse(fs.readFileSync(filePath, 'utf8'));
      expect(bundle).toEqual(
        expect.objectContaining({
          schema: 'aetherlink.ready-check.diagnostics.v1',
          device: expect.objectContaining({
            id: seed.id
          }),
          source: expect.objectContaining({
            sourceKey: 'home_first_device_onboarding',
            firstDeviceOnboarding: true
          }),
          readiness: expect.objectContaining({
            level: expect.any(String),
            code: expect.any(String),
            summary: expect.any(String),
            evaluatedAt: expect.any(String),
            connectionGuideSummary: expect.any(String)
          }),
          telemetry: expect.objectContaining({
            latest: expect.any(String),
            latestValue: expect.any(String)
          }),
          diagnostics: expect.objectContaining({
            nextActions: expect.any(Array),
            debug: expect.any(Object),
            recentFailures: expect.any(Array),
            partialWarnings: expect.any(Array)
          }),
          evidenceCards: expect.any(Array),
          backendNextSteps: expect.any(Array),
          deepLinks: expect.any(Array),
          collectionFailures: expect.any(Array),
          evidenceBoundary: expect.any(String)
        })
      );
      expect(Object.prototype.hasOwnProperty.call(bundle.readiness, 'ready')).toBe(true);
      expect(typeof bundle.telemetry.currentCount === 'number' || bundle.telemetry.currentCount === null).toBe(true);

      expectReadyCheckDeepLink(bundle, 'telemetry', seed.id);
      expectReadyCheckDeepLink(bundle, 'device-twin', seed.id);
      expect(bundle.evidenceCards).toEqual(
        expect.arrayContaining([
          expect.objectContaining({ key: 'twin', boundary: expect.any(String) }),
          expect.objectContaining({ key: 'command', boundary: expect.any(String) })
        ])
      );
      ['credentials', 'publish_telemetry', 'ready_check', 'control_loop'].forEach(key => {
        expectReadyCheckBackendStep(bundle, key);
      });
    } finally {
      await seed.cleanup();
    }
  });

  test('seeded device config detail renders its API identity and loads data processing for that config', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureDeviceConfig('tenant_admin');
    const scriptName = 'e2e_data_script_' + Date.now();
    const scriptDescription = 'browser-visible data processing seed for ' + seed.id;
    let scriptId = null;
    expect(seed.id).toEqual(expect.any(String));

    try {
      const createScriptResp = await api.post('/data_script', {
        name: scriptName,
        device_config_id: seed.id,
        description: scriptDescription,
        content: 'function encodeInp(msg, topic) return msg end',
        script_type: 'A',
        last_analog_input: 'e2e-input'
      }, 'tenant_admin');
      expect(createScriptResp.code).toBe(200);
      scriptId = pickId(createScriptResp.data);
      expect(scriptId).toEqual(expect.any(String));

      const detailResp = await api.get('/device_config/' + seed.id, {}, 'tenant_admin');
      expect(detailResp.code).toBe(200);
      expect(pickId(detailResp.data)).toBe(seed.id);
      expect(detailResp.data.name).toEqual(expect.any(String));

      const detailLoadPromise = rolePage.waitForResponse(
        response =>
          response.request().method() === 'GET' &&
          response.url().includes('/device_config/' + seed.id) &&
          response.status() === 200,
        { timeout: 20000 }
      );
      await rolePage.goto('/device/config-detail?id=' + encodeURIComponent(seed.id), {
        waitUntil: 'domcontentloaded'
      });
      const detailLoad = await detailLoadPromise;
      const loadedDetail = await detailLoad.json();

      expect(loadedDetail.code).toBe(200);
      expect(pickId(loadedDetail.data)).toBe(seed.id);
      expect(loadedDetail.data.name).toBe(detailResp.data.name);
      await expect(rolePage).toHaveURL(/\/device\/config-detail\?id=/);
      await expect(rolePage.getByText(detailResp.data.name, { exact: true }).first()).toBeVisible({ timeout: 15000 });

      const dataScriptLoadPromise = rolePage.waitForResponse(
        response => {
          if (response.request().method() !== 'GET' || response.status() !== 200) return false;
          const url = new URL(response.url());
          return url.pathname.endsWith('/data_script') && url.searchParams.get('device_config_id') === seed.id;
        },
        { timeout: 20000 }
      );
      // Naive UI renders the tab label as a clickable generic node in this route.
      // Target the visible label so the lazy DataHandle child actually mounts.
      await rolePage.getByText('Data Processing', { exact: true }).click();
      const dataScriptLoad = await dataScriptLoadPromise;
      const dataScriptPayload = await dataScriptLoad.json();

      expect(dataScriptPayload.code).toBe(200);
      expect(dataScriptPayload.data).toEqual(
        expect.objectContaining({
          list: expect.any(Array),
          total: expect.any(Number)
        })
      );
      expect(dataScriptPayload.data.list).toEqual(expect.arrayContaining([
        expect.objectContaining({
          id: scriptId,
          name: scriptName,
          description: scriptDescription,
          device_config_id: seed.id,
          script_type: 'A'
        })
      ]));
      await expect(rolePage.getByText(scriptName, { exact: true })).toBeVisible();
      await expect(rolePage.getByText(scriptDescription, { exact: true })).toBeVisible();
      await expect(rolePage.getByRole('button', { name: /^Add Data Processing$/ }).first()).toBeVisible();
    } finally {
      if (scriptId) {
        const deleteScriptResp = await api.delete('/data_script/' + scriptId, {}, 'tenant_admin');
        expect(deleteScriptResp.code).toBe(200);
      }
      await seed.cleanup();
    }
  });

  test('seeded thing model is searchable in the UI and matches list and detail APIs', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const name = 'e2e_thing_model_' + Date.now();
    const description = 'browser and API consistency evidence for ' + name;
    let templateId = null;

    try {
      const createResp = await api.post('/device/template', {
        name,
        description,
        author: 'automation',
        version: '1.0.0',
        label: 'e2e,data'
      }, 'tenant_admin');
      expect(createResp.code).toBe(200);
      templateId = pickId(createResp.data);
      expect(templateId).toEqual(expect.any(String));

      await rolePage.goto('/device/thingsmodel', { waitUntil: 'domcontentloaded' });
      const searchResponsePromise = rolePage.waitForResponse(
        response => {
          if (response.request().method() !== 'GET' || response.status() !== 200) return false;
          const url = new URL(response.url());
          return url.pathname.endsWith('/device/template') && url.searchParams.get('name') === name;
        },
        { timeout: 20000 }
      );
      await rolePage.getByPlaceholder('Enter Thing Model name').fill(name);
      await rolePage.getByRole('button', { name: 'Search' }).click();
      const searchResponse = await searchResponsePromise;
      const searchPayload = await searchResponse.json();

      expect(searchPayload.code).toBe(200);
      const browserRows = listFromResponse(searchPayload);
      expect(browserRows).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            id: templateId,
            name,
            description
          })
        ])
      );
      await expect(rolePage.getByText(name, { exact: true }).first()).toBeVisible({ timeout: 15000 });
      await expect(rolePage.getByText(description, { exact: true }).first()).toBeVisible();

      const listResp = await api.get('/device/template', { page: 1, page_size: 10, name }, 'tenant_admin');
      expect(listResp.code).toBe(200);
      const apiRow = listFromResponse(listResp).find(row => pickId(row) === templateId);
      expect(apiRow).toEqual(expect.objectContaining({ name, description }));

      const detailResp = await api.get('/device/template/detail/' + templateId, {}, 'tenant_admin');
      expect(detailResp.code).toBe(200);
      expect(detailResp.data).toEqual(expect.objectContaining({ id: templateId, name, description }));

      const deletedTemplateId = templateId;
      const deleteResp = await api.delete('/device/template/' + deletedTemplateId, {}, 'tenant_admin');
      expect(deleteResp.code).toBe(200);
      templateId = null;

      const afterDeleteResp = await api.get('/device/template', { page: 1, page_size: 10, name }, 'tenant_admin');
      expect(afterDeleteResp.code).toBe(200);
      expect(listFromResponse(afterDeleteResp).some(row => pickId(row) === deletedTemplateId)).toBe(false);
    } finally {
      if (templateId) {
        const deleteResp = await api.delete('/device/template/' + templateId, {}, 'tenant_admin');
        expect(deleteResp.code).toBe(200);
      }
    }
  });

  test('share link shows owner success and revoked-token error states backed by the API', async ({ rolePage, api }) => {
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
        device_id: deviceId,
        token: expect.stringMatching(/\S/),
        share_path: expect.stringContaining('/device/share?share_token='),
        expires_at: expect.any(Number)
      }));
      shareToken = shareResp.data.token;
      expect(shareResp.data.share_path).toContain(encodeURIComponent(shareToken));

      const publicResp = await api.getNoAuth('/rdi/shared/' + encodeURIComponent(shareToken));
      expect(publicResp.code).toBe(200);
      expect(publicResp.data.device_id).toBe(deviceId);
      expect(typeof publicResp.data.config).toBe('object');
      expect(typeof publicResp.data.system_info).toBe('object');
      expect(typeof publicResp.data.thing_model).toBe('object');

      const acceptPath = '/rdi/share-tokens/' + encodeURIComponent(shareToken) + '/accept';
      const ownerAcceptPromise = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return response.request().method() === 'POST' && url.pathname.endsWith(acceptPath);
      }, { timeout: 20000 });
      await rolePage.goto(shareResp.data.share_path, { waitUntil: 'domcontentloaded' });
      const ownerAcceptResponse = await ownerAcceptPromise;
      expect(ownerAcceptResponse.status()).toBe(200);
      const ownerAcceptBody = await ownerAcceptResponse.json();
      expect(ownerAcceptBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({
          device: expect.objectContaining({ device_id: deviceId }),
          already_accepted: true,
          shared_with_me: false
        })
      }));
      await expect(rolePage).toHaveURL(new RegExp('/device/share\\?share_token='));
      await expect(rolePage.getByTestId('share-page')).toBeVisible();
      await expect(rolePage.getByTestId('share-success')).toHaveAttribute('data-already-accepted', 'true');
      await expect(rolePage.getByTestId('share-open-device')).toBeEnabled();
      await expect(rolePage.getByTestId('share-open-shared-with-me')).toHaveCount(0);

      const revokeResp = await api.delete(
        '/rdi/devices/' + deviceId + '/share-tokens/' + encodeURIComponent(shareToken),
        {},
        'tenant_admin'
      );
      expect(revokeResp.code).toBe(200);
      expect(revokeResp.data).toEqual(expect.objectContaining({
        device_id: deviceId,
        revoked_tokens: 1,
        revoked_recipients: 0,
        revoked_at: expect.any(Number)
      }));

      const revokedAcceptPromise = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return response.request().method() === 'POST' && url.pathname.endsWith(acceptPath);
      }, { timeout: 20000 });
      await rolePage.getByTestId('share-refresh').click();
      const revokedAcceptResponse = await revokedAcceptPromise;
      expect(revokedAcceptResponse.status()).toBe(200);
      const revokedAcceptBody = await revokedAcceptResponse.json();
      expect(revokedAcceptBody).toEqual(expect.objectContaining({
        code: 201001,
        message: expect.stringMatching(/invalid|expired|permission/i)
      }));
      await expect(rolePage.getByTestId('share-error')).toBeVisible({ timeout: 15000 });
      await expect(rolePage.getByTestId('share-retry')).toBeVisible();
      await expect(rolePage.getByTestId('share-success')).toHaveCount(0);

      const revokedToken = shareToken;
      shareToken = '';
      const revokedPublicResp = await api.getNoAuth('/rdi/shared/' + encodeURIComponent(revokedToken));
      expect(revokedPublicResp).toEqual(expect.objectContaining({
        code: 201001,
        message: expect.stringMatching(/invalid|expired|permission/i)
      }));
    } finally {
      if (shareToken) {
        await api.delete(
          '/rdi/devices/' + deviceId + '/share-tokens/' + encodeURIComponent(shareToken),
          {},
          'tenant_admin'
        );
      }
      await seed.cleanup();
    }
  });

  test('tenant RDI alarm overview matches its tenant-scoped API responses without requesting all tenants', async ({ rolePage }) => {
    const boardResponsePromise = rolePage.waitForResponse(
      response =>
        response.request().method() === 'GET' &&
        response.url().includes('/board/tenant/device/info') &&
        response.status() === 200,
      { timeout: 20000 }
    );
    const alarmCountResponsePromise = rolePage.waitForResponse(
      response =>
        response.request().method() === 'GET' &&
        response.url().includes('/alarm/device/counts') &&
        response.status() === 200,
      { timeout: 20000 }
    );
    const activeSystemsResponsePromise = rolePage.waitForResponse(
      response => {
        if (response.request().method() !== 'GET' || response.status() !== 200) return false;
        const url = new URL(response.url());
        return url.pathname.endsWith('/device') && url.searchParams.get('warn_status') === 'Y';
      },
      { timeout: 20000 }
    );

    await rolePage.goto('/alarm/rdi-overview', { waitUntil: 'domcontentloaded' });
    const [boardResponse, alarmCountResponse, activeSystemsResponse] = await Promise.all([
      boardResponsePromise,
      alarmCountResponsePromise,
      activeSystemsResponsePromise
    ]);
    const boardPayload = await boardResponse.json();
    const alarmCountPayload = await alarmCountResponse.json();
    const activeSystemsPayload = await activeSystemsResponse.json();

    expect(boardPayload.code).toBe(200);
    expect(alarmCountPayload.code).toBe(200);
    expect(activeSystemsPayload.code).toBe(200);
    [boardResponse, alarmCountResponse, activeSystemsResponse].forEach(response => {
      expect(new URL(response.url()).searchParams.has('all_tenants')).toBe(false);
    });

    const board = boardPayload.data || {};
    const counts = alarmCountPayload.data || {};
    const totalDevices = requiredNumericField(board, ['device_total', 'DeviceTotal']);
    const onlineDevices = requiredNumericField(board, ['device_on', 'DeviceOn']);
    const offlineDevices = Number(
      board.device_offline ?? board.DeviceOffline ?? Math.max(totalDevices - onlineDevices, 0)
    );
    expect(Number.isFinite(offlineDevices)).toBe(true);
    const alarmDevices = requiredNumericField(counts, ['alarm_device_total', 'AlarmDeviceTotal']);
    const alarmHistoryTotal = requiredNumericField(counts, ['alarm_history_total', 'AlarmHistoryTotal']);
    const activeRows = listFromResponse(activeSystemsPayload);
    expect(activeSystemsPayload.data).toEqual(expect.objectContaining({
      total: expect.any(Number),
      list: expect.any(Array)
    }));

    await expect(rolePage.getByText('RDI Overview', { exact: true }).first()).toBeVisible({ timeout: 15000 });
    await expectStatisticValue(rolePage, 'Devices', totalDevices);
    await expectStatisticValue(rolePage, 'Online', onlineDevices);
    await expectStatisticValue(rolePage, 'Offline', offlineDevices);
    await expectStatisticValue(rolePage, 'Alarm devices', alarmDevices);
    await expectStatisticValue(rolePage, 'Alarm history records', alarmHistoryTotal);
    await expect(rolePage.getByText('Systems with active alerts', { exact: true }).first()).toBeVisible();

    const snapshotCards = rolePage.locator('.snapshot-card');
    await expect(snapshotCards).toHaveCount(activeRows.length, { timeout: 30000 });
    if (activeRows.length === 0) {
      await expect(rolePage.locator('.snapshot-empty')).toBeVisible();
    } else {
      for (const row of activeRows.slice(0, 3)) {
        const rowId = pickId(row);
        expect(rowId).toEqual(expect.any(String));
        const rowName = row.name || row.device_name || row.DeviceName || rowId;
        await expect(snapshotCards.filter({ hasText: String(rowName) }).first()).toBeVisible();
      }
    }
  });
});
