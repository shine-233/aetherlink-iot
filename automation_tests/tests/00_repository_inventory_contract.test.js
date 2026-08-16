/**
 * 仓库全量文件台账契约。
 *
 * 该测试验证台账生成器覆盖 tracked、工作树缺失和未忽略的 untracked 文件，
 * 同时保证生成文件、第三方契约、敏感文件和运行产物边界不会在整理时被误判。
 */
const { expect } = require('chai');
const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');
const {
  categoryFor,
  excludedOutputPaths,
  ignoredBoundaryCategory,
  isSensitivePath,
  listIgnoredBoundaries,
  listRepositoryPaths,
  moduleFor,
  projectRoot,
  reviewedDeletionReasons
} = require('../scripts/generate_repository_inventory');

function gitPaths(args) {
  return execFileSync('git', args, {
    cwd: projectRoot,
    encoding: 'buffer',
    maxBuffer: 64 * 1024 * 1024
  })
    .toString('utf8')
    .split('\0')
    .filter(Boolean)
    .map(value => value.replace(/\\/g, '/'));
}

describe('repository inventory contract [00_repository_inventory_contract]', function () {
  // The full boundary scan invokes Git repeatedly and must not fail under Mocha's 2s default.
  this.timeout(30000);

  it('covers the tracked and non-ignored untracked repository boundary exactly once', function () {
    const expected = new Set([
      ...gitPaths(['ls-files', '-z']),
      ...gitPaths(['ls-files', '--others', '--exclude-standard', '-z'])
    ]);
    for (const outputPath of excludedOutputPaths) expected.delete(outputPath);

    const entries = listRepositoryPaths();
    const actual = entries.map(entry => entry.path);

    expect(new Set(actual).size).to.equal(actual.length);
    expect(actual).to.deep.equal([...expected].sort((left, right) => left.localeCompare(right, 'en')));
  });

  it('records each ignored boundary exactly once without expanding it into source inventory', function () {
    const expected = gitPaths([
      'ls-files',
      '--others',
      '--ignored',
      '--exclude-standard',
      '--directory',
      '-z'
    ]).sort((left, right) => left.localeCompare(right, 'en'));
    const boundaries = listIgnoredBoundaries();
    const actual = boundaries.map(entry => entry.path);
    const repositoryPaths = new Set(listRepositoryPaths().map(entry => entry.path));

    expect(new Set(actual).size).to.equal(actual.length);
    expect(actual).to.deep.equal(expected);
    expect(actual.some(relativePath => repositoryPaths.has(relativePath))).to.equal(false);
    for (const boundary of boundaries) {
      expect(boundary).to.have.all.keys('path', 'module', 'category', 'sensitive', 'recommendation');
    }
  });

  it('classifies ignored boundaries without reading their contents', function () {
    expect(ignoredBoundaryCategory('frontend/node_modules/')).to.equal('dependency-tree');
    expect(ignoredBoundaryCategory('automation_tests/e2e/.auth/')).to.equal('local-sensitive-config');
    expect(ignoredBoundaryCategory('backend/configs/rsa_key/private_key.pem')).to.equal('local-sensitive-config');
    expect(ignoredBoundaryCategory('automation_tests/reports/')).to.equal('runtime-or-report');
    expect(ignoredBoundaryCategory('frontend/dist/')).to.equal('generated-output');
    expect(ignoredBoundaryCategory('PROJECT_FOLDER_AUDIT_ALL_20260801.csv')).to.equal('local-audit-evidence');
  });

  it('keeps missing tracked files visible and requires every deletion to be reviewed', function () {
    const missingTracked = listRepositoryPaths().filter(entry => {
      if (!entry.tracked) return false;
      const absolutePath = path.join(projectRoot, ...entry.path.split('/'));
      return !fs.existsSync(absolutePath);
    });

    for (const entry of missingTracked) {
      expect(entry.tracked).to.equal(true);
      expect(entry.untracked).to.equal(false);
      expect(reviewedDeletionReasons.get(entry.path), `${entry.path} must have a deletion review reason`)
        .to.be.a('string').and.not.be.empty;
    }

    expect([...reviewedDeletionReasons.keys()].sort()).to.deep.equal(
      missingTracked.map(entry => entry.path).sort()
    );
  });

  it('classifies contract-sensitive repository surfaces deterministically', function () {
    expect(categoryFor('backend/internal/model/devices.gen.go')).to.equal('generated-source');
    expect(categoryFor('backend/third_party/grpc/client.go')).to.equal('third-party');
    expect(categoryFor('frontend/src/views/device/index.test.ts')).to.equal('test');
    expect(categoryFor('frontend/pnpm-lock.yaml')).to.equal('lockfile');
    expect(categoryFor('deploy/docker-compose.optional-integrations.yml')).to.equal('configuration');
    expect(categoryFor('frontend/.env.example')).to.equal('configuration');
    expect(categoryFor('frontend/.gitignore')).to.equal('configuration');
    expect(categoryFor('.github/CODEOWNERS')).to.equal('configuration');
    expect(categoryFor('mqtt-broker/plugin/admin/protos/client.proto')).to.equal('source');
    expect(categoryFor('start-aetherlink.cmd')).to.equal('source');
    expect(categoryFor('backend/LICENSE')).to.equal('documentation');
    expect(categoryFor('frontend/src/assets/illustrations/.gitkeep')).to.equal('data-or-asset');
    expect(categoryFor('frontend/src/views/device/index.vue')).to.equal('source');
    expect(categoryFor('clean-room-20260814/frontend-coverage/lcov.info')).to.equal('generated-output');
    expect(moduleFor('mqtt-broker/server/server.go')).to.equal('mqtt-broker');
    expect(moduleFor('README.md')).to.equal('root');
  });

  it('leaves no repository file in the ambiguous other category', function () {
    const ambiguous = listRepositoryPaths().filter(entry => categoryFor(entry.path) === 'other');
    expect(ambiguous.map(entry => entry.path)).to.deep.equal([]);
  });

  it('does not read real secret candidates as ordinary text inputs', function () {
    expect(isSensitivePath('.env')).to.equal(true);
    expect(isSensitivePath('deploy/production.env')).to.equal(true);
    expect(isSensitivePath('backend/configs/rsa_key/private_key.pem')).to.equal(true);
    expect(isSensitivePath('automation_tests/.env.example')).to.equal(false);
    expect(isSensitivePath('deploy/service.env.sample')).to.equal(false);
  });

  it('keeps generated output paths outside the self-referential input set', function () {
    const paths = new Set(listRepositoryPaths().map(entry => entry.path));
    expect([...excludedOutputPaths]).to.deep.equal([
      'references/repository-file-inventory.csv',
      'references/repository-file-inventory-summary.md'
    ]);
    for (const outputPath of excludedOutputPaths) expect(paths.has(outputPath)).to.equal(false);
  });
});
