/**
 * Ready Check to Command Center browser evidence.
 *
 * This is business evidence only when it runs against a live seeded backend:
 * the browser opens Command Center with the same route draft that Ready Check
 * builds, then previews and submits that draft through the browser before
 * verifying the persisted job through the API.
 */

const { test, expect } = require('./fixtures');
const seedData = require('../lib/seed_data');
const { skipWhenBlocked } = require('../lib/integration_blocked');

function recommendedCommandFromResponse(response) {
  const commands = response && Array.isArray(response.data) ? response.data : [];
  const command = commands.find(row => String(row?.data_identifier || '').trim());
  if (!command) return null;
  const rawValue = command.params;
  const normalizedValue = rawValue === undefined || rawValue === null
    ? ''
    : (typeof rawValue === 'string' ? rawValue.trim() : JSON.stringify(rawValue));
  return {
    identify: command.data_identifier.trim(),
    value: normalizedValue.length <= 4000 ? normalizedValue : ''
  };
}

function isRecommendedCommandResponse(response, deviceId) {
  const url = new URL(response.url());
  return response.request().method() === 'GET'
    && url.pathname.includes('/command/datas/' + deviceId);
}

test.describe('ready check command draft handoff [21_ready_check_command_draft]', () => {
  test.use({ role: 'tenant_admin' });

  test('Ready Check route draft previews, submits, and persists the same command job', async ({ rolePage, api }, testInfo) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureReadyCheckCommandFixture('tenant_admin');
    if (seed.blocked) {
      skipWhenBlocked(testInfo, true, {
        reason: seed.reason,
        category: 'runtime-external',
        seedable: false
      });
      return;
    }
    expect(String(seed.id || '').trim()).not.toBe('');

    try {
      const commandApiResp = await api.get('/command/datas/' + seed.id, {}, 'tenant_admin');
      expect(commandApiResp.code).toBe(200);
      const commandDraft = recommendedCommandFromResponse(commandApiResp);
      expect(commandDraft, 'the real Ready Check fixture must expose a command draft').not.toBeNull();
      const commandIdentify = commandDraft.identify;
      const commandValue = commandDraft.value;

      const browserCommandPromise = rolePage.waitForResponse(
        response => isRecommendedCommandResponse(response, seed.id),
        { timeout: 20000 }
      );
      await rolePage.goto('/device/details?d_id=' + encodeURIComponent(seed.id) + '&tab=ready-check', {
        waitUntil: 'domcontentloaded'
      });
      const browserCommandResp = await browserCommandPromise;
      expect(browserCommandResp.status()).toBe(200);
      const browserCommandBody = await browserCommandResp.json();
      expect(browserCommandBody.code).toBe(200);
      expect(browserCommandBody.data).toEqual(
        expect.arrayContaining([expect.objectContaining({ data_identifier: commandIdentify })])
      );

      const draftPanel = rolePage.getByTestId('device-ready-check-command-draft');
      await expect(draftPanel).toBeVisible({ timeout: 20000 });
      await expect(draftPanel).toContainText(commandIdentify);
      await Promise.all([
        rolePage.waitForURL(url => url.pathname === '/device/command-center'),
        draftPanel.getByRole('button').click()
      ]);

      await expect(rolePage).toHaveURL(/\/device\/command-center/);
      await expect(rolePage.getByText(/Draft loaded from Ready Check|已从 Ready Check 载入草稿/i).first()).toBeVisible({
        timeout: 15000
      });
      await expect(rolePage.getByPlaceholder('Command identifier', { exact: true })).toHaveValue(commandIdentify);
      await expect(rolePage.getByPlaceholder('Command value or JSON params, optional', { exact: true })).toHaveValue(commandValue);

      const commandCenterRoute = new URL(rolePage.url());
      expect(commandCenterRoute.searchParams.get('device_ids')).toBe(seed.id);
      expect(commandCenterRoute.searchParams.get('fleet_source')).toBe('device_details');
      expect(commandCenterRoute.searchParams.get('fleet_scope')).toBe('single_device');
      expect(commandCenterRoute.searchParams.get('command_source')).toBe('ready_check');
      expect(commandCenterRoute.searchParams.get('command_identify')).toBe(commandIdentify);
      expect(commandCenterRoute.searchParams.get('command_value')).toBe(commandValue);
      expect(commandCenterRoute.searchParams.get('timeout_seconds')).toBe('60');

      const submitButton = rolePage.getByRole('button', {
        name: /Submit eligible devices|提交可执行设备|Enviar dispositivos elegibles|Envoyer les appareils eligibles/i
      }).first();
      await expect(submitButton).toBeDisabled();

      const previewButton = rolePage.getByRole('button', {
        name: /Preview loaded draft|预览载入的草稿|Previsualizar borrador|Previsualiser le brouillon/i
      });
      await expect(previewButton).toBeVisible();
      const previewResponse = rolePage.waitForResponse(response =>
        response.request().method() === 'POST' && response.url().includes('/command/datas/jobs/preview')
      );
      await previewButton.click();
      const previewBrowserResp = await previewResponse;
      expect(previewBrowserResp.status()).toBe(200);
      const previewBody = await previewBrowserResp.json();
      expect(previewBody.code).toBe(200);
      expect(previewBody.data.preview_token).toEqual(expect.any(String));
      expect(previewBrowserResp.request().postDataJSON()).toEqual(
        expect.objectContaining({
          device_ids: [seed.id],
          identify: commandIdentify,
          value: commandValue,
          timeout_seconds: 60
        })
      );

      await expect(submitButton).toBeEnabled();
      const submitResponse = rolePage.waitForResponse(response =>
        response.request().method() === 'POST' && response.url().includes('/command/datas/jobs/submit')
      );
      await submitButton.click();
      const submitBrowserResp = await submitResponse;
      expect(submitBrowserResp.status()).toBe(200);
      const submitBody = await submitBrowserResp.json();
      expect(submitBody.code).toBe(200);
      const jobId = submitBody.data.job_id;
      expect(jobId).toEqual(expect.any(String));
      expect(submitBrowserResp.request().postDataJSON()).toEqual(
        expect.objectContaining({
          preview_token: previewBody.data.preview_token,
          identify: commandIdentify,
          value: commandValue
        })
      );

      await expect(rolePage.getByText(jobId).first()).toBeVisible({ timeout: 15000 });
      const detailResp = await api.get('/command/datas/jobs/' + jobId, { include_rows: true }, 'tenant_admin');
      expect(detailResp.code).toBe(200);
      expect(detailResp.data).toEqual(expect.objectContaining({
        job_id: jobId,
        identify: commandIdentify
      }));
      expect(detailResp.data.rows).toEqual(
        expect.arrayContaining([expect.objectContaining({ device_id: seed.id })])
      );

      const historyResp = await api.get(
        '/command/datas/jobs',
        { page: 1, page_size: 20, search: jobId },
        'tenant_admin'
      );
      expect(historyResp.code).toBe(200);
      expect(seedData.listFromResponse(historyResp)).toEqual(
        expect.arrayContaining([expect.objectContaining({
          job_id: jobId,
          identify: commandIdentify,
          command_value: commandValue
        })])
      );
    } finally {
      await seed.cleanup();
    }
  });
});
