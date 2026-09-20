'use strict';

const crypto = require('crypto');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const coverageContract = require('./coverage_contract');
const { evaluateGoEvidence } = require('./go-test-evidence');

const INSPECTOR_SCHEMA = 'aetherlink.coverage.inspector.v1';
const ARCHIVE_SCHEMA = 'aetherlink.automation.archive.v1';
const REQUIRED_ROOT_ENTRIES = [
  'AGENTS.md',
  'frontend',
  'backend',
  'mqtt-broker',
  'automation_tests'
];
const REPORT_NAMES = {
  summary: 'summary.json',
  endpointCoverage: 'endpoint-coverage.json',
  pageCoverage: 'page-coverage.json',
  goTestEvidence: 'go-test-evidence.json'
};
const EVIDENCE_KINDS = new Set([
  'synthetic',
  'local-real',
  'real-device',
  'target-deploy'
]);

function check(id, status, reasonCode, message, severity = 'blocking') {
  return { id, status, severity, reasonCode: reasonCode || null, message, evidenceRefs: [] };
}

function parseTimestamp(value) {
  if (typeof value !== 'string' || !value.trim()) return null;
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? new Date(timestamp) : null;
}

function parseRate(value) {
  if (typeof value === 'string') value = value.trim().replace(/%$/, '');
  const rate = Number(value);
  return Number.isFinite(rate) && rate >= 0 && rate <= 100 ? rate : null;
}

function sha256File(filePath) {
  return crypto.createHash('sha256').update(fs.readFileSync(filePath)).digest('hex');
}

function normalizeRelativePath(value) {
  if (typeof value !== 'string' || !value.trim() || path.isAbsolute(value)) return null;
  const normalized = path.posix.normalize(value.replace(/\\/g, '/'));
  if (normalized === '..' || normalized.startsWith('../')) return null;
  return normalized;
}

function readJson(filePath) {
  try {
    return { ok: true, value: JSON.parse(fs.readFileSync(filePath, 'utf8')) };
  } catch (error) {
    return { ok: false, error: error.message };
  }
}

function git(projectRoot, args) {
  const result = spawnSync('git', ['-C', projectRoot, ...args], {
    encoding: 'utf8',
    windowsHide: true
  });
  return {
    ok: !result.error && result.status === 0,
    status: result.status,
    stdout: String(result.stdout || '').trim(),
    stderr: String(result.stderr || result.error?.message || '').trim()
  };
}

function inspectProjectRoot(projectRoot) {
  const missingEntries = REQUIRED_ROOT_ENTRIES.filter(entry => !fs.existsSync(path.join(projectRoot, entry)));
  if (missingEntries.length > 0) {
    return {
      valid: false,
      git: null,
      check: check(
        'project-root',
        'fail',
        'invalid-project-root',
        'Project root is missing required entries: ' + missingEntries.join(', ')
      )
    };
  }

  const topLevel = git(projectRoot, ['rev-parse', '--show-toplevel']);
  if (!topLevel.ok) {
    return {
      valid: false,
      git: null,
      check: check('project-root', 'unknown', 'git-unavailable', 'Git root could not be verified: ' + topLevel.stderr)
    };
  }

  if (path.resolve(topLevel.stdout) !== path.resolve(projectRoot)) {
    return {
      valid: false,
      git: null,
      check: check('project-root', 'fail', 'not-git-root', 'Project root is not the Git top-level directory')
    };
  }

  const revision = git(projectRoot, ['rev-parse', 'HEAD']);
  const branch = git(projectRoot, ['branch', '--show-current']);
  const dirty = git(projectRoot, ['status', '--porcelain=v1']);
  if (!revision.ok || !branch.ok || !dirty.ok) {
    return {
      valid: false,
      git: null,
      check: check('project-root', 'unknown', 'git-inspection-failed', 'Git revision or worktree state could not be read')
    };
  }

  return {
    valid: true,
    git: {
      root: topLevel.stdout,
      revision: revision.stdout,
      branch: branch.stdout || null,
      dirty: dirty.stdout.length > 0
    },
    check: check('project-root', 'pass', null, 'Project identity and Git root are valid')
  };
}

function emptyInputs() {
  return {
    verificationDirectory: { status: 'not-evaluated', path: null },
    archivesScanned: 0,
    canonicalArchives: 0,
    legacyArchives: 0,
    invalidArchives: 0
  };
}

