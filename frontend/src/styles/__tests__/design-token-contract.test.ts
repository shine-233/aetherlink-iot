/**
 * 文件用途: 设计系统收敛绊线契约——锁定 views/components 样式块内的硬编码 hex 基线。
 * 核心逻辑: 扫描 <style> 块中的 #hex 字面量并与基线比较；只允许减少，不允许净新增。
 * 关键注意事项: 迁移存量时请同步下调 BASELINE（写明迁移 lane），让数字单调下降。
 */
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// 基线轨迹：审计日 1042 → 首轮迁移(linkage-edit 等) 994 → 第二轮批量迁移
// （DeviceAccessGuide/CommandCenter 四件套/home 两件套/fleet 等）733。
// 后续 hex→token 迁移 lane 每迁完一批就同步下调此数字，只降不升。
const HEX_BASELINE = 733

function collectVueFiles(dir: string): string[] {
  const out: string[] = []
  for (const name of readdirSync(dir)) {
    const full = join(dir, name)
    if (statSync(full).isDirectory()) {
      out.push(...collectVueFiles(full))
    } else if (name.endsWith('.vue')) {
      out.push(full)
    }
  }
  return out
}

function countStyleBlockHex(file: string): number {
  const content = readFileSync(file, 'utf8')
  const styleBlocks = content.match(/<style[^>]*>([\s\S]*?)<\/style>/g) ?? []
  let count = 0
  for (const block of styleBlocks) {
    count += (block.match(/#[0-9a-fA-F]{3,8}\b/g) ?? []).length
  }
  return count
}

describe('design token contract', () => {
  it('style-block hardcoded hex colors must not exceed the audited baseline', () => {
    const roots = ['src/views', 'src/components']
    let total = 0
    for (const root of roots) {
      for (const file of collectVueFiles(root)) {
        total += countStyleBlockHex(file)
      }
    }
    expect(total).toBeLessThanOrEqual(HEX_BASELINE)
  })

  it('breakpoint css variables stay single-sourced (no --bp-* duplicates)', () => {
    const globalCss = readFileSync('src/styles/css/global.css', 'utf8')
    expect(globalCss.includes('--bp-sm')).toBe(false)
    expect(globalCss.includes('--bp-md')).toBe(false)
    expect(globalCss.includes('--bp-lg')).toBe(false)
  })
})
