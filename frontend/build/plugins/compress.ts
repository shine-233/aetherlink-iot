/**
 * 文件用途：构建结束后为文本类产物生成 `.gz` 预压缩副本，供 nginx `gzip_static on` 直接发送。
 * 核心逻辑：遍历 outDir，把超过阈值的 js/css/html/svg/json 等文件用 zlib 最高压缩级别写出同名 `.gz`。
 * 关键注意事项：只用 Node 内置 zlib，不新增依赖；原文件保持不变，nginx 找不到 `.gz` 时仍会按需压缩。
 *   预压缩只在 `vite build` 生效，`VITE_PRECOMPRESS=N` 可关闭（例如只做体积分析时）。
 * 重构建议：如需 brotli，可在 nginx 镜像具备 ngx_brotli 模块后再启用 `brotliCompressSync` 分支。
 */
import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import zlib from 'node:zlib'
import type { Plugin, ResolvedConfig } from 'vite'

/** 需要预压缩的扩展名（图片/字体等已压缩格式不在列表中） */
const COMPRESSIBLE_EXTENSIONS = new Set(['.js', '.mjs', '.css', '.html', '.svg', '.json', '.txt', '.xml', '.map'])

/** 小于该字节数的文件压缩收益低于额外请求协商成本，直接跳过 */
const MIN_COMPRESS_BYTES = 1024

function collectFiles(directory: string): string[] {
  const files: string[] = []
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const fullPath = path.join(directory, entry.name)
    if (entry.isDirectory()) {
      files.push(...collectFiles(fullPath))
    } else if (entry.isFile()) {
      files.push(fullPath)
    }
  }
  return files
}

/** 判断文件是否值得生成 `.gz` 副本。导出以便单元测试。 */
export function shouldPrecompress(filePath: string, size: number): boolean {
  if (size < MIN_COMPRESS_BYTES) return false
  return COMPRESSIBLE_EXTENSIONS.has(path.extname(filePath).toLowerCase())
}

/** 为 outDir 下的文本产物写出 `.gz`，只保留确实变小的结果。返回写出的文件数。 */
export function precompressDirectory(outDir: string): number {
  if (!fs.existsSync(outDir)) return 0

  let written = 0
  for (const file of collectFiles(outDir)) {
    if (file.endsWith('.gz')) continue
    const source = fs.readFileSync(file)
    if (!shouldPrecompress(file, source.length)) continue

    const compressed = zlib.gzipSync(source, { level: zlib.constants.Z_BEST_COMPRESSION })
    if (compressed.length >= source.length) continue

    fs.writeFileSync(`${file}.gz`, compressed)
    written += 1
  }
  return written
}

export function setupPrecompressPlugin(): Plugin[] {
  if (process.env.VITE_PRECOMPRESS === 'N') return []

  let config: ResolvedConfig

  return [
    {
      name: 'aetherlink-precompress',
      apply: 'build',
      enforce: 'post',
      configResolved(resolvedConfig) {
        config = resolvedConfig
      },
      closeBundle() {
        const outDir = path.resolve(config.root, config.build.outDir)
        const count = precompressDirectory(outDir)
        config.logger.info(`[precompress] wrote ${count} .gz files to ${path.relative(config.root, outDir) || '.'}`)
      }
    }
  ]
}
