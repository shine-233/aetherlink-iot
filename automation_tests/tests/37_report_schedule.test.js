const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'Report schedule public API contract [37_report_schedule]';
const ACCOUNT = 'tenant_admin';
const OTHER_TENANT = 'tenant_admin_b';
const DENIED_ROLES = ['tenant_user', 'readonly_user'];
const REPORT_TERMINAL_STATUSES = new Set(['succeeded', 'failed', 'ambiguous']);
const REPORT_ACTIVE_GENERATION_STATUSES = new Set(['pending', 'processing', 'retrying']);
const REPORT_ACTIVE_DELIVERY_STATUSES = new Set(['pending', 'processing', 'retrying']);
const REPORT_POLL_INTERVAL_MS = 2000;
const REPORT_POLL_TIMEOUT_MS = 8 * 60 * 1000;
// errcode.CodeNotFound: tenant-scoped report reads fail closed as not-found.
const CODE_NOT_FOUND = 100404;
// errcode.CodeParamError: a missing Idempotency-Key is rejected before lookup.
const CODE_PARAM_ERROR = 100002;
// errcode.CodeOpDenied: revision conflicts and non-retryable runs.
const CODE_OP_DENIED = 201002;

function uniqueValue(prefix) {
  return `${prefix}_${Date.now()}_${Math.floor(Math.random() * 100000)}`;
}

function delay(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function isTerminalRun(run) {
  return Boolean(run && REPORT_TERMINAL_STATUSES.has(run.overall_status));
}

function hasActiveWork(run) {
  if (!run) return true;
  return REPORT_ACTIVE_GENERATION_STATUSES.has(run.generation_status) ||
    REPORT_ACTIVE_DELIVERY_STATUSES.has(run.delivery_status);
}

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectProductError(resp, code) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(code);
}

function scheduleData(resp) {
  expectOk(resp);
  // A bare "is an object" check would be a weak oracle; require a non-empty,
  // non-array payload so empty-success responses cannot satisfy the contract.
  expect(resp.data).to.satisfy(
    value =>
      value !== null &&
      typeof value === 'object' &&
      !Array.isArray(value) &&
      Object.keys(value).length > 0,
    'report API response must carry a non-empty object payload'
  );
  return resp.data;
}

async function readSchedule(id, accountKey = ACCOUNT) {
  return apiClient.get(`/report/schedules/${encodeURIComponent(id)}`, {}, accountKey);
}

async function readRun(scheduleId, runId, accountKey = ACCOUNT) {
  return apiClient.get(runPath(scheduleId, runId), {}, accountKey);
}

function runPath(scheduleId, runId) {
  return `/report/schedules/${encodeURIComponent(scheduleId)}/runs/${encodeURIComponent(runId)}`;
}

async function listRuns(scheduleId, accountKey = ACCOUNT, query = { page: 1, page_size: 100 }) {
  return apiClient.get(`/report/schedules/${encodeURIComponent(scheduleId)}/runs`, query, accountKey);
}

async function waitForRun(scheduleId, runId, predicate, description, timeoutMs = REPORT_POLL_TIMEOUT_MS) {
  const deadline = Date.now() + timeoutMs;
  let lastResponse;
  while (Date.now() < deadline) {
    lastResponse = await readRun(scheduleId, runId);
    if (lastResponse && lastResponse.code === 200 && lastResponse.data && predicate(lastResponse.data)) {
      return lastResponse.data;
    }
    await delay(REPORT_POLL_INTERVAL_MS);
  }
  throw new Error(`${description}: ${JSON.stringify(lastResponse)}`);
}

async function waitForTerminalRun(scheduleId, runId, timeoutMs = REPORT_POLL_TIMEOUT_MS) {
  return waitForRun(
    scheduleId,
    runId,
    isTerminalRun,
    `report run ${runId} did not reach a terminal state`,
    timeoutMs
  );
}

async function waitForInactiveScheduleWork(scheduleId, timeoutMs = REPORT_POLL_TIMEOUT_MS) {
  const deadline = Date.now() + timeoutMs;
  let lastRuns;
  while (Date.now() < deadline) {
    lastRuns = await listRuns(scheduleId);
    if (lastRuns && lastRuns.code === 200 && Array.isArray(lastRuns.data.list)) {
      const active = lastRuns.data.list.filter(hasActiveWork);
      if (active.length === 0) return lastRuns.data.list;
    }
    await delay(REPORT_POLL_INTERVAL_MS);
  }
  throw new Error(
    `schedule ${scheduleId} retained active report work; last runs: ${JSON.stringify(lastRuns && lastRuns.data)}`
  );
}

