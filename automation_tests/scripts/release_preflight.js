#!/usr/bin/env node
/**
 * 文件用途：统一执行离线、只读的 release 静态与部署契约门禁。
 * 核心逻辑：顺序运行本地默认检查并输出单个 JSON；外部能力保持显式 not-run。
 * 关键注意事项：本脚本不联网、不启动服务，也不把 API/E2E 或外部扫描伪装为通过。
 */
const path = require('path');
const fs = require('fs');
const { spawnSync } = require('child_process');

const projectRoot = path.resolve(__dirname, '..', '..');
const diagnosticLimit = 2000;

function createLocalChecks(root = projectRoot) {
  const automationScripts = path.join(root, 'automation_tests', 'scripts');
  const deployTests = path.join(root, 'deploy', 'tests');
  return [
    {
      id: 'supply-chain',
      mode: 'local-default',
      command: process.execPath,
      args: [path.join(automationScripts, 'check_supply_chain.js')]
    },
    {
      id: 'generated-artifacts',
      mode: 'local-default',
      command: process.execPath,
      args: [path.join(automationScripts, 'check_generated_artifacts.js')]
    },
    ...[
      'optional-integrations-contract.test.sh',
      'docker-build-context-contract.test.sh',
      'package-source-boundary-contract.test.sh',
      'backend-readiness-contract.test.sh',
      'redis-memory-contract.test.sh',
      'container-runtime-security-contract.test.sh',
      'network-segmentation-contract.test.sh',
      'backup-restore-contract.test.sh'
    ].map(file => ({
      id: `deploy:${file.replace(/\.test\.sh$/, '')}`,
      mode: 'local-default',
      command: 'sh',
      args: [path.join(deployTests, file)]
    }))
  ];
}

function shortDiagnostic(value) {
  const text = String(value || '').trim();
  if (text.length <= diagnosticLimit) return text;
  return `${text.slice(0, diagnosticLimit)}\n...[truncated]`;
}

function findWindowsShell() {
  if (process.platform !== 'win32') return null;
  const roots = [
    process.env.GIT_INSTALL_ROOT,
    process.env.ProgramW6432 && path.join(process.env.ProgramW6432, 'Git'),
    process.env.ProgramFiles && path.join(process.env.ProgramFiles, 'Git'),
    process.env.LOCALAPPDATA && path.join(process.env.LOCALAPPDATA, 'Programs', 'Git')
  ].filter(Boolean);
  return roots
    .map(root => path.join(root, 'bin', 'bash.exe'))
    .find(candidate => fs.existsSync(candidate)) || null;
}

function runLocalCheck(check, runner, root) {
  let result;
  const runnerOptions = {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 16 * 1024 * 1024,
    windowsHide: true
  };
  try {
    result = runner(check.command, [...check.args], runnerOptions) || {};
    if (result.error && result.error.code === 'ENOENT' && check.command === 'sh') {
      const fallbackShell = findWindowsShell();
      if (fallbackShell) {
        result = runner(fallbackShell, [...check.args], runnerOptions) || {};
      }
    }
  } catch (error) {
    result = { status: null, error };
  }

  const exitCode = Number.isInteger(result.status) ? result.status : null;
  const passed = !result.error && exitCode === 0;
  let stderr = shortDiagnostic(result.stderr);
  if (result.error) {
    const isMissingShell = check.command === 'sh' && result.error.code === 'ENOENT';
    stderr = shortDiagnostic(isMissingShell
      ? 'Required shell executable "sh" was not found; deploy contract cannot run.'
      : result.error.message || result.error);
  }

  return {
    id: check.id,
    mode: check.mode,
    status: passed ? 'pass' : 'fail',
    exitCode,
    stdout: shortDiagnostic(result.stdout),
    stderr
  };
}

function externalChecks() {
  return [
    {
      id: 'runtime-api-e2e',
      mode: 'blocked-external',
      status: 'not-run',
      detail: 'requires running services, release credentials, preflight:api-e2e, and E2E execution'
    },
    {
      id: 'vulnerability-database',
      mode: 'optional-external',
      status: 'not-run',
      detail: 'requires installed scanners and current advisory data'
    },
    {
      id: 'sbom-generation',
      mode: 'optional-external',
      status: 'not-run',
      detail: 'requires an explicit release SBOM toolchain'
    },
    {
      id: 'hosted-dependency-review',
      mode: 'blocked-external',
      status: 'not-run',
      detail: 'requires repository hosting and service configuration'
    }
  ];
}

function runReleasePreflight(options = {}) {
  const root = options.projectRoot ? path.resolve(options.projectRoot) : projectRoot;
  const runner = options.runner || spawnSync;
  const definitions = options.checks
    ? options.checks.map(check => ({ ...check, args: [...check.args] }))
    : createLocalChecks(root);
  const checks = definitions.map(check => runLocalCheck(check, runner, root));

  return {
    kind: 'aetherlink-release-preflight-local',
    checks,
    external: externalChecks(),
    ok: checks.every(check => check.mode !== 'local-default' || check.status === 'pass')
  };
}

if (require.main === module) {
  const result = runReleasePreflight();
  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
  if (!result.ok) process.exitCode = 1;
}

module.exports = {
  createLocalChecks,
  externalChecks,
  projectRoot,
  runLocalCheck,
  runReleasePreflight,
  shortDiagnostic
};
