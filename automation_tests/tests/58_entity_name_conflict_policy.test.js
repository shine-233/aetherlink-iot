/**
 * 文件用途：TB-15 实体名冲突策略（对标 ThingsBoard 4.3.0 #14118）自动化契约测试。
 *
 * 核心验证矩阵：
 *  1. Device 设备冲突策略：
 *     - FAIL：同租户同名报错拒绝（100002）
 *     - 缺省策略：默认按 FAIL 处理拦截重名
 *     - RENAME：同名自动更名自增后缀 (1), (2)
 *     - IGNORE：同名静默跳过并返回已有实体
 *     - UPDATE：同名就地覆盖更新已有实体描述与标签并返回
 *     - URL Query 参数兜底生效（?conflict_policy=fail）
 *  2. Board 看板冲突策略：FAIL / RENAME / IGNORE / UPDATE 四种模式闭环
 *  3. DeviceConfig 物模型模板冲突策略：FAIL / RENAME / IGNORE / UPDATE 四种模式闭环
 *  4. Asset 资产树节点冲突策略：FAIL / RENAME / IGNORE / UPDATE 四种模式闭环
 *  5. 严格多租户拓扑隔离：Tenant A 存在同名实体时，Tenant B 即使指定 conflict_policy=fail 也必须成功创建，跨租户绝对互不干扰
 *  6. 严格向后兼容：当指定 conflict_policy=allow 时放行同名
 */

const { expect } = require('chai');
require('../lib/runtime_config');
const apiClient = require('../lib/api_client');
const {
  expectSuccess,
  expectBusinessError
} = require('../lib/response_assertions');

const SUITE = 'TB-15 Entity Name Conflict Resolution Strategy [58_entity_name_conflict_policy]';
const ACCOUNT = 'tenant_admin';
const OTHER_ACCOUNT = 'tenant_admin_b';

const CODE_PARAM_ERROR = 100002;

function pickId(row) {
  return row && (row.id || row.ID);
}

