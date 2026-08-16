/**
 * 文件用途：用于验证API/E2E 预检契约测试。
 * 核心逻辑：以快速 Node 测试保护覆盖率契约、运行配置、oracle 或预检逻辑的结构和边界行为。
 * 关键注意事项：这类测试证明自动化框架契约，不等同于真实后端或浏览器业务流程通过。
 * 重构建议：当契约 schema 或分类规则变化时，应同步更新 fixture 和负向用例，避免只改快照。
 */

const fs = require('fs');
const http = require('http');
const path = require('path');
const { expect } = require('chai');

const fileConfig = require('../config.json');
const runtimeConfig = require('../lib/runtime_config');
const {
  resolvePreflightSettings,
  evaluatePreflight,
  evaluateConnectivity,
  runPreflightCli
} = require('../scripts/preflight_api_e2e');

const releaseRequiredAccounts = runtimeConfig.releaseRequiredAccounts;

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function makeConfig({ frontendURL = 'http://127.0.0.1:9725', realAccounts = true } = {}) {
  const config = {
    ...clone(fileConfig),
    frontendURL,
    releaseRequiredAccounts
  };

  if (realAccounts) {
    for (const accountName of releaseRequiredAccounts) {
      config.accounts[accountName] = {
        ...config.accounts[accountName],
        email: `${accountName.replace(/_/g, '.')}@local.test`,
        password: `${accountName}-local-password`
      };
    }
  }

  return config;
}

const releaseEnv = {
  PREVIEW_URL: 'http://127.0.0.1:9725',
  API_TARGET: 'http://127.0.0.1:9999',
  API_BASE_URL: 'http://127.0.0.1:9999/api/v1',
  HEALTH_URL: 'http://127.0.0.1:9999/health',
  PLAYWRIGHT_USE_PREVIEW_PROXY: '1',
  PLAYWRIGHT_REUSE_EXISTING_SERVER: '0'
};

function listen(server) {
  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      server.removeListener('error', reject);
      resolve(server.address());
    });
  });
}

function close(server) {
  return new Promise((resolve, reject) => {
    server.close(error => (error ? reject(error) : resolve()));
  });
}

function response(body, { status = 200, contentType = 'application/json' } = {}) {
  return new Response(body, {
    status,
    headers: { 'content-type': contentType }
  });
}

