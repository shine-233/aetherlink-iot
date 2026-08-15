/**
 * Regression coverage for the Playwright worker-restart persistence seam.
 * This validates the measurement harness only; it does not prove browser
 * business behavior.
 */
const { expect } = require('chai');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const trackerPath = path.resolve(__dirname, '../lib/page_coverage.js');

function runCoverageWorker(coverageFile, route) {
  const script = [
    `const tracker = require(${JSON.stringify(trackerPath)});`,
    `tracker.hitPage(${JSON.stringify(route)});`
  ].join('\n');
  return spawnSync(process.execPath, ['-e', script], {
    encoding: 'utf8',
    env: { ...process.env, PAGE_COVERAGE_FILE: coverageFile }
  });
}

describe('Page coverage persistence contract [00_page_coverage_persistence]', function () {
  it('merges routes written by replacement Playwright workers', function () {
    const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-page-coverage-'));
    const coverageFile = path.join(outputDir, 'page.json');

    try {
      expect(runCoverageWorker(coverageFile, '/device/manage').status).to.equal(0);
      expect(runCoverageWorker(coverageFile, '/device/grouping').status).to.equal(0);

      const payload = JSON.parse(fs.readFileSync(coverageFile, 'utf8'));
      expect(payload.pages.map(item => item.key)).to.have.members([
        '/device/manage',
        '/device/grouping'
      ]);
      expect(payload.flows.map(item => item.key)).to.have.members([
        'route:/device/manage',
        'route:/device/grouping'
      ]);
    } finally {
      fs.rmSync(outputDir, { recursive: true, force: true });
    }
  });
});
