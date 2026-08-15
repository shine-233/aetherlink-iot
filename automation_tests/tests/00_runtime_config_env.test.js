/**
 * 文件用途：用于验证运行时配置环境变量测试。
 * 核心逻辑：以快速 Node 测试保护覆盖率契约、运行配置、oracle 或预检逻辑的结构和边界行为。
 * 关键注意事项：这类测试证明自动化框架契约，不等同于真实后端或浏览器业务流程通过。
 * 重构建议：当契约 schema 或分类规则变化时，应同步更新 fixture 和负向用例，避免只改快照。
 */

const { expect } = require('chai');
const path = require('path');
const { getAccountOutputPaths } = require('../scripts/prepare_local_accounts');
const mqttRuntime = require('../lib/mqtt_runtime');

const CONFIG_ENV_KEYS = [
  'API_BASE_URL',
  'HEALTH_URL',
  'FRONTEND_URL',
  'AUTOMATION_REPORT_DIR',
  'E2E_AUTH_DIR',
  'SUPER_ADMIN_EMAIL',
  'SUPER_ADMIN_PASSWORD',
  'TENANT_ADMIN_EMAIL',
  'TENANT_ADMIN_PASSWORD',
  'TENANT_ADMIN_B_EMAIL',
  'TENANT_ADMIN_B_PASSWORD',
  'TENANT_USER_EMAIL',
  'TENANT_USER_PASSWORD',
  'READONLY_USER_EMAIL',
  'READONLY_USER_PASSWORD',
  'EMAIL_CHANGE_TENANT_EMAIL',
  'EMAIL_CHANGE_TENANT_PASSWORD',
  'AETHERLINK_RUNTIME_CONFIG_SKIP_ENV_FILE'
];

function loadRuntimeConfig(env = {}) {
  const previous = {};
  const keys = new Set([...CONFIG_ENV_KEYS, ...Object.keys(env)]);

  for (const key of keys) {
    previous[key] = process.env[key];
    if (Object.prototype.hasOwnProperty.call(env, key)) {
      process.env[key] = env[key];
    } else {
      delete process.env[key];
    }
  }

  // The shared .env.local belongs to the prepared local runtime, not to this
  // unit-level environment override contract. Keep each require isolated from
  // that file while still exercising the real runtime_config module.
  process.env.AETHERLINK_RUNTIME_CONFIG_SKIP_ENV_FILE = '1';

  const modulePath = require.resolve('../lib/runtime_config');
  delete require.cache[modulePath];

  try {
    return require('../lib/runtime_config');
  } finally {
    delete require.cache[modulePath];
    for (const key of keys) {
      if (previous[key] === undefined) {
        delete process.env[key];
      } else {
        process.env[key] = previous[key];
      }
    }
  }
}

