/**
 * 文件用途：边缘规则链快照下发 API 既有契约守护（TB-21 scoped v1）活栈契约测试。
 *
 * 背景：TB-21 v1 在边缘侧新增本地规则执行器（backend/internal/edgerules），按修订号消费
 * edge_sync 下发的规则链快照。云端 API 契约必须保持不变——本用例锁定：
 *   1. POST /api/v1/edge/sync（rule_chain）：返回任务含 payload 快照信封，字段
 *      type=rule_chain / resource_id / name / content(=规则链 graph) / revision(>=1 且新建资源首版=1)
 *      / version=1 / generated_at；
 *   2. 快照幂等与重试不重拍：GET /edge/sync/:id 与 POST /edge/sync/:id/retry 返回的
 *      payload 与创建时逐字节一致（边缘按 revision 去重的地基）；
 *   3. 负例：未知网关、非法 resource_type、跨租户 resource_id 均被拒（fail-closed）；
 *   4. 严格多租户隔离：租户 B 无法查看/重试租户 A 的同步任务，列表互不可见。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Edge Sync Snapshot Contract [86_edge_sync_snapshot_contract]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

// 与后端单测一致的合法最小图：遥测触发 → 阈值 → 告警（云端画布/边缘执行器共同支持的子集）。
const THRESHOLD_ALARM_GRAPH = JSON.stringify({
  nodes: [
    { id: 't', type: 'trigger.telemetry' },
    { id: 'f', type: 'filter.threshold', config: { key: 'temperature', op: '>', value: 80 } },
    { id: 'a', type: 'action.alarm', config: { name: '高温告警', severity: 'H' } }
  ],
  edges: [
    { from: 't', to: 'f' },
    { from: 'f', to: 'a' }
  ]
});

function pickId(record) {
  return record && (record.id || record.ID) ? record.id || record.ID : null;
}

function parsePayload(task) {
  expect(task, JSON.stringify(task)).to.be.an('object');
  expect(task.payload, '任务必须携带 payload 快照').to.be.a('string').and.not.equal('');
  return JSON.parse(task.payload);
}

describe(SUITE, function () {
  this.timeout(120000);

  let chainId = null;
  let configId = null;
  let deviceId = null;
  let taskId = null;
  let taskPayload = null;
  let foreignChainId = null; // 租户 B 的链，用于跨租户 resource_id 负例

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 86_edge_sync_snapshot_contract.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);

    // 租户 A：规则链 + 档案 + 网关设备（贯穿主链路）
    const chainRes = await apiClient.post(
      '/rule-chains',
      { name: 'edge_sync_chain_86_' + Date.now(), enabled: true, graph: THRESHOLD_ALARM_GRAPH },
      TENANT_A
    );
    expect(chainRes.code, JSON.stringify(chainRes)).to.equal(200);
    chainId = pickId(chainRes.data);

    const configRes = await apiClient.post(
      '/device_config',
      {
        name: 'edge_sync_cfg_86_' + Date.now(),
        device_type: '1',
        protocol_type: 'MQTT',
        voucher_type: 'ACCESSTOKEN',
        device_conn_type: 'A'
      },
      TENANT_A
    );
    expect(configRes.code, JSON.stringify(configRes)).to.equal(200);
    configId = pickId(configRes.data);

    const deviceRes = await apiClient.post(
      '/device',
      {
        name: 'edge_sync_gw_86_' + Date.now(),
        device_number: 'edge_gw_86_' + Date.now(),
        device_config_id: configId,
        voucher: JSON.stringify({ username: 'edge_gw_86_' + Date.now(), password: 'edge_gw_86_pwd' })
      },
      TENANT_A
    );
    expect(deviceRes.code, JSON.stringify(deviceRes)).to.equal(200);
    deviceId = pickId(deviceRes.data);

    // 租户 B：一条链，供跨租户 resource_id 负例
    const foreignRes = await apiClient.post(
      '/rule-chains',
      { name: 'edge_sync_foreign_86_' + Date.now(), enabled: true, graph: THRESHOLD_ALARM_GRAPH },
      TENANT_B
    );
    expect(foreignRes.code, JSON.stringify(foreignRes)).to.equal(200);
    foreignChainId = pickId(foreignRes.data);

    expect(chainId && configId && deviceId && foreignChainId, '前置数据必须创建成功').to.be.a('string');
  });

  after(async function () {
    // 顺序：设备 → 档案 → 链（无档案绑定链的删除守卫压力，仍按依赖序清理）
    try {
      if (deviceId) {
        await apiClient.delete('/device/' + deviceId, {}, TENANT_A);
      }
      if (configId) {
        await apiClient.delete('/device_config/' + configId, {}, TENANT_A);
      }
      if (chainId) {
        await apiClient.delete('/rule-chains/' + chainId, {}, TENANT_A);
      }
      if (foreignChainId) {
        await apiClient.delete('/rule-chains/' + foreignChainId, {}, TENANT_B);
      }
    } catch (e) {
      /* ignore cleanup error */
    }
  });

  it('1. POST /edge/sync 下发规则链快照并返回任务', async function () {
    const res = await apiClient.post(
      '/edge/sync',
      { gateway_device_id: deviceId, resource_type: 'rule_chain', resource_id: chainId },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    const task = res.data;
    expect(task).to.be.an('object');
    expect(task.id).to.be.a('string').and.not.equal('');
    expect(task.gateway_device_id).to.equal(deviceId);
    expect(task.gateway_device_number).to.be.a('string').and.not.equal('');
    expect(task.resource_type).to.equal('rule_chain');
    expect(task.resource_id).to.equal(chainId);
    expect(task.attempts).to.be.a('number').at.least(1);
    // 投递结果取决于本机 broker 是否可用，但状态机只能落在三态内
    expect(['pending', 'synced', 'failed']).to.include(task.status);
    taskId = task.id;
    taskPayload = task.payload;
  });

  it('2. payload 快照信封契约：type/resource_id/name/content/revision/version/generated_at', function () {
    const payload = parsePayload({ payload: taskPayload });
    expect(payload.type).to.equal('rule_chain');
    expect(payload.resource_id).to.equal(chainId);
    expect(payload.name).to.be.a('string').and.include('edge_sync_chain_86_');
    // content 即规则链 graph：节点/边与创建时提交的图一致
    expect(payload.content).to.be.an('object');
    expect(payload.content.nodes).to.have.lengthOf(3);
    const nodeTypes = payload.content.nodes.map((n) => n.type).sort();
    expect(nodeTypes).to.deep.equal(['action.alarm', 'filter.threshold', 'trigger.telemetry']);
    expect(payload.content.edges).to.have.lengthOf(2);
    // 修订号：新建资源无历史，首版必须为 1；version 为快照结构版本（当前恒为 1）
    expect(payload.revision).to.be.a('number').and.equal(1);
    expect(payload.version).to.be.a('number').and.equal(1);
    expect(payload.generated_at).to.be.a('string').and.not.equal('');
  });

  it('3. GET /edge/sync/:id 回读与创建 payload 一致（快照不重拍）', async function () {
    const res = await apiClient.get('/edge/sync/' + taskId, {}, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.id).to.equal(taskId);
    expect(res.data.payload).to.equal(taskPayload);
  });

  it('4. POST /edge/sync/:id/retry 重放同一快照（修订号不变，幂等）', async function () {
    const res = await apiClient.post('/edge/sync/' + taskId + '/retry', {}, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    const payload = parsePayload(res.data);
    expect(payload.revision).to.equal(1);
    expect(res.data.payload).to.equal(taskPayload);
  });

  it('5. GET /edge/sync 列表可按网关过滤命中任务', async function () {
    const res = await apiClient.get('/edge/sync', { gateway_device_id: deviceId, resource_type: 'rule_chain' }, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data).to.be.an('array');
    const hit = res.data.find((item) => item.id === taskId);
    expect(hit, 'created task should appear in list').to.be.an('object');
    expect(hit.payload).to.equal(taskPayload);
  });

  it('6. 负例：未知网关设备被拒绝', async function () {
    const res = await apiClient.post(
      '/edge/sync',
      { gateway_device_id: 'nonexistent-gateway-86', resource_type: 'rule_chain', resource_id: chainId },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('7. 负例：非法 resource_type 被拒绝', async function () {
    const res = await apiClient.post(
      '/edge/sync',
      { gateway_device_id: deviceId, resource_type: 'ota', resource_id: chainId },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('8. 负例：跨租户 resource_id（租户 B 的链）被拒绝', async function () {
    const res = await apiClient.post(
      '/edge/sync',
      { gateway_device_id: deviceId, resource_type: 'rule_chain', resource_id: foreignChainId },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('9. 多租户隔离：租户 B 无法读取/重试租户 A 的同步任务', async function () {
    const getRes = await apiClient.get('/edge/sync/' + taskId, {}, TENANT_B);
    expect(getRes.code, JSON.stringify(getRes)).to.not.equal(200);

    const retryRes = await apiClient.post('/edge/sync/' + taskId + '/retry', {}, TENANT_B);
    expect(retryRes.code, JSON.stringify(retryRes)).to.not.equal(200);

    const listRes = await apiClient.get('/edge/sync', {}, TENANT_B);
    expect(listRes.code, JSON.stringify(listRes)).to.equal(200);
    const leaked = (listRes.data || []).find((item) => item.id === taskId);
    expect(leaked, '租户 B 的任务列表不得包含租户 A 的任务').to.equal(undefined);
  });
});
