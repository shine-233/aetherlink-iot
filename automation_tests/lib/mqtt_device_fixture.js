const crypto = require('crypto');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFile, spawn } = require('child_process');
const { promisify } = require('util');
const apiClient = require('./api_client');
const mqttRuntime = require('./mqtt_runtime');

const execFileAsync = promisify(execFile);

function closeWriteStream(stream) {
  if (!stream || stream.writableEnded) return Promise.resolve();
  return new Promise(resolve => stream.end(resolve));
}

function parseVoucher(row) {
  let voucher = row && row.voucher;
  if (typeof voucher === 'string') {
    try { voucher = JSON.parse(voucher); } catch (_) { return null; }
  }
  if (!voucher || typeof voucher !== 'object' || !String(voucher.username || '').trim()) return null;
  return {
    username: String(voucher.username).trim(),
    password: voucher.password == null ? '' : String(voucher.password)
  };
}

function readCommandReceipts(receiptPath) {
  if (!receiptPath || !fs.existsSync(receiptPath)) return [];
  const lines = fs.readFileSync(receiptPath, 'utf8').split(/\r?\n/);
  const receipts = [];
  for (const line of lines) {
    if (!line.trim()) continue;
    try { receipts.push(JSON.parse(line)); } catch (_) { break; }
  }
  return receipts;
}

function readOTAProgressReceipts(receiptPath) {
  if (!receiptPath || !fs.existsSync(receiptPath)) return [];
  const lines = fs.readFileSync(receiptPath, 'utf8').split(/\r?\n/);
  const receipts = [];
  for (const line of lines) {
    if (!line.trim()) continue;
    try { receipts.push(JSON.parse(line)); } catch (_) { break; }
  }
  return receipts;
}

function validateOTAProgressReceipts(receipts, expectedProgress = [0, 10, 50, 100]) {
  if (!Array.isArray(receipts)) {
    throw new Error('OTA progress receipts must be an array');
  }
  const informs = receipts.filter(row => row && row.kind === 'inform');
  const progressRows = receipts.filter(row => row && row.kind === 'progress');
  if (informs.length < 1) {
    throw new Error('OTA emulator did not receive an OTA inform message');
  }
  if (progressRows.length < expectedProgress.length) {
    throw new Error(
      `OTA emulator returned ${progressRows.length} progress receipts; expected at least ${expectedProgress.length}`
    );
  }
  const actualProgress = progressRows
    .slice(0, expectedProgress.length)
    .map(row => Number(row.progress));
  if (actualProgress.some(value => !Number.isInteger(value))) {
    throw new Error('OTA progress receipts must expose integer progress values');
  }
  if (JSON.stringify(actualProgress) !== JSON.stringify(expectedProgress)) {
    throw new Error(
      `OTA progress sequence = ${JSON.stringify(actualProgress)}, want ${JSON.stringify(expectedProgress)}`
    );
  }
  for (const row of progressRows.slice(0, expectedProgress.length)) {
    if (row.topic !== 'ota/devices/progress') {
      throw new Error(`OTA progress receipt has unexpected topic: ${row.topic || '<missing>'}`);
    }
    let payload = row.payload;
    if (typeof payload === 'string') {
      try { payload = JSON.parse(payload); } catch (_) {
        throw new Error('OTA progress receipt payload is not valid JSON');
      }
    }
    if (!payload || payload.method !== 'ota_progress' || !payload.params) {
      throw new Error('OTA progress receipt must contain method=ota_progress and params');
    }
    if (Number(payload.params.progress) !== Number(row.progress)) {
      throw new Error('OTA progress receipt metadata does not match its payload');
    }
  }
  return receipts;
}

