#!/usr/bin/env node
// 文件用途: src/core 可达性守卫。从 src/main.ts 出发遍历静态 import / export-from / 动态 import() /
//   import.meta.glob，若 src/core 下存在不可达的非测试模块则以非零码退出。
// 背景: 2026-10 删除了约 4.2 万行从入口不可达的 core/data-architecture 与 core/interaction-system，
//   本守卫防止同类“只有自己和测试在用”的死代码层再次积累。
// 关键注意事项: 解析是基于正则的近似实现，宁可多算可达（glob 按目录前缀放宽匹配），不可误报；
//   别名与 vite.config.ts 保持一致: `@/` -> src，`~/` -> 工作区根（frontend 的上一级）。
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const RESOLVE_SUFFIXES = [
  '',
  '.ts',
  '.tsx',
  '.js',
  '.mjs',
  '.vue',
  '/index.ts',
  '/index.tsx',
  '/index.js',
  '/index.vue'
]
const SOURCE_FILE = /\.(vue|ts|tsx|js|mjs)$/
const TEST_FILE = /(\.(test|spec)\.[jt]sx?$)|([\\/]__tests__[\\/])/

const IMPORT_RE =
  /(?:import|export)\s[^'"`;]*?from\s*['"]([^'"]+)['"]|import\s*\(\s*['"]([^'"]+)['"]\s*\)|import\s+['"]([^'"]+)['"]|import\.meta\.glob(?:<[^>]*>)?\(\s*(\[[^\]]*\]|['"][^'"]+['"])/g

const toPosix = (p) => p.split(path.sep).join('/')

function isFile(p) {
  try {
    return fs.statSync(p).isFile()
  } catch {
    return false
  }
}

/** 把别名或相对说明符映射到绝对基路径；裸包名返回 null。 */
export function aliasBase(fromFile, spec, { srcDir, workspaceDir }) {
  if (spec.startsWith('@/')) return path.join(srcDir, spec.slice(2))
  if (spec.startsWith('~/')) return path.join(workspaceDir, spec.slice(2))
  if (spec.startsWith('./') || spec.startsWith('../')) return path.resolve(path.dirname(fromFile), spec)
  return null
}

/** 解析一个导入说明符到真实文件（尝试扩展名与 index），不存在返回 null。 */
export function resolveSpecifier(fromFile, spec, dirs) {
  const base = aliasBase(fromFile, spec.split('?')[0], dirs)
  if (!base) return null
  for (const suffix of RESOLVE_SUFFIXES) {
    // normalize: '/index.ts' 后缀在 Windows 上会产生混合分隔符，导致同一文件以两种拼写进入集合。
    if (isFile(base + suffix)) return path.normalize(base + suffix)
  }
  return null
}

function listFiles(dir) {
  const out = []
  if (!fs.existsSync(dir)) return out
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, entry.name)
    if (entry.isDirectory()) out.push(...listFiles(p))
    else out.push(p)
  }
  return out
}

/** 展开 import.meta.glob 模式：取首个通配符前的目录，按扩展名放宽匹配其下所有源文件。 */
export function expandGlob(fromFile, pattern, dirs) {
  const base = aliasBase(fromFile, pattern, dirs)
  if (!base) return []
  const head = toPosix(base).split(/[*{]/)[0]
  const dir = head.slice(0, head.lastIndexOf('/') + 1)
  const extMatch = pattern.match(/\.(\{[^}]+\}|[a-z]+)$/i)
  const exts = extMatch ? extMatch[1].replace(/[{}]/g, '').split(',') : null
  return listFiles(dir).filter((p) => {
    if (!SOURCE_FILE.test(p) || TEST_FILE.test(p)) return false
    return !exts || exts.some((e) => p.endsWith(`.${e}`))
  })
}

/** 提取一个源文件内容里的全部依赖文件。 */
export function collectDependencies(fromFile, text, dirs) {
  const deps = []
  IMPORT_RE.lastIndex = 0
  let m
  while ((m = IMPORT_RE.exec(text))) {
    if (m[4]) {
      for (const quoted of m[4].match(/['"][^'"]+['"]/g) || []) {
        const pattern = quoted.slice(1, -1)
        if (!pattern.startsWith('!')) deps.push(...expandGlob(fromFile, pattern, dirs))
      }
      continue
    }
    const resolved = resolveSpecifier(fromFile, m[1] || m[2] || m[3], dirs)
    if (resolved) deps.push(resolved)
  }
  return deps
}

/** 从入口出发做 BFS，返回可达文件绝对路径集合。 */
export function walkReachable(entry, dirs) {
  const seen = new Set()
  const queue = [path.normalize(entry)]
  while (queue.length) {
    const file = queue.pop()
    if (seen.has(file) || !isFile(file)) continue
    seen.add(file)
    queue.push(...collectDependencies(file, fs.readFileSync(file, 'utf8'), dirs))
  }
  return seen
}

/** 返回 coreDir 下不可达的非测试源文件（相对 frontendDir 的 posix 路径）。 */
export function findUnreachableCore(frontendDir) {
  const srcDir = path.join(frontendDir, 'src')
  const dirs = { srcDir, workspaceDir: path.resolve(frontendDir, '..') }
  const reachable = walkReachable(path.join(srcDir, 'main.ts'), dirs)
  const unreachable = listFiles(path.join(srcDir, 'core'))
    .filter((p) => SOURCE_FILE.test(p) && !TEST_FILE.test(p) && !p.endsWith('.d.ts'))
    .filter((p) => !reachable.has(p))
    .map((p) => toPosix(path.relative(frontendDir, p)))
    .sort()
  return { reachableCount: reachable.size, unreachable }
}

/**
 * 已知不可达债务（只能缩减）。script-engine 在 2026-10 审计时全仓零调用方（仅 README 提及），
 * 待单独评审删除；删除后必须移除此条，否则守卫以“过期条目”失败。
 */
export const KNOWN_UNREACHABLE = ['src/core/script-engine/']

/** 把不可达列表按已知债务拆分，并找出已不再匹配任何不可达文件的过期条目。 */
export function classifyUnreachable(unreachable, known = KNOWN_UNREACHABLE) {
  const isKnown = (p) => known.some((prefix) => p.startsWith(prefix))
  return {
    fresh: unreachable.filter((p) => !isKnown(p)),
    tolerated: unreachable.filter(isKnown),
    stale: known.filter((prefix) => !unreachable.some((p) => p.startsWith(prefix)))
  }
}

const isCli = process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
if (isCli) {
  const frontendDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
  const { reachableCount, unreachable } = findUnreachableCore(frontendDir)
  const { fresh, tolerated, stale } = classifyUnreachable(unreachable)
  if (fresh.length) {
    console.error(`[check:reachability] ${fresh.length} module(s) under src/core are not reachable from src/main.ts:`)
    for (const p of fresh) console.error(`  - ${p}`)
    console.error('Wire them into the app or delete them.')
  }
  if (stale.length) {
    console.error(`[check:reachability] stale KNOWN_UNREACHABLE entries (remove them): ${stale.join(', ')}`)
  }
  if (fresh.length || stale.length) process.exit(1)
  const debt = tolerated.length ? `; ${tolerated.length} known-debt file(s) tolerated` : ''
  console.log(`[check:reachability] ok: ${reachableCount} files reachable, no new dead modules under src/core${debt}`)
}
