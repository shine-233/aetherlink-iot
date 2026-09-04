/**
 * 文件用途：规则链 CRUD API 的真实业务状态证据。
 * 核心逻辑：创建规则链后通过 list/get 精确回读，更新启用状态与名称，
 *   验证非法 DAG 被拒绝且不落库，最后删除并确认资源不可再读取。
 * 关键注意事项：graph 必须作为 JSON 对象往返，不能接受 base64、空图或仅状态码成功；
 *   after 钩子始终执行幂等清理，运行证据仍需来自健康后端上的新鲜归档结果。
 */

const crypto = require('crypto');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  createSimulationDevice,
  isMqttBrokerAvailable
} = require('../lib/seed_data');
const {
  publishMqttTelemetry,
  startMqttCommandDevice
} = require('../lib/mqtt_device_fixture');
const { skipIfBlocked } = require('../lib/integration_blocked');
const {
  expectBusinessError,
  expectSuccess
} = require('../lib/response_assertions');

const RULE_CHAINS_PATH = '/rule-chains';
const NOT_FOUND_CODE = 100404;
const PARAM_ERROR_CODE = 100002;
const NOT_FOUND_MESSAGE = 'rule chain not found';

function uniqueName(label) {
  return `codex-rule-chain-${label}-${Date.now()}-${crypto.randomBytes(5).toString('hex')}`;
}

function ruleChainPath(id) {
  return RULE_CHAINS_PATH + '/' + id;
}

function assertSuccessEnvelope(resp) {
  expectSuccess(resp);
  expect(resp).to.have.keys('code', 'message', 'data');
  expect(resp.message).to.be.a('string').and.not.equal('');
}

function assertGraph(actual, expected) {
  expect(actual, 'rule chain graph must be returned as JSON, not base64 text').to.be.an('object');
  expect(actual).to.deep.equal(expected);
}

function assertRuleChainRow(row, expected) {
  expect(row).to.be.an('object');
  expect(row).to.include.keys(
    'id',
    'tenant_id',
    'name',
    'description',
    'enabled',
    'graph',
    'created_at',
    'updated_at'
  );
  expect(row.id).to.equal(expected.id);
  expect(row.tenant_id).to.be.a('string').and.not.equal('');
  if (expected.tenantId) {
    expect(row.tenant_id).to.equal(expected.tenantId);
  }
  expect(row.name).to.equal(expected.name);
  expect(row.description).to.equal(expected.description);
  expect(row.enabled).to.equal(expected.enabled);
  assertGraph(row.graph, expected.graph);
  expect(row.created_at).to.be.a('string').and.not.equal('');
  expect(row.updated_at).to.be.a('string').and.not.equal('');
}

function findExactName(rows, name) {
  return rows.filter(row => row && row.name === name);
}

async function readCurrentTelemetry(deviceId) {
  const response = await apiClient.get(
    '/telemetry/datas/current/' + deviceId,
    {},
    'tenant_admin'
  );
  assertSuccessEnvelope(response);
  expect(response.data).to.be.an('array');
  return response.data;
}

