/**
 * 文件用途：ROADMAP P1.6「模板市场产品化」最后一项缺口的浏览器 E2E 取证。
 *
 * ## 为什么需要这份用例
 *
 * P1.6 的 API 面早已齐备（`45_template_upgrade_rollback.test.js` 15/15），
 * 前端接线也在（`views/device/template/components/template-upgrade-drawer.vue`
 * 由 `index.vue` 挂载），但**从未在真实浏览器里走过一遍**。
 * 缺这一层会漏掉整整一类缺陷：接口能返回、composable 也存了，
 * 但组件没挂载、按钮点不到、抽屉根本不出现——API 测试与单测都会绿。
 * P0.5 就是这么抓到三个阻断缺陷的（见 `28_p05_preregister_csv.spec.js` 的注释）。
 *
 * ## 覆盖 P1.6 升级/回滚门禁
 *   1. 列表视图里的「版本历史」入口能打开抽屉（抽屉真的挂载了，不是死代码）
 *   2. 空历史态可见，升级入口可见
 *   3. 真实浏览器选择 JSON 文件 → 升级成功 → 历史里出现回滚点（1.0.0 → 2.0.0）
 *   4. 「回滚到此点」走二次确认 → 回滚成功 → **历史行数不变**
 *      （回滚是幂等重放，不建行；这条锁死 45 组用例记录的实测语义）
 *
 * ## 关键注意事项
 *   - 默认视图是**卡片**视图，而「版本历史」按钮只在**列表**视图的表格列里渲染，
 *     所以必须先切视图，否则用例会以"找不到按钮"的形式失败，掩盖真实原因。
 *   - 列表分页只取前 10 条，必须用搜索把目标物模型筛出来，不能假设它出现在第一页。
 *   - 升级会在租户下**新建一行**（版本 2.0.0），所以回收要按**名称**列出后逐行删除，
 *     不能只删基线那一个 id。
 *   - 抽屉标题、按钮文案在组件里是硬编码中文（未走 i18n），
 *     因此断言用中文即可，不受浏览器语言影响；但搜索框 placeholder 走 i18n，需双语文案。
 */

const { test, expect } = require('./fixtures');
const apiClient = require('../lib/api_client');

const PAGE_PATH = '/device/thingsmodel';
const IMPORT_PATH = '/device/template/import';
const ACCOUNT = 'tenant_admin';

// 组件内硬编码文案（未 i18n）
const DRAWER_TITLE = /物模型版本管理与升级回滚/;
const UPGRADE_BUTTON = '上传新版本 JSON 升级';
const ROLLBACK_BUTTON = '回滚到此点';
const EMPTY_HISTORY = '暂无版本升级历史';
const UPGRADE_OK = /物模型升级成功/;
const ROLLBACK_OK = /物模型版本回滚成功/;

// 走 i18n 的文案，需覆盖中英双语
const LIST_VIEW_TOGGLE = 'button[title="List View"], button[title="列表视图"]';
const SEARCH_INPUT = /Enter Thing Model Name|请输入物模型名称/;
const SEARCH_BUTTON = /^(Search|搜索)$/;

let nameSeq = 0;

function makeTemplateName(tag) {
  nameSeq += 1;
  return `e2e_p16_${tag}_${Date.now().toString(36)}_${nameSeq}`;
}

function importPayload(name, version, description) {
  return {
    kind: 'aetherlink-device-template',
    name,
    version,
    type_key: 'automation',
    description
  };
}

/** 构造一次真实 file chooser 用的 JSON 文件（升级入口读的是文件内容，不是表单字段） */
function upgradeFile(name, version) {
  return {
    name: `${name}-${version}.json`,
    mimeType: 'application/json',
    buffer: Buffer.from(JSON.stringify(importPayload(name, version, `e2e upgrade to ${version}`)), 'utf8')
  };
}

async function seedBaseline(name, version = '1.0.0') {
  const resp = await apiClient.post(IMPORT_PATH, importPayload(name, version, 'e2e P1.6 baseline'), ACCOUNT);
  expect(resp.code, `导入基线物模型失败：${(resp && resp.message) || ''}`).toBe(200);
  const id = resp.data && resp.data.template && resp.data.template.id;
  expect(id, '基线物模型必须返回 id').toEqual(expect.any(String));
  return id;
}

/**
 * 按名称回收：升级会在租户下新建一行，只删基线 id 会留下孤儿行污染后续用例。
 * 失败只记录，不掩盖断言结论。
 */
