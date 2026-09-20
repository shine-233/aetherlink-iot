const path = require('path');

function normalizeCaseFile(file) {
  const normalized = String(file || '').replace(/\\/g, '/').replace(/^\.\//, '');
  const testsIndex = normalized.lastIndexOf('/tests/');
  if (testsIndex >= 0) return normalized.slice(testsIndex + 1);
  return normalized.replace(/^\/+/, '');
}

function caseIdentity(file, fullTitle) {
  return `${normalizeCaseFile(file)}::${String(fullTitle || '').trim()}`;
}

function getMetadataCaseIdentity(file, metadataCase) {
  return caseIdentity(file, metadataCase && (metadataCase.fullTitle || metadataCase.title));
}

function getRuntimeOutcome(test) {
  if (test && (test.pass === true || test.state === 'passed')) return 'passed';
  if (test && (test.pending === true || test.skipped === true || test.state === 'pending')) return 'skipped';
  if (test && (test.fail === true || test.state === 'failed')) return 'failed';
  return 'unknown';
}

function collectMochaReportCases(report) {
  const cases = [];
  const visitSuite = (suite, inheritedFile = '') => {
    if (!suite || typeof suite !== 'object') return;
    const file = normalizeCaseFile(suite.fullFile || suite.file || inheritedFile);
    for (const test of Array.isArray(suite.tests) ? suite.tests : []) {
      cases.push({
        file,
        title: typeof test.title === 'string' ? test.title : '',
        fullTitle: typeof test.fullTitle === 'string' ? test.fullTitle : '',
        outcome: getRuntimeOutcome(test),
        duration: Number(test.duration || 0)
      });
    }
    for (const child of Array.isArray(suite.suites) ? suite.suites : []) {
      visitSuite(child, file);
    }
  };
  for (const result of Array.isArray(report && report.results) ? report.results : []) {
    visitSuite(result);
  }
  return cases;
}

function countIdentities(items, identityForItem) {
  const counts = new Map();
  for (const item of items) {
    const identity = identityForItem(item);
    counts.set(identity, (counts.get(identity) || 0) + 1);
  }
  return counts;
}

function reconcileManagedMochaCases({ metadataFile, metadata, reportCases }) {
  const canonicalFile = normalizeCaseFile(metadataFile);
  const managed = Boolean(metadata && metadata.fileFlags && metadata.fileFlags.caseMetadataManaged === true);
  if (!managed) {
    return { managed: false, valid: false, metadataFile: canonicalFile, caseResults: [], oracleCases: [], errors: [] };
  }

  const metadataCases = Array.isArray(metadata.cases) ? metadata.cases : [];
  const actualCases = (Array.isArray(reportCases) ? reportCases : [])
    .filter(item => normalizeCaseFile(item.file) === canonicalFile);
  const metadataCounts = countIdentities(metadataCases, item => getMetadataCaseIdentity(canonicalFile, item));
  const actualCounts = countIdentities(actualCases, item => caseIdentity(item.file, item.fullTitle));
  const errors = [];

  const duplicateCaseIds = new Set();
  const caseIdCounts = countIdentities(
    metadataCases.filter(item => item && item.caseId),
    item => item.caseId
  );
  for (const [caseId, count] of caseIdCounts) {
    if (count > 1) duplicateCaseIds.add(caseId);
  }
  for (const caseId of duplicateCaseIds) {
    errors.push({ caseId, reason: 'duplicate-metadata-case-id' });
  }

  for (const [identity, count] of metadataCounts) {
    if (count > 1) errors.push({ identity, reason: 'duplicate-metadata-identity' });
  }
  for (const [identity, count] of actualCounts) {
    if (count > 1) errors.push({ identity, reason: 'duplicate-actual-identity' });
  }
  if (actualCases.length === 0) {
    errors.push({ file: canonicalFile, reason: 'managed-file-has-no-runtime-cases' });
  }
  for (const [identity] of metadataCounts) {
    if (!actualCounts.has(identity)) errors.push({ identity, reason: 'stale-or-missing-runtime-case' });
  }
  for (const [identity] of actualCounts) {
    if (!metadataCounts.has(identity)) errors.push({ identity, reason: 'missing-case-metadata' });
  }

  const metadataByIdentity = new Map(metadataCases.map(item => [getMetadataCaseIdentity(canonicalFile, item), item]));
  const caseResults = actualCases.map(item => {
    const identity = caseIdentity(item.file, item.fullTitle);
    const caseMetadata = metadataByIdentity.get(identity) || null;
    return {
      file: canonicalFile,
      title: item.title,
      fullTitle: item.fullTitle,
      outcome: item.outcome,
      duration: item.duration,
      caseId: caseMetadata && caseMetadata.caseId || null,
      operationIds: caseMetadata && Array.isArray(caseMetadata.operationIds)
        ? [...caseMetadata.operationIds]
        : [],
      operationDimensions: caseMetadata && Array.isArray(caseMetadata.operationDimensions)
        ? [...caseMetadata.operationDimensions]
        : [],
      semantics: caseMetadata && caseMetadata.semantics
        ? { ...caseMetadata.semantics }
        : null,
      evidenceKind: caseMetadata && caseMetadata.evidenceKind || 'unknown',
      businessClosureEvidence: Boolean(caseMetadata && caseMetadata.businessClosureEvidence === true)
    };
  });
  const valid = errors.length === 0;
  const oracleCases = valid
    ? caseResults.filter(item => item.outcome === 'passed' && ['business', 'boundary'].includes(item.evidenceKind))
    : [];

  return { managed, valid, metadataFile: canonicalFile, caseResults, oracleCases, errors };
}

async function collectMochaFileInventory(testFile, options = {}) {
  const Mocha = require('mocha');
  const rootDir = options.rootDir || path.dirname(testFile);
  const mocha = new Mocha({ dryRun: true });
  mocha.addFile(testFile);
  await mocha.loadFilesAsync();

  const focusedTests = new Set();
  const visitFocused = suite => {
    for (const test of Array.isArray(suite && suite._onlyTests) ? suite._onlyTests : []) {
      focusedTests.add(test);
    }
    for (const child of Array.isArray(suite && suite._onlySuites) ? suite._onlySuites : []) {
      child.eachTest(test => focusedTests.add(test));
    }
    for (const child of Array.isArray(suite && suite.suites) ? suite.suites : []) {
      visitFocused(child);
    }
  };
  visitFocused(mocha.suite);

  const tests = [];
  mocha.suite.eachTest(test => {
    tests.push({
      file: normalizeCaseFile(path.relative(rootDir, test.file || testFile)),
      title: test.title,
      fullTitle: test.fullTitle(),
      pending: test.isPending(),
      focused: focusedTests.has(test)
    });
  });
  mocha.unloadFiles();
  return tests;
}

module.exports = {
  normalizeCaseFile,
  caseIdentity,
  getMetadataCaseIdentity,
  getRuntimeOutcome,
  collectMochaReportCases,
  collectMochaFileInventory,
  reconcileManagedMochaCases
};
