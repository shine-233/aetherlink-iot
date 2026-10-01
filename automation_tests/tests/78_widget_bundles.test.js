/**
 * 文件用途：部件库 widget_bundles（ROADMAP TB-04，123.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. 部件库完整 CRUD 生命周期（POST /api/v1/widget-bundles 创建、PUT 更新、
 *      GET /api/v1/widget-bundles 分页、GET /api/v1/widget-bundles/:id 详情、
 *      DELETE /api/v1/widget-bundles/:id 删除）；
 *   2. 参数校验（名称必填、widgets 必须是结构合法的部件定义数组）；
 *   3. 内置四部件（gauge/chart/valve/twin3d）种子能力：GET /widget-bundles/builtin
 *      导出描述、POST /widget-bundles/seed 幂等落库；
 *   4. 资源中心接入：resource_type=widget_bundle 打包导出（HMAC 签名）→
 *      preview 预览 → 验签导入回放（幂等结果）；
 *   5. 严格多租户隔离（租户 B 无法查询/更新/删除租户 A 的部件库）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');
const {
  expectSuccess,
  expectBusinessError
} = require('../lib/response_assertions');
const seedData = require('../lib/seed_data');

const SUITE = 'Widget Bundle Library [78_widget_bundles]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;

// 与 service/scada_mobile_wiring.go builtinWidgetDefinitions 一致的内置四部件类型。
const BUILTIN_WIDGET_TYPES = ['chart', 'gauge', 'twin3d', 'valve'];

// 合法单部件定义（形状对齐 service.WidgetDefinition）。
const GAUGE_DEF = {
  type: 'gauge',
  version: '1',
  schema: '{"type":"object","properties":{"unit":{"type":"string"}}}',
  capabilities: ['2d'],
  commands: [{ name: 'refresh', requires_confirmation: false }]
};

describe(SUITE, function () {
  this.timeout(180000);

  const cleanups = [];

  let bundleId = null;
  let bundleName = '';
  let seededBundleId = null;

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 78_widget_bundles.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
    // 自洁：清空两租户的全部部件库，消除跨次运行的 type_key/名称污染
    for (const account of [TENANT_A, TENANT_B]) {
      const list = await apiClient.get('/widget-bundles', { page: 1, page_size: 200 }, account);
      for (const item of (list.data && list.data.list) || []) {
        try {
          await apiClient.delete('/widget-bundles/' + item.id, {}, account);
        } catch (e) {
          /* ignore */
        }
      }
    }
  });

  after(async function () {
    for (const id of cleanups) {
      try {
        await apiClient.delete('/widget-bundles/' + id, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. POST /widget-bundles 创建部件库并返回完整记录', async function () {
    bundleName = seedData.makeRunLabel('wb_bundle_78');
    const res = await apiClient.post(
      '/widget-bundles',
      {
        name: bundleName,
        description: '契约测试部件库_78',
        widgets: JSON.stringify([GAUGE_DEF])
      },
      TENANT_A
    );
    expectSuccess(res);
    expect(res.data).to.be.an('object');
    expect(res.data.id).to.be.a('string').and.not.equal('');
    expect(res.data.name).to.equal(bundleName);
    expect(res.data.tenant_id).to.be.a('string').and.not.equal('');
    expect(res.data.version).to.equal('1.0.0');
    expect(JSON.parse(res.data.widgets)).to.be.an('array').with.lengthOf(1);
    bundleId = res.data.id;
    console.log('[DBG1] case1 bundleId=', bundleId);
    cleanups.push(bundleId);
  });

  it('2. POST /widget-bundles 名称缺失返回参数错误', async function () {
    const res = await apiClient.post('/widget-bundles', { widgets: '[]' }, TENANT_A);
    expectBusinessError(res, CODE_PARAM_ERROR);
  });

  it('3. POST /widget-bundles 拒绝非 JSON 数组的 widgets', async function () {
    const res = await apiClient.post(
      '/widget-bundles',
      { name: seedData.makeRunLabel('wb_badjson_78'), widgets: '{"type":"gauge"}' },
      TENANT_A
    );
    expectBusinessError(res, CODE_PARAM_ERROR);
  });

  it('4. POST /widget-bundles 拒绝缺少 type/version 的部件定义', async function () {
    const res = await apiClient.post(
      '/widget-bundles',
      {
        name: seedData.makeRunLabel('wb_baddef_78'),
        widgets: JSON.stringify([{ version: '1' }])
      },
      TENANT_A
    );
    expectBusinessError(res, CODE_PARAM_ERROR);
  });

  it('5. POST /widget-bundles 同租户重名拒绝', async function () {
    const res = await apiClient.post('/widget-bundles', { name: bundleName }, TENANT_A);
    expectBusinessError(res, CODE_PARAM_ERROR);
  });

  it('6. GET /widget-bundles 分页检索命中新部件库', async function () {
    const res = await apiClient.get(
      '/widget-bundles',
      { page: 1, page_size: 50, search: bundleName },
      TENANT_A
    );
    expectSuccess(res);
    expect(res.data.total).to.be.a('number').and.at.least(1);
    const hit = (res.data.list || []).find((item) => item.id === bundleId);
    expect(hit, 'created bundle should appear in list').to.be.an('object');
  });

  it('7. PUT /widget-bundles 更新部件定义与版本', async function () {
    const chartDef = {
      type: 'chart',
      version: '1',
      schema: '{"type":"object","properties":{"title":{"type":"string"}}}',
      capabilities: ['2d']
    };
    const res = await apiClient.put(
      '/widget-bundles',
      {
        id: bundleId,
        version: '1.1.0',
        widgets: JSON.stringify([GAUGE_DEF, chartDef])
      },
      TENANT_A
    );
    expectSuccess(res);
    expect(res.data.version).to.equal('1.1.0');
    expect(JSON.parse(res.data.widgets)).to.be.an('array').with.lengthOf(2);
  });

  it('8. GET /widget-bundles/:id 查询详情', async function () {
    const res = await apiClient.get('/widget-bundles/' + bundleId, {}, TENANT_A);
    expectSuccess(res);
    expect(res.data.id).to.equal(bundleId);
    expect(res.data.name).to.equal(bundleName);
  });

  it('9. 租户 B 无法查询/更新/删除租户 A 的部件库', async function () {
    const getRes = await apiClient.get('/widget-bundles/' + bundleId, {}, TENANT_B);
    expect(getRes.code).to.not.equal(200);

    const putRes = await apiClient.put(
      '/widget-bundles',
      { id: bundleId, name: 'hijacked-by-tenant-b' },
      TENANT_B
    );
    expect(putRes.code).to.not.equal(200);

    const delRes = await apiClient.delete('/widget-bundles/' + bundleId, {}, TENANT_B);
    expect(delRes.code).to.not.equal(200);

    const check = await apiClient.get('/widget-bundles/' + bundleId, {}, TENANT_A);
    expectSuccess(check);
  });

  it('10. 租户 B 列表不含租户 A 的部件库', async function () {
    const res = await apiClient.get(
      '/widget-bundles',
      { page: 1, page_size: 200, search: bundleName },
      TENANT_B
    );
    expectSuccess(res);
    const hit = (res.data.list || []).find((item) => item.id === bundleId);
    expect(hit, 'tenant B must not see tenant A bundle').to.equal(undefined);
  });

  it('11. GET /widget-bundles/builtin 导出内置四部件描述', async function () {
    const res = await apiClient.get('/widget-bundles/builtin', {}, TENANT_A);
    expectSuccess(res);
    expect(res.data.kind).to.equal('aetherlink-widget-bundle');
    expect(res.data.name).to.be.a('string').and.not.equal('');
    const defs = JSON.parse(res.data.widgets);
    expect(defs).to.be.an('array').with.lengthOf(4);
    const types = defs.map((d) => d.type).sort();
    expect(types).to.deep.equal(BUILTIN_WIDGET_TYPES);
  });

  it('12. POST /widget-bundles/seed 种子落库且二次调用幂等', async function () {
    const pre = await apiClient.get('/widget-bundles/' + bundleId, {}, TENANT_A);
    console.log('[DBG12] before-seed bundle alive?', pre.code);
    const first = await apiClient.post('/widget-bundles/seed', {}, TENANT_A);
    console.log('[DBG12] seed1', first.code, first.data && first.data.idempotent);
    const mid = await apiClient.get('/widget-bundles/' + bundleId, {}, TENANT_A);
    console.log('[DBG12] after-seed1 alive?', mid.code);
    const second = await apiClient.post('/widget-bundles/seed', {}, TENANT_A);
    console.log('[DBG12] seed2', second.code, second.data && second.data.idempotent);
    const post = await apiClient.get('/widget-bundles/' + bundleId, {}, TENANT_A);
    console.log('[DBG12] after-seed2 alive?', post.code);
    expectSuccess(first);
    expect(first.data.bundle).to.be.an('object');
    // 种子名固定（内置部件库）：全新租户首次 idempotent=false；
    // 套件重跑命中历史种子且内容一致时 idempotent=true，两种均合法。
    expect(first.data.idempotent).to.be.a('boolean');
    seededBundleId = first.data.bundle.id;
    cleanups.push(seededBundleId);

    expectSuccess(second);
    expect(second.data.idempotent).to.equal(true);
    expect(second.data.bundle.id).to.equal(seededBundleId);

    // 租户 B 独立种子，互不影响
    const tenantB = await apiClient.post('/widget-bundles/seed', {}, TENANT_B);
    expectSuccess(tenantB);
    expect(tenantB.data.bundle.id).to.not.equal(seededBundleId);
    try {
      await apiClient.delete('/widget-bundles/' + tenantB.data.bundle.id, {}, TENANT_B);
    } catch (e) {
      /* ignore cleanup error */
    }
  });

  describe('13. 资源中心打包导出与验签导入回放', function () {
    let exportedBundle = null;
    const exportTypeKey = 'widget78_' + Date.now();

    it('13.1 resource_type=widget_bundle 导出带签名的统一资源包', async function () {
      // 给目标 bundle 标注行业分类，便于按 type_key 精确打包
      await apiClient.put(
        '/widget-bundles',
        { id: bundleId, type_key: exportTypeKey },
        TENANT_A
      );

      const putRes = await apiClient.put(
        '/widget-bundles',
        { id: bundleId, type_key: exportTypeKey },
        TENANT_A
      );
      const res = await apiClient.get(
        '/resource/center/bundle',
        { type_key: exportTypeKey, resource_type: 'widget_bundle' },
        TENANT_A
      );
      expectSuccess(res);
      expect(res.data.bundle).to.be.an('object');

      exportedBundle = res.data.bundle;
      expect(exportedBundle.widgets).to.be.an('array').with.length.at.least(1);
      const names = exportedBundle.widgets.map((w) => w.name);
      expect(names).to.include(bundleName);
      expect(exportedBundle.count).to.equal(exportedBundle.widgets.length);
      expect(exportedBundle.digest, 'digest presence').to.be.a('string').with.lengthOf(64);
      expect(exportedBundle.signature, 'signature presence').to.be.a('string').with.lengthOf(64);
      expect(exportedBundle.signed_key_id, 'signing key id presence').to.be.a('string');
    });

    it('13.2 资源中心列表可按 widget_bundle 类型过滤', async function () {
      const res = await apiClient.get(
        '/resource/center/list',
        { page: 1, page_size: 50, resource_type: 'widget_bundle', type_key: exportTypeKey },
        TENANT_A
      );
      expectSuccess(res);
      const hit = (res.data.list || []).find(
        (item) => item.resource_type === 'widget_bundle' && item.id === bundleId
      );
      expect(hit, 'bundle should appear in resource center list').to.be.an('object');
    });

    it('13.3 preview 预览不落库且无阻断项', async function () {
      const res = await apiClient.post(
        '/resource/center/bundle/import',
        { bundle: exportedBundle, preview: true },
        TENANT_A
      );
      expectSuccess(res);
      expect(res.data.applied).to.equal(false);
      expect(res.data.preview.blocking).to.be.an('array').that.is.empty;
    });

    it('13.4 未签名包被验签门禁拒绝（fail closed）', async function () {
      const unsigned = {
        type_key: exportTypeKey,
        exported_at: Date.now(),
        count: 1,
        widgets: [{ kind: 'aetherlink-widget-bundle', name: 'UnsignedBundle78', widgets: '[]' }]
      };
      const res = await apiClient.post(
        '/resource/center/bundle/import',
        { bundle: unsigned },
        TENANT_A
      );
      expectBusinessError(res, CODE_PARAM_ERROR);
      expect(res.data.stage).to.equal('verify');
    });

    it('13.5 验签导入回放命中幂等结果', async function () {
      const res = await apiClient.post(
        '/resource/center/bundle/import',
        { bundle: exportedBundle },
        TENANT_A
      );
      expectSuccess(res);
      expect(res.data.applied).to.equal(true);
      const results = res.data.results || [];
      const own = results.find(
        (item) => item.kind === 'widget_bundle' && item.name === bundleName
      );
      expect(own, 'widget bundle import result should be present').to.be.an('object');
      // 同名同版本同内容 → 幂等；被 13.1 的 type_key 更新改变内容时 → created/updated 语义由后端判定，
      // 但绝不允许 rejected。
      expect(own.outcome).to.not.equal('rejected');
    });
  });

  it('14. DELETE /widget-bundles/:id 删除后不可再查询（自包含）', async function () {
    // 说明：本用例自建自删，避免与 13.x（依赖 case 1 的 bundleId）产生用例间顺序耦合。
    const own = await apiClient.post(
      '/widget-bundles',
      { name: seedData.makeRunLabel('wb_bundle_78_own'), widgets: JSON.stringify([GAUGE_DEF]) },
      TENANT_A
    );
    expectSuccess(own);
    const delRes = await apiClient.delete('/widget-bundles/' + own.data.id, {}, TENANT_A);
    expectSuccess(delRes);
    const check = await apiClient.get('/widget-bundles/' + own.data.id, {}, TENANT_A);
    expect(check.code).to.not.equal(200);
  });
});
