/**
 * 文件用途：P1.4 移动端门禁——H5 一条完整业务 E2E（真实 uni-app H5 构建产物）。
 *
 * 覆盖（P1.4 门禁「至少 Android/H5 一条完整业务 E2E」的 H5 侧）：
 *   1. 登录——错误密码被拒且展示错误文案；正确凭证进入设备列表页；
 *   2. 设备列表——种子设备按名称可见、在线徽章渲染；
 *   3. 业务闭环——点击设备展开「最新遥测」面板，断言真实遥测键值
 *      （temperature_1 = 25.5 来自模拟遥测发布，非 UI 桩）。
 *
 * 运行前提：
 *   - 后端活栈（9999）与移动端 H5 构建产物（active/mobile-app-uni/dist/build/h5）；
 *   - 以 PLAYWRIGHT_USE_PREVIEW_PROXY=1 PREVIEW_DIST_DIR=<h5 dist> 运行，
 *     webServer 即为 serve_preview_with_api_proxy.js（监听 9725，CORS 白名单内 origin）；
 *     移动端应用直连 http://127.0.0.1:9999/api/v1（BASE_URL 内建），9725 已在后端
 *     cors.allowed_origins 白名单中，x-token 头在 cors.allowed_headers 中。
 *
 * 判定：全部通过即 P1.4 的「H5 完整业务 E2E」门禁满足（原生商店上架仍为环境阻塞）。
 */

const { test, expect } = require('@playwright/test');
const apiClient = require('../lib/api_client');
const seedData = require('../lib/seed_data');

const H5_ORIGIN = process.env.MOBILE_H5_ORIGIN || 'http://127.0.0.1:9725';

let seeded;
let adminEmail;
let adminPassword;

test.beforeAll(async () => {
  const healthy = await apiClient.healthCheck();
  if (!healthy) {
    throw new Error('backend unhealthy; H5 business E2E must run against the live stack');
  }
  await apiClient.login('tenant_admin');
  adminEmail = process.env.TENANT_ADMIN_EMAIL;
  adminPassword = process.env.TENANT_ADMIN_PASSWORD;
  if (!adminEmail || !adminPassword) {
    throw new Error('TENANT_ADMIN credentials missing from env');
  }
  // 种子设备带真实模拟遥测（temperature_1=25.5 等），最新优先排序会落在列表第一页。
  seeded = await seedData.ensureDeviceWithTelemetry('tenant_admin');
  if (!seeded.row || !seeded.row.name) {
    throw new Error('seeded device name missing; list assertion would be ambiguous');
  }
});

test.afterAll(async () => {
  if (seeded && seeded.cleanup) {
    try {
      await seeded.cleanup();
    } catch (err) {
      /* 清理失败不掩盖主结果 */
    }
  }
  apiClient.clearAllTokens();
});

async function loginViaUI(page, email, password) {
  await page.goto(H5_ORIGIN + '/', { waitUntil: 'domcontentloaded' });
  await expect(page.locator('.login .brand')).toBeVisible();
  // uni-app H5 把 view/input 编译为 uni-view/uni-input 自定义元素：.field 落在
  // uni-input 上，可填充的原生 input 在其内部，必须下钻一层再 fill。
  const emailInput = page.locator('.login uni-input.field').first().locator('input');
  const passwordInput = page.locator('.login uni-input.field').nth(1).locator('input');
  await emailInput.fill(email);
  await passwordInput.fill(password);
  await page.locator('.login .btn').click();
}

test('H5 登录：错误密码被拒展示错误，正确凭证进入设备列表', async ({ page }) => {
  await loginViaUI(page, adminEmail, 'WrongPassword#9');
  await expect(page.locator('.login .err')).toBeVisible();

  const passwordInput = page.locator('.login uni-input.field').nth(1).locator('input');
  await passwordInput.fill(adminPassword);
  await page.locator('.login .btn').click();
  await expect(page).toHaveURL(/pages\/device\/list/);
  await expect(page.locator('.list-page .row').first()).toBeVisible();
});

test('设备列表展示种子设备与在线徽章', async ({ page }) => {
  await loginViaUI(page, adminEmail, adminPassword);
  await expect(page).toHaveURL(/pages\/device\/list/);

  const targetRow = page.locator('.list-page .row', { hasText: seeded.row.name });
  await expect(targetRow).toBeVisible();
  await expect(targetRow.locator('.badge')).toBeVisible();
});

test('点击设备展开最新遥测面板并渲染真实键值', async ({ page }) => {
  await loginViaUI(page, adminEmail, adminPassword);
  await expect(page).toHaveURL(/pages\/device\/list/);

  const targetRow = page.locator('.list-page .row', { hasText: seeded.row.name });
  await targetRow.click();

  const panel = page.locator('.list-page .tele', { has: page.locator('.tele-title') });
  await expect(panel.locator('.tele-title')).toHaveText('最新遥测');
  const tempRow = panel.locator('.tele-row', { hasText: 'temperature_1' });
  await expect(tempRow).toBeVisible();
  await expect(tempRow.locator('.tele-val')).toHaveText(/25\.5/);
});
