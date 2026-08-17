const { expect } = require('chai');
const {
  REQUIRED_ACCOUNTS,
  collectMissing
} = require('../scripts/validate_live_integration_config');

function validEnvironment(overrides = {}) {
  const env = {
    AETHERLINK_RUN_LIVE_TESTS: 'true',
    AETHERLINK_DEVICE_VALIDATION_MODE: 'generic-emulator',
    AETHERLINK_API_BASE_URL: 'https://api.integration.example.test/api/v1',
    AETHERLINK_API_TARGET: 'https://api.integration.example.test',
    AETHERLINK_ALLOWED_ORIGINS: 'https://api.integration.example.test',
    AETHERLINK_ALLOWED_PROXY_ORIGINS: 'https://api.integration.example.test',
    AETHERLINK_MQTT_ACCESS_ADDRESS: 'mqtt.integration.example.test:1883',
    AUTOMATION_READY_CHECK_DEVICE_ID: 'ready-check-device-id',
    AUTOMATION_READY_CHECK_AUTO_START: 'true'
  };
  for (const account of REQUIRED_ACCOUNTS) env[account] = `configured-${account.toLowerCase()}`;
  return { ...env, ...overrides };
}

describe('hosted live integration configuration gate', function () {
  it('fails closed when the environment is empty', function () {
    const result = collectMissing({});

    expect(result.missing).to.include('vars.AETHERLINK_RUN_LIVE_TESTS=true');
    expect(result.missing).to.include('vars.AETHERLINK_DEVICE_VALIDATION_MODE=generic-emulator');
    expect(result.missing).to.include('vars.AETHERLINK_MQTT_ACCESS_ADDRESS');
    expect(result.missing).to.include('vars.AUTOMATION_READY_CHECK_DEVICE_ID');
  });

  it('accepts a complete credential-free external generic-emulator configuration', function () {
    expect(collectMissing(validEnvironment()).missing).to.deep.equal([]);
  });

  it('rejects hosted-runner loopback MQTT instead of silently using 127.0.0.1', function () {
    const result = collectMissing(validEnvironment({
      AETHERLINK_MQTT_ACCESS_ADDRESS: '127.0.0.1:1883'
    }));

    expect(result.missing).to.include(
      'vars.AETHERLINK_MQTT_ACCESS_ADDRESS must not point to the hosted runner loopback'
    );
  });

  it('does not turn a real-rdi label into physical-device evidence', function () {
    const result = collectMissing(validEnvironment({
      AETHERLINK_DEVICE_VALIDATION_MODE: 'real-rdi'
    }));

    expect(result.missing).to.deep.include(
      'real-rdi validation is not implemented by this hosted generic-emulator lane; no physical-device claim is allowed'
    );
  });
});