function listArchiveDirectories(verificationRoot) {
  if (!fs.existsSync(verificationRoot)) return [];
  return fs.readdirSync(verificationRoot, { withFileTypes: true })
    .filter(entry => entry.isDirectory() && !['templates', '.staging'].includes(entry.name))
    .map(entry => path.join(verificationRoot, entry.name));
}

function resolveReportPath(archiveDir, reportReference) {
  const normalized = normalizeRelativePath(reportReference);
  if (!normalized) return null;
  const resolved = path.resolve(archiveDir, normalized);
  const relative = path.relative(archiveDir, resolved);
  if (relative === '..' || relative.startsWith('..' + path.sep) || path.isAbsolute(relative)) return null;
  return resolved;
}

function reportReferences(manifest) {
  if (!manifest || typeof manifest !== 'object' || !manifest.reports || typeof manifest.reports !== 'object') {
    return null;
  }
  return {
    summary: manifest.reports.summary || manifest.reports.summary_json || null,
    endpointCoverage: manifest.reports.endpointCoverage || manifest.reports.endpoint_coverage_json || null,
    pageCoverage: manifest.reports.pageCoverage || manifest.reports.page_coverage_json || null,
    goTestEvidence: manifest.reports.goTestEvidence || manifest.reports.go_test_evidence_json || null
  };
}

function canonicalReportDescriptor(reference) {
  if (typeof reference === 'string') return { path: reference, sha256: null };
  if (reference && typeof reference === 'object') {
    return { path: reference.path, sha256: reference.sha256 || null };
  }
  return { path: null, sha256: null };
}

function containsCredential(value) {
  if (typeof value !== 'string') return false;
  return /:\/\/[^/\s:@]+:[^/\s@]+@/.test(value) || /(?:password|passwd|token|secret|api[_-]?key)=/i.test(value);
}

function containsSensitiveManifestValues(manifest) {
  const runCommand = Array.isArray(manifest.run?.command) ? manifest.run.command : [];
  const environmentValues = [
    manifest.environment?.frontendUrl,
    manifest.environment?.backendUrl,
    manifest.environment?.mqttAddress,
    manifest.environment?.database,
    manifest.environment?.accountSource
  ];
  return [...runCommand, ...environmentValues].some(containsCredential);
}

