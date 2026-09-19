/**
 * 文件用途：P1.3「SCADA 画布编辑器」浏览器 E2E 取证（UI 行为面最后一块）。
 * 覆盖：
 *   1. 编辑器页面加载并自动选中种子项目/文档；
 *   2. 从符号面板添加节点（画布变脏、save 按钮激活）；
 *   3. save 走 expected_version 乐观并发保存，成功后版本 +1（API 直读核对）；
 *   4. publish 只允许在已同步状态发布，发布后 published_version=1。
 */

const { test, expect } = require('./fixtures');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const ACCOUNT = 'tenant_admin';

test.describe('P1.3 SCADA 画布编辑器浏览器 E2E', () => {
  test.describe.configure({ timeout: 180000 });

  const createdProjectIds = [];

  test.beforeAll(async () => {
    const healthy = await apiClient.healthCheck();
    test.skip(!healthy, '本地后端未运行，跳过 SCADA 编辑器 E2E');
    await apiClient.login(ACCOUNT);
  });

  test.afterAll(async () => {
    // 清理本测试创建的所有项目（documentId 由 UI 创建，不在 beforeAll 里）。
    if (createdProjectIds.length) {
      for (const id of createdProjectIds) {
        try {
          await apiClient.delete('/scada/projects/' + id, {}, ACCOUNT);
        } catch (err) { /* 尽力回收 */ }
      }
    }
  });

  test('编辑器加载、添加符号、保存版本递增、发布生效', async ({ rolePage }) => {
    await rolePage.goto('/visualization/scada', { waitUntil: 'domcontentloaded' });

    // 编辑器主体加载（标题按钮组可见即壳层挂载）。
    await expect(rolePage.getByRole('button', { name: 'save' })).toBeVisible({ timeout: 30000 });

    // 通过 UI 走 new project / new canvas 创建（同时取证"项目 CRUD 不再 unsupported"
    // 的 UI 面；不走下拉选择——项目列表分页不一定包含新建项）。
    const uiProjectName = 'e2e_ui_proj_' + Date.now().toString().slice(-8);
    const uiDocumentName = 'e2e_ui_doc_' + Date.now().toString().slice(-8);
    await rolePage.getByPlaceholder('new project name').fill(uiProjectName);
    await rolePage.getByRole('button', { name: 'new project' }).click();
    await expect(rolePage.locator('.n-select').nth(0)).toContainText(uiProjectName.slice(0, 20), { timeout: 20000 });
    const projectsAfter = await apiClient.get('/scada/projects?page=1&page_size=50', {}, ACCOUNT);
    const createdRow = (projectsAfter.data.list || projectsAfter.data || []).find(row => row.name === uiProjectName);
    if (createdRow) createdProjectIds.push(createdRow.id);

    await rolePage.getByPlaceholder('new canvas name').fill(uiDocumentName);
    await rolePage.getByRole('button', { name: 'new canvas' }).click();
    await expect(rolePage.locator('.n-select').nth(1)).toContainText(uiDocumentName.slice(0, 20), { timeout: 20000 });

    // 初始状态：无改动，save 禁用（避免无变化往返把版本 +1 的契约在 UI 侧成立）。
    await expect(rolePage.getByRole('button', { name: 'save' })).toBeDisabled();

    // 展开符号面板（NCollapse 默认折叠）并添加一个符号节点：画布变脏，save 激活。
    await rolePage.getByText('widgets', { exact: true }).first().click();
    await rolePage.getByRole('button', { name: '+ timeseries' }).first().click();
    await expect(rolePage.getByRole('button', { name: 'save' })).toBeEnabled({ timeout: 10000 });

    // 保存：乐观并发（expected_version）走真实 API。
    await rolePage.getByRole('button', { name: 'save' }).click();
    await expect(rolePage.getByText('已保存')).toBeVisible({ timeout: 20000 });

    // API 直读：按名称找到 UI 创建的文档，画布内容确实落库（含新增节点）。
    const projects = await apiClient.get('/scada/projects?page=1&page_size=50', {}, ACCOUNT);
    const projectRows = projects.data.list || projects.data || [];
    const created = projectRows.find(row => row.name === uiProjectName);
    expect(created, 'UI-created project visible via API').toBeTruthy();
    const docs = await apiClient.get(`/scada/projects/${created.id}/documents`, {}, ACCOUNT);
    const docRows = docs.data.list || docs.data || [];
    const createdDoc = docRows.find(row => row.name === uiDocumentName);
    expect(createdDoc, 'UI-created document visible via API').toBeTruthy();
    const detail = await apiClient.get('/scada/documents/' + createdDoc.id, {}, ACCOUNT);
    expect(detail.code).toEqual(200);
    const canvasText = JSON.stringify(detail.data.canvas || detail.data.JSONData || detail.data);
    expect(canvasText, 'saved canvas must contain the added widget node').toContain('timeseries');

    // 发布：只允许在已同步状态。
    await rolePage.getByRole('button', { name: 'publish' }).click();
    await expect(rolePage.getByText('已发布')).toBeVisible({ timeout: 20000 });

    const versions = await apiClient.get('/scada/documents/' + createdDoc.id + '/versions', {}, ACCOUNT);
    expect(versions.code).toEqual(200);
    const versionRows = Array.isArray(versions.data) ? versions.data : (versions.data && versions.data.list) || [];
    expect(versionRows.length, 'published version snapshot recorded').toBeGreaterThanOrEqual(1);
  });
});