async function waitForCurrentTelemetry(deviceId, key, expected, timeoutMs = 30000) {
  const deadline = Date.now() + timeoutMs;
  let lastRows = [];
  while (Date.now() < deadline) {
    lastRows = await readCurrentTelemetry(deviceId);
    const row = lastRows.find(item => item && item.key === key);
    if (row && Number(row.value) === Number(expected)) {
      return row;
    }
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  throw new Error(
    `current telemetry ${key}=${expected} was not readable for device ${deviceId}: ${JSON.stringify(lastRows)}`
  );
}

async function readCommandLogs(deviceId, identify) {
  const response = await apiClient.get(
    '/command/datas/set/logs',
    {
      device_id: deviceId,
      identify,
      page: 1,
      page_size: 50
    },
    'tenant_admin'
  );
  assertSuccessEnvelope(response);
  expect(response.data).to.be.an('object');
  expect(response.data.list).to.be.an('array');
  expect(response.data.total).to.be.a('number');
  return response.data.list;
}

async function waitForCommandLog(deviceId, identify, messageId, timeoutMs = 45000) {
  const deadline = Date.now() + timeoutMs;
  let lastLogs = [];
  while (Date.now() < deadline) {
    lastLogs = await readCommandLogs(deviceId, identify);
    const row = lastLogs.find(item => (
      item &&
      item.message_id === messageId &&
      item.identify === identify
    ));
    if (row) return row;
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  throw new Error(
    `command log ${identify}/${messageId} was not readable for device ${deviceId}: ${JSON.stringify(lastLogs)}`
  );
}

function parseJSONField(row, field) {
  const raw = row && row[field];
  if (raw && typeof raw === 'object') return raw;
  expect(raw, `${field} must be persisted as JSON`).to.be.a('string').and.not.equal('');
  let parsed;
  try {
    parsed = JSON.parse(raw);
  } catch (error) {
    throw new Error(`${field} is not valid JSON: ${error.message}; raw=${raw}`);
  }
  expect(parsed).to.be.an('object');
  return parsed;
}

async function assertNoCommandEvidence(deviceId, identify, durationMs = 4000) {
  const deadline = Date.now() + durationMs;
  let samples = 0;
  while (Date.now() < deadline) {
    const logs = await readCommandLogs(deviceId, identify);
    expect(logs, `below-threshold telemetry must not create ${identify} command logs`).to.deep.equal([]);
    samples += 1;
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  expect(samples, 'negative rule-chain control must collect command-log samples').to.be.greaterThan(0);
}

describe('Rule chain API business flow [29_rule_chain_business]', function () {
  this.timeout(180000);

  const initialGraph = {
    nodes: [
      { id: 'telemetry-trigger', type: 'trigger.telemetry', name: 'Telemetry trigger', config: {} },
      {
        id: 'threshold-filter',
        type: 'filter.threshold',
        name: 'Temperature threshold',
        config: { key: 'temperature', op: '>', value: 30 }
      },
      {
        id: 'command-action',
        type: 'action.command',
        name: 'Cooling command',
        config: { identify: 'set-cooling', params: { enabled: true } }
      }
    ],
    edges: [
      { from: 'telemetry-trigger', to: 'threshold-filter' },
      { from: 'threshold-filter', to: 'command-action' }
    ]
  };
  const updatedGraph = {
    nodes: [
      { id: 'telemetry-trigger', type: 'trigger.telemetry', name: 'Telemetry trigger', config: {} },
      {
        id: 'threshold-filter',
        type: 'filter.threshold',
        name: 'Temperature threshold',
        config: { key: 'temperature', op: '>=', value: 35 }
      },
      {
        id: 'command-action',
        type: 'action.command',
        name: 'Cooling command',
        config: { identify: 'set-cooling', params: { enabled: true, level: 2 } }
      }
    ],
    edges: [
      { from: 'telemetry-trigger', to: 'threshold-filter' },
      { from: 'threshold-filter', to: 'command-action' }
    ]
  };
  const initialName = uniqueName('create');
  const updatedName = initialName + '-enabled';
  const invalidName = uniqueName('invalid');
  const initialDescription = 'API business flow fixture';
  const updatedDescription = 'API business flow fixture updated';
  let createdId = '';
  let tenantId = '';

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      skipIfBlocked(this, {
        reason: 'backend service is unavailable; rule-chain API/runtime evidence requires a healthy API service',
        category: 'runtime-external',
        seedable: false
      });
    }
    await apiClient.login('tenant_admin');
  });

  after(async function () {
    let cleanupError;
    try {
      if (createdId) {
        const cleanupResp = await apiClient.delete(ruleChainPath(createdId), {}, 'tenant_admin');
        expect(cleanupResp).to.be.an('object');
        expect(
          [200, NOT_FOUND_CODE],
          'cleanup must either delete the fixture or confirm that it was already deleted'
        ).to.include(cleanupResp.code);
        if (cleanupResp.code === NOT_FOUND_CODE) {
          expect(cleanupResp.message).to.equal(NOT_FOUND_MESSAGE);
        }

        const cleanupReadback = await apiClient.get(ruleChainPath(createdId), {}, 'tenant_admin');
        expectBusinessError(cleanupReadback, NOT_FOUND_CODE, NOT_FOUND_MESSAGE);
        expect(cleanupReadback).to.not.have.property('data');
      }
    } catch (error) {
      cleanupError = error;
    } finally {
      apiClient.clearAllTokens();
    }
    if (cleanupError) throw cleanupError;
  });

  it('creates a tenant-owned disabled rule chain with the exact graph', async function () {
    const resp = await apiClient.post(RULE_CHAINS_PATH, {
      name: initialName,
      description: initialDescription,
      enabled: false,
      graph: initialGraph
    }, 'tenant_admin');

    assertSuccessEnvelope(resp);
    expect(resp.data.id).to.be.a('string').and.not.equal('');
    createdId = resp.data.id;
    tenantId = resp.data.tenant_id;
    assertRuleChainRow(resp.data, {
      id: createdId,
      tenantId,
      name: initialName,
      description: initialDescription,
      enabled: false,
      graph: initialGraph
    });
  });

  it('lists and gets the created chain with exact persisted state', async function () {
    expect(createdId, 'create case must provide a rule chain id').to.be.a('string').and.not.equal('');

    const listResp = await apiClient.get(
      RULE_CHAINS_PATH + '/list',
      { keyword: initialName, page: 1, page_size: 20 },
      'tenant_admin'
    );
    assertSuccessEnvelope(listResp);
    expect(listResp.data).to.have.keys('total', 'list');
    expect(listResp.data.total).to.equal(1);
    expect(listResp.data.list).to.be.an('array').with.lengthOf(1);
    assertRuleChainRow(listResp.data.list[0], {
      id: createdId,
      tenantId,
      name: initialName,
      description: initialDescription,
      enabled: false,
      graph: initialGraph
    });

    const getResp = await apiClient.get(ruleChainPath(createdId), {}, 'tenant_admin');
    assertSuccessEnvelope(getResp);
    assertRuleChainRow(getResp.data, {
      id: createdId,
      tenantId,
      name: initialName,
      description: initialDescription,
      enabled: false,
      graph: initialGraph
    });
  });

  it('updates name and enabled state and persists both changes', async function () {
    expect(createdId, 'create case must provide a rule chain id').to.be.a('string').and.not.equal('');

    const updateResp = await apiClient.put(RULE_CHAINS_PATH, {
      id: createdId,
      name: updatedName,
      description: updatedDescription,
      enabled: true,
      graph: updatedGraph
    }, 'tenant_admin');
    assertSuccessEnvelope(updateResp);
    assertRuleChainRow(updateResp.data, {
      id: createdId,
      tenantId,
      name: updatedName,
      description: updatedDescription,
      enabled: true,
      graph: updatedGraph
    });

    const readback = await apiClient.get(ruleChainPath(createdId), {}, 'tenant_admin');
    assertSuccessEnvelope(readback);
    assertRuleChainRow(readback.data, {
      id: createdId,
      tenantId,
      name: updatedName,
      description: updatedDescription,
      enabled: true,
      graph: updatedGraph
    });

    const oldNameList = await apiClient.get(
      RULE_CHAINS_PATH + '/list',
      { keyword: initialName, page: 1, page_size: 20 },
      'tenant_admin'
    );
    assertSuccessEnvelope(oldNameList);
    expect(findExactName(oldNameList.data.list, initialName)).to.have.lengthOf(0);

    const updatedNameList = await apiClient.get(
      RULE_CHAINS_PATH + '/list',
      { keyword: updatedName, page: 1, page_size: 20 },
      'tenant_admin'
    );
    assertSuccessEnvelope(updatedNameList);
    expect(findExactName(updatedNameList.data.list, updatedName)).to.have.lengthOf(1);
    assertRuleChainRow(findExactName(updatedNameList.data.list, updatedName)[0], {
      id: createdId,
      tenantId,
      name: updatedName,
      description: updatedDescription,
      enabled: true,
      graph: updatedGraph
    });
  });

  it('rejects a disconnected action root with an exact parameter error and no row', async function () {
    const invalidGraph = {
      nodes: [
        { id: 'valid-trigger', type: 'trigger.telemetry', config: {} },
        { id: 'orphan-action', type: 'action.command', config: { identify: 'must-not-run' } }
      ],
      edges: []
    };
    const resp = await apiClient.post(RULE_CHAINS_PATH, {
      name: invalidName,
      description: 'must not persist',
      graph: invalidGraph
    }, 'tenant_admin');

    expectBusinessError(
      resp,
      PARAM_ERROR_CODE,
      'invalid graph: non-trigger node "orphan-action" cannot be a root'
    );
    expect(resp).to.have.keys('code', 'message');

    const listResp = await apiClient.get(
      RULE_CHAINS_PATH + '/list',
      { keyword: invalidName, page: 1, page_size: 20 },
      'tenant_admin'
    );
    assertSuccessEnvelope(listResp);
    expect(findExactName(listResp.data.list, invalidName)).to.have.lengthOf(0);
  });

  it('deletes the chain and returns exact not-found state afterwards', async function () {
    expect(createdId, 'create case must provide a rule chain id').to.be.a('string').and.not.equal('');

    const deleteResp = await apiClient.delete(ruleChainPath(createdId), {}, 'tenant_admin');
    assertSuccessEnvelope(deleteResp);
    expect(deleteResp.data).to.deep.equal({});

    const getResp = await apiClient.get(ruleChainPath(createdId), {}, 'tenant_admin');
    expectBusinessError(getResp, NOT_FOUND_CODE, NOT_FOUND_MESSAGE);
    expect(getResp).to.have.keys('code', 'message');

    const listResp = await apiClient.get(
      RULE_CHAINS_PATH + '/list',
      { keyword: updatedName, page: 1, page_size: 20 },
      'tenant_admin'
    );
    assertSuccessEnvelope(listResp);
    expect(findExactName(listResp.data.list, updatedName)).to.have.lengthOf(0);
  });

  it('executes telemetry rules only above threshold and records an acknowledged automatic MQTT command', async function () {
    if (!(await isMqttBrokerAvailable())) {
      skipIfBlocked(this, {
        reason: 'MQTT broker is unavailable; rule-chain telemetry side-effect evidence requires a live authenticated broker',
        category: 'runtime-external',
        seedable: false
      });
    }

    const deviceSeed = await createSimulationDevice('tenant_admin');
    const runtimeName = uniqueName('runtime');
    const identify = `rule-chain-e2e-${Date.now()}-${crypto.randomBytes(3).toString('hex')}`;
    const graph = {
      nodes: [
        { id: 'trigger', type: 'trigger.telemetry', config: {} },
        {
          id: 'filter',
          type: 'filter.threshold',
          config: { key: 'temperature', op: '>', value: 30 }
        },
        {
          id: 'command',
          type: 'action.command',
          config: { identify, params: { level: 2 } }
        }
      ],
      edges: [
        { from: 'trigger', to: 'filter' },
        { from: 'filter', to: 'command' }
      ]
    };
    let ruleChainId = '';
    let mqttDevice;
    let cleanupError;

    try {
      const createResponse = await apiClient.post(RULE_CHAINS_PATH, {
        name: runtimeName,
        description: 'real MQTT telemetry rule-chain side-effect fixture',
        enabled: true,
        graph
      }, 'tenant_admin');
      assertSuccessEnvelope(createResponse);
      expect(createResponse.data).to.be.an('object');
      expect(createResponse.data.id).to.be.a('string').and.not.equal('');
      ruleChainId = createResponse.data.id;
      assertRuleChainRow(createResponse.data, {
        id: ruleChainId,
        tenantId: createResponse.data.tenant_id,
        name: runtimeName,
        description: 'real MQTT telemetry rule-chain side-effect fixture',
        enabled: true,
        graph
      });

      const readback = await apiClient.get(ruleChainPath(ruleChainId), {}, 'tenant_admin');
      assertSuccessEnvelope(readback);
      assertRuleChainRow(readback.data, {
        id: ruleChainId,
        tenantId: createResponse.data.tenant_id,
        name: runtimeName,
        description: 'real MQTT telemetry rule-chain side-effect fixture',
        enabled: true,
        graph
      });

      // Rule-chain writes invalidate the process cache, but the real uplink
      // path is asynchronous. Let the write/cache boundary settle before the
      // first authenticated telemetry publish.
      await new Promise(resolve => setTimeout(resolve, 1500));
      mqttDevice = await startMqttCommandDevice(deviceSeed, 'tenant_admin');

      await publishMqttTelemetry(deviceSeed, { temperature: 20 }, 'tenant_admin');
      await waitForCurrentTelemetry(deviceSeed.id, 'temperature', 20);
      await assertNoCommandEvidence(deviceSeed.id, identify, 4000);
      expect(
        mqttDevice.readReceipts(),
        'below-threshold telemetry must not reach the command device'
      ).to.deep.equal([]);

      await publishMqttTelemetry(deviceSeed, { temperature: 40 }, 'tenant_admin');
      await waitForCurrentTelemetry(deviceSeed.id, 'temperature', 40);
      const receipts = await mqttDevice.waitForReceipts(1, 45000);
      const receipt = receipts.find(item => item && item.method === identify);
      expect(receipt, 'above-threshold telemetry must create the configured command').to.be.an('object');
      expect(receipt.message_id).to.be.a('string').and.not.equal('');
      expect(receipt.topic).to.match(/^devices\/command\/[^/]+\/[^/]+$/);
      expect(receipt.params).to.deep.equal({ level: 2 });
      expect(receipt.ack_topic).to.equal(`devices/command/response/${receipt.message_id}`);
      expect(receipt.ack_payload).to.include({ result: 0, method: identify });

      const commandLog = await waitForCommandLog(
        deviceSeed.id,
        identify,
        receipt.message_id,
        45000
      );
      expect(commandLog).to.include({
        device_id: deviceSeed.id,
        message_id: receipt.message_id,
        identify,
        status: '3',
        operation_type: '2'
      });
      expect(parseJSONField(commandLog, 'data')).to.deep.include({
        method: identify,
        params: { level: 2 }
      });
      expect(parseJSONField(commandLog, 'rsp_data')).to.deep.include({
        result: 0,
        method: identify
      });
    } finally {
      try {
        if (mqttDevice) await mqttDevice.cleanup();
        if (ruleChainId) {
          const deleteResponse = await apiClient.delete(
            ruleChainPath(ruleChainId),
            {},
            'tenant_admin'
          );
          expect([200, NOT_FOUND_CODE]).to.include(deleteResponse.code);
        }
        await deviceSeed.cleanup();
      } catch (error) {
        cleanupError = error;
      }
    }

    if (cleanupError) throw cleanupError;
  });
});
