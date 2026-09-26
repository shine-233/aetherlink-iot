/**
 * 文件用途: TP-22 大屏轮播纯逻辑的单元测试——URL 参数解析与轮播计时器。
 * 核心逻辑: parseCarouselQuery 的双形态 token/interval 归一与 fail-soft 语义；
 *   createCarouselTimer 用注入的假时钟逐 tick 验证切换、取模循环与停摆条件。
 * 关键注意事项: 计时器永不依赖真实 setTimeout——测试里 schedule/cancel 全部注入，
 *   否则"切换间隔漂移"这类缺陷会在 CI 上随机逃逸。
 * 重构建议: 若后续加入"每屏独立停留时长"，在 PlaylistQuery 上扩展并以同款注入方式补用例。
 */
import { describe, expect, it, vi } from 'vitest'

import {
  CAROUSEL_DEFAULT_INTERVAL_SECONDS,
  CAROUSEL_MAX_INTERVAL_SECONDS,
  CAROUSEL_MIN_INTERVAL_SECONDS,
  createCarouselTimer,
  normalizeCarouselIntervalSeconds,
  parseCarouselQuery
} from '../carousel'

describe('parseCarouselQuery', () => {
  it('parses a comma separated token list', () => {
    expect(parseCarouselQuery({ tokens: 'tok-a, tok-b,,tok-c', interval: '10' })).toEqual({
      tokens: ['tok-a', 'tok-b', 'tok-c'],
      intervalSeconds: 10
    })
  })

  it('parses repeated query params and dedupes keeping first occurrence order', () => {
    expect(parseCarouselQuery({ tokens: ['tok-b', 'tok-a', 'tok-b'] })).toEqual({
      tokens: ['tok-b', 'tok-a'],
      intervalSeconds: CAROUSEL_DEFAULT_INTERVAL_SECONDS
    })
  })

  it('returns null without tokens so the page falls back to single-board preview', () => {
    expect(parseCarouselQuery({})).toBeNull()
    expect(parseCarouselQuery({ tokens: '' })).toBeNull()
    expect(parseCarouselQuery({ tokens: ' , ' })).toBeNull()
    expect(parseCarouselQuery({ tokens: 42 })).toBeNull()
  })

  it('keeps a single-token playlist usable (one screen, no rotation)', () => {
    expect(parseCarouselQuery({ tokens: 'tok-a' })).toEqual({
      tokens: ['tok-a'],
      intervalSeconds: CAROUSEL_DEFAULT_INTERVAL_SECONDS
    })
  })
})

describe('normalizeCarouselIntervalSeconds', () => {
  it('defaults on missing, broken or non-positive values', () => {
    expect(normalizeCarouselIntervalSeconds(undefined)).toBe(CAROUSEL_DEFAULT_INTERVAL_SECONDS)
    expect(normalizeCarouselIntervalSeconds('abc')).toBe(CAROUSEL_DEFAULT_INTERVAL_SECONDS)
    expect(normalizeCarouselIntervalSeconds(0)).toBe(CAROUSEL_DEFAULT_INTERVAL_SECONDS)
    expect(normalizeCarouselIntervalSeconds(-5)).toBe(CAROUSEL_DEFAULT_INTERVAL_SECONDS)
    expect(normalizeCarouselIntervalSeconds(Number.NaN)).toBe(CAROUSEL_DEFAULT_INTERVAL_SECONDS)
  })

  it('clamps to the supported range and floors fractional seconds', () => {
    expect(normalizeCarouselIntervalSeconds(1)).toBe(CAROUSEL_MIN_INTERVAL_SECONDS)
    expect(normalizeCarouselIntervalSeconds(2.9)).toBe(CAROUSEL_MIN_INTERVAL_SECONDS)
    expect(normalizeCarouselIntervalSeconds(10.9)).toBe(10)
    expect(normalizeCarouselIntervalSeconds(99999)).toBe(CAROUSEL_MAX_INTERVAL_SECONDS)
  })
})

