/**
 * Prepare an explicitly synthetic RDI pre-registration row for isolated tests.
 *
 * This script never turns an ordinary device into an RDI device.  It creates
 * the inactive pre-registration state that the real activation endpoint
 * expects, and the caller must still POST /rdi/devices/activate afterwards.
 *
 * Writes are intentionally fail-closed.  A caller must opt in and target the
 * known local PostgreSQL port/database.  Credentials are read from the
 * process environment and are never printed or written to an artifact.
 */

const crypto = require('crypto');
const fs = require('fs');
const { spawn } = require('child_process');

const { syntheticRdiFixtureVoucher } = require('../lib/synthetic_rdi_contract');

const ALLOWED_HOST = '127.0.0.1';
const DEFAULT_ALLOWED_PORT = '55432';
const DEFAULT_DATABASE = 'aetherlink_iot_local';
const PROVENANCE = 'synthetic-rdi';
const DEFAULT_PID = 'SYNTHRDI0001';

function env(name, fallback = '') {
  return String(process.env[name] || fallback).trim();
}

function getMode() {
  const modes = process.argv.slice(2).filter(arg => ['--seed', '--cleanup', '--status'].includes(arg));
  if (modes.length !== 1) {
    throw new Error('Use exactly one of --seed, --cleanup, or --status');
  }
  return modes[0].slice(2);
}

function requireWriteOptIn(mode) {
  if (mode === 'status') return;
  if (env('AETHERLINK_SYNTHETIC_RDI_ALLOW') !== '1' && !process.argv.includes('--confirm')) {
    throw new Error(
      'Refusing synthetic-rdi write; set AETHERLINK_SYNTHETIC_RDI_ALLOW=1 or pass --confirm explicitly'
    );
  }
}

function getDatabaseOptions() {
  const allowedPort = env('AETHERLINK_SYNTHETIC_RDI_ALLOWED_PORT', DEFAULT_ALLOWED_PORT);
  if (!/^\d{1,5}$/.test(allowedPort) || Number(allowedPort) < 1 || Number(allowedPort) > 65535) {
    throw new Error(`AETHERLINK_SYNTHETIC_RDI_ALLOWED_PORT must be a TCP port 1-65535: ${allowedPort}`);
  }
  const options = {
    host: env('AETHERLINK_DB_HOST', env('GOTP_DB_PSQL_HOST', '')),
    port: env('AETHERLINK_DB_PORT', env('GOTP_DB_PSQL_PORT', '')),
    database: env('AETHERLINK_DB_NAME', env('GOTP_DB_PSQL_DBNAME', '')),
    user: env('AETHERLINK_DB_USER', env('GOTP_DB_PSQL_USERNAME', 'postgres')),
    password: process.env.PGPASSWORD || process.env.AETHERLINK_DB_PASSWORD || process.env.GOTP_DB_PSQL_PASSWORD || ''
  };

  if (options.host !== ALLOWED_HOST) {
    throw new Error(`Refusing synthetic-rdi target host ${options.host || '<unset>'}; only ${ALLOWED_HOST} is allowed`);
  }
  if (options.port !== allowedPort) {
    throw new Error(
      `Refusing synthetic-rdi target port ${options.port || '<unset>'}; only explicitly allowed local port ${allowedPort} is allowed`
    );
  }
  const explicitDatabaseAllowlist = env('AETHERLINK_SYNTHETIC_RDI_ALLOWED_DATABASES')
    .split(',')
    .map(value => value.trim())
    .filter(Boolean);
  const explicitlyAllowed = explicitDatabaseAllowlist.includes(options.database);
  if (!options.database || !explicitlyAllowed) {
    throw new Error(
      `Refusing synthetic-rdi target database ${options.database || '<unset>'}; ` +
      'set AETHERLINK_SYNTHETIC_RDI_ALLOWED_DATABASES to an exact local test database name'
    );
  }
  if (!options.user) throw new Error('AETHERLINK_DB_USER/GOTP_DB_PSQL_USERNAME is required');
  return options;
}

function findPsql() {
  const candidates = [
    env('PSQL_PATH'),
    'C:\\Program Files\\PostgreSQL\\17\\bin\\psql.exe',
    'psql'
  ].filter(Boolean);
  const existing = candidates.find(candidate => candidate === 'psql' || fs.existsSync(candidate));
  if (!existing) throw new Error('psql.exe was not found; set PSQL_PATH to the local PostgreSQL client');
  return existing;
}

