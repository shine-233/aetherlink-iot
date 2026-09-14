/**
 * 文件用途：P1.5 / P1.6 / P2.2 / P3 新增页面的浏览器可达性门禁。
 *
 * 核心逻辑：这四个页面此前在 20 个 Playwright spec 里 **0 覆盖**——
 *   - anomaly / report：页面已存在，但从未有浏览器用例证明能打开；
 *   - market/browse：打包导入 UI 已实现（预览 / 验签 / 覆盖闸门），
 *     但**路由从未注册**，用户无处可达，后端整条闸门是死门（本次补注册后才有意义）；
 *   - edge-nodes / license：页面已存在，同样无 UI 用例。
 *
 * 关键注意事项：
 *   1. 这里只证明"能打开、能渲染、不是 404/403"。业务正确性由 tests/38–42 的
 *      API 契约用例覆盖，不在浏览器层重复断言，避免两套脆弱断言互相打架。
 *   2. license 是平台管理员视图，tenant_admin 会落到 403，必须用 super_admin。
 *   3. 文案断言统一用中英双语正则，避免受默认语言切换影响。
 *   4. 若前端 preview 未启动，这些用例会整体失败——这是预期的：
 *      没有运行期证据就不算闭环，不要改成跳过。
 */

const { test, expect } = require('./fixtures');

const ROUTES = {
  anomaly: '/visualization/anomaly',
  report: '/visualization/report',
  marketBrowse: '/market/browse',
  edgeNodes: '/management/edge-nodes',
  license: '/management/license',
  deviceGrouping: '/device/grouping'
};

/**
 * 打开路由并断言真的渲染出页面内容。
 * SPA 内部跳转不会返回 HTTP 404，而是渲染异常页，所以必须同时看 URL 与页面文本，
 * 否则"路由存在但页面报错"会被漏过。
 */
async function expectRendered(page, route, marker) {
  await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 20000 });
  await expect(page).toHaveURL(new RegExp(`${route.replace(/\//g, '\\/')}$`));
  await expect(page.getByText(/404|Not Found|页面不存在/i)).toHaveCount(0);
  if (marker) {
    await expect(page.getByText(marker).first()).toBeVisible({ timeout: 20000 });
  }
}

test.describe('P1 console surfaces [24_p1_console_surfaces]', () => {
  test.describe('tenant admin surfaces', () => {
    test.use({ role: 'tenant_admin' });

    test('anomaly workbench renders the metric-key control', async ({ rolePage }) => {
      await expectRendered(rolePage, ROUTES.anomaly, /指标键|Metric key/i);
    });

    test('report workbench route renders instead of falling through to the exception page', async ({ rolePage }) => {
      await expectRendered(rolePage, ROUTES.report, /报表|Report/i);
    });

    test('market browse exposes the bundle import entry', async ({ rolePage }) => {
      // 这条是 P1.6 的死门验收：导入入口若不在页面上，
      // 后端"必须显式 confirm_overwrite"的闸门对用户界面上根本不存在。
      await expectRendered(rolePage, ROUTES.marketBrowse, /导入模板|Import Template/i);
    });

    test('edge node console renders the health column', async ({ rolePage }) => {
      await expectRendered(rolePage, ROUTES.edgeNodes, /健康|Health/i);
    });

    test('device grouping renders the statistics columns', async ({ rolePage }) => {
      // ROADMAP TP-8②：后端把统计挂进分组列表项的 statistics 字段，
      // 但"后端有字段"不等于"用户看得见"——必须在表头真的渲染出统计列。
      // 断言用离线列表头（导航菜单里不会出现 Offline / 离线），避免与菜单项撞词。
      await expectRendered(rolePage, ROUTES.deviceGrouping, /离线|Offline/i);
    });
  });

  test.describe('platform admin surfaces', () => {
    test.use({ role: 'super_admin' });

    test('license console renders the edition field', async ({ rolePage }) => {
      await expectRendered(rolePage, ROUTES.license, /版本|Edition/i);
    });

    test('operation log export endpoint answers the console session with a structured envelope', async ({ api }) => {
      await api.login('super_admin');
      const end = Date.now();
      const start = end - 60 * 60 * 1000;
      const resp = await api.post(
        '/operation_logs/export',
        { start_time: new Date(start).toISOString(), end_time: new Date(end).toISOString() },
        'super_admin'
      );
      // 空窗口会被显式拒绝（契约见 tests/42），这里只要求端点可达并返回结构化信封，
      // 不复制 42 号用例的逐条拒绝语义。
      expect(resp).toBeTruthy();
      expect(typeof resp.code).toBe('number');
    });
  });
});
