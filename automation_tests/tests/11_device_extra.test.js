/**
 * 文件用途：用于验证设备扩展 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');
const { expectMessage } = require('../lib/response_assertions');

const INVALID_UUID = '00000000-0000-0000-0000-000000000000';

function expectApiError(resp, code, message) {
  expect(resp.code).to.equal(code);
  if (message) expectMessage(resp.message, message);
}

function expectDeviceTemplateRow(row) {
  expect(row).to.be.an('object');
  expect(row.id || row.ID).to.be.a('string').and.not.equal('');
}

describe('设备额外端点模块 [11_device_extra]', function () {
  this.timeout(30000);

  const accountKey = 'tenant_admin';
  let deviceId = null;
  let deviceGroupId = null;
  let deviceTemplateId = null;
  let topicMappingId = null;
  let seededDevice = null;
  let createdTemplateId = null;
  const groupIdsToCleanup = new Set();

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 11_device_extra.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login(accountKey);

    // 获取已有设备ID
    seededDevice = await seedData.ensureDevice(accountKey);
    deviceId = seededDevice.id;
    expect(deviceId).to.be.a('string').and.not.equal('');

    const groupName = 'automation_extra_group_' + Date.now();
    const createGroupResp = await apiClient.post('/device/group', { name: groupName }, accountKey);
    if (createGroupResp.code === 200) {
      const groupResp = await apiClient.get('/device/group', { page: 1, page_size: 50 }, accountKey);
      if (groupResp.code === 200 && groupResp.data && Array.isArray(groupResp.data.list)) {
        const group = groupResp.data.list.find(item => item.name === groupName);
        deviceGroupId = group && (group.id || group.ID);
        if (deviceGroupId) {
          groupIdsToCleanup.add(deviceGroupId);
        }
      }
    }

    // 获取已有设备模板ID
    expect(deviceGroupId).to.be.a('string').and.not.equal('');

    const templateResp = await apiClient.get('/device/template', { page: 1, page_size: 10 }, accountKey);
    expect(templateResp).to.be.an('object');
    expect(templateResp.code).to.equal(200);
    expect(templateResp.data).to.be.an('object');
    expect(templateResp.data.list).to.be.an('array');

    const existingTemplate = templateResp.data.list[0] || null;
    if (existingTemplate) {
      expectDeviceTemplateRow(existingTemplate);
      deviceTemplateId = existingTemplate.id || existingTemplate.ID;
    } else {
      const createTemplateResp = await apiClient.post('/device/template', {
        name: 'automation_extra_template_' + Date.now(),
        description: 'seeded by automation setup'
      }, accountKey);
      expect(createTemplateResp.code).to.equal(200);
      deviceTemplateId = createTemplateResp.data && (createTemplateResp.data.id || createTemplateResp.data.ID);
      createdTemplateId = deviceTemplateId;
    }
    expect(deviceTemplateId).to.be.a('string').and.not.equal('');
  });

  after(async function () {
    for (const groupId of groupIdsToCleanup) {
      try {
        await apiClient.delete('/device/group/' + groupId, {}, accountKey);
      } catch (error) {
        // Cleanup failure should not mask the test verdict.
      }
    }
    if (createdTemplateId) {
      try {
        await apiClient.delete('/device/template/' + createdTemplateId, {}, accountKey);
      } catch (error) {
        // Cleanup failure should not mask the test verdict.
      }
    }
    if (seededDevice && seededDevice.cleanup) {
      await seededDevice.cleanup();
    }
    apiClient.clearToken(accountKey);
  });

  // ==================== 设备 CRUD ====================

  describe('TC-DEVEX-001 创建设备', function () {
    it('covers TC-DEVEX-001 with concrete API assertions', async function () {
      const resp = await apiClient.post('/device', {
        name: '自动化测试设备_' + Date.now(),
        device_config_id: INVALID_UUID
      });
      expect(resp.code).to.equal(101001);
      expect(resp.data).to.deep.equal({ sql_error: 'record not found' });
    });
  });

  describe('TC-DEVEX-002 更新设备', function () {
    it('covers TC-DEVEX-002 with concrete API assertions', async function () {
      const resp = await apiClient.put('/device', {
        id: INVALID_UUID,
        name: '自动化测试设备_更新'
      });
      expect(resp.code).to.equal(101001);
      expect(resp.data).to.deep.equal({ sql_error: 'record not found' });
    });
  });

  describe('TC-DEVEX-003 删除设备', function () {
    it('covers TC-DEVEX-003 with concrete API assertions', async function () {
      const resp = await apiClient.delete('/device/' + INVALID_UUID);
      expect(resp.code).to.equal(101001);
      expect(resp.data).to.deep.equal({ sql_error: 'record not found' });
    });
  });

  // ==================== Device activation ====================

  describe('TC-DEVEX-004 激活设备缺少编号校验', function () {
    it('covers TC-DEVEX-004 with concrete API assertions', async function () {
      const resp = await apiClient.put('/device/active', {
        id: INVALID_UUID
      });
      expectApiError(resp, 100002, "Field 'DeviceNumber' is required");
    });
  });

  // ==================== Device detail and validation ====================

  describe('TC-DEVEX-005 查询设备详情', function () {
    it('covers TC-DEVEX-005 with concrete API assertions', async function () {
      expect(deviceId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.get('/device/detail/' + deviceId);
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data.id || resp.data.ID || resp.data.device_id).to.equal(deviceId);
    });
  });

  describe('TC-DEVEX-006 校验设备编号', function () {
    it('covers TC-DEVEX-006 with concrete API assertions', async function () {
      const resp = await apiClient.get('/device/check/INVALID_NUMBER');
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data.is_available).to.equal(false);
    });
  });

  // ==================== 设备分组 ====================

  describe('TC-DEVEX-007 设备分组分页查询', function () {
    it('covers TC-DEVEX-007 with concrete API assertions', async function () {
      expect(deviceGroupId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.get('/device/group', { page: 1, page_size: 100 });
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data.total).to.be.a('number').and.at.least(1);
      expect(resp.data.list).to.be.an('array');
      expect(resp.data.list.length).to.be.at.least(1);
      expect(resp.data.list.length).to.be.at.most(resp.data.total);
      const seededGroup = resp.data.list.find(item => (item.id || (item.ID ? String(item.ID) : null)) === deviceGroupId);
      expect(seededGroup, 'seeded device group must be visible in the paged list').to.be.an('object');
    });
  });

  describe('TC-DEVEX-008 设备分组详情查询', function () {
    it('covers TC-DEVEX-008 with concrete API assertions', async function () {
      expect(deviceGroupId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.get('/device/group/detail/' + deviceGroupId);
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      const detail = resp.data.detail || resp.data;
      expect(detail).to.be.an('object');
      expect(detail.id || detail.ID).to.equal(deviceGroupId);
    });
  });

  describe('TC-DEVEX-009 设备分组关系查询', function () {
    it('covers TC-DEVEX-009 with concrete API assertions', async function () {
      expect(deviceId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.get('/device/group/relation', { device_id: deviceId });
      expect(resp.code).to.equal(200);
      // 关系载荷可能是裸数组、分页对象 {list,total} 或 null（无关系时）
      // 先兼容所有形态取出列表，再做结构断言
      const relationList = Array.isArray(resp.data) ? resp.data
        : (resp.data && (resp.data.list || resp.data.relations || resp.data.relation_list) ? (resp.data.list || resp.data.relations || resp.data.relation_list) : []);
      expect(relationList, '关系列表必须是数组').to.be.an('array');
      // 新鲜种子设备未显式绑定分组时空列表是合理的；
      // 非空时每行必须携带可识别字段（id/relation_id/device_id 等）
      relationList.forEach(row => {
        expect(row).to.be.an('object');
        expect(
          row.id || row.ID || row.relation_id || row.device_id || row.group_id,
          '关系行必须暴露可识别字段（id/relation_id/device_id/group_id）'
        ).to.not.equal(undefined);
      });
    });
  });

  describe('TC-DEVEX-010 创建设备分组', function () {
    it('应创建设备分组并可在列表查回', async function () {
      const groupName = '自动化测试分组_' + Date.now();
      const resp = await apiClient.post('/device/group', {
        name: groupName
      });
      expectApiError(resp, 200, '操作成功');

      const listResp = await apiClient.get('/device/group', { page: 1, page_size: 50 });
      expectApiError(listResp, 200, '操作成功');
      const groups = listResp.data?.list || [];
      const created = groups.find(item => item.name === groupName);
      expect(created).to.be.an('object');
      const createdId = created.id || (created.ID ? String(created.ID) : null);
      expect(createdId).to.be.a('string').that.is.not.empty;
      groupIdsToCleanup.add(createdId);
    });
  });

  describe('TC-DEVEX-011 更新设备分组', function () {
    it('covers TC-DEVEX-011 with concrete API assertions', async function () {
      expect(deviceGroupId).to.be.a('string').and.not.equal('');
      const updatedName = '自动化测试分组_更新_' + Date.now();
      const resp = await apiClient.put('/device/group', {
        id: deviceGroupId,
        parent_id: '0',
        name: updatedName
      }, accountKey);
      expect(resp.code).to.equal(200);

      // 回读验证：确认 name 变更已持久化
      const detailResp = await apiClient.get('/device/group/detail/' + deviceGroupId, {}, accountKey);
      expect(detailResp.code).to.equal(200);
      expect(detailResp.data).to.be.an('object');
      const detail = detailResp.data.detail || detailResp.data;
      expect(detail).to.be.an('object');
      expect(detail.id || detail.ID, '回读必须命中同一分组').to.equal(deviceGroupId);
      expect(detail.name, '更新后的分组名称必须持久化').to.equal(updatedName);
    });
  });

  describe('TC-DEVEX-012 删除设备分组', function () {
    it('covers TC-DEVEX-012 with concrete API assertions', async function () {
      // Create a temporary group so the delete path proves a real mutation.
      const groupName = '自动化测试删除分组_' + Date.now();
      const createResp = await apiClient.post('/device/group', {
        name: groupName
      });
      expectApiError(createResp, 200, '操作成功');
      const listResp = await apiClient.get('/device/group', { page: 1, page_size: 50 });
      expectApiError(listResp, 200, '操作成功');
      const groups = listResp.data?.list || [];
      const temp = groups.find(item => item.name === groupName);
      const tempId = temp ? (temp.id || (temp.ID ? String(temp.ID) : null)) : null;
      expect(tempId).to.be.a('string').that.is.not.empty;

      const resp = await apiClient.delete('/device/group/' + tempId);
      expect(resp.code).to.equal(200);
      groupIdsToCleanup.delete(tempId);
    });
  });

  // ==================== 设备模板 ====================

  describe('TC-DEVEX-013 设备模板详情查询', function () {
    it('covers TC-DEVEX-013 with concrete API assertions', async function () {
      expect(deviceTemplateId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.get('/device/template/detail/' + deviceTemplateId);
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data.id || resp.data.ID).to.equal(deviceTemplateId);
      expect(resp.data.name).to.be.a('string').and.not.equal('');
    });
  });

  describe('TC-DEVEX-014 更新设备模板', function () {
    it('covers TC-DEVEX-014 with concrete API assertions', async function () {
      const resp = await apiClient.put('/device/template', {
        id: INVALID_UUID,
        name: '无效模板更新'
      });
      expect(resp.code).to.equal(101001);
      expect(resp.data).to.deep.equal({ sql_error: 'record not found' });
    });
  });

  describe('TC-DEVEX-015 删除设备模板', function () {
    it('covers TC-DEVEX-015 with concrete API assertions', async function () {
      const resp = await apiClient.delete('/device/template/' + INVALID_UUID);
      expect(resp.code).to.equal(101001);
      expect(resp.data).to.deep.equal({ sql_error: 'record not found' });
    });
  });

  // ==================== 模板市场 ====================

  describe('TC-DEVEX-016 模板市场列表查询', function () {
    it('covers TC-DEVEX-016 with concrete API assertions', async function () {
      const resp = await apiClient.get('/device/template/market/list', { page: 1, page_size: 10 });
      expectApiError(resp, 100000, '系统内部错误');
      expect(resp.data.error).to.match(/market|failed|unavailable|http/i);
    });
  });

  describe('TC-DEVEX-017 模板市场详情查询', function () {
    it('covers TC-DEVEX-017 with concrete API assertions', async function () {
      const resp = await apiClient.get('/device/template/market/detail/' + INVALID_UUID);
      expectApiError(resp, 100000, '系统内部错误');
      expect(resp.data.error).to.match(/market|failed|unavailable|http/i);
    });
  });

  describe('TC-DEVEX-018 模板市场登录', function () {
    it('covers TC-DEVEX-018 with concrete API assertions', async function () {
      const resp = await apiClient.post('/device/template/market/login', {
        username: 'invalid_user',
        password: 'invalid_password'
      });
      expect(resp.code).to.equal(100000);
      expect(resp.message).to.match(/login request failed|market service unavailable/i);
      expect(resp.message).to.match(/market|unavailable|http/i);
    });
  });

  describe('TC-DEVEX-019 模板市场发布', function () {
    it('covers TC-DEVEX-019 with concrete API assertions', async function () {
      const resp = await apiClient.post('/device/template/market/publish', {
        template_id: INVALID_UUID
      });
      expectApiError(resp, 100002, "Field 'DeviceConfigID' is required");
    });
  });

  describe('TC-DEVEX-020 模板市场安装', function () {
    it('covers TC-DEVEX-020 with concrete API assertions', async function () {
      const resp = await apiClient.post('/device/template/market/install', {
        market_template_id: INVALID_UUID
      });
      expectApiError(resp, 100002, "Field 'MarketToken' is required");
    });
  });

  // ==================== 设备调试 ====================

  describe('TC-DEVEX-021 设备调试状态查询', function () {
    it('covers TC-DEVEX-021 with concrete API assertions', async function () {
      expect(deviceId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.get('/device/' + deviceId + '/debug/status');
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data.enabled).to.be.a('boolean');
      expect(resp.data.remaining_seconds).to.be.a('number');
      expect(resp.data.config).to.be.an('object');
    });
  });

  describe('TC-DEVEX-022 设备调试日志查询', function () {
    it('covers TC-DEVEX-022 with concrete API assertions', async function () {
      expect(deviceId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.get('/device/' + deviceId + '/debug/logs', { page: 1, page_size: 10 });
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data.total).to.be.a('number');
      expect(resp.data.list).to.be.an('array');
    });
  });

  describe('TC-DEVEX-023 启用设备调试', function () {
    it('covers TC-DEVEX-023 with concrete API assertions', async function () {
      expect(deviceId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.post('/device/' + deviceId + '/debug', {});
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data.enabled).to.equal(true);
      expect(resp.data.remaining_seconds).to.be.a('number').and.greaterThan(0);
      expect(resp.data.config).to.be.an('object');
    });
  });

  // ==================== 设备主题映射 ====================

  describe('TC-DEVEX-024 更新设备主题映射', function () {
    it('covers TC-DEVEX-024 with concrete API assertions', async function () {
      const resp = await apiClient.put('/device/topic-mappings/' + INVALID_UUID, {
        name: '自动化测试映射_更新'
      });
      expectApiError(resp, 100002, 'invalid id');
    });
  });

  describe('TC-DEVEX-025 删除设备主题映射', function () {
    it('covers TC-DEVEX-025 with concrete API assertions', async function () {
      const resp = await apiClient.delete('/device/topic-mappings/' + INVALID_UUID);
      expectApiError(resp, 100002, 'invalid id');
    });
  });

  // ==================== 设备认证 ====================

  describe('TC-DEVEX-026 设备认证', function () {
    it('covers TC-DEVEX-026 with concrete API assertions', async function () {
      const resp = await apiClient.post('/device/auth', {
        device_id: INVALID_UUID,
        token: 'invalid_token'
      });
      expectApiError(resp, 100002, "Field 'TemplateSecret' is required");
    });
  });
});



// asserted: device/group.list seeded-group-visible + name (TC-DEVEX-007)
