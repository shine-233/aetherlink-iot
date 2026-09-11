'use strict';

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const coverageInspector = require('../coverage_inspector');
const { BUSINESS_OPERATIONS } = require('../coverage-contract/business-operations');

const ARCHIVE_SCHEMA = 'aetherlink.automation.archive.v1';
const REPORT_FILES = Object.freeze({
  summary: 'summary.json',
  endpointCoverage: 'endpoint-coverage.json',
  pageCoverage: 'page-coverage.json',
  goTestEvidence: 'go-test-evidence.json'
});
const VALID_EVIDENCE_KINDS = new Set([
  'synthetic',
  'local-real',
  'real-device',
  'target-deploy'
]);

function runGit(projectRoot, args, encoding = 'utf8') {
  const result = spawnSync('git', ['-C', projectRoot, ...args], {
    encoding,
    windowsHide: true,
    maxBuffer: 64 * 1024 * 1024
  });
  if (result.error || result.status !== 0) {
    throw new Error(`Git source inspection failed: ${String(result.stderr || result.error?.message || '').trim()}`);
  }
  return result.stdout;
}

function hashBufferParts(parts) {
  const hash = crypto.createHash('sha256');
  parts.forEach(part => hash.update(part));
  return hash.digest('hex');
}

function inspectSource(projectRoot) {
  const gitCommit = String(runGit(projectRoot, ['rev-parse', 'HEAD'])).trim();
  const gitRef = String(runGit(projectRoot, ['branch', '--show-current'])).trim() || null;
  const status = runGit(projectRoot, ['status', '--porcelain=v1', '-z', '--untracked-files=all'], null);
  const dirty = status.length > 0;
  if (!dirty) {
    return { gitCommit, gitRef, dirty: false, diffHash: null };
  }

  const diff = runGit(projectRoot, ['diff', '--binary', 'HEAD', '--', '.'], null);
  const untrackedParts = [];
  status.toString('utf8').split('\0').filter(Boolean).forEach(entry => {
    if (!entry.startsWith('?? ')) return;
    const relativePath = entry.slice(3);
    const absolutePath = path.resolve(projectRoot, relativePath);
    if (!absolutePath.startsWith(path.resolve(projectRoot) + path.sep) || !fs.existsSync(absolutePath)) return;
    const stat = fs.lstatSync(absolutePath);
    if (!stat.isFile()) return;
    untrackedParts.push(Buffer.from(`\0${relativePath}\0`, 'utf8'), fs.readFileSync(absolutePath));
  });

  return {
    gitCommit,
    gitRef,
    dirty: true,
    diffHash: hashBufferParts([status, diff, ...untrackedParts])
  };
}

function timestampForName(date) {
  return date.toISOString().replace(/[-:]/g, '').replace(/\..+$/, '').replace('T', '-');
}

function sanitizeMetadataValue(value) {
  if (typeof value !== 'string') return { value, redacted: false };
  let sanitized = value.replace(/:\/\/[^/\s:@]+:[^/\s@]+@/g, '://');
  sanitized = sanitized.replace(
    /(?:password|passwd|token|secret|api[_-]?key)=[^&\s]+/ig,
    '[redacted-credential]'
  );
  return { value: sanitized, redacted: sanitized !== value };
}

function sanitizeRunMetadata(command, environment) {
  let redactedCount = 0;
  const sanitizedCommand = (Array.isArray(command) ? command : []).map(item => {
    const sanitized = sanitizeMetadataValue(String(item));
    if (sanitized.redacted) redactedCount++;
    return sanitized.value;
  });
  const sanitizedEnvironment = Object.fromEntries(
    Object.entries(environment || {}).map(([key, value]) => {
      const sanitized = sanitizeMetadataValue(value);
      if (sanitized.redacted) redactedCount++;
      return [key, sanitized.value];
    })
  );
  return {
    command: sanitizedCommand,
    environment: sanitizedEnvironment,
    redaction: {
      secretsRemoved: true,
      notes: redactedCount > 0
        ? `${redactedCount} credential-like manifest value(s) were redacted.`
        : 'Manifest contains no account identities or secret values.'
    }
  };
}

