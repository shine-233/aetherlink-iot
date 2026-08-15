/**
 * 文件用途：用于验证数据脚本 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');

describe('数据处理脚本模块 [13_data_script]', function () {
  this.timeout(30000);

  const accountKey = 'tenant_admin';
  const scriptType = 'A';
  const scriptContent = 'function encodeInp(msg, topic) return msg .. ":ok:" .. topic end';
  let dataScriptId = null;
  let deviceConfigId = null;

  function getEntityId(resp) {
    return resp && resp.data ? (resp.data.id || (resp.data.ID ? String(resp.data.ID) : null)) : null;
  }

  function getPagedList(resp) {
    expect(resp.code).to.equal(200);
    expect(resp.data).to.be.an('object');
    expect(resp.data.total).to.be.a('number');
    expect(resp.data.list).to.be.an('array');
    return resp.data.list;
  }

  async function findCreatedScript() {
    const resp = await apiClient.get('/data_script', {
      page: 1,
      page_size: 100,
      device_config_id: deviceConfigId,
      script_type: scriptType
    }, accountKey);
    const list = getPagedList(resp);
    return list.find((item) => item.id === dataScriptId) || null;
  }

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 13_data_script.test.js; unified verification requires a healthy API service');
    }
    await apiClient.login(accountKey);

    const cfgResp = await apiClient.post('/device_config', {
      name: '自动化数据脚本配置_' + Date.now(),
      device_type: '1',
      protocol_type: 'MQTT',
      voucher_type: 'ACCESSTOKEN',
      device_conn_type: 'A',
      protocol_config: '{}'
    }, accountKey);
    expect(cfgResp.code).to.equal(200);
    deviceConfigId = getEntityId(cfgResp);
    expect(deviceConfigId).to.be.a('string').and.not.equal('');

    const scriptResp = await apiClient.post('/data_script', {
      name: 'seed_data_script_' + Date.now(),
      device_config_id: deviceConfigId,
      description: 'seeded by automation setup',
      content: scriptContent,
      script_type: scriptType,
      last_analog_input: 'raw-input'
    }, accountKey);
    expect(scriptResp.code).to.equal(200);
    dataScriptId = getEntityId(scriptResp);
    expect(dataScriptId).to.be.a('string').and.not.equal('');
  });

  describe('TC-DSCRIPT-001 脚本分页查询', function () {
    it('covers TC-DSCRIPT-001 with concrete API assertions', async function () {
      const resp = await apiClient.get('/data_script', {
        page: 1,
        page_size: 100,
        device_config_id: deviceConfigId
      }, accountKey);
      const list = getPagedList(resp);
      expect(list.length).to.be.greaterThan(0);
      const seededRow = list.find((item) => item.id === dataScriptId);
      expect(seededRow, 'seeded data script must be visible in the paged list').to.be.an('object');
      expect(seededRow.device_config_id).to.equal(deviceConfigId);
      expect(seededRow.script_type).to.equal(scriptType);
    });
  });

  describe('TC-DSCRIPT-002 创建脚本', function () {
    it('covers TC-DSCRIPT-002 with concrete API assertions', async function () {
      const name = '自动化测试脚本_' + Date.now();
      const resp = await apiClient.post('/data_script', {
        name,
        device_config_id: deviceConfigId,
        description: '由自动化测试创建',
        content: scriptContent,
        script_type: scriptType,
        last_analog_input: 'raw-input'
      }, accountKey);
      expect(resp.code).to.equal(200);
      expect(resp.data).to.be.an('object');
      dataScriptId = getEntityId(resp);
      expect(dataScriptId).to.be.a('string').and.not.equal('');

      const created = await findCreatedScript();
      expect(created).to.be.an('object');
      expect(created.name).to.equal(name);
      expect(created.device_config_id).to.equal(deviceConfigId);
      expect(created.script_type).to.equal(scriptType);
      expect(created.enable_flag).to.equal('N');
    });
  });

  describe('TC-DSCRIPT-003 更新脚本', function () {
    it('应成功更新脚本并可在列表中查回新名称', async function () {
      expect(dataScriptId).to.be.a('string').and.not.equal('');
      const newName = '自动化测试脚本_更新_' + Date.now();
      const resp = await apiClient.put('/data_script', {
        id: dataScriptId,
        name: newName,
        device_config_id: deviceConfigId,
        content: scriptContent,
        script_type: scriptType,
        last_analog_input: 'raw-input-updated',
        description: '由自动化测试更新'
      }, accountKey);
      expect(resp.code).to.equal(200);

      const updated = await findCreatedScript();
      expect(updated).to.be.an('object');
      expect(updated.name).to.equal(newName);
      expect(updated.last_analog_input).to.equal('raw-input-updated');
    });
  });

  describe('TC-DSCRIPT-004 测试脚本', function () {
    it('应执行脚本并返回处理后的模拟输入', async function () {
      const resp = await apiClient.post('/data_script/quiz', {
        content: scriptContent,
        last_analog_input: 'raw-input',
        topic: 'topic-1'
      }, accountKey);
      expect(resp.code).to.equal(200);
      expect(resp.data).to.equal('raw-input:ok:topic-1');
    });
  });

  describe('TC-DSCRIPT-005 启用/禁用脚本', function () {
    it('covers TC-DSCRIPT-005 with concrete API assertions', async function () {
      expect(dataScriptId).to.be.a('string').and.not.equal('');
      const enableResp = await apiClient.put('/data_script/enable', {
        id: dataScriptId,
        enable_flag: 'Y'
      }, accountKey);
      expect(enableResp.code).to.equal(200);

      let current = await findCreatedScript();
      expect(current).to.be.an('object');
      expect(current.enable_flag).to.equal('Y');

      const disableResp = await apiClient.put('/data_script/enable', {
        id: dataScriptId,
        enable_flag: 'N'
      }, accountKey);
      expect(disableResp.code).to.equal(200);

      current = await findCreatedScript();
      expect(current).to.be.an('object');
      expect(current.enable_flag).to.equal('N');
    });
  });

  describe('TC-DSCRIPT-006 删除脚本', function () {
    it('covers TC-DSCRIPT-006 with concrete API assertions', async function () {
      expect(dataScriptId).to.be.a('string').and.not.equal('');
      const deletedId = dataScriptId;
      const resp = await apiClient.delete('/data_script/' + deletedId, {}, accountKey);
      expect(resp.code).to.equal(200);
      dataScriptId = null;

      const listResp = await apiClient.get('/data_script', {
        page: 1,
        page_size: 100,
        device_config_id: deviceConfigId,
        script_type: scriptType
      }, accountKey);
      const list = getPagedList(listResp);
      expect(list.some((item) => item.id === deletedId)).to.equal(false);
    });
  });

  after(async function () {
    if (dataScriptId) {
      try {
        await apiClient.delete('/data_script/' + dataScriptId, {}, accountKey);
      } catch (e) { /* ignore */ }
    }
    if (deviceConfigId) {
      try {
        await apiClient.delete('/device_config/' + deviceConfigId, {}, accountKey);
      } catch (e) { /* ignore */ }
    }
  });
});



// asserted: data_script.list seeded-script-visible + device_config_id + script_type (TC-DSCRIPT-001)
