/**
 * 文件用途：用于作为API 自动化套件运行入口。
 * 核心逻辑：发现并按约定执行 API 自动化测试文件，汇总通过、失败、跳过和阻塞信息。
 * 关键注意事项：这是 broad API 自动化入口；当前任务只做文件头和语法检查，不启动整套运行。
 * 重构建议：后续可继续把发现、调度、报告和退出码策略拆成可单测的独立模块。
 */

const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const axios = require('axios');
const reporter = require('./lib/reporter');
const apiClient = require('./lib/api_client');
const endpointCoverage = require('./lib/endpoint_coverage');
const pageCoverage = require('./lib/page_coverage');
const coverageProvenance = require('./lib/coverage_provenance');
const {
  summarizeMochaResult,
  summarizePlaywrightResult,
  qualifyCoverageProvenance
} = require('./lib/runner/result-summary');
const {
  EXIT_CODES,
  parseCliArgs,
  parseArgs,
  shouldArchiveReports,
  isStrictIntegrationEnabled,
  getStrictIntegrationGaps,
  getRunnerExitCode
} = require('./lib/runner/cli-policy');
const testMetadata = require('./lib/test_metadata');
const { reconcileManagedMochaCases } = require('./lib/runner/mocha-case-inventory');
const { reconcilePlaywrightCases } = require('./lib/runner/playwright-case-reconciliation');
const runtimeConfig = require('./lib/runtime_config');
const networkConfig = require('./lib/network_runtime');
const runArtifacts = require('./lib/runner/run-artifacts');
const coverageContract = require('./lib/coverage_contract');
const goTestRuntime = require('./lib/go-test-runtime');

let reportsDir = null;
const projectRoot = path.resolve(__dirname, '..');
const verificationDir = path.resolve(
  __dirname,
  process.env.AUTOMATION_VERIFICATION_DIR || path.join('..', 'verification')
);
let activeRunArtifacts = null;

const {
  NON_BUSINESS_EVIDENCE_LABELS,
  getModuleEvidenceLabel,
  getEvidenceLabelPresentation,
  getReportDisplayName,
  keyFromFilename,
  discoverSuites,
  discoverApiModules,
  discoverE2EModules,
  buildExecutionPlan: buildModuleExecutionPlan,
  selectModules
} = require('./lib/runner/module-catalog');

const DISCOVERED_SUITES = discoverSuites();
const API_MODULES = DISCOVERED_SUITES.apiModules;
const E2E_MODULES = DISCOVERED_SUITES.e2eModules;

function createArtifactEnvironment() {
  return {
    evidenceKind: process.env.AUTOMATION_EVIDENCE_KIND || 'local-real',
    lane: process.env.AUTOMATION_EVIDENCE_LANE || 'local-api-e2e',
    frontendUrl: runtimeConfig.frontendURL,
    backendUrl: runtimeConfig.baseURL,
    mqttAddress: process.env.AUTOTEST_MQTT_BROKER || '',
    database: process.env.AUTOMATION_DATABASE_IDENTITY || 'configured-local-database',
    accountSource: process.env.AUTOMATION_ACCOUNT_SOURCE || 'ignored local automation environment'
  };
}

function prepareRunArtifacts(plan) {
  const scope = runArtifacts.classifyPlanScope(plan, plan.discoveredSuites);
  activeRunArtifacts = runArtifacts.createRunArtifacts({
    projectRoot,
    verificationRoot: verificationDir,
    startedAt: new Date(),
    command: [process.execPath, path.join(__dirname, 'run_tests.js'), ...process.argv.slice(2)],
    scope,
    publicationRequested: shouldArchiveReports(plan.args, scope),
    environment: createArtifactEnvironment()
  });
  reportsDir = activeRunArtifacts.reportDir;
  process.env.AUTOMATION_REPORT_DIR = reportsDir;
  return activeRunArtifacts;
}

function getCoverageTempFile(testFile, type = 'api') {
  const safeName = testFile.replace(/[\\/]/g, '_').replace(/\.[^.]+$/g, '');
  const suffix = type === 'page' ? 'page-coverage' : 'endpoint-coverage';
  return path.join(reportsDir, `${type}-${safeName}-${suffix}.json`);
}

function createCoverageRunId(type, testFile) {
  const safeName = testFile.replace(/[\\/]/g, '_').replace(/\.[^.]+$/g, '');
  return `${type}-${safeName}-${process.pid}-${Date.now()}`;
}