function sameModuleSet(selected, discovered) {
  if (!Array.isArray(selected) || !Array.isArray(discovered) || selected.length !== discovered.length) {
    return false;
  }
  const identities = modules => modules.map(item => `${item.type}:${item.key}`).sort();
  return JSON.stringify(identities(selected)) === JSON.stringify(identities(discovered));
}

function classifyPlanScope(plan, suites) {
  const fullApi = sameModuleSet(plan.apiModulesToRun, suites.apiModules);
  const fullE2e = sameModuleSet(plan.e2eModulesToRun, suites.e2eModules);
  return fullApi && fullE2e && suites.apiModules.length > 0 && suites.e2eModules.length > 0
    ? 'full'
    : 'diagnostic';
}

function createRunArtifacts(options) {
  const startedAt = options.startedAt instanceof Date ? options.startedAt : new Date();
  const verificationRoot = path.resolve(options.verificationRoot);
  const stagingRoot = path.join(verificationRoot, '.staging');
  const token = crypto.randomBytes(5).toString('hex');
  const runName = `automation-run-${timestampForName(startedAt)}-${process.pid}-${token}`;
  const stagingDir = path.join(stagingRoot, runName);
  fs.mkdirSync(stagingRoot, { recursive: true });
  fs.mkdirSync(stagingDir, { recursive: false });
  const metadata = sanitizeRunMetadata(options.command, options.environment);

  return {
    runName,
    stagingDir,
    reportDir: stagingDir,
    verificationRoot,
    startedAt: startedAt.toISOString(),
    command: metadata.command,
    scope: options.scope,
    publicationRequested: options.publicationRequested === true,
    source: inspectSource(path.resolve(options.projectRoot)),
    environment: metadata.environment,
    redaction: metadata.redaction
  };
}

function sha256File(filePath) {
  return crypto.createHash('sha256').update(fs.readFileSync(filePath)).digest('hex');
}

function readJsonObject(filePath) {
  if (!fs.existsSync(filePath)) {
    throw new Error(`Required canonical report is missing: ${path.basename(filePath)}`);
  }
  let value;
  try {
    value = JSON.parse(fs.readFileSync(filePath, 'utf8'));
  } catch (error) {
    throw new Error(`Canonical report is not valid JSON (${path.basename(filePath)}): ${error.message}`);
  }
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`Canonical report is not an object: ${path.basename(filePath)}`);
  }
  return value;
}

function setSummaryIdentity(reportPath, source, verdict, startedAt, finishedAt) {
  const report = readJsonObject(reportPath);
  report.revision = source.gitCommit;
  report.verdict = verdict;
  report.startedAt = startedAt;
  report.finishedAt = finishedAt;
  fs.writeFileSync(reportPath, JSON.stringify(report, null, 2), 'utf8');
}

function buildReportIndex(reportDir, options = {}) {
  const required = options.requireAll === true;
  const reports = {};
  Object.entries(REPORT_FILES).forEach(([key, name]) => {
    const reportPath = path.join(reportDir, name);
    if (!fs.existsSync(reportPath)) {
      if (required) {
        throw new Error(`Required canonical report is missing: ${name}`);
      }
      return;
    }
    readJsonObject(reportPath);
    reports[key] = { path: name, sha256: sha256File(reportPath) };
  });
  return reports;
}

function normalizeCleanup(cleanup) {
  const status = cleanup && ['passed', 'failed', 'not-run'].includes(cleanup.status)
    ? cleanup.status
    : 'not-run';
  const notes = cleanup && Array.isArray(cleanup.notes)
    ? cleanup.notes.map(note => String(note))
    : [];
  return { status, notes };
}

