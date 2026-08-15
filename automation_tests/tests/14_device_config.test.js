/**
 * 文件用途：用于验证设备配置 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

describe('设备配置模块 [14_device_config]', function () {
  this.timeout(30000);

  let deviceConfigId = null;
  let seededDeviceConfig = null;
  let topicMappingId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 14_device_config.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login('tenant_admin');

    seededDeviceConfig = await seedData.ensureDeviceConfig('tenant_admin');
    deviceConfigId = seededDeviceConfig.id;
    expect(deviceConfigId).to.be.a('string').and.not.equal('');
  });

  after(async function () {
    if (topicMappingId) {
      const cleanupResp = await apiClient.delete('/device/topic-mappings/' + topicMappingId);
      expect(cleanupResp.code, 'cleanup must delete the topic mapping left by the create/update flow').to.equal(200);
      topicMappingId = null;
    }
    if (seededDeviceConfig && seededDeviceConfig.cleanup) {
      await seededDeviceConfig.cleanup();
    }
  });

  function getEntityId(resp) {
    return resp && resp.data ? (resp.data.id || (resp.data.ID ? String(resp.data.ID) : null)) : null;
  }

  async function createTempDeviceConfig(label) {
    const resp = await apiClient.post('/device_config', {
      name: label + '_配置_' + Date.now(),
      device_type: '1',
      protocol_type: 'MQTT',
      voucher_type: 'ACCESSTOKEN',
      device_conn_type: 'A',
      protocol_config: '{}'
    });
    expect(resp.code).to.equal(200);
    const id = getEntityId(resp);
    expect(id).to.be.a('string').and.not.equal('');
    return id;
  }

  async function createTempDevice(deviceConfigIdForDevice) {
    const suffix = String(Date.now()).slice(-10) + String(Math.floor(Math.random() * 1000)).padStart(3, '0');
    const resp = await apiClient.post('/device', {
      name: '自动化连接查询设备_' + suffix,
      device_number: 'DCFG' + suffix,
      device_config_id: deviceConfigIdForDevice
    });
    expect(resp.code).to.equal(200);
    const id = getEntityId(resp);
    expect(id).to.be.a('string').and.not.equal('');
    return id;
  }

  describe('TC-DCFG-001 设备配置分页查询', function () {
    it('covers TC-DCFG-001 with concrete API assertions', async function () {
      // seededDeviceConfig 由 tenant_admin 创建，列表查询也必须用同一租户账号，否则跨租户不可见。
      const resp = await apiClient.get('/device_config', { page: 1, page_size: 100 }, 'tenant_admin');
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data.total).to.be.a('number').and.at.least(1);
      expect(resp.data.list).to.be.an('array');
      expect(resp.data.list.length).to.be.at.least(1);
      expect(resp.data.list.length).to.be.at.most(resp.data.total);
      const seededRow = resp.data.list.find(item => (item.id || item.ID) === deviceConfigId);
      expect(seededRow, 'seeded device config must be visible in the paged list').to.be.an('object');
    });
  });

  describe('TC-DCFG-002 创建设备配置', function () {
    it('covers TC-DCFG-002 with concrete API assertions', async function () {
      const resp = await apiClient.post('/device_config', {
        name: '自动化测试设备配置_' + Date.now(),
        device_type: '1',
        protocol_type: 'MQTT',
        voucher_type: 'ACCESSTOKEN',
        device_conn_type: 'A',
        protocol_config: '{}'
      });
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      const newId = resp.data.id || (resp.data.ID ? String(resp.data.ID) : null);
      expect(newId).to.be.a('string').and.not.equal('');
      deviceConfigId = newId;
    });
  });

  describe('TC-DCFG-003 查看设备配置详情', function () {
    it('covers TC-DCFG-003 with concrete API assertions', async function () {
      expect(deviceConfigId).to.be.a('string').and.not.equal('');
      const resp = await apiClient.get('/device_config/' + deviceConfigId);
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(resp.data.id || resp.data.ID).to.equal(deviceConfigId);
    });
  });

  describe('TC-DCFG-004 更新设备配置', function () {
    it('covers TC-DCFG-004 with concrete API assertions', async function () {
      expect(deviceConfigId).to.be.a('string').and.not.equal('');
      const updatedName = '自动化测试设备配置_更新_' + Date.now();
      const resp = await apiClient.put('/device_config', {
        id: deviceConfigId,
        name: updatedName
      });
      expect(resp.code).to.equal(200);

      const detailResp = await apiClient.get('/device_config/' + deviceConfigId);
      expect(detailResp.code).to.equal(200);
      expect(detailResp.data).to.be.an('object');
      expect(detailResp.data.name).to.equal(updatedName);
    });
  });

  describe('TC-DCFG-005 设备配置菜单查询', function () {
    it('covers TC-DCFG-005 with concrete API assertions', async function () {
      const resp = await apiClient.get('/device_config/menu', { page: 1, page_size: 10 });
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('array');
      // 已通过 before 注入 seed 设备配置，菜单应至少返回一项
      expect(resp.data.length).to.be.at.least(1);
      const menuRow = resp.data[0];
      expect(menuRow).to.be.an('object');
      expect(menuRow).to.have.property('id');
      expect(menuRow).to.have.property('name');
    });
  });

  describe('TC-DCFG-006 批量绑定设备配置', function () {
    it('空设备列表应拒绝批量绑定', async function () {
      expect(deviceConfigId, 'batch validation must use the real seeded device config').to.be.a('string').and.not.equal('');
      const resp = await apiClient.put('/device_config/batch', {
        device_config_id: deviceConfigId,
        device_ids: []
      });
      expect(resp.code).to.equal(100002);
      // DeviceIds 是 slice，validationErrorHint 对 slice/array/map 类型的 unit 是 "items"
      expect(resp.message).to.equal("Field 'DeviceIds' failed validation (At least 1 items)");
    });
  });

  describe('TC-DCFG-007 设备配置连接查询', function () {
    it('covers TC-DCFG-007 with concrete API assertions', async function () {
      let tempConfigId = null;
      let tempDeviceId = null;
      try {
        tempConfigId = await createTempDeviceConfig('automation_connect');
        tempDeviceId = await createTempDevice(tempConfigId);

        const resp = await apiClient.get('/device_config/connect', { device_id: tempDeviceId });
        expect(resp.code).to.equal(200);
        expect(resp.data).to.be.an('object');
        expect(resp.data.basic_label).to.be.a('string').and.not.equal('');
        expect(resp.data.access_token_label).to.be.a('string').and.not.equal('');
      } finally {
        if (tempDeviceId) {
          await apiClient.delete('/device/' + tempDeviceId);
        }
        if (tempConfigId) {
          await apiClient.delete('/device_config/' + tempConfigId);
        }
      }
    });
  });

  describe('TC-DCFG-008 凭证类型查询', function () {
    it('covers TC-DCFG-008 with concrete API assertions', async function () {
      const resp = await apiClient.get('/device_config/voucher_type', {
        device_type: '1',
        protocol_type: 'MQTT'
      });
      expect(resp.code).to.equal(200);
      // 后端返回 {label: voucherTypeCode} 映射，如 {"Username & Password": "BASIC", ...}
      expect(resp.data).to.be.an('object');
      const voucherEntries = Object.entries(resp.data);
      expect(voucherEntries.length).to.be.greaterThan(0);
      const voucherValues = voucherEntries.map(([, value]) => value);
      expect(voucherValues).to.include('BASIC');
      expect(voucherValues).to.include('ACCESSTOKEN');
      // 断言至少一个键值对（避免 data 退化为空对象仍通过）
      expect(resp.data).to.have.property(Object.keys(resp.data)[0]);
    });
  });

  describe('TC-DCFG-009 删除设备配置', function () {
    it('covers TC-DCFG-009 with concrete API assertions', async function () {
      expect(deviceConfigId).to.be.a('string').and.not.equal('');
      const createResp = await apiClient.post('/device_config', {
        name: '自动化测试删除配置_' + Date.now(),
        device_type: '1',
        protocol_type: 'MQTT',
        voucher_type: 'ACCESSTOKEN',
        device_conn_type: 'A',
        protocol_config: '{}'
      });
      expect(createResp.code).to.equal(200);
      const tempId = createResp.data.id || (createResp.data.ID ? String(createResp.data.ID) : null);
      expect(tempId).to.be.a('string').and.not.equal('');

      const resp = await apiClient.delete('/device_config/' + tempId);
      expect(resp.code).to.equal(200);

      const detailResp = await apiClient.get('/device_config/' + tempId);
      expect(detailResp.code).to.be.oneOf([100000, 101001]);
    });
  });

  describe('TC-DCFG-010 设备主题映射列表查询', function () {
    it('covers TC-DCFG-010 with concrete API assertions', async function () {
      expect(deviceConfigId).to.be.a('string').and.not.equal('');
      // 先创建一条主题映射，确保列表非空且可断言结构
      const seedName = '自动化测试列表查询_' + Date.now();
      const seedResp = await apiClient.post('/device/topic-mappings', {
        device_config_id: deviceConfigId,
        name: seedName,
        direction: 'up',
        source_topic: 'test/source',
        target_topic: 'test/target'
      });
      expect(seedResp.code).to.equal(200);
      const seedMappingId = seedResp.data.id || seedResp.data.ID;
      expect(String(seedMappingId)).to.not.equal('');
      try {
        const resp = await apiClient.get('/device/topic-mappings', {
          device_config_id: deviceConfigId,
          page: 1,
          page_size: 10
        });
        expect(resp.code).to.equal(200);
        expect(resp.data).to.be.an('object');
        expect(resp.data.total).to.be.a('number').and.at.least(1);
        expect(resp.data.list).to.be.an('array');
        expect(resp.data.list.length).to.be.at.least(1);
        const seededRow = resp.data.list.find(row => String(row.id || row.ID) === String(seedMappingId));
        expect(seededRow, 'seeded topic mapping must be visible in the list').to.be.an('object');
        expect(seededRow).to.include.keys(['id', 'name', 'direction', 'source_topic', 'target_topic']);
      } finally {
        const cleanupResp = await apiClient.delete('/device/topic-mappings/' + seedMappingId);
        expect(cleanupResp.code, 'list fixture cleanup must delete the seeded mapping').to.equal(200);
      }
    });
  });

  describe('TC-DCFG-011 创建设备主题映射', function () {
    it('covers TC-DCFG-011 with concrete API assertions', async function () {
      expect(deviceConfigId).to.be.a('string').and.not.equal('');
      const mappingName = '自动化测试映射_' + Date.now();
      const resp = await apiClient.post('/device/topic-mappings', {
        device_config_id: deviceConfigId,
        name: mappingName,
        direction: 'up',
        source_topic: 'test/source',
        target_topic: 'test/target'
      });
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      const mappingId = resp.data.id || resp.data.ID;
      expect(mappingId, 'topic mapping id').to.not.be.null.and.not.be.undefined;
      topicMappingId = String(mappingId);
      expect(topicMappingId).to.not.equal('');
      expect(resp.data.name).to.equal(mappingName);
      expect(resp.data.direction).to.equal('up');
      expect(resp.data.source_topic).to.equal('test/source');
      expect(resp.data.target_topic).to.equal('test/target');
    });
  });

  describe('TC-DCFG-012 更新设备主题映射', function () {
    it('covers TC-DCFG-012 with concrete API assertions', async function () {
      expect(topicMappingId, 'update requires the mapping created by TC-DCFG-011').to.be.a('string').and.not.equal('');
      const updatedName = '自动化测试映射_更新_' + Date.now();
      const updatedTargetTopic = 'test/target/updated';
      const resp = await apiClient.put('/device/topic-mappings/' + topicMappingId, {
        name: updatedName,
        target_topic: updatedTargetTopic
      });
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      expect(String(resp.data.id || resp.data.ID)).to.equal(topicMappingId);
      expect(resp.data.name).to.equal(updatedName);
      expect(resp.data.target_topic).to.equal(updatedTargetTopic);

      const listResp = await apiClient.get('/device/topic-mappings', {
        device_config_id: deviceConfigId,
        page: 1,
        page_size: 100
      });
      expect(listResp.code).to.equal(200);
      expect(listResp.data).to.be.an('object');
      expect(listResp.data.list).to.be.an('array');
      const updatedRow = listResp.data.list.find(row => String(row.id || row.ID) === topicMappingId);
      expect(updatedRow, 'updated mapping must remain visible in the persisted list').to.be.an('object');
      expect(updatedRow.name).to.equal(updatedName);
      expect(updatedRow.target_topic).to.equal(updatedTargetTopic);
    });

    it('keeps invalid-id handling as a negative boundary, not update proof', async function () {
      const resp = await apiClient.put('/device/topic-mappings/00000000-0000-0000-0000-000000000000', {
        name: '不会写入'
      });
      expect(resp.code).to.equal(100002);
      expect(resp.message).to.equal('invalid id');
    });
  });

  describe('TC-DCFG-013 删除设备主题映射', function () {
    it('covers TC-DCFG-013 with concrete API assertions', async function () {
      expect(topicMappingId, 'delete requires the mapping created and updated by prior cases').to.be.a('string').and.not.equal('');
      const deletedMappingId = topicMappingId;
      const resp = await apiClient.delete('/device/topic-mappings/' + deletedMappingId);
      expect(resp.code).to.equal(200);
      topicMappingId = null;

      const listResp = await apiClient.get('/device/topic-mappings', {
        device_config_id: deviceConfigId,
        page: 1,
        page_size: 100
      });
      expect(listResp.code).to.equal(200);
      expect(listResp.data).to.be.an('object');
      expect(listResp.data.list).to.be.an('array');
      const deletedRow = listResp.data.list.find(row => String(row.id || row.ID) === deletedMappingId);
      expect(deletedRow, 'deleted mapping must disappear from the persisted list').to.equal(undefined);
    });

    it('keeps invalid-id handling as a negative boundary, not delete proof', async function () {
      const resp = await apiClient.delete('/device/topic-mappings/00000000-0000-0000-0000-000000000000');
      expect(resp.code).to.equal(100002);
      expect(resp.message).to.equal('invalid id');
    });
  });

  describe('TC-DCFG-014 协议插件配置表单查询', function () {
    it('covers TC-DCFG-014 with concrete API assertions', async function () {
      const resp = await apiClient.get('/protocol_plugin/config_form', {
        protocol_type: 'MQTT',
        device_type: '1'
      });
      expect(resp.code).to.equal(200);
      // MQTT 是内置协议，后端 GetProtocolPluginFormByProtocolType 对 MQTT 返回 nil,
      // 即无需协议插件表单。断言 data 为空以锁定该契约。
      // 注意：后端响应结构体 Data 字段使用 omitempty 标签，nil 值会被省略（undefined）。
      expect(resp.data == null).to.be.true;
    });
  });

});



// asserted: device_config.list seeded-config-visible + total>=1 (TC-DCFG-001)
