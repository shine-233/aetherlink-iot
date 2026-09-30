/**
 * 文件用途：验证设备状态 store 的"条件变更取消未归请求"行为。
 * 核心逻辑：快速切换设备 id 时，旧详情请求被 abort 且过期结果不覆盖新数据；
 *   正常路径维持原契约（成功写入 deviceData、失败清空）。
 * 关键注意事项：deviceDetail 的 signal 透传由 src/service/api/device.ts 提供；
 *   本文件 mock 掉 service 层，专注 store 与 runner 的编排语义。
 * 重构建议：若更多 store 出现同类竞态，可把 runner 接法抽成 useLatestFetch 组合式工具。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const hoisted = vi.hoisted(() => ({
  deviceDetail: vi.fn()
}))

vi.mock('@/service/api', () => ({
  deviceDetail: hoisted.deviceDetail
}))

import { useDeviceDataStore } from '../modules/device'

const flush = async () => {
  for (let i = 0; i < 6; i += 1) {
    await Promise.resolve()
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}

describe('device store fetchData 竞态治理', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('快速切换设备：旧请求被 abort，旧结果不覆盖新数据', async () => {
    const store = useDeviceDataStore()
    const calls: Array<{ id: string; signal?: AbortSignal; deferred: ReturnType<typeof deferred> }> = []

    hoisted.deviceDetail.mockImplementation((id: string, config?: { signal?: AbortSignal }) => {
      const entry = { id, signal: config?.signal, deferred: deferred() }
      calls.push(entry)
      return entry.deferred.promise
    })

    const first = store.fetchData('device-a')
    const second = store.fetchData('device-b')

    expect(calls).toHaveLength(2)
    // 后一次发起时，前一次的 signal 已被中止（网络层真正取消）
    expect(calls[0].signal?.aborted).toBe(true)
    expect(calls[1].signal?.aborted).toBe(false)

    calls[1].deferred.resolve({ data: { id: 'device-b', name: 'B' }, error: null })
    calls[0].deferred.resolve({ data: { id: 'device-a', name: 'A' }, error: null })
    await Promise.all([first, second])
    await flush()

    expect(store.deviceData).toEqual({ id: 'device-b', name: 'B' })
  })

  it('单个请求正常成功时写入 deviceData', async () => {
    const store = useDeviceDataStore()
    hoisted.deviceDetail.mockResolvedValue({ data: { id: 'device-a' }, error: null })

    await store.fetchData('device-a')

    expect(store.deviceData).toEqual({ id: 'device-a' })
  })

  it('请求失败时清空 deviceData，而非抛错', async () => {
    const store = useDeviceDataStore()
    hoisted.deviceDetail.mockResolvedValue({ data: null, error: { message: 'boom', code: 'E1' } })

    await store.fetchData('device-a')

    expect(store.deviceData).toEqual({})
  })
})
