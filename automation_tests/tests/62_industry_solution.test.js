/**
 * 文件用途：TB-19 解决方案模板引擎（一键装一套行业方案）的 API 契约测试。
 *
 * 覆盖：
 *   1. 创建——引用校验（不存在的引用被拒；创建绝不产生副作用实例）；
 *   2. 列表 / 详情（含安装流水回查）；
 *   3. 一键安装——逐项 applied、逐项留流水、目标实例 ID 可追溯；
 *   4. 多租户隔离——他租户对方案不可见、不可安装；
 *   5. 删除与参数校验；
 *   6. 规则链资源类型（TB-19 剩余缺口闭环）——引用/安装实例化新链、源链只读、跨租户引用被拒。
 *
 * 关键注意事项：
 *   - 方案只存引用：安装走资源中心既有应用管道（export→import），
 *     物模型模板导入租户幂等、看板导入每次实例化新看板（与 TB 语义一致）；
 *   - 安装会在本租户产生新实例，cleanup 尽力回收 target_id。
 */

const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const SUITE = 'TB-19 industry solution templates [62_industry_solution]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;
const CODE_NOT_FOUND = 100404;

describe(SUITE, function () {
  this.timeout(180000);

  const cleanups = [];
  let solutionName = null;
  let solutionId = null;
  let templateId = null;
  let boardId = null;
  let ruleChainId = null;
  let installTargets = [];

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 62_industry_solution.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(OTHER_ACCOUNT);

    // 1. 种子看板（tenant A）
    const boardResp = await apiClient.post('/board', {
      name: seedData.makeRunLabel('tb19_board_seed'),
      config: JSON.stringify({ widgets: [{ id: 'w1', type: 'chart' }] }),
      home_flag: 'N',
      menu_flag: 'N',
      description: 'board created for TB-19 solution testing',
      vis_type: 'native',
      type_key: 'automation',
      author: 'TestAuthor',
      version: '1.0.0'
    }, ACCOUNT);
    expect(boardResp.code, 'seed board').to.equal(200);
    boardId = boardResp.data.id;
    cleanups.push(async () => {
      await apiClient.delete('/board/' + boardId, {}, ACCOUNT);
    });

    // 2. 种子物模型模板（tenant A）
    const tplResp = await apiClient.post('/device/template/import', {
      kind: 'aetherlink-device-template',
      name: seedData.makeRunLabel('tb19_tpl_seed'),
      version: '1.0.0',
      type_key: 'automation',
      author: 'TestAuthor',
      description: 'device template created for TB-19 solution testing'
    }, ACCOUNT);
    expect(tplResp.code, 'seed template').to.equal(200);
    templateId = tplResp.data.template ? tplResp.data.template.id : tplResp.data.id;
    cleanups.push(async () => {
      await apiClient.delete('/device/template/' + templateId, {}, ACCOUNT);
    });

    // 3. 种子规则链（tenant A，单触发器节点的最小合法 DAG）
    const rcResp = await apiClient.post('/rule-chains', {
      name: seedData.makeRunLabel('tb19_chain_seed'),
      graph: {
        nodes: [{ id: 't1', type: 'trigger.telemetry', config: {} }],
        edges: []
      }
    }, ACCOUNT);
    expect(rcResp.code, 'seed rule chain').to.equal(200);
    ruleChainId = rcResp.data.id;
    cleanups.push(async () => {
      await apiClient.delete('/rule-chains/' + ruleChainId, {}, ACCOUNT);
    });

    solutionName = seedData.makeRunLabel('tb19_solution');
  });

  after(async function () {
    for (const target of installTargets) {
      try {
        await apiClient.delete(target.path, {}, ACCOUNT);
      } catch (err) { /* 尽力回收 */ }
    }
    for (let i = cleanups.length - 1; i >= 0; i -= 1) {
      try {
        await cleanups[i]();
      } catch (err) { /* 清理失败不掩盖主结果 */ }
    }
    apiClient.clearAllTokens();
  });

  it('creates a solution referencing existing tenant resources without side effects', async function () {
    const resp = await apiClient.post('/solutions', {
      name: solutionName,
      description: 'one-click industry solution for testing',
      resources: [
        { resource_type: 'device_template', resource_id: templateId },
        { resource_type: 'board_template', resource_id: boardId, target_name: 'tb19_installed_board' }
      ]
    }, ACCOUNT);
    expect(resp.code, 'create solution').to.equal(200);
    expect(resp.data.id, 'solution id').to.be.a('string').and.not.equal('');
    expect(resp.data.resources, 'ordered refs kept').to.have.lengthOf(2);
    solutionId = resp.data.id;
    cleanups.push(async () => {
      await apiClient.delete('/solutions/' + solutionId, {}, ACCOUNT);
    });

    // 创建不产生安装副作用：再次安装时才应产生新实例（见后续用例）。
    const detail = await apiClient.get('/solutions/' + solutionId, {}, ACCOUNT);
    expect(detail.code, 'detail after create').to.equal(200);
    expect(detail.data.installs || [], 'no install history yet').to.have.lengthOf(0);
  });

  it('rejects a duplicate solution name in the same tenant', async function () {
    const resp = await apiClient.post('/solutions', {
      name: solutionName,
      resources: [{ resource_type: 'board_template', resource_id: boardId }]
    }, ACCOUNT);
    expect(resp.code, 'duplicate name').to.equal(CODE_PARAM_ERROR);
  });

  it('rejects references to resources that do not exist in the tenant', async function () {
    const resp = await apiClient.post('/solutions', {
      name: seedData.makeRunLabel('tb19_solution_bad'),
      resources: [{ resource_type: 'device_template', resource_id: 'no-such-template' }]
    }, ACCOUNT);
    expect(resp.code, 'unusable reference').to.equal(CODE_PARAM_ERROR);
    expect(resp.message || '', 'error points at the bad item').to.include('resources[0]');
  });

  it('lists solutions inside the tenant only', async function () {
    const listA = await apiClient.get('/solutions', {}, ACCOUNT);
    expect(listA.code, 'list A').to.equal(200);
    const rowsA = listA.data.list || [];
    expect(rowsA.some(row => row.id === solutionId), 'own solution visible').to.equal(true);

    const listB = await apiClient.get('/solutions', {}, OTHER_ACCOUNT);
    expect(listB.code, 'list B').to.equal(200);
    const rowsB = listB.data.list || [];
    expect(rowsB.some(row => row.id === solutionId), 'cross-tenant solutions hidden').to.equal(false);
  });

  it('installs the whole solution with per-item results and audit rows', async function () {
    const resp = await apiClient.post('/solutions/' + solutionId + '/install', {}, ACCOUNT);
    expect(resp.code, 'install').to.equal(200);
    expect(resp.data.total, 'total items').to.equal(2);
    expect(resp.data.applied, 'all applied').to.equal(2);
    expect(resp.data.failed, 'none failed').to.equal(0);

    for (const item of resp.data.items) {
      expect(item.status, 'item applied').to.equal('applied');
      expect(item.target_id, 'target instance recorded').to.be.a('string').and.not.equal('');
      installTargets.push({ path: item.resource_type === 'board_template'
        ? '/board/' + item.target_id
        : '/device/template/' + item.target_id });
    }

    // 安装流水可回查：详情接口的 installs 应有 2 条 applied。
    const detail = await apiClient.get('/solutions/' + solutionId, {}, ACCOUNT);
    expect(detail.code, 'detail after install').to.equal(200);
    const installs = detail.data.installs || [];
    expect(installs.length, 'audit rows persisted').to.be.at.least(2);
    expect(installs.every(row => row.status === 'applied'), 'all audit rows applied').to.equal(true);
  });

  it('hides the solution from other tenants and blocks cross-tenant install', async function () {
    const detail = await apiClient.get('/solutions/' + solutionId, {}, OTHER_ACCOUNT);
    expect(detail.code, 'cross-tenant detail').to.equal(CODE_NOT_FOUND);

    const install = await apiClient.post('/solutions/' + solutionId + '/install', {}, OTHER_ACCOUNT);
    expect(install.code, 'cross-tenant install').to.equal(CODE_NOT_FOUND);
  });

  it('references and installs a rule chain as a solution resource (TB-19 剩余缺口闭环)', async function () {
    // 创建引用规则链的方案：探测走只读导出，不产生副作用实例。
    const create = await apiClient.post('/solutions', {
      name: seedData.makeRunLabel('tb19_solution_rc'),
      resources: [
        { resource_type: 'rule_chain', resource_id: ruleChainId, target_name: 'tb19_installed_chain' }
      ]
    }, ACCOUNT);
    expect(create.code, 'create rule-chain solution').to.equal(200);
    const rcSolutionId = create.data.id;
    cleanups.push(async () => {
      await apiClient.delete('/solutions/' + rcSolutionId, {}, ACCOUNT);
    });

    // 他租户不能引用本租户的规则链（探测路径按租户校验归属）。
    const crossTenant = await apiClient.post('/solutions', {
      name: seedData.makeRunLabel('tb19_solution_rc_b'),
      resources: [{ resource_type: 'rule_chain', resource_id: ruleChainId }]
    }, OTHER_ACCOUNT);
    expect(crossTenant.code, 'cross-tenant rule chain reference').to.equal(CODE_PARAM_ERROR);

    // 安装：实例化一条新链（与看板模板语义一致），源链只读不动。
    const install = await apiClient.post('/solutions/' + rcSolutionId + '/install', {}, ACCOUNT);
    expect(install.code, 'install rule-chain solution').to.equal(200);
    expect(install.data.total, 'total items').to.equal(1);
    expect(install.data.applied, 'applied').to.equal(1);
    const item = install.data.items[0];
    expect(item.status, 'item applied').to.equal('applied');
    expect(item.resource_type, 'resource type kept').to.equal('rule_chain');
    expect(item.target_id, 'new chain id').to.be.a('string').and.not.equal(ruleChainId);
    installTargets.push({ path: '/rule-chains/' + item.target_id });

    // 新链真实落库可读且按 target_name 命名；源链原样可读。
    const installed = await apiClient.get('/rule-chains/' + item.target_id, {}, ACCOUNT);
    expect(installed.code, 'installed chain readable').to.equal(200);
    expect(installed.data.name, 'installed chain named by target_name').to.equal('tb19_installed_chain');

    const source = await apiClient.get('/rule-chains/' + ruleChainId, {}, ACCOUNT);
    expect(source.code, 'source chain untouched').to.equal(200);
  });

  it('deletes the solution and validates parameters', async function () {
    expectBusinessCode(await apiClient.post('/solutions', {
      name: seedData.makeRunLabel('tb19_solution_empty'),
      resources: []
    }, ACCOUNT), CODE_PARAM_ERROR);

    const ghost = await apiClient.get('/solutions/00000000-0000-0000-0000-00000000nope', {}, ACCOUNT);
    expect(ghost.code, 'unknown solution').to.equal(CODE_NOT_FOUND);

    const del = await apiClient.delete('/solutions/' + solutionId, {}, ACCOUNT);
    expect(del.code, 'delete own solution').to.equal(200);
    const after = await apiClient.get('/solutions/' + solutionId, {}, ACCOUNT);
    expect(after.code, 'deleted solution gone').to.equal(CODE_NOT_FOUND);
    solutionId = null;
  });
});

function expectBusinessCode(resp, code) {
  expect(resp, 'response envelope').to.be.an('object');
  expect(resp.code, `expected ${code} but got ${resp.code}: ${resp.message || ''}`).to.equal(code);
}
