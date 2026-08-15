/**
 * 文件用途：覆盖 edit-permission-modal 在角色管理场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchUIElementList: vi.fn(),
  getRolePermissions: vi.fn(),
  modifyRolePermissions: vi.fn(),
  deleteRolePermissions: vi.fn(),
  treeRef: {
    getIndeterminateData: vi.fn(() => ({ keys: [] }))
  },
  currentInstanceProxy: {}
}))

vi.mock('@/service/api', () => ({
  fetchUIElementList: hoisted.fetchUIElementList,
  getRolePermissions: hoisted.getRolePermissions,
  modifyRolePermissions: hoisted.modifyRolePermissions,
  deleteRolePermissions: hoisted.deleteRolePermissions
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue', async importOriginal => {
  const actual = await importOriginal<typeof import('vue')>()
  return {
    ...actual,
    getCurrentInstance: () => ({
      proxy: {
        $refs: {
          treeRef: hoisted.treeRef
        }
      }
    })
  }
})

import EditPermissionModal from '../edit-permission-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props: Record<string, any> = {}) => {
  const wrapper = shallowMount(EditPermissionModal, {
    props: {
      visible: false,
      editData: null,
      ...props
    },
    global: {
      stubs: {
        NModal: defineComponent({ name: 'NModal', props: { show: Boolean, title: String }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NForm: defineComponent({ name: 'NForm', setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NTree: defineComponent({ name: 'NTree', props: { data: { type: Array, default: () => [] }, checkedKeys: { type: Array, default: () => [] }, cascade: Boolean, checkable: Boolean, blockLine: Boolean }, emits: ['update:checkedKeys'], setup() { return () => h('div') } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

const mockUIElements = [
  {
    id: 'home',
    parent_id: '0',
    element_code: 'home',
    element_type: 1,
    description: '首页',
    children: []
  },
  {
    id: 'user',
    parent_id: '0',
    element_code: 'user',
    element_type: 1,
    description: '用户管理',
    children: [
      {
        id: 'user-list',
        parent_id: 'user',
        element_code: 'user-list',
        element_type: 2,
        description: '用户列表',
        children: []
      }
    ]
  }
]

describe('management/role/modules/edit-permission-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchUIElementList.mockResolvedValue(mockUIElements)
    hoisted.getRolePermissions.mockResolvedValue(['user'])
    hoisted.modifyRolePermissions.mockResolvedValue({ error: null })
    hoisted.deleteRolePermissions.mockResolvedValue({ error: null })
    hoisted.treeRef.getIndeterminateData.mockReturnValue({ keys: [] })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds permission modal title and tree selection state', () => {
    const wrapper = mountComponent({ visible: true, editData: { id: 'r-1', name: 'Admin' } })
    const state = getSetupState(wrapper)
    const modal = wrapper.getComponent({ name: 'NModal' })
    const tree = wrapper.getComponent({ name: 'NTree' })

    expect(modal.props('show')).toBe(true)
    expect(modal.props('title')).toBe('page.manage.role.editPermission - Admin')
    expect(tree.props('data')).toBe(state.treeOptions)
    expect(tree.props('checkedKeys')).toBe(state.selectedPermissions)
    expect(tree.props('cascade')).toBe(false)
    expect(tree.props('checkable')).toBe(true)
    expect(tree.props('blockLine')).toBe(true)
  })

  it('modalVisible get returns props.visible', () => {
    const wrapper = mountComponent({ visible: true })
    const state = getSetupState(wrapper)
    expect(state.modalVisible).toBe(true)
  })

  it('modalVisible set emits update:visible', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.modalVisible = false
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('closeModal emits update:visible false', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.closeModal()
    expect(wrapper.emitted('update:visible')![0]).toEqual([false])
  })

  it('title includes edit permission text and role name', () => {
    const wrapper = mountComponent({ editData: { id: 'r-1', name: 'Admin' } })
    const state = getSetupState(wrapper)
    expect(state.title).toBe('page.manage.role.editPermission - Admin')
  })

  it('convertToTreeNodes transforms elements to tree nodes', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const result = state.convertToTreeNodes(mockUIElements)
    expect(result).toHaveLength(2)
    expect(result[0].label).toBe('首页')
    expect(result[0].key).toBe('home')
    expect(result[0].disabled).toBe(true)
    expect(result[1].label).toBe('用户管理')
    expect(result[1].children).toHaveLength(1)
    expect(result[1].children[0].label).toBe('用户列表')
  })

  it('initUIElementList fetches UI elements and populates treeOptions', async () => {
    const wrapper = mountComponent({ editData: { id: 'r-1', name: 'Admin' } })
    const state = getSetupState(wrapper)
    await state.initUIElementList()
    await flushPromises()
    expect(hoisted.fetchUIElementList).toHaveBeenCalledTimes(1)
    expect(state.treeOptions).toHaveLength(2)
  })

  it('initRolePermissions sets selected permissions with home and role permissions', async () => {
    const wrapper = mountComponent({ editData: { id: 'r-1', name: 'Admin' } })
    const state = getSetupState(wrapper)
    state.treeOptions = state.convertToTreeNodes(mockUIElements)
    await state.initRolePermissions()
    await flushPromises()
    expect(hoisted.getRolePermissions).toHaveBeenCalledWith('r-1')
    expect(state.selectedPermissions).toContain('home')
    expect(state.selectedPermissions).toContain('user')
  })

  it('initRolePermissions sets only home when no editData', async () => {
    const wrapper = mountComponent({ editData: null })
    const state = getSetupState(wrapper)
    state.treeOptions = state.convertToTreeNodes(mockUIElements)
    await state.initRolePermissions()
    await flushPromises()
    expect(state.selectedPermissions).toEqual(['home'])
  })

  it('handleSubmit calls modifyRolePermissions when permissions exist', async () => {
    const wrapper = mountComponent({ editData: { id: 'r-1', name: 'Admin' } })
    const state = getSetupState(wrapper)
    state.selectedPermissions = ['home', 'user']
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.modifyRolePermissions).toHaveBeenCalledWith('r-1', ['home', 'user'])
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit calls deleteRolePermissions when no permissions', async () => {
    const wrapper = mountComponent({ editData: { id: 'r-1', name: 'Admin' } })
    const state = getSetupState(wrapper)
    state.selectedPermissions = []
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.deleteRolePermissions).toHaveBeenCalledWith('r-1')
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit does not emit success when API returns error', async () => {
    hoisted.modifyRolePermissions.mockResolvedValue({ error: 'fail' })
    const wrapper = mountComponent({ editData: { id: 'r-1', name: 'Admin' } })
    const state = getSetupState(wrapper)
    state.selectedPermissions = ['home']
    await state.handleSubmit()
    await flushPromises()
    expect(wrapper.emitted('success')).toBeUndefined()
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('handleSubmit clears selectedPermissions before submit', async () => {
    const wrapper = mountComponent({ editData: { id: 'r-1', name: 'Admin' } })
    const state = getSetupState(wrapper)
    state.selectedPermissions = ['home', 'user']
    await state.handleSubmit()
    await flushPromises()
    expect(state.selectedPermissions).toHaveLength(0)
  })

  it('handleSubmit includes indeterminate data in current permissions', async () => {
    hoisted.treeRef.getIndeterminateData.mockReturnValue({ keys: ['indeterminate-1'] })
    const wrapper = mountComponent({ editData: { id: 'r-1', name: 'Admin' } })
    const state = getSetupState(wrapper)
    state.selectedPermissions = ['home']
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.modifyRolePermissions).toHaveBeenCalledWith('r-1', ['home', 'indeterminate-1'])
  })
})
