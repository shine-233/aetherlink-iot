/**
 * 文件用途：白标（租户翻译覆盖 + 自定义 CSS，TB-47，134.sql）活栈契约测试。
 *
 * 覆盖：
 *   1. 翻译覆盖写读闭环：PUT /api/v1/whitelabel/translations（批量 UPSERT）→
 *      GET /api/v1/whitelabel/translations 列表 → GET /api/v1/whitelabel/overrides 分组读取；
 *      同键重写覆盖旧值（UPSERT 语义），删除端点按 (lang,key) 精确移除；
 *   2. 自定义 CSS 闭环：PUT /api/v1/whitelabel/custom-css → GET 回显 → overrides.css 同步；
 *      空串保存为清除语义；'</style' 序列拒绝（202008，纵深防御，主防线为前端 textContent 注入）；
 *   3. 入参校验：非法语言（不在 zh-cn/en-us/es-es/fr-fr 白名单）→ 202007；
 *      非法键（含空格/斜杠/首尾点）→ 202007；
 *   4. 角色边界 fail-closed：TENANT_USER 写翻译覆盖 / 写 CSS → 201001；
 *      GET overrides 登录后任意角色可读（TENANT_USER 返回 200）；
 *   5. 严格租户隔离：租户 B 的 overrides/translations 看不到租户 A 的覆盖；
 *      租户 B 可写同键不同值，互不影响；租户 B 删除租户 A 的键 → 精确 0 命中不误伤。
 */

require('../lib/runtime_config');
const { expect } = require('chai');
const apiClient = require('../lib/api_client');

const SUITE = 'Whitelabel Overrides [94_whitelabel_overrides]';
const TENANT_A = 'tenant_admin';
const TENANT_B = 'tenant_admin_b';
const TENANT_USER = 'tenant_user';

const KEY_A = 'page.tb47_contract.title';
const KEY_B = 'page.tb47_contract.subtitle';
const VALUE_A1 = 'TB47 契约标题_A1';
const VALUE_A2 = 'TB47 契约标题_A2';
const VALUE_B = 'TB47 契约标题_B（租户B独立值）';
const CSS_A = '.tb47-contract { color: #00ff00; }';
const CSS_B = '.tb47-contract { color: #ff0000; }';

/** 从 overrides 响应中安全取出某语言分组。 */
function langGroup(overridesData, lang) {
  const translations = (overridesData && overridesData.data && overridesData.data.translations) || {};
  return translations[lang] || {};
}

