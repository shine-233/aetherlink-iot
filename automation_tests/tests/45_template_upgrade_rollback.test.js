/**
 * 文件用途：P1.6 模板升级 / 回滚（ROADMAP §6 第一优先级第 5 项）的 API 契约测试。
 *
 * 覆盖的是"模板版本推进与回退"这条链路的四面：
 *   1. 入参校验：空 payload、无名称 payload、非点分数字版本号一律 100002。
 *   2. 存在性：不存在的模板不能升级、不存在的 history_id 不能回滚。
 *   3. 版本语义：**目标版本必须严格新于当前**——同版本重发与降级都被拒，
 *      且必须走回滚通道。这是本片最核心的一条断言。
 *   4. 正常链路：升级（记录回滚点）→ 历史读回 → 回滚（重放旧载荷）→ 不删行。
 *   5. 租户边界：跨租户升级 / 回滚被拒，历史不跨租户泄漏。
 *
 * 关键注意事项：
 *   - **回滚是重放不是删除**：用例锁死"回滚后新版本行仍在、旧版本行仍在"，
 *     两个版本共存。删新版本行不可逆且会牵连引用它的设备配置。
 *   - **"当前版本"由 created_at DESC 决定，版本号不参与排序**（DAL 注释明确：
 *     点分字符串在数据库里排序会 1.10 < 1.2）。因此回滚重放旧载荷是**幂等命中**
 *     （旧行已存在、不再建行），它**不会**把"当前版本"指针拨回旧版本。
 *     用例第 10 条锁死这个实测语义：回滚后再次升级，from_version 仍是 2.0.0。
 *     这不是缺陷，是"切换动作交给引用方"的设计，但必须被用例固定住，否则
 *     后来者会误以为回滚等于版本指针回退。
 *   - 历史响应**不含 previous_payload**（模型上 `json:"-"`）——它是回滚凭据，
 *     不随列表下发。用例锁死这一点，防止有人图省事把它加进响应。
 *   - 跨租户对照用 tenant_admin_b（第二租户管理员），不是 TENANT_USER：
 *     后者按 Casbin 只有 `upgrade/history` 的读权限，测出来的拒绝分不清是
 *     "跨租户被拦"还是"角色本身没权限"。
 *   - 夹具走 `/device/template/import`（导出载荷原样回传），不用 `POST /device/template`：
 *     后者不写 version 列，而升级语义要求"当前版本"非空。
 *
 * 重构建议：若后续引入"当前版本指针"表（而非 created_at 推断），
 * 第 10 条的期望必须同步改写，并补"回滚后指针显式回退"的用例。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Device template upgrade / rollback [45_template_upgrade_rollback]';
const ACCOUNT = 'tenant_admin';
const OTHER_TENANT_ACCOUNT = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_UNAUTHORIZED = 401;

const MISSING_ID = '00000000-0000-0000-0000-000000000000';

const UPGRADE_PATH = '/device/template/upgrade';

function expectOk(resp) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected 200 but got ${resp.code}: ${resp.message || ''}`).to.equal(200);
  return resp.data;
}

function expectCode(resp, code, label) {
  expect(resp, `${label}: response envelope`).to.be.an('object');
  expect(resp.code, `${label}: expected ${code} but got ${resp.code} (${resp.message || ''})`).to.equal(code);
  return resp.data;
}

function upgradePayload(name, version, extra) {
  return {
    payload: Object.assign(
      {
        kind: 'aetherlink-device-template',
        name,
        version,
        type_key: 'automation'
      },
      extra || {}
    )
  };
}

function rollbackPath(historyId) {
  return `${UPGRADE_PATH}/${historyId}/rollback`;
}

function historyList(data) {
  expect(data, 'history payload must be a bare array (no {list} wrapper)').to.be.an('array');
  return data;
}

async function listHistory(templateName, accountKey = ACCOUNT) {
  return historyList(expectOk(await apiClient.get(`${UPGRADE_PATH}/history`, { template_name: templateName }, accountKey)));
}

describe(SUITE, function () {
  this.timeout(120000);

  let templateName = null;
  let v1Id = null;
  let v2Id = null;
  let firstHistoryId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 45_template_upgrade_rollback.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(OTHER_TENANT_ACCOUNT);

    templateName = `automation_upgrade_${Date.now()}`;
    const imported = expectOk(
      await apiClient.post(
        '/device/template/import',
        {
          kind: 'aetherlink-device-template',
          name: templateName,
          version: '1.0.0',
          type_key: 'automation',
          description: 'v1 baseline created by 45_template_upgrade_rollback'
        },
        ACCOUNT
      )
    );
    expect(imported.created, 'the baseline version must be a fresh row').to.equal(true);
    v1Id = imported.template.id;
    expect(v1Id, 'baseline template id').to.be.a('string').and.not.equal('');
  });

  it('rejects an upgrade request without a payload', async function () {
    expectCode(await apiClient.post(UPGRADE_PATH, {}, ACCOUNT), CODE_PARAM_ERROR, 'missing payload');
  });

  it('rejects an upgrade payload without a template name', async function () {
    const resp = await apiClient.post(
      UPGRADE_PATH,
      { payload: { kind: 'aetherlink-device-template', version: '2.0.0' } },
      ACCOUNT
    );
    expectCode(resp, CODE_PARAM_ERROR, 'missing template name');
  });

  it('rejects upgrading a template that does not exist in the tenant', async function () {
    const resp = await apiClient.post(
      UPGRADE_PATH,
      upgradePayload(`automation_ghost_${Date.now()}`, '2.0.0'),
      ACCOUNT
    );
    expectCode(resp, CODE_PARAM_ERROR, 'unknown template');
  });

  it('rejects a target version that is not dotted numeric', async function () {
    const resp = await apiClient.post(UPGRADE_PATH, upgradePayload(templateName, 'v2'), ACCOUNT);
    expectCode(resp, CODE_PARAM_ERROR, 'non-dotted-numeric version');
  });

  it('upgrades to a strictly newer version and records a rollback point', async function () {
    const upgraded = expectOk(
      await apiClient.post(
        UPGRADE_PATH,
        upgradePayload(templateName, '2.0.0', { description: 'v2 created by the upgrade channel' }),
        ACCOUNT
      )
    );
    expect(upgraded.history_id, 'history id').to.be.a('string').and.not.equal('');
    expect(upgraded.template, 'upgraded template').to.be.an('object');
    expect(upgraded.template.version, 'the upgrade creates a new version row').to.equal('2.0.0');
    expect(upgraded.template.id, 'the new version is a distinct row, not an in-place update').to.not.equal(v1Id);

    firstHistoryId = upgraded.history_id;
    v2Id = upgraded.template.id;
  });

  it('rejects repeating the upgrade to the same target version', async function () {
    // 实测语义（不是猜测）：同版本重发被"严格新于"闸门拒掉，返回 100002 并带上
    // from_version / to_version。也就是说升级通道**不是**幂等重放——重复调用会失败，
    // 要回到旧版本必须走回滚。
    const data = expectCode(
      await apiClient.post(UPGRADE_PATH, upgradePayload(templateName, '2.0.0'), ACCOUNT),
      CODE_PARAM_ERROR,
      'repeated upgrade to the same version'
    );
    expect(data, 'rejection must explain itself').to.be.an('object');
    expect(data.from_version, 'from_version in the rejection').to.equal('2.0.0');
    expect(data.to_version, 'to_version in the rejection').to.equal('2.0.0');
  });

  it('rejects downgrading through the upgrade channel', async function () {
    const data = expectCode(
      await apiClient.post(UPGRADE_PATH, upgradePayload(templateName, '1.5.0'), ACCOUNT),
      CODE_PARAM_ERROR,
      'downgrade via upgrade'
    );
    expect(data.from_version, 'downgrade is measured against the current version').to.equal('2.0.0');
    expect(data.to_version).to.equal('1.5.0');
    expect(data.error, 'the rejection must point at the rollback channel').to.be.a('string');
    expect(data.error).to.contain('rollback');
  });

  it('lists the upgrade history newest first, with audit fields and no previous_payload', async function () {
    const rows = await listHistory(templateName);
    expect(rows.length, 'the upgrade above must have left one rollback point').to.be.at.least(1);

    const first = rows.find(row => row.id === firstHistoryId);
    expect(first, 'the recorded history must be listed').to.be.an('object');
    expect(first.template_name).to.equal(templateName);
    expect(first.from_version).to.equal('1.0.0');
    expect(first.to_version).to.equal('2.0.0');
    expect(first.tenant_id, 'history is tenant stamped').to.be.a('string').and.not.equal('');
    expect(first.actor_id, 'history records who upgraded').to.be.a('string').and.not.equal('');
    expect(first.created_at, 'history is timestamped').to.be.a('string').and.not.equal('');
    // previous_payload 是回滚凭据，绝不能随列表下发。
    expect(first.previous_payload, 'previous_payload must never be exposed').to.equal(undefined);

    for (let i = 1; i < rows.length; i += 1) {
      const prev = Date.parse(rows[i - 1].created_at);
      const curr = Date.parse(rows[i].created_at);
      expect(curr, 'history must be descending by created_at').to.be.at.most(prev);
    }
  });

  it('rolls back by replaying the previous payload and deletes no rows', async function () {
    const before = await listHistory(templateName);

    const restored = expectOk(await apiClient.post(rollbackPath(firstHistoryId), {}, ACCOUNT));
    expect(restored.version, 'rollback restores the previous version').to.equal('1.0.0');
    expect(restored.id, 'rollback is an idempotent replay of the existing row').to.equal(v1Id);

    // 不删行：新版本行仍在，旧版本行仍在，两个版本共存。
    const v1 = expectOk(await apiClient.get(`/device/template/detail/${v1Id}`, {}, ACCOUNT));
    expect(v1.version, 'the old row survives the rollback').to.equal('1.0.0');
    const v2 = expectOk(await apiClient.get(`/device/template/detail/${v2Id}`, {}, ACCOUNT));
    expect(v2.version, 'the upgraded row is NOT deleted by the rollback').to.equal('2.0.0');

    // 回滚也不写新的历史行：它不是一次"反向升级"。
    const after = await listHistory(templateName);
    expect(after.length, 'rollback must not append a history row').to.equal(before.length);
  });

  it('keeps the current-version pointer on the newest row after a rollback', async function () {
    // 实测语义锁定：当前版本 = created_at 最新的一行。回滚是重放（幂等命中，不建行），
    // 因此不会把指针拨回 1.0.0——下一次升级的 from_version 仍是 2.0.0。
    const upgraded = expectOk(
      await apiClient.post(UPGRADE_PATH, upgradePayload(templateName, '3.0.0'), ACCOUNT)
    );
    expect(upgraded.template.version).to.equal('3.0.0');

    const rows = await listHistory(templateName);
    const latest = rows[0];
    expect(latest.id, 'the newest history row is the one just written').to.equal(upgraded.history_id);
    expect(latest.from_version, 'the rollback did NOT move the current version back to 1.0.0').to.equal('2.0.0');
    expect(latest.to_version).to.equal('3.0.0');
    expect(rows.length, 'the second upgrade appends a second rollback point').to.be.at.least(2);
  });

  it('rejects rolling back an unknown history id', async function () {
    expectCode(await apiClient.post(rollbackPath(MISSING_ID), {}, ACCOUNT), CODE_PARAM_ERROR, 'unknown history id');
  });

  it('rejects upgrading another tenant template', async function () {
    // tenant_admin_b 租户里没有这个模板，"升级"无从谈起。
    const resp = await apiClient.post(UPGRADE_PATH, upgradePayload(templateName, '9.9.9'), OTHER_TENANT_ACCOUNT);
    expectCode(resp, CODE_PARAM_ERROR, 'cross-tenant upgrade');
  });

  it('rejects rolling back another tenant upgrade history', async function () {
    const resp = await apiClient.post(rollbackPath(firstHistoryId), {}, OTHER_TENANT_ACCOUNT);
    expectCode(resp, CODE_PARAM_ERROR, 'cross-tenant rollback');
  });

  it('does not leak upgrade history across tenants', async function () {
    const rows = await listHistory(templateName, OTHER_TENANT_ACCOUNT);
    const leaked = rows.filter(row => row.template_name === templateName);
    expect(leaked.length, 'the other tenant must not see this tenant rollback points').to.equal(0);
  });

  it('rejects unauthenticated access to all three upgrade endpoints', async function () {
    const history = await apiClient.getNoAuth(`${UPGRADE_PATH}/history`, {});
    expect(history.code, 'unauthenticated history read must not succeed').to.equal(CODE_UNAUTHORIZED);

    const upgrade = await apiClient.postNoAuth(UPGRADE_PATH, upgradePayload(templateName, '4.0.0'));
    expect(upgrade.code, 'unauthenticated upgrade must not succeed').to.equal(CODE_UNAUTHORIZED);

    const rollback = await apiClient.postNoAuth(rollbackPath(firstHistoryId), {});
    expect(rollback.code, 'unauthenticated rollback must not succeed').to.equal(CODE_UNAUTHORIZED);
  });
});
