const fs = require('fs');
const path = require('path');
const testMetadata = require('../test_metadata');

const automationTestsRoot = path.resolve(__dirname, '..', '..');

const MODULE_LABELS = {
  alarm: 'Alarm',
  'api-boundary-smoke': 'API boundary smoke',
  'api-coverage-closure': 'API domain boundary evidence',
  'attribute-command-event': 'Attribute command event',
  auth: 'Authentication',
  automation: 'Automation scene',
  board: 'Board',
  casbin: 'Casbin policy',
  config: 'Device config',
  'coverage-contract': 'Coverage contract',
  data: 'Telemetry data',
  'data-script': 'Data script',
  device: 'Device',
  'device-alarm-share': 'Device alarm share',
  'device-config': 'Device config extra',
  'device-config-openapi': 'Device config OpenAPI',
  'device-extra': 'Device extra',
  dict: 'Dictionary',
  'dict-notification': 'Dictionary notification',
  'endpoint-coverage': 'Endpoint catalog matcher',
  login: 'Login',
  management: 'Management',
  notification: 'Notification',
  openapi: 'OpenAPI',
  'oracle-contract': 'Oracle contract',
  ota: 'OTA',
  'ota-data-script': 'OTA data script',
  'preflight-api-e2e': 'API/E2E preflight gate',
  role: 'Role',
  'role-casbin': 'Role casbin policy',
  'runtime-config-env': 'Runtime config env',
  'seeded-automation-scene': 'Seeded automation scene business',
  'seeded-command-jobs': 'Seeded command jobs business',
  'seeded-scene-automations': 'Seeded scene automation business',
  system: 'System',
  'telemetry-extra': 'Telemetry extra',
  'uncovered-endpoints': 'Previously uncovered endpoints',
  'user-extra': 'User extra',
  'write-flows': 'Write flows'
};

const MODULE_EVIDENCE_LABELS = {
  'api:api-boundary-smoke': 'boundary',
  'api:api-coverage-closure': 'boundary',
  'api:coverage-contract': 'contract',
  'api:endpoint-coverage': 'catalog',
  'api:oracle-contract': 'contract',
  'api:preflight-api-e2e': 'preflight',
  'api:runtime-config-env': 'config',
  'e2e:apply-marketplace': 'boundary',
  'e2e:dashboard': 'boundary',
  'e2e:route-coverage-closure': 'business'
};

const NON_BUSINESS_EVIDENCE_LABELS = new Set([
  'boundary',
  'catalog',
  'config',
  'contract',
  'preflight',
  'page-coverage-only',
  'unknown'
]);

const NON_BUSINESS_EVIDENCE_DESCRIPTIONS = {
  boundary: 'boundary/API-contract evidence only',
  catalog: 'catalog alignment evidence only',
  config: 'runtime configuration evidence only',
  contract: 'harness/source contract evidence only',
  preflight: 'environment preflight evidence only',
  'page-coverage-only': 'page visit evidence only',
  unknown: 'metadata missing; not business closure'
};

function getModuleEvidenceLabel(key, type) {
  const metadataPath = type === 'e2e'
    ? `e2e/${key}.spec.js`
    : `tests/${key}.test.js`;
  return getModuleEvidenceLabelFromMetadata(
    key,
    type,
    testMetadata.getTestMetadata(metadataPath)
  );
}

function isBrowserE2EBusinessClosureCase(item) {
  return Boolean(
    item &&
    item.evidenceKind === 'business' &&
    item.businessClosureEvidence === true &&
    item.hasBrowserUserFlow !== false &&
    item.evidenceLayer === 'browser-e2e-with-api-setup'
  );
}

function isApiFixtureOnlyE2ECase(item) {
  return Boolean(
    item &&
    item.evidenceLayer === 'api-via-e2e-fixture' &&
    item.hasBrowserUserFlow === false
  );
}

function getE2EModuleEvidenceLabelFromMetadata(metadata) {
  if (!metadata || metadata.evidenceKind !== 'business') {
    return metadata && metadata.evidenceKind;
  }

  const cases = Array.isArray(metadata.cases) ? metadata.cases : [];
  if (cases.some(isBrowserE2EBusinessClosureCase)) {
    return 'business';
  }

  if (cases.some(isApiFixtureOnlyE2ECase)) {
    return 'boundary';
  }

  return metadata.evidenceKind;
}

function getModuleEvidenceLabelFromMetadata(key, type, metadata) {
  if (metadata && metadata.evidenceKind) {
    if (type === 'e2e') {
      return getE2EModuleEvidenceLabelFromMetadata(metadata);
    }
    return metadata.evidenceKind;
  }
  const explicit = MODULE_EVIDENCE_LABELS[`${type}:${key}`];
  if (explicit) {
    return explicit;
  }
  if (type === 'api' && key.startsWith('seeded-')) {
    return 'business';
  }
  return 'unknown';
}

function getNonBusinessClosureDescription(evidenceLabel) {
  return NON_BUSINESS_EVIDENCE_DESCRIPTIONS[evidenceLabel] || 'non-business evidence only';
}

function getEvidenceLabelPresentation(evidenceLabel) {
  const nonBusiness = NON_BUSINESS_EVIDENCE_LABELS.has(evidenceLabel);
  return {
    nonBusiness,
    reportSuffix: nonBusiness ? '; not business closure' : '',
    closureDescription: nonBusiness ? getNonBusinessClosureDescription(evidenceLabel) : ''
  };
}

