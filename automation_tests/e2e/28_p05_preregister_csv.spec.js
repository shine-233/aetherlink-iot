/**
 * 文件用途：ROADMAP P0.5「CSV 浏览器 E2E」门禁取证。
 *
 * ## 为什么需要这份用例
 *
 * P0.5 此前唯一的未闭环项就是"真实浏览器 file chooser E2E（需活栈）"。
 * 其余部分（导入链路、一次性凭证、脱敏导出、清理）都已有 API 级与单测级证据，
 * 但**从未在真实浏览器里走过一遍**。缺这一层会漏掉整整一类缺陷：
 * API 响应里有数据、composable 也存了，但**模板根本没渲染** ——
 * 接口测试与单测都会绿，只有用户可见的流程是断的。
 *
 * 本用例写完即抓到一例：`views/product/pre-register/index.vue` 的结果面板
 * 只显示"凭证仅展示一次"的提示，却从不渲染 `importResult.devices[].voucher`，
 * 导致批量建档后**一台设备的凭证都拿不到**，这批设备实际无法接入。
 *
 * ## 为什么这里要用 psql 种前置数据（这不是偷懒）
 *
 * 预注册必须选一个产品，但**`products` 表在全系统没有任何写入路径**：
 * 后端只注册了 `GET /product`（下拉数据源），没有 POST/PUT/DELETE；
 * 没有 DAL 写入、没有迁移种子、没有前端产品管理页；`CreateProductReq`
 * 这个类型定义了却没有任何 handler 使用（死类型）。
 *
 * 项目自己的 Go 测试（`device_preregister_cleanup_postgres_test.go:55`）
 * 就是这么做的——裸 SQL 插一行产品。本文件沿用同一做法，并把
 * **"产品没有创建路径"作为独立缺陷上报**，不在这里顺手发明一个产品 CRUD。
 *
 * ## 覆盖 P0.5 门禁五条
 *   1. 真实浏览器选择文件（setInputFiles 走真实 file chooser 路径）
 *   2. 坏行逐行反馈（后端返回 csv_row，页面必须显示出行号）
 *   3. 批量建档 + 凭证真的渲染出来
 *   4. 凭证只出现一次（关闭弹窗重开后结果面板即消失，无任何持久化）
 *   5. 跨租户产品不可选
 *
 * ## 关键注意事项
 *   - 用例会真的建档，因此**必须**在 finally 里调 `/device/preRegister/cleanup` 回收，
 *     否则会污染设备池（本仓库有过"设备池被污染导致其他用例时绿时红"的先例）。
 *   - 设备编号刻意带连字符、避开 12 位纯字母数字，防止命中 device-tab-plan 的 RDI 判定。
 *   - psql 前置数据 fail-closed：只允许 127.0.0.1，且必须显式给
 *     `AETHERLINK_DB_PASSWORD`；连不上就整体 skip 并说明原因，不做假绿。
 */

const { spawn } = require('child_process');
const crypto = require('crypto');
const fs = require('fs');

const { test, expect } = require('./fixtures');
const seedData = require('../lib/seed_data');
const apiClient = require('../lib/api_client');

const CSV_HEADER = 'device_number,name';
const BATCH_PREFIX = 'p05csv';
const LOCAL_HOST = '127.0.0.1';

// ---------------------------------------------------------------------------
// psql 前置数据辅助（fail-closed）
// ---------------------------------------------------------------------------

function envv(name, fallback = '') {
  return String(process.env[name] || fallback).trim();
}

function dbConfig() {
  const host = envv('AETHERLINK_DB_HOST', LOCAL_HOST);
  if (host !== LOCAL_HOST) {
    throw new Error(`拒绝连接非本机数据库：${host}`);
  }
  const password = envv('AETHERLINK_DB_PASSWORD');
  if (!password) {
    throw new Error('需要 AETHERLINK_DB_PASSWORD 才能为预注册用例种产品前置数据');
  }
  return {
    host,
    port: envv('AETHERLINK_DB_PORT', '55433'),
    user: envv('AETHERLINK_DB_USER', 'postgres'),
    password,
    database: envv('AETHERLINK_DB_NAME', 'aetherlink_go99')
  };
}

function findPsql() {
  const candidates = [envv('PSQL_PATH'), 'C:\\Program Files\\PostgreSQL\\17\\bin\\psql.exe', 'psql'].filter(Boolean);
  const hit = candidates.find(candidate => candidate === 'psql' || fs.existsSync(candidate));
  if (!hit) throw new Error('找不到 psql.exe；可设 PSQL_PATH 指向本地 PostgreSQL 客户端');
  return hit;
}

