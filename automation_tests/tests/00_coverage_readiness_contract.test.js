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
      hasBackend: true,
      hasGMQTT: true
    };
    const incomplete = { ...complete, id: 'incomplete', hasBackend: false };
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
