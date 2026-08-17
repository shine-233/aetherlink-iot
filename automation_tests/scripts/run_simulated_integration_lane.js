#!/usr/bin/env node
/**
 * Run the repository's offline, fail-closed simulated integration lane.
 *
 * This is intentionally an executable simulation rather than a claim that
 * the real AetherLink environment is available. It starts an HTTP API on
 * 127.0.0.1, routes MQTT-like messages through an in-memory broker, replays a
 * synthetic RDI envelope, inspects Compose statically, and invokes the local
 * source SBOM generator. It never contacts an external API, broker, registry,
 * device, or Docker daemon as part of the simulated pass.
 */
'use strict';

const crypto = require('crypto');
const fs = require('fs');
const http = require('http');
const path = require('path');
const { spawnSync } = require('child_process');

const PROJECT_ROOT = path.resolve(__dirname, '..', '..');
const AUTOMATION_ROOT = path.resolve(__dirname, '..');
const SCHEMA = 'aetherlink.simulated-integration-lane.v1';
const SYNTHETIC_EMAIL = 'simulated-user@example.test';
const SYNTHETIC_PASSWORD = 'simulated-password-not-a-secret';
const SYNTHETIC_PROVENANCE = 'synthetic-rdi';
const RDI_EVIDENCE_CLASS = 'protocol-emulator';
const TOPICS = Object.freeze({
  status: deviceId => `devices/status/${deviceId}`,
  telemetry: 'devices/telemetry',
  command: (pid, messageId) => `devices/command/${pid}/${messageId}`,
  response: messageId => `devices/command/response/${messageId}`
});

class LaneError extends Error {
  constructor(message, code = 'lane-failed') {
    super(message);
    this.code = code;
  }
}

function now() {
  return new Date().toISOString();
}

function assert(condition, message) {
  if (!condition) throw new LaneError(message);
}

function sha256(value) {
  return crypto.createHash('sha256').update(String(value)).digest('hex');
}

function randomId(prefix) {
  return `${prefix}-${crypto.randomBytes(8).toString('hex')}`;
}

function isObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function redact(value) {
  return String(value || '')
    .replace(/Bearer\s+[A-Za-z0-9._-]+/gi, 'Bearer [REDACTED]')
    .replace(/(password|passwd|secret|token|authorization|cookie)\s*[:=]\s*[^\s,}]+/gi, '$1=[REDACTED]');
}

function check(name, ok, evidence, extra = {}) {
  return { name, ok: Boolean(ok), evidence, ...extra };
}

function assertNoRealClaim(value, label) {
  const text = JSON.stringify(value);
  assert(!/real[-_ ]?(api|mqtt|rdi|device)[-_ ]?passed/i.test(text), `${label} contains a forbidden real-pass claim`);
}

function makeEvidence(extra = {}) {
  const evidence = {
    fixture_provenance: SYNTHETIC_PROVENANCE,
    evidence_class: RDI_EVIDENCE_CLASS,
    device_execution: 'not-proven',
    real_rdi_status: 'not-tested',
    production_signoff: 'not-ready',
    ...extra
  };
  assert(evidence.fixture_provenance === SYNTHETIC_PROVENANCE, 'fixture_provenance must remain synthetic-rdi');
  assert(evidence.evidence_class === RDI_EVIDENCE_CLASS, 'synthetic RDI evidence class changed unexpectedly');
  assert(evidence.device_execution === 'not-proven', 'synthetic evidence cannot prove device execution');
  assert(evidence.real_rdi_status === 'not-tested', 'synthetic evidence cannot mark real RDI tested');
  assert(evidence.production_signoff === 'not-ready', 'synthetic evidence cannot grant production signoff');
  assertNoRealClaim(evidence, 'synthetic evidence');
  return evidence;
}

function parseArguments(argv) {
  const options = {
    json: false,
    reportDir: null,
    apiMode: 'simulated',
    mqttMode: 'simulated',
    rdiMode: 'synthetic',
    deploymentMode: 'dry-run',
    help: false
  };

  const modes = {
    apiMode: ['simulated', 'real'],
    mqttMode: ['simulated', 'real'],
    rdiMode: ['synthetic', 'real'],
    deploymentMode: ['dry-run', 'real']
  };

  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === '--json') {
      options.json = true;
      continue;
    }
    if (argument === '--help' || argument === '-h') {
      options.help = true;
      continue;
    }
    if (argument === '--report-dir') {
      options.reportDir = argv[++index];
      if (!options.reportDir || options.reportDir.startsWith('-')) {
        throw new LaneError('--report-dir requires a path', 'invalid-arguments');
      }
      continue;
    }
    const modeMatch = argument.match(/^--(api-mode|mqtt-mode|rdi-mode|deployment-mode)=(.+)$/);
    if (modeMatch) {
      const key = {
        'api-mode': 'apiMode',
        'mqtt-mode': 'mqttMode',
        'rdi-mode': 'rdiMode',
        'deployment-mode': 'deploymentMode'
      }[modeMatch[1]];
      options[key] = modeMatch[2];
      continue;
    }
    throw new LaneError(`unknown argument: ${argument}`, 'invalid-arguments');
  }

  for (const [key, allowed] of Object.entries(modes)) {
    if (!allowed.includes(options[key])) {
      throw new LaneError(`${key} must be one of ${allowed.join(', ')}`, 'invalid-arguments');
    }
  }
  return options;
}

