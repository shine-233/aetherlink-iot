/**
 * 文件用途：用于支撑 automation_tests 的自动化种子数据定义模块。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：共享库变更会影响多类自动化套件，必须保持错误信息和前置条件可诊断。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const apiClient = require('./api_client');
const testData = require('./test_data');
const fs = require('fs');
const os = require('os');
const path = require('path');
const net = require('net');
const crypto = require('crypto');
const { execFile, spawn } = require('child_process');
const { promisify } = require('util');
const mqttRuntime = require('./mqtt_runtime');

const execFileAsync = promisify(execFile);

function randomFixtureToken(prefix) {
  return `${prefix}-${crypto.randomBytes(16).toString('hex')}`;
}

function resolveOtaSeedDatabaseOptions(env = process.env) {
  const aetherlinkDatabase = String(env.AETHERLINK_DB_NAME || '').trim();
  const gotpDatabase = String(env.GOTP_DB_PSQL_DBNAME || '').trim();
  const pgDatabase = String(env.PGDATABASE || '').trim();
  const strictTarget = String(env.AETHERLINK_STRICT_DB_TARGET || '').trim() === '1';

  if (strictTarget && aetherlinkDatabase && gotpDatabase && aetherlinkDatabase !== gotpDatabase) {
    throw new Error(
      'AETHERLINK_STRICT_DB_TARGET=1 requires AETHERLINK_DB_NAME and GOTP_DB_PSQL_DBNAME to match'
    );
  }

  const database = aetherlinkDatabase || gotpDatabase || pgDatabase;
  if (strictTarget && !database) {
    throw new Error(
      'AETHERLINK_STRICT_DB_TARGET=1 requires an explicit database target via ' +
      'AETHERLINK_DB_NAME, GOTP_DB_PSQL_DBNAME, or PGDATABASE'
    );
  }

  return {
    host: env.AETHERLINK_DB_HOST || env.GOTP_DB_PSQL_HOST || '127.0.0.1',
    port: env.AETHERLINK_DB_PORT || env.GOTP_DB_PSQL_PORT || '5432',
    user: env.AETHERLINK_DB_USER || env.GOTP_DB_PSQL_USERNAME || env.PGUSER || 'postgres',
    database: database || 'aetherlink_iot_local',
    password: env.PGPASSWORD || env.AETHERLINK_DB_PASSWORD || env.GOTP_DB_PSQL_PASSWORD || ''
  };
}

function getOtaSeedDatabaseOptions() {
  return resolveOtaSeedDatabaseOptions(process.env);
}

function pickId(row) {
  return row && (
    row.id ||
    row.ID ||
    row.device_id ||
    row.DeviceID ||
    row.scene_id ||
    row.SceneID ||
    row.scene_automation_id ||
    row.SceneAutomationID ||
    null
  );
}

function pickOtaPackageId(row) {
  return row && (
    row.ota_upgrade_package_id ||
    row.OtaUpgradePackageID ||
    row.OTAUpgradePackageID ||
    row.package_id ||
    row.PackageID ||
    row.id ||
    row.ID ||
    null
  );
}

function pickOtaTaskId(row) {
  return row && (
    row.ota_upgrade_task_id ||
    row.OtaUpgradeTaskID ||
    row.OTAUpgradeTaskID ||
    row.task_id ||
    row.TaskID ||
    row.id ||
    row.ID ||
    null
  );
}

function pickAlarmHistoryId(row) {
  return row && (
    row.id ||
    row.ID ||
    row.alarm_history_id ||
    row.AlarmHistoryID ||
    null
  );
}

function listFromResponse(resp) {
  if (!resp || resp.code !== 200 || !resp.data) return [];
  if (Array.isArray(resp.data)) return resp.data;
  if (Array.isArray(resp.data.list)) return resp.data.list;
  if (Array.isArray(resp.data.data)) return resp.data.data;
  return [];
}

function requireSuccess(resp, label) {
  if (!resp || resp.code !== 200) {
    const message = resp && resp.message ? resp.message : JSON.stringify(resp);
    throw new Error(label + ' failed: ' + message);
  }
  return resp;
}

let runLabelSequence = 0;

function makeRunLabel(prefix) {
  const timestamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
  // Keep the longest seed name within the backend's 36-character limit while
  // avoiding same-second collisions between Playwright workers.
  const processPart = process.pid.toString(36).slice(-1);
  const sequencePart = (runLabelSequence++ % 36).toString(36);
  const randomPart = crypto.randomBytes(2).toString('hex').slice(0, 3);
  const uniquePart = `${processPart}${sequencePart}${randomPart}`;
  const prefixLimit = 36 - timestamp.length - uniquePart.length - 2;
  const boundedPrefix = String(prefix).slice(0, prefixLimit);
  return `${boundedPrefix}_${timestamp}_${uniquePart}`;
}

function alarmHistoryMatchesSceneSeed(row, seed) {
  if (!row || !seed || !pickAlarmHistoryId(row)) return false;

  const rowAlarmConfigId = row.alarm_config_id || row.AlarmConfigID || '';
  const rowSceneId = row.scene_automation_id || row.SceneAutomationID || row.scene_id || '';
  return rowAlarmConfigId === seed.alarmConfigId || rowSceneId === seed.id;
}

function isConfiguredPid(row) {
  const configured = [
    testData.getDevicePID('activated_pid'),
    testData.getDevicePID('activated_pid_2'),
    testData.getDevicePID('inactive_pid')
  ].filter(Boolean);
  const syntheticPid = getOptInSyntheticRdiPid();
  if (syntheticPid) configured.push(syntheticPid);
  const rowPid = row && (row.pid_number || row.device_number || row.DeviceNumber || row.PIDNumber || '');
  return configured.includes(rowPid);
}

/**
 * Synthetic RDI is an explicit isolated-test mode.  Keeping its PID out of
 * the normal candidate list prevents a local fixture from silently becoming
 * a substitute for a real configured controller in ordinary runs.
 */
function getOptInSyntheticRdiPid() {
  if (process.env.AETHERLINK_RDI_FIXTURE_MODE !== 'synthetic-rdi') return '';
  const pid = String(
    process.env.AETHERLINK_RDI_FIXTURE_PID ||
    process.env.SYNTHETIC_RDI_PID ||
    ''
  ).trim().toUpperCase();
  if (!pid) {
    throw new Error('AETHERLINK_RDI_FIXTURE_MODE=synthetic-rdi requires AETHERLINK_RDI_FIXTURE_PID or SYNTHETIC_RDI_PID');
  }
  if (!/^[A-Z0-9]{12}$/.test(pid)) {
    throw new Error(`Synthetic RDI fixture PID must be exactly 12 alphanumeric characters: ${pid}`);
  }
  return pid;
}

function getRdiCandidatePids() {
  return [
    getOptInSyntheticRdiPid(),
    testData.getDevicePID('inactive_pid'),
    testData.getDevicePID('activated_pid'),
    testData.getDevicePID('activated_pid_2')
  ].filter(Boolean);
}

/**
 * Ready Check command handoff is a shared-device flow, not an RDI customer
 * detail flow.  RDI devices intentionally replace the generic operation tabs
 * with the four customer tabs, so they do not mount the ready-check collector.
 * Keep the classification aligned with the frontend tab plan and the seeded
 * RDI PID list before choosing a browser fixture device.
 */