function spawnPsql(sql, options) {
  const executable = findPsql();
  return new Promise((resolve, reject) => {
    const child = spawn(executable, [
      '-X',
      '-v', 'ON_ERROR_STOP=1',
      '-h', options.host,
      '-p', options.port,
      '-U', options.user,
      '-d', options.database,
      '-At',
      '-q',
      // The fixture payload is controlled by this script and contains no pipe
      // characters.  A visible delimiter is more robust than a tab across
      // Windows PowerShell -> Node -> psql argument boundaries.
      '-F', '|',
      '-c', sql
    ], {
      env: { ...process.env, PGPASSWORD: options.password },
      windowsHide: true,
      stdio: ['ignore', 'pipe', 'pipe']
    });

    let stdout = '';
    let stderr = '';
    child.stdout.on('data', chunk => { stdout += chunk.toString(); });
    child.stderr.on('data', chunk => { stderr += chunk.toString(); });
    child.on('error', error => reject(new Error(`psql could not start: ${error.message}`)));
    child.on('close', code => {
      if (code !== 0) {
        const detail = stderr.trim().replace(/password\s*=\s*[^\s]+/ig, 'password=<redacted>');
        reject(new Error(`psql exited with code ${code}${detail ? `: ${detail}` : ''}`));
        return;
      }
      resolve(stdout.trim());
    });
  });
}

function sqlLiteral(value) {
  return `'${String(value).replace(/'/g, "''")}'`;
}

function jsonLiteral(value) {
  const encoded = Buffer.from(JSON.stringify(value), 'utf8').toString('base64');
  return `convert_from(decode(${sqlLiteral(encoded)}, 'base64'), 'UTF8')::json`;
}

function parseRow(output) {
  if (!output) return null;
  // psql can append transaction/command status lines when the caller wraps a
  // RETURNING query in BEGIN/COMMIT. Select the first row-shaped line instead
  // of letting those status lines contaminate the JSON field below.
  const rowLine = String(output)
    .split(/\r?\n/)
    .map(line => line.trim())
    .find(line => line.split('|').length >= 7);
  if (!rowLine) throw new Error('Unexpected synthetic-rdi fixture query shape');
  const fields = rowLine.split('|');
  if (fields.length < 7) throw new Error('Unexpected synthetic-rdi fixture query shape');
  let additionalInfo = {};
  try {
    additionalInfo = JSON.parse(fields[6] || '{}');
  } catch (_) {
    throw new Error('Synthetic-rdi fixture row contains invalid additional_info JSON');
  }
  return {
    id: fields[0],
    deviceNumber: fields[1],
    tenantId: fields[2],
    isEnabled: fields[3],
    activateFlag: fields[4],
    isOnline: fields[5],
    additionalInfo
  };
}

function pidFromEnvironment() {
  const pid = env('SYNTHETIC_RDI_PID', DEFAULT_PID).toUpperCase();
  if (!/^[A-Z0-9]{12}$/.test(pid)) {
    throw new Error(`SYNTHETIC_RDI_PID must be exactly 12 alphanumeric characters: ${pid || '<unset>'}`);
  }
  return pid;
}

function fixtureQuery(pid) {
  return [
    'SELECT id, device_number, tenant_id, is_enabled, activate_flag, is_online, additional_info::text',
    'FROM public.devices',
    `WHERE device_number = ${sqlLiteral(pid)}`,
    'LIMIT 1;'
  ].join(' ');
}

function assertOwnedSyntheticRow(row, pid) {
  if (!row) return;
  const info = row.additionalInfo || {};
  if (info.fixture_provenance !== PROVENANCE) {
    throw new Error(`Refusing to modify existing device_number ${pid}; it is not marked ${PROVENANCE}`);
  }
}

function hardwareIdentityForPid(pid) {
  return {
    kind: 'synthetic',
    serial: `SYNTH-HW-${pid}`,
    provenance: PROVENANCE
  };
}