function psql(sql) {
  const options = dbConfig();
  return new Promise((resolve, reject) => {
    const child = spawn(
      findPsql(),
      ['-X', '-v', 'ON_ERROR_STOP=1', '-h', options.host, '-p', options.port, '-U', options.user, '-d', options.database, '-At', '-q', '-c', sql],
      { env: { ...process.env, PGPASSWORD: options.password }, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] }
    );
    let out = '';
    let err = '';
    child.stdout.on('data', chunk => (out += chunk));
    child.stderr.on('data', chunk => (err += chunk));
    child.on('error', reject);
    child.on('close', code => {
      if (code === 0) resolve(out.trim());
      else reject(new Error(err.trim() || `psql 退出码 ${code}`));
    });
  });
}

async function seedProduct(name, account) {
  const resp = await apiClient.post('/product', { name, conflict_policy: 'allow' }, account);
  if (!resp || !resp.data) {
    throw new Error(`创建产品失败: ${JSON.stringify(resp)}`);
  }
  return resp.data;
}

async function deleteProduct(id, account) {
  if (!id) return;
  try {
    await apiClient.delete(`/product/${id}`, {}, account);
  } catch (e) {
    // ignore cleanup error
  }
}

// ---------------------------------------------------------------------------
// 页面交互辅助
// ---------------------------------------------------------------------------

function csvText(rows) {
  return [CSV_HEADER, ...rows].join('\n') + '\n';
}

function uniqueBatch() {
  return `${BATCH_PREFIX}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`;
}

/** 带连字符，避开 12 位纯字母数字（RDI 判定） */
function uniqueDeviceNumber(tag) {
  return `p05-${tag}-${Math.random().toString(36).slice(2, 10)}`;
}

async function openImportModal(page) {
  await page.goto('/product/pre-register', { waitUntil: 'domcontentloaded' });
  await page.getByRole('button', { name: /Import Devices|导入设备/ }).first().click();
  await expect(page.getByText(/From CSV File|CSV 导入/).first()).toBeVisible({ timeout: 20000 });
}

async function switchToCsvMode(page) {
  await page.getByText(/From CSV File|CSV 导入/).first().click();
}

/**
 * 打开产品下拉并选中指定产品。
 *
 * 必须限定在弹窗内：列表页筛选栏上也有一个 `.n-select`（`.n-select.filter-control`），
 * 直接取 `.n-select` 的第一个会点到筛选栏，且被弹窗遮罩拦截点击。
 */
async function openProductDropdown(page) {
  await page.locator('.import-modal .n-select').first().click();
  await expect(page.locator('.n-base-select-option').first()).toBeVisible({ timeout: 15000 });
}

async function selectProduct(page, productName) {
  await openProductDropdown(page);
  await page.getByText(productName, { exact: true }).first().click({ timeout: 15000 });
}

/**
 * 填批次号。
 *
 * 必须限定在弹窗内：列表页筛选栏的批次号筛选框用的是**同一个 i18n placeholder**
 * （`batchPlaceholder`），所以 `getByPlaceholder(...)` 会命中 2 个元素；
 * 取 `.first()` 会填到弹窗背后那个筛选框，弹窗里的批次号仍为空，
 * 表现为"提交按钮一直 disabled"——很难从表象看出根因。
 */
async function fillBatchNumber(page, batch) {
  await page.locator('.import-modal').getByPlaceholder(/Enter batch number|输入批次编号/).first().fill(batch);
}

/** 走真实 file chooser 路径：定位 NUpload 内部的 input[type=file] 塞文件 */
async function chooseCsvFile(page, content, filename) {
  await page.locator('.import-modal input[type="file"]').first().setInputFiles({
    name: filename,
    mimeType: 'text/csv',
    buffer: Buffer.from(content, 'utf8')
  });
}

async function submitImport(page) {
  await page.getByRole('button', { name: /Create Devices|创建设备/ }).first().click();
}

/** 回收本用例建的预注册设备；失败只记录，不掩盖断言结论 */
async function cleanupBatch(api, productId, batch) {
  if (!productId || !batch) return;
  try {
    await api.post('/device/preRegister/cleanup', { product_id: productId, batch_number: batch }, 'tenant_admin');
  } catch (e) {
    console.warn(`[cleanup] 批次 ${batch} 回收失败：${e && e.message}`);
  }
}

// ---------------------------------------------------------------------------
// 前置数据
// ---------------------------------------------------------------------------

let ownProduct = null;
let foreignProduct = null;
let fixtureSkipReason = '';

