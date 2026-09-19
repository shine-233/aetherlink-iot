#!/usr/bin/env node
/**
 * 文件用途：P1.5 断云演练的边缘节点模拟器——一个**真实运行的边缘客户端进程**。
 *
 * 它做什么（全部走平台真实 HTTP API，不做任何内部捷径）：
 *   1. boot：POST /edge/nodes 注册（node_id + 版本 + 能力）；
 *   2. 稳态循环：POST /edge/nodes/:id/heartbeat 心跳，每第 2 拍附带一次
 *      POST /edge/nodes/:id/reconcile（上报本地资源版本，接收编排计划）；
 *   3. 断云窗口：从 --outage-after 秒起，持续 --outage-seconds 秒把 API 基址
 *      切到一个不可达端口——心跳失败、带退避重试、本地状态保留；
 *   4. 云恢复：窗口结束后切回真实基址，心跳恢复、reconcile 重新收敛。
 *
 * 回执（JSONL，逐行）是测试与证据的唯一事实源：
 *   { kind: 'registered' | 'heartbeat' | 'reconcile' | 'outage_start' |
 *           'outage_end' | 'error', ok, health, detail, at }
 *
 * 边界（如实）：本进程模拟的是边缘侧的**连接行为与状态保留**（断云重试/恢复收敛），
 * 不模拟真实硬件或边缘业务负载；「断云」从边缘视角是云不可达，与生产语义一致。
 */

const fs = require('fs');
const path = require('path');
const http = require('http');

function parseArgs(argv) {
  const args = {};
  for (let i = 2; i < argv.length; i += 2) {
    const key = argv[i].replace(/^--/, '');
    args[key] = argv[i + 1];
  }
  return args;
}

