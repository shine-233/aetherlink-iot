/**
 * 文件用途: 覆盖系统日志纯查询层（深链解析、时间范围归一化、请求参数序列化）。
 */
import { describe, expect, it } from 'vitest'
import { defaultSystemLogQuery, normalizeLogRange, serializeSystemLogQuery, systemLogQueryFromRoute } from '../query'

describe('system-log query', () => {
  it('pushes a date-only end bound to end of day and keeps explicit times', () => {
    const start = new Date(2024, 0, 1, 0, 0, 0).valueOf()
    const midnight = new Date(2024, 0, 2, 0, 0, 0).valueOf()
    const explicit = new Date(2024, 0, 2, 13, 30, 0).valueOf()
    expect(normalizeLogRange([start, midnight])).toEqual([start, new Date(2024, 0, 2, 23, 59, 59, 999).valueOf()])
    expect(normalizeLogRange([start, explicit])).toEqual([start, explicit])
    expect(normalizeLogRange(null)).toBeNull()
  })

  it('serializes range into start_time/end_time and never leaks range', () => {
    const params = serializeSystemLogQuery({ ...defaultSystemLogQuery(), username: 'admin', range: null })
    expect(params).toEqual({
      username: 'admin',
      method: '',
      path: '',
      ip: '',
      action: '',
      entity_type: '',
      entity_id: '',
      start_time: '',
      end_time: ''
    })
  })

  it('parses deep links: whitelisted method, array values, both-or-neither time bounds', () => {
    const parsed = systemLogQueryFromRoute({ method: ['put'], path: '/api/v1/x', start_time: '2024-03-01T00:00:00Z' })
    expect(parsed.method).toBe('PUT')
    expect(parsed.path).toBe('/api/v1/x')
    // 只有起点没有终点：保留默认的最近一个月
    expect(parsed.range?.[1]).toBeGreaterThan(Date.parse('2024-03-02T00:00:00Z'))
    expect(systemLogQueryFromRoute({ method: 'GET' }).method).toBe('')
  })
})
