/**
 * 文件用途：用于执行 API 与 E2E 预检脚本。
 * 核心逻辑：作为独立 Node 脚本编排本地预检、账号准备、预览代理或页面渲染验证，并输出可诊断结果。
 * 关键注意事项：运行前必须确认目标环境、账号和端口配置，避免把预检失败误判为业务失败。
 * 重构建议：后续应把环境解析、错误分类和可复用检查步骤抽到共享库，保持脚本入口薄而明确。
 */

const runtimeConfig = require('../lib/runtime_config');

const DEFAULT_PREFLIGHT_SETTINGS = Object.freeze({
  profile: 'full',
  previewPort: 9725,
  httpTimeoutMs: 5000
});

const MAX_DIAGNOSTIC_BODY_LENGTH = 240;

const PROFILE_CHECKS = Object.freeze({
  'local-lite': Object.freeze({ releaseAccounts: false }),
  full: Object.freeze({ releaseAccounts: true })
});

function parsePreviewPort(value) {
  const text = String(value).trim();
  if (!/^\d+$/.test(text)) {
    return null;
  }

  const port = Number(text);
  return Number.isInteger(port) && port >= 1 && port <= 65535 ? port : null;
}

function resolvePreflightSettings({ env = {}, profile, previewPort } = {}) {
  const requestedProfile = profile === undefined ? env.PREFLIGHT_PROFILE : profile;
  const requestedPreviewPort = previewPort === undefined ? env.PREVIEW_PORT : previewPort;
  const resolvedProfile = requestedProfile === undefined
    ? DEFAULT_PREFLIGHT_SETTINGS.profile
    : String(requestedProfile).trim().toLowerCase();
  const resolvedPreviewPort = requestedPreviewPort === undefined
    ? DEFAULT_PREFLIGHT_SETTINGS.previewPort
    : parsePreviewPort(requestedPreviewPort);
  const problems = [];

  if (!Object.prototype.hasOwnProperty.call(PROFILE_CHECKS, resolvedProfile)) {
    problems.push(
      `invalid PREFLIGHT_PROFILE/profile: ${requestedProfile === '' ? '<empty>' : requestedProfile}; expected local-lite or full`
    );
  }

  if (resolvedPreviewPort === null) {
    problems.push(
      `invalid PREVIEW_PORT/previewPort: ${requestedPreviewPort === '' ? '<empty>' : requestedPreviewPort}; expected integer 1-65535`
    );
  }

  const effectiveProfile = PROFILE_CHECKS[resolvedProfile]
    ? resolvedProfile
    : DEFAULT_PREFLIGHT_SETTINGS.profile;

  return {
    profile: effectiveProfile,
    previewPort: resolvedPreviewPort === null ? DEFAULT_PREFLIGHT_SETTINGS.previewPort : resolvedPreviewPort,
    checks: PROFILE_CHECKS[effectiveProfile],
    problems
  };
}

function isPlaceholder(value) {
  return !value || String(value).includes('CHANGE_ME');
}

function parseHttpURL(value) {
  try {
    const url = new URL(value);
    return /^https?:$/.test(url.protocol) ? url : null;
  } catch (error) {
    return null;
  }
}

function isPreviewPort(value, previewPort = DEFAULT_PREFLIGHT_SETTINGS.previewPort) {
  const url = parseHttpURL(value);
  return Boolean(url && url.port === String(previewPort));
}

function checkAccount(config, name) {
  const account = (config.accounts || {})[name] || {};
  const missing = [];

  if (isPlaceholder(account.email)) {
    missing.push(`${name}.email`);
  }

  if (isPlaceholder(account.password)) {
    missing.push(`${name}.password`);
  }

  return missing;
}

