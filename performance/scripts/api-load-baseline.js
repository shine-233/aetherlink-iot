#!/usr/bin/env node
/**
 * 文件用途：API 基线负载生成与延迟百分位统计（ROADMAP P2.3「零容量数字」的最小可执行闭环）。
 *
 * 背景：`performance/` 里已有 tiers.json（1c2g / 2c4g / 4c8g 三档 SLO）与场景目录，
 * 但 `run-tier-benchmark.ps1` **只抓健康检查并跑既有测试套件，没有负载生成器**——
 * 这就是"路线图里没有任何容量数字"的直接原因：脚手架齐了，尺子没造出来。
 *
 * 核心逻辑：
 *   固定并发 N 个 worker 持续打目标端点，记录每次请求的端到端延迟，最后算
 *   p50/p95/p99/max、吞吐与错误率。全程只用 Node 标准库（http/https），
 *   不引入 k6 / autocannon 等外部依赖——本机网络受限，加依赖本身就是风险。
 *
 * 关键注意事项：
 *   - **这不是 tier 结果**。tiers.json 的档位描述的是资源限制（CPU/内存配额），
 *     本脚本不施加任何配额。因此输出里显式标注 `evidenceKind: "local-baseline"`，
 *     并写明"不可当作 1c2g/2c4g/4c8g 的达标证据"。
 *     把本地笔记本的数字冒充 tier 达标，比没有数字更糟。
 *   - 预热期样本**不计入统计**：首轮请求要付连接建立、连接池填充与 JIT 的代价，
 *     混进 p95 会把基线抬高到没有参考价值。
 *   - 百分位用**最近秩（nearest-rank）**，不是插值：样本量小时插值会给出
 *     一个实际从未发生过的延迟值。
 *   - 错误一律计入错误率，**不丢弃**：只统计成功请求的延迟会得出一个漂亮的假基线。
 *
 * 用法：
 *   node performance/scripts/api-load-baseline.js \
 *     --base-url http://127.0.0.1:9999 --path /health \
 *     --concurrency 4 --duration 10 --warmup 2 --out <报告路径>
 *
 * 重构建议：需要鉴权端点时给 `--header` 传入 `Key: Value`；需要多端点混合时
 * 把 `--path` 改成逗号分隔列表并在这里加权重。
 */

const http = require('http')
const https = require('https')
const fs = require('fs')
const path = require('path')
const os = require('os')

function parseArgs(argv) {
  const args = {}
  for (let index = 2; index < argv.length; index += 1) {
    const token = argv[index]
    if (!token.startsWith('--')) continue
    const key = token.slice(2)
    const next = argv[index + 1]
    if (next === undefined || next.startsWith('--')) {
      args[key] = true
      continue
    }
    args[key] = next
    index += 1
  }
  return args
}

