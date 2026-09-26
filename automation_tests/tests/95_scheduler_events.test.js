/**
 * 文件用途：统一调度器（scheduler_events，TB-48，137.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. 聚合列表 GET /api/v1/scheduler/events：只读聚合既有三套调度（scene_automation_timers、
 *      report_schedules、fleet_command_jobs 定时行）+ 注册行为统一事件列表；
 *      每条目携带来源类型 source_type（scene|report|rpc）与来源系统 origin；
 *      支持 source_type 过滤与 from_ms/to_ms 时间窗口过滤；
 *   2. 注册面 CRUD：POST（rpc 一次性事件 / report cron 事件）→ GET 详情 → PUT 更新
 *      （改名/改触发时刻/停用）→ DELETE；event_type 不可变；
 *   3. 校验矩阵：rpc 事件必须提供未来 next_run_at 且 cron 为空；scene/report 事件的
 *      next_run_at 由后端按 UTC 从 cron 计算（传入即拒）；report 事件必须有 cron；
 *      scene 事件指向不存在/他租户的场景自动化直接拒（场景执行行的落地同步由后端
 *      Go 单测覆盖：internal/service TestSchedulerServiceCreateSceneEventLandsInTimerMechanism）；
 *   4. 聚合可见性：注册的 rpc 事件创建后以 origin=scheduler_registry 出现在聚合列表，
 *      删除后从聚合消失；
 *   5. 严格租户隔离：租户 B 无法查看/更新/删除租户 A 的注册事件（一律 100404，
 *      不泄露存在性）；租户 B 的聚合列表不含租户 A 的注册事件。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Unified Scheduler [95_scheduler_events]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const EVENT_NAME_RPC = 'TB48-契约测试-一次性命令';
const EVENT_NAME_REPORT = 'TB48-契约测试-报表登记';
const CRON_DAILY = '0 9 * * *';

/** 未来 2 小时的 RFC3339 时刻（注册面 rpc 事件用；墙钟推进留足余量）。 */
function futureIso(hours) {
  const d = new Date(Date.now() + hours * 3600 * 1000);
  return d.toISOString().replace(/\.\d{3}Z$/, 'Z');
}

function hourWindow() {
  const now = Date.now();
  return { from_ms: now - 3600 * 1000, to_ms: now + 24 * 3600 * 1000 };
}