describe('runtime config env overrides', function() {
  it('keeps config.json defaults when env overrides are absent', function() {
    const config = loadRuntimeConfig();

    expect(config.frontendURL).to.equal('http://127.0.0.1:5002');
    expect(config.baseURL).to.equal('http://127.0.0.1:9999/api/v1');
    expect(config.healthURL).to.equal('http://127.0.0.1:9999/health');
  });

  it('allows preview E2E to override frontendURL without mutating config.json', function() {
    const config = loadRuntimeConfig({
      FRONTEND_URL: 'http://127.0.0.1:9725'
    });

    expect(config.frontendURL).to.equal('http://127.0.0.1:9725');
    expect(config.baseURL).to.equal('http://127.0.0.1:9999/api/v1');
    expect(config.healthURL).to.equal('http://127.0.0.1:9999/health');
  });

  it('allows concurrent E2E runs to isolate auth state and temporary reports', function() {
    const config = loadRuntimeConfig({
      AUTOMATION_REPORT_DIR: 'C:/temp/aetherlink-instance-b/reports',
      E2E_AUTH_DIR: 'C:/temp/aetherlink-instance-b/auth'
    });

    expect(config.report.outputDir).to.equal('C:/temp/aetherlink-instance-b/reports');
    expect(config.e2e.storageStateDir).to.equal('C:/temp/aetherlink-instance-b/auth');
  });

  it('keeps generated account credentials out of the shared dotenv file for an isolated instance', function() {
    const defaultPaths = getAccountOutputPaths({});
    const isolatedPaths = getAccountOutputPaths({
      AUTOMATION_ACCOUNT_DIR: 'C:/temp/aetherlink-instance-b/accounts'
    });

    expect(defaultPaths.envLocalPath).to.equal(path.join(__dirname, '..', '.env.local'));
    expect(isolatedPaths.localDir).to.equal(path.resolve('C:/temp/aetherlink-instance-b/accounts'));
    expect(isolatedPaths.envLocalPath).to.equal(
      path.join(path.resolve('C:/temp/aetherlink-instance-b/accounts'), '.env.local')
    );
    expect(isolatedPaths.envLocalPath).not.to.equal(defaultPaths.envLocalPath);
  });

  it('allows local credentials to be provided by env without committing secrets', function() {
    const config = loadRuntimeConfig({
      SUPER_ADMIN_EMAIL: 'super.local@example.com',
      SUPER_ADMIN_PASSWORD: 'SuperLocalPassword',
      TENANT_ADMIN_EMAIL: 'tenant.local@example.com',
      TENANT_ADMIN_PASSWORD: 'TenantLocalPassword',
      TENANT_ADMIN_B_EMAIL: 'tenant-b.local@example.com',
      TENANT_ADMIN_B_PASSWORD: 'TenantBLocalPassword',
      TENANT_USER_EMAIL: 'tenant-user.local@example.com',
      TENANT_USER_PASSWORD: 'TenantUserLocalPassword',
      READONLY_USER_EMAIL: 'readonly.local@example.com',
      READONLY_USER_PASSWORD: 'ReadonlyLocalPassword',
      EMAIL_CHANGE_TENANT_EMAIL: 'email-change.local@example.com',
      EMAIL_CHANGE_TENANT_PASSWORD: 'EmailChangeLocalPassword'
    });

    expect(config.accounts.super_admin.email).to.equal('super.local@example.com');
    expect(config.accounts.super_admin.password).to.equal('SuperLocalPassword');
    expect(config.accounts.tenant_admin.email).to.equal('tenant.local@example.com');
    expect(config.accounts.tenant_admin.password).to.equal('TenantLocalPassword');
    expect(config.accounts.tenant_admin_b.email).to.equal('tenant-b.local@example.com');
    expect(config.accounts.tenant_admin_b.password).to.equal('TenantBLocalPassword');
    expect(config.accounts.tenant_user.email).to.equal('tenant-user.local@example.com');
    expect(config.accounts.tenant_user.password).to.equal('TenantUserLocalPassword');
    expect(config.accounts.readonly_user.email).to.equal('readonly.local@example.com');
    expect(config.accounts.readonly_user.password).to.equal('ReadonlyLocalPassword');
    expect(config.accounts.email_change_tenant.email).to.equal('email-change.local@example.com');
    expect(config.accounts.email_change_tenant.password).to.equal('EmailChangeLocalPassword');
  });

  it('uses the standard MQTT endpoint unless an explicit local-dev profile is selected', function() {
    expect(mqttRuntime.getMqttEndpoint({})).to.deep.include({
      server: '127.0.0.1',
      port: 1883,
      profile: 'standard'
    });
    expect(mqttRuntime.getMqttEndpoint({ AUTOMATION_MQTT_PROFILE: 'localdev-status' })).to.deep.include({
      server: '127.0.0.1',
      port: 1885,
      profile: 'localdev-status'
    });
  });

  it('lets an explicit MQTT port override the profile and validates it', function() {
    expect(mqttRuntime.mqttEndpointDescription({
      AUTOMATION_MQTT_PROFILE: 'localdev-status',
      AUTOMATION_MQTT_SERVER: 'mqtt.example.test',
      AUTOMATION_MQTT_PORT: '1887'
    })).to.equal('mqtt.example.test:1887');
    expect(() => mqttRuntime.getMqttEndpoint({ AUTOMATION_MQTT_PORT: 'not-a-port' }))
      .to.throw(/AUTOMATION_MQTT_PORT/);
  });

  it('follows the backend MQTT access address when no automation port override is set', function() {
    expect(mqttRuntime.getMqttEndpoint({
      GOTP_MQTT_ACCESS_ADDRESS: '127.0.0.1:1885'
    })).to.deep.include({
      server: '127.0.0.1',
      port: 1885
    });

    expect(mqttRuntime.getMqttEndpoint({
      AUTOMATION_MQTT_PROFILE: 'localdev-status',
      AETHERLINK_MQTT_ACCESS_ADDRESS: 'mqtt.example.test:1883'
    })).to.deep.include({
      server: 'mqtt.example.test',
      port: 1883
    });

    expect(mqttRuntime.getMqttEndpoint({
      GOTP_MQTT_ACCESS_ADDRESS: 'mqtt.example.test:1883',
      AUTOMATION_MQTT_SERVER: 'override.example.test',
      AUTOMATION_MQTT_PORT: '1887'
    })).to.deep.include({
      server: 'override.example.test',
      port: 1887
    });
  });

  it('publishes the release account gate used by API/E2E preflight', function() {
    const config = loadRuntimeConfig();

    expect(config.releaseRequiredAccounts).to.deep.equal([
      'super_admin',
      'tenant_admin',
      'tenant_admin_b',
      'tenant_user',
      'readonly_user',
      'email_change_tenant'
    ]);

    expect(config.accountEnvOverrides.tenant_admin_b).to.deep.equal([
      'TENANT_ADMIN_B_EMAIL',
      'TENANT_ADMIN_B_PASSWORD'
    ]);
  });
});
