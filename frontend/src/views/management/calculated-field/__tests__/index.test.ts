/**
 * 文件用途：覆盖 management/calculated-field/index.vue 的列表渲染、提交载荷、启停事件与空态。
 * 核心逻辑：mock 计算字段与模板 API，验证首屏拉取参数、create/update 载荷含 output_key 与 expression、
 *   toggle 事件透传目标状态并刷新列表，以及空响应下的空态数据流。
 * 关键注意事项：本套件只覆盖前端组件行为，不证明后端派生遥测结果。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getCalculatedFields: vi.fn(),
  getCalculatedField: vi.fn(),
  createCalculatedField: vi.fn(),
  updateCalculatedField: vi.fn(),
  toggleCalculatedField: vi.fn(),
  deleteCalculatedField: vi.fn(),
  deviceTemplate: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api/calculated_field', () => ({
  getCalculatedFields: hoisted.getCalculatedFields,
  getCalculatedField: hoisted.getCalculatedField,
  createCalculatedField: hoisted.createCalculatedField,
  updateCalculatedField: hoisted.updateCalculatedField,
  toggleCalculatedField: hoisted.toggleCalculatedField,
  deleteCalculatedField: hoisted.deleteCalculatedField
}))

vi.mock('@/service/api/device-template-model', () => ({
  deviceTemplate: hoisted.deviceTemplate
}))

vi.mock('~/packages/hooks', () => ({
  useLoading: (initial = false) => {
    const loading = ref(initial)
    return {
      loading,
      startLoading: () => {
        loading.value = true
      },
      endLoading: () => {
        loading.value = false
      }
    }
  }
}))

import CalculatedFieldSetting from '../index.vue'

type CalcFieldRowFixture = {
  id: string
  enabled: boolean
  output_key: string
}

type SetupState = Record<string, unknown> & {
  tableData: CalcFieldRowFixture[]
  pagination: { page: number; pageSize: number; itemCount: number }
  formModel: Record<string, string>
}

const mountedWrappers: Array<VueWrapper> = []

function simpleStub(tag = 'div') {
  return defineComponent({
    setup(_, { slots }) {
      return () => h(tag, slots.default ? slots.default() : [])
    }
  })
}

// 暴露与 NaiveUI NForm 兼容的 validate，供模板引用在弹窗提交链路中调用。
function formStub() {
  return defineComponent({
    name: 'NFormStub',
    setup(_, { slots, expose }) {
      expose({ validate: () => Promise.resolve(true) })
      return () => h('form', slots.default ? slots.default() : [])
    }
  })
}

function mountComponent() {
  const wrapper = shallowMount(CalculatedFieldSetting, {
    global: {
      stubs: {
        NSpace: simpleStub(),
        NButton: simpleStub('button'),
        NDataTable: simpleStub(),
        NModal: simpleStub(),
        NForm: formStub(),
        NFormItem: simpleStub(),
        NInput: simpleStub('input'),
        NSelect: simpleStub('select')
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

function getSetupState(wrapper: VueWrapper): SetupState {
  return wrapper.vm.$.setupState as unknown as SetupState
}

const templateListResponse = {
  data: {
    total: 2,
    list: [
      { id: 'tpl-1', name: 'Template One' },
      { id: 'tpl-2', name: 'Template Two' }
    ]
  }
}

describe('management/calculated-field/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getCalculatedFields.mockResolvedValue({
      data: {
        total: 1,
        list: [
          {
            id: 'cf-1',
            tenant_id: 'tenant-a',
            name: 'Power',
            device_template_id: 'tpl-1',
            output_key: 'power_w',
            expression: '(voltage * current) / 1000',
            enabled: false,
            remark: null,
            created_at: '2026-08-01T00:00:00Z',
            updated_at: '2026-08-02T00:00:00Z'
          }
        ]
      }
    })
    hoisted.deviceTemplate.mockResolvedValue(templateListResponse)
    ;(window as unknown as Record<string, unknown>).$message = {
      success: hoisted.messageSuccess
    }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('renders rows on mount and maps template ids to names', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.getCalculatedFields).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(hoisted.deviceTemplate).toHaveBeenCalledTimes(1)
    const state = getSetupState(wrapper)
    expect(state.tableData).toHaveLength(1)
    expect(state.tableData[0].output_key).toBe('power_w')
    const templateNameFn = getSetupState(wrapper).templateName as (id: string) => string
    expect(templateNameFn('tpl-1')).toBe('Template One')
    expect(templateNameFn('tpl-missing')).toBe('tpl-missing')
  })

  it('submits create payloads containing output_key and expression', async () => {
    hoisted.createCalculatedField.mockResolvedValue({ data: null })
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.formModel.name = 'Energy'
    state.formModel.device_template_id = 'tpl-2'
    state.formModel.output_key = 'energy_wh'
    state.formModel.expression = '(voltage * current) / 1000'
    await (wrapper.vm.$.setupState.handleSubmit as () => Promise<void>)()
    await flushPromises()

    expect(hoisted.createCalculatedField).toHaveBeenCalledTimes(1)
    const payload = hoisted.createCalculatedField.mock.calls[0][0] as Record<string, string>
    expect(payload.output_key).toBe('energy_wh')
    expect(payload.expression).toBe('(voltage * current) / 1000')
    expect(payload.device_template_id).toBe('tpl-2')
  })

  it('forwards toggle events with the target state and updates the local row', async () => {
    hoisted.toggleCalculatedField.mockResolvedValue({ data: null })
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    await (wrapper.vm.$.setupState.handleToggle as (row: unknown, next: boolean) => Promise<void>)(
      state.tableData[0],
      true
    )
    await flushPromises()

    expect(hoisted.toggleCalculatedField).toHaveBeenCalledWith('cf-1', true)
    expect(state.tableData[0].enabled).toBe(true)
    expect(hoisted.getCalculatedFields).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalled()
  })

  it('stays empty when the backend returns no rows', async () => {
    hoisted.getCalculatedFields.mockResolvedValue({ data: { total: 0, list: [] } })
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    expect(hoisted.getCalculatedFields).toHaveBeenCalledTimes(1)
    expect(state.tableData).toHaveLength(0)
    expect(state.pagination.itemCount).toBe(0)

    await (wrapper.vm.$.setupState.getTableData as () => Promise<void>)()
    await flushPromises()
    expect(state.tableData).toHaveLength(0)
    expect(state.pagination.itemCount).toBe(0)
  })

  it('routes edits through update with the stored id and upsert payload', async () => {
    hoisted.updateCalculatedField.mockResolvedValue({ data: null })
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    const openEditModal = wrapper.vm.$.setupState.openEditModal as (row: unknown) => void
    openEditModal({
      id: 'cf-1',
      name: 'Power',
      device_template_id: 'tpl-1',
      output_key: 'power_w',
      expression: 'voltage * current',
      remark: 'demo'
    })
    state.formModel.expression = 'voltage + current'
    await (wrapper.vm.$.setupState.handleSubmit as () => Promise<void>)()
    await flushPromises()

    expect(hoisted.updateCalculatedField).toHaveBeenCalledTimes(1)
    const [id, payload] = hoisted.updateCalculatedField.mock.calls[0] as [string, Record<string, string>]
    expect(id).toBe('cf-1')
    expect(payload.output_key).toBe('power_w')
    expect(payload.expression).toBe('voltage + current')
  })
})
