/**
 * Network-only runtime configuration for automation clients.
 *
 * This module deliberately contains no filesystem reads. Network destinations
 * and credentials are supplied by the process environment (with loopback
 * defaults for local development), so an arbitrary value in config.json cannot
 * silently redirect test traffic or be sent to an unexpected host.
 */

const LOCAL_TARGET_HOSTS = new Set(['localhost', '127.0.0.1', '::1']);

function envOrDefault(name, fallback) {
  return process.env[name] || fallback;
}

function configuredOriginAllowlist() {
  return new Set(
    String(process.env.AETHERLINK_ALLOWED_ORIGINS || '')
      .split(',')
      .map(value => value.trim())
      .filter(Boolean)
  );
}

function validateTrustedURL(rawValue, name) {
  const value = String(rawValue || '').trim();
  let parsed;
  try {
    parsed = new URL(value);
  } catch {
    throw new Error(`${name} must be an absolute HTTP(S) URL`);
  }
  if (!['http:', 'https:'].includes(parsed.protocol) || !parsed.hostname) {
    throw new Error(`${name} must use http or https`);
  }
  if (parsed.username || parsed.password) {
    throw new Error(`${name} must not contain embedded credentials`);
  }

  const isLocal = LOCAL_TARGET_HOSTS.has(parsed.hostname.toLowerCase());
  const allowExternal = process.env.AETHERLINK_ALLOW_EXTERNAL_TARGETS === '1';
  if (!isLocal && !allowExternal && !configuredOriginAllowlist().has(parsed.origin)) {
    throw new Error(
      `${name} points to an external origin; set AETHERLINK_ALLOWED_ORIGINS or ` +
      'AETHERLINK_ALLOW_EXTERNAL_TARGETS=1 explicitly'
    );
  }
  return parsed.toString().replace(/\/$/, '');
}

const accountEnv = {
  super_admin: ['SUPER_ADMIN_EMAIL', 'SUPER_ADMIN_PASSWORD', 'SYS_ADMIN'],
  tenant_admin: ['TENANT_ADMIN_EMAIL', 'TENANT_ADMIN_PASSWORD', 'TENANT_ADMIN'],
  tenant_user: ['TENANT_USER_EMAIL', 'TENANT_USER_PASSWORD', 'TENANT_USER'],
  tenant_admin_b: ['TENANT_ADMIN_B_EMAIL', 'TENANT_ADMIN_B_PASSWORD', 'TENANT_ADMIN'],
  readonly_user: ['READONLY_USER_EMAIL', 'READONLY_USER_PASSWORD', 'TENANT_USER'],
  email_change_tenant: ['EMAIL_CHANGE_TENANT_EMAIL', 'EMAIL_CHANGE_TENANT_PASSWORD', 'TENANT_ADMIN']
};

const accounts = Object.fromEntries(
  Object.entries(accountEnv).map(([key, [emailName, passwordName, role]]) => [key, {
    email: String(process.env[emailName] || ''),
    password: String(process.env[passwordName] || ''),
    role,
    description: 'Credentials must be supplied through the process environment.'
  }])
);

module.exports = {
  baseURL: validateTrustedURL(envOrDefault('API_BASE_URL', 'http://127.0.0.1:9999/api/v1'), 'API_BASE_URL'),
  healthURL: validateTrustedURL(envOrDefault('HEALTH_URL', 'http://127.0.0.1:9999/health'), 'HEALTH_URL'),
  frontendURL: validateTrustedURL(envOrDefault('FRONTEND_URL', 'http://127.0.0.1:5002'), 'FRONTEND_URL'),
  timeout: Number(envOrDefault('API_TIMEOUT_MS', '15000')),
  e2e: {
    storageStateDir: envOrDefault('E2E_AUTH_DIR', './e2e/.auth')
  },
  accounts,
  accountEnvOverrides: Object.fromEntries(
    Object.entries(accountEnv).map(([key, [emailName, passwordName]]) => [key, [emailName, passwordName]])
  ),
  releaseRequiredAccounts: Object.keys(accountEnv),
  validateTrustedURL
};
