/**
 * 文件用途：租户资产层级（ROADMAP C2）业务自动化测试。
 * 核心逻辑：对 /asset 全部六个端点做真实业务闭环验证——创建后重读持久化、
 *           树层级关系、关键字过滤、删除后不可达，以及成环/自父/子节点/
 *           跨租户/无租户等负向分支的显式业务码断言。
 * 关键注意事项：资产是租户作用域资源；跨租户隔离断言使用与 owner 不同层级
 *               分支的租户管理员账号（email_change_tenant，经 C5 验证互不串扰）。
 * 重构建议：后端若引入资产-设备绑定端点，应在本文件同步补充绑定关系闭环用例。
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

function expectAssetShape(row) {
  expect(row).to.be.an('object');
  expect(row.id, 'asset id').to.be.a('string').and.not.equal('');
  expect(row.name, 'asset name').to.be.a('string').and.not.equal('');
  expect(row.tenant_id, 'asset tenant_id').to.be.a('string').and.not.equal('');
  expect(row.asset_type, 'asset_type').to.be.a('string').and.not.equal('');
}

function collectTreeIds(nodes, out = []) {
  for (const node of Array.isArray(nodes) ? nodes : []) {
    expect(node).to.be.an('object');
    expect(node.id, 'tree node id').to.be.a('string').and.not.equal('');
    out.push(node.id);
    if (Array.isArray(node.children)) {
      collectTreeIds(node.children, out);
    }
  }
  return out;
}

function findTreeNode(nodes, id) {
  for (const node of Array.isArray(nodes) ? nodes : []) {
    if (node.id === id) return node;
    const hit = findTreeNode(node.children, id);
    if (hit) return hit;
  }
  return null;
}

describe('Tenant asset management [33_asset_management]', function () {
  this.timeout(30000);

  const createdAssetIds = [];
  let rootAssetId = null;
  let childAssetId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 33_asset_management.test.js; asset coverage requires a healthy API service');
    }
    await apiClient.login('tenant_admin');
    await apiClient.login('tenant_admin_b');
    await apiClient.login('email_change_tenant');
    await apiClient.login('super_admin');
  });

  after(async function () {
    // 叶子优先清理，忽略已删除导致的失败，保证测试残留自清。
    for (const id of createdAssetIds.slice().reverse()) {
      try {
        await apiClient.delete('/asset/' + id, {}, 'tenant_admin');
      } catch (error) {
        // cleanup failure must not mask the test verdict
      }
    }
  });

  it('creates a root asset, re-reads it, and finds it in list and tree', async function () {
    const name = uniqueName('audit-asset-root');
    const createResp = await apiClient.post('/asset', {
      name,
      asset_type: 'site'
    });
    expectOk(createResp);
    expect(createResp.data, 'create payload').to.be.an('object');
    expect(createResp.data.name).to.equal(name);
    expect(createResp.data.asset_type).to.equal('site');
    expect(createResp.data.parent_id).to.equal('');
    expect(createResp.data.id).to.be.a('string').and.not.equal('');
    rootAssetId = createResp.data.id;
    createdAssetIds.push(rootAssetId);

    // 持久化重读：GET 返回与创建一致的值
    const readResp = await apiClient.get('/asset/' + rootAssetId);
    expectOk(readResp);
    expectAssetShape(readResp.data);
    expect(readResp.data.id).to.equal(rootAssetId);
    expect(readResp.data.name).to.equal(name);
    expect(readResp.data.asset_type).to.equal('site');

    // 列表可见
    const listResp = await apiClient.get('/asset/list', { page: 1, page_size: 50 });
    expectOk(listResp);
    expect(listResp.data.total, 'list total').to.be.a('number').and.be.at.least(1);
    expect(listResp.data.list).to.be.an('array');
    const listed = listResp.data.list.find(row => row.id === rootAssetId);
    expect(listed, 'created root asset must appear in list').to.be.an('object');
    expect(listed.name).to.equal(name);

    // 树可见
    const treeResp = await apiClient.get('/asset/tree');
    expectOk(treeResp);
    expect(treeResp.data).to.be.an('array');
    const treeNode = findTreeNode(treeResp.data, rootAssetId);
    expect(treeNode, 'created root asset must appear in tree').to.be.an('object');
    expect(treeNode.name).to.equal(name);
  });

  it('creates a child asset and proves hierarchy in list and tree', async function () {
    expect(rootAssetId, 'root asset from previous case').to.be.a('string').and.not.equal('');
    const childName = uniqueName('audit-asset-child');
    const createResp = await apiClient.post('/asset', {
      name: childName,
      parent_id: rootAssetId,
      asset_type: 'device'
    });
    expectOk(createResp);
    expect(createResp.data.parent_id).to.equal(rootAssetId);
    childAssetId = createResp.data.id;
    createdAssetIds.push(childAssetId);

    const readResp = await apiClient.get('/asset/' + childAssetId);
    expectOk(readResp);
    expect(readResp.data.parent_id).to.equal(rootAssetId);
    expect(readResp.data.name).to.equal(childName);

    const childListResp = await apiClient.get('/asset/list', { parent_id: rootAssetId, page: 1, page_size: 50 });
    expectOk(childListResp);
    expect(childListResp.data.total).to.be.a('number').and.be.at.least(1);
    const childRow = childListResp.data.list.find(row => row.id === childAssetId);
    expect(childRow, 'child asset under parent listing').to.be.an('object');
    expect(childRow.name).to.equal(childName);

    const treeResp = await apiClient.get('/asset/tree');
    expectOk(treeResp);
    const parentNode = findTreeNode(treeResp.data, rootAssetId);
    expect(parentNode, 'parent node in tree').to.be.an('object');
    const childNode = findTreeNode(parentNode.children, childAssetId);
    expect(childNode, 'child node nested under parent in tree').to.be.an('object');
    expect(childNode.name).to.equal(childName);
  });

  it('updates an owned asset and verifies persistence by re-read', async function () {
    expect(childAssetId, 'child asset from previous case').to.be.a('string').and.not.equal('');
    const renamed = uniqueName('audit-asset-renamed');
    const updateResp = await apiClient.put('/asset', {
      id: childAssetId,
      name: renamed,
      parent_id: rootAssetId,
      asset_type: 'device'
    });
    expectOk(updateResp);
    expect(updateResp.data.name).to.equal(renamed);

    const readResp = await apiClient.get('/asset/' + childAssetId);
    expectOk(readResp);
    expect(readResp.data.name).to.equal(renamed);
  });

  it('stores valid meta JSON and rejects invalid meta JSON with a param error', async function () {
    const name = uniqueName('audit-asset-meta');
    const createResp = await apiClient.post('/asset', {
      name,
      meta: JSON.stringify({ floor: 3, zone: 'A' })
    });
    expectOk(createResp);
    const assetId = createResp.data.id;
    createdAssetIds.push(assetId);

    const readResp = await apiClient.get('/asset/' + assetId);
    expectOk(readResp);
    expect(readResp.data.meta, 'meta persisted as JSON text').to.be.a('string');
    const parsedMeta = JSON.parse(readResp.data.meta);
    expect(parsedMeta).to.deep.equal({ floor: 3, zone: 'A' });

    const invalidResp = await apiClient.post('/asset', {
      name: uniqueName('audit-asset-meta-bad'),
      meta: '{not-valid-json'
    });
    expectBusinessError(invalidResp, CODE_PARAM_ERROR);
  });

  it('rejects asset name omission with a required-field error', async function () {
    const resp = await apiClient.post('/asset', { name: '   ' });
    expectBusinessError(resp, CODE_PARAM_ERROR);
    expect(resp.message).to.be.a('string').and.not.equal('');
  });

  it('rejects creating an asset under a non-existent parent with a product error', async function () {
    const resp = await apiClient.post('/asset', {
      name: uniqueName('audit-asset-orphan'),
      parent_id: 'nonexistent-parent-id'
    });
    expectBusinessError(resp, CODE_PARAM_ERROR);
    expect(resp.message).to.be.a('string').and.not.equal('');
  });

  it('rejects self-parent and cycle updates with explicit product errors', async function () {
    expect(rootAssetId).to.be.a('string').and.not.equal('');
    expect(childAssetId).to.be.a('string').and.not.equal('');

    const selfParentResp = await apiClient.put('/asset', {
      id: rootAssetId,
      name: uniqueName('audit-asset-self-parent'),
      parent_id: rootAssetId
    });
    expectBusinessError(selfParentResp, CODE_PARAM_ERROR);

    const cycleResp = await apiClient.put('/asset', {
      id: rootAssetId,
      name: uniqueName('audit-asset-cycle'),
      parent_id: childAssetId
    });
    expectBusinessError(cycleResp, CODE_PARAM_ERROR);

    // 拒绝后父链保持不变，防负向用例污染状态
    const readResp = await apiClient.get('/asset/' + rootAssetId);
    expectOk(readResp);
    expect(readResp.data.parent_id).to.equal('');
  });

  it('blocks deletion of a parent with children, then deletes leaf-first to absence', async function () {
    expect(rootAssetId).to.be.a('string').and.not.equal('');
    expect(childAssetId).to.be.a('string').and.not.equal('');

    const blockedResp = await apiClient.delete('/asset/' + rootAssetId);
    expectBusinessError(blockedResp, CODE_PARAM_ERROR);
    expect(blockedResp.message).to.include('子节点');

    // 阻断后资产仍存在（拒绝删除不得破坏数据）
    const stillThere = await apiClient.get('/asset/' + rootAssetId);
    expectOk(stillThere);

    const deleteChildResp = await apiClient.delete('/asset/' + childAssetId);
    expectOk(deleteChildResp);

    const deleteRootResp = await apiClient.delete('/asset/' + rootAssetId);
    expectOk(deleteRootResp);

    // 删除后不可达：读取必须返回 100404
    const childGone = await apiClient.get('/asset/' + childAssetId);
    expectBusinessError(childGone, CODE_NOT_FOUND);
    const rootGone = await apiClient.get('/asset/' + rootAssetId);
    expectBusinessError(rootGone, CODE_NOT_FOUND);

    createdAssetIds.length = 0;
    rootAssetId = null;
    childAssetId = null;
  });

  it('filters asset list by keyword with response evidence', async function () {
    const name = uniqueName('audit-asset-keyword');
    const createResp = await apiClient.post('/asset', { name });
    expectOk(createResp);
    createdAssetIds.push(createResp.data.id);

    const hitResp = await apiClient.get('/asset/list', { keyword: name, page: 1, page_size: 50 });
    expectOk(hitResp);
    expect(hitResp.data.total).to.be.a('number').and.be.at.least(1);
    const hitRow = hitResp.data.list.find(row => row.id === createResp.data.id);
    expect(hitRow, 'keyword filter must return the created asset').to.be.an('object');
    expect(hitRow.name).to.equal(name);

    const missResp = await apiClient.get('/asset/list', { keyword: 'audit-asset-no-such-keyword-' + Date.now(), page: 1, page_size: 50 });
    expectOk(missResp);
    expect(missResp.data.total).to.equal(0);
    expect(missResp.data.list).to.be.an('array');
    expect(missResp.data.list.map(row => row.id)).to.not.include(createResp.data.id);
  });

  it('keeps asset tree tenant-isolated across tenant admins', async function () {
    const name = uniqueName('audit-asset-isolated');
    const createResp = await apiClient.post('/asset', { name }, 'tenant_admin');
    expectOk(createResp);
    const assetId = createResp.data.id;
    createdAssetIds.push(assetId);
    expect(createResp.data.tenant_id).to.be.a('string').and.not.equal('');

    // 其他租户：单读 100404、列表与树均不可见
    const foreignGet = await apiClient.get('/asset/' + assetId, {}, 'email_change_tenant');
    expectBusinessError(foreignGet, CODE_NOT_FOUND);

    const foreignList = await apiClient.get('/asset/list', { keyword: name, page: 1, page_size: 50 }, 'email_change_tenant');
    expectOk(foreignList);
    expect(foreignList.data.list.map(row => row.id)).to.not.include(assetId);

    const foreignTree = await apiClient.get('/asset/tree', {}, 'email_change_tenant');
    expectOk(foreignTree);
    expect(collectTreeIds(foreignTree.data)).to.not.include(assetId);

    // 第二租户管理员同样不可达，且不能更新/删除他人资产
    const foreignUpdate = await apiClient.put('/asset', { id: assetId, name: uniqueName('audit-asset-hijack') }, 'tenant_admin_b');
    expectBusinessError(foreignUpdate, CODE_NOT_FOUND);

    // owner 侧仍然可见
    const ownerGet = await apiClient.get('/asset/' + assetId, {}, 'tenant_admin');
    expectOk(ownerGet);
    expect(ownerGet.data.name).to.equal(name);
  });

  it('rejects platform-level (no-tenant) asset creation for super admin', async function () {
    const resp = await apiClient.post('/asset', { name: uniqueName('audit-asset-sysadmin') }, 'super_admin');
    expectBusinessError(resp, CODE_PARAM_ERROR);
    expect(resp.message).to.include('平台级');

    const sysListResp = await apiClient.get('/asset/list', { page: 1, page_size: 10 }, 'super_admin');
    expectOk(sysListResp);
    expect(sysListResp.data.total).to.equal(0);
    expect(sysListResp.data.list).to.deep.equal([]);
  });

  it('rejects reads and deletes for non-existent asset ids', async function () {
    const missingGet = await apiClient.get('/asset/nonexistent-asset-id');
    expectBusinessError(missingGet, CODE_NOT_FOUND);

    const missingDelete = await apiClient.delete('/asset/nonexistent-asset-id');
    expectBusinessError(missingDelete, CODE_NOT_FOUND);

    const missingUpdate = await apiClient.put('/asset', {
      id: 'nonexistent-asset-id',
      name: uniqueName('audit-asset-update-missing')
    });
    expectBusinessError(missingUpdate, CODE_NOT_FOUND);
  });
});
