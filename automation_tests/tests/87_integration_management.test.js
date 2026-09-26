/**
 * 文件用途：Integration 统一集成实体（TB-45，130.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. 集成实例完整 CRUD 生命周期（POST /api/v1/integrations 创建、PUT 更新、
 *      GET /api/v1/integrations 分页、GET /api/v1/integrations/:id 详情、DELETE 删除）；
 *   2. 参数校验（名称必填、connector_type oneof=opcua,snmp,plugin、config 必须合法 JSON）；
 *   3. 转换器绑定（绑 UPLINK 转换器回读一致；绑定不存在/跨租户转换器被拒绝——fail-closed）；
 *   4. 启停切换（enabled true/false 经 PUT 生效）；
 *   5. 严格多租户隔离（租户 B 无法查询/删除租户 A 的集成实例）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Integration Management [87_integration_management]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

describe(SUITE, function () {
  this.timeout(120000);

  let createdIntegrationId = null;
  let createdConverterId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 87_integration_management.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);

    // 前置：在租户 A 种一条 UPLINK 转换器，供绑定用例使用。
    const res = await apiClient.post(
      '/converters',
      {
        name: '契约测试转换器_87',
        type: 'UPLINK',
        converter_mode: 'JSON_PATH',
        configuration: JSON.stringify({ telemetry: { temperature: 'temp' } })
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    createdConverterId = res.data.id;
  });

  after(async function () {
    if (createdIntegrationId) {
      try {
        await apiClient.delete('/integrations/' + createdIntegrationId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
    if (createdConverterId) {
      try {
        await apiClient.delete('/converters/' + createdConverterId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. POST /integrations 创建集成实例并返回完整档案', async function () {
    const res = await apiClient.post(
      '/integrations',
      {
        name: '契约测试集成_87',
        connector_type: 'opcua',
        converter_uplink_id: createdConverterId,
        config: JSON.stringify({ device_ids: ['contract-device-87'] }),
        enabled: true
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.id).to.be.a('string').and.not.equal('');
    expect(res.data.name).to.equal('契约测试集成_87');
    expect(res.data.connector_type).to.equal('opcua');
    expect(res.data.converter_uplink_id).to.equal(createdConverterId);
    expect(res.data.tenant_id).to.be.a('string').and.not.equal('');
    expect(res.data.enabled).to.equal(true);
    createdIntegrationId = res.data.id;
  });

  it('2. POST /integrations 名称缺失返回参数错误', async function () {
    const res = await apiClient.post('/integrations', { connector_type: 'opcua' }, TENANT_A);
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('3. POST /integrations 非法 connector_type 被拒绝', async function () {
    const res = await apiClient.post(
      '/integrations',
      { name: '契约测试集成_87_坏类型', connector_type: 'modbus' },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('4. POST /integrations config 非法 JSON 被拒绝', async function () {
    const res = await apiClient.post(
      '/integrations',
      { name: '契约测试集成_87_坏配置', connector_type: 'opcua', config: '{not-json' },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('5. POST /integrations 绑定不存在的转换器被拒绝（fail-closed）', async function () {
    const res = await apiClient.post(
      '/integrations',
      {
        name: '契约测试集成_87_悬空绑定',
        connector_type: 'opcua',
        converter_uplink_id: '00000000-0000-0000-0000-00000000aa87'
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
  });

  it('6. GET /integrations 分页检索命中新集成', async function () {
    const res = await apiClient.get(
      '/integrations',
      { page: 1, page_size: 10, search: '契约测试集成_87' },
      TENANT_A
    );
    expect(res.code).to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.total).to.be.a('number').and.at.least(1);
    const hit = (res.data.list || []).find((item) => item.id === createdIntegrationId);
    expect(hit, 'created integration should appear in list').to.be.an('object');
    expect(hit.connector_type).to.equal('opcua');
  });

  it('7. GET /integrations 支持 connector_type 过滤', async function () {
    const res = await apiClient.get(
      '/integrations',
      { page: 1, page_size: 10, connector_type: 'snmp' },
      TENANT_A
    );
    expect(res.code).to.equal(200);
    const list = res.data.list || [];
    for (const item of list) {
      expect(item.connector_type).to.equal('snmp');
    }
  });

  it('8. PUT /integrations 更新名称/绑定设备配置/启停', async function () {
    const res = await apiClient.put(
      '/integrations',
      {
        id: createdIntegrationId,
        name: '契约测试集成_87_改',
        config: JSON.stringify({ device_ids: ['contract-device-87-b'] }),
        enabled: false
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data.name).to.equal('契约测试集成_87_改');
    expect(res.data.enabled).to.equal(false);
    const parsedConfig = JSON.parse(res.data.config);
    expect(parsedConfig.device_ids).to.deep.equal(['contract-device-87-b']);
  });

  it('9. GET /integrations/:id 查询集成详情', async function () {
    const res = await apiClient.get('/integrations/' + createdIntegrationId, {}, TENANT_A);
    expect(res.code).to.equal(200);
    expect(res.data.id).to.equal(createdIntegrationId);
    expect(res.data.name).to.equal('契约测试集成_87_改');
    expect(res.data.converter_uplink_id).to.equal(createdConverterId);
  });

  it('10. PUT /integrations 传空串解绑上行转换器', async function () {
    const res = await apiClient.put(
      '/integrations',
      { id: createdIntegrationId, converter_uplink_id: '' },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(
      res.data.converter_uplink_id === null || res.data.converter_uplink_id === undefined,
      'unbind should normalize to null'
    ).to.equal(true);
    // 恢复绑定，保持后续用例口径一致。
    const restore = await apiClient.put(
      '/integrations',
      { id: createdIntegrationId, converter_uplink_id: createdConverterId },
      TENANT_A
    );
    expect(restore.code).to.equal(200);
  });

  it('11. POST /integrations 绑定跨租户转换器被拒绝', async function () {
    // 租户 B 建一条自己的转换器，租户 A 尝试绑定应被拒绝（不泄露他租户资源存在性）。
    const convB = await apiClient.post(
      '/converters',
      { name: '契约测试转换器_87_B租户', type: 'UPLINK', converter_mode: 'JSON_PATH', configuration: '{}' },
      TENANT_B
    );
    expect(convB.code, JSON.stringify(convB)).to.equal(200);
    const res = await apiClient.post(
      '/integrations',
      {
        name: '契约测试集成_87_跨租户绑定',
        connector_type: 'opcua',
        converter_uplink_id: convB.data.id
      },
      TENANT_A
    );
    expect(res.code, JSON.stringify(res)).to.not.equal(200);
    await apiClient.delete('/converters/' + convB.data.id, {}, TENANT_B);
  });

  it('12. 租户 B 无法查询租户 A 的集成详情', async function () {
    const res = await apiClient.get('/integrations/' + createdIntegrationId, {}, TENANT_B);
    expect(res.code).to.not.equal(200);
  });

  it('13. 租户 B 无法删除租户 A 的集成', async function () {
    const res = await apiClient.delete('/integrations/' + createdIntegrationId, {}, TENANT_B);
    expect(res.code).to.not.equal(200);
    const check = await apiClient.get('/integrations/' + createdIntegrationId, {}, TENANT_A);
    expect(check.code).to.equal(200);
  });

  it('14. DELETE /integrations/:id 删除后不可再查询', async function () {
    const res = await apiClient.delete('/integrations/' + createdIntegrationId, {}, TENANT_A);
    expect(res.code).to.equal(200);
    const check = await apiClient.get('/integrations/' + createdIntegrationId, {}, TENANT_A);
    expect(check.code).to.not.equal(200);
    createdIntegrationId = null;
  });
});
