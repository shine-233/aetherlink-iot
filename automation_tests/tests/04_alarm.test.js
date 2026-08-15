/**
 * 文件用途：用于验证告警 API 自动化测试。
 * 核心逻辑：通过共享 API 客户端和测试数据访问目标接口，断言响应结构、错误分支或可观察状态。
 * 关键注意事项：接口命中不等同于业务正确；计入证据前需要确认断言覆盖真实状态和前置条件。
 * 重构建议：后续应优先补强负向用例、状态校验和清理路径，而不是扩大无断言冒烟范围。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const testData = require('../lib/test_data');
const seedData = require('../lib/seed_data');

function pickId(record) {
  if (!record || typeof record !== 'object') {
    return null;
  }
  return record.id || record.ID || null;
}

function expectOk(resp) {
  expect(resp).to.be.an('object');
  expect(resp.code).to.equal(200);
}

function expectPagedObject(data) {
  expect(data).to.be.an('object');
  expect(data).to.have.property('total').that.is.a('number').and.is.at.least(0);
  expect(data).to.have.property('list').that.is.an('array');
  expect(data.list.length).to.be.at.most(data.total);
}

describe('Alarm API module [04_alarm]', function () {
  this.timeout(30000);

  let alarmConfigId = null;
  let alarmHistoryId = null;
  let deviceId = null;
  let deviceSeed = null;
  let historySeed = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 04_alarm.test.js; unified verification requires a healthy API service');
    }

    await apiClient.login('tenant_admin');

    deviceSeed = await seedData.ensureDevice('tenant_admin');
    deviceId = deviceSeed.id;
    expect(deviceId).to.be.a('string').and.not.equal('');

    historySeed = await seedData.ensureSceneAlarmHistory('tenant_admin');
    expect(historySeed.blocked, historySeed.reason || 'alarm history fixture').to.equal(false);
    alarmHistoryId = historySeed.id;
    expect(alarmHistoryId).to.be.a('string').and.not.equal('');
  });

  after(async function () {
    if (alarmConfigId) {
      await apiClient.delete('/alarm/config/' + alarmConfigId);
    }
    if (historySeed && typeof historySeed.cleanup === 'function') {
      await historySeed.cleanup();
    }
    if (deviceSeed && typeof deviceSeed.cleanup === 'function') {
      await deviceSeed.cleanup();
    }
    apiClient.clearAllTokens();
  });

  describe('TC-ALARM-001 alarm config list', function () {
    it('returns a paged alarm config payload', async function () {
      const resp = await apiClient.get('/alarm/config', { page: 1, page_size: 10 });

      expectOk(resp);
      expectPagedObject(resp.data);
    });
  });

  describe('TC-ALARM-002 create alarm config', function () {
    it('creates an alarm config through the public API', async function () {
      const createReq = testData.getCreateAlarmConfigReq();
      const resp = await apiClient.post('/alarm/config', createReq);

      expectOk(resp);
      expect(resp.data).to.be.an('object');
      alarmConfigId = pickId(resp.data);
      expect(alarmConfigId).to.be.a('string').and.not.equal('');
      expect(resp.data.name).to.equal(createReq.name);
      expect(resp.data.alarm_level).to.equal(createReq.alarm_level);
      expect(resp.data.enabled).to.equal(createReq.enabled);
    });
  });

  describe('TC-ALARM-003 update alarm config', function () {
    it('updates the alarm config created by this test run', async function () {
      expect(alarmConfigId).to.be.a('string').and.not.equal('');
      const updatedName = 'automation_alarm_updated_' + Date.now();

      const resp = await apiClient.put('/alarm/config', {
        id: alarmConfigId,
        name: updatedName,
        enabled: 'Y'
      });

      expectOk(resp);
      expect(resp.data).to.be.an('object');
      expect(pickId(resp.data)).to.equal(alarmConfigId);
      expect(resp.data.name).to.equal(updatedName);
      expect(resp.data.enabled).to.equal('Y');
    });
  });

  describe('TC-ALARM-004 alarm info list', function () {
    it('returns a paged alarm info payload', async function () {
      const resp = await apiClient.get('/alarm/info', { page: 1, page_size: 10 });

      expectOk(resp);
      expectPagedObject(resp.data);
    });
  });

  describe('TC-ALARM-005 alarm history list', function () {
    it('returns the scene-seeded alarm history row', async function () {
      const resp = await apiClient.get('/alarm/info/history', { page: 1, page_size: 10 });

      expectOk(resp);
      expectPagedObject(resp.data);

      const list = resp.data.list;
      const seededRow = list.find(row => pickId(row) === alarmHistoryId);
      expect(seededRow, 'scene-seeded alarm history must be visible in the first page').to.be.an('object');
      expect(pickId(seededRow)).to.equal(alarmHistoryId);
    });
  });

  describe('TC-ALARM-006 alarm history time filter', function () {
    it('accepts a bounded history time range', async function () {
      const end = new Date();
      const start = new Date(end.getTime() - 7 * 24 * 60 * 60 * 1000);
      const startMs = start.getTime();
      const endMs = end.getTime();
      const resp = await apiClient.get('/alarm/info/history', {
        page: 1,
        page_size: 10,
        start_time: start.toISOString(),
        end_time: end.toISOString()
      });

      expectOk(resp);
      expectPagedObject(resp.data);

      const list = resp.data.list;
      list.forEach(row => {
        const ts = row.create_at || row.createTime || row.alarm_time || row.ts || row.created_at || row.time || row.alarm_at;
        expect(ts, 'history row must expose a timestamp for range check').to.not.equal(undefined);
        const tsMs = new Date(ts).getTime();
        expect(tsMs, 'row timestamp must be parseable as a date').to.be.a('number');
        expect(tsMs).to.be.at.least(startMs);
        expect(tsMs).to.be.at.most(endMs);
      });
    });
  });

  describe('TC-ALARM-007 alarm history status filter', function () {
    it('accepts an alarm status filter', async function () {
      const resp = await apiClient.get('/alarm/info/history', {
        page: 1,
        page_size: 10,
        alarm_status: 'H'
      });

      expectOk(resp);
      expectPagedObject(resp.data);

      const list = resp.data.list;
      list.forEach(row => {
        expect(row, 'history row must expose alarm_status for status filter check').to.include.keys('alarm_status');
        expect(row.alarm_status).to.equal('H');
      });
    });
  });

  describe('TC-ALARM-008 acknowledge alarm history', function () {
    it('acknowledges the scene-seeded history row', async function () {
      const resp = await apiClient.put('/alarm/info/history/' + alarmHistoryId + '/acknowledge');
      expectOk(resp);
      expect(resp.data).to.be.an('object');
      expect(pickId(resp.data)).to.equal(alarmHistoryId);

      // 回读验证：确认 acknowledge 状态已持久化
      const readbackResp = await apiClient.get('/alarm/info/history', {
        page: 1,
        page_size: 50
      });
      expectOk(readbackResp);
      expectPagedObject(readbackResp.data);
      const row = readbackResp.data.list.find(item => pickId(item) === alarmHistoryId);
      expect(row, 'acknowledged history row must remain visible in list').to.be.an('object');
      // acknowledge 状态存储在 remark 字段（JSON 字符串），解析后断言 acknowledged === true
      expect(row.remark, 'history row must have a remark field with acknowledge state').to.be.a('string');
      let remarkObj;
      try { remarkObj = JSON.parse(row.remark); } catch (e) {
        expect.fail('remark must be valid JSON after acknowledge, got: ' + row.remark);
      }
      expect(remarkObj.acknowledged, 'remark.acknowledged must be true after acknowledge').to.equal(true);
      expect(remarkObj.acknowledged_at, 'remark.acknowledged_at must be set after acknowledge').to.be.a('string').and.not.equal('');
      expect(remarkObj.acknowledged_by, 'remark.acknowledged_by must be set after acknowledge').to.be.a('string').and.not.equal('');
    });
  });

  describe('TC-ALARM-009 reset alarm history', function () {
    it('resets the scene-seeded history row', async function () {
      const resp = await apiClient.put('/alarm/info/history/' + alarmHistoryId + '/reset');
      expectOk(resp);
      expect(resp.data).to.be.an('object');
      expect(pickId(resp.data)).to.equal(alarmHistoryId);

      // 回读验证：确认 reset 状态已持久化
      const readbackResp = await apiClient.get('/alarm/info/history', {
        page: 1,
        page_size: 50
      });
      expectOk(readbackResp);
      expectPagedObject(readbackResp.data);
      const row = readbackResp.data.list.find(item => pickId(item) === alarmHistoryId);
      expect(row, 'reset history row must remain visible in list').to.be.an('object');
      // reset 状态存储在 remark 字段（JSON 字符串），解析后断言 reset === true
      expect(row.remark, 'history row must have a remark field with reset state').to.be.a('string');
      let remarkObj;
      try { remarkObj = JSON.parse(row.remark); } catch (e) {
        expect.fail('remark must be valid JSON after reset, got: ' + row.remark);
      }
      expect(remarkObj.reset, 'remark.reset must be true after reset').to.equal(true);
      expect(remarkObj.reset_at, 'remark.reset_at must be set after reset').to.be.a('string').and.not.equal('');
      expect(remarkObj.reset_by, 'remark.reset_by must be set after reset').to.be.a('string').and.not.equal('');
    });
  });

  describe('TC-ALARM-010 device alarm status', function () {
    it('returns the seeded device alarm status', async function () {
      const resp = await apiClient.get('/alarm/info/history/device', { device_id: deviceId });
      expectOk(resp);
      expect(resp.data).to.be.an('object');
      expect(resp.data.alarm).to.be.a('boolean');

      // 交叉校验：alarm 布尔值必须与该设备的告警历史记录一致
      // 若 alarm===true，则该设备在历史列表中必须存在至少一条告警记录；
      // 若 alarm===false 且无告警历史记录，亦视为一致（新鲜种子设备无告警属合理状态）
      const historyResp = await apiClient.get('/alarm/info/history', {
        page: 1,
        page_size: 50
      });
      expectOk(historyResp);
      expectPagedObject(historyResp.data);
      const deviceRows = historyResp.data.list.filter(
        row => (row.device_id || row.deviceId || row.device_ID) === deviceId
      );
      if (resp.data.alarm === true) {
        // alarm=true 暗示设备存在告警历史记录；若不存在则布尔值与持久化状态不一致，需排查
        expect(
          deviceRows.length,
          'alarm=true 必须对应设备在历史列表中存在告警记录，否则布尔值与持久化状态不一致'
        ).to.be.at.least(1);
      } else {
        expect(
          deviceRows.length,
          'alarm=false must not coexist with an active alarm-history row for the seeded device'
        ).to.equal(0);
      }
      // alarm===false 时，设备在当前查询页无活跃告警历史，对新鲜种子设备属合理状态
    });
  });

  describe('TC-ALARM-011 device alarm counts', function () {
    it('returns alarm counts for devices in the tenant scope', async function () {
      const resp = await apiClient.get('/alarm/device/counts');

      expectOk(resp);
      expect(resp.data).to.be.an('object');
      expect(resp.data.alarm_device_total).to.be.a('number');
      expect(resp.data.alarm_device_total).to.be.at.least(0);
    });
  });
});
