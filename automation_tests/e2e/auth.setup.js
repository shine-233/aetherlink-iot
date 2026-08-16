/**
 * 文件用途：用于在 Playwright 执行前准备Playwright 登录态准备脚本。
 * 核心逻辑：按角色登录目标环境并写入浏览器 storage state，供后续 E2E 套件复用认证上下文。
 * 关键注意事项：生成的认证状态可能包含敏感信息且应保持忽略；登录成功不等同于业务流程通过。
 * 重构建议：若账号模型变化，应把角色配置、失败诊断和状态文件路径继续集中维护。
 */

const axios = require('axios');
const fs = require('fs');
const path = require('path');
const config = require('../lib/network_runtime');

const AUTH_DIR = path.resolve(__dirname, '..', config.e2e.storageStateDir);
const AUTH_ROOT = path.resolve(__dirname, '.auth');
if (AUTH_DIR !== AUTH_ROOT && !AUTH_DIR.startsWith(`${AUTH_ROOT}${path.sep}`)) {
  throw new Error('E2E_AUTH_DIR must remain under automation_tests/e2e/.auth');
}

const ROLE_FILES = {
  super_admin: 'super-admin.json',
  tenant_admin: 'tenant-admin.json',
  tenant_user: 'tenant-user.json',
  tenant_admin_b: 'tenant-admin-b.json',
  readonly_user: 'readonly-user.json',
  email_change_tenant: 'email-change-tenant.json'
};

const REQUIRED_ROLES = new Set([
  'super_admin',
  'tenant_admin',
  'tenant_user',
  'tenant_admin_b',
  'readonly_user',
  'email_change_tenant'
]);

function getAuthStatePath(accountKey) {
  return path.join(AUTH_DIR, ROLE_FILES[accountKey]);
}

function removeAuthState(accountKey) {
  const filePath = getAuthStatePath(accountKey);
  if (fs.existsSync(filePath)) {
    fs.unlinkSync(filePath);
  }
}

function createStorageState(loginToken, userInfo) {
  const origin = new URL(config.frontendURL).origin;
  const expiresIn = Number(loginToken.expires_in || loginToken.expiresIn || 7200);
  const normalizedUserInfo = {
    ...userInfo,
    roles: Array.isArray(userInfo.roles) && userInfo.roles.length ? userInfo.roles : [userInfo.authority].filter(Boolean)
  };

  return {
    cookies: [],
    origins: [
      {
        origin,
        localStorage: [
          { name: 'token', value: JSON.stringify(loginToken.token) },
          { name: 'token_expires_in', value: JSON.stringify(String(Date.now() + expiresIn * 1000)) },
          { name: 'userInfo', value: JSON.stringify(normalizedUserInfo) }
        ]
      }
    ]
  };
}

function writePrivateJSON(filePath, value) {
  const descriptor = fs.openSync(filePath, 'w', 0o600);
  try {
    fs.writeFileSync(descriptor, JSON.stringify(value, null, 2), 'utf8');
  } finally {
    fs.closeSync(descriptor);
  }
  fs.chmodSync(filePath, 0o600);
}

async function loginAndSave(accountKey) {
  const account = config.accounts[accountKey];
  if (!account) {
    throw new Error(`Account ${accountKey} is not configured in config.json`);
  }

  const client = axios.create({
    baseURL: config.baseURL,
    timeout: config.timeout,
    headers: { 'Content-Type': 'application/json' }
  });

  try {
    const loginResp = await client.post('/login', {
      email: account.email,
      password: account.password,
      salt: null
    });
    const loginBody = loginResp.data;
    const loginToken = loginBody && loginBody.code === 200 ? loginBody.data : null;
    if (!loginToken || !loginToken.token) {
      throw new Error(`login response has no token: ${JSON.stringify(loginBody)}`);
    }

    const detailResp = await client.get('/user/detail', {
      headers: { 'x-token': loginToken.token }
    });
    const detailBody = detailResp.data;
    const userInfo = detailBody && detailBody.code === 200 ? detailBody.data : null;
    if (!userInfo) {
      throw new Error(`user detail response has no data: ${JSON.stringify(detailBody)}`);
    }

    const filePath = getAuthStatePath(accountKey);
    writePrivateJSON(filePath, createStorageState(loginToken, userInfo));
    console.log(`  saved auth state: ${accountKey}`);
  } catch (err) {
    throw new Error(`${accountKey}: authentication setup failed`);
  }
}

async function globalSetup() {
  if (!fs.existsSync(AUTH_DIR)) {
    fs.mkdirSync(AUTH_DIR, { recursive: true });
  }

  console.log('\n[globalSetup] preparing auth state...');

  for (const role of Object.keys(ROLE_FILES)) {
    try {
      await loginAndSave(role);
    } catch (err) {
      removeAuthState(role);

      if (REQUIRED_ROLES.has(role)) {
        console.error(`  required auth state failed: ${err.message}`);
        throw err;
      }

      console.log(`  optional auth state skipped: ${err.message}`);
    }
  }

  console.log('[globalSetup] auth state ready\n');
}

module.exports = globalSetup;
