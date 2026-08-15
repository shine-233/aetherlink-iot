/*
 * Purpose: run the narrow, software-only synthetic RDI MQTT/API lane.
 *
 * This script deliberately does not seed a database, activate a device, or
 * claim a physical RDI result. It expects an already prepared isolated
 * backend/broker and an explicitly selected synthetic fixture. The only
 * network peer it starts is the repository's protocol emulator.
 */

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');

const apiClient = require('../lib/api_client');

const PROVENANCE = 'synthetic-rdi';
const EVIDENCE_CLASS = 'protocol-emulator';
const DEFAULT_PID = 'SYNTHRDI0001';
const DEFAULT_DEVICE_ID = '64afc1ec-8a74-4a85-ae8f-5727ff52d720';
const DEFAULT_BROKER = '127.0.0.1:11086';
const DEFAULT_DURATION = '12s';
const DEFAULT_IDENTIFIER = 'test_dry_contact';

function env(name, fallback = '') {
  const value = process.env[name];
  return value === undefined || value === '' ? fallback : value;
}

function resolvePath(value, fallback) {
  const selected = env(value, fallback);
  return path.isAbsolute(selected) ? selected : path.resolve(process.cwd(), selected);
}

function validateLoopbackBroker(value) {
  const broker = String(value || '').trim();
  const match = broker.match(/^(127\.0\.0\.1|localhost|\[::1\]):(\d{1,5})$/i);
  if (!match || Number(match[2]) < 1 || Number(match[2]) > 65535) {
    throw new Error(`synthetic protocol validation only permits an explicit loopback broker host:port, got ${broker || '<empty>'}`);
  }
  return broker;
}

function fixturePid() {
  const pid = env('AETHERLINK_RDI_FIXTURE_PID', env('SYNTHETIC_RDI_PID', DEFAULT_PID)).trim().toUpperCase();
  if (!/^SYN[A-Z0-9]{9}$/.test(pid)) {
    throw new Error('AETHERLINK_RDI_FIXTURE_PID must use the SYN namespace and contain 12 alphanumeric characters');
  }
  return pid;
}

function fixtureDeviceId() {
  const deviceId = env('SYNTHETIC_RDI_DEVICE_ID', DEFAULT_DEVICE_ID).trim();
  if (!deviceId || /[\/# +]/.test(deviceId)) {
    throw new Error('SYNTHETIC_RDI_DEVICE_ID must be a non-empty MQTT topic-safe value');
  }
  return deviceId;
}

function evidenceFields(extra = {}) {
  const result = {
    evidence_class: EVIDENCE_CLASS,
    fixture_provenance: PROVENANCE,
    device_execution: 'not-proven',
    real_rdi_status: 'not-tested',
    ...extra
  };
  if (result.fixture_provenance !== PROVENANCE) {
    throw new Error(`synthetic evidence fixture_provenance must remain ${PROVENANCE}`);
  }
  if (result.real_rdi_status !== 'not-tested') {
    throw new Error('synthetic evidence real_rdi_status must remain not-tested');
  }
  if (result.device_execution !== 'not-proven') {
    throw new Error('synthetic evidence device_execution must remain not-proven');
  }
  if (Object.prototype.hasOwnProperty.call(result, 'production_signoff') && result.production_signoff !== 'not-ready') {
    throw new Error('synthetic evidence production_signoff must remain not-ready');
  }
  if (typeof result.verdict === 'string' && /^real-rdi[-_ ]?passed$/i.test(result.verdict)) {
    throw new Error('synthetic evidence verdict cannot claim real-rdi passed');
  }
  return result;
}

function hash(value) {
  return crypto.createHash('sha256').update(String(value)).digest('hex');
}

function redactText(value, secrets = []) {
  let text = String(value || '');
  for (const secret of secrets) {
    if (secret) text = text.split(String(secret)).join('[REDACTED]');
  }
  return text
    .replace(/Bearer\s+[A-Za-z0-9._-]+/gi, 'Bearer [REDACTED]')
    .replace(/(password|passwd|secret|token|authorization|cookie)\s*[:=]\s*[^\s,}]+/gi, '$1=[REDACTED]');
}

