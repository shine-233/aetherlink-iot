/**
 * Preview-proxy regression coverage.
 *
 * This starts the real proxy implementation and a local HTTP upstream.  It
 * verifies the same browser-facing `/thingsvis-api` path used by the E2E
 * suite is rewritten to the external service's `/api/v1` contract, including
 * query strings.  A string-only source assertion would not catch the
 * request-target regression that originally made the proxy serve index.html.
 */

const http = require('http');
const net = require('net');
const { expect } = require('chai');
const { createServer, buildThingsVisTargetURL } = require('../scripts/serve_preview_with_api_proxy');

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

function requestJSON(port, path) {
  return new Promise((resolve, reject) => {
    const request = http.get({ host: '127.0.0.1', port, path }, response => {
      const chunks = [];
      response.on('data', chunk => chunks.push(chunk));
      response.on('end', () => {
        const body = Buffer.concat(chunks).toString('utf8');
        resolve({
          statusCode: response.statusCode,
          headers: response.headers,
          body: JSON.parse(body)
        });
      });
    });
    request.on('error', reject);
  });
}

function requestWebSocketUpgrade(port, path) {
  return new Promise((resolve, reject) => {
    const socket = net.connect(port, '127.0.0.1');
    let response = '';
    let settled = false;
    const timeout = setTimeout(() => {
      socket.destroy();
      reject(new Error('timed out waiting for WebSocket upgrade response'));
    }, 5000);

    const fail = error => {
      if (settled) return;
      settled = true;
      clearTimeout(timeout);
      socket.destroy();
      reject(error);
    };

    socket.once('error', fail);
    socket.once('connect', () => {
      socket.write(
        [
          `GET ${path} HTTP/1.1`,
          'Host: preview.local',
          'Connection: Upgrade',
          'Upgrade: websocket',
          'Sec-WebSocket-Version: 13',
          'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==',
          '\r\n'
        ].join('\r\n')
      );
    });
    socket.on('data', chunk => {
      response += chunk.toString('latin1');
      if (!response.includes('\r\n\r\n') || settled) return;
      settled = true;
      clearTimeout(timeout);
      resolve({ response, socket });
    });
  });
}

describe('release preview ThingsVis proxy', function() {
  it('normalizes a target origin to the ThingsVis /api/v1 base path', function() {
    expect(buildThingsVisTargetURL('/thingsvis-api/auth/sso?scope=e2e', 'http://tv.local:8000').toString())
      .to.equal('http://tv.local:8000/api/v1/auth/sso?scope=e2e');
    expect(buildThingsVisTargetURL('/thingsvis-api/projects', 'http://tv.local:8000/api/v1').toString())
      .to.equal('http://tv.local:8000/api/v1/projects');
  });

  it('forwards the real ThingsVis request path instead of returning the SPA shell', async function() {
    let upstreamRequest;
    const upstream = http.createServer((request, response) => {
      upstreamRequest = { method: request.method, url: request.url };
      response.writeHead(200, { 'Content-Type': 'application/json' });
      response.end(JSON.stringify({ ok: true, path: request.url }));
    });
    const upstreamAddress = await listen(upstream);
    const upstreamTarget = `http://127.0.0.1:${upstreamAddress.port}`;

    const preview = createServer({
      thingsVisApiTarget: upstreamTarget,
      distDir: 'C:/aetherlink-nonexistent-preview-dist'
    });
    const previewAddress = await listen(preview);

    try {
      const result = await requestJSON(
        previewAddress.port,
        '/thingsvis-api/auth/sso?scope=e2e'
      );
      expect(result.statusCode).to.equal(200);
      expect(result.body).to.deep.equal({
        ok: true,
        path: '/api/v1/auth/sso?scope=e2e'
      });
      expect(upstreamRequest).to.deep.equal({
        method: 'GET',
        url: '/api/v1/auth/sso?scope=e2e'
      });
    } finally {
      await close(preview);
      await close(upstream);
    }
  });

  it('forwards API WebSocket upgrades and relays the upstream 101 response', async function() {
    let upstreamUpgrade;
    const upstream = http.createServer();
    upstream.on('upgrade', (request, socket) => {
      upstreamUpgrade = { method: request.method, url: request.url, host: request.headers.host };
      socket.write(
        'HTTP/1.1 101 Switching Protocols\r\n' +
        'Upgrade: websocket\r\n' +
        'Connection: Upgrade\r\n' +
        '\r\n'
      );
      socket.end();
    });
    const upstreamAddress = await listen(upstream);
    const preview = createServer({
      apiTarget: `http://127.0.0.1:${upstreamAddress.port}`,
      distDir: 'C:/aetherlink-nonexistent-preview-dist'
    });
    const previewAddress = await listen(preview);
    let clientSocket;

    try {
      const result = await requestWebSocketUpgrade(
        previewAddress.port,
        '/api/v1/device/stream?device_id=e2e-ws'
      );
      clientSocket = result.socket;
      expect(result.response).to.match(/^HTTP\/1\.1 101 Switching Protocols/i);
      expect(result.response).to.match(/Upgrade: websocket/i);
      expect(upstreamUpgrade).to.deep.equal({
        method: 'GET',
        url: '/api/v1/device/stream?device_id=e2e-ws',
        host: `127.0.0.1:${upstreamAddress.port}`
      });
    } finally {
      if (clientSocket && !clientSocket.destroyed) clientSocket.destroy();
      await close(preview);
      await close(upstream);
    }
  });
});