describe('API/E2E preflight gate', function() {
  it('rejects the public placeholder config before release automation starts', function() {
    const result = evaluatePreflight({
      config: {
        ...clone(fileConfig),
        releaseRequiredAccounts
      },
      env: {}
    });

    expect(result.ok).to.equal(false);
    expect(result.missingCredentials).to.include.members([
      'super_admin.email',
      'super_admin.password',
      'tenant_admin.email',
      'tenant_admin.password',
      'tenant_admin_b.email',
      'tenant_admin_b.password',
      'tenant_user.email',
      'tenant_user.password'
    ]);
    expect(result.problems).to.include('PLAYWRIGHT_USE_PREVIEW_PROXY must be 1 for release E2E evidence on port 9725');
    expect(result.problems).to.include('PLAYWRIGHT_REUSE_EXISTING_SERVER must be 0 so Playwright starts the preview API proxy for release evidence');
    expect(result.problems).to.include('FRONTEND_URL/frontendURL must use preview port 9725 for release E2E evidence: http://127.0.0.1:5002');
    expect(result.problems).to.include('invalid PREVIEW_URL: <empty>');
    expect(result.problems).to.include('API_TARGET must be an HTTP URL for preview proxying: <empty>');
  });

  it('rejects real-looking accounts when the run still targets the dev frontend port', function() {
    const result = evaluatePreflight({
      config: makeConfig({ frontendURL: 'http://127.0.0.1:5002' }),
      env: releaseEnv
    });

    expect(result.ok).to.equal(false);
    expect(result.missingCredentials).to.deep.equal([]);
    expect(result.problems).to.deep.equal([
      'FRONTEND_URL/frontendURL must use preview port 9725 for release E2E evidence: http://127.0.0.1:5002'
    ]);
  });

  it('passes only when release accounts, 9725 preview, API target, proxy, and server reuse gate are all set', function() {
    const result = evaluatePreflight({
      config: makeConfig(),
      env: releaseEnv
    });

    expect(result).to.include({
      ok: true
    });
    expect(result.problems).to.deep.equal([]);
    expect(result.summary).to.deep.include({
      frontendURL: 'http://127.0.0.1:9725',
      previewURL: 'http://127.0.0.1:9725',
      apiTarget: 'http://127.0.0.1:9999',
      previewProxy: '1',
      reuseExistingServer: '0'
    });
    expect(result.summary.accounts).to.deep.equal(releaseRequiredAccounts);
    expect(result.summary.settings).to.deep.equal({
      profile: 'full',
      previewPort: 9725
    });
    expect(result.summary.checks).to.deep.equal({
      releaseAccounts: true
    });
  });

  it('resolves profile and preview port without mutating its inputs', function() {
    const env = {
      PREFLIGHT_PROFILE: 'LOCAL-LITE',
      PREVIEW_PORT: '10825'
    };
    const settings = resolvePreflightSettings({ env });

    expect(settings).to.deep.equal({
      profile: 'local-lite',
      previewPort: 10825,
      checks: { releaseAccounts: false },
      problems: []
    });
    expect(env).to.deep.equal({
      PREFLIGHT_PROFILE: 'LOCAL-LITE',
      PREVIEW_PORT: '10825'
    });
  });

  it('supports a custom preview port and local-lite account scope', function() {
    const result = evaluatePreflight({
      config: makeConfig({
        frontendURL: 'http://127.0.0.1:10825',
        realAccounts: false
      }),
      env: {
        ...releaseEnv,
        PREFLIGHT_PROFILE: 'local-lite',
        PREVIEW_PORT: '10825',
        PREVIEW_URL: 'http://127.0.0.1:10825'
      }
    });

    expect(result.ok).to.equal(true);
    expect(result.missingCredentials).to.deep.equal([]);
    expect(result.summary.settings).to.deep.equal({
      profile: 'local-lite',
      previewPort: 10825
    });
    expect(result.summary.checks).to.deep.equal({
      releaseAccounts: false
    });
  });

  it('fails closed for an invalid profile instead of applying local-lite checks', function() {
    const result = evaluatePreflight({
      config: makeConfig({ realAccounts: false }),
      env: {
        ...releaseEnv,
        PREFLIGHT_PROFILE: 'quick'
      }
    });

    expect(result.ok).to.equal(false);
    expect(result.problems).to.include(
      'invalid PREFLIGHT_PROFILE/profile: quick; expected local-lite or full'
    );
    expect(result.missingCredentials).to.include.members([
      'super_admin.email',
      'super_admin.password'
    ]);
    expect(result.problems.some(problem => problem.startsWith('placeholder credentials:'))).to.equal(true);
    expect(result.summary.settings.profile).to.equal('full');
    expect(result.summary.checks.releaseAccounts).to.equal(true);
  });

  it('fails closed for an invalid preview port and retains the safe default', function() {
    const result = evaluatePreflight({
      config: makeConfig(),
      env: {
        ...releaseEnv,
        PREVIEW_PORT: '0'
      }
    });

    expect(result.ok).to.equal(false);
    expect(result.problems).to.deep.equal([
      'invalid PREVIEW_PORT/previewPort: 0; expected integer 1-65535'
    ]);
    expect(result.summary.settings.previewPort).to.equal(9725);
  });

  it('returns a machine-readable success report without exiting when used as a module', async function() {
    const stdout = [];
    const stderr = [];
    const requestedURLs = [];
    const fetchImpl = async url => {
      requestedURLs.push(url);
      return url.endsWith('/')
        ? response('<!doctype html><html><body>preview</body></html>', { contentType: 'text/html' })
        : response(JSON.stringify({ status: 'ok' }));
    };
    const exitCode = await runPreflightCli({
      config: makeConfig(),
      env: releaseEnv,
      fetchImpl,
      stdout: { write: text => stdout.push(text) },
      stderr: { write: text => stderr.push(text) }
    });

    expect(exitCode).to.equal(0);
    expect(requestedURLs).to.deep.equal([
      'http://127.0.0.1:9725/',
      'http://127.0.0.1:9725/api/v1/deployment/health',
      'http://127.0.0.1:9999/health'
    ]);
    const report = JSON.parse(stdout.join(''));
    expect(stderr).to.deep.equal([]);
    expect(report).to.include({
      kind: 'aetherlink-release-preflight',
      ok: true,
      exitCode: 0,
      profile: 'full'
    });
    expect(report.required.map(check => check.status)).to.deep.equal(['passed', 'passed']);
    expect(report.problems).to.deep.equal([]);
  });

  it('reports optional and external blockers without presenting them as passed or failing the core gate', async function() {
    const stdout = [];
    const exitCode = await runPreflightCli({
      config: makeConfig({ realAccounts: false }),
      env: {
        ...releaseEnv,
        PREFLIGHT_PROFILE: 'local-lite',
        PREFLIGHT_OPTIONAL_BLOCKED: 'smtp, map-sdk',
        PREFLIGHT_EXTERNAL_BLOCKED: 'thingsvis, plugin-voucher, thingsvis'
      },
      fetchImpl: async url => url.endsWith('/')
        ? response('<html><body>preview</body></html>', { contentType: 'text/html' })
        : response(JSON.stringify({ status: 'ok' })),
      stdout: { write: text => stdout.push(text) },
      stderr: { write: () => {} }
    });

    const report = JSON.parse(stdout.join(''));
    expect(exitCode).to.equal(0);
    expect(report.ok).to.equal(true);
    expect(report.optional).to.deep.equal([
      { name: 'smtp', ok: false, status: 'blocked' },
      { name: 'map-sdk', ok: false, status: 'blocked' }
    ]);
    expect(report.externalBlocked).to.deep.equal([
      { name: 'thingsvis', ok: false, status: 'external-blocked' },
      { name: 'plugin-voucher', ok: false, status: 'external-blocked' }
    ]);
  });

  it('emits parseable JSON and a non-zero exit code when required configuration fails', async function() {
    const stdout = [];
    const exitCode = await runPreflightCli({
      config: makeConfig({ realAccounts: false }),
      env: releaseEnv,
      stdout: { write: text => stdout.push(text) },
      stderr: { write: () => {} }
    });

    const report = JSON.parse(stdout.join(''));
    expect(exitCode).to.equal(1);
    expect(report.ok).to.equal(false);
    expect(report.exitCode).to.equal(1);
    expect(report.required).to.deep.include({
      name: 'connectivity',
      ok: false,
      status: 'not-run',
      checks: []
    });
    expect(report.problems.some(problem => problem.includes('placeholder credentials'))).to.equal(true);
  });

  it('does not weaken configuration fail-closed gates or issue requests after they fail', async function() {
    let requestCount = 0;
    const writes = [];
    const exitCode = await runPreflightCli({
      config: makeConfig({ realAccounts: false }),
      env: releaseEnv,
      fetchImpl: async () => {
        requestCount += 1;
        return response(JSON.stringify({ status: 'ok' }));
      },
      stdout: { write: text => writes.push(text) },
      stderr: { write: text => writes.push(text) }
    });

    expect(exitCode).to.equal(1);
    expect(requestCount).to.equal(0);
    expect(writes.join('')).to.include('placeholder credentials');
  });

  it('fails closed when the preview API health path returns the SPA HTML shell', async function() {
    const result = await evaluateConnectivity({
      config: makeConfig(),
      env: releaseEnv,
      fetchImpl: async url => url.endsWith('/')
        ? response('<html><body>preview</body></html>', { contentType: 'text/html' })
        : url.includes('/api/v1/')
          ? response('<html><body>preview fallback</body></html>', { contentType: 'text/html' })
          : response(JSON.stringify({ status: 'ok' }))
    });

    expect(result.ok).to.equal(false);
    expect(result.problems).to.have.length(1);
    expect(result.problems[0]).to.include('/api/v1 health path returned non-JSON content');
    expect(result.problems[0]).to.include('preview fallback');
  });

  it('fails closed when proxied deployment health returns HTTP 200 with a down status', async function() {
    const result = await evaluateConnectivity({
      config: makeConfig(),
      env: releaseEnv,
      fetchImpl: async url => url.endsWith('/')
        ? response('<html><body>preview</body></html>', { contentType: 'text/html' })
        : url.includes('/api/v1/deployment/health')
          ? response(JSON.stringify({ status: 'down' }))
          : response(JSON.stringify({ status: 'ok' }))
    });

    expect(result.ok).to.equal(false);
    expect(result.problems).to.deep.equal([
      '/api/v1 health path reported status "down" from http://127.0.0.1:9725/api/v1/deployment/health; expected "ok"'
    ]);
  });

  it('fails closed when the API target health endpoint is unhealthy', async function() {
    const result = await evaluateConnectivity({
      config: makeConfig(),
      env: releaseEnv,
      fetchImpl: async url => url.endsWith('/')
        ? response('<html><body>preview</body></html>', { contentType: 'text/html' })
        : url === 'http://127.0.0.1:9999/health'
          ? response(JSON.stringify({ status: 'unavailable' }), { status: 503 })
          : response(JSON.stringify({ status: 'ok' }))
    });

    expect(result.ok).to.equal(false);
    expect(result.problems).to.deep.equal([
      'API target health returned HTTP 503 from http://127.0.0.1:9999/health; body: {"status":"unavailable"}'
    ]);
  });

  it('emits parseable JSON and a non-zero exit code when required connectivity fails', async function() {
    const stdout = [];
    const stderr = [];
    const exitCode = await runPreflightCli({
      config: makeConfig(),
      env: releaseEnv,
      fetchImpl: async url => url.endsWith('/')
        ? response('<html><body>preview</body></html>', { contentType: 'text/html' })
        : url.includes('/api/v1/deployment/health')
          ? response(JSON.stringify({ status: 'down' }))
          : response(JSON.stringify({ status: 'ok' })),
      stdout: { write: text => stdout.push(text) },
      stderr: { write: text => stderr.push(text) }
    });

    const report = JSON.parse(stdout.join(''));
    expect(exitCode).to.equal(1);
    expect(report).to.include({
      kind: 'aetherlink-release-preflight',
      ok: false,
      exitCode: 1
    });
    expect(report.required.find(check => check.name === 'connectivity')).to.include({
      ok: false,
      status: 'failed'
    });
    expect(report.problems).to.deep.equal([
      '/api/v1 health path reported status "down" from http://127.0.0.1:9725/api/v1/deployment/health; expected "ok"'
    ]);
    expect(stderr.join('')).to.include('API/E2E preflight failed:');
  });

  it('fails closed with bounded timeout diagnostics', async function() {
    const writes = [];
    const exitCode = await runPreflightCli({
      config: makeConfig(),
      env: releaseEnv,
      timeoutMs: 100,
      fetchImpl: async (url, options) => new Promise((resolve, reject) => {
        options.signal.addEventListener('abort', () => reject(new Error('aborted')), { once: true });
      }),
      stdout: { write: text => writes.push(text) },
      stderr: { write: text => writes.push(text) }
    });

    expect(exitCode).to.equal(1);
    expect(writes.join('')).to.include('timed out after 100ms');
    expect(writes.join('')).to.include('preview page request failed');
  });

  it('checks preview HTML, proxied API JSON, and API target health over real HTTP', async function() {
    const server = http.createServer((request, reply) => {
      if (request.url === '/') {
        reply.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
        reply.end('<!doctype html><html><body>AetherLink preview</body></html>');
        return;
      }
      reply.writeHead(200, { 'Content-Type': 'application/json; charset=utf-8' });
      reply.end(JSON.stringify({ status: 'ok', path: request.url }));
    });
    const address = await listen(server);
    const origin = `http://127.0.0.1:${address.port}`;

    try {
      const result = await evaluateConnectivity({
        env: {
          ...releaseEnv,
          PREVIEW_URL: origin,
          API_TARGET: origin,
          API_BASE_URL: `${origin}/api/v1`,
          HEALTH_URL: `${origin}/health`
        },
        timeoutMs: 1000
      });

      expect(result.ok).to.equal(true);
      expect(result.checks.map(check => check.url)).to.deep.equal([
        `${origin}/`,
        `${origin}/api/v1/deployment/health`,
        `${origin}/health`
      ]);
      expect(result.checks.every(check => check.ok)).to.equal(true);
    } finally {
      await close(server);
    }
  });

  it('fails closed if the centralized release account list is accidentally emptied', function() {
    const result = evaluatePreflight({
      config: {
        ...makeConfig(),
        releaseRequiredAccounts: []
      },
      env: releaseEnv
    });

    expect(result.ok).to.equal(false);
    expect(result.problems).to.deep.equal([
      'releaseRequiredAccounts must list the API/E2E release accounts'
    ]);
  });

  it('rejects preview URLs outside the release preview port', function() {
    const result = evaluatePreflight({
      config: makeConfig(),
      env: {
        ...releaseEnv,
        PREVIEW_URL: 'http://127.0.0.1:5002'
      }
    });

    expect(result.ok).to.equal(false);
    expect(result.problems).to.deep.equal([
      'PREVIEW_URL must use preview port 9725 for release E2E evidence: http://127.0.0.1:5002'
    ]);
  });

  it('rejects API base URL and proxy target origins that point at different backends', function() {
    const result = evaluatePreflight({
      config: {
        ...makeConfig(),
        baseURL: 'http://127.0.0.1:9999/api/v1'
      },
      env: {
        ...releaseEnv,
        API_TARGET: 'http://127.0.0.1:19999'
      }
    });

    expect(result.ok).to.equal(false);
    expect(result.problems).to.deep.equal([
      'API_TARGET origin must match API_BASE_URL/baseURL origin: http://127.0.0.1:19999 !== http://127.0.0.1:9999'
    ]);
  });

  it('keeps the performance runner fail-fast after a live preflight failure', function() {
    const runnerPath = path.resolve(__dirname, '../../performance/scripts/run-tier-benchmark.ps1');
    const source = fs.readFileSync(runnerPath, 'utf8');
    const preflightIndex = source.indexOf('npm run preflight:api-e2e');
    const exitCheckIndex = source.indexOf('if ($LASTEXITCODE -ne 0)', preflightIndex);
    const automationIndex = source.indexOf('node .\\run_tests.js --include-e2e --archive', preflightIndex);

    expect(preflightIndex).to.be.greaterThan(-1);
    expect(exitCheckIndex).to.be.greaterThan(preflightIndex);
    expect(automationIndex).to.be.greaterThan(exitCheckIndex);
  });
});