async function ensureSyntheticHardwareIdentity(row, pid, options) {
  const expected = hardwareIdentityForPid(pid);
  const current = row && row.additionalInfo && row.additionalInfo.hardware_identity;
  if (
    current &&
    current.kind === expected.kind &&
    current.serial === expected.serial &&
    current.provenance === expected.provenance
  ) {
    return row;
  }

  const sql = [
    'UPDATE public.devices',
    `SET additional_info = (additional_info::jsonb || ${jsonLiteral({ hardware_identity: expected })}::jsonb)::json`,
    `WHERE id = ${sqlLiteral(row.id)}`,
    `AND device_number = ${sqlLiteral(pid)}`,
    `AND additional_info->>'fixture_provenance' = ${sqlLiteral(PROVENANCE)}`,
    'RETURNING id, device_number, tenant_id, is_enabled, activate_flag, is_online, additional_info::text;'
  ].join(' ');
  const updated = parseRow(await spawnPsql(`BEGIN; ${sql} COMMIT;`, options));
  if (!updated || !updated.additionalInfo.hardware_identity) {
    throw new Error(`Synthetic-rdi hardware identity update did not persist for ${pid}`);
  }
  return updated;
}

function fixtureInfo(pid, fixtureId) {
  const config = {
    data_collection_interval: 60,
    alarm_sensor_1_enabled: true,
    alarm_sensor_2_enabled: true,
    sensor_1_upper: 80,
    sensor_1_lower: -10,
    sensor_2_upper: 80,
    sensor_2_lower: -10,
    sensor_1_duration: 30,
    sensor_2_duration: 30,
    switch_1_alarm_mode: 'powered_on',
    switch_2_alarm_mode: 'powered_off',
    switch_1_alarm_duration: 30,
    switch_2_alarm_duration: 30,
    dry_contact_alarm_level: 'high',
    dry_contact_normal_level: 'low',
    dry_contact_alarm_delay: 10,
    dry_contact_normal_delay: 5,
    notification_enabled: true,
    notification_temperature_alarm: true,
    notification_switch_alarm: true,
    notification_warranty_alarm: false,
    sensor_alarm_emails: 'sensor@test.invalid',
    switch_alarm_emails: 'switch@test.invalid',
    warranty_alarm_emails: '',
    sensor_1_alarm_emails: 'sensor1@test.invalid',
    sensor_2_alarm_emails: 'sensor2@test.invalid',
    switch_1_alarm_emails: 'switch1@test.invalid',
    switch_2_alarm_emails: 'switch2@test.invalid',
    field_setting: {}
  };
  const systemInfo = {
    installation_location: 'synthetic-rdi-lab',
    address: '127.0.0.1 local test fixture',
    installation_date: '2026-08-10',
    installer_company: 'AetherLink synthetic fixture',
    installer_contact: 'automation',
    installer_name: 'synthetic-rdi',
    installer_phone: '+86 13900000000',
    installer_email: 'synthetic-rdi@test.invalid',
    controller_serial_number: `SYNTH-${pid}`,
    maintenance_technician: 'automation',
    customer_name: 'synthetic-rdi',
    contact_email: 'synthetic-rdi@test.invalid',
    contact_phone: '+86 13800000000',
    warranty_status: 'active',
    extra_fields: { fixture_id: fixtureId, provenance: PROVENANCE }
  };
  return {
    fixture_provenance: PROVENANCE,
    fixture_id: fixtureId,
    fixture_pid: pid,
    fixture_created_at: new Date().toISOString(),
    connection_type: PROVENANCE,
    hardware_identity: {
      kind: 'synthetic',
      serial: `SYNTH-HW-${pid}`,
      provenance: PROVENANCE
    },
    rdi_config: config,
    rdi_system_info: systemInfo,
    rdi_share_tokens: [],
    rdi_share_recipients: []
  };
}

