/**
 * Activate the explicitly synthetic RDI fixture through the public API.
 *
 * This is intentionally separate from the SQL seed.  The seed creates an
 * inactive/unbound pre-registration row; this module proves that the API
 * activation contract changes it to active/enabled for the tenant account.
 * It never creates or activates an ordinary device and it never prints the
 * API response wholesale (the response may contain fixture details).
 */

const apiClient = require('../lib/api_client');

const PROVENANCE = 'synthetic-rdi';

function env(name, fallback = '') {
  return String(process.env[name] || fallback).trim();
}

function redact(value) {
  return String(value || '')
    .replace(/Bearer\s+[A-Za-z0-9._-]+/gi, 'Bearer [REDACTED]')
    .replace(/(password|passwd|secret|token|authorization|cookie)\s*[:=]\s*[^\s,}]+/gi, '$1=[REDACTED]');
}

function fixturePid() {
  const pid = env('AETHERLINK_RDI_FIXTURE_PID', env('SYNTHETIC_RDI_PID')).toUpperCase();
  if (!/^SYN[A-Z0-9]{9}$/.test(pid)) {
    throw new Error('synthetic activation requires a SYN-prefixed 12-character fixture PID');
  }
  return pid;
}

function listFromResponse(response) {
  if (!response || Number(response.code) !== 200 || !response.data) return [];
  if (Array.isArray(response.data)) return response.data;
  if (Array.isArray(response.data.list)) return response.data.list;
  if (Array.isArray(response.data.data)) return response.data.data;
  return [];
}

function rowPid(row) {
  return String(
    row && (row.device_number || row.deviceNumber || row.pid_number || row.pidNumber || '')
  ).trim().toUpperCase();
}

function rowId(row) {
  return String(row && (row.id || row.device_id || row.deviceId || '')).trim();
}

function parseObject(value) {
  if (!value) return null;
  if (typeof value === 'object') return value;
  try {
    return JSON.parse(value);
  } catch (_) {
    return null;
  }
}

function assertSyntheticDetail(detail, pid, expectedId = '') {
  if (!detail || typeof detail !== 'object') throw new Error('synthetic activation detail is missing');
  const actualPid = rowPid(detail);
  if (actualPid !== pid) {
    throw new Error(`synthetic activation PID mismatch: got ${actualPid || '<empty>'}, expected ${pid}`);
  }
  if (expectedId && rowId(detail) !== expectedId) {
    throw new Error(`synthetic activation device ID mismatch: got ${rowId(detail) || '<empty>'}, expected ${expectedId}`);
  }
  const additional = parseObject(detail.additional_info || detail.additionalInfo);
  if (!additional || additional.fixture_provenance !== PROVENANCE) {
    throw new Error('activation target is not explicitly marked synthetic-rdi');
  }
  return {
    id: rowId(detail) || expectedId,
    pid,
    activate_flag: String(detail.activate_flag || detail.activateFlag || '').trim().toLowerCase(),
    is_enabled: String(detail.is_enabled || detail.isEnabled || '').trim().toLowerCase(),
    fixture_id: String(additional.fixture_id || '')
  };
}

async function readDetailById(id, pid) {
  if (!id) return null;
  const response = await apiClient.get(`/device/detail/${encodeURIComponent(id)}`, {}, 'tenant_admin');
  if (Number(response && response.code) !== 200) return null;
  return assertSyntheticDetail(response.data, pid, id);
}

async function findVisibleFixture(pid) {
  const response = await apiClient.get('/device', { page: 1, page_size: 100, device_number: pid }, 'tenant_admin');
  for (const row of listFromResponse(response)) {
    if (rowPid(row) !== pid) continue;
    const detail = await readDetailById(rowId(row), pid);
    if (detail) return detail;
  }

  const configuredId = env('SYNTHETIC_RDI_DEVICE_ID');
  return readDetailById(configuredId, pid);
}

function activationResult(action, pid, detail, extra = {}) {
  if (!detail || detail.activate_flag !== 'active' || detail.is_enabled !== 'enabled') {
    throw new Error(
      `synthetic activation did not produce active/enabled state: ${JSON.stringify(detail || {})}`
    );
  }
  return {
    mode: PROVENANCE,
    evidence_class: 'api-activation',
    claim_scope: 'isolated-software-path-only',
    real_rdi_status: 'not-tested',
    production_signoff: 'not-ready',
    action,
    endpoint: 'POST /api/v1/rdi/devices/activate',
    pid,
    device_id: detail.id,
    activate_flag: detail.activate_flag,
    is_enabled: detail.is_enabled,
    fixture_id: detail.fixture_id,
    ...extra
  };
}

async function main() {
  const pid = fixturePid();
  const name = `Synthetic RDI ${pid}`;
  const response = await apiClient.post('/rdi/devices/activate', { pid_number: pid, name }, 'tenant_admin');
  const code = Number(response && response.code);

  if (code === 200) {
    const responseId = String(response.data && (response.data.device_id || response.data.id || '')).trim();
    const detail = await readDetailById(responseId || env('SYNTHETIC_RDI_DEVICE_ID'), pid);
    if (!detail) throw new Error('activation returned success but the activated fixture detail was not readable');
    process.stdout.write(`${JSON.stringify(activationResult('activated-this-run', pid, detail, {
      response_code: code,
      duplicate_activation: false
    }))}\n`);
    return;
  }

  if (code === 204002) {
    const detail = await findVisibleFixture(pid);
    if (!detail) throw new Error('API reported already active but the synthetic fixture is not visible to tenant_admin');
    process.stdout.write(`${JSON.stringify(activationResult('reused-existing', pid, detail, {
      response_code: code,
      duplicate_activation: true
    }))}\n`);
    return;
  }

  const message = redact(response && response.message ? response.message : JSON.stringify(response || {}));
  throw new Error(`synthetic API activation failed with code ${code || '<empty>'}: ${message}`);
}

if (require.main === module) {
  main().catch(error => {
    process.stderr.write(`synthetic API activation failed: ${redact(error && error.message)}\n`);
    process.exitCode = 1;
  });
}

module.exports = {
  PROVENANCE,
  fixturePid,
  assertSyntheticDetail,
  activationResult
};