function classifyArchive(archiveDir, options) {
  const manifestPath = path.join(archiveDir, 'archive-manifest.json');
  const directReports = Object.fromEntries(
    Object.entries(REPORT_NAMES).map(([key, name]) => [key, path.join(archiveDir, name)])
  );
  const hasDirectReports = Object.values(directReports).filter(fs.existsSync);
  if (!fs.existsSync(manifestPath)) {
    return {
      directory: archiveDir,
      compatibility: hasDirectReports.length > 0 ? 'unknown' : 'unknown',
      validity: hasDirectReports.length > 0 ? 'invalid' : 'artifact-only',
      eligible: false,
      reasonCodes: [hasDirectReports.length > 0 ? 'reports-without-manifest' : 'no-coverage-artifacts'],
      timestamp: null,
      revision: null,
      reports: null,
      manifest: null
    };
  }

  const parsedManifest = readJson(manifestPath);
  if (!parsedManifest.ok || !parsedManifest.value || typeof parsedManifest.value !== 'object') {
    return {
      directory: archiveDir,
      compatibility: 'unknown',
      validity: 'invalid',
      eligible: false,
      reasonCodes: ['malformed-manifest'],
      timestamp: null,
      revision: null,
      reports: null,
      manifest: null
    };
  }

  const manifest = parsedManifest.value;
  const compatibility = manifest.schema === ARCHIVE_SCHEMA ? 'canonical' : 'legacy';
  if (compatibility === 'legacy') {
    return {
      directory: archiveDir,
      compatibility,
      validity: reportReferences(manifest) ? 'legacy-structured' : 'legacy-manifest-only',
      eligible: false,
      reasonCodes: ['legacy-manifest'],
      timestamp: parseTimestamp(manifest.finishedAt || manifest.archivedAt || manifest.timestamp),
      revision: manifest.revision || manifest.git_commit || null,
      reports: null,
      manifest
    };
  }

  const reasons = [];
  const outcomeReasons = [];
  const run = manifest.run && typeof manifest.run === 'object' ? manifest.run : {};
  const source = manifest.source && typeof manifest.source === 'object' ? manifest.source : {};
  const environment = manifest.environment && typeof manifest.environment === 'object'
    ? manifest.environment
    : {};
  const timestamp = parseTimestamp(run.finishedAt);
  const startedAt = parseTimestamp(run.startedAt);
  const archivedAt = parseTimestamp(manifest.archivedAt);
  if (manifest.kind !== 'automation-coverage') reasons.push('invalid-archive-kind');
  if (manifest.scope !== 'full') reasons.push('non-full-scope');
  if (!startedAt) reasons.push('invalid-started-at');
  if (!timestamp) reasons.push('invalid-finished-at');
  if (!archivedAt) reasons.push('invalid-archived-at');
  if (startedAt && timestamp && timestamp < startedAt) reasons.push('invalid-run-interval');
  if (timestamp && archivedAt && Math.abs(archivedAt - timestamp) > options.archiveTimestampSkewMs) {
    reasons.push('archive-timestamp-mismatch');
  }
  const now = options.now;
  if (timestamp && timestamp.getTime() > now.getTime() + options.futureSkewMs) reasons.push('future-timestamp');
  if (timestamp && now.getTime() - timestamp.getTime() > options.maxAgeMs) reasons.push('stale-archive');
  if (!/^[0-9a-f]{40}$/.test(source.gitCommit || '')) reasons.push('missing-revision');
  if (source.gitCommit && source.gitCommit !== options.git.revision) reasons.push('revision-mismatch');
  if (source.dirty !== false) reasons.push('dirty-archive-source');
  if (!Number.isInteger(run.exitCode)) reasons.push('missing-exit-code');
  else if (run.exitCode !== 0) outcomeReasons.push('run-failed');
  if (run.strictIntegration !== true) reasons.push('non-strict-integration');
  if (!['passed', 'failed', 'blocked'].includes(manifest.verdict)) reasons.push('invalid-verdict');
  else if (manifest.verdict !== 'passed') outcomeReasons.push('non-pass-verdict');
  if (!EVIDENCE_KINDS.has(environment.evidenceKind)) reasons.push('invalid-evidence-kind');
  if (options.requiredEvidenceKind && environment.evidenceKind !== options.requiredEvidenceKind) {
    outcomeReasons.push('insufficient-evidence-kind');
  }
  if (!Array.isArray(run.command) || run.command.length === 0) reasons.push('missing-command');
  if (manifest.redaction?.secretsRemoved !== true) reasons.push('redaction-not-confirmed');
  if (containsSensitiveManifestValues(manifest)) reasons.push('manifest-contains-credentials');
  if (!['passed', 'failed', 'not-run'].includes(manifest.cleanup?.status)) reasons.push('invalid-cleanup-status');
  else if (manifest.cleanup.status !== 'passed') outcomeReasons.push('cleanup-incomplete');

  const references = reportReferences(manifest);
  if (!references) reasons.push('missing-report-index');
  const reports = {};
  for (const reportKey of Object.keys(REPORT_NAMES)) {
    const descriptor = canonicalReportDescriptor(references?.[reportKey]);
    const reportPath = resolveReportPath(archiveDir, descriptor.path);
    if (!reportPath) {
      reasons.push('invalid-report-path:' + reportKey);
      continue;
    }
    if (!fs.existsSync(reportPath)) {
      reasons.push('missing-report:' + reportKey);
      continue;
    }
    if (!/^[0-9a-f]{64}$/.test(descriptor.sha256 || '')) {
      reasons.push('missing-report-hash:' + reportKey);
      continue;
    }
    if (descriptor.sha256 !== sha256File(reportPath)) {
      reasons.push('report-hash-mismatch:' + reportKey);
      continue;
    }
    const parsed = readJson(reportPath);
    if (!parsed.ok || !parsed.value || typeof parsed.value !== 'object') {
      reasons.push('malformed-report:' + reportKey);
      continue;
    }
    reports[reportKey] = { path: reportPath, value: parsed.value, sha256: sha256File(reportPath) };
  }

  return {
    directory: archiveDir,
    compatibility,
    validity: reasons.length === 0 ? 'canonical' : 'invalid',
    eligible: reasons.length === 0,
    runOutcome: outcomeReasons.length === 0 ? 'passed' : 'not-passed',
    reasonCodes: [...reasons, ...outcomeReasons],
    integrityReasonCodes: reasons,
    outcomeReasonCodes: outcomeReasons,
    timestamp,
    revision: source.gitCommit || null,
    reports,
    manifest
  };
}

