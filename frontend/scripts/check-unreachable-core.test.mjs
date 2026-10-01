// 文件用途: 守护 check-unreachable-core 的解析器——别名、相对路径、动态 import()、import.meta.glob、
//   type-only 导入与已知债务分类，确保守卫既不漏报死代码，也不把可达模块误判为死代码。
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { classifyUnreachable, findUnreachableCore, resolveSpecifier, walkReachable } from './check-unreachable-core.mjs'

let workspace
let frontend
let dirs

function write(rel, text = 'export {}\n') {
  const p = path.join(frontend, rel)
  fs.mkdirSync(path.dirname(p), { recursive: true })
  fs.writeFileSync(p, text)
  return p
}

beforeAll(() => {
  workspace = fs.mkdtempSync(path.join(os.tmpdir(), 'reach-guard-'))
  frontend = path.join(workspace, 'frontend')
  dirs = { srcDir: path.join(frontend, 'src'), workspaceDir: workspace }
  write(
    'src/main.ts',
    [
      "import { a } from '@/core/alias/a'",
      "import './side-effect'",
      "export * from './reexport'",
      "import type { T } from '@/core/typed'",
      "const lazy = () => import('@/core/lazy/page.vue')",
      "const mods = import.meta.glob(['@/core/globbed/*.ts', '!@/core/globbed/skip.ts'])",
      "const shared = import.meta.glob('~/packages/shared/*.ts')"
    ].join('\n')
  )
  write('src/side-effect.ts', "import { b } from './core/relative/b'\n")
  write('src/reexport.ts')
  write('src/core/alias/a/index.ts')
  write('src/core/relative/b.ts')
  write('src/core/typed.ts')
  write('src/core/lazy/page.vue', '<script setup lang="ts">\nimport \'../lazy/child\'\n</script>\n')
  write('src/core/lazy/child.ts')
  write('src/core/globbed/one.ts')
  write('src/core/globbed/one.test.ts')
  write('src/core/dead/orphan.ts', "import '../relative/b'\n")
  write('src/core/dead/orphan.test.ts', "import './orphan'\n")
  write('src/core/types.d.ts')
  fs.mkdirSync(path.join(workspace, 'packages/shared'), { recursive: true })
  fs.writeFileSync(path.join(workspace, 'packages/shared/util.ts'), 'export {}\n')
})

afterAll(() => {
  fs.rmSync(workspace, { recursive: true, force: true })
})

describe('resolveSpecifier', () => {
  it('resolves @/ alias to index files, relative specs with extensions, and ignores bare packages', () => {
    const main = path.join(dirs.srcDir, 'main.ts')
    expect(resolveSpecifier(main, '@/core/alias/a', dirs)).toBe(path.join(dirs.srcDir, 'core/alias/a/index.ts'))
    expect(resolveSpecifier(main, './core/relative/b', dirs)).toBe(path.join(dirs.srcDir, 'core/relative/b.ts'))
    expect(resolveSpecifier(main, '@/core/lazy/page.vue?raw', dirs)).toBe(path.join(dirs.srcDir, 'core/lazy/page.vue'))
    expect(resolveSpecifier(main, 'vue', dirs)).toBeNull()
    expect(resolveSpecifier(main, './missing', dirs)).toBeNull()
  })
})

describe('walkReachable', () => {
  it('follows static, side-effect, re-export, type-only, dynamic and glob imports', () => {
    const rel = [...walkReachable(path.join(dirs.srcDir, 'main.ts'), dirs)].map((p) =>
      path.relative(workspace, p).split(path.sep).join('/')
    )
    expect(rel).toEqual(
      expect.arrayContaining([
        'frontend/src/core/alias/a/index.ts',
        'frontend/src/core/relative/b.ts',
        'frontend/src/reexport.ts',
        'frontend/src/core/typed.ts',
        'frontend/src/core/lazy/page.vue',
        'frontend/src/core/lazy/child.ts',
        'frontend/src/core/globbed/one.ts',
        'packages/shared/util.ts'
      ])
    )
    // glob 展开排除测试文件；死代码即使被自己的测试导入也不可达。
    expect(rel).not.toContain('frontend/src/core/globbed/one.test.ts')
    expect(rel).not.toContain('frontend/src/core/dead/orphan.ts')
  })
})

describe('findUnreachableCore', () => {
  it('reports only non-test, non-declaration modules under src/core that main.ts cannot reach', () => {
    expect(findUnreachableCore(frontend).unreachable).toEqual(['src/core/dead/orphan.ts'])
  })
})

describe('classifyUnreachable', () => {
  it('splits fresh dead code from tolerated debt and flags stale debt entries', () => {
    const result = classifyUnreachable(
      ['src/core/dead/orphan.ts', 'src/core/legacy/x.ts'],
      ['src/core/legacy/', 'src/core/gone/']
    )
    expect(result).toEqual({
      fresh: ['src/core/dead/orphan.ts'],
      tolerated: ['src/core/legacy/x.ts'],
      stale: ['src/core/gone/']
    })
  })

  it('passes against the real repository (no new dead modules, no stale debt entries)', () => {
    const real = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
    const { fresh, stale } = classifyUnreachable(findUnreachableCore(real).unreachable)
    expect({ fresh, stale }).toEqual({ fresh: [], stale: [] })
  })
})
