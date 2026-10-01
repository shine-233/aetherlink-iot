import { describe, expect, it } from 'vitest'
import {
  compareToneSeverity,
  createToneResolver,
  describeProgressState,
  readinessTone,
  resolveProgressState,
  resolveProgressTone,
  type StatusTone
} from './status-tone'

describe('status-tone', () => {
  it('maps progress states to tag tones with a default fallback', () => {
    expect(resolveProgressTone('done')).toBe('success')
    expect(resolveProgressTone('active')).toBe('warning')
    expect(resolveProgressTone('todo')).toBe('default')
    expect(resolveProgressTone('unknown')).toBe('default')
    expect(resolveProgressTone(undefined)).toBe('default')
  })

  it('derives progress state with done taking precedence over current', () => {
    expect(resolveProgressState(true, true)).toBe('done')
    expect(resolveProgressState(true, false)).toBe('done')
    expect(resolveProgressState(false, true)).toBe('active')
    expect(resolveProgressState(false, false)).toBe('todo')
  })

  it('describes a progress state with caller-supplied labels', () => {
    const labels = { done: '已通过', active: '当前卡点', todo: '待处理' }
    expect(describeProgressState('active', labels)).toEqual({ label: '当前卡点', tone: 'warning' })
    expect(describeProgressState('done', labels)).toEqual({ label: '已通过', tone: 'success' })
  })

  it('builds table-driven resolvers that never return undefined', () => {
    const jobTone = createToneResolver<string, StatusTone>(
      { complete: 'success', scheduled: 'info', running: 'info', timeout_risk: 'warning' },
      'error'
    )
    expect(jobTone('complete')).toBe('success')
    expect(jobTone('running')).toBe('info')
    expect(jobTone('timeout_risk')).toBe('warning')
    expect(jobTone('failed')).toBe('error')
    expect(jobTone(null)).toBe('error')
    // 原型链上的键不能被当成已知状态
    expect(jobTone('toString')).toBe('error')
  })

  it('orders tones by severity and maps readiness', () => {
    const tones = ['success', 'info', 'error', 'default', 'warning']
    expect([...tones].sort(compareToneSeverity)).toEqual(['error', 'warning', 'info', 'success', 'default'])
    expect(readinessTone(true)).toBe('success')
    expect(readinessTone(false)).toBe('warning')
  })
})
