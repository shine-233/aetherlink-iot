/**
 * 文件用途：实体版本控制差异对比（TB-25）业务自动化测试。
 * 核心逻辑：以 board 为载体走通对比闭环——两份快照间 JSON 语义 diff（点号路径列表）、
 *           无变化快照的零差异、不存在版本的负向分支，以及跨租户 fail-closed 隔离。
 * 关键注意事项：快照内容由后端从实体当前行读取；看板名称变更会同时更新 updated_at，
 *           故断言"包含 name 路径"而非全量路径集合相等。
 * 重构建议：若后续接入 Git 仓库后端（branch/commit 语义），按后端类型分套补用例。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectSuccess: expectOk,
  expectBusinessError
} = require('../lib/response_assertions');

const SUITE = 'Entity version diff [83_entity_version_diff]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const CODE_NOT_FOUND = 100404;

function uniqueName(prefix) {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 100000)}`;
}

function findChange(diff, path) {
  return (diff.changes || []).find((change) => change.path === path);
}

describe(SUITE, function () {
  this.timeout(120000);

  let boardId = null;
  let boardNameV1 = null;
  let boardNameV2 = null;
  let versionV1 = null;
  let versionV2 = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 83_entity_version_diff.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);

    // 建板 -> 快照 v1 -> 改名 -> 快照 v2，构造一份含已知差异（name/updated_at）的版本对。
    boardNameV1 = uniqueName('diff-board-v1');
    const createResp = await apiClient.post('/board', { name: boardNameV1, home_flag: 'N' }, TENANT_A);
    expectOk(createResp);
    boardId = createResp.data.id;
    expect(boardId).to.be.a('string').and.not.equal('');

    const v1Resp = await apiClient.post('/entity_versions', {
      entity_type: 'board',
      entity_id: boardId,
      remark: 'diff-baseline'
    });
    expectOk(v1Resp);
    versionV1 = v1Resp.data;

    boardNameV2 = uniqueName('diff-board-v2');
    const updateResp = await apiClient.put('/board', { id: boardId, name: boardNameV2 }, TENANT_A);
    expectOk(updateResp);

    const v2Resp = await apiClient.post('/entity_versions', {
      entity_type: 'board',
      entity_id: boardId,
      remark: 'diff-after-mutation'
    });
    expectOk(v2Resp);
    versionV2 = v2Resp.data;
  });

  after(async function () {
    try {
      if (boardId) {
        await apiClient.delete('/board/' + boardId, {}, TENANT_A);
      }
    } catch (e) {
      /* ignore cleanup error */
    } finally {
      apiClient.clearAllTokens();
    }
  });

  it('1. GET /entity_versions/:id/diff/:target_id 返回两侧元信息与语义差异', async function () {
    const res = await apiClient.get(`/entity_versions/${versionV1.id}/diff/${versionV2.id}`, {}, TENANT_A);
    expectOk(res);
    expect(res.data).to.be.an('object');

    // 两侧版本元信息：source 为基准（v1），target 为对比（v2）。
    expect(res.data.source).to.be.an('object');
    expect(res.data.source.id).to.equal(versionV1.id);
    expect(res.data.source.version_number).to.equal(versionV1.version_number);
    expect(res.data.target).to.be.an('object');
    expect(res.data.target.id).to.equal(versionV2.id);

    const diff = res.data.diff;
    expect(diff).to.be.an('object');
    expect(diff.added).to.be.an('array');
    expect(diff.removed).to.be.an('array');
    expect(diff.modified).to.be.an('array');
    expect(diff.changes).to.be.an('array');
    expect(diff.total).to.be.a('number').and.at.least(1);

    // 看板名称变更必须报 name 路径 modified，且旧值/新值与两次快照一致。
    expect(diff.modified, `name must be modified, diff=${JSON.stringify(diff)}`).to.include('name');
    const nameChange = findChange(diff, 'name');
    expect(nameChange, 'name change detail must be present').to.be.an('object');
    expect(nameChange.kind).to.equal('modified');
    expect(nameChange.old_value).to.equal(boardNameV1);
    expect(nameChange.new_value).to.equal(boardNameV2);
  });

  it('2. 内容一致的两份快照返回零差异', async function () {
    // v2 之后不再变更实体，再拍一份 v3：与 v2 应完全一致。
    const v3Resp = await apiClient.post('/entity_versions', {
      entity_type: 'board',
      entity_id: boardId,
      remark: 'diff-no-change'
    });
    expectOk(v3Resp);

    const res = await apiClient.get(`/entity_versions/${versionV2.id}/diff/${v3Resp.data.id}`, {}, TENANT_A);
    expectOk(res);
    const diff = res.data.diff;
    expect(diff.total, `identical snapshots must yield zero diff, got ${JSON.stringify(diff)}`).to.equal(0);
    expect(diff.added).to.deep.equal([]);
    expect(diff.removed).to.deep.equal([]);
    expect(diff.modified).to.deep.equal([]);
    expect(diff.changes).to.deep.equal([]);
  });

  it('3. 源或目标版本不存在返回 100404', async function () {
    const missingId = '00000000-0000-0000-0000-000000000000';

    const sourceMissing = await apiClient.get(`/entity_versions/${missingId}/diff/${versionV2.id}`, {}, TENANT_A);
    expectBusinessError(sourceMissing, CODE_NOT_FOUND);

    const targetMissing = await apiClient.get(`/entity_versions/${versionV1.id}/diff/${missingId}`, {}, TENANT_A);
    expectBusinessError(targetMissing, CODE_NOT_FOUND);
  });

  it('4. 租户 B 无法对比租户 A 的版本（fail-closed）', async function () {
    const res = await apiClient.get(`/entity_versions/${versionV1.id}/diff/${versionV2.id}`, {}, TENANT_B);
    expectBusinessError(res, CODE_NOT_FOUND);
  });
});
