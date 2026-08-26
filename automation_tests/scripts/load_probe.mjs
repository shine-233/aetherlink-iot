// 文件用途：轻量 API 负载探测（无新增依赖，Node 原生 fetch）。
// 核心逻辑：对公共端点（/health、/api/v1/sys_version）按固定 RPS 打流，
//   输出 p50/p95/p99/max、非 2xx 计数与实际达成 RPS，JSON 归档到
//   reports/performance/load-probe-<timestamp>.json，供 nightly compose 栈
//   的容量趋势追踪使用。
// 关键注意事项：这是「公开端点延迟地板」测量，不含认证/写路径/遥测链路；
//   结果不等于 performance/tiers.json 的容量承诺。目标经
//   AETHERLINK_API_BASE_URL 覆盖（compose 栈默认 http://127.0.0.1:9999）。

import { writeFileSync, mkdirSync } from 'node:fs';
import path from 'node:path';

function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`);
  return i > -1 ? process.argv[i + 1] : fallback;
}

const baseUrl = (process.env.AETHERLINK_API_BASE_URL || 'http://127.0.0.1:9999').replace(/\/$/, '');
const durationSec = Number(arg('duration', '30'));
const rps = Number(arg('rps', '20'));
const endpoints = (arg('endpoints', '/health,/api/v1/sys_version')).split(',').map(s => s.trim());

if (!(durationSec > 0 && rps > 0)) {
  console.error('[load-probe] --duration and --rps must be positive numbers');
  process.exit(2);
}

const intervalMs = 1000 / rps;
const results = [];
let scheduled = 0;
const startedAt = Date.now();

async function fire(url) {
  const t0 = performance.now();
  let status = 0;
  try {
    const res = await fetch(url, { signal: AbortSignal.timeout(10_000) });
    status = res.status;
    await res.arrayBuffer();
  } catch {
    status = 0; // 网络错误/超时统一记 0
  }
  results.push({ url, status, latencyMs: performance.now() - t0 });
}

await new Promise(resolve => {
  const timer = setInterval(() => {
    if (scheduled >= durationSec * rps) {
      clearInterval(timer);
      resolve();
      return;
    }
    const url = baseUrl + endpoints[scheduled % endpoints.length];
    scheduled++;
    void fire(url);
  }, intervalMs);
});

// 等待在途请求收尾
const drainDeadline = Date.now() + 15_000;
while (results.length < scheduled && Date.now() < drainDeadline) {
  await new Promise(r => setTimeout(r, 50));
}

const latencies = results.map(r => r.latencyMs).sort((a, b) => a - b);
function percentile(p) {
  if (latencies.length === 0) return null;
  const idx = Math.min(latencies.length - 1, Math.ceil((p / 100) * latencies.length) - 1);
  return Number(latencies[Math.max(0, idx)].toFixed(1));
}
const failures = results.filter(r => r.status < 200 || r.status >= 300).length;
const wallMs = Date.now() - startedAt;

const report = {
  schema: 'aetherlink.performance.load-probe.v1',
  verdict: 'measured',
  boundary: 'unauthenticated public endpoints only; not a tiers.json capacity claim',
  target: baseUrl,
  endpoints,
  requested: { durationSec, rps },
  actual: {
    scheduled,
    completed: results.length,
    achievedRps: Number(((results.length / wallMs) * 1000).toFixed(2)),
    failureCount: failures,
    failureRate: Number((failures / Math.max(1, results.length)).toFixed(4))
  },
  latencyMs: { p50: percentile(50), p95: percentile(95), p99: percentile(99), max: percentile(100) },
  generatedAt: new Date().toISOString()
};

const outDir = path.join(process.cwd(), 'reports', 'performance');
mkdirSync(outDir, { recursive: true });
const outFile = path.join(outDir, `load-probe-${Date.now()}.json`);
writeFileSync(outFile, JSON.stringify(report, null, 2));

console.log('[load-probe] target=%s scheduled=%d completed=%d fail=%d p50=%sms p95=%sms p99=%sms',
  baseUrl, scheduled, results.length, failures,
  report.latencyMs.p50, report.latencyMs.p95, report.latencyMs.p99);
console.log('[load-probe] report written:', outFile);