test.beforeAll(async () => {
  try {
    await apiClient.login('tenant_admin');
    await apiClient.login('tenant_admin_b');

    ownProduct = await seedProduct(`p05-own-${Date.now().toString(36)}`, 'tenant_admin');
    // 跨租户用例：属于 tenant_admin_b 的产品，本租户列表里绝不能出现
    foreignProduct = await seedProduct(`p05-foreign-${Date.now().toString(36)}`, 'tenant_admin_b');
  } catch (e) {
    fixtureSkipReason = `无法准备产品前置数据：${e && e.message}`;
  }
});

test.afterAll(async () => {
  try {
    if (ownProduct) await deleteProduct(ownProduct.id || ownProduct.ID, 'tenant_admin');
    if (foreignProduct) await deleteProduct(foreignProduct.id || foreignProduct.ID, 'tenant_admin_b');
  } catch (e) {
    console.warn(`[cleanup] 产品前置数据清理失败：${e && e.message}`);
  }
});

// ---------------------------------------------------------------------------
// 用例
// ---------------------------------------------------------------------------

test.describe('P0.5 预注册 CSV 浏览器 E2E', () => {
  // 每条用例都要开弹窗、等产品下拉、走一次上传 + 提交，默认 30s 不够
  test.describe.configure({ timeout: 90000 });


  test.beforeEach(() => {
    test.skip(Boolean(fixtureSkipReason), fixtureSkipReason);
  });

  test("真实浏览器选文件导入 CSV，并渲染出一次性凭证", async ({ rolePage, api }) => {
    const batch = uniqueBatch();
    const numbers = [uniqueDeviceNumber('a'), uniqueDeviceNumber('b')];

    try {
      await openImportModal(rolePage);
      await switchToCsvMode(rolePage);
      await selectProduct(rolePage, ownProduct.name);
      await fillBatchNumber(rolePage, batch);
      await chooseCsvFile(rolePage, csvText([[numbers[0], 'P05 Device A'], [numbers[1], 'P05 Device B']]), 'p05-ok.csv');
      await submitImport(rolePage);

      // 门禁 3：批量建档成功
      await expect(rolePage.getByText(/Created 2 devices|已创建 2 台设备/).first()).toBeVisible({ timeout: 30000 });

      // 门禁 3（关键）：凭证必须真的渲染出来。
      // 这一条正是本次修掉的缺陷——修复前结果面板只有提示文案、没有凭证。
      await expect(rolePage.getByTestId('pre-register-credential-row')).toHaveCount(2, { timeout: 15000 });

      for (const number of numbers) {
        await expect(
          rolePage.getByTestId('pre-register-credential-number').filter({ hasText: number }).first()
        ).toBeVisible();
      }

      const vouchers = await rolePage.getByTestId('pre-register-credential-voucher').allInnerTexts();
      expect(vouchers, '每台设备都应展示一条凭证').toHaveLength(2);
      for (const voucher of vouchers) {
        expect(voucher.trim().length, '凭证不能为空').toBeGreaterThan(0);
        // 与后端 newPreRegisterDevice 的约定一致：voucher 是含 username 的 JSON 明文
        expect(voucher).toContain('username');
      }
      expect(new Set(vouchers.map(v => v.trim())).size, '两台设备的凭证必须不同').toBe(2);
    } finally {
      await cleanupBatch(api, ownProduct.id, batch);
    }
  });

  /**
   * ⚠️ 2026-09-16 未通过，卡在**后端错误消息模板**这一环（不是上传、不是前端传参）。
   *
   * 排查结论（已用排除法定论，详见
   * docs/validation/2026-09-15-p05-csv-browser-evidence.md §3.6）：
   *   - 请求体完全正确：batch_file 带值，与 curl 成功那次完全同形；
   *   - 后端行为**正确**：buildFilePreRegisterRows 识别出第 2 行缺 name，
   *     并带上了 csv_row: 2 与 message: "device_number and name are required"；
   *   - 但错误模板 backend/configs/messages.yaml:26 写死了
   *       zh_CN: "${field}不能为空"
   *     只插值 ${field}，把 message 与 csv_row **一起丢掉**，
   *     渲染成「batch_file不能为空」。
   *
   * 前端侧已修（提交 9c6790e）：此前 submitImport 只有 try/finally 没有 catch，
   * 而请求层是 reject 抛错的，导致 submitError 永不赋值、页面无任何反馈；
   * 现在 alert 能显示了，内容就是上面那句被吞掉子原因的文本。
   *
   * 所以本用例要转正，需要先修错误模板（让它优先透出调用方 message 并保留 csv_row）。
   * 模板是全站共享的（100005 被多处使用），改动需配套回归测试。
   */
  test("坏行逐行反馈：缺字段的行要带上 csv_row 行号", async ({ rolePage, api }) => {
    const batch = uniqueBatch();

    try {
      await openImportModal(rolePage);
      await switchToCsvMode(rolePage);
      await selectProduct(rolePage, ownProduct.name);
      await fillBatchNumber(rolePage, batch);

      // 表头算第 1 行，因此这条数据是第 2 行：name 为空
      await chooseCsvFile(rolePage, csvText([[uniqueDeviceNumber('bad'), '']]), 'p05-bad-row.csv');
      await submitImport(rolePage);

      // 门禁 2：坏行必须被拒绝并给出反馈
      const alert = rolePage.locator('.n-alert').filter({ hasText: /\S/ }).first();
      await expect(alert).toBeVisible({ timeout: 30000 });

      const text = (await alert.innerText()).trim();
      expect(text.length, '坏行必须给出可读的错误信息').toBeGreaterThan(0);
      expect(text, `坏行反馈应包含行号 2，实际显示："${text}"`).toMatch(/\b2\b/);

      // 整批 fail-fast：不应出现成功结果面板
      await expect(rolePage.getByTestId('pre-register-credentials')).toHaveCount(0);
    } finally {
      await cleanupBatch(api, ownProduct.id, batch);
    }
  });

  /**
   * ⚠️ 2026-09-16 未通过：**错误路径是另一条独立缺陷，与上传根因无关**。
   *
   * 实测（page.on('response') 抓 POST /device/preRegister）：
   *   提交坏行 CSV 时后端返回 {"code":100005,"message":"batch_file不能为空"}
   *   —— 请求里**根本没带 batch_file**（不是 CSV 内容校验失败）。
   *   而且页面既没有 .n-alert 也没有 toast，**用户看不到任何反馈**。
   *
   * 所以这条用例目前测不到"逐行反馈"，它先卡在更前面：文件路径没传上去。
   * 修好 batch_file 传递后，才能验证 csv_row 行号展示。
   * 排查入口：use-pre-register-import.ts:127 的 payload.batch_file = uploadedPath.value
   * 与 submitImport 里 mode.value === 'file' 的分支判断。
   */
  test("表头不合规的文件被拒绝", async ({ rolePage, api }) => {
    const batch = uniqueBatch();

    try {
      await openImportModal(rolePage);
      await switchToCsvMode(rolePage);
      await selectProduct(rolePage, ownProduct.name);
      await fillBatchNumber(rolePage, batch);

      await chooseCsvFile(
        rolePage,
        ['sn,name', `${uniqueDeviceNumber('hdr')},Header Wrong`].join('\n') + '\n',
        'p05-bad-header.csv'
      );
      await submitImport(rolePage);

      const alert = rolePage.locator('.n-alert').filter({ hasText: /\S/ }).first();
      await expect(alert).toBeVisible({ timeout: 30000 });
      await expect(rolePage.getByTestId('pre-register-credentials')).toHaveCount(0);
    } finally {
      await cleanupBatch(api, ownProduct.id, batch);
    }
  });

  test("凭证只出现一次：关闭弹窗重开后不再展示", async ({ rolePage, api }) => {
    const batch = uniqueBatch();

    try {
      await openImportModal(rolePage);
      await switchToCsvMode(rolePage);
      await selectProduct(rolePage, ownProduct.name);
      await fillBatchNumber(rolePage, batch);
      await chooseCsvFile(rolePage, csvText([[uniqueDeviceNumber('once'), 'P05 Once']]), 'p05-once.csv');
      await submitImport(rolePage);

      await expect(rolePage.getByTestId('pre-register-credentials')).toBeVisible({ timeout: 30000 });

      // 关掉弹窗再重开：openModal 会重置 importResult，凭证不应残留
      await rolePage.locator('.import-modal').getByRole('button', { name: /Cancel|取消/ }).first().click();
      await rolePage.waitForTimeout(800);
      await openImportModal(rolePage);

      await expect(rolePage.getByTestId('pre-register-credentials')).toHaveCount(0);
    } finally {
      await cleanupBatch(api, ownProduct.id, batch);
    }
  });

  test('跨租户产品不可选', async ({ rolePage }) => {
    await openImportModal(rolePage);
    await openProductDropdown(rolePage);

    // 门禁 5：他租户的产品不应出现在本租户的下拉里
    await expect(rolePage.getByText(foreignProduct.name, { exact: true })).toHaveCount(0);
    // 反向对照：本租户的产品必须能出现。否则上面那条断言可能只是因为下拉根本没加载 ——
    // 一个永远为真的"不存在"断言是没有价值的。
    await expect(rolePage.getByText(ownProduct.name, { exact: true }).first()).toBeVisible({ timeout: 15000 });
  });
});
