/**
 * Contract tests for the Ready Check fixture's device-selection policy.
 *
 * These tests do not replace the browser evidence: they lock the prerequisite
 * boundary so the real E2E cannot silently fall back to an arbitrary shared
 * offline device when no command emulator was configured.
 */

const fs = require('fs');
const path = require('path');
const { expect } = require('chai');
const { selectReadyCheckDevice, selectRdiDevice } = require('../lib/seed_data');
const runner = require('../run_tests');

describe('Ready Check fixture device selection', function() {
  const online = {
    id: 'online-device-id',
    device_number: 'readycheck-online',
    is_online: 1,
    device_status: 1
  };
  const offline = {
    id: 'offline-device-id',
    device_number: 'offline-device',
    is_online: 0,
    device_status: 0
  };

  it('requires an explicit emulator device instead of selecting a shared row', function() {
    const result = selectReadyCheckDevice([online, offline], '');

    expect(result).to.include({ blocked: true });
    expect(result.reason).to.match(/AUTOMATION_READY_CHECK_DEVICE_ID/);
    expect(result.reason).to.match(/shared-device fallback is disabled/);
  });

  it('accepts only the explicitly requested online non-RDI device', function() {
    const result = selectReadyCheckDevice([offline, online], online.id);

    expect(result.blocked).not.to.equal(true);
    expect(result.invalid).not.to.equal(true);
    expect(result.device).to.include({ id: online.id, row: online, created: false });
  });

  it('blocks an explicitly requested offline device', function() {
    const result = selectReadyCheckDevice([offline, online], offline.id);

    expect(result).to.include({ blocked: true });
    expect(result.reason).to.match(/offline/);
  });

  it('blocks a configured device id that is not visible to the account', function() {
    const result = selectReadyCheckDevice([online], 'missing-device-id');

    expect(result).to.include({ blocked: true });
    expect(result.reason).to.match(/not visible/);
  });

  it('rejects an RDI device because it has no generic Ready Check operation tabs', function() {
    const result = selectReadyCheckDevice([{
      ...online,
      id: 'rdi-device-id',
      device_number: '123456789012'
    }], 'rdi-device-id');

    expect(result).to.include({ invalid: true });
    expect(result.reason).to.match(/RDI device/);
  });

  it('selects an existing RDI device for RDI-only shared fixtures', function() {
    // In the explicit synthetic lane, selectRdiDevice must remain fail-closed
    // and only accept the configured synthetic PID.  The contract fixture
    // therefore follows that PID instead of silently using an arbitrary
    // twelve-character device number.
    const syntheticPid = String(
      process.env.AETHERLINK_RDI_FIXTURE_PID ||
      process.env.SYNTHETIC_RDI_PID ||
      '123456789012'
    ).trim().toUpperCase();
    const rdiDevice = { ...online, id: 'existing-rdi-device', device_number: syntheticPid };

    expect(selectRdiDevice([online, rdiDevice])).to.equal(rdiDevice);
  });

  it('classifies an all-skipped runtime-external E2E module as partial-skip', function() {
    const summary = runner.summarizePlaywrightResult(
      {
        code: 0,
        stdout: [
          'integration-blocked: Ready Check command emulator/device is unavailable',
          'integration-blocked-meta: {"reason":"Ready Check command emulator/device is unavailable","category":"runtime-external","seedable":false}'
        ].join('\n'),
        stderr: '',
        reportJson: null
      },
      {
        stats: { expected: 0, skipped: 1, unexpected: 0 },
        errors: []
      }
    );

    expect(summary).to.include({
      passed: true,
      outcome: 'partial-skip',
      skipped: 1
    });
    expect(summary.blockedReasons).to.deep.include({
      reason: 'Ready Check command emulator/device is unavailable',
      category: 'runtime-external',
      seedable: false
    });
  });

  it('keeps the predeploy runner pointed at the generic command emulator', function() {
    const script = fs.readFileSync(
      path.join(__dirname, '..', 'scripts', 'predeploy_full_retest.ps1'),
      'utf8'
    ).replace(/\r\n/g, '\n');

    expect(script).to.match(
      /\$readyCheckEmulatorPath\s*=\s*Resolve-RunPath\s+\(Join-Path\s+\$ProjectRoot\s+'backend\\cmd\\aetherlink-device-autotest\\_localrun\\ready-check-command-emulator\.exe'\)/
    );
    expect(script).not.to.include(
      '$env:AUTOMATION_READY_CHECK_EMULATOR_BIN = $emulatorPath'
    );
    expect(script).not.to.match(/\bR5_(?:TEST_EXIT_CODE|FIXTURE_PID|RUN_DIR|RUN_ERROR)\b/);
    expect(script).to.include('PREDEPLOY_TEST_EXIT_CODE');
    expect(script).to.include('$cleanupExecutablePaths');
    expect(script).to.include('$cleanupExecutableNames');
    expect(script).to.match(/Accept both dotenv account sources/);
    expect(script).to.match(/\(\?:\\\$env:\)\?/);
  });

  it('provisions a missing explicit database without dropping an existing one', function() {
    const script = fs.readFileSync(
      path.join(__dirname, '..', 'scripts', 'predeploy_full_retest.ps1'),
      'utf8'
    ).replace(/\r\n/g, '\n');

    expect(script).to.match(/function Ensure-IsolatedDatabase/);
    expect(script).to.match(/CREATE DATABASE/);
    expect(script).to.match(/Ensure-IsolatedDatabase[\s`]+-Name[\s`]+\$DatabaseName/);
    expect(script).not.to.match(/DROP DATABASE/);
  });
});
