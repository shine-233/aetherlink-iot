/**
 * Protects direct runtime dependencies used by shared automation clients.
 * Imports required by project code must not rely on another package's
 * transitive dependency tree, which can change independently.
 */
const { expect } = require('chai');
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..');
const manifest = require('../package.json');
const lockfile = require('../package-lock.json');

describe('automation dependency contract', function () {
  it('declares API client external imports as direct dependencies', function () {
    const source = fs.readFileSync(path.join(root, 'lib', 'api_client.js'), 'utf8');

    for (const dependency of ['axios', 'form-data']) {
      expect(source).to.include(`require('${dependency}')`);
      expect(manifest.dependencies).to.have.property(dependency);
    }
  });

  it('keeps package and lockfile root dependencies aligned', function () {
    expect(lockfile.packages[''].dependencies).to.deep.equal(manifest.dependencies);
    expect(lockfile.packages[''].devDependencies).to.deep.equal(manifest.devDependencies);
  });

  it('resolves the Playwright CLI from the declared direct test dependency', function () {
    const runnerSource = fs.readFileSync(path.join(root, 'run_tests.js'), 'utf8');

    expect(manifest.devDependencies).to.have.property('@playwright/test');
    expect(runnerSource).to.include("require.resolve('@playwright/test/cli')");
    expect(runnerSource).not.to.include("path.join(__dirname, 'node_modules', 'playwright', 'cli.js')");
    expect(require.resolve('@playwright/test/cli')).to.be.a('string').and.not.to.equal('');
  });

  it('locks a compatible direct form-data package', function () {
    expect(lockfile.packages['node_modules/form-data']).to.be.an('object');
    expect(lockfile.packages['node_modules/form-data'].version).to.match(/^4\./);
  });

  it('keeps default and integration test entry points layered', function () {
    expect(manifest.scripts.test).to.match(/^mocha tests\/00_\*\.test\.js(?: |$)/);
    expect(manifest.scripts['test:integration']).to.match(/^mocha tests\/\*\.test\.js(?: |$)/);
  });

  it('keeps automation API and full entry points available', function () {
    expect(manifest.scripts['test:automation:api']).to.equal('node run_tests.js');
    expect(manifest.scripts['test:automation:full']).to.equal('node run_tests.js --include-e2e');
  });

  it('keeps live auth credentials on the environment-only API client path', function () {
    for (const file of ['01_auth.test.js', '07_board.test.js', '20_seeded_system_permission.test.js']) {
      const source = fs.readFileSync(path.join(root, 'tests', file), 'utf8');

      expect(source, file).to.include('apiClient.getConfig()');
      expect(source, file).not.to.include("require('../lib/runtime_config')");
    }
  });

  it('keeps shared network clients independent from file-backed config', function () {
    const apiClientSource = fs.readFileSync(path.join(root, 'lib', 'api_client.js'), 'utf8');
    const networkRuntimeSource = fs.readFileSync(path.join(root, 'lib', 'network_runtime.js'), 'utf8');
    const preflightSource = fs.readFileSync(path.join(root, 'scripts', 'preflight_api_e2e.js'), 'utf8');
    const executableNetworkRuntimeSource = networkRuntimeSource
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .replace(/\/\/.*$/gm, '');

    expect(apiClientSource).not.to.include("require('./runtime_config')");
    expect(executableNetworkRuntimeSource).not.to.match(/require\(['"](?:fs|path)['"]\)/);
    expect(executableNetworkRuntimeSource).not.to.match(/(?:readFile|existsSync|path\.)/);
    expect(preflightSource).to.include('env.API_BASE_URL');
    expect(preflightSource).to.include('env.HEALTH_URL');
    expect(preflightSource).not.to.include('new URL(config.baseURL)');
    expect(preflightSource).not.to.include('new URL(config.healthURL');
  });

  it('keeps browser fixtures on network credentials and static request fixtures', function () {
    const fixtureSource = fs.readFileSync(path.join(root, 'e2e', 'fixtures.js'), 'utf8');
    const loginSource = fs.readFileSync(path.join(root, 'e2e', '01_login.spec.js'), 'utf8');
    const deviceSource = fs.readFileSync(path.join(root, 'e2e', '02_device.spec.js'), 'utf8');
    const testDataSource = fs.readFileSync(path.join(root, 'lib', 'test_data.js'), 'utf8');

    for (const [file, source] of [
      ['e2e/fixtures.js', fixtureSource],
      ['e2e/01_login.spec.js', loginSource],
      ['e2e/02_device.spec.js', deviceSource]
    ]) {
      expect(source, file).to.include("require('../lib/network_runtime')");
      expect(source, file).not.to.include("require('../lib/runtime_config')");
    }
    expect(fixtureSource).to.include("require('../lib/test_data')");
    expect(testDataSource).not.to.include("require('./runtime_config')");
  });
});