function helpText() {
  return [
    'Usage: node automation_tests/scripts/run_simulated_integration_lane.js [options]',
    '',
    'Options:',
    '  --json                         print one machine-readable JSON report',
    '  --report-dir <path>            persist summary.json and lane evidence',
    '  --api-mode=simulated|real      real mode is fail-closed and never guessed',
    '  --mqtt-mode=simulated|real     real mode is fail-closed and never guessed',
    '  --rdi-mode=synthetic|real      real mode is fail-closed and never guessed',
    '  --deployment-mode=dry-run|real real mode is fail-closed and never guessed',
    '  -h, --help                     show this help'
  ].join('\n') + '\n';
}

function topicMatches(filter, topic) {
  const filterParts = String(filter).split('/');
  const topicParts = String(topic).split('/');
  for (let index = 0; index < filterParts.length; index += 1) {
    const filterPart = filterParts[index];
    if (filterPart === '#') return true;
    if (topicParts[index] === undefined) return false;
    if (filterPart !== '+' && filterPart !== topicParts[index]) return false;
  }
  return filterParts.length === topicParts.length;
}

class InMemoryMqttBroker {
  constructor() {
    this.subscriptions = new Map();
    this.clients = new Map();
    this.messages = [];
  }

  connect(clientId) {
    this.clients.set(clientId, { connectedAt: now() });
  }

  disconnect(clientId) {
    this.clients.delete(clientId);
  }

  subscribe(clientId, filter, handler) {
    const subscription = { clientId, filter, handler };
    const list = this.subscriptions.get(clientId) || [];
    list.push(subscription);
    this.subscriptions.set(clientId, list);
    return () => {
      const current = this.subscriptions.get(clientId) || [];
      this.subscriptions.set(clientId, current.filter(item => item !== subscription));
    };
  }

  async publish(topic, payload, publisher = 'simulated-publisher') {
    const message = {
      topic,
      payload: isObject(payload) ? JSON.parse(JSON.stringify(payload)) : payload,
      publisher,
      published_at: now()
    };
    this.messages.push(message);
    const deliveries = [];
    for (const list of this.subscriptions.values()) {
      for (const subscription of list) {
        if (topicMatches(subscription.filter, topic)) {
          deliveries.push(Promise.resolve().then(() => subscription.handler(message)));
        }
      }
    }
    await Promise.all(deliveries);
    return message;
  }

  topics() {
    return [...new Set(this.messages.map(message => message.topic))];
  }
}

function waitFor(predicate, { timeoutMs = 2000, intervalMs = 5, label = 'condition' } = {}) {
  const deadline = Date.now() + timeoutMs;
  return new Promise((resolve, reject) => {
    const poll = () => {
      let result;
      try {
        result = predicate();
      } catch (error) {
        reject(error);
        return;
      }
      if (result) {
        resolve(result);
        return;
      }
      if (Date.now() >= deadline) {
        reject(new LaneError(`${label} was not observed before timeout`));
        return;
      }
      setTimeout(poll, intervalMs);
    };
    poll();
  });
}

function parseJsonBody(request) {
  return new Promise((resolve, reject) => {
    let body = '';
    request.on('data', chunk => {
      body += chunk.toString('utf8');
      if (body.length > 1024 * 1024) {
        reject(new LaneError('request body exceeds simulated API limit'));
        request.destroy();
      }
    });
    request.on('end', () => {
      if (!body) {
        resolve({});
        return;
      }
      try {
        resolve(JSON.parse(body));
      } catch (error) {
        reject(new LaneError(`invalid JSON request: ${error.message}`));
      }
    });
    request.on('error', reject);
  });
}

function sendJson(response, statusCode, body) {
  const text = JSON.stringify(body);
  response.writeHead(statusCode, {
    'content-type': 'application/json; charset=utf-8',
    'content-length': Buffer.byteLength(text),
    'cache-control': 'no-store'
  });
  response.end(text);
}

function bearerToken(request) {
  const header = request.headers.authorization || '';
  return header.replace(/^Bearer\s+/i, '').trim();
}

class SimulatedBackend {
  constructor(broker) {
    this.broker = broker;
    this.sessions = new Map();
    this.devices = new Map();
    this.commands = new Map();
    this.status = new Map();
    this.telemetry = new Map();
    this.unsubscribe = [];
    this.broker.connect('simulated-backend');
    this.unsubscribe.push(this.broker.subscribe('simulated-backend', 'devices/status/+', message => {
      const deviceId = message.payload && message.payload.device_id;
      if (deviceId) this.status.set(deviceId, message.payload);
    }));
    this.unsubscribe.push(this.broker.subscribe('simulated-backend', TOPICS.telemetry, message => {
      const deviceId = message.payload && message.payload.device_id;
      if (deviceId) this.telemetry.set(deviceId, message.payload);
    }));
  }

  login(email, password) {
    if (email !== SYNTHETIC_EMAIL || password !== SYNTHETIC_PASSWORD) return null;
    const token = `sim-token-${crypto.randomBytes(24).toString('hex')}`;
    this.sessions.set(token, { email, created_at: now() });
    return token;
  }

  session(token) {
    return this.sessions.get(token) || null;
  }

