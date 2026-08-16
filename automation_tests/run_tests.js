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
const {
  summarizeMochaResult,
  summarizePlaywrightResult
} = require('./lib/runner/result-summary');
const {
  EXIT_CODES,
  parseCliArgs,
  parseArgs,
  shouldArchiveReports,
  getRunnerExitCode
} = require('./lib/runner/cli-policy');
const testMetadata = require('./lib/test_metadata');
const runtimeConfig = require('./lib/runtime_config');
const networkConfig = require('./lib/network_runtime');

const reportsDir = path.resolve(__dirname, runtimeConfig.report.outputDir);
const verificationDir = path.resolve(
  __dirname,
  process.env.AUTOMATION_VERIFICATION_DIR || path.join('..', 'verification')
);

function prepareReportsDir() {
  if (!fs.existsSync(reportsDir)) {
    fs.mkdirSync(reportsDir, { recursive: true });
  }
}

prepareReportsDir();

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

function getCoverageTempFile(testFile, type = 'api') {
  const safeName = testFile.replace(/[\\/]/g, '_').replace(/\.[^.]+$/g, '');
  const suffix = type === 'page' ? 'page-coverage' : 'endpoint-coverage';
  return path.join(reportsDir, `${type}-${safeName}-${suffix}.json`);
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
    '  node run_tests.js --include-e2e --archive Archive reports into verification/',
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

function archiveReportsIfRequested(args) {
  if (!shouldArchiveReports(args)) {
    return null;
  }

  if (!fs.existsSync(reportsDir)) {
    return null;
  }

  const timestamp = new Date().toISOString().replace(/[-:]/g, '').replace(/\..+$/, '').replace('T', '-');
  const archiveDir = path.join(verificationDir, 'automation-run-' + timestamp);
  fs.mkdirSync(archiveDir, { recursive: true });

  for (const entry of fs.readdirSync(reportsDir, { withFileTypes: true })) {
    const source = path.join(reportsDir, entry.name);
    const target = path.join(archiveDir, entry.name);
    if (entry.isFile()) {
      fs.copyFileSync(source, target);
    }
  }

  const manifest = {
    archivedAt: new Date().toISOString(),
    command: ['node', 'run_tests.js', ...process.argv.slice(2)],
    reportSource: reportsDir,
    note: 'Copied after runner completion to avoid shared reports being mistaken for durable evidence.'
  };
  fs.writeFileSync(path.join(archiveDir, 'archive-manifest.json'), JSON.stringify(manifest, null, 2), 'utf8');
  return archiveDir;
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
      '--reporter', 'mochawesome',
      '--reporter-options',
      `reportDir=${reportsDir},reportFilename=${reportFilename},overwrite=true,quiet=true`
    ];

    const coverageFile = getCoverageTempFile(testFile, 'api');
    removeFileIfExists(coverageFile);
    removeFileIfExists(reportJson);

    const proc = spawn(cmd, args, {
      stdio: ['ignore', 'pipe', 'pipe'],
      env: {
        ...process.env,
        ENDPOINT_COVERAGE_FILE: coverageFile
      }
    });

    let stdout = '';
    let stderr = '';
    proc.stdout.on('data', data => { stdout += data.toString(); });
    proc.stderr.on('data', data => { stderr += data.toString(); });

    proc.on('close', code => {
      resolve({ code, stdout, stderr, coverageFile, reportJson });
    });
  });
}

function runPlaywright(testFile) {
  return new Promise(resolve => {
    const cmd = process.execPath;
    const pwPath = require.resolve('@playwright/test/cli');
    const reportJson = path.join(reportsDir, 'e2e-results.json');
    const args = [
      pwPath,
      'test',
      testFile,
      '--config', path.join(__dirname, 'playwright.config.js')
    ];

    removeFileIfExists(reportJson);

    const coverageFile = getCoverageTempFile(testFile, 'page');
    removeFileIfExists(coverageFile);

    const proc = spawn(cmd, args, {
      stdio: ['ignore', 'pipe', 'pipe'],
      cwd: __dirname,
      env: {
        ...process.env,
        PAGE_COVERAGE_FILE: coverageFile
      }
    });

    let stdout = '';
    let stderr = '';
    proc.stdout.on('data', data => { stdout += data.toString(); });
    proc.stderr.on('data', data => { stderr += data.toString(); });

    proc.on('close', code => {
      resolve({ code, stdout, stderr, reportJson, coverageFile });
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

function recordModuleSummary(mod, type, summary) {
  // Metadata is case-level by design. Once a module has actually run to
  // completion, promote only explicitly marked business cases into the
  // runtime report; boundary/catalog cases must not inflate closure.
  const metadata = testMetadata.getTestMetadata(mod.file);
  const businessCases = metadata && Array.isArray(metadata.cases)
    ? metadata.cases.filter(item => (
      item &&
      item.evidenceKind === 'business' &&
      item.businessClosureEvidence === true
    ))
    : [];
  if (
    summary.passed === true &&
    summary.skipped === 0 &&
    summary.blockedReasons.length === 0 &&
    businessCases.length > 0
  ) {
    summary.caseLevelBusinessClosureEvidence = true;
    summary.oracleCases = businessCases.map(item => ({
      title: item.title,
      businessClosureEvidence: true
    }));
  }
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
  recordModuleSummary(mod, type, summary);
  printModuleRunTail(result, summary);
  mergeCoverage(result.coverageFile);
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

function writeCoverageReport(title, coverage) {
  printPhaseHeader(title);
  coverage.report();
  coverage.writeReport(reportsDir);
}

function writeCoverageReportsForPlan(plan) {
  if (plan.apiModulesToRun.length > 0) {
    writeCoverageReport('Phase 3: API endpoint coverage', endpointCoverage);
  }

  if (plan.e2eModulesToRun.length > 0) {
    writeCoverageReport('Phase 4: E2E page coverage', pageCoverage);
  }
}

function printReportLocations(jsonReport, archiveDir) {
  console.log('\nReports:');
  console.log('  HTML: ' + path.resolve(reportsDir));
  console.log('  JSON: ' + path.resolve(jsonReport));
  if (archiveDir) {
    console.log('  Archive: ' + path.resolve(archiveDir));
  }
  console.log('');
}

function createFinalizedRunResult(summary, jsonReport, archiveDir) {
  return {
    summary,
    jsonReport,
    archiveDir
  };
}

function finalizeRunReports(plan) {
  const summary = reporter.end();
  const jsonReport = reporter.generateJsonReport(reportsDir);

  writeCoverageReportsForPlan(plan);

  const archiveDir = archiveReportsIfRequested(plan.args);
  printReportLocations(jsonReport, archiveDir);

  return createFinalizedRunResult(summary, jsonReport, archiveDir);
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
  const servicesReady = await ensureServicesReady(plan);
  if (!servicesReady) {
    exitRunner(EXIT_CODES.serviceUnavailable);
    return;
  }

  prepareRunReporting(plan.args);
  await runApiPhase(plan);
  await runE2EPhase(plan);

  const result = finalizeRunReports(plan);
  exitRunner(getRunnerExitCode(result.summary));
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
  summarizePhaseResults,
  summarizeMochaResult,
  summarizePlaywrightResult,
  recordModuleSummary,
  getRunnerExitCode,
  API_MODULES,
  E2E_MODULES
};