function toNumber(value, fallback) {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

/**
 * nearestRank 百分位：取第 ceil(p/100 * n) 个样本（1-based）。
 * 刻意不用线性插值——小样本下插值会产出"从未实际发生过的延迟值"。
 */
function nearestRank(sortedSamples, percentile) {
  if (sortedSamples.length === 0) return null
  const rank = Math.ceil((percentile / 100) * sortedSamples.length)
  const index = Math.min(Math.max(rank, 1), sortedSamples.length) - 1
  return sortedSamples[index]
}

function requestOnce(url, headers, timeoutMs) {
  return new Promise(resolve => {
    const startedAt = process.hrtime.bigint()
    const transport = url.protocol === 'https:' ? https : http
    const request = transport.request(
      {
        protocol: url.protocol,
        hostname: url.hostname,
        port: url.port,
        path: url.pathname + url.search,
        method: 'GET',
        headers
      },
      response => {
        // 必须把响应体读完，否则连接不会回到池里，后续请求会不断新建连接。
        response.resume()
        response.on('end', () => {
          const elapsedMs = Number(process.hrtime.bigint() - startedAt) / 1e6
          resolve({ ok: response.statusCode >= 200 && response.statusCode < 400, statusCode: response.statusCode, elapsedMs })
        })
      }
    )
    request.setTimeout(timeoutMs, () => {
      request.destroy(new Error(`timeout after ${timeoutMs}ms`))
    })
    request.on('error', error => {
      const elapsedMs = Number(process.hrtime.bigint() - startedAt) / 1e6
      resolve({ ok: false, statusCode: null, elapsedMs, error: error.message })
    })
    request.end()
  })
}

async function runPhase({ url, headers, concurrency, durationMs, timeoutMs, collect }) {
  const deadline = Date.now() + durationMs
  const samples = []
  const failures = []

  async function worker() {
    while (Date.now() < deadline) {
      const result = await requestOnce(url, headers, timeoutMs)
      if (collect) {
        samples.push(result.elapsedMs)
        if (!result.ok) failures.push({ statusCode: result.statusCode, error: result.error })
      }
    }
  }

  await Promise.all(Array.from({ length: concurrency }, () => worker()))
  return { samples, failures }
}

async function main() {
  const args = parseArgs(process.argv)
  const baseUrl = String(args['base-url'] || 'http://127.0.0.1:9999')
  const targetPath = String(args.path || '/health')
  const concurrency = Math.max(1, Math.trunc(toNumber(args.concurrency, 4)))
  const durationSeconds = Math.max(1, toNumber(args.duration, 10))
  const warmupSeconds = Math.max(0, toNumber(args.warmup, 2))
  const timeoutMs = Math.max(100, toNumber(args.timeout, 10000))

  const headers = { 'User-Agent': 'aetherlink-api-baseline/1' }
  if (typeof args.header === 'string') {
    const separator = args.header.indexOf(':')
    if (separator > 0) {
      headers[args.header.slice(0, separator).trim()] = args.header.slice(separator + 1).trim()
    }
  }

  const url = new URL(targetPath, baseUrl)

  if (warmupSeconds > 0) {
    await runPhase({ url, headers, concurrency, durationMs: warmupSeconds * 1000, timeoutMs, collect: false })
  }

  const startedAt = new Date()
  const measured = await runPhase({
    url,
    headers,
    concurrency,
    durationMs: durationSeconds * 1000,
    timeoutMs,
    collect: true
  })
  const finishedAt = new Date()

  const sorted = [...measured.samples].sort((left, right) => left - right)
  const total = measured.samples.length
  const failureCount = measured.failures.length
  const wallSeconds = (finishedAt.getTime() - startedAt.getTime()) / 1000

  const report = {
    schema: 'aetherlink.performance.api-baseline.v1',
    // 明确标注证据种类：本脚本不施加资源配额，因此**不是 tier 结果**。
    evidenceKind: 'local-baseline',
    tierClaim: null,
    tierClaimReason:
      'tiers.json 的 1c2g/2c4g/4c8g 描述资源限制；本脚本未施加任何 CPU/内存配额，' +
      '故不可作为任一档位的达标证据。',
    target: { baseUrl, path: url.pathname + url.search },
    parameters: {
      concurrency,
      durationSeconds,
      warmupSeconds,
      timeoutMs
    },
    host: {
      platform: process.platform,
      arch: process.arch,
      cpuModel: (os.cpus()[0] || {}).model || 'unknown',
      cpuCount: os.cpus().length,
      totalMemoryMb: Math.round(os.totalmem() / (1024 * 1024)),
      loadAverage: os.loadavg()
    },
    window: { startedAt: startedAt.toISOString(), finishedAt: finishedAt.toISOString(), wallSeconds },
    results: {
      requests: total,
      failures: failureCount,
      errorRate: total === 0 ? null : failureCount / total,
      requestsPerSecond: wallSeconds > 0 ? total / wallSeconds : null,
      latencyMs: {
        min: total > 0 ? sorted[0] : null,
        p50: nearestRank(sorted, 50),
        p90: nearestRank(sorted, 90),
        p95: nearestRank(sorted, 95),
        p99: nearestRank(sorted, 99),
        max: total > 0 ? sorted[sorted.length - 1] : null,
        mean: total > 0 ? measured.samples.reduce((sum, value) => sum + value, 0) / total : null
      }
    },
    failureSample: measured.failures.slice(0, 10)
  }

  const rendered = JSON.stringify(report, null, 2)
  if (typeof args.out === 'string' && args.out.length > 0) {
    fs.mkdirSync(path.dirname(path.resolve(args.out)), { recursive: true })
    fs.writeFileSync(path.resolve(args.out), rendered, 'utf8')
  }
  process.stdout.write(rendered + '\n')
}

main().catch(error => {
  process.stderr.write(`api-load-baseline failed: ${error && error.stack ? error.stack : error}\n`)
  process.exitCode = 1
})