function isRdiDeviceRow(row) {
  if (!row) return false;

  const deviceNumber = String(
    row.device_number || row.deviceNumber || row.DeviceNumber || row.pid_number || row.PIDNumber || ''
  ).trim();
  if (/^[A-Za-z0-9]{12}$/.test(deviceNumber) || isConfiguredPid(row)) return true;

  const rawAdditionalInfo = row.additional_info || row.additionalInfo || row.AdditionalInfo;
  if (!rawAdditionalInfo) return false;
  try {
    const additionalInfo = typeof rawAdditionalInfo === 'string'
      ? JSON.parse(rawAdditionalInfo)
      : rawAdditionalInfo;
    return Boolean(additionalInfo && (additionalInfo.rdi_config || additionalInfo.rdi_system_info));
  } catch {
    return false;
  }
}

function isOnlineDevice(row) {
  if (!row) return false;
  return row.is_online === 1 || row.is_online === true ||
    row.device_status === 1 || row.device_status === true ||
    String(row.is_online || '').toLowerCase() === 'true' ||
    String(row.device_status || '').toLowerCase() === 'online';
}

/**
 * Ready Check must run against a deliberately selected live command device.
 * Reusing an arbitrary shared row makes an offline device look like a broken
 * browser flow and can also race another fixture while its config is restored.
 * Keep this policy pure so the contract can be tested without mutating the
 * backend; the caller still performs the real API binding and preview.
 */
function selectReadyCheckDevice(detailedDevices, requestedDeviceId = '') {
  const requestedId = String(requestedDeviceId || '').trim();
  if (!requestedId) {
    return {
      blocked: true,
      reason: 'AUTOMATION_READY_CHECK_DEVICE_ID must identify a live non-RDI command emulator device; shared-device fallback is disabled'
    };
  }

  const row = (Array.isArray(detailedDevices) ? detailedDevices : [])
    .find(candidate => pickId(candidate) === requestedId);
  if (!row) {
    return {
      blocked: true,
      reason: `AUTOMATION_READY_CHECK_DEVICE_ID ${requestedId} is not visible to the test account`
    };
  }
  if (isRdiDeviceRow(row)) {
    return {
      invalid: true,
      reason: `AUTOMATION_READY_CHECK_DEVICE_ID ${requestedId} is an RDI device; the Ready Check command handoff requires a non-RDI device with the generic operation tabs`
    };
  }
  if (!isOnlineDevice(row)) {
    return {
      blocked: true,
      reason: `AUTOMATION_READY_CHECK_DEVICE_ID ${requestedId} is visible but offline; start the real MQTT command emulator before running Ready Check evidence`
    };
  }

  return {
    device: {
      id: pickId(row),
      row,
      created: false,
      cleanup: async () => {}
    }
  };
}

function isLoopbackHost(hostname) {
  const value = String(hostname || '').trim().toLowerCase();
  return value === 'localhost' || value === '::1' || value === '0:0:0:0:0:0:0:1' ||
    value === '127.0.0.1' || value.startsWith('127.');
}

function readyCheckAutoStartEnabled() {
  const configured = String(process.env.AUTOMATION_READY_CHECK_AUTO_START || '').trim().toLowerCase();
  if (['0', 'false', 'no', 'off'].includes(configured)) return false;
  if (['1', 'true', 'yes', 'on'].includes(configured)) return true;

  try {
    const baseURL = String(apiClient.getConfig()?.baseURL || '');
    return isLoopbackHost(new URL(baseURL).hostname);
  } catch (_) {
    return false;
  }
}

function readyCheckReportDirectory() {
  const configured = String(process.env.AUTOMATION_REPORT_DIR || '').trim();
  const directory = configured
    ? (path.isAbsolute(configured) ? configured : path.resolve(process.cwd(), configured))
    : path.resolve(__dirname, '..', '_localrun', 'ready-check-runtime');
  fs.mkdirSync(directory, { recursive: true });
  return directory;
}

function resolveReadyCheckEmulatorBinary() {
  const configured = String(process.env.AUTOMATION_READY_CHECK_EMULATOR_BIN || '').trim();
  if (configured) {
    return path.isAbsolute(configured) ? configured : path.resolve(process.cwd(), configured);
  }
  return path.resolve(
    __dirname,
    '..',
    '..',
    'backend',
    'cmd',
    'aetherlink-device-autotest',
    '_localrun',
    'ready-check-command-emulator.exe'
  );
}

async function ensureReadyCheckEmulatorBinary() {
  const configured = String(process.env.AUTOMATION_READY_CHECK_EMULATOR_BIN || '').trim();
  const binaryPath = resolveReadyCheckEmulatorBinary();
  if (fs.existsSync(binaryPath)) return { path: binaryPath };
  if (configured) {
    return {
      blocked: true,
      reason: `AUTOMATION_READY_CHECK_EMULATOR_BIN does not exist: ${binaryPath}`
    };
  }
  if (['0', 'false', 'no', 'off'].includes(String(process.env.AUTOMATION_READY_CHECK_BUILD_EMULATOR || '').trim().toLowerCase())) {
    return {
      blocked: true,
      reason: `Ready Check command emulator binary is missing: ${binaryPath}`
    };
  }

  const moduleRoot = path.resolve(__dirname, '..', '..', 'backend', 'cmd', 'aetherlink-device-autotest');
  fs.mkdirSync(path.dirname(binaryPath), { recursive: true });
  try {
    await execFileAsync(process.env.AUTOMATION_GO_BIN || 'go', [
      'build',
      '-o',
      binaryPath,
      './cmd/autotest'
    ], {
      cwd: moduleRoot,
      windowsHide: true,
      timeout: 120000,
      env: { ...process.env }
    });
  } catch (error) {
    return {
      blocked: true,
      reason: `Ready Check command emulator could not be built: ${error.message}`
    };
  }
  if (!fs.existsSync(binaryPath)) {
    return {
      blocked: true,
      reason: `Ready Check command emulator build produced no binary: ${binaryPath}`
    };
  }
  return { path: binaryPath };
}

function parseReadyCheckVoucher(row) {
  const raw = row && row.voucher;
  if (!raw) return null;
  let parsed = raw;
  if (typeof raw === 'string') {
    try {
      parsed = JSON.parse(raw);
    } catch (_) {
      return null;
    }
  }
  if (!parsed || typeof parsed !== 'object') return null;
  const username = String(parsed.username || '').trim();
  if (!username) return null;
  return {
    username,
    password: parsed.password === undefined || parsed.password === null ? '' : String(parsed.password)
  };
}

function readyCheckDeviceNumber(row) {
  return String(
    row && (row.device_number || row.deviceNumber || row.DeviceNumber || row.id || '')
  ).trim();
}

function buildReadyCheckEmulatorConfig() {
  // JSON is valid YAML.  The file is a static launcher template: all values
  // obtained from the API or MQTT discovery are injected through the child
  // process environment below, so network responses are never persisted in a
  // local configuration artifact.
  return JSON.stringify({
    device_type: 'direct',
    mqtt: {
      broker: '127.0.0.1:1883',
      client_id: 'ready-check-runtime',
      username: '',
      password: '',
      qos: 1,
      clean_session: true,
      keep_alive: 60
    },
    device: {
      device_id: 'runtime-device-id',
      device_number: 'runtime-device-number'
    },
    database: {
      host: '127.0.0.1',
      port: 5432,
      dbname: 'aetherlink_iot',
      username: 'postgres',
      password: '',
      sslmode: 'disable',
      max_open_conns: 2,
      max_idle_conns: 1
    },
    api: {
      base_url: 'http://127.0.0.1:9999',
      api_key: 'local-ready-check',
      timeout: 30
    },
    test: {
      wait_db_sync_seconds: 1,
      wait_mqtt_response_seconds: 5,
      retry_times: 3,
      log_level: 'info'
    }
  }, null, 2);
}