function printUsage() {
  console.log([
    'AetherLink IoT automation runner',
    '',
    'Commands:',
    '  node run_tests.js                         Run all API automation modules',
    '  node run_tests.js --parallel              Run all API modules with bounded parallelism',
    '  node run_tests.js --parallel --workers 4  Override API worker count',
    '  node run_tests.js --module device         Run matching API module(s)',
    '  node run_tests.js --e2e                   Run all E2E modules only',
    '  node run_tests.js --include-e2e           Run API modules, then E2E modules',
    '  node run_tests.js --include-e2e --archive Publish a canonical archive only for a clean strict full run',
    '  node run_tests.js --module device --e2e   Run matching E2E module(s)',
    '  node run_tests.js --list                  Print discovered modules',
    '',
    'Module filters accept comma-separated keys, aliases, or file stems.'
  ].join('\n'));
}

function printModuleList() {
  const printGroup = (label, modules) => {
    console.log('\n' + label);
    modules.forEach(mod => {
      const closureLabel = NON_BUSINESS_EVIDENCE_LABELS.has(mod.evidenceLabel)
        ? ' non-business'
        : ' business-evidence-candidate';
      console.log('  ' + mod.key.padEnd(26) + mod.file.padEnd(45) + '[' + mod.evidenceLabel + ' | ' + closureLabel + ']');
    });
  };
  printGroup('API modules (' + API_MODULES.length + ')', API_MODULES);
  printGroup('E2E modules (' + E2E_MODULES.length + ')', E2E_MODULES);
}

function getApiWorkerCount(args, moduleCount) {
  if (!args.parallel || moduleCount <= 1) return 1;
  const config = apiClient.getConfig();
  const configured = args.workers || Number(process.env.API_PARALLEL_WORKERS) || config.parallelWorkers || 4;
  const workers = Number.isFinite(configured) && configured > 0 ? Math.floor(configured) : 4;
  return Math.max(1, Math.min(workers, moduleCount));
}

async function runWithConcurrency(items, limit, worker) {
  const results = new Array(items.length);
  let nextIndex = 0;

  async function runNext() {
    while (nextIndex < items.length) {
      const currentIndex = nextIndex++;
      results[currentIndex] = await worker(items[currentIndex], currentIndex);
    }
  }

  const runners = Array.from({ length: Math.min(limit, items.length) }, runNext);
  await Promise.all(runners);
  return results;
}

function describeMissingModule(filters, apiModules, e2eModules) {
  if (!filters.length || apiModules.length || e2eModules.length) return false;
  console.error('No module matched: ' + filters.join(', '));
  console.error('Available API modules: ' + API_MODULES.map(m => m.key).join(', '));
  console.error('Available E2E modules: ' + E2E_MODULES.map(m => m.key).join(', '));
  return true;
}

const buildExecutionPlan = buildModuleExecutionPlan;

function printExecutionPlan(plan) {
  console.log('\n' + '='.repeat(70));
  console.log('  AetherLink IoT automation suite');
  console.log('  Types: ' + (plan.types.join(' + ') || 'none'));
  console.log('  Mode: ' + plan.runMode);
  console.log('  API modules: ' + (plan.apiModulesToRun.map(m => m.key).join(', ') || 'none'));
  console.log('  E2E modules: ' + (plan.e2eModulesToRun.map(m => m.key).join(', ') || 'none'));
  console.log('  Start: ' + new Date().toLocaleString('zh-CN'));
  console.log('='.repeat(70) + '\n');
}

function removeFileIfExists(filePath) {
  if (fs.existsSync(filePath)) {
    fs.unlinkSync(filePath);
  }
}