function validateShadowCommandReceipts(receipts) {
  if (!Array.isArray(receipts) || receipts.length !== 2) {
    throw new Error(`expected exactly two shadow command receipts, got ${receipts ? receipts.length : 0}`);
  }
  if (new Set(receipts.map(row => row.message_id)).size !== 2 || receipts.some(row => !row.message_id)) {
    throw new Error('shadow command receipts must have a unique message_id');
  }
  if (receipts.some(row => row.method !== 'shadow-e2e')) {
    throw new Error('shadow command receipts must use method shadow-e2e');
  }
  const seqs = receipts.map(row => Number(row.params && row.params.seq)).sort((a, b) => a - b);
  if (seqs[0] !== 1 || seqs[1] !== 2) {
    throw new Error('shadow command receipts must contain seq 1 and 2');
  }
  for (const row of receipts) {
    const topicParts = String(row.topic || '').split('/');
    if (topicParts.length !== 4 || topicParts[0] !== 'devices' || topicParts[1] !== 'command' ||
        !topicParts[2] || topicParts[3] !== row.message_id) {
      throw new Error(`shadow command ${row.message_id || '<missing>'} has an invalid downlink topic`);
    }
    if (row.ack_topic !== `devices/command/response/${row.message_id}` ||
        !row.ack_payload || Number(row.ack_payload.result) !== 0 ||
        row.ack_payload.method !== 'shadow-e2e') {
      throw new Error(`shadow command ${row.message_id || '<missing>'} has no successful ACK evidence`);
    }
  }
  return receipts;
}

function emulatorConfig() {
  return JSON.stringify({
    device_type: 'direct',
    mqtt: { broker: '127.0.0.1:1883', client_id: 'shadow-runtime', username: '', password: '', qos: 1, clean_session: true, keep_alive: 60 },
    device: { device_id: 'runtime-device-id', device_number: 'runtime-device-number' },
    database: { host: '127.0.0.1', port: 5432, dbname: 'aetherlink_iot', username: 'postgres', password: '', sslmode: 'disable', max_open_conns: 2, max_idle_conns: 1 },
    api: { base_url: 'http://127.0.0.1:9999', api_key: 'local-shadow', timeout: 30 },
    test: { wait_db_sync_seconds: 1, wait_mqtt_response_seconds: 5, retry_times: 3, log_level: 'info' }
  }, null, 2);
}

async function buildEmulatorBinary(runtimeDir) {
  const moduleRoot = path.resolve(__dirname, '..', '..', 'backend', 'cmd', 'aetherlink-device-autotest');
  const binaryPath = path.join(runtimeDir, 'shadow-command-emulator.exe');
  await execFileAsync(process.env.AUTOMATION_GO_BIN || 'go', ['build', '-o', binaryPath, './cmd/autotest'], {
    cwd: moduleRoot,
    windowsHide: true,
    timeout: 120000,
    env: { ...process.env }
  });
  return { moduleRoot, binaryPath };
}

