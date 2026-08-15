/**
 * 文件用途：用于支撑 automation_tests 的覆盖率 oracle 契约评估模块。
 * 核心逻辑：封装自动化运行所需的配置、客户端、覆盖率、报告、种子数据或断言能力，供 API 与 E2E 套件复用。
 * 关键注意事项：覆盖率命中只证明执行或访问发生过，不能单独替代业务 oracle 和负向证据。
 * 重构建议：继续按职责拆分深模块，避免把运行配置、业务断言和报告生成耦合在同一入口。
 */

const fs = require('fs');
const path = require('path');

const coverageContract = require('./coverage_contract');

const PROJECT_ROOT = path.resolve(__dirname, '..', '..');
const AUTOMATION_ROOT = path.join(PROJECT_ROOT, 'automation_tests');

const STATEFUL_CAPABILITIES = new Set([
  'device-telemetry',
  'rdi',
  'alarm-notification',
  'automation-scene',
  'command-jobs',
  'visualization',
  'ota-script-openapi-service',
  'mqtt-broker-pipeline',
  'system-deployment'
]);

const MQTT_CAPABILITIES = new Set([
  'device-telemetry',
  'rdi',
  'mqtt-broker-pipeline'
]);

function readText(filePath) {
  return fs.existsSync(filePath) ? fs.readFileSync(filePath, 'utf8') : '';
}

function getCapabilityOracleStatus(capability) {
  const rawAutomationEvidence = capability.automationEvidence || [];
  const automationEvidence = rawAutomationEvidence.filter(item => item.evidenceKind !== 'boundary');
  const negativeAutomationEvidence = rawAutomationEvidence.filter(item =>
    item.evidenceKind === 'business' || item.evidenceKind === 'boundary'
  );
  const e2eEvidence = capability.e2eEvidence || [];
  const businessCases = e2eEvidence.flatMap(item => item.businessCases || []);
  const backendEvidence = capability.backendEvidence || [];
  const gmqttEvidence = capability.gmqttEvidence || [];

  const statusOracle = automationEvidence.some(item => item.hasStatusBodyCase || item.hasNegativeStatusCase);
  const bodyOracle = automationEvidence.some(item => item.hasStatusBodyCase);
  const negativeOracle = negativeAutomationEvidence.some(item =>
    item.hasNegativeStatusCase || item.rawHasNegativeStatusCase
  );
  const stateOracle = !STATEFUL_CAPABILITIES.has(capability.id) ||
    automationEvidence.some(item => item.hasStatefulStatusBodyCase) ||
    businessCases.some(item => item.hasSeedOrApiSetup);
  const userVisibleOracle = capability.e2eTests.length > 0 &&
    businessCases.some(item => item.provesBusinessFlow && item.hasBusinessAssertion);
  const sourceOracle = backendEvidence.some(item => item.exists && item.hasTestFunction);
  const mqttOracle = !MQTT_CAPABILITIES.has(capability.id) ||
    gmqttEvidence.some(item => item.exists && item.hasTestFunction);

  return {
    capability: capability.id,
    priority: capability.priority,
    statusOracle,
    bodyOracle,
    negativeOracle,
    stateOracle,
    userVisibleOracle,
    sourceOracle,
    mqttOracle,
    passed: statusOracle &&
      bodyOracle &&
      negativeOracle &&
      stateOracle &&
      userVisibleOracle &&
      sourceOracle &&
      mqttOracle
  };
}

function getPreviewProxyOracle() {
  const configPath = path.join(AUTOMATION_ROOT, 'playwright.config.js');
  const text = readText(configPath);
  return {
    file: 'playwright.config.js',
    hasPreviewProxyScript: text.includes('serve_preview_with_api_proxy.js'),
    hasUsePreviewProxyEnv: text.includes('PLAYWRIGHT_USE_PREVIEW_PROXY'),
    disablesPreviewReuse: text.includes('PLAYWRIGHT_REUSE_EXISTING_SERVER') &&
      text.includes('!shouldUsePreviewProxy'),
    passed: text.includes('serve_preview_with_api_proxy.js') &&
      text.includes('PLAYWRIGHT_USE_PREVIEW_PROXY') &&
      text.includes('PLAYWRIGHT_REUSE_EXISTING_SERVER') &&
      text.includes('!shouldUsePreviewProxy')
  };
}