function post(base, apiPath, body, timeoutMs = 4000, apiToken = '') {
  return new Promise(resolve => {
    const payload = body ? JSON.stringify(body) : '';
    let url;
    try {
      url = new URL(apiPath, base);
    } catch (err) {
      resolve({ ok: false, error: 'bad base: ' + err.message });
      return;
    }
    const headers = { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(payload) };
    // 平台认证：x-token（边缘客户端以租户令牌自证身份）。
    if (apiToken) headers['x-token'] = apiToken;
    const req = http.request(url, {
      method: 'POST',
      headers,
      timeout: timeoutMs
    }, res => {
      let raw = '';
      res.on('data', chunk => { raw += chunk; });
      res.on('end', () => {
        let parsed = null;
        try { parsed = JSON.parse(raw); } catch (_) { /* 非 JSON 视为失败 */ }
        resolve({ ok: res.statusCode === 200 && parsed && parsed.code === 200, status: res.statusCode, body: parsed });
      });
    });
    req.on('timeout', () => { req.destroy(new Error('timeout')); });
    req.on('error', err => resolve({ ok: false, error: err.message }));
    req.end(payload);
  });
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function main() {
  const args = parseArgs(process.argv);
  const apiBase = String(args['api-base'] || '').replace(/\/$/, '');
  const deadBase = 'http://127.0.0.1:9'; // 断云窗口的不可达基址
  const nodeId = args['node-id'];
  const version = args['version'] || '1.0.0';
  const gatewayDeviceId = args['gateway-device-id'];
  const heartbeatIntervalMs = Number(args['heartbeat-interval-ms'] || 2000);
  const outageAfterSec = Number(args['outage-after'] || 8);
  const outageSec = Number(args['outage-seconds'] || 6);
  const runtimeSec = Number(args['runtime'] || 26);

  if (!apiBase || !nodeId || !gatewayDeviceId) {
    console.error('usage: --api-base --node-id --gateway-device-id [--receipts --outage-after --outage-seconds --runtime]');
    process.exit(2);
  }
  const apiToken = String(args['api-token'] || '');
  const receiptPath = args.receipts || path.join(process.cwd(), 'edge-receipts.jsonl');
  const appendReceipt = entry => {
    fs.appendFileSync(receiptPath, JSON.stringify({ at: new Date().toISOString(), ...entry }) + '\n');
  };

  const receiptsDir = path.dirname(receiptPath);
  if (!fs.existsSync(receiptsDir)) fs.mkdirSync(receiptsDir, { recursive: true });
  fs.writeFileSync(receiptPath, '');

  const bootAt = Date.now();
  const inOutage = now => now >= bootAt + outageAfterSec * 1000 && now < bootAt + (outageAfterSec + outageSec) * 1000;
  const outageStarted = { value: false };
  const outageEnded = { value: false };
  let backoffMs = 0;

  // 1. boot：注册
  const register = await post(apiBase, '/api/v1/edge/nodes', {
    node_id: nodeId,
    version,
    capabilities: ['telemetry', 'control', 'sync']
  }, 4000, apiToken);
  appendReceipt({
    kind: 'registered',
    ok: register.ok,
    health: register.body && register.body.data ? register.body.data.health : undefined,
    detail: register.ok ? 'registered' : (register.error || JSON.stringify(register.body && register.body.message))
  });
  if (!register.ok) {
    process.exit(1);
  }

  // 2. 稳态 + 断云 + 恢复循环
  let beat = 0;
  const deadline = bootAt + runtimeSec * 1000;
  while (Date.now() < deadline) {
    await sleep(heartbeatIntervalMs + backoffMs);
    if (Date.now() >= deadline) break;
    beat += 1;
    const now = Date.now();
    const target = inOutage(now) ? deadBase : apiBase;

    if (inOutage(now) && !outageStarted.value) {
      outageStarted.value = true;
      appendReceipt({ kind: 'outage_start', ok: false, detail: 'cloud unreachable window begins' });
    }
    if (!inOutage(now) && outageStarted.value && !outageEnded.value) {
      outageEnded.value = true;
      appendReceipt({ kind: 'outage_end', ok: true, detail: 'cloud reachable again' });
      backoffMs = 0;
    }

    const health = await post(target, `/api/v1/edge/nodes/${nodeId}/heartbeat`, { version }, 4000, apiToken);
    if (health.ok) {
      backoffMs = 0;
      const entry = {
        kind: 'heartbeat',
        ok: true,
        health: health.body && health.body.data ? health.body.data.health : undefined
      };
      // 每第 2 拍做一次 reconcile（上报本地版本，接收计划）。
      if (beat % 2 === 0) {
        const reconcile = await post(target, `/api/v1/edge/nodes/${nodeId}/reconcile`, {
          gateway_device_id: gatewayDeviceId,
          resources: [{ resource_type: 'config', resource_id: 'local-config-1', revision: '1.0.0' }]
        }, 4000, apiToken);
        entry.reconcile = reconcile.ok
          ? {
              ok: true,
              health: reconcile.body && reconcile.body.data ? reconcile.body.data.health : undefined,
              synced: reconcile.body && reconcile.body.data ? reconcile.body.data.synced : undefined
            }
          : { ok: false, error: reconcile.error || (reconcile.body && reconcile.body.message) };
        appendReceipt({ kind: 'reconcile', ...entry.reconcile });
      }
      if (outageEnded.value && !entry.recoveredLogged) {
        appendReceipt({ kind: 'recovered', ok: true, detail: 'first successful heartbeat after outage' });
        entry.recoveredLogged = true;
        appendReceipt(entry);
        continue;
      }
      appendReceipt(entry);
    } else {
      // 断云：退避重试，本地状态保留（不丢心跳节拍计数）。
      backoffMs = Math.min(backoffMs + 500, 1500);
      appendReceipt({
        kind: 'heartbeat',
        ok: false,
        detail: health.error || ('status ' + health.status)
      });
    }
  }
  appendReceipt({ kind: 'done', ok: true, detail: 'runtime elapsed' });
  process.exit(0);
}

main().catch(err => {
  console.error(err.message || err);
  process.exit(1);
});
