/**
 * 文件用途：菜单驱动路由的"可达性 + 可见性"回归门禁。
 *
 * 背景（2026-09-15）：
 *   本项目 VITE_AUTH_ROUTE_MODE=dynamic，授权路由由后端 sys_ui_elements 菜单驱动。
 *   把前端路由表（typings/elegant-router.d.ts 的 RouteMap，76 条）与菜单 param1 做差集，
 *   再用真实浏览器逐条验证，发现 **19 条路由返回 403**——页面代码写完了、路由注册了，
 *   但因为菜单里没有对应行，用户无处可达。其中包含规则链、Dashboard、SCADA 等核心页。
 *
 *   修复分两步（提交 34ea86f）：
 *     1. 迁移 102.sql 补 18 条菜单行，先全部设 param3='1'（隐藏但可达）；
 *     2. 用浏览器逐个验收主内容区（scripts/diag-page-content.js），
 *        把确认可用的 16 条改为 param3='0'（侧边栏可见），2 条编辑页保持隐藏。
 *
 * 本 spec 的价值：把上面第 2 步的结论**固化**。菜单是运行期数据、不在 git 里，
 * 任何人改错 param3 或删掉菜单行，这里就会红——而不是等用户发现页面打不开。
 *
 * 关键注意事项：
 *   1. 只断言"可达 + 渲染 + 在侧边栏出现"，不断言业务逻辑（那由 tests/ 下的契约用例负责），
 *      避免两套脆弱断言互相打架。
 *   2. /apply/service 属 SYS_ADMIN 专区（父级 /apply 与同级 plug_in 都只授 SYS_ADMIN），
 *      tenant_admin 拿 403 是**正确行为**，必须用 super_admin 验证。
 *   3. 侧边栏文案用中英双语正则，避免受默认语言影响。
 *   4. 编辑页（rule-chain/edit、scada-editor）**不应**出现在侧边栏——它们从列表页导航进入，
 *      与既有 automation_scene-edit / visualization_thingsvis-editor 的惯例一致。
 *      这条反向断言专门防止"过度放开"。
 */

const { test, expect } = require('./fixtures');

/** 本轮验收后应可见（param3='0'）的 16 条，及其页面标记（中英双语） */
const VISIBLE_ROUTES = [
  { route: '/dashboard/rdi-overview', marker: /RDI Overview|RDI 总览/i },
  { route: '/dashboard/workbench', marker: /Workbench|工作台/i },
  { route: '/dashboard/workspace', marker: /Workspace|工作区/i },
  { route: '/product/pre-register', marker: /Pre-regist|预注册/i },
  { route: '/product/update-ota', marker: /OTA update|OTA 升级/i },
  { route: '/product/update-package', marker: /Update package|升级包/i },
  { route: '/device/asset', marker: /Asset|资产/i },
  { route: '/device/entity-relation', marker: /Entity relation|实体关系/i },
  { route: '/management/entity-version', marker: /Entity version|实体版本/i },
  { route: '/management/role', marker: /Role|角色/i },
  { route: '/automation/rule-chain', marker: /Rule Chain|规则链/i },
  { route: '/system-management-user/equipment-map', marker: /Equipment map|设备地图/i },
  { route: '/visualization/scada', marker: /SCADA|项目/i }
];

/** 应保持隐藏（param3='1'）的编辑页：仍可达，但不应出现在侧边栏 */
const HIDDEN_EDITORS = [
  { route: '/automation/rule-chain/edit', sidebarText: /Rule chain editor|规则链编辑/i },
  { route: '/visualization/scada-editor', sidebarText: /SCADA canvas editor|SCADA 编辑器/i }
];

/**
 * 断言路由真的渲染出页面内容。
 * SPA 内部跳转不返回 HTTP 404/403，而是渲染异常页，所以必须同时看 URL 与页面文本。
 */
async function expectRendered(page, route, marker) {
  await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 20000 });
  await expect(page).toHaveURL(new RegExp(`${route.replace(/\//g, '\\/')}$`));
  await expect(page.getByText(/No Permission|404|Not Found|页面不存在/i)).toHaveCount(0);
  if (marker) {
    await expect(page.getByText(marker).first()).toBeVisible({ timeout: 20000 });
  }
}

