/**
 * 文件用途：用于执行预览前端 API 代理服务脚本。
 * 核心逻辑：作为独立 Node 脚本编排本地预检、账号准备、预览代理或页面渲染验证，并输出可诊断结果。
 * 关键注意事项：该脚本会启动本地预览代理，当前任务只做语法检查，不运行服务。
 * 重构建议：后续应把环境解析、错误分类和可复用检查步骤抽到共享库，保持脚本入口薄而明确。
 */

const http = require('http');
const net = require('net');
const path = require('path');
const fs = require('fs');

const rootDir = path.resolve(__dirname, '..', '..');
const distDir = path.join(rootDir, 'frontend', 'dist');
const host = process.env.PREVIEW_PROXY_HOST || '127.0.0.1';
const port = Number(process.env.PREVIEW_PROXY_PORT || 9725);
const apiTarget = process.env.API_TARGET || 'http://127.0.0.1:9999';
// Keep the release preview proxy aligned with the Vite development proxy:
// `/thingsvis-api/*` is a browser-facing prefix and the external service
// exposes the actual API under `/api/v1/*`.  A failed optional service should
// become a concrete 502 JSON response, not the SPA index.html (which would
// make a real integration failure look like an HTML/JSON parsing accident).
const thingsVisApiTarget =
  process.env.THINGSVIS_API_TARGET ||
  process.env.VITE_THINGSVIS_API_URL ||
  'http://127.0.0.1:8000';
const thingsVisProxyPath = '/thingsvis-api';

const contentTypes = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
  '.wav': 'audio/wav',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2'
};

function safeJoin(baseDir, requestPath) {
  const decodedPath = decodeURIComponent(requestPath.split('?')[0]);
  const normalized = path.normalize(decodedPath).replace(/^([/\\])+/, '');
  const resolved = path.resolve(baseDir, normalized);

  if (!resolved.startsWith(baseDir)) {
    return null;
  }
  return resolved;
}

function serveFile(response, filePath) {
  fs.readFile(filePath, (error, data) => {
    if (error) {
      response.writeHead(404);
      response.end('Not found');
      return;
    }

    response.writeHead(200, {
      'Content-Type': contentTypes[path.extname(filePath).toLowerCase()] || 'application/octet-stream'
    });
    response.end(data);
  });
}

function proxyApi(request, response, targetURL) {
  const target = targetURL instanceof URL ? targetURL : new URL(request.url, targetURL);
  const upstream = http.request(
    target,
    {
      method: request.method,
      headers: {
        ...request.headers,
        host: target.host
      }
    },
    upstreamResponse => {
      response.writeHead(upstreamResponse.statusCode || 502, upstreamResponse.headers);
      upstreamResponse.pipe(response);
    }
  );

  upstream.on('error', error => {
    response.writeHead(502, { 'Content-Type': 'application/json; charset=utf-8' });
    response.end(JSON.stringify({ code: 502, message: error.message }));
  });

  request.pipe(upstream);
}

function buildThingsVisTargetURL(requestURL, configuredTarget = thingsVisApiTarget) {
  const request = new URL(requestURL, 'http://preview.local');
  const configured = new URL(configuredTarget);
  const configuredPath = configured.pathname.replace(/\/$/, '');
  const apiBasePath = configuredPath.endsWith('/api/v1')
    ? configuredPath
    : `${configuredPath}/api/v1`.replace(/\/\/+/g, '/');
  const suffix = request.pathname.slice(thingsVisProxyPath.length).replace(/^\//, '');
  const targetPath = `${apiBasePath}/${suffix}`.replace(/\/+/g, '/');
  configured.pathname = targetPath;
  configured.search = request.search;
  return configured;
}

function proxyThingsVisApi(request, response, configuredTarget = thingsVisApiTarget) {
  const target = buildThingsVisTargetURL(request.url, configuredTarget);
  proxyApi(request, response, target);
}

function formatUpgradeHeaders(headers, target) {
  const lines = [`Host: ${target.host}`];

  for (const [name, rawValue] of Object.entries(headers)) {
    if (name.toLowerCase() === 'host') continue;
    const values = Array.isArray(rawValue) ? rawValue : [rawValue];
    for (const value of values) {
      if (value !== undefined) lines.push(`${name}: ${value}`);
    }
  }

  return lines;
}

function proxyWebSocket(request, clientSocket, head, targetURL) {
  let target;
  try {
    target = targetURL instanceof URL ? new URL(request.url, targetURL) : new URL(request.url, targetURL);
  } catch (error) {
    clientSocket.destroy(error);
    return;
  }

  if (target.protocol !== 'http:') {
    clientSocket.destroy(new Error(`WebSocket preview proxy only supports http upstreams: ${target.protocol}`));
    return;
  }

  const upstreamSocket = net.createConnection(
    { host: target.hostname, port: Number(target.port || 80) },
    () => {
      const pathWithQuery = `${target.pathname || '/'}${target.search}`;
      const headers = formatUpgradeHeaders(request.headers, target);
      upstreamSocket.write(`GET ${pathWithQuery} HTTP/1.1\r\n${headers.join('\r\n')}\r\n\r\n`);
      if (head && head.length) upstreamSocket.write(head);
      clientSocket.pipe(upstreamSocket);
      upstreamSocket.pipe(clientSocket);
    }
  );

  const closeBoth = () => {
    if (!clientSocket.destroyed) clientSocket.destroy();
    if (!upstreamSocket.destroyed) upstreamSocket.destroy();
  };

  upstreamSocket.on('error', closeBoth);
  clientSocket.on('error', closeBoth);
  clientSocket.on('close', () => {
    if (!upstreamSocket.destroyed) upstreamSocket.destroy();
  });
  upstreamSocket.on('close', () => {
    if (!clientSocket.destroyed) clientSocket.destroy();
  });
}

function createServer(options = {}) {
  const configuredApiTarget = options.apiTarget || apiTarget;
  const configuredThingsVisApiTarget = options.thingsVisApiTarget || thingsVisApiTarget;
  const configuredDistDir = options.distDir || distDir;

  const server = http.createServer((request, response) => {
  if (request.url.startsWith('/api/') || request.url.startsWith('/uploads')) {
    proxyApi(request, response, configuredApiTarget);
    return;
  }

  if (request.url === thingsVisProxyPath || request.url.startsWith(`${thingsVisProxyPath}/`)) {
    proxyThingsVisApi(request, response, configuredThingsVisApiTarget);
    return;
  }

  const requestedFile = safeJoin(configuredDistDir, request.url);
  if (requestedFile && fs.existsSync(requestedFile) && fs.statSync(requestedFile).isFile()) {
    serveFile(response, requestedFile);
    return;
  }

  serveFile(response, path.join(configuredDistDir, 'index.html'));
  });

  server.on('upgrade', (request, socket, head) => {
    if (request.url.startsWith('/api/') || request.url.startsWith('/uploads')) {
      proxyWebSocket(request, socket, head, configuredApiTarget);
      return;
    }
    socket.destroy();
  });

  return server;
}

if (require.main === module) {
  const server = createServer();
  server.listen(port, host, () => {
    console.log(JSON.stringify({
      previewProxyURL: `http://${host}:${port}`,
      distDir,
      apiTarget,
      thingsVisApiTarget
    }));
  });
}

module.exports = {
  buildThingsVisTargetURL,
  createServer
};
