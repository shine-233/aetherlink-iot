'use strict';

/*
 * Purpose: single source of truth for the synthetic-rdi fixture voucher contract.
 *
 * 凭证哈希 Phase 2a（references/backend-hardening-plan.md 车道1）：/device/detail 自本批次起
 * 只返回脱敏 voucher。synthetic-rdi 夹具由 seed_synthetic_rdi_fixture.js 以确定性契约直接写入
 * devices.voucher 列（username = synthetic-rdi-<fixtureId>，password 固定占位），
 * 运行器必须按同一契约重建凭证，而不是从详情接口读取。
 */

const FIXTURE_VOUCHER_PASSWORD = 'not-a-device-secret';

function syntheticRdiFixtureVoucher(fixtureId) {
  return {
    username: `synthetic-rdi-${fixtureId}`,
    password: FIXTURE_VOUCHER_PASSWORD
  };
}

module.exports = { FIXTURE_VOUCHER_PASSWORD, syntheticRdiFixtureVoucher };
