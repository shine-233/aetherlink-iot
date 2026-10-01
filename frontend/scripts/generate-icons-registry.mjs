#!/usr/bin/env node
/**
 * 文件用途：重新生成核心图标清单与分组加载器（icons-registry-core-manifest.ts）。
 *
 * 背景：manifest 的文件头写着「自动生成：请勿手工编辑」，但仓库里一直没有生成器，
 * 导致这条约束无法执行——改图标只能手改一个自称不该手改的文件。
 * 本脚本把「源」明确为 icons-registry-core-buckets/ 下的分组文件（它们持有真实的
 * import 与导出名单），据此重算清单、名字→分组映射与分组加载器。
 *
 * 核心逻辑：
 *   1. 读取 icons-registry-core-buckets/*.ts，从 `export const icons = { ... }` 块解析出
 *      每个分组持有的图标名（这些名字就是清单的事实来源）；
 *   2. coreIconNames = 全部名字按 ASCII 升序；
 *   3. coreIconBucketOf[name] = 该名字所在分组（分组名即文件名的首字母）；
 *   4. coreBucketLoaders[分组] = () => import('./icons-registry-core-buckets/<分组>')...
 *
 * 关键注意事项：
 *   - 分组文件本身不重写（它们是源）。新增图标要改分组文件，再跑本脚本更新 manifest。
 *   - 输出必须与已提交的 manifest 逐字节一致；不一致说明分组文件与清单已漂移，
 *     应当先查清是「漏跑生成器」还是「有人手改了 manifest」。
 *   - `--check` 只校验不写盘，适合放进 CI 或本地门禁。
 *
 * 用法：
 *   node scripts/generate-icons-registry.mjs          # 重新生成
 *   node scripts/generate-icons-registry.mjs --check  # 只校验是否同步（CI 用）
 */
import { readFileSync, writeFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

const here = path.dirname(fileURLToPath(import.meta.url))
const commonDir = path.resolve(here, '../src/components/common')
const bucketsDir = path.join(commonDir, 'icons-registry-core-buckets')
const manifestPath = path.join(commonDir, 'icons-registry-core-manifest.ts')

const HEADER = '// 自动生成：核心图标清单与分组加载器（请勿手工编辑，源：icons-registry-core-buckets/）'

/** 从分组文件里解析 `export const icons = { ... }` 块中的标识符，保持文件内顺序。 */
function parseBucketNames(source) {
  const start = source.indexOf('export const icons = {')
  if (start === -1) {
    throw new Error('分组文件缺少 `export const icons = {` 块')
  }
  const end = source.indexOf('} as unknown as Record<string, Component>', start)
  if (end === -1) {
    throw new Error('分组文件的 icons 块缺少 `} as unknown as Record<string, Component>` 结尾')
  }
  return source
    .slice(start, end)
    .split('\n')
    .map((line) => line.trim().replace(/,$/, ''))
    .filter((line) => /^[A-Za-z_][A-Za-z0-9_]*$/.test(line))
}

function collectBuckets() {
  const files = readdirSync(bucketsDir)
    .filter((f) => f.endsWith('.ts'))
    .sort()

  const buckets = new Map()
  for (const file of files) {
    const bucket = path.basename(file, '.ts')
    const names = parseBucketNames(readFileSync(path.join(bucketsDir, file), 'utf8'))
    if (names.length === 0) {
      throw new Error(`分组 ${bucket} 未解析到任何图标名`)
    }
    // 一致性：分组名必须等于其成员首字母，否则加载器会指向错误的 chunk
    for (const n of names) {
      if (n[0].toUpperCase() !== bucket) {
        throw new Error(`分组 ${bucket} 里的图标 ${n} 首字母不匹配`)
      }
    }
    buckets.set(bucket, names)
  }
  return buckets
}

function buildManifest(buckets) {
  const allNames = [...buckets.values()].flat().sort()

  const dupes = allNames.filter((n, i) => i > 0 && allNames[i - 1] === n)
  if (dupes.length) {
    throw new Error(`图标名重复：${[...new Set(dupes)].join(', ')}`)
  }

  const bucketOf = new Map()
  for (const [bucket, names] of buckets) {
    for (const n of names) bucketOf.set(n, bucket)
  }

  const lines = []
  lines.push(HEADER)
  lines.push("import type { Component } from 'vue'")
  lines.push('')
  lines.push('export const coreIconNames = [')
  lines.push(...allNames.map((n) => `  '${n}'${n === allNames.at(-1) ? '' : ','}`))
  // `as const` 不能省：coreIconNames 的元组字面量类型被 icons-registry-lazy 的
  // createLazyIconRegistry 依赖，去掉会退化成 string[]。
  lines.push('] as const')
  lines.push('')
  lines.push('export const coreIconBucketOf: Record<string, string> = {')
  lines.push(
    ...allNames.map((n) => `  ${n}: '${bucketOf.get(n)}'${n === allNames.at(-1) ? '' : ','}`)
  )
  lines.push('}')
  lines.push('')
  lines.push('export const coreBucketLoaders: Record<string, () => Promise<Record<string, Component>>> = {')
  const bucketKeys = [...buckets.keys()]
  lines.push(
    ...bucketKeys.map(
      (b) =>
        `  ${b}: () => import('./icons-registry-core-buckets/${b}').then((m) => m.icons)${
          b === bucketKeys.at(-1) ? '' : ','
        }`
    )
  )
  lines.push('}')
  return lines.join('\n') + '\n'
}

const buckets = collectBuckets()
const generated = buildManifest(buckets)
const current = readFileSync(manifestPath, 'utf8')

const totalIcons = [...buckets.values()].flat().length
console.log(`分组数: ${buckets.size}  图标数: ${totalIcons}`)

if (process.argv.includes('--check')) {
  if (generated === current) {
    console.log('✓ manifest 与分组文件同步')
    process.exit(0)
  }
  console.error('✗ manifest 与分组文件不同步——请运行 node scripts/generate-icons-registry.mjs')
  process.exit(1)
}

if (generated === current) {
  console.log('✓ 已是最新，无需改写')
  process.exit(0)
}

writeFileSync(manifestPath, generated, 'utf8')
console.log(`✓ 已重新生成 ${path.relative(process.cwd(), manifestPath)}`)