function buildNextSteps(result) {
  const steps = [];
  const problems = new Set(result.problems || []);
  const missingCredentials = result.missingCredentials || [];

  if (missingCredentials.length > 0) {
    steps.push(
      'prepare real local release accounts first. You can start from `node scripts/prepare_local_accounts.js` and then replace any remaining placeholder credentials in ignored local env files.'
    );
  }

  if ([...problems].some(problem => problem.includes('FRONTEND_URL/frontendURL') || problem.includes('PREVIEW_URL'))) {
    steps.push(
      'point both `FRONTEND_URL` and `PREVIEW_URL` at the release preview proxy on port `9725`, for example `http://127.0.0.1:9725`.'
    );
  }

  if ([...problems].some(problem => problem.includes('API_TARGET'))) {
    steps.push(
      'set `API_TARGET` to the real backend origin used by `API_BASE_URL`, for example `http://127.0.0.1:9999`.'
    );
  }

  if (problems.has('PLAYWRIGHT_USE_PREVIEW_PROXY must be 1 for release E2E evidence on port 9725')) {
    steps.push('set `PLAYWRIGHT_USE_PREVIEW_PROXY=1` so Playwright uses the preview API proxy.');
  }

  if (problems.has('PLAYWRIGHT_REUSE_EXISTING_SERVER must be 0 so Playwright starts the preview API proxy for release evidence')) {
    steps.push('set `PLAYWRIGHT_REUSE_EXISTING_SERVER=0` so Playwright starts the preview API proxy instead of reusing a stale server.');
  }

  if (steps.length === 0) {
    steps.push('re-check the local release env, preview proxy, and backend target before collecting release API/E2E evidence.');
  }

  return steps;
}

function evaluatePreflight({ config = runtimeConfig, env = process.env, profile, previewPort } = {}) {
  const settings = resolvePreflightSettings({ env, profile, previewPort });
  const requiredAccounts = config.releaseRequiredAccounts || [];
  const missing = settings.checks.releaseAccounts
    ? requiredAccounts.flatMap(name => checkAccount(config, name))
    : [];
  const problems = [...settings.problems];
  const baseURL = parseHttpURL(config.baseURL);
  const frontendURL = parseHttpURL(config.frontendURL);
  const previewURL = parseHttpURL(env.PREVIEW_URL);
  const apiTarget = parseHttpURL(env.API_TARGET);
  const portLabel = settings.previewPort;

  if (settings.checks.releaseAccounts && requiredAccounts.length === 0) {
    problems.push('releaseRequiredAccounts must list the API/E2E release accounts');
  }

  if (missing.length > 0) {
    problems.push(`placeholder credentials: ${missing.join(', ')}`);
  }

  if (!baseURL) {
    problems.push(`invalid API_BASE_URL/baseURL: ${config.baseURL || '<empty>'}`);
  }

  if (!frontendURL) {
    problems.push(`invalid FRONTEND_URL/frontendURL: ${config.frontendURL || '<empty>'}`);
  }

  if (env.PLAYWRIGHT_USE_PREVIEW_PROXY !== '1') {
    problems.push(`PLAYWRIGHT_USE_PREVIEW_PROXY must be 1 for release E2E evidence on port ${portLabel}`);
  }

  if (env.PLAYWRIGHT_REUSE_EXISTING_SERVER !== '0') {
    problems.push('PLAYWRIGHT_REUSE_EXISTING_SERVER must be 0 so Playwright starts the preview API proxy for release evidence');
  }

  if (!isPreviewPort(config.frontendURL, settings.previewPort)) {
    problems.push(`FRONTEND_URL/frontendURL must use preview port ${portLabel} for release E2E evidence: ${config.frontendURL || '<empty>'}`);
  }

  if (!previewURL) {
    problems.push(`invalid PREVIEW_URL: ${env.PREVIEW_URL || '<empty>'}`);
  } else if (!isPreviewPort(env.PREVIEW_URL, settings.previewPort)) {
    problems.push(`PREVIEW_URL must use preview port ${portLabel} for release E2E evidence: ${env.PREVIEW_URL}`);
  }

  if (!apiTarget) {
    problems.push(`API_TARGET must be an HTTP URL for preview proxying: ${env.API_TARGET || '<empty>'}`);
  } else if (baseURL && apiTarget.origin !== baseURL.origin) {
    problems.push(`API_TARGET origin must match API_BASE_URL/baseURL origin: ${apiTarget.origin} !== ${baseURL.origin}`);
  }

  return {
    ok: problems.length === 0,
    problems,
    missingCredentials: missing,
    summary: {
      baseURL: config.baseURL,
      frontendURL: config.frontendURL,
      previewURL: env.PREVIEW_URL,
      apiTarget: env.API_TARGET,
      previewProxy: env.PLAYWRIGHT_USE_PREVIEW_PROXY,
      reuseExistingServer: env.PLAYWRIGHT_REUSE_EXISTING_SERVER,
      accounts: requiredAccounts,
      settings: {
        profile: settings.profile,
        previewPort: settings.previewPort
      },
      checks: {
        releaseAccounts: settings.checks.releaseAccounts
      }
    }
  };
}

function parseHttpTimeout(value) {
  if (value === undefined) {
    return DEFAULT_PREFLIGHT_SETTINGS.httpTimeoutMs;
  }

  const text = String(value).trim();
  if (!/^\d+$/.test(text)) {
    return null;
  }

  const timeout = Number(text);
  return Number.isInteger(timeout) && timeout >= 100 && timeout <= 60000 ? timeout : null;
}