function selfCheck() {
  const coverage = coverageContract.selfCheck();
  const capabilityOracles = coverage.traceability.map(getCapabilityOracleStatus);
  const previewProxyOracle = getPreviewProxyOracle();
  const mappedTestFileAudit = coverage.mappedTestFileAudit || [];
  const catalogClassificationAudit = coverage.catalogClassificationAudit || {
    unclassifiedEndpoints: [],
    unclassifiedRoutes: []
  };
  const explicitBusinessInventoryAudit = coverage.explicitBusinessInventoryAudit || {
    missingEndpoints: [],
    missingRoutes: []
  };
  const explicitBusinessInventoryGapReport = coverage.explicitBusinessInventoryGapReport || {
    byCapability: [],
    nextCapability: null
  };
  const missingCapabilityOracles = capabilityOracles.filter(item => !item.passed);
  const missing = [];

  for (const item of missingCapabilityOracles) {
    for (const key of [
      'statusOracle',
      'bodyOracle',
      'negativeOracle',
      'stateOracle',
      'userVisibleOracle',
      'sourceOracle',
      'mqttOracle'
    ]) {
      if (!item[key]) {
        missing.push({ capability: item.capability, missing: key });
      }
    }
  }

  if (!previewProxyOracle.passed) {
    missing.push({ capability: 'preview-deployment', missing: 'previewProxyOracle' });
  }

  for (const item of mappedTestFileAudit) {
    missing.push({
      capability: item.capability,
      missing: 'mappedTestFileOracle',
      layer: item.layer,
      file: item.file,
      exists: item.exists,
      hasTestFunction: item.hasTestFunction
    });
  }

  for (const item of catalogClassificationAudit.unclassifiedEndpoints) {
    missing.push({
      capability: 'business-inventory',
      missing: 'unclassifiedEndpoint',
      endpoint: item.endpoint
    });
  }

  for (const item of catalogClassificationAudit.unclassifiedRoutes) {
    missing.push({
      capability: 'business-inventory',
      missing: 'unclassifiedRoute',
      route: item.route
    });
  }

  for (const item of explicitBusinessInventoryAudit.missingEndpoints) {
    missing.push({
      capability: item.capability,
      missing: 'explicitCapabilityEndpoint',
      endpoint: item.endpoint
    });
  }

  for (const item of explicitBusinessInventoryAudit.missingRoutes) {
    missing.push({
      capability: item.capability,
      missing: 'explicitCapabilityRoute',
      route: item.route
    });
  }

  const staticOracleReady = coverage.trustworthy &&
    coverage.businessClosureReady &&
    coverage.allLayerStructureReady &&
    missing.length === 0;
  const runtimeReleaseEvidenceReady = false;

  return {
    trustworthyCoverageContract: coverage.trustworthy,
    businessClosureReady: coverage.businessClosureReady,
    allLayerStructureReady: coverage.allLayerStructureReady,
    businessAssertionAudit: coverage.businessAssertionAudit,
    frontendWeakAssertionAudit: coverage.frontendWeakAssertionAudit,
    frontendSourceContractAudit: coverage.frontendSourceContractAudit,
    capabilityOracles,
    catalogClassificationAudit,
    explicitBusinessInventoryAudit,
    explicitBusinessInventoryGapReport,
    mappedTestFileAudit,
    previewProxyOracle,
    missing,
    staticOracleReady,
    runtimeReleaseEvidenceReady,
    runtimeReleaseEvidenceStatus: 'not evaluated',
    ready: staticOracleReady && runtimeReleaseEvidenceReady
  };
}

module.exports = {
  getCapabilityOracleStatus,
  getPreviewProxyOracle,
  selfCheck
};
