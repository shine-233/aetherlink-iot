/**
 * 文件用途：TB-19「解决方案模板引擎」浏览器 E2E 取证（ROADMAP §7.1 TB-19 UI 面）。
 * 覆盖：
 *   1. 系统管理 → 解决方案模板 菜单可达（路由四件套 + 114.sql 菜单行）；
 *   2. 方案列表渲染（API 预置的种子方案出现）；
 *   3. 一键安装确认 → 逐项安装结果（applied/failed 计数）展示；
 *   4. route-adapter 不再出现 management_secrets/management_solutions 的
 *      「skip invalid menu route」警告（TB-18 死菜单同批修复的证据）。
 */

const { test, expect } = require('./fixtures');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const ACCOUNT = 'tenant_admin';

test.describe('TB-19 解决方案模板浏览器 E2E', () => {
  test.describe.configure({ timeout: 180000 });

  let boardId = null;
  let templateId = null;
  let solutionName = null;
  let solutionId = null;
  const consoleWarnings = [];

  test.beforeAll(async () => {
    const healthy = await apiClient.healthCheck();
    test.skip(!healthy, '本地后端未运行，跳过 TB-19 解决方案模板 E2E');
    await apiClient.login(ACCOUNT);

    // 种子：一个看板 + 一个物模型模板 → 组装成一个方案。
    const boardResp = await apiClient.post('/board', {
      name: seedData.makeRunLabel('tb19e2e_board'),
      config: JSON.stringify({ widgets: [{ id: 'w1', type: 'chart' }] }),
      home_flag: 'N',
      menu_flag: 'N',
      description: 'board for TB-19 e2e',
      vis_type: 'native',
      type_key: 'automation',
      author: 'TestAuthor',
      version: '1.0.0'
    }, ACCOUNT);
    if (boardResp.code === 200) boardId = boardResp.data.id;

    const tplResp = await apiClient.post('/device/template/import', {
      kind: 'aetherlink-device-template',
      name: seedData.makeRunLabel('tb19e2e_tpl'),
      version: '1.0.0',
      type_key: 'automation',
      author: 'TestAuthor',
      description: 'template for TB-19 e2e'
    }, ACCOUNT);
    if (tplResp.code === 200) {
      templateId = tplResp.data.template ? tplResp.data.template.id : tplResp.data.id;
    }

    if (boardId && templateId) {
      solutionName = seedData.makeRunLabel('tb19e2e_solution');
      const createResp = await apiClient.post('/solutions', {
        name: solutionName,
        description: 'created by TB-19 browser e2e',
        resources: [
          { resource_type: 'device_template', resource_id: templateId },
          { resource_type: 'board_template', resource_id: boardId }
        ]
      }, ACCOUNT);
      if (createResp.code === 200) solutionId = createResp.data.id;
    }
  });

  test.afterAll(async () => {
    for (const [path, id] of [
      ['/solutions/', solutionId],
      ['/board/', boardId],
      ['/device/template/', templateId]
    ]) {
      if (id) {
        try {
          await apiClient.delete(path + id, {}, ACCOUNT);
        } catch (err) { /* 尽力回收 */ }
      }
    }
  });

  test('菜单可达、方案列表渲染、一键安装出逐项结果', async ({ rolePage }) => {
    test.skip(!solutionId, '种子组装失败（看板/模板/方案创建被拒），跳过 UI 用例');

    rolePage.on('console', message => {
      if (message.type() === 'warning') consoleWarnings.push(message.text());
    });

    // 1. 菜单直达方案页
    await rolePage.goto('/management/solutions', { waitUntil: 'domcontentloaded' });
    const pageCard = rolePage.locator('.n-card').first();
    await expect(pageCard, '方案页必须加载').toBeVisible({ timeout: 30000 });
    await expect(pageCard.getByText('解决方案模板', { exact: true }).first(), '页面标题').toBeVisible();

    // 2. 种子方案出现在列表里
    const row = rolePage.locator('tr', { hasText: solutionName }).first();
    await expect(row, '种子方案行必须渲染').toBeVisible({ timeout: 20000 });

    // 3. 一键安装（确认框）→ 安装结果面板
    await row.getByRole('button', { name: /一键安装/ }).first().click();
    await rolePage.getByText('开始安装').first().click();
    await expect(rolePage.getByText(/安装完成：共 2 项，成功 2 项，失败 0 项/)).toBeVisible({ timeout: 30000 });
    await expect(rolePage.getByText(/已应用/).first()).toBeVisible();

    // 4. 路由适配器不再跳过 secrets/solutions 菜单
    const skipped = consoleWarnings.filter(text => text.includes('skip invalid menu route'));
    expect(skipped, `不应再有死菜单: ${skipped.join(' | ')}`).toHaveLength(0);
  });
});
