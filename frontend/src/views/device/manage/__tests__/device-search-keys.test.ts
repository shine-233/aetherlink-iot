// 文件用途: 守护 REQ-58 设备列表搜索增强——锁定 index.vue 的 searchConfigs 筛选键集。
// 核心逻辑: 读取 index.vue 源码,提取 searchConfigs 数组里所有 key:'...',与权威清单
//   DEVICE_SEARCH_KEYS 比对,任何键被误删/漏加(无对应契约)即 FAIL。
// 关键注意事项: 此前 17+1 个内联筛选键零断言(仅 fleet 预设子集被测),属假覆盖。searchConfigs
//   是组件内联 const 无法直接 import,故用源码解析锁定键集——增删键都会被此测试捕获。
// 重构建议: 若 searchConfigs 提取为可导入工厂,可改为直接断言其返回键集,免去源码解析。

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { DEVICE_SEARCH_KEYS } from '../device-search-keys'

const here = dirname(fileURLToPath(import.meta.url))
const indexVue = resolve(here, '../index.vue')

// 从 index.vue 源码里提取 searchConfigs 数组内的所有 key:'...' 值。
function extractSearchConfigKeys(): string[] {
  const src = readFileSync(indexVue, 'utf8')
  const start = src.indexOf('const searchConfigs')
  if (start < 0) throw new Error('searchConfigs 未在 index.vue 中找到')
  // 取 searchConfigs 声明之后、到 defineExpose/return 之前的一段,避免误吞其它对象的 key。
  const tail = src.slice(start)
  const keys: string[] = []
  const re = /key:\s*'([^']+)'/g
  let m: RegExpExecArray | null
  // 只扫描 searchConfigs 数组字面量范围。注意声明是 `ref<SearchConfig[]>([`,
  // 泛型里的 '[' 会先出现,故从 `(` 之后的第一个 '[' 起算,跳过泛型参数。
  const parenOpen = tail.indexOf('(')
  const arrOpen = tail.indexOf('[', parenOpen)
  let depth = 0
  let end = -1
  for (let i = arrOpen; i < tail.length; i++) {
    if (tail[i] === '[') depth++
    else if (tail[i] === ']') {
      depth--
      if (depth === 0) {
        end = i
        break
      }
    }
  }
  const arrLiteral = tail.slice(arrOpen, end < 0 ? undefined : end + 1)
  while ((m = re.exec(arrLiteral)) !== null) keys.push(m[1])
  return keys
}

describe('device-manage search keys contract (REQ-58)', () => {
  const actual = extractSearchConfigKeys()

  it('searchConfigs exposes exactly the authoritative key set (no missing/extra)', () => {
    expect([...actual].sort()).toEqual([...DEVICE_SEARCH_KEYS].sort())
  })

  it('includes the REQ-58 search-enhancement keys named by the customer', () => {
    // 主清单点名的增强项:设备编号/PID/固件/描述/标签/共享状态/上报前后界/自由文本。
    const enhancement = [
      'device_number',
      'pid_number',
      'firmware_version',
      'description',
      'label',
      'shared_status',
      'last_reported_after',
      'last_reported_before',
      'search'
    ]
    for (const k of enhancement) expect(actual).toContain(k)
  })

  it('includes the REQ-05b lifecycle_status filter added this session', () => {
    expect(actual).toContain('lifecycle_status')
  })

  it('has no duplicate keys', () => {
    expect(new Set(actual).size).toBe(actual.length)
  })
})
