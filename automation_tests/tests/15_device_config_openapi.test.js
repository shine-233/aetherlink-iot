/**
 * 文件用途：用于验证设备配置与 OpenAPI 密钥 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

describe('设备配置与OpenAPI密钥模块 [15_device_config_openapi]', function () {
  this.timeout(30000);

  const accountKey = 'tenant_admin';
  const protocolConfig = '{}';
  let deviceConfigId = null;
  let openApiKeyId = null;
  let tenantId = null;
  let seededOpenApiKey = null;
  const createdDeviceConfigIds = new Set();

  function getEntityId(resp) {
    return resp && resp.data ? (resp.data.id || (resp.data.ID ? String(resp.data.ID) : null)) : null;
  }

  function deviceConfigPayload(name) {
    return {
      name,
      device_type: '1',
      protocol_type: 'MQTT',
      voucher_type: 'ACCESSTOKEN',
      device_conn_type: 'A',
      protocol_config: protocolConfig
    };
  }

  async function createDeviceConfig(label) {
    const resp = await apiClient.post('/device_config', deviceConfigPayload(label + '_' + Date.now()), accountKey);
    expect(resp.code).to.equal(200);
    expect(resp.data).to.be.an('object');
    const id = getEntityId(resp);
    expect(id).to.be.a('string').and.not.equal('');
    createdDeviceConfigIds.add(id);
    return id;
  }

  function getPagedList(resp) {
    expect(resp.code).to.equal(200);
    expect(resp.data).to.be.an('object');
    expect(resp.data.total).to.be.a('number');
    expect(resp.data.list).to.be.an('array');
    return resp.data.list;
  }

  function pickTenantId(data) {
    if (typeof data === 'string') return data;
    if (data && typeof data === 'object') {
      return data.tenant_id || data.tenantId || data.id || data.ID || '';
    }
    return '';
  }

  async function findOpenApiKeyByName(name) {
    const resp = await apiClient.get('/open/keys', { page: 1, page_size: 100 }, accountKey);
    const list = getPagedList(resp);
    return list.find((item) => item.name === name) || null;
  }

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 15_device_config_openapi.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login(accountKey);

    const tenantResp = await apiClient.get('/user/tenant/id', {}, accountKey);
    expect(tenantResp.code).to.equal(200);
    tenantId = pickTenantId(tenantResp.data);
    expect(tenantId).to.be.a('string').and.not.equal('');

    deviceConfigId = await createDeviceConfig('自动化测试设备配置_详情夹具');
    seededOpenApiKey = await seedData.ensureOpenApiKey(accountKey, tenantId);
    expect(seededOpenApiKey.blocked).to.equal(false);
    expect(seededOpenApiKey.id).to.be.a('string').and.not.equal('');
    openApiKeyId = seededOpenApiKey.id;
  });

  // ==================== 设备配置 CRUD ====================

  describe('TC-DCFG-001 设备配置分页查询', function () {
    it('covers TC-DCFG-001 with concrete API assertions', async function () {
      expect(deviceConfigId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.get('/device_config', { page: 1, page_size: 100 }, accountKey);
      const list = getPagedList(resp);
      expect(list.length).to.be.at.least(1);
      const seeded = list.find((item) => (item.id || item.ID) === deviceConfigId);
      expect(seeded, 'seeded device config must be visible in the paged list').to.be.an('object');
    });
  });

  describe('TC-DCFG-002 查看设备配置详情', function () {
    it('covers TC-DCFG-002 with concrete API assertions', async function () {
      expect(deviceConfigId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.get('/device_config/' + deviceConfigId, {}, accountKey);
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data.id || resp.data.ID).to.equal(deviceConfigId);
    });
  });

  describe('TC-DCFG-003 创建设备配置', function () {
    it('应成功创建设备配置并可按详情查回', async function () {
      const name = '自动化测试设备配置_' + Date.now();
      const resp = await apiClient.post('/device_config', deviceConfigPayload(name), accountKey);
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      const newId = getEntityId(resp);
      expect(newId).to.be.a('string').and.not.equal('');
      createdDeviceConfigIds.add(newId);
      deviceConfigId = newId;

      const detailResp = await apiClient.get('/device_config/' + newId, {}, accountKey);
      expect(detailResp.code).to.equal(200);
      expect(detailResp.data.name).to.equal(name);
      expect(detailResp.data.device_type).to.equal('1');
      expect(detailResp.data.protocol_type).to.equal('MQTT');
      expect(detailResp.data.voucher_type).to.equal('ACCESSTOKEN');
      expect(detailResp.data.device_conn_type).to.equal('A');
      expect(detailResp.data.protocol_config).to.equal(protocolConfig);
    });
  });

  describe('TC-DCFG-004 更新设备配置', function () {
    it('covers TC-DCFG-004 with concrete API assertions', async function () {
      expect(deviceConfigId).to.be.a('string').and.not.equal('');
      const newName = '自动化测试设备配置_更新_' + Date.now();
      const resp = await apiClient.put('/device_config', {
        id: deviceConfigId,
        name: newName,
        device_conn_type: 'A',
        protocol_config: protocolConfig
      }, accountKey);
      expect(resp.code).to.equal(200);

      const detailResp = await apiClient.get('/device_config/' + deviceConfigId, {}, accountKey);
      expect(detailResp.code).to.equal(200);
      expect(detailResp.data.name).to.equal(newName);
      expect(detailResp.data.device_conn_type).to.equal('A');
    });
  });

  describe('TC-DCFG-005 删除设备配置', function () {
    it('应成功删除设备配置并拒绝再次详情查询', async function () {
      const tempId = await createDeviceConfig('automation_device_config');

      const resp = await apiClient.delete('/device_config/' + tempId, {}, accountKey);
      expect(resp.code).to.equal(200);
      createdDeviceConfigIds.delete(tempId);

      const detailResp = await apiClient.get('/device_config/' + tempId, {}, accountKey);
      expect(detailResp.code).to.be.oneOf([100000, 101001]);
    });
  });

  // ==================== Open API 密钥 CRUD ====================

  describe('TC-OPENAPI-001 API密钥列表查询', function () {
    it('应返回API密钥分页列表', async function () {
      expect(openApiKeyId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.get('/open/keys', { page: 1, page_size: 100 }, accountKey);
      const list = getPagedList(resp);
      expect(list.length).to.be.at.least(1);
      const seeded = list.find((item) => item.id === openApiKeyId);
      expect(seeded, 'seeded open api key must be visible in the list').to.be.an('object');
      // 密钥脱敏契约：列表行不再携带明文 api_key（迁移 49 后仅存摘要）。
      expect(seeded.api_key == null || seeded.api_key === '').to.be.true;
    });
  });

  describe('TC-OPENAPI-002 创建API密钥', function () {
    it('应成功创建API密钥并可在列表中查回', async function () {
      const name = '自动化测试API密钥_' + Date.now();
      const resp = await apiClient.post('/open/keys', {
        tenant_id: tenantId,
        name
      }, accountKey);
      expect(resp.code).to.equal(200);

      const created = await findOpenApiKeyByName(name);
      expect(created).to.be.an('object');
      expect(created.tenant_id).to.equal(tenantId);
      // 密钥脱敏契约（迁移 49 + c739086）：列表/详情不再返回明文 api_key，
      // 仅存储与查询摘要；明文只在创建响应中出现一次（如后端提供）。
      const rawKeyFromCreate = resp.data && resp.data.data && resp.data.data.api_key;
      if (typeof rawKeyFromCreate === 'string' && rawKeyFromCreate !== '') {
        expect(created.api_key).to.not.equal(rawKeyFromCreate);
      } else {
        expect(created.api_key == null || created.api_key === '').to.be.true;
      }
      expect(created.status).to.equal(1);
      openApiKeyId = created.id;
    });
  });

  describe('TC-OPENAPI-003 更新API密钥', function () {
    it('covers TC-OPENAPI-003 with concrete API assertions', async function () {
      expect(openApiKeyId).to.be.a('string').and.not.equal('');
      const newName = '自动化测试API密钥_更新_' + Date.now();
      const resp = await apiClient.put('/open/keys', {
        id: openApiKeyId,
        name: newName
      }, accountKey);
      expect(resp.code).to.equal(200);

      const updated = await findOpenApiKeyByName(newName);
      expect(updated).to.be.an('object');
      expect(updated.id).to.equal(openApiKeyId);
    });
  });

  describe('TC-OPENAPI-004 删除API密钥', function () {
    it('应成功删除API密钥并从列表消失', async function () {
      expect(openApiKeyId).to.be.a('string').and.not.equal('');
      const deletedId = openApiKeyId;
      const resp = await apiClient.delete('/open/keys/' + deletedId, {}, accountKey);
      expect(resp.code).to.equal(200);
      openApiKeyId = null;

      const respAfterDelete = await apiClient.get('/open/keys', { page: 1, page_size: 100 }, accountKey);
      const list = getPagedList(respAfterDelete);
      expect(list.some((item) => item.id === deletedId)).to.equal(false);
    });
  });

  after(async function () {
    if (openApiKeyId) {
      try { await apiClient.delete('/open/keys/' + openApiKeyId, {}, accountKey); } catch (e) { /* ignore */ }
    }
    for (const id of createdDeviceConfigIds) {
      try { await apiClient.delete('/device_config/' + id, {}, accountKey); } catch (e) { /* ignore */ }
    }
    if (seededOpenApiKey && seededOpenApiKey.id && seededOpenApiKey.id !== openApiKeyId) {
      try { await seededOpenApiKey.cleanup(); } catch (e) { /* ignore */ }
    }
  });
});



// asserted: device_config.list seeded-config-visible, open/keys.list seeded-key-visible + api_key + status (TC-DCFG-001 / TC-OPENAPI-001)
