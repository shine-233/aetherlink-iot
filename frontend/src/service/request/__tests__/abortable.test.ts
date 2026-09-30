/**
 * 文件用途：验证"只保留最后一次查询"运行器与取消语义识别。
 * 核心逻辑：后一次 run 会 abort 前一次的 signal；被取代/被取消的查询返回 null 而不抛错；
 *   只有真实错误才向外抛；isCanceledError 同时识别原始 AxiosError 与 flat 归一化形状。
 * 关键注意事项：runner 不持有定时器，测试无需 fake timers，但并发交错需手动控制 promise。
 * 重构建议：可补充"同一 runner 跨组件复用"的用例（当前契约不包含）。
 */
import { describe, expect, it } from 'vitest'
import { createLatestQueryRunner, isCanceledError } from '../abortable'

describe('isCanceledError', () => {
  it('识别 axios 原始取消形状', () => {
    expect(isCanceledError({ code: 'ERR_CANCELED', name: 'CanceledError' })).toBe(true)
  })

  it('识别 flatRequest 归一化后的取消形状', () => {
    expect(isCanceledError({ data: null, error: { code: 'ERR_CANCELED', message: 'canceled' } })).toBe(true)
  })

  it('识别 AbortError 名称', () => {
    expect(isCanceledError({ name: 'AbortError' })).toBe(true)
  })

  it('普通错误与非对象值不算取消', () => {
    expect(isCanceledError(new Error('boom'))).toBe(false)
    expect(isCanceledError({ code: 'ECONNABORTED' })).toBe(false)
    expect(isCanceledError(null)).toBe(false)
    expect(isCanceledError('canceled')).toBe(false)
  })
})

describe('createLatestQueryRunner', () => {
  it('无竞争时返回查询结果', async () => {
    const runner = createLatestQueryRunner<number>()
    const result = await runner.run(() => Promise.resolve(42))
    expect(result).toBe(42)
  })

  it('后一次 run 会 abort 前一次的 signal', async () => {
    const runner = createLatestQueryRunner<string>()
    const signals: AbortSignal[] = []

    let releaseFirst!: (value: string) => void
    const first = runner.run((signal) => {
      signals.push(signal)
      return new Promise<string>((resolve) => (releaseFirst = resolve))
    })
    const second = runner.run((signal) => {
      signals.push(signal)
      return Promise.resolve('second')
    })

    // 前一次的 signal 已被中止，后一次没有
    expect(signals[0].aborted).toBe(true)
    expect(signals[1].aborted).toBe(false)

    releaseFirst('stale')
    expect(await first).toBeNull()
    expect(await second).toBe('second')
  })

  it('被取消的查询返回 null 而不抛错', async () => {
    const runner = createLatestQueryRunner<string>()

    const canceled = runner.run(
      () =>
        new Promise<string>((_resolve, reject) => {
          reject({ code: 'ERR_CANCELED', name: 'CanceledError' })
        })
    )

    expect(await canceled).toBeNull()
  })

  it('被取代但底层不支持 signal 时，旧结果按过期丢弃', async () => {
    const runner = createLatestQueryRunner<number>()

    let releaseFirst!: (value: number) => void
    const first = runner.run(() => new Promise<number>((resolve) => (releaseFirst = resolve)))
    const second = runner.run(() => Promise.resolve(2))

    releaseFirst(1)
    expect(await first).toBeNull()
    expect(await second).toBe(2)
  })

  it('未被取代的真实错误原样外抛', async () => {
    const runner = createLatestQueryRunner<number>()
    const boom = new Error('network down')

    await expect(runner.run(() => Promise.reject(boom))).rejects.toBe(boom)
  })

  it('cancel() 中止当前查询并把 isActive 置回 false', async () => {
    const runner = createLatestQueryRunner<number>()
    expect(runner.isActive()).toBe(false)

    let capturedSignal: AbortSignal | null = null
    const pending = runner.run((signal) => {
      capturedSignal = signal
      return new Promise<number>(() => {})
    })

    expect(runner.isActive()).toBe(true)
    runner.cancel()
    expect(capturedSignal?.aborted).toBe(true)
    expect(runner.isActive()).toBe(false)

    // 取消后 promise 永不 settle 也不会泄漏计时器；被取代判定覆盖它
    const next = runner.run(() => Promise.resolve(3))
    expect(await next).toBe(3)
    void pending
  })
})
