const path = require('path');

const PLAYWRIGHT_CASE_SCHEMA = 'aetherlink.playwright.case.v2';
const INVALID_ANNOTATION_TYPES = new Set(['fixme', 'skip']);

function normalizePlaywrightFile(file, rootDir = '') {
  let normalized = String(file || '').trim().replace(/\\/g, '/');
  const normalizedRoot = String(rootDir || '').trim().replace(/\\/g, '/').replace(/\/$/, '');
  if (normalizedRoot && normalized.toLowerCase().startsWith((normalizedRoot + '/').toLowerCase())) {
    normalized = normalized.slice(normalizedRoot.length + 1);
  }
  normalized = normalized.replace(/^\.\//, '').replace(/^automation_tests\//, '');
  const e2eIndex = normalized.toLowerCase().lastIndexOf('/e2e/');
  if (e2eIndex >= 0) normalized = normalized.slice(e2eIndex + 1);
  if (!normalized.startsWith('e2e/')) normalized = 'e2e/' + path.posix.basename(normalized);
  return normalized;
}

function caseIdentity(file, fullTitle) {
  return `${file} :: ${fullTitle}`;
}

function collectAnnotationTypes(test, results) {
  const types = [];
  for (const annotation of test.annotations || []) {
    if (annotation && annotation.type) types.push(String(annotation.type).toLowerCase());
  }
  for (const result of results) {
    for (const annotation of result.annotations || []) {
      if (annotation && annotation.type) types.push(String(annotation.type).toLowerCase());
    }
  }
  return [...new Set(types)];
}

function extractPlaywrightJsonCases(report) {
  const cases = [];
  const rootDir = report && report.config ? report.config.rootDir : '';

  function visitSuite(suite, parentTitles = [], inheritedFile = '') {
    const suiteFile = suite.file || inheritedFile;
    const suiteTitles = suite.title ? parentTitles.concat(suite.title) : parentTitles;
    for (const spec of suite.specs || []) {
      const file = normalizePlaywrightFile(spec.file || suiteFile, rootDir);
      const fullTitle = suiteTitles.concat(spec.title).filter(Boolean).join(' › ');
      for (const test of spec.tests || []) {
        const results = Array.isArray(test.results) ? test.results : [];
        const attempts = results.map(result => ({
          retry: Number.isInteger(result.retry) ? result.retry : null,
          status: result.status || 'missing',
          duration: Number.isFinite(result.duration) ? result.duration : null,
          startTime: result.startTime || null,
        }));
        cases.push({
          file,
          title: spec.title,
          fullTitle,
          identity: caseIdentity(file, fullTitle),
          specId: String(spec.id || ''),
          projectId: String(test.projectId || ''),
          projectName: String(test.projectName || ''),
          expectedStatus: String(test.expectedStatus || ''),
          status: String(test.status || ''),
          annotations: collectAnnotationTypes(test, results),
          attempts,
        });
      }
    }
    for (const child of suite.suites || []) visitSuite(child, suiteTitles, suiteFile);
  }

  for (const suite of (report && report.suites) || []) visitSuite(suite);
  return cases;
}

function metadataCaseFullTitle(metadata, item) {
  return String(item.fullTitle || `${metadata.suiteTitle || ''} › ${item.title}`).trim();
}

function isPassedCase(item) {
  return item.expectedStatus === 'passed' &&
    item.status === 'expected' &&
    item.attempts.length > 0 &&
    item.attempts[item.attempts.length - 1].status === 'passed' &&
    !item.annotations.some(annotation => INVALID_ANNOTATION_TYPES.has(annotation));
}

function reconcilePlaywrightCases(report, metadata) {
  const file = normalizePlaywrightFile(metadata && metadata.file);
  const managedCases = metadata && Array.isArray(metadata.cases)
    ? metadata.cases.filter(item => (
      item &&
      item.evidenceKind === 'business' &&
      item.businessClosureEvidence === true &&
      item.caseId &&
      item.fullTitle
    ))
    : [];
  const errors = [];
  const invalidManagedCases = metadata && Array.isArray(metadata.cases)
    ? metadata.cases.filter(item => item && (item.caseId || item.fullTitle) && !(item.caseId && item.fullTitle))
    : [];
  for (const item of invalidManagedCases) {
    errors.push(`incomplete managed metadata identity: ${file} :: ${item.title || '<untitled>'}`);
  }
  const extracted = extractPlaywrightJsonCases(report);
  if (managedCases.length === 0) {
    errors.push(`no managed Playwright identities configured: ${file}`);
  }
  if (report && Array.isArray(report.errors) && report.errors.length > 0) {
    errors.push('Playwright JSON report contains top-level errors');
  }
  const allMetadataCases = metadata && Array.isArray(metadata.cases) ? metadata.cases : [];
  const knownMetadataIdentities = new Set(allMetadataCases.map(item => (
    caseIdentity(file, metadataCaseFullTitle(metadata, item))
  )));
  const knownFullTitles = new Set(allMetadataCases.map(item => metadataCaseFullTitle(metadata, item)));
  const reportCases = extracted.filter(item => item.file === file);
  const wrongFileCases = extracted.filter(item => (
    item.file !== file && knownFullTitles.has(item.fullTitle)
  ));
  const unrelatedFileCases = extracted.filter(item => (
    item.file !== file && !knownFullTitles.has(item.fullTitle)
  ));

  const reportByIdentity = new Map();
  for (const item of reportCases) {
    const sameIdentity = reportByIdentity.get(item.identity) || [];
    sameIdentity.push(item);
    reportByIdentity.set(item.identity, sameIdentity);
  }
  for (const [identity, matches] of reportByIdentity.entries()) {
    const projectIds = new Set(matches.map(match => match.projectId));
    if (projectIds.size > 1) errors.push(`ambiguous multi-project Playwright identity: ${identity}`);
    if (matches.length > 1 && projectIds.size <= 1) errors.push(`duplicate Playwright identity: ${identity}`);
  }

  const metadataIdentities = new Set();
  const caseIds = new Set();
  for (const item of managedCases) {
    const fullTitle = metadataCaseFullTitle(metadata, item);
    const identity = caseIdentity(file, fullTitle);
    if (caseIds.has(item.caseId)) errors.push(`duplicate metadata case ID: ${item.caseId}`);
    caseIds.add(item.caseId);
    if (!/^pw-v2-[a-z0-9]+(?:-[a-z0-9]+)*$/.test(item.caseId)) {
      errors.push(`invalid v2 metadata case ID: ${item.caseId}`);
    }
    if (!item.operationDimensions || typeof item.operationDimensions !== 'object') {
      errors.push(`missing operation dimensions: ${identity}`);
    }
    if (metadataIdentities.has(identity)) errors.push(`duplicate metadata identity: ${identity}`);
    metadataIdentities.add(identity);

    const matches = reportByIdentity.get(identity) || [];
    if (matches.length === 0) errors.push(`missing or stale Playwright identity: ${identity}`);
    for (const match of matches) {
      if (match.title !== item.title) errors.push(`stale Playwright title: ${identity}`);
      if (match.expectedStatus !== 'passed') errors.push(`skipped or expected-failure Playwright identity: ${identity}`);
      if (match.annotations.some(annotation => INVALID_ANNOTATION_TYPES.has(annotation))) {
        errors.push(`skipped Playwright identity: ${identity}`);
      }
      if (
        match.status !== 'expected' ||
        match.attempts.length === 0 ||
        match.attempts[match.attempts.length - 1].status !== 'passed'
      ) {
        errors.push(`Playwright identity did not reconcile to passed: ${identity}`);
      }
    }
  }

  for (const item of wrongFileCases) errors.push(`wrong-file Playwright identity: ${item.identity}`);
  for (const item of unrelatedFileCases) errors.push(`unrelated Playwright report identity: ${item.identity}`);
  for (const item of reportCases) {
    if (!knownMetadataIdentities.has(item.identity)) {
      errors.push(`stale or unmanaged Playwright identity: ${item.identity}`);
    }
  }
  if (report && report.config && report.config.forbidOnly !== true) {
    errors.push('focused Playwright selection is not rejected because config.forbidOnly is not true');
  }

  const oracleCases = errors.length === 0
    ? managedCases.map(item => {
      const fullTitle = metadataCaseFullTitle(metadata, item);
      const match = reportByIdentity.get(caseIdentity(file, fullTitle))[0];
      return {
        schema: PLAYWRIGHT_CASE_SCHEMA,
        caseId: item.caseId,
        file,
        title: item.title,
        fullTitle,
        playwrightSpecId: match.specId,
        operationDimensions: item.operationDimensions,
        businessClosureEvidence: item.businessClosureEvidence === true,
        status: match.status,
        projectId: match.projectId,
        projectName: match.projectName,
        attempts: match.attempts,
      };
    }).filter(item => {
      const match = reportByIdentity.get(caseIdentity(file, item.fullTitle))[0];
      return item.businessClosureEvidence && isPassedCase(match);
    })
    : [];

  return { file, extractedCases: reportCases, oracleCases, errors };
}

module.exports = {
  PLAYWRIGHT_CASE_SCHEMA,
  normalizePlaywrightFile,
  extractPlaywrightJsonCases,
  reconcilePlaywrightCases,
};