async function waitForReadyCheckDeviceOnline(deviceId, child, exitState, attempts = 40) {
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    if (exitState.value) {
      return null;
    }
    const response = await apiClient.get('/device/detail/' + deviceId, {}, 'tenant_admin');
    if (response && response.code === 200 && response.data && isOnlineDevice(response.data)) {
      return response.data;
    }
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  return null;
}

async function startReadyCheckEmulator(device, accountKey = 'tenant_admin', options = {}) {
  const detailResp = await apiClient.get('/device/detail/' + device.id, {}, accountKey);
  requireSuccess(detailResp, 'read Ready Check emulator device detail');
  const detail = detailResp.data || {};
  const voucher = parseReadyCheckVoucher(detail);
  if (!voucher) {
    return {
      blocked: true,
      reason: `Ready Check device ${device.id} does not expose a JSON MQTT voucher with a username`
    };
  }

  const endpoint = mqttRuntime.getMqttEndpoint();
  if (!(await isMqttBrokerAvailable(endpoint.server, endpoint.port))) {
    return {
      blocked: true,
      reason: `MQTT broker ${endpoint.server}:${endpoint.port} is unavailable for Ready Check auto-start`
    };
  }

  const binary = await ensureReadyCheckEmulatorBinary();
  if (binary.blocked) return binary;

  const deviceNumber = readyCheckDeviceNumber(detail) || device.id;
  // The device id is API data. Keep it in the child environment, but never
  // derive a local artifact path from it. A locally generated run id also
  // prevents concurrent Ready Check workers from sharing the same files.
  const runId = crypto.randomUUID();
  const clientId = `ready-check-${runId}`.slice(0, 128);
  const reportDir = readyCheckReportDirectory();
  const configPath = path.join(reportDir, `ready-check-emulator-${runId}.json`);
  const stdoutPath = path.join(reportDir, `ready-check-emulator-${runId}.stdout.log`);
  const stderrPath = path.join(reportDir, `ready-check-emulator-${runId}.stderr.log`);
  fs.writeFileSync(configPath, buildReadyCheckEmulatorConfig(), 'utf8');

  const moduleRoot = path.resolve(__dirname, '..', '..', 'backend', 'cmd', 'aetherlink-device-autotest');
  const stdout = fs.createWriteStream(stdoutPath, { flags: 'a' });
  const stderr = fs.createWriteStream(stderrPath, { flags: 'a' });
  const child = spawn(binary.path, [
    '-config', configPath,
    '-mode', 'command-emulator',
    '-command-success'
  ], {
    cwd: moduleRoot,
    windowsHide: true,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: {
      ...process.env,
      AUTOTEST_MQTT_BROKER: `${endpoint.server}:${endpoint.port}`,
      AUTOTEST_MQTT_CLIENT_ID: clientId,
      AUTOTEST_MQTT_USERNAME: voucher.username,
      AUTOTEST_MQTT_PASSWORD: voucher.password,
      AUTOTEST_DEVICE_ID: device.id,
      AUTOTEST_DEVICE_NUMBER: deviceNumber,
      AUTOTEST_API_BASE_URL: String(apiClient.getConfig()?.baseURL || '').replace(/\/api\/v1\/?$/, ''),
      AUTOTEST_API_KEY: 'local-ready-check',
      // The failure lane is still a real MQTT acknowledgement from the
      // emulator.  It is deliberately opt-in so the normal command fixture
      // remains a success fixture and callers cannot accidentally turn a
      // generic device into an RDI-looking fake.
      AUTOTEST_COMMAND_FAILURE_IDENTIFY: String(options.failureIdentify || '').trim()
    }
  });
  child.stdout.pipe(stdout);
  child.stderr.pipe(stderr);

  const exitState = { value: null };
  const exitPromise = new Promise(resolve => {
    child.once('error', error => {
      exitState.value = { error };
      resolve(exitState.value);
    });
    child.once('exit', (code, signal) => {
      exitState.value = { code, signal };
      resolve(exitState.value);
    });
  });

  const stop = async () => {
    if (!exitState.value) {
      child.kill('SIGTERM');
      await Promise.race([
        exitPromise,
        new Promise(resolve => setTimeout(resolve, 5000))
      ]);
    }
    if (!exitState.value) child.kill('SIGKILL');
    stdout.end();
    stderr.end();
  };

  const onlineDetail = await waitForReadyCheckDeviceOnline(device.id, child, exitState);
  if (!onlineDetail) {
    await stop();
    return {
      blocked: true,
      reason: `Ready Check emulator did not publish online status for ${device.id}; see ${stderrPath}`
    };
  }

  return {
    row: onlineDetail,
    cleanup: stop
  };
}

async function createReadyCheckEmulatorDevice(accountKey = 'tenant_admin') {
  const suffix = `${Date.now().toString(36)}${crypto.randomBytes(3).toString('hex')}`;
  const voucher = JSON.stringify({
    username: `ready-check-${suffix}`.slice(0, 64),
    password: `ready-check-${crypto.randomBytes(12).toString('hex')}`
  });
  const createResp = await apiClient.post('/device', {
    name: makeRunLabel('ready-check-node-emulator'),
    device_config_id: '',
    voucher,
    description: 'local Ready Check command emulator fixture'
  }, accountKey);
  requireSuccess(createResp, 'create isolated Ready Check emulator device');
  const id = pickId(createResp.data);
  if (!id) throw new Error('isolated Ready Check emulator device returned no id');
  return {
    id,
    row: createResp.data,
    created: true,
    cleanup: async () => {
      const deleteResp = await apiClient.delete('/device/' + id, {}, accountKey);
      requireSuccess(deleteResp, 'delete isolated Ready Check emulator device');
    }
  };
}

async function prepareReadyCheckRuntimeDevice(accountKey = 'tenant_admin', options = {}) {
  const requestedDeviceId = String(process.env.AUTOMATION_READY_CHECK_DEVICE_ID || '').trim();
  const autoStart = readyCheckAutoStartEnabled();
  const listedDevices = await listDevices(accountKey);
  const detailedDevices = await loadDeviceDetailsForFixture(listedDevices, accountKey);

  if (requestedDeviceId) {
    const selection = selectReadyCheckDevice(detailedDevices, requestedDeviceId);
    if (selection.invalid || !selection.blocked) {
      if (!selection.blocked) return selection;
      return selection;
    }
    if (!autoStart) return selection;

    const requestedRow = detailedDevices.find(row => pickId(row) === requestedDeviceId);
    if (!requestedRow) return selection;
    const runtime = await startReadyCheckEmulator({ id: requestedDeviceId, row: requestedRow }, accountKey, options);
    if (runtime.blocked) return runtime;
    const afterStart = selectReadyCheckDevice([runtime.row], requestedDeviceId);
    if (afterStart.blocked || afterStart.invalid) {
      await runtime.cleanup();
      return afterStart;
    }
    return {
      device: {
        ...afterStart.device,
        row: runtime.row,
        cleanup: runtime.cleanup
      }
    };
  }

  if (!autoStart) {
    return selectReadyCheckDevice(detailedDevices, requestedDeviceId);
  }

  let created;
  try {
    created = await createReadyCheckEmulatorDevice(accountKey);
    const runtime = await startReadyCheckEmulator(created, accountKey, options);
    if (runtime.blocked) {
      await created.cleanup();
      return runtime;
    }
    const selection = selectReadyCheckDevice([runtime.row], created.id);
    if (selection.blocked || selection.invalid) {
      await runtime.cleanup();
      await created.cleanup();
      return selection;
    }
    return {
      device: {
        ...selection.device,
        row: runtime.row,
        created: true,
        cleanup: async () => {
          await runtime.cleanup();
          await created.cleanup();
        }
      }
    };
  } catch (error) {
    if (created) {
      try { await created.cleanup(); } catch (_) { /* best effort */ }
    }
    throw error;
  }
}

