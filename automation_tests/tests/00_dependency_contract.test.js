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
    const authSource = fs.readFileSync(path.join(root, 'tests', '01_auth.test.js'), 'utf8');

    expect(authSource).to.include('const config = apiClient.getConfig();');
    expect(authSource).not.to.include("require('../lib/runtime_config')");
  });
});