function runMocha(testFile) {
  return new Promise(resolve => {
    const cmd = process.execPath;
    const mochaPath = path.join(__dirname, 'node_modules', 'mocha', 'bin', 'mocha.js');
    const reportFilename = testFile.replace('.test.js', '-report');
    const reportJson = path.join(reportsDir, reportFilename + '.json');
    const args = [
      mochaPath,
      path.join(__dirname, 'tests', testFile),
      '--timeout', '30000',
      '--reporter', path.join(__dirname, 'lib', 'coverage_mocha_reporter.js'),
      '--reporter-options',
      `reportDir=${reportsDir},reportFilename=${reportFilename},overwrite=true,quiet=true`
    ];

    const coverageFile = getCoverageTempFile(testFile, 'api');
    const provenanceFile = coverageProvenance.provenanceFileForCoverage(coverageFile);
    const coverageRunId = createCoverageRunId('api', testFile);
    removeFileIfExists(coverageFile);
    removeFileIfExists(provenanceFile);
    removeFileIfExists(reportJson);

    const proc = spawn(cmd, args, {
      stdio: ['ignore', 'pipe', 'pipe'],
      env: {
        ...process.env,
        ENDPOINT_COVERAGE_FILE: coverageFile,
        COVERAGE_PROVENANCE_FILE: provenanceFile,
        AETHERLINK_COVERAGE_RUN_ID: coverageRunId,
        AETHERLINK_COVERAGE_MODULE: keyFromFilename(testFile),
        AUTOMATION_REPORT_DIR: reportsDir
      }
    });

    let stdout = '';
    let stderr = '';
    proc.stdout.on('data', data => { stdout += data.toString(); });
    proc.stderr.on('data', data => { stderr += data.toString(); });

    proc.on('close', code => {
      resolve({ code, stdout, stderr, coverageFile, provenanceFile, coverageRunId, reportJson });
    });
  });
}

function runPlaywright(testFile) {
  return new Promise(resolve => {
    const cmd = process.execPath;
    const pwPath = require.resolve('@playwright/test/cli');
    const safeName = testFile.replace(/[\\/]/g, '_').replace(/\.[^.]+$/g, '');
    const reportJson = path.join(reportsDir, `e2e-${safeName}-results.json`);
    const args = [
      pwPath,
      'test',
      testFile,
      '--config', path.join(__dirname, 'playwright.config.js')
    ];

    removeFileIfExists(reportJson);

    const coverageFile = getCoverageTempFile(testFile, 'page');
    const provenanceFile = coverageProvenance.provenanceFileForCoverage(coverageFile);
    const coverageRunId = createCoverageRunId('e2e', testFile);
    removeFileIfExists(coverageFile);
    removeFileIfExists(provenanceFile);

    const proc = spawn(cmd, args, {
      stdio: ['ignore', 'pipe', 'pipe'],
      cwd: __dirname,
      env: {
        ...process.env,
        PAGE_COVERAGE_FILE: coverageFile,
        COVERAGE_PROVENANCE_FILE: provenanceFile,
        AETHERLINK_COVERAGE_RUN_ID: coverageRunId,
        AETHERLINK_COVERAGE_MODULE: keyFromFilename(testFile),
        PLAYWRIGHT_JSON_OUTPUT: reportJson,
        AUTOMATION_REPORT_DIR: reportsDir
      }
    });

    let stdout = '';
    let stderr = '';
    proc.stdout.on('data', data => { stdout += data.toString(); });
    proc.stderr.on('data', data => { stderr += data.toString(); });

    proc.on('close', code => {
      resolve({ code, stdout, stderr, reportJson, coverageFile, provenanceFile, coverageRunId });
    });
  });
}

function createModuleRunRecord(mod, result, summary) {
  return {
    mod,
    result,
    summary,
    passed: summary.passed,
    outcome: summary.outcome,
    skipped: summary.skipped,
    blockedReasons: summary.blockedReasons,
    reason: summary.reason
  };
}

async function frontendHealthCheck(url) {
  try {
    const trustedURL = networkConfig.validateTrustedURL(url, 'health check URL');
    const resp = await axios.get(trustedURL, { timeout: 5000, validateStatus: () => true });
    return resp.status >= 200 && resp.status < 500;
  } catch (err) {
    return false;
  }
}

function printBackendUnavailable(config) {
  console.error('Backend service unavailable: ' + networkConfig.healthURL);
  console.error('Start the AetherLink IoT backend before running API/E2E automation.');
}

async function ensureBackendReady() {
  const config = apiClient.getConfig();
  const healthy = await apiClient.healthCheck();
  if (!healthy) {
    printBackendUnavailable(config);
    return false;
  }
  console.log('Backend service is healthy.');
  return true;
}

function shouldCheckFrontendReady(plan) {
  return plan.e2eModulesToRun.length > 0;
}

async function ensureFrontendReady(plan) {
  if (!shouldCheckFrontendReady(plan)) {
    return true;
  }

  const cfg = apiClient.getConfig();
  const frontendOk = await frontendHealthCheck(cfg.frontendURL);
  if (!frontendOk) {
    console.warn('Frontend is not ready yet: ' + cfg.frontendURL);
    console.warn('Playwright webServer will try to start the local frontend.');
    return false;
  }

  console.log('Frontend service is reachable.');
  return true;
}