function selectRdiDevice(detailedDevices) {
  const syntheticPid = getOptInSyntheticRdiPid();
  if (syntheticPid) {
    return (Array.isArray(detailedDevices) ? detailedDevices : [])
      .find(row => {
        const rowPid = String(
          row && (row.pid_number || row.device_number || row.deviceNumber || row.DeviceNumber || '')
        ).trim().toUpperCase();
        return rowPid === syntheticPid;
      });
  }
  return (Array.isArray(detailedDevices) ? detailedDevices : [])
    .find(row => isRdiDeviceRow(row));
}

async function listDevices(accountKey = 'tenant_admin') {
  const listResp = await apiClient.get('/device', { page: 1, page_size: 100 }, accountKey);
  return listFromResponse(listResp).filter(row => pickId(row));
}

async function loadDeviceDetailsForFixture(rows, accountKey = 'tenant_admin') {
  const detailedRows = [];
  for (const row of rows) {
    const id = pickId(row);
    if (!id) continue;
    const response = await apiClient.get('/device/detail/' + id, {}, accountKey);
    if (response && response.code === 200 && response.data) {
      detailedRows.push({ ...row, ...response.data });
    }
  }
  return detailedRows;
}

async function activateFixtureDevice(accountKey = 'tenant_admin') {
  const candidatePids = getRdiCandidatePids();

  for (const pid of candidatePids) {
    const resp = await apiClient.post('/rdi/devices/activate', {
      pid_number: pid,
      name: testData.generateDeviceName('seed-rdi')
    }, accountKey);

    if (resp && resp.code === 200 && resp.data) {
      return resp.data;
    }

    // A non-success response is followed by a tenant-scoped list check.  This
    // preserves the fail-closed behavior for missing real/synthetic fixtures.
    if (resp && (resp.code === 204001 || resp.code === 204002)) {
      const devices = await listDevices(accountKey);
      const syntheticPid = getOptInSyntheticRdiPid();
      const matched = devices.find(row => {
        const rowPid = String(
          row && (row.pid_number || row.device_number || row.deviceNumber || row.DeviceNumber || '')
        ).trim().toUpperCase();
        return syntheticPid ? rowPid === syntheticPid : isConfiguredPid(row);
      });
      if (matched) return matched;
    }
  }

  return null;
}

async function ensureDevice(accountKey = 'tenant_admin') {
  const existingDevices = await listDevices(accountKey);
  // A real command/browser flow needs an eligible online device. Prefer a
  // configured fixture when one exists, then an explicitly online device,
  // and only fall back to the first row for read-only API coverage. The old
  // first-row choice routinely selected stale offline data and made the E2E
  // command module skip even though the backend had a usable device.
  const existing = existingDevices.find(row => isConfiguredPid(row)) ||
    existingDevices.find(row => isOnlineDevice(row)) ||
    existingDevices[0];
  if (existing) {
    return {
      id: pickId(existing),
      row: existing,
      created: false,
      cleanup: async () => {}
    };
  }

  const activated = await activateFixtureDevice(accountKey);
  if (activated) {
    return {
      id: pickId(activated),
      row: activated,
      created: false,
      cleanup: async () => {}
    };
  }

  const pid = testData.getDevicePID('activated_pid') || testData.getDevicePID('pid') || '';
  if (!pid) {
    throw new Error('No configured test device PID; seed_data cannot create a deterministic device yet');
  }

  // devices_unique_1 是 voucher 上的 *全局* 唯一约束（sql/1.sql），而设备查找是
  // 租户内的：一旦别的租户/历史运行占用了固定 PID 作为 voucher，本租户既看不到
  // 那台设备、又插不进去，seed 会永久硬失败（23505）。所以 voucher 必须唯一，
  // PID 语义由 device_number 承载；没有任何断言要求 voucher 等于 PID。
  const seedVoucher = JSON.stringify({
    username: randomFixtureToken('seed'),
    password: randomFixtureToken('seed-password')
  });
  const createResp = await apiClient.post('/device', {
    name: testData.generateDeviceName('seed-device'),
    device_config_id: '',
    voucher: seedVoucher
  }, accountKey);
  requireSuccess(createResp, 'create seed device');

  const id = pickId(createResp.data);
  return {
    id,
    row: createResp.data,
    created: true,
    cleanup: async () => {
      if (id) await apiClient.delete('/device/' + id, {}, accountKey);
    }
  };
}

async function createSimulationDevice(accountKey = 'tenant_admin') {
  const suffix = randomFixtureToken('simulation');
  const voucher = JSON.stringify({
    username: 'simulation-' + suffix,
    password: 'simulation-' + suffix.slice(-10)
  });
  const createResp = await apiClient.post('/device', {
    name: makeRunLabel('simulation-device'),
    device_config_id: '',
    voucher
  }, accountKey);
  requireSuccess(createResp, 'create simulation device');
  const id = pickId(createResp.data);
  if (!id) throw new Error('create simulation device returned no id');
  return {
    id,
    row: createResp.data,
    created: true,
    cleanup: async () => {
      const deleteResp = await apiClient.delete('/device/' + id, {}, accountKey);
      requireSuccess(deleteResp, 'delete simulation device');
    }
  };
}

async function ensureDeviceWithTelemetry(accountKey = 'tenant_admin') {
  const seed = await ensureDevice(accountKey);
  const telemetryPayload = {
    temperature_1: 25.5,
    temperature_2: 26.25,
    switch_1: 1,
    switch_2: 0
  };
  const telemetryResult = await publishSimulatedTelemetryAndReadCurrent(
    seed.id,
    telemetryPayload,
    accountKey,
    { waitForHistory: true }
  );

  return {
    ...seed,
    telemetryPayload,
    telemetrySeeded: true,
    telemetrySeedResponse: telemetryResult.publishResp,
    telemetryRows: telemetryResult.rows
  };
}

function telemetryValueMatches(actual, expected) {
  if (actual === expected) return true;
  if (String(actual) === String(expected)) return true;
  const actualNumber = Number(actual);
  const expectedNumber = Number(expected);
  return Number.isFinite(actualNumber) &&
    Number.isFinite(expectedNumber) &&
    actualNumber === expectedNumber;
}

function mqttEndpointDescription() {
  return mqttRuntime.mqttEndpointDescription();
}