describe(SUITE, function () {
  this.timeout(60000);

  const cleanups = [];
  const suffix = Date.now().toString().slice(-6);

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 58_entity_name_conflict_policy.test.js');
    }
    await apiClient.login(ACCOUNT);
    await apiClient.login(OTHER_ACCOUNT);
  });

  after(async function () {
    for (const action of cleanups.reverse()) {
      try {
        await action();
      } catch (err) {
        // ignore cleanup error
      }
    }
  });

  // ==========================================
  // 1. Device 实体名冲突策略
  // ==========================================
  describe('1. Device (设备) 冲突策略测试', function () {
    const devBaseName = `TB15_Dev_${suffix}`;
    let primaryDevId = null;

    it('1.1 创建基准设备（默认模式）', async function () {
      const resp = await apiClient.post('/device', {
        name: devBaseName,
        description: '初始设备'
      }, ACCOUNT);

      expectSuccess(resp);
      primaryDevId = pickId(resp.data);
      expect(primaryDevId).to.be.a('string');
      expect(resp.data.name).to.equal(devBaseName);

      cleanups.push(() => apiClient.delete(`/device/${primaryDevId}`, {}, ACCOUNT));
    });

    it('1.2 conflict_policy=fail: 同名再次创建应抛错拒绝', async function () {
      const resp = await apiClient.post('/device', {
        name: devBaseName,
        conflict_policy: 'fail'
      }, ACCOUNT);

      expectBusinessError(resp, CODE_PARAM_ERROR);
      expect(resp.message).to.include('already exists');
    });

    it('1.2b 缺省 conflict_policy: 默认按 FAIL 策略拦截同名冲突并返回 100002', async function () {
      const resp = await apiClient.post('/device', {
        name: devBaseName
      }, ACCOUNT);

      expectBusinessError(resp, CODE_PARAM_ERROR);
      expect(resp.message).to.include('already exists');
    });

    it('1.3 conflict_policy=rename: 同名应自动追加递增后缀 (1), (2)', async function () {
      const resp1 = await apiClient.post('/device', {
        name: devBaseName,
        conflict_policy: 'rename'
      }, ACCOUNT);

      expectSuccess(resp1);
      const dev1 = resp1.data;
      expect(dev1.name).to.equal(`${devBaseName} (1)`);
      cleanups.push(() => apiClient.delete(`/device/${pickId(dev1)}`, {}, ACCOUNT));

      const resp2 = await apiClient.post('/device', {
        name: devBaseName,
        conflict_policy: 'rename'
      }, ACCOUNT);

      expectSuccess(resp2);
      const dev2 = resp2.data;
      expect(dev2.name).to.equal(`${devBaseName} (2)`);
      cleanups.push(() => apiClient.delete(`/device/${pickId(dev2)}`, {}, ACCOUNT));
    });

    it('1.4 conflict_policy=ignore: 同名应跳过创建并直接返回已有设备', async function () {
      const resp = await apiClient.post('/device', {
        name: devBaseName,
        description: '被忽略的新属性',
        conflict_policy: 'ignore'
      }, ACCOUNT);

      expectSuccess(resp);
      const dev = resp.data;
      expect(pickId(dev)).to.equal(primaryDevId);
      expect(dev.name).to.equal(devBaseName);
      expect(dev.description).to.equal('初始设备');
    });

    it('1.5 conflict_policy=update: 同名应更新已有设备信息并返回', async function () {
      const updatedDesc = '通过update冲突策略更新的描述';
      const resp = await apiClient.post('/device', {
        name: devBaseName,
        description: updatedDesc,
        conflict_policy: 'update'
      }, ACCOUNT);

      expectSuccess(resp);
      const dev = resp.data;
      expect(pickId(dev)).to.equal(primaryDevId);
      expect(dev.description).to.equal(updatedDesc);
    });

    it('1.6 Query 参数兜底: POST /device?conflict_policy=fail 同名应拒绝', async function () {
      const resp = await apiClient.post('/device?conflict_policy=fail', {
        name: devBaseName
      }, ACCOUNT);

      expectBusinessError(resp, CODE_PARAM_ERROR);
    });

    it('1.7 conflict_policy=allow: 显式放行同名创建（向后兼容行为）', async function () {
      const resp = await apiClient.post('/device', {
        name: devBaseName,
        conflict_policy: 'allow'
      }, ACCOUNT);

      expectSuccess(resp);
      const dev = resp.data;
      expect(pickId(dev)).to.not.equal(primaryDevId);
      expect(dev.name).to.equal(devBaseName);
      cleanups.push(() => apiClient.delete(`/device/${pickId(dev)}`, {}, ACCOUNT));
    });
  });

  // ==========================================
  // 2. Board 看板冲突策略
  // ==========================================
  describe('2. Board (看板) 冲突策略测试', function () {
    const boardBaseName = `TB15_Board_${suffix}`;
    let primaryBoardId = null;

    it('2.1 创建基准看板', async function () {
      const resp = await apiClient.post('/board', {
        name: boardBaseName,
        home_flag: 'N',
        description: '初始看板'
      }, ACCOUNT);

      expectSuccess(resp);
      primaryBoardId = pickId(resp.data);
      expect(primaryBoardId).to.be.a('string');
      expect(resp.data.name).to.equal(boardBaseName);

      cleanups.push(() => apiClient.delete(`/board/${primaryBoardId}`, {}, ACCOUNT));
    });

    it('2.2 Board conflict_policy=fail: 同名再次创建应抛错拒绝', async function () {
      const resp = await apiClient.post('/board', {
        name: boardBaseName,
        home_flag: 'N',
        conflict_policy: 'fail'
      }, ACCOUNT);

      expectBusinessError(resp, CODE_PARAM_ERROR);
      expect(resp.message).to.include('already exists');
    });

    it('2.3 Board conflict_policy=rename: 同名应自动追加递增后缀 (1)', async function () {
      const resp = await apiClient.post('/board', {
        name: boardBaseName,
        home_flag: 'N',
        conflict_policy: 'rename'
      }, ACCOUNT);

      expectSuccess(resp);
      const b1 = resp.data;
      expect(b1.name).to.equal(`${boardBaseName} (1)`);
      cleanups.push(() => apiClient.delete(`/board/${pickId(b1)}`, {}, ACCOUNT));
    });

    it('2.4 Board conflict_policy=ignore: 同名应跳过创建并直接返回已有看板', async function () {
      const resp = await apiClient.post('/board', {
        name: boardBaseName,
        home_flag: 'N',
        description: '被忽略看板描述',
        conflict_policy: 'ignore'
      }, ACCOUNT);

      expectSuccess(resp);
      const b = resp.data;
      expect(pickId(b)).to.equal(primaryBoardId);
      expect(b.name).to.equal(boardBaseName);
      expect(b.description).to.equal('初始看板');
    });

    it('2.5 Board conflict_policy=update: 同名应更新已有看板描述并返回', async function () {
      const updatedDesc = '看板已原地更新';
      const resp = await apiClient.post('/board', {
        name: boardBaseName,
        home_flag: 'N',
        description: updatedDesc,
        conflict_policy: 'update'
      }, ACCOUNT);

      expectSuccess(resp);
      const b = resp.data;
      expect(pickId(b)).to.equal(primaryBoardId);
      expect(b.description).to.equal(updatedDesc);
    });
  });

  // ==========================================
  // 3. DeviceConfig 物模型模板冲突策略
  // ==========================================
  describe('3. DeviceConfig (物模型模板) 冲突策略测试', function () {
    const configBaseName = `TB15_Cfg_${suffix}`;
    let primaryConfigId = null;

    it('3.1 创建基准设备配置', async function () {
      const resp = await apiClient.post('/device_config', {
        name: configBaseName,
        device_type: '1',
        description: '初始配置'
      }, ACCOUNT);

      expectSuccess(resp);
      primaryConfigId = pickId(resp.data);
      expect(primaryConfigId).to.be.a('string');
      expect(resp.data.name).to.equal(configBaseName);

      cleanups.push(() => apiClient.delete(`/device_config/${primaryConfigId}`, {}, ACCOUNT));
    });

    it('3.2 DeviceConfig conflict_policy=fail: 同名再次创建应抛错拒绝', async function () {
      const resp = await apiClient.post('/device_config', {
        name: configBaseName,
        device_type: '1',
        conflict_policy: 'fail'
      }, ACCOUNT);

      expectBusinessError(resp, CODE_PARAM_ERROR);
      expect(resp.message).to.include('already exists');
    });

    it('3.3 DeviceConfig conflict_policy=rename: 同名应自动更名为 (1)', async function () {
      const resp = await apiClient.post('/device_config', {
        name: configBaseName,
        device_type: '1',
        conflict_policy: 'rename'
      }, ACCOUNT);

      expectSuccess(resp);
      const cfg = resp.data;
      expect(cfg.name).to.equal(`${configBaseName} (1)`);
      cleanups.push(() => apiClient.delete(`/device_config/${pickId(cfg)}`, {}, ACCOUNT));
    });

    it('3.4 DeviceConfig conflict_policy=ignore: 同名应返回已有配置', async function () {
      const resp = await apiClient.post('/device_config', {
        name: configBaseName,
        device_type: '1',
        description: '被忽略描述',
        conflict_policy: 'ignore'
      }, ACCOUNT);

      expectSuccess(resp);
      const cfg = resp.data;
      expect(pickId(cfg)).to.equal(primaryConfigId);
      expect(cfg.name).to.equal(configBaseName);
      expect(cfg.description).to.equal('初始配置');
    });

    it('3.5 DeviceConfig conflict_policy=update: 同名应更新已有配置描述', async function () {
      const updatedDesc = '配置已更新';
      const resp = await apiClient.post('/device_config', {
        name: configBaseName,
        device_type: '1',
        description: updatedDesc,
        conflict_policy: 'update'
      }, ACCOUNT);

      expectSuccess(resp);
      const cfg = resp.data;
      expect(pickId(cfg)).to.equal(primaryConfigId);
      expect(cfg.description).to.equal(updatedDesc);
    });
  });

  // ==========================================
  // 4. Asset 资产冲突策略
  // ==========================================
  describe('4. Asset (资产) 冲突策略测试', function () {
    const assetBaseName = `TB15_Asset_${suffix}`;
    let primaryAssetId = null;

    it('4.1 创建基准资产', async function () {
      const resp = await apiClient.post('/asset', {
        name: assetBaseName,
        asset_type: 'device',
        meta: JSON.stringify({ version: '1.0' })
      }, ACCOUNT);

      expectSuccess(resp);
      primaryAssetId = pickId(resp.data);
      expect(primaryAssetId).to.be.a('string');
      expect(resp.data.name).to.equal(assetBaseName);

      cleanups.push(() => apiClient.delete(`/asset/${primaryAssetId}`, {}, ACCOUNT));
    });

    it('4.2 Asset conflict_policy=fail: 同名再次创建应抛错拒绝', async function () {
      const resp = await apiClient.post('/asset', {
        name: assetBaseName,
        asset_type: 'device',
        conflict_policy: 'fail'
      }, ACCOUNT);

      expectBusinessError(resp, CODE_PARAM_ERROR);
      expect(resp.message).to.include('already exists');
    });

    it('4.3 Asset conflict_policy=rename: 同名应自动更名为 (1)', async function () {
      const resp = await apiClient.post('/asset', {
        name: assetBaseName,
        asset_type: 'device',
        conflict_policy: 'rename'
      }, ACCOUNT);

      expectSuccess(resp);
      const a = resp.data;
      expect(a.name).to.equal(`${assetBaseName} (1)`);
      cleanups.push(() => apiClient.delete(`/asset/${pickId(a)}`, {}, ACCOUNT));
    });

    it('4.4 Asset conflict_policy=ignore: 同名应返回已有资产', async function () {
      const resp = await apiClient.post('/asset', {
        name: assetBaseName,
        asset_type: 'device',
        conflict_policy: 'ignore'
      }, ACCOUNT);

      expectSuccess(resp);
      const a = resp.data;
      expect(pickId(a)).to.equal(primaryAssetId);
      expect(a.name).to.equal(assetBaseName);
    });

    it('4.5 Asset conflict_policy=update: 同名应原地更新资产元数据', async function () {
      const resp = await apiClient.post('/asset', {
        name: assetBaseName,
        asset_type: 'substation',
        meta: JSON.stringify({ version: '2.0' }),
        conflict_policy: 'update'
      }, ACCOUNT);

      expectSuccess(resp);
      const a = resp.data;
      expect(pickId(a)).to.equal(primaryAssetId);
      expect(a.asset_type).to.equal('substation');
    });
  });

  // ==========================================
  // 5. Product 产品冲突策略
  // ==========================================
  describe('5. Product (产品) 冲突策略测试', function () {
    const prodBaseName = `TB15_Prod_${suffix}`;
    let primaryProdId = null;

    it('5.1 创建基准产品（默认模式）', async function () {
      const resp = await apiClient.post('/product', {
        name: prodBaseName,
        description: '初始产品',
        product_type: '1'
      }, ACCOUNT);

      expectSuccess(resp);
      primaryProdId = pickId(resp.data);
      expect(primaryProdId).to.be.a('string');
      expect(resp.data.name).to.equal(prodBaseName);

      cleanups.push(() => apiClient.delete(`/product/${primaryProdId}`, {}, ACCOUNT));
    });

    it('5.2 Product conflict_policy=fail: 同名再次创建应抛错拒绝 (100002)', async function () {
      const resp = await apiClient.post('/product', {
        name: prodBaseName,
        conflict_policy: 'fail'
      }, ACCOUNT);

      expectBusinessError(resp, CODE_PARAM_ERROR);
      expect(resp.message).to.include('already exists');
    });

    it('5.2b Product 缺省 conflict_policy: 默认按 FAIL 策略拦截同名冲突', async function () {
      const resp = await apiClient.post('/product', {
        name: prodBaseName
      }, ACCOUNT);

      expectBusinessError(resp, CODE_PARAM_ERROR);
      expect(resp.message).to.include('already exists');
    });

    it('5.3 Product conflict_policy=rename: 同名应自动追加递增后缀 (1), (2)', async function () {
      const resp1 = await apiClient.post('/product', {
        name: prodBaseName,
        conflict_policy: 'rename'
      }, ACCOUNT);

      expectSuccess(resp1);
      const prod1 = resp1.data;
      expect(prod1.name).to.equal(`${prodBaseName} (1)`);
      cleanups.push(() => apiClient.delete(`/product/${pickId(prod1)}`, {}, ACCOUNT));

      const resp2 = await apiClient.post('/product', {
        name: prodBaseName,
        conflict_policy: 'rename'
      }, ACCOUNT);

      expectSuccess(resp2);
      const prod2 = resp2.data;
      expect(prod2.name).to.equal(`${prodBaseName} (2)`);
      cleanups.push(() => apiClient.delete(`/product/${pickId(prod2)}`, {}, ACCOUNT));
    });

    it('5.4 Product conflict_policy=ignore: 同名应跳过创建并直接返回已有产品', async function () {
      const resp = await apiClient.post('/product', {
        name: prodBaseName,
        description: '被忽略的新描述',
        conflict_policy: 'ignore'
      }, ACCOUNT);

      expectSuccess(resp);
      const prod = resp.data;
      expect(pickId(prod)).to.equal(primaryProdId);
      expect(prod.name).to.equal(prodBaseName);
      expect(prod.description).to.equal('初始产品');
    });

    it('5.5 Product conflict_policy=update: 同名应更新已有产品信息并返回', async function () {
      const updatedDesc = '通过update冲突策略更新的产品描述';
      const resp = await apiClient.post('/product', {
        name: prodBaseName,
        description: updatedDesc,
        product_model: 'MOD-999',
        conflict_policy: 'update'
      }, ACCOUNT);

      expectSuccess(resp);
      const prod = resp.data;
      expect(pickId(prod)).to.equal(primaryProdId);
      expect(prod.description).to.equal(updatedDesc);
      expect(prod.product_model).to.equal('MOD-999');
    });

    it('5.6 Product Query 参数兜底: POST /product?conflict_policy=fail 同名应拒绝', async function () {
      const resp = await apiClient.post('/product?conflict_policy=fail', {
        name: prodBaseName
      }, ACCOUNT);

      expectBusinessError(resp, CODE_PARAM_ERROR);
    });

    it('5.7 Product conflict_policy=allow: 显式放行同名创建', async function () {
      const resp = await apiClient.post('/product', {
        name: prodBaseName,
        conflict_policy: 'allow'
      }, ACCOUNT);

      expectSuccess(resp);
      const prod = resp.data;
      expect(pickId(prod)).to.not.equal(primaryProdId);
      expect(prod.name).to.equal(prodBaseName);
      cleanups.push(() => apiClient.delete(`/product/${pickId(prod)}`, {}, ACCOUNT));
    });

    it('5.8 Product CRUD 详情与删除闭环验证', async function () {
      const getResp = await apiClient.get(`/product/${primaryProdId}`, {}, ACCOUNT);
      expectSuccess(getResp);
      expect(getResp.data.name).to.equal(prodBaseName);

      // 创建一个临时产品并删除
      const tempResp = await apiClient.post('/product', {
        name: `Temp_Prod_${suffix}`,
        conflict_policy: 'allow'
      }, ACCOUNT);
      expectSuccess(tempResp);
      const tempId = pickId(tempResp.data);

      const delResp = await apiClient.delete(`/product/${tempId}`, {}, ACCOUNT);
      expectSuccess(delResp);

      // 删除后再次查询应返回 100404 Not Found
      const verifyResp = await apiClient.get(`/product/${tempId}`, {}, ACCOUNT);
      expectBusinessError(verifyResp, 100404);
    });
  });

  // ==========================================
  // 6. 严格多租户拓扑隔离防线
  // ==========================================
  describe('6. 严格多租户隔离防线测试', function () {
    const sharedName = `TB15_TenantCross_${suffix}`;
    let tenantADevId = null;
    let tenantBDevId = null;

    it('6.1 Tenant A 创建实体', async function () {
      const respA = await apiClient.post('/device', {
        name: sharedName,
        description: 'Tenant A 设备'
      }, ACCOUNT);

      expectSuccess(respA);
      tenantADevId = pickId(respA.data);
      expect(tenantADevId).to.be.a('string');
      cleanups.push(() => apiClient.delete(`/device/${tenantADevId}`, {}, ACCOUNT));
    });

    it('6.2 Tenant B 以 conflict_policy=fail 创建同名实体必须成功（租户间隔离）', async function () {
      const respB = await apiClient.post('/device', {
        name: sharedName,
        description: 'Tenant B 设备',
        conflict_policy: 'fail'
      }, OTHER_ACCOUNT);

      expectSuccess(respB);
      tenantBDevId = pickId(respB.data);
      expect(tenantBDevId).to.be.a('string');
      expect(tenantBDevId).to.not.equal(tenantADevId);
      expect(respB.data.name).to.equal(sharedName);
      cleanups.push(() => apiClient.delete(`/device/${tenantBDevId}`, {}, OTHER_ACCOUNT));
    });

    it('6.3 Tenant B 再次创建同名实体以 conflict_policy=fail 应被自己租户拦截', async function () {
      const respB2 = await apiClient.post('/device', {
        name: sharedName,
        conflict_policy: 'fail'
      }, OTHER_ACCOUNT);

      expectBusinessError(respB2, CODE_PARAM_ERROR);
    });

    it('6.4 Tenant A 与 Tenant B 同名产品隔离：跨租户同名互不干扰', async function () {
      const sharedProdName = `TB15_ProdTenant_${suffix}`;
      const respA = await apiClient.post('/product', {
        name: sharedProdName,
        description: 'Tenant A 产品'
      }, ACCOUNT);
      expectSuccess(respA);
      const prodAId = pickId(respA.data);
      cleanups.push(() => apiClient.delete(`/product/${prodAId}`, {}, ACCOUNT));

      const respB = await apiClient.post('/product', {
        name: sharedProdName,
        description: 'Tenant B 产品',
        conflict_policy: 'fail'
      }, OTHER_ACCOUNT);
      expectSuccess(respB);
      const prodBId = pickId(respB.data);
      expect(prodBId).to.not.equal(prodAId);
      cleanups.push(() => apiClient.delete(`/product/${prodBId}`, {}, OTHER_ACCOUNT));
    });
  });
});
