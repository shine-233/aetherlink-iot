const { expect } = require('chai');

const runner = require('../run_tests');
const catalog = require('../lib/runner/module-catalog');

describe('Runner module catalog contract', function() {
  it('keeps the facade evidence references compatible', function() {
    expect(runner.getModuleEvidenceLabel).to.equal(catalog.getModuleEvidenceLabel);
    expect(runner.NON_BUSINESS_EVIDENCE_LABELS).to.equal(catalog.NON_BUSINESS_EVIDENCE_LABELS);
    expect(runner.getReportDisplayName).to.equal(catalog.getReportDisplayName);
  });

  it('discovers unique API and E2E keys with matching file suffixes', function() {
    const suites = catalog.discoverSuites();
    const allModules = [...suites.apiModules, ...suites.e2eModules];
    const keys = allModules.map(mod => `${mod.type}:${mod.key}`);

    expect(suites.apiModules).to.not.be.empty;
    expect(suites.e2eModules).to.not.be.empty;
    expect(new Set(keys).size).to.equal(keys.length);
    expect(suites.apiModules.every(mod => mod.type === 'api' && /\.test\.js$/.test(mod.file))).to.equal(true);
    expect(suites.e2eModules.every(mod => mod.type === 'e2e' && /\.spec\.js$/.test(mod.file))).to.equal(true);
  });

  it('supports auth and e2e-auth aliases for login modules', function() {
    const suites = {
      apiModules: [{ key: 'login', aliases: ['login', 'api-login', '01_login', 'auth', 'e2e-auth'] }],
      e2eModules: [{ key: 'login', aliases: ['login', 'e2e-login', '01_login', 'auth', 'e2e-auth'] }]
    };
    const args = { modules: ['auth'], e2e: false, includeE2e: false, parallel: false };
    const e2eArgs = { ...args, e2e: true, modules: ['e2e-auth'] };

    expect(catalog.createSelectedModules(args, suites).apiModulesToRun).to.have.length(1);
    expect(catalog.createSelectedModules(e2eArgs, suites).e2eModulesToRun).to.have.length(1);
  });

  it('builds API, E2E, and include-e2e plans without mutating input', function() {
    const apiModule = { key: 'api-one', aliases: ['api-one'], type: 'api' };
    const e2eModule = { key: 'e2e-one', aliases: ['e2e-one'], type: 'e2e' };
    const suites = { apiModules: [apiModule], e2eModules: [e2eModule] };
    const apiArgs = { modules: [], e2e: false, includeE2e: false, parallel: false };
    const e2eArgs = { modules: [], e2e: true, includeE2e: false, parallel: false };
    const bothArgs = { modules: [], e2e: false, includeE2e: true, parallel: false };

    expect(catalog.buildExecutionPlan(apiArgs, suites).types).to.deep.equal(['API']);
    expect(catalog.buildExecutionPlan(e2eArgs, suites).types).to.deep.equal(['E2E']);
    expect(catalog.buildExecutionPlan(bothArgs, suites).types).to.deep.equal(['API', 'E2E']);
    expect(apiArgs).to.deep.equal({ modules: [], e2e: false, includeE2e: false, parallel: false });
    expect(suites).to.deep.equal({ apiModules: [apiModule], e2eModules: [e2eModule] });
  });

  it('does not promote api-via-e2e-fixture metadata to business evidence', function() {
    const label = catalog.getModuleEvidenceLabelFromMetadata('fixture', 'e2e', {
      evidenceKind: 'business',
      cases: [{
        evidenceKind: 'business',
        businessClosureEvidence: true,
        hasBrowserUserFlow: false,
        evidenceLayer: 'api-via-e2e-fixture'
      }]
    });

    expect(label).to.equal('boundary');
  });
});