async function readCurrentTelemetryForKey(deviceId, key, expectedValue, accountKey = 'tenant_admin') {
  const resp = await apiClient.get('/telemetry/datas/current/' + deviceId, {}, accountKey);
  const rows = resp && resp.code === 200 && Array.isArray(resp.data) ? resp.data : [];
  const row = rows.find(item =>
    item &&
    item.device_id === deviceId &&
    item.key === key &&
    telemetryValueMatches(item.value, expectedValue)
  );
  return { resp, row, rows };
}

async function readHistoryTelemetryForKey(deviceId, key, accountKey = 'tenant_admin') {
  const range = testData.getHistoryTimeRange();
  const resp = await apiClient.get('/telemetry/datas/history', {
    device_id: deviceId,
    key,
    start_time: range.startTime,
    end_time: range.endTime
  }, accountKey);
  const rows = resp && resp.code === 200 && Array.isArray(resp.data) ? resp.data : [];
  return { resp, rows };
}

async function publishSimulatedTelemetryAndReadCurrent(
  deviceId,
  payload,
  accountKey = 'tenant_admin',
  options = {}
) {
  const attempts = options.attempts || 10;
  const delayMs = options.delayMs || 500;
  const keys = Object.keys(payload || {});
  if (!deviceId || keys.length === 0) {
    throw new Error('publishSimulatedTelemetryAndReadCurrent requires a device id and payload keys');
  }

  const mqttEndpoint = mqttRuntime.getMqttEndpoint();
  const simulationTarget = {
    server: mqttEndpoint.server,
    port: mqttEndpoint.port
  };
  const mqttTopic = String(process.env.AUTOMATION_MQTT_TOPIC || '').trim();
  if (mqttTopic) {
    simulationTarget.topic = mqttTopic;
  }

  const publishResp = await apiClient.post('/telemetry/datas/simulation/send', {
    device_id: deviceId,
    data: JSON.stringify(payload),
    ...simulationTarget
  }, accountKey);
  requireSuccess(publishResp, 'publish simulated telemetry');

  let lastRead = null;
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    const reads = await Promise.all(keys.map(key =>
      readCurrentTelemetryForKey(deviceId, key, payload[key], accountKey)
    ));
    lastRead = reads;
    if (reads.every(item => item.row)) {
      if (options.waitForHistory) {
        let lastHistoryRead = null;
        for (let historyAttempt = 0; historyAttempt < attempts; historyAttempt += 1) {
          const historyReads = await Promise.all(keys.map(key =>
            readHistoryTelemetryForKey(deviceId, key, accountKey)
          ));
          lastHistoryRead = historyReads;
          if (historyReads.every(item => item.rows.length > 0)) {
            return {
              publishResp,
              rows: reads.map(item => item.row),
              allRows: reads.flatMap(item => item.rows),
              historyRows: historyReads.flatMap(item => item.rows)
            };
          }
          if (historyAttempt < attempts - 1) {
            await new Promise(resolve => setTimeout(resolve, delayMs));
          }
        }

        const missingHistory = keys.filter((key, index) => !(
          lastHistoryRead &&
          lastHistoryRead[index] &&
          lastHistoryRead[index].rows.length > 0
        ));
        throw new Error(
          'Published telemetry was readable from current telemetry but not history: ' +
          missingHistory.join(', ')
        );
      }

      return {
        publishResp,
        rows: reads.map(item => item.row),
        allRows: reads.flatMap(item => item.rows)
      };
    }
    if (attempt < attempts - 1) {
      await new Promise(resolve => setTimeout(resolve, delayMs));
    }
  }

  const missing = keys.filter((key, index) => !(lastRead && lastRead[index] && lastRead[index].row));
  throw new Error('Published telemetry was not readable from current telemetry: ' + missing.join(', '));
}

async function ensureAlarmConfig(accountKey = 'tenant_admin') {
  const req = testData.getCreateAlarmConfigReq();
  req.name = makeRunLabel('seed_alarm');
  const createResp = await apiClient.post('/alarm/config', req, accountKey);
  if (createResp.code !== 200) {
    return {
      blocked: true,
      reason: createResp.message || 'alarm config endpoint unavailable',
      cleanup: async () => {}
    };
  }
  const id = pickId(createResp.data);
  return {
    id,
    row: createResp.data,
    blocked: false,
    cleanup: async () => {
      if (id) await apiClient.delete('/alarm/config/' + id, {}, accountKey);
    }
  };
}

async function ensureDeviceConfig(accountKey = 'tenant_admin') {
  const name = makeRunLabel('seed_device_config');
  const createResp = await apiClient.post('/device_config', {
    name,
    device_type: '1',
    protocol_type: 'MQTT',
    voucher_type: 'ACCESSTOKEN',
    device_conn_type: 'A',
    protocol_config: '{}'
  }, accountKey);
  requireSuccess(createResp, 'create seed device config');
  const id = pickId(createResp.data);
  return {
    id,
    row: createResp.data,
    blocked: false,
    cleanup: async () => {
      if (id) await apiClient.delete('/device_config/' + id, {}, accountKey);
    }
  };
}

/**
 * Build the real template -> command model -> device config -> device binding
 * chain consumed by the Ready Check browser handoff.  This deliberately uses
 * public APIs instead of inserting rows or inventing a command response.  The
 * selected device is restored to its previous config during cleanup so the
 * fixture is safe to run against a shared local emulator.
 */
