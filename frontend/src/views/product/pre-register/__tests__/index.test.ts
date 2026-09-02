/**
 * 文件用途: 覆盖产品预注册页面的前端行为与契约——列表加载、导入向导双模式提交。
 * 核心逻辑: 通过 Vue Test Utils 与 Vitest mock 服务接口，验证关键渲染、交互和数据流。
 * 关键注意事项: mock 响应需贴合后端扁平 {data,error} 包装，避免契约漂移。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getPreProductList: vi.fn(),
  getProductList: vi.fn(),
  addDevice: vi.fn(),
  exportDevice: vi.fn(),
  uploadImportBatchFile: vi.fn(),
  messageError: vi.fn(),
  messageWarning: vi.fn()
}))

vi.mock('@/service/product/list', () => ({
  getPreProductList: hoisted.getPreProductList,
  getProductList: hoisted.getProductList,
  addDevice: hoisted.addDevice,
  exportDevice: hoisted.exportDevice,
  uploadImportBatchFile: hoisted.uploadImportBatchFile
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import PreRegister from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const globalStubs = {
  NSpace: defineComponent({
    name: 'NSpace',
    setup(_, { slots }) {
      return () => h('div', slots.default?.())
    }
  }),
  NButton: defineComponent({
    name: 'NButton',
    props: ['disabled', 'loading', 'type'],
    emits: ['click'],
    setup(props, { slots, emit }) {
      return () => h('button', { disabled: props.disabled, onClick: () => emit('click') }, slots.default?.())
    }
  }),
  NInput: defineComponent({ name: 'NInput', props: ['value'], emits: ['update:value'], setup: () => () => h('div') }),
  NSelect: defineComponent({
    name: 'NSelect',
    props: ['value', 'options'],
    emits: ['update:value', 'search'],
    setup: () => () => h('div')
  }),
  NCard: defineComponent({
    name: 'NCard',
    setup(_, { slots }) {
      return () => h('div', slots.default?.())
    }
  }),
  NDataTable: defineComponent({
    name: 'NDataTable',
    props: ['data', 'loading', 'pagination'],
    setup() {
      return () => h('table')
    }
  }),
  NModal: defineComponent({
    name: 'NModal',
    props: ['show'],
    setup(_, { slots }) {
      return () => h('div', slots.default?.())
    }
  }),
  NForm: defineComponent({
    name: 'NForm',
    setup(_, { slots }) {
      return () => h('form', slots.default?.())
    }
  }),
  NFormItemGi: defineComponent({
    name: 'NFormItemGi',
    setup(_, { slots }) {
      return () => h('div', slots.default?.())
    }
  }),
  NFormItem: defineComponent({
    name: 'NFormItem',
    setup(_, { slots }) {
      return () => h('div', slots.default?.())
    }
  }),
  NGrid: defineComponent({
    name: 'NGrid',
    setup(_, { slots }) {
      return () => h('div', slots.default?.())
    }
  }),
  NRadioGroup: defineComponent({
    name: 'NRadioGroup',
    props: ['value'],
    emits: ['update:value'],
    setup(_, { slots }) {
      return () => h('div', slots.default?.())
    }
  }),
  NRadio: defineComponent({
    name: 'NRadio',
    props: ['value'],
    setup(_, { slots }) {
      return () => h('label', slots.default?.())
    }
  }),
  NInputNumber: defineComponent({
    name: 'NInputNumber',
    props: ['value'],
    emits: ['update:value'],
    setup: () => () => h('div')
  }),
  NUpload: defineComponent({
    name: 'NUpload',
    emits: ['update:file-list'],
    setup(_, { slots }) {
      return () => h('div', slots.default?.())
    }
  }),
  NEmpty: defineComponent({ name: 'NEmpty', props: ['description'], setup: () => () => h('div') }),
  NAlert: defineComponent({
    name: 'NAlert',
    props: ['type'],
    setup(_, { slots }) {
      return () => h('div', slots.default?.())
    }
  }),
  NResult: defineComponent({
    name: 'NResult',
    props: ['title'],
    setup(_, { slots }) {
      return () => h('div', slots.footer?.())
    }
  }),
  NTag: defineComponent({
    name: 'NTag',
    setup(_, { slots }) {
      return () => h('span', slots.default?.())
    }
  })
}

const mountComponent = () => {
  const wrapper = shallowMount(PreRegister, {
    global: { stubs: globalStubs }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('views/product/pre-register/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getPreProductList.mockResolvedValue({
      error: null,
      data: {
        total: 1,
        list: [
          {
            id: 'dev-1',
            name: 'batch202608-0001',
            device_number: 'PR-abcdef123456',
            activate_flag: 'inactive',
            batch_number: 'batch202608',
            current_version: 'v1.2.0',
            created_at: '2026-08-26T08:00:00Z'
          }
        ]
      }
    })
    hoisted.getProductList.mockResolvedValue({
      error: null,
      data: { total: 1, list: [{ id: 'p-1', name: 'Product A' }] }
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads pre-register list on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.getPreProductList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(getSetupState(wrapper).tableData).toHaveLength(1)
  })

  it('submits auto mode payload and refreshes list', async () => {
    hoisted.addDevice.mockResolvedValue({
      error: null,
      data: { created_count: 2, devices: [], skipped_existing: [], skipped_duplicate_rows: [] }
    })
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.openModal()
    state.mode = 'auto'
    state.form.product_id = 'p-1'
    state.form.batch_number = 'batchA'
    state.form.device_count = 2

    const ok = await state.submitImport()
    await flushPromises()

    expect(ok).toBe(true)
    expect(hoisted.addDevice).toHaveBeenCalledWith(
      expect.objectContaining({ product_id: 'p-1', batch_number: 'batchA', create_type: '1', device_count: 2 })
    )
    expect(hoisted.getPreProductList).toHaveBeenCalledTimes(2)
  })

  it('file mode without uploaded csv blocks submit until upload succeeds', async () => {
    hoisted.addDevice.mockResolvedValue({
      error: null,
      data: { created_count: 0, devices: [], skipped_existing: [], skipped_duplicate_rows: [] }
    })
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.openModal()
    state.mode = 'file'
    state.form.product_id = 'p-1'
    state.form.batch_number = 'batchB'

    expect(state.canSubmit).toBe(false)

    state.selectFile(new File(['device_number,name\nX-1,X\n'], 'batch.csv', { type: 'text/csv' }))
    hoisted.uploadImportBatchFile.mockResolvedValue({ error: null, data: { path: 'files/upload/importBatch/x.csv' } })
    expect(await state.submitImport()).toBe(true)
    await flushPromises()

    expect(hoisted.uploadImportBatchFile).toHaveBeenCalled()
    expect(hoisted.addDevice).toHaveBeenCalledWith(
      expect.objectContaining({ create_type: '2', batch_file: 'files/upload/importBatch/x.csv' })
    )
  })
})