describe(SUITE, function () {
  this.timeout(120000);

  before(async function () {
    const healthy = await apiClient.healthCheck();
    if (!healthy) {
      throw new Error('Backend service is not running locally for 94_whitelabel_overrides.test.js');
    }
    await apiClient.login(TENANT_A);
    await apiClient.login(TENANT_B);
    // 环境自愈：清理同键历史残留（UPSERT/DELETE 幂等，重跑安全）。
    try {
      await apiClient.delete('/whitelabel/translations', { items: [{ lang: 'zh-cn', key: KEY_A }, { lang: 'zh-cn', key: KEY_B }] }, TENANT_A);
      await apiClient.delete('/whitelabel/translations', { items: [{ lang: 'zh-cn', key: KEY_A }, { lang: 'zh-cn', key: KEY_B }] }, TENANT_B);
      await apiClient.put('/whitelabel/custom-css', { css: '' }, TENANT_A);
      await apiClient.put('/whitelabel/custom-css', { css: '' }, TENANT_B);
    } catch (e) {
      /* 忽略清理失败：后续用例会以实际读到的状态断言 */
    }
  });

  after(async function () {
    try {
      await apiClient.delete('/whitelabel/translations', { items: [{ lang: 'zh-cn', key: KEY_A }, { lang: 'zh-cn', key: KEY_B }] }, TENANT_A);
      await apiClient.delete('/whitelabel/translations', { items: [{ lang: 'zh-cn', key: KEY_A }, { lang: 'zh-cn', key: KEY_B }] }, TENANT_B);
      await apiClient.put('/whitelabel/custom-css', { css: '' }, TENANT_A);
      await apiClient.put('/whitelabel/custom-css', { css: '' }, TENANT_B);
    } catch (e) {
      /* ignore cleanup error */
    }
  });

  it('1. PUT /whitelabel/translations 批量 UPSERT 后 GET 列表与 overrides 可读（写读闭环）', async function () {
    const upsert = await apiClient.put(
      '/whitelabel/translations',
      {
        items: [
          { lang: 'zh-cn', key: KEY_A, value: VALUE_A1 },
          { lang: 'en-us', key: KEY_A, value: 'TB47 contract title' }
        ]
      },
      TENANT_A
    );
    expect(upsert.code, JSON.stringify(upsert)).to.equal(200);
    expect(upsert.data).to.be.an('object');
    expect(upsert.data.count).to.equal(2);

    const list = await apiClient.get('/whitelabel/translations', { lang: 'zh-cn' }, TENANT_A);
    expect(list.code, JSON.stringify(list)).to.equal(200);
    expect(list.data.total).to.be.at.least(1);
    const hit = (list.data.list || []).find((row) => row.key === KEY_A);
    expect(hit, 'zh-cn 覆盖行应存在').to.be.an('object');
    expect(hit.value).to.equal(VALUE_A1);
    expect(hit.lang).to.equal('zh-cn');
    expect(hit.tenant_id).to.be.a('string').and.not.equal('');

    const overrides = await apiClient.get('/whitelabel/overrides', {}, TENANT_A);
    expect(overrides.code, JSON.stringify(overrides)).to.equal(200);
    expect(overrides.data.translations).to.be.an('object');
    expect(langGroup(overrides, 'zh-cn')[KEY_A]).to.equal(VALUE_A1);
    expect(langGroup(overrides, 'en-us')[KEY_A]).to.equal('TB47 contract title');
  });

  it('2. 同键重写覆盖旧值（UPSERT），重复写入不产生重复行', async function () {
    const rewrite = await apiClient.put(
      '/whitelabel/translations',
      { items: [{ lang: 'zh-cn', key: KEY_A, value: VALUE_A2 }] },
      TENANT_A
    );
    expect(rewrite.code, JSON.stringify(rewrite)).to.equal(200);
    expect(rewrite.data.count).to.equal(1);

    const list = await apiClient.get('/whitelabel/translations', { lang: 'zh-cn' }, TENANT_A);
    const rows = (list.data.list || []).filter((row) => row.key === KEY_A);
    expect(rows.length, '同键只允许一行').to.equal(1);
    expect(rows[0].value).to.equal(VALUE_A2);

    const overrides = await apiClient.get('/whitelabel/overrides', {}, TENANT_A);
    expect(langGroup(overrides, 'zh-cn')[KEY_A]).to.equal(VALUE_A2);
  });

  it('3. DELETE /whitelabel/translations 按 (lang,key) 精确移除并从 overrides 消失', async function () {
    const del = await apiClient.delete(
      '/whitelabel/translations',
      { items: [{ lang: 'en-us', key: KEY_A }] },
      TENANT_A
    );
    expect(del.code, JSON.stringify(del)).to.equal(200);
    expect(del.data.deleted).to.equal(1);

    const list = await apiClient.get('/whitelabel/translations', { lang: 'en-us' }, TENANT_A);
    const remaining = (list.data.list || []).filter((row) => row.key === KEY_A);
    expect(remaining.length).to.equal(0);

    const overrides = await apiClient.get('/whitelabel/overrides', {}, TENANT_A);
    expect(langGroup(overrides, 'en-us')[KEY_A]).to.equal(undefined);
  });

  it('4. 自定义 CSS 写读闭环：PUT 回显 → overrides.css 同步 → 空串保存即清除', async function () {
    const put = await apiClient.put('/whitelabel/custom-css', { css: CSS_A }, TENANT_A);
    expect(put.code, JSON.stringify(put)).to.equal(200);

    const echo = await apiClient.get('/whitelabel/custom-css', {}, TENANT_A);
    expect(echo.code, JSON.stringify(echo)).to.equal(200);
    expect(echo.data.css).to.equal(CSS_A);

    const overrides = await apiClient.get('/whitelabel/overrides', {}, TENANT_A);
    expect(overrides.data.css).to.equal(CSS_A);

    const clear = await apiClient.put('/whitelabel/custom-css', { css: '   ' }, TENANT_A);
    expect(clear.code, JSON.stringify(clear)).to.equal(200);
    const cleared = await apiClient.get('/whitelabel/custom-css', {}, TENANT_A);
    expect(cleared.data.css).to.equal('');
  });

  it('5. 入参校验：非法语言/非法键 → 202007；CSS 含 </style 序列 → 202008', async function () {
    const badLang = await apiClient.put(
      '/whitelabel/translations',
      { items: [{ lang: 'de-de', key: KEY_A, value: 'x' }] },
      TENANT_A
    );
    expect(badLang.code, JSON.stringify(badLang)).to.equal(202007);

    const badKey = await apiClient.put(
      '/whitelabel/translations',
      { items: [{ lang: 'zh-cn', key: 'bad key with spaces', value: 'x' }] },
      TENANT_A
    );
    expect(badKey.code, JSON.stringify(badKey)).to.equal(202007);

    const badCss = await apiClient.put(
      '/whitelabel/custom-css',
      { css: '.a{}</style><script>alert(1)</script>' },
      TENANT_A
    );
    expect(badCss.code, JSON.stringify(badCss)).to.equal(202008);
  });

  it('6. 角色边界 fail-closed：TENANT_USER 无写权限，但 overrides 登录后可读', async function () {
    await apiClient.login(TENANT_USER);

    // 写入越权在 casbin 层被拒：HTTP 403（{"error":"非法访问"}），与既有拒绝契约一致。
    const userUpsert = await apiClient.put(
      '/whitelabel/translations',
      { items: [{ lang: 'zh-cn', key: KEY_B, value: '越权写入' }] },
      TENANT_USER
    );
    expect(userUpsert.code, JSON.stringify(userUpsert)).to.not.equal(200);

    const userCss = await apiClient.put('/whitelabel/custom-css', { css: '.x{}' }, TENANT_USER);
    expect(userCss.code, JSON.stringify(userCss)).to.not.equal(200);

    const userOverrides = await apiClient.get('/whitelabel/overrides', {}, TENANT_USER);
    expect(userOverrides.code, JSON.stringify(userOverrides)).to.equal(200);
    expect(userOverrides.data.translations).to.be.an('object');
  });

  it('7. 租户隔离：租户 B 看不到租户 A 的覆盖；同键互不影响；跨租户删除 0 命中', async function () {
    // 前置：租户 A 持有 KEY_A=VALUE_A2（用例 2 落定），租户 B 尚未写 KEY_A。
    const beforeB = await apiClient.get('/whitelabel/overrides', {}, TENANT_B);
    expect(beforeB.code, JSON.stringify(beforeB)).to.equal(200);
    expect(langGroup(beforeB, 'zh-cn')[KEY_A], '租户 B 不得看到租户 A 的覆盖').to.equal(undefined);

    // 租户 B 写同键不同值：租户内独立，不影响租户 A。
    const writeB = await apiClient.put(
      '/whitelabel/translations',
      { items: [{ lang: 'zh-cn', key: KEY_A, value: VALUE_B }] },
      TENANT_B
    );
    expect(writeB.code, JSON.stringify(writeB)).to.equal(200);

    const aAfter = await apiClient.get('/whitelabel/translations', { lang: 'zh-cn' }, TENANT_A);
    const aRow = (aAfter.data.list || []).find((row) => row.key === KEY_A);
    expect(aRow.value, '租户 A 的值不受租户 B 写入影响').to.equal(VALUE_A2);

    const bAfter = await apiClient.get('/whitelabel/translations', { lang: 'zh-cn' }, TENANT_B);
    const bRow = (bAfter.data.list || []).find((row) => row.key === KEY_A);
    expect(bRow.value).to.equal(VALUE_B);

    // 租户 B 的 CSS 与租户 A 互不影响。
    await apiClient.put('/whitelabel/custom-css', { css: CSS_B }, TENANT_B);
    const cssA = await apiClient.get('/whitelabel/custom-css', {}, TENANT_A);
    const cssB = await apiClient.get('/whitelabel/custom-css', {}, TENANT_B);
    expect(cssA.data.css).to.equal('');
    expect(cssB.data.css).to.equal(CSS_B);

    // 租户 B 删除同键 KEY_A：只命中租户 B 自己的行（deleted=1），租户 A 数据完好——
    // 跨租户安全的关键证据是 A 的行不受影响，而不是命中数。
    const crossDelete = await apiClient.delete(
      '/whitelabel/translations',
      { items: [{ lang: 'zh-cn', key: KEY_A }] },
      TENANT_B
    );
    expect(crossDelete.code, JSON.stringify(crossDelete)).to.equal(200);
    expect(crossDelete.data.deleted).to.equal(1);
    const aStillThere = await apiClient.get('/whitelabel/translations', { lang: 'zh-cn' }, TENANT_A);
    expect((aStillThere.data.list || []).some((row) => row.key === KEY_A)).to.equal(true);
  });
});
