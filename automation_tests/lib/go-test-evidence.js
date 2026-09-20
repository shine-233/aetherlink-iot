'use strict';

const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

const GO_TEST_EVIDENCE_SCHEMA = 'aetherlink.go-test.evidence.v1';
const VALID_EVIDENCE_ROLES = new Set(['business', 'boundary', 'contract', 'source-structure']);
const VALID_RUNTIME_OUTCOMES = new Set(['passed', 'failed', 'skipped', 'unknown']);
const REPOSITORY_ROOTS = Object.freeze({
  backend: 'aetherlink-iot/backend',
  gmqtt: 'github.com/DrmagicE/gmqtt'
});

function normalizeRepositoryPath(value) {
  return String(value || '').replace(/\\/g, '/').replace(/^\.\//, '');
}

function isSafeRepositoryPath(value) {
  const normalized = normalizeRepositoryPath(value);
  return normalized.length > 0 &&
    !normalized.startsWith('/') &&
    !/^[A-Za-z]:\//.test(normalized) &&
    !normalized.split('/').includes('..');
}

function getLayer(evidence) {
  const file = normalizeRepositoryPath(evidence && evidence.repositoryFile);
  if (!isSafeRepositoryPath(file)) return null;
  if (file.startsWith('backend/')) return 'backend';
  if (file.startsWith('mqtt-broker/')) return 'gmqtt';
  return null;
}

function isPackageWithinRoot(root, packageName) {
  return packageName === root || packageName.startsWith(`${root}/`);
}

function getModulePackage(layer, packageName) {
  const root = REPOSITORY_ROOTS[layer];
  if (!root || typeof packageName !== 'string' || !isPackageWithinRoot(root, packageName)) return null;
  const suffix = packageName.slice(root.length).replace(/^\//, '');
  return suffix ? `./${suffix}` : '.';
}

function runtimeIdentity(evidence) {
  return `${evidence.package}::${evidence.testFunction}`;
}

function normalizePackageToken(value) {
  return String(value || '').replace(/[^A-Za-z0-9]/g, '').toLowerCase();
}

function getDeclaredPackageCandidates(evidence, projectRoot) {
  const layer = getLayer(evidence);
  const moduleRoot = path.resolve(
    projectRoot,
    layer === 'backend' ? 'backend' : 'mqtt-broker'
  );
  const sourceDirectory = path.dirname(path.resolve(
    projectRoot,
    normalizeRepositoryPath(evidence && evidence.repositoryFile)
  ));
  const relativeDirectory = path.relative(moduleRoot, sourceDirectory);
  const packageDirectory = relativeDirectory && relativeDirectory !== '.'
    ? relativeDirectory.split(path.sep).join('/')
    : '';
  const importSuffix = String(evidence && evidence.package || '')
    .split('/')
    .filter(Boolean)
    .pop() || '';
  const directorySuffix = packageDirectory
    .split('/')
    .filter(Boolean)
    .pop() || '';
  return new Set([
    importSuffix,
    `${importSuffix}_test`,
    directorySuffix,
    `${directorySuffix}_test`,
    importSuffix.replace(/[^A-Za-z0-9_]/g, ''),
    `${importSuffix.replace(/[^A-Za-z0-9_]/g, '')}_test`,
    directorySuffix.replace(/[^A-Za-z0-9_]/g, ''),
    `${directorySuffix.replace(/[^A-Za-z0-9_]/g, '')}_test`
  ].filter(Boolean));
}

function hasCompatibleDeclaredPackage(evidence, projectRoot, packageName) {
  const declared = String(packageName || '');
  const baseDeclared = declared.endsWith('_test')
    ? declared.slice(0, -5)
    : declared;
  return getDeclaredPackageCandidates(evidence, projectRoot).has(declared) ||
    Array.from(getDeclaredPackageCandidates(evidence, projectRoot)).some(candidate => {
      const baseCandidate = candidate.endsWith('_test')
        ? candidate.slice(0, -5)
        : candidate;
      return normalizePackageToken(baseCandidate) === normalizePackageToken(baseDeclared);
    });
}

function inspectGoPackage(projectRoot, evidence, options = {}) {
  const layer = getLayer(evidence);
  const modulePackage = getModulePackage(layer, evidence && evidence.package);
  if (!layer || !modulePackage) {
    return { resolved: false, error: 'package-outside-module', importPath: null };
  }
  const moduleRoot = path.join(projectRoot, layer === 'backend' ? 'backend' : 'mqtt-broker');
  const listGoPackage = options.listGoPackage || ((cwd, target) => spawnSync(
    'go',
    ['list', '-f', '{{.ImportPath}}', target],
    {
      cwd,
      encoding: 'utf8',
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024
    }
  ));
  const result = listGoPackage(moduleRoot, modulePackage);
  if (result.error || result.status !== 0) {
    return {
      resolved: false,
      error: String(result.stderr || result.error?.message || 'go list failed').trim(),
      importPath: null
    };
  }
  const importPath = String(result.stdout || '').trim();
  return {
    resolved: importPath === evidence.package,
    error: importPath === evidence.package ? null : 'package-import-path-mismatch',
    importPath: importPath || null
  };
}

function inspectGoFile(projectRoot, repositoryFile, options = {}) {
  const inspectorPath = options.inspectorPath || path.join(__dirname, '..', 'scripts', 'inspect_go_test_file.go');
  const normalizedFile = normalizeRepositoryPath(repositoryFile);
  const rootPath = path.resolve(projectRoot);
  const absolutePath = path.resolve(rootPath, normalizedFile);
  if (
    !isSafeRepositoryPath(normalizedFile) ||
    (!absolutePath.startsWith(rootPath + path.sep) && absolutePath !== rootPath) ||
    !fs.existsSync(absolutePath)
  ) {
    return { exists: false, parseError: null, packageName: null, tests: [] };
  }
  const runInspector = options.runInspector || ((cwd, scriptPath, input) => spawnSync(
    'go',
    ['run', scriptPath],
    {
      cwd,
      input,
      encoding: 'utf8',
      windowsHide: true,
      maxBuffer: 8 * 1024 * 1024
    }
  ));
  const result = runInspector(
    projectRoot,
    inspectorPath,
    JSON.stringify({ file: absolutePath })
  );
  if (result.error || result.status !== 0) {
    return {
      exists: true,
      parseError: String(result.stderr || result.error?.message || 'Go AST inspector failed').trim(),
      packageName: null,
      tests: []
    };
  }
  try {
    const parsed = JSON.parse(result.stdout);
    return {
      exists: true,
      parseError: null,
      packageName: parsed.package,
      tests: Array.isArray(parsed.tests) ? parsed.tests : []
    };
  } catch (error) {
    return { exists: true, parseError: error.message, packageName: null, tests: [] };
  }
}

function validateGoEvidenceInventory(evidenceItems, projectRoot, options = {}) {
  const errors = [];
  const ids = new Map();
  const anchors = new Map();
  const identities = new Map();
  const capabilityIds = new Set(options.capabilityIds || []);
  const sourceCache = new Map();
  const packageCache = new Map();
  for (const evidence of evidenceItems || []) {
    const layer = getLayer(evidence);
    const file = normalizeRepositoryPath(evidence && evidence.repositoryFile);
    const identity = evidence && evidence.package && evidence.testFunction
      ? runtimeIdentity(evidence)
      : null;
    if (!evidence || typeof evidence.evidenceId !== 'string' || !evidence.evidenceId.trim()) {
      errors.push({ evidenceId: evidence?.evidenceId || null, reason: 'missing-evidence-id' });
    } else if (ids.has(evidence.evidenceId)) {
      errors.push({ evidenceId: evidence.evidenceId, reason: 'duplicate-evidence-id' });
    } else {
      ids.set(evidence.evidenceId, evidence);
    }
    if (!evidence || typeof evidence.semanticAnchor !== 'string' || !evidence.semanticAnchor.trim()) {
      errors.push({ evidenceId: evidence?.evidenceId || null, reason: 'missing-semantic-anchor' });
    } else if (anchors.has(evidence.semanticAnchor)) {
      errors.push({ evidenceId: evidence.evidenceId, semanticAnchor: evidence.semanticAnchor, reason: 'duplicate-semantic-anchor' });
    } else {
      anchors.set(evidence.semanticAnchor, evidence);
    }
    if (identity && identities.has(identity)) {
      errors.push({ evidenceId: evidence.evidenceId, runtimeIdentity: identity, reason: 'duplicate-runtime-identity' });
    } else if (identity) {
      identities.set(identity, evidence);
    }
    if (!layer) {
      errors.push({ evidenceId: evidence?.evidenceId || null, file, reason: 'unsupported-repository-file' });
      continue;
    }
    if (!VALID_EVIDENCE_ROLES.has(evidence.evidenceRole)) {
      errors.push({ evidenceId: evidence.evidenceId, evidenceRole: evidence.evidenceRole, reason: 'invalid-evidence-role' });
    }
    if (!/^Test[A-Z0-9_]/.test(String(evidence.testFunction || ''))) {
      errors.push({ evidenceId: evidence.evidenceId, testFunction: evidence.testFunction || null, reason: 'invalid-test-function-name' });
    }
    if (
      !evidence ||
      !Array.isArray(evidence.capabilityIds) ||
      evidence.capabilityIds.length === 0
    ) {
      errors.push({ evidenceId: evidence?.evidenceId || null, reason: 'missing-capability-ids' });
    } else {
      const seenCapabilityIds = new Set();
      for (const capabilityId of evidence.capabilityIds) {
        if (typeof capabilityId !== 'string' || !capabilityId.trim()) {
          errors.push({ evidenceId: evidence.evidenceId, capabilityId, reason: 'invalid-capability-id' });
          continue;
        }
        if (seenCapabilityIds.has(capabilityId)) {
          errors.push({ evidenceId: evidence.evidenceId, capabilityId, reason: 'duplicate-capability-id' });
        }
        seenCapabilityIds.add(capabilityId);
        if (capabilityIds.size > 0 && !capabilityIds.has(capabilityId)) {
          errors.push({ evidenceId: evidence.evidenceId, capabilityId, reason: 'unknown-capability-id' });
        }
      }
    }
    const modulePackage = getModulePackage(layer, evidence.package);
    if (!modulePackage) {
      errors.push({ evidenceId: evidence.evidenceId, package: evidence.package, reason: 'package-outside-module' });
    } else {
      const packageKey = `${layer}:${evidence.package}`;
      let packageInspection = packageCache.get(packageKey);
      if (!packageInspection) {
        packageInspection = inspectGoPackage(projectRoot, evidence, options);
        packageCache.set(packageKey, packageInspection);
      }
      if (!packageInspection.resolved) {
        errors.push({
          evidenceId: evidence.evidenceId,
          package: evidence.package,
          resolvedPackage: packageInspection.importPath,
          detail: packageInspection.error,
          reason: 'go-package-resolution-failed'
        });
      }
    }
    if (!file.endsWith('_test.go')) {
      errors.push({ evidenceId: evidence.evidenceId, file, reason: 'not-go-test-file' });
      continue;
    }
    let source = sourceCache.get(file);
    if (!source) {
      source = inspectGoFile(projectRoot, file, options);
      sourceCache.set(file, source);
    }
    if (!source.exists) {
      errors.push({ evidenceId: evidence.evidenceId, file, reason: 'missing-test-file' });
      continue;
    }
    if (source.parseError) {
      errors.push({ evidenceId: evidence.evidenceId, file, detail: source.parseError, reason: 'go-ast-parse-failed' });
      continue;
    }
    if (!hasCompatibleDeclaredPackage(evidence, projectRoot, source.packageName)) {
      errors.push({ evidenceId: evidence.evidenceId, file, package: evidence.package, declaredPackage: source.packageName, reason: 'package-mismatch' });
    }
    const matching = source.tests.filter(item => item.name === evidence.testFunction);
    if (matching.length === 0) {
      errors.push({ evidenceId: evidence.evidenceId, file, testFunction: evidence.testFunction, reason: 'missing-test-function' });
    } else if (matching.length > 1) {
      errors.push({ evidenceId: evidence.evidenceId, file, testFunction: evidence.testFunction, reason: 'duplicate-test-function' });
    } else if (matching[0].parameter !== '*testing.T' || matching[0].exported !== true) {
      errors.push({ evidenceId: evidence.evidenceId, file, testFunction: evidence.testFunction, reason: 'invalid-test-signature' });
    }
  }
  return { valid: errors.length === 0, errors };
}

function normalizeRuntimeEvidence(report) {
  const outcomes = report && Array.isArray(report.outcomes) ? report.outcomes : [];
  const groups = new Map();
  const errors = [];
  if (report !== null && report !== undefined && !Array.isArray(report.outcomes)) {
    errors.push({ reason: 'missing-runtime-outcomes' });
  }
  for (const item of outcomes) {
    if (
      !item ||
      typeof item.package !== 'string' ||
      !item.package.trim() ||
      typeof item.testFunction !== 'string' ||
      !item.testFunction.trim()
    ) {
      errors.push({ reason: 'malformed-runtime-identity' });
      continue;
    }
    const identity = `${item.package}::${item.testFunction}`;
    if (!VALID_RUNTIME_OUTCOMES.has(item.outcome)) {
      errors.push({ runtimeIdentity: identity, outcome: item.outcome, reason: 'invalid-runtime-outcome' });
    }
    const group = groups.get(identity) || [];
    group.push(item);
    groups.set(identity, group);
  }
  return { groups, errors };
}

function evaluateGoEvidence(evidenceItems, runtimeReport = null) {
  const { groups, errors } = normalizeRuntimeEvidence(runtimeReport);
  const schemaValid = runtimeReport !== null && runtimeReport.schema === GO_TEST_EVIDENCE_SCHEMA;
  if (runtimeReport !== null && !schemaValid) errors.push({ reason: 'invalid-runtime-schema' });
  const expectedGroups = new Map();
  for (const evidence of evidenceItems || []) {
    const identity = runtimeIdentity(evidence);
    const group = expectedGroups.get(identity) || [];
    group.push(evidence);
    expectedGroups.set(identity, group);
  }
  for (const [identity, group] of expectedGroups) {
    if (group.length > 1) {
      errors.push({ runtimeIdentity: identity, reason: 'duplicate-expected-runtime-identity' });
    }
    if (runtimeReport !== null && !groups.has(identity)) {
      errors.push({ runtimeIdentity: identity, reason: 'missing-runtime-identity' });
    }
  }
  for (const [identity, group] of groups) {
    if (!expectedGroups.has(identity)) errors.push({ runtimeIdentity: identity, reason: 'unexpected-runtime-identity' });
    if (group.length > 1) errors.push({ runtimeIdentity: identity, reason: 'duplicate-runtime-identity' });
  }
  const evaluations = (evidenceItems || []).map(evidence => {
    const identity = runtimeIdentity(evidence);
    const group = groups.get(identity) || [];
    const outcome = group.length === 1 && VALID_RUNTIME_OUTCOMES.has(group[0].outcome)
      ? group[0].outcome
      : 'unknown';
    const runtimePassed = schemaValid && errors.length === 0 && outcome === 'passed';
    return { ...evidence, runtimeIdentity: identity, runtimeOutcome: outcome, runtimePassed };
  });
  return { schemaValid, errors, evaluations, passed: evaluations.length > 0 && evaluations.every(item => item.runtimePassed) && errors.length === 0 };
}

module.exports = {
  GO_TEST_EVIDENCE_SCHEMA,
  VALID_EVIDENCE_ROLES,
  evaluateGoEvidence,
  getLayer,
  getModulePackage,
  hasCompatibleDeclaredPackage,
  inspectGoFile,
  inspectGoPackage,
  isSafeRepositoryPath,
  normalizeRepositoryPath,
  runtimeIdentity,
  validateGoEvidenceInventory
};
