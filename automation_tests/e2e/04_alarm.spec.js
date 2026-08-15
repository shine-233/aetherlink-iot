/**
 * Alarm module E2E evidence. Business cases bind seeded alarm or notification
 * records to browser-visible state; filter and export cases remain boundary or contract evidence.
 */

const { test, expect } = require('./fixtures');
const fs = require('fs');
const seedData = require('../lib/seed_data');

async function downloadCurrentPageAlarmClosureBundle(rolePage) {
  await expect(rolePage.getByTestId('alarm-download-current-page-evidence')).toBeVisible({
    timeout: 15000
  });

  const downloadPromise = rolePage.waitForEvent('download');
  await rolePage.getByTestId('alarm-download-current-page-evidence').click();
  const download = await downloadPromise;
  const filePath = await download.path();
  expect(filePath).toEqual(expect.any(String));

  const bundle = JSON.parse(fs.readFileSync(filePath, 'utf8'));
  expect(bundle.schema).toBe('aetherlink.alarm.closure-evidence-bundle.v1');
  expect(bundle.loadedPageEvidence).toEqual(expect.any(Array));
  expect(bundle.verificationBoundary).toEqual(
    expect.objectContaining({
      platformEvidenceOnly: true,
      fieldRecoveryNotProven: true
    })
  );
  return bundle;
}

async function expectSuccessfulAlarmBatchAction(response, seed, action, note, expectedStatus) {
  expect(response.status()).toBe(200);
  const body = await response.json();
  expect(response.request().postDataJSON()).toEqual({ ids: [seed.id], action, note });
  expect(body.code).toBe(200);
  expect(body.data).toEqual(
    expect.objectContaining({
      action,
      success_count: 1,
      failure_count: 0
    })
  );
  expect(body.data.results).toEqual(expect.any(Array));
  expect(body.data.results).toHaveLength(1);
  const [result] = body.data.results;
  expect(result).toEqual(
    expect.objectContaining({
      id: seed.id,
      ok: true
    })
  );
  expect(result.history).toEqual(
    expect.objectContaining({
      id: seed.id,
      alarm_status: expectedStatus,
      action_note: note
    })
  );
  return result.history;
}

function expectSeededAlarmEvidenceRow(bundle, seed, expected) {
  const seededRow = bundle.loadedPageEvidence.find(row => row.id === seed.id);
  expect(seededRow).toEqual(
    expect.objectContaining({
      id: seed.id,
      alarmConfigName: seed.alarmConfigName,
      ...expected
    })
  );
  return seededRow;
}

function isApiResponseFor(response, path, method = 'GET') {
  try {
    const url = new URL(response.url());
    // Dev uses the Vite /proxy-default prefix while preview uses /api/v1;
    // the backend resource suffix is stable across both transports.
    return response.request().method() === method && url.pathname.endsWith(path);
  } catch {
    return false;
  }
}

async function expectSuccessfulPagedResponse(response) {
  expect(response.status()).toBe(200);
  const body = await response.json();
  expect(body).toEqual(expect.objectContaining({
    code: 200,
    data: expect.objectContaining({
      total: expect.any(Number),
      list: expect.any(Array)
    })
  }));
  return body;
}

async function openAlarmCenterWithHistoryResponse(rolePage) {
  const responsePromise = rolePage.waitForResponse(
    response => isApiResponseFor(response, '/alarm/info/history'),
    { timeout: 20000 }
  );
  await rolePage.goto('/alarm/warning-message', { waitUntil: 'domcontentloaded' });
  return expectSuccessfulPagedResponse(await responsePromise);
}

async function selectAlarmLevel(rolePage, label) {
  const alarmLevelFormItem = rolePage.locator('.n-form-item').filter({
    has: rolePage.getByText(/Alarm Level|告警等级/i).first()
  });
  await alarmLevelFormItem.locator('.n-base-selection').click();
  const option = label === 'High'
    ? rolePage.getByText(/^(High Alert|High|高告警)$/i).last()
    : rolePage.getByText(label, { exact: true }).last();
  await option.click();
}