function responseContentType(response) {
  return String(response.headers && response.headers.get
    ? response.headers.get('content-type') || ''
    : '').toLowerCase();
}

function diagnosticBody(text) {
  return String(text).replace(/\s+/g, ' ').trim().slice(0, MAX_DIAGNOSTIC_BODY_LENGTH) || '<empty>';
}

async function fetchWithTimeout(url, { fetchImpl = globalThis.fetch, timeoutMs } = {}) {
  if (typeof fetchImpl !== 'function') {
    throw new Error('fetch implementation is unavailable');
  }

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetchImpl(url, {
      method: 'GET',
      headers: { accept: 'application/json, text/html;q=0.9' },
      redirect: 'follow',
      signal: controller.signal
    });
    const text = await response.text();
    return { response, text };
  } catch (error) {
    if (controller.signal.aborted) {
      throw new Error(`timed out after ${timeoutMs}ms`);
    }
    throw error;
  } finally {
    clearTimeout(timer);
  }
}

async function checkHTTPResource(check, options) {
  const startedAt = Date.now();
  try {
    const { response, text } = await fetchWithTimeout(check.url, options);
    const contentType = responseContentType(response);
    const base = {
      name: check.name,
      url: check.url,
      status: response.status,
      contentType,
      elapsedMs: Date.now() - startedAt
    };

    if (!response.ok) {
      return {
        ...base,
        ok: false,
        problem: `${check.name} returned HTTP ${response.status} from ${check.url}; body: ${diagnosticBody(text)}`
      };
    }

    if (check.expected === 'html') {
      const isHTML = contentType.includes('text/html') && /<html(?:\s|>)/i.test(text);
      return isHTML
        ? { ...base, ok: true, bytes: Buffer.byteLength(text) }
        : {
            ...base,
            ok: false,
            problem: `${check.name} did not return an HTML page from ${check.url}; content-type: ${contentType || '<missing>'}; body: ${diagnosticBody(text)}`
          };
    }

    if (!contentType.includes('json')) {
      return {
        ...base,
        ok: false,
        problem: `${check.name} returned non-JSON content from ${check.url}; content-type: ${contentType || '<missing>'}; body: ${diagnosticBody(text)}`
      };
    }

    try {
      const body = JSON.parse(text);
      if (body === null || typeof body !== 'object') {
        throw new Error('JSON body must be an object or array');
      }
      if (check.requiredStatus !== undefined && body.status !== check.requiredStatus) {
        return {
          ...base,
          ok: false,
          problem: `${check.name} reported status ${JSON.stringify(body.status)} from ${check.url}; expected ${JSON.stringify(check.requiredStatus)}`
        };
      }
      return { ...base, ok: true, bodyType: Array.isArray(body) ? 'array' : 'object' };
    } catch (error) {
      return {
        ...base,
        ok: false,
        problem: `${check.name} returned invalid JSON from ${check.url}: ${error.message}; body: ${diagnosticBody(text)}`
      };
    }
  } catch (error) {
    return {
      name: check.name,
      url: check.url,
      ok: false,
      elapsedMs: Date.now() - startedAt,
      problem: `${check.name} request failed for ${check.url}: ${error.message}`
    };
  }
}

function buildConnectivityChecks({ config = runtimeConfig, env = process.env } = {}) {
  const previewURL = new URL(env.PREVIEW_URL);
  const apiTarget = new URL(env.API_TARGET);
  const apiBaseURL = new URL(config.baseURL);
  const healthURL = new URL(config.healthURL || '/health', apiTarget);
  const apiHealthPath = `${apiBaseURL.pathname.replace(/\/$/, '')}/deployment/health`;

  return [
    { name: 'preview page', url: new URL('/', previewURL).toString(), expected: 'html' },
    { name: '/api/v1 health path', url: new URL(apiHealthPath, previewURL).toString(), expected: 'json', requiredStatus: 'ok' },
    { name: 'API target health', url: new URL(`${healthURL.pathname}${healthURL.search}`, apiTarget).toString(), expected: 'json' }
  ];
}