async function seed(options, pid) {
  const existing = parseRow(await spawnPsql(fixtureQuery(pid), options));
  assertOwnedSyntheticRow(existing, pid);
  if (existing) {
    const normalized = await ensureSyntheticHardwareIdentity(existing, pid, options);
    const result = {
      mode: 'synthetic-rdi',
      action: 'existing',
      id: normalized.id,
      pid,
      activate_flag: normalized.activateFlag,
      is_enabled: normalized.isEnabled,
      is_online: Number(normalized.isOnline) === 1,
      fixture_id: normalized.additionalInfo.fixture_id || null,
      hardware_identity: normalized.additionalInfo.hardware_identity
    };
    process.stdout.write(`${JSON.stringify(result)}\n`);
    return;
  }

  const id = crypto.randomUUID();
  const fixtureId = `${pid}-${crypto.randomBytes(4).toString('hex')}`;
  const now = new Date().toISOString();
  const additionalInfo = fixtureInfo(pid, fixtureId);
  // 凭证哈希 Phase 2a：voucher 走 lib/synthetic_rdi_contract.js 的确定性契约，
  // 运行器（run_synthetic_rdi_protocol_validation.js）按同一契约重建，不再读详情接口。
  const voucher = JSON.stringify(syntheticRdiFixtureVoucher(fixtureId));
  const voucherHash = crypto.createHmac('sha256', 'aetherlink:voucher-cache:v1').update(voucher).digest('hex');
  const sql = [
    'INSERT INTO public.devices (',
    'id, name, voucher, tenant_id, is_enabled, owner_user_id, activate_flag,',
    'created_at, update_at, device_number, additional_info, protocol_config,',
    'current_version, is_online, access_way, description, voucher_hash',
    ') VALUES (',
    sqlLiteral(id),
    ',', sqlLiteral(`Synthetic RDI ${pid}`),
    ',', sqlLiteral(voucher),
    ",'', 'disabled', NULL, 'inactive',",
    sqlLiteral(now), ',', sqlLiteral(now), ',', sqlLiteral(pid),
    ',', jsonLiteral(additionalInfo),
    ',', jsonLiteral({ fixture_provenance: PROVENANCE, protocol: PROVENANCE }),
    ',', sqlLiteral('synthetic-rdi-1.0.0'),
    ',0,', sqlLiteral('A'), ',', sqlLiteral('Synthetic RDI fixture; never real hardware'),
    ',', sqlLiteral(voucherHash),
    ') RETURNING id, device_number, tenant_id, is_enabled, activate_flag, is_online, additional_info::text;'
  ].join(' ');
  const row = parseRow(await spawnPsql(`BEGIN; ${sql} COMMIT;`, options));
  if (!row) throw new Error('Synthetic-rdi fixture INSERT returned no row');
  process.stdout.write(`${JSON.stringify({
    mode: 'synthetic-rdi',
    action: 'created',
    id: row.id,
    pid,
    activate_flag: row.activateFlag,
    is_enabled: row.isEnabled,
    is_online: Number(row.isOnline) === 1,
    fixture_id: row.additionalInfo.fixture_id,
    hardware_identity: row.additionalInfo.hardware_identity
  })}\n`);
}

async function status(options, pid) {
  const row = parseRow(await spawnPsql(fixtureQuery(pid), options));
  if (!row) {
    process.stdout.write(`${JSON.stringify({ mode: 'synthetic-rdi', action: 'absent', pid })}\n`);
    return;
  }
  assertOwnedSyntheticRow(row, pid);
  process.stdout.write(`${JSON.stringify({
    mode: 'synthetic-rdi',
    action: 'status',
    id: row.id,
    pid,
    tenant_id: row.tenantId,
    activate_flag: row.activateFlag,
    is_enabled: row.isEnabled,
    is_online: Number(row.isOnline) === 1,
    fixture_id: row.additionalInfo.fixture_id || null
  })}\n`);
}

async function cleanup(options, pid) {
  const row = parseRow(await spawnPsql(fixtureQuery(pid), options));
  if (!row) {
    process.stdout.write(`${JSON.stringify({ mode: 'synthetic-rdi', action: 'absent', pid })}\n`);
    return;
  }
  assertOwnedSyntheticRow(row, pid);
  const sql = [
    'DELETE FROM public.devices',
    `WHERE id = ${sqlLiteral(row.id)}`,
    `AND device_number = ${sqlLiteral(pid)}`,
    `AND additional_info->>'fixture_provenance' = ${sqlLiteral(PROVENANCE)}`,
    'RETURNING id;'
  ].join(' ');
  const deleted = await spawnPsql(`BEGIN; ${sql} COMMIT;`, options);
  if (!deleted) throw new Error('Synthetic-rdi cleanup deleted no row');
  process.stdout.write(`${JSON.stringify({ mode: 'synthetic-rdi', action: 'deleted', id: row.id, pid })}\n`);
}

async function main() {
  const mode = getMode();
  requireWriteOptIn(mode);
  const options = getDatabaseOptions();
  const pid = pidFromEnvironment();
  if (mode === 'seed') return seed(options, pid);
  if (mode === 'cleanup') return cleanup(options, pid);
  return status(options, pid);
}

if (require.main === module) {
  main().catch(error => {
    process.stderr.write(`synthetic-rdi fixture failed: ${error.message}\n`);
    process.exitCode = 1;
  });
}

module.exports = {
  getDatabaseOptions
};
