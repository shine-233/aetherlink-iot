/**
 * 文件用途：覆盖 data-clear-setting 在 系统与账号设置 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchDataClearList: vi.fn(),
  editDataClear: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api/setting', () => ({
  fetchDataClearList: hoisted.fetchDataClearList,
  editDataClear: hoisted.editDataClear
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
  dataClearSettingEnabledTypeOptions: [
    { label: 'Enable', value: '1' },
    { label: 'Disable', value: '0' }
  ]
}))

vi.mock('@/utils/common/tool', () => ({
  deepClone: (obj: any) => JSON.parse(JSON.stringify(obj))
}))

import DataClearSetting from '../data-clear-setting.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(DataClearSetting, {
    global: {
      stubs: {
        NDataTable: defineComponent({
          props: { data: { type: Array, default: () => [] }, loading: Boolean },
          setup() {
            return () => h('div')
          }
        }),
        NModal: defineComponent({
          props: { show: Boolean },
          emits: ['update:show'],
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NForm: defineComponent({
          props: { model: Object, labelPlacement: String, labelWidth: [String, Number] },
          setup(_, { slots }) {
            const validate = vi.fn().mockResolvedValue(undefined)
            const restoreValidation = vi.fn().mockResolvedValue(undefined)
            // Expose validate so template ref formRef.value.validate() works
            return { validate, restoreValidation, default: () => slots.default ? slots.default() : [] }
          },
          render() {
            return h('form', this.default ? this.default() : [])
          }
        }),
        NFormItem: defineComponent({
          props: { label: String, path: String },
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NFormItemGridItem: defineComponent({
          props: { span: Number, label: String, path: String },
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NGrid: defineComponent({
          props: { cols: Number, xGap: [String, Number] },
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NInput: defineComponent({
          props: { value: { default: '' }, type: String, placeholder: String },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NInputNumber: defineComponent({
          props: { value: { default: 0 } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NRadioGroup: defineComponent({
          props: { value: { default: null } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NRadio: defineComponent({
          props: { value: [String, Number] },
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NSpace: defineComponent({
          props: { size: [String, Number], justify: String },
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NButton: defineComponent({
          props: { type: String, size: String },
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : [])
          }
        }),
        NTag: defineComponent({
          props: { type: String },
          setup(_, { slots }) {
            return () => h('span', slots.default ? slots.default() : [])
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) =>
  wrapper.vm.$.setupState as Record<string, any>

const mockDataClear = (overrides: Record<string, any> = {}) => ({
  id: 'dc-1',
  data_type: '1',
  retention_days: 30,
  last_cleanup_time: 1718900000,
  last_cleanup_data_time: 1718900100,
  remark: 'remark',
  enabled: '1',
  ...overrides
})

describe('management/setting/components/data-clear-setting.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchDataClearList.mockResolvedValue({
      data: { list: [mockDataClear()], total: 1 }
    })
    hoisted.editDataClear.mockResolvedValue({ error: null, msg: 'success' })
    ;(globalThis as any).$message = { success: hoisted.messageSuccess }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds data-clear policies to the table on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.fetchDataClearList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(state.tableData).toEqual([mockDataClear()])
    expect(state.loading).toBe(false)
  })

  it('calls fetchDataClearList on init', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.fetchDataClearList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchDataClearList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
  })

  it('populates tableData on successful load', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.tableData).toHaveLength(1)
    expect(state.loading).toBe(false)
  })

  it('gracefully falls back to empty table data when fetchDataClearList rejects', async () => {
    hoisted.fetchDataClearList.mockRejectedValueOnce(new Error('no permission to manage data policy'))
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.tableData).toEqual([])
    expect(state.loading).toBe(false)
  })

  it('createDefaultFormModel returns correct default values', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const model = state.createDefaultFormModel()
    expect(model.retention_days).toBe(0)
    expect(model.enabled).toBe('1')
    expect(model.remark).toBeNull()
  })

  it('setEditData assigns data to editData', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const data = mockDataClear({ retention_days: 60 })
    state.setEditData(data)
    expect(state.editData.retention_days).toBe(60)
  })

  it('handleEditTable sets editData and opens modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const row = mockDataClear({ retention_days: 90 })
    state.handleEditTable(row)
    expect(state.editData.retention_days).toBe(90)
    expect(state.visible).toBe(true)
  })

  it('handleSubmit validates, calls editDataClear and refreshes on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editDataClear.mockResolvedValue({ error: null, msg: 'success' })
    hoisted.fetchDataClearList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.handleEditTable(mockDataClear())
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.editDataClear).toHaveBeenCalledTimes(1)
    expect(hoisted.editDataClear).toHaveBeenCalledWith(expect.objectContaining({
      id: 'dc-1',
      retention_days: 30,
      enabled: '1'
    }))
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('success')
    expect(hoisted.fetchDataClearList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchDataClearList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
  })

  it('handleSubmit does not refresh when API returns error', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editDataClear.mockResolvedValue({ error: 'fail' })
    const state = getSetupState(wrapper)
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.fetchDataClearList).toHaveBeenCalledTimes(0)
  })

  it('handleSubmit closes modal when editDataClear rejects', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editDataClear.mockRejectedValue(new Error('save failed'))
    const state = getSetupState(wrapper)
    state.handleEditTable(mockDataClear())
    await state.handleSubmit()
    await flushPromises()
    expect(state.visible).toBe(false)
    expect(hoisted.fetchDataClearList).toHaveBeenCalledTimes(0)
  })

  it('handleSubmit closes modal after completion', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editDataClear.mockResolvedValue({ error: null, msg: 'success' })
    hoisted.fetchDataClearList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    await state.handleSubmit()
    await flushPromises()
    expect(state.visible).toBe(false)
  })

  it('handleSubmit deep clones editData before submit', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editDataClear.mockResolvedValue({ error: null, msg: 'success' })
    hoisted.fetchDataClearList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined) }
    state.editData.retention_days = 100
    await state.handleSubmit()
    await flushPromises()
    const callArgs = hoisted.editDataClear.mock.calls[0][0]
    expect(callArgs.retention_days).toBe(100)
    expect(callArgs).not.toBe(state.editData)
  })

  it('columns are defined with correct keys', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(Array.isArray(state.columns)).toBe(true)
    const keys = state.columns.map((c: any) => c.key)
    expect(keys).toContain('id')
    expect(keys).toContain('data_type')
    expect(keys).toContain('retention_days')
    expect(keys).toContain('actions')
  })

  it('queryParams has correct initial values', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.queryParams.page).toBe(1)
    expect(state.queryParams.page_size).toBe(10)
  })

  it('setTableData populates tableData correctly', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const data = [mockDataClear(), mockDataClear({ id: 'dc-2' })]
    state.setTableData(data)
    expect(state.tableData).toHaveLength(2)
  })
})