async function evaluateConnectivity({ config = runtimeConfig, env = process.env, fetchImpl, timeoutMs } = {}) {
  const resolvedTimeout = timeoutMs === undefined
    ? parseHttpTimeout(env.PREFLIGHT_HTTP_TIMEOUT_MS)
    : parseHttpTimeout(timeoutMs);
  if (resolvedTimeout === null) {
    return {
      ok: false,
      problems: [`invalid PREFLIGHT_HTTP_TIMEOUT_MS/timeoutMs: ${timeoutMs === undefined ? env.PREFLIGHT_HTTP_TIMEOUT_MS : timeoutMs}; expected integer 100-60000`],
      checks: []
    };
  }

  const checks = [];
  for (const check of buildConnectivityChecks({ config, env })) {
    checks.push(await checkHTTPResource(check, { fetchImpl, timeoutMs: resolvedTimeout }));
  }

  return {
    ok: checks.every(check => check.ok),
    problems: checks.filter(check => !check.ok).map(check => check.problem),
    timeoutMs: resolvedTimeout,
    checks
  };
}

function writeFailure(result, stderr) {
  stderr.write('API/E2E preflight failed:\n');
  for (const problem of result.problems) {
    stderr.write(`- ${problem}\n`);
  }
  stderr.write('\nNext steps:\n');
  for (const step of buildNextSteps(result)) {
    stderr.write(`- ${step}\n`);
  }
}

function parseBlockedChecks(value) {
  if (!value) {
    return [];
  }

  return [...new Set(String(value)
    .split(',')
    .map(item => item.trim())
    .filter(Boolean))];
}

function buildPreflightReport({ result, connectivity, env = process.env }) {
  const configurationCheck = {
    name: 'configuration',
    ok: result.ok,
    status: result.ok ? 'passed' : 'failed'
  };
  const connectivityCheck = connectivity
    ? {
        name: 'connectivity',
        ok: connectivity.ok,
        status: connectivity.ok ? 'passed' : 'failed',
        checks: connectivity.checks
      }
    : {
        name: 'connectivity',
        ok: false,
        status: 'not-run',
        checks: []
      };
  const required = [configurationCheck, connectivityCheck];
  const optional = parseBlockedChecks(env.PREFLIGHT_OPTIONAL_BLOCKED).map(name => ({
    name,
    ok: false,
    status: 'blocked'
  }));
  const externalBlocked = parseBlockedChecks(env.PREFLIGHT_EXTERNAL_BLOCKED).map(name => ({
    name,
    ok: false,
    status: 'external-blocked'
  }));
  const problems = [
    ...(result.problems || []),
    ...((connectivity && connectivity.problems) || [])
  ];
  const ok = required.every(check => check.ok);

  return {
    kind: 'aetherlink-release-preflight',
    ok,
    exitCode: ok ? 0 : 1,
    profile: result.summary.settings.profile,
    required,
    optional,
    externalBlocked,
    problems,
    checks: {
      required: required.length,
      optional: optional.length,
      externalBlocked: externalBlocked.length
    },
    summary: {
      ...result.summary,
      connectivity: connectivity
        ? { timeoutMs: connectivity.timeoutMs, checks: connectivity.checks }
        : { status: 'not-run', checks: [] }
    }
  };
}

async function runPreflightCli({
  config = runtimeConfig,
  env = process.env,
  stdout = process.stdout,
  stderr = process.stderr,
  fetchImpl,
  timeoutMs
} = {}) {
  const result = evaluatePreflight({ config, env });

  if (!result.ok) {
    const report = buildPreflightReport({ result, env });
    writeFailure(result, stderr);
    stdout.write(`${JSON.stringify(report)}\n`);
    return report.exitCode;
  }

  const connectivity = await evaluateConnectivity({ config, env, fetchImpl, timeoutMs });
  const report = buildPreflightReport({ result, connectivity, env });
  if (!connectivity.ok) {
    writeFailure({ ...result, problems: connectivity.problems }, stderr);
  }
  stdout.write(`${JSON.stringify(report)}\n`);
  return report.exitCode;
}

if (require.main === module) {
  runPreflightCli()
    .then(exitCode => {
      process.exitCode = exitCode;
    })
    .catch(error => {
      process.stderr.write(`API/E2E preflight failed unexpectedly: ${error.stack || error}\n`);
      process.exitCode = 1;
    });
}

module.exports = {
  parsePreviewPort,
  resolvePreflightSettings,
  isPlaceholder,
  parseHttpURL,
  isPreviewPort,
  checkAccount,
  buildNextSteps,
  evaluatePreflight,
  parseHttpTimeout,
  fetchWithTimeout,
  checkHTTPResource,
  buildConnectivityChecks,
  evaluateConnectivity,
  parseBlockedChecks,
  buildPreflightReport,
  runPreflightCli
};