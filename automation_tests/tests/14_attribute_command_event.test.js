/**
 * 文件用途：用于验证属性、命令与事件 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectCountList(data) {
  expect(data).to.be.an('object');
  expect(data).to.have.property('count').that.is.a('number').and.is.at.least(0);
  expect(data).to.have.property('list');
  if (data.list === null) {
    expect(data.count).to.equal(0);
    return;
  }
  expect(data.list).to.be.an('array');
  expect(data.list.length).to.be.at.most(data.count);
}

describe('Attribute, command, and event API module [14_attribute_command_event]', function () {
  this.timeout(30000);

  const fakeId = '00000000-0000-0000-0000-000000000000';
  let deviceId = null;
  let seededDevice = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 14_attribute_command_event.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login('tenant_admin');
    seededDevice = await seedData.ensureDevice('tenant_admin');
    deviceId = seededDevice.id;
    expect(deviceId).to.be.a('string').and.not.equal('');
  });

  after(async function () {
    try {
      if (seededDevice && seededDevice.cleanup) {
        await seededDevice.cleanup();
      }
    } finally {
      apiClient.clearAllTokens();
    }
  });

  it('returns the current nullable attribute payload for the tenant device', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get('/attribute/datas/' + deviceId, {}, 'tenant_admin');

    expectOk(resp);
    if (resp.data === null) {
      return;
    }
    expect(resp.data).to.be.an('array');
    resp.data.forEach(row => {
      expect(row).to.be.an('object');
      expect(row.device_id).to.equal(deviceId);
      expect(row.key).to.be.a('string').and.not.equal('');
      expect(row).to.include.keys('id', 'ts', 'value');
    });
  });

  it('returns attribute set logs with count and list fields', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get(
      '/attribute/datas/set/logs',
      {
        device_id: deviceId,
        page: 1,
        page_size: 10
      },
      'tenant_admin'
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.count).to.be.a('number');
    // 后端对空结果返回 list:null（平台空列表约定）；仅在 count>0 时要求 array。
    if (resp.data.count === 0) {
      expect(resp.data.list, 'empty set-logs list must be null per platform convention').to.equal(null);
    } else {
      expect(resp.data.list).to.be.an('array');
      expect(resp.data.list.length).to.be.at.most(resp.data.count);
      resp.data.list.forEach(row => {
        expect(row).to.be.an('object');
        expect(row.device_id).to.equal(deviceId);
        expect(row).to.include.keys('id', 'created_at');
      });
    }
  });

  it('returns record-not-found for invalid attribute deletion', async function () {
    const resp = await apiClient.delete('/attribute/datas/' + fakeId, {}, 'tenant_admin');

    expect(resp).to.be.an('object');
    expect(resp.code).to.equal(100000);
    expect(resp.message).to.equal('record not found');
  });

  it('returns command set logs for the tenant device', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get(
      '/command/datas/set/logs',
      {
        device_id: deviceId,
        page: 1,
        page_size: 10
      },
      'tenant_admin'
    );

    expectOk(resp);
    expect(resp.data).to.be.an('object');
    expect(resp.data.list).to.be.an('array');
    expect(resp.data.total).to.be.a('number');
    expect(resp.data.list.length).to.be.at.most(resp.data.total);
    resp.data.list.forEach(row => {
      expect(row.device_id).to.equal(deviceId);
      expect(row).to.include.keys('id', 'message_id', 'status', 'created_at');
    });
  });

  it('returns command definitions as an array for the tenant device', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get('/command/datas/' + deviceId, {}, 'tenant_admin');

    expectOk(resp);
    expect(resp.data).to.be.an('array');
    resp.data.forEach(row => {
      expect(row).to.be.an('object');
      // GET /command/datas/:id returns the device-template command
      // definitions used to build the command picker.  Current command
      // values/logs are separate endpoints and must not be asserted here.
      expect(row).to.include.keys('data_name', 'data_identifier', 'params', 'description');
      expect(row.data_identifier).to.be.a('string').and.not.equal('');
    });
  });

  it('returns event data with count and list for the tenant device', async function () {
    expect(deviceId).to.be.a('string').and.not.equal('');

    const resp = await apiClient.get(
      '/event/datas',
      {
        device_id: deviceId,
        page: 1,
        page_size: 10
      },
      'tenant_admin'
    );

    expectOk(resp);
    expectCountList(resp.data);
    (resp.data.list || []).forEach(row => {
      expect(row.device_id).to.equal(deviceId);
      expect(row).to.include.keys('id', 'key', 'created_at');
    });
  });
});