function deriveCleanupEvidence(summary) {
  if (!summary || !Array.isArray(summary.caseOutcomes)) {
    return {
      status: 'not-run',
      notes: ['No canonical case outcomes were available to verify cleanup.']
    };
  }
  const cleanupCases = BUSINESS_OPERATIONS.flatMap(operation => (
    operation.cases
      .filter(item => Array.isArray(item.dimensions) && item.dimensions.includes('cleanup'))
      .map(item => `${item.file}::${item.title}`)
  ));
  if (cleanupCases.length === 0) {
    return { status: 'not-run', notes: ['No cleanup evidence cases are declared.'] };
  }
  const outcomes = new Map(summary.caseOutcomes.map(item => [
    `${item && item.file}::${item && item.title}`,
    item && item.outcome
  ]));
  const failed = cleanupCases.filter(identity => outcomes.get(identity) === 'failed');
  const incomplete = cleanupCases.filter(identity => outcomes.get(identity) !== 'passed');
  if (failed.length > 0) {
    return { status: 'failed', notes: [`${failed.length} cleanup evidence case(s) failed.`] };
  }
  if (incomplete.length > 0) {
    return { status: 'not-run', notes: [`${incomplete.length} cleanup evidence case(s) lack passing runtime evidence.`] };
  }
  return { status: 'passed', notes: [`${cleanupCases.length} declared cleanup evidence case(s) passed.`] };
}

function validateManifestForPublication(manifest) {
  const errors = [];
  if (manifest.schema !== ARCHIVE_SCHEMA) errors.push('invalid schema');
  if (manifest.kind !== 'automation-coverage') errors.push('invalid kind');
  if (manifest.scope !== 'full') errors.push('scope is not full');
  if (manifest.run.exitCode !== 0) errors.push('run exit code is not zero');
  if (manifest.run.strictIntegration !== true) errors.push('strict integration is disabled');
  if (manifest.source.dirty !== false) errors.push('source worktree is dirty');
  if (manifest.verdict !== 'passed') errors.push('verdict is not passed');
  if (manifest.cleanup.status !== 'passed') errors.push('cleanup is incomplete');
  if (!VALID_EVIDENCE_KINDS.has(manifest.environment.evidenceKind)) errors.push('invalid evidence kind');
  for (const key of Object.keys(REPORT_FILES)) {
    if (!manifest.reports[key]) errors.push(`missing report: ${key}`);
  }
  return errors;
}

function validateArchiveCandidate(archiveDir, run, finishedAt) {
  const classified = coverageInspector.classifyArchive(archiveDir, {
    git: {
      revision: run.source.gitCommit,
      branch: run.source.gitRef,
      dirty: run.source.dirty
    },
    now: finishedAt,
    maxAgeMs: Number.POSITIVE_INFINITY,
    futureSkewMs: 5 * 60 * 1000,
    archiveTimestampSkewMs: 5 * 60 * 1000,
    requiredEvidenceKind: null
  });
  if (!classified.eligible) {
    return classified.reasonCodes.map(code => `inspector: ${code}`);
  }
  const evaluation = coverageInspector.evaluateArchive(classified);
  return evaluation.checks
    .filter(item => item.status !== 'pass')
    .map(item => `inspector: ${item.reasonCode || item.id}`);
}

function remapResultPaths(result, fromDir, toDir) {
  const remap = value => (
    typeof value === 'string' && path.resolve(value).startsWith(path.resolve(fromDir) + path.sep)
      ? path.join(toDir, path.relative(fromDir, value))
      : value
  );
  return {
    ...result,
    reportDir: toDir,
    reports: Object.fromEntries(
      Object.entries(result.reports || {}).map(([key, value]) => [key, remap(value)])
    )
  };
}

function writeInterruptedManifest(run, result = {}) {
  const finishedAt = result.finishedAt instanceof Date ? result.finishedAt : new Date();
  const exitCode = Number.isInteger(result.exitCode) ? result.exitCode : 1;
  const cleanup = normalizeCleanup(result.cleanup);
  const reports = buildReportIndex(run.reportDir, { requireAll: false });
  const manifest = {
    schema: ARCHIVE_SCHEMA,
    kind: 'automation-coverage',
    archivedAt: finishedAt.toISOString(),
    scope: run.scope,
    run: {
      startedAt: run.startedAt,
      finishedAt: finishedAt.toISOString(),
      command: run.command,
      exitCode,
      strictIntegration: result.strictIntegration === true
    },
    source: run.source,
    environment: run.environment,
    reports,
    cleanup,
    redaction: run.redaction || {
      secretsRemoved: true,
      notes: 'Manifest contains no account identities or secret values.'
    },
    verdict: 'failed',
    blockingGaps: [...new Set(Array.isArray(result.blockingGaps) ? result.blockingGaps : ['interrupted-run'])]
  };
  fs.writeFileSync(
    path.join(run.stagingDir, 'archive-manifest.json'),
    JSON.stringify(manifest, null, 2),
    'utf8'
  );
  return {
    archiveDir: null,
    diagnosticDir: run.stagingDir,
    reportDir: run.stagingDir,
    reports: Object.fromEntries(
      Object.entries(reports).map(([key, descriptor]) => [key, path.join(run.stagingDir, descriptor.path)])
    ),
    manifest,
    publicationErrors: ['run did not reach normal finalization']
  };
}

