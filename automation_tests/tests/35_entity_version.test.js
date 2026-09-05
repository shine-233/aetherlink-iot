/**
 * 文件用途：实体版本控制（ROADMAP C7）业务自动化测试。
 * 核心逻辑：以 board 为载体走通版本化闭环——快照创建、版本历史、详情读取、
 *           dry_run 只回显不落库、真实恢复使实体回退到快照时刻状态，并覆盖
 *           白名单外实体类型与不存在版本 id 的负向分支。
 * 关键注意事项：快照内容由后端从实体当前行读取（不接受客户端传入）；
 *           恢复剔除 id/tenant_id/created_at 三个不可变列。
 * 重构建议：若后续开放 device_config/rule_chain 之外的实体类型，按白名单逐个补用例。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectSuccess: expectOk,
  expectBusinessError
} = require('../lib/response_assertions');

const CODE_PARAM_ERROR = 100002;
const CODE_NOT_FOUND = 100404;

function uniqueName(prefix) {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 100000)}`;
}

function parseSnapshot(versionRow) {
  expect(versionRow.snapshot, 'snapshot must be persisted as JSON text').to.be.a('string');
  return JSON.parse(versionRow.snapshot);
}

describe('Entity version snapshot and restore [35_entity_version]', function () {
  this.timeout(30000);

  let boardId = null;
  let boardNameV1 = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 35_entity_version.test.js; entity version coverage requires a healthy API service');
    }
    await apiClient.login('tenant_admin');

    boardNameV1 = uniqueName('audit-versioned-board');
    const createResp = await apiClient.post('/board', {
      name: boardNameV1,
      home_flag: 'N'
    }, 'tenant_admin');
    expectOk(createResp);
    boardId = createResp.data.id;
    expect(boardId).to.be.a('string').and.not.equal('');
  });

  after(async function () {
    try {
      if (boardId) {
        await apiClient.delete('/board/' + boardId, {}, 'tenant_admin');
      }
    } catch (error) {
      // cleanup failure must not mask the test verdict
    } finally {
      apiClient.clearAllTokens();
    }
  });

  it('snapshots a board, lists the version history, and reads the snapshot detail', async function () {
    const createResp = await apiClient.post('/entity_versions', {
      entity_type: 'board',
      entity_id: boardId,
      remark: 'audit-baseline-snapshot'
    });
    expectOk(createResp);
    expect(createResp.data).to.be.an('object');
    expect(createResp.data.id).to.be.a('string').and.not.equal('');
    expect(createResp.data.entity_type).to.equal('board');
    expect(createResp.data.entity_id).to.equal(boardId);
    expect(createResp.data.version_number).to.be.a('number').and.at.least(1);

    const listResp = await apiClient.get('/entity_versions', {
      entity_type: 'board',
      entity_id: boardId,
      page: 1,
      page_size: 10
    });
    expectOk(listResp);
    expect(listResp.data.total).to.be.a('number').and.at.least(1);
    expect(listResp.data.list).to.be.an('array');
    const listedRow = listResp.data.list.find(row => row.id === createResp.data.id);
    expect(listedRow, 'created version must appear in the entity history').to.be.an('object');

    const detailResp = await apiClient.get('/entity_versions/' + createResp.data.id);
    expectOk(detailResp);
    expect(detailResp.data.id).to.equal(createResp.data.id);
    const snapshot = parseSnapshot(detailResp.data);
    expect(snapshot.name, 'snapshot must capture the board name at snapshot time').to.equal(boardNameV1);
  });

  it('restores an earlier snapshot after a mutation and verifies the reversion', async function () {
    const boardNameV2 = uniqueName('audit-versioned-board-v2');
    const updateResp = await apiClient.put('/board', { id: boardId, name: boardNameV2 }, 'tenant_admin');
    expectOk(updateResp);
    expect(updateResp.data.name).to.equal(boardNameV2);

    const v2SnapshotResp = await apiClient.post('/entity_versions', {
      entity_type: 'board',
      entity_id: boardId,
      remark: 'audit-after-mutation'
    });
    expectOk(v2SnapshotResp);

    const listResp = await apiClient.get('/entity_versions', {
      entity_type: 'board',
      entity_id: boardId,
      page: 1,
      page_size: 10
    });
    expectOk(listResp);
    expect(listResp.data.total).to.be.at.least(2);
    const v1Row = listResp.data.list.find(row => row.version_number === 1);
    expect(v1Row, 'version 1 must be present in the history').to.be.an('object');

    // dry_run：只回显将写入的字段，不得改动实体。
    const dryRunResp = await apiClient.post('/entity_versions/' + v1Row.id + '/restore', {
      dry_run: true
    });
    expectOk(dryRunResp);
    expect(dryRunResp.data.dry_run).to.equal(true);
    expect(dryRunResp.data.fields).to.be.an('object');
    expect(dryRunResp.data.fields.name).to.equal(boardNameV1);

    const afterDryRunResp = await apiClient.get('/board/' + boardId, {}, 'tenant_admin');
    expectOk(afterDryRunResp);
    expect(afterDryRunResp.data.name, 'dry run must not mutate the entity').to.equal(boardNameV2);

    // 真实恢复：实体必须回退到 v1 快照时刻的名称。
    const restoreResp = await apiClient.post('/entity_versions/' + v1Row.id + '/restore', {});
    expectOk(restoreResp);
    expect(restoreResp.data.dry_run).to.equal(false);
    expect(restoreResp.data.fields.name).to.equal(boardNameV1);

    const readBackResp = await apiClient.get('/board/' + boardId, {}, 'tenant_admin');
    expectOk(readBackResp);
    expect(readBackResp.data.name, 'board name must revert to the snapshot value').to.equal(boardNameV1);
  });

  it('rejects unsupported entity types with the whitelist message', async function () {
    const resp = await apiClient.post('/entity_versions', {
      entity_type: 'widget',
      entity_id: boardId
    });
    expectBusinessError(resp, CODE_PARAM_ERROR);
    expect(resp.message).to.include('unsupported entity_type');
  });

  it('rejects reads and restores for non-existent version ids', async function () {
    const fakeId = '00000000-0000-0000-0000-0000000000cc';

    const detailResp = await apiClient.get('/entity_versions/' + fakeId);
    expectBusinessError(detailResp, CODE_NOT_FOUND);

    const restoreResp = await apiClient.post('/entity_versions/' + fakeId + '/restore', {});
    expectBusinessError(restoreResp, CODE_NOT_FOUND);
  });

  it('refuses to restore a snapshot whose target entity no longer exists', async function () {
    const doomedName = uniqueName('audit-versioned-doomed');
    const createResp = await apiClient.post('/board', {
      name: doomedName,
      home_flag: 'N'
    }, 'tenant_admin');
    expectOk(createResp);
    const doomedId = createResp.data.id;

    const snapshotResp = await apiClient.post('/entity_versions', {
      entity_type: 'board',
      entity_id: doomedId
    });
    expectOk(snapshotResp);

    const deleteResp = await apiClient.delete('/board/' + doomedId, {}, 'tenant_admin');
    expectOk(deleteResp);

    // 目标实体已被删除：恢复必须以 100404 拒绝，而不是把快照写进虚空。
    const restoreResp = await apiClient.post('/entity_versions/' + snapshotResp.data.id + '/restore', {});
    expectBusinessError(restoreResp, CODE_NOT_FOUND);
    expect(restoreResp.message).to.include('target not found');
  });
});
