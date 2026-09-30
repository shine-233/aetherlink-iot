/**
 * 文件用途: 设备配置远程选项组合函数（use-device-config-options）的单元测试。
 * 核心逻辑: 覆盖载荷归一化（list/records）、标签回退、选中项保留、防抖搜索与过期响应丢弃。
 * 关键注意事项: 防抖窗口 250ms 与选中项合并行为是筛选/弹窗共用选项的关键契约。
 * 重构建议: 若增加分页加载更多选项，补充对应分支测试。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getDeviceConfigList: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  getDeviceConfigList: hoisted.getDeviceConfigList
}))

import { useDeviceConfigOptions } from '../use-device-config-options'

describe('useDeviceConfigOptions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('fetches with the fixed page size and normalizes list payloads', async () => {
    hoisted.getDeviceConfigList.mockResolvedValue({
      data: { list: [{ id: 'cfg-1', name: 'Config A' }] },
      error: null
    })
    const { deviceConfigOptions, fetchDeviceConfigs } = useDeviceConfigOptions()

    await fetchDeviceConfigs()

    expect(hoisted.getDeviceConfigList).toHaveBeenCalledWith({ page: 1, page_size: 20 })
    expect(deviceConfigOptions.value).toEqual([{ label: 'Config A', value: 'cfg-1' }])
  })

  it('supports records payloads and label fallbacks (name > device_config_name > id)', async () => {
    hoisted.getDeviceConfigList.mockResolvedValue({
      data: {
        records: [{ id: 'cfg-2', device_config_name: 'Config B' }, { id: 'cfg-3' }]
      },
      error: null
    })
    const { deviceConfigOptions, fetchDeviceConfigs } = useDeviceConfigOptions()

    await fetchDeviceConfigs('b')

    expect(hoisted.getDeviceConfigList).toHaveBeenLastCalledWith({ page: 1, page_size: 20, name: 'b' })
    expect(deviceConfigOptions.value).toEqual([
      { label: 'Config B', value: 'cfg-2' },
      { label: 'cfg-3', value: 'cfg-3' }
    ])
  })

  it('ensureDeviceConfigOption prepends a missing option and ignores empties', () => {
    const { deviceConfigOptions, ensureDeviceConfigOption } = useDeviceConfigOptions()

    ensureDeviceConfigOption(null)
    ensureDeviceConfigOption({ label: 'Selected', value: 'cfg-0' })
    ensureDeviceConfigOption({ label: 'Selected Again', value: 'cfg-0' })

    expect(deviceConfigOptions.value).toEqual([{ label: 'Selected', value: 'cfg-0' }])
  })

  it('keeps the selected option visible across remote searches', async () => {
    hoisted.getDeviceConfigList.mockResolvedValue({
      data: { list: [{ id: 'cfg-1', name: 'Other' }] },
      error: null
    })
    const { deviceConfigOptions, ensureDeviceConfigOption, fetchDeviceConfigs } = useDeviceConfigOptions({
      getSelectedId: () => 'cfg-0'
    })
    ensureDeviceConfigOption({ label: 'Selected', value: 'cfg-0' })

    await fetchDeviceConfigs('other')

    expect(deviceConfigOptions.value).toEqual([
      { label: 'Selected', value: 'cfg-0' },
      { label: 'Other', value: 'cfg-1' }
    ])
  })

  it('debounces remote search to a single request', async () => {
    vi.useFakeTimers()
    hoisted.getDeviceConfigList.mockResolvedValue({ data: { list: [] }, error: null })
    const { handleDeviceConfigSearch } = useDeviceConfigOptions()

    handleDeviceConfigSearch('a')
    handleDeviceConfigSearch('ab')
    expect(hoisted.getDeviceConfigList).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(250)

    expect(hoisted.getDeviceConfigList).toHaveBeenCalledTimes(1)
    expect(hoisted.getDeviceConfigList).toHaveBeenCalledWith({ page: 1, page_size: 20, name: 'ab' })
  })

  it('discards a stale response that resolves after a newer search', async () => {
    let resolveStale!: (value: unknown) => void
    hoisted.getDeviceConfigList
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveStale = resolve
          })
      )
      .mockResolvedValueOnce({ data: { list: [{ id: 'cfg-2', name: 'Latest' }] }, error: null })
    const { deviceConfigOptions, fetchDeviceConfigs } = useDeviceConfigOptions()

    const stale = fetchDeviceConfigs('slow')
    await fetchDeviceConfigs('fast')
    expect(deviceConfigOptions.value).toEqual([{ label: 'Latest', value: 'cfg-2' }])

    resolveStale({ data: { list: [{ id: 'cfg-1', name: 'Stale' }] }, error: null })
    await stale
    expect(deviceConfigOptions.value).toEqual([{ label: 'Latest', value: 'cfg-2' }])
  })
})
