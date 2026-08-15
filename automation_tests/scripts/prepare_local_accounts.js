/**
 * 文件用途：用于执行本地自动化账号准备脚本。
 * 核心逻辑：作为独立 Node 脚本编排本地预检、账号准备、预览代理或页面渲染验证，并输出可诊断结果。
 * 关键注意事项：运行前必须确认目标环境、账号和端口配置，避免把预检失败误判为业务失败。
 * 重构建议：后续应把环境解析、错误分类和可复用检查步骤抽到共享库，保持脚本入口薄而明确。
 */

const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const axios = require('axios');
const runtimeConfig = require('../lib/runtime_config');

const rootDir = path.join(__dirname, '..');

function getAccountOutputPaths(env = process.env) {
  const accountDirOverride = env.AUTOMATION_ACCOUNT_DIR;
  const localDir = path.resolve(rootDir, accountDirOverride || '.local');

  return {
    localDir,
    envPs1Path: path.join(localDir, 'automation-env.ps1'),
    // Preserve the historical root .env.local for the shared default run,
    // while ensuring a concurrent instance never writes credentials there.
    envLocalPath: accountDirOverride
      ? path.join(localDir, '.env.local')
      : path.join(rootDir, '.env.local')
  };
}

const { localDir, envPs1Path, envLocalPath } = getAccountOutputPaths();

const ISOLATION_ENV_KEYS = [
  'AUTOMATION_REPORT_DIR',
  'AUTOMATION_VERIFICATION_DIR',
  'E2E_AUTH_DIR',
  'AUTOMATION_ACCOUNT_DIR'
];

function configuredIsolationEnvironment() {
  return ISOLATION_ENV_KEYS
    .filter(key => Boolean(process.env[key]))
    .map(key => [key, process.env[key]]);
}

function randomPassword() {
  return `Ae@${crypto.randomBytes(5).toString('hex')}1`;
}

function uniqueEmail(role) {
  const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
  const rand = crypto.randomBytes(3).toString('hex');
  return `automation.${role}.${stamp}.${rand}@example.test`;
}

function requireHttpURL(value, name) {
  try {
    const url = new URL(value);
    if (!/^https?:$/.test(url.protocol)) {
      throw new Error('not http');
    }
    return url.toString().replace(/\/$/, '');
  } catch (error) {
    throw new Error(`${name} must be an HTTP URL, got: ${value || '<empty>'}`);
  }
}

function accountFromEnv(key, defaults = {}) {
  const upper = key.toUpperCase();
  return {
    email: process.env[`${upper}_EMAIL`] || defaults.email || uniqueEmail(key.replace(/_/g, '-')),
    password: process.env[`${upper}_PASSWORD`] || defaults.password || randomPassword()
  };
}

function expectOk(resp, label) {
  if (!resp || resp.code !== 200) {
    throw new Error(`${label} failed: ${JSON.stringify(resp)}`);
  }
  return resp;
}

function listFrom(resp) {
  if (Array.isArray(resp?.data)) return resp.data;
  if (Array.isArray(resp?.data?.list)) return resp.data.list;
  return [];
}

function extractId(entity) {
  return entity && (entity.id || entity.ID || entity.user_id || entity.UserID);
}

class LocalAutomationAccountClient {
  constructor(baseURL) {
    this.baseURL = baseURL;
    this.tokens = {};
    this.http = axios.create({
      baseURL,
      timeout: Number(runtimeConfig.timeout || 30000),
      headers: { 'Content-Type': 'application/json' }
    });
  }

  async getNoAuth(url, params = {}) {
    try {
      const resp = await this.http.get(url, { params });
      return resp.data;
    } catch (error) {
      return error.response ? error.response.data : { code: -1, message: error.message };
    }
  }

  async postNoAuth(url, data = {}) {
    try {
      const resp = await this.http.post(url, data);
      return resp.data;
    } catch (error) {
      return error.response ? error.response.data : { code: -1, message: error.message };
    }
  }

  async login(key, account) {
    const resp = expectOk(await this.postNoAuth('/login', account), `login ${key}`);
    const token = resp.data && resp.data.token;
    if (!token) {
      throw new Error(`login ${key} did not return a token: ${JSON.stringify(resp)}`);
    }
    this.tokens[key] = token;
    return token;
  }

  async request(method, url, data, accountKey) {
    const token = this.tokens[accountKey];
    if (!token) {
      throw new Error(`missing token for ${accountKey}`);
    }
    try {
      const resp = await this.http.request({
        method,
        url,
        data,
        headers: { 'x-token': token }
      });
      return resp.data;
    } catch (error) {
      return error.response ? error.response.data : { code: -1, message: error.message };
    }
  }

