/**
 * 文件用途：设备分组列表列配置的行为契约测试（ROADMAP TP-8②）。
 *
 * 覆盖三件事：
 *  1. 分组列里确实存在四个统计列（设备总数 / 在线 / 离线 / 告警）——
 *     后端加了 statistics 字段但前端不消费，等于能力没落地。
 *  2. statistics 缺失时显示 0，而不是留空。
 *     留空会被读成"平台没有这项能力"，0 才是事实（可能真的零台设备）。
 *  3. 有值时按真实数字渲染，且在线/告警用状态色标签。
 */
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import { group_columns } from '../all-columns'

type AnyColumn = {
  key?: string
  title?: unknown
  render?: (row: Record<string, unknown>) => unknown
}

function columns(): AnyColumn[] {
  return group_columns(() => {}, () => {}) as unknown as AnyColumn[]
}

function columnByKey(key: string): AnyColumn {
  const found = columns().find(c => c.key === key)
  if (!found) throw new Error(`column ${key} not found; keys=${columns().map(c => c.key).join(',')}`)
  return found
}

/**
 * 取渲染结果的可见文本。
 * Naive UI 列 render 可能返回：
 *  - 裸字符串/数字（普通列）
 *  - VNode，其 children 是字符串（简单元素）
 *  - VNode，其 children 是插槽对象（`<NTag>text</NTag>` 会被 Vue JSX 编成 default 插槽）
 */
function renderedText(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string' || typeof value === 'number') return String(value)
  if (Array.isArray(value)) return value.map(renderedText).join('')

  const vnode = value as { children?: unknown }
  const children = vnode.children
  if (children === null || children === undefined) return ''
  if (typeof children === 'string' || typeof children === 'number') return String(children)
  if (Array.isArray(children)) return children.map(renderedText).join('')

  if (typeof children === 'object') {
    const slots = children as Record<string, unknown>
    for (const key of ['default', 'text', '_']) {
      const slot = slots[key]
      if (typeof slot === 'function') return renderedText((slot as () => unknown)())
    }
  }
  return ''
}

const STAT_KEYS = ['stat_device_total', 'stat_online_total', 'stat_offline_total', 'stat_alarm_total']

describe('device group columns / statistics', () => {
  it('exposes the four statistics columns', () => {
    const keys = columns().map(c => c.key)
    for (const key of STAT_KEYS) {
      expect(keys, `missing statistics column ${key}`).toContain(key)
    }
  })

  it('keeps the pre-existing group columns intact', () => {
    const keys = columns().map(c => c.key)
    for (const key of ['name', 'description', 'created_at', 'actions']) {
      expect(keys, `regressed: ${key} disappeared`).toContain(key)
    }
  })

  it('renders zeros when statistics is absent', () => {
    const row = { id: 'g1', name: 'G', description: '', created_at: '2026-09-14T00:00:00Z' }
    for (const key of STAT_KEYS) {
      expect(renderedText(columnByKey(key).render?.(row)), `${key} must fall back to 0`).toBe('0')
    }
  })

  it('renders the real counts when statistics is present', () => {
    const row = {
      id: 'g1',
      name: 'G',
      description: '',
      created_at: '2026-09-14T00:00:00Z',
      statistics: { device_total: 7, online_total: 3, offline_total: 4, alarm_total: 2 }
    }
    expect(renderedText(columnByKey('stat_device_total').render?.(row))).toBe('7')
    expect(renderedText(columnByKey('stat_online_total').render?.(row))).toBe('3')
    expect(renderedText(columnByKey('stat_offline_total').render?.(row))).toBe('4')
    expect(renderedText(columnByKey('stat_alarm_total').render?.(row))).toBe('2')
  })

  it('treats partially missing statistics fields as zero', () => {
    // 后端补零值之前的老响应、或字段被裁剪时，不能渲染出 undefined。
    const row = {
      id: 'g1',
      name: 'G',
      description: '',
      created_at: '2026-09-14T00:00:00Z',
      statistics: { device_total: 5 }
    }
    expect(renderedText(columnByKey('stat_device_total').render?.(row))).toBe('5')
    expect(renderedText(columnByKey('stat_online_total').render?.(row))).toBe('0')
    expect(renderedText(columnByKey('stat_offline_total').render?.(row))).toBe('0')
    expect(renderedText(columnByKey('stat_alarm_total').render?.(row))).toBe('0')
  })
})