describe(SUITE, function () {
  this.timeout(120000);

  let rpcEventId = null;
  let reportEventId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 95_scheduler_events.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  after(async function () {
    if (rpcEventId) {
      try {
        await apiClient.delete('/scheduler/events/' + rpcEventId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
    if (reportEventId) {
      try {
        await apiClient.delete('/scheduler/events/' + reportEventId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. GET /scheduler/events 聚合列表可达，条目含来源类型与来源系统', async function () {
    const list = await apiClient.get('/scheduler/events', { page: 1, page_size: 50 }, TENANT_A);
    expect(list.code, JSON.stringify(list)).to.equal(200);
    expect(list.data).to.be.an('object');
    expect(list.data.list).to.be.an('array');
    expect(list.data.total).to.be.a('number');
    expect(list.data.page).to.equal(1);
    expect(list.data.page_size).to.equal(50);
    for (const item of list.data.list) {
      expect(item.id).to.be.a('string').and.not.equal('');
      expect(item.name).to.be.a('string');
      expect(['scene', 'report', 'rpc']).to.include(item.source_type);
      expect(['scene_timer', 'report_schedule', 'fleet_command_job', 'scheduler_registry']).to.include(item.origin);
      expect(item.enabled).to.be.a('boolean');
      if (item.next_run_at !== null && item.next_run_at !== undefined) {
        expect(item.next_run_at).to.be.a('string');
      }
    }
  });

  it('2. POST /scheduler/events 注册 rpc 一次性事件（未来时刻，cron 为空）', async function () {
    const created = await apiClient.post(
      '/scheduler/events',
      {
        name: EVENT_NAME_RPC,
        event_type: 'rpc',
        ref_id: 'device-tb95-contract',
        next_run_at: futureIso(2)
      },
      TENANT_A
    );
    expect(created.code, JSON.stringify(created)).to.equal(200);
    expect(created.data).to.be.an('object');
    expect(created.data.id).to.be.a('string').and.not.equal('');
    expect(created.data.event_type).to.equal('rpc');
    expect(created.data.tenant_id).to.be.a('string').and.not.equal('');
    expect(created.data.cron).to.equal('');
    expect(created.data.next_run_at).to.be.a('string');
    // ref_type 缺省口径：rpc + ref_id → fleet_command_job
    expect(created.data.ref_type).to.equal('fleet_command_job');
    expect(created.data.enabled).to.equal(true);
    rpcEventId = created.data.id;
  });

  it('3. GET /scheduler/events/:id 详情与注册行一致', async function () {
    const detail = await apiClient.get('/scheduler/events/' + rpcEventId, {}, TENANT_A);
    expect(detail.code, JSON.stringify(detail)).to.equal(200);
    expect(detail.data.id).to.equal(rpcEventId);
    expect(detail.data.name).to.equal(EVENT_NAME_RPC);
    expect(detail.data.event_type).to.equal('rpc');
  });

  it('4. 注册的 rpc 事件以 scheduler_registry 形态进入聚合列表，删除后消失', async function () {
    const window = hourWindow();
    const list = await apiClient.get(
      '/scheduler/events',
      { page: 1, page_size: 500, from_ms: window.from_ms, to_ms: window.to_ms },
      TENANT_A
    );
    expect(list.code, JSON.stringify(list)).to.equal(200);
    const found = (list.data.list || []).filter((item) => item.id === rpcEventId);
    expect(found, 'registered rpc event must appear in the aggregate').to.have.lengthOf(1);
    expect(found[0].origin).to.equal('scheduler_registry');
    expect(found[0].source_type).to.equal('rpc');
  });

  it('5. PUT /scheduler/events/:id 更新名称与触发时刻（event_type 不可变）', async function () {
    const updated = await apiClient.put(
      '/scheduler/events/' + rpcEventId,
      { name: EVENT_NAME_RPC + '-改', next_run_at: futureIso(3), enabled: false },
      TENANT_A
    );
    expect(updated.code, JSON.stringify(updated)).to.equal(200);
    expect(updated.data.name).to.equal(EVENT_NAME_RPC + '-改');
    expect(updated.data.enabled).to.equal(false);
    expect(updated.data.event_type).to.equal('rpc');
    expect(new Date(updated.data.next_run_at).getTime()).to.be.greaterThan(Date.now() + 2 * 3600 * 1000);

    const detail = await apiClient.get('/scheduler/events/' + rpcEventId, {}, TENANT_A);
    expect(detail.code).to.equal(200);
    expect(detail.data.name).to.equal(EVENT_NAME_RPC + '-改');
    expect(detail.data.enabled).to.equal(false);
  });

  it('6. POST 校验矩阵：rpc 事件缺 next_run_at / 带 cron / 过去时刻均拒绝', async function () {
    const noTime = await apiClient.post('/scheduler/events', { name: 'x', event_type: 'rpc' }, TENANT_A);
    expect(noTime.code, JSON.stringify(noTime)).to.equal(100002);

    const withCron = await apiClient.post(
      '/scheduler/events',
      { name: 'x', event_type: 'rpc', cron: CRON_DAILY, next_run_at: futureIso(2) },
      TENANT_A
    );
    expect(withCron.code, JSON.stringify(withCron)).to.equal(100002);

    const past = await apiClient.post(
      '/scheduler/events',
      { name: 'x', event_type: 'rpc', next_run_at: new Date(Date.now() - 60 * 1000).toISOString() },
      TENANT_A
    );
    expect(past.code, JSON.stringify(past)).to.equal(100002);
  });

  it('7. POST 校验矩阵：report 事件必须给 cron；scene 事件不允许自带 next_run_at', async function () {
    const noCron = await apiClient.post('/scheduler/events', { name: 'x', event_type: 'report' }, TENANT_A);
    expect(noCron.code, JSON.stringify(noCron)).to.equal(100002);

    const sceneWithTime = await apiClient.post(
      '/scheduler/events',
      { name: 'x', event_type: 'scene', ref_id: 'some-automation', cron: CRON_DAILY, next_run_at: futureIso(2) },
      TENANT_A
    );
    expect(sceneWithTime.code, JSON.stringify(sceneWithTime)).to.equal(100002);

    const sceneMissingAutomation = await apiClient.post(
      '/scheduler/events',
      { name: 'x', event_type: 'scene', ref_id: 'no-such-automation-tb95', cron: CRON_DAILY },
      TENANT_A
    );
    expect(sceneMissingAutomation.code, JSON.stringify(sceneMissingAutomation)).to.equal(100002);
  });

  it('8. report 注册事件由 cron 以 UTC 计算 next_run_at（严格晚于当前时刻）', async function () {
    const created = await apiClient.post(
      '/scheduler/events',
      { name: EVENT_NAME_REPORT, event_type: 'report', cron: CRON_DAILY },
      TENANT_A
    );
    expect(created.code, JSON.stringify(created)).to.equal(200);
    expect(created.data.event_type).to.equal('report');
    expect(created.data.cron).to.equal(CRON_DAILY);
    expect(created.data.next_run_at).to.be.a('string');
    expect(new Date(created.data.next_run_at).getTime()).to.be.greaterThan(Date.now());
    reportEventId = created.data.id;
  });

  it('9. source_type 过滤只返回对应来源类型的条目', async function () {
    const rpcOnly = await apiClient.get('/scheduler/events', { page: 1, page_size: 500, source_type: 'rpc' }, TENANT_A);
    expect(rpcOnly.code, JSON.stringify(rpcOnly)).to.equal(200);
    for (const item of rpcOnly.data.list || []) {
      expect(item.source_type).to.equal('rpc');
    }
  });

  it('10. 租户隔离：租户 B 无法查看/更新/删除租户 A 的注册事件（一律 100404）', async function () {
    const detail = await apiClient.get('/scheduler/events/' + rpcEventId, {}, TENANT_B);
    expect(detail.code, JSON.stringify(detail)).to.equal(100404);

    const updated = await apiClient.put(
      '/scheduler/events/' + rpcEventId,
      { name: 'hijacked' },
      TENANT_B
    );
    expect(updated.code, JSON.stringify(updated)).to.equal(100404);

    const removed = await apiClient.delete('/scheduler/events/' + rpcEventId, {}, TENANT_B);
    expect(removed.code, JSON.stringify(removed)).to.equal(100404);

    const untouched = await apiClient.get('/scheduler/events/' + rpcEventId, {}, TENANT_A);
    expect(untouched.code).to.equal(200);
    expect(untouched.data.name).to.equal(EVENT_NAME_RPC + '-改');
  });

  it('11. 租户隔离：租户 B 的聚合列表不含租户 A 的注册事件', async function () {
    const listB = await apiClient.get('/scheduler/events', { page: 1, page_size: 500 }, TENANT_B);
    expect(listB.code, JSON.stringify(listB)).to.equal(200);
    const leaked = (listB.data.list || []).filter((item) => item.id === rpcEventId || item.id === reportEventId);
    expect(leaked, 'tenant A registry events must not leak into tenant B aggregate').to.have.lengthOf(0);
  });

  it('12. DELETE /scheduler/events/:id 删除注册事件并从聚合消失', async function () {
    const targetId = rpcEventId;
    const removed = await apiClient.delete('/scheduler/events/' + targetId, {}, TENANT_A);
    expect(removed.code, JSON.stringify(removed)).to.equal(200);
    rpcEventId = null;

    const gone = await apiClient.get('/scheduler/events/' + targetId, {}, TENANT_A);
    expect(gone.code, JSON.stringify(gone)).to.equal(100404);

    const window = hourWindow();
    const list = await apiClient.get(
      '/scheduler/events',
      { page: 1, page_size: 500, from_ms: window.from_ms, to_ms: window.to_ms },
      TENANT_A
    );
    expect(list.code, JSON.stringify(list)).to.equal(200);
    const stillThere = (list.data.list || []).filter((item) => item.id === targetId);
    expect(stillThere, 'deleted registry event must leave the aggregate').to.have.lengthOf(0);
  });
});
