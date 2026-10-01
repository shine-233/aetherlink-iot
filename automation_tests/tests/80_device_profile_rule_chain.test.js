/**
 * 文件用途：设备 Profile 档案级默认规则链（TB-18，125.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. 档案绑定/解绑 API 契约：PUT /api/v1/device_config 携带 default_rule_chain_id
 *      绑定规则链、传空字符串解绑；详情回显 default_rule_chain_id；
 *   2. 绑定校验（fail-closed）：绑定不存在/跨租户的链被拒绝；
 *   3. 执行解析接口契约：GET /api/v1/rule-chains/device-effective/:deviceId 返回
 *      档案绑定链（source=profile）优先、租户级启用链（source=tenant）兜底的清单；
 *   4. 删除守卫：仍被档案绑定的规则链删除被拒绝，解绑后可删除；
 *   5. 严格多租户隔离：租户 B 无法解析租户 A 设备的生效链。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Device Profile Default Rule Chain [80_device_profile_rule_chain]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

// 与后端单测一致的合法最小图：遥测触发 + 字段映射，保证规则链创建可过 DAG 校验。
const SIMPLE_GRAPH = JSON.stringify({
  nodes: [
    { id: 't', type: 'trigger.telemetry' },
    { id: 'm', type: 'transform.mapping', config: { fields: { temperature: 'temp' } } }
  ],
  edges: [{ from: 't', to: 'm' }]
});

function pickId(record) {
  return record && (record.id || record.ID) ? record.id || record.ID : null;
}

describe(SUITE, function () {
  this.timeout(120000);

  let createdChainId = null;
  let createdConfigId = null;
  let createdDeviceId = null;
  let foreignChainId = null; // 租户 B 的链，用于跨租户负例

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 80_device_profile_rule_chain.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);

    // 租户 A：建启用链 + 档案 + 设备（贯穿主链路）
    const chainRes = await apiClient.post(
      '/rule-chains',
      { name: 'profile_chain_80_' + Date.now(), enabled: true, graph: SIMPLE_GRAPH },
      TENANT_A
    );
    expect(chainRes.code, JSON.stringify(chainRes)).to.equal(200);
    createdChainId = pickId(chainRes.data);
    expect(createdChainId).to.be.a('string').and.not.equal('');

    const configRes = await apiClient.post(
      '/device_config',
      {
        name: 'profile_chain_cfg_80_' + Date.now(),
        device_type: '1',
        protocol_type: 'MQTT',
        voucher_type: 'ACCESSTOKEN',
        device_conn_type: 'A'
      },
      TENANT_A
    );
    expect(configRes.code, JSON.stringify(configRes)).to.equal(200);
    createdConfigId = pickId(configRes.data);

    const deviceRes = await apiClient.post(
      '/device',
      {
        name: 'profile_chain_device_80_' + Date.now(),
        device_config_id: createdConfigId,
        voucher: JSON.stringify({ username: 'chain80_' + Date.now(), password: 'chain80_pwd' })
      },
      TENANT_A
    );
    expect(deviceRes.code, JSON.stringify(deviceRes)).to.equal(200);
    createdDeviceId = pickId(deviceRes.data);

    // 租户 B：建一条启用链，供跨租户绑定负例
    const foreignRes = await apiClient.post(
      '/rule-chains',
      { name: 'foreign_chain_80_' + Date.now(), enabled: true, graph: SIMPLE_GRAPH },
      TENANT_B
    );
    expect(foreignRes.code, JSON.stringify(foreignRes)).to.equal(200);
    foreignChainId = pickId(foreignRes.data);
  });

  after(async function () {
    // 顺序：解绑 → 删设备/档案/链（删除守卫要求先解绑）
    try {
      if (createdConfigId) {
        await apiClient.put('/device_config', { id: createdConfigId, default_rule_chain_id: '' }, TENANT_A);
      }
      if (createdDeviceId) {
        await apiClient.delete('/device/' + createdDeviceId, {}, TENANT_A);
      }
      if (createdConfigId) {
        await apiClient.delete('/device_config/' + createdConfigId, {}, TENANT_A);
      }
      if (createdChainId) {
        await apiClient.delete('/rule-chains/' + createdChainId, {}, TENANT_A);
      }
      if (foreignChainId) {
        await apiClient.delete('/rule-chains/' + foreignChainId, {}, TENANT_B);
      }
    } catch (e) {
      /* ignore cleanup error */
    }
  });

  it('1. PUT /device_config 绑定档案级默认规则链并在详情回显', async function () {
    const bindRes = await apiClient.put(
      '/device_config',
      { id: createdConfigId, default_rule_chain_id: createdChainId },
      TENANT_A
    );
    expect(bindRes.code, JSON.stringify(bindRes)).to.equal(200);

    const detail = await apiClient.get('/device_config/' + createdConfigId, {}, TENANT_A);
    expect(detail.code).to.equal(200);
    expect(detail.data.default_rule_chain_id).to.equal(createdChainId);
  });

  it('2. 绑定不存在的链被拒绝', async function () {
    const res = await apiClient.put(
      '/device_config',
      { id: createdConfigId, default_rule_chain_id: '00000000-0000-0000-0000-00000000aa80' },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
    const detail = await apiClient.get('/device_config/' + createdConfigId, {}, TENANT_A);
    expect(detail.data.default_rule_chain_id).to.equal(createdChainId, '绑定失败后原绑定不受影响');
  });

  it('3. 绑定跨租户的链被拒绝', async function () {
    const res = await apiClient.put(
      '/device_config',
      { id: createdConfigId, default_rule_chain_id: foreignChainId },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('4. 解析接口：档案绑定链优先（source=profile）、租户启用链兜底（source=tenant）', async function () {
    const res = await apiClient.get(
      '/rule-chains/device-effective/' + createdDeviceId,
      {},
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.device_id).to.equal(createdDeviceId);
    expect(res.data.device_config_id).to.equal(createdConfigId);
    expect(res.data.default_rule_chain_id).to.equal(createdChainId);
    expect(res.data.chains).to.be.an('array').and.have.length.of.at.least(1);
    const profileHit = res.data.chains.find((c) => c.source === 'profile');
    expect(profileHit, '档案绑定链应出现在清单首位').to.be.an('object');
    expect(profileHit.id).to.equal(createdChainId);
    expect(profileHit.name).to.be.a('string').and.not.equal('');
    // 档案链之外只允许 tenant 来源，且不重复出现档案链
    for (const ref of res.data.chains) {
      if (ref.id === createdChainId) {
        expect(ref.source).to.equal('profile');
      } else {
        expect(ref.source).to.equal('tenant');
      }
    }
  });

  it('5. 解析接口：解绑后回落租户级启用链（default_rule_chain_id 为空）', async function () {
    const unbindRes = await apiClient.put(
      '/device_config',
      { id: createdConfigId, default_rule_chain_id: '' },
      TENANT_A
    );
    expect(unbindRes.code, JSON.stringify(unbindRes)).to.equal(200);

    const detail = await apiClient.get('/device_config/' + createdConfigId, {}, TENANT_A);
    expect(detail.code).to.equal(200);
    expect(
      detail.data.default_rule_chain_id === null || detail.data.default_rule_chain_id === undefined,
      '解绑后详情不应再回显绑定值'
    ).to.equal(true);

    const res = await apiClient.get('/rule-chains/device-effective/' + createdDeviceId, {}, TENANT_A);
    expect(res.code).to.equal(200);
    expect(res.data.default_rule_chain_id === null || res.data.default_rule_chain_id === undefined).to.equal(true);
    expect(res.data.chains.every((c) => c.source === 'tenant'), '解绑后全部为租户级链').to.equal(true);

    // 重新绑定，供后续删除守卫用例使用
    const rebind = await apiClient.put(
      '/device_config',
      { id: createdConfigId, default_rule_chain_id: createdChainId },
      TENANT_A
    );
    expect(rebind.code).to.equal(200);
  });

  it('6. 删除守卫：仍被档案绑定的规则链删除被拒绝，解绑后可删除', async function () {
    const blocked = await apiClient.delete('/rule-chains/' + createdChainId, {}, TENANT_A);
    expect(blocked.code, JSON.stringify(blocked)).to.not.equal(200);

    const unbind = await apiClient.put(
      '/device_config',
      { id: createdConfigId, default_rule_chain_id: '' },
      TENANT_A
    );
    expect(unbind.code).to.equal(200);
    const removed = await apiClient.delete('/rule-chains/' + createdChainId, {}, TENANT_A);
    expect(removed.code, JSON.stringify(removed)).to.equal(200);
    createdChainId = null; // 已删除，after 不再清理
  });

  it('7. 租户隔离：租户 B 无法解析租户 A 设备的生效规则链', async function () {
    const res = await apiClient.get('/rule-chains/device-effective/' + createdDeviceId, {}, TENANT_B);
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });
});