function finalizeRunArtifacts(run, result) {
  const finishedAt = result.finishedAt instanceof Date ? result.finishedAt : new Date();
  const finishedAtIso = finishedAt.toISOString();
  const strictIntegration = result.strictIntegration === true;
  const cleanup = normalizeCleanup(result.cleanup);
  const blockingGaps = Array.isArray(result.blockingGaps) ? [...result.blockingGaps] : [];
  let verdict = result.exitCode === 0 && strictIntegration && cleanup.status === 'passed' && blockingGaps.length === 0
    ? 'passed'
    : result.exitCode === 0 ? 'blocked' : 'failed';

  if (run.scope !== 'full') {
    verdict = result.exitCode === 0 ? 'blocked' : 'failed';
    blockingGaps.push('filtered-or-partial-run');
  }
  if (run.source.dirty) {
    if (verdict === 'passed') verdict = 'blocked';
    blockingGaps.push('dirty-source');
  }

  const summaryPath = path.join(run.reportDir, REPORT_FILES.summary);
  setSummaryIdentity(summaryPath, run.source, verdict, run.startedAt, finishedAtIso);
  const reports = buildReportIndex(run.reportDir, { requireAll: run.scope === 'full' });
  const manifest = {
    schema: ARCHIVE_SCHEMA,
    kind: 'automation-coverage',
    archivedAt: finishedAtIso,
    scope: run.scope,
    run: {
      startedAt: run.startedAt,
      finishedAt: finishedAtIso,
      command: run.command,
      exitCode: result.exitCode,
      strictIntegration
    },
    source: run.source,
    environment: run.environment,
    reports,
    cleanup,
    redaction: run.redaction || {
      secretsRemoved: true,
      notes: 'Manifest contains no account identities or secret values.'
    },
    verdict,
    blockingGaps: [...new Set(blockingGaps)]
  };

  const publicationErrors = validateManifestForPublication(manifest);
  const manifestPath = path.join(run.stagingDir, 'archive-manifest.json');
  fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2), 'utf8');
  if (run.publicationRequested && publicationErrors.length === 0) {
    publicationErrors.push(...validateArchiveCandidate(run.stagingDir, run, finishedAt));
  }
  const canPublish = run.publicationRequested && publicationErrors.length === 0;

  const resultPaths = {
    reportDir: run.stagingDir,
    reports: Object.fromEntries(
      Object.entries(reports).map(([key, descriptor]) => [key, path.join(run.stagingDir, descriptor.path)])
    )
  };
  if (!canPublish) {
    return {
      archiveDir: null,
      diagnosticDir: run.stagingDir,
      manifest,
      publicationErrors,
      ...resultPaths
    };
  }

  const archiveDir = path.join(run.verificationRoot, run.runName);
  if (fs.existsSync(archiveDir)) {
    throw new Error(`Archive destination already exists: ${archiveDir}`);
  }
  fs.renameSync(run.stagingDir, archiveDir);
  return {
    archiveDir,
    diagnosticDir: null,
    manifest,
    publicationErrors: [],
    ...remapResultPaths(resultPaths, run.stagingDir, archiveDir)
  };
}

module.exports = {
  ARCHIVE_SCHEMA,
  REPORT_FILES,
  VALID_EVIDENCE_KINDS,
  sanitizeRunMetadata,
  inspectSource,
  classifyPlanScope,
  createRunArtifacts,
  deriveCleanupEvidence,
  writeInterruptedManifest,
  finalizeRunArtifacts,
  sha256File
};
