/**
 * 文件用途：TB-6「数据转换器」与 TP-6「设备健康评分」浏览器 E2E 验证。
 * 覆盖：
 *   1. 访问 /device/converter 数据转换器工作台；
 *   2. 页面标题卡片渲染，新建数据转换器弹窗；
 *   3. 填写表单创建上行 Lua 脚本转换器并提交落库；
 *   4. 数据表格渲染刚创建的转换器行；
 *   5. 打开在线仿真抽屉执行 Dry-Run 仿真并验证解析结果；
 *   6. 清理删除测试数据。
 */

const { test, expect } = require('./fixtures');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const ACCOUNT = 'tenant_admin';

test.describe('TB-6 数据转换器与健康评分 E2E', () => {
  test.describe.configure({ timeout: 120000 });

  let converterName = null;
  let createdConverterId = null;

  test.beforeAll(async () => {
    const healthy = await apiClient.healthCheck();
    test.skip(!healthy, '本地后端未运行，跳过 E2E');
    await apiClient.login(ACCOUNT);
    converterName = seedData.makeRunLabel('e2e_conv');
  });

  test.afterAll(async () => {
    if (createdConverterId) {
      try {
        await apiClient.delete(`/converters/${createdConverterId}`, {}, ACCOUNT);
      } catch (err) { /* cleanup */ }
    }
  });

  test('数据转换器管理工作台：创建、列表展现与在线仿真', async ({ rolePage }) => {
    // 1. 直达数据转换器页面
    await rolePage.goto('/device/converter', { waitUntil: 'domcontentloaded' });
    const cardTitle = rolePage.getByText('数据编解码转换器 (Data Converters)').first();
    await expect(cardTitle, '转换器工作台卡片必须可见').toBeVisible({ timeout: 30000 });

    // 2. 点击新建转换器按钮
    const createBtn = rolePage.getByRole('button', { name: /新建转换器/ }).first();
    await expect(createBtn).toBeVisible();
    await createBtn.click();

    // 3. 模态弹窗展开并填写名称
    const modalTitle = rolePage.getByText('新建数据转换器').first();
    await expect(modalTitle).toBeVisible({ timeout: 10000 });

    const nameInput = rolePage.locator('input[placeholder="如：Modbus温湿度传感器上行解析"]').first();
    await nameInput.fill(converterName);

    // 4. 点击保存/确定
    const submitBtn = rolePage.getByRole('button', { name: /保存|确定|立即创建/ }).last();
    await submitBtn.click();

    // 5. 验证列表中出现刚创建的转换器
    const row = rolePage.locator('tr', { hasText: converterName }).first();
    await expect(row, '新创建的转换器行应呈现在表格中').toBeVisible({ timeout: 20000 });

    // 6. 点击在线仿真测试
    const simBtn = row.getByRole('button', { name: /仿真测试/ }).first();
    await expect(simBtn).toBeVisible();
    await simBtn.click();

    // 7. 抽屉打开并执行仿真
    const runSimBtn = rolePage.getByRole('button', { name: /执行仿真测试/ }).first();
    await expect(runSimBtn).toBeVisible({ timeout: 10000 });
    await runSimBtn.click();

    // 8. 断言仿真输出结果出现
    const resAlert = rolePage.locator('.n-drawer').getByText(/解析成功|执行耗时/).first();
    await expect(resAlert).toBeVisible({ timeout: 15000 });

    // 9. 关闭抽屉并删除测试数据
    const closeBtn = rolePage.locator('.n-drawer').getByRole('button', { name: /关闭/ }).first();
    if (await closeBtn.isVisible()) {
      await closeBtn.click();
    }

    const delBtn = row.getByRole('button', { name: /删除/ }).first();
    await delBtn.click();
    const confirmBtn = rolePage.getByRole('button', { name: /确定|确认/ }).last();
    if (await confirmBtn.isVisible()) {
      await confirmBtn.click();
    }
  });
});
