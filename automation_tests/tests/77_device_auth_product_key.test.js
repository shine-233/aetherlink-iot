/**
 * 文件用途：一型一密动态注册的产品级交叉校验（TP-05 安全修复）活栈契约测试。
 *
 * 背景：修复前，POST /api/v1/device/auth 只校验 product_key 对应产品"存在"，
 * 不校验产品与命中设备档案的租户/绑定关系——攻击者可用 A 租户的档案密钥
 * 把设备注册到任意（甚至跨租户的）产品下。
 *
 * 覆盖：
 *   1. 档案种子：创建设备配置并开启 auto_register + template_secret；
 *   2. 跨租户 product_key 被拒绝（错误码 200087）——修复前该请求会成功建档；
 *   3. 同租户 product_key 正常注册（正向对照），设备成功创建；
 *   4. 清理种子数据（设备/配置/产品）。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Device Auth Product Cross-Check [77_device_auth_product_key]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';
const CODE_PRODUCT_MISMATCH = 200087;

const SUFFIX = String(Date.now());

describe(SUITE, function () {
  this.timeout(180000);

  let configId = null;
  let templateSecret = 'tp05_secret_' + SUFFIX;
  let tenantADeviceId = null;
  /** 产品登记：{ id, owner }，owner 用于按租户清理。 */
  const productRegistry = [];

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 77_device_auth_product_key.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
  });

  after(async function () {
    if (tenantADeviceId) {
      try {
        await apiClient.delete('/device/' + tenantADeviceId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
    for (const product of productRegistry) {
      try {
        await apiClient.delete('/product/' + product.id, {}, product.owner);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
    if (configId) {
      try {
        await apiClient.delete('/device/config/' + configId, {}, TENANT_A);
      } catch (e) {
        /* ignore cleanup error */
      }
    }
  });

  it('1. 种子：创建设备配置并开启一型一密自动注册', async function () {
    const create = await apiClient.post(
      '/device/config',
      { name: 'TP05_Config_' + SUFFIX, device_type: '1' },
      TENANT_A
    );
    expect(create.code, JSON.stringify(create)).to.equal(200);
    configId = create.data && (create.data.id || (create.data.data && create.data.data.id));
    expect(configId).to.be.a('string').and.not.equal('');

    const update = await apiClient.put(
      '/device/config',
      { id: configId, auto_register: 1, template_secret: templateSecret },
      TENANT_A
    );
    expect(update.code, JSON.stringify(update)).to.equal(200);
  });

  it('2. 种子：租户 A 与租户 B 各创建一个产品', async function () {
    const prodA = await apiClient.post(
      '/product',
      { name: 'TP05_ProdA_' + SUFFIX, product_type: '1' },
      TENANT_A
    );
    expect(prodA.code, JSON.stringify(prodA)).to.equal(200);
    const prodAId = prodA.data && (prodA.data.id || (prodA.data.data && prodA.data.data.id));
    expect(prodAId).to.be.a('string');
    productRegistry.push({ id: prodAId, owner: TENANT_A });
    expect(prodA.data.product_key || (prodA.data.data && prodA.data.data.product_key)).to.be.a('string');

    const prodB = await apiClient.post(
      '/product',
      { name: 'TP05_ProdB_' + SUFFIX, product_type: '1' },
      TENANT_B
    );
    expect(prodB.code, JSON.stringify(prodB)).to.equal(200);
    // 租户 B 的产品仅用于制造跨租户 product_key，清理时用租户 B 账号删除。
    const prodBId = prodB.data && (prodB.data.id || (prodB.data.data && prodB.data.data.id));
    expect(prodBId).to.be.a('string');
    productRegistry.push({ id: prodBId, owner: TENANT_B });
  });

  it('3. 跨租户 product_key 注册被拒绝（200087，修复前会成功建档）', async function () {
    const crossTenantKey = await tenantBProductKey();
    expect(crossTenantKey).to.be.a('string').and.not.equal('');

    const res = await apiClient.post('/device/auth', {
      template_secret: templateSecret,
      product_key: crossTenantKey,
      device_number: 'TP05_CROSS_' + SUFFIX,
      device_name: 'TP05 跨租户负向用例'
    });
    expect(res.code, JSON.stringify(res)).to.equal(CODE_PRODUCT_MISMATCH);
  });

  it('4. 同租户 product_key 正常注册（正向对照）', async function () {
    const res = await apiClient.post('/device/auth', {
      template_secret: templateSecret,
      product_key: await tenantAProductKey(),
      device_number: 'TP05_SAME_' + SUFFIX,
      device_name: 'TP05 正向用例'
    });
    expect(res.code, JSON.stringify(res)).to.equal(200);
    expect(res.data).to.be.an('object');
    expect(res.data.device_id).to.be.a('string').and.not.equal('');
    tenantADeviceId = res.data.device_id;
  });

  /** 登记在用例 2 里的租户 A 产品 key。 */
  let tenantAKey = null;
  async function tenantAProductKey() {
    if (tenantAKey) return tenantAKey;
    const list = await apiClient.get('/product', { page: 1, page_size: 50 }, TENANT_A);
    expect(list.code).to.equal(200);
    const rows = (list.data && list.data.list) || [];
    const hit = rows.find((row) => row.name === 'TP05_ProdA_' + SUFFIX);
    expect(hit, 'tenant A product should be listed').to.be.an('object');
    tenantAKey = hit.product_key;
    return tenantAKey;
  }

  /** 登记在用例 2 里的租户 B 产品 key。 */
  async function tenantBProductKey() {
    const list = await apiClient.get('/product', { page: 1, page_size: 50 }, TENANT_B);
    expect(list.code).to.equal(200);
    const rows = (list.data && list.data.list) || [];
    const hit = rows.find((row) => row.name === 'TP05_ProdB_' + SUFFIX);
    expect(hit, 'tenant B product should be listed').to.be.an('object');
    return hit.product_key;
  });
});