function getBusinessFlowStats(pageReport) {
  const candidate = pageReport?.businessFlows || pageReport?.businessFlowCoverage || null;
  if (!candidate || typeof candidate !== 'object') return null;
  const total = Number(candidate.total);
  const covered = Number(candidate.covered);
  const uncovered = Number(candidate.uncovered);
  const rate = parseRate(candidate.rate);
  if (![total, covered, uncovered].every(Number.isFinite) || rate === null) return null;
  if (total < 0 || covered < 0 || uncovered < 0 || covered + uncovered !== total) return null;
  return { total, covered, uncovered, rate };
}

function getReachabilityStats(report) {
  if (!report || typeof report !== 'object') return null;
  const total = Number(report.total);
  const covered = Number(report.covered);
  const uncovered = Number(report.uncovered);
  const rate = parseRate(report.rate);
  if (![total, covered, uncovered].every(Number.isFinite) || rate === null) return null;
  if (total < 0 || covered < 0 || uncovered < 0 || covered + uncovered !== total) return null;
  return { total, covered, uncovered, rate };
}

function validateReportInterval(report, manifest) {
  const reportStartedAt = parseTimestamp(report?.startedAt || report?.startTime || report?.summary?.startTime);
  const reportFinishedAt = parseTimestamp(report?.finishedAt || report?.endTime || report?.summary?.endTime);
  if (!reportStartedAt && !reportFinishedAt) return 'missing';
  if (!reportStartedAt || !reportFinishedAt || reportFinishedAt < reportStartedAt) return 'invalid';
  const runStartedAt = parseTimestamp(manifest.run?.startedAt);
  const runFinishedAt = parseTimestamp(manifest.run?.finishedAt);
  if (!runStartedAt || !runFinishedAt) return 'invalid';
  const tolerance = 5 * 60 * 1000;
  if (reportStartedAt < new Date(runStartedAt.getTime() - tolerance)) return 'mismatch';
  if (reportFinishedAt > new Date(runFinishedAt.getTime() + tolerance)) return 'mismatch';
  return 'valid';
}

function canonicalSummaryDocument(value) {
  if (!value || typeof value !== 'object') return null;
  const nested = value.summary && typeof value.summary === 'object'
    ? value.summary
    : value;
  return {
    ...nested,
    revision: value.revision ?? nested.revision,
    verdict: value.verdict ?? nested.verdict,
    startedAt: value.startedAt ?? nested.startedAt,
    finishedAt: value.finishedAt ?? nested.finishedAt,
    moduleOutcomes: value.moduleOutcomes ?? nested.moduleOutcomes,
    caseOutcomes: value.caseOutcomes ?? nested.caseOutcomes
  };
}

