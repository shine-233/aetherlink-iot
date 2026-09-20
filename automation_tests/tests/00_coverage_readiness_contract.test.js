/**
 * Pure readiness predicates remain independently testable after extraction.
 * These checks do not start services or claim runtime business closure.
 */
const { expect } = require('chai');
const coverageContract = require('../lib/coverage_contract');
const readiness = require('../lib/coverage-contract/readiness');
const businessCapabilities = require('../lib/coverage-contract/business-capabilities');

describe('Coverage readiness predicates [00_coverage_readiness_contract]', function () {
  it('keeps the facade inventory identity and required unique capabilities', function () {
    expect(coverageContract.BUSINESS_CAPABILITIES).to.equal(businessCapabilities.BUSINESS_CAPABILITIES);

    const capabilityIds = coverageContract.BUSINESS_CAPABILITIES.map(capability => capability.id);
    expect(new Set(capabilityIds).size).to.equal(capabilityIds.length);
    expect(capabilityIds).to.include.members([
      'device-telemetry',
      'command-jobs',
      'visualization',
      'mqtt-broker-pipeline',
      'system-deployment'
    ]);
  });
  it('requires valid inventory and every canonical operation runtime outcome for closure', function () {
    expect(readiness.hasCompleteOperationInventory({ valid: true })).to.equal(true);
    expect(readiness.hasCompleteOperationInventory({ valid: false })).to.equal(false);
    expect(readiness.hasCompleteOperationInventory(null)).to.equal(false);

    const complete = coverageContract.BUSINESS_OPERATIONS.map(item => ({
      id: item.id,
      ready: true,
      staticInventoryValid: true,
      runtimeStatus: 'passed',
      missingDimensions: [],
      runtimeOutcomeErrors: []
    }));
    expect(readiness.hasCompleteOperationCoverage(complete)).to.equal(true);
    expect(readiness.hasCompleteOperationCoverage(complete.slice(1))).to.equal(false);
    expect(readiness.hasCompleteOperationCoverage([
      ...complete,
      { ...complete[0] }
    ])).to.equal(false);
    expect(readiness.hasCompleteOperationCoverage(complete.map((item, index) =>
      index === 0 ? { ...item, staticInventoryValid: false } : item
    ))).to.equal(false);
    expect(readiness.hasCompleteOperationCoverage(complete.map((item, index) =>
      index === 0 ? { ...item, ready: false } : item
    ))).to.equal(false);
    expect(readiness.hasCompleteOperationCoverage([])).to.equal(false);
  });

  it('rejects duplicate catalog and metadata identities', function () {
    const complete = {
      duplicateEndpoints: [],
      duplicateRoutes: [],
      duplicateMetadataFiles: []
    };
    expect(readiness.hasNoCatalogIdentityGaps(complete)).to.equal(true);
    expect(readiness.hasNoCatalogIdentityGaps({
      ...complete,
      duplicateEndpoints: [{ key: 'POST /api/v1/example', count: 2 }]
    })).to.equal(false);
    expect(readiness.hasNoCatalogIdentityGaps({
      ...complete,
      duplicateMetadataFiles: [{ file: 'tests/example.test.js', count: 2 }]
    })).to.equal(false);
    expect(readiness.hasNoCatalogIdentityGaps(null)).to.equal(false);
  });

  it('requires both endpoint and route inventory to be complete', function () {
    expect(readiness.hasCompleteExplicitBusinessInventory({
      missingEndpoints: [],
      missingRoutes: []
    })).to.equal(true);
    expect(readiness.hasCompleteExplicitBusinessInventory({
      missingEndpoints: ['GET /api/v1/example'],
      missingRoutes: []
    })).to.equal(false);
    expect(readiness.hasCompleteExplicitBusinessInventory({
      missingEndpoints: [],
      missingRoutes: ['/example']
    })).to.equal(false);
  });

  it('reports every missing traceability layer without mutating input', function () {
    const complete = {
      id: 'complete',
      hasFrontendRoute: true,
      hasEndpoint: true,
      hasAutomation: true,
      hasTrueAutomation: true,
      hasE2E: true,
      hasTrueE2E: true,
      hasBackendDeclaration: true,
      hasGMQTTDeclaration: true
    };
    const incomplete = { ...complete, id: 'incomplete', hasBackendDeclaration: false };
    const input = [complete, incomplete];
    const result = readiness.getMissingTraceability(input);

    expect(result).to.deep.equal([incomplete]);
    expect(input).to.deep.equal([complete, incomplete]);
  });

  it('supports the deliberate weak-body assertion exception only when requested', function () {
    const audit = {
      seedBlockedReturns: [],
      weakBodyAssertions: ['weak body'],
      weakExistenceAssertions: [],
      weakFlexibleShapeAssertions: [],
      weakObjectOnlyAssertions: [],
      weakBareObjectAssertions: [],
      weakConditionalEmptyAssertions: [],
      weakNullableHelperAssertions: [],
      broadNon200Assertions: [],
      businessE2EFallbackAssertions: [],
      e2eMetadataSourceGaps: [],
      e2eRouteSmokeAssertions: [],
      e2eCurrentStateAssertions: [],
      prohibitedCoverageMarkerTitles: [],
      genericBlockedReasons: []
    };

    expect(readiness.hasNoBusinessAssertionGaps(audit)).to.equal(false);
    expect(readiness.hasNoBusinessAssertionGaps(audit, {
      includeWeakBodyAssertions: false
    })).to.equal(true);
  });

  it('fails closed when source-review boundary evidence is absent or unsafe', function () {
    expect(readiness.hasNoSourceReviewBoundaryGaps(null)).to.equal(false);
    expect(readiness.hasNoSourceReviewBoundaryGaps({
      sourceInventoryDeclaresStaticBoundary: true,
      qualityReviewRejectsRequestWrapperClosure: true,
      qualityReviewRejectsSourceInventoryClosure: true,
      qualityReviewRejectsSmokeClosure: true,
      qualityReviewKeepsReleaseGateOpen: true,
      unsafeClosureClaims: []
    })).to.equal(true);
    expect(readiness.hasNoSourceReviewBoundaryGaps({
      sourceInventoryDeclaresStaticBoundary: true,
      qualityReviewRejectsRequestWrapperClosure: true,
      qualityReviewRejectsSourceInventoryClosure: true,
      qualityReviewRejectsSmokeClosure: true,
      qualityReviewKeepsReleaseGateOpen: true,
      unsafeClosureClaims: [{ line: 1 }]
    })).to.equal(false);
  });
});