async function ensureServicesReady(plan) {
  const backendReady = await ensureBackendReady();
  if (!backendReady) {
    return false;
  }

  await ensureFrontendReady(plan);
  console.log('');
  return true;
}

function prepareRunReporting(args) {
  if (args.parallel) {
    reporter.setParallel(true);
  }
  reporter.start();
  endpointCoverage.reset();
  pageCoverage.reset();
}

function printModuleRunStart(kind, mod) {
  const evidence = getEvidenceLabelPresentation(mod.evidenceLabel);
  console.log('\n> ' + kind + ' module: ' + mod.name + ' (' + mod.file + ')');
  console.log('  Evidence label: ' + mod.evidenceLabel);
  if (evidence.nonBusiness) {
    console.log('  Business closure: no (' + evidence.closureDescription + ')');
  }
  console.log('-'.repeat(50));
}

function readJsonReportIfPresent(filePath) {
  if (!filePath || !fs.existsSync(filePath)) return null;
  try {
    return JSON.parse(fs.readFileSync(filePath, 'utf8'));
  } catch (_error) {
    return null;
  }
}

function persistModuleSummary(mod, type, summary) {
  reporter.record(
    mod.key,
    getReportDisplayName(mod),
    summary.passed,
    summary.reason,
    type,
    mod.evidenceLabel,
    summary
  );
}

function recordModuleSummary(
  mod,
  type,
  summary,
  result = null,
  explicitReport = null,
  deferRecord = false
) {
  const metadataFile = mod.metadataFile || mod.file;
  const metadata = testMetadata.getTestMetadata(metadataFile);
  if (type === 'api') {
    const reconciliation = reconcileManagedMochaCases({
      metadataFile,
      metadata,
      reportCases: summary.caseResults
    });
    summary.caseMetadataManaged = reconciliation.managed;
    summary.caseReconciliationValid = reconciliation.valid;
    summary.caseReconciliationErrors = reconciliation.errors;
    summary.caseResults = reconciliation.managed ? reconciliation.caseResults : summary.caseResults;
    const runtimeEligible = summary.passed === true &&
      summary.outcome === 'passed' &&
      summary.skipped === 0 &&
      summary.blockedReasons.length === 0 &&
      reconciliation.valid;
    summary.oracleCases = runtimeEligible ? reconciliation.oracleCases : [];
    summary.caseLevelBusinessClosureEvidence = runtimeEligible &&
      summary.oracleCases.some(item => (
        item.evidenceKind === 'business' && item.businessClosureEvidence === true
      ));
  } else if (type === 'e2e') {
    const managed = Boolean(metadata && Array.isArray(metadata.cases) && metadata.cases.some(item => (
      item && (item.caseId || item.fullTitle)
    )));
    if (managed) {
      const report = explicitReport || readJsonReportIfPresent(result && result.reportJson);
      if (!report) {
        summary.passed = false;
        summary.outcome = 'failed';
        summary.caseMetadataManaged = true;
        summary.caseReconciliationValid = false;
        summary.caseReconciliationErrors = [
          'Playwright JSON report is missing for managed case reconciliation'
        ];
        summary.reason = summary.caseReconciliationErrors[0];
      } else {
        const reconciliation = reconcilePlaywrightCases(report, metadata);
        summary.caseMetadataManaged = true;
        summary.caseReconciliationValid = reconciliation.errors.length === 0;
        summary.caseReconciliationErrors = reconciliation.errors;
        const caseIdByIdentity = new Map(reconciliation.oracleCases.map(item => [
          `${item.file} :: ${item.fullTitle}`,
          item.caseId
        ]));
        summary.caseResults = reconciliation.extractedCases.map(item => ({
          ...item,
          caseId: caseIdByIdentity.get(item.identity) || null,
          titlePath: item.fullTitle
            ? item.fullTitle.split(' › ').map(part => part.trim()).filter(Boolean)
            : []
        }));
        const runtimeEligible = summary.passed === true &&
          summary.outcome === 'passed' &&
          summary.skipped === 0 &&
          summary.blockedReasons.length === 0 &&
          summary.caseReconciliationValid;
        if (!summary.caseReconciliationValid) {
          summary.passed = false;
          summary.outcome = 'failed';
          summary.reason = reconciliation.errors.join('; ');
        }
        summary.oracleCases = runtimeEligible ? reconciliation.oracleCases : [];
        summary.caseLevelBusinessClosureEvidence = runtimeEligible &&
          summary.oracleCases.some(item => item.businessClosureEvidence === true);
      }
    }
  }
  if (!deferRecord) {
    persistModuleSummary(mod, type, summary);
  }
}

