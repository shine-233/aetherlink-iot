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

const LOCAL_TARGET_HOSTS = new Set(['localhost', '127.0.0.1', '::1']);

function validateProxyTarget(rawValue, name) {
  let target;
  try {
    target = new URL(String(rawValue || '').trim());
  } catch {
    throw new Error(`${name} must be an absolute HTTP URL`);
  }
  if (target.protocol !== 'http:' || !target.hostname) {
    throw new Error(`${name} must use http and include a hostname`);
  }
  if (target.username || target.password) {
    throw new Error(`${name} must not contain embedded credentials`);
  }

  const allowedOrigins = new Set(
    String(process.env.AETHERLINK_ALLOWED_PROXY_ORIGINS || '')
      .split(',')
      .map(value => value.trim())
      .filter(Boolean)
  );
  const local = LOCAL_TARGET_HOSTS.has(target.hostname.toLowerCase());
  if (!local && process.env.AETHERLINK_ALLOW_EXTERNAL_TARGETS !== '1' && !allowedOrigins.has(target.origin)) {
    throw new Error(
      `${name} points to an external origin; configure AETHERLINK_ALLOWED_PROXY_ORIGINS explicitly`
    );
  }
  return target.toString().replace(/\/$/, '');
}

const apiTarget = validateProxyTarget(
  process.env.API_TARGET || 'http://127.0.0.1:9999',
  'API_TARGET'
);
// Keep the release preview proxy aligned with the Vite development proxy:
// `/thingsvis-api/*` is a browser-facing prefix and the external service
// exposes the actual API under `/api/v1/*`.  A failed optional service should
// become a concrete 502 JSON response, not the SPA index.html (which would
// make a real integration failure look like an HTML/JSON parsing accident).
const thingsVisApiTarget =
  validateProxyTarget(
    process.env.THINGSVIS_API_TARGET ||
      process.env.VITE_THINGSVIS_API_URL ||
      'http://127.0.0.1:8000',
    'THINGSVIS_API_TARGET'
  );
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

const PREVIEW_ORIGIN = 'http://preview.local';
const API_PROXY_PREFIXES = ['/api/', '/uploads'];

function pathMatchesPrefix(pathname, prefix) {
  const normalizedPrefix = prefix.replace(/\/$/, '');
  return pathname === normalizedPrefix || pathname.startsWith(`${normalizedPrefix}/`);
}

function requestMatchesPrefix(requestURL, prefix) {
  const requestPath = String(requestURL || '').split('?')[0];
  return pathMatchesPrefix(requestPath, prefix);
}

function parsePreviewRequestURL(requestURL, allowedPrefixes = []) {
  let request;
  try {
    request = new URL(String(requestURL || ''), PREVIEW_ORIGIN);
  } catch {
    throw new Error('preview proxy request URL is invalid');
  }

  // A request such as //attacker.example/path is an absolute URL when it is
  // resolved against a base.  Reject it before any target URL is constructed.
  if (request.origin !== PREVIEW_ORIGIN) {
    throw new Error('preview proxy request must stay on the local preview origin');
  }
  if (allowedPrefixes.length > 0 && !allowedPrefixes.some(prefix => pathMatchesPrefix(request.pathname, prefix))) {
    throw new Error('preview proxy request path is not allowed');
  }
  return request;
}

function buildProxyTargetURL(requestURL, configuredTarget) {
  const request = parsePreviewRequestURL(requestURL, API_PROXY_PREFIXES);
  const target = new URL(configuredTarget instanceof URL ? configuredTarget.toString() : configuredTarget);

  // Copy only the already-routed path and query.  Never use request.url as a
  // URL base, because a network request can otherwise replace the upstream
  // origin with a scheme-relative URL.
  target.pathname = request.pathname;
  target.search = request.search;
  target.hash = '';
  return target;
}

