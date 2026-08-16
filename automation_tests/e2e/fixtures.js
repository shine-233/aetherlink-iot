/**
 * 文件用途：用于支撑Playwright 共享 fixture 模块。
 * 核心逻辑：封装 Playwright 上下文、路由访问、页面覆盖率记录或通用断言，供多个 E2E 用例复用。
 * 关键注意事项：helper 不应隐藏关键业务断言；缺少数据或账号时要显式暴露前置条件。
 * 重构建议：新增复用能力时优先保持小接口，并把业务断言留在调用方或专门的断言 helper 中。
 */

const pw = require('@playwright/test');
const base = pw.test;
const expect = pw.expect;
const path = require('path');
const fs = require('fs');
const config = require('../lib/network_runtime');
const apiClient = require('../lib/api_client');
const testData = require('../lib/test_data');
const selectors = require('./selectors');
const pageCoverage = require('../lib/page_coverage');

const AUTH_DIR = path.resolve(__dirname, '..', config.e2e.storageStateDir);

// 角色到 storageState 文件名的映射（与 auth.setup.js 保持一致）
const ROLE_FILES = {
  super_admin: 'super-admin.json',
  tenant_admin: 'tenant-admin.json',
  tenant_user: 'tenant-user.json',
  tenant_admin_b: 'tenant-admin-b.json',
  readonly_user: 'readonly-user.json',
  email_change_tenant: 'email-change-tenant.json'
};

function instrumentPageCoverage(page) {
  let gotoInFlight = 0;
  page.on('framenavigated', frame => {
    if (frame === page.mainFrame() && gotoInFlight === 0) {
      // SPA router.push/history navigation does not pass through page.goto.
      pageCoverage.hitPage(frame.url());
    }
  });

  const originalGoto = page.goto.bind(page);
  page.goto = async function(url, options) {
    gotoInFlight++;
    let completed = false;
    try {
      const response = await originalGoto(url, options);
      completed = true;
      return response;
    } finally {
      gotoInFlight--;
      // Record only the settled URL, after redirects and failed navigations.
      if (completed && gotoInFlight === 0) pageCoverage.hitPage(page.url());
    }
  };
}

/**
 * 获取指定角色的 storageState 文件路径
 */
function getStorageStatePath(role) {
  const file = ROLE_FILES[role];
  if (!file) {
    throw new Error('未知角色: ' + role + '，可用: ' + Object.keys(ROLE_FILES).join(', '));
  }
  return path.join(AUTH_DIR, file);
}

/**
 * 判断指定角色的 storageState 是否存在
 */
function storageStateExists(role) {
  return fs.existsSync(getStorageStatePath(role));
}

/**
 * 扩展后的 test fixture，提供 rolePage / api / data 等便捷对象
 */
const test = base.extend({
  page: async ({ page }, use) => {
    instrumentPageCoverage(page);

    await use(page);
  },

  /**
   * 当前测试使用的账号角色。
   * Playwright 自定义 option 必须先声明，test.use({ role }) 才会稳定生效。
   */
  role: ['tenant_admin', { option: true }],

  /**
   * 提供一个已注入指定角色登录态的 page
   * 用法：
   *   test('用例', async ({ rolePage }) => { ... })
   *   test.use({ role: 'tenant_admin' })
   */
  rolePage: async ({ browser, role }, use) => {
    const storageStatePath = getStorageStatePath(role);

    if (!storageStateExists(role)) {
      throw new Error('角色 ' + role + ' 的登录态文件不存在: ' + storageStatePath + '，请先运行 globalSetup');
    }

    const context = await browser.newContext({
      storageState: storageStatePath,
      baseURL: config.frontendURL
    });
    const page = await context.newPage();

    // 拦截 page.goto() 调用，自动记录页面覆盖率
    instrumentPageCoverage(page);

    await use(page);
    await context.close();
  },

  /**
   * API 辅助对象，复用 lib/api_client.js
   */
  api: async ({}, use) => {
    await use({
      client: apiClient,
      config: config,
      /**
       * 以指定角色登录并返回 token
       */
      login: (accountKey) => apiClient.login(accountKey),
      /**
       * GET 请求
       */
      get: (url, params, accountKey) => apiClient.get(url, params, accountKey),
      getNoAuth: (url, params) => apiClient.getNoAuth(url, params),
      /**
       * POST 请求
       */
      post: (url, data, accountKey) => apiClient.post(url, data, accountKey),
      postNoAuth: (url, data) => apiClient.postNoAuth(url, data),
      /**
       * PUT 请求
       */
      put: (url, data, accountKey) => apiClient.put(url, data, accountKey),
      putNoAuth: (url, data) => apiClient.putNoAuth(url, data),
      /**
       * DELETE 请求
       */
      delete: (url, data, accountKey) => apiClient.delete(url, data, accountKey),
      deleteNoAuth: (url, data) => apiClient.deleteNoAuth(url, data),
      /**
       * 探测账号在当前本地后端是否可登录
       */
      isAccountAvailable: (accountKey) => apiClient.isAccountAvailable(accountKey)
    });
  },

  /**
   * 测试数据生成辅助
   */
  data: async ({}, use) => {
    await use({
      /**
       * 获取测试设备 PID
       */
      device: testData.getConfig().testDevice,
      /**
       * 获取测试账号信息
       */
      account: (key) => config.accounts[key],
      /**
       * 生成唯一标识（用于避免重复数据冲突）
       */
      uuid: () => 'e2e-' + Date.now() + '-' + Math.floor(Math.random() * 10000),
      /**
       * 当前时间戳字符串
       */
      timestamp: () => new Date().toISOString().replace(/[:.]/g, '-')
    });
  },

  /**
   * 页面导航辅助
   */
  nav: async ({}, use) => {
    await use({
      /**
       * 选择器集合
       */
      selectors,
      /**
       * 等待页面主内容加载完成
       */
      waitForAppReady: async (page) => {
        await page.waitForLoadState('domcontentloaded');
      },
      /**
       * 导航到指定路径并等待就绪
       */
      goto: async (page, url) => {
        await page.goto(url, { waitUntil: 'domcontentloaded' });
      }
    });
  }
});

module.exports = {
  test,
  expect,
  getStorageStatePath,
  storageStateExists,
  instrumentPageCoverage
};
