/**
 * 文件用途：覆盖 index 在 权限元素管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchElementList: vi.fn(),
  delElement: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api/route', () => ({
  fetchElementList: hoisted.fetchElementList,
  delElement: hoisted.delElement,
  addElement: vi.fn(),
  editElement: vi.fn()
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@aetherlink/hooks', () => ({
  useBoolean: (init = false) => {
    const bool = ref(init)
    return {
      bool,
      setTrue: vi.fn(() => {
        bool.value = true
      }),
      setFalse: vi.fn(() => {
        bool.value = false
      })
    }
  },
  useLoading: (init = false) => {
    const loading = ref(init)
    return {
      loading,
      startLoading: vi.fn(() => {
        loading.value = true
      }),
      endLoading: vi.fn(() => {
        loading.value = false
      })
    }
  }
}))

vi.mock('@/constants/business', () => ({
  routerSysFlagLabels: { '1': 'Admin', '2': 'User' },
  routerTypeLabels: { '1': 'Menu', '3': 'Button' },
  routeSysFlagOptions: [{ label: 'Admin', value: '1' }, { label: 'User', value: '2' }],
  routeTypeOptions: [{ label: 'Menu', value: '1' }, { label: 'Button', value: '3' }]
}))

vi.mock('@/utils/common/tool', () => ({
  deepClone: (obj: unknown) => JSON.parse(JSON.stringify(obj))
}))

vi.mock('../components/table-action-modal.vue', () => ({
  default: defineComponent({
    name: 'TableActionModalStub',
    setup() {
      return () => h('div', { class: 'table-action-modal-stub' })
    }
  })
}))

import AuthIndex from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(AuthIndex, {
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NDataTable: defineComponent({ name: 'NDataTable', props: { data: { type: Array, default: () => [] }, loading: Boolean, pagination: { default: null } }, setup() { return () => h('div') } }),
        NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } }),
        NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        IconIcRoundPlus: true,
        SvgIcon: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

// setupState 的受控视图：只声明测试实际触达的成员，其余成员走 unknown 兜底。
interface AuthManageSetupColumn {
  key: string
}

interface AuthManageSetupState {
  columns: AuthManageSetupColumn[]
  rowKey: (row: Record<string, unknown>) => unknown
  [key: string]: unknown
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as AuthManageSetupState

const mockRoute = (overrides: Record<string, unknown> = {}) => ({
  id: 'm-1',
  description: 'User Management',
  element_code: 'user',
  param1: 'user',
  param2: 'mdi-user',
  element_type: '1',
  authority: ['1'],
  remark: 'remark',
  multilingual: 'default',
  ...overrides
})

describe('management/auth/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchElementList.mockResolvedValue({
      data: { list: [mockRoute()], total: 1 }
    })
    hoisted.delElement.mockResolvedValue({ error: null })
    ;(globalThis as unknown as { $message: Record<string, (...args: unknown[]) => void> }).$message = { success: hoisted.messageSuccess }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds fetched routes to the data table on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const table = wrapper.getComponent({ name: 'NDataTable' })

    expect(hoisted.fetchElementList).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 1,
        page_size: 10
      })
    )
    expect(table.props('data')).toEqual([mockRoute()])
    expect(table.props('loading')).toBe(false)
    expect(table.props('pagination')).toMatchObject({
      page: 1,
      pageSize: 10,
      itemCount: 1
    })
  })

  it('calls getTableData on init', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.fetchElementList).toHaveBeenCalledTimes(1)
  })

  it('populates tableData and pagination on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.tableData).toHaveLength(1)
    expect(state.pagination.itemCount).toBe(1)
    expect(state.loading).toBe(false)
  })

  it('handleAddTable opens modal with add type', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleAddTable()
    expect(state.modalType).toBe('add')
    expect(state.visible).toBe(true)
  })

  it('handleEditTable sets modal type to edit and opens modal with deep cloned edit data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const row = mockRoute()
    state.handleEditTable(row)
    expect(state.modalType).toBe('edit')
    expect(state.visible).toBe(true)
    expect(state.editData).toEqual(expect.objectContaining({ id: 'm-1' }))
    expect(state.editData).not.toBe(row)
  })

  it('handleDeleteTable calls delElement and refreshes data on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.delElement.mockResolvedValue({ error: null })
    hoisted.fetchElementList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    await state.handleDeleteTable('m-1')
    await flushPromises()
    expect(hoisted.delElement).toHaveBeenCalledWith('m-1')
    expect(hoisted.messageSuccess).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('common.deleteSuccess')
    expect(hoisted.fetchElementList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchElementList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
  })

  it('handleDeleteTable does not refresh when error occurs', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.delElement.mockResolvedValue({ error: 'fail' })
    const state = getSetupState(wrapper)
    await state.handleDeleteTable('m-1')
    expect(hoisted.fetchElementList).toHaveBeenCalledTimes(0)
  })

  it('pagination.onChange updates page and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.fetchElementList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.pagination.onChange(3)
    await flushPromises()
    expect(state.queryParams.page).toBe(3)
    expect(state.pagination.page).toBe(3)
    expect(hoisted.fetchElementList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchElementList).toHaveBeenCalledWith({ page: 3, page_size: 10 })
  })

  it('pagination.onUpdatePageSize resets to page 1 and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.fetchElementList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.queryParams.page = 4
    state.pagination.onUpdatePageSize(20)
    await flushPromises()
    expect(state.queryParams.page_size).toBe(20)
    expect(state.queryParams.page).toBe(1)
    expect(state.pagination.page).toBe(1)
    expect(hoisted.fetchElementList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchElementList).toHaveBeenCalledWith({ page: 1, page_size: 20 })
  })

  it('rowKey returns row id', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.rowKey({ id: 'test-id' })).toBe('test-id')
  })

  it('columns are defined', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(Array.isArray(state.columns)).toBe(true)
    expect(state.columns.map(column => column.key)).toEqual([
      'description',
      'param2',
      'element_code',
      'param1',
      'element_type',
      'authority',
      'remark',
      'actions'
    ])
  })

  it('queryParams has correct initial values', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.queryParams.page).toBe(1)
    expect(state.queryParams.page_size).toBe(10)
  })
})
