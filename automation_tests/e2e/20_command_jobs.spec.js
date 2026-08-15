/**
 * Command Jobs browser evidence.
 *
 * This is business evidence only when it runs against a live seeded backend:
 * the API creates a real selected-device job, then the browser must show the
 * submitted job, result summary, evidence rows, refresh, and support preview.
 */

const { test, expect } = require('./fixtures');
const fs = require('fs');
const seedData = require('../lib/seed_data');
const testData = require('../lib/test_data');
const { skipWhenBlocked } = require('../lib/integration_blocked');

async function mqttBrokerAvailable() {
  return seedData.isMqttBrokerAvailable();
}

const failureCommandIdentify = 'e2e_forced_failure';

function buildCommandJobPayload(deviceId, extra = {}) {
  const subsetLimit = 10;
  return {
    scope_type: 'selected_devices',
    device_ids: [deviceId],
    identify: 'test_dry_contact',
    value: JSON.stringify(testData.getTestDryContactParams()),
    timeout_seconds: 60,
    subset_limit: subsetLimit,
    sample_limit: subsetLimit,
    ...extra
  };
}

async function createSelectedDeviceCommandJob(api, extra = {}, testInfo) {
  if (!testInfo || typeof testInfo.skip !== 'function') {
    throw new Error('createSelectedDeviceCommandJob requires Playwright testInfo for blocked fixture handling');
  }
  await api.login('tenant_admin');
  const commandIdentify = String(extra.identify || 'test_dry_contact').trim();
  const seed = await seedData.ensureReadyCheckCommandFixture('tenant_admin', {
    commandIdentify,
    failureIdentify: commandIdentify === failureCommandIdentify ? failureCommandIdentify : ''
  });
  if (seed.blocked) {
    skipWhenBlocked(testInfo, true, {
      reason: seed.reason,
      category: 'runtime-external',
      seedable: false
    });
    return;
  }
  expect(String(seed.id || '').trim()).not.toBe('');

  const payload = buildCommandJobPayload(seed.id, extra);

  const previewResp = await api.post(
    '/command/datas/jobs/preview',
    payload,
    'tenant_admin'
  );
  expect(previewResp.code).toBe(200);
  expect(typeof previewResp.data.preview_token).toBe('string');
  expect(previewResp.data.preview_token).not.toBe('');
  expect(previewResp.data.rows.some(row => row && row.device_id === seed.id)).toBe(true);
  if (Number(previewResp.data.eligible_count || 0) <= 0) {
    await seed.cleanup();
    skipWhenBlocked(testInfo, true, {
      reason: 'seeded device is offline or otherwise ineligible; real command dispatch requires an online device or MQTT emulator',
      category: 'runtime-external',
      seedable: false
    });
  }

  const submitResp = await api.post(
    '/command/datas/jobs/submit?include_rows=true',
    {
      ...payload,
      preview_token: previewResp.data.preview_token
    },
    'tenant_admin'
  );
  expect(submitResp.code).toBe(200);
  expect(typeof submitResp.data.job_id).toBe('string');
  expect(submitResp.data.job_id).not.toBe('');
  expect(submitResp.data.rows.some(row => row && row.device_id === seed.id)).toBe(true);

  return {
    deviceId: seed.id,
    jobId: submitResp.data.job_id,
    cleanup: seed.cleanup
  };
}