  registerDevice({ name }) {
    const deviceId = randomId('synthetic-device');
    const pid = `SYN${crypto.randomBytes(5).toString('hex').toUpperCase()}`;
    const device = {
      id: deviceId,
      pid,
      name: String(name || 'synthetic device'),
      fixture_provenance: SYNTHETIC_PROVENANCE,
      created_at: now(),
      deleted: false
    };
    this.devices.set(deviceId, device);
    return device;
  }

  getDevice(deviceId) {
    const device = this.devices.get(deviceId);
    return device && !device.deleted ? device : null;
  }

  async waitForResponse(topic, timeoutMs = 1500) {
    const clientId = `simulated-command-waiter-${crypto.randomBytes(4).toString('hex')}`;
    this.broker.connect(clientId);
    return new Promise((resolve, reject) => {
      let done = false;
      const unsubscribe = this.broker.subscribe(clientId, topic, message => {
        if (done) return;
        done = true;
        clearTimeout(timer);
        unsubscribe();
        this.broker.disconnect(clientId);
        resolve(message.payload);
      });
      const timer = setTimeout(() => {
        if (done) return;
        done = true;
        unsubscribe();
        this.broker.disconnect(clientId);
        reject(new LaneError(`no simulated command response for ${topic}`));
      }, timeoutMs);
    });
  }

  async command(deviceId, outcome = 'success') {
    const device = this.getDevice(deviceId);
    assert(device, `cannot command unknown device ${deviceId}`);
    const messageId = randomId('sim-message');
    const maxAttempts = outcome === 'retry' ? 2 : 1;
    let lastResponse = null;
    let attempts = 0;
    for (attempts = 1; attempts <= maxAttempts; attempts += 1) {
      const responsePromise = this.waitForResponse(TOPICS.response(messageId));
      await this.broker.publish(TOPICS.command(device.pid, messageId), {
        message_id: messageId,
        device_id: deviceId,
        pid: device.pid,
        identifier: 'simulated.command',
        outcome,
        attempt: attempts
      }, 'simulated-backend');
      lastResponse = await responsePromise;
      if (Number(lastResponse.result) === 0 || !lastResponse.retryable || attempts === maxAttempts) break;
    }
    const result = {
      message_id: messageId,
      device_id: deviceId,
      outcome,
      attempts,
      response: lastResponse,
      status: Number(lastResponse && lastResponse.result) === 0 ? 'success' : 'failure'
    };
    this.commands.set(messageId, result);
    return result;
  }

  async deleteDevice(deviceId) {
    const device = this.devices.get(deviceId);
    if (!device) return false;
    device.deleted = true;
    this.devices.set(deviceId, device);
    return true;
  }
}

class SimulatedRdiDevice {
  constructor({ broker, backend, device }) {
    this.broker = broker;
    this.backend = backend;
    this.device = device;
    this.clientId = `synthetic-rdi-client-${crypto.randomBytes(4).toString('hex')}`;
    this.unsubscribe = null;
    this.connected = false;
    this.attempts = new Map();
  }

  async connect() {
    this.broker.connect(this.clientId);
    this.unsubscribe = this.broker.subscribe(
      this.clientId,
      `devices/command/${this.device.pid}/#`,
      message => this.handleCommand(message)
    );
    this.connected = true;
    await this.broker.publish(TOPICS.status(this.device.id), {
      device_id: this.device.id,
      pid: this.device.pid,
      status: 'online',
      fixture_provenance: SYNTHETIC_PROVENANCE,
      timestamp: Date.now()
    }, this.clientId);
    await this.broker.publish(TOPICS.telemetry, {
      device_id: this.device.id,
      pid: this.device.pid,
      key: 'temperature_1',
      value: 25.5,
      fixture_provenance: SYNTHETIC_PROVENANCE,
      timestamp: Date.now()
    }, this.clientId);
  }

  async handleCommand(message) {
    const payload = message.payload || {};
    const messageId = String(payload.message_id || '');
    if (!messageId) return;
    const attempts = (this.attempts.get(messageId) || 0) + 1;
    this.attempts.set(messageId, attempts);
    const retryFirst = payload.outcome === 'retry' && attempts === 1;
    const failed = payload.outcome === 'failure' || retryFirst;
    await this.broker.publish(TOPICS.response(messageId), {
      message_id: messageId,
      device_id: this.device.id,
      pid: this.device.pid,
      result: failed ? 1 : 0,
      message: failed ? 'failed' : 'success',
      retryable: retryFirst,
      fixture_provenance: SYNTHETIC_PROVENANCE,
      timestamp: Date.now()
    }, this.clientId);
  }

  async disconnect() {
    if (!this.connected) return;
    await this.broker.publish(TOPICS.status(this.device.id), {
      device_id: this.device.id,
      pid: this.device.pid,
      status: 'offline',
      fixture_provenance: SYNTHETIC_PROVENANCE,
      timestamp: Date.now()
    }, this.clientId);
    if (this.unsubscribe) this.unsubscribe();
    this.broker.disconnect(this.clientId);
    this.connected = false;
  }
}