function finalizeCoverageProvenance(mod, result, summary) {
  const ledger = coverageProvenance.readLedger(result.provenanceFile);
  const qualified = qualifyCoverageProvenance(
    ledger.events,
    summary,
    mod.key,
    result.coverageRunId
  );
  result.coverageArtifactQualified = true;
  coverageProvenance.replaceLedger(result.provenanceFile, qualified);
  if (result.coverageFile && fs.existsSync(result.coverageFile)) {
    try {
      const coveragePayload = JSON.parse(fs.readFileSync(result.coverageFile, 'utf8'));
      coveragePayload.provenance = {
        schema: coverageProvenance.SCHEMA,
        events: qualified
      };
      fs.writeFileSync(result.coverageFile, JSON.stringify(coveragePayload, null, 2), 'utf8');
    } catch (error) {
      result.coverageArtifactQualified = false;
      qualified.forEach(event => {
        event.disposition = 'diagnostic';
        event.diagnostics = {
          ...event.diagnostics,
          coverageArtifactRewriteFailed: error.message
        };
      });
      coverageProvenance.replaceLedger(result.provenanceFile, qualified);
    }
  }
  summary.coverageProvenance = {
    ...coverageProvenance.summarizeEvents(qualified),
    file: result.provenanceFile
  };
  return qualified;
}

function printModuleRunTail(result, summary) {
  if (summary.passed && summary.reason) {
    console.warn('  ' + summary.reason);
  }
  if (result.stdout) {
    const lines = result.stdout.split('\n').filter(line => line.trim());
    lines.slice(-10).forEach(line => console.log('  ' + line));
  }
}

async function executeModuleRun(options) {
  const {
    mod,
    kind,
    type,
    execute,
    summarize,
    mergeCoverage
  } = options;

  printModuleRunStart(kind, mod);
  const result = await execute(mod.file);
  const summary = summarize(result);
  recordModuleSummary(mod, type, summary, result, null, true);
  finalizeCoverageProvenance(mod, result, summary);
  persistModuleSummary(mod, type, summary);
  printModuleRunTail(result, summary);
  if (result.coverageArtifactQualified !== false) {
    mergeCoverage(result.coverageFile);
  }
  return createModuleRunRecord(mod, result, summary);
}

async function runApiModule(mod) {
  return executeModuleRun({
    mod,
    kind: 'API',
    type: 'api',
    execute: runMocha,
    summarize: summarizeMochaResult,
    mergeCoverage: coverageFile => endpointCoverage.mergeFromFile(coverageFile)
  });
}

async function runE2EModule(mod) {
  return executeModuleRun({
    mod,
    kind: 'E2E',
    type: 'e2e',
    execute: runPlaywright,
    summarize: summarizePlaywrightResult,
    mergeCoverage: coverageFile => pageCoverage.mergeFromFile(coverageFile)
  });
}

function summarizePhaseResults(results) {
  const failed = results.filter(result => !result.passed).length;
  return {
    total: results.length,
    passed: results.length - failed,
    failed
  };
}

function printPhaseHeader(title) {
  console.log('\n' + '#'.repeat(70));
  console.log('  ' + title);
  console.log('#'.repeat(70));
}

function printParallelPhaseStart(kind, modules, workerCount) {
  console.log('\n[parallel] starting ' + modules.length + ' ' + kind + ' modules with ' + workerCount + ' workers');
}

function printParallelPhaseComplete(kind, phaseSummary) {
  console.log('\n[parallel] ' + kind + ' complete: ' + phaseSummary.passed + ' passed / ' + phaseSummary.failed + ' failed');
}

// Repository-wide generated-artifact inventory is intentionally read-only, but
// it cannot produce a stable snapshot while sibling API modules are creating
// reports, SBOMs, or other ignored runtime files. Keep this contract in the
// parallel CLI without allowing concurrent writers to turn a clean run into a
// false failure.
const API_MODULES_REQUIRING_SERIAL_INVENTORY = new Set([
  'generated-artifact-boundary-contract'
]);