function staticRequestPath(requestURL) {
  let request;
  try {
    request = parsePreviewRequestURL(requestURL);
  } catch {
    return null;
  }

  let decodedPath;
  try {
    decodedPath = decodeURIComponent(request.pathname);
  } catch {
    return null;
  }

  // Keep the lookup key URL-shaped and reject traversal rather than
  // normalizing it into a different file name. Backslashes are separators on
  // Windows and therefore cannot be accepted as URL path data.
  if (!decodedPath.startsWith('/') || decodedPath.includes('\0') || decodedPath.includes('\\')) {
    return null;
  }
  const normalized = path.posix.normalize(decodedPath);
  if (normalized !== decodedPath) return null;
  return normalized;
}

function buildStaticFileIndex(distDirectory) {
  const root = path.resolve(distDirectory);
  const files = new Map();

  function visit(directory) {
    let entries;
    try {
      entries = fs.readdirSync(directory, { withFileTypes: true });
    } catch {
      return;
    }

    for (const entry of entries) {
      const filePath = path.resolve(directory, entry.name);
      if (entry.isDirectory()) {
        visit(filePath);
        continue;
      }
      if (!entry.isFile() || (filePath !== root && !filePath.startsWith(`${root}${path.sep}`))) {
        continue;
      }

      const relativePath = path.relative(root, filePath).split(path.sep).join('/');
      if (!relativePath || relativePath.startsWith('../')) continue;
      files.set(`/${relativePath}`, {
        filePath,
        contentType: contentTypes[path.extname(filePath).toLowerCase()] || 'application/octet-stream'
      });
    }
  }

  visit(root);
  return files;
}

function serveFile(response, fileEntry) {
  if (!fileEntry) {
    response.writeHead(404);
    response.end('Not found');
    return;
  }

  fs.readFile(fileEntry.filePath, (error, data) => {
    if (error) {
      response.writeHead(404);
      response.end('Not found');
      return;
    }

    response.writeHead(200, {
      'Content-Type': fileEntry.contentType
    });
    response.end(data);
  });
}

function proxyApi(request, response, targetURL) {
  let target;
  try {
    // A URL object is accepted only for the already-validated ThingsVis
    // rewrite produced by buildThingsVisTargetURL. String targets still have
    // to pass the normal API-path builder here.
    target = targetURL instanceof URL
      ? new URL(targetURL.toString())
      : buildProxyTargetURL(request.url, targetURL);
  } catch {
    response.writeHead(400, { 'Content-Type': 'application/json; charset=utf-8' });
    response.end(JSON.stringify({ code: 400, message: 'invalid preview proxy request' }));
    return;
  }
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
  const request = parsePreviewRequestURL(requestURL, [thingsVisProxyPath]);
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
    target = buildProxyTargetURL(request.url, targetURL);
  } catch (error) {
    clientSocket.destroy(new Error('invalid preview WebSocket proxy request'));
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
  const configuredApiTarget = validateProxyTarget(options.apiTarget || apiTarget, 'API_TARGET');
  const configuredThingsVisApiTarget = validateProxyTarget(
    options.thingsVisApiTarget || thingsVisApiTarget,
    'THINGSVIS_API_TARGET'
  );
  const configuredDistDir = options.distDir || distDir;
  const staticFiles = buildStaticFileIndex(configuredDistDir);

  const server = http.createServer((request, response) => {
  if (API_PROXY_PREFIXES.some(prefix => requestMatchesPrefix(request.url, prefix))) {
    proxyApi(request, response, configuredApiTarget);
    return;
  }

  if (requestMatchesPrefix(request.url, thingsVisProxyPath)) {
    proxyThingsVisApi(request, response, configuredThingsVisApiTarget);
    return;
  }

  const requestedFile = staticFiles.get(staticRequestPath(request.url));
  serveFile(response, requestedFile || staticFiles.get('/index.html'));
  });

  server.on('upgrade', (request, socket, head) => {
    if (API_PROXY_PREFIXES.some(prefix => requestMatchesPrefix(request.url, prefix))) {
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
  buildProxyTargetURL,
  buildThingsVisTargetURL,
  buildStaticFileIndex,
  createServer
};