function writeJson(filePath, value) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`, 'utf8');
}

function asList(response) {
  if (!response || response.code !== 200 || !response.data) return [];
  if (Array.isArray(response.data)) return response.data;
  if (Array.isArray(response.data.list)) return response.data.list;
  if (Array.isArray(response.data.data)) return response.data.data;
  return [];
}

function responseError(response, label) {
  if (response && response.code === 200) return null;
  const message = response && response.message ? response.message : JSON.stringify(response || {});
  return new Error(`${label} failed: ${redactText(message)}`);
}

function isOnline(row) {
  if (!row) return false;
  const value = row.online ?? row.is_online ?? row.isOnline ?? row.online_status ?? row.status;
  return value === true || value === 1 || value === '1' || String(value).toLowerCase() === 'online';
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

function timestampSeconds(value) {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value > 1e12 ? value / 1000 : value;
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const numeric = Number(value);
    if (Number.isFinite(numeric)) return numeric > 1e12 ? numeric / 1000 : numeric;
    const parsed = Date.parse(value);
    if (Number.isFinite(parsed)) return parsed / 1000;
  }
  return NaN;
}

function parseVoucher(detail) {
  const parsed = parseObject(detail && detail.voucher);
  if (!parsed || !parsed.username) {
    throw new Error('synthetic fixture detail did not expose a JSON MQTT voucher username');
  }
  return {
    username: String(parsed.username).trim(),
    password: parsed.password === undefined || parsed.password === null ? '' : String(parsed.password)
  };
}

function fixtureProvenance(detail) {
  const additional = parseObject(detail && (detail.additional_info || detail.additionalInfo));
  return String(additional && additional.fixture_provenance || '').trim();
}

function validateSyntheticFixtureDetail(detail, expectedPid, expectedDeviceId) {
  if (!detail || typeof detail !== 'object') {
    throw new Error('synthetic fixture detail is missing');
  }
  const actualPid = String(detail.device_number || detail.deviceNumber || detail.pid_number || '').trim().toUpperCase();
  const pid = String(expectedPid || '').trim().toUpperCase();
  if (!/^SYN[A-Z0-9]{9}$/.test(pid) || actualPid !== pid) {
    throw new Error(`fixture identity mismatch: API device_number=${actualPid || '<empty>'}, expected=${pid || '<empty>'}`);
  }
  const deviceId = String(expectedDeviceId || '').trim();
  const actualDeviceId = String(detail.id || detail.device_id || detail.deviceId || '').trim();
  if (!deviceId || actualDeviceId !== deviceId) {
    throw new Error(`fixture device ID mismatch: API id=${actualDeviceId || '<empty>'}, expected=${deviceId || '<empty>'}`);
  }

  const additional = parseObject(detail.additional_info || detail.additionalInfo);
  if (!additional || additional.fixture_provenance !== PROVENANCE) {
    throw new Error('fixture_provenance must be explicitly synthetic-rdi');
  }
  if (additional.fixture_pid !== pid) {
    throw new Error(`synthetic fixture_pid mismatch: ${String(additional.fixture_pid || '<empty>')}`);
  }
  if (!String(additional.fixture_id || '').startsWith(`${pid}-`)) {
    throw new Error('synthetic fixture_id must be scoped to the expected PID');
  }
  if (additional.connection_type !== PROVENANCE) {
    throw new Error('synthetic fixture connection_type must be synthetic-rdi');
  }

  const hardware = additional.hardware_identity;
  if (!hardware || hardware.kind !== 'synthetic' || !String(hardware.serial || '').startsWith('SYNTH-HW-')) {
    throw new Error('synthetic fixture hardware identity must explicitly declare kind=synthetic and a SYNTH-HW serial');
  }

  const voucher = parseVoucher(detail);
  if (!/^synthetic-rdi-/.test(voucher.username)) {
    throw new Error('voucher username must be scoped to the synthetic fixture');
  }
  if (!voucher.password) {
    throw new Error('synthetic fixture voucher password is empty; refusing to run live emulator');
  }

  const activateFlag = String(detail.activate_flag || detail.activateFlag || '').trim().toLowerCase();
  const isEnabled = String(detail.is_enabled || detail.isEnabled || '').trim().toLowerCase();
  if (activateFlag !== 'active' || isEnabled !== 'enabled') {
    throw new Error(`synthetic fixture must be active/enabled before protocol validation; got activate_flag=${activateFlag || '<empty>'}, is_enabled=${isEnabled || '<empty>'}`);
  }

  return {
    fixtureId: String(additional.fixture_id),
    hardware: { kind: hardware.kind, serial: String(hardware.serial) },
    activation: { activateFlag, isEnabled, action: 'not-executed' },
    voucherUsername: voucher.username
  };
}

function assertSyntheticStateTransition(states, label = 'synthetic emulator') {
  const beforeOffline = !isOnline(states && states.before);
  const onlineTransition = isOnline(states && states.online);
  const offlineTransition = !isOnline(states && states.offline);
  if (!beforeOffline) throw new Error(`${label} must be offline before emulator start`);
  if (!onlineTransition) throw new Error(`${label} online transition was not observed`);
  if (!offlineTransition) throw new Error(`${label} offline transition was not observed after emulator exit`);
  return { beforeOffline, onlineTransition, offlineTransition };
}

async function readDetail(deviceId) {
  const response = await apiClient.get(`/device/detail/${deviceId}`, {}, 'tenant_admin');
  const error = responseError(response, 'read synthetic fixture detail');
  if (error) throw error;
  return response.data || {};
}

async function waitFor(label, read, predicate, timeoutMs = 10000, intervalMs = 250) {
  const deadline = Date.now() + timeoutMs;
  let last = null;
  while (Date.now() < deadline) {
    last = await read();
    if (predicate(last)) return last;
    await new Promise(resolve => setTimeout(resolve, intervalMs));
  }
  throw new Error(`${label} was not observed before timeout; last=${redactText(JSON.stringify(last))}`);
}

function startEmulator({ binary, broker, pid, deviceId, voucher, ackMode, duration, logDir }) {
  const args = [
    '-mode', 'device',
    '-allow-network',
    '-broker', broker,
    '-pid', pid,
    '-device-id', deviceId,
    '-username', voucher.username,
    '-password', voucher.password,
    '-ack-mode', ackMode,
    '-duration', duration
  ];
  const child = spawn(binary, args, {
    cwd: path.resolve(__dirname, '..', '..', 'backend'),
    windowsHide: true,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env }
  });
  let stdout = '';
  let stderr = '';
  child.stdout.on('data', chunk => { stdout += chunk.toString(); });
  child.stderr.on('data', chunk => { stderr += chunk.toString(); });
  const exit = new Promise(resolve => {
    child.once('error', error => resolve({ code: null, signal: null, error: error.message }));
    child.once('exit', (code, signal) => resolve({ code, signal }));
  });
  const persistLogs = async () => {
    fs.mkdirSync(logDir, { recursive: true });
    const secrets = [voucher.password, voucher.username];
    fs.writeFileSync(path.join(logDir, `emulator-${ackMode}.stdout.log`), redactText(stdout, secrets), 'utf8');
    fs.writeFileSync(path.join(logDir, `emulator-${ackMode}.stderr.log`), redactText(stderr, secrets), 'utf8');
  };
  return { child, exit, persistLogs };
}

async function stopEmulator(runtime) {
  if (!runtime) return { code: null, signal: null };
  const current = await Promise.race([
    runtime.exit,
    new Promise(resolve => setTimeout(() => resolve(null), 100))
  ]);
  // Let -duration drive a normal device shutdown first. On Windows,
  // child.kill('SIGTERM') is implemented as process termination and does not
  // give the emulator a chance to publish its envelope-compatible offline
  // status frame. Only force termination after the configured grace period.
  const gracefulTimeoutMs = Number(env('SYNTHETIC_RDI_GRACEFUL_STOP_TIMEOUT_MS', '30000'));
  if (current) {
    await runtime.persistLogs();
    return current;
  }
  const result = await Promise.race([
    runtime.exit,
    new Promise(resolve => setTimeout(() => resolve(null), Number.isFinite(gracefulTimeoutMs) ? gracefulTimeoutMs : 30000))
  ]);
  if (!result) {
    runtime.child.kill('SIGTERM');
  }
  await runtime.persistLogs();
  return result || { code: null, signal: 'forced' };
}

async function readTelemetry(deviceId, startedAtSeconds) {
  const response = await apiClient.get(`/telemetry/datas/current/${deviceId}`, {}, 'tenant_admin');
  const error = responseError(response, 'read current synthetic telemetry');
  if (error) throw error;
  const rows = asList(response);
  const row = rows.find(item => item && item.key === 'temperature_1');
  if (!row) return null;
  const value = typeof row.value === 'string' ? Number(JSON.parse(row.value)) : Number(row.value);
  const ts = timestampSeconds(row.ts ?? row.timestamp);
  if (value !== 25.5 || !Number.isFinite(ts) || ts <= startedAtSeconds) return null;
  return { key: row.key, value, ts };
}

async function readCommandLog(deviceId, messageId, identifier) {
  const response = await apiClient.get('/command/datas/set/logs', {
    device_id: deviceId,
    identify: identifier,
    page: 1,
    page_size: 100
  }, 'tenant_admin');
  const error = responseError(response, 'read command set log');
  if (error) throw error;
  const row = asList(response).find(item => String(item && item.message_id || '') === String(messageId));
  if (!row) return null;
  const responseData = parseObject(row.rsp_data || row.response_data);
  return {
    status: String(row.status || ''),
    identify: String(row.identify || row.data_identifier || identifier),
    response: responseData ? {
      result: responseData.result,
      message: responseData.message,
      method: responseData.method
    } : null,
    error_message: row.error_message ? String(row.error_message) : ''
  };
}

async function runAckCase({ ackMode, binary, broker, pid, deviceId, voucher, duration, logDir }) {
  const startedAt = Math.floor(Date.now() / 1000);
  const before = await readDetail(deviceId);
  if (isOnline(before)) {
    throw new Error(`${ackMode} emulator fixture must be offline before emulator start`);
  }
  const runtime = startEmulator({ binary, broker, pid, deviceId, voucher, ackMode, duration, logDir });
  let detail;
  let commandResponse;
  let commandLog;
  let telemetry;
  let exit;
  try {
    detail = await waitFor(`${ackMode} emulator online status`, () => readDetail(deviceId), isOnline, 10000);
    telemetry = await waitFor(`${ackMode} fresh telemetry`, () => readTelemetry(deviceId, startedAt), value => Boolean(value), 10000);
    commandResponse = await apiClient.post(`/rdi/devices/${deviceId}/commands`, {
      identifier: DEFAULT_IDENTIFIER,
      params: { level: 'high', duration_seconds: 5 }
    }, 'tenant_admin');
    const commandError = responseError(commandResponse, `submit ${ackMode} RDI command`);
    if (commandError) throw commandError;
    const messageId = commandResponse.data && commandResponse.data.message_id;
    if (!messageId) throw new Error(`${ackMode} command response did not include message_id`);
    commandLog = await waitFor(`${ackMode} terminal command ACK`, () => readCommandLog(deviceId, messageId, DEFAULT_IDENTIFIER), row => {
      return row && ['3', '4'].includes(row.status);
    }, 10000);
    const expectedStatus = ackMode === 'success' ? '3' : '4';
    const expectedResult = ackMode === 'success' ? 0 : 1;
    if (commandLog.status !== expectedStatus) {
      throw new Error(`${ackMode} command log status=${commandLog.status}, expected=${expectedStatus}`);
    }
    if (!commandLog.response || Number(commandLog.response.result) !== expectedResult) {
      throw new Error(`${ackMode} ACK result did not match expected ${expectedResult}`);
    }
    if (commandLog.response.message !== (ackMode === 'success' ? 'success' : 'failed')) {
      throw new Error(`${ackMode} ACK message did not match canonical emulator payload`);
    }
  } finally {
    exit = await stopEmulator(runtime);
  }

  if (!exit || exit.code !== 0 || exit.signal !== null) {
    throw new Error(`${ackMode} emulator did not exit cleanly: ${redactText(JSON.stringify(exit))}`);
  }

  const offline = await waitFor(`${ackMode} emulator offline status`, () => readDetail(deviceId), detailRow => !isOnline(detailRow), 10000);
  assertSyntheticStateTransition({ before, online: detail, offline }, `${ackMode} emulator`);
  const messageId = commandResponse && commandResponse.data && commandResponse.data.message_id;
  return evidenceFields({
    ack_mode: ackMode,
    broker,
    pid,
    device_id: deviceId,
    voucher_username: voucher.username,
    message_id_hash: messageId ? hash(messageId) : null,
    command: {
      identifier: DEFAULT_IDENTIFIER,
      status: commandLog && commandLog.status,
      response: commandLog && commandLog.response,
      error_message_present: Boolean(commandLog && commandLog.error_message)
    },
    online_observed: isOnline(detail),
    state_transition: {
      before_offline: !isOnline(before),
      online_transition: isOnline(detail),
      offline_transition: !isOnline(offline)
    },
    telemetry,
    offline_observed: !isOnline(offline),
    emulator_exit: exit,
    observed_at: new Date().toISOString()
  });
}

async function main() {
  const pid = fixturePid();
  const deviceId = fixtureDeviceId();
  const broker = validateLoopbackBroker(env('SYNTHETIC_RDI_BROKER', DEFAULT_BROKER));
  const binary = resolvePath(
    'SYNTHETIC_RDI_EMULATOR_BIN',
    path.resolve(__dirname, '..', '..', 'backend', '_localrun', 'current-goal-20260812', 'synthetic-rdi', 'synthetic-rdi-protocol-emulator.exe')
  );
  const reportDir = resolvePath('SYNTHETIC_RDI_REPORT_DIR', path.resolve(process.cwd(), 'verification', `synthetic-rdi-${Date.now()}`));
  const logDir = path.join(reportDir, 'raw');
  if (!fs.existsSync(binary)) throw new Error(`synthetic protocol emulator binary is missing: ${binary}`);

  const healthy = await apiClient.healthCheck();
  if (!healthy) throw new Error('configured isolated backend health endpoint is unavailable');

  const detail = await readDetail(deviceId);
  const fixture = validateSyntheticFixtureDetail(detail, pid, deviceId);
  const voucher = parseVoucher(detail);

  const manifest = evidenceFields({
    schema: 'aetherlink.synthetic-rdi.protocol-validation.v1',
    pid,
    device_id: deviceId,
    voucher_username: voucher.username,
    voucher_secret_redacted: true,
    fixture_id: fixture.fixtureId,
    hardware_identity: fixture.hardware,
    activation: fixture.activation,
    backend: apiClient.getConfig().baseURL,
    broker,
    binary: path.relative(process.cwd(), binary),
    started_at: new Date().toISOString()
  });
  writeJson(path.join(reportDir, 'manifest.json'), manifest);

  const results = {};
  for (const ackMode of ['success', 'failure']) {
    results[ackMode] = await runAckCase({
      ackMode,
      binary,
      broker,
      pid,
      deviceId,
      voucher,
      duration: env('SYNTHETIC_RDI_EMULATOR_DURATION', DEFAULT_DURATION),
      logDir
    });
    writeJson(path.join(reportDir, `${ackMode}-ack.json`), results[ackMode]);
  }

  const summary = evidenceFields({
    schema: 'aetherlink.synthetic-rdi.protocol-validation.summary.v1',
    verdict: 'software-path-passed',
    production_signoff: 'not-ready',
    claim_scope: 'isolated-software-path-only',
    real_rdi_status: 'not-tested',
    cases: results,
    limitations: [
      'PID, voucher, hardware identity and telemetry are synthetic fixtures',
      'MQTT session is generated by protocol-emulator, not real firmware',
      'ACK is generated by protocol-emulator, not physical hardware',
      'fail-once helper contract is not evidence of backend automatic retry'
    ],
    finished_at: new Date().toISOString()
  });
  writeJson(path.join(reportDir, 'summary.json'), summary);
  process.stdout.write(`${JSON.stringify({ report_dir: reportDir, verdict: summary.verdict, cases: Object.keys(results) })}\n`);
}

if (require.main === module) {
  main().catch(error => {
    process.stderr.write(`synthetic-rdi protocol validation failed: ${redactText(error && error.message)}\n`);
    process.exitCode = 1;
  });
}

module.exports = {
  PROVENANCE,
  evidenceFields,
  validateSyntheticFixtureDetail,
  assertSyntheticStateTransition,
  isOnline,
  redactText,
  validateLoopbackBroker
};