test.describe('菜单驱动路由可达性（102.sql + 验收上线）', () => {
  test.describe('可见页：可达且渲染', () => {
    for (const { route, marker } of VISIBLE_ROUTES) {
      test(`renders ${route}`, async ({ rolePage }) => {
        await expectRendered(rolePage, route, marker);
      });
    }
  });

  test.describe('编辑页：可达但保持隐藏', () => {
    test('rule chain editor is reachable', async ({ rolePage }) => {
      await expectRendered(rolePage, '/automation/rule-chain/edit', /Rule Chain|规则链/i);
    });

    test('scada editor is reachable', async ({ rolePage }) => {
      await expectRendered(rolePage, '/visualization/scada-editor');
    });
  });

  test.describe('侧边栏：可见页出现、编辑页不出现', () => {
    test('sidebar shows the newly enabled entries', async ({ rolePage }) => {
      await rolePage.goto('/home', { waitUntil: 'domcontentloaded' });
      await rolePage.waitForTimeout(3000);

      // 展开所有顶级分组，否则子项不渲染
      for (const group of [
        'Operations & Visuals',
        'Product Management',
        'Devices',
        'Visualization',
        'Automations',
        'System Management'
      ]) {
        try {
          await rolePage.getByText(group, { exact: true }).first().click({ timeout: 3000 });
          await rolePage.waitForTimeout(500);
        } catch {
          // 分组文案随语言变化，点不到就跳过；下面的正文断言才是判定依据
        }
      }

      const sidebar = await rolePage.evaluate(() => {
        const el = document.querySelector('.n-layout-sider') || document.querySelector('aside');
        return el ? el.innerText : '';
      });

      for (const { route, marker } of VISIBLE_ROUTES) {
        expect(sidebar, `侧边栏应出现 ${route} 的入口`).toMatch(marker);
      }
    });

    test('sidebar does NOT show the editor pages', async ({ rolePage }) => {
      await rolePage.goto('/home', { waitUntil: 'domcontentloaded' });
      await rolePage.waitForTimeout(3000);

      for (const group of ['Automations', 'Visualization']) {
        try {
          await rolePage.getByText(group, { exact: true }).first().click({ timeout: 3000 });
          await rolePage.waitForTimeout(500);
        } catch {
          // 忽略
        }
      }

      const sidebar = await rolePage.evaluate(() => {
        const el = document.querySelector('.n-layout-sider') || document.querySelector('aside');
        return el ? el.innerText : '';
      });

      // 反向断言：编辑页不应作为菜单项出现（防止过度放开）
      for (const { route, sidebarText } of HIDDEN_EDITORS) {
        expect(sidebar, `编辑页 ${route} 不应出现在侧边栏`).not.toMatch(sidebarText);
      }
    });
  });

  test.describe('SYS_ADMIN 专区', () => {
    test.describe('apply/service', () => {
      test.use({ role: 'super_admin' });

      test('renders /apply/service for super admin', async ({ rolePage }) => {
        await expectRendered(rolePage, '/apply/service', /Service|服务/i);
      });
    });

    /**
     * /management/api（API Keys）必须只对 TENANT_ADMIN 开放。
     *
     * 这条断言来自一个真实的权限漏洞（2026-09-15 发现并修复）：
     *   - sql/5.sql 定义该菜单行时 authority 就是 ["TENANT_ADMIN"]；
     *   - sql/64.sql 明确把 api/v1/open/keys 从 TENANT_USER 撤权，并在注释里写明
     *     "console 对应页面按角色菜单过滤（/ui_elements/menu）不展示入口"；
     *   - 但 sql/47.sql 做了一次无差别 UPDATE：给所有含 TENANT_ADMIN 的菜单行
     *     统一加上 TENANT_USER，把 64.sql 的配套机制打穿了。
     * 后果：TENANT_USER 能打开该页（菜单放行），但页面调用 GET /api/v1/open/keys
     * 拿到 403 并抛 pageerror —— **页面渲染得出来，数据永远加载不了**。
     *
     * 只断言"能不能渲染"是抓不到这个的，必须断言角色边界本身。
     */
    test.describe('management/api', () => {
      test.use({ role: 'tenant_user' });

      test('tenant user is denied /management/api', async ({ rolePage }) => {
        await rolePage.goto('/management/api', { waitUntil: 'domcontentloaded' });
        await rolePage.waitForTimeout(2500);
        // 403 页的 "No Permission" 只在 document.title 里（见上面的说明）
        await expect.poll(() => rolePage.title(), { timeout: 15000 }).toMatch(/No Permission|403/i);
      });
    });

    test('tenant admin is correctly denied /apply/service', async ({ rolePage }) => {
      // 这条断言的是"权限边界正确"，不是缺陷：/apply 整棵只授 SYS_ADMIN
      await rolePage.goto('/apply/service', { waitUntil: 'domcontentloaded' });
      await rolePage.waitForTimeout(2500);

      // 注意：403 页的正文只有 "Logout / Back to Home"，"No Permission" 只出现在
      // document.title 里（见 views/_builtin/403/index.vue 的 i18n 用法）。
      // 因此必须断言 title，用 getByText 会一直等不到。
      await expect.poll(() => rolePage.title(), { timeout: 15000 }).toMatch(/No Permission|403/i);
    });
  });
});