async function deleteScheduleExact(scheduleId) {
  const current = await readSchedule(scheduleId);
  if (!(current && current.code === 200 && current.data && current.data.revision)) {
    return false;
  }
  await waitForInactiveScheduleWork(scheduleId).catch(() => null);
  const reloaded = await readSchedule(scheduleId);
  if (!(reloaded && reloaded.code === 200 && reloaded.data && reloaded.data.revision)) {
    return false;
  }
  const deleted = await apiClient.delete(
    `/report/schedules/${encodeURIComponent(scheduleId)}?revision=${reloaded.data.revision}`,
    {},
    ACCOUNT
  );
  expectOk(deleted);
  expectProductError(await readSchedule(scheduleId), CODE_NOT_FOUND);
  return true;
}

describe(SUITE, function () {
  this.timeout(120000);

  const baseName = uniqueValue('automation_report');
  const idempotencyKey = uniqueValue('report-run');
  let deviceSeed;
  let scheduleId;
  let revision;
  let initialSchedule;
  let manualRun;
  let terminalRun;
  let cleanupVerified = false;

  before(async function () {
    deviceSeed = await seedData.ensureDevice(ACCOUNT);
  });

  after(async function () {
    if (scheduleId && manualRun && !terminalRun) {
      terminalRun = await waitForTerminalRun(scheduleId, manualRun.run_id).catch(() => null);
    }
    if (scheduleId && !cleanupVerified) {
      await deleteScheduleExact(scheduleId).catch(() => null);
    }
    if (deviceSeed) await deviceSeed.cleanup();
  });

  it('creates a schedule with omitted enabled defaulting to true', async function () {
    const account = apiClient.getConfig().accounts[ACCOUNT];
    expect(account && account.email).to.be.a('string').and.not.equal('');
    const resp = await apiClient.post('/report/schedules', {
      name: baseName,
      cron_expr: '0 0 * * *',
      timezone: 'UTC',
      recipients: account.email,
      device_ids: [deviceSeed.id],
      keys: ['temperature'],
      lookback_hours: 24,
      format: 'csv'
    }, ACCOUNT);
    initialSchedule = scheduleData(resp);
    expect(initialSchedule).to.include({ name: baseName, enabled: true, revision: 1 });
    expect(initialSchedule.timezone).to.equal('UTC');
    expect(initialSchedule.next_run_at).to.be.a('string').and.not.equal('');
    expect(initialSchedule.schedule_error_code).to.equal(null);
    scheduleId = initialSchedule.id;
    revision = initialSchedule.revision;
  });

  it('lists and gets the tenant schedule with exact persisted state', async function () {
    const listResp = await apiClient.get('/report/schedules', {
      page: 1,
      page_size: 100,
      search: baseName
    }, ACCOUNT);
    expectOk(listResp);
    expect(listResp.data.list).to.be.an('array');
    expect(listResp.data.list.some(row => row.id === scheduleId && row.name === baseName)).to.equal(true);

    const detail = scheduleData(await readSchedule(scheduleId));
    for (const key of ['id', 'name', 'cron_expr', 'timezone', 'recipients', 'lookback_hours', 'format', 'enabled', 'revision']) {
      expect(detail[key], key).to.deep.equal(initialSchedule[key]);
    }
    expect(detail.device_ids).to.deep.equal([deviceSeed.id]);
    expect(detail.keys).to.deep.equal(['temperature']);
  });

  it('updates by route identity and persists the next revision', async function () {
    const resp = await apiClient.put(`/report/schedules/${encodeURIComponent(scheduleId)}`, {
      revision,
      name: `${baseName}_updated`,
      lookback_hours: 48
    }, ACCOUNT);
    const updated = scheduleData(resp);
    expect(updated).to.include({ name: `${baseName}_updated`, lookback_hours: 48 });
    expect(updated.revision).to.equal(revision + 1);
    revision = updated.revision;

    const detail = scheduleData(await readSchedule(scheduleId));
    expect(detail).to.include({ name: `${baseName}_updated`, lookback_hours: 48, revision });
  });

  it('rejects a stale revision and proves the schedule was not mutated', async function () {
    const before = scheduleData(await readSchedule(scheduleId));
    const rejected = await apiClient.put(`/report/schedules/${encodeURIComponent(scheduleId)}`, {
      revision: revision - 1,
      name: `${baseName}_stale`
    }, ACCOUNT);
    expectProductError(rejected, CODE_OP_DENIED);

    const after = scheduleData(await readSchedule(scheduleId));
    expect(after.name).to.equal(before.name);
    expect(after.revision).to.equal(before.revision);
    expect(after.updated_at).to.equal(before.updated_at);
  });

  it('accepts one durable manual run with exact 202 Location and stable replay', async function () {
    const options = {
      headers: { 'Idempotency-Key': idempotencyKey },
      rawResponse: true,
      transportRetries: 2,
      transportRetryBackoffMs: 100
    };
    const first = await apiClient.post(
      `/report/schedules/${encodeURIComponent(scheduleId)}/run`, {}, ACCOUNT, options
    );
    expect(first.status).to.equal(202);
    expect(first.data.code).to.equal(200);
    manualRun = first.data.data;
    expect(manualRun).to.include({ schedule_id: scheduleId, status: 'queued', idempotent_replay: false });
    expect(manualRun.run_id).to.be.a('string').and.not.equal('');
    expect(manualRun.status_url).to.equal(`/api/v1/report/schedules/${scheduleId}/runs/${manualRun.run_id}`);
    expect(first.headers.location).to.equal(manualRun.status_url);

    const replay = await apiClient.post(
      `/report/schedules/${encodeURIComponent(scheduleId)}/run`, {}, ACCOUNT, options
    );
    expect(replay.status).to.equal(202);
    expect(replay.headers.location).to.equal(manualRun.status_url);
    expect(replay.data.code).to.equal(200);
    expect(replay.data.data).to.include({
      run_id: manualRun.run_id,
      schedule_id: scheduleId,
      status_url: manualRun.status_url,
      idempotent_replay: true
    });

    const listResp = await listRuns(scheduleId);
    expectOk(listResp);
    expect(listResp.data.list.filter(row => row.run_id === manualRun.run_id)).to.have.length(1);
  });

  it('exposes durable run identity and immutable window without assuming a queued race', async function () {
    const run = scheduleData(await readRun(scheduleId, manualRun.run_id));
    expect(run).to.include({ run_id: manualRun.run_id, schedule_id: scheduleId, trigger: 'manual' });
    expect(['queued', 'running', 'succeeded', 'failed', 'ambiguous']).to.include(run.overall_status);
    expect(run.duplicate_delivery_risk).to.be.a('boolean');
    expect(run.window_start_at).to.be.a('string').and.not.equal('');
    expect(run.window_end_at).to.be.a('string').and.not.equal('');
    expect(run.generation_attempts).to.be.a('number');
    expect(run.delivery_attempts).to.be.a('number');

    terminalRun = await waitForTerminalRun(scheduleId, manualRun.run_id);
    expect(REPORT_TERMINAL_STATUSES.has(terminalRun.overall_status)).to.equal(true);
    expect(terminalRun.run_id).to.equal(manualRun.run_id);
    expect(terminalRun.schedule_id).to.equal(scheduleId);
    expect(['succeeded', 'failed', 'ambiguous']).to.include(terminalRun.generation_status);
    expect(['accepted', 'failed', 'ambiguous']).to.include(terminalRun.delivery_status);
    expect(terminalRun.generation_completed_at).to.be.a('string').and.not.equal('');

    const history = scheduleData(await listRuns(scheduleId));
    const persisted = history.list.filter(row => row.run_id === manualRun.run_id);
    expect(persisted).to.have.length(1);
    expect(persisted[0].overall_status).to.equal(terminalRun.overall_status);
    expect(persisted[0].generation_status).to.equal(terminalRun.generation_status);
    expect(persisted[0].delivery_status).to.equal(terminalRun.delivery_status);

    // An idempotent replay of a run that already reached a terminal state must
    // report that real projection. A hardcoded "queued" here is the P2 defect:
    // it tells the caller work is still pending when it has already settled.
    const terminalReplay = await apiClient.post(
      `/report/schedules/${encodeURIComponent(scheduleId)}/run`, {}, ACCOUNT,
      { headers: { 'Idempotency-Key': idempotencyKey }, rawResponse: true }
    );
    expect(terminalReplay.status).to.equal(202);
    expect(terminalReplay.data.data).to.include({
      run_id: manualRun.run_id,
      schedule_id: scheduleId,
      idempotent_replay: true
    });
    expect(
      REPORT_TERMINAL_STATUSES.has(terminalReplay.data.data.status),
      `terminal replay must not report queued work; got ${terminalReplay.data.data.status}`
    ).to.equal(true);
    expect(terminalReplay.data.data.status).to.equal(terminalRun.overall_status);
  });

  it('requires an idempotency key before retrying a report run', async function () {
    const rejected = await apiClient.post(
      `/report/schedules/${encodeURIComponent(scheduleId)}/runs/${encodeURIComponent(manualRun.run_id)}/retry`,
      {},
      ACCOUNT
    );
    expectProductError(rejected, 100002);
  });

  it('enforces retry eligibility for a terminal parent and rejects ineligible targets', async function () {
    const detail = scheduleData(await readRun(scheduleId, manualRun.run_id));
    // DAL rule: generation failed with no delivery row, or generation succeeded
    // with a failed/ambiguous delivery. Anything else is not retryable.
    const retryable =
      detail.generation_status === 'failed' ||
      detail.delivery_status === 'failed' ||
      detail.delivery_status === 'ambiguous';

    const missingKey = await apiClient.post(
      `/report/schedules/${encodeURIComponent(scheduleId)}/runs/${encodeURIComponent(manualRun.run_id)}/retry`,
      {},
      ACCOUNT
    );
    expectProductError(missingKey, CODE_PARAM_ERROR);

    if (!retryable) {
      const rejected = await apiClient.post(
        `/report/schedules/${encodeURIComponent(scheduleId)}/runs/${encodeURIComponent(manualRun.run_id)}/retry`,
        {},
        ACCOUNT,
        { headers: { 'Idempotency-Key': uniqueValue('report-retry') } }
      );
      expectProductError(rejected, CODE_OP_DENIED);
      return;
    }

    const retryKey = uniqueValue('report-retry');
    const accepted = await apiClient.post(
      `/report/schedules/${encodeURIComponent(scheduleId)}/runs/${encodeURIComponent(manualRun.run_id)}/retry`,
      {},
      ACCOUNT,
      { headers: { 'Idempotency-Key': retryKey }, rawResponse: true }
    );
    expect(accepted.status).to.equal(202);
    const child = accepted.data.data;
    expect(child.run_id).to.be.a('string').and.not.equal('');
    expect(child.run_id).to.not.equal(manualRun.run_id);
    expect(child.schedule_id).to.equal(scheduleId);
    expect(child.idempotent_replay).to.equal(false);

    const replay = await apiClient.post(
      `/report/schedules/${encodeURIComponent(scheduleId)}/runs/${encodeURIComponent(manualRun.run_id)}/retry`,
      {},
      ACCOUNT,
      { headers: { 'Idempotency-Key': retryKey }, rawResponse: true }
    );
    expect(replay.status).to.equal(202);
    expect(replay.data.data).to.include({ run_id: child.run_id, idempotent_replay: true });

    const childDetail = scheduleData(await readRun(scheduleId, child.run_id));
    expect(childDetail).to.include({
      run_id: child.run_id,
      schedule_id: scheduleId,
      trigger: 'retry',
      parent_run_id: manualRun.run_id
    });

    const childTerminal = await waitForTerminalRun(scheduleId, child.run_id);
    expect(REPORT_TERMINAL_STATUSES.has(childTerminal.overall_status)).to.equal(true);
  });

  it('isolates report reads by tenant and denies non-admin write roles', async function () {
    expectProductError(await readSchedule(scheduleId, OTHER_TENANT), CODE_NOT_FOUND);
    const otherList = await apiClient.get('/report/schedules', { page: 1, page_size: 100 }, OTHER_TENANT);
    expectOk(otherList);
    expect(otherList.data.list.some(row => row.id === scheduleId)).to.equal(false);

    for (const role of DENIED_ROLES) {
      const deniedName = uniqueValue(`denied_${role}`);
      const denied = await apiClient.post('/report/schedules', {
        name: deniedName,
        cron_expr: '0 0 * * *',
        timezone: 'UTC',
        recipients: apiClient.getConfig().accounts[role].email,
        device_ids: [deviceSeed.id],
        keys: ['temperature'],
        lookback_hours: 24,
        format: 'csv'
      }, role);
      expect(denied).to.be.an('object');
      expect(denied.code).to.be.a('number');
      expect(denied.code === 200).to.equal(false);
      const ownerList = await apiClient.get('/report/schedules', {
        page: 1,
        page_size: 100,
        search: deniedName
      }, ACCOUNT);
      expectOk(ownerList);
      expect(ownerList.data.list.some(row => row.name === deniedName)).to.equal(false);
    }
  });

  it('isolates nested run list and run detail by tenant', async function () {
    expectProductError(await listRuns(scheduleId, OTHER_TENANT), CODE_NOT_FOUND);
    expectProductError(await readRun(scheduleId, manualRun.run_id, OTHER_TENANT), CODE_NOT_FOUND);
  });

  it('deletes with the current revision and verifies exact not-found cleanup', async function () {
    terminalRun = await waitForTerminalRun(scheduleId, manualRun.run_id);
    await waitForInactiveScheduleWork(scheduleId);
    cleanupVerified = await deleteScheduleExact(scheduleId);
    expect(cleanupVerified).to.equal(true);
    scheduleId = null;
  });
});
