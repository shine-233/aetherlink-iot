/**
 * 文件用途：执行可自启动预览代理的本地 API/E2E 预检。
 * 核心逻辑：校验前端构建产物，启动一次性 preview proxy，复用严格 preflight 配置与连通性检查，最后可靠关闭服务。
 * 关键注意事项：该入口使用 local-lite 账号范围，但仍要求真实后端可达；完整发布证据必须继续使用 preflight:api-e2e 的 full profile。
 * 重构建议：如后续增加更多本地依赖，仅扩展显式生命周期步骤，不要把业务 E2E 混入预检。
 */

const fs = require('fs');
const path = require('path');

const runtimeConfig = require('../lib/runtime_config');
const { runPreflightCli } = require('./preflight_api_e2e');
const { createServer } = require('./serve_preview_with_api_proxy');

const DEFAULT_PREVIEW_HOST = '127.0.0.1';
const DEFAULT_PREVIEW_PORT = 9725;

function resolveLocalPreflightOptions({ env = process.env, config = runtimeConfig } = {}) {
  const previewPort = Number(env.PREVIEW_PORT || env.PREVIEW_PROXY_PORT || DEFAULT_PREVIEW_PORT);
  const previewHost = env.PREVIEW_PROXY_HOST || DEFAULT_PREVIEW_HOST;
  const previewURL = `http://${previewHost}:${previewPort}`;
  const baseURL = new URL(config.baseURL);
  const apiTarget = env.API_TARGET || baseURL.origin;

  return {
    previewHost,
    previewPort,
    previewURL,
    apiTarget,
    distDir: env.PREVIEW_DIST_DIR || path.resolve(__dirname, '..', '..', 'frontend', 'dist'),
    config: {
      ...config,
      frontendURL: previewURL
    },
    env: {
      ...env,
      PREFLIGHT_PROFILE: 'local-lite',
      PREVIEW_PORT: String(previewPort),
      PREVIEW_PROXY_HOST: previewHost,
      PREVIEW_PROXY_PORT: String(previewPort),
      PREVIEW_URL: previewURL,
      API_TARGET: apiTarget,
      PLAYWRIGHT_USE_PREVIEW_PROXY: '1',
      PLAYWRIGHT_REUSE_EXISTING_SERVER: '0'
    }
  };
}

function assertPreviewBuild(distDir) {
  const indexPath = path.join(distDir, 'index.html');
  if (!fs.existsSync(indexPath) || !fs.statSync(indexPath).isFile()) {
    throw new Error(`frontend preview build is missing: ${indexPath}; run \`pnpm build\` in frontend first`);
  }
}

function listen(server, port, host) {
  return new Promise((resolve, reject) => {
    const onError = error => {
      server.removeListener('listening', onListening);
      reject(error);
    };
    const onListening = () => {
      server.removeListener('error', onError);
      resolve();
    };
    server.once('error', onError);
    server.once('listening', onListening);
    server.listen(port, host);
  });
}

function close(server) {
  return new Promise((resolve, reject) => {
    server.close(error => (error ? reject(error) : resolve()));
  });
}

async function runLocalPreflight({
  env = process.env,
  config = runtimeConfig,
  stdout = process.stdout,
  stderr = process.stderr,
  createServerImpl = createServer,
  runPreflightImpl = runPreflightCli
} = {}) {
  const options = resolveLocalPreflightOptions({ env, config });
  assertPreviewBuild(options.distDir);
  const server = createServerImpl({ apiTarget: options.apiTarget, distDir: options.distDir });

  try {
    await listen(server, options.previewPort, options.previewHost);
    return await runPreflightImpl({
      config: options.config,
      env: options.env,
      stdout,
      stderr
    });
  } finally {
    if (server.listening) {
      await close(server);
    }
  }
}

if (require.main === module) {
  runLocalPreflight()
    .then(exitCode => {
      process.exitCode = exitCode;
    })
    .catch(error => {
      process.stderr.write(`Local API/E2E preflight failed: ${error.stack || error}\n`);
      process.exitCode = 1;
    });
}

module.exports = {
  DEFAULT_PREVIEW_HOST,
  DEFAULT_PREVIEW_PORT,
  resolveLocalPreflightOptions,
  assertPreviewBuild,
  listen,
  close,
  runLocalPreflight
};