function getReportDisplayName(mod) {
  const evidenceLabel = mod && mod.evidenceLabel;
  const type = mod && mod.type;
  const file = mod && mod.file;
  if (!evidenceLabel || evidenceLabel === type) {
    return file;
  }
  return `${file} [evidence: ${evidenceLabel}${getEvidenceLabelPresentation(evidenceLabel).reportSuffix}]`;
}

function ensureDir(dir) {
  if (!fs.existsSync(dir)) {
    return [];
  }
  return fs.readdirSync(dir, { withFileTypes: true })
    .filter(entry => entry.isFile())
    .map(entry => entry.name);
}

function keyFromFilename(filename) {
  return filename
    .replace(/\.(test|spec)\.js$/, '')
    .replace(/^\d+_/, '')
    .replace(/_/g, '-')
    .toLowerCase();
}

function sortByLeadingNumberThenName(a, b) {
  const getNumber = file => {
    const match = file.match(/^(\d+)_/);
    return match ? Number(match[1]) : 9999;
  };
  const diff = getNumber(a.rawFile) - getNumber(b.rawFile);
  return diff || a.rawFile.localeCompare(b.rawFile);
}

function buildAliases(key, type, rawFile) {
  const aliases = new Set([key, `${type}-${key}`, rawFile.replace(/\.(test|spec)\.js$/, '')]);
  if (key === 'login') {
    aliases.add('auth');
    aliases.add('e2e-auth');
  }
  return Array.from(aliases);
}

function getModuleFileForType(type, file) {
  return type === 'e2e'
    ? path.join('e2e', file).replace(/\\/g, '/')
    : file;
}

function getMetadataPathForModule(type, file, moduleFile) {
  return type === 'e2e' ? moduleFile : `tests/${file}`;
}

function getModuleNameForType(type, key) {
  const label = MODULE_LABELS[key] || key;
  return type === 'e2e' ? 'E2E-' + label : label;
}

function createDiscoveredModule(type, file) {
  const key = keyFromFilename(file);
  const moduleFile = getModuleFileForType(type, file);
  const metadata = testMetadata.getTestMetadata(getMetadataPathForModule(type, file, moduleFile));
  return {
    key,
    aliases: buildAliases(key, type, file),
    name: getModuleNameForType(type, key),
    evidenceLabel: getModuleEvidenceLabelFromMetadata(key, type, metadata),
    file: moduleFile,
    rawFile: file,
    type
  };
}

function discoverModules(dir, filePattern, type) {
  return ensureDir(dir)
    .filter(file => filePattern.test(file))
    .map(file => createDiscoveredModule(type, file))
    .sort(sortByLeadingNumberThenName);
}

function discoverApiModules() {
  return discoverModules(path.join(automationTestsRoot, 'tests'), /\.test\.js$/, 'api');
}

function discoverE2EModules() {
  return discoverModules(path.join(automationTestsRoot, 'e2e'), /\.spec\.js$/, 'e2e');
}

function discoverSuites() {
  return {
    apiModules: discoverApiModules(),
    e2eModules: discoverE2EModules()
  };
}

function normalizeModuleFilter(value) {
  return String(value || '')
    .toLowerCase()
    .replace(/_/g, '-')
    .replace(/\.(test|spec)\.js$/, '');
}

function matchesModule(mod, filters) {
  if (!filters.length) return true;
  return filters.some(filter => {
    const normalized = normalizeModuleFilter(filter);
    return mod.aliases.some(alias => normalizeModuleFilter(alias) === normalized);
  });
}

function selectModules(modules, filters) {
  return modules.filter(mod => matchesModule(mod, filters));
}

function createSelectedModules(args, suites) {
  const selected = {
    apiModulesToRun: [],
    e2eModulesToRun: []
  };

  if (args.e2e) {
    selected.e2eModulesToRun = selectModules(suites.e2eModules, args.modules);
    return selected;
  }

  selected.apiModulesToRun = selectModules(suites.apiModules, args.modules);
  if (args.includeE2e) {
    selected.e2eModulesToRun = selectModules(suites.e2eModules, args.modules);
  }
  return selected;
}

function getPlanTypes(apiModulesToRun, e2eModulesToRun) {
  const types = [];
  if (apiModulesToRun.length) types.push('API');
  if (e2eModulesToRun.length) types.push('E2E');
  return types;
}

function buildExecutionPlan(args, suites = discoverSuites()) {
  const { apiModulesToRun, e2eModulesToRun } = createSelectedModules(args, suites);

  return {
    args,
    apiModulesToRun,
    e2eModulesToRun,
    runMode: args.parallel ? 'parallel' : 'sequential',
    types: getPlanTypes(apiModulesToRun, e2eModulesToRun)
  };
}

module.exports = {
  MODULE_LABELS,
  MODULE_EVIDENCE_LABELS,
  NON_BUSINESS_EVIDENCE_LABELS,
  NON_BUSINESS_EVIDENCE_DESCRIPTIONS,
  getModuleEvidenceLabel,
  isBrowserE2EBusinessClosureCase,
  isApiFixtureOnlyE2ECase,
  getE2EModuleEvidenceLabelFromMetadata,
  getModuleEvidenceLabelFromMetadata,
  getNonBusinessClosureDescription,
  getEvidenceLabelPresentation,
  getReportDisplayName,
  ensureDir,
  keyFromFilename,
  sortByLeadingNumberThenName,
  buildAliases,
  getModuleFileForType,
  getMetadataPathForModule,
  getModuleNameForType,
  createDiscoveredModule,
  discoverModules,
  discoverApiModules,
  discoverE2EModules,
  discoverSuites,
  normalizeModuleFilter,
  matchesModule,
  selectModules,
  createSelectedModules,
  getPlanTypes,
  buildExecutionPlan
};
