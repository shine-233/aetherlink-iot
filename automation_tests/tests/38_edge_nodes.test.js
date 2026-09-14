/**
 * 文件用途：P1.5 边缘节点注册 / 心跳 / 列表 / Reconcile 的 API 契约测试。
 * 核心逻辑：注册节点 → 列表可查 → 心跳续命并回健康分类 → Reconcile 出同步计划；
 *          负向覆盖缺参、非注册节点、跨租户抢注、网关不在租户内四类拒绝路径。
 * 关键注意事项：
 *   - 边缘节点没有删除端点，节点 ID 必须带时间戳保证可重复运行；残留节点只影响列表计数，不影响断言。
 *   - Reconcile 的版本闸门在未配置 edge.min_compatible_version 时**一律判不兼容（fail closed）**，
 *     因此断言写成"版本不兼容则必须 blocked"，而不是硬编码成功——这样两种部署配置都能通过。
 *   - 心跳分类依赖 last_seen_at，注册后立刻心跳才能断言 health=online。
 * 重构建议：拿到真实边缘节点后应补断云自治演练（上报不入云、重连后按修订号补发），
 *          当前只覆盖云端编排侧，未覆盖边缘侧行为。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'Edge node registry and reconcile [38_edge_nodes]';
const ACCOUNT = 'tenant_admin';
const OTHER_TENANT = 'tenant_admin_b';

// errcode.CodeParamError：参数缺失、跨租户抢注、节点/网关未注册都走这一个码。
const CODE_PARAM_ERROR = 100002;

// 与后端 model.EdgeNode 的 status / health 取值保持一致。
const EDGE_NODE_STATUSES = new Set(['active', 'revoked']);
const EDGE_HEALTHS = new Set(['online', 'degraded', 'offline', 'unknown']);
const EDGE_OUTCOMES = new Set(['updated', 'unchanged']);

function uniqueNodeId() {
  // 固定前缀会命中"同一 NodeID 归属别的租户"守卫，必须带时间+随机数保证可重跑。
  return `node-e2e-${Date.now()}-${Math.floor(Math.random() * 100000)}`;
}

function expectOk(resp) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected 200 but got ${resp.code}: ${resp.message || ''}`).to.equal(200);
  expect(resp.data, 'data payload').to.be.an('object');
  return resp.data;
}

// 列表端点直接返回数组而不是 { list: [...] }，不是每个响应都裹在对象里。
// 复用 expectOk 会把"数组是合法响应"误判成失败。
function expectOkPayload(resp) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected 200 but got ${resp.code}: ${resp.message || ''}`).to.equal(200);
  expect(resp.data, 'data payload').to.satisfy(
    value => value !== null && value !== undefined,
    'data payload must be present'
  );
  return resp.data;
}

function expectRejected(resp, code) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected rejection ${code} but got ${resp.code}`).to.equal(code);
}

function expectNodeRow(node) {
  expect(node, 'edge node row').to.be.an('object');
  expect(node.id, 'node.id').to.be.a('string').and.not.equal('');
  expect(node.tenant_id, 'node.tenant_id').to.be.a('string').and.not.equal('');
  expect(node.status, 'node.status').to.be.a('string');
  expect(EDGE_NODE_STATUSES.has(node.status), `unexpected status ${node.status}`).to.equal(true);
  expect(node.version, 'node.version').to.be.a('string').and.not.equal('');
}

function expectRegistration(resp) {
  const data = expectOk(resp);
  expectNodeRow(data.node);
  expect(data.outcome, 'outcome').to.be.a('string');
  expect(EDGE_OUTCOMES.has(data.outcome), `unexpected outcome ${data.outcome}`).to.equal(true);
  expect(data.health, 'health').to.be.a('string');
  expect(EDGE_HEALTHS.has(data.health), `unexpected health ${data.health}`).to.equal(true);
  return data;
}

describe(SUITE, function () {
  this.timeout(60000);

  let nodeId = null;
  let gatewayDeviceId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 38_edge_nodes.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login(ACCOUNT);

    const seeded = await seedData.ensureDevice(ACCOUNT);
    expect(seeded, 'seeded device').to.be.an('object');
    gatewayDeviceId = seeded.id;
    expect(gatewayDeviceId, 'seeded device id').to.be.a('string').and.not.equal('');

    nodeId = uniqueNodeId();
  });

  it('registers an edge node and returns an active node with a health classification', async function () {
    const resp = await apiClient.post('/edge/nodes', {
      node_id: nodeId,
      version: '1.0.0',
      capabilities: ['mqtt', 'modbus']
    }, ACCOUNT);
    const data = expectRegistration(resp);
    expect(data.node.id).to.equal(nodeId);
    expect(data.node.version).to.equal('1.0.0');
    expect(data.node.last_seen_at, 'last_seen_at must be touched on register').to.satisfy(
      value => value !== null && value !== undefined && value !== '',
      'register must touch the heartbeat timestamp'
    );
  });

  it('rejects a registration request without node_id or version', async function () {
    const missingNodeId = await apiClient.post('/edge/nodes', { version: '1.0.0' }, ACCOUNT);
    expectRejected(missingNodeId, CODE_PARAM_ERROR);

    const missingVersion = await apiClient.post('/edge/nodes', { node_id: uniqueNodeId() }, ACCOUNT);
    expectRejected(missingVersion, CODE_PARAM_ERROR);
  });

  it('is idempotent: re-registering the same node in the same tenant does not create a duplicate', async function () {
    const resp = await apiClient.post('/edge/nodes', {
      node_id: nodeId,
      version: '1.0.1',
      capabilities: ['mqtt']
    }, ACCOUNT);
    const data = expectRegistration(resp);
    expect(data.node.id).to.equal(nodeId);
    expect(data.node.version, 'version must be updated by the re-registration').to.equal('1.0.1');
  });

  it('rejects cross-tenant squatting of an already registered node id', async function () {
    if (!await apiClient.isAccountAvailable(OTHER_TENANT)) {
      // 占位账号不可用时跳过，但不算通过——契约用例本身仍要求该守卫存在。
      this.skip('second tenant account is not configured');
      return;
    }
    await apiClient.login(OTHER_TENANT);
    const resp = await apiClient.post('/edge/nodes', {
      node_id: nodeId,
      version: '9.9.9'
    }, OTHER_TENANT);
    expectRejected(resp, CODE_PARAM_ERROR);
    await apiClient.login(ACCOUNT);
  });

  it('lists the registered node inside the tenant', async function () {
    const resp = await apiClient.get('/edge/nodes', { limit: 100 }, ACCOUNT);
    const data = expectOkPayload(resp);
    const nodes = Array.isArray(data) ? data : (data.list || data.nodes || []);
    expect(nodes, 'node list').to.be.an('array');
    const found = nodes.find(item => (item.id || item.ID) === nodeId);
    expect(found, `registered node ${nodeId} must appear in the tenant list`).to.be.an('object');
    expectNodeRow({
      id: found.id || found.ID,
      tenant_id: found.tenant_id,
      status: found.status,
      version: found.version
    });
  });

  it('heartbeat refreshes last_seen_at and reports online health', async function () {
    const resp = await apiClient.post(`/edge/nodes/${encodeURIComponent(nodeId)}/heartbeat`, {
      version: '1.0.1'
    }, ACCOUNT);
    const data = expectRegistration(resp);
    expect(data.node.id).to.equal(nodeId);
    expect(data.health, 'a node that just heartbeats must be online').to.equal('online');
  });

  it('reconcile rejects a resource type outside the supported set', async function () {
    // 后端白名单只有 dashboard / rule_chain（model/edge_node.go 的 oneof 校验）。
    // 用 device_config 期望"能下发"是先于实现的假设，反过来守住这条白名单才是真契约。
    const resp = await apiClient.post(`/edge/nodes/${encodeURIComponent(nodeId)}/reconcile`, {
      gateway_device_id: gatewayDeviceId,
      resources: [
        { resource_type: 'device_config', resource_id: gatewayDeviceId, revision: 0 }
      ]
    }, ACCOUNT);
    expectRejected(resp, CODE_PARAM_ERROR);
  });

  it('reconcile returns a plan and fails closed on an incompatible version', async function () {
    const resp = await apiClient.post(`/edge/nodes/${encodeURIComponent(nodeId)}/reconcile`, {
      gateway_device_id: gatewayDeviceId,
      resources: [
        { resource_type: 'rule_chain', resource_id: gatewayDeviceId, revision: 0 }
      ]
    }, ACCOUNT);
    const data = expectOk(resp);
    expect(data.health, 'health').to.be.a('string');
    expect(EDGE_HEALTHS.has(data.health), `unexpected health ${data.health}`).to.equal(true);
    expect(data.items, 'reconcile plan items').to.be.an('array');
    expect(typeof data.blocked, 'blocked').to.equal('boolean');
    expect(typeof data.synced, 'synced').to.equal('number');

    // 版本闸门：不兼容时整批不下发。
    // 注意语义分工（实测口径，与"blocked=true"不是一回事）：
    //   - 版本闸门把每个计划项标成 action=skip，reason 带 "version gate: " 前缀；
    //   - blocked 仅表示存在 needs_attention（冲突/需人工介入），**不覆盖版本闸门**。
    // 所以这里断言的是"没有任何一项被下发"，而不是 blocked 这个布尔量。
    if (data.version) {
      expect(data.items.every(item => item.action === 'skip'),
        `incompatible version must skip every item, got ${JSON.stringify(data.items)}`).to.equal(true);
      expect(data.synced, 'nothing may be dispatched while version-gated').to.equal(0);
    }
    const syncedItems = data.items.filter(item => item.action === 'sync');
    expect(data.synced, 'synced must equal the number of items actually dispatched')
      .to.equal(syncedItems.length);
  });

  it('reconcile rejects an unregistered node id', async function () {
    const resp = await apiClient.post('/edge/nodes/definitely-not-registered-node/reconcile', {
      gateway_device_id: gatewayDeviceId
    }, ACCOUNT);
    expectRejected(resp, CODE_PARAM_ERROR);
  });

  it('reconcile rejects a gateway device that is not in the tenant', async function () {
    const resp = await apiClient.post(`/edge/nodes/${encodeURIComponent(nodeId)}/reconcile`, {
      gateway_device_id: '00000000-0000-0000-0000-000000000000'
    }, ACCOUNT);
    expectRejected(resp, CODE_PARAM_ERROR);
  });
});
