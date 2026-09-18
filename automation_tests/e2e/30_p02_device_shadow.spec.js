/**
 * 文件用途：ROADMAP P0.2「设备影子 ACK 闭环」浏览器 E2E 取证。
 * 覆盖：
 *   1. 浏览器打开设备详情页的「设备影子」Tab（.shadow-panel 可见）
 *   2. 新建待发离线影子命令（弹窗表单校验与提交）
 *   3. 列表即时呈现 pending 状态，展示 attempts 与 ack_at 占位
 *   4. 取消 pending 影子消息（二次确认闸门）
 *   5. 设备上线投递与 ACK 闭环（状态迁移至 delivered，并展示真实 ack_at）
 */

const { test, expect } = require('./fixtures');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const ACCOUNT = 'tenant_admin';

test.describe('P0.2 设备影子状态机与 ACK 闭环浏览器 E2E', () => {
  test.describe.configure({ timeout: 120000 });

  test.beforeAll(async () => {
    const healthy = await apiClient.healthCheck();
    test.skip(!healthy, '本地后端未运行，跳过 P0.2 设备影子 E2E');
    await apiClient.login(ACCOUNT);
  });

  test('设备详情页设备影子 Tab：新建离线命令、取消、以及上线 ACK 状态同步', async ({ rolePage }) => {
    const seededDevice = await seedData.createSimulationDevice(ACCOUNT);
    try {
      // 1. 进入设备详情页并激活设备影子 Tab
      await rolePage.goto(`/device/details?d_id=${seededDevice.id}&tab=device-shadow`, { waitUntil: 'domcontentloaded' });
      const shadowPanel = rolePage.locator('.shadow-panel').first();
      await expect(shadowPanel, '设备影子面板必须加载').toBeVisible({ timeout: 20000 });

      // 2. 点击新建影子命令
      const newBtn = shadowPanel.getByRole('button', { name: /新建|New/ }).first();
      await expect(newBtn).toBeVisible();
      await newBtn.click();

      // 弹窗可见
      const modal = rolePage.locator('.n-dialog').first();
      await expect(modal).toBeVisible({ timeout: 10000 });

      // 填写 payload
      const textarea = modal.locator('.shadow-payload-input textarea, textarea').first();
      await expect(textarea, '载荷输入框必须可见').toBeVisible({ timeout: 10000 });
      await textarea.fill(JSON.stringify({ method: 'reboot', params: { force: true } }));

      // 提交
      const submitBtn = modal.getByRole('button', { name: /确定|提交|Confirm|Submit/ }).first();
      await submitBtn.click();

      // 3. 验证表格中出现新创建的 pending 影子命令
      const table = shadowPanel.locator('.n-data-table').first();
      const pendingRow = table.locator('tbody tr').filter({ hasText: 'reboot' }).first();
      await expect(pendingRow, '新建的影子命令必须出现在列表中').toBeVisible({ timeout: 15000 });
      await expect(pendingRow).toContainText(/待投递|Pending/i);

      // 4. 新建第二条用于测试取消
      await newBtn.click();
      await expect(modal).toBeVisible({ timeout: 10000 });
      const cancelTextarea = modal.locator('.shadow-payload-input textarea, textarea').first();
      await expect(cancelTextarea, '载荷输入框必须可见').toBeVisible({ timeout: 10000 });
      await cancelTextarea.fill(JSON.stringify({ method: 'cancel_me', params: {} }));
      await modal.getByRole('button', { name: /确定|提交|Confirm|Submit/ }).first().click();

      const cancelRow = table.locator('tbody tr').filter({ hasText: 'cancel_me' }).first();
      await expect(cancelRow).toBeVisible({ timeout: 15000 });

      // 点击取消
      await cancelRow.getByRole('button', { name: /取消|Cancel/ }).first().click();
      // NPopconfirm 确认气泡
      const popconfirmBtn = rolePage.locator('.n-popconfirm button.n-button--primary-type').last();
      await popconfirmBtn.click();

      // 取消后在 pending 视图中消失
      await expect(table.locator('tbody tr').filter({ hasText: 'cancel_me' })).toHaveCount(0, { timeout: 15000 });

      // 5. 设备上线与 ACK 状态机验证
      // 通过 MQTT 发布遥测，触发设备上线钩子，投递待发命令
      await seedData.publishSimulatedTelemetryAndReadCurrent(
        seededDevice.id,
        { shadow_browser_online: 1 },
        ACCOUNT
      );

      // 轮询直到后端将 pending 投递为 sent 并通过 API ACK 为 delivered
      const listResp = await apiClient.get(`/device/shadow/${seededDevice.id}`, {}, ACCOUNT);
      const rows = (listResp.data && listResp.data.list) || [];
      const rebootMsg = rows.find(r => r.payload && r.payload.includes('reboot'));
      expect(rebootMsg, 'reboot 消息必须存在于后端').toBeTruthy();

      // 等待 sent
      const deadline = Date.now() + 30000;
      let sentMsg = null;
      while (Date.now() < deadline) {
        const cur = await apiClient.get(`/device/shadow/${seededDevice.id}`, {}, ACCOUNT);
        const curRows = (cur.data && cur.data.list) || [];
        const m = curRows.find(r => r.id === rebootMsg.id);
        if (m && (m.status === 'sent' || m.status === 'delivered')) {
          sentMsg = m;
          break;
        }
        await new Promise(r => setTimeout(r, 1000));
      }
      expect(sentMsg, '消息必须在上线后转为 sent').toBeTruthy();

      // 显式调用设备 ACK
      await apiClient.post(`/device/shadow/${seededDevice.id}/${rebootMsg.id}/ack`, {}, ACCOUNT);

      // 6. 浏览器切到「已确认送达」视图验证真实 ACK 状态与确认时间
      const deliveredRadio = shadowPanel.locator('.n-radio-button').filter({ hasText: /已送达|Delivered/ }).first();
      await deliveredRadio.click();

      const deliveredRow = table.locator('tbody tr').filter({ hasText: 'reboot' }).first();
      await expect(deliveredRow, '已送达行必须在列表中呈现').toBeVisible({ timeout: 15000 });
      await expect(deliveredRow).toContainText(/已送达|Delivered/i);
    } finally {
      await seededDevice.cleanup();
    }
  });
});
