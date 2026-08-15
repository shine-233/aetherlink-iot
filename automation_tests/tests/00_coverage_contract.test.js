/**
 * Coverage harness contract tests.
 *
 * These tests validate the static measuring system. Passing here means the
 * catalog, metadata, oracle, reporter, and weak-assertion guards are wired; it
 * does not prove the backend or browser business flows passed at runtime.
 */
const { expect } = require("chai");
const fs = require("fs");
const os = require("os");
const path = require("path");
const coverageContract = require("../lib/coverage_contract");
const pageCoverage = require("../lib/page_coverage");
const reporter = require("../lib/reporter");
const runner = require("../run_tests");
const testMetadata = require("../lib/test_metadata");
const integrationBlocked = require("../lib/integration_blocked");

describe("Coverage harness contract [00_coverage_contract]", function () {
  it("preserves numeric-looking exception routes in page coverage", function () {
    for (const route of ["/403", "/404", "/500"]) {
      expect(pageCoverage.normalizeRoute(route)).to.equal(route);
    }
    expect(pageCoverage.normalizeRoute("/device/details/123")).to.equal(
      "/device/details/:id",
    );
    expect(pageCoverage.normalizeRoute("/tv-preview?id=dashboard-1")).to.equal(
      "/visualization/thingsvis-preview",
    );
  });

  it("keeps the page catalog aligned with generated frontend routes", function () {
    const comparison = coverageContract.comparePageCatalogToSource();
    expect(comparison.sourceRoutes.length).to.be.at.least(45);
    expect(comparison.missingFromCatalog).to.deep.equal([]);
    expect(comparison.extraInCatalog).to.deep.equal([]);
  });

  it("keeps endpoint catalog route shapes aligned and reports parameter aliases separately", function () {
    const comparison = coverageContract.compareEndpointCatalogToSource();
    expect(comparison.missingFromCatalog).to.deep.equal([]);
    expect(comparison.extraInCatalog).to.deep.equal([]);
    expect(comparison.parameterNameMismatches).to.have.length.greaterThan(0);
    for (const mismatch of comparison.parameterNameMismatches) {
      expect(mismatch.source).to.not.equal(mismatch.catalog);
      expect(mismatch.shape).to.equal(
        mismatch.source.replace(/:[^/\s]+/g, ":param"),
      );
      expect(mismatch.shape).to.equal(
        mismatch.catalog.replace(/:[^/\s]+/g, ":param"),
      );
    }
    expect(comparison.catalogEndpoints).to.include.members([
      "GET /api/v1/command/datas/saved-filters",
      "POST /api/v1/payload-schema/validate",
      "GET /api/v1/device/twin-drift",
      "GET /api/v1/telemetry/datas/uplink-dead-letters",
      "POST /api/v1/reset/password/link",
    ]);
  });

  it("keeps mapped P0/P1 capability endpoints in the endpoint catalog", function () {
    const missing = coverageContract.compareEndpointCapabilityMap();
    expect(missing).to.deep.equal([]);
  });

  it("keeps command delivery diagnostics classified before generic command data routes", function () {
    expect(
      coverageContract.classifyEndpointCatalogItem(
        "GET /api/v1/command/datas/delivery/diagnostics/:device_id",
      ),
    ).to.deep.equal({
      endpoint: "GET /api/v1/command/datas/delivery/diagnostics/:device_id",
      scope: "P0/P1",
      capability: "command-jobs",
    });
    expect(
      coverageContract.classifyEndpointCatalogItem(
        "GET /api/v1/command/datas/:id",
      ),
    ).to.deep.equal({
      endpoint: "GET /api/v1/command/datas/:id",
      scope: "P0/P1",
      capability: "device-telemetry",
    });
    const traceability = coverageContract.getBusinessTraceability();
    const commandJobs = traceability.find((item) => item.id === "command-jobs");
    const deviceTelemetry = traceability.find(
      (item) => item.id === "device-telemetry",
    );
    expect(commandJobs.endpoints).to.include(
      "GET /api/v1/command/datas/delivery/diagnostics/:device_id",
    );
    expect(deviceTelemetry.endpoints).not.to.include(
      "GET /api/v1/command/datas/delivery/diagnostics/:device_id",
    );
  });

  it("keeps deployment health endpoints classified as system deployment evidence", function () {
    expect(
      coverageContract.classifyEndpointCatalogItem("GET /deployment/health"),
    ).to.deep.equal({
      endpoint: "GET /deployment/health",
      scope: "P0/P1",
      capability: "system-deployment",
    });
    expect(
      coverageContract.classifyEndpointCatalogItem(
        "GET /api/v1/deployment/health",
      ),
    ).to.deep.equal({
      endpoint: "GET /api/v1/deployment/health",
      scope: "P0/P1",
      capability: "system-deployment",
    });
  });

  it("classifies monthly alarm history as alarm-notification coverage", function () {
    expect(
      coverageContract.classifyEndpointCatalogItem(
        "GET /api/v1/alarm/info/history/monthly",
      ),
    ).to.deep.equal({
      endpoint: "GET /api/v1/alarm/info/history/monthly",
      scope: "P0/P1",
      capability: "alarm-notification",
    });
  });

  it("classifies telemetry dead-letter operator routes as device-telemetry coverage", function () {
    const endpoints = [
      "GET /api/v1/telemetry/datas/dead-letters",
      "POST /api/v1/telemetry/datas/dead-letters/drain",
      "PATCH /api/v1/telemetry/datas/dead-letters/:id/status",
      "GET /api/v1/telemetry/datas/uplink-dead-letters",
      "POST /api/v1/telemetry/datas/uplink-dead-letters/drain",
      "PATCH /api/v1/telemetry/datas/uplink-dead-letters/:id/status",
    ];

    for (const endpoint of endpoints) {
      expect(
        coverageContract.classifyEndpointCatalogItem(endpoint),
      ).to.deep.equal({
        endpoint,
        scope: "P0/P1",
        capability: "device-telemetry",
      });
    }
  });

  it("maps each P0/P1 capability and does not count boundary tests as business automation", function () {
    const check = coverageContract.selfCheck();
    const automationScene = check.traceability.find(
      (item) => item.id === "automation-scene",
    );
    const otaScriptOpenapiService = check.traceability.find(
      (item) => item.id === "ota-script-openapi-service",
    );
    const commandJobs = check.traceability.find(
      (item) => item.id === "command-jobs",
    );
    const permissionTenancy = check.traceability.find(
      (item) => item.id === "permission-tenancy",
    );
    const visualization = check.traceability.find(
      (item) => item.id === "visualization",
    );
    const mqttPipeline = check.traceability.find(
      (item) => item.id === "mqtt-broker-pipeline",
    );
    const systemDeployment = check.traceability.find(
      (item) => item.id === "system-deployment",
    );

    expect(check.missingTraceability.map((item) => item.id)).to.deep.equal([]);
    expect(commandJobs).to.include({
      hasAutomation: true,
      hasTrueAutomation: true,
      hasE2E: true,
      hasTrueE2E: true,
    });
    expect(
      commandJobs.automationEvidence.map((item) => ({
        file: item.file,
        evidenceKind: item.evidenceKind,
        businessClosureEvidence: item.businessClosureEvidence,
        hasStatusBodyCase: item.hasStatusBodyCase,
        hasStatefulStatusBodyCase: item.hasStatefulStatusBodyCase,
        hasNegativeStatusCase: item.hasNegativeStatusCase,
      })),
    ).to.deep.equal([
      {
        file: "tests/25_seeded_command_jobs.test.js",
        evidenceKind: "business",
        businessClosureEvidence: true,
        hasStatusBodyCase: true,
        hasStatefulStatusBodyCase: true,
        hasNegativeStatusCase: true,
      },
    ]);
    expect(permissionTenancy).to.include({
      hasAutomation: true,
      hasTrueAutomation: true,
      hasE2E: true,
      hasTrueE2E: true,
    });
    expect(
      permissionTenancy.automationEvidence
        .filter((item) => item.businessClosureEvidence)
        .filter((item) =>
          item.oracleCases.some((testCase) =>
            testCase.capabilityIds.includes("permission-tenancy"),
          ),
        )
        .map((item) => ({
          file: item.file,
          evidenceKind: item.evidenceKind,
          businessClosureEvidence: item.businessClosureEvidence,
          hasStatusBodyCase: item.hasStatusBodyCase,
          hasStatefulStatusBodyCase: item.hasStatefulStatusBodyCase,
          hasNegativeStatusCase: item.hasNegativeStatusCase,
          titles: item.oracleCases.map((testCase) => testCase.title),
        })),
    ).to.deep.equal([
      {
        file: "tests/20_seeded_system_permission.test.js",
        evidenceKind: "business",
        businessClosureEvidence: true,
        hasStatusBodyCase: true,
        hasStatefulStatusBodyCase: false,
        hasNegativeStatusCase: true,
        titles: ["refreshes token and rejects protected endpoints without auth"],
      },
    ]);
    expect(
      permissionTenancy.e2eEvidence
        .flatMap((item) => item.businessCases)
        .map((item) => ({
          title: item.title,
          evidenceLayer: item.evidenceLayer,
          capabilityIds: item.capabilityIds,
          runtimeEvidenceRequired: item.runtimeEvidenceRequired,
        })),
    ).to.deep.equal([
      {
        title: "super admin login reaches the authenticated shell",
        evidenceLayer: "browser-e2e-with-api-setup",
        capabilityIds: ["permission-tenancy"],
        runtimeEvidenceRequired: true,
      },
      {
        title: "tenant admin login reaches the authenticated shell",
        evidenceLayer: "browser-e2e-with-api-setup",
        capabilityIds: ["permission-tenancy"],
        runtimeEvidenceRequired: true,
      },
      {
        title: "second tenant admin login reaches the authenticated shell",
        evidenceLayer: "browser-e2e-with-api-setup",
        capabilityIds: ["permission-tenancy"],
        runtimeEvidenceRequired: true,
      },
      {
        title: "tenant user login reaches the authenticated shell",
        evidenceLayer: "browser-e2e-with-api-setup",
        capabilityIds: ["permission-tenancy"],
        runtimeEvidenceRequired: true,
      },
      {
        title: "additional tenant user login reaches the authenticated shell",
        evidenceLayer: "browser-e2e-with-api-setup",
        capabilityIds: ["permission-tenancy"],
        runtimeEvidenceRequired: true,
      },
      {
        title: "email-change tenant login reaches the authenticated shell",
        evidenceLayer: "browser-e2e-with-api-setup",
        capabilityIds: ["permission-tenancy"],
        runtimeEvidenceRequired: true,
      },
      {
        title: "super-admin tenant list in the browser matches authenticated user API state",
        evidenceLayer: "browser-e2e-with-api-setup",
        capabilityIds: ["permission-tenancy"],
        runtimeEvidenceRequired: true,
      },
      {
        title: "personal-center renders the authenticated profile returned by the API",
        evidenceLayer: "browser-e2e-with-api-setup",
        capabilityIds: ["permission-tenancy"],
        runtimeEvidenceRequired: true,
      },
      {
        title: "management/auth renders a menu element that is present in the API payload",
        evidenceLayer: "browser-e2e-with-api-setup",
        capabilityIds: ["permission-tenancy"],
        runtimeEvidenceRequired: true,
      },
    ]);
    expect(
      testMetadata.getCaseMetadata(
        "e2e/09_management.spec.js",
        "super-admin role API access is separated from its unassigned UI route permission",
      ),
    ).to.include({
      evidenceKind: "boundary",
      businessClosureEvidence: false,
      provesBusinessFlow: true,
    });
    expect(visualization).to.include({
      hasAutomation: true,
      hasTrueAutomation: true,
      hasE2E: true,
      hasTrueE2E: true,
    });
    expect(
      visualization.automationEvidence
        .filter((item) => item.businessClosureEvidence)
        .map((item) => ({
          file: item.file,
          evidenceKind: item.evidenceKind,
          businessClosureEvidence: item.businessClosureEvidence,
          hasStatusBodyCase: item.hasStatusBodyCase,
          hasStatefulStatusBodyCase: item.hasStatefulStatusBodyCase,
          hasNegativeStatusCase: item.hasNegativeStatusCase,
          titles: item.oracleCases.map((testCase) => testCase.title),
        })),
    ).to.deep.equal([
      {
        file: "tests/07_board.test.js",
        evidenceKind: "business",
        businessClosureEvidence: true,
        hasStatusBodyCase: true,
        hasStatefulStatusBodyCase: true,
        hasNegativeStatusCase: true,
        titles: [
          "returns the tenant board list with list and total fields",
          "creates a tenant board with the current local payload shape",
          "updates the created board",
          "rejects tenant overview for tenant_admin in the current local deployment",
        ],
      },
    ]);
    expect(otaScriptOpenapiService).to.include({
      hasE2E: true,
      hasTrueE2E: true,
    });
    expect(mqttPipeline).to.include({
      hasAutomation: true,
      hasTrueAutomation: true,
      hasE2E: true,
      hasTrueE2E: true,
    });
    expect(
      mqttPipeline.automationEvidence
        .filter((item) => item.businessClosureEvidence)
        .map((item) => ({
          file: item.file,
          evidenceKind: item.evidenceKind,
          businessClosureEvidence: item.businessClosureEvidence,
          hasStatusBodyCase: item.hasStatusBodyCase,
          hasStatefulStatusBodyCase: item.hasStatefulStatusBodyCase,
          hasNegativeStatusCase: item.hasNegativeStatusCase,
          titles: item.oracleCases.map((testCase) => testCase.title),
        })),
    ).to.deep.equal([
      {
        file: "tests/22_mqtt_device_pipeline.test.js",
        evidenceKind: "business",
        businessClosureEvidence: true,
        hasStatusBodyCase: true,
        hasStatefulStatusBodyCase: true,
        hasNegativeStatusCase: true,
        titles: [
          "validates backend publish/simulation API request shape for MQTT uplink and downlink",
          "publishes unique simulated telemetry and reads the same current value back",
        ],
      },
    ]);
    expect(
      mqttPipeline.e2eEvidence
        .flatMap((item) => item.businessCases)
        .map((item) => ({
          title: item.title,
          evidenceLayer: item.evidenceLayer,
          capabilityIds: item.capabilityIds,
          requiresSeededDevice: item.requiresSeededDevice,
          runtimeEvidenceRequired: item.runtimeEvidenceRequired,
        })),
    ).to.deep.equal([
      {
        title: "Ready Check shows uniquely published MQTT telemetry evidence",
        evidenceLayer: "browser-e2e-with-api-setup",
        capabilityIds: ["mqtt-broker-pipeline"],
        requiresSeededDevice: true,
        runtimeEvidenceRequired: true,
      },
    ]);
    expect(systemDeployment).to.include({
      hasAutomation: true,
      hasTrueAutomation: true,
      hasE2E: true,
      hasTrueE2E: true,
    });
    expect(
      systemDeployment.automationEvidence
        .filter((item) => item.businessClosureEvidence)
        .map((item) => ({
          file: item.file,
          evidenceKind: item.evidenceKind,
          businessClosureEvidence: item.businessClosureEvidence,
          hasStatusBodyCase: item.hasStatusBodyCase,
          hasStatefulStatusBodyCase: item.hasStatefulStatusBodyCase,
          hasNegativeStatusCase: item.hasNegativeStatusCase,
          titles: item.oracleCases.map((testCase) => testCase.title),
        })),
    ).to.deep.equal([
      {
        file: "tests/06_system.test.js",
        evidenceKind: "business",
        businessClosureEvidence: true,
        hasStatusBodyCase: true,
        hasStatefulStatusBodyCase: true,
        hasNegativeStatusCase: true,
        titles: [
          "creates a UI element and verifies it appears in the list",
          "rejects updates for a non-existent UI element id",
          "re-saves the current logo payload without changing values",
        ],
      },
    ]);
    expect(
      systemDeployment.e2eEvidence
        .flatMap((item) => item.businessCases)
        .map((item) => ({
          title: item.title,
          evidenceLayer: item.evidenceLayer,
          capabilityIds: item.capabilityIds,
          runtimeEvidenceRequired: item.runtimeEvidenceRequired,
        })),
    ).to.deep.equal([
      {
        title: "data cleanup policy edited in the browser persists through the API and refreshed table",
        evidenceLayer: "browser-e2e-with-api-setup",
        capabilityIds: ["system-deployment"],
        runtimeEvidenceRequired: true,
      },
      {
        title: "system log path filter sends an exact API query and renders the empty result",
        evidenceLayer: "browser-e2e-with-api-setup",
        capabilityIds: ["system-deployment"],
        runtimeEvidenceRequired: true,
      },
    ]);
    expect(
      mqttPipeline.automationEvidence.map((item) => ({
        file: item.file,
        evidenceKind: item.evidenceKind,
        evidenceSource: item.evidenceSource,
        businessClosureEvidence: item.businessClosureEvidence,
        rawHasStatusBodyCase: item.rawHasStatusBodyCase,
        rawHasStatefulStatusBodyCase: item.rawHasStatefulStatusBodyCase,
        rawHasNegativeStatusCase: item.rawHasNegativeStatusCase,
        hasStatusBodyCase: item.hasStatusBodyCase,
        hasStatefulStatusBodyCase: item.hasStatefulStatusBodyCase,
        hasNegativeStatusCase: item.hasNegativeStatusCase,
      })),
    ).to.deep.equal([
      {
        file: "tests/22_mqtt_device_pipeline.test.js",
        evidenceKind: "business",
        evidenceSource: "test-metadata",
        businessClosureEvidence: true,
        rawHasStatusBodyCase: true,
        rawHasStatefulStatusBodyCase: true,
        rawHasNegativeStatusCase: true,
        hasStatusBodyCase: true,
        hasStatefulStatusBodyCase: true,
        hasNegativeStatusCase: true,
      },
    ]);
    expect(automationScene).to.include({
      hasAutomation: true,
      hasTrueAutomation: true,
    });
    expect(
      automationScene.automationEvidence.map((item) => ({
        file: item.file,
        evidenceKind: item.evidenceKind,
        evidenceSource: item.evidenceSource,
        businessClosureEvidence: item.businessClosureEvidence,
        hasStatusBodyCase: item.hasStatusBodyCase,
        hasStatefulStatusBodyCase: item.hasStatefulStatusBodyCase,
        hasNegativeStatusCase: item.hasNegativeStatusCase,
        rawHasStatusBodyCase: item.rawHasStatusBodyCase,
        rawHasStatefulStatusBodyCase: item.rawHasStatefulStatusBodyCase,
        rawHasNegativeStatusCase: item.rawHasNegativeStatusCase,
      })),
    ).to.deep.equal([
      {
        file: "tests/23_seeded_automation_scene.test.js",
        evidenceKind: "business",
        evidenceSource: "test-metadata",
        businessClosureEvidence: true,
        hasStatusBodyCase: true,
        hasStatefulStatusBodyCase: true,
        hasNegativeStatusCase: false,
        rawHasStatusBodyCase: true,
        rawHasStatefulStatusBodyCase: true,
        rawHasNegativeStatusCase: true,
      },
      {
        file: "tests/24_seeded_scene_automations.test.js",
        evidenceKind: "business",
        evidenceSource: "test-metadata",
        businessClosureEvidence: true,
        hasStatusBodyCase: true,
        hasStatefulStatusBodyCase: true,
        hasNegativeStatusCase: true,
        rawHasStatusBodyCase: true,
        rawHasStatefulStatusBodyCase: true,
        rawHasNegativeStatusCase: true,
      },
      {
        file: "tests/17_api_coverage_closure.test.js",
        evidenceKind: "boundary",
        evidenceSource: "explicit-entry",
        businessClosureEvidence: false,
        hasStatusBodyCase: false,
        hasStatefulStatusBodyCase: false,
        hasNegativeStatusCase: false,
        rawHasStatusBodyCase: true,
        rawHasStatefulStatusBodyCase: true,
        rawHasNegativeStatusCase: true,
      },
    ]);
    expect(check.traceability.map((item) => item.id)).to.include.members([
      "device-telemetry",
      "rdi",
      "alarm-notification",
      "permission-tenancy",
      "mqtt-broker-pipeline",
    ]);
  });

  it("keeps automation quality gates from hiding fake or unclassified coverage", function () {
    const check = coverageContract.selfCheck();
    const explicitBusinessInventoryComplete =
      check.explicitBusinessInventoryAudit.missingEndpoints.length === 0 &&
      check.explicitBusinessInventoryAudit.missingRoutes.length === 0;
    const expectedStructuredBusinessEvidence =
      check.catalogClassificationAudit.unclassifiedEndpoints.length === 0 &&
      check.catalogClassificationAudit.unclassifiedRoutes.length === 0 &&
      explicitBusinessInventoryComplete &&
      check.mappedTestFileAudit.length === 0 &&
      check.businessAssertionAudit.seedBlockedReturns.length === 0 &&
      check.businessAssertionAudit.weakBodyAssertions.length === 0 &&
      check.businessAssertionAudit.weakExistenceAssertions.length === 0 &&
      check.businessAssertionAudit.weakFlexibleShapeAssertions.length === 0 &&
      check.businessAssertionAudit.weakObjectOnlyAssertions.length === 0 &&
      check.businessAssertionAudit.weakBareObjectAssertions.length === 0 &&
      check.businessAssertionAudit.weakConditionalEmptyAssertions.length ===
        0 &&
      check.businessAssertionAudit.weakNullableHelperAssertions.length === 0 &&
      check.businessAssertionAudit.broadNon200Assertions.length === 0 &&
      check.businessAssertionAudit.businessE2EFallbackAssertions.length === 0 &&
      check.businessAssertionAudit.e2eRouteSmokeAssertions.length === 0 &&
      check.businessAssertionAudit.e2eCurrentStateAssertions.length === 0 &&
      check.businessAssertionAudit.prohibitedCoverageMarkerTitles.length ===
        0 &&
      check.businessAssertionAudit.genericBlockedReasons.length === 0;
    const expectedBatchStructureReady =
      check.blockedReasonAudit.seedableReasons.length === 0 &&
      expectedStructuredBusinessEvidence;
    const expectedBusinessClosureReady =
      check.skipAudit.explicitBlockedHelpers === 0 &&
      expectedStructuredBusinessEvidence &&
      check.missingTraceability.length === 0;

    expect(check.skipAudit.rawMochaSkips).to.deep.equal([]);
    expect(check.skipAudit.rawPlaywrightSkips).to.deep.equal([]);
    expect(
      check.businessAssertionAudit.prohibitedCoverageMarkerTitles,
    ).to.deep.equal([]);
    expect(check.businessAssertionAudit.genericBlockedReasons).to.deep.equal(
      [],
    );
    expect(check.businessAssertionAudit.broadNon200Assertions).to.deep.equal(
      [],
    );
    expect(
      check.businessAssertionAudit.businessE2EFallbackAssertions,
    ).to.deep.equal([]);
    expect(check.seedableBlockedHelpers).to.equal(0);
    expect(check.batchStructureReady).to.equal(expectedBatchStructureReady);
    expect(check.businessClosureReady).to.equal(expectedBusinessClosureReady);
    expect(check.allLayerStructureReady).to.equal(
      expectedBatchStructureReady &&
        check.goSourceStringContractAudit.length === 0 &&
        check.frontendWeakAssertionAudit.length === 0,
    );
  });

  it("classifies frontend source-string tests separately from runtime weak assertions", function () {
    const check = coverageContract.selfCheck();
    const sourceContractFiles = check.frontendSourceContractAudit.map(item => item.file);

    expect(sourceContractFiles).to.include.members([
      "src/__tests__/nginx-lightweight-contract.test.ts",
      "src/views/device/manage/__tests__/device-search-keys.test.ts",
    ]);
    expect(check.frontendSourceContractAudit).to.satisfy(items =>
      items.every(item => item.category === "source-contract"),
    );
    expect(check.frontendWeakAssertionAudit).to.deep.equal([]);
  });

  it("classifies harness-only and boundary API modules as non-business evidence", function () {
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/17_api_coverage_closure.test.js",
      ),
    ).to.equal("boundary");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/17_api_boundary_smoke.test.js",
      ),
    ).to.equal("boundary");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/00_endpoint_coverage.test.js",
      ),
    ).to.equal("catalog");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/00_coverage_contract.test.js",
      ),
    ).to.equal("contract");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/00_oracle_contract.test.js",
      ),
    ).to.equal("contract");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/00_runtime_config_env.test.js",
      ),
    ).to.equal("config");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/00_preflight_api_e2e.test.js",
      ),
    ).to.equal("preflight");
    expect(
      coverageContract.getAutomationEvidenceKind("tests/02_device.test.js"),
    ).to.equal("business");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/16_device_alarm_share.test.js",
      ),
    ).to.equal("business");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/18_seeded_device_data.test.js",
      ),
    ).to.equal("boundary");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/19_seeded_alarm_notification.test.js",
      ),
    ).to.equal("business");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/20_seeded_system_permission.test.js",
      ),
    ).to.equal("business");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/21_seeded_ota_script_openapi.test.js",
      ),
    ).to.equal("business");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/22_mqtt_device_pipeline.test.js",
      ),
    ).to.equal("business");
    expect(
      [
        "tests/00_endpoint_coverage.test.js",
        "tests/00_coverage_contract.test.js",
        "tests/00_oracle_contract.test.js",
        "tests/00_runtime_config_env.test.js",
        "tests/00_preflight_api_e2e.test.js",
        "tests/02_device.test.js",
        "tests/16_device_alarm_share.test.js",
        "tests/17_api_coverage_closure.test.js",
        "tests/18_seeded_device_data.test.js",
        "tests/19_seeded_alarm_notification.test.js",
        "tests/20_seeded_system_permission.test.js",
        "tests/21_seeded_ota_script_openapi.test.js",
        "tests/22_mqtt_device_pipeline.test.js",
      ].map((file) => coverageContract.getAutomationEvidenceMetadata(file)),
    ).to.deep.equal([
      { evidenceKind: "catalog", evidenceSource: "test-metadata" },
      { evidenceKind: "contract", evidenceSource: "test-metadata" },
      { evidenceKind: "contract", evidenceSource: "test-metadata" },
      { evidenceKind: "config", evidenceSource: "test-metadata" },
      { evidenceKind: "preflight", evidenceSource: "test-metadata" },
      { evidenceKind: "business", evidenceSource: "test-metadata" },
      { evidenceKind: "business", evidenceSource: "test-metadata" },
      { evidenceKind: "boundary", evidenceSource: "test-metadata" },
      { evidenceKind: "boundary", evidenceSource: "test-metadata" },
      { evidenceKind: "business", evidenceSource: "test-metadata" },
      { evidenceKind: "business", evidenceSource: "test-metadata" },
      { evidenceKind: "business", evidenceSource: "test-metadata" },
      { evidenceKind: "business", evidenceSource: "test-metadata" },
    ]);
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/23_seeded_automation_scene.test.js",
      ),
    ).to.equal("business");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "e2e/14_route_coverage_closure.spec.js",
      ),
    ).to.equal("business");
    expect(
      coverageContract.getAutomationEvidenceKind(
        "e2e/15_apply_marketplace.spec.js",
      ),
    ).to.equal("boundary");
    expect(
      coverageContract.getAutomationEvidenceKind({
        file: "tests/23_seeded_automation_scene.test.js",
        evidenceKind: "typo",
      }),
    ).to.equal("unknown");
    expect(
      coverageContract.getAutomationEvidenceKindDetails("business"),
    ).to.include({
      businessClosureEvidence: true,
    });
    expect(
      coverageContract.getAutomationEvidenceKindDetails("boundary"),
    ).to.include({
      businessClosureEvidence: false,
    });
    expect(
      coverageContract.getAutomationEvidenceKindDetails("unknown"),
    ).to.include({
      businessClosureEvidence: false,
    });
  });

  it("keeps P0 API metadata case-level so seeded and MQTT files cannot fake business closure", function () {
    const apiCases = (file) =>
      coverageContract.getAutomationOracleCases(
        fs.readFileSync(path.join(__dirname, file), "utf8"),
        `tests/${file}`,
      );
    const businessCaseTitles = (file) =>
      apiCases(file)
        .filter((item) => item.businessClosureEvidence)
        .map((item) => item.title);

    expect(
      apiCases("17_api_coverage_closure.test.js").every(
        (item) => item.businessClosureEvidence === false,
      ),
    ).to.equal(true);
    expect(
      apiCases("14_attribute_command_event.test.js").every(
        (item) => item.businessClosureEvidence === false,
      ),
    ).to.equal(true);
    expect(
      apiCases("14_attribute_command_event.test.js").map((item) => item.title),
    ).to.deep.equal([
      "returns the current local attribute payload for the tenant device",
      "returns attribute set logs with count and list fields",
      "returns record-not-found for invalid attribute deletion",
      "returns command set logs for the tenant device",
      "returns command data as an array for the tenant device",
      "returns event data with count and list for the tenant device",
    ]);
    expect(
      coverageContract.getAutomationEvidenceMetadata(
        "tests/14_attribute_command_event.test.js",
      ),
    ).to.deep.equal({
      evidenceKind: "boundary",
      evidenceSource: "test-metadata",
    });
    expect(
      apiCases("20_seeded_system_permission.test.js")
        .filter((item) => item.businessClosureEvidence)
        .map((item) => ({
          title: item.title,
          evidenceKind: item.evidenceKind,
          capabilityIds: item.capabilityIds,
          hasStatusBodyCase: item.hasStatusBodyCase,
          hasNegativeStatusCase: item.hasNegativeStatusCase,
        })),
    ).to.deep.equal([
      {
        title: "refreshes token and rejects protected endpoints without auth",
        evidenceKind: "business",
        capabilityIds: ["permission-tenancy"],
        hasStatusBodyCase: true,
        hasNegativeStatusCase: true,
      },
    ]);
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/20_seeded_system_permission.test.js",
      ),
    ).to.equal("business");
    expect(
      apiCases("22_mqtt_device_pipeline.test.js").filter(
        (item) => item.evidenceKind === "boundary",
      ),
    ).to.deep.include({
      title: "ties seeded device identity to connect info and debug endpoints",
      capabilityIds: ["device-telemetry", "mqtt-broker-pipeline"],
      hasExactStatusAssertion: true,
      hasBodyAssertion: true,
      hasMutationOrSeedAction: true,
      hasNegativeAssertion: true,
      hasStatusBodyCase: true,
      hasStatefulStatusBodyCase: true,
      hasNegativeStatusCase: true,
      evidenceKind: "boundary",
      businessClosureEvidence: false,
    });
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/22_mqtt_device_pipeline.test.js",
      ),
    ).to.equal("business");

    expect(businessCaseTitles("18_seeded_device_data.test.js")).to.deep.equal([
      "sets desired twin state and reads platform-visible convergence evidence",
      "walks seeded device onboarding guide through access, readiness, diagnostics, and next-step evidence",
    ]);
    expect(businessCaseTitles("14_device_config.test.js")).to.deep.equal([
      "covers TC-DCFG-011 with concrete API assertions",
      "covers TC-DCFG-012 with concrete API assertions",
      "covers TC-DCFG-013 with concrete API assertions",
    ]);
    expect(
      coverageContract.getAutomationEvidenceKind(
        "tests/14_device_config.test.js",
      ),
    ).to.equal("boundary");
    expect(
      businessCaseTitles("19_seeded_alarm_notification.test.js"),
    ).to.deep.equal([
      "batch-acknowledges and batch-resets a seeded alarm history with detail readback",
      "activates a seeded alarm scene and verifies acknowledge/reset history readback",
      "uses seed helper for notification groups and verifies the created row",
    ]);
    expect(
      businessCaseTitles("21_seeded_ota_script_openapi.test.js"),
    ).to.deep.equal([
      "returns OTA task support archive with task-level rollout counts and conditional Ready Check handoff fields",
      "uses seeded OpenAPI key helper and verifies the created key appears in list results",
    ]);
    expect(
      businessCaseTitles("22_mqtt_device_pipeline.test.js"),
    ).to.deep.equal([
      "validates backend publish/simulation API request shape for MQTT uplink and downlink",
      "publishes unique simulated telemetry and reads the same current value back",
    ]);
  });

  it("filters case-level P0 API metadata by capability so mixed files cannot inflate traceability", function () {
    const traceability = coverageContract.getBusinessTraceability();
    const byId = (id) => traceability.find((item) => item.id === id);
    const rawTitlesFor = (capabilityId, file) => {
      const capability = byId(capabilityId);
      const evidence = capability.automationEvidence.find(
        (item) => item.file === file,
      );
      return evidence ? evidence.rawOracleCases.map((item) => item.title) : [];
    };
    const businessTitlesFor = (capabilityId, file) => {
      const capability = byId(capabilityId);
      const evidence = capability.automationEvidence.find(
        (item) => item.file === file,
      );
      return evidence ? evidence.oracleCases.map((item) => item.title) : [];
    };

    expect(
      rawTitlesFor("device-telemetry", "tests/02_device.test.js"),
    ).to.not.include("creates a share token and exposes the public share path");
    expect(rawTitlesFor("rdi", "tests/02_device.test.js")).to.not.include(
      "creates a device template and exposes it in the stats endpoint",
    );
    expect(
      rawTitlesFor("device-telemetry", "tests/17_api_coverage_closure.test.js"),
    ).to.deep.equal([
      "rejects model list requests without a device template id",
      "rejects incomplete model create and update payloads",
      "rejects delete and custom command detail requests for non-existent model ids",
    ]);
    expect(
      rawTitlesFor("device-telemetry", "tests/25_seeded_command_jobs.test.js"),
    ).to.deep.equal([]);
    expect(
      businessTitlesFor("command-jobs", "tests/25_seeded_command_jobs.test.js"),
    ).to.deep.equal([
      "creates, lists, updates, and deletes an owned fleet saved filter",
      "returns seeded command delivery diagnostics with operator next actions",
      "filters and searches command job rows with response evidence shape",
      "previews, submits, refreshes, and packages a seeded command job",
    ]);
    expect(
      businessTitlesFor("visualization", "tests/07_board.test.js"),
    ).to.deep.equal([
      "returns the tenant board list with list and total fields",
      "creates a tenant board with the current local payload shape",
      "updates the created board",
      "rejects tenant overview for tenant_admin in the current local deployment",
    ]);
    expect(
      businessTitlesFor("permission-tenancy", "tests/07_board.test.js"),
    ).to.deep.equal([]);
    expect(
      businessTitlesFor("system-deployment", "tests/06_system.test.js"),
    ).to.deep.equal([
      "creates a UI element and verifies it appears in the list",
      "rejects updates for a non-existent UI element id",
      "re-saves the current logo payload without changing values",
    ]);
    expect(
      businessTitlesFor("mqtt-broker-pipeline", "tests/22_mqtt_device_pipeline.test.js"),
    ).to.deep.equal([
      "validates backend publish/simulation API request shape for MQTT uplink and downlink",
      "publishes unique simulated telemetry and reads the same current value back",
    ]);
    expect(
      businessTitlesFor("rdi", "tests/22_mqtt_device_pipeline.test.js"),
    ).to.deep.equal([]);
    expect(
      businessTitlesFor("visualization", "tests/06_system.test.js"),
    ).to.deep.equal([]);
    expect(
      rawTitlesFor(
        "ota-script-openapi-service",
        "tests/17_api_coverage_closure.test.js",
      ),
    ).to.deep.equal([
      "returns service and access lists",
      "rejects incomplete service and service access mutations",
    ]);
    expect(
      rawTitlesFor(
        "system-deployment",
        "tests/20_seeded_system_permission.test.js",
      ),
    ).to.deep.equal([
      "covers system identity, function flags, logo, and version read APIs",
    ]);
  });

  it("keeps P1 E2E metadata from promoting page smoke into business flows", function () {
    const e2eBusinessCases = (file) =>
      coverageContract
        .getE2EBusinessCases(
          fs.readFileSync(path.join(__dirname, "..", "e2e", file), "utf8"),
          `e2e/${file}`,
        );
    const e2eBusinessTitles = (file) =>
      e2eBusinessCases(file).map((item) => item.title);

    expect(e2eBusinessTitles("02_device.spec.js")).to.deep.equal([
      "seeded device search matches API state and opens the selected device",
      "manual add submits a real device and exposes it through list and detail APIs",
      "created thing model is searchable in UI and matches API state",
      "created device group is searchable in UI and opens matching details",
      "share page distinguishes valid, invalid, and empty token states",
      "Ready Check shows uniquely published MQTT telemetry evidence",
      "share link acceptance closes the loop when a recipient tenant is available",
    ]);
    const mqttReadyCheckCase = e2eBusinessCases("02_device.spec.js").find(
      (item) =>
        item.title === "Ready Check shows uniquely published MQTT telemetry evidence",
    );
    expect(mqttReadyCheckCase).to.include({
      businessClosureEvidence: true,
      evidenceLayer: "browser-e2e-with-api-setup",
      hasBrowserUserFlow: true,
      requiresSeededDevice: true,
      runtimeEvidenceRequired: true,
    });
    expect(mqttReadyCheckCase.capabilityIds).to.include("mqtt-broker-pipeline");
    expect(e2eBusinessTitles("03_data.spec.js")).to.deep.equal([
      "new device telemetry tab renders the same empty snapshot returned by the API",
      "device twin tab downloads platform-visible evidence bundle for a seeded desired state",
      "first-device Ready Check downloads diagnostics bundle with connection-guide evidence",
      "seeded device config detail renders its API identity and loads data processing for that config",
      "seeded thing model is searchable in the UI and matches list and detail APIs",
      "share link shows owner success and revoked-token error states backed by the API",
    ]);
    expect(
      testMetadata.getCaseMetadata(
        "e2e/03_data.spec.js",
        "tenant RDI alarm overview matches its tenant-scoped API responses without requesting all tenants",
      ),
    ).to.include({
      evidenceKind: "boundary",
      businessClosureEvidence: false,
      provesBusinessFlow: false,
    });
    expect(e2eBusinessTitles("11_visualization.spec.js")).to.deep.equal([
      "native board CRUD is persisted by the local provider across list viewer and editor routes",
      "native board flows through the local provider on all ThingsVis compatibility routes",
      "seeded ThingsVis project and dashboard render across project list editor preview and menu routes",
      "dashboard menu persists for a real ThingsVis dashboard when the mirror is available",
    ]);
    expect(e2eBusinessTitles("04_alarm.spec.js")).to.deep.equal([
      "user search refresh keeps the seeded alarm aligned with the history API response",
      "alarm level filter sends H to the history API and keeps the seeded high alarm visible",
      "alert center downloads current page closure evidence JSON bundle",
      "seeded alarm scene is visible in downloaded closure evidence bundle",
      "browser acknowledge/reset actions persist and appear in downloaded closure evidence",
      "seeded notification group is visible from the notification group page",
    ]);
    const readyCheckCase = e2eBusinessCases("03_data.spec.js").find(
      (item) =>
        item.title ===
        "first-device Ready Check downloads diagnostics bundle with connection-guide evidence",
    );
    expect(readyCheckCase).to.include({
      businessClosureEvidence: true,
      evidenceLayer: "browser-e2e-with-api-setup",
      firstDeviceOnboarding: true,
      hasBrowserUserFlow: true,
      readyCheckDiagnosticsBundle: true,
      requiresSeededDevice: true,
      runtimeEvidenceRequired: true,
    });
    expect(readyCheckCase.capabilityIds).to.include("device-telemetry");
    const alarmClosureCase = e2eBusinessCases("04_alarm.spec.js").find(
      (item) =>
        item.title ===
        "browser acknowledge/reset actions persist and appear in downloaded closure evidence",
    );
    expect(alarmClosureCase).to.include({
      businessClosureEvidence: true,
      evidenceLayer: "browser-e2e-with-api-setup",
      hasBrowserUserFlow: true,
    });
    expect(alarmClosureCase.capabilityIds).to.include("alarm-notification");
    expect(e2eBusinessTitles("06_system.spec.js")).to.deep.equal([
      "data cleanup policy edited in the browser persists through the API and refreshed table",
    ]);
    const systemSettingCase = e2eBusinessCases("06_system.spec.js").find(
      (item) =>
        item.title ===
        "data cleanup policy edited in the browser persists through the API and refreshed table",
    );
    expect(systemSettingCase).to.include({
      businessClosureEvidence: true,
      evidenceLayer: "browser-e2e-with-api-setup",
      hasBrowserUserFlow: true,
      runtimeEvidenceRequired: true,
    });
    expect(systemSettingCase.capabilityIds).to.include("system-deployment");
    expect(e2eBusinessTitles("13_write_flows.spec.js")).to.deep.equal([
      "seeded device template is searchable in the UI and matches the list/detail APIs",
    ]);
    expect(e2eBusinessTitles("19_device_details_app.spec.js")).to.deep.equal([]);
    expect(e2eBusinessTitles("20_command_jobs.spec.js")).to.deep.equal([
      "browser draft previews, submits, and finds the persisted job in history",
      "scheduled command job is canceled from the browser and persists the canceled state",
      "failed device acknowledgement is retried from the browser through the real broker path",
      "selected-device command job stays visible with result and support evidence",
      "selected-device command job downloads support bundle JSON for operator handoff",
      "support preview failed devices link directly to Ready Check and Job detail",
    ]);
    expect(
      e2eBusinessTitles("21_ready_check_command_draft.spec.js"),
    ).to.deep.equal([
      "Ready Check route draft previews, submits, and persists the same command job",
    ]);
    expect(e2eBusinessTitles("22_ota_support_archive.spec.js")).to.deep.equal([
      "OTA task detail downloads support archive with task-level counts and conditional Ready Check handoff fields",
    ]);
    const otaSupportArchiveCase = e2eBusinessCases(
      "22_ota_support_archive.spec.js",
    ).find(
      (item) =>
        item.title ===
        "OTA task detail downloads support archive with task-level counts and conditional Ready Check handoff fields",
    );
    expect(otaSupportArchiveCase).to.include({
      businessClosureEvidence: true,
      evidenceLayer: "browser-e2e-with-api-setup",
      hasBrowserUserFlow: true,
      otaSupportArchive: true,
      requiresSeededOtaTask: true,
      runtimeEvidenceRequired: true,
    });
    expect(otaSupportArchiveCase.capabilityIds).to.include(
      "ota-script-openapi-service",
    );

    expect(
      coverageContract.getAutomationEvidenceMetadata("e2e/06_system.spec.js"),
    ).to.deep.equal({
      evidenceKind: "business",
      evidenceSource: "test-metadata",
    });
    expect(
      coverageContract.getAutomationEvidenceMetadata(
        "e2e/19_device_details_app.spec.js",
      ),
    ).to.deep.equal({
      evidenceKind: "boundary",
      evidenceSource: "test-metadata",
    });
    expect(
      coverageContract.getCoverageTagMetadata(
        fs.readFileSync(
          path.join(__dirname, "..", "e2e", "19_device_details_app.spec.js"),
          "utf8",
        ),
        null,
        null,
        "e2e/19_device_details_app.spec.js",
      ),
    ).to.deep.equal({
      pageCoverageOnly: false,
      marker: null,
      source: "none",
    });
    expect(
      coverageContract
        .getE2EBusinessCases(
          fs.readFileSync(
            path.join(__dirname, "..", "e2e", "19_device_details_app.spec.js"),
            "utf8",
          ),
          "e2e/19_device_details_app.spec.js",
        )
        .map((item) => item.title),
    ).to.deep.equal([]);
  });

  it("keeps runner report names explicit for boundary and contract modules", function () {
    const boundaryModules = runner.API_MODULES.filter((mod) =>
      ["api-boundary-smoke", "api-coverage-closure"].includes(mod.key),
    );
    const contractModule = runner.API_MODULES.find(
      (mod) => mod.key === "coverage-contract",
    );
    const businessModule = runner.API_MODULES.find(
      (mod) => mod.key === "seeded-automation-scene",
    );
    const ordinaryModule = runner.API_MODULES.find(
      (mod) => mod.key === "device",
    );

    expect(boundaryModules.map((mod) => mod.evidenceLabel)).to.deep.equal([
      "boundary",
      "boundary",
    ]);
    expect(boundaryModules.map(runner.getReportDisplayName)).to.deep.equal([
      "17_api_boundary_smoke.test.js [evidence: boundary; not business closure]",
      "17_api_coverage_closure.test.js [evidence: boundary; not business closure]",
    ]);
    expect(contractModule.evidenceLabel).to.equal("contract");
    expect(runner.getReportDisplayName(contractModule)).to.equal(
      "00_coverage_contract.test.js [evidence: contract; not business closure]",
    );
    expect(businessModule.evidenceLabel).to.equal("business");
    expect(runner.getReportDisplayName(businessModule)).to.equal(
      "23_seeded_automation_scene.test.js [evidence: business]",
    );
    const routeClosureModule = runner.E2E_MODULES.find(
      (mod) => mod.key === "route-coverage-closure",
    );
    const applyCatalogModule = runner.E2E_MODULES.find(
      (mod) => mod.key === "apply-marketplace",
    );
    expect(routeClosureModule.evidenceLabel).to.equal("business");
    expect(runner.getReportDisplayName(routeClosureModule)).to.equal(
      "e2e/14_route_coverage_closure.spec.js [evidence: business]",
    );
    expect(applyCatalogModule.evidenceLabel).to.equal("boundary");
    expect(runner.getReportDisplayName(applyCatalogModule)).to.equal(
      "e2e/15_apply_marketplace.spec.js [evidence: boundary; not business closure]",
    );
    expect(ordinaryModule.evidenceLabel).to.equal("business");
    expect(runner.getReportDisplayName(ordinaryModule)).to.equal(
      "02_device.test.js [evidence: business]",
    );
  });

  it("returns machine-readable partial-skip and blocked-reason details for runner summaries", function () {
    const summary = runner.summarizeMochaResult(
      {
        code: 0,
        stdout: [
          "  integration-blocked: requires runtime fixture or external dependency: tenant_admin_b account is unavailable",
          "  1 passing",
          "  2 pending",
        ].join("\n"),
        stderr: "",
        reportJson: null,
      },
      {
        stats: {
          tests: 3,
          passes: 1,
          pending: 2,
          skipped: 0,
          failures: 0,
        },
      },
    );

    expect(summary).to.include({
      passed: true,
      outcome: "partial-skip",
      skipped: 2,
      reason: "skipped 2/3; check fixture readiness if this was unexpected",
    });
    expect(summary.blockedReasons).to.deep.equal([
      {
        reason:
          "requires runtime fixture or external dependency: tenant_admin_b account is unavailable",
        category: "runtime-external",
        seedable: false,
      },
    ]);
    expect(
      coverageContract.classifyBlockedReason(summary.blockedReasons[0].reason),
    ).to.deep.equal({
      reason:
        "requires runtime fixture or external dependency: tenant_admin_b account is unavailable",
      category: "runtime-external",
      seedable: false,
    });
  });

  it("preserves structured blocked metadata emitted by integration helpers", function () {
    const originalWarn = console.warn;
    const warnings = [];
    console.warn = message => warnings.push(String(message));
    try {
      integrationBlocked.skipWhenBlocked(
        { skip() {} },
        true,
        {
          reason: "MQTT broker is unavailable for command ACK",
          category: "runtime-external",
          seedable: false
        }
      );
    } finally {
      console.warn = originalWarn;
    }

    const metadataLine = warnings.find(line => line.includes("integration-blocked-meta:"));
    expect(metadataLine).to.be.a("string");
    const summary = runner.summarizeMochaResult(
      {
        code: 0,
        stdout: warnings.join("\n"),
        stderr: "",
        reportJson: null
      },
      {
        stats: { tests: 1, passes: 0, pending: 1, skipped: 0, failures: 0 }
      }
    );

    expect(summary.blockedReasons).to.deep.equal([
      {
        reason: "MQTT broker is unavailable for command ACK",
        category: "runtime-external",
        seedable: false
      }
    ]);
  });

  it("keeps Playwright runner summaries on the same machine-readable contract", function () {
    const summary = runner.summarizePlaywrightResult(
      {
        code: 0,
        stdout: [
          "  integration-blocked: requires runtime fixture or external dependency: preview proxy is not configured",
          "  1 skipped",
        ].join("\n"),
        stderr: "",
        reportJson: null,
      },
      {
        stats: {
          expected: 3,
          skipped: 1,
          unexpected: 0,
        },
        errors: [],
      },
    );

    expect(summary).to.include({
      passed: true,
      outcome: "partial-skip",
      skipped: 1,
      reason: "skipped 1; check fixture readiness if this was unexpected",
    });
    expect(summary.blockedReasons).to.deep.equal([
      {
        reason:
          "requires runtime fixture or external dependency: preview proxy is not configured",
        category: "runtime-external",
        seedable: false,
      },
    ]);
  });

  it("writes summary JSON with business and non-business evidence separated", function () {
    const outputDir = fs.mkdtempSync(
      path.join(os.tmpdir(), "aetherlink-summary-contract-"),
    );

    try {
      reporter.results = [];
      reporter.parallel = false;
      reporter.startTime = new Date("2026-07-02T00:00:00.000Z");
      reporter.endTime = new Date("2026-07-02T00:00:03.000Z");

      reporter.record(
        "api-coverage-closure",
        "17_api_coverage_closure.test.js",
        true,
        "",
        "api",
        "boundary",
        {
          outcome: "passed",
          skipped: 0,
          blockedReasons: [],
        },
      );
      reporter.record(
        "endpoint-coverage",
        "00_endpoint_coverage.test.js",
        true,
        "",
        "api",
        "catalog",
        {
          outcome: "passed",
          skipped: 0,
          blockedReasons: [],
        },
      );
      reporter.record(
        "preflight-api-e2e",
        "00_preflight_api_e2e.test.js",
        false,
        "missing proxy",
        "api",
        "preflight",
        {
          outcome: "all-skipped",
          skipped: 2,
          blockedReasons: [
            {
              reason:
                "requires runtime fixture or external dependency: preview proxy is not configured",
              category: "runtime-external",
              seedable: false,
            },
          ],
        },
      );
      reporter.record(
        "seeded-automation-scene",
        "23_seeded_automation_scene.test.js",
        true,
        "",
        "api",
        "business",
        {
          outcome: "partial-skip",
          skipped: 1,
          oracleCases: [
            {
              title:
                "creates a seeded scene and verifies detail, list, and activation surfaces",
              businessClosureEvidence: true,
            },
          ],
          blockedReasons: [
            {
              reason: "missing seeded scene fixture",
              category: "seed-data",
              seedable: true,
            },
          ],
        },
      );

      const report = JSON.parse(
        fs.readFileSync(reporter.generateJsonReport(outputDir), "utf8"),
      );

      expect(report.byEvidenceKind).to.deep.include({
        boundary: { total: 1, passed: 1, failed: 0, passRate: 100 },
        catalog: { total: 1, passed: 1, failed: 0, passRate: 100 },
        preflight: { total: 1, passed: 0, failed: 1, passRate: 0 },
        business: { total: 1, passed: 1, failed: 0, passRate: 100 },
      });
      expect(report.businessClosureEvidence).to.deep.equal({
        business: { total: 1, passed: 1, failed: 0, passRate: 100 },
        nonBusiness: { total: 3, passed: 2, failed: 1, passRate: 66.67 },
      });
      expect(report.evidenceContract).to.include({
        businessClosureRequiresEvidenceKind: "business",
      });
      expect(
        report.evidenceContract.nonBusinessEvidenceKinds,
      ).to.include.members(["boundary", "catalog", "preflight"]);
      expect(report.modules["api-coverage-closure"]).to.include({
        evidenceKind: "boundary",
        businessClosureEvidence: false,
        outcome: "passed",
        skipped: 0,
      });
      expect(report.modules["seeded-automation-scene"]).to.include({
        evidenceKind: "business",
        businessClosureEvidence: true,
        outcome: "partial-skip",
        skipped: 1,
      });
      expect(report.modules["preflight-api-e2e"].blockedReasons).to.deep.equal([
        {
          reason:
            "requires runtime fixture or external dependency: preview proxy is not configured",
          category: "runtime-external",
          seedable: false,
        },
      ]);
      expect(
        report.details.map((item) => ({
          module: item.module,
          outcome: item.outcome,
          skipped: item.skipped,
          blockedReasons: item.blockedReasons,
        })),
      ).to.deep.include.members([
        {
          module: "preflight-api-e2e",
          outcome: "all-skipped",
          skipped: 2,
          blockedReasons: [
            {
              reason:
                "requires runtime fixture or external dependency: preview proxy is not configured",
              category: "runtime-external",
              seedable: false,
            },
          ],
        },
        {
          module: "seeded-automation-scene",
          outcome: "partial-skip",
          skipped: 1,
          blockedReasons: [
            {
              reason: "missing seeded scene fixture",
              category: "seed-data",
              seedable: true,
            },
          ],
        },
      ]);
    } finally {
      reporter.results = [];
      fs.rmSync(outputDir, { recursive: true, force: true });
    }
  });

  it("requires business evidence kind before promoting closure evidence", function () {
    reporter.results = [];

    try {
      reporter.record(
        "boundary-with-business-flag",
        "boundary.test.js",
        true,
        "",
        "api",
        "boundary",
        { businessClosureEvidence: true },
      );

      expect(reporter.results[0].businessClosureEvidence).to.equal(false);

      const metadataKey = "tests/contract-mismatched-business-flag.test.js";
      testMetadata.TEST_METADATA[metadataKey] = {
        file: metadataKey,
        evidenceKind: "business",
        cases: [
          {
            title: "boundary case must not be promoted",
            evidenceKind: "boundary",
            businessClosureEvidence: true,
          },
        ],
      };

      reporter.results = [];
      runner.recordModuleSummary(
        {
          key: "contract-mismatched-business-flag",
          file: metadataKey,
          type: "api",
          evidenceLabel: "business",
        },
        "api",
        {
          passed: true,
          outcome: "passed",
          skipped: 0,
          blockedReasons: [],
        },
      );

      expect(reporter.results[0].businessClosureEvidence).to.equal(false);
    } finally {
      delete testMetadata.TEST_METADATA["tests/contract-mismatched-business-flag.test.js"];
      reporter.results = [];
    }
  });

  it("does not promote page-coverage-only E2E checks into business cases", function () {
    const pageOnlyFile = `
      // @file-page-coverage-only: route catalog smoke only.
      test('route catalog click with business-looking assertions', async ({ page }) => {
        await page.getByRole('button', { name: 'Save' }).click();
        await expect(page.getByTestId('saved-state')).toHaveText('saved');
      });
    `;
    const pageOnlyBlock = `
      // @page-coverage-only: local route boundary only.
      test('single route click with business-looking assertions', async ({ page }) => {
        await page.getByRole('button', { name: 'Search' }).click();
        await expect(page.getByTestId('table')).toHaveText('result');
      });
    `;
    const businessBlock = `
      test('seeded user flow proves a visible state change', async ({ page, api }) => {
        await api.post('/scene', { name: 'contract-scene' });
        await page.getByRole('button', { name: 'Save' }).click();
        await expect(page.getByTestId('toast')).toHaveText('saved');
      });
    `;
    const pageOnlyBodyMarker = `
      test('route fallback marker inside test body', async ({ page }) => {
        // @page-coverage-only: fallback route text is not business closure.
        await page.getByRole('button', { name: 'Save' }).click();
        await expect(page.getByTestId('saved-state')).toHaveText('saved');
      });
    `;

    expect(coverageContract.getE2EBusinessCases(pageOnlyFile)).to.deep.equal(
      [],
    );
    expect(coverageContract.getE2EBusinessCases(pageOnlyBlock)).to.deep.equal(
      [],
    );
    expect(
      coverageContract.getE2EBusinessCases(pageOnlyBodyMarker),
    ).to.deep.equal([]);
    expect(
      coverageContract
        .getE2EBusinessCases(businessBlock)
        .map((item) => item.title),
    ).to.deep.equal(["seeded user flow proves a visible state change"]);
    expect(coverageContract.getCoverageTagMetadata(pageOnlyFile)).to.deep.equal(
      {
        pageCoverageOnly: true,
        marker: "@file-page-coverage-only",
        source: "file-marker",
      },
    );
    expect(
      coverageContract.getCoverageTagMetadata(
        pageOnlyBlock,
        pageOnlyBlock.split(/\r?\n/),
        3,
      ),
    ).to.deep.equal({
      pageCoverageOnly: true,
      marker: "@page-coverage-only",
      source: "block-marker",
    });
  });

  it("prefers explicit evidence and blocked annotations before heuristic fallback", function () {
    expect(
      coverageContract.getAutomationEvidenceMetadata({
        file: "tests/custom-contract.test.js",
        evidenceKind: "boundary",
      }),
    ).to.deep.equal({
      evidenceKind: "boundary",
      evidenceSource: "explicit-entry",
    });
    expect(
      coverageContract.getAutomationEvidenceMetadata(
        "tests/17_api_boundary_smoke.test.js",
      ),
    ).to.deep.equal({
      evidenceKind: "boundary",
      evidenceSource: "test-metadata",
    });
    expect(
      coverageContract.getAutomationEvidenceMetadata(
        "tests/17_api_coverage_closure.test.js",
      ),
    ).to.deep.equal({
      evidenceKind: "boundary",
      evidenceSource: "test-metadata",
    });
    expect(
      coverageContract.getAutomationEvidenceMetadata(
        "tests/23_seeded_automation_scene.test.js",
      ),
    ).to.deep.equal({
      evidenceKind: "business",
      evidenceSource: "test-metadata",
    });
    expect(
      coverageContract.getAutomationEvidenceMetadata(
        "e2e/14_route_coverage_closure.spec.js",
      ),
    ).to.deep.equal({
      evidenceKind: "business",
      evidenceSource: "test-metadata",
    });
    expect(
      coverageContract.getCoverageTagMetadata(
        fs.readFileSync(
          path.join(
            __dirname,
            "..",
            "e2e",
            "14_route_coverage_closure.spec.js",
          ),
          "utf8",
        ),
        null,
        null,
        "e2e/14_route_coverage_closure.spec.js",
      ),
    ).to.deep.equal({
      pageCoverageOnly: false,
      marker: null,
      source: "none",
    });
    expect(
      coverageContract
        .getE2EBusinessCases(
        fs.readFileSync(
          path.join(__dirname, "..", "e2e", "10_automation.spec.js"),
          "utf8",
        ),
        "e2e/10_automation.spec.js",
      )
      .map((item) => item.title),
    ).to.deep.equal([
      "scene manage search submits a seeded scene name",
      "scene edit echoes a seeded scene and matches the detail API",
      "scene linkage search finds a seeded automation and matches its detail API",
      "automation editor pre-validates, saves, and keeps the updated rule after refresh",
    ]);
    expect(
      testMetadata.getCaseMetadata(
        "e2e/10_automation.spec.js",
        "automation editor pre-validates, saves, and keeps the updated rule after refresh",
      ),
    ).to.include({
      evidenceKind: "business",
      businessClosureEvidence: true,
      provesBusinessFlow: true,
    });
    expect(
      coverageContract
        .getAutomationOracleCases(
          fs.readFileSync(
            path.join(__dirname, "23_seeded_automation_scene.test.js"),
            "utf8",
          ),
          "tests/23_seeded_automation_scene.test.js",
        )
        .map((item) => ({
          title: item.title,
          evidenceKind: item.evidenceKind,
          businessClosureEvidence: item.businessClosureEvidence,
        })),
    ).to.deep.equal([
      {
        title:
          "creates a seeded scene and verifies detail, list, and activation surfaces",
        evidenceKind: "business",
        businessClosureEvidence: true,
      },
      {
        title:
          "asserts scene and automation negative branches with explicit product errors",
        evidenceKind: "boundary",
        businessClosureEvidence: false,
      },
    ]);
    expect(
      coverageContract
        .getAutomationOracleCases(
          fs.readFileSync(
            path.join(__dirname, "24_seeded_scene_automations.test.js"),
            "utf8",
          ),
          "tests/24_seeded_scene_automations.test.js",
        )
        .map((item) => ({
          title: item.title,
          evidenceKind: item.evidenceKind,
          businessClosureEvidence: item.businessClosureEvidence,
        })),
    ).to.deep.equal([
      {
        title:
          "creates, updates, switches, filters, and deletes a seeded scene automation",
        evidenceKind: "business",
        businessClosureEvidence: true,
      },
      {
        title:
          "rejects non-existent scene automation ids with explicit product errors",
        evidenceKind: "boundary",
        businessClosureEvidence: false,
      },
      {
        title:
          "keeps create response id shape explicit for seeded automation helpers",
        evidenceKind: "contract",
        businessClosureEvidence: false,
      },
      {
        title:
          "dry-runs a seeded scene automation payload without saving or executing it",
        evidenceKind: "business",
        businessClosureEvidence: true,
      },
      {
        title:
          "dry-run explains invalid action references without accepting the payload",
        evidenceKind: "boundary",
        businessClosureEvidence: false,
      },
    ]);
    expect(
      coverageContract.getE2EBusinessCases(
        fs.readFileSync(
          path.join(
            __dirname,
            "..",
            "e2e",
            "14_route_coverage_closure.spec.js",
          ),
          "utf8",
        ),
        "e2e/14_route_coverage_closure.spec.js",
      ).map((item) => ({
        title: item.title,
        businessClosureEvidence: item.businessClosureEvidence,
      }))
    ).to.deep.equal([
      {
        title: "seeded child-device detail route renders the selected device API state",
        businessClosureEvidence: true,
      },
      {
        title: "personal-center renders the authenticated profile returned by the API",
        businessClosureEvidence: true,
      },
      {
        title: "seeded OpenAPI key appears in management/api and remains tenant-scoped",
        businessClosureEvidence: true,
      },
      {
        title: "system log path filter sends an exact API query and renders the empty result",
        businessClosureEvidence: true,
      },
      {
        title: "management/auth renders a menu element that is present in the API payload",
        businessClosureEvidence: true,
      },
    ]);
    expect(
      coverageContract.getAutomationEvidenceKindDetails("page-coverage-only"),
    ).to.include({
      businessClosureEvidence: false,
    });
    expect(
      coverageContract.getBlockedReasonMetadata(
        "tenant fixture needs local seed",
        { category: "seedable-local", seedable: true },
      ),
    ).to.deep.equal({
      reason: "tenant fixture needs local seed",
      category: "seedable-local",
      seedable: true,
      classificationSource: "annotation",
    });
    expect(
      coverageContract.getBlockedReasonMetadata(
        "requires runtime fixture or external dependency: tenant_admin_b account is unavailable",
      ),
    ).to.deep.equal({
      reason:
        "requires runtime fixture or external dependency: tenant_admin_b account is unavailable",
      category: "runtime-external",
      seedable: false,
      classificationSource: "heuristic",
    });
    expect(
      coverageContract.normalizeRunnerOutcome("partial-skip"),
    ).to.deep.equal({
      outcome: "partial-skip",
      recognized: true,
      partialSkip: true,
      allSkipped: false,
      failed: false,
      passed: false,
    });
  });

  it("flags conditional-empty and nullable helper API assertions as weak business evidence", function () {
    const audit = coverageContract.getWeakAutomationAssertionFindings(`
      it('allows empty rows to pass', async () => {
        const rows = await api.get('/example');
        if (rows.length > 0) {
          expect(rows[0]).to.have.property('id');
        }
        expectNullablePagedList(rows);
      });
    `);

    expect(
      audit.weakConditionalEmptyAssertions.map((item) => item.line),
    ).to.deep.equal([4]);
    expect(
      audit.weakNullableHelperAssertions.map((item) => item.line),
    ).to.deep.equal([7]);
  });

  it("keeps source inventory review evidence below business-closure evidence", function () {
    const audit = coverageContract.getSourceReviewBoundaryAudit();

    expect(audit.docs).to.deep.equal([
      "references/source-quality-review.md",
    ]);
    expect(audit.priorityScopeDifferenceExplained).to.equal(true);
    expect(audit.sourceInventoryDeclaresStaticBoundary).to.equal(true);
    expect(audit.qualityReviewRejectsRequestWrapperClosure).to.equal(true);
    expect(audit.qualityReviewRejectsSourceInventoryClosure).to.equal(true);
    expect(audit.qualityReviewRejectsSmokeClosure).to.equal(true);
    expect(audit.qualityReviewKeepsReleaseGateOpen).to.equal(true);
    expect(audit.unsafeClosureClaims).to.deep.equal([]);

    const check = coverageContract.selfCheck();
    expect(check.sourceReviewBoundaryAudit).to.deep.equal(audit);
  });

  it("flags unsafe source inventory table rows as non-business closure leaks", function () {
    const unsafe = coverageContract.getUnsafeSourceReviewClosureClaimsFromDocs([
      {
        file: "references/source-file-inventory.md",
        text: [
          "# synthetic source inventory",
          "| File | Lines | Purpose | Key symbols | Recommendation |",
          "|---|---:|---|---|---|",
          "| `frontend/src/service/request/request.ts` | 123 | Frontend HTTP request wrapper/interceptor code. | request | business closure evidence |",
          "| `frontend/src/service/request/instance.ts` | 80 | Frontend HTTP request wrapper/interceptor code. | instance | not business closure by itself |",
        ].join("\n"),
      },
    ]);

    expect(unsafe).to.deep.equal([
      {
        file: "references/source-file-inventory.md",
        line: 4,
        text: "| `frontend/src/service/request/request.ts` | 123 | Frontend HTTP request wrapper/interceptor code. | request | business closure evidence |",
      },
    ]);
  });
});
