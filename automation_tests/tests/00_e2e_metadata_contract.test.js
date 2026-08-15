const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const { expect } = require('chai');
const coverageContract = require('../lib/coverage_contract');
const oracleContract = require('../lib/oracle_contract');
const {
  TEST_METADATA,
  normalizeTestPath,
  getTestMetadata,
  getCaseMetadata
} = require('../lib/test_metadata');

const AUTOMATION_ROOT = path.resolve(__dirname, '..');
const NON_BEHAVIOR_TITLE_PATTERN = /\b(?:page[- ]?smoke(?:\s+only)?|route[- ]?(?:only|smoke)|smoke(?:\s+only)?|weak[- ]?assertion)\b/i;
function normalizeE2EPath(file) {
  const normalized = String(file || '').replace(/\\/g, '/').replace(/^\.\//, '');
  return normalized.startsWith('e2e/') ? normalized : 'e2e/' + normalized;
}

function collectPlaywrightInventory(report) {
  const tests = [];
  const describes = [];

  function visitSuite(suite, inheritedFile = '') {
    const suiteFile = suite.file || inheritedFile;
    if (suite.line > 0 && suite.title) {
      describes.push({
        file: normalizeE2EPath(suiteFile),
        title: suite.title
      });
    }

    for (const spec of suite.specs || []) {
      tests.push({
        file: normalizeE2EPath(spec.file || suiteFile),
        title: spec.title
      });
    }
    for (const childSuite of suite.suites || []) {
      visitSuite(childSuite, suiteFile);
    }
  }

  for (const suite of report.suites || []) {
    visitSuite(suite);
  }

  return { tests, describes };
}

function listPlaywrightInventory() {
  const cliPath = require.resolve('@playwright/test/cli');
  const result = spawnSync(
    process.execPath,
    [cliPath, 'test', '--list', '--reporter=json', '--no-deps'],
    {
      cwd: AUTOMATION_ROOT,
      encoding: 'utf8',
      env: {
        ...process.env,
        PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD: '1'
      },
      maxBuffer: 4 * 1024 * 1024,
      timeout: 30000
    }
  );

  expect(result.error, result.error && result.error.message).to.equal(undefined);
  expect(result.status, result.stderr || result.stdout).to.equal(0);

  const report = JSON.parse(result.stdout);
  expect(report.errors, 'Playwright collection errors').to.deep.equal([]);
  return collectPlaywrightInventory(report);
}

function findBehaviorClaimProblems(item) {
  const problems = [];
  const claimsBehavior = item.evidenceKind === 'business' ||
    item.businessClosureEvidence === true ||
    item.provesBusinessFlow === true;
  const explicitlyNonBehavior = item.evidenceKind === 'page-coverage-only' ||
    item.pageSmokeOnly === true ||
    item.weakAssertionOnly === true ||
    NON_BEHAVIOR_TITLE_PATTERN.test(item.title);
  const explicitlyNonBrowser = item.hasBrowserUserFlow === false ||
    item.evidenceLayer === 'api-via-e2e-fixture';

  if (claimsBehavior && explicitlyNonBehavior) {
    problems.push('non-behavior evidence claims browser business coverage');
  }
  if (claimsBehavior && explicitlyNonBrowser) {
    problems.push('non-browser evidence claims browser business coverage');
  }
  if (item.businessClosureEvidence === true && item.evidenceKind !== 'business') {
    problems.push('business closure is true while evidenceKind is not business');
  }
  if (item.businessClosureEvidence === true && item.provesBusinessFlow !== true) {
    problems.push('business closure is true while provesBusinessFlow is not true');
  }
  if (item.provesBusinessFlow === true && item.hasBrowserUserFlow === false) {
    problems.push('provesBusinessFlow is true without a browser user flow');
  }
  if (item.evidenceKind === 'business' && item.businessClosureEvidence !== true) {
    problems.push('business evidenceKind does not opt in to business closure');
  }

  return problems;
}

describe('Playwright E2E metadata contract', function () {
  this.timeout(40000);

  let inventory;

  before(function () {
    inventory = listPlaywrightInventory();
  });

  it('classifies this static harness check as contract evidence', function () {
    expect(TEST_METADATA['tests/00_e2e_metadata_contract.test.js']).to.include({
      type: 'api',
      evidenceKind: 'contract'
    });
  });

  it('keeps the metadata facade lookup contract stable across ordered data parts', function () {
    const keys = Object.keys(TEST_METADATA);
    expect(keys).to.have.length(60);
    expect(new Set(keys).size).to.equal(keys.length);
    expect(keys[0]).to.equal('tests/00_coverage_contract.test.js');
    expect(keys[keys.length - 1]).to.equal('e2e/14_route_coverage_closure.spec.js');
    expect(normalizeTestPath('.\\e2e\\10_automation.spec.js')).to.equal('e2e/10_automation.spec.js');
    expect(getTestMetadata('C:/tmp/00_coverage_contract.test.js')).to.equal(
      TEST_METADATA['tests/00_coverage_contract.test.js']
    );

    const sample = TEST_METADATA['e2e/10_automation.spec.js'].cases[0];
    expect(getCaseMetadata('e2e/10_automation.spec.js', sample.title)).to.equal(sample);
    expect(getCaseMetadata('missing.test.js', sample.title)).to.equal(null);
  });

  it('collects actual test and describe titles through Playwright without running browsers', function () {
    expect(inventory.tests.length).to.be.greaterThan(0);
    expect(inventory.describes.length).to.be.greaterThan(0);
    expect(inventory.tests.every(item => item.file.startsWith('e2e/'))).to.equal(true);
    expect(inventory.describes.every(item => item.file.startsWith('e2e/'))).to.equal(true);
  });

  it('keeps describe titles free of smoke and weak-assertion coverage labels', function () {
    const prohibited = inventory.describes
      .filter(item => NON_BEHAVIOR_TITLE_PATTERN.test(item.title))
      .map(item => item.file + ' :: ' + item.title);

    expect(prohibited).to.deep.equal([]);
  });

  it('keeps every actual E2E case and metadata case in exact one-to-one sync', function () {
    const actualByFile = new Map();
    for (const item of inventory.tests) {
      const titles = actualByFile.get(item.file) || [];
      titles.push(item.title);
      actualByFile.set(item.file, titles);
    }

    const metadataEntries = Object.entries(TEST_METADATA)
      .filter(([, metadata]) => metadata.type === 'e2e');
    const metadataFiles = metadataEntries.map(([file]) => file).sort();
    const actualFiles = [...actualByFile.keys()].sort();
    expect(metadataFiles).to.deep.equal(actualFiles);

    for (const [file, metadata] of metadataEntries) {
      expect(metadata.file).to.equal(file);
      const actualTitles = actualByFile.get(file) || [];
      const metadataTitles = metadata.cases.map(item => item.title);
      expect(new Set(actualTitles).size, file + ' has duplicate Playwright titles').to.equal(actualTitles.length);
      expect(new Set(metadataTitles).size, file + ' has duplicate metadata titles').to.equal(metadataTitles.length);
      expect([...metadataTitles].sort(), file).to.deep.equal([...actualTitles].sort());
    }
  });

  it('does not let smoke, page-only, API-only, or weak metadata claim browser behavior coverage', function () {
    const findings = [];
    for (const metadata of Object.values(TEST_METADATA).filter(item => item.type === 'e2e')) {
      for (const item of metadata.cases) {
        for (const problem of findBehaviorClaimProblems(item)) {
          findings.push(metadata.file + ' :: ' + item.title + ' :: ' + problem);
        }
      }
    }

    expect(findings).to.deep.equal([]);
  });

  it('backs every business metadata claim with source preparation, user interaction, and an observable result', function () {
    const findings = [];
    for (const metadata of Object.values(TEST_METADATA).filter(item => item.type === 'e2e')) {
      const source = fs.readFileSync(path.join(AUTOMATION_ROOT, metadata.file), 'utf8');
      findings.push(...coverageContract.getE2EMetadataSourceAudit(source, metadata.file));
    }

    expect(findings).to.deep.equal([]);
  });

  it('recognizes prohibited evidence labels even when a future case is marked business', function () {
    for (const title of [
      'route-only',
      'route only',
      'route smoke',
      'page smoke only',
      'weak assertion only'
    ]) {
      const problems = findBehaviorClaimProblems({
        title,
        evidenceKind: 'business',
        businessClosureEvidence: true,
        provesBusinessFlow: true,
        hasBrowserUserFlow: true,
        evidenceLayer: 'browser-e2e-with-api-setup'
      });
      expect(problems, title).to.include('non-behavior evidence claims browser business coverage');
    }
  });

  it('keeps alarm and OTA capability negative oracles backed by explicit metadata', function () {
    const statuses = new Map(
      oracleContract.selfCheck().capabilityOracles.map(item => [item.capability, item])
    );

    expect(statuses.get('alarm-notification').negativeOracle).to.equal(true);
    expect(statuses.get('ota-script-openapi-service').negativeOracle).to.equal(true);
  });
});