function evaluateArchive(archive) {
  if (!archive.eligible) {
    return {
      checks: [check('archive-integrity', 'fail', archive.reasonCodes[0] || 'invalid-archive', 'Archive is not eligible')],
      metrics: null,
      ready: false
    };
  }

  const summary = canonicalSummaryDocument(archive.reports.summary.value);
  const endpoint = getReachabilityStats(archive.reports.endpointCoverage.value);
  const pageRoot = archive.reports.pageCoverage.value;
  const goReport = archive.reports.goTestEvidence.value;
  const goEvaluation = evaluateGoEvidence(coverageContract.ALL_GO_EVIDENCE, goReport);
  const pages = getReachabilityStats(pageRoot.pages || pageRoot);
  const businessFlows = getBusinessFlowStats(pageRoot);
  const checks = [check('archive-integrity', 'pass', null, 'Canonical archive provenance is valid')];
  if (archive.outcomeReasonCodes.length > 0) {
    checks.push(check(
      'run-outcome',
      'fail',
      archive.outcomeReasonCodes[0],
      'The newest current-revision run did not complete successfully'
    ));
  } else {
    checks.push(check('run-outcome', 'pass', null, 'The archived run and cleanup completed successfully'));
  }

  const reportIntervals = Object.values(archive.reports).map(report =>
    validateReportInterval(report.value, archive.manifest)
  );
  if (reportIntervals.includes('invalid') || reportIntervals.includes('mismatch')) {
    checks.push(check('report-intervals', 'fail', 'report-interval-mismatch', 'Report timestamps contradict the run interval'));
  } else if (reportIntervals.includes('missing')) {
    checks.push(check('report-intervals', 'unknown', 'missing-report-timestamps', 'One or more reports lack run timestamps'));
  } else {
    checks.push(check('report-intervals', 'pass', null, 'Report timestamps agree with the run interval'));
  }

  if (goReport.revision !== archive.manifest.source.gitCommit) {
    checks.push(check('go-test-evidence', 'fail', 'go-evidence-revision-mismatch', 'Go test evidence revision contradicts the manifest'));
  } else if (
    !goReport.commandPolicy ||
    goReport.commandPolicy.json !== true ||
    goReport.commandPolicy.count !== 1 ||
    goReport.commandPolicy.cacheDisabled !== true
  ) {
    checks.push(check('go-test-evidence', 'fail', 'invalid-go-command-policy', 'Go test evidence does not prove uncached JSON execution'));
  } else if (goReport.source?.dirty !== archive.manifest.source.dirty) {
    checks.push(check('go-test-evidence', 'fail', 'go-evidence-source-mismatch', 'Go test evidence source state contradicts the manifest'));
  } else if (Array.isArray(goReport.errors) && goReport.errors.length > 0) {
    checks.push(check('go-test-evidence', 'fail', 'go-runtime-errors', 'Go test evidence contains runtime collection errors'));
  } else if (!goEvaluation.passed || goReport.passed !== true) {
    checks.push(check('go-test-evidence', 'fail', 'incomplete-go-test-evidence', 'Exact Go runtime evidence is incomplete or non-passing'));
  } else {
    checks.push(check('go-test-evidence', 'pass', null, 'Every declared exact Go identity passed in the archived run'));
  }

  if (!endpoint) checks.push(check('endpoint-report', 'fail', 'invalid-endpoint-report', 'Endpoint report totals are invalid'));
  else checks.push(check('endpoint-report', 'pass', null, 'Endpoint reachability report is structurally valid'));
  if (!pages) checks.push(check('page-report', 'fail', 'invalid-page-report', 'Page route-render report totals are invalid'));
  else checks.push(check('page-report', 'pass', null, 'Page route-render report is structurally valid'));
  if (!businessFlows) {
    checks.push(check('business-flow-report', 'unknown', 'missing-business-flow-evidence', 'No valid authored browser business-flow evidence is present'));
  } else if (businessFlows.total === 0 || businessFlows.covered < businessFlows.total) {
    checks.push(check('business-flow-report', 'fail', 'incomplete-business-flow-evidence', 'Authored browser business-flow evidence is incomplete'));
  } else {
    checks.push(check('business-flow-report', 'pass', null, 'All declared browser business flows have evidence'));
  }

  if (!summary) {
    checks.push(check('case-outcomes', 'fail', 'invalid-summary-report', 'Summary report has no canonical summary document'));
  } else {
    const outcomes = Array.isArray(summary.moduleOutcomes) ? summary.moduleOutcomes : [];
    if (outcomes.some(item => ['failed', 'partial-skip', 'all-skipped', 'skipped'].includes(item?.outcome))) {
      checks.push(check('case-outcomes', 'fail', 'incomplete-module-outcomes', 'Summary includes failed or skipped module outcomes'));
    } else if (!Array.isArray(summary.caseOutcomes) || summary.caseOutcomes.length === 0) {
      checks.push(check('case-outcomes', 'unknown', 'missing-case-outcomes', 'Summary has no case-level runtime outcomes'));
    } else {
      const operationTraceability = coverageContract.getOperationTraceability(
        coverageContract.BUSINESS_OPERATIONS,
        summary.caseOutcomes
      );
      const runtimeErrors = operationTraceability.flatMap(item => item.runtimeOutcomeErrors || []);
      const failedOperations = operationTraceability.filter(item => item.runtimeStatus === 'failed');
      const incompleteRuntimeOperations = operationTraceability.filter(item => item.runtimeStatus !== 'passed');
      const incompleteStaticOperations = operationTraceability.filter(item => (
        item.staticInventoryValid !== true || item.missingDimensions.length > 0
      ));
      if (runtimeErrors.length > 0) {
        checks.push(check('case-outcomes', 'fail', 'invalid-operation-case-outcomes', 'Case outcomes contain malformed, duplicate, or unexpected operation identities'));
      } else if (failedOperations.length > 0) {
        checks.push(check('case-outcomes', 'fail', 'failed-operation-case-outcomes', 'One or more canonical operation cases failed'));
      } else if (incompleteRuntimeOperations.length > 0) {
        checks.push(check('case-outcomes', 'unknown', 'incomplete-operation-case-outcomes', 'Case outcomes do not pass every canonical operation case'));
      } else if (incompleteStaticOperations.length > 0) {
        checks.push(check('case-outcomes', 'unknown', 'incomplete-operation-static-evidence', 'Canonical operation cases passed, but required static inventory or dimensions remain incomplete'));
      } else if (operationTraceability.some(item => item.ready !== true)) {
        checks.push(check('case-outcomes', 'unknown', 'incomplete-operation-readiness', 'Canonical operations are not fully ready'));
      } else {
        checks.push(check('case-outcomes', 'pass', null, 'Every canonical operation is statically complete and has exact passing runtime cases'));
      }
    }
  }

  if (summary && summary.revision && summary.revision !== archive.manifest.source.gitCommit) {
    checks.push(check('summary-consistency', 'fail', 'summary-revision-mismatch', 'Summary revision contradicts the manifest'));
  } else if (summary && summary.verdict && summary.verdict !== archive.manifest.verdict) {
    checks.push(check('summary-consistency', 'fail', 'summary-verdict-mismatch', 'Summary verdict contradicts the manifest'));
  } else {
    checks.push(check('summary-consistency', 'pass', null, 'Summary does not contradict the manifest'));
  }

  return {
    checks,
    metrics: { endpointReachability: endpoint, routeRender: pages, businessFlows },
    ready: checks.every(item => item.status === 'pass')
  };
}

