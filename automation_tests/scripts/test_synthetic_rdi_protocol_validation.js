const assert = require('assert');

const {
  PROVENANCE,
  evidenceFields,
  validateSyntheticFixtureDetail,
  assertSyntheticStateTransition,
  validateLoopbackBroker,
  redactText
} = require('./run_synthetic_rdi_protocol_validation');
const { FIXTURE_VOUCHER_PASSWORD, syntheticRdiFixtureVoucher } = require('../lib/synthetic_rdi_contract');

function validDetail(overrides = {}) {
  return {
    id: '64afc1ec-8a74-4a85-ae8f-5727ff52d720',
    device_number: 'SYNTHRDI0001',
    activate_flag: 'active',
    is_enabled: 'enabled',
    voucher: JSON.stringify({
      username: 'synthetic-rdi-SYNTHRDI0001-0070aaa3',
      password: 'not-a-device-secret'
    }),
    additional_info: JSON.stringify({
      fixture_provenance: PROVENANCE,
      fixture_id: 'SYNTHRDI0001-0070aaa3',
      fixture_pid: 'SYNTHRDI0001',
      connection_type: PROVENANCE,
      hardware_identity: {
        kind: 'synthetic',
        serial: 'SYNTH-HW-SYNTHRDI0001',
        provenance: PROVENANCE
      }
    }),
    ...overrides
  };
}

function rejects(fn, expectedText) {
  assert.throws(fn, error => {
    assert.match(String(error && error.message), expectedText);
    return true;
  });
}

// 凭证哈希 Phase 2a：校验结果同时携带契约重建的 username/password，
// 运行器不再从 /device/detail 读明文（详情只回掩码）。
assert.deepStrictEqual(
  validateSyntheticFixtureDetail(validDetail(), 'SYNTHRDI0001', '64afc1ec-8a74-4a85-ae8f-5727ff52d720'),
  {
    fixtureId: 'SYNTHRDI0001-0070aaa3',
    hardware: { kind: 'synthetic', serial: 'SYNTH-HW-SYNTHRDI0001' },
    activation: { activateFlag: 'active', isEnabled: 'enabled', action: 'not-executed' },
    voucherUsername: 'synthetic-rdi-SYNTHRDI0001-0070aaa3',
    voucherPassword: FIXTURE_VOUCHER_PASSWORD
  }
);

// 详情返回掩码 voucher（Phase 2a 形态）时仍可通过：凭证按 fixture 契约重建。
assert.deepStrictEqual(
  validateSyntheticFixtureDetail(
    validDetail({ voucher: '{"username…' }),
    'SYNTHRDI0001',
    '64afc1ec-8a74-4a85-ae8f-5727ff52d720'
  ),
  {
    fixtureId: 'SYNTHRDI0001-0070aaa3',
    hardware: { kind: 'synthetic', serial: 'SYNTH-HW-SYNTHRDI0001' },
    activation: { activateFlag: 'active', isEnabled: 'enabled', action: 'not-executed' },
    voucherUsername: syntheticRdiFixtureVoucher('SYNTHRDI0001-0070aaa3').username,
    voucherPassword: FIXTURE_VOUCHER_PASSWORD
  }
);

rejects(
  () => validateSyntheticFixtureDetail(
    validDetail({ additional_info: JSON.stringify({ mode: PROVENANCE }) }),
    'SYNTHRDI0001',
    '64afc1ec-8a74-4a85-ae8f-5727ff52d720'
  ),
  /fixture_provenance must be explicitly synthetic-rdi/
);

rejects(
  () => validateSyntheticFixtureDetail(
    validDetail({
      voucher: JSON.stringify({ username: 'physical-rdi-voucher', password: 'secret' })
    }),
    'SYNTHRDI0001',
    '64afc1ec-8a74-4a85-ae8f-5727ff52d720'
  ),
  /voucher username must be scoped to the synthetic fixture/
);

rejects(
  () => evidenceFields({ real_rdi_status: 'passed' }),
  /real_rdi_status must remain not-tested/
);

assert.deepStrictEqual(
  assertSyntheticStateTransition(
    { before: { is_online: 0 }, online: { is_online: 1 }, offline: { is_online: 0 } },
    'success'
  ),
  { beforeOffline: true, onlineTransition: true, offlineTransition: true }
);

rejects(
  () => assertSyntheticStateTransition(
    { before: { is_online: 1 }, online: { is_online: 1 }, offline: { is_online: 0 } },
    'bad-precondition'
  ),
  /must be offline before emulator start/
);

assert.strictEqual(validateLoopbackBroker('127.0.0.1:11086'), '127.0.0.1:11086');
rejects(() => validateLoopbackBroker('198.51.100.20:1883'), /only permits an explicit loopback broker/);
assert.strictEqual(redactText('password=fixture-secret Bearer abc.def.ghi'), 'password=[REDACTED] Bearer [REDACTED]');

process.stdout.write('synthetic-rdi validation unit checks: 9 passed\n');
