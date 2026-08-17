/**
 * Validate the hosted live-integration boundary without contacting a service.
 *
 * This is deliberately a configuration gate, not an API/device test.  It
 * rejects incomplete or misleading hosted configuration before Playwright or
 * MQTT work starts, and it never prints secret values.
 */

const LOCAL_HOSTS = new Set(['localhost', '127.0.0.1', '::1']);
const REQUIRED_ACCOUNTS = [
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
  'EMAIL_CHANGE_TENANT_PASSWORD'
];

function present(value) {
  return value !== undefined && value !== null && String(value).trim() !== '';
}

function addMissing(missing, item) {
  if (!missing.includes(item)) missing.push(item);
}

function parseCredentialFreeHttpUrl(name, raw, allowlistName, allowlist, missing) {
  let parsed;
  try {
    parsed = new URL(raw);
  } catch {
    addMissing(missing, `${name} must be an absolute URL`);
    return null;
  }

  if (!['http:', 'https:'].includes(parsed.protocol) || parsed.username || parsed.password) {
    addMissing(missing, `${name} must be credential-free HTTP(S)`);
    return null;
  }

  const origin = parsed.origin;
  if (!LOCAL_HOSTS.has(parsed.hostname.toLowerCase()) && !allowlist.has(origin)) {
    addMissing(missing, `${allowlistName} must allow ${origin}`);
  }
  return parsed;
}

function parseMqttAddress(raw, missing) {
  const value = String(raw || '').trim();
  const candidate = /^[a-z][a-z\d+.-]*:\/\//i.test(value) ? value : `mqtt://${value}`;
  let parsed;
  try {
    parsed = new URL(candidate);
  } catch {
    addMissing(missing, 'vars.AETHERLINK_MQTT_ACCESS_ADDRESS must be a valid MQTT host[:port] address');
    return null;
  }

  if (!['mqtt:', 'mqtts:', 'tcp:'].includes(parsed.protocol) || parsed.username || parsed.password) {
    addMissing(missing, 'vars.AETHERLINK_MQTT_ACCESS_ADDRESS must be credential-free MQTT(S)');
    return null;
  }
  if (!parsed.hostname) {
    addMissing(missing, 'vars.AETHERLINK_MQTT_ACCESS_ADDRESS must include a host');
    return null;
  }
  if (parsed.port && (!Number.isInteger(Number(parsed.port)) || Number(parsed.port) < 1 || Number(parsed.port) > 65535)) {
    addMissing(missing, 'vars.AETHERLINK_MQTT_ACCESS_ADDRESS must use a TCP port between 1 and 65535');
  }
  if (LOCAL_HOSTS.has(parsed.hostname.toLowerCase())) {
    addMissing(missing, 'vars.AETHERLINK_MQTT_ACCESS_ADDRESS must not point to the hosted runner loopback');
  }
  return parsed;
}

function collectMissing(env = process.env) {
  const missing = [];
  const mode = String(env.AETHERLINK_DEVICE_VALIDATION_MODE || '').trim().toLowerCase();

  if (String(env.AETHERLINK_RUN_LIVE_TESTS || '').trim() !== 'true') {
    addMissing(missing, 'vars.AETHERLINK_RUN_LIVE_TESTS=true');
  }
  if (mode !== 'generic-emulator') {
    if (mode === 'real-rdi') {
      addMissing(missing, 'real-rdi validation is not implemented by this hosted generic-emulator lane; no physical-device claim is allowed');
    } else {
      addMissing(missing, 'vars.AETHERLINK_DEVICE_VALIDATION_MODE=generic-emulator');
    }
  }

  for (const account of REQUIRED_ACCOUNTS) {
    if (!present(env[account])) addMissing(missing, `secrets.${account}`);
  }

  for (const [name, allowlistName] of [
    ['AETHERLINK_API_BASE_URL', 'AETHERLINK_ALLOWED_ORIGINS'],
    ['AETHERLINK_API_TARGET', 'AETHERLINK_ALLOWED_PROXY_ORIGINS']
  ]) {
    if (!present(env[name])) {
      addMissing(missing, `vars.${name}`);
      continue;
    }
    const allowlist = new Set(String(env[allowlistName] || '')
      .split(',')
      .map(value => value.trim())
      .filter(Boolean));
    parseCredentialFreeHttpUrl(`vars.${name}`, env[name], `vars.${allowlistName}`, allowlist, missing);
  }

  if (!present(env.AETHERLINK_MQTT_ACCESS_ADDRESS)) {
    addMissing(missing, 'vars.AETHERLINK_MQTT_ACCESS_ADDRESS');
  } else {
    parseMqttAddress(env.AETHERLINK_MQTT_ACCESS_ADDRESS, missing);
  }

  if (!present(env.AUTOMATION_READY_CHECK_DEVICE_ID)) {
    addMissing(missing, 'vars.AUTOMATION_READY_CHECK_DEVICE_ID');
  }
  if (String(env.AUTOMATION_READY_CHECK_AUTO_START || '').trim().toLowerCase() !== 'true') {
    addMissing(missing, 'vars.AUTOMATION_READY_CHECK_AUTO_START=true');
  }

  return { missing, mode };
}

function main() {
  const result = collectMissing();
  if (result.missing.length > 0) {
    console.error('Live integration is fail-closed. Configure the following GitHub environment integration values:');
    for (const item of result.missing) console.error(`- ${item}`);
    process.exitCode = 1;
    return;
  }
  console.log('Live integration configuration gate passed for generic-emulator mode.');
}

if (require.main === module) main();

module.exports = {
  LOCAL_HOSTS,
  REQUIRED_ACCOUNTS,
  collectMissing,
  parseMqttAddress,
  parseCredentialFreeHttpUrl
};
