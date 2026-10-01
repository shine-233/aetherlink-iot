/**
 * 文件用途: 覆盖测试在产品升级场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证页面初始化、筛选分页
 *   （useListPage 语义）、弹窗编排、删除/下载与 OTA 回跳流程。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, nextTick } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getOtaPackageList: vi.fn(),
  addOtaPackage: vi.fn(),
  editOtaPackage: vi.fn(),
  deleteOtaPackage: vi.fn(),
  getDeviceConfigList: vi.fn(),
  uploadFile: vi.fn(),
  routeQuery: {} as Record<string, any>,
  routerPush: vi.fn()
}))

vi.mock('@/service/product/update-package', () => ({
  getOtaPackageList: hoisted.getOtaPackageList,
  addOtaPackage: hoisted.addOtaPackage,
  editOtaPackage: hoisted.editOtaPackage,
  deleteOtaPackage: hoisted.deleteOtaPackage
}))

vi.mock('@/service/api/device', () => ({
  getDeviceConfigList: hoisted.getDeviceConfigList
}))

vi.mock('@/service/api/personal-center', () => ({
  uploadFile: hoisted.uploadFile
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: hoisted.routeQuery }),
  useRouter: () => ({ push: hoisted.routerPush })
}))

import UpdatePackage from '../index.vue'
import PackageDetailModal from '../package-detail-modal.vue'
import PackageFormModal from '../package-form-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(UpdatePackage, {
    props,
    global: {
      stubs: {
        NSpace: defineComponent({
          props: ['vertical', 'align', 'wrap'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NCard: defineComponent({
          props: ['bordered'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NButton: defineComponent({
          emits: ['click'],
          props: ['loading', 'disabled', 'type', 'size'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default?.())
          }
        }),
        NAlert: defineComponent({
          props: ['type', 'showIcon'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NInput: defineComponent({
          props: { value: { default: '' } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NSelect: defineComponent({
          props: { value: { default: null }, options: { default: () => [] } },
          emits: ['update:value', 'search'],
          setup() {
            return () => h('div')
          }
        }),
        NDataTable: defineComponent({
          props: ['data', 'loading', 'columns', 'pagination', 'remote', 'scrollX'],
          setup() {
            return () => h('div')
          }
        }),
        NEmpty: defineComponent({
          props: ['description'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('UpdatePackage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.routeQuery = {}
    hoisted.getOtaPackageList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.getDeviceConfigList.mockResolvedValue({ data: { list: [] }, error: null })
    Object.defineProperty(window, '$message', {
      configurable: true,
      value: {
        success: vi.fn(),
        warning: vi.fn(),
        error: vi.fn()
      }
    })
    Object.defineProperty(window, '$dialog', {
      configurable: true,
      value: {
        warning: vi.fn()
      }
    })
    vi.spyOn(window, 'open').mockImplementation(() => null)
  })

  afterEach(() => {
    mountedWrappers.forEach((w) => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount and fetch packages and device configs', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getOtaPackageList).toHaveBeenCalledTimes(1)
    expect(hoisted.getOtaPackageList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      name: '',
      version: '',
      device_config_id: ''
    })
    expect(hoisted.getDeviceConfigList).toHaveBeenCalledTimes(1)
    expect(hoisted.getDeviceConfigList).toHaveBeenCalledWith({ page: 1, page_size: 20 })
  })

  it('should normalize package and device config list payloads on fetch', async () => {
    hoisted.getOtaPackageList.mockResolvedValue({
      data: {
        data: {
          list: [{ id: 'pkg-1', name: 'Pkg 1' }],
          total: 1
        }
      },
      error: null
    })
    hoisted.getDeviceConfigList.mockResolvedValue({
      data: {
        records: [
          { id: 'cfg-1', name: 'Config A' },
          { id: 'cfg-2', device_config_name: 'Config B' }
        ]
      },
      error: null
    })

    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    expect(state.tableData).toEqual([{ id: 'pkg-1', name: 'Pkg 1' }])
    expect(state.pagination.itemCount).toBe(1)
    expect(state.deviceConfigOptions).toEqual([
      { label: 'Config A', value: 'cfg-1' },
      { label: 'Config B', value: 'cfg-2' }
    ])
  })

  it('should open create modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.openCreateModal()
    expect(state.modalVisible).toBe(true)
    expect(state.isEditing).toBe(false)
  })

  it('should open edit modal with row data and ensure its device config option', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.openPackageEditModal({
      id: '1',
      name: 'Pkg1',
      version: '1.0',
      target_version: '2.0',
      device_config_id: 'dc1',
      module: 'mod',
      package_type: 2,
      signature_type: 'MD5',
      package_url: '/pkg.bin',
      additional_info: '{}',
      description: 'desc',
      remark: ''
    })
    expect(state.modalVisible).toBe(true)
    expect(state.isEditing).toBe(true)
    expect(state.form.id).toBe('1')
    expect(state.form.name).toBe('Pkg1')
    expect(state.deviceConfigOptions).toEqual([{ label: 'dc1', value: 'dc1' }])
  })

  it('should open detail modal and pass the record to the detail modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const row = { id: '1', name: 'Pkg1' }
    state.openDetailModal(row)
    expect(state.detailVisible).toBe(true)
    expect(state.detailRecord).toEqual(row)

    // openDetailModal 只改 ref，子组件 props 要等下一次渲染 tick 才同步
    await nextTick()
    const detailModal = wrapper.findComponent(PackageDetailModal)
    expect(detailModal.props('record')).toEqual(row)
    expect(detailModal.props('show')).toBe(true)
  })

  it('should fetch with updated pagination parameters', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getState(wrapper)

    state.pagination.onUpdatePage(3)
    await flushPromises()
    expect(hoisted.getOtaPackageList).toHaveBeenLastCalledWith(expect.objectContaining({ page: 3, page_size: 10 }))

    state.pagination.onUpdatePageSize(20)
    await flushPromises()
    expect(hoisted.getOtaPackageList).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, page_size: 20 }))
    expect(state.pagination.page).toBe(1)
    expect(state.pagination.pageSize).toBe(20)
  })

  it('should search from page 1 after paging away', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    state.pagination.onUpdatePage(3)
    await flushPromises()
    vi.clearAllMocks()

    await state.searchPackages()
    expect(hoisted.getOtaPackageList).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, page_size: 10 }))
  })

  it('should reset query to defaults and reload page 1', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.filter.name = 'test'
    state.filter.device_config_id = 'cfg-1'
    state.resetQuery()
    await flushPromises()
    expect(state.filter.name).toBe('')
    expect(state.filter.device_config_id).toBeNull()
    expect(hoisted.getOtaPackageList).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 10,
      name: '',
      version: '',
      device_config_id: ''
    })
  })

  it('should wire the form modal: form model, selected file and save flow', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const formModal = wrapper.findComponent(PackageFormModal)
    expect(formModal.props('form')).toBe(state.form)
    expect(formModal.props('deviceConfigOptions')).toEqual([])

    const file = new File(['abc'], 'pkg.bin')
    await formModal.vm.$emit('select-file', file)
    expect(state.selectedFile).toEqual(file)

    state.form.name = 'Pkg 1'
    state.form.version = '1.0.0'
    state.form.device_config_id = 'cfg-1'
    state.form.package_url = '/files/pkg.bin'
    hoisted.addOtaPackage.mockResolvedValue({ error: null })
    vi.clearAllMocks()

    await formModal.vm.$emit('save')
    await flushPromises()

    expect(hoisted.addOtaPackage).toHaveBeenCalledTimes(1)
    expect(state.modalVisible).toBe(false)
    expect(hoisted.getOtaPackageList).toHaveBeenCalledTimes(1)
    expect(hoisted.routerPush).not.toHaveBeenCalled()
  })

  it('returns to OTA task creation after saving a package from OTA onboarding', async () => {
    hoisted.routeQuery = { return_to: 'ota_task' }
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.form.name = 'Pkg 1'
    state.form.version = '1.0.0'
    state.form.device_config_id = 'cfg-1'
    state.form.package_url = '/files/pkg.bin'
    hoisted.addOtaPackage.mockResolvedValue({ error: null })

    await state.savePackageAndContinue()
    await flushPromises()

    expect(hoisted.addOtaPackage).toHaveBeenCalledTimes(1)
    expect(hoisted.routerPush).toHaveBeenCalledWith({ name: 'product_update-ota' })
  })

  it('should delete package only after dialog confirmation', async () => {
    hoisted.deleteOtaPackage.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.deleteOtaPackage.mockResolvedValue({ error: null })
    const state = getState(wrapper)

    state.deletePackage({ id: 'pkg-1', name: 'Pkg 1' })
    const dialogOptions = (window as any).$dialog.warning.mock.calls[0][0]
    await dialogOptions.onPositiveClick()
    await flushPromises()

    expect(hoisted.deleteOtaPackage).toHaveBeenCalledTimes(1)
    expect(hoisted.deleteOtaPackage).toHaveBeenCalledWith('pkg-1')
    expect(hoisted.getOtaPackageList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      name: '',
      version: '',
      device_config_id: ''
    })
  })

  it('should open normalized package URL in a new tab', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    state.downloadPackage({ id: 'pkg-1', package_url: './upgradePackage/pkg.bin' })

    expect(window.open).toHaveBeenCalledWith('/upgradePackage/pkg.bin', '_blank', 'noopener,noreferrer')
  })
})