async function cleanupByName(name) {
  if (!name) return;
  try {
    const resp = await apiClient.get('/device/template', { page: 1, page_size: 100, name }, ACCOUNT);
    const rows = (resp.data && (resp.data.list || resp.data)) || [];
    for (const row of rows) {
      if (!row || row.name !== name || !row.id) continue;
      try {
        await apiClient.delete(`${IMPORT_PATH}/${row.id}`, {}, ACCOUNT);
      } catch (e) {
        console.warn(`[cleanup] 删除物模型 ${row.id} 失败：${e && e.message}`);
      }
    }
  } catch (e) {
    console.warn(`[cleanup] 列出物模型 ${name} 失败：${e && e.message}`);
  }
}

/** 打开物模型页 → 切列表视图 → 搜索 → 点「版本历史」→ 返回抽屉 locator */
async function openUpgradeDrawer(page, name) {
  await page.goto(PAGE_PATH, { waitUntil: 'domcontentloaded' });
  await expect(page).toHaveURL(/\/device\/thingsmodel$/);

  await page.locator(LIST_VIEW_TOGGLE).first().click();

  await page.getByPlaceholder(SEARCH_INPUT).first().fill(name);
  await page.getByRole('button', { name: SEARCH_BUTTON }).first().click();

  const row = page.locator('tbody tr').filter({ hasText: name }).first();
  await expect(row, '新建的物模型必须出现在列表视图里').toBeVisible({ timeout: 20000 });

  await row.getByRole('button', { name: '版本历史' }).click();

  const drawer = page.locator('.n-drawer').first();
  await expect(drawer.getByText(DRAWER_TITLE).first(), '版本管理抽屉必须真的打开').toBeVisible({ timeout: 15000 });
  return drawer;
}

async function expectToast(page, matcher) {
  await expect(page.locator('.n-message').filter({ hasText: matcher }).first()).toBeVisible({ timeout: 30000 });
}

test.describe('P1.6 模板升级 / 回滚浏览器 E2E', () => {
  test.describe.configure({ timeout: 120000 });

  test.beforeAll(async () => {
    const healthy = await apiClient.healthCheck();
    test.skip(!healthy, '本地后端未运行，跳过 P1.6 升级/回滚 E2E');
    await apiClient.login(ACCOUNT);
  });

  test('列表视图的「版本历史」能打开抽屉，并展示空历史与升级入口', async ({ rolePage }) => {
    const name = makeTemplateName('open');
    try {
      await seedBaseline(name);
      const drawer = await openUpgradeDrawer(rolePage, name);

      await expect(drawer.getByText(/版本升级说明/)).toBeVisible();
      await expect(drawer.getByRole('button', { name: UPGRADE_BUTTON })).toBeVisible();
      await expect(drawer.getByText(EMPTY_HISTORY)).toBeVisible();
    } finally {
      await cleanupByName(name);
    }
  });

  test('真实浏览器选文件升级后，历史里出现回滚点', async ({ rolePage }) => {
    const name = makeTemplateName('upgrade');
    try {
      await seedBaseline(name, '1.0.0');
      const drawer = await openUpgradeDrawer(rolePage, name);
      await expect(drawer.getByText(EMPTY_HISTORY)).toBeVisible();

      // 真实 file chooser 路径：NUpload 内部有 input[type=file]
      await drawer.locator('input[type="file"]').first().setInputFiles(upgradeFile(name, '2.0.0'));

      await expectToast(rolePage, UPGRADE_OK);

      const historyRow = drawer.locator('tbody tr').filter({ hasText: '2.0.0' }).first();
      await expect(historyRow, '升级后历史里必须出现 2.0.0 的回滚点').toBeVisible({ timeout: 15000 });
      await expect(historyRow, '回滚点必须记录源版本 1.0.0').toContainText('1.0.0');
      await expect(drawer.getByText(EMPTY_HISTORY), '有历史后空态必须消失').toHaveCount(0);
    } finally {
      await cleanupByName(name);
    }
  });

  test('「回滚到此点」走二次确认，且历史行数不增（回滚是幂等重放）', async ({ rolePage }) => {
    const name = makeTemplateName('rollback');
    try {
      await seedBaseline(name, '1.0.0');
      const drawer = await openUpgradeDrawer(rolePage, name);

      await drawer.locator('input[type="file"]').first().setInputFiles(upgradeFile(name, '2.0.0'));
      await expectToast(rolePage, UPGRADE_OK);

      const rows = drawer.locator('tbody tr');
      await expect(rows).toHaveCount(1, { timeout: 15000 });

      await drawer.getByRole('button', { name: ROLLBACK_BUTTON }).first().click();

      // NPopconfirm 的确认按钮：naive-ui 的 positive 按钮带 primary type
      await rolePage.locator('.n-popconfirm button.n-button--primary-type').last().click();

      await expectToast(rolePage, ROLLBACK_OK);

      // 回滚是重放不是新建：历史行数必须保持不变（45 组用例锁死的实测语义）
      await expect(drawer.locator('tbody tr'), '回滚不得新增历史行').toHaveCount(1, { timeout: 15000 });
    } finally {
      await cleanupByName(name);
    }
  });
});