function createApiServer(backend) {
  return http.createServer(async (request, response) => {
    try {
      const parsed = new URL(request.url, 'http://127.0.0.1');
      const segments = parsed.pathname.split('/').filter(Boolean);
      const token = bearerToken(request);
      const authenticated = backend.session(token);

      if (request.method === 'GET' && parsed.pathname === '/api/health') {
        sendJson(response, 200, { status: 'ok', mode: 'simulated' });
        return;
      }
      if (request.method === 'POST' && parsed.pathname === '/api/auth/login') {
        const body = await parseJsonBody(request);
        const issuedToken = backend.login(body.email, body.password);
        if (!issuedToken) {
          sendJson(response, 401, { status: 'unauthorized' });
          return;
        }
        sendJson(response, 200, { status: 'ok', token: issuedToken, mode: 'simulated' });
        return;
      }
      if (!authenticated) {
        sendJson(response, 401, { status: 'unauthorized' });
        return;
      }
      if (request.method === 'GET' && parsed.pathname === '/api/session') {
        sendJson(response, 200, { status: 'ok', email: authenticated.email, mode: 'simulated' });
        return;
      }
      if (request.method === 'POST' && parsed.pathname === '/api/devices') {
        const body = await parseJsonBody(request);
        const device = backend.registerDevice(body);
        sendJson(response, 201, { status: 'created', device });
        return;
      }

      const deviceMatch = parsed.pathname.match(/^\/api\/devices\/([^/]+)(?:\/(status|telemetry|commands))?$/);
      if (deviceMatch) {
        const device = backend.getDevice(deviceMatch[1]);
        if (!device) {
          sendJson(response, 404, { status: 'not_found' });
          return;
        }
        const suffix = deviceMatch[2];
        if (request.method === 'GET' && suffix === 'status') {
          sendJson(response, 200, {
            status: 'ok',
            device_id: device.id,
            online: backend.status.get(device.id)?.status === 'online',
            source: 'simulated-mqtt'
          });
          return;
        }
        if (request.method === 'GET' && suffix === 'telemetry') {
          sendJson(response, 200, { status: 'ok', telemetry: backend.telemetry.get(device.id) || null });
          return;
        }
        if (request.method === 'POST' && suffix === 'commands') {
          const body = await parseJsonBody(request);
          const result = await backend.command(device.id, body.outcome || 'success');
          sendJson(response, 200, result);
          return;
        }
      }

      const deleteMatch = parsed.pathname.match(/^\/api\/devices\/([^/]+)$/);
      if (request.method === 'DELETE' && deleteMatch) {
        const deleted = await backend.deleteDevice(deleteMatch[1]);
        sendJson(response, deleted ? 200 : 404, { status: deleted ? 'deleted' : 'not_found' });
        return;
      }
      sendJson(response, 404, { status: 'not_found' });
    } catch (error) {
      if (!response.headersSent) sendJson(response, 500, { status: 'error', message: redact(error.message) });
      else response.destroy();
    }
  });
}

function listen(server) {
  return new Promise((resolve, reject) => {
    const onError = error => {
      server.removeListener('listening', onListening);
      reject(error);
    };
    const onListening = () => {
      server.removeListener('error', onError);
      const address = server.address();
      resolve({ host: '127.0.0.1', port: address.port, url: `http://127.0.0.1:${address.port}` });
    };
    server.once('error', onError);
    server.once('listening', onListening);
    server.listen(0, '127.0.0.1');
  });
}

function close(server) {
  return new Promise((resolve, reject) => {
    if (!server.listening) return resolve();
    server.close(error => (error ? reject(error) : resolve()));
  });
}

async function fetchJson(url, options = {}) {
  const response = await fetch(url, {
    ...options,
    headers: { accept: 'application/json', 'content-type': 'application/json', ...(options.headers || {}) }
  });
  const text = await response.text();
  let body;
  try {
    body = JSON.parse(text);
  } catch (error) {
    throw new LaneError(`simulated API returned invalid JSON (${response.status}): ${redact(text)}`);
  }
  return { response, body };
}

function authHeaders(token) {
  return { authorization: `Bearer ${token}` };
}

async function runApiLoginLane(baseUrl, backend) {
  const checks = [];
  const health = await fetchJson(`${baseUrl}/api/health`);
  checks.push(check('local API health', health.response.status === 200 && health.body.status === 'ok', 'in-process loopback HTTP server responded'));
  const login = await fetchJson(`${baseUrl}/api/auth/login`, {
    method: 'POST',
    body: JSON.stringify({ email: SYNTHETIC_EMAIL, password: SYNTHETIC_PASSWORD })
  });
  const token = login.body.token;
  checks.push(check('synthetic account login', login.response.status === 200 && typeof token === 'string' && token.length > 20, 'token was issued in memory and is not persisted'));
  const session = await fetchJson(`${baseUrl}/api/session`, { headers: authHeaders(token) });
  checks.push(check('authenticated session read', session.response.status === 200 && session.body.email === SYNTHETIC_EMAIL, 'Bearer token accepted by the in-process API'));
  assert(checks.every(item => item.ok), 'simulated API login assertions did not all pass');
  assert(!backend.sessions.size || [...backend.sessions.keys()].every(value => value.startsWith('sim-token-')), 'simulated session store contains a non-synthetic token');
  return { token, checks, account: SYNTHETIC_EMAIL, token_reported: false };
}