async function waitForCommandJob(api, jobId, predicate, timeoutMs = 45000) {
  const deadline = Date.now() + timeoutMs;
  let lastResponse = null;
  while (Date.now() < deadline) {
    lastResponse = await api.get('/command/datas/jobs/' + jobId, { include_rows: true }, 'tenant_admin');
    if (lastResponse.code === 200 && predicate(lastResponse.data)) return lastResponse.data;
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  throw new Error('command job did not reach the expected state: ' + JSON.stringify(lastResponse));
}

test.describe('command jobs module', () => {
  test.use({ role: 'tenant_admin' });

  test('browser draft previews, submits, and finds the persisted job in history', async ({ rolePage, api }, testInfo) => {
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
    const identify = 'test_dry_contact';
    const value = JSON.stringify(testData.getTestDryContactParams());

    try {
      // A real submit requires at least one eligible device. Do not turn an
      // offline-only fixture into a green fake: preserve this test for the
      // live/online environment and report the external prerequisite clearly.
      const preflight = await api.post('/command/datas/jobs/preview', {
        ...buildCommandJobPayload(seed.id),
        identify,
        value
      }, 'tenant_admin');
      expect(preflight.code).toBe(200);
      expect(preflight.data.rows).toEqual(
        expect.arrayContaining([expect.objectContaining({ device_id: seed.id })])
      );
      if (Number(preflight.data.eligible_count || 0) <= 0) {
        skipWhenBlocked(testInfo, true, {
          reason: 'seeded device is offline or otherwise ineligible; browser submit requires an online device or MQTT emulator',
          category: 'runtime-external',
          seedable: false
        });
      }

      const commandCenterUrl =
        '/device/command-center' +
        '?device_ids=' + encodeURIComponent(seed.id) +
        '&fleet_source=device_manage' +
        '&fleet_scope=selected_devices' +
        '&fleet_selected_count=1';
      await rolePage.goto(commandCenterUrl, { waitUntil: 'domcontentloaded' });

      await rolePage.getByPlaceholder('Command identifier').fill(identify);
      await rolePage.getByPlaceholder('Command value or JSON params, optional').fill(value);

      const previewResponse = rolePage.waitForResponse(response =>
        response.request().method() === 'POST' && response.url().includes('/command/datas/jobs/preview')
      );
      // The page intentionally exposes preview from the progress guide and
      // the draft form. Drive the form control that owns the entered payload;
      // a page-wide locator is ambiguous once the real page is rendered.
      await rolePage.locator('.command-job-form').getByRole('button', {
        name: 'Preview command dispatch',
        exact: true
      }).click();
      const previewBrowserResp = await previewResponse;
      expect(previewBrowserResp.status()).toBe(200);
      const previewBody = await previewBrowserResp.json();
      expect(previewBody.code).toBe(200);
      expect(previewBody.data.preview_token).toEqual(expect.any(String));
      expect(previewBody.data.rows).toEqual(
        expect.arrayContaining([expect.objectContaining({ device_id: seed.id, eligible: true })])
      );
      expect(previewBrowserResp.request().postDataJSON()).toEqual(
        expect.objectContaining({ device_ids: [seed.id], identify, value })
      );

      const submitButton = rolePage.locator('.command-job-form').getByRole('button', {
        name: 'Submit eligible devices',
        exact: true
      });
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
        expect.objectContaining({ preview_token: previewBody.data.preview_token, identify, value })
      );
      await expect(rolePage.getByText(jobId).first()).toBeVisible({ timeout: 15000 });

      const detailResp = await api.get('/command/datas/jobs/' + jobId, { include_rows: true }, 'tenant_admin');
      expect(detailResp.code).toBe(200);
      expect(detailResp.data).toEqual(expect.objectContaining({ job_id: jobId, identify }));
      expect(detailResp.data.rows).toEqual(
        expect.arrayContaining([expect.objectContaining({ device_id: seed.id })])
      );

      const listResp = await api.get(
        '/command/datas/jobs',
        { page: 1, page_size: 20, search: jobId },
        'tenant_admin'
      );
      expect(listResp.code).toBe(200);
      expect(seedData.listFromResponse(listResp)).toEqual(
        expect.arrayContaining([expect.objectContaining({ job_id: jobId, identify })])
      );

      const historySearch = rolePage.getByPlaceholder('Job ID or command identifier');
      if (!(await historySearch.isVisible())) {
        // Scrolling the real deferred viewport lets IntersectionObserver mount
        // the panel without clicking a placeholder that can be detached by
        // the same mount transition.
        const deferredHistory = rolePage.locator('.command-history-deferred-placeholder');
        await expect(deferredHistory).toBeVisible({ timeout: 15000 });
        await deferredHistory.scrollIntoViewIfNeeded();
      }
      await expect(historySearch).toBeVisible({ timeout: 15000 });
      await historySearch.fill(jobId);
      const historyResponse = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return url.pathname.endsWith('/command/datas/jobs') && url.searchParams.get('search') === jobId;
      });
      await rolePage.getByRole('button', { name: 'Search history', exact: true }).click();
      const historyBrowserResp = await historyResponse;
      expect(historyBrowserResp.status()).toBe(200);
      const historyBody = await historyBrowserResp.json();
      expect(historyBody.code).toBe(200);
      expect(historyBody.data.list).toEqual(
        expect.arrayContaining([expect.objectContaining({ job_id: jobId, identify })])
      );
      await expect(rolePage.getByText(jobId).first()).toBeVisible();
    } finally {
      await seed.cleanup();
    }
  });

  test('scheduled command job is canceled from the browser and persists the canceled state', async ({ rolePage, api }, testInfo) => {
    const scheduledAt = new Date(Date.now() + 5 * 60 * 1000).toISOString();
    const job = await createSelectedDeviceCommandJob(api, { scheduled_at: scheduledAt }, testInfo);

    try {
      await rolePage.goto('/device/command-center?command_job_id=' + encodeURIComponent(job.jobId), {
        waitUntil: 'domcontentloaded'
      });
      await expect(rolePage.getByText(job.jobId).first()).toBeVisible({ timeout: 15000 });

      const cancelResponse = rolePage.waitForResponse(response =>
        response.request().method() === 'POST' &&
        response.url().includes('/command/datas/jobs/' + job.jobId + '/cancel')
      );
      await rolePage.getByRole('button', { name: 'Cancel job', exact: true }).click();
      const browserResp = await cancelResponse;
      expect(browserResp.status()).toBe(200);
      const browserBody = await browserResp.json();
      expect(browserBody.code).toBe(200);
      expect(browserBody.data).toEqual(expect.objectContaining({ job_id: job.jobId, status: 'canceled' }));

      const detail = await api.get('/command/datas/jobs/' + job.jobId, { include_rows: true }, 'tenant_admin');
      expect(detail.code).toBe(200);
      expect(detail.data).toEqual(expect.objectContaining({
        job_id: job.jobId,
        status: 'canceled',
        can_cancel: false,
        can_retry_failed: false
      }));
      await expect(rolePage.getByText(/Canceled/i).first()).toBeVisible();
    } finally {
      await job.cleanup();
    }
  });

  test('failed device acknowledgement is retried from the browser through the real broker path', async ({ rolePage, api }, testInfo) => {
    testInfo.setTimeout(90000);
      skipWhenBlocked(testInfo, !(await mqttBrokerAvailable()), {
      reason: `MQTT broker ${seedData.mqttEndpointDescription()} is not listening; real device ACK path unavailable`,
      category: 'runtime-external',
      seedable: false
    });
    const job = await createSelectedDeviceCommandJob(api, { identify: failureCommandIdentify }, testInfo);

    try {
      const submitted = await waitForCommandJob(
        api,
        job.jobId,
        data => Array.isArray(data.rows) && data.rows.some(row => row.device_id === job.deviceId && row.message_id)
      );
      const submittedRow = submitted.rows.find(row => row.device_id === job.deviceId && row.message_id);
      expect(submittedRow.message_id).toEqual(expect.any(String));

      const failed = await waitForCommandJob(
        api,
        job.jobId,
        data => data.can_retry_failed === true && Array.isArray(data.rows) && data.rows.some(row =>
          row.device_id === job.deviceId && row.status === 'failed' && row.response_status === '4'
        )
      );
      const failedRow = failed.rows.find(row => row.device_id === job.deviceId && row.status === 'failed');
      const retryAfterMs = Date.parse(failedRow.next_retry_after || '');
      const retryDelayMs = Number.isFinite(retryAfterMs)
        ? Math.max(0, retryAfterMs - Date.now() + 250)
        : 0;
      await new Promise(resolve => setTimeout(resolve, retryDelayMs));

      await rolePage.goto('/device/command-center?command_job_id=' + encodeURIComponent(job.jobId), {
        waitUntil: 'domcontentloaded'
      });
      await expect(rolePage.getByText(job.jobId).first()).toBeVisible({ timeout: 15000 });
      const refreshResponse = rolePage.waitForResponse(response =>
        response.request().method() === 'GET' &&
        response.url().includes('/command/datas/jobs/' + job.jobId),
        { timeout: 20000 }
      );
      await rolePage
        .locator('.command-job-result-section--next')
        .getByRole('button', { name: 'Refresh job', exact: true })
        .click();
      const refreshedJobResponse = await refreshResponse;
      expect(refreshedJobResponse.status()).toBe(200);

      const retryButton = rolePage.getByRole('button', { name: 'Retry failed devices', exact: true });
      await expect(retryButton).toBeEnabled({ timeout: 15000 });
      const retryResponse = rolePage.waitForResponse(response =>
        response.request().method() === 'POST' &&
        response.url().includes('/command/datas/jobs/' + job.jobId + '/retry')
      );
      await retryButton.click();
      const browserResp = await retryResponse;
      expect(browserResp.status()).toBe(200);
      const browserBody = await browserResp.json();
      expect(browserBody.code).toBe(200);
      expect(browserBody.data.job_id).toBe(job.jobId);

      const retried = await api.get('/command/datas/jobs/' + job.jobId, { include_rows: true }, 'tenant_admin');
      expect(retried.code).toBe(200);
      expect(retried.data.job_id).toBe(job.jobId);
      expect(retried.data.rows).toEqual(
        expect.arrayContaining([expect.objectContaining({
          device_id: job.deviceId,
          dispatch_attempts: expect.any(Number)
        })])
      );
      expect(retried.data.rows.find(row => row.device_id === job.deviceId).dispatch_attempts)
        .toBeGreaterThan(submittedRow.dispatch_attempts || 0);
    } finally {
      await job.cleanup();
    }
  });

  test('selected-device command job stays visible with result and support evidence', async ({ rolePage, api }, testInfo) => {
    const job = await createSelectedDeviceCommandJob(api, {}, testInfo);

    try {
      const commandCenterUrl =
        '/device/command-center' +
        '?device_ids=' + encodeURIComponent(job.deviceId) +
        '&fleet_source=device_manage' +
        '&fleet_scope=selected_devices' +
        '&fleet_selected_count=1' +
        '&command_job_id=' + encodeURIComponent(job.jobId);

      await rolePage.goto(commandCenterUrl, { waitUntil: 'domcontentloaded' });

      await expect(rolePage).toHaveURL(/\/device\/command-center/);
      await expect(rolePage.getByText(/Command Center|Selected-device command dispatch/i).first()).toBeVisible({
        timeout: 15000
      });
      await expect(rolePage.getByText(job.jobId).first()).toBeVisible({ timeout: 15000 });
      await expect(rolePage.getByText(/Result overview|Command summary/i).first()).toBeVisible();
      await expect(rolePage.getByText(/Next actions|下一步/i).first()).toBeVisible();
      await expect(rolePage.getByText(/Evidence and timeline|Timeline/i).first()).toBeVisible();
      await expect(rolePage.getByText(/Device response/i).first()).toBeVisible();
      await expect(rolePage.getByText(/Response evidence/i).first()).toBeVisible();

      const refreshResponse = rolePage.waitForResponse(response => {
        const url = new URL(response.url());
        return response.request().method() === 'GET' &&
          url.pathname.includes('/command/datas/jobs/' + job.jobId);
      });
      // The result view has a primary-action refresh and the explicit
      // operator refresh control. Both are real actions; use the explicit
      // bottom control so the locator remains stable when the primary action
      // changes with job state.
      await rolePage.locator('.command-job-result-section--next').getByRole('button', {
        name: /Refresh job/i
      }).last().click();
      const refreshBrowserResp = await refreshResponse;
      expect(refreshBrowserResp.status()).toBe(200);
      const refreshBody = await refreshBrowserResp.json();
      expect(refreshBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({ job_id: job.jobId })
      }));
      await expect(rolePage.getByText(job.jobId).first()).toBeVisible({ timeout: 15000 });

      await rolePage.locator('.command-job-result-section--next .n-space').getByRole('button', {
        name: /Preview support bundle/i,
        exact: true
      }).click();
      const supportPreview = rolePage.locator('.command-support-preview');
      await expect(supportPreview).toBeVisible({ timeout: 15000 });
      await expect(
        supportPreview.getByText(/support bundle|Support evidence preview|支持证据预览|Next actions|Share hint/i).first()
      ).toBeVisible();
      await expect(
        supportPreview.getByText(/Ready to retry|Waiting for retry window|Retry limit reached|准备重试|等待重试|重试上限/i).first()
      ).toBeVisible();
    } finally {
      await job.cleanup();
    }
  });

  test('selected-device command job downloads support bundle JSON for operator handoff', async ({ rolePage, api }, testInfo) => {
    const job = await createSelectedDeviceCommandJob(api, {}, testInfo);

    try {
      const commandCenterUrl =
        '/device/command-center' +
        '?device_ids=' + encodeURIComponent(job.deviceId) +
        '&fleet_source=device_manage' +
        '&fleet_scope=selected_devices' +
        '&fleet_selected_count=1' +
        '&command_job_id=' + encodeURIComponent(job.jobId);

      await rolePage.goto(commandCenterUrl, { waitUntil: 'domcontentloaded' });

      await expect(rolePage.getByText(job.jobId).first()).toBeVisible({ timeout: 15000 });
      await expect(rolePage.getByRole('button', { name: /Download support bundle/i })).toBeVisible({
        timeout: 15000
      });

      const sanitizedJobId = job.jobId.replace(/[^a-zA-Z0-9._-]/g, '_');
      const downloadPromise = rolePage.waitForEvent('download');
      await rolePage.getByRole('button', { name: /Download support bundle/i }).click();
      const download = await downloadPromise;
      const filePath = await download.path();

      expect(download.suggestedFilename()).toBe(
        'aetherlink-command-job-' + sanitizedJobId + '-support-bundle.json'
      );
      expect(filePath).toEqual(expect.any(String));

      const bundle = JSON.parse(fs.readFileSync(filePath, 'utf8'));
      expect(bundle).toEqual(
        expect.objectContaining({
          job_id: job.jobId,
          scope_type: 'selected_devices',
          identify: 'test_dry_contact',
          status_counts: expect.any(Object),
          next_actions: expect.any(Array),
          generated_at: expect.any(String),
          share_hint: expect.any(String)
        })
      );
      expect(Number.isFinite(Date.parse(bundle.generated_at))).toBe(true);
      expect(bundle.next_actions.length).toBeGreaterThan(0);
      expect(bundle.share_hint.trim()).not.toBe('');
    } finally {
      await job.cleanup();
    }
  });

  test('support preview failed devices link directly to Ready Check and Job detail', async ({ rolePage, api }, testInfo) => {
    testInfo.setTimeout(90000);
    skipWhenBlocked(testInfo, !(await mqttBrokerAvailable()), {
      reason: `MQTT broker ${seedData.mqttEndpointDescription()} is not listening; failed-device support links require a real ACK failure`,
      category: 'runtime-external',
      seedable: false
    });
    const job = await createSelectedDeviceCommandJob(api, { identify: failureCommandIdentify }, testInfo);

    try {
      // Wait for the job to be submitted and capture the message_id. The
      // dedicated emulator publishes the failure ACK for this identifier.
      const submitted = await waitForCommandJob(
        api,
        job.jobId,
        data => Array.isArray(data.rows) && data.rows.some(row => row.device_id === job.deviceId && row.message_id)
      );
      const submittedRow = submitted.rows.find(row => row.device_id === job.deviceId && row.message_id);

      // Wait for the job to reach the failed state with can_retry_failed=true
      const failed = await waitForCommandJob(
        api,
        job.jobId,
        data => data.can_retry_failed === true && Array.isArray(data.rows) && data.rows.some(row =>
          row.device_id === job.deviceId && row.status === 'failed' && row.response_status === '4'
        )
      );

      // API cross-check: download the real support bundle and assert failed_devices with links
      const supportResp = await api.get('/command/datas/jobs/' + job.jobId + '/support-bundle', {}, 'tenant_admin');
      expect(supportResp.code).toBe(200);
      expect(supportResp.data).toEqual(expect.objectContaining({
        job_id: job.jobId,
        failed_devices: expect.arrayContaining([
          expect.objectContaining({
            device_id: job.deviceId,
            status: 'failed',
            ready_check_url: expect.any(String),
            job_detail_url: expect.any(String)
          })
        ])
      }));
      const failedDevice = supportResp.data.failed_devices.find(d => d.device_id === job.deviceId);
      const detailId = failedDevice.detail_id;
      expect(String(detailId || '').trim()).not.toBe('');
      expect(failedDevice.ready_check_url).toContain('/device/details');
      expect(failedDevice.ready_check_url).toContain('command_detail_id=' + encodeURIComponent(detailId));
      expect(failedDevice.job_detail_url).toContain('/device/command-center');
      expect(failedDevice.job_detail_url).toContain('detail_id=' + encodeURIComponent(detailId));

      // Browser: open the support preview and assert the real links are rendered
      const commandCenterUrl =
        '/device/command-center' +
        '?device_ids=' + encodeURIComponent(job.deviceId) +
        '&fleet_source=device_manage' +
        '&fleet_scope=selected_devices' +
        '&fleet_selected_count=1' +
        '&command_job_id=' + encodeURIComponent(job.jobId);

      await rolePage.goto(commandCenterUrl, { waitUntil: 'domcontentloaded' });
      await expect(rolePage.getByText(job.jobId).first()).toBeVisible({ timeout: 15000 });

      await rolePage.locator('.command-job-operator-action').getByRole('button', {
        name: /Preview support bundle/i,
        exact: true
      }).click();
      const supportPreview = rolePage.locator('.command-support-preview');
      await expect(supportPreview).toBeVisible({ timeout: 15000 });

      const readyCheckLink = supportPreview.getByRole('link', {
        name: /Open Ready Check|打开 Ready Check|Abrir Ready Check/i
      });
      const jobDetailLink = supportPreview.getByRole('link', {
        name: /View Job detail|查看 Job 明细|Ver detalle del Job/i
      });

      await expect(readyCheckLink).toBeVisible();
      await expect(jobDetailLink).toBeVisible();
      await expect(readyCheckLink).toHaveAttribute('href', new RegExp('/device/details.*command_detail_id=' + encodeURIComponent(detailId)));
      await expect(jobDetailLink).toHaveAttribute('href', new RegExp('/device/command-center.*detail_id=' + encodeURIComponent(detailId)));
    } finally {
      await job.cleanup();
    }
  });
});
