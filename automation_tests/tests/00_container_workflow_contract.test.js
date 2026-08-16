/**
 * The release workflow publishes images, but pull requests must prove that
 * all three production Dockerfiles still build before they can reach main.
 * This contract keeps the PR gate build-only and least-privileged.
 */
const { expect } = require('chai');
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..', '..');
const workflowPath = path.join(root, '.github', 'workflows', 'container-ci.yml');

function readWorkflow() {
  return fs.readFileSync(workflowPath, 'utf8').replace(/\r\n/g, '\n');
}

describe('container CI workflow contract [00_container_workflow_contract]', function () {
  it('runs on PRs, main pushes, release tags, and manual dispatch', function () {
    const source = readWorkflow();
    expect(source).to.match(/^on:\n  pull_request:\n  push:\n/m);
    expect(source).to.include('branches:\n      - main');
    expect(source).to.include("tags:\n      - 'v*.*.*'");
    expect(source).to.include('workflow_dispatch:');
  });

  it('builds each production image for the release architecture', function () {
    const source = readWorkflow();
    for (const image of [
      'aetherlink-iot-backend',
      'aetherlink-iot-frontend',
      'aetherlink-iot-mqtt-broker'
    ]) {
      expect(source, image).to.include(`image: ${image}`);
    }
    expect(source).to.include('platforms: linux/amd64');
    expect(source).to.include('context: ${{ matrix.context }}');
    expect(source).to.include('file: ${{ matrix.file }}');
  });

  it('does not publish or request write credentials', function () {
    const source = readWorkflow();
    expect(source).to.match(/uses: actions\/checkout@[0-9a-f]{40}(?:\s+#.*)?$/m);
    expect(source).to.match(/uses: docker\/setup-buildx-action@[0-9a-f]{40}(?:\s+#.*)?$/m);
    expect(source).to.match(/uses: docker\/build-push-action@[0-9a-f]{40}(?:\s+#.*)?$/m);
    expect(source).to.include('push: false');
    expect(source).to.include('provenance: false');
    expect(source).to.include('sbom: false');
    expect(source).not.to.match(/^\s+packages:\s+write$/m);
    expect(source).not.to.match(/secrets\./);
  });
});
