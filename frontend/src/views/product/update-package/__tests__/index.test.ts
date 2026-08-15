/**
 * 文件用途: 覆盖测试在产品升级场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
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
  routerPush: vi.fn(),
}))

vi.mock('@/service/product/update-package', () => ({
  getOtaPackageList: hoisted.getOtaPackageList,
  addOtaPackage: hoisted.addOtaPackage,
  editOtaPackage: hoisted.editOtaPackage,
  deleteOtaPackage: hoisted.deleteOtaPackage,
}))

vi.mock('@/service/api/device', () => ({
  getDeviceConfigList: hoisted.getDeviceConfigList,
}))

vi.mock('@/service/api/personal-center', () => ({
  uploadFile: hoisted.uploadFile,
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: hoisted.routeQuery }),
  useRouter: () => ({ push: hoisted.routerPush })
}))

import UpdatePackage from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(UpdatePackage, {
    props,
    global: {
      stubs: {
        NSpace: defineComponent({ props: ['vertical', 'align', 'wrap'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NCard: defineComponent({ props: ['bordered'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NButton: defineComponent({ emits: ['click'], props: ['loading', 'disabled', 'type', 'size'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NAlert: defineComponent({ props: ['type', 'showIcon'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: { value: { default: null }, options: { default: () => [] } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NDataTable: defineComponent({ props: ['data', 'loading', 'columns', 'pagination', 'remote', 'scrollX'], setup() { return () => h('div') } }),
        NEmpty: defineComponent({ props: ['description'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ props: ['labelPlacement', 'model'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label', 'required', 'path'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NDescriptions: defineComponent({ props: ['bordered', 'column', 'labelPlacement', 'size'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NDescriptionsItem: defineComponent({ props: ['label'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        NGrid: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItemGi: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInputNumber: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
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
    mountedWrappers.forEach(w => w.unmount())
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

  it('should open edit modal with row data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const row = { id: '1', name: 'Pkg1', version: '1.0', target_version: '2.0', device_config_id: 'dc1', module: 'mod', package_type: 2, signature_type: 'MD5', package_url: '/pkg.bin', additional_info: '{}', description: 'desc', remark: '' }
    state.openEditModal(row)
    expect(state.modalVisible).toBe(true)
    expect(state.isEditing).toBe(true)
    expect(state.form.id).toBe('1')
    expect(state.form.name).toBe('Pkg1')
  })

  it('should open detail modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const row = { id: '1', name: 'Pkg1' }
    state.openDetailModal(row)
    expect(state.detailVisible).toBe(true)
    expect(state.detailRecord).toEqual(row)
  })

  it('should reset form', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.form.name = 'test'
    state.resetForm()
    expect(state.form.name).toBe('')
    expect(state.form.package_type).toBe(2)
  })

  it('should format time', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.formatTime(undefined)).toBe('-')
    expect(state.formatTime('2024-01-01')).toBe('2024-01-01 00:00:00')
  })

  it('should get package type label', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.packageTypeLabel(1)).toBe('page.product.update-package.diff')
    expect(state.packageTypeLabel(2)).toBe('page.product.update-package.full')
    expect(state.packageTypeLabel(undefined)).toBe('page.product.update-package.full')
  })

  it('should normalize package URL', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.normalizePackageUrl('./pkg.bin')).toBe('/pkg.bin')
    expect(state.normalizePackageUrl('/pkg.bin')).toBe('/pkg.bin')
    expect(state.normalizePackageUrl('')).toBe('')
  })

  it('should not build payload with empty required fields', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.form.name = ''
    state.form.version = ''
    state.form.device_config_id = null
    state.form.package_url = ''
    state.form.additional_info = '{}'
    const payload = state.buildPayload()
    expect(payload).toBeNull()
  })

  it('should reject invalid additional info before saving', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.form.name = 'Pkg 1'
    state.form.version = '1.0.0'
    state.form.device_config_id = 'cfg-1'
    state.form.package_url = '/files/pkg.bin'
    state.form.additional_info = '{bad json'

    expect(state.buildPayload()).toBeNull()
    expect((window as any).$message.error).toHaveBeenCalledWith('page.product.update-package.customInfo')
  })

  it('should trim fields and build a create payload', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.form.name = '  Pkg 1  '
    state.form.version = ' 1.0.0 '
    state.form.target_version = ' 2.0.0 '
    state.form.device_config_id = 'cfg-1'
    state.form.module = ' firmware '
    state.form.package_type = 1
    state.form.signature_type = 'SHA256'
    state.form.package_url = ' /files/pkg.bin '
    state.form.additional_info = ''
    state.form.description = ' release '
    state.form.remark = ' remark '

    expect(state.buildPayload()).toEqual({
      id: undefined,
      name: 'Pkg 1',
      version: '1.0.0',
      target_version: '2.0.0',
      device_config_id: 'cfg-1',
      module: 'firmware',
      package_type: 1,
      signature_type: 'SHA256',
      package_url: '/files/pkg.bin',
      additional_info: '{}',
      description: 'release',
      remark: 'remark'
    })
  })

  it('should add package and refresh list on successful save', async () => {
    hoisted.addOtaPackage.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getState(wrapper)
    state.form.name = 'Pkg 1'
    state.form.version = '1.0.0'
    state.form.device_config_id = 'cfg-1'
    state.form.package_url = '/files/pkg.bin'

    await state.savePackage()
    await flushPromises()

    expect(hoisted.addOtaPackage).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Pkg 1',
      version: '1.0.0',
      device_config_id: 'cfg-1',
      package_url: '/files/pkg.bin'
    }))
    expect(hoisted.getOtaPackageList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      name: '',
      version: '',
      device_config_id: ''
    })
    expect(state.modalVisible).toBe(false)
  })

  it('returns to OTA task creation after saving a package from OTA onboarding', async () => {
    hoisted.routeQuery = { return_to: 'ota_task' }
    hoisted.addOtaPackage.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getState(wrapper)
    state.form.name = 'Pkg 1'
    state.form.version = '1.0.0'
    state.form.device_config_id = 'cfg-1'
    state.form.package_url = '/files/pkg.bin'

    await state.savePackageAndContinue()
    await flushPromises()

    expect(hoisted.addOtaPackage).toHaveBeenCalledTimes(1)
    expect(hoisted.routerPush).toHaveBeenCalledWith({ name: 'product_update-ota' })
  })

  it('should edit package instead of adding when edit modal is active', async () => {
    hoisted.editOtaPackage.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getState(wrapper)
    state.openEditModal({
      id: 'pkg-1',
      name: 'Pkg 1',
      version: '1.0.0',
      device_config_id: 'cfg-1',
      package_url: '/files/pkg.bin'
    })

    await state.savePackage()

    expect(hoisted.editOtaPackage).toHaveBeenCalledTimes(1)
    expect(hoisted.editOtaPackage).toHaveBeenCalledWith(expect.objectContaining({ id: 'pkg-1' }))
    expect(hoisted.addOtaPackage).toHaveBeenCalledTimes(0)
    expect(hoisted.getOtaPackageList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      name: '',
      version: '',
      device_config_id: ''
    })
  })

  it('should upload the selected firmware file and store returned path', async () => {
    hoisted.uploadFile.mockResolvedValue({ data: { path: './upgradePackage/pkg.bin' }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.selectPackageFile(new File(['abc'], 'pkg.bin'))

    await state.uploadSelectedFile()

    expect(hoisted.uploadFile).toHaveBeenCalledTimes(1)
    const formData = hoisted.uploadFile.mock.calls[0][0] as FormData
    expect(formData.get('type')).toBe('upgradePackage')
    expect(state.form.package_url).toBe('./upgradePackage/pkg.bin')
  })

  it('should warn and skip upload when no firmware file is selected', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)

    await state.uploadSelectedFile()

    expect((window as any).$message.warning).toHaveBeenCalledWith('page.product.update-package.packagePlaceholder')
    expect(hoisted.uploadFile).toHaveBeenCalledTimes(0)
  })

  it('should delete package only after dialog confirmation', async () => {
    hoisted.deleteOtaPackage.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getState(wrapper)

    state.deletePackage({ id: 'pkg-1', name: 'Pkg 1' })
    const dialogOptions = (window as any).$dialog.warning.mock.calls[0][0]
    await dialogOptions.onPositiveClick()

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

  it('should fetch with updated pagination parameters', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getState(wrapper)

    state.pagination.onChange(3)
    state.pagination.onUpdatePageSize(20)
    await flushPromises()

    expect(hoisted.getOtaPackageList).toHaveBeenCalledWith(expect.objectContaining({ page: 3, page_size: 10 }))
    expect(hoisted.getOtaPackageList).toHaveBeenCalledWith(expect.objectContaining({ page: 1, page_size: 20 }))
    expect(state.queryParams.page).toBe(1)
    expect(state.queryParams.page_size).toBe(20)
  })

  it('should reset query', async () => {
    hoisted.getOtaPackageList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.queryParams.name = 'test'
    state.resetQuery()
    await flushPromises()
    expect(state.queryParams.name).toBe('')
  })

  it('should format optional value', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.formatOptional(null)).toBe('-')
    expect(state.formatOptional('')).toBe('-')
    expect(state.formatOptional('test')).toBe('test')
  })

  it('should get package file name from URL', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.packageFileName('/path/to/file.bin')).toBe('file.bin')
    expect(state.packageFileName(null)).toBe('-')
  })
})