function parseAlarmRemark(row) {
  if (!row || !row.remark) return {};
  if (typeof row.remark === 'object') return row.remark;
  try {
    const parsed = JSON.parse(row.remark);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function expectAlarmActionInList(body, seed, actionFields) {
  const row = body.data.list.find(item => item.id === seed.id);
  expect(row).toEqual(expect.objectContaining({ id: seed.id }));
  const remark = parseAlarmRemark(row);
  expect(remark).toEqual(expect.objectContaining(actionFields));
  return row;
}

test.describe('alarm module', () => {
  test.use({ role: 'tenant_admin' });

  test('user search refresh keeps the seeded alarm aligned with the history API response', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureSceneAlarmHistory('tenant_admin');
    if (seed.blocked) {
      throw new Error('scene alarm history seed is blocked: ' + (seed.reason || 'missing deterministic seed prerequisite'));
    }

    try {
      await openAlarmCenterWithHistoryResponse(rolePage);
      const refreshedResponsePromise = rolePage.waitForResponse(
        response => isApiResponseFor(response, '/alarm/info/history'),
        { timeout: 20000 }
      );
      await rolePage.getByRole('button', { name: 'Search' }).click();
      const body = await expectSuccessfulPagedResponse(await refreshedResponsePromise);
      const apiRow = body.data.list.find(row => row.id === seed.id);
      expect(apiRow).toEqual(expect.objectContaining({
        id: seed.id,
        alarm_config_name: seed.alarmConfigName,
        alarm_status: 'H'
      }));

      await expect(rolePage).toHaveURL(/\/alarm\/warning-message$/);
      const seededTableRow = rolePage.locator('.n-data-table-tr').filter({
        hasText: seed.alarmConfigName
      });
      await expect(seededTableRow).toHaveCount(1);
      await expect(seededTableRow).toContainText(seed.alarmConfigName);
    } finally {
      await seed.cleanup();
    }
  });

  test('alarm level filter sends H to the history API and keeps the seeded high alarm visible', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureSceneAlarmHistory('tenant_admin');
    if (seed.blocked) {
      throw new Error('scene alarm history seed is blocked: ' + (seed.reason || 'missing deterministic seed prerequisite'));
    }

    try {
      await openAlarmCenterWithHistoryResponse(rolePage);
      await selectAlarmLevel(rolePage, 'High');

      const filteredResponsePromise = rolePage.waitForResponse(response => {
        if (!isApiResponseFor(response, '/alarm/info/history')) return false;
        return new URL(response.url()).searchParams.get('alarm_status') === 'H';
      }, { timeout: 20000 });
      await rolePage.getByRole('button', { name: 'Search' }).click();
      const body = await expectSuccessfulPagedResponse(await filteredResponsePromise);

      expect(body.data.list.length).toBeGreaterThan(0);
      expect(body.data.list.every(row => row.alarm_status === 'H')).toBe(true);
      expect(body.data.list).toEqual(expect.arrayContaining([
        expect.objectContaining({ id: seed.id, alarm_config_name: seed.alarmConfigName })
      ]));
      await expect(rolePage.locator('.n-data-table-tr').filter({ hasText: seed.alarmConfigName })).toHaveCount(1);

      const bundle = await downloadCurrentPageAlarmClosureBundle(rolePage);
      expect(bundle.pageContext.filters).toEqual(expect.objectContaining({ alarmStatus: 'H' }));
      expectSeededAlarmEvidenceRow(bundle, seed, { status: 'H', severity: 'H' });
    } finally {
      await seed.cleanup();
    }
  });

  test('alert center downloads current page closure evidence JSON bundle', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureSceneAlarmHistory('tenant_admin');
    if (seed.blocked) {
      throw new Error('scene alarm history seed is blocked: ' + (seed.reason || 'missing deterministic seed prerequisite'));
    }

    try {
      // API cross-check: seeded alarm is visible in the history API
      const historyResp = await api.get('/alarm/info/history', { page: 1, page_size: 100 }, 'tenant_admin');
      expect(historyResp.code).toBe(200);
      const apiRow = seedData.listFromResponse(historyResp).find(row => row.id === seed.id);
      expect(apiRow).toEqual(expect.objectContaining({
        id: seed.id,
        alarm_config_name: seed.alarmConfigName
      }));

      // Browser: open alert center and download the closure evidence bundle
      const historyResponsePromise = rolePage.waitForResponse(
        response => isApiResponseFor(response, '/alarm/info/history'),
        { timeout: 20000 }
      );
      await rolePage.goto('/alarm/warning-message', { waitUntil: 'domcontentloaded' });
      await expectSuccessfulPagedResponse(await historyResponsePromise);

      await expect(rolePage.getByTestId('alarm-download-current-page-evidence')).toBeVisible({
        timeout: 15000
      });

      const downloadPromise = rolePage.waitForEvent('download');
      await rolePage.getByTestId('alarm-download-current-page-evidence').click();
      const download = await downloadPromise;
      const filePath = await download.path();
      expect(download.suggestedFilename()).toMatch(
        /^aetherlink-alarm-closure-evidence-[a-zA-Z0-9_-]+-\d{8}-\d{6}\.json$/
      );
      expect(filePath).toEqual(expect.any(String));

      const bundle = JSON.parse(fs.readFileSync(filePath, 'utf8'));
      expect(bundle.schema).toBe('aetherlink.alarm.closure-evidence-bundle.v1');
      expect(typeof bundle.generatedAt).toBe('string');
      expect(bundle.pageContext).toEqual(
        expect.objectContaining({
          scope: expect.any(String),
          filters: expect.any(Object),
          pagination: expect.objectContaining({
            page: expect.any(Number),
            pageSize: expect.any(Number),
            loadedRowCount: expect.any(Number)
          }),
          routeContext: expect.objectContaining({
            hasDeviceContext: expect.any(Boolean),
            fleetDeviceCount: expect.any(Number)
          }),
          selection: expect.objectContaining({
            selectedRowKeys: expect.any(Array),
            selectedLoadedRowCount: expect.any(Number),
            selectedLoadedRows: expect.any(Array)
          })
        })
      );
      expect(bundle.loadedPageEvidence).toEqual(expect.any(Array));
      expect(bundle.verificationBoundary).toEqual(
        expect.objectContaining({
          platformEvidenceOnly: true,
          fieldRecoveryNotProven: true,
          message: expect.any(String)
        })
      );

      // Cross-check: the seeded alarm row appears in the downloaded evidence bundle
      const seededRow = bundle.loadedPageEvidence.find(row => row.id === seed.id);
      expect(seededRow).toEqual(expect.objectContaining({
        id: seed.id,
        alarmConfigName: seed.alarmConfigName
      }));
    } finally {
      await seed.cleanup();
    }
  });

  test('seeded alarm scene is visible in downloaded closure evidence bundle', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureSceneAlarmHistory('tenant_admin');
    if (seed.blocked) {
      throw new Error('scene alarm history seed is blocked: ' + (seed.reason || 'missing deterministic seed prerequisite'));
    }

    try {
      await rolePage.goto('/alarm/warning-message', { waitUntil: 'domcontentloaded' });

      await expect(rolePage.getByTestId('alarm-download-current-page-evidence')).toBeVisible({
        timeout: 15000
      });

      const downloadPromise = rolePage.waitForEvent('download');
      await rolePage.getByTestId('alarm-download-current-page-evidence').click();
      const download = await downloadPromise;
      const filePath = await download.path();
      expect(filePath).toEqual(expect.any(String));

      const bundle = JSON.parse(fs.readFileSync(filePath, 'utf8'));
      expect(bundle.schema).toBe('aetherlink.alarm.closure-evidence-bundle.v1');
      expect(bundle.loadedPageEvidence).toEqual(expect.any(Array));

      const seededRow = bundle.loadedPageEvidence.find(row => row.id === seed.id);
      expect(seededRow).toEqual(
        expect.objectContaining({
          id: seed.id,
          alarmConfigName: seed.alarmConfigName,
          status: 'H',
          severity: 'H'
        })
      );
      expect(bundle.verificationBoundary).toEqual(
        expect.objectContaining({
          platformEvidenceOnly: true,
          fieldRecoveryNotProven: true
        })
      );
    } finally {
      await seed.cleanup();
    }
  });

  test('browser acknowledge/reset actions persist and appear in downloaded closure evidence', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureSceneAlarmHistory('tenant_admin');
    if (seed.blocked) {
      throw new Error('scene alarm history seed is blocked: ' + (seed.reason || 'missing deterministic seed prerequisite'));
    }

    try {
      await openAlarmCenterWithHistoryResponse(rolePage);
      const ackNote = 'e2e acknowledge seeded alarm closure';
      const seededAlarmRow = rolePage.locator('.n-data-table-tr').filter({
        hasText: seed.alarmConfigName
      });
      await expect(seededAlarmRow).toHaveCount(1);
      await seededAlarmRow.getByRole('checkbox').click();
      const acknowledgeButton = rolePage.getByRole('button', { name: 'Acknowledge selected' });
      await expect(acknowledgeButton).toBeEnabled();
      await acknowledgeButton.click();

      const acknowledgeModal = rolePage.locator('.n-modal').filter({
        hasText: 'Acknowledge selected alarms'
      });
      await expect(acknowledgeModal).toBeVisible();
      await acknowledgeModal.locator('textarea').fill(ackNote);
      const acknowledgeResponsePromise = rolePage.waitForResponse(
        response => isApiResponseFor(response, '/alarm/info/history/batch-action', 'PUT'),
        { timeout: 20000 }
      );
      const acknowledgeRefreshPromise = rolePage.waitForResponse(
        response => isApiResponseFor(response, '/alarm/info/history'),
        { timeout: 20000 }
      );
      await acknowledgeModal.getByRole('button', { name: 'Confirm', exact: true }).click();
      const ackHistory = await expectSuccessfulAlarmBatchAction(
        await acknowledgeResponsePromise,
        seed,
        'acknowledge',
        ackNote,
        'H'
      );
      const acknowledgeRefresh = await expectSuccessfulPagedResponse(await acknowledgeRefreshPromise);
      expectAlarmActionInList(acknowledgeRefresh, seed, {
        acknowledged: true,
        acknowledged_by: ackHistory.acknowledged_by,
        acknowledged_at: ackHistory.acknowledged_at
      });

      const persistedAck = await api.get('/alarm/info/history', { page: 1, page_size: 100 }, 'tenant_admin');
      expect(persistedAck.code).toBe(200);
      expectAlarmActionInList(persistedAck, seed, {
        acknowledged: true,
        acknowledged_by: ackHistory.acknowledged_by,
        acknowledged_at: ackHistory.acknowledged_at
      });

      let bundle = await downloadCurrentPageAlarmClosureBundle(rolePage);
      let seededRow = expectSeededAlarmEvidenceRow(bundle, seed, {
        status: 'H',
        acknowledged: true
      });
      expect(seededRow.acknowledgedBy).toEqual(expect.any(String));
      expect(seededRow.acknowledgedBy.trim()).not.toBe('');
      expect(seededRow.acknowledgedAt).toEqual(expect.any(String));
      expect(seededRow.acknowledgedAt.trim()).not.toBe('');
      expect(seededRow.acknowledgedBy).toBe(ackHistory.acknowledged_by);
      expect(seededRow.acknowledgedAt).toBe(ackHistory.acknowledged_at);

      const resetNote = 'e2e reset seeded alarm closure';
      const acknowledgedAlarmRow = rolePage.locator('.n-data-table-tr').filter({
        hasText: seed.alarmConfigName
      });
      await expect(acknowledgedAlarmRow).toHaveCount(1);
      await acknowledgedAlarmRow.getByRole('checkbox').click();
      const resetButton = rolePage.getByRole('button', { name: 'Reset selected' });
      await expect(resetButton).toBeEnabled();
      await resetButton.click();

      const resetModal = rolePage.locator('.n-modal').filter({
        hasText: 'Reset selected alarms'
      });
      await expect(resetModal).toBeVisible();
      await resetModal.locator('textarea').fill(resetNote);
      const resetResponsePromise = rolePage.waitForResponse(
        response => isApiResponseFor(response, '/alarm/info/history/batch-action', 'PUT'),
        { timeout: 20000 }
      );
      const resetRefreshPromise = rolePage.waitForResponse(
        response => isApiResponseFor(response, '/alarm/info/history'),
        { timeout: 20000 }
      );
      await resetModal.getByRole('button', { name: 'Confirm', exact: true }).click();
      const resetHistory = await expectSuccessfulAlarmBatchAction(
        await resetResponsePromise,
        seed,
        'reset',
        resetNote,
        'N'
      );
      const resetRefresh = await expectSuccessfulPagedResponse(await resetRefreshPromise);
      const resetRefreshRow = expectAlarmActionInList(resetRefresh, seed, {
        reset_by: resetHistory.reset_by,
        reset_at: resetHistory.reset_at
      });
      expect(resetRefreshRow.alarm_status).toBe('N');

      const persistedReset = await api.get('/alarm/info/history', { page: 1, page_size: 100 }, 'tenant_admin');
      expect(persistedReset.code).toBe(200);
      const persistedResetRow = expectAlarmActionInList(persistedReset, seed, {
        reset_by: resetHistory.reset_by,
        reset_at: resetHistory.reset_at
      });
      expect(persistedResetRow.alarm_status).toBe('N');

      bundle = await downloadCurrentPageAlarmClosureBundle(rolePage);
      seededRow = expectSeededAlarmEvidenceRow(bundle, seed, {
        status: 'N',
        acknowledged: true,
        reset: true
      });
      expect(seededRow.resetBy).toEqual(expect.any(String));
      expect(seededRow.resetBy.trim()).not.toBe('');
      expect(seededRow.resetAt).toEqual(expect.any(String));
      expect(seededRow.resetAt.trim()).not.toBe('');
      expect(seededRow.resetBy).toBe(resetHistory.reset_by);
      expect(seededRow.resetAt).toBe(resetHistory.reset_at);
    } finally {
      await seed.cleanup();
    }
  });

  test('notification recipient filter sends an exact negative query and renders the empty result', async ({ rolePage }) => {
    const initialResponsePromise = rolePage.waitForResponse(
      response => isApiResponseFor(response, '/notification_history/list'),
      { timeout: 20000 }
    );
    await rolePage.goto('/alarm/notification-record', { waitUntil: 'domcontentloaded' });
    await expectSuccessfulPagedResponse(await initialResponsePromise);

    const missingRecipient = 'codex-no-notification-' + Date.now() + '@example.invalid';
    const recipientInput = rolePage.getByPlaceholder('Recipient');
    await recipientInput.fill(missingRecipient);

    const filteredResponsePromise = rolePage.waitForResponse(response => {
      if (!isApiResponseFor(response, '/notification_history/list')) return false;
      return new URL(response.url()).searchParams.get('send_target') === missingRecipient;
    }, { timeout: 20000 });
    await rolePage.getByRole('button', { name: 'Search' }).click();
    const body = await expectSuccessfulPagedResponse(await filteredResponsePromise);

    expect(body.data).toEqual(expect.objectContaining({ total: 0, list: [] }));
    await expect(recipientInput).toHaveValue(missingRecipient);
    await expect(rolePage.locator('.n-data-table-tbody .n-data-table-tr')).toHaveCount(0);
    await expect(rolePage.locator('.n-data-table-empty')).toBeVisible();
  });

  test('notification reset clears the recipient filter and reloads the unfiltered API query', async ({ rolePage }) => {
    const initialResponsePromise = rolePage.waitForResponse(
      response => isApiResponseFor(response, '/notification_history/list'),
      { timeout: 20000 }
    );
    await rolePage.goto('/alarm/notification-record', { waitUntil: 'domcontentloaded' });
    const initialBody = await expectSuccessfulPagedResponse(await initialResponsePromise);

    const recipientInput = rolePage.getByPlaceholder('Recipient');
    await recipientInput.fill('codex-reset-filter@example.invalid');

    const resetResponsePromise = rolePage.waitForResponse(response => {
      if (!isApiResponseFor(response, '/notification_history/list')) return false;
      const params = new URL(response.url()).searchParams;
      const sendTarget = params.get('send_target');
      return (sendTarget === null || sendTarget === '')
        && params.get('page') === '1'
        && params.get('page_size') === '10';
    }, { timeout: 20000 });
    await rolePage.getByRole('button', { name: 'Reset' }).click();
    const body = await expectSuccessfulPagedResponse(await resetResponsePromise);

    await expect(recipientInput).toHaveValue('');
    expect(body.data.total).toBe(initialBody.data.total);
    expect(body.data.list.map(row => row.id)).toEqual(initialBody.data.list.map(row => row.id));
    await expect(rolePage.getByText('Notification Record').first()).toBeVisible();
  });

  test('seeded notification group is visible from the notification group page', async ({ rolePage, api }) => {
    await api.login('tenant_admin');
    const seed = await seedData.ensureNotificationGroup('tenant_admin');
    try {
      const persistedBefore = await api.get('/notification_group/' + seed.id, {}, 'tenant_admin');
      expect(persistedBefore.code).toBe(200);
      expect(persistedBefore.data).toEqual(expect.objectContaining({
        id: seed.id,
        name: seed.row.name,
        description: 'automation seed',
        notification_type: 'EMAIL',
        status: 'CLOSE'
      }));

      const initialListPromise = rolePage.waitForResponse(
        response => isApiResponseFor(response, '/notification_group/list'),
        { timeout: 20000 }
      );
      await rolePage.goto('/alarm/notification-group', { waitUntil: 'domcontentloaded' });
      const initialList = await expectSuccessfulPagedResponse(await initialListPromise);
      expect(initialList.data.list).toEqual(expect.arrayContaining([
        expect.objectContaining({
          id: seed.id,
          name: seed.row.name,
          notification_type: 'EMAIL',
          status: 'CLOSE'
        })
      ]));

      await expect(rolePage).toHaveURL(/\/alarm\/notification-group$/);
      await expect(rolePage.getByText('Notification Group').first()).toBeVisible();
      const seededTableRow = rolePage.locator('.n-data-table-tr').filter({ hasText: seed.row.name });
      await expect(seededTableRow).toHaveCount(1);
      await expect(seededTableRow).toContainText(seed.row.name);
      const statusSwitch = seededTableRow.getByRole('switch');
      await expect(statusSwitch).toHaveAttribute('aria-checked', 'false');

      const updatePromise = rolePage.waitForResponse(
        response => isApiResponseFor(response, '/notification_group/' + seed.id, 'PUT'),
        { timeout: 20000 }
      );
      const refreshedListPromise = rolePage.waitForResponse(
        response => isApiResponseFor(response, '/notification_group/list'),
        { timeout: 20000 }
      );
      await statusSwitch.click();
      const updateResponse = await updatePromise;
      expect(updateResponse.status()).toBe(200);
      const updateBody = await updateResponse.json();
      expect(updateBody).toEqual(expect.objectContaining({
        code: 200,
        data: expect.objectContaining({
          name: seed.row.name,
          notification_type: 'EMAIL',
          status: 'OPEN'
        })
      }));

      const refreshedList = await expectSuccessfulPagedResponse(await refreshedListPromise);
      expect(refreshedList.data.list).toEqual(expect.arrayContaining([
        expect.objectContaining({ id: seed.id, name: seed.row.name, status: 'OPEN' })
      ]));
      await expect(seededTableRow.getByRole('switch')).toHaveAttribute('aria-checked', 'true');

      const persistedAfter = await api.get('/notification_group/' + seed.id, {}, 'tenant_admin');
      expect(persistedAfter.code).toBe(200);
      expect(persistedAfter.data).toEqual(expect.objectContaining({
        id: seed.id,
        name: seed.row.name,
        notification_type: 'EMAIL',
        status: 'OPEN'
      }));
    } finally {
      await seed.cleanup();
    }
  });

});
