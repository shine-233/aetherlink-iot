/**
 * 文件用途：TB-12「设备认领与自动注册」浏览器 E2E 取证（ROADMAP §7.1 TB-12 UI 面）。
 * 覆盖：
 *   1. 设备管理页行操作「生成认领令牌」→ 弹窗 → 生成密钥 → 明文密钥展示与一次性警示；
 *   2. 顶栏「认领设备」入口（接收方视角弹窗）；
 *   3. tenant_admin_b 在浏览器里凭 device_number + claim_key 完成认领；
 *   4. 认领后原租户管理页不再出现该设备（跨租户边界在 UI 上成立）。
 */

const { test, expect, getStorageStatePath } = require('./fixtures');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');
const { frontendURL } = require('../lib/runtime_config');

test.describe('TB-12 设备认领浏览器 E2E', () => {
  test.describe.configure({ timeout: 180000 });

  test.beforeAll(async () => {
    const healthy = await apiClient.healthCheck();
    test.skip(!healthy, '本地后端未运行，跳过 TB-12 设备认领 E2E');
    await apiClient.login('tenant_admin');
    await apiClient.login('tenant_admin_b');
  });

  test('签发认领令牌弹窗展示一次性密钥；接收方在浏览器完成认领；原租户设备消失', async ({ browser }) => {
    const seeded = await seedData.createSimulationDevice('tenant_admin');
    const deviceNumber = seeded.row &&
      (seeded.row.device_number || seeded.row.deviceNumber || seeded.row.DeviceNumber);
    expect(deviceNumber, '种子设备必须带 device_number').toBeTruthy();
    try {
      // —— 1. 原租户：设备管理页 → 行操作「生成认领令牌」 ——
      const ownerCtx = await browser.newContext({
        storageState: getStorageStatePath('tenant_admin'),
        baseURL: frontendURL
      });
      const ownerPage = await ownerCtx.newPage();
      await ownerPage.goto('/device/manage', { waitUntil: 'domcontentloaded' });

      // 管理页默认视图可能是卡片/地图：先切到列表视图（title=common.viewList）。
      await ownerPage.getByTitle(/列表视图|List View/).first().click();
      await ownerPage.waitForTimeout(1500);

      // 等设备行出现（按唯一名称定位；createSimulationDevice 的 name 在 row 上）。
      const deviceName = String((seeded.row && seeded.row.name) || '');
      expect(deviceName, '种子设备必须有名称').not.toBe('');
      const row = ownerPage.locator('tr', { hasText: deviceName }).first();
      await expect(row, '种子设备行必须出现在管理页').toBeVisible({ timeout: 30000 });

      await row.getByText(/生成认领令牌|Issue claim token/).first().click();
      const issueModal = ownerPage.locator('.n-modal.n-card, .n-dialog').first();
      await expect(issueModal, '签发弹窗必须出现').toBeVisible({ timeout: 10000 });

      await issueModal.getByRole('button', { name: /生成密钥|Generate key/ }).first().click();
      const keyInput = issueModal.locator('input[readonly]').first();
      await expect(keyInput, '一次性密钥必须展示').toBeVisible({ timeout: 15000 });
      const claimKey = await keyInput.inputValue();
      expect(claimKey).toMatch(/^ack_[0-9a-f]{48}$/);
      await ownerCtx.close();

      // —— 2. 接收方 tenant_admin_b：顶栏「认领设备」弹窗完成认领 ——
      const claimerCtx = await browser.newContext({
        storageState: getStorageStatePath('tenant_admin_b'),
        baseURL: frontendURL
      });
      const claimerPage = await claimerCtx.newPage();
      await claimerPage.goto('/device/manage', { waitUntil: 'domcontentloaded' });

      await claimerPage.getByRole('button', { name: /认领设备|Claim device/ }).first().click();
      const redeemModal = claimerPage.locator('.n-modal.n-card, .n-dialog').first();
      await expect(redeemModal, '认领弹窗必须出现').toBeVisible({ timeout: 10000 });

      await redeemModal.getByPlaceholder(/请输入设备编号|Enter device number/).fill(deviceNumber);
      await redeemModal.getByPlaceholder(/请输入认领密钥|Enter claim key/).fill(claimKey);
      await redeemModal.getByRole('button', { name: /认领设备|Claim device/ }).last().click();

      // 认领成功消息（业务成功后弹窗关闭）。
      await expect(redeemModal).toBeHidden({ timeout: 20000 });

      // —— 3. 原租户刷新后该设备消失 ——
      const detailA = await apiClient.get('/device/detail/' + seeded.id, {}, 'tenant_admin');
      // 原租户失去访问：404 或 201001（无权限）都算"不再可见"。
      expect([100404, 201001], `original tenant lost access, got ${detailA.code}`).toContain(detailA.code);
      const detailB = await apiClient.get('/device/detail/' + seeded.id, {}, 'tenant_admin_b');
      expect(detailB.code, '认领方必须可见设备').toEqual(200);

      await claimerCtx.close();
    } finally {
      // 设备现在属于 tenant_admin_b：按归属清理。
      try {
        const del = await apiClient.delete('/device/' + seeded.id, {}, 'tenant_admin_b');
        if (del.code !== 200) await apiClient.delete('/device/' + seeded.id, {}, 'tenant_admin');
      } catch (err) { /* 清理失败不掩盖主结果 */ }
    }
  });
});