async function startMqttCommandDevice(device, accountKey = 'tenant_admin') {
  const detailResp = await apiClient.get('/device/detail/' + device.id, {}, accountKey);
  if (!detailResp || detailResp.code !== 200 || !detailResp.data) {
    throw new Error(`cannot read MQTT fixture device ${device.id}`);
  }
  const detail = detailResp.data;
  const voucher = parseVoucher(device.row) || parseVoucher(detail);
  if (!voucher) throw new Error(`MQTT fixture device ${device.id} has no usable one-time voucher`);
  const deviceNumber = String(detail.device_number || detail.deviceNumber || detail.id || device.id).trim();
  const endpoint = mqttRuntime.getMqttEndpoint();
  const runtimeDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-shadow-mqtt-'));
  const configPath = path.join(runtimeDir, 'config.json');
  const receiptPath = path.join(runtimeDir, 'receipts.jsonl');
  const stderrPath = path.join(runtimeDir, 'stderr.log');
  fs.writeFileSync(configPath, emulatorConfig(), 'utf8');
  let executable;
  try {
    executable = await buildEmulatorBinary(runtimeDir);
  } catch (error) {
    fs.rmSync(runtimeDir, { recursive: true, force: true });
    throw error;
  }
  const { moduleRoot, binaryPath } = executable;
  const stderr = fs.createWriteStream(stderrPath, { flags: 'a' });
  const child = spawn(binaryPath, [
    '-config', configPath,
    '-mode', 'command-emulator',
    '-command-success',
    '-command-receipt-path', receiptPath
  ], {
    cwd: moduleRoot,
    windowsHide: true,
    stdio: ['ignore', 'ignore', 'pipe'],
    env: {
      ...process.env,
      AUTOTEST_MQTT_BROKER: `${endpoint.server}:${endpoint.port}`,
      AUTOTEST_MQTT_CLIENT_ID: `shadow-${crypto.randomUUID()}`.slice(0, 128),
      AUTOTEST_MQTT_USERNAME: voucher.username,
      AUTOTEST_MQTT_PASSWORD: voucher.password,
      AUTOTEST_DEVICE_ID: device.id,
      AUTOTEST_DEVICE_NUMBER: deviceNumber
    }
  });
  child.stderr.pipe(stderr);
  let exit = null;
  const exitPromise = new Promise(resolve => {
    child.once('error', error => { exit = { error }; resolve(exit); });
    child.once('exit', (code, signal) => { exit = { code, signal }; resolve(exit); });
  });

  const onlineDeadline = Date.now() + 20000;
  let online = false;
  while (Date.now() < onlineDeadline && !exit) {
    const response = await apiClient.get('/device/detail/' + device.id, {}, accountKey);
    if (response && response.code === 200 && response.data && Number(response.data.is_online || response.data.device_status) === 1) {
      online = true;
      break;
    }
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  if (exit || !online) {
    if (!exit) child.kill('SIGTERM');
    await Promise.race([exitPromise, new Promise(resolve => setTimeout(resolve, 5000))]);
    const diagnostic = fs.existsSync(stderrPath) ? fs.readFileSync(stderrPath, 'utf8') : '';
    await closeWriteStream(stderr);
    fs.rmSync(runtimeDir, { recursive: true, force: true });
    throw new Error(`MQTT command emulator did not become online: ${diagnostic}`);
  }

  return {
    readReceipts() {
      return readCommandReceipts(receiptPath);
    },
    async waitForReceipts(count = 2, timeoutMs = 45000) {
      const deadline = Date.now() + timeoutMs;
      while (Date.now() < deadline) {
        if (exit) throw new Error(`MQTT command emulator exited while waiting for receipts: ${JSON.stringify(exit)}`);
        const receipts = readCommandReceipts(receiptPath);
        if (receipts.length >= count) {
          await new Promise(resolve => setTimeout(resolve, 1000));
          return readCommandReceipts(receiptPath);
        }
        await new Promise(resolve => setTimeout(resolve, 250));
      }
      throw new Error(`expected ${count} MQTT command receipts within ${timeoutMs}ms; got ${readCommandReceipts(receiptPath).length}`);
    },
    async cleanup() {
      if (!exit) child.kill('SIGTERM');
      await Promise.race([exitPromise, new Promise(resolve => setTimeout(resolve, 5000))]);
      await closeWriteStream(stderr);
      fs.rmSync(runtimeDir, { recursive: true, force: true });
    }
  };
}

async function startMqttOTADevice(device, accountKey = 'tenant_admin', options = {}) {
  const detailResp = await apiClient.get('/device/detail/' + device.id, {}, accountKey);
  if (!detailResp || detailResp.code !== 200 || !detailResp.data) {
    throw new Error(`cannot read OTA MQTT fixture device ${device.id}`);
  }
  const detail = detailResp.data;
  const voucher = parseVoucher(device.row) || parseVoucher(detail);
  if (!voucher) throw new Error(`OTA MQTT fixture device ${device.id} has no usable one-time voucher`);

  const configuredProgress = Array.isArray(options.progressValues) && options.progressValues.length > 0
    ? options.progressValues.map(value => Number(value))
    : [0, 10, 50, 100];
  if (configuredProgress.some(value => !Number.isInteger(value) || value < 0 || value > 100)) {
    throw new Error('OTA progress values must be integers between 0 and 100');
  }
  const otaVersion = String(options.version || '').trim();
  const deviceNumber = String(detail.device_number || detail.deviceNumber || detail.id || device.id).trim();
  const endpoint = mqttRuntime.getMqttEndpoint();
  const runtimeDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-ota-mqtt-'));
  const configPath = path.join(runtimeDir, 'config.json');
  const receiptPath = path.join(runtimeDir, 'receipts.jsonl');
  const stderrPath = path.join(runtimeDir, 'stderr.log');
  fs.writeFileSync(configPath, emulatorConfig(), 'utf8');
  let executable;
  try {
    executable = await buildEmulatorBinary(runtimeDir);
  } catch (error) {
    fs.rmSync(runtimeDir, { recursive: true, force: true });
    throw error;
  }

  const { moduleRoot, binaryPath } = executable;
  const stderr = fs.createWriteStream(stderrPath, { flags: 'a' });
  const args = [
    '-config', configPath,
    '-mode', 'ota-emulator',
    '-ota-progress-path', receiptPath,
    '-ota-progress-values', configuredProgress.join(',')
  ];
  if (otaVersion) args.push('-ota-version', otaVersion);
  if (options.failure) args.push('-ota-failure');
  const child = spawn(binaryPath, args, {
    cwd: moduleRoot,
    windowsHide: true,
    stdio: ['ignore', 'ignore', 'pipe'],
    env: {
      ...process.env,
      AUTOTEST_MQTT_BROKER: `${endpoint.server}:${endpoint.port}`,
      AUTOTEST_MQTT_CLIENT_ID: `ota-${crypto.randomUUID()}`.slice(0, 128),
      AUTOTEST_MQTT_USERNAME: voucher.username,
      AUTOTEST_MQTT_PASSWORD: voucher.password,
      AUTOTEST_DEVICE_ID: device.id,
      AUTOTEST_DEVICE_NUMBER: deviceNumber
    }
  });
  child.stderr.pipe(stderr);
  let exit = null;
  const exitPromise = new Promise(resolve => {
    child.once('error', error => { exit = { error }; resolve(exit); });
    child.once('exit', (code, signal) => { exit = { code, signal }; resolve(exit); });
  });

  const onlineDeadline = Date.now() + Number(options.onlineTimeoutMs || 20000);
  let online = false;
  while (Date.now() < onlineDeadline && !exit) {
    const response = await apiClient.get('/device/detail/' + device.id, {}, accountKey);
    const row = response && response.code === 200 ? response.data : null;
    if (row && (Number(row.is_online) === 1 || Number(row.device_status) === 1)) {
      online = true;
      break;
    }
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  if (exit || !online) {
    if (!exit) child.kill('SIGTERM');
    await Promise.race([exitPromise, new Promise(resolve => setTimeout(resolve, 5000))]);
    const diagnostic = fs.existsSync(stderrPath) ? fs.readFileSync(stderrPath, 'utf8') : '';
    await closeWriteStream(stderr);
    fs.rmSync(runtimeDir, { recursive: true, force: true });
    throw new Error(`MQTT OTA emulator did not become online: ${diagnostic}`);
  }

  return {
    receiptPath,
    expectedProgress: configuredProgress,
    readReceipts() {
      return readOTAProgressReceipts(receiptPath);
    },
    async waitForOTAProgress(timeoutMs = 60000) {
      const deadline = Date.now() + timeoutMs;
      while (Date.now() < deadline) {
        if (exit) throw new Error(`MQTT OTA emulator exited while waiting for progress: ${JSON.stringify(exit)}`);
        const receipts = readOTAProgressReceipts(receiptPath);
        const progressCount = receipts.filter(row => row && row.kind === 'progress').length;
        if (progressCount >= configuredProgress.length) {
          await new Promise(resolve => setTimeout(resolve, 250));
          return readOTAProgressReceipts(receiptPath);
        }
        await new Promise(resolve => setTimeout(resolve, 250));
      }
      throw new Error(
        `expected OTA inform plus ${configuredProgress.length} progress receipts within ${timeoutMs}ms; got ` +
        JSON.stringify(readOTAProgressReceipts(receiptPath))
      );
    },
    async cleanup() {
      if (!exit) child.kill('SIGTERM');
      await Promise.race([exitPromise, new Promise(resolve => setTimeout(resolve, 5000))]);
      await closeWriteStream(stderr);
      fs.rmSync(runtimeDir, { recursive: true, force: true });
    }
  };
}

// Publish one caller-supplied telemetry payload through a real authenticated
// Paho device session. The process exits after the publish token completes;
// callers must verify the backend-visible outcome separately because an MQTT
// publish token only proves broker acceptance, not downstream persistence.
async function publishMqttTelemetryPayload(device, mode, payload, accountKey = 'tenant_admin') {
  const detailResp = await apiClient.get('/device/detail/' + device.id, {}, accountKey);
  if (!detailResp || detailResp.code !== 200 || !detailResp.data) {
    throw new Error(`cannot read MQTT telemetry fixture device ${device.id}`);
  }
  const detail = detailResp.data;
  const voucher = parseVoucher(device.row) || parseVoucher(detail);
  if (!voucher) throw new Error(`MQTT telemetry fixture device ${device.id} has no usable one-time voucher`);
  if (mode === 'telemetry-json' && (!payload || typeof payload !== 'object' || Array.isArray(payload))) {
    throw new Error('MQTT telemetry fixture payload must be a JSON object');
  }
  if (mode === 'telemetry-raw' && (typeof payload !== 'string' || payload.length === 0)) {
    throw new Error('MQTT raw telemetry fixture payload must be a non-empty string');
  }

  const endpoint = mqttRuntime.getMqttEndpoint();
  const runtimeDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-telemetry-mqtt-'));
  const configPath = path.join(runtimeDir, 'config.json');
  const stderrPath = path.join(runtimeDir, 'stderr.log');
  fs.writeFileSync(configPath, emulatorConfig(), 'utf8');
  let binaryPath;
  try {
    ({ binaryPath } = await buildEmulatorBinary(runtimeDir));
  } catch (error) {
    fs.rmSync(runtimeDir, { recursive: true, force: true });
    throw error;
  }

  const stderr = fs.createWriteStream(stderrPath, { flags: 'a' });
  const args = ['-config', configPath, '-mode', mode];
  args.push(mode === 'telemetry-json' ? '-telemetry-payload' : '-telemetry-raw-payload');
  args.push(mode === 'telemetry-json' ? JSON.stringify(payload) : payload);
  const child = spawn(binaryPath, args, {
    cwd: path.resolve(__dirname, '..', '..', 'backend', 'cmd', 'aetherlink-device-autotest'),
    windowsHide: true,
    stdio: ['ignore', 'ignore', 'pipe'],
    env: {
      ...process.env,
      AUTOTEST_MQTT_BROKER: `${endpoint.server}:${endpoint.port}`,
      AUTOTEST_MQTT_CLIENT_ID: `telemetry-${crypto.randomUUID()}`.slice(0, 128),
      AUTOTEST_MQTT_USERNAME: voucher.username,
      AUTOTEST_MQTT_PASSWORD: voucher.password,
      AUTOTEST_DEVICE_ID: device.id,
      AUTOTEST_DEVICE_NUMBER: String(detail.device_number || detail.deviceNumber || detail.id || device.id)
    }
  });
  child.stderr.pipe(stderr);
  const result = await new Promise(resolve => {
    child.once('error', error => resolve({ error }));
    child.once('exit', (code, signal) => resolve({ code, signal }));
  });
  await closeWriteStream(stderr);
  const diagnostic = fs.existsSync(stderrPath) ? fs.readFileSync(stderrPath, 'utf8') : '';
  fs.rmSync(runtimeDir, { recursive: true, force: true });
  if (result.error) throw result.error;
  if (result.code !== 0) {
    throw new Error(`MQTT telemetry device exited with code ${result.code}: ${diagnostic}`);
  }
  return { ...result, endpoint: `${endpoint.server}:${endpoint.port}` };
}

async function publishMqttTelemetry(device, payload, accountKey = 'tenant_admin') {
  return publishMqttTelemetryPayload(device, 'telemetry-json', payload, accountKey);
}

// Publish raw bytes for protocol-negative checks. This deliberately bypasses
// the JSON parser in the device CLI so the broker/backend schema gate is the
// component under test.
async function publishMqttRawTelemetry(device, rawPayload, accountKey = 'tenant_admin') {
  return publishMqttTelemetryPayload(device, 'telemetry-raw', rawPayload, accountKey);
}

module.exports = {
  readCommandReceipts,
  readOTAProgressReceipts,
  validateShadowCommandReceipts,
  validateOTAProgressReceipts,
  startMqttCommandDevice,
  startMqttOTADevice,
  publishMqttTelemetry,
  publishMqttRawTelemetry
};