describe('createCarouselTimer', () => {
  /** Fake clock harness: advance(ms) runs every callback scheduled within that window. */
  function harness() {
    const callbacks: Array<{ at: number; run: () => void }> = []
    let now = 0
    const schedule = (run: () => void, ms: number) => {
      const entry = { at: now + ms, run }
      callbacks.push(entry)
      return entry
    }
    const cancel = (handle: unknown) => {
      const index = callbacks.indexOf(handle as { at: number; run: () => void })
      if (index >= 0) callbacks.splice(index, 1)
    }
    const advance = (ms: number) => {
      const deadline = now + ms
      // 逐条弹出到期回调（切换本身会再排下一个 tick，所以用循环而非快照）。
      for (;;) {
        callbacks.sort((a, b) => a.at - b.at)
        const next = callbacks[0]
        if (!next || next.at > deadline) break
        callbacks.shift()
        now = next.at
        next.run()
      }
      now = deadline
    }
    return { schedule, cancel, advance, scheduled: () => callbacks.length }
  }

  it('does not schedule a rotation for playlists with fewer than two screens', () => {
    const clock = harness()
    const onAdvance = vi.fn()
    const timer = createCarouselTimer({ intervalMs: 5000, count: () => 1, onAdvance, schedule: clock.schedule!, cancel: clock.cancel! })
    timer.start()
    expect(timer.running).toBe(false)
    expect(clock.scheduled()).toBe(0)
    timer.advanceNow()
    expect(onAdvance).not.toHaveBeenCalled()
    expect(timer.index).toBe(0)
    expect(clock.scheduled()).toBe(0)
  })

  it('rotates in order and wraps around modulo the playlist length', () => {
    const clock = harness()
    const onAdvance = vi.fn()
    const timer = createCarouselTimer({ intervalMs: 1000, count: () => 3, onAdvance, schedule: clock.schedule!, cancel: clock.cancel! })
    timer.start()
    expect(timer.running).toBe(true)
    expect(clock.scheduled()).toBe(1)

    clock.advance(1000)
    expect(onAdvance).toHaveBeenLastCalledWith(1)
    clock.advance(1000)
    expect(onAdvance).toHaveBeenLastCalledWith(2)
    clock.advance(1000)
    expect(onAdvance).toHaveBeenLastCalledWith(0)
    expect(onAdvance).toHaveBeenCalledTimes(3)
    expect(timer.index).toBe(0)
    expect(clock.scheduled()).toBe(1)
  })

  it('keeps the original cadence across switches', () => {
    const clock = harness()
    const seen: number[] = []
    const timer = createCarouselTimer({
      intervalMs: 15000,
      count: () => 2,
      onAdvance: (index) => seen.push(index),
      schedule: clock.schedule!,
      cancel: clock.cancel!
    })
    timer.start()
    clock.advance(15000)
    clock.advance(14000)
    expect(seen).toEqual([1])
    clock.advance(1000)
    expect(seen).toEqual([1, 0])
  })

  it('stop cancels the pending switch and is idempotent', () => {
    const clock = harness()
    const onAdvance = vi.fn()
    const timer = createCarouselTimer({ intervalMs: 1000, count: () => 2, onAdvance, schedule: clock.schedule!, cancel: clock.cancel! })
    timer.start()
    timer.stop()
    timer.stop()
    expect(clock.scheduled()).toBe(0)
    clock.advance(10000)
    expect(onAdvance).not.toHaveBeenCalled()
  })

  it('start is idempotent: it never stacks parallel timers', () => {
    const clock = harness()
    const onAdvance = vi.fn()
    const timer = createCarouselTimer({ intervalMs: 1000, count: () => 2, onAdvance, schedule: clock.schedule!, cancel: clock.cancel! })
    timer.start()
    timer.start()
    timer.start()
    expect(clock.scheduled()).toBe(1)
    clock.advance(1000)
    expect(onAdvance).toHaveBeenCalledTimes(1)
    expect(clock.scheduled()).toBe(1)
  })

  it('advanceNow switches immediately and re-arms the cadence', () => {
    const clock = harness()
    const onAdvance = vi.fn()
    const timer = createCarouselTimer({ intervalMs: 2000, count: () => 3, onAdvance, schedule: clock.schedule!, cancel: clock.cancel! })
    timer.start()
    timer.advanceNow()
    expect(onAdvance).toHaveBeenLastCalledWith(1)
    expect(clock.scheduled()).toBe(1)
    clock.advance(2000)
    expect(onAdvance).toHaveBeenLastCalledWith(2)
  })
})
