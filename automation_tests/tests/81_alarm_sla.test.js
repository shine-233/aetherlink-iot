/**
 * 文件用途：告警 SLA 计时与超时升级（TB-27，126.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. 配置 SLA：POST /alarm/config 携带 sla_hours 创建并回显；sla_hours=0 关闭（回显 null）；
 *      负数时限参数错误（100002）；
 *   2. 触发写 due_at：场景联动（action_type=30）激活告警配置 → alarm_history 行写入
 *      sla_due_at（从触发时刻起算 sla_hours）且 sla_breached=false；详情接口回读一致；
 *   3. 未启用 SLA（sla_hours=0）触发的告警 sla_due_at 为空；
 *   4. 严格多租户隔离：租户 B 无法查询租户 A 的 SLA 告警详情，配置列表互不可见。
 *
 * 说明：cron 超时升级（标记 sla_breached + 严重度升一档）依赖 5 分钟一轮的后台任务，
 * 活栈契约不等待真实时钟；升级判定逻辑由 backend/internal/service/alarm_sla_test.go 纯函数单测锚定。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const { expectSuccess, expectBusinessError } = require('../lib/response_assertions');
const seedData = require('../lib/seed_data');

const SUITE = 'Alarm SLA timing & escalation [81_alarm_sla]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

describe(SUITE, function () {
  this.timeout(180000);

  const cleanups = [];
  let slaConfigId = null;
  let slaSceneId = null;
  let slaHistoryId = null;
  let plainConfigId = null;
  let plainSceneId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 81_alarm_sla.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  after(async function () {
    for (let i = cleanups.length - 1; i >= 0; i--) {
      try {
        await cleanups[i]();
      } catch (err) {
        console.warn('Cleanup step error:', err.message);
      }
    }
    apiClient.clearAllTokens();
  });

  // Helper：创建带/不带 SLA 时限的告警配置（notify group 留空走默认邮件兜底，不影响断言）。
  async function createAlarmConfig(name, slaHours) {
    const payload = {
      name,
      alarm_level: 'L',
      enabled: 'Y'
    };
    if (slaHours !== undefined) {
      payload.sla_hours = slaHours;
    }
    const resp = await apiClient.post('/alarm/config', payload, TENANT_A);
    expectSuccess(resp);
    return resp.data;
  }

  // Helper：创建指向告警配置的场景联动（action_type '30' = 告警动作，口径同 seed_data.ensureScene）。
  async function createAlarmScene(name, alarmConfigId) {
    const resp = await apiClient.post(
      '/scene',
      {
        name,
        description: 'alarm sla contract seed',
        actions: [
          {
            action_type: '30',
            action_target: alarmConfigId,
            remark: 'trigger alarm config for SLA contract'
          }
        ]
      },
      TENANT_A
    );
    expectSuccess(resp);
    return seedData.pickId(resp.data);
  }

  // Helper：激活场景并在告警历史中轮询定位本次触发的记录。
  async function activateSceneAndFindHistory(sceneId, alarmConfigId) {
    const resp = await apiClient.post('/scene/active/' + sceneId, {}, TENANT_A);
    expectSuccess(resp);

    for (let i = 0; i < 15; i++) {
      const histResp = await apiClient.get(
        '/alarm/info/history',
        { page: 1, page_size: 50 },
        TENANT_A
      );
      if (histResp && histResp.code === 200 && histResp.data && Array.isArray(histResp.data.list)) {
        const row = histResp.data.list.find((item) => item.alarm_config_id === alarmConfigId);
        if (row) {
          return row;
        }
      }
      await new Promise((resolve) => setTimeout(resolve, 600));
    }
    throw new Error('activated scene did not produce a matching alarm history row');
  }

  it('1. POST /alarm/config 创建带 SLA 时限的告警配置并回显 sla_hours', async function () {
    const name = seedData.makeRunLabel('sla_cfg');
    const data = await createAlarmConfig(name, 1);
    slaConfigId = data.id;
    expect(data.sla_hours).to.equal(1);
    cleanups.push(async () => {
      if (slaConfigId) await apiClient.delete('/alarm/config/' + slaConfigId, {}, TENANT_A);
    });
  });

  it('2. POST /alarm/config 负数 SLA 时限返回参数错误', async function () {
    const resp = await apiClient.post(
      '/alarm/config',
      {
        name: seedData.makeRunLabel('sla_cfg_neg'),
        alarm_level: 'L',
        enabled: 'Y',
        sla_hours: -1
      },
      TENANT_A
    );
    expectBusinessError(resp, 100002);
  });

  it('3. 配置 SLA 后场景触发告警，历史行写入 sla_due_at（触发时刻+sla_hours）', async function () {
    slaSceneId = await createAlarmScene(seedData.makeRunLabel('sla_scene'), slaConfigId);
    cleanups.push(async () => {
      if (slaSceneId) await apiClient.delete('/scene/' + slaSceneId, {}, TENANT_A);
    });

    const row = await activateSceneAndFindHistory(slaSceneId, slaConfigId);
    slaHistoryId = row.id;

    expect(row.sla_due_at, 'sla_due_at should be set when SLA configured').to.be.a('string').and.not
      .equal('');
    expect(row.sla_breached, 'fresh alarm must not be breached yet').to.equal(false);

    // due_at 应约等于 create_at + 1h（timestamptz 微秒截断与传输取整留 ±2 分钟容差）。
    const deltaMs = new Date(row.sla_due_at).getTime() - new Date(row.create_at).getTime();
    expect(deltaMs, 'sla_due_at - create_at should be about 1 hour').to.be.within(
      58 * 60 * 1000,
      62 * 60 * 1000
    );
  });

  it('4. GET /alarm/info/history/:id 详情回读 sla_due_at 与 sla_breached', async function () {
    const resp = await apiClient.get('/alarm/info/history/' + encodeURIComponent(slaHistoryId), {}, TENANT_A);
    expectSuccess(resp);
    expect(resp.data.sla_due_at).to.be.a('string').and.not.equal('');
    expect(resp.data.sla_breached).to.equal(false);
    // remark 审计键 sla_escalation 尚未升级，不应出现在展开字段中。
    expect(resp.data.sla_escalation).to.equal(undefined);
  });

  it('5. 未启用 SLA（sla_hours=0）触发的告警 sla_due_at 为空', async function () {
    const data = await createAlarmConfig(seedData.makeRunLabel('sla_cfg_off'), 0);
    plainConfigId = data.id;
    expect(data.sla_hours, '0 collapses to NULL on create').to.equal(null);
    cleanups.push(async () => {
      if (plainConfigId) await apiClient.delete('/alarm/config/' + plainConfigId, {}, TENANT_A);
    });

    plainSceneId = await createAlarmScene(seedData.makeRunLabel('sla_scene_off'), plainConfigId);
    cleanups.push(async () => {
      if (plainSceneId) await apiClient.delete('/scene/' + plainSceneId, {}, TENANT_A);
    });

    const row = await activateSceneAndFindHistory(plainSceneId, plainConfigId);
    expect(row.sla_due_at == null, 'sla_due_at must be empty without SLA').to.equal(true);
  });

  it('6. PUT /alarm/config sla_hours=0 关闭 SLA 后列表回显 null', async function () {
    const resp = await apiClient.put(
      '/alarm/config',
      { id: slaConfigId, sla_hours: 0 },
      TENANT_A
    );
    expectSuccess(resp);

    const listResp = await apiClient.get('/alarm/config', { page: 1, page_size: 50 }, TENANT_A);
    expectSuccess(listResp);
    const hit = (listResp.data.list || []).find((item) => item.id === slaConfigId);
    expect(hit, 'config should appear in tenant list').to.be.an('object');
    expect(hit.sla_hours == null, 'sla_hours=0 must read back as null').to.equal(true);
  });

  it('7. 租户 B 无法查询租户 A 的 SLA 告警详情', async function () {
    const resp = await apiClient.get(
      '/alarm/info/history/' + encodeURIComponent(slaHistoryId),
      {},
      TENANT_B
    );
    expect(resp.code).to.not.equal(200);
  });

  it('8. 租户 B 的告警配置列表不包含租户 A 的 SLA 配置', async function () {
    const listResp = await apiClient.get('/alarm/config', { page: 1, page_size: 50 }, TENANT_B);
    expectSuccess(listResp);
    const leaked = (listResp.data.list || []).find((item) => item.id === slaConfigId);
    expect(leaked, 'tenant A SLA config must not leak to tenant B').to.equal(undefined);
  });
});
