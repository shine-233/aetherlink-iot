/**
 * 文件用途：用于验证业务 oracle 契约测试。
 * 核心逻辑：以快速 Node 测试保护覆盖率契约、运行配置、oracle 或预检逻辑的结构和边界行为。
 * 关键注意事项：这类测试证明自动化框架契约，不等同于真实后端或浏览器业务流程通过。
 * 重构建议：当契约 schema 或分类规则变化时，应同步更新 fixture 和负向用例，避免只改快照。
 */

const { expect } = require('chai');
const coverageContract = require('../lib/coverage_contract');
const oracleContract = require('../lib/oracle_contract');

describe('Business oracle contract [00_oracle_contract]', function () {
  it('does not let broad catalog classification hide missing explicit capability inventory', function () {
    const check = oracleContract.selfCheck();
    expect(check.catalogClassificationAudit.unclassifiedEndpoints).to.deep.equal([]);
    expect(check.catalogClassificationAudit.unclassifiedRoutes).to.deep.equal([]);
    expect(check.mappedTestFileAudit).to.deep.equal([]);
    expect(check.businessAssertionAudit.weakExistenceAssertions).to.deep.equal([]);
    expect(check.businessAssertionAudit.weakFlexibleShapeAssertions).to.deep.equal([]);
    expect(check.businessAssertionAudit.weakObjectOnlyAssertions).to.deep.equal([]);
    expect(check.businessAssertionAudit.weakBareObjectAssertions).to.deep.equal([]);
    expect(check.frontendWeakAssertionAudit).to.deep.equal([]);
    expect(check.frontendSourceContractAudit.map(({ file, category }) => ({ file, category }))).to.include.deep.members([
      {
        file: 'src/__tests__/nginx-lightweight-contract.test.ts',
        category: 'source-contract'
      },
      {
        file: 'src/views/device/manage/__tests__/device-search-keys.test.ts',
        category: 'source-contract'
      }
    ]);

    expect(check.explicitBusinessInventoryAudit.missingEndpointCount).to.equal(0);
    expect(check.explicitBusinessInventoryAudit.missingRouteCount).to.equal(0);
    expect(check.explicitBusinessInventoryGapReport.nextCapability).to.equal(null);
    // The catalog and assertion audits are now structurally trustworthy even
    // when runtime business closure is still blocked by external fixtures.
    expect(check.trustworthyCoverageContract).to.equal(true);
    expect(check.businessClosureReady).to.equal(false);
    expect(check.staticOracleReady).to.equal(false);
    expect(check.runtimeReleaseEvidenceReady).to.equal(false);
    expect(check.runtimeReleaseEvidenceStatus).to.equal('not evaluated');
    expect(check.ready).to.equal(false);
    expect(check.missing).to.not.deep.include({
      capability: 'command-jobs',
      missing: 'userVisibleOracle'
    });

    const endpointClassification = coverageContract.classifyEndpointCatalogItem(
      'GET /api/v1/device/not-explicit-sentinel'
    );
    const routeClassification = coverageContract.classifyPageCatalogRoute('/device/not-explicit-sentinel');
    expect(endpointClassification).to.include({
      scope: 'P0/P1',
      capability: 'device-telemetry'
    });
    expect(routeClassification).to.include({
      scope: 'P0/P1',
      capability: 'device-telemetry'
    });

    const negativeAudit = coverageContract.getExplicitBusinessInventoryAudit({
      endpointClassifications: [endpointClassification],
      routeClassifications: [routeClassification]
    });
    expect(negativeAudit.missingEndpoints).to.deep.equal([endpointClassification]);
    expect(negativeAudit.missingRoutes).to.deep.equal([routeClassification]);
  });

  it('keeps 9725 preview evidence tied to the API proxy instead of plain frontend HTML', function () {
    const check = oracleContract.selfCheck();
    expect(check.previewProxyOracle).to.include({
      hasPreviewProxyScript: true,
      hasUsePreviewProxyEnv: true,
      disablesPreviewReuse: true,
      passed: true
    });
  });

  it('detects mapped test files that are missing or do not contain real tests', function () {
    const missingBackendTest = coverageContract.getMappedTestFileStatus(
      'negative-control',
      'backend',
      'internal/api/does_not_exist_test.go'
    );

    expect(missingBackendTest).to.include({
      capability: 'negative-control',
      layer: 'backend',
      file: 'internal/api/does_not_exist_test.go',
      exists: false,
      hasTestFunction: false
    });
  });

  it('does not accept source or MQTT oracles that only exist as catalog strings', function () {
    const sourceStatus = oracleContract.getCapabilityOracleStatus({
      id: 'negative-source',
      priority: 'P0',
      automationEvidence: [{
        hasExactStatusAssertion: true,
        hasBodyAssertion: true,
        hasNegativeAssertion: true,
        hasMutationOrSeedAction: true,
        hasStatusBodyCase: true,
        hasStatefulStatusBodyCase: true,
        hasNegativeStatusCase: true
      }],
      e2eTests: [],
      e2eEvidence: [],
      backendTests: ['internal/api/renamed_test.go'],
      backendEvidence: [{
        capability: 'negative-source',
        layer: 'backend',
        file: 'internal/api/renamed_test.go',
        exists: false,
        hasTestFunction: false
      }],
      gmqttTests: []
    });
    const mqttStatus = oracleContract.getCapabilityOracleStatus({
      id: 'device-telemetry',
      priority: 'P0',
      automationEvidence: [{
        hasExactStatusAssertion: true,
        hasBodyAssertion: true,
        hasNegativeAssertion: true,
        hasMutationOrSeedAction: true,
        hasStatusBodyCase: true,
        hasStatefulStatusBodyCase: true,
        hasNegativeStatusCase: true
      }],
      e2eTests: [],
      e2eEvidence: [],
      backendTests: ['internal/api/device_api_test.go'],
      backendEvidence: [{
        capability: 'device-telemetry',
        layer: 'backend',
        file: 'internal/api/device_api_test.go',
        exists: true,
        hasTestFunction: true
      }],
      gmqttTests: ['plugin/missing_case/does_not_exist_test.go'],
      gmqttEvidence: [{
        capability: 'device-telemetry',
        layer: 'gmqtt',
        file: 'plugin/missing_case/does_not_exist_test.go',
        exists: false,
        hasTestFunction: false
      }]
    });

    expect(sourceStatus.sourceOracle).to.equal(false);
    expect(sourceStatus.passed).to.equal(false);
    expect(mqttStatus.mqttOracle).to.equal(false);
    expect(mqttStatus.passed).to.equal(false);
  });

  it('lets explicit boundary failures satisfy only the negative oracle', function () {
    const status = oracleContract.getCapabilityOracleStatus({
      id: 'automation-scene',
      priority: 'P1',
      automationEvidence: [{
        evidenceKind: 'boundary',
        hasExactStatusAssertion: true,
        hasBodyAssertion: true,
        hasNegativeAssertion: true,
        hasMutationOrSeedAction: true,
        hasStatusBodyCase: true,
        hasStatefulStatusBodyCase: true,
        hasNegativeStatusCase: true
      }],
      e2eTests: [],
      e2eEvidence: [],
      backendTests: ['internal/api/api_router_contract_test.go'],
      backendEvidence: [{
        capability: 'automation-scene',
        layer: 'backend',
        file: 'internal/api/api_router_contract_test.go',
        exists: true,
        hasTestFunction: true
      }],
      gmqttTests: [],
      gmqttEvidence: []
    });

    expect(status.statusOracle).to.equal(false);
    expect(status.bodyOracle).to.equal(false);
    expect(status.negativeOracle).to.equal(true);
    expect(status.passed).to.equal(false);
  });

  it('does not treat account names as negative business assertions', function () {
    const cases = coverageContract.getAutomationOracleCases(`
      describe('seeded positive case', function () {
        it('uses tenant_admin for a successful stateful request', async function () {
          const resp = await apiClient.post('/scene', { name: 'ok' }, 'tenant_admin');
          expectSuccess(resp);
          expect(resp.data).to.have.property('scene_id');
        });
      });
    `);

    expect(cases).to.deep.include({
      title: 'uses tenant_admin for a successful stateful request',
      hasExactStatusAssertion: true,
      hasBodyAssertion: true,
      hasMutationOrSeedAction: true,
      hasNegativeAssertion: false,
      hasStatusBodyCase: true,
      hasStatefulStatusBodyCase: true,
      hasNegativeStatusCase: false
    });
  });

  it('does not treat request wrapper error normalization as a business oracle', function () {
    const cases = coverageContract.getAutomationOracleCases(`
      describe('request wrapper contract', function () {
        it('normalizes network failures into request error objects', async function () {
          const resp = await apiClient.get('/scene');
          expect(resp._requestError).to.equal(true);
          expect(resp.message).to.be.a('string');
          expect(resp.data).to.equal(null);
        });
      });
    `);

    expect(cases).to.have.length(1);
    expect(cases[0]).to.include({
      title: 'normalizes network failures into request error objects',
      hasExactStatusAssertion: false,
      hasBodyAssertion: true,
      hasMutationOrSeedAction: false,
      hasNegativeAssertion: false,
      hasStatusBodyCase: false,
      hasStatefulStatusBodyCase: false,
      hasNegativeStatusCase: false
    });
  });

  it('does not let new catalog endpoints or pages drift outside the business inventory', function () {
    expect(coverageContract.classifyEndpointCatalogItem('GET /api/v1/new-business-domain/resource')).to.include({
      scope: 'unknown',
      capability: null
    });
    expect(coverageContract.classifyPageCatalogRoute('/new-business-domain/page')).to.include({
      scope: 'unknown',
      capability: null
    });
  });

  it('recognizes dynamic route and endpoint references instead of reporting false unreferenced gaps', function () {
    const routeNeedles = coverageContract.getRouteReferenceNeedles('/login/:module(pwd-login|register|register-email|register-super-admin|reset-pwd|bind-wechat)?');
    const endpointNeedles = coverageContract.getEndpointReferenceNeedles('GET /api/v1/rdi/shared/:token');
    const files = [{
      file: 'inline-dynamic-reference.spec.js',
      text: [
        "await page.goto('/login/register-email');",
        "const resp = await apiClient.getNoAuth('/rdi/shared/' + shareToken);"
      ].join('\n')
    }];

    expect(routeNeedles).to.include('/login/');
    expect(endpointNeedles).to.include('/rdi/shared/');
    expect(coverageContract.findDirectReferences(routeNeedles, files)).to.deep.equal([
      'inline-dynamic-reference.spec.js'
    ]);
    expect(coverageContract.findDirectReferences(endpointNeedles, files)).to.deep.equal([
      'inline-dynamic-reference.spec.js'
    ]);
  });

  it('does not merge unrelated API assertions from different test cases into one oracle', function () {
    const splitCases = coverageContract.getAutomationOracleCases(`
      describe('split oracle false positive', function () {
        it('only checks status', async function () {
          expectSuccess(await apiClient.get('/device'));
        });

        it('only checks body', async function () {
          expect(resp.data).to.have.property('id');
        });

        it('only checks negative text', async function () {
          expect(message).to.include('permission');
        });
      });
    `);
    const completeCases = coverageContract.getAutomationOracleCases(`
      describe('complete oracle', function () {
        it('checks status and body in the same business action', async function () {
          const resp = await apiClient.post('/device', {});
          expectSuccess(resp);
          expect(resp.data).to.have.property('id');
        });
      });
    `);

    expect(splitCases.some(item => item.hasStatusBodyCase)).to.equal(false);
    expect(splitCases.some(item => item.hasNegativeStatusCase)).to.equal(false);
    expect(completeCases.some(item => item.hasStatusBodyCase)).to.equal(true);
  });

  it('detects existence-only assertions before they can count as business oracle evidence', function () {
    const source = [
      "it('accepts any non-empty shape', function () {",
      '  expect(row.id || row.name || row.topic).to.' + 'exist;',
      '  assert.' + 'ok(resp.data);',
      '});'
    ].join('\n');
    const findings = coverageContract.getWeakAutomationAssertionFindings(source, 'inline-negative-control.js');

    const expectedToExistText = 'expect(row.id || row.name || row.topic).to.' + 'exist;';
    const expectedAssertOkText = 'assert.' + 'ok(resp.data);';

    expect(findings.weakExistenceAssertions).to.deep.equal([
      {
        file: 'inline-negative-control.js',
        line: 2,
        text: expectedToExistText
      },
      {
        file: 'inline-negative-control.js',
        line: 3,
        text: expectedAssertOkText
      }
    ]);
  });

  it('detects overly flexible nullable shape assertions before they hide body oracle gaps', function () {
    const flexibleAssertion = "expect(resp.data === null || typeof resp.data === 'object' || " +
      "Array.isArray(resp.data)).to.equal(true);";
    const source = [
      "it('accepts almost any payload shape', function () {",
      '  ' + flexibleAssertion,
      '});'
    ].join('\n');
    const findings = coverageContract.getWeakAutomationAssertionFindings(source, 'inline-flexible-shape.js');

    expect(findings.weakFlexibleShapeAssertions).to.deep.equal([{
      file: 'inline-flexible-shape.js',
      line: 2,
      text: flexibleAssertion
    }]);
  });

  it('detects object-only helper calls before they hide missing response fields', function () {
    const objectOnlyAssertion = 'expectObject' + 'Payload(templateDetailResp);';
    const source = [
      "it('only proves the response body is some object', function () {",
      '  ' + objectOnlyAssertion,
      '});'
    ].join('\n');
    const findings = coverageContract.getWeakAutomationAssertionFindings(source, 'inline-object-only.js');

    expect(findings.weakObjectOnlyAssertions).to.deep.equal([{
      file: 'inline-object-only.js',
      line: 2,
      text: objectOnlyAssertion
    }]);
  });

  it('detects bare object response assertions when no business fields are checked nearby', function () {
    const bareAssertion = "expect(resp.data).to.be.an('object');";
    const source = [
      "it('only proves the endpoint returned an object', function () {",
      '  const resp = await apiClient.get("/service/plugin/select");',
      '  ' + bareAssertion,
      '});'
    ].join('\n');
    const strongSource = [
      "it('checks concrete fields after the object guard', function () {",
      '  const resp = await apiClient.get("/service/plugin/select");',
      '  expect(resp.data).to.be.an(\'object\');',
      "  expect(resp.data.protocol).to.be.an('array');",
      "  expect(resp.data.service).to.be.an('array');",
      '});'
    ].join('\n');

    const findings = coverageContract.getWeakAutomationAssertionFindings(source, 'inline-bare-object.js');
    const strongFindings = coverageContract.getWeakAutomationAssertionFindings(strongSource, 'inline-strong-object.js');

    expect(findings.weakBareObjectAssertions).to.deep.equal([{
      file: 'inline-bare-object.js',
      line: 3,
      text: bareAssertion
    }]);
    expect(strongFindings.weakBareObjectAssertions).to.deep.equal([]);
  });
});