function selectArchive(archives, expectedRevision = null) {
  const structurallyValid = archives
    .filter(archive => archive.eligible)
    .sort((a, b) => b.timestamp.getTime() - a.timestamp.getTime());
  if (structurallyValid.length === 0) {
    return { status: 'none', archive: null, rejectedArchive: null, reasonCode: 'no-eligible-archive' };
  }

  const revision = expectedRevision || structurallyValid[0].revision;
  const currentCanonical = archives
    .filter(archive => (
      archive.compatibility === 'canonical' &&
      archive.revision === revision &&
      archive.timestamp instanceof Date
    ))
    .sort((a, b) => b.timestamp.getTime() - a.timestamp.getTime());
  const newestValid = structurallyValid[0];
  const newestCandidate = currentCanonical[0] || newestValid;
  const latestTimestamp = newestCandidate.timestamp.getTime();
  const tiedLatest = currentCanonical.filter(archive => archive.timestamp.getTime() === latestTimestamp);

  if (tiedLatest.length > 1) {
    return { status: 'ambiguous', archive: null, rejectedArchive: null, reasonCode: 'ambiguous-latest-archive' };
  }
  if (newestCandidate !== newestValid && latestTimestamp > newestValid.timestamp.getTime()) {
    return {
      status: 'invalid-latest',
      archive: null,
      rejectedArchive: newestCandidate,
      reasonCode: 'latest-canonical-archive-invalid'
    };
  }
  return { status: 'selected', archive: newestValid, rejectedArchive: null, reasonCode: null };
}

