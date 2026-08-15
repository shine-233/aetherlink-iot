/**
 * 文件用途：验证本地 preflight 编排器的配置推导、构建门禁和 preview proxy 生命周期。
 * 核心逻辑：使用临时构建目录与真实本地 HTTP server，确保严格 preflight 前后服务可靠启动和关闭。
 * 关键注意事项：测试不会访问真实后端或账号，也不能替代完整发布 E2E 证据。
 * 重构建议：新增生命周期步骤时同时覆盖成功、失败和异常清理路径。
 */

const fs = require('fs');
const os = require('os');
const path = require('path');
const { expect } = require('chai');

const {
  resolveLocalPreflightOptions,
  assertPreviewBuild,
  runLocalPreflight
} = require('../scripts/preflight_local');
const { createServer } = require('../scripts/serve_preview_with_api_proxy');

function temporaryPreviewBuild() {
  const distDir = fs.mkdtempSync(path.join(os.tmpdir(), 'aetherlink-preflight-'));
  fs.writeFileSync(path.join(distDir, 'index.html'), '<!doctype html><html><body>preview</body></html>');
  return distDir;
}

function removeDirectory(directory) {
  fs.rmSync(directory, { recursive: true, force: true });
}

describe('local API/E2E preflight orchestrator [00_preflight_local]', function() {
  it('derives a local-lite proxy environment without mutating caller input', function() {
    const env = {};
    const config = {
      baseURL: 'http://127.0.0.1:9999/api/v1',
      frontendURL: 'http://127.0.0.1:5002'
    };

    const result = resolveLocalPreflightOptions({ env, config });

    expect(result).to.deep.include({
      previewHost: '127.0.0.1',
      previewPort: 9725,
      previewURL: 'http://127.0.0.1:9725',
      apiTarget: 'http://127.0.0.1:9999'
    });
    expect(result.config.frontendURL).to.equal('http://127.0.0.1:9725');
    expect(result.env).to.include({
      PREFLIGHT_PROFILE: 'local-lite',
      PREVIEW_PORT: '9725',
      PREVIEW_URL: 'http://127.0.0.1:9725',
      API_TARGET: 'http://127.0.0.1:9999',
      PLAYWRIGHT_USE_PREVIEW_PROXY: '1',
      PLAYWRIGHT_REUSE_EXISTING_SERVER: '0'
    });
    expect(env).to.deep.equal({});
    expect(config.frontendURL).to.equal('http://127.0.0.1:5002');
  });

  it('honors custom preview settings and explicit API target', function() {
    const result = resolveLocalPreflightOptions({
      env: {
        PREVIEW_PROXY_HOST: 'localhost',
        PREVIEW_PROXY_PORT: '10825',
        PREVIEW_DIST_DIR: 'C:/preview-dist',
        API_TARGET: 'http://127.0.0.1:19999'
      },
      config: { baseURL: 'http://127.0.0.1:9999/api/v1' }
    });

    expect(result.previewHost).to.equal('localhost');
    expect(result.previewPort).to.equal(10825);
    expect(result.previewURL).to.equal('http://localhost:10825');
    expect(result.apiTarget).to.equal('http://127.0.0.1:19999');
    expect(result.distDir).to.equal('C:/preview-dist');
  });

  it('fails fast when the frontend preview build is missing', function() {
    const missing = path.join(os.tmpdir(), `aetherlink-missing-${Date.now()}`);
    expect(() => assertPreviewBuild(missing)).to.throw('frontend preview build is missing');
  });

  for (const scenario of [
    { name: 'success', exitCode: 0 },
    { name: 'failed strict preflight', exitCode: 1 },
    { name: 'thrown strict preflight', error: new Error('preflight exploded') }
  ]) {
    it(`closes the auto-started proxy after ${scenario.name}`, async function() {
      const distDir = temporaryPreviewBuild();
      let captured;
      let server;
      const runPreflightImpl = async options => {
        captured = options;
        if (scenario.error) throw scenario.error;
        return scenario.exitCode;
      };
      const createServerImpl = options => {
        server = createServer(options);
        return server;
      };

      try {
        const invocation = runLocalPreflight({
          env: {
            PREVIEW_PORT: '0',
            PREVIEW_DIST_DIR: distDir,
            API_TARGET: 'http://127.0.0.1:9999'
          },
          config: { baseURL: 'http://127.0.0.1:9999/api/v1' },
          createServerImpl,
          runPreflightImpl,
          stdout: { write: () => {} },
          stderr: { write: () => {} }
        });

        if (scenario.error) {
          let caught;
          try {
            await invocation;
          } catch (error) {
            caught = error;
          }
          expect(caught).to.equal(scenario.error);
        } else {
          expect(await invocation).to.equal(scenario.exitCode);
        }
        expect(captured.env.PREFLIGHT_PROFILE).to.equal('local-lite');
        expect(captured.env.PLAYWRIGHT_USE_PREVIEW_PROXY).to.equal('1');
        expect(server.listening).to.equal(false);
      } finally {
        if (server && server.listening) {
          await new Promise(resolve => server.close(resolve));
        }
        removeDirectory(distDir);
      }
    });
  }
});
