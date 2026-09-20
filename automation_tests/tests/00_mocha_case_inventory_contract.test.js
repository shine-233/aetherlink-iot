const fs = require('fs');
const os = require('os');
const path = require('path');
const { expect } = require('chai');

const inventory = require('../lib/runner/mocha-case-inventory');
const reporter = require('../lib/reporter');
const testMetadata = require('../lib/test_metadata');

function reportCase(file, title, fullTitle, state = 'passed') {
  return {
    results: [{
      suites: [{
        fullFile: file,
        tests: [{
          title,
          fullTitle,
          state,
          pass: state === 'passed',
          fail: state === 'failed',
          pending: state === 'pending',
          skipped: state === 'pending',
          duration: 4
        }],
        suites: []
      }]
    }]
  };
}

function managedMetadata(fullTitle, overrides = {}) {
  return {
    fileFlags: { caseMetadataManaged: true },
    cases: [{
      title: 'works',
      fullTitle,
      evidenceKind: 'business',
      businessClosureEvidence: true,
      ...overrides
    }]
  };
}

describe('Mocha case inventory contract [00_mocha_case_inventory_contract]', function() {
  it('normalizes Windows and repository-relative test paths', function() {
    expect(inventory.normalizeCaseFile('C:\\repo\\automation_tests\\tests\\29_rule_chain_business.test.js'))
      .to.equal('tests/29_rule_chain_business.test.js');
    expect(inventory.normalizeCaseFile('./tests/example.test.js')).to.equal('tests/example.test.js');
  });

  it('collects nested fullTitle identities and outcomes from Mochawesome suites', function() {
    const report = reportCase(
      'C:\\repo\\automation_tests\\tests\\example.test.js',
      'works',
      'outer nested works'
    );
    expect(inventory.collectMochaReportCases(report)).to.deep.equal([{
      file: 'tests/example.test.js',
      title: 'works',
      fullTitle: 'outer nested works',
      outcome: 'passed',
      duration: 4
    }]);
  });

  it('reconciles only an exhaustive exact file plus fullTitle match', function() {
    const fullTitle = 'outer nested works';
    const result = inventory.reconcileManagedMochaCases({
      metadataFile: 'tests/example.test.js',
      metadata: managedMetadata(fullTitle),
      reportCases: inventory.collectMochaReportCases(reportCase(
        '/repo/automation_tests/tests/example.test.js', 'works', fullTitle
      ))
    });

    expect(result).to.include({ managed: true, valid: true });
    expect(result.errors).to.deep.equal([]);
    expect(result.oracleCases).to.have.length(1);
    expect(result.oracleCases[0]).to.include({
      file: 'tests/example.test.js',
      fullTitle,
      outcome: 'passed',
      caseId: null,
      evidenceKind: 'business',
      businessClosureEvidence: true
    });
    expect(result.oracleCases[0].operationIds).to.deep.equal([]);
    expect(result.oracleCases[0].operationDimensions).to.deep.equal([]);
  });

  it('propagates v2 case identity and semantic operation evidence after exact reconciliation', function() {
    const fullTitle = 'outer works';
    const metadata = managedMetadata(fullTitle, {
      schemaVersion: 2,
      caseId: 'example.operation.works',
      operationIds: ['example.operation'],
      operationDimensions: ['response', 'stateReadback'],
      semantics: {
        actor: 'prepared-tenant-admin',
        role: 'tenant_admin',
        tenant: 'own-tenant',
        idempotency: 'not-applicable',
        state: ['persisted'],
        visibleResult: 'api-response-asserted',
        cleanup: 'not-applicable'
      }
    });
    const result = inventory.reconcileManagedMochaCases({
      metadataFile: 'tests/example.test.js',
      metadata,
      reportCases: [{
        file: 'tests/example.test.js', title: 'works', fullTitle, outcome: 'passed', duration: 1
      }]
    });

    expect(result).to.include({ valid: true });
    expect(result.oracleCases[0]).to.deep.include({
      caseId: 'example.operation.works',
      operationIds: ['example.operation'],
      operationDimensions: ['response', 'stateReadback']
    });
    expect(result.oracleCases[0].semantics.state).to.deep.equal(['persisted']);
  });

  it('fails closed for duplicate, stale, missing, and case-empty runtime identities', function() {
    const actual = {
      file: 'tests/example.test.js', title: 'works', fullTitle: 'outer works', outcome: 'passed', duration: 1
    };
    const duplicateMetadata = managedMetadata('outer works');
    duplicateMetadata.cases.push({ ...duplicateMetadata.cases[0] });
    const duplicate = inventory.reconcileManagedMochaCases({
      metadataFile: actual.file, metadata: duplicateMetadata, reportCases: [actual, actual]
    });
    expect(duplicate.errors.map(item => item.reason)).to.include.members([
      'duplicate-metadata-identity', 'duplicate-actual-identity'
    ]);
    expect(duplicate.oracleCases).to.deep.equal([]);

    const duplicateCaseIdMetadata = managedMetadata('outer works', { caseId: 'example.duplicate' });
    duplicateCaseIdMetadata.cases.push({
      ...duplicateCaseIdMetadata.cases[0],
      title: 'other',
      fullTitle: 'outer other'
    });
    const duplicateCaseId = inventory.reconcileManagedMochaCases({
      metadataFile: actual.file,
      metadata: duplicateCaseIdMetadata,
      reportCases: [
        actual,
        { ...actual, title: 'other', fullTitle: 'outer other' }
      ]
    });
    expect(duplicateCaseId.errors.map(item => item.reason))
      .to.include('duplicate-metadata-case-id');

    const mismatch = inventory.reconcileManagedMochaCases({
      metadataFile: actual.file,
      metadata: managedMetadata('outer stale'),
      reportCases: [actual]
    });
    expect(mismatch.errors.map(item => item.reason)).to.have.members([
      'stale-or-missing-runtime-case', 'missing-case-metadata'
    ]);

    const empty = inventory.reconcileManagedMochaCases({
      metadataFile: actual.file,
      metadata: managedMetadata('outer works'),
      reportCases: []
    });
    expect(empty.errors.map(item => item.reason)).to.include('managed-file-has-no-runtime-cases');
  });

  it('does not promote failed, skipped, boundary-free, or unmanaged cases', function() {
    for (const outcome of ['failed', 'skipped', 'unknown']) {
      const result = inventory.reconcileManagedMochaCases({
        metadataFile: 'tests/example.test.js',
        metadata: managedMetadata('outer works'),
        reportCases: [{ file: 'tests/example.test.js', title: 'works', fullTitle: 'outer works', outcome }]
      });
      expect(result.valid).to.equal(true);
      expect(result.oracleCases).to.deep.equal([]);
    }
    const unmanaged = inventory.reconcileManagedMochaCases({
      metadataFile: 'tests/example.test.js', metadata: { cases: [] }, reportCases: []
    });
    expect(unmanaged).to.include({ managed: false, valid: false });
  });

  it('collects nested tests without executing bodies and detects pending and focused tests', async function() {
    const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-mocha-inventory-'));
    const testFile = path.join(tempDir, 'nested.test.js');
    global.__aetherlinkInventoryBodyRan = false;
    fs.writeFileSync(testFile, [
      "describe('outer', function () {",
      "  describe('nested', function () {",
      "    it.only('focused', function () { global.__aetherlinkInventoryBodyRan = true; });",
      "    it.skip('pending', function () { global.__aetherlinkInventoryBodyRan = true; });",
      "  });",
      "});"
    ].join('\n'));

    try {
      const cases = await inventory.collectMochaFileInventory(testFile, { rootDir: tempDir });
      expect(global.__aetherlinkInventoryBodyRan).to.equal(false);
      expect(cases.map(item => item.fullTitle)).to.deep.equal([
        'outer nested focused', 'outer nested pending'
      ]);
      expect(cases.find(item => item.title === 'focused').focused).to.equal(true);
      expect(cases.find(item => item.title === 'pending').pending).to.equal(true);
    } finally {
      delete global.__aetherlinkInventoryBodyRan;
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it('rejects malformed v2 metadata instead of silently discarding fields', function() {
    const helpers = require('../lib/test-metadata/helpers');
    const base = {
      schemaVersion: 2,
      caseId: 'example.valid-case',
      title: 'works',
      fullTitle: 'outer works',
      evidenceKind: 'business',
      businessClosureEvidence: true,
      assertions: { exactStatus: true, body: true, mutationOrSeed: true, negative: false },
      capabilityIds: ['example'],
      operationIds: ['example.operation'],
      operationDimensions: ['response'],
      semantics: {
        actor: 'prepared-account', role: 'tenant_admin', tenant: 'own-tenant',
        idempotency: 'not-applicable', state: ['persisted'],
        visibleResult: 'api-response-asserted', cleanup: 'not-applicable'
      }
    };

    expect(() => helpers.metadataCase({ ...base, unknown: true })).to.throw('unknown key');
    expect(() => helpers.metadataCase({
      ...base,
      operationDimensions: ['unknown-dimension']
    })).to.throw('unsupported value');
    expect(() => helpers.metadataCase({
      ...base,
      assertions: { ...base.assertions, exactStatus: false }
    })).to.throw('business closure metadata requires');
    expect(() => helpers.metadataCase({
      ...base,
      operationDimensions: ['cleanup']
    })).to.throw('verified-by-case');
  });

  it('publishes only reconciled canonical oracle cases into summary outcomes', function() {
    const operationCase = {
      file: 'tests/29_rule_chain_business.test.js',
      title: 'creates a tenant-owned disabled rule chain with the exact graph',
      fullTitle: 'Rule chain API business flow [29_rule_chain_business] creates a tenant-owned disabled rule chain with the exact graph',
      caseId: 'automation.rule-chain.lifecycle.create-disabled',
      operationIds: ['automation.rule-chain.lifecycle'],
      operationDimensions: ['response', 'mutation', 'tenantScope'],
      semantics: {
        actor: 'prepared-tenant-admin',
        role: 'tenant_admin',
        tenant: 'own-tenant',
        idempotency: 'not-applicable',
        state: ['created'],
        visibleResult: 'api-response-asserted',
        cleanup: 'not-applicable'
      },
      outcome: 'passed',
      evidenceKind: 'business',
      businessClosureEvidence: true
    };
    reporter.results = [];
    reporter.record('rule-chain-business', '29_rule_chain_business.test.js', true, '', 'api', 'business', {
      outcome: 'passed',
      skipped: 0,
      blockedReasons: [],
      oracleCases: [
        operationCase,
        { ...operationCase, file: 'tests/unmapped.test.js', title: 'unmapped' }
      ]
    });

    try {
      expect(reporter.getCanonicalOperationCaseOutcomes()).to.deep.equal([{
        ...operationCase,
        module: 'rule-chain-business'
      }]);
    } finally {
      reporter.results = [];
    }
  });

  it('keeps the initial exhaustive operation suites explicitly managed with full titles', function() {
    const files = [
      'tests/29_rule_chain_business.test.js',
      'tests/31_scene_action_20_runtime.test.js',
      'tests/32_ota_runtime.test.js',
      'tests/36_template_market.test.js',
      'tests/37_report_schedule.test.js'
    ];
    for (const file of files) {
      const metadata = testMetadata.getTestMetadata(file);
      expect(metadata.fileFlags.caseMetadataManaged).to.equal(true);
      expect(metadata.cases).to.not.be.empty;
      expect(metadata.cases.every(item => item.fullTitle && item.fullTitle !== item.title)).to.equal(true);
      expect(metadata.cases.every(item => item.schemaVersion === 2 && item.caseId)).to.equal(true);
      expect(metadata.cases.every(item => testMetadata.getCaseMetadataById(item.caseId) === item))
        .to.equal(true);
    }
    expect(testMetadata.DUPLICATE_CASE_IDS).to.deep.equal([]);
  });
});
