/**
 * 文件用途：用于支撑 automation_tests 的自动化运行时配置解析模块。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：共享库变更会影响多类自动化套件，必须保持错误信息和前置条件可诊断。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const fs = require('fs');
const path = require('path');

const configPath = path.join(__dirname, '..', 'config.json');
const fileConfig = JSON.parse(fs.readFileSync(configPath, 'utf8'));

// 自动加载 .env.local（若存在），让 npx mocha / playwright / node run_tests.js
// 三类入口都能读到 prepare:local-accounts 写入的账号与 API_BASE_URL 等运行时变量。
// 已存在的 process.env 值优先，不覆盖调用方显式注入的环境变量。
// 测试隔离可设置 AETHERLINK_RUNTIME_CONFIG_SKIP_ENV_FILE=1，避免共享的
// .env.local 把“未设置环境变量”测试误变成“从 dotenv 文件设置”。
function loadEnvLocalIfPresent() {
  if (process.env.AETHERLINK_RUNTIME_CONFIG_SKIP_ENV_FILE === '1') return;

  const envLocalPath = path.join(__dirname, '..', '.env.local');
  if (!fs.existsSync(envLocalPath)) return;
  const raw = fs.readFileSync(envLocalPath, 'utf8');
  for (const rawLine of raw.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith('#')) continue;
    const eqIndex = line.indexOf('=');
    if (eqIndex < 0) continue;
    const key = line.slice(0, eqIndex).trim();
    if (!key) continue;
    let value = line.slice(eqIndex + 1).trim();
    if ((value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    if (process.env[key] === undefined) {
      process.env[key] = value;
    }
  }
}
loadEnvLocalIfPresent();

function envOrDefault(name, fallback) {
  return process.env[name] || fallback;
}

const ACCOUNT_ENV_OVERRIDES = {
  super_admin: ['SUPER_ADMIN_EMAIL', 'SUPER_ADMIN_PASSWORD'],
  tenant_admin: ['TENANT_ADMIN_EMAIL', 'TENANT_ADMIN_PASSWORD'],
  tenant_user: ['TENANT_USER_EMAIL', 'TENANT_USER_PASSWORD'],
  tenant_admin_b: ['TENANT_ADMIN_B_EMAIL', 'TENANT_ADMIN_B_PASSWORD'],
  readonly_user: ['READONLY_USER_EMAIL', 'READONLY_USER_PASSWORD'],
  email_change_tenant: ['EMAIL_CHANGE_TENANT_EMAIL', 'EMAIL_CHANGE_TENANT_PASSWORD']
};

function withRuntimePaths(config) {
  return {
    ...config,
    report: {
      ...config.report,
      outputDir: envOrDefault('AUTOMATION_REPORT_DIR', config.report.outputDir)
    },
    e2e: {
      ...config.e2e,
      storageStateDir: envOrDefault('E2E_AUTH_DIR', config.e2e.storageStateDir)
    }
  };
}

const RELEASE_REQUIRED_ACCOUNTS = [
  'super_admin',
  'tenant_admin',
  'tenant_admin_b',
  'tenant_user',
  'readonly_user',
  'email_change_tenant'
];

function accountsWithEnvOverrides(accounts = {}) {
  const next = {};

  for (const [key, account] of Object.entries(accounts)) {
    const [emailEnv, passwordEnv] = ACCOUNT_ENV_OVERRIDES[key] || [];
    next[key] = {
      ...account,
      email: emailEnv ? envOrDefault(emailEnv, account.email) : account.email,
      password: passwordEnv ? envOrDefault(passwordEnv, account.password) : account.password
    };
  }

  return next;
}

module.exports = withRuntimePaths({
  ...fileConfig,
  baseURL: envOrDefault('API_BASE_URL', fileConfig.baseURL),
  healthURL: envOrDefault('HEALTH_URL', fileConfig.healthURL),
  frontendURL: envOrDefault('FRONTEND_URL', fileConfig.frontendURL),
  accounts: accountsWithEnvOverrides(fileConfig.accounts),
  accountEnvOverrides: ACCOUNT_ENV_OVERRIDES,
  releaseRequiredAccounts: RELEASE_REQUIRED_ACCOUNTS
});
