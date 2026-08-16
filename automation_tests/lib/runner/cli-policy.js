const EXIT_CODES = {
  failed: 1,
  serviceUnavailable: 2
};

function createDefaultArgs() {
  return {
    modules: [],
    parallel: false,
    workers: null,
    e2e: false,
    includeE2e: false,
    archive: false,
    list: false,
    help: false
  };
}

function parseModuleFilters(value) {
  return String(value || '')
    .split(',')
    .map(item => item.trim())
    .filter(Boolean);
}

function parseCliArgs(argv) {
  const result = createDefaultArgs();

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if ((arg === '--module' || arg === '-m') && argv[i + 1] && !argv[i + 1].startsWith('-')) {
      result.modules.push(...parseModuleFilters(argv[i + 1]));
      i++;
    } else if (arg.startsWith('--module=')) {
      result.modules.push(...parseModuleFilters(arg.slice('--module='.length)));
    } else if (arg === '--parallel') {
      result.parallel = true;
    } else if (arg === '--workers' && argv[i + 1] && !argv[i + 1].startsWith('-')) {
      result.workers = Number(argv[i + 1]);
      i++;
    } else if (arg.startsWith('--workers=')) {
      result.workers = Number(arg.slice('--workers='.length));
    } else if (arg === '--e2e') {
      result.e2e = true;
    } else if (arg === '--include-e2e') {
      result.includeE2e = true;
    } else if (arg === '--archive') {
      result.archive = true;
    } else if (arg === '--list') {
      result.list = true;
    } else if (arg === '--help' || arg === '-h') {
      result.help = true;
    } else {
      console.error('Unknown argument: ' + arg);
      result.help = true;
    }
  }
  return result;
}

function parseArgs(argv = process.argv.slice(2)) {
  return parseCliArgs(argv);
}

function shouldArchiveReports(args) {
  return args.archive || (args.includeE2e && args.modules.length === 0);
}

function isStrictIntegrationEnabled(env = process.env) {
  const value = String(env.CI_STRICT_INTEGRATION || '').trim().toLowerCase();
  return value === '1' || value === 'true' || value === 'yes';
}

function getStrictIntegrationGaps(summary = {}) {
  const gaps = [];

  if (Number(summary.skipped || 0) > 0) {
    gaps.push('skipped tests are not allowed in strict integration mode');
  }
  if (Number(summary.partialSkipped || 0) > 0) {
    gaps.push('partial-skip module results are not allowed in strict integration mode');
  }
  if (Number(summary.allSkipped || 0) > 0) {
    gaps.push('all-skipped module results are not allowed in strict integration mode');
  }

  const blockedReasons = Array.isArray(summary.blockedReasons) ? summary.blockedReasons : [];
  if (blockedReasons.some(item => item && item.category === 'runtime-external')) {
    gaps.push('runtime-external prerequisites are blocked in strict integration mode');
  }

  return gaps;
}

function getRunnerExitCode(summary = {}, options = {}) {
  if (Number(summary.failed || 0) > 0) {
    return EXIT_CODES.failed;
  }

  const strictIntegration = typeof options.strictIntegration === 'boolean'
    ? options.strictIntegration
    : isStrictIntegrationEnabled();
  return strictIntegration && getStrictIntegrationGaps(summary).length > 0
    ? EXIT_CODES.failed
    : 0;
}

module.exports = {
  EXIT_CODES,
  createDefaultArgs,
  parseModuleFilters,
  parseCliArgs,
  parseArgs,
  shouldArchiveReports,
  isStrictIntegrationEnabled,
  getStrictIntegrationGaps,
  getRunnerExitCode
};
