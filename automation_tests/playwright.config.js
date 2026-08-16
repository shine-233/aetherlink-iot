/**
 * 文件用途：用于配置Playwright E2E 运行配置。
 * 核心逻辑：定义 Playwright 项目、账号状态、报告、超时和预览访问参数，供 E2E 套件统一读取。
 * 关键注意事项：配置变化会影响所有 E2E 证据；当前任务不启动浏览器或服务，只验证语法。
 * 重构建议：若环境矩阵继续扩大，应把本地、预览和 CI 配置分层，减少条件分支堆积。
 */

const path = require('path');
const { defineConfig, devices } = require('@playwright/test');
const config = require('./lib/runtime_config');
const networkConfig = require('./lib/network_runtime');

const frontendDir = path.resolve(__dirname, '../frontend');
const browserChannel = process.env.PLAYWRIGHT_BROWSER_CHANNEL || 'msedge';
const browserExecutablePath = process.env.PLAYWRIGHT_BROWSER_EXECUTABLE_PATH;
const reportsDir = path.resolve(__dirname, config.report.outputDir);

function isTruthyEnv(value) {
  return ['1', 'true', 'yes', 'on'].includes(String(value || '').toLowerCase());
}

function isPreviewURL(value) {
  try {
    return new URL(value).port === '9725';
  } catch (error) {
    return false;
  }
}

const shouldUsePreviewProxy =
  isTruthyEnv(process.env.PLAYWRIGHT_USE_PREVIEW_PROXY) ||
  isPreviewURL(config.frontendURL) ||
  isPreviewURL(process.env.PREVIEW_URL);

const webServerCommand =
  process.env.PLAYWRIGHT_WEBSERVER_COMMAND ||
  (shouldUsePreviewProxy ? 'node scripts/serve_preview_with_api_proxy.js' : 'pnpm dev');
const webServerCwd =
  process.env.PLAYWRIGHT_WEBSERVER_CWD ||
  (shouldUsePreviewProxy ? __dirname : frontendDir);
const reuseExistingServer = process.env.PLAYWRIGHT_REUSE_EXISTING_SERVER === undefined
  ? !process.env.CI && !shouldUsePreviewProxy
  : isTruthyEnv(process.env.PLAYWRIGHT_REUSE_EXISTING_SERVER);

const browserUse = { ...devices['Desktop Chrome'] };

if (browserExecutablePath) {
  browserUse.launchOptions = {
    executablePath: browserExecutablePath
  };
} else {
  browserUse.channel = browserChannel;
}

module.exports = defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: config.e2e.retries,
  workers: process.env.PAGE_COVERAGE_FILE ? 1 : config.e2e.workers,
  reporter: [
    ['html', { outputFolder: path.join(reportsDir, 'e2e-html'), open: 'never' }],
    ['json', { outputFile: path.join(reportsDir, 'e2e-results.json') }],
    ['list']
  ],
  use: {
    baseURL: networkConfig.frontendURL,
    trace: config.e2e.trace,
    screenshot: config.e2e.screenshotOnFailure ? 'only-on-failure' : 'off',
    video: 'retain-on-failure',
    actionTimeout: config.e2e.timeout,
    navigationTimeout: config.e2e.timeout
  },
  projects: [
    { name: browserChannel || 'system-browser', use: browserUse }
  ],
  globalSetup: require.resolve('./e2e/auth.setup.js'),
  webServer: {
    command: webServerCommand,
    cwd: webServerCwd,
    url: networkConfig.frontendURL,
    reuseExistingServer,
    timeout: 120000
  }
});