async function runBusinessE2eLane(baseUrl, backend, broker, token) {
  const checks = [];
  let device = null;
  let simulator = null;
  let created = false;
  let cleanup = { device_disconnected: false, api_delete_attempted: false, api_delete_succeeded: false };
  try {
    const create = await fetchJson(`${baseUrl}/api/devices`, {
      method: 'POST', headers: authHeaders(token), body: JSON.stringify({ name: 'synthetic business device' })
    });
    assert(create.response.status === 201 && create.body.device && create.body.device.id, 'business E2E device creation did not return an ID');
    device = create.body.device;
    created = true;
    checks.push(check('create synthetic device', true, 'API returned a newly allocated synthetic device ID'));

    simulator = new SimulatedRdiDevice({ broker, backend, device });
    await simulator.connect();
    const status = await fetchJson(`${baseUrl}/api/devices/${device.id}/status`, { headers: authHeaders(token) });
    checks.push(check('observe device online state', status.response.status === 200 && status.body.online === true, 'status came from the simulated MQTT status topic'));
    const telemetry = await fetchJson(`${baseUrl}/api/devices/${device.id}/telemetry`, { headers: authHeaders(token) });
    checks.push(check('observe telemetry business state', telemetry.response.status === 200 && telemetry.body.telemetry?.key === 'temperature_1' && Number(telemetry.body.telemetry.value) === 25.5, 'telemetry assertion checked key and value'));

    for (const outcome of ['success', 'failure', 'retry']) {
      const command = await fetchJson(`${baseUrl}/api/devices/${device.id}/commands`, {
        method: 'POST', headers: authHeaders(token), body: JSON.stringify({ outcome })
      });
      const expectedStatus = outcome === 'success' || outcome === 'retry' ? 'success' : 'failure';
      const expectedAttempts = outcome === 'retry' ? 2 : 1;
      const ok = command.response.status === 200 && command.body.status === expectedStatus && command.body.attempts === expectedAttempts && command.body.response?.message === (expectedStatus === 'success' ? 'success' : 'failed');
      checks.push(check(`business command ${outcome}`, ok, `asserted terminal result and attempt count for ${outcome}`));
    }
  } finally {
    if (simulator) {
      await simulator.disconnect();
      cleanup.device_disconnected = true;
    }
    if (created && device) {
      cleanup.api_delete_attempted = true;
      const deleted = await fetchJson(`${baseUrl}/api/devices/${device.id}`, { method: 'DELETE', headers: authHeaders(token) });
      cleanup.api_delete_succeeded = deleted.response.status === 200 && deleted.body.status === 'deleted';
      checks.push(check('cleanup synthetic device', cleanup.api_delete_succeeded, 'device was disconnected and deleted through the simulated API'));
    }
  }
  assert(checks.every(item => item.ok), 'simulated business E2E assertions did not all pass');
  assert(cleanup.device_disconnected && cleanup.api_delete_succeeded, 'simulated business E2E cleanup evidence is incomplete');
  return { checks, cleanup, browser: false, business_assertions: true };
}

async function runMqttLane(backend, broker) {
  const checks = [];
  const device = backend.registerDevice({ name: 'synthetic MQTT contract device' });
  const simulator = new SimulatedRdiDevice({ broker, backend, device });
  try {
    await simulator.connect();
    checks.push(check('online status topic', backend.status.get(device.id)?.status === 'online', TOPICS.status(device.id)));
    checks.push(check('telemetry topic', backend.telemetry.get(device.id)?.value === 25.5, TOPICS.telemetry));
    const success = await backend.command(device.id, 'success');
    const failure = await backend.command(device.id, 'failure');
    const retry = await backend.command(device.id, 'retry');
    checks.push(check('command ACK success', success.status === 'success' && success.attempts === 1 && success.response.result === 0, TOPICS.response(success.message_id)));
    checks.push(check('command ACK failure', failure.status === 'failure' && failure.attempts === 1 && failure.response.result === 1, TOPICS.response(failure.message_id)));
    checks.push(check('command ACK retry', retry.status === 'success' && retry.attempts === 2 && retry.response.result === 0, 'first retryable failure followed by success'));
    await simulator.disconnect();
    checks.push(check('offline status topic', backend.status.get(device.id)?.status === 'offline', TOPICS.status(device.id)));
  } finally {
    if (simulator.connected) await simulator.disconnect();
    await backend.deleteDevice(device.id);
  }
  const topics = broker.topics();
  checks.push(check('canonical topic coverage', [
    topics.some(topic => topic.startsWith('devices/status/')),
    topics.includes(TOPICS.telemetry),
    topics.some(topic => topic.startsWith('devices/command/')),
    topics.some(topic => topic.startsWith('devices/command/response/'))
  ].every(Boolean), 'status, telemetry, command, and command-response topics were observed'));
  assert(checks.every(item => item.ok), 'simulated MQTT broker assertions did not all pass');
  return { checks, broker: 'in-memory-router', external_connection: 'not-run', message_count: broker.messages.length };
}

