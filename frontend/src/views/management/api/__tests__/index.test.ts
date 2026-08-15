/**
 * 文件用途：覆盖 index 在 API 密钥管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchKeyList: vi.fn(),
  apiKeyDel: vi.fn(),
  updateKey: vi.fn(),
  messageSuccess: vi.fn(),
  currentInstanceProxy: {
    getPlatform: () => false
  }
}))

vi.mock('@/service/api', () => ({
  fetchKeyList: hoisted.fetchKeyList,
  apiKeyDel: hoisted.apiKeyDel,
  updateKey: hoisted.updateKey,
  addKey: vi.fn()
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue', async importOriginal => {
  const actual = await importOriginal<typeof import('vue')>()
  return {
    ...actual,
    getCurrentInstance: () => ({
      proxy: hoisted.currentInstanceProxy
    })
  }
})

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

vi.mock('@/utils/common/datetime', () => ({
  formatDateTime: (v: any) => String(v || '')
}))

vi.mock('../modules/table-action-modal.vue', () => ({
  default: defineComponent({
    name: 'TableActionModalStub',
    setup() {
      return () => h('div', { class: 'table-action-modal-stub' })
    }
  })
}))

import ApiIndex from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(ApiIndex, {
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NDataTable: defineComponent({ name: 'NDataTable', props: { data: { type: Array, default: () => [] }, loading: Boolean, pagination: { default: null } }, setup() { return () => h('div') } }),
        NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } }),
        NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NSwitch: defineComponent({ props: { value: { default: false } }, emits: ['update:value', 'change'], setup() { return () => h('div') } }),
        IconIcRoundPlus: true,
        SvgIcon: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

const mockKey = (overrides: Record<string, any> = {}) => ({
  id: 'k-1',
  name: 'API Key 1',
  api_key: 'test-api-key-sample',
  status: 1,
  created_at: 1718900000,
  updated_at: 1718900100,
  show: false,
  ...overrides
})

describe('management/api/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchKeyList.mockResolvedValue({
      data: { list: [mockKey()], total: 1 }
    })
    hoisted.apiKeyDel.mockResolvedValue({ error: null })
    hoisted.updateKey.mockResolvedValue({ error: null })
    ;(globalThis as any).$message = { success: hoisted.messageSuccess }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds fetched API keys to the data table on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const table = wrapper.getComponent({ name: 'NDataTable' })

    expect(hoisted.fetchKeyList).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 1,
        page_size: 10
      })
    )
    expect(table.props('data')).toEqual([mockKey()])
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
    expect(hoisted.fetchKeyList).toHaveBeenCalledTimes(1)
  })

  it('populates tableData on success and sets show to false', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.tableData).toHaveLength(1)
    expect(state.tableData[0].show).toBe(false)
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

  it('handleEditTable sets modal type to edit and opens modal with edit data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleEditTable('k-1')
    expect(state.modalType).toBe('edit')
    expect(state.visible).toBe(true)
    expect(state.editData).toEqual(expect.objectContaining({ id: 'k-1' }))
  })

  it('handleEditTable does not set editData when row not found', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleEditTable('non-existent')
    expect(state.modalType).toBe('edit')
    expect(state.visible).toBe(true)
    expect(state.editData).toBeNull()
  })

  it('handleOpenEye sets show to true for the row', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleOpenEye('k-1')
    expect(state.tableData[0].show).toBe(true)
  })

  it('handleCloseEye sets show to false for the row', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.tableData[0].show = true
    state.handleCloseEye('k-1')
    expect(state.tableData[0].show).toBe(false)
  })

  it('handleSwitchChange toggles status and calls updateKey', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.updateKey.mockResolvedValue({ error: null })
    const state = getSetupState(wrapper)
    await state.handleSwitchChange('k-1')
    await flushPromises()
    expect(state.tableData[0].status).toBe(0)
    expect(hoisted.updateKey).toHaveBeenCalledTimes(1)
    expect(hoisted.updateKey).toHaveBeenCalledWith(expect.objectContaining({
      id: 'k-1',
      status: 0
    }))
  })

  it('handleSwitchChange toggles status from 0 to 1', async () => {
    hoisted.fetchKeyList.mockResolvedValue({ data: { list: [mockKey({ status: 0 })], total: 1 } })
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.updateKey.mockResolvedValue({ error: null })
    const state = getSetupState(wrapper)
    await state.handleSwitchChange('k-1')
    await flushPromises()
    expect(state.tableData[0].status).toBe(1)
    expect(hoisted.updateKey).toHaveBeenCalledTimes(1)
    expect(hoisted.updateKey).toHaveBeenCalledWith(expect.objectContaining({
      id: 'k-1',
      status: 1
    }))
  })

  it('handleDeleteTable calls apiKeyDel and refreshes data on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.apiKeyDel.mockResolvedValue({ error: null })
    hoisted.fetchKeyList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    await state.handleDeleteTable('k-1')
    await flushPromises()
    expect(hoisted.apiKeyDel).toHaveBeenCalledWith('k-1')
    expect(hoisted.messageSuccess).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('common.deleteSuccess')
    expect(hoisted.fetchKeyList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchKeyList).toHaveBeenCalledWith({
      name: null,
      status: null,
      page: 1,
      page_size: 10
    })
  })

  it('handleDeleteTable does not refresh when error occurs', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.apiKeyDel.mockResolvedValue({ error: 'fail' })
    const state = getSetupState(wrapper)
    await state.handleDeleteTable('k-1')
    expect(hoisted.fetchKeyList).toHaveBeenCalledTimes(0)
  })

  it('pagination.onChange updates page and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.fetchKeyList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.pagination.onChange(3)
    await flushPromises()
    expect(state.queryParams.page).toBe(3)
    expect(state.pagination.page).toBe(3)
    expect(hoisted.fetchKeyList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchKeyList).toHaveBeenCalledWith({
      name: null,
      status: null,
      page: 3,
      page_size: 10
    })
  })

  it('pagination.onUpdatePageSize resets to page 1 and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.fetchKeyList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.queryParams.page = 4
    state.pagination.onUpdatePageSize(20)
    await flushPromises()
    expect(state.queryParams.page_size).toBe(20)
    expect(state.queryParams.page).toBe(1)
    expect(state.pagination.page).toBe(1)
    expect(hoisted.fetchKeyList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchKeyList).toHaveBeenCalledWith({
      name: null,
      status: null,
      page: 1,
      page_size: 20
    })
  })

  it('getPlatform returns value from proxy', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.getPlatform).toBe(false)
  })

  it('handleCopyKey attempts to copy via clipboard API', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(globalThis, 'navigator', {
      value: { clipboard: { writeText } },
      configurable: true,
      writable: true
    })
    Object.defineProperty(globalThis, 'isSecureContext', { value: true, configurable: true, writable: true })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.handleCopyKey('test-api-key-copy')
    expect(writeText).toHaveBeenCalledTimes(1)
    expect(writeText).toHaveBeenCalledWith('test-api-key-copy')
    expect(hoisted.messageSuccess).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('theme.configOperation.copySuccess')
  })
})
