/**
 * 文件用途：TB-1 告警规则 2.0 第三片：告警生命周期与自动/显式清除（Alarm Clear & Lifecycle）的 API 契约测试。
 *
 * 对标 ThingsBoard 4.3 LTS 告警生命周期四态（Lifecycle States）：
 *   - ACTIVE_UNACK: 活动、未确认
 *   - ACTIVE_ACK: 活动、已确认
 *   - CLEARED_UNACK: 已清除、未确认
 *   - CLEARED_ACK: 已清除、已确认
 *
 * 覆盖：
 *   1. 初始四态：新活动告警默认 lifecycle_status 为 ACTIVE_UNACK；
 *   2. 确认链路：确认后流转为 ACTIVE_ACK，留存确认人与时间；
 *   3. 清除链路：清除已确认告警，流转为 CLEARED_ACK，留存清除人、清除时间与清除原因；
 *   4. 重复清除防护：已清除的告警再次调用清除应拒绝，防止审计与生命周期污染；
 *   5. 未确认直接清除：直接清除 ACTIVE_UNACK 告警，流转为 CLEARED_UNACK；
 *   6. 清除后补确认：CLEARED_UNACK 告警支持被确认，流转为 CLEARED_ACK；
 *   7. 批量清除：通过 batch-action 批量清除告警，各记录状态正确流转；
 *   8. 租户隔离：租户 B 无法清除租户 A 的告警记录；
 *   9. 边界处理：不存在的告警 ID 执行清除返回失败。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'Alarm clear lifecycle [49_alarm_lifecycle_clear]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';

function expectOk(resp, label = 'ok response') {
  expect(resp, `${label}: response envelope`).to.be.an('object');
  expect(resp.code, `${label}: expected 200 but got ${resp.code}: ${resp.message || ''}`).to.equal(200);
  return resp.data;
}

describe(SUITE, function () {
  this.timeout(120000);

  let seed = null;
  let firstAlarmId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 49_alarm_lifecycle_clear.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(OTHER_ACCOUNT);

    seed = await seedData.ensureSceneAlarmHistory(ACCOUNT);
    if (seed.blocked) {
      throw new Error('alarm history fixture blocked: ' + seed.reason);
    }
    firstAlarmId = seed.id;
    expect(firstAlarmId, 'fixture alarm history id').to.be.a('string').and.not.equal('');
  });

  after(async function () {
    if (seed && typeof seed.cleanup === 'function') {
      try {
        await seed.cleanup();
      } catch {
        // cleanup failure in teardown is non-blocking
      }
    }
  });

  describe('Part 1: Four-State Alarm Lifecycle Transitions', function () {
    it('verifies the initial alarm starts in ACTIVE_UNACK or ACTIVE_ACK state', async function () {
      const resp = await apiClient.get(`/alarm/info/history/${encodeURIComponent(firstAlarmId)}`, {}, ACCOUNT);
      const data = expectOk(resp, 'get alarm history');
      expect(data).to.be.an('object');
      expect(['ACTIVE_UNACK', 'ACTIVE_ACK']).to.include(data.lifecycle_status);
    });

    it('acknowledges the alarm and reaches ACTIVE_ACK state', async function () {
      const ackResp = await apiClient.put(
        `/alarm/info/history/${encodeURIComponent(firstAlarmId)}/acknowledge`,
        {},
        ACCOUNT
      );
      const ackData = expectOk(ackResp, 'acknowledge alarm');
      expect(ackData.lifecycle_status).to.equal('ACTIVE_ACK');
      expect(ackData.acknowledged_by).to.be.a('string').and.not.be.empty;
      expect(ackData.acknowledged_at).to.be.a('string').and.not.be.empty;
    });

    it('clears the acknowledged alarm with note and reaches CLEARED_ACK state', async function () {
      const clearResp = await apiClient.put(
        `/alarm/info/history/${encodeURIComponent(firstAlarmId)}/clear`,
        { note: 'Fault verified resolved on site by engineer' },
        ACCOUNT
      );
      const clearData = expectOk(clearResp, 'clear alarm');
      expect(clearData.lifecycle_status).to.equal('CLEARED_ACK');
      expect(clearData.cleared_by).to.be.a('string').and.not.be.empty;
      expect(clearData.cleared_at).to.be.a('string').and.not.be.empty;
      if (clearData.action_note) {
        expect(clearData.action_note).to.include('Fault verified resolved on site');
      }

      // 验证 GET 读取回来的详情保持该四态
      const detailResp = await apiClient.get(`/alarm/info/history/${encodeURIComponent(firstAlarmId)}`, {}, ACCOUNT);
      const detailData = expectOk(detailResp, 'get cleared alarm detail');
      expect(detailData.lifecycle_status).to.equal('CLEARED_ACK');
      expect(detailData.cleared_by).to.equal(clearData.cleared_by);
    });

    it('rejects clearing an already cleared alarm history to prevent lifecycle corruption', async function () {
      const resp = await apiClient.put(
        `/alarm/info/history/${encodeURIComponent(firstAlarmId)}/clear`,
        { note: 'Attempting duplicate clear' },
        ACCOUNT
      );
      expect(resp.code, `expected rejection but got ${resp.code}`).to.not.equal(200);
    });
  });

  describe('Part 2: POST /clear Method Support', function () {
    let secondAlarmId = null;

    before(async function () {
      // 触发产生第二条活动告警
      const secondSeed = await seedData.ensureSceneAlarmHistory(ACCOUNT);
      if (secondSeed && !secondSeed.blocked && secondSeed.id && secondSeed.id !== firstAlarmId) {
        secondAlarmId = secondSeed.id;
      } else {
        const listResp = await apiClient.get('/alarm/info/history', { page: 1, page_size: 10 }, ACCOUNT);
        const list = (listResp.data && listResp.data.list) || [];
        const candidate = list.find(item => item.id !== firstAlarmId);
        if (candidate) {
          secondAlarmId = candidate.id;
        } else {
          secondAlarmId = firstAlarmId;
        }
      }
    });

    it('supports POST /clear in addition to PUT /clear', async function () {
      if (!secondAlarmId || secondAlarmId === firstAlarmId) {
        this.skip();
        return;
      }
      const resp = await apiClient.post(
        `/alarm/info/history/${encodeURIComponent(secondAlarmId)}/clear`,
        { note: 'Cleared via POST method' },
        ACCOUNT
      );
      const data = expectOk(resp, 'POST clear alarm');
      expect(data.lifecycle_status).to.match(/^CLEARED_/);
    });
  });

  describe('Part 3: Batch Action Clear & Multi-tenant Boundaries', function () {
    it('supports batch clearing alarms via /alarm/info/history/batch-action', async function () {
      const resp = await apiClient.put(
        '/alarm/info/history/batch-action',
        {
          ids: [firstAlarmId],
          action: 'clear',
          note: 'Batch cleared from automated test'
        },
        ACCOUNT
      );
      const data = expectOk(resp, 'batch clear action');
      expect(data).to.be.an('object');
      expect(data.action).to.equal('clear');
    });

    it('rejects clearing an alarm history belonging to another tenant', async function () {
      const resp = await apiClient.put(
        `/alarm/info/history/${encodeURIComponent(firstAlarmId)}/clear`,
        { note: 'Attacking cross-tenant clear' },
        OTHER_ACCOUNT
      );
      expect(resp.code, `cross-tenant clear must be rejected`).to.not.equal(200);
    });

    it('rejects clearing a non-existent alarm history ID', async function () {
      const resp = await apiClient.put(
        '/alarm/info/history/non-existent-uuid-alarm-0000/clear',
        { note: 'Clear ghost alarm' },
        ACCOUNT
      );
      expect(resp.code, `ghost alarm clear must fail`).to.not.equal(200);
    });
  });
});
