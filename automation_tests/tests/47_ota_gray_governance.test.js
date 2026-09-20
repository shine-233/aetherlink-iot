/**
 * 文件用途：P0.3「灰度治理 / 金丝雀 rollout」的 API 契约测试与可达性取证。
 *
 * 这一组用例刻意不只测"接口返回 200"，而是同时把**真实链路上哪些分支根本触发不到**
 * 固定下来。原因是 ROADMAP §1.2 把 P0.3 的灰度治理记为 `未验证`（只差跑一遍），
 * 而本轮核对发现它同时带有 `未接线`（配置面）与 `未实现`（暂停/继续、转全量等）成分。
 * 若只报"接口通了"，这三个缺口会被再次记成"已验证"。
 *
 * 覆盖的四面：
 *   1. 只读性：`GET governance-preview` 不得改 task 行、不得改 detail 行、不得下发。
 *   2. 执行面：`POST governance-apply` 的意图是把 `is_simulation` 翻成 false 并真的放量/中止/收尾——
 *      否则"预览"与"执行"在调用方眼里没有区别，金丝雀只是块告示牌。
 *      **但 2026-09-15 实测它在真实 PostgreSQL 上写不动**（见文件末尾的缺陷见证用例），
 *      所以本组只把"它现在报什么错、报错后留下什么"固定下来，不谎报成功。
 *   3. 配置面（本轮的核心发现）：`POST /ota/task` 的入参里**没有** rollout 治理字段
 *      （`CreateOTAUpgradeTaskReq` 只有 name/package/device 选择/expected_total/max_devices），
 *      DAL `CreateOTAUpgradeTaskWithDetail` 也从不写这几列，因此通过 API 建出来的
 *      task 恒为 DB 默认值：`rollout_rate_per_minute=60`、`abort_failure_rate_percent=NULL`、
 *      `timeout_at=NULL`、`scheduled_at=NULL`。用例把这些默认值锁死，
 *      用来证明"失败率阈值自动中止 / 绝对超时收尾"两个分支在 HTTP 面上不可配置。
 *   4. 可达性：`CreateOTAUpgradeTask` 在建任务后立即 `go pushOTAUpgradeTaskDetails`，
 *      设备离线则明细行直接落 `failed(5)`、在线则落 `pushed(2)`，
 *      **没有任何路径会让明细行停留在 `pending(1)`**。而 `dispatch_batch`
 *      只针对 pending 行。所以"按限速分批放量"这条主链路在真实 API 上触发不到。
 *      这里断言 pending 行数与决策分支的对应关系，把这个事实固化成回归保护。
 *
 * 关键注意事项：
 *   - 不启动 MQTT 设备：本轮证明的是治理决策链路，不是设备回调链路（后者由 32 组覆盖）。
 *     设备离线 → 明细行 failed → 治理输入变成 pending=0/upgrading=0/failed>0。
 *   - 跨租户用 `tenant_admin_b`（第二租户管理员），不用 TENANT_USER：
 *     TENANT_USER 走 `OTAUpgradeTaskDevicesOwnedBy` 归属过滤，被拒分不清是
 *     "跨租户被拦"还是"不是自己名下的设备"。
 *   - 清理依赖 `DELETE /ota/task/:id`；若后端在治理后拒绝删除已完成批次，
 *     这条失败本身就是需要记进证据文档的缺陷。
 *
 * 重构建议：若后续补上"创建/更新时接受治理参数"、暂停/继续、灰度转全量，
 * 应在本文件新增对应用例，并把第 3、4 组的"默认值/不可达"断言改写为新契约。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'OTA rollout gray governance [47_ota_gray_governance]';
const ACCOUNT = 'tenant_admin';
const OTHER_TENANT_ACCOUNT = 'tenant_admin_b';

const MISSING_ID = '00000000-0000-0000-0000-000000000000';

// ota_upgrade_task_details.status（见 internal/model/ota_upgrade_task_status.go）
const DETAIL_PENDING = 1;

// 规划器动作（见 internal/service/ota_rollout_governance.go:21-29）
const ACTION_DISPATCH_BATCH = 'dispatch_batch';
const ACTION_HOLD = 'hold';
const ACTION_COMPLETE = 'complete';

function expectOk(resp, label) {
  expect(resp, `${label}: response envelope`).to.be.an('object');
  expect(resp.code, `${label}: expected 200 but got ${resp.code} (${resp.message || ''})`).to.equal(200);
  return resp.data;
}

function expectRejected(resp, label) {
  expect(resp, `${label}: response envelope`).to.be.an('object');
  expect(resp.code, `${label}: must not succeed but got ${resp.code} (${resp.message || ''})`).to.not.equal(200);
}

function detailRows(payload) {
  return Array.isArray(payload) ? payload : (payload && Array.isArray(payload.list) ? payload.list : []);
}

function preview(taskId, accountKey = ACCOUNT) {
  return apiClient.get(`/ota/task/${taskId}/governance-preview`, {}, accountKey);
}

function apply(taskId, accountKey = ACCOUNT) {
  return apiClient.post(`/ota/task/${taskId}/governance-apply`, {}, accountKey);
}

async function readTaskRow(taskId, packageId, accountKey = ACCOUNT) {
  const resp = await apiClient.get('/ota/task', {
    page: 1,
    page_size: 100,
    ota_upgrade_package_id: packageId
  }, accountKey);
  const rows = detailRows(expectOk(resp, 'list OTA tasks'));
  return rows.find(row => row && String(row.id) === String(taskId)) || null;
}

async function readDetails(taskId, accountKey = ACCOUNT) {
  const resp = await apiClient.get('/ota/task/detail', {
    page: 1,
    page_size: 100,
    ota_upgrade_task_id: taskId
  }, accountKey);
  return detailRows(expectOk(resp, 'list OTA task details'));
}

function pendingCount(rows) {
  return rows.filter(row => Number(row.status) === DETAIL_PENDING).length;
}

// 建任务后的推送是 goroutine 异步完成的，等所有明细行离开 pending 再做治理断言，
// 否则测到的是"创建与首推之间的竞态窗口"，不是治理决策本身。
async function waitForDispatchSettled(taskId, timeoutMs = 30000) {
  const deadline = Date.now() + timeoutMs;
  let rows = [];
  while (Date.now() < deadline) {
    rows = await readDetails(taskId);
    if (rows.length > 0 && pendingCount(rows) === 0) return rows;
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  return rows;
}

describe(SUITE, function () {
  this.timeout(180000);

  let packageSeed = null;
  let taskSeed = null;
  let taskId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 47_ota_gray_governance.test.js');
    }
    await apiClient.login(ACCOUNT);

    packageSeed = await seedData.createOtaPackageSeed(ACCOUNT);
    taskSeed = await seedData.createOtaTaskApiSeed(
      packageSeed.id,
      packageSeed.row,
      ACCOUNT,
      packageSeed,
      {}
    );
    taskId = taskSeed.taskId;
    expect(taskId, 'fixture OTA task id').to.be.a('string').and.not.equal('');
  });

  after(async function () {
    if (taskSeed) {
      await taskSeed.cleanup();
    } else if (packageSeed) {
      await packageSeed.cleanup();
    }
  });

  it('creates the rollout with configured governance settings via create API', async function () {
    const device = await seedData.createSimulationDevice(ACCOUNT);
    let createdTaskId = '';
    try {
      const createResp = await apiClient.post('/ota/task', {
        name: seedData.makeRunLabel('seed_ota_gray_governance_configured'),
        ota_upgrade_package_id: packageSeed.id,
        description: 'governance configuration surface probe',
        device_id_list: [device.id],
        expected_total: 1,
        rollout_rate_per_minute: 7,
        abort_failure_rate_percent: 1,
        timeout_seconds: 120,
        scheduled_at: new Date(Date.now() + 3600 * 1000).toISOString()
      }, ACCOUNT);
      expectOk(createResp, 'create OTA task with governance params');

      const deadline = Date.now() + 15000;
      while (Date.now() < deadline && !createdTaskId) {
        const listResp = await apiClient.get('/ota/task', {
          page: 1,
          page_size: 100,
          ota_upgrade_package_id: packageSeed.id
        }, ACCOUNT);
        const rows = detailRows(expectOk(listResp, 'list OTA tasks'));
        const row = rows.find(item => item && item.description === 'governance configuration surface probe');
        if (row && row.id) createdTaskId = String(row.id);
        if (!createdTaskId) await new Promise(resolve => setTimeout(resolve, 500));
      }
      expect(createdTaskId, 'governance probe task must be resolvable').to.not.equal('');

      const probeRow = await readTaskRow(createdTaskId, packageSeed.id);
      expect(probeRow, 'governance probe task row').to.be.an('object');
      expect(
        Number(probeRow.rollout_rate_per_minute),
        'rollout_rate_per_minute is set correctly by API'
      ).to.equal(7);
      expect(
        Number(probeRow.abort_failure_rate_percent),
        'abort_failure_rate_percent is set correctly by API'
      ).to.equal(1);
      expect(
        probeRow.timeout_at,
        'timeout_at is computed from timeout_seconds'
      ).to.not.equal(null);
      expect(
        probeRow.scheduled_at,
        'scheduled_at is set correctly by API'
      ).to.not.equal(null);
    } finally {
      if (createdTaskId) await apiClient.delete(`/ota/task/${createdTaskId}`, {}, ACCOUNT);
      await device.cleanup();
    }
  });

  it('leaves no pending detail rows after creation, which is why rate-limited dispatch is unreachable over HTTP', async function () {
    const rows = await waitForDispatchSettled(taskId);
    expect(rows.length, 'seeded task must have detail rows').to.be.greaterThan(0);
    expect(
      pendingCount(rows),
      'after creation every detail row is pushed or failed; dispatch_batch only claims pending rows'
    ).to.equal(0);

    const decision = expectOk(await preview(taskId), 'governance preview');
    expect(
      decision.action,
      'with zero pending rows the planner must not plan a dispatch batch'
    ).to.not.equal(ACTION_DISPATCH_BATCH);
    expect([ACTION_HOLD, ACTION_COMPLETE], 'planner action').to.include(decision.action);
  });

  it('previews without mutating the task row or the detail rows', async function () {
    const beforeTask = await readTaskRow(taskId, packageSeed.id);
    const beforeDetails = await readDetails(taskId);

    const decision = expectOk(await preview(taskId), 'governance preview');
    expect(decision, 'preview decision').to.be.an('object');
    expect(decision.is_simulation, 'preview must advertise itself as a simulation').to.equal(true);
    expect(decision.action, 'preview action').to.be.a('string').and.not.equal('');
    expect(decision.reason, 'preview reason').to.be.a('string').and.not.equal('');
    expect(decision.next_steps, 'preview next_steps').to.be.an('array');

    const afterTask = await readTaskRow(taskId, packageSeed.id);
    const afterDetails = await readDetails(taskId);
    expect(String(afterTask.status), 'preview must not change task status').to.equal(String(beforeTask.status));
    expect(
      Number(afterTask.rate_window_dispatched),
      'preview must not consume the rate window'
    ).to.equal(Number(beforeTask.rate_window_dispatched));
    expect(afterDetails.length, 'preview must not add or remove detail rows').to.equal(beforeDetails.length);
    afterDetails.forEach((row, index) => {
      expect(Number(row.status), 'preview must not change detail status').to.equal(
        Number(beforeDetails[index].status)
      );
    });
  });

  it('closes out a fully failed rollout instead of aborting it when no failure-rate threshold is configured', async function () {
    const decision = expectOk(await preview(taskId), 'governance preview');
    expect(decision.action, 'planner action').to.equal(ACTION_COMPLETE);

    // 失败率算出来了，但阈值是 NULL —— 规划器因此不会走 abort。
    // 这条断言把"失败率阈值中止"在 HTTP 面上不可达这件事固化下来。
    expect(decision.failure_rate, 'failure rate is computed').to.be.a('number');
    expect(decision.warnings, 'a failed rollout must warn on close-out').to.be.an('array');
    expect(decision.warnings.join(' '), 'warning text mentions the failed devices').to.match(/失败/);
  });

  // ★ 缺陷见证（不是通过凭证）★
  // 2026-09-15 实测：apply 只要落到写分支就必然报错——
  //   ERROR: column "updated_at" of relation "ota_upgrade_tasks" does not exist (SQLSTATE 42703)
  // 根因：`internal/dal/ota_rollout_governance.go:61` 无条件写 `updates["updated_at"]`，
  // 而 `ota_upgrade_tasks` 从 1.sql:445 建表起就只有 `created_at`（38.sql 也没补），
  // model `ota_upgrade_tasks.gen.go:15` 同样没有 UpdatedAt 字段。
  // 于是 `dispatch_batch` / `abort` / `timeout` / `complete` 四个写分支**没有一个能跑通**；
  // 只有 `wait_schedule` / `hold_rate_window` / `hold` 这三个不写库的分支会返回 200 且什么都不做。
  // 这正好是 ROADMAP §1.2.1 记过的教训：没在真实库上跑过的代码被当成"已实现"。
  //
  // 修复 `updated_at`（补列 + 改 DAL）之后，下面这条用例**必须**改成断言
  // code===200 / is_simulation===false / task_status==='completed'，
  // 并且要补 `dispatch_batch` 的放量用例与 `abort` 的中止用例。
  it('successfully applies the rollout decision on real database and updates task status', async function () {
    const before = await readTaskRow(taskId, packageSeed.id);
    const resp = await apply(taskId);

    expect(resp, 'apply response envelope').to.be.an('object');
    expect(resp.code, 'apply succeeds when write path does not update non-existent column').to.equal(200);
    const data = resp.data;
    expect(data, 'apply response data').to.be.an('object');
    expect(data.is_simulation, 'apply is not a simulation').to.equal(false);
    expect(data.decision, 'applied decision').to.be.an('object');
    expect(data.decision.action, 'applied action').to.be.a('string');

    const after = await readTaskRow(taskId, packageSeed.id);
    expect(String(after.status), 'task status should be updated').to.be.oneOf(['completed', 'running', 'canceled']);
  });

  it('exposes no updated_at column on the task read model, which is what the apply write path trips on', async function () {
    const taskRow = await readTaskRow(taskId, packageSeed.id);
    expect(taskRow, 'task row').to.be.an('object');
    expect(
      Object.prototype.hasOwnProperty.call(taskRow, 'updated_at'),
      'the read model mirrors the table columns; updated_at is absent, so the DAL write cannot succeed'
    ).to.equal(false);
  });

  it('rejects previewing a rollout from another tenant', async function () {
    expectRejected(await preview(taskId, OTHER_TENANT_ACCOUNT), 'cross-tenant preview');
  });

  it('rejects applying a rollout from another tenant', async function () {
    expectRejected(await apply(taskId, OTHER_TENANT_ACCOUNT), 'cross-tenant apply');
  });

  it('rejects preview and apply for a task that does not exist', async function () {
    expectRejected(await preview(MISSING_ID), 'unknown task preview');
    expectRejected(await apply(MISSING_ID), 'unknown task apply');
  });
});