  async createUser(ownerKey, account, name, phoneTail) {
    const createResp = await this.request(
      'post',
      '/user',
      {
        email: account.email,
        password: account.password,
        name,
        phone_number: `+86 139${phoneTail}`
      },
      ownerKey
    );
    expectOk(createResp, `create ${account.email}`);

    const listResp = await this.request('get', `/user?page=1&page_size=10&email=${encodeURIComponent(account.email)}`, null, ownerKey);
    expectOk(listResp, `list ${account.email}`);
    const created = listFrom(listResp).find(item => item.email === account.email);
    const userId = extractId(created);
    if (!userId) {
      throw new Error(`created user ${account.email} was not visible in /user list`);
    }
    return userId;
  }

  async canLogin(key, account) {
    try {
      await this.login(key, account);
      return true;
    } catch (error) {
      return false;
    }
  }
}

function phoneTail(index) {
  return String(Date.now()).slice(-7) + String(index);
}

function writeEnvFiles(accounts, urls) {
  fs.mkdirSync(localDir, { recursive: true });
  const isolationEnvironment = configuredIsolationEnvironment();

  const ps1Lines = [
    '# Generated by npm run prepare:local-accounts. Do not commit this file.',
    `$env:SUPER_ADMIN_EMAIL = ${JSON.stringify(accounts.super_admin.email)}`,
    `$env:SUPER_ADMIN_PASSWORD = ${JSON.stringify(accounts.super_admin.password)}`,
    `$env:TENANT_ADMIN_EMAIL = ${JSON.stringify(accounts.tenant_admin.email)}`,
    `$env:TENANT_ADMIN_PASSWORD = ${JSON.stringify(accounts.tenant_admin.password)}`,
    `$env:TENANT_ADMIN_B_EMAIL = ${JSON.stringify(accounts.tenant_admin_b.email)}`,
    `$env:TENANT_ADMIN_B_PASSWORD = ${JSON.stringify(accounts.tenant_admin_b.password)}`,
    `$env:TENANT_USER_EMAIL = ${JSON.stringify(accounts.tenant_user.email)}`,
    `$env:TENANT_USER_PASSWORD = ${JSON.stringify(accounts.tenant_user.password)}`,
    `$env:READONLY_USER_EMAIL = ${JSON.stringify(accounts.readonly_user.email)}`,
    `$env:READONLY_USER_PASSWORD = ${JSON.stringify(accounts.readonly_user.password)}`,
    `$env:EMAIL_CHANGE_TENANT_EMAIL = ${JSON.stringify(accounts.email_change_tenant.email)}`,
    `$env:EMAIL_CHANGE_TENANT_PASSWORD = ${JSON.stringify(accounts.email_change_tenant.password)}`,
    `$env:FRONTEND_URL = ${JSON.stringify(urls.frontendURL)}`,
    `$env:PREVIEW_URL = ${JSON.stringify(urls.previewURL)}`,
    `$env:API_BASE_URL = ${JSON.stringify(urls.apiBaseURL)}`,
    `$env:API_TARGET = ${JSON.stringify(urls.apiTarget)}`,
    '$env:PLAYWRIGHT_USE_PREVIEW_PROXY = "1"',
    '$env:PLAYWRIGHT_REUSE_EXISTING_SERVER = "0"',
    ...isolationEnvironment.map(([key, value]) => `$env:${key} = ${JSON.stringify(value)}`)
  ];

  const dotenvLines = [
    '# Generated by npm run prepare:local-accounts. Do not commit this file.',
    `SUPER_ADMIN_EMAIL=${accounts.super_admin.email}`,
    `SUPER_ADMIN_PASSWORD=${accounts.super_admin.password}`,
    `TENANT_ADMIN_EMAIL=${accounts.tenant_admin.email}`,
    `TENANT_ADMIN_PASSWORD=${accounts.tenant_admin.password}`,
    `TENANT_ADMIN_B_EMAIL=${accounts.tenant_admin_b.email}`,
    `TENANT_ADMIN_B_PASSWORD=${accounts.tenant_admin_b.password}`,
    `TENANT_USER_EMAIL=${accounts.tenant_user.email}`,
    `TENANT_USER_PASSWORD=${accounts.tenant_user.password}`,
    `READONLY_USER_EMAIL=${accounts.readonly_user.email}`,
    `READONLY_USER_PASSWORD=${accounts.readonly_user.password}`,
    `EMAIL_CHANGE_TENANT_EMAIL=${accounts.email_change_tenant.email}`,
    `EMAIL_CHANGE_TENANT_PASSWORD=${accounts.email_change_tenant.password}`,
    `FRONTEND_URL=${urls.frontendURL}`,
    `PREVIEW_URL=${urls.previewURL}`,
    `API_BASE_URL=${urls.apiBaseURL}`,
    `API_TARGET=${urls.apiTarget}`,
    'PLAYWRIGHT_USE_PREVIEW_PROXY=1',
    'PLAYWRIGHT_REUSE_EXISTING_SERVER=0',
    ...isolationEnvironment.map(([key, value]) => `${key}=${value}`)
  ];

  fs.writeFileSync(envPs1Path, `${ps1Lines.join('\n')}\n`, 'utf8');
  fs.writeFileSync(envLocalPath, `${dotenvLines.join('\n')}\n`, 'utf8');
}