async function ensureReadyCheckCommandFixture(accountKey = 'tenant_admin', options = {}) {
  const requestedIdentify = String(options.commandIdentify || 'test_dry_contact').trim();
  if (!requestedIdentify) {
    throw new Error('Ready Check command fixture requires a non-empty command identifier');
  }
  const selection = await prepareReadyCheckRuntimeDevice(accountKey, {
    failureIdentify: options.failureIdentify || ''
  });
  if (selection.blocked) {
    return {
      blocked: true,
      reason: selection.reason,
      cleanup: async () => {}
    };
  }
  if (selection.invalid) {
    throw new Error(selection.reason);
  }
  const device = selection.device;
  const previousDeviceConfigId = String(
    device.row && (
      device.row.device_config_id ||
      device.row.deviceConfigId ||
      device.row.DeviceConfigID ||
      ''
    ) || ''
  ).trim();
  const templateName = makeRunLabel('ready_check_template');
  const configName = makeRunLabel('ready_check_config');
  let templateId = '';
  let commandId = '';
  let configId = '';
  let bound = false;

  const cleanup = async () => {
    if (bound) {
      const restoreResp = await apiClient.put('/device/update/config', {
        device_id: device.id,
        // The API treats an empty string as the explicit unbind sentinel;
        // JSON null fails the max-length validator before service logic runs.
        device_config_id: previousDeviceConfigId
      }, accountKey);
      requireSuccess(restoreResp, 'restore device config after Ready Check fixture');
      bound = false;
    }
    if (configId) {
      const deleteConfigResp = await apiClient.delete('/device_config/' + configId, {}, accountKey);
      requireSuccess(deleteConfigResp, 'delete Ready Check fixture device config');
      configId = '';
    }
    if (commandId) {
      const deleteCommandResp = await apiClient.delete('/device/model/commands/' + commandId, {}, accountKey);
      requireSuccess(deleteCommandResp, 'delete Ready Check fixture command model');
      commandId = '';
    }
    if (templateId) {
      const deleteTemplateResp = await apiClient.delete('/device/template/' + templateId, {}, accountKey);
      requireSuccess(deleteTemplateResp, 'delete Ready Check fixture template');
      templateId = '';
    }
    await device.cleanup();
  };

  try {
    let response = await apiClient.post('/device/template', {
      name: templateName,
      author: 'automation',
      version: '1.0.0',
      description: 'real Ready Check command handoff fixture'
    }, accountKey);
    requireSuccess(response, 'create Ready Check fixture template');
    templateId = pickId(response.data);
    if (!templateId) throw new Error('Ready Check fixture template returned no id');

    response = await apiClient.post('/device/model/commands', {
      device_template_id: templateId,
      data_name: 'Test Dry Contact',
      data_identifier: requestedIdentify,
      params: JSON.stringify(testData.getTestDryContactParams()),
      description: 'real Ready Check command handoff fixture'
    }, accountKey);
    requireSuccess(response, 'create Ready Check fixture command model');
    commandId = pickId(response.data);
    if (!commandId) throw new Error('Ready Check fixture command model returned no id');

    response = await apiClient.post('/device_config', {
      name: configName,
      device_template_id: templateId,
      device_type: '1',
      protocol_type: 'MQTT',
      voucher_type: 'ACCESSTOKEN',
      device_conn_type: 'A',
      protocol_config: '{}'
    }, accountKey);
    requireSuccess(response, 'create Ready Check fixture device config');
    configId = pickId(response.data);
    if (!configId) throw new Error('Ready Check fixture device config returned no id');

    response = await apiClient.put('/device/update/config', {
      device_id: device.id,
      device_config_id: configId
    }, accountKey);
    requireSuccess(response, 'bind device to Ready Check fixture config');
    bound = true;

    response = await apiClient.get('/command/datas/' + device.id, {}, accountKey);
    requireSuccess(response, 'read Ready Check fixture command data');
    const commands = Array.isArray(response.data) ? response.data : [];
    const command = commands.find(row => String(row && row.data_identifier || '').trim() === requestedIdentify);
    if (!command) {
      throw new Error('Ready Check fixture command is not visible through /command/datas/' + device.id);
    }

    // Verify that the public bind endpoint and the command-preview service see
    // the same config before handing control to the browser.  A mismatch here
    // is a fixture/backend state error; it must not be downgraded to a green
    // browser assertion or an arbitrary offline-device skip.
    const boundDetailResp = await apiClient.get('/device/detail/' + device.id, {}, accountKey);
    requireSuccess(boundDetailResp, 'verify Ready Check fixture device binding');
    const boundDetail = boundDetailResp.data || {};
    const visibleConfigId = String(
      boundDetail.device_config_id ||
      boundDetail.deviceConfigId ||
      boundDetail.DeviceConfigID ||
      ''
    ).trim();
    if (visibleConfigId !== String(configId)) {
      throw new Error(
        `Ready Check fixture binding mismatch: expected config ${configId}, API returned ${visibleConfigId || '<empty>'}`
      );
    }

    const commandValue = command.params === undefined || command.params === null
      ? ''
      : (typeof command.params === 'string' ? command.params.trim() : JSON.stringify(command.params));
    const previewResp = await apiClient.post('/command/datas/jobs/preview', {
      scope_type: 'selected_devices',
      device_ids: [device.id],
      identify: String(command.data_identifier).trim(),
      value: commandValue,
      timeout_seconds: 60,
      subset_limit: 10,
      sample_limit: 10
    }, accountKey);
    requireSuccess(previewResp, 'preview Ready Check fixture command');
    const previewRow = Array.isArray(previewResp.data?.rows)
      ? previewResp.data.rows.find(row => row && row.device_id === device.id)
      : null;
    if (!previewRow) {
      throw new Error('Ready Check fixture preview returned no row for ' + device.id);
    }
    if (Number(previewResp.data?.eligible_count || 0) <= 0) {
      const previewReason = String(previewRow.reason || '').trim();
      if (!previewRow.online || /offline|not online|不在线|离线/i.test(previewReason)) {
        let cleanupError = '';
        try {
          await cleanup();
        } catch (error) {
          cleanupError = `; fixture cleanup failed: ${error.message}`;
        }
        return {
          blocked: true,
          reason: `Ready Check command emulator/device became unavailable during preview${previewReason ? `: ${previewReason}` : ''}${cleanupError}`,
          cleanup: async () => {}
        };
      }
      throw new Error(
        `Ready Check fixture preview unexpectedly blocked the online device: ${previewReason || JSON.stringify(previewRow)}`
      );
    }

    return {
      ...device,
      templateId,
      commandId,
      configId,
      commandResponse: response,
      commandIdentify: String(command.data_identifier).trim(),
      commandValue,
      fixturePreview: previewResp,
      cleanup
    };
  } catch (error) {
    try {
      await cleanup();
    } catch (cleanupError) {
      error.message += `; Ready Check fixture cleanup failed: ${cleanupError.message}`;
    }
    throw error;
  }
}

async function ensureRdiDevice(accountKey = 'tenant_admin') {
  const listedDevices = await listDevices(accountKey);
  const detailedDevices = await loadDeviceDetailsForFixture(listedDevices, accountKey);
  const existing = selectRdiDevice(detailedDevices);
  if (existing) {
    return {
      id: pickId(existing),
      row: existing,
      created: false,
      cleanup: async () => {}
    };
  }

  const activated = await activateFixtureDevice(accountKey);
  if (activated) {
    const id = pickId(activated);
    const detailResp = id
      ? await apiClient.get('/device/detail/' + id, {}, accountKey)
      : null;
    return {
      id,
      row: detailResp && detailResp.code === 200 ? detailResp.data : activated,
      created: false,
      cleanup: async () => {}
    };
  }

  throw new Error('No real RDI fixture device is visible; activate a configured RDI PID before running RDI share evidence');
}

async function ensureOpenApiKey(accountKey = 'super_admin', tenantId = '') {
  const name = makeRunLabel('seed_openapi');
  const payload = tenantId ? { tenant_id: tenantId, name } : { name, remark: 'automation seed' };
  const createResp = await apiClient.post('/open/keys', payload, accountKey);
  if (createResp.code !== 200) {
    return {
      blocked: true,
      reason: createResp.message || 'openapi key endpoint unavailable',
      cleanup: async () => {}
    };
  }
  const listResp = await apiClient.get('/open/keys', { page: 1, page_size: 100 }, accountKey);
  requireSuccess(listResp, 'list seed openapi keys');
  const created = listFromResponse(listResp).find(row => row && row.name === name);
  const row = created || createResp.data;
  const id = pickId(row);
  return {
    id,
    row,
    blocked: false,
    cleanup: async () => {
      if (id) await apiClient.delete('/open/keys/' + id, {}, accountKey);
    }
  };
}

