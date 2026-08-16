/**
 * Keeps release gates aligned with the checks enforced on main.
 *
 * Dependency review runs on pull requests and on main pushes. The push
 * trigger is intentional: a tag points at a main commit, while a PR-only
 * check is attached to the PR head and cannot be discovered on that commit.
 */
const { expect } = require('chai');
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..', '..');
const releaseWorkflows = [
  '.github/workflows/release.yml',
  '.github/workflows/container-release.yml'
];

const protectedChecks = [
  'Offline release preflight',
  'Frontend build and unit tests',
  'Backend Go tests and build',
  'MQTT broker Go tests and build',
  'Device autotest Go tests and build',
  'Automation contract tests',
  'Dependency review',
  'CodeQL actions',
  'CodeQL go',
  'CodeQL javascript-typescript'
];

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), 'utf8');
}

function requiredChecks(source) {
  const match = source.match(/required_checks=\(\s*([\s\S]*?)\s*\n\s*\)/);
  expect(match, 'release workflow must declare required_checks').not.to.equal(null);
  return [...match[1].matchAll(/"([^"]+)"/g)].map(item => item[1]);
}

describe('release workflow contract [00_release_workflow_contract]', function () {
  it('requires every protected check in both tag release gates', function () {
    for (const workflow of releaseWorkflows) {
      expect(requiredChecks(read(workflow)), workflow).to.deep.equal(protectedChecks);
    }
  });

  it('publishes dependency review on main with an exact push range', function () {
    const source = read('.github/workflows/dependency-review.yml');

    expect(source).to.match(/push:\s+branches:\s+- main/);
    expect(source).to.include(
      "base-ref: ${{ github.event_name == 'push' && github.event.before || '' }}"
    );
    expect(source).to.include(
      "head-ref: ${{ github.event_name == 'push' && github.sha || '' }}"
    );
    expect(source).to.include(
      "comment-summary-in-pr: ${{ github.event_name == 'pull_request' && 'always' || 'never' }}"
    );
    expect(source).to.include('fail-on-severity: moderate');
    expect(source).to.include('fail-on-scopes: runtime, development, unknown');
  });

  it('binds every source release asset and release object to the validated SHA', function () {
    const source = read('.github/workflows/release.yml');

    expect(source).to.include('sha256sum -c SHA256SUMS.txt');
    expect(source).to.include('subject-path: aetherlink-iot-${{ github.ref_name }}.tar.gz');
    expect(source).to.include('subject-path: source-sbom.json');
    expect(source).to.include('subject-path: SHA256SUMS.txt');
    expect(source).to.include('--target "$RELEASE_SHA"');
  });

  it('fails the hosted integration job when runtime tests are skipped', function () {
    const source = read('.github/workflows/integration-nightly.yml');

    expect(source).to.include("CI_STRICT_INTEGRATION: '1'");
    expect(source).to.include('Run API automation');
    expect(source).to.include('Run Playwright E2E');
  });
});
