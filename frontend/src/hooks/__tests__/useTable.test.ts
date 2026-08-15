/*
 * 文件用途：验证通用表格 Hook 的初始化、分页、请求和数据转换行为。
 * 核心逻辑：通过 mock API、store 和响应式引用覆盖 getData、searchParams、pagination 等公开状态。
 * 关键注意事项：测试应保持调用方视角，避免只锁定内部临时变量。
 * 重构建议：后续可把复杂分页场景拆成更小的用例，提升失败定位速度。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'

const hoisted = vi.hoisted(() => ({
  mockApiFn: vi.fn(),
  mockUseLoading: vi.fn(),
  mockUseBoolean: vi.fn(),
  mockAppStoreLocale: 'zh-CN'
}))

vi.mock('@aetherlink/hooks', () => ({
  useLoading: hoisted.mockUseLoading,
  useBoolean: hoisted.mockUseBoolean
}))

vi.mock('@/store/modules/app', () => ({
  useAppStore: () => ({
    locale: ref(hoisted.mockAppStoreLocale),
    reloadPage: vi.fn()
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import { useTable } from '../common/table'

describe('useTable hook', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    const loadingRef = ref(false)
    hoisted.mockUseLoading.mockReturnValue({
      loading: loadingRef,
      startLoading: vi.fn(() => { loadingRef.value = true }),
      endLoading: vi.fn(() => { loadingRef.value = false })
    })

    const emptyRef = ref(false)
    hoisted.mockUseBoolean.mockReturnValue({
      bool: emptyRef,
      setBool: vi.fn((val: boolean) => { emptyRef.value = val }),
      setTrue: vi.fn(() => { emptyRef.value = true }),
      setFalse: vi.fn(() => { emptyRef.value = false })
    })
  })

  const createConfig = (overrides: Record<string, any> = {}) => ({
    apiFn: hoisted.mockApiFn,
    apiParams: { page: 1, page_size: 10 },
    transformer: (response: any) => ({
      data: response?.data?.list || [],
      pageNum: response?.data?.page || 1,
      pageSize: response?.data?.page_size || 10,
      total: response?.data?.total || 0
    }),
    columns: () => [
      { key: 'name', title: 'Name' },
      { key: 'age', title: 'Age' }
    ],
    immediate: false,
    ...overrides
  })

  describe('initial state', () => {
    it('returns loading, empty, data, columns, pagination, searchParams', () => {
      hoisted.mockApiFn.mockResolvedValue({ data: { list: [], total: 0, page: 1, page_size: 10 } })
      const result = useTable(createConfig())

      expect(result.loading.value).toBe(false)
      expect(result.empty.value).toBe(false)
      expect(result.data.value).toEqual([])
      expect(result.columns.value.map((column: any) => column.key)).toEqual(['name', 'age'])
      expect(result.filteredColumns.value.map((column: any) => column.key)).toEqual(['name', 'age'])
      expect(result.pagination).toMatchObject({ page: 1, pageSize: 10 })
      expect(result.pagination.itemCount).toBeUndefined()
      expect(result.searchParams).toMatchObject({ page: 1, page_size: 10 })
      expect(typeof result.reloadColumns).toBe('function')
      expect(typeof result.getData).toBe('function')
      expect(typeof result.updateSearchParams).toBe('function')
      expect(typeof result.resetSearchParams).toBe('function')
      expect(typeof result.updatePagination).toBe('function')
    })

    it('initializes data as empty array', () => {
      hoisted.mockApiFn.mockResolvedValue({ data: { list: [], total: 0 } })
      const result = useTable(createConfig())
      expect(result.data.value).toEqual([])
    })

    it('initializes searchParams from apiParams', () => {
      hoisted.mockApiFn.mockResolvedValue({ data: { list: [], total: 0 } })
      const result = useTable(createConfig({ apiParams: { page: 2, page_size: 20 } }))
      expect(result.searchParams.page).toBe(2)
      expect(result.searchParams.page_size).toBe(20)
    })

    it('initializes pagination with default values', () => {
      hoisted.mockApiFn.mockResolvedValue({ data: { list: [], total: 0 } })
      const result = useTable(createConfig())
      expect(result.pagination.page).toBe(1)
      expect(result.pagination.pageSize).toBe(10)
    })
  })

  describe('getData', () => {
    it('calls apiFn with searchParams and transforms response', async () => {
      hoisted.mockApiFn.mockResolvedValue({
        data: {
          list: [{ name: 'Item 1', age: 20 }],
          total: 1,
          page: 1,
          page_size: 10
        },
        error: null
      })

      const result = useTable(createConfig())
      await result.getData()
      await nextTick()

      expect(hoisted.mockApiFn).toHaveBeenCalledWith(result.searchParams)
      expect(result.data.value).toHaveLength(1)
      expect(result.data.value[0].name).toBe('Item 1')
    })

    it('sets empty to true when data is empty', async () => {
      hoisted.mockApiFn.mockResolvedValue({
        data: { list: [], total: 0, page: 1, page_size: 10 },
        error: null
      })

      const result = useTable(createConfig())
      await result.getData()
      await nextTick()

      expect(result.empty.value).toBe(true)
    })

    it('updates pagination after fetch', async () => {
      hoisted.mockApiFn.mockResolvedValue({
        data: { list: [{ name: 'Item 1' }], total: 50, page: 2, page_size: 10 },
        error: null
      })

      const result = useTable(createConfig())
      await result.getData()
      await nextTick()

      expect(result.pagination.itemCount).toBe(50)
    })
  })

  describe('updateSearchParams', () => {
    it('merges new params into searchParams', () => {
      hoisted.mockApiFn.mockResolvedValue({ data: { list: [], total: 0 } })
      const result = useTable(createConfig({ apiParams: { page: 1, name: '' } }))

      result.updateSearchParams({ name: 'test' })
      expect(result.searchParams.name).toBe('test')
    })
  })

  describe('resetSearchParams', () => {
    it('resets searchParams to initial apiParams', () => {
      hoisted.mockApiFn.mockResolvedValue({ data: { list: [], total: 0 } })
      const result = useTable(createConfig({ apiParams: { page: 1, name: '' } }))

      result.updateSearchParams({ name: 'modified' })
      result.resetSearchParams()
      expect(result.searchParams.name).toBe('')
    })
  })

  describe('updatePagination', () => {
    it('merges new values into pagination', () => {
      hoisted.mockApiFn.mockResolvedValue({ data: { list: [], total: 0 } })
      const result = useTable(createConfig())

      result.updatePagination({ page: 3, itemCount: 100 })
      expect(result.pagination.page).toBe(3)
      expect(result.pagination.itemCount).toBe(100)
    })
  })

  describe('columns', () => {
    it('generates columns from factory function', () => {
      hoisted.mockApiFn.mockResolvedValue({ data: { list: [], total: 0 } })
      const result = useTable(createConfig())
      expect(result.columns.value).toHaveLength(2)
    })

    it('generates filteredColumns from factory function', () => {
      hoisted.mockApiFn.mockResolvedValue({ data: { list: [], total: 0 } })
      const result = useTable(createConfig())
      expect(result.filteredColumns.value).toHaveLength(2)
      expect(result.filteredColumns.value[0].key).toBe('name')
      expect(result.filteredColumns.value[0].checked).toBe(true)
    })
  })

  describe('immediate option', () => {
    it('does not call apiFn when immediate is false', () => {
      hoisted.mockApiFn.mockResolvedValue({ data: { list: [], total: 0 } })
      useTable(createConfig({ immediate: false }))
      expect(hoisted.mockApiFn).toHaveBeenCalledTimes(0)
    })

    it('calls apiFn when immediate is true', () => {
      hoisted.mockApiFn.mockResolvedValue({ data: { list: [], total: 0 } })
      useTable(createConfig({ immediate: true }))
      expect(hoisted.mockApiFn).toHaveBeenCalledTimes(1)
    })
  })
})
