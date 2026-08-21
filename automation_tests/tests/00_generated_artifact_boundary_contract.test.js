/**
 * 本地生成物边界契约。
 *
 * 只读证明运行产物、构建输出、缓存、归档和二进制不会混入源码边界；
 * 保留期限与归档内容审查必须保持显式可选，不能由清理脚本自行决定。
 */
const fs = require('fs');
const { expect } = require('chai');
const {
  collectCandidates,
  inspectGeneratedArtifacts,
  isGeneratedCandidate,
  projectRoot
} = require('../scripts/check_generated_artifacts');

function isLiveGeneratedArtifact(relativePath) {
  return /(^|\/)_(?:localrun|localrun_instance_b)(?:\/|$)/.test(relativePath)
    || relativePath.startsWith('.playwright-cli/');
}

describe('generated artifact boundary contract [00_generated_artifact_boundary_contract]', function () {
  // The contract intentionally audits the whole repository. On Windows the
  // current workspace contains tens of thousands of ignored build/runtime
  // files, and the read-only repeatability case performs three inventories.
  // Keep the assertions strict while allowing the bounded audit to finish.
  this.timeout(60000);

  it('keeps every discovered generated artifact outside the tracked source boundary', function () {
    const result = inspectGeneratedArtifacts();
    expect(result.external.filter(item => item.mode === 'blocked-external')).to.deep.equal([]);
    expect(result.reviewRequired).to.deep.equal([]);
    // A clean public clone is allowed to contain no ignored runtime output.
    // The boundary is verified by the candidate classifier and the tracked /
    // ignored assertions below; it must not require a pre-existing artifact.
    expect(result.summary.candidates).to.be.a('number').and.at.least(0);
    expect(result.summary.localDefault).to.equal(result.summary.candidates);
    expect(result.ok).to.equal(true);
  });

  it('recognizes the explicit runtime, build, cache, archive and binary boundaries', function () {
    expect(isGeneratedCandidate('_localrun/example.log')).to.equal(true);
    expect(isGeneratedCandidate('backend/_localrun/example.log')).to.equal(true);
    expect(isGeneratedCandidate('frontend/dist-lite/assets/index.js')).to.equal(true);
    expect(isGeneratedCandidate('frontend/output/playwright/home.png')).to.equal(true);
    expect(isGeneratedCandidate('automation_tests/output/playwright/home.png')).to.equal(true);
    expect(isGeneratedCandidate('frontend/.tsbuildinfo')).to.equal(true);
    expect(isGeneratedCandidate('verification/evidence.zip')).to.equal(true);
    expect(isGeneratedCandidate('mqtt-broker/gmqttd.exe')).to.equal(true);
    expect(isGeneratedCandidate('backend/internal/service/example.go')).to.equal(false);
  });

  it('is read-only across repeated inventory runs', function () {
    const before = collectCandidates();
    inspectGeneratedArtifacts();
    const after = collectCandidates();

    // Runtime output can legitimately be created or removed while this
    // whole-repository inventory is running. The source-boundary contract is
    // about stable source candidates; live runtime paths are governed by the
    // size/change assertions below. Sort the stable set as well because Git's
    // ignored-boundary enumeration is not a stable ordering contract.
    const stablePaths = entries => entries
      .filter(item => !isLiveGeneratedArtifact(item.path))
      .map(item => item.path)
      .sort();
    expect(stablePaths(after)).to.deep.equal(stablePaths(before));

    const beforeSizes = new Map(before.map(item => [item.path, item.size]));
    const changedSizes = after.filter(item => beforeSizes.get(item.path) !== item.size);
    expect(changedSizes.every(item => isLiveGeneratedArtifact(item.path))).to.equal(true);

    const source = fs.readFileSync(`${projectRoot}/automation_tests/scripts/check_generated_artifacts.js`, 'utf8');
    expect(source).not.to.match(/\b(?:writeFile|appendFile|unlink|rmSync|renameSync)\b/);
  });

  it('keeps retention and archive-content review explicitly external', function () {
    const result = inspectGeneratedArtifacts();
    expect(result.external.map(item => [item.id, item.mode, item.status])).to.deep.equal([
      ['artifact-retention-and-archive-content', 'optional-external', 'not-run']
    ]);
  });
});
