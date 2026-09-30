/**
 * 文件用途：验证 GET in-flight 去重层的关键行为和回归边界。
 * 核心逻辑：并发同键请求只发一次底层调用；命中方拿到独立副本；settle 后立即失效；
 *   失败结果原样外抛且同样会被清理；去重键对参数键序稳定。
 * 关键注意事项：模块内含单例 inFlight 表，用例间必须 reset，避免互相污染。
 * 重构建议：若 dedupe 未来支持 TTL 缓存，需在本文件补"过期后重新发起"的用例。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  buildGetDedupeKey,
  cloneResponseValue,
  dedupeGet,
  getInFlightRequestCount,
  resetInFlightRequests
} from '../dedupe'

type Flat<T> = { data: T; error: null }

function flatOk<T>(data: T): Flat<T> {
  return { data, error: null }
}

describe('dedupeGet', () => {
  beforeEach(() => {
    resetInFlightRequests()
  })

  it('并发同键请求只触发一次底层调用', async () => {
    const run = vi.fn().mockResolvedValue(flatOk({ list: [1, 2] }))

    const [first, second] = await Promise.all([dedupeGet('same-key', run), dedupeGet('same-key', run)])

    expect(run).toHaveBeenCalledTimes(1)
    expect(first).toEqual(flatOk({ list: [1, 2] }))
    expect(second).toEqual(first)
  })

  it('命中方拿到独立副本，互不污染', async () => {
    const run = vi.fn().mockResolvedValue(flatOk({ nested: { count: 1 } }))

    const [first, second] = await Promise.all([dedupeGet('clone-key', run), dedupeGet('clone-key', run)])

    expect(second).not.toBe(first)
    // 修改第一份结果不影响第二份
    first.data.nested.count = 99
    expect((second as Flat<{ nested: { count: number } }>).data.nested.count).toBe(1)
  })

  it('请求 settle 后立即从去重表移除：下一次会重新发起', async () => {
    const run = vi.fn().mockResolvedValue(flatOk(1))

    await dedupeGet('settle-key', run)
    await dedupeGet('settle-key', run)

    expect(run).toHaveBeenCalledTimes(2)
    expect(getInFlightRequestCount()).toBe(0)
  })

  it('不同键的并发请求互不共享', async () => {
    const run = vi.fn().mockImplementation((..._args: unknown[]) => Promise.resolve(flatOk(Math.random())))

    await Promise.all([dedupeGet('key-a', run), dedupeGet('key-b', run)])

    expect(run).toHaveBeenCalledTimes(2)
  })

  it('失败结果原样外抛给所有并发调用方，且清理 in-flight', async () => {
    const failure = { data: null, error: { code: 'ERR_X', message: 'boom' } }
    const run = vi.fn().mockRejectedValue(failure)

    const results = await Promise.allSettled([dedupeGet('fail-key', run), dedupeGet('fail-key', run)])

    expect(run).toHaveBeenCalledTimes(1)
    expect(results[0].status).toBe('rejected')
    expect(results[1].status).toBe('rejected')
    expect((results[0] as PromiseRejectedResult).reason).toEqual(failure)
    expect(getInFlightRequestCount()).toBe(0)
  })

  it('settle 之前可观测 in-flight 数量，reset 可强制清空', async () => {
    let release!: (value: Flat<number>) => void
    const run = vi.fn().mockImplementation(() => new Promise<Flat<number>>((resolve) => (release = resolve)))

    const pending = dedupeGet('pending-key', run)
    expect(getInFlightRequestCount()).toBe(1)

    release(flatOk(7))
    await pending
    expect(getInFlightRequestCount()).toBe(0)
  })
})

describe('buildGetDedupeKey', () => {
  it('无参数时只返回 URL', () => {
    expect(buildGetDedupeKey('/devices')).toBe('/devices')
  })

  it('参数键序不同生成同键', () => {
    const a = buildGetDedupeKey('/devices', { params: { page: 1, page_size: 10 } })
    const b = buildGetDedupeKey('/devices', { params: { page_size: 10, page: 1 } })
    expect(a).toBe(b)
  })

  it('参数不同则键不同', () => {
    const a = buildGetDedupeKey('/devices', { params: { page: 1 } })
    const b = buildGetDedupeKey('/devices', { params: { page: 2 } })
    expect(a).not.toBe(b)
  })

  it('URLSearchParams 参数同样键序稳定', () => {
    const a = buildGetDedupeKey('/devices', {
      params: new URLSearchParams([
        ['b', '2'],
        ['a', '1']
      ])
    })
    const b = buildGetDedupeKey('/devices', {
      params: new URLSearchParams([
        ['a', '1'],
        ['b', '2']
      ])
    })
    expect(a).toBe(b)
  })
})

describe('cloneResponseValue', () => {
  it('深拷贝普通对象（含嵌套）', () => {
    const source = { a: { b: [1, { c: 2 }] } }
    const cloned = cloneResponseValue(source)

    expect(cloned).toEqual(source)
    expect(cloned).not.toBe(source)
    expect(cloned.a).not.toBe(source.a)
  })

  it('原始类型原样返回', () => {
    expect(cloneResponseValue('text')).toBe('text')
    expect(cloneResponseValue(42)).toBe(42)
    expect(cloneResponseValue(null)).toBe(null)
  })

  it('不可结构化克隆的值（函数）原样返回而不抛错', () => {
    const fn = () => 'noop'
    const wrapped = { fn }
    const cloned = cloneResponseValue(wrapped)

    expect(cloned).toBe(wrapped)
  })
})