async function runRdiLane(backend, broker) {
  const checks = [];
  const device = backend.registerDevice({ name: 'synthetic RDI replay device' });
  const simulator = new SimulatedRdiDevice({ broker, backend, device });
  const evidence = makeEvidence({
    schema: 'aetherlink.synthetic-rdi.replay.v1',
    pid: device.pid,
    device_id: device.id,
    fixture_id: `${device.pid}-fixture`,
    hardware_identity: { kind: 'synthetic', serial: `SYNTH-HW-${sha256(device.id).slice(0, 12)}` },
    claim_scope: 'protocol-envelope-replay-only'
  });
  try {
    await simulator.connect();
    const command = await backend.command(device.id, 'success');
    const requiredTopics = [
      TOPICS.status(device.id),
      TOPICS.telemetry,
      TOPICS.command(device.pid, command.message_id),
      TOPICS.response(command.message_id)
    ];
    checks.push(check('synthetic fixture provenance', evidence.fixture_provenance === SYNTHETIC_PROVENANCE, 'fixture_provenance=synthetic-rdi'));
    checks.push(check('protocol topic replay', requiredTopics.every(topic => broker.messages.some(message => message.topic === topic)), 'status, telemetry, command, response'));
    checks.push(check('synthetic ACK result', command.status === 'success' && command.response.result === 0, 'protocol emulator produced an ACK payload'));
  } finally {
    await simulator.disconnect();
    await backend.deleteDevice(device.id);
  }
  assert(checks.every(item => item.ok), 'synthetic RDI replay assertions did not all pass');
  return { checks, evidence, real_rdi_status: 'not-tested', device_execution: 'not-proven' };
}

