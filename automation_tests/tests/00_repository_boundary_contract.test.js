/**
 * Repository-boundary contract for reproducible build and test artifacts.
 * Runtime output stays outside the source boundary, while source-like build
 * configuration remains visible for review and packaging.
 */
const { expect } = require('chai');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

const projectRoot = path.resolve(__dirname, '..', '..');

function readProjectFile(relativePath) {
  return fs.readFileSync(path.join(projectRoot, relativePath), 'utf8').replace(/\r\n/g, '\n');
}

function isGitIgnored(relativePath) {
  const result = spawnSync('git', ['check-ignore', '--quiet', '--no-index', '--', relativePath], {
    cwd: projectRoot,
    encoding: 'utf8'
  });

  expect(result.error, `git check-ignore failed for ${relativePath}`).to.equal(undefined);
  expect([0, 1], result.stderr || `unexpected git check-ignore status for ${relativePath}`).to.include(result.status);
  return result.status === 0;
}

describe('repository generated-file boundary [00_repository_boundary_contract]', function () {
  const gitignore = readProjectFile('.gitignore');
  const generatedFilesPolicy = readProjectFile('GENERATED_FILES.md');

  it('ignores reproducible frontend and browser automation output', function () {
    for (const ignoredPath of [
      'frontend/dist/',
      'frontend/dist-lite/',
      'frontend/output/',
      '.playwright-cli/',
      'automation_tests/reports/',
      'automation_tests/output/',
      'playwright-report/',
      'test-results/',
      'backend/_localrun/',
      'frontend/_localrun/',
      'automation_tests/_localrun/',
      'mqtt-broker/_localrun/'
    ]) {
      expect(gitignore, `${ignoredPath} must stay outside the source boundary`)
        .to.include(ignoredPath);
    }
  });

  it('keeps heavyweight local runtime artifacts outside the source boundary', function () {
    for (const runtimePath of [
      '_localrun/backend.stdout.log',
      'verification/runtime-artifacts-archive/runtime-artifacts.zip',
      'automation_tests/reports/test-report.html',
      'frontend/.tsbuildinfo',
      'mqtt-broker/gmqttd.exe',
      'backend/cmd/device-emulator.exe',
      'backend/.gocache-noopt/cache-entry'
    ]) {
      expect(isGitIgnored(runtimePath), `${runtimePath} must remain ignored`).to.equal(true);
    }
    expect(gitignore).to.include('backend/.gocache-*/');
    expect(isGitIgnored('backend/internal/service/example.go')).to.equal(false);
  });

  it('documents generated output separately from retained generated source', function () {
    expect(generatedFilesPolicy).to.include('## 保留在源码中的生成文件');
    expect(generatedFilesPolicy).to.include('## 不保留在源码中的生成文件');
    expect(generatedFilesPolicy).to.include('`.playwright-cli/`');
    expect(generatedFilesPolicy).to.include('`automation_tests/reports/`');
    expect(generatedFilesPolicy).to.include('`frontend/output/`');
    expect(generatedFilesPolicy).to.include('`automation_tests/output/`');
    expect(generatedFilesPolicy).to.include('`verification/<timestamp>/`');
  });

  it('keeps the visual page sweep helper in the automation script seam', function () {
    const relativePath = 'automation_tests/scripts/visual-page-sweep.js';
    const source = readProjectFile(relativePath);

    expect(fs.existsSync(path.join(projectRoot, relativePath))).to.equal(true);
    expect(source).to.include('VISUAL_OUTPUT_DIR');
    expect(source).not.to.include('frontend/output/playwright');
  });

  it('keeps frontend build configuration visible while ignoring real build output', function () {
    const buildSourcePaths = [
      'frontend/build/config/index.ts',
      'frontend/build/config/proxy.ts',
      'frontend/build/plugins/icons.ts',
      'frontend/build/plugins/index.ts',
      'frontend/build/plugins/router.ts',
      'frontend/build/plugins/unocss.ts'
    ];

    for (const sourcePath of buildSourcePaths) {
      expect(fs.existsSync(path.join(projectRoot, sourcePath)), `${sourcePath} must exist`).to.equal(true);
      expect(isGitIgnored(sourcePath), `${sourcePath} must remain visible to Git`).to.equal(false);
    }

    for (const outputPath of [
      'build/root-output.bin',
      'frontend/dist/index.html',
      'frontend/dist-lite/index.html',
      'mqtt-broker/build/gmqttd'
    ]) {
      expect(isGitIgnored(outputPath), `${outputPath} must remain ignored`).to.equal(true);
    }
  });

  it('keeps immutable workspace ledgers local without moving their captured paths', function () {
    for (const auditPath of [
      'PROJECT_CLEANUP_PLAN.md',
      'PROJECT_FOLDER_AUDIT_ALL_20260801.csv',
      'PROJECT_FOLDER_CONTENTS_ALL_20260801.csv',
      'PROJECT_FOLDER_AUDIT_MANAGED_20260801.md',
      'PROJECT_FOLDER_AUDIT_GENERATOR.ps1'
    ]) {
      expect(isGitIgnored(auditPath), `${auditPath} must remain local audit evidence`).to.equal(true);
    }
  });

  it('keeps manual administrative SQL outside automatic test discovery', function () {
    const relativePath = 'automation_tests/scripts/fix-tenant-user-menus.sql';
    const normalizedPath = relativePath.replace(/\\/g, '/');

    expect(normalizedPath).not.to.match(/^automation_tests\/tests\/[^/]+\.test\.js$/);
    expect(normalizedPath).not.to.match(/^automation_tests\/e2e\/[^/]+\.spec\.js$/);
    expect(readProjectFile(relativePath)).to.include('MANUAL_ADMIN_ONLY');
  });
});