async function runModulesInParallel(modules, workerCount, runModule, kind) {
  printParallelPhaseStart(kind, modules, workerCount);
  const results = await runWithConcurrency(modules, workerCount, mod => runModule(mod));
  const phaseSummary = summarizePhaseResults(results);
  printParallelPhaseComplete(kind, phaseSummary);
  return results;
}

async function runApiModulesInParallel(args, modules) {
  const parallelModules = modules.filter(mod => !API_MODULES_REQUIRING_SERIAL_INVENTORY.has(mod.key));
  const serialModules = modules.filter(mod => API_MODULES_REQUIRING_SERIAL_INVENTORY.has(mod.key));
  const resultsByKey = new Map();

  if (parallelModules.length > 0) {
    const apiWorkers = getApiWorkerCount(args, parallelModules.length);
    const parallelResults = await runModulesInParallel(parallelModules, apiWorkers, runApiModule, 'API');
    parallelResults.forEach((result, index) => resultsByKey.set(parallelModules[index].key, result));
  }

  if (serialModules.length > 0) {
    console.log('\n[serial-after-parallel] running repository-inventory API modules after concurrent writers finish');
    const serialResults = await runModulesSequentially(serialModules, runApiModule);
    serialResults.forEach((result, index) => resultsByKey.set(serialModules[index].key, result));
  }

  return modules.map(mod => resultsByKey.get(mod.key));
}

async function runModulesSequentially(modules, runModule) {
  const results = [];
  for (const mod of modules) {
    results.push(await runModule(mod));
  }
  return results;
}

async function runApiPhase(plan) {
  const { args, apiModulesToRun } = plan;
  if (apiModulesToRun.length === 0) {
    return [];
  }

  printPhaseHeader('Phase 1: API automation');

  if (args.parallel && apiModulesToRun.length > 1) {
    return runApiModulesInParallel(args, apiModulesToRun);
  }

  return runModulesSequentially(apiModulesToRun, runApiModule);
}

async function runE2EPhase(plan) {
  const { e2eModulesToRun } = plan;
  if (e2eModulesToRun.length === 0) {
    return [];
  }

  printPhaseHeader('Phase 2: E2E automation');

  return runModulesSequentially(e2eModulesToRun, runE2EModule);
}

function writeCoverageReport(title, coverage, interval) {
  printPhaseHeader(title);
  coverage.report();
  coverage.writeReport(reportsDir, interval);
}

function writeCoverageReportsForPlan(plan, interval) {
  if (plan.apiModulesToRun.length > 0) {
    writeCoverageReport('Phase 3: API endpoint coverage', endpointCoverage, interval);
  }

  if (plan.e2eModulesToRun.length > 0) {
    writeCoverageReport('Phase 4: E2E page coverage', pageCoverage, interval);
  }
}

function writeGoTestEvidenceForPlan(plan) {
  if (activeRunArtifacts.scope !== 'full') return null;
  printPhaseHeader('Phase 5: Exact Go runtime evidence');
  const report = goTestRuntime.collectGoTestEvidence({
    projectRoot,
    evidenceItems: coverageContract.ALL_GO_EVIDENCE,
    startedAt: new Date().toISOString(),
    source: activeRunArtifacts.source
  });
  goTestRuntime.writeGoTestEvidenceReport(reportsDir, report);
  return report;
}

function printReportLocations(artifactResult) {
  console.log('\nReports:');
  console.log('  Run-scoped output: ' + path.resolve(artifactResult.reportDir));
  if (artifactResult.reports.summary) {
    console.log('  JSON: ' + path.resolve(artifactResult.reports.summary));
  }
  if (artifactResult.archiveDir) {
    console.log('  Canonical archive: ' + path.resolve(artifactResult.archiveDir));
  } else {
    console.log('  Diagnostic staging: ' + path.resolve(artifactResult.diagnosticDir));
    if (artifactResult.publicationErrors.length > 0) {
      console.log('  Not published: ' + artifactResult.publicationErrors.join('; '));
    }
  }
  console.log('');
}

function createFinalizedRunResult(summary, artifactResult, exitCode) {
  return {
    summary,
    jsonReport: artifactResult.reports.summary || null,
    reportDir: artifactResult.reportDir,
    archiveDir: artifactResult.archiveDir,
    diagnosticDir: artifactResult.diagnosticDir,
    exitCode
  };
}