function parseComposeServices(source) {
  const lines = source.replace(/\r\n/g, '\n').split('\n');
  const servicesIndex = lines.indexOf('services:');
  if (servicesIndex < 0) return [];
  const services = [];
  for (const line of lines.slice(servicesIndex + 1)) {
    if (/^[^\s#][^:]*:/.test(line)) break;
    const match = line.match(/^  ([A-Za-z0-9_-]+):\s*$/);
    if (match) services.push(match[1]);
  }
  return services;
}

function dockerRuntimeProbe() {
  const result = spawnSync('docker', ['compose', 'version'], {
    cwd: PROJECT_ROOT,
    encoding: 'utf8',
    windowsHide: true,
    timeout: 5000
  });
  if (result.error || result.status !== 0) {
    return { status: 'not_run', reason: 'Docker CLI/daemon unavailable in this environment', command_status: result.status ?? null };
  }
  return { status: 'available', version: redact((result.stdout || '').trim()).slice(0, 200) };
}

async function runComposeLane() {
  const source = fs.readFileSync(path.join(PROJECT_ROOT, 'docker-compose.yml'), 'utf8');
  const services = parseComposeServices(source);
  const expected = ['backend', 'frontend', 'mqtt-broker', 'postgres', 'redis'];
  const checks = [
    check('default service graph', expected.every(name => services.includes(name)) && services.length === expected.length, `services=${services.join(',')}`),
    check('bounded healthchecks', (source.match(/^    healthcheck:/gm) || []).length === expected.length, 'five default services expose healthchecks'),
    check('required secret interpolation', ['POSTGRES_PASSWORD', 'REDIS_PASSWORD', 'GOTP_JWT_KEY'].every(name => source.includes(`\${${name}:?`)), 'required secrets fail closed when missing'),
    check('build contexts exist', ['backend/Dockerfile', 'frontend/Dockerfile', 'mqtt-broker/Dockerfile'].every(file => fs.existsSync(path.join(PROJECT_ROOT, file))), 'backend, frontend, and broker Dockerfiles exist')
  ];
  const docker = dockerRuntimeProbe();
  checks.push(check('Docker Compose runtime', docker.status === 'available', docker.status === 'available' ? docker.version : docker.reason, {
    status: docker.status,
    required: false
  }));
  const staticChecks = checks.filter(item => item.name !== 'Docker Compose runtime');
  assert(staticChecks.every(item => item.ok), 'Compose static dry-run assertions did not all pass');
  return {
    checks,
    docker_runtime: docker.status === 'available' ? 'not-validated' : 'not-run',
    deployment_equivalence: 'not-proven',
    static_dry_run: 'simulated_pass',
    blockers: docker.status === 'available' ? ['Docker runtime was detected but this lane does not start containers.'] : ['Docker CLI/daemon unavailable; container startup was not run.']
  };
}

function createTempDirectory() {
  const parent = path.join(AUTOMATION_ROOT, 'verification');
  fs.mkdirSync(parent, { recursive: true });
  return fs.mkdtempSync(path.join(parent, '.simulated-sbom-'));
}

async function runSbomLane() {
  const temporaryDirectory = createTempDirectory();
  const outputPath = path.join(temporaryDirectory, 'source-sbom.json');
  try {
    const relativeOutput = path.relative(PROJECT_ROOT, outputPath);
    const result = spawnSync(process.execPath, [
      path.join(AUTOMATION_ROOT, 'scripts', 'generate_local_sbom.js'),
      '--output', relativeOutput
    ], { cwd: PROJECT_ROOT, encoding: 'utf8', windowsHide: true, timeout: 30000 });
    assert(result.status === 0, `local SBOM generator failed: ${redact(result.stderr || result.stdout)}`);
    const bom = JSON.parse(fs.readFileSync(outputPath, 'utf8'));
    const properties = new Map((bom.metadata?.properties || []).map(item => [item.name, item.value]));
    const checks = [
      check('CycloneDX source SBOM', bom.bomFormat === 'CycloneDX' && bom.specVersion === '1.6', 'CycloneDX 1.6'),
      check('declared-and-locked component set', properties.get('completeness') === 'declared-and-locked-components' && Array.isArray(bom.components) && bom.components.length > 0, `components=${bom.components.length}`),
      check('source fingerprint', typeof properties.get('source.files.sha256') === 'string' && properties.get('source.files.sha256').length > 20, 'source files are fingerprinted'),
      check('registry enrichment boundary', properties.get('external.registry-enrichment') === 'not-run', 'external registry enrichment remains explicit and not-run')
    ];
    assert(checks.every(item => item.ok), 'source SBOM assertions did not all pass');
    return {
      checks,
      component_count: bom.components.length,
      format: `${bom.bomFormat} ${bom.specVersion}`,
      source_scope: properties.get('scope'),
      registry_enrichment: 'not-run',
      deployment_image_sbom: 'not-proven'
    };
  } finally {
    fs.rmSync(temporaryDirectory, { recursive: true, force: true });
  }
}

function laneResult(name, mode, evidenceClass, startedAt, data, error = null) {
  const checks = data?.checks || [];
  const blockers = data?.blockers || [];
  const safeData = { ...(data || {}) };
  // The token is needed by the in-process business flow, but must never enter
  // a persisted or printed evidence report, even though it is synthetic.
  delete safeData.token;
  delete safeData.password;
  return {
    name,
    status: error ? 'failed' : 'simulated_pass',
    mode,
    evidence_class: evidenceClass,
    started_at: startedAt,
    finished_at: now(),
    checks,
    blockers: error ? [...blockers, redact(error.message)] : blockers,
    ...safeData
  };
}

function externalRequirements(options) {
  const blockers = [];
  if (options.apiMode === 'real') blockers.push('real API mode requires a user-provided API URL, account, and credential; this offline lane will not guess them');
  if (options.mqttMode === 'real') blockers.push('real MQTT mode requires a user-provided broker address and authentication; this offline lane will not guess them');
  if (options.rdiMode === 'real') blockers.push('real RDI mode requires a user-provided device/PID, voucher, firmware, and physical or approved device environment');
  if (options.deploymentMode === 'real') blockers.push('real deployment mode requires a target server, network, secrets, and deployment credentials');
  return blockers;
}

function buildExternalBlockedReport(options, blockers, startedAt = now()) {
  return {
    schema: SCHEMA,
    status: 'external_blocked',
    mode: 'fail-closed',
    evidence_class: 'configuration-gate',
    started_at: startedAt,
    finished_at: now(),
    lanes: {},
    blockers,
    claims: {
      real_api_login: 'not-run',
      real_business_e2e: 'not-run',
      real_mqtt_broker: 'not-run',
      real_physical_rdi: 'not-run',
      target_server_deployment: 'not-run',
      docker_compose_runtime: 'not-run',
      registry_enrichment: 'not-run',
      deployment_equivalence: 'not-proven'
    },
    cleanup: { status: 'not-started' }
  };
}

async function runSimulatedIntegration(options = parseArguments([])) {
  const startedAt = now();
  const externalBlockers = externalRequirements(options);
  if (externalBlockers.length > 0) return buildExternalBlockedReport(options, externalBlockers, startedAt);

  const broker = new InMemoryMqttBroker();
  const backend = new SimulatedBackend(broker);
  const server = createApiServer(backend);
  const address = await listen(server);
  const lanes = {};
  let login = null;
  try {
    const apiStarted = now();
    try {
      const data = await runApiLoginLane(address.url, backend);
      login = data;
      lanes.api_login = laneResult('api_login', 'simulated', 'in-process-http', apiStarted, data);
    } catch (error) {
      lanes.api_login = laneResult('api_login', 'simulated', 'in-process-http', apiStarted, null, error);
    }

    const e2eStarted = now();
    try {
      assert(login && login.token, 'business E2E cannot start because simulated API login did not produce an in-memory token');
      const data = await runBusinessE2eLane(address.url, backend, broker, login.token);
      lanes.business_e2e = laneResult('business_e2e', 'simulated', 'business-flow-in-process', e2eStarted, data);
    } catch (error) {
      lanes.business_e2e = laneResult('business_e2e', 'simulated', 'business-flow-in-process', e2eStarted, null, error);
    }

    const mqttStarted = now();
    try {
      const data = await runMqttLane(backend, broker);
      lanes.mqtt_broker = laneResult('mqtt_broker', 'simulated', 'in-memory-mqtt-router', mqttStarted, data);
    } catch (error) {
      lanes.mqtt_broker = laneResult('mqtt_broker', 'simulated', 'in-memory-mqtt-router', mqttStarted, null, error);
    }

    const rdiStarted = now();
    try {
      const data = await runRdiLane(backend, broker);
      lanes.rdi = laneResult('rdi', 'synthetic', RDI_EVIDENCE_CLASS, rdiStarted, data);
    } catch (error) {
      lanes.rdi = laneResult('rdi', 'synthetic', RDI_EVIDENCE_CLASS, rdiStarted, null, error);
    }

    const composeStarted = now();
    try {
      const data = await runComposeLane();
      lanes.deployment = laneResult('deployment', 'compose-dry-run', 'static-compose-contract', composeStarted, data);
    } catch (error) {
      lanes.deployment = laneResult('deployment', 'compose-dry-run', 'static-compose-contract', composeStarted, null, error);
    }

    const sbomStarted = now();
    try {
      const data = await runSbomLane();
      lanes.sbom = laneResult('sbom', 'local-source-generator', 'source-sbom', sbomStarted, data);
    } catch (error) {
      lanes.sbom = laneResult('sbom', 'local-source-generator', 'source-sbom', sbomStarted, null, error);
    }
  } finally {
    await close(server);
    for (const unsubscribe of backend.unsubscribe) unsubscribe();
    broker.disconnect('simulated-backend');
  }

  const failures = Object.values(lanes).filter(lane => lane.status !== 'simulated_pass');
  const claims = {
    real_api_login: 'not-proven',
    real_business_e2e: 'not-proven',
    real_mqtt_broker: 'not-proven',
    real_physical_rdi: 'not-proven',
    target_server_deployment: 'not-run',
    docker_compose_runtime: lanes.deployment?.docker_runtime || 'not-run',
    registry_enrichment: lanes.sbom?.registry_enrichment || 'not-run',
    deployment_equivalence: lanes.deployment?.deployment_equivalence || 'not-proven'
  };
  const report = {
    schema: SCHEMA,
    status: failures.length === 0 ? 'simulated_pass' : 'failed',
    mode: 'offline-in-process',
    evidence_class: 'simulation-not-production-evidence',
    started_at: startedAt,
    finished_at: now(),
    lanes,
    claims,
    blockers: [
      'Simulation does not authenticate to a real API or prove a real account.',
      'In-memory MQTT is not a real broker or physical firmware session.',
      'Synthetic RDI replay is not physical device/RDI acceptance.',
      'Compose runtime and target-server deployment were not run here.',
      'Source SBOM registry enrichment and image/deployment SBOM equivalence were not run.'
    ],
    cleanup: {
      status: 'passed',
      api_server_closed: true,
      synthetic_tokens_not_persisted: true,
      business_device_cleanup: lanes.business_e2e?.cleanup || null
    }
  };
  assertNoRealClaim(report, 'simulated integration report');
  return report;
}

function validateReportDir(value) {
  const resolved = path.resolve(PROJECT_ROOT, value);
  const relative = path.relative(PROJECT_ROOT, resolved);
  if (!relative || relative === '..' || relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative)) {
    throw new LaneError('--report-dir must remain inside the repository', 'invalid-arguments');
  }
  return resolved;
}

function writeReport(report, reportDir) {
  const directory = validateReportDir(reportDir);
  fs.mkdirSync(directory, { recursive: true });
  fs.writeFileSync(path.join(directory, 'summary.json'), `${JSON.stringify(report, null, 2)}\n`, { encoding: 'utf8', mode: 0o600 });
  for (const [name, lane] of Object.entries(report.lanes || {})) {
    fs.writeFileSync(path.join(directory, `${name}.json`), `${JSON.stringify(lane, null, 2)}\n`, { encoding: 'utf8', mode: 0o600 });
  }
}

function humanSummary(report) {
  const lines = [`simulated integration lane: ${report.status}`];
  for (const lane of Object.values(report.lanes || {})) lines.push(`- ${lane.name}: ${lane.status} (${lane.mode})`);
  lines.push('- real API/account evidence: not-proven');
  lines.push('- real MQTT broker evidence: not-proven');
  lines.push('- physical RDI/device evidence: not-proven');
  lines.push(`- Docker Compose runtime: ${report.claims?.docker_compose_runtime || 'not-run'}`);
  lines.push(`- source SBOM registry enrichment: ${report.claims?.registry_enrichment || 'not-run'}`);
  lines.push(`- deployment equivalence: ${report.claims?.deployment_equivalence || 'not-proven'}`);
  return lines.join('\n') + '\n';
}

async function main(argv = process.argv.slice(2), stdout = process.stdout, stderr = process.stderr) {
  let options;
  try {
    options = parseArguments(argv);
  } catch (error) {
    stderr.write(`simulated integration lane: ${error.message}\n${helpText()}`);
    return 2;
  }
  if (options.help) {
    stdout.write(helpText());
    return 0;
  }

  let report;
  try {
    report = await runSimulatedIntegration(options);
    if (options.reportDir) writeReport(report, options.reportDir);
  } catch (error) {
    report = {
      schema: SCHEMA,
      status: 'failed',
      mode: 'offline-in-process',
      evidence_class: 'simulation-not-production-evidence',
      started_at: now(),
      finished_at: now(),
      lanes: {},
      blockers: [redact(error.stack || error.message || error)],
      claims: { real_api_login: 'not-proven', real_mqtt_broker: 'not-proven', real_physical_rdi: 'not-proven', deployment_equivalence: 'not-proven' },
      cleanup: { status: 'unknown' }
    };
  }
  if (options.json) stdout.write(`${JSON.stringify(report)}\n`);
  else stdout.write(humanSummary(report));
  if (report.status === 'external_blocked') return 2;
  return report.status === 'simulated_pass' ? 0 : 1;
}

if (require.main === module) {
  main().then(code => { process.exitCode = code; }).catch(error => {
    process.stderr.write(`simulated integration lane failed unexpectedly: ${redact(error.stack || error)}\n`);
    process.exitCode = 1;
  });
}

module.exports = {
  PROJECT_ROOT,
  SCHEMA,
  SYNTHETIC_EMAIL,
  SYNTHETIC_PASSWORD,
  SYNTHETIC_PROVENANCE,
  RDI_EVIDENCE_CLASS,
  TOPICS,
  parseArguments,
  topicMatches,
  InMemoryMqttBroker,
  SimulatedBackend,
  SimulatedRdiDevice,
  parseComposeServices,
  dockerRuntimeProbe,
  makeEvidence,
  runSimulatedIntegration,
  buildExternalBlockedReport,
  main
};