async function createOtaPackageSeed(accountKey = 'super_admin') {
  const configSeed = await ensureDeviceConfig(accountKey);
  const name = makeRunLabel('seed_ota_package');
  let packageId = '';
  try {
    const uploadResp = await apiClient.upload(
      '/file/up',
      Buffer.from('aetherlink-ota-fixture', 'utf8'),
      { type: 'upgradePackage' },
      accountKey
    );
    requireSuccess(uploadResp, 'upload OTA fixture package');
    const packageUrl = uploadResp.data && uploadResp.data.path;
    if (!packageUrl) throw new Error('upload OTA fixture package returned no path');

    const version = `0.0.${Date.now()}`;
    const createResp = await apiClient.post('/ota/package', {
      name,
      version,
      target_version: version,
      device_config_id: configSeed.id,
      package_type: 2,
      signature_type: 'MD5',
      package_url: packageUrl,
      additional_info: '{}',
      description: 'isolated automation support-bundle fixture'
    }, accountKey);
    requireSuccess(createResp, 'create OTA fixture package');

    const listResp = await apiClient.get('/ota/package', { page: 1, page_size: 50, name }, accountKey);
    requireSuccess(listResp, 'list OTA fixture package');
    const row = listFromResponse(listResp).find(item => item && item.name === name);
    packageId = pickOtaPackageId(row);
    if (!packageId) throw new Error('created OTA fixture package was not visible in list');

    return {
      id: packageId,
      row,
      cleanup: async () => {
        try {
          await apiClient.delete('/ota/package/' + packageId, {}, accountKey);
        } finally {
          await configSeed.cleanup();
        }
      }
    };
  } catch (error) {
    try {
      await configSeed.cleanup();
    } catch (cleanupError) {
      error.message += `; OTA fixture config cleanup failed: ${cleanupError.message}`;
    }
    throw error;
  }
}

function sqlLiteral(value) {
  return `'${String(value).replace(/'/g, "''")}'`;
}

async function createOtaTaskPersistenceSeed(packageId, packageRow, accountKey, packageSeed) {
  const psqlPath = process.env.AETHERLINK_PSQL_PATH || 'C:\\Program Files\\PostgreSQL\\17\\bin\\psql.exe';
  if (!fs.existsSync(psqlPath)) {
    if (packageSeed) await packageSeed.cleanup();
    return {
      blocked: true,
      reason: 'PostgreSQL psql client is unavailable for isolated OTA task fixture',
      cleanup: async () => {}
    };
  }

  const deviceSeed = await ensureDevice('tenant_admin');
  const taskId = crypto.randomUUID();
  const detailId = crypto.randomUUID();
  const taskName = makeRunLabel('seed_ota_task');
  const database = getOtaSeedDatabaseOptions();
  const sql = [
    `INSERT INTO ota_upgrade_tasks (id, name, ota_upgrade_package_id, description, created_at, remark, target_mode, preview_total, selected_count, created_by_authority) VALUES (${sqlLiteral(taskId)}, ${sqlLiteral(taskName)}, ${sqlLiteral(packageId)}, ${sqlLiteral('isolated support-bundle fixture')}, NOW(), ${sqlLiteral('no-dispatch fixture')}, 'explicit', 1, 1, 'SYS_ADMIN')`,
    `INSERT INTO ota_upgrade_task_details (id, ota_upgrade_task_id, device_id, steps, status, status_description, updated_at, remark) VALUES (${sqlLiteral(detailId)}, ${sqlLiteral(taskId)}, ${sqlLiteral(deviceSeed.id)}, 0, 1, ${sqlLiteral('pending support-bundle fixture')}, NOW(), ${sqlLiteral('no-dispatch fixture')})`
  ].join('; ');

  try {
    await execFileAsync(psqlPath, [
      '-h', database.host,
      '-p', database.port,
      '-U', database.user,
      '-d', database.database,
      '-v', 'ON_ERROR_STOP=1',
      '-c', sql
    ], { windowsHide: true, timeout: 15000, env: { ...process.env, PGPASSWORD: database.password } });

    return {
      id: taskId,
      taskId,
      packageId,
      package: packageRow,
      task: { id: taskId, name: taskName, ota_upgrade_package_id: packageId },
      blocked: false,
      cleanup: async () => {
        try {
          await execFileAsync(psqlPath, [
            '-h', database.host,
            '-p', database.port,
            '-U', database.user,
            '-d', database.database,
            '-v', 'ON_ERROR_STOP=1',
            '-c', `DELETE FROM ota_upgrade_tasks WHERE id = ${sqlLiteral(taskId)}`
          ], { windowsHide: true, timeout: 15000, env: { ...process.env, PGPASSWORD: database.password } });
        } finally {
          await deviceSeed.cleanup();
          if (packageSeed) await packageSeed.cleanup();
        }
      }
    };
  } catch (error) {
    await deviceSeed.cleanup();
    if (packageSeed) await packageSeed.cleanup();
    throw new Error(`create isolated OTA task fixture failed: ${error.message}`);
  }
}

async function ensureOtaTaskSupportBundleSource(accountKey = 'super_admin') {
  const seedAccount = accountKey === 'super_admin' ? 'tenant_admin' : accountKey;
  let packageResp = await apiClient.get('/ota/package', { page: 1, page_size: 20 }, accountKey);
  if (!packageResp || packageResp.code !== 200) {
    return {
      blocked: true,
      reason: packageResp?.message || 'OTA package list endpoint unavailable',
      cleanup: async () => {}
    };
  }

  let packageSeed = null;
  let packages = listFromResponse(packageResp).filter(row => pickOtaPackageId(row));
  if (packages.length === 0 && seedAccount !== accountKey) {
    const tenantPackageResp = await apiClient.get('/ota/package', { page: 1, page_size: 20 }, seedAccount);
    if (tenantPackageResp && tenantPackageResp.code === 200) {
      packages = listFromResponse(tenantPackageResp).filter(row => pickOtaPackageId(row));
    }
  }
  if (packages.length === 0) {
    packageSeed = await createOtaPackageSeed(seedAccount);
    packages = [packageSeed.row];
  }

  const taskErrors = [];
  const taskAccount = packageSeed ? seedAccount : (packages.length > 0 && seedAccount !== accountKey ? seedAccount : accountKey);
  for (const otaPackage of packages) {
    const packageId = pickOtaPackageId(otaPackage);
    const taskResp = await apiClient.get(
      '/ota/task',
      { page: 1, page_size: 20, ota_upgrade_package_id: packageId },
      taskAccount
    );

    if (!taskResp || taskResp.code !== 200) {
      taskErrors.push(taskResp?.message || `task list failed for package ${packageId}`);
      continue;
    }

    const task = listFromResponse(taskResp).find(row => pickOtaTaskId(row));
    if (!task) continue;

    const taskId = pickOtaTaskId(task);
    return {
      id: taskId,
      taskId,
      packageId,
      package: otaPackage,
      task,
      blocked: false,
      cleanup: packageSeed ? packageSeed.cleanup : async () => {}
    };
  }

  const packageForTask = packages[0];
  const packageId = pickOtaPackageId(packageForTask);
  if (packageId) {
    return createOtaTaskPersistenceSeed(packageId, packageForTask, accountKey, packageSeed);
  }

  if (packageSeed) await packageSeed.cleanup();
  return {
    blocked: true,
    reason: taskErrors.length
      ? `No readable OTA task found; task list errors: ${taskErrors.join('; ')}`
      : 'No OTA package is available for support-bundle API coverage',
    cleanup: async () => {}
  };
}

async function ensureNotificationGroup(accountKey = 'tenant_admin') {
  const name = makeRunLabel('seed_notify');
  const createResp = await apiClient.post('/notification_group', {
    name,
    description: 'automation seed',
    notification_type: 'EMAIL',
    notification_config: JSON.stringify({ EMAIL: 'seed@example.com' }),
    status: 'CLOSE'
  }, accountKey);
  requireSuccess(createResp, 'create seed notification group');
  const id = pickId(createResp.data);
  return {
    id,
    row: createResp.data,
    cleanup: async () => {
      if (id) await apiClient.delete('/notification_group/' + id, {}, accountKey);
    }
  };
}