function finalizeRunReports(plan) {
  const summary = reporter.end();
  const initialInterval = {
    startedAt: reporter.startTime ? reporter.startTime.toISOString() : activeRunArtifacts.startedAt
  };
  reporter.generateJsonReport(reportsDir);
  const goReport = writeGoTestEvidenceForPlan(plan);
  const finishedAt = new Date();
  const interval = {
    startedAt: initialInterval.startedAt,
    finishedAt: finishedAt.toISOString()
  };
  writeCoverageReportsForPlan(plan, interval);

  const strictIntegration = isStrictIntegrationEnabled();
  const runnerExitCode = getRunnerExitCode(summary, { strictIntegration });
  const goEvidenceFailed = activeRunArtifacts.scope === 'full' && (!goReport || goReport.passed !== true);
  const exitCode = goEvidenceFailed && runnerExitCode === 0 ? EXIT_CODES.failed : runnerExitCode;
  const blockingGaps = getStrictIntegrationGaps(summary);
  if (goEvidenceFailed) blockingGaps.push('go-test-evidence-failed');
  const artifactResult = runArtifacts.finalizeRunArtifacts(activeRunArtifacts, {
    finishedAt,
    exitCode,
    strictIntegration,
    cleanup: runArtifacts.deriveCleanupEvidence({
      ...summary,
      caseOutcomes: reporter.getCanonicalOperationCaseOutcomes()
    }),
    blockingGaps
  });
  reportsDir = artifactResult.reportDir;
  process.env.AUTOMATION_REPORT_DIR = reportsDir;
  printReportLocations(artifactResult);

  return createFinalizedRunResult(summary, artifactResult, exitCode);
}

function exitRunner(code) {
  process.exit(code);
}

function handleInformationalArgs(args) {
  if (args.help) {
    printUsage();
    return true;
  }
  if (args.list) {
    printModuleList();
    return true;
  }
  return false;
}

async function executePlan(plan) {
  prepareRunArtifacts(plan);
  const servicesReady = await ensureServicesReady(plan);
  if (!servicesReady) {
    runArtifacts.writeInterruptedManifest(activeRunArtifacts, {
      exitCode: EXIT_CODES.serviceUnavailable,
      strictIntegration: isStrictIntegrationEnabled(),
      cleanup: { status: 'not-run', notes: ['Test execution did not start because service readiness failed.'] },
      blockingGaps: ['service-unavailable']
    });
    exitRunner(EXIT_CODES.serviceUnavailable);
    return;
  }

  prepareRunReporting(plan.args);
  await runApiPhase(plan);
  await runE2EPhase(plan);

  const result = finalizeRunReports(plan);
  exitRunner(result.exitCode);
}

async function main() {
  const args = parseArgs();
  if (handleInformationalArgs(args)) {
    return;
  }

  const plan = buildExecutionPlan(args);
  if (describeMissingModule(args.modules, plan.apiModulesToRun, plan.e2eModulesToRun)) {
    exitRunner(EXIT_CODES.failed);
    return;
  }
  printExecutionPlan(plan);
  await executePlan(plan);
}

if (require.main === module) {
  main().catch(err => {
    console.error('Automation runner failed:', err);
    if (activeRunArtifacts && fs.existsSync(activeRunArtifacts.stagingDir)) {
      try {
        runArtifacts.writeInterruptedManifest(activeRunArtifacts, {
          exitCode: EXIT_CODES.failed,
          strictIntegration: isStrictIntegrationEnabled(),
          cleanup: { status: 'not-run', notes: ['Runner terminated before normal finalization.'] },
          blockingGaps: ['runner-interrupted']
        });
      } catch (manifestError) {
        console.error('Failed to preserve interrupted-run manifest:', manifestError);
      }
    }
    exitRunner(EXIT_CODES.failed);
  });
}

module.exports = {
  EXIT_CODES,
  getModuleEvidenceLabel,
  getReportDisplayName,
  NON_BUSINESS_EVIDENCE_LABELS,
  keyFromFilename,
  parseCliArgs,
  parseArgs,
  discoverSuites,
  discoverApiModules,
  discoverE2EModules,
  buildExecutionPlan,
  selectModules,
  prepareRunArtifacts,
  summarizePhaseResults,
  summarizeMochaResult,
  summarizePlaywrightResult,
  finalizeCoverageProvenance,
  executeModuleRun,
  recordModuleSummary,
  getRunnerExitCode,
  API_MODULES,
  E2E_MODULES
};