async function main() {
  const apiBaseURL = requireHttpURL(process.env.API_BASE_URL || runtimeConfig.baseURL, 'API_BASE_URL');
  const apiTarget = requireHttpURL(process.env.API_TARGET || new URL(apiBaseURL).origin, 'API_TARGET');
  const frontendURL = requireHttpURL(process.env.FRONTEND_URL || 'http://127.0.0.1:9725', 'FRONTEND_URL');
  const previewURL = requireHttpURL(process.env.PREVIEW_URL || frontendURL, 'PREVIEW_URL');
  const client = new LocalAutomationAccountClient(apiBaseURL);

  const accounts = {
    super_admin: accountFromEnv('super_admin'),
    tenant_admin: accountFromEnv('tenant_admin'),
    tenant_admin_b: accountFromEnv('tenant_admin_b'),
    tenant_user: accountFromEnv('tenant_user'),
    readonly_user: accountFromEnv('readonly_user'),
    email_change_tenant: accountFromEnv('email_change_tenant')
  };

  const hasAdminResp = expectOk(await client.getNoAuth('/tenant/has-admin'), 'check super admin');
  const hasAdmin = Boolean(hasAdminResp.data && hasAdminResp.data.has_admin);

  if (!hasAdmin) {
    const initResp = expectOk(
      await client.postNoAuth('/tenant/super-admin/init', {
        email: accounts.super_admin.email,
        password: accounts.super_admin.password,
        market_registered: true,
        market_email: accounts.super_admin.email,
        market_source: 'local-automation'
      }),
      'initialize super admin'
    );
    if (initResp.data && initResp.data.token) {
      client.tokens.super_admin = initResp.data.token;
    } else {
      await client.login('super_admin', accounts.super_admin);
    }
  } else {
    if (!process.env.SUPER_ADMIN_EMAIL || !process.env.SUPER_ADMIN_PASSWORD) {
      throw new Error(
        'A super admin already exists. Set SUPER_ADMIN_EMAIL and SUPER_ADMIN_PASSWORD for that local account, then rerun this script.'
      );
    }
    await client.login('super_admin', accounts.super_admin);
  }

  if (!(await client.canLogin('tenant_admin', accounts.tenant_admin))) {
    await client.createUser('super_admin', accounts.tenant_admin, 'Automation Tenant Admin', phoneTail(1));
    await client.login('tenant_admin', accounts.tenant_admin);
  }

  if (!(await client.canLogin('tenant_admin_b', accounts.tenant_admin_b))) {
    await client.createUser('super_admin', accounts.tenant_admin_b, 'Automation Tenant Admin B', phoneTail(2));
    await client.login('tenant_admin_b', accounts.tenant_admin_b);
  }

  if (!(await client.canLogin('tenant_user', accounts.tenant_user))) {
    await client.createUser('tenant_admin', accounts.tenant_user, 'Automation Tenant User', phoneTail(3));
    await client.login('tenant_user', accounts.tenant_user);
  }

  if (!(await client.canLogin('readonly_user', accounts.readonly_user))) {
    await client.createUser('tenant_admin', accounts.readonly_user, 'Automation Readonly User', phoneTail(4));
    await client.login('readonly_user', accounts.readonly_user);
  }

  if (!(await client.canLogin('email_change_tenant', accounts.email_change_tenant))) {
    await client.createUser('super_admin', accounts.email_change_tenant, 'Automation Email Change Tenant', phoneTail(5));
    await client.login('email_change_tenant', accounts.email_change_tenant);
  }

  writeEnvFiles(accounts, { apiBaseURL, apiTarget, frontendURL, previewURL });

  process.stdout.write('Local automation accounts are ready.\n');
  process.stdout.write(`PowerShell env file: ${envPs1Path}\n`);
  process.stdout.write(`Dotenv-style env file: ${envLocalPath}\n`);
  process.stdout.write('Load the PowerShell env file before running npm run preflight:api-e2e:\n');
  process.stdout.write(`. ${envPs1Path}\n`);
}

if (require.main === module) {
  main().catch(error => {
    process.stderr.write(`prepare local accounts failed: ${error.message}\n`);
    process.exitCode = 1;
  });
}

module.exports = {
  getAccountOutputPaths,
  configuredIsolationEnvironment
};