function inspect(options = {}) {
  const projectRoot = path.resolve(options.projectRoot || path.resolve(__dirname, '..', '..'));
  const now = options.now instanceof Date ? options.now : new Date();
  const rootInspection = inspectProjectRoot(projectRoot);
  const result = {
    schema: INSPECTOR_SCHEMA,
    generatedAt: now.toISOString(),
    project: {
      root: projectRoot,
      identity: rootInspection.valid ? 'aetherlink-iot' : 'unknown',
      options: {
        requiredEvidenceKind: options.requiredEvidenceKind || null
      },
      git: rootInspection.git
    },
    inputs: emptyInputs(),
    archiveSelection: { status: 'none', reasonCode: null, selectedPath: null },
    archives: [],
    metrics: null,
    checks: [rootInspection.check],
    readiness: { status: 'unknown', blockingReasons: [] }
  };

  if (!rootInspection.valid) {
    result.readiness.blockingReasons = [rootInspection.check.reasonCode];
    return result;
  }

  if (options.requireCleanCurrentWorktree && rootInspection.git.dirty) {
    const dirtyCheck = check(
      'current-worktree',
      'fail',
      'current-worktree-dirty',
      'Current worktree is dirty and cannot be compared to a clean archived source'
    );
    result.checks.push(dirtyCheck);
  }
  const verificationRoot = path.resolve(options.verificationRoot || path.join(projectRoot, 'verification'));
  result.inputs.verificationDirectory = {
    status: fs.existsSync(verificationRoot) ? 'present' : 'missing',
    path: verificationRoot
  };
  if (!fs.existsSync(verificationRoot)) {
    const missingCheck = check('verification-directory', 'unknown', 'missing-verification-directory', 'Verification directory is missing');
    result.checks.push(missingCheck);
    result.readiness.blockingReasons = [missingCheck.reasonCode];
    return result;
  }

  result.checks.push(check('verification-directory', 'pass', null, 'Verification directory is present'));
  const archiveOptions = {
    git: rootInspection.git,
    now,
    maxAgeMs: options.maxAgeMs ?? 7 * 24 * 60 * 60 * 1000,
    futureSkewMs: options.futureSkewMs ?? 5 * 60 * 1000,
    archiveTimestampSkewMs: options.archiveTimestampSkewMs ?? 5 * 60 * 1000,
    requiredEvidenceKind: options.requiredEvidenceKind || null
  };
  const archives = listArchiveDirectories(verificationRoot)
    .map(directory => classifyArchive(directory, archiveOptions));
  result.archives = archives.map(archive => ({
    directory: archive.directory,
    compatibility: archive.compatibility,
    validity: archive.validity,
    eligible: archive.eligible,
    reasonCodes: archive.reasonCodes,
    timestamp: archive.timestamp ? archive.timestamp.toISOString() : null,
    revision: archive.revision
  }));
  result.inputs.archivesScanned = archives.length;
  result.inputs.canonicalArchives = archives.filter(item => item.validity === 'canonical').length;
  result.inputs.legacyArchives = archives.filter(item => item.compatibility === 'legacy').length;
  result.inputs.invalidArchives = archives.filter(item => item.validity === 'invalid').length;

  const selection = selectArchive(archives, rootInspection.git.revision);
  result.archiveSelection = {
    status: selection.status,
    reasonCode: selection.reasonCode,
    selectedPath: selection.archive?.directory || null,
    rejectedPath: selection.rejectedArchive?.directory || null
  };
  if (!selection.archive) {
    const selectionCheck = check('archive-selection', 'unknown', selection.reasonCode, 'No unambiguous current canonical archive is eligible');
    result.checks.push(selectionCheck);
    result.readiness.status = result.checks.some(item => item.status === 'fail') ? 'fail' : 'unknown';
    result.readiness.blockingReasons = result.checks
      .filter(item => item.status !== 'pass' && item.severity === 'blocking')
      .map(item => item.reasonCode)
      .filter(Boolean);
    return result;
  }

  result.checks.push(check('archive-selection', 'pass', null, 'A current canonical archive was selected'));
  const evaluation = evaluateArchive(selection.archive);
  result.checks.push(...evaluation.checks);
  result.metrics = evaluation.metrics;
  const allBlockingChecksPass = result.checks.every(item => item.severity !== 'blocking' || item.status === 'pass');
  result.readiness.status = evaluation.ready && allBlockingChecksPass ? 'pass' : 'fail';
  result.readiness.blockingReasons = result.checks
    .filter(item => item.status !== 'pass' && item.severity === 'blocking')
    .map(item => item.reasonCode)
    .filter(Boolean);
  return result;
}

function exitCode(result) {
  return result.readiness.status === 'pass' ? 0 : 1;
}

module.exports = {
  ARCHIVE_SCHEMA,
  INSPECTOR_SCHEMA,
  classifyArchive,
  evaluateArchive,
  exitCode,
  inspect,
  inspectProjectRoot,
  normalizeRelativePath,
  selectArchive
};