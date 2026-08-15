/**
 * 文件用途：覆盖 index 在角色管理场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  listRoles: vi.fn(),
  deleteRole: vi.fn(),
  messageSuccess: vi.fn(),
  currentInstanceProxy: {
    getPlatform: () => false
  }
}))

vi.mock('@/service/api', () => ({
  listRoles: hoisted.listRoles,
  deleteRole: hoisted.deleteRole,
  createRole: vi.fn(),
  updateRole: vi.fn()
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

vi.mock('../modules/edit-permission-modal.vue', () => ({
  default: defineComponent({
    name: 'EditPermissionModalStub',
    setup() {
      return () => h('div', { class: 'edit-permission-modal-stub' })
    }
  })
}))

import RoleIndex from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(RoleIndex, {
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NDataTable: defineComponent({ name: 'NDataTable', props: { data: { type: Array, default: () => [] }, loading: Boolean, pagination: { default: null } }, setup() { return () => h('div') } }),
        NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        IconIcRoundPlus: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

const mockRole = (overrides: Record<string, any> = {}) => ({
  id: 'r-1',
  name: 'Admin',
  description: 'Administrator role',
  created_at: 1718900000,
  updated_at: 1718900100,
  ...overrides
})

describe('management/role/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.listRoles.mockResolvedValue({
      data: { list: [mockRole()], total: 1 }
    })
    hoisted.deleteRole.mockResolvedValue({ error: null })
    ;(globalThis as any).$message = { success: hoisted.messageSuccess }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds fetched roles to the data table on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const table = wrapper.getComponent({ name: 'NDataTable' })

    expect(hoisted.listRoles).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      email: null,
      name: null,
      status: null
    })
    expect(table.props('data')).toEqual([mockRole()])
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
    expect(hoisted.listRoles).toHaveBeenCalledTimes(1)
  })

  it('populates tableData on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.tableData).toHaveLength(1)
    expect(state.loading).toBe(false)
  })

  it('handleAddTable opens modal with add type', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.setEditData(mockRole())
    state.handleAddTable()
    expect(state.modalType).toBe('add')
    expect(state.visible).toBe(true)
    expect(state.editData).toBeNull()
  })

  it('handleEditTable sets modal type to edit and opens modal with edit data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleEditTable('r-1')
    expect(state.modalType).toBe('edit')
    expect(state.visible).toBe(true)
    expect(state.editData).toEqual(expect.objectContaining({ id: 'r-1' }))
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

  it('handleEditPermission opens edit permission modal with edit data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleEditPermission('r-1')
    expect(state.editPermissionVisible).toBe(true)
    expect(state.editData).toEqual(expect.objectContaining({ id: 'r-1' }))
  })

  it('handleDeleteTable calls deleteRole and refreshes data on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.deleteRole.mockResolvedValue({ error: null })
    hoisted.listRoles.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    await state.handleDeleteTable('r-1')
    await flushPromises()
    expect(hoisted.deleteRole).toHaveBeenCalledWith('r-1')
    expect(hoisted.messageSuccess).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('common.deleteSuccess')
    expect(hoisted.listRoles).toHaveBeenCalledTimes(1)
    expect(hoisted.listRoles).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      email: null,
      name: null,
      status: null
    })
  })

  it('handleDeleteTable does not refresh when error occurs', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.deleteRole.mockResolvedValue({ error: 'fail' })
    const state = getSetupState(wrapper)
    await state.handleDeleteTable('r-1')
    expect(hoisted.listRoles).toHaveBeenCalledTimes(0)
  })

  it('pagination.onChange updates page and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.listRoles.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.pagination.onChange(3)
    await flushPromises()
    expect(state.queryParams.page).toBe(3)
    expect(state.pagination.page).toBe(3)
    expect(hoisted.listRoles).toHaveBeenCalledTimes(1)
    expect(hoisted.listRoles).toHaveBeenCalledWith({
      page: 3,
      page_size: 10,
      email: null,
      name: null,
      status: null
    })
  })

  it('pagination.onUpdatePageSize resets to page 1 and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.listRoles.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.queryParams.page = 4
    state.pagination.onUpdatePageSize(20)
    await flushPromises()
    expect(state.queryParams.page_size).toBe(20)
    expect(state.queryParams.page).toBe(1)
    expect(state.pagination.page).toBe(1)
    expect(hoisted.listRoles).toHaveBeenCalledTimes(1)
    expect(hoisted.listRoles).toHaveBeenCalledWith({
      page: 1,
      page_size: 20,
      email: null,
      name: null,
      status: null
    })
  })

  it('getPlatform returns value from proxy', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.getPlatform).toBe(false)
  })

  it('columns are defined', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(Array.isArray(state.columns)).toBe(true)
    expect(state.columns.map((column: any) => column.key)).toEqual([
      'name',
      'description',
      'created_at',
      'updated_at',
      'actions'
    ])
  })

  it('queryParams has correct initial values', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.queryParams.page).toBe(1)
    expect(state.queryParams.page_size).toBe(10)
    expect(state.queryParams.email).toBeNull()
    expect(state.queryParams.name).toBeNull()
    expect(state.queryParams.status).toBeNull()
  })
})