async function ensureScene(accountKey = 'tenant_admin') {
  const alarmSeed = await ensureAlarmConfig(accountKey);
  if (alarmSeed.blocked) {
    return {
      blocked: true,
      reason: alarmSeed.reason,
      cleanup: alarmSeed.cleanup
    };
  }

  const name = makeRunLabel('seed_scene');
  const createResp = await apiClient.post('/scene', {
    name,
    description: 'automation seed',
    actions: [
      {
        action_type: '30',
        action_target: alarmSeed.id,
        remark: 'trigger seeded alarm config'
      }
    ]
  }, accountKey);

  if (createResp.code !== 200) {
    await alarmSeed.cleanup();
    return {
      blocked: true,
      reason: createResp.message || 'scene endpoint unavailable',
      cleanup: async () => {}
    };
  }

  const id = pickId(createResp.data);
  return {
    id,
    name,
    row: createResp.data,
    alarmConfigId: alarmSeed.id,
    alarmConfigName: alarmSeed.row && alarmSeed.row.name,
    blocked: false,
    cleanup: async () => {
      if (id) await apiClient.delete('/scene/' + id, {}, accountKey);
      await alarmSeed.cleanup();
    }
  };
}

async function ensureSceneAlarmHistory(accountKey = 'tenant_admin') {
  const sceneSeed = await ensureScene(accountKey);
  if (sceneSeed.blocked) {
    return sceneSeed;
  }

  const activationResp = await apiClient.post('/scene/active/' + sceneSeed.id, {}, accountKey);
  if (!activationResp || activationResp.code !== 200) {
    return {
      blocked: true,
      reason: activationResp?.message || 'scene activation endpoint unavailable',
      sceneSeed,
      cleanup: sceneSeed.cleanup
    };
  }

  const historyResp = await apiClient.get('/alarm/info/history', { page: 1, page_size: 100 }, accountKey);
  if (!historyResp || historyResp.code !== 200) {
    return {
      blocked: true,
      reason: historyResp?.message || 'alarm history list endpoint unavailable after scene activation',
      sceneSeed,
      activationResponse: activationResp,
      cleanup: sceneSeed.cleanup
    };
  }

  const historyRow = listFromResponse(historyResp).find(row => alarmHistoryMatchesSceneSeed(row, sceneSeed));
  const historyId = pickAlarmHistoryId(historyRow);
  if (!historyId) {
    return {
      blocked: true,
      reason: 'scene activation completed but no matching alarm history row was visible on the first history page',
      sceneSeed,
      activationResponse: activationResp,
      cleanup: sceneSeed.cleanup
    };
  }

  // The alarm page serializes its default end time to whole seconds. Wait
  // until the next second before handing the fixture to a browser test so a
  // record created at `12:34:56.900` is not excluded by an end bound of
  // `12:34:56`.
  const waitMs = Math.max(100, 1050 - (Date.now() % 1000));
  await new Promise(resolve => setTimeout(resolve, waitMs));

  return {
    id: historyId,
    row: historyRow,
    sceneId: sceneSeed.id,
    sceneName: sceneSeed.name,
    alarmConfigId: sceneSeed.alarmConfigId,
    alarmConfigName: sceneSeed.alarmConfigName,
    activationResponse: activationResp,
    blocked: false,
    cleanup: async () => {
      if (historyId) await apiClient.delete('/alarm/info/history/' + historyId, {}, accountKey);
      await sceneSeed.cleanup();
    }
  };
}

async function ensureSceneAutomation(accountKey = 'tenant_admin') {
  const deviceSeed = await ensureDevice(accountKey);
  const alarmSeed = await ensureAlarmConfig(accountKey);
  if (alarmSeed.blocked) {
    await deviceSeed.cleanup();
    return {
      blocked: true,
      reason: alarmSeed.reason,
      cleanup: alarmSeed.cleanup
    };
  }

  const name = makeRunLabel('seed_scene_auto');
  const basePayload = {
    name,
    description: 'automation seed',
    enabled: 'N',
    trigger_condition_groups: [
      [
        {
          trigger_conditions_type: '10',
          trigger_source: deviceSeed.id,
          trigger_param_type: 'STATUS',
          trigger_operator: '=',
          trigger_value: 'online'
        }
      ]
    ],
    actions: [
      {
        action_type: '30',
        action_target: alarmSeed.id,
        remark: 'trigger seeded alarm config'
      }
    ],
    remark: 'automation seed'
  };

  const createResp = await apiClient.post('/scene_automations', basePayload, accountKey);
  if (createResp.code !== 200) {
    await alarmSeed.cleanup();
    await deviceSeed.cleanup();
    return {
      blocked: true,
      reason: createResp.message || 'scene automation endpoint unavailable',
      cleanup: async () => {}
    };
  }

  const id = pickId(createResp.data);
  return {
    id,
    name,
    payload: basePayload,
    row: createResp.data,
    deviceId: deviceSeed.id,
    alarmConfigId: alarmSeed.id,
    blocked: false,
    cleanup: async () => {
      if (id) await apiClient.delete('/scene_automations/' + id, {}, accountKey);
      await alarmSeed.cleanup();
      await deviceSeed.cleanup();
    }
  };
}

async function withSeededDevice(fn, accountKey = 'tenant_admin') {
  const seed = await ensureDevice(accountKey);
  try {
    return await fn(seed);
  } finally {
    await seed.cleanup();
  }
}

/**
 * 探测测试配置指定的 MQTT broker 是否在监听；未指定时保持 1883 默认值。
 * 走原始 TCP connect，不依赖 mqtt 客户端库，避免环境耦合。
 * 失败/超时一律视为不可用，让上层用 skipIfBlocked 跳过 MQTT 依赖用例。
 */
async function isMqttBrokerAvailable(
  host = mqttRuntime.getMqttEndpoint().server,
  port = mqttRuntime.getMqttEndpoint().port
) {
  return new Promise(resolve => {
    const socket = new net.Socket();
    const cleanup = () => {
      socket.removeAllListeners();
      socket.destroy();
    };
    socket.setTimeout(1500);
    socket.once('connect', () => {
      cleanup();
      resolve(true);
    });
    socket.once('timeout', () => {
      cleanup();
      resolve(false);
    });
    socket.once('error', () => {
      cleanup();
      resolve(false);
    });
    socket.connect(port, host);
  });
}

module.exports = {
  ensureAlarmConfig,
  ensureDevice,
  createSimulationDevice,
  ensureDeviceConfig,
  ensureReadyCheckCommandFixture,
  ensureDeviceWithTelemetry,
  ensureNotificationGroup,
  ensureOtaTaskSupportBundleSource,
  ensureOpenApiKey,
  ensureScene,
  ensureSceneAlarmHistory,
  ensureSceneAutomation,
  publishSimulatedTelemetryAndReadCurrent,
  listFromResponse,
  makeRunLabel,
  pickAlarmHistoryId,
  pickId,
  requireSuccess,
  selectReadyCheckDevice,
  selectRdiDevice,
  ensureRdiDevice,
  getOptInSyntheticRdiPid,
  getRdiCandidatePids,
  withSeededDevice,
  isMqttBrokerAvailable,
  mqttEndpointDescription,
  getMqttEndpoint: mqttRuntime.getMqttEndpoint,
  resolveOtaSeedDatabaseOptions
};
