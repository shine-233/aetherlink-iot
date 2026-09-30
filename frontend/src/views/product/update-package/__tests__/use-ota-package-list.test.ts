/**
 * 文件用途: 升级包列表状态机（use-ota-package-list，基于共享 useListPage）的单元测试。
 * 核心逻辑: 验证请求载荷契约、list/data.list/records 载荷归一化、分页联动、重置与失败保留旧行。
 * 关键注意事项: 载荷断言（device_config_id 空值传 ''）是迁移前后的兼容契约，不可放宽。
 * 重构建议: 若增加路由同步或序列化逻辑，在此补充对应分支。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getOtaPackageList: vi.fn()
}))

vi.mock('@/service/product/update-package', () => ({
  getOtaPackageList: hoisted.getOtaPackageList
}))

import { useOtaPackageList } from '../use-ota-package-list'

describe('useOtaPackageList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getOtaPackageList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
  })

  it('keeps the legacy request payload contract on first load', async () => {
    const list = useOtaPackageList()
    await list.load()
    expect(hoisted.getOtaPackageList).toHaveBeenCalledTimes(1)
    expect(hoisted.getOtaPackageList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      name: '',
      version: '',
      device_config_id: ''
    })
  })

  it('normalizes nested list payloads and records payloads', async () => {
    const list = useOtaPackageList()

    hoisted.getOtaPackageList.mockResolvedValueOnce({
      data: { data: { list: [{ id: 'pkg-1', name: 'Pkg 1' }], total: 7 } },
      error: null
    })
    await list.load()
    expect(list.rows.value).toEqual([{ id: 'pkg-1', name: 'Pkg 1' }])
    expect(list.total.value).toBe(7)

    hoisted.getOtaPackageList.mockResolvedValueOnce({
      data: { records: [{ id: 'pkg-2', name: 'Pkg 2' }], total: 9 },
      error: null
    })
    await list.load()
    expect(list.rows.value).toEqual([{ id: 'pkg-2', name: 'Pkg 2' }])
    expect(list.total.value).toBe(9)
  })

  it('sends trimmed-less filters verbatim and maps null device_config_id to empty string', async () => {
    const list = useOtaPackageList()
    list.query.name = 'firmware'
    list.query.version = '1.0'
    list.query.device_config_id = 'cfg-1'
    await list.search()
    expect(hoisted.getOtaPackageList).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 10,
      name: 'firmware',
      version: '1.0',
      device_config_id: 'cfg-1'
    })

    list.query.device_config_id = null
    await list.load()
    expect(hoisted.getOtaPackageList).toHaveBeenLastCalledWith(expect.objectContaining({ device_config_id: '' }))
  })

  it('page change keeps page_size, size change resets page (useListPage pagination)', async () => {
    const list = useOtaPackageList()
    await list.setPage(3)
    expect(hoisted.getOtaPackageList).toHaveBeenLastCalledWith(expect.objectContaining({ page: 3, page_size: 10 }))
    await list.setPageSize(20)
    expect(hoisted.getOtaPackageList).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, page_size: 20 }))
    expect(list.pagination.pageSizes).toEqual([10, 20, 50])
  })

  it('reset restores default filters and jumps back to page 1', async () => {
    const list = useOtaPackageList()
    list.query.name = 'x'
    list.query.device_config_id = 'cfg-9'
    await list.setPage(2)
    await list.reset()
    expect(list.query.name).toBe('')
    expect(list.query.device_config_id).toBeNull()
    expect(list.page.value).toBe(1)
    expect(hoisted.getOtaPackageList).toHaveBeenLastCalledWith(
      expect.objectContaining({ page: 1, name: '', device_config_id: '' })
    )
  })

  it('keeps current rows when the fetch fails', async () => {
    hoisted.getOtaPackageList
      .mockResolvedValueOnce({ data: { list: [{ id: 'a' }], total: 1 }, error: null })
      .mockResolvedValueOnce({ data: null, error: 'boom' })
    const list = useOtaPackageList()
    await list.load()
    await list.load()
    expect(list.rows.value).toEqual([{ id: 'a' }])
    expect(list.total.value).toBe(1)
    expect(list.loading.value).toBe(false)
  })
})
