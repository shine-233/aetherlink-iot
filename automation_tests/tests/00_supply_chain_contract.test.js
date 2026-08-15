/**
 * 本地供应链输入契约。
 *
 * 只检查离线可证明的版本、manifest 和 lockfile 边界；联网漏洞数据、SBOM
 * 生成器和托管平台检查必须保持显式可选，不能伪装为默认核心能力。
 */
const { expect } = require('chai');
const { compareVersions, inspectSupplyChain } = require('../scripts/check_supply_chain');

describe('supply chain contract [00_supply_chain_contract]', function () {
  it('keeps every local-default supply-chain input check passing', function () {
    const result = inspectSupplyChain();
    expect(result.checks).not.to.be.empty;
    expect(result.checks.filter(check => check.status !== 'pass')).to.deep.equal([]);
    expect(result.ok).to.equal(true);
  });

  it('keeps component license declarations and standard files in the local contract', function () {
    const result = inspectSupplyChain();
    const licenseChecks = result.checks.filter(check => check.id.startsWith('license:'));
    expect(licenseChecks.map(check => check.id)).to.deep.equal([
      'license:frontend-declaration',
      'license:frontend-file',
      'license:backend-file',
      'license:mqtt-broker-file'
    ]);
    expect(licenseChecks.every(check => check.status === 'pass')).to.equal(true);
  });

  it('does not report network or hosted capabilities as local success', function () {
    const result = inspectSupplyChain();
    expect(result.external.map(item => [item.id, item.mode, item.status])).to.deep.equal([
      ['vulnerability-database', 'optional-external', 'not-run'],
      ['dependency-license-analysis', 'optional-external', 'not-run'],
      ['sbom-generation', 'optional-external', 'not-run'],
      ['hosted-dependency-review', 'blocked-external', 'not-run']
    ]);
  });

  it('compares toolchain versions numerically', function () {
    expect(compareVersions('1.26.4', '1.25.0')).to.be.greaterThan(0);
    expect(compareVersions('1.25', '1.25.0')).to.equal(0);
    expect(compareVersions('1.24.9', '1.25.0')).to.be.lessThan(0);
  });
});
