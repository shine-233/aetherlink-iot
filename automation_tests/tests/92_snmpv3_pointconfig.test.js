/**
 * 文件用途：SNMPv3 点表契约（TB-22，backend/internal/collector/pointconfig）活栈契约测试。
 *
 * 覆盖：
 *   1. v3 点表保存契约：POST /api/v1/device_config 携带 SNMP v3_user/auth_proto/
 *      auth_passphrase 点表创建成功，详情按原文回显（前后端动态表单契约单一来源）；
 *   2. v2c 既有契约不回归：无 v3 字段的 SNMP 点表、缺 community/缺 points 等负例行为不变；
 *   3. v3 fail-closed 校验（保存链路与采集链路共用 pointconfig 单一解析器）：
 *      缺 v3_user 的孤儿 v3 字段、缺/短 auth_passphrase、非法 auth_proto、
 *      priv_proto 非 none、悬空 priv_passphrase 一律拒绝；
 *   4. 更新路径：PUT 携带非法点表被拒绝，合法点表可更新（生效值语义）；
 *   5. 严格多租户隔离：租户 B 无法读取租户 A 的设备配置。
 *
 * 说明：LwM2M 多客户端隔离为 CoAP/UDP 运行时行为，无法经 HTTP 契约面验证，
 * 由 backend/internal/protocolgw、internal/coap、internal/lwm2m 的 Go 单测闭环覆盖。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'SNMPv3 Point Config [92_snmpv3_pointconfig]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

// 合法 v3 SNMP 点表：v3_user 非空即走 v3 路径，community 可缺省。
const VALID_V3_CONFIG = JSON.stringify({
  target: '10.0.0.5:161',
  v3_user: 'authUser',
  auth_proto: 'sha',
  auth_passphrase: 'authkey123',
  timeout_ms: 1500,
  points: [{ key: 'temperature', oid: '1.3.6.1.2.1.1.3.0' }]
});

// 合法 v2c 点表（既有契约基准）。
const VALID_V2C_CONFIG = JSON.stringify({
  target: '10.0.0.5:161',
  community: 'public',
  timeout_ms: 1500,
  points: [{ key: 'temperature', oid: '1.3.6.1.2.1.1.3.0' }]
});

function snmpConfigBody(name, protocolConfig) {
  return {
    name: name,
    device_type: '1',
    protocol_type: 'SNMP',
    device_conn_type: 'A',
    protocol_config: protocolConfig
  };
}

function pickId(record) {
  return record && (record.id || record.ID) ? record.id || record.ID : null;
}

describe(SUITE, function () {
  this.timeout(120000);

  let createdConfigId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 92_snmpv3_pointconfig.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  after(async function () {
    if (createdConfigId) {
      try {
        await apiClient.delete('/device_config/' + createdConfigId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. 合法 v3 点表创建成功并按原文回显（契约单一来源）', async function () {
    const res = await apiClient.post('/device_config', snmpConfigBody('snmp_v3_cfg_92_' + Date.now(), VALID_V3_CONFIG), TENANT_A);
    expect(res.code, JSON.stringify(res)).to.equal(200);
    createdConfigId = pickId(res.data);
    expect(createdConfigId).to.be.a('string').and.not.equal('');

    const detail = await apiClient.get('/device_config/' + createdConfigId, {}, TENANT_A);
    expect(detail.code, JSON.stringify(detail)).to.equal(200);
    expect(detail.data.protocol_type).to.equal('SNMP');
    const saved = JSON.parse(detail.data.protocol_config);
    // 五个 v3 键 + points 原文回显：动态表单 dataKey 契约与 pointconfig 结构一致。
    expect(saved.v3_user).to.equal('authUser');
    expect(saved.auth_proto).to.equal('sha');
    expect(saved.auth_passphrase).to.equal('authkey123');
    expect(saved.target).to.equal('10.0.0.5:161');
    expect(saved.points).to.be.an('array').with.lengthOf(1);
    expect(saved.points[0].key).to.equal('temperature');
    expect(saved.points[0].oid).to.equal('1.3.6.1.2.1.1.3.0');
  });

  it('2. v2c 既有契约不回归：无 v3 字段的合法点表创建成功', async function () {
    const res = await apiClient.post(
      '/device_config',
      snmpConfigBody('snmp_v2c_cfg_92_' + Date.now(), VALID_V2C_CONFIG),
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    const v2cId = pickId(res.data);
    expect(v2cId).to.be.a('string').and.not.equal('');
    const cleanup = await apiClient.delete('/device_config/' + v2cId, {}, TENANT_A);
    expect(cleanup.code, JSON.stringify(cleanup)).to.equal(200);
  });

  const REJECTED_CASES = [
    ['缺 v3_user 的孤儿 v3 字段', JSON.stringify({ target: '10.0.0.5:161', community: 'public', auth_proto: 'sha', points: [{ key: 'k', oid: '1.3' }] })],
    ['缺 auth_passphrase', JSON.stringify({ target: '10.0.0.5:161', v3_user: 'authUser', auth_proto: 'sha', points: [{ key: 'k', oid: '1.3' }] })],
    ['auth_passphrase 少于 8 字符', JSON.stringify({ target: '10.0.0.5:161', v3_user: 'authUser', auth_proto: 'sha', auth_passphrase: 'short', points: [{ key: 'k', oid: '1.3' }] })],
    ['非法 auth_proto', JSON.stringify({ target: '10.0.0.5:161', v3_user: 'authUser', auth_proto: 'des', auth_passphrase: 'authkey123', points: [{ key: 'k', oid: '1.3' }] })],
    ['缺 auth_proto', JSON.stringify({ target: '10.0.0.5:161', v3_user: 'authUser', auth_passphrase: 'authkey123', points: [{ key: 'k', oid: '1.3' }] })],
    ['priv_proto 非 none（authPriv 未实现）', JSON.stringify({ target: '10.0.0.5:161', v3_user: 'authUser', auth_proto: 'sha', auth_passphrase: 'authkey123', priv_proto: 'aes', points: [{ key: 'k', oid: '1.3' }] })],
    ['priv 启用前悬空 priv_passphrase', JSON.stringify({ target: '10.0.0.5:161', v3_user: 'authUser', auth_proto: 'sha', auth_passphrase: 'authkey123', priv_passphrase: 'privkey123', points: [{ key: 'k', oid: '1.3' }] })]
  ];

  for (const [label, config] of REJECTED_CASES) {
    it(`3. fail-closed：${label} 的点表创建被拒绝`, async function () {
      const res = await apiClient.post(
        '/device_config',
        snmpConfigBody('snmp_bad_92_' + Date.now(), config),
        TENANT_A
      );
      expect(res.code, JSON.stringify(res)).to.not.equal(200);
    });
  }

  it('3.8 v2c 负例不回归：缺 community/缺 points 仍被拒绝', async function () {
    const noCommunity = await apiClient.post(
      '/device_config',
      snmpConfigBody('snmp_nocom_92_' + Date.now(), JSON.stringify({ target: '10.0.0.5:161', points: [{ key: 'k', oid: '1.3' }] })),
      TENANT_A
    );
    expect(noCommunity.code, JSON.stringify(noCommunity)).to.not.equal(200);

    const noPoints = await apiClient.post(
      '/device_config',
      snmpConfigBody('snmp_nopoints_92_' + Date.now(), JSON.stringify({ target: '10.0.0.5:161', community: 'public' })),
      TENANT_A
    );
    expect(noPoints.code, JSON.stringify(noPoints)).to.not.equal(200);
  });

  it('4. 更新路径：非法点表被拒绝，合法点表可更新', async function () {
    const badUpdate = await apiClient.put(
      '/device_config',
      {
        id: createdConfigId,
        protocol_config: JSON.stringify({ target: '10.0.0.5:161', v3_user: 'authUser', auth_proto: 'bogus', auth_passphrase: 'authkey123', points: [{ key: 'k', oid: '1.3' }] })
      },
      TENANT_A
    );
    expect(badUpdate.code, JSON.stringify(badUpdate)).to.not.equal(200);

    const goodUpdate = await apiClient.put(
      '/device_config',
      {
        id: createdConfigId,
        protocol_config: VALID_V3_CONFIG
      },
      TENANT_A
    );
    expect(goodUpdate.code, JSON.stringify(goodUpdate)).to.equal(200);
  });

  it('5. 租户隔离：租户 B 无法读取租户 A 的设备配置', async function () {
    const res = await apiClient.get('/device_config/' + createdConfigId, {}, TENANT_B);
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });
});
